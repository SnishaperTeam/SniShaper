package handler

import (
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"reflect"
	"runtime"
	"sync"
	"time"
	"unsafe"
)

// SessionResponse 定义了 RESTful API 传输 TLS 会话凭证的数据结构
type SessionResponse struct {
	Ticket      string `json:"ticket"`       // Hex 编码的 Session Ticket 密文
	Secret      string `json:"secret"`       // Hex 编码的 Session Master Secret
	CipherSuite uint16 `json:"cipher_suite"` // 密码套件 ID，例如 0xc02b
	Version     uint16 `json:"version"`      // TLS 协议版本，例如 0x0303 (TLS 1.2)
	TargetIP    string `json:"target_ip"`    // 服务端无污染解析出的目标网站真实 IP
}

// InterceptSessionCache 缓存拦截器，用于在服务端代为握手时捕获 Session Ticket
type InterceptSessionCache struct {
	mu    sync.Mutex
	state *tls.ClientSessionState
}

func (c *InterceptSessionCache) Get(sessionKey string) (*tls.ClientSessionState, bool) {
	// 服务端代握手时我们总是以完整握手拉取新 Ticket，所以直接返回 nil
	return nil, false
}

func (c *InterceptSessionCache) Put(sessionKey string, cs *tls.ClientSessionState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = cs
	log.Printf("[Server Cache] Intercepted ClientSessionState for %q", sessionKey)
}

func (c *InterceptSessionCache) GetState() *tls.ClientSessionState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// SetUnexportedField 利用反射和 unsafe 强行修改私有字段的值
func SetUnexportedField(obj interface{}, fieldName string, value interface{}) {
	v := reflect.ValueOf(obj).Elem()
	fieldVal := v.FieldByName(fieldName)
	if !fieldVal.IsValid() {
		log.Printf("[Warn] field %s not found in struct", fieldName)
		return
	}
	ptr := unsafe.Pointer(fieldVal.UnsafeAddr())

	switch val := value.(type) {
	case []byte:
		*(*[]byte)(ptr) = val
	case uint16:
		*(*uint16)(ptr) = val
	case bool:
		*(*bool)(ptr) = val
	case uint64:
		*(*uint64)(ptr) = val
	default:
		log.Printf("[Warn] unsupported type %T for field %s", value, fieldName)
	}
}

// ExtractClientSessionState 从 tls.ClientSessionState 实例中利用反射和 unsafe.Pointer
// 提取出私有的 sessionTicket 字节切片，以及内部 tls.SessionState 结构体中的 secret (Master Secret)、cipherSuite 和 version。
func ExtractClientSessionState(cs *tls.ClientSessionState) (ticket []byte, secret []byte, cipherSuite uint16, version uint16, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in reflection: %v", r)
		}
	}()

	v := reflect.ValueOf(cs).Elem()

	// 1. 提取 session 字段 (*tls.SessionState)
	sessionField := v.FieldByName("session")
	if !sessionField.IsValid() {
		err = fmt.Errorf("field 'session' not found in ClientSessionState")
		return
	}

	ptr := unsafe.Pointer(sessionField.UnsafeAddr())
	statePtr := *(**tls.SessionState)(ptr)
	if statePtr == nil {
		err = fmt.Errorf("session pointer in ClientSessionState is nil")
		return
	}

	sv := reflect.ValueOf(statePtr).Elem()

	// 2. 从 SessionState 提取 secret []byte (Master Secret)
	secretField := sv.FieldByName("secret")
	if secretField.IsValid() {
		sptr := unsafe.Pointer(secretField.UnsafeAddr())
		secret = *(*[]byte)(sptr)
	}

	// 3. 提取 ticket []byte (Session Ticket)，兼容 Go 1.22（外层）与 Go 1.25（内层）
	ticketField := v.FieldByName("ticket") // 先尝试从外层 ClientSessionState 提取
	if !ticketField.IsValid() {
		ticketField = sv.FieldByName("ticket") // 找不到再试从内层 SessionState 提取
	}
	if ticketField.IsValid() {
		tptr := unsafe.Pointer(ticketField.UnsafeAddr())
		ticket = *(*[]byte)(tptr)
	}

	// 4. 从 SessionState 提取 cipherSuite uint16
	csField := sv.FieldByName("cipherSuite")
	if csField.IsValid() {
		cptr := unsafe.Pointer(csField.UnsafeAddr())
		cipherSuite = *(*uint16)(cptr)
	}

	// 5. 从 SessionState 提取 version uint16
	verField := sv.FieldByName("version")
	if verField.IsValid() {
		vptr := unsafe.Pointer(verField.UnsafeAddr())
		version = *(*uint16)(vptr)
	}

	if len(ticket) == 0 || len(secret) == 0 {
		err = fmt.Errorf("extracted empty ticket or secret (ticket_len=%d, secret_len=%d)", len(ticket), len(secret))
	}
	return
}

// Handler 是 Vercel Go Serverless Function 的入口函数
func Handler(w http.ResponseWriter, r *http.Request) {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	target := r.URL.Query().Get("target")
	if target == "" {
		http.Error(w, "Query parameter 'target' is required (e.g. google.com:443)", http.StatusBadRequest)
		return
	}

	customIP := r.URL.Query().Get("ip")

	log.Printf("[Vercel Handler] Request received to fetch credentials for target: %s (custom ip: %s)", target, customIP)

	// 1. 解析目标地址的 Host 和 Port
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		// 没有指定端口，默认为 443
		host = target
		port = "443"
	}

	var targetIP string
	if customIP != "" {
		targetIP = customIP
		log.Printf("[Vercel Handler] Using client-specified target IP: %s", targetIP)
	} else {
		// 2. 解析目标域名公网 IP
		log.Printf("[Vercel Handler] Resolving IP for host: %s...", host)
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			log.Printf("[Vercel Handler] DNS resolution failed for %s: %v", host, err)
			http.Error(w, "DNS resolution failed for target host", http.StatusBadGateway)
			return
		}
		targetIP = ips[0].String()
		log.Printf("[Vercel Handler] Resolved %s to IP: %s", host, targetIP)
	}

	// 3. 服务端建立与该物理 IP 端口的临时 TLS 1.2 连接，拦截 Ticket
	cache := &InterceptSessionCache{}
	tlsConfig := &tls.Config{
		ServerName:         host,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS12,
		ClientSessionCache: cache,
		InsecureSkipVerify: true, // 忽略直接使用 IP 连接时的证书匹配校验
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	connectAddr := net.JoinHostPort(targetIP, port)
	log.Printf("[Vercel Handler] Initiating handshake with %s...", connectAddr)

	conn, err := tls.DialWithDialer(dialer, "tcp", connectAddr, tlsConfig)
	if err != nil {
		log.Printf("[Vercel Handler] TLS Handshake failed: %v", err)
		http.Error(w, "Failed to connect and handshake with target server", http.StatusBadGateway)
		return
	}
	conn.Close()

	// 4. 判断是否成功拦截到 Session Ticket
	state := cache.GetState()
	if state == nil {
		log.Printf("[Vercel Handler] Error: Target server did not return a session ticket")
		http.Error(w, "Target server did not issue a session ticket", http.StatusBadGateway)
		return
	}

	// 5. 反射提取私有数据
	ticket, secret, cipherSuite, version, err := ExtractClientSessionState(state)
	if err != nil {
		log.Printf("[Vercel Handler] Error extracting session fields: %v", err)
		var fields []string
		if state != nil {
			t := reflect.TypeOf(state).Elem()
			for i := 0; i < t.NumField(); i++ {
				fields = append(fields, fmt.Sprintf("%d: Name=%s, Type=%v", i, t.Field(i).Name, t.Field(i).Type))
			}
		}
		errMsg := fmt.Sprintf("Failed to extract session: %v | GoVersion: %s | ClientSessionStateFields: %v", err, runtime.Version(), fields)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}

	// 6. 构造 JSON 响应
	resp := SessionResponse{
		Ticket:      hex.EncodeToString(ticket),
		Secret:      hex.EncodeToString(secret),
		CipherSuite: cipherSuite,
		Version:     version,
		TargetIP:    targetIP,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("[Vercel Handler] Error writing response JSON: %v", err)
	} else {
		log.Printf("[Vercel Handler] Session credentials dispatched successfully for %s", host)
	}
}
