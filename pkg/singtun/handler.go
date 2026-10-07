package singtun

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"snishaper/common"

	"github.com/miekg/dns"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"snishaper/pkg/dohresolver"
	"snishaper/pkg/netiface"
)

// ponytail: JudgeFlow returns ActionAccept unconditionally; add flow-level
// bypass logic if per-flow direct routing is ever needed.

// Handler 实现 sing-tun 的 Handler 接口
// 负责将 TUN 流量转发到 SniShaper Proxy
type Handler struct {
	proxyAddr   string
	resolver    *dohresolver.FailoverResolver
	fakeIP      *FakeIPStore
	logf        func(string)
	logger      *slog.Logger
	ifaceConfig netiface.Config
	mu          sync.Mutex
	live        map[net.Conn]struct{}
	livePacket  map[N.PacketConn]struct{}
	closed      bool
	wg          sync.WaitGroup
}

// track 登记一个 TCP 连接。返回 false 表示 Handler 已关闭，调用方必须
// 自行关闭该连接 —— 否则连接会落进已被清空的 map，变成无人回收的泄漏。
// wg.Add 必须在锁内与 track 同步完成：若在 track 返回后再 Add，
// Close 可能已经进入 wg.Wait，Add 会触发 "concurrent Add and Wait" panic。
func (h *Handler) track(c net.Conn) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return false
	}
	if h.live == nil {
		h.live = make(map[net.Conn]struct{})
	}
	h.live[c] = struct{}{}
	h.wg.Add(1)
	return true
}

func (h *Handler) untrack(c net.Conn) {
	h.mu.Lock()
	_, ok := h.live[c]
	delete(h.live, c)
	h.mu.Unlock()
	if ok {
		h.wg.Done()
	}
}

func (h *Handler) trackPacketConn(c N.PacketConn) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return false
	}
	if h.livePacket == nil {
		h.livePacket = make(map[N.PacketConn]struct{})
	}
	h.livePacket[c] = struct{}{}
	h.wg.Add(1)
	return true
}

func (h *Handler) untrackPacketConn(c N.PacketConn) {
	h.mu.Lock()
	_, ok := h.livePacket[c]
	delete(h.livePacket, c)
	h.mu.Unlock()
	if ok {
		h.wg.Done()
	}
}

// Close 终止 Handler：置 closed 后拒绝新连接并回收全部存量连接。
// 这是终态，调用后 Handler 不可复用。
func (h *Handler) Close() {
	h.mu.Lock()
	h.closed = true
	h.mu.Unlock()
	h.Release()
	h.mu.Lock()
	h.live = nil
	h.livePacket = nil
	h.mu.Unlock()
}

// releaseMaxRounds 是 Release 快照收敛循环的最大轮次（设计决策 D6）。
// Release 不置 closed，理论上新连接可能在快照间隙持续进入；无上限的
// 收敛循环在极端场景下会让 Stop 永不返回。到顶后记录剩余连接数并返回。
const releaseMaxRounds = 5

// Release 回收全部存量连接并等待转发 goroutine 退出，但不复用 closed 标记。
// Manager 的 releaseLocked 走这条路径：设备已关闭但 Handler 语义上仍可继续
// 接收派发，Start 失败回滚时也无需重建 Handler。
//
// 因为不置 closed，snapshot 之后仍可能有新连接被 track，所以按轮次收敛：
// 每轮重新快照并关闭，直到某一轮快照为空；最多 releaseMaxRounds 轮（D6）。
func (h *Handler) Release() {
	for round := 1; round <= releaseMaxRounds; round++ {
		h.mu.Lock()
		conns := make([]net.Conn, 0, len(h.live))
		for c := range h.live {
			conns = append(conns, c)
		}
		packets := make([]N.PacketConn, 0, len(h.livePacket))
		for c := range h.livePacket {
			packets = append(packets, c)
		}
		h.mu.Unlock()

		for _, c := range conns {
			c.Close()
		}
		for _, c := range packets {
			_ = c.Close()
		}

		// 等所有转发 goroutine 真正退出，避免它们持有的 socket 与 buffer 滞留。
		// 注意不能在 Wait 之前清空 live/livePacket：untrack 依赖 map 里的条目
		// 决定是否 wg.Done，先清空会让计数永远无法归零。
		// untrack 是先 delete 再 Done，因此 Wait 返回时本轮快照的条目必然已清空。
		h.wg.Wait()

		h.mu.Lock()
		remaining := len(h.live) + len(h.livePacket)
		closed := h.closed
		h.mu.Unlock()
		if remaining == 0 {
			return
		}
		if closed {
			// closed 后不会再有新 track，剩余条目必将在下一轮归零；
			// 继续循环收敛而不是原地无限 Wait（D6：关闭路径全部有界）。
			h.logger.Debug("handler release: closed with remaining entries, draining",
				"round", round, "remaining", remaining)
			continue
		}
		// 未 closed 仍有剩余：快照间隙有新连接进入，进入下一轮前记录剩余数。
		h.logger.Warn("handler release round finished with remaining entries",
			"round", round, "remaining", remaining)
	}
	// 轮次上限到顶仍未收敛（D6）：记告警后返回。剩余连接由其自身转发
	// goroutine 结束时释放，随 Handler 生命周期终结回收。
	h.mu.Lock()
	remaining := len(h.live) + len(h.livePacket)
	h.mu.Unlock()
	h.logger.Warn("handler release exceeded max rounds, giving up",
		"rounds", releaseMaxRounds, "remaining", remaining)
}

// liveCounts 返回当前存活的 TCP 连接数与 UDP 会话数（启停 instrumentation 用）。
func (h *Handler) liveCounts() (tcp, udp int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.live), len(h.livePacket)
}

// NewHandler 创建新的 Handler
func NewHandler(proxyAddr string, resolver *dohresolver.FailoverResolver, logf func(string)) *Handler {
	h := &Handler{
		proxyAddr: proxyAddr,
		resolver:  resolver,
		fakeIP:    NewFakeIPStore(),
		logf:      logf,
		logger:    newBridgedLogger(logf),
	}
	h.logger.Info("Handler created", "proxy", proxyAddr)
	return h
}

// interfaceConfig 读取当前出站网卡配置。forwardUDPDirect 与 forwardUDPDirect
// 之外的转发路径会在数据面并发读取，因此必须走锁，不能直接读字段。
func (h *Handler) interfaceConfig() netiface.Config {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ifaceConfig
}

func (h *Handler) SetInterfaceConfig(cfg netiface.Config) {
	h.mu.Lock()
	h.ifaceConfig = cfg
	h.mu.Unlock()
}

func (h *Handler) JudgeFlow(network uint8, source netip.AddrPort, destination netip.AddrPort, firstPacket []byte) tun.FlowVerdict {
	return tun.FlowVerdict{Action: tun.ActionAccept}
}

func (h *Handler) NewDNSPacket(payload []byte, source M.Socksaddr, destination M.Socksaddr, writer N.PacketWriter) {
	h.handleRawDNSPacket(payload, source, destination, writer)
}

// NewConnectionEx 处理新的 TCP 连接
func (h *Handler) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	if shouldHijackDNS(destination) {
		h.serveDNSOverStream(ctx, conn, source, destination, onClose)
		return
	}

	if !h.track(conn) {
		_ = conn.Close()
		if onClose != nil {
			onClose(nil)
		}
		return
	}
	trackedConn := conn

	// 查找真实域名（fake-ip 反查）
	targetHost := h.resolveHost(destination)

	// 浏览器可能用 DoH/系统缓存解析出真实 IP（绕过 TUN 的 fake-ip 劫持），
	// 导致规则按域名匹配失效。此时从 TLS ClientHello 嗅探 SNI 重建域名。
	if net.ParseIP(targetHost) != nil {
		if sni, c := h.sniffTLSSNI(conn); sni != "" {
			h.logger.Info("SNI sniffed", "sni", sni, "was_ip", targetHost)
			targetHost = sni
			conn = c
		}
	}
	h.logger.Debug("TCP flow", "source", source.String(), "destination", destination.String(), "resolved", targetHost)

	// 连接到 ProxyServer
	// loopback (127.0.0.0/8) 已被 Inet4RouteExcludeAddress 排除出 TUN，
	// 连接 127.0.0.1 不会进 TUN，无需绑定物理网卡。
	upstream, err := h.dialProxy()
	if err != nil {
		h.logger.Warn("failed to connect to proxy", "error", err)
		_ = conn.Close()
		h.untrack(trackedConn)
		if onClose != nil {
			onClose(err)
		}
		return
	}

	_ = upstream.SetDeadline(time.Now().Add(15 * time.Second))

	// 发送 CONNECT 请求 (使用域名，不是 IP)
	// 用 net.JoinHostPort 正确处理 IPv6 地址（自动加方括号）
	target := net.JoinHostPort(targetHost, strconv.Itoa(int(destination.Port)))
	connectReq := "CONNECT " + target + " HTTP/1.1\r\nHost: " + target + "\r\n\r\n"
	h.logger.Debug("CONNECT request", "request", connectReq)
	if _, err := upstream.Write([]byte(connectReq)); err != nil {
		h.logger.Warn("failed to send CONNECT", "error", err)
		_ = conn.Close()
		_ = upstream.Close()
		h.untrack(trackedConn)
		if onClose != nil {
			onClose(err)
		}
		return
	}

	// 读取响应 — 用 bufio.Reader 确保读到完整的响应头
	br := bufio.NewReader(upstream)
	statusLine, err := br.ReadString('\n')
	if err != nil {
		h.logger.Warn("failed to read CONNECT response", "error", err)
		_ = conn.Close()
		_ = upstream.Close()
		h.untrack(trackedConn)
		if onClose != nil {
			onClose(err)
		}
		return
	}
	statusLine = strings.TrimRight(statusLine, "\r\n")
	h.logger.Debug("CONNECT response", "status", statusLine)

	// 解析状态码（不能用子串匹配 "200"，状态行其他字段也可能包含 "200"）
	if !isHTTPSuccess(statusLine) {
		// 读取错误响应的剩余内容用于日志
		rest, _ := io.ReadAll(io.LimitReader(br, 4096))
		errMsg := statusLine
		if len(rest) > 0 {
			errMsg += "\r\n" + string(rest)
		}
		h.logger.Warn("CONNECT failed", "detail", errMsg)
		err := fmt.Errorf("proxy connect failed: %s", statusLine)
		_ = conn.Close()
		_ = upstream.Close()
		h.untrack(trackedConn)
		if onClose != nil {
			onClose(err)
		}
		return
	}

	// 读取并丢弃剩余响应头，直到空行（\r\n）
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			h.logger.Warn("failed to read CONNECT headers", "error", err)
			_ = conn.Close()
			_ = upstream.Close()
			h.untrack(trackedConn)
			if onClose != nil {
				onClose(err)
			}
			return
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	_ = upstream.SetDeadline(time.Time{})

	// 用 bufio.Reader 包装 upstream，确保 br 中已缓冲的隧道数据不丢失
	// （代理在 200 响应后可能立即发送 TLS ServerHello 等数据）
	upstream = &bufferedConn{Conn: upstream, br: br}

	// 先登记再起 goroutine：反过来的话，Close 可能在 track 之前跑完，
	// 连接就会落进已清空的 map 里，Close 永远看不到它。
	if !h.track(upstream) {
		_ = conn.Close()
		_ = upstream.Close()
		h.untrack(trackedConn)
		if onClose != nil {
			onClose(nil)
		}
		return
	}

	go func() {
		h.proxyConn(ctx, conn, upstream, onClose)
		h.untrack(trackedConn)
		h.untrack(upstream)
	}()
}

// isHTTPSuccess 检查 HTTP 状态行是否为 2xx
func isHTTPSuccess(statusLine string) bool {
	// statusLine 形如 "HTTP/1.1 200 Connection Established"
	parts := strings.SplitN(statusLine, " ", 3)
	if len(parts) < 2 {
		return false
	}
	return strings.HasPrefix(parts[1], "2")
}

// bufferedConn 用 bufio.Reader 包装 net.Conn，使 Read 先从缓冲区读
type bufferedConn struct {
	net.Conn
	br *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.br.Read(p)
}

// CloseWrite 委托给底层连接，支持半关闭
func (c *bufferedConn) CloseWrite() error {
	type closeWriter interface {
		CloseWrite() error
	}
	if cw, ok := c.Conn.(closeWriter); ok {
		return cw.CloseWrite()
	}
	return c.Conn.Close()
}

// sniffTLSSNI 从 TLS ClientHello 中嗅探 SNI 域名。
// T012：切换到独立解析模块 sni_parser（ECH 检测/分片到达/畸形安全）。
// 行为与旧内联实现兼容：非 TLS / 超时返回空 SNI；返回的连接总是包装后的，
// 已读字节经 prefixConn 完整回放——旧实现经 bufio 消费后不回放，会把整个
// ClientHello 丢掉不发给上游，这是 US3 明确要求修复的丢字节缺陷。
func (h *Handler) sniffTLSSNI(conn net.Conn) (string, net.Conn) {
	info, wrapped := SniffClientHello(conn, 3*time.Second)
	if info.Err != nil {
		h.logger.Debug("SNI sniff failed", "error", info.Err.Error())
	}
	if info.HasECH {
		// D4：ECH 存在时 inner SNI 加密不可见，用外层 SNI 参与规则匹配
		// 并在日志中标记；重写决策层会据此跳过重写。
		h.logger.Info("SNI sniffed with ECH present, using outer SNI for rule matching", "sni", info.SNI)
	}
	return info.SNI, wrapped
}

// resolveHost 解析目标地址的真实域名
// 如果是 fake-ip，反查域名；否则返回 IP
func (h *Handler) resolveHost(destination M.Socksaddr) string {
	addr := destination.Addr

	// 检查是否是 fake-ip
	if h.fakeIP.Contains(addr) {
		if domain, ok := h.fakeIP.Lookup(addr); ok {
			return domain
		}
		// fake-ip 在范围内但反查失败（映射丢失），记录警告
		h.logger.Warn("fake-ip has no domain mapping", "addr", addr.String())
	}

	// 不是 fake-ip，返回原始地址
	return addr.String()
}

// shouldHijackDNS 判定一条 TCP/UDP 流是否为 DNS。
//
// sing-tun 在 Windows/gVisor 下不做任何 DNS 劫持：Options.DNSServerAddress()
// 只被 tun_linux.go 消费，EXP_DisableDNSHijack 只被 Linux 的 nftables 规则读取。
// 因此 NewDNSPacket 在 Windows 上永远不会被调用，53 端口只能在这里接管。
// mihomo 走的是同一条路（listener/sing_tun/dns.go 里的 ShouldHijackDns）。
func shouldHijackDNS(destination M.Socksaddr) bool {
	return destination.Port == 53
}

// NewPacketConnectionEx 处理新的 UDP 连接；53 端口由本层接管 DNS。
func (h *Handler) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	if shouldHijackDNS(destination) {
		h.serveDNSOverPacketConn(ctx, conn, source, destination, onClose)
		return
	}
	h.forwardUDPDirect(ctx, conn, source, destination, onClose)
}

const dnsStreamIdleTimeout = 30 * time.Second

// dnsStreamWriter 把 DNS 应答按 RFC 7766 写回 TCP 流：2 字节大端长度前缀 + 载荷。
func (h *Handler) serveDNSOverStream(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	if !h.track(conn) {
		_ = conn.Close()
		if onClose != nil {
			onClose(nil)
		}
		return
	}

	go func() {
		defer func() {
			_ = conn.Close()
			if onClose != nil {
				onClose(nil)
			}
		}()
		defer h.untrack(conn)

		writer := &dnsStreamWriter{conn: conn}
		lenBuf := make([]byte, 2)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			_ = conn.SetReadDeadline(time.Now().Add(dnsStreamIdleTimeout))
			if _, err := io.ReadFull(conn, lenBuf); err != nil {
				return
			}
			length := int(lenBuf[0])<<8 | int(lenBuf[1])
			if length == 0 {
				return
			}
			payload := make([]byte, length)
			if _, err := io.ReadFull(conn, payload); err != nil {
				return
			}
			h.handleRawDNSPacket(payload, source, destination, writer)
		}
	}()
}

type dnsStreamWriter struct {
	conn net.Conn
}

func (w *dnsStreamWriter) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	payload := buffer.Bytes()
	if len(payload) > 0xFFFF {
		return fmt.Errorf("DNS over TCP response too large: %d bytes", len(payload))
	}
	out := make([]byte, 2+len(payload))
	out[0] = byte(len(payload) >> 8)
	out[1] = byte(len(payload))
	copy(out[2:], payload)
	// 必须设写超时：客户端停止读取时，若查询应答写不进去，
	// Write 会永久阻塞，serveDNSOverStream 的 goroutine 与 Handler.wg 一起挂死。
	if err := w.conn.SetWriteDeadline(time.Now().Add(dnsRelayWriteTimeout)); err != nil {
		return err
	}
	_, err := w.conn.Write(out)
	return err
}

type packetConnDNSWriter struct {
	conn N.PacketConn
}

func (w *packetConnDNSWriter) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	return w.conn.WritePacket(buffer, destination)
}

const (
	dnsRelayReadTimeout  = 5 * time.Second
	dnsRelayWriteTimeout = 5 * time.Second
	dnsResolveTimeout    = 5 * time.Second
)

// serveDNSOverPacketConn 读取 UDP DNS 查询并把应答写回客户端。
// 载荷交给 handleRawDNSPacket 统一处理，因此 fake-ip 分配与真实解析走同一条路径。
func (h *Handler) serveDNSOverPacketConn(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	if !h.trackPacketConn(conn) {
		_ = conn.Close()
		if onClose != nil {
			onClose(nil)
		}
		return
	}

	go func() {
		defer h.untrackPacketConn(conn)
		defer func() {
			_ = conn.Close()
			if onClose != nil {
				onClose(nil)
			}
		}()

		writer := &packetConnDNSWriter{conn: conn}
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			_ = conn.SetReadDeadline(time.Now().Add(dnsRelayReadTimeout))
			packetBuf := buf.NewPacket()
			_, err := conn.ReadPacket(packetBuf)
			if err != nil {
				packetBuf.Release()
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return
			}
			payload := append([]byte(nil), packetBuf.Bytes()...)
			packetBuf.Release()
			h.handleRawDNSPacket(payload, source, destination, writer)
		}
	}()
}

// ipv6EgressTTL 是 IPv6 出口探测结果的缓存时长。
const ipv6EgressTTL = 30 * time.Second

var (
	ipv6EgressMu   sync.Mutex
	ipv6EgressAt   time.Time
	ipv6EgressOK   bool
	ipv6EgressDone bool
)

// ipv6EgressAvailable 报告当前是否存在可用的 IPv6 出口。
//
// 用于 AAAA 抑制：没有 IPv6 出口时若仍分配 fd65:198:18::/64 的 fake-ip，
// 客户端会先尝试 IPv6 并在超时后回退 IPv4，白白拖慢首连。
func ipv6EgressAvailable(ifaceCfg netiface.Config, logf func(string)) bool {
	ipv6EgressMu.Lock()
	defer ipv6EgressMu.Unlock()
	if ipv6EgressDone && time.Since(ipv6EgressAt) < ipv6EgressTTL {
		return ipv6EgressOK
	}
	_, err := netiface.Select(netiface.FamilyIPv6, ifaceCfg, nil)
	ipv6EgressOK = err == nil
	ipv6EgressDone = true
	ipv6EgressAt = time.Now()
	if !ipv6EgressOK && logf != nil {
		logf("[sing-tun] no usable IPv6 egress, AAAA answers will be suppressed")
	}
	return ipv6EgressOK
}

// invalidateIPv6Egress 强制下次 AAAA 查询重新探测 IPv6 出口。
func invalidateIPv6Egress() {
	ipv6EgressMu.Lock()
	ipv6EgressDone = false
	ipv6EgressMu.Unlock()
}

// packEmptyReply 以 NOERROR + 空应答回应查询，让客户端立刻判定该类型无记录
// 并回退到另一种地址族，而不是等超时。
func packEmptyReply(msg *dns.Msg) ([]byte, error) {
	resp := new(dns.Msg)
	resp.SetReply(msg)
	resp.RecursionAvailable = true
	return resp.Pack()
}

// handleRawDNSPacket handles DNS packets delivered via NewDNSPacket.
// Unlike handleDNS (which reads from a PacketConn), this receives the raw
// payload and a PacketWriter for responses.
func (h *Handler) handleRawDNSPacket(payload []byte, source M.Socksaddr, destination M.Socksaddr, writer N.PacketWriter) {
	msg := new(dns.Msg)
	if err := msg.Unpack(payload); err != nil {
		h.logger.Warn("failed to parse DNS packet", "error", err)
		return
	}

	if len(msg.Question) == 0 {
		return
	}
	question := msg.Question[0]
	domain := dns.CanonicalName(question.Name)

	if question.Qtype != dns.TypeA && question.Qtype != dns.TypeAAAA {
		h.handleDNSRealPacket(msg, domain, destination, writer)
		return
	}

	if question.Qtype == dns.TypeAAAA && !ipv6EgressAvailable(h.interfaceConfig(), h.logf) {
		if h.serveEmptyReply(msg, destination, writer) {
			return
		}
	}

	var fakeIP netip.Addr
	var isNew bool
	if question.Qtype == dns.TypeA {
		fakeIP, isNew = h.fakeIP.Create(domain)
	} else {
		fakeIP, isNew = h.fakeIP.CreateIPv6(domain)
	}
	if isNew {
		h.logger.Debug("fake-ip allocated", "domain", domain, "fake_ip", fakeIP.String(), "qtype", question.Qtype)
	}

	resp := new(dns.Msg)
	resp.SetReply(msg)
	resp.RecursionAvailable = true

	if question.Qtype == dns.TypeA {
		resp.Answer = append(resp.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   domain,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			A: AsNetIP(fakeIP).To4(),
		})
	} else {
		resp.Answer = append(resp.Answer, &dns.AAAA{
			Hdr: dns.RR_Header{
				Name:   domain,
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			AAAA: AsNetIP(fakeIP).To16(),
		})
	}

	respBytes, err := resp.Pack()
	if err != nil {
		h.logger.Warn("failed to pack DNS response", "error", err)
		return
	}
	respBuf := buf.NewPacket()
	respBuf.Write(respBytes)
	if err := writer.WritePacket(respBuf, destination); err != nil {
		h.logger.Warn("failed to write DNS response", "error", err)
	}
}

// serveEmptyReply 以 NOERROR 空应答回应查询，返回是否已成功写回。
func (h *Handler) serveEmptyReply(msg *dns.Msg, destination M.Socksaddr, writer N.PacketWriter) bool {
	respBytes, err := packEmptyReply(msg)
	if err != nil {
		h.logger.Warn("failed to pack empty DNS reply", "error", err)
		return false
	}
	respBuf := buf.NewPacket()
	respBuf.Write(respBytes)
	if err := writer.WritePacket(respBuf, destination); err != nil {
		h.logger.Warn("failed to write empty DNS reply", "error", err)
		return false
	}
	return true
}

// serveDNSFailure 以 QR=1 的 SERVFAIL 应答客户端。
// 不能直接改查询报文的 Rcode 再 Pack：那会得到 QR=0 的废包，客户端会丢弃并重试。
func (h *Handler) serveDNSFailure(msg *dns.Msg, destination M.Socksaddr, writer N.PacketWriter) {
	resp := new(dns.Msg)
	resp.SetReply(msg)
	resp.RecursionAvailable = true
	resp.Rcode = dns.RcodeServerFailure
	respBytes, packErr := resp.Pack()
	if packErr != nil {
		return
	}
	respBuf := buf.NewPacket()
	respBuf.Write(respBytes)
	if writeErr := writer.WritePacket(respBuf, destination); writeErr != nil {
		h.logger.Warn("failed to write DNS error response", "error", writeErr)
	}
}

// handleDNSRealPacket resolves non-A/AAAA queries via DoH
func (h *Handler) handleDNSRealPacket(msg *dns.Msg, domain string, destination M.Socksaddr, writer N.PacketWriter) {
	if h.resolver == nil {
		h.logger.Warn("DNS resolve unavailable: no resolver configured", "domain", domain)
		h.serveDNSFailure(msg, destination, writer)
		return
	}
	// 必须带超时：DoH 上游可能长时间不响应，无超时的解析会永久占住
	// serveDNSOverStream / serveDNSOverPacketConn 的 goroutine，
	// 使该 TCP/53 连接既读不进下一个查询也无法退出（Handler.Close 也会被拖住）。
	ctx, cancel := context.WithTimeout(context.Background(), dnsResolveTimeout)
	defer cancel()
	ips, err := h.resolver.ResolveIPs(ctx, domain)
	if err != nil {
		h.logger.Warn("DNS resolve failed", "domain", domain, "error", err)
		h.serveDNSFailure(msg, destination, writer)
		return
	}

	resp := new(dns.Msg)
	resp.SetReply(msg)
	resp.RecursionAvailable = true

	for _, ip := range ips {
		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			continue
		}
		if parsedIP.To4() != nil {
			resp.Answer = append(resp.Answer, &dns.A{
				Hdr: dns.RR_Header{
					Name:   domain,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				A: parsedIP.To4(),
			})
		} else {
			resp.Answer = append(resp.Answer, &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   domain,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				AAAA: parsedIP.To16(),
			})
		}
	}

	respBytes, err := resp.Pack()
	if err != nil {
		h.logger.Warn("failed to pack DNS response", "error", err)
		return
	}
	respBuf := buf.NewPacket()
	respBuf.Write(respBytes)
	if err := writer.WritePacket(respBuf, destination); err != nil {
		h.logger.Warn("failed to write DNS response", "error", err)
	}
}

// udpIdleTimeout UDP 中继的读写轮询间隔。
// 仅用于让 ReadPacket/ReadFrom 定期唤醒以检查 ctx 取消，
// 不代表会话超时——空闲会话由 sing-tun 的 NAT 表自行回收。
const udpIdleTimeout = 5 * time.Second

// forwardUDPDirect 直接转发 UDP 流量到上游
// 仅处理非 fake-ip 的真实 IP 目标（如 DoH 自行解析的应用 QUIC 流量）
// fake-ip 目标的 UDP 流量直接丢弃（浏览器会回退到 TCP，走代理规则链路）
func (h *Handler) forwardUDPDirect(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	// fake-ip 目标无法直接转发（是假地址），丢弃让浏览器回退 TCP
	if h.fakeIP.Contains(destination.Addr) {
		if onClose != nil {
			onClose(nil)
		}
		return
	}

	// 按目标地址族选择 udp4/udp6 与对应的物理网卡地址，避免绑定族不匹配
	network := "udp4"
	wantIPv6 := destination.Addr.Is6()
	if wantIPv6 {
		network = "udp6"
	}

	// 绑定物理网卡的**接口索引**，而不只是源地址：只设 LocalAddr 时内核
	// 仍可能按路由表把包送回 TUN。netiface.Binding.Control 里的
	// IP_UNICAST_IF（Windows）/ SO_BINDTOIFINDEX（Linux）才是绕开隧道的那一步。
	// 地址必须绑本地通配（0.0.0.0 / ::），绑具体地址会收不到回包。
	binding, err := netiface.Select(netifaceFamily(wantIPv6), h.interfaceConfig(), h.logf)
	if err != nil {
		h.logger.Warn("no physical UDP binding", "error", err)
		if onClose != nil {
			onClose(err)
		}
		return
	}
	listenConfig := &net.ListenConfig{
		Control: func(n, a string, c syscall.RawConn) error {
			if control := binding.Control(netifaceFamily(wantIPv6)); control != nil {
				return control(n, a, c)
			}
			return nil
		},
	}
	packetConn, err := listenConfig.ListenPacket(context.Background(), network, ":0")
	if err != nil {
		h.logger.Warn("failed to create UDP conn", "error", err)
		if onClose != nil {
			onClose(err)
		}
		return
	}
	remoteConn, ok := packetConn.(*net.UDPConn)
	if !ok {
		_ = packetConn.Close()
		h.logger.Warn("unexpected UDP socket type")
		if onClose != nil {
			onClose(err)
		}
		return
	}

	// 解析目标地址
	destAddr := destination.String()
	destUDPAddr, err := net.ResolveUDPAddr(network, destAddr)
	if err != nil {
		h.logger.Warn("failed to resolve dest", "error", err)
		remoteConn.Close()
		if onClose != nil {
			onClose(err)
		}
		return
	}

	// 3. 转发数据包（双向独立中继）
	// ★ 必须是两个独立方向的 goroutine 并发转发：
	//   旧的"读客户端包 → 写上游 → 读一个上游响应 → 回写 → 再等客户端包"
	//   串行一问一答模型对 QUIC 是致命的：QUIC 握手期服务端会连续回多个
	//   datagram（ServerHello / EncryptedExtensions / Cert / Finished），
	//   而串行模型读完第一个响应后必须等客户端下一个包才继续读上游，
	//   客户端此时正在等服务端后续包 → 双向僵死，浏览器迟迟不回退 TCP。
	// ★ socket 生命周期必须跟随转发 goroutine：defer 放在 goroutine 内，
	//   否则函数返回时 remoteConn 已被关闭，goroutine 的 WriteTo/ReadFrom
	//   必然报 "use of closed network connection"（此前所有 UDP/QUIC 转发失败的根因）。
	if !h.trackPacketConn(conn) {
		remoteConn.Close()
		_ = conn.Close()
		if onClose != nil {
			onClose(nil)
		}
		return
	}

	go func() {
		defer remoteConn.Close()
		defer h.untrackPacketConn(conn)
		defer func() {
			_ = conn.Close()
			if onClose != nil {
				onClose(nil)
			}
		}()

		var wg sync.WaitGroup
		wg.Add(2)

stopped := make(chan struct{})
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() { close(stopped) })
	}

	// 方向一：客户端 → 上游
	go func() {
		defer wg.Done()
		defer stop()
		for {
			select {
			case <-stopped:
				return
			case <-ctx.Done():
				return
			default:
			}

			conn.SetReadDeadline(time.Now().Add(udpIdleTimeout))
			packetBuf := buf.NewPacket()
			_, err := conn.ReadPacket(packetBuf)
			if err != nil {
				packetBuf.Release()
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // 客户端暂时空闲，等待会话由 sing-tun NAT 回收
				}
				return
			}

			_, err = remoteConn.WriteTo(packetBuf.Bytes(), destUDPAddr)
			packetBuf.Release()
			if err != nil {
				h.logger.Warn("failed to forward UDP", "error", err)
				return
			}
		}
	}()

	// 方向二：上游 → 客户端（独立持续读取，QUIC 多包响应不会丢失）
	go func() {
		defer wg.Done()
		defer stop()
		responseBuf := make([]byte, 65535)
		for {
			select {
			case <-stopped:
				return
			case <-ctx.Done():
				return
			default:
			}

			remoteConn.SetReadDeadline(time.Now().Add(udpIdleTimeout))
			n, _, err := remoteConn.ReadFrom(responseBuf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // 上游暂时无数据（QUIC 静默期），等待另一方向结束
				}
				return
			}

			responsePacket := buf.NewPacket()
			responsePacket.Write(responseBuf[:n])
			// WritePacket 的 dest 是响应包的源地址（远端服务器），不是目标（应用）
			// 所有权随 WritePacket 转移，由 gvisor 背压写端负责 Release
			if err := conn.WritePacket(responsePacket, destination); err != nil {
				h.logger.Warn("failed to write UDP response", "error", err)
				return
			}
		}
	}()

	wg.Wait()
}()
}

// proxyConn 双向复制数据，正确处理 TCP 半关闭
func (h *Handler) proxyConn(ctx context.Context, client, upstream net.Conn, onClose N.CloseHandlerFunc) {
	done := make(chan struct{}, 2)

	// client -> upstream
	go func() {
		io.Copy(upstream, client)
		common.HalfClose(upstream)
		done <- struct{}{}
	}()
	// upstream -> client
	go func() {
		io.Copy(client, upstream)
		common.HalfClose(client)
		done <- struct{}{}
	}()

	// 等待第一个方向结束。已消费一个 done，剩余待收数量必须相应扣减，
	// 否则下面的收尾循环会永远等不到第二个信号，每次关闭都白等 5 秒。
	pending := 2
	select {
	case <-done:
		pending--
		// 第一个方向结束，等待第二个方向（有超时防悬挂，也监听 ctx 外部取消）
		select {
		case <-done:
			pending--
		case <-time.After(30 * time.Second):
		case <-ctx.Done():
		}
	case <-ctx.Done():
	}

	client.Close()
	upstream.Close()

	// 两个方向都发 done：只等一个会让另一个 io.Copy 带着 conn 引用滞留到
	// 下一次 GC，反复启停时表现为内存增长。close 后最多再等 5 秒。
	for ; pending > 0; pending-- {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			pending = 0
		}
	}

	if onClose != nil {
		onClose(nil)
	}
}

func netifaceFamily(wantIPv6 bool) int {
	if wantIPv6 {
		return netiface.FamilyIPv6
	}
	return netiface.FamilyIPv4
}

// dialProxy 连接到代理服务器
// loopback (127.0.0.0/8) 已被 TUN 路由排除，连接 127.0.0.1 不会进 TUN，
// 无需绑定物理网卡。绑定物理网卡去连 loopback 反而可能失败或选错网卡。
func (h *Handler) dialProxy() (net.Conn, error) {
	return net.DialTimeout("tcp", h.proxyAddr, 5*time.Second)
}
