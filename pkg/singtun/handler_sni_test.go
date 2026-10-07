package singtun

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	M "github.com/sagernet/sing/common/metadata"
)

// ---- T016 集成用例：NewConnectionEx 端到端（fake proxy listener + net.Pipe） ----

// runFlowThroughHandler 把一条客户端流推入 NewConnectionEx，返回代理侧观察到的
// CONNECT 目标主机与隧道首条 TLS record。setup 在内部 Handler 上配置决策回调，
// 并返回该流的目标地址（可用 h.fakeIP 注册 fake-ip 域名目标）。
func runFlowThroughHandler(t *testing.T, setup func(h *Handler) M.Socksaddr, clientHello []byte) (connectHost string, firstRecord []byte) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()

	h := NewHandler(l.Addr().String(), nil, func(string) {})
	destination := setup(h)

	client, serverEnd := net.Pipe()
	defer client.Close()
	src := M.SocksaddrFrom(netip.MustParseAddr("10.0.0.1"), 50000)

	go func() {
		_, _ = client.Write(clientHello)
	}()
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.NewConnectionEx(context.Background(), serverEnd, src, destination, nil)
	}()

	px, err := l.Accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	defer px.Close()
	_ = px.SetDeadline(time.Now().Add(5 * time.Second))

	br := bufio.NewReader(px)
	line, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read CONNECT line: %v", err)
	}
	// "CONNECT example.com:443 HTTP/1.1\r\n"
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) != 3 || fields[0] != "CONNECT" {
		t.Fatalf("unexpected CONNECT line: %q", line)
	}
	hostPort := fields[1]
	if i := strings.LastIndex(hostPort, ":"); i >= 0 {
		connectHost = hostPort[:i]
	} else {
		connectHost = hostPort
	}

	// 消费 CONNECT 请求的剩余头（Host 行 + 空行），再回 200。
	for {
		hl, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("read CONNECT headers: %v", err)
		}
		if hl == "\r\n" || hl == "\n" {
			break
		}
	}

	if _, err := px.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		t.Fatalf("write 200: %v", err)
	}

	// 读隧道首段数据：TLS record（首字节 0x16）按头内长度读全；
	// 非 TLS 直通字节则按原始请求长度读全。
	hdr := make([]byte, 5)
	if _, err := io.ReadFull(br, hdr); err != nil {
		t.Fatalf("read tunnel first bytes: %v", err)
	}
	rest := 0
	if hdr[0] == 0x16 {
		rest = int(hdr[3])<<8 | int(hdr[4])
	} else {
		rest = len(clientHello) - len(hdr)
	}
	body := make([]byte, rest)
	if _, err := io.ReadFull(br, body); err != nil {
		t.Fatalf("read tunnel payload: %v", err)
	}
	firstRecord = append(hdr, body...)
	return connectHost, firstRecord
}

// fakeIPDest 在 Handler 的 fake-ip 池注册域名并返回对应目标地址。
func fakeIPDest(h *Handler, domain string, port uint16) M.Socksaddr {
	addr, ok := h.fakeIP.Create(domain)
	if !ok {
		panic("fake-ip create failed: " + domain)
	}
	return M.SocksaddrFrom(addr, port)
}

// 用例 1：IP 目标 + 决策命中 → CONNECT 用嗅探域名，隧道内的 ClientHello
// 已被重写为新 SNI。
func TestNewConnectionRewritesSNIWhenDeciderMatches(t *testing.T) {
	hello := buildClientHelloRecord("example.com", nil)

	connectHost, rec := runFlowThroughHandler(t, func(h *Handler) M.Socksaddr {
		h.SetSNIDecider(func(sni string) (string, bool) {
			if sni == "example.com" {
				return "cdn.example.org", true
			}
			return "", false
		})
		return M.SocksaddrFrom(netip.MustParseAddr("93.184.216.34"), 443)
	}, hello)

	if connectHost != "example.com" {
		t.Fatalf("CONNECT must use sniffed domain, got %q", connectHost)
	}
	info, err := ParseClientHelloRecord(rec)
	if err != nil {
		t.Fatalf("tunneled record must parse: %v", err)
	}
	if info.SNI != "cdn.example.org" {
		t.Fatalf("tunneled ClientHello must carry rewritten SNI, got %q", info.SNI)
	}
}

// 用例 2：决策回调为 nil → 行为等同现状：IP 场景仍嗅探 SNI 用于 CONNECT，
// ClientHello 原样回放（不丢字节、不重写）。
func TestNewConnectionNilDeciderKeepsCurrentBehavior(t *testing.T) {
	hello := buildClientHelloRecord("example.com", nil)

	connectHost, rec := runFlowThroughHandler(t, func(h *Handler) M.Socksaddr {
		return M.SocksaddrFrom(netip.MustParseAddr("93.184.216.34"), 443)
	}, hello)

	if connectHost != "example.com" {
		t.Fatalf("CONNECT must use sniffed domain, got %q", connectHost)
	}
	info, err := ParseClientHelloRecord(rec)
	if err != nil {
		t.Fatalf("tunneled record must parse: %v", err)
	}
	if info.SNI != "example.com" {
		t.Fatalf("nil decider must not rewrite, got %q", info.SNI)
	}
}

// 用例 3：ECH 流量即使决策命中也跳过重写（D4）：外层 SNI 用于 CONNECT，
// ClientHello 原样透传。
func TestNewConnectionSkipsRewriteForECH(t *testing.T) {
	ech := testExt{typ: 0xfe0d, data: []byte{0x01, 0x00, 0x02, 0x03}}
	hello := buildClientHelloRecord("outer.example.com", []testExt{ech})

	connectHost, rec := runFlowThroughHandler(t, func(h *Handler) M.Socksaddr {
		h.SetSNIDecider(func(string) (string, bool) { // 无条件命中，验证 ECH 防线
			return "evil.example.org", true
		})
		return M.SocksaddrFrom(netip.MustParseAddr("93.184.216.34"), 443)
	}, hello)

	if connectHost != "outer.example.com" {
		t.Fatalf("CONNECT must use outer SNI, got %q", connectHost)
	}
	info, err := ParseClientHelloRecord(rec)
	if err != nil {
		t.Fatalf("tunneled record must parse: %v", err)
	}
	if !info.HasECH {
		t.Fatal("ECH marker must survive passthrough")
	}
	if info.SNI != "outer.example.com" {
		t.Fatalf("ECH flow must pass through unrewritten, got %q", info.SNI)
	}
}

// 用例 4：惰性嗅探（D5）——fake-ip 域名目标且决策无重写时，不读首包、
// 不等待嗅探超时：非 TLS 字节直通，CONNECT 直接用反查域名。
func TestNewConnectionLazySniffSkipsNonTLSWhenNoRewrite(t *testing.T) {
	hello := []byte("GET / HTTP/1.1\r\nHost: plain.example\r\n\r\n")

	start := time.Now()
	connectHost, rec := runFlowThroughHandler(t, func(h *Handler) M.Socksaddr {
		h.SetSNIDecider(func(string) (string, bool) { return "", false }) // 不重写
		return fakeIPDest(h, "plain.example", 80)
	}, hello)
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("flow must not wait for sniff timeout, took %v", elapsed)
	}
	if connectHost != "plain.example" {
		t.Fatalf("CONNECT must use fake-ip reverse-lookup domain, got %q", connectHost)
	}
	if string(rec[:len("GET / HTTP")]) != "GET / HTTP" {
		t.Fatalf("non-TLS bytes must pass through untouched, got %q", string(rec[:16]))
	}
}
