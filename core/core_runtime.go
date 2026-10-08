package core

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"

	"snishaper/common"
	"snishaper/pkg/certmanager"
	"snishaper/pkg/netiface"
	"snishaper/pkg/singtun"
	"snishaper/proxy"
)

// logBufPool provides reusable byte buffers for log formatting
// to reduce memory allocation and GC pressure in high-frequency logging.
var logBufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 0, 256)
		return &buf
	},
}

type coreRuntime struct {
	mu                sync.RWMutex
	execPath          string
	execDir           string
	certPath          string
	ruleManager       *proxy.RuleManager
	proxyServer       *proxy.ProxyServer
	nativeTUN         *singtun.Manager
	certManager       *certmanager.CertManager
	logBuffer         *common.RingLogWriter
	logCaptureMu      sync.RWMutex
	logCaptureEnabled bool
	proxyOpMu         sync.Mutex
	tunOpMu           sync.Mutex
	tunStateMu        sync.RWMutex
	tunStarting       bool
	tunStartErr       string
	routeEventsMu     sync.Mutex
	routeEvents       []RouteEvent
	rulesWatchStop    func()
}

func newCoreRuntime() (*coreRuntime, error) {
	execPath, err := os.Executable()
	if err != nil {
		return nil, err
	}
	execDir := filepath.Dir(execPath)
	settingsPath := common.ConfigSettingsPath(execDir)
	rulesPath := common.ConfigRulesPath(execDir)

	ruleManager := proxy.NewRuleManager(settingsPath, rulesPath)
	if err := ruleManager.LoadConfig(); err != nil {
		return nil, err
	}

	port := ruleManager.GetListenPort()
	if port == "" {
		port = "8080"
	}

	r := &coreRuntime{
		execPath:    execPath,
		execDir:     execDir,
		certPath:    common.ConfigCertDir(execDir),
		ruleManager: ruleManager,
		proxyServer: proxy.NewProxyServer("127.0.0.1:" + port),
		logBuffer:   common.NewRingLogWriter(5000),
	}
	// Initialize nativeTUN with DoH resolver
	r.nativeTUN = singtun.NewManager(r.proxyServer.GetDoHResolver(), r.appendLog)

	if err := r.start(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *coreRuntime) start() error {
	r.setupLogger()
	var err error
	r.certManager, err = certmanager.InitCertManager(r.certPath)
	if err != nil {
		r.appendLog("[core] Failed to init cert manager: " + err.Error())
	}
	r.proxyServer.SetRuleManager(r.ruleManager)
	r.proxyServer.UpdateCloudflareIPPool(r.ruleManager.GetCloudflareConfig().PreferredIPs)
	r.proxyServer.SetCertGenerator(r.certManager)
	r.proxyServer.SetLogCallback(r.appendLog)
	r.ruleManager.InitAutoRouter(r.proxyServer.GetDoHResolver())

	r.ruleManager.SetRouteEventCallback(func(domain, mode string) {
		r.routeEventsMu.Lock()
		defer r.routeEventsMu.Unlock()
		r.routeEvents = append(r.routeEvents, RouteEvent{Domain: domain, Mode: mode})
		if len(r.routeEvents) > 200 {
			// Create new slice to release underlying array memory
			newSlice := make([]RouteEvent, 100)
			copy(newSlice, r.routeEvents[len(r.routeEvents)-100:])
			r.routeEvents = newSlice
		}
	})

	if stop, err := r.ruleManager.WatchRulesFile(func() {
		if err := r.reloadConfig(); err != nil {
			r.appendLog("[core] rules file changed but reload failed: " + err.Error())
			return
		}
		r.appendLog("[core] rules file changed, reloaded automatically")
	}); err != nil {
		r.appendLog("[core] rules file watch unavailable: " + err.Error())
	} else {
		r.rulesWatchStop = stop
	}

	r.appendLog("[core] runtime ready")
	return nil
}

func (r *coreRuntime) shutdown() {
	if r.rulesWatchStop != nil {
		r.rulesWatchStop()
		r.rulesWatchStop = nil
	}
	if r.nativeTUN != nil {
		//进程退出路径用 Shutdown：这是唯一允许终态关闭 Handler 的地方，
		// 可确保 TUN 停止后残留派发的连接也无法再生效。
		_ = r.nativeTUN.Shutdown()
	}
	_ = r.proxyServer.Stop()
	r.appendLog("[core] runtime stopped")
}

func (r *coreRuntime) reloadConfig() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.ruleManager.LoadConfig(); err != nil {
		return err
	}
	port := r.ruleManager.GetListenPort()
	if port == "" {
		port = "8080"
	}
	if err := r.proxyServer.SetListenAddr("127.0.0.1:" + port); err != nil {
		return err
	}
	r.proxyServer.SetRuleManager(r.ruleManager)
	r.proxyServer.UpdateCloudflareIPPool(r.ruleManager.GetCloudflareConfig().PreferredIPs)
	r.proxyServer.SetCertGenerator(r.certManager)
	r.ruleManager.InitAutoRouter(r.proxyServer.GetDoHResolver())
	// nativeTUN doesn't need restart on config reload
	r.appendLog("[core] config reloaded")
	return nil
}

func (r *coreRuntime) reloadCertificate() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cm, err := certmanager.InitCertManager(r.certPath)
	if err != nil {
		return err
	}
	r.certManager = cm
	r.proxyServer.SetCertGenerator(r.certManager)
	r.proxyServer.ClearCertCache()
	r.appendLog("[core] certificate reloaded")
	return nil
}

func (r *coreRuntime) setupLogger() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(io.MultiWriter(r.logBuffer, os.Stdout))
}

func (r *coreRuntime) appendLog(message string) {
	if r.logBuffer == nil {
		r.logBuffer = common.NewRingLogWriter(500)
	}
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return
	}

	// Get buffer from pool to reduce allocation
	buf := logBufPool.Get().(*[]byte)
	*buf = (*buf)[:0] // Reset buffer

	// Write timestamp directly to buffer
	*buf = time.Now().AppendFormat(*buf, "2006/01/02 15:04:05.000000")
	*buf = append(*buf, ' ')
	*buf = append(*buf, trimmed...)
	if !strings.HasSuffix(trimmed, "\n") {
		*buf = append(*buf, '\n')
	}

	_, _ = r.logBuffer.Write(*buf)
	logBufPool.Put(buf) // Return to pool
}

func (r *coreRuntime) isLogCaptureEnabled() bool {
	r.logCaptureMu.RLock()
	defer r.logCaptureMu.RUnlock()
	return r.logCaptureEnabled
}

func (r *coreRuntime) startLogCapture() {
	r.logCaptureMu.Lock()
	r.logCaptureEnabled = true
	r.logCaptureMu.Unlock()
	if r.logBuffer != nil {
		r.logBuffer.Clear()
	}
	r.appendLog("[core] log capture started")
}

func (r *coreRuntime) stopLogCapture() {
	r.appendLog("[core] log capture stopping")
	r.logCaptureMu.Lock()
	r.logCaptureEnabled = false
	r.logCaptureMu.Unlock()
}

func (r *coreRuntime) recentLogs(limit int) string {
	if r.logBuffer == nil {
		return ""
	}
	return strings.Join(r.logBuffer.Snapshot(limit), "\n")
}

func (r *coreRuntime) clearLogs() {
	if r.logBuffer != nil {
		r.logBuffer.Clear()
	}
	r.appendLog("[core] logs cleared")
}

func (r *coreRuntime) startProxy() error {
	r.proxyOpMu.Lock()
	defer r.proxyOpMu.Unlock()

	if r.proxyServer.IsRunning() {
		return nil
	}

	originalPort := r.getListenPort()
	if originalPort == 0 {
		originalPort = 8080
	}
	availablePort, err := proxy.EnsurePortAvailable(originalPort, []string{"snishaper", "usque"})
	if err != nil {
		availablePort = originalPort
	}
	if availablePort != originalPort {
		if err := r.setListenPort(availablePort); err != nil {
			return err
		}
	}
	if err := r.proxyServer.Start(); err != nil {
		return err
	}
	addr := r.proxyServer.GetListenAddr()
	if err := waitForListen(addr, 2*time.Second); err != nil {
		_ = r.proxyServer.Stop()
		return fmt.Errorf("proxy started but not listening on %s: %w", addr, err)
	}
	r.appendLog("[core] proxy started")
	return nil
}

func (r *coreRuntime) stopProxy() error {
	r.proxyOpMu.Lock()
	defer r.proxyOpMu.Unlock()
	var errs []error
	if err := r.proxyServer.Stop(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errorsJoin(errs...)
	}
	r.appendLog("[core] proxy stopped")
	return nil
}

func (r *coreRuntime) startTUN() (err error) {
	// 串行化 start/stop：core_api 的 TUN 请求都在各自的 goroutine 里执行，
	// 没有互斥时连点会并发进入 singtun.Manager 的锁，表现为界面长时间无响应。
	r.tunOpMu.Lock()
	defer r.tunOpMu.Unlock()

	r.setTUNStartState(true, nil)
	defer func() {
		r.setTUNStartState(false, err)
	}()

	if !isProcessElevated() {
		err = fmt.Errorf("TUN requires administrator privileges on Windows; please restart SniShaper as administrator")
		return err
	}
	if !r.proxyServer.IsRunning() {
		r.appendLog("[core] proxy not running, starting proxy before TUN")
		if err = r.startProxy(); err != nil {
			return fmt.Errorf("start proxy before TUN: %w", err)
		}
	}
	if r.nativeTUN == nil {
		err = fmt.Errorf("native TUN manager is not initialized")
		return err
	}
	listenPort := r.currentListenPort()
	if listenPort == "" {
		err = fmt.Errorf("proxy listen port is empty")
		return err
	}
	tunCfg := r.ruleManager.GetTUNConfig()
	r.appendLog("[core] startTUN: calling nativeTUN.Start with proxy=" + "127.0.0.1:" + listenPort)
	proxyAddr := "127.0.0.1:" + listenPort
	if err = r.nativeTUN.Start(tunCfg, proxyAddr); err != nil {
		r.appendLog("[core] startTUN: nativeTUN.Start failed: " + err.Error())
		return err
	}
	r.appendLog("[core] native sing-tun started, status=" + fmt.Sprintf("%v", r.nativeTUN.Status().Running))
	// 通知 ProxyServer 启用 TUN 模式，出站连接绑物理网卡。
	// 出站网卡在 TUN 启动之后才配置：此时网卡列表已包含 SniShaper 自己的虚拟
	// 网卡，必须让 netiface 重新扫描，否则可能把 TUN 自己选成出站网卡。
	// 注意 SetTUNMode 必须在 Start 成功之后：Start 失败时 TUN 并不存在，
	// 此时打开 TUN 模式会让所有出站连接去找一张并不存在的网卡。
	r.proxyServer.SetTUNMode(true)
	netiface.InvalidateCache()
	if binding, selErr := netiface.Select(netiface.FamilyIPv4, tunCfg.InterfaceConfig(), r.appendLog); selErr == nil {
		r.appendLog("[core] TUN outbound interface: " + binding.Describe())
	}
	// TUN 数据面自检：通过 TUN 发送 DNS 查询，验证 gvisor 栈正常工作。
	// 解决 gvisor 数据面静默失效时（网卡存在但流量不通）无任何错误日志的问题。
	if err := verifyTUNDataPlane(singtun.TUNGateway4(), 2*time.Second); err != nil {
		r.appendLog("[error] TUN data plane check failed: " + err.Error())
	} else {
		r.appendLog("[core] TUN data plane check passed")
	}
	return nil
}

// verifyTUNDataPlane 通过 TUN 接口发送 DNS 查询（绑定 TUN 自身地址作为源），
// 验证 gvisor 数据面是否正常。查询会被 gvisor 劫持并由 Handler 返回 fake-ip 响应。
func verifyTUNDataPlane(tunIP string, timeout time.Duration) error {
	msg := new(dns.Msg)
	msg.Id = dns.Id()
	msg.RecursionDesired = true
	msg.Question = []dns.Question{{Name: "example.com.", Qtype: dns.TypeA, Qclass: dns.ClassINET}}
	query, err := msg.Pack()
	if err != nil {
		return err
	}

	laddr, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(tunIP, "0"))
	if err != nil {
		return err
	}
	conn, err := net.DialUDP("udp4", laddr, &net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 53})
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.Write(query); err != nil {
		return err
	}

	deadline := time.Now().Add(timeout)
	buf := make([]byte, 512)
	for time.Now().Before(deadline) {
		conn.SetReadDeadline(deadline)
		n, err := conn.Read(buf)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				break
			}
			return err
		}
		resp := new(dns.Msg)
		if err := resp.Unpack(buf[:n]); err != nil || resp.Id != msg.Id {
			continue
		}
		return nil
	}
	return fmt.Errorf("no DNS response through TUN within %v (data plane may be down)", timeout)
}

func (r *coreRuntime) stopTUN() error {
	// 与 startTUN 共用一把锁，保证同一时刻只有一个 TUN 生命周期操作在跑。
	r.tunOpMu.Lock()
	defer r.tunOpMu.Unlock()

	r.setTUNStartState(false, nil)
	if r.nativeTUN == nil {
		return fmt.Errorf("native TUN manager is not initialized")
	}
	// 顺序要求：必须先停 TUN 再退出 TUN 模式。
	// 反过来的话，设备还在、默认路由还指向隧道，但出站 socket 已经不再绑物理网卡，
	// 这段时间内所有出站流量都会被 TUN 重新捕获，形成自激回环。
	if err := r.nativeTUN.Stop(); err != nil {
		return err
	}
	// 通知 ProxyServer 退出 TUN 模式
	r.proxyServer.SetTUNMode(false)
	// TUN 网卡已消失，重新扫描网卡列表，避免下次出站仍引用已释放的接口索引。
	netiface.InvalidateCache()
	r.appendLog("[core] native sing-tun stopped")
	return nil
}

func (r *coreRuntime) getTUNStatus() proxy.TUNStatus {
	var status proxy.TUNStatus
	if r.nativeTUN != nil {
		status = r.nativeTUN.Status()
	} else {
		status = proxy.TUNStatus{
			Supported: false,
			Enabled:   false,
			Running:   false,
			Message:   "Native TUN is not initialized",
		}
	}

	r.tunStateMu.RLock()
	starting := r.tunStarting
	startErr := strings.TrimSpace(r.tunStartErr)
	r.tunStateMu.RUnlock()

	if status.Running {
		status.Enabled = true
		return status
	}
	status.Enabled = false
	if starting {
		return status
	}
	if startErr != "" {
		status.Message = startErr
	}
	return status
}

func (r *coreRuntime) setTUNStartState(starting bool, err error) {
	r.tunStateMu.Lock()
	defer r.tunStateMu.Unlock()
	r.tunStarting = starting
	if starting {
		r.tunStartErr = ""
		return
	}
	if err != nil {
		r.tunStartErr = strings.TrimSpace(err.Error())
		return
	}
	r.tunStartErr = ""
}

func (r *coreRuntime) failTUNStart(err error) {
	r.setTUNStartState(false, err)
	if err != nil {
		r.appendLog("[core] TUN panic: " + err.Error())
		r.appendLog(string(debug.Stack()))
	}
}

func (r *coreRuntime) getListenPort() int {
	addr := r.proxyServer.GetListenAddr()
	var port int
	fmt.Sscanf(addr, "127.0.0.1:%d", &port)
	return port
}

func (r *coreRuntime) currentListenPort() string {
	addr := strings.TrimSpace(r.proxyServer.GetListenAddr())
	if addr != "" {
		if _, port, err := net.SplitHostPort(addr); err == nil && strings.TrimSpace(port) != "" {
			return strings.TrimSpace(port)
		}
	}
	return strings.TrimSpace(r.ruleManager.GetListenPort())
}

func (r *coreRuntime) setListenPort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port number: %d", port)
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if err := r.proxyServer.SetListenAddr(addr); err != nil {
		return err
	}
	r.ruleManager.SetListenPort(fmt.Sprintf("%d", port))
	return r.ruleManager.SaveConfig()
}

func waitForListen(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timeout")
	}
	return lastErr
}

func errorsJoin(errs ...error) error {
	var filtered []error
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	msgs := make([]string, 0, len(filtered))
	for _, err := range filtered {
		msgs = append(msgs, err.Error())
	}
	return errors.New(strings.Join(msgs, "; "))
}

func (r *coreRuntime) popRouteEvents() []RouteEvent {
	r.routeEventsMu.Lock()
	defer r.routeEventsMu.Unlock()
	if len(r.routeEvents) == 0 {
		return nil
	}
	events := r.routeEvents
	r.routeEvents = nil
	return events
}
