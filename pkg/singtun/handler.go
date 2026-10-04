package singtun

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
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
	ifaceConfig netiface.Config
	mu          sync.Mutex
	live        map[net.Conn]struct{}
}

func (h *Handler) track(c net.Conn) {
	h.mu.Lock()
	if h.live == nil {
		h.live = make(map[net.Conn]struct{})
	}
	h.live[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Handler) untrack(c net.Conn) {
	h.mu.Lock()
	delete(h.live, c)
	h.mu.Unlock()
}

func (h *Handler) Close() {
	h.mu.Lock()
	conns := h.live
	h.live = nil
	h.mu.Unlock()
	for c := range conns {
		c.Close()
	}
}

// NewHandler 创建新的 Handler
func NewHandler(proxyAddr string, resolver *dohresolver.FailoverResolver, logf func(string)) *Handler {
	h := &Handler{
		proxyAddr: proxyAddr,
		resolver:  resolver,
		fakeIP:    NewFakeIPStore(),
		logf:      logf,
	}
	h.logf("[sing-tun] Handler created, proxy: " + proxyAddr)
	return h
}

func (h *Handler) SetInterfaceConfig(cfg netiface.Config) {
	h.ifaceConfig = cfg
}

func (h *Handler) JudgeFlow(network uint8, source netip.AddrPort, destination netip.AddrPort, firstPacket []byte) tun.FlowVerdict {
	return tun.FlowVerdict{Action: tun.ActionAccept}
}

func (h *Handler) NewDNSPacket(payload []byte, source M.Socksaddr, destination M.Socksaddr, writer N.PacketWriter) {
	h.handleRawDNSPacket(payload, source, destination, writer)
}

// NewConnectionEx 处理新的 TCP 连接
func (h *Handler) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	// 查找真实域名（fake-ip 反查）
	targetHost := h.resolveHost(destination)

	// 浏览器可能用 DoH/系统缓存解析出真实 IP（绕过 TUN 的 fake-ip 劫持），
	// 导致规则按域名匹配失效。此时从 TLS ClientHello 嗅探 SNI 重建域名。
	if net.ParseIP(targetHost) != nil {
		if sni, c := h.sniffTLSSNI(conn); sni != "" {
			h.logf("[sing-tun] SNI sniffed: " + sni + " (was IP " + targetHost + ")")
			targetHost = sni
			conn = c
		}
	}
	h.logf(fmt.Sprintf("[sing-tun] TCP %s -> %s (resolved: %s)", source.String(), destination.String(), targetHost))

	// 连接到 ProxyServer
	// loopback (127.0.0.0/8) 已被 Inet4RouteExcludeAddress 排除出 TUN，
	// 连接 127.0.0.1 不会进 TUN，无需绑定物理网卡。
	upstream, err := h.dialProxy()
	if err != nil {
		h.logf("[sing-tun] failed to connect to proxy: " + err.Error())
		conn.Close()
		if onClose != nil {
			onClose(err)
		}
		return
	}

	// 发送 CONNECT 请求 (使用域名，不是 IP)
	// 用 net.JoinHostPort 正确处理 IPv6 地址（自动加方括号）
	target := net.JoinHostPort(targetHost, strconv.Itoa(int(destination.Port)))
	connectReq := "CONNECT " + target + " HTTP/1.1\r\nHost: " + target + "\r\n\r\n"
	h.logf(fmt.Sprintf("[sing-tun] CONNECT request: %q", connectReq))
	if _, err := upstream.Write([]byte(connectReq)); err != nil {
		h.logf("[sing-tun] failed to send CONNECT: " + err.Error())
		conn.Close()
		upstream.Close()
		if onClose != nil {
			onClose(err)
		}
		return
	}

	// 读取响应 — 用 bufio.Reader 确保读到完整的响应头
	br := bufio.NewReader(upstream)
	statusLine, err := br.ReadString('\n')
	if err != nil {
		h.logf("[sing-tun] failed to read CONNECT response: " + err.Error())
		conn.Close()
		upstream.Close()
		if onClose != nil {
			onClose(err)
		}
		return
	}
	statusLine = strings.TrimRight(statusLine, "\r\n")
	h.logf(fmt.Sprintf("[sing-tun] CONNECT response: %q", statusLine))

	// 解析状态码（不能用子串匹配 "200"，状态行其他字段也可能包含 "200"）
	if !isHTTPSuccess(statusLine) {
		// 读取错误响应的剩余内容用于日志
		rest, _ := io.ReadAll(io.LimitReader(br, 4096))
		errMsg := statusLine
		if len(rest) > 0 {
			errMsg += "\r\n" + string(rest)
		}
		h.logf("[sing-tun] CONNECT failed: " + errMsg)
		err := fmt.Errorf("proxy connect failed: %s", statusLine)
		conn.Close()
		upstream.Close()
		if onClose != nil {
			onClose(err)
		}
		return
	}

	// 读取并丢弃剩余响应头，直到空行（\r\n）
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			h.logf("[sing-tun] failed to read CONNECT headers: " + err.Error())
			conn.Close()
			upstream.Close()
			if onClose != nil {
				onClose(err)
			}
			return
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	// 用 bufio.Reader 包装 upstream，确保 br 中已缓冲的隧道数据不丢失
	// （代理在 200 响应后可能立即发送 TLS ServerHello 等数据）
	upstream = &bufferedConn{Conn: upstream, br: br}

	go func() {
		h.track(conn)
		h.track(upstream)
		h.proxyConn(ctx, conn, upstream, onClose)
		h.untrack(conn)
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
// 返回的 net.Conn 总是包装后的 bufferedConn，保证已读字节不丢失。
// 非 TLS 或超时（客户端迟迟不发包）返回空 SNI。
func (h *Handler) sniffTLSSNI(conn net.Conn) (string, net.Conn) {
	br := bufio.NewReader(conn)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	defer conn.SetReadDeadline(time.Time{})

	sni := sniffSNIFromReader(br)
	return sni, &bufferedConn{Conn: conn, br: br}
}

// sniffSNIFromReader 读取并解析 TLS ClientHello 的 SNI 扩展
func sniffSNIFromReader(br *bufio.Reader) string {
	// record header: type(1) + version(2) + length(2)
	hdr := make([]byte, 5)
	if _, err := io.ReadFull(br, hdr); err != nil {
		return ""
	}
	if hdr[0] != 0x16 { // not TLS handshake
		return ""
	}
	bodyLen := int(hdr[3])<<8 | int(hdr[4])
	if bodyLen < 4 || bodyLen > 1<<14 {
		return ""
	}
	body := make([]byte, bodyLen)
	if _, err := io.ReadFull(br, body); err != nil {
		return ""
	}
	// handshake header: msgType(1) + length(3)
	if len(body) < 4 || body[0] != 0x01 { // not ClientHello
		return ""
	}
	// ClientHello: version(2) + random(32) + sessionID...
	p := 4 + 2 + 32
	if p >= len(body) {
		return ""
	}
	sidLen := int(body[p])
	p++
	if p+sidLen > len(body) {
		return ""
	}
	p += sidLen
	if p+2 > len(body) {
		return ""
	}
	cipherLen := int(body[p])<<8 | int(body[p+1])
	p += 2
	if p+cipherLen > len(body) {
		return ""
	}
	p += cipherLen
	if p >= len(body) {
		return ""
	}
	compLen := int(body[p])
	p++
	if p+compLen > len(body) {
		return ""
	}
	p += compLen
	if p+2 > len(body) {
		return ""
	}
	extLen := int(body[p])<<8 | int(body[p+1])
	p += 2
	if p+extLen > len(body) {
		return ""
	}
	end := p + extLen
	for p+4 <= end {
		extType := int(body[p])<<8 | int(body[p+1])
		extDataLen := int(body[p+2])<<8 | int(body[p+3])
		p += 4
		if p+extDataLen > end {
			return ""
		}
		if extType == 0 { // server_name
			data := body[p : p+extDataLen]
			// listLength(2) + nameType(1) + nameLen(2) + name
			if len(data) >= 5 && data[2] == 0 {
				nameLen := int(data[3])<<8 | int(data[4])
				if 5+nameLen <= len(data) {
					return string(data[5 : 5+nameLen])
				}
			}
			return ""
		}
		p += extDataLen
	}
	return ""
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
		h.logf(fmt.Sprintf("[sing-tun] WARNING: fake-ip %s has no domain mapping", addr))
	}

	// 不是 fake-ip，返回原始地址
	return addr.String()
}

// NewPacketConnectionEx 处理新的 UDP 连接（非 DNS；DNS 由 NewDNSPacket 接管）
func (h *Handler) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	// sing-tun does not always divert DNS to NewDNSPacket: with AutoRoute on,
	// a query to the TUN resolver arrives here as an ordinary UDP flow instead.
	// Without this branch the query is forwarded upstream and never becomes a
	// fake-ip, so the domain is lost before it reaches the proxy and
	// domain-based rules stop matching. Answering it here keeps the fake-ip
	// table populated.
	if destination.Port == dnsPort {
		h.serveDNSOverPacketConn(ctx, conn, source, destination, onClose)
		return
	}
	h.forwardUDPDirect(ctx, conn, source, destination, onClose)
}

// dnsPort is the standard DNS service port.
const dnsPort = 53

// serveDNSOverPacketConn answers DNS queries that reached the generic packet
// path rather than NewDNSPacket.
func (h *Handler) serveDNSOverPacketConn(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	go func() {
		defer func() {
			if onClose != nil {
				onClose(nil)
			}
		}()

		packet := buf.NewPacket()
		defer packet.Release()

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			conn.SetReadDeadline(time.Now().Add(dnsReadTimeout))
			if _, err := conn.ReadPacket(packet); err != nil {
				return
			}
			h.handleRawDNSPacket(packet.Bytes(), source, destination, conn)
		}
	}()
}

// dnsReadTimeout bounds how long the fallback DNS reader waits for a query.
const dnsReadTimeout = 5 * time.Second

// handleRawDNSPacket handles DNS packets delivered via NewDNSPacket.
// Unlike handleDNS (which reads from a PacketConn), this receives the raw
// payload and a PacketWriter for responses.
func (h *Handler) handleRawDNSPacket(payload []byte, source M.Socksaddr, destination M.Socksaddr, writer N.PacketWriter) {
	msg := new(dns.Msg)
	if err := msg.Unpack(payload); err != nil {
		h.logf("[sing-tun] failed to parse DNS packet: " + err.Error())
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

	// Windows 判定网卡"有 Internet"时，NCSI 会把 dns.msftncsi.com 解析结果当作
	// 连通性证据。fake-ip 段（198.18.0.0/15，RFC 2544 基准测试保留）不是公网地址，
	// Windows 拿到后直接判定不可用，网卡就显示"无法访问互联网"，根本不会发出
	// HTTP 探测。mihomo / sing-box 的 fake-ip-filter 默认收录这些域名正是为此。
	//
	// 所以这些域名必须走真实解析：拿到真实公网 IP，NCSI 的 HTTP 探测经隧道打到
	// 微软服务器，拿到真正的 "Microsoft NCSI" 响应。
	if shouldBypassFakeIP(domain) {
		h.logf("[sing-tun] fake-ip bypass: " + domain)
		h.handleDNSRealPacket(msg, domain, destination, writer)
		return
	}

	var fakeIP netip.Addr
	var isNew bool
	if question.Qtype == dns.TypeA {
		fakeIP, isNew = h.fakeIP.Create(domain)
	} else {
		fakeIP, isNew = h.fakeIP.CreateIPv6(domain)
	}
	if isNew {
		h.logf(fmt.Sprintf("[sing-tun] fake-ip: %s -> %s (type: %d)", domain, fakeIP, question.Qtype))
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

	respBytes, err := msg.Pack()
	if err != nil {
		h.logf("[sing-tun] failed to pack DNS response: " + err.Error())
		return
	}
	respBytes, err = resp.Pack()
	if err != nil {
		h.logf("[sing-tun] failed to pack DNS response: " + err.Error())
		return
	}
	respBuf := buf.NewPacket()
	respBuf.Write(respBytes)
	if err := writer.WritePacket(respBuf, destination); err != nil {
		respBuf.Release()
		h.logf("[sing-tun] failed to write DNS response: " + err.Error())
	}
}

// handleDNSRealPacket resolves non-A/AAAA queries via DoH
func (h *Handler) handleDNSRealPacket(msg *dns.Msg, domain string, destination M.Socksaddr, writer N.PacketWriter) {
	ips, err := h.resolver.ResolveIPs(context.Background(), domain)
	if err != nil {
		h.logf("[sing-tun] DNS resolve failed for " + domain + ": " + err.Error())
		msg.Rcode = dns.RcodeServerFailure
		respBytes, packErr := msg.Pack()
		if packErr != nil {
			return
		}
		respBuf := buf.NewPacket()
		respBuf.Write(respBytes)
		if writeErr := writer.WritePacket(respBuf, destination); writeErr != nil {
			respBuf.Release()
		}
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
		h.logf("[sing-tun] failed to pack DNS response: " + err.Error())
		return
	}
	respBuf := buf.NewPacket()
	respBuf.Write(respBytes)
	if err := writer.WritePacket(respBuf, destination); err != nil {
		respBuf.Release()
		h.logf("[sing-tun] failed to write DNS response: " + err.Error())
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

	// 绑定物理网卡，避免 UDP 包进 TUN 形成循环
	var laddr *net.UDPAddr
	if bindIP := h.getPhysicalUDPAddr(wantIPv6); bindIP != nil {
		laddr = &net.UDPAddr{IP: bindIP}
	}
	remoteConn, err := net.ListenUDP(network, laddr)
	if err != nil {
		h.logf("[sing-tun] failed to create UDP conn: " + err.Error())
		if onClose != nil {
			onClose(err)
		}
		return
	}

	// 解析目标地址
	destAddr := destination.String()
	destUDPAddr, err := net.ResolveUDPAddr(network, destAddr)
	if err != nil {
		h.logf("[sing-tun] failed to resolve dest: " + err.Error())
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
	go func() {
		defer remoteConn.Close()
		defer func() {
			if onClose != nil {
				onClose(nil)
			}
		}()

		var wg sync.WaitGroup
		wg.Add(2)

		// 方向一：客户端 → 上游
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					remoteConn.Close() // 解除另一方向阻塞
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
					// 客户端关闭/会话结束 → 关闭上游 socket，结束整个转发流
					remoteConn.Close()
					return
				}

				_, err = remoteConn.WriteTo(packetBuf.Bytes(), destUDPAddr)
				packetBuf.Release()
				if err != nil {
					h.logf("[sing-tun] failed to forward UDP: " + err.Error())
					remoteConn.Close()
					return
				}
			}
		}()

		// 方向二：上游 → 客户端（独立持续读取，QUIC 多包响应不会丢失）
		go func() {
			defer wg.Done()
			responseBuf := make([]byte, 65535)
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				remoteConn.SetReadDeadline(time.Now().Add(udpIdleTimeout))
				n, _, err := remoteConn.ReadFrom(responseBuf)
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue // 上游暂时无数据（QUIC 静默期），等待方向一关闭 socket
					}
					// 上游 socket 被关闭 / 上游不可达 → 关闭并结束，确保方向一也及时退出
					remoteConn.Close()
					return
				}

				responsePacket := buf.NewPacket()
				responsePacket.Write(responseBuf[:n])
				// WritePacket 的 dest 是响应包的源地址（远端服务器），不是目标（应用）
				// 所有权随 WritePacket 转移，由 gvisor 背压写端负责 Release
				if err := conn.WritePacket(responsePacket, destination); err != nil {
					responsePacket.Release()
					h.logf("[sing-tun] failed to write UDP response: " + err.Error())
					remoteConn.Close()
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
	}()
	// upstream -> client
	go func() {
		io.Copy(client, upstream)
		common.HalfClose(client)
		done <- struct{}{}
	}()

	// 等待第一个方向结束
	select {
	case <-done:
		// 第一个方向结束，等待第二个方向（有超时防悬挂，也监听 ctx 外部取消）
		select {
		case <-done:
		case <-time.After(30 * time.Second):
		case <-ctx.Done():
		}
	case <-ctx.Done():
	}

	client.Close()
	upstream.Close()
	if onClose != nil {
		onClose(nil)
	}
}

// dialProxy 连接到代理服务器
// loopback (127.0.0.0/8) 已被 TUN 路由排除，连接 127.0.0.1 不会进 TUN，
// 无需绑定物理网卡。绑定物理网卡去连 loopback 反而可能失败或选错网卡。
func (h *Handler) dialProxy() (net.Conn, error) {
	return net.DialTimeout("tcp", h.proxyAddr, 5*time.Second)
}

// getPhysicalUDPAddr 获取物理网卡对应地址族的地址（排除 TUN/Loopback）
// wantIPv6=true 时返回 IPv6 地址，否则返回 IPv4 地址
// 用于 forwardUDPDirect 绑定物理网卡，避免 UDP 包进 TUN 循环
func (h *Handler) getPhysicalUDPAddr(wantIPv6 bool) net.IP {
	family := netiface.FamilyIPv4
	if wantIPv6 {
		family = netiface.FamilyIPv6
	}
	binding, err := netiface.Select(family, h.ifaceConfig, h.logf)
	if err != nil {
		h.logf("[sing-tun] no physical UDP bind address: " + err.Error())
		return nil
	}
	return net.IP(binding.Address.AsSlice())
}
