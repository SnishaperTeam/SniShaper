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
	"time"

	"github.com/miekg/dns"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"snishaper/pkg/dohresolver"
)

// Handler 实现 sing-tun 的 Handler 接口
// 负责将 TUN 流量转发到 SniShaper Proxy
type Handler struct {
	proxyAddr string
	resolver  *dohresolver.FailoverResolver
	fakeIP    *FakeIPStore
	logf      func(string)
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

// PrepareConnection 在连接建立前调用，可用于规则预匹配
func (h *Handler) PrepareConnection(
	network string,
	source M.Socksaddr,
	destination M.Socksaddr,
	routeContext tun.DirectRouteContext,
	timeout time.Duration,
) (tun.DirectRouteDestination, error) {
	// 返回 nil 表示不直连，走代理
	return nil, nil
}

// NewConnectionEx 处理新的 TCP 连接
func (h *Handler) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	// 查找真实域名（fake-ip 反查）
	targetHost := h.resolveHost(destination)
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

	// 双向复制数据
	go h.proxyConn(ctx, conn, upstream, onClose)
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

// NewPacketConnectionEx 处理新的 UDP 连接
func (h *Handler) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	// 检测 DNS 请求（目标端口 53）
	if destination.Port == 53 {
		h.handleDNS(ctx, conn, source, destination, onClose)
		return
	}
	// 其他 UDP 流量直接转发到上游
	h.forwardUDPDirect(ctx, conn, source, destination, onClose)
}

// handleDNS 处理 DNS 查询，实现 fake-ip
func (h *Handler) handleDNS(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	// 1. 读取 DNS 查询
	queryBuf := buf.NewPacket()
	defer queryBuf.Release()
	_, err := conn.ReadPacket(queryBuf)
	if err != nil {
		h.logf("[sing-tun] failed to read DNS: " + err.Error())
		if onClose != nil {
			onClose(err)
		}
		return
	}

	// 2. 解析 DNS 消息
	msg := new(dns.Msg)
	if err := msg.Unpack(queryBuf.Bytes()); err != nil {
		h.logf("[sing-tun] failed to parse DNS: " + err.Error())
		if onClose != nil {
			onClose(err)
		}
		return
	}

	// 3. 提取查询域名
	if len(msg.Question) == 0 {
		if onClose != nil {
			onClose(nil)
		}
		return
	}
	question := msg.Question[0]
	domain := dns.CanonicalName(question.Name)

	// 4. 只处理 A/AAAA 查询，其他类型用真实 DNS
	if question.Qtype != dns.TypeA && question.Qtype != dns.TypeAAAA {
		h.handleDNSReal(ctx, conn, msg, destination, onClose)
		return
	}

	// 5. 生成 fake-ip 并存储映射
	var fakeIP netip.Addr
	var isNew bool
	if question.Qtype == dns.TypeA {
		fakeIP, isNew = h.fakeIP.Create(domain)
	} else {
		fakeIP, isNew = h.fakeIP.CreateIPv6(domain)
	}
	// 仅新建时打日志，避免 Windows DNS 重试导致同域名刷屏
	if isNew {
		h.logf(fmt.Sprintf("[sing-tun] fake-ip: %s -> %s (type: %d)", domain, fakeIP, question.Qtype))
	}

	// 6. 构建 DNS 响应（返回 fake-ip）
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

	// 7. 发送响应
	// WritePacket 的 dest 参数是响应包的【源地址】（即 DNS 服务器地址），
	// 不是目标地址——目标地址由 NAT 自动填为应用地址。
	// 传 destination（DNS 服务器），不是 source（应用）。
	h.sendDNSResponse(conn, resp, destination)
	if onClose != nil {
		onClose(nil)
	}
}

// handleDNSReal 使用真实 DNS 解析（非 A/AAAA 查询）
func (h *Handler) handleDNSReal(ctx context.Context, conn N.PacketConn, msg *dns.Msg, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	question := msg.Question[0]
	domain := dns.CanonicalName(question.Name)

	// 调用 DoH 解析器
	ips, err := h.resolver.ResolveIPs(ctx, domain)
	if err != nil {
		h.logf("[sing-tun] DNS resolve failed for " + domain + ": " + err.Error())
		msg.Rcode = dns.RcodeServerFailure
		h.sendDNSResponse(conn, msg, destination)
		if onClose != nil {
			onClose(nil)
		}
		return
	}

	// 构建响应
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

	h.sendDNSResponse(conn, resp, destination)
	if onClose != nil {
		onClose(nil)
	}
}

// sendDNSResponse 发送 DNS 响应
// dest 参数是 DNS 服务器的地址（响应包的源地址），不是应用的地址。
// sing-tun 的 UDPBackWriter.WritePacket 用 dest 作为源地址，
// 目标地址由 NAT 自动填为应用地址。
func (h *Handler) sendDNSResponse(conn N.PacketConn, msg *dns.Msg, dest M.Socksaddr) {
	respBytes, err := msg.Pack()
	if err != nil {
		h.logf("[sing-tun] failed to pack DNS response: " + err.Error())
		return
	}
	respBuf := buf.NewPacket()
	defer respBuf.Release()
	respBuf.Write(respBytes)
	if err := conn.WritePacket(respBuf, dest); err != nil {
		h.logf("[sing-tun] failed to write DNS response to " + dest.String() + ": " + err.Error())
	}
}

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

	// 绑定物理网卡，避免 UDP 包进 TUN 形成循环
	var laddr *net.UDPAddr
	if bindIP := h.getPhysicalUDPAddr(); bindIP != nil {
		laddr = &net.UDPAddr{IP: bindIP}
	}
	remoteConn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		h.logf("[sing-tun] failed to create UDP conn: " + err.Error())
		if onClose != nil {
			onClose(err)
		}
		return
	}
	defer remoteConn.Close()

	// 解析目标地址
	destAddr := destination.String()
	destUDPAddr, err := net.ResolveUDPAddr("udp4", destAddr)
	if err != nil {
		h.logf("[sing-tun] failed to resolve dest: " + err.Error())
		if onClose != nil {
			onClose(err)
		}
		return
	}

	// 3. 转发数据包
	go func() {
		defer func() {
			if onClose != nil {
				onClose(nil)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			packetBuf := buf.NewPacket()
			_, err := conn.ReadPacket(packetBuf)
			if err != nil {
				packetBuf.Release()
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return
			}

			_, err = remoteConn.WriteTo(packetBuf.Bytes(), destUDPAddr)
			packetBuf.Release()
			if err != nil {
				h.logf("[sing-tun] failed to forward UDP: " + err.Error())
				continue
			}

			remoteConn.SetReadDeadline(time.Now().Add(5 * time.Second))
			responseBuf := make([]byte, 1500)
			n, _, err := remoteConn.ReadFrom(responseBuf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				continue
			}

			responsePacket := buf.NewPacket()
			responsePacket.Write(responseBuf[:n])
			// WritePacket 的 dest 是响应包的源地址（远端服务器），不是目标（应用）
			_ = conn.WritePacket(responsePacket, destination)
			responsePacket.Release()
		}
	}()
}

// proxyConn 双向复制数据，正确处理 TCP 半关闭
func (h *Handler) proxyConn(ctx context.Context, client, upstream net.Conn, onClose N.CloseHandlerFunc) {
	done := make(chan struct{}, 2)

	// client -> upstream
	go func() {
		io.Copy(upstream, client)
		halfClose(upstream) // 告知上游：客户端已发完数据
		done <- struct{}{}
	}()
	// upstream -> client
	go func() {
		io.Copy(client, upstream)
		halfClose(client) // 告知客户端：上游已发完数据
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

// halfClose 关闭连接的写端（发送 EOF），保留读端
func halfClose(conn net.Conn) {
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.CloseWrite()
		return
	}
	type closeWriter interface {
		CloseWrite() error
	}
	if cw, ok := conn.(closeWriter); ok {
		_ = cw.CloseWrite()
		return
	}
	conn.Close()
}

// dialProxy 连接到代理服务器
// loopback (127.0.0.0/8) 已被 TUN 路由排除，连接 127.0.0.1 不会进 TUN，
// 无需绑定物理网卡。绑定物理网卡去连 loopback 反而可能失败或选错网卡。
func (h *Handler) dialProxy() (net.Conn, error) {
	return net.DialTimeout("tcp", h.proxyAddr, 5*time.Second)
}

// getPhysicalUDPAddr 获取物理网卡的 IPv4 地址（排除 TUN/Loopback）
// 用于 forwardUDPDirect 绑定物理网卡，避免 UDP 包进 TUN 循环
func (h *Handler) getPhysicalUDPAddr() net.IP {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		name := iface.Name
		if strings.Contains(name, "SniShaper") || strings.Contains(name, "tun") || strings.Contains(name, "TAP") {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil || len(addrs) == 0 {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				continue
			}
			return ipNet.IP
		}
	}
	return nil
}
