package proxy

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strings"
	"time"

	utls "github.com/refraction-networking/utls"
)

func mapNAT64Addr(ipStr string, prefix string) (string, bool) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return ipStr, true
	}
	parsedIP := net.ParseIP(ipStr)
	if parsedIP == nil {
		return ipStr, true
	}
	ipv4 := parsedIP.To4()
	if ipv4 == nil {
		return ipStr, false
	}

	var prefixIP net.IP
	if strings.Contains(prefix, "/") {
		_, ipnet, err := net.ParseCIDR(prefix)
		if err == nil && ipnet != nil {
			prefixIP = ipnet.IP
		}
	} else {
		prefixIP = net.ParseIP(prefix)
	}

	if prefixIP == nil || len(prefixIP) != 16 {
		return ipStr, true
	}
	mappedIP := make(net.IP, 16)
	copy(mappedIP, prefixIP[:12])
	copy(mappedIP[12:], ipv4)
	return mappedIP.String(), true
}

func (p *ProxyServer) resolveDomainCandidates(ctx context.Context, host, port, dnsMode string) []string {
	if isLiteralIP(host) {
		return []string{net.JoinHostPort(host, port)}
	}

	if dnsMode == "system" {
		ips, err := net.LookupIP(host)
		if err == nil && len(ips) > 0 {
			candidates := make([]string, 0, len(ips))
			for _, ip := range ips {
				candidates = append(candidates, net.JoinHostPort(ip.String(), port))
			}
			return candidates
		}
		return []string{net.JoinHostPort(host, port)}
	}

	if p.dohResolver != nil {
		ips, err := p.dohResolver.ResolveIPs(ctx, host)
		if err == nil && len(ips) > 0 {
			// IPv4 优先：除非 dnsMode == "ipv6_only"，否则 IPv4 排在前面
			// 避免 IPv6 成为唯一候选导致连接失败
			var v4, v6 []string
			for _, ip := range ips {
				parsed := net.ParseIP(ip)
				if parsed == nil {
					continue
				}
				if parsed.To4() != nil {
					v4 = append(v4, net.JoinHostPort(ip, port))
				} else {
					v6 = append(v6, net.JoinHostPort(ip, port))
				}
			}
			if dnsMode == "ipv6_only" {
				return append(v6, v4...)
			}
			return append(v4, v6...)
		}
	}

	return []string{net.JoinHostPort(host, port)}
}

func (p *ProxyServer) buildDialCandidates(ctx context.Context, targetHost, targetAddr string, rule Rule, effectiveMode string) []string {
	resolvedUpstream := resolveRuleUpstream(targetHost, rule)
	isWarpRoute := strings.EqualFold(strings.TrimSpace(rule.Upstream), "warp")
	defaultPort := "443"

	if isWarpRoute {
		if resolved := p.resolveDomainCandidates(ctx, targetHost, defaultPort, rule.DNSMode); len(resolved) > 0 {
			return resolved
		}
		return []string{targetAddr}
	}

	if effectiveMode == "mitm" || effectiveMode == "transparent" || effectiveMode == "tls-rf" || effectiveMode == "quic" || effectiveMode == "direct" {
		if strings.TrimSpace(resolvedUpstream) != "" {
			upstreamCandidates := splitUpstreamCandidates(targetHost, resolvedUpstream, defaultPort)
			if len(upstreamCandidates) == 0 {
				return []string{targetAddr}
			}

			firstHost, firstPort, err := net.SplitHostPort(upstreamCandidates[0])
			if err != nil {
				firstHost = upstreamCandidates[0]
				firstPort = defaultPort
			}
			if firstHost != "" && !isLiteralIP(firstHost) {
				if resolved := p.resolveDomainCandidates(ctx, firstHost, firstPort, rule.DNSMode); len(resolved) > 0 {
					return resolved
				}
			}
			return upstreamCandidates
		}

		if rule.UseCFPool && p.CFPoolUsable() {
			topIPs := p.cfPool.GetTopIPs(5)
			if len(topIPs) > 0 {
				prefs := make([]string, 0, len(topIPs))
				for _, ip := range topIPs {
					prefs = append(prefs, net.JoinHostPort(ip, defaultPort))
				}
				return dedupeDialCandidates(prefs)
			}
		}

		if resolved := p.resolveDomainCandidates(ctx, targetHost, defaultPort, rule.DNSMode); len(resolved) > 0 {
			return resolved
		}
	}

	return []string{targetAddr}
}

func (p *ProxyServer) prepareConnect(targetHost, targetAddr string, rule Rule) *connectResult {
	effectiveMode := rule.Mode
	if effectiveMode == "" {
		effectiveMode = p.GetMode()
	}

	resolvedUpstream := rule.Upstream
	if resolvedUpstream == "" {
		resolvedUpstream = "DIRECT"
	}

	p.tracef("[Connect] target=%s host=%s mode=%s->%s upstream=%s sni_fake=%s", targetAddr, targetHost, rule.Mode, effectiveMode, resolvedUpstream, rule.SniFake)

	return &connectResult{
		effectiveMode: effectiveMode,
		targetHost:    targetHost,
		targetAddr:    targetAddr,
		rule:          rule,
	}
}

func (p *ProxyServer) dialUpstream(cr *connectResult) error {
	if cr.rule.UseCFPool && p.CFPoolUsable() {
		_, port, _ := net.SplitHostPort(cr.targetAddr)
		if port == "" {
			port = "443"
		}
		var prefix string
		if cr.rule.NAT64Enabled && cr.rule.NAT64ProfileID != "" {
			prefix = p.rules.GetNAT64PrefixByID(cr.rule.NAT64ProfileID)
		}
		conn, dialedAddr, err := p.cfPool.DialParallel(context.Background(), "tcp", port, prefix)
		if err == nil {
			cr.dialCandidates = []string{dialedAddr}
			cr.dialAddr = dialedAddr
			cr.conn = conn
			p.tracef("[Connect] CFPool DialParallel success: %s", dialedAddr)
			return nil
		}
		p.tracef("[Connect] CFPool DialParallel failed: %v, falling back to buildDialCandidates", err)
	} else if cr.rule.UseCFPool {
		// Pool is nil, empty, or stale — fall back to DNS, trigger async refresh if stale
		p.maybeRefreshCFPoolAsync()
	}

	dialCandidates := p.buildDialCandidates(context.Background(), cr.targetHost, cr.targetAddr, cr.rule, cr.effectiveMode)
	if len(dialCandidates) == 0 {
		dialCandidates = []string{cr.targetAddr}
	}

	if cr.rule.NAT64Enabled && cr.rule.NAT64ProfileID != "" {
		prefix := p.rules.GetNAT64PrefixByID(cr.rule.NAT64ProfileID)
		if prefix != "" {
			var mapped []string
			for _, candidate := range dialCandidates {
				host, port, err := net.SplitHostPort(candidate)
				if err != nil {
					host = candidate
					port = "443"
				}
				mappedIP, ok := mapNAT64Addr(host, prefix)
				if ok {
					mapped = append(mapped, net.JoinHostPort(mappedIP, port))
				} else {
					p.tracef("[NAT64] Drop native IPv6 candidate: %s", host)
				}
			}
			dialCandidates = mapped
		}
	}

	if len(dialCandidates) == 0 {
		return errors.New("no valid NAT64 candidates available for dial")
	}

	cr.dialCandidates = dialCandidates
	cr.dialAddr = dialCandidates[0]

	p.tracef("[Connect] Using candidates %v for host %s", dialCandidates, cr.targetHost)

	dial := func(network, addr string) (net.Conn, error) {
		return p.dialWithRule(context.Background(), network, addr, cr.rule)
	}

	if len(dialCandidates) > 1 {
		var lastErr error
		for _, addr := range dialCandidates {
			conn, err := dial("tcp", addr)
			if err == nil {
				cr.dialAddr = addr
				cr.conn = conn
				if cr.rule.UseCFPool && p.cfPool != nil {
					h, _, _ := net.SplitHostPort(addr)
					if h != "" {
						p.cfPool.ReportSuccess(h)
					}
				}
				return nil
			}
			if cr.rule.UseCFPool && p.cfPool != nil {
				h, _, _ := net.SplitHostPort(addr)
				if h != "" {
					p.cfPool.ReportFailure(h)
				}
			}
			lastErr = err
		}
		return lastErr
	}

	conn, err := dial("tcp", cr.dialAddr)
	if err != nil {
		return err
	}
	cr.conn = conn
	return nil
}

func (p *ProxyServer) dialWithRule(ctx context.Context, network, addr string, rule Rule) (net.Conn, error) {
	if rule.NAT64Enabled && rule.NAT64ProfileID != "" {
		prefix := p.rules.GetNAT64PrefixByID(rule.NAT64ProfileID)
		if prefix != "" {
			host, port, err := net.SplitHostPort(addr)
			if err == nil {
				mappedIP, ok := mapNAT64Addr(host, prefix)
				if ok {
					addr = net.JoinHostPort(mappedIP, port)
				}
			}
		}
	}

	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	// TUN 模式下绑定物理网卡，避免出站流量被 TUN 捕获形成循环
	p.mu.RLock()
	tunMode := p.tunMode
	p.mu.RUnlock()
	if tunMode {
		// 如果目标是域名，通过 DoH 解析器解析（不走系统 DNS，系统 DNS 被 TUN 劫持返回 fake-ip）
		// 用 context 标记防止递归：DoH 解析器内部也会调 DialWithRule → dialWithRule
		host, port, splitErr := net.SplitHostPort(addr)
		if splitErr == nil && !isLiteralIP(host) && ctx.Value(dohResolveCtxKey) == nil && p.dohResolver != nil {
			dohCtx := context.WithValue(ctx, dohResolveCtxKey, true)
			if ips, err := p.dohResolver.ResolveIPs(dohCtx, host); err == nil && len(ips) > 0 {
				for _, ip := range ips {
					if net.ParseIP(ip).To4() != nil {
						addr = net.JoinHostPort(ip, port)
						break
					}
				}
			}
		}
		if localAddr := p.getPhysicalLocalAddr(addr); localAddr != nil {
			dialer.LocalAddr = localAddr
		}
	}

	return dialer.DialContext(ctx, network, addr)
}

// dohResolveCtxKey 用于防止 DoH 解析器和 dialWithRule 之间无限递归
type dohResolveCtxKeyType int

const dohResolveCtxKey dohResolveCtxKeyType = 0

// getPhysicalLocalAddr 根据目标地址的 IP 族选择对应的物理网卡本地地址
// IPv4 目标 → 返回 IPv4 地址，IPv6 目标 → 返回 IPv6 地址
// 避免绑定 IPv4 去连 IPv6（会导致 dial 失败 → 502）
func (p *ProxyServer) getPhysicalLocalAddr(targetAddr string) *net.TCPAddr {
	host, _, err := net.SplitHostPort(targetAddr)
	if err != nil {
		host = targetAddr
	}
	targetIP := net.ParseIP(host)
	wantIPv6 := targetIP != nil && targetIP.To4() == nil

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
			if !ok {
				continue
			}
			if wantIPv6 {
				// IPv6 目标：跳过 IPv4 地址
				if ipNet.IP.To4() != nil {
					continue
				}
				return &net.TCPAddr{IP: ipNet.IP}
			}
			// IPv4 目标：跳过 IPv6 地址
			if ipNet.IP.To4() == nil {
				continue
			}
			return &net.TCPAddr{IP: ipNet.IP}
		}
	}
	return nil
}

// getPhysicalInterfaceAddr 获取物理网卡的 IPv4 地址（排除 TUN/Loopback）
// 已废弃，保留向后兼容，新代码应使用 getPhysicalLocalAddr
func (p *ProxyServer) getPhysicalInterfaceAddr() *net.TCPAddr {
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
			return &net.TCPAddr{IP: ipNet.IP}
		}
	}
	return nil
}

func (p *ProxyServer) DialWithRule(ctx context.Context, network, addr string, rule Rule) (net.Conn, error) {
	return p.dialWithRule(ctx, network, addr, rule)
}

func (p *ProxyServer) EstablishUpstreamConn(host string, rule Rule, dialCandidates []string, initialALPN string) (net.Conn, string, error) {
	return p.establishUpstreamConn(host, rule, dialCandidates, initialALPN)
}

func (p *ProxyServer) establishUpstreamConn(host string, rule Rule, dialCandidates []string, initialALPN string) (net.Conn, string, error) {
	// Ultimate Defense: Flatten and sanitize candidates to guarantee they are split and contain ports
	var sanitized []string
	seen := map[string]struct{}{}
	for _, c := range dialCandidates {
		c = strings.ReplaceAll(c, "，", ",")
		c = strings.ReplaceAll(c, ";", ",")
		c = strings.ReplaceAll(c, " ", ",")
		parts := strings.Split(c, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			addr := ensureAddrWithPort(part, "443")
			if _, ok := seen[addr]; !ok {
				seen[addr] = struct{}{}
				sanitized = append(sanitized, addr)
			}
		}
	}
	dialCandidates = sanitized

	if len(dialCandidates) == 0 {
		return nil, "", errors.New("no upstream dial candidates available")
	}

	p.tracef("[Upstream] Establishing connection to %s, candidates: %v, initial ALPN: %s", host, dialCandidates, initialALPN)

	if len(dialCandidates) == 1 {
		conn, err := p.dialWithRule(context.Background(), "tcp", dialCandidates[0], rule)
		if err != nil {
			return nil, "", err
		}

		// 单 IP 也有 2 次机会：ECH 被拒时优先用服务端 RetryConfigList，其次 DNS 刷新后原地重试
		var forcedECH []byte
		for attempt := 0; attempt < 2; attempt++ {
			var echBytes []byte
			if rule.ECHEnabled {
				if len(forcedECH) > 0 {
					echBytes = forcedECH
				} else {
					echBytes = p.resolveRuleECHConfig(host, rule)
				}
			}

			allowInsecure := len(echBytes) == 0
			uconn := p.GetUConn(conn, rule.SniFake, host, rule, allowInsecure, initialALPN, echBytes)

			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			err := uconn.HandshakeContext(ctx)
			cancel()

			if err == nil {
				return uconn, uconn.ConnectionState().NegotiatedProtocol, nil
			}

			// ECH 被拒 → 恢复新 config → 原地重试
			var echErr *utls.ECHRejectionError
			if errors.As(err, &echErr) && attempt == 0 && rule.ECHEnabled {
				p.tracef("[Upstream] ECH rejected by %s for %s (retryConfigs=%d): %v", dialCandidates[0], host, len(echErr.RetryConfigList), err)
				conn.Close()
				if newECH := p.recoverECHConfig(host, rule, echBytes, echErr); len(newECH) > 0 {
					forcedECH = newECH
					conn, err = p.dialWithRule(context.Background(), "tcp", dialCandidates[0], rule)
					if err != nil {
						return nil, "", err
					}
					p.tracef("[Upstream] Retrying TLS handshake for %s with recovered ECH config (%d bytes)", host, len(newECH))
					continue
				}
				p.tracef("[Upstream] ECH recovery failed for %s; no usable retry config", host)
				return nil, "", err
			}

			conn.Close()
			return nil, "", err
		}
		return nil, "", errors.New("all single-candidate ECH handshake attempts failed")
	}

	// 多个 IP，开启全链路 Happy Eyeballs 并发建连与 TLS 握手竞速
	type raceResult struct {
		conn     net.Conn
		protocol string
		err      error
	}

	runRace := func() (net.Conn, string, error) {
		resChan := make(chan raceResult, len(dialCandidates))
		raceCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		for _, cand := range dialCandidates {
			go func(addr string) {
				// 1. TCP 拨号
				tcpConn, err := p.dialWithRule(raceCtx, "tcp", addr, rule)
				if err != nil {
					select {
					case resChan <- raceResult{err: err}:
					case <-raceCtx.Done():
					}
					return
				}

				// 2. TLS 握手
				var echBytes []byte
				if rule.ECHEnabled {
					echBytes = p.resolveRuleECHConfig(host, rule)
				}
				allowInsecure := len(echBytes) == 0
				uconn := p.GetUConn(tcpConn, rule.SniFake, host, rule, allowInsecure, initialALPN, echBytes)

				handshakeCtx, handshakeCancel := context.WithTimeout(raceCtx, 5*time.Second)
				defer handshakeCancel()

				err = uconn.HandshakeContext(handshakeCtx)
				if err != nil {
					tcpConn.Close()
					select {
					case resChan <- raceResult{err: err}:
					case <-raceCtx.Done():
					}
					return
				}

				// 握手成功！成为竞胜者
				select {
				case resChan <- raceResult{conn: uconn, protocol: uconn.ConnectionState().NegotiatedProtocol}:
				case <-raceCtx.Done():
					uconn.Close()
				}
			}(cand)
		}

		// 逐个收集结果：遇到第一个 ECH 被拒立即返回信号，不等其他协程
		for i := 0; i < len(dialCandidates); i++ {
			res := <-resChan
			if res.err == nil && res.conn != nil {
				// 成功！清理其余协程
				cancel()
				go func(remaining int) {
					for k := 0; k < remaining; k++ {
						r := <-resChan
						if r.err == nil && r.conn != nil {
							r.conn.Close()
						}
					}
				}(len(dialCandidates) - 1 - i)
				p.tracef("[Upstream] Happy Eyeballs won, negotiated ALPN: %s", res.protocol)
				return res.conn, res.protocol, nil
			}
			// 检查是否 ECH 被拒 → 立即返回信号，触发外层恢复 + 重跑
			if res.err != nil && rule.ECHEnabled {
				p.tracef("[Upstream] Candidate handshake error: %v (type: %T)", res.err, res.err)
				var echErr *utls.ECHRejectionError
				if errors.As(res.err, &echErr) {
					p.tracef("[Upstream] ECH REJECTED by candidate for %s (retryConfigs=%d), recovering config + re-race", host, len(echErr.RetryConfigList))
					cancel() // 立即终止其余协程
					go func(remaining int) {
						for k := 0; k < remaining; k++ {
							r := <-resChan
							if r.err == nil && r.conn != nil {
								r.conn.Close()
							}
						}
					}(len(dialCandidates) - 1 - i)
					return nil, "", res.err // 传回 ECHRejectionError 触发外层恢复
				}
			}
		}
		return nil, "", errors.New("all candidates failed")
	}

	conn, proto, raceErr := runRace()
	if conn != nil {
		return conn, proto, nil
	}

	// 第一个 ECH 被拒 → 恢复 config 后立即重跑，不等其他候选
	if rule.ECHEnabled && raceErr != nil {
		var echErr *utls.ECHRejectionError
		if errors.As(raceErr, &echErr) {
			oldECH := p.resolveRuleECHConfig(host, rule)
			if newECH := p.recoverECHConfig(host, rule, oldECH, echErr); len(newECH) > 0 {
				p.tracef("[Upstream] Re-racing %s with recovered ECH config (%d bytes)", host, len(newECH))
				conn, proto, retryErr := runRace()
				if conn != nil {
					return conn, proto, nil
				}
				if retryErr != nil {
					return nil, "", retryErr
				}
			} else {
				p.tracef("[Upstream] ECH recovery failed for %s after multi-candidate rejection", host)
			}
		}
	}

	if raceErr != nil {
		return nil, "", raceErr
	}
	return nil, "", errors.New("all upstream dial and handshake attempts failed in parallel race")
}

// recoverECHConfig recovers a usable ECH config after the server rejects ECH.
// Priority:
//  1. RetryConfigList from the server (RFC 9180 — authoritative for the next attempt)
//  2. Fresh HTTPS/SVCB ECH config via safe (non-ECH) DNS
//
// Returns nil when no different usable config is available.
func (p *ProxyServer) recoverECHConfig(host string, rule Rule, oldECH []byte, echErr *utls.ECHRejectionError) []byte {
	// 1. Server-provided retry configs — preferred, no DNS dependency.
	if echErr != nil && len(echErr.RetryConfigList) > 0 {
		if !bytes.Equal(echErr.RetryConfigList, oldECH) {
			p.tracef("[Upstream] Using server RetryConfigList for %s (%d bytes)", host, len(echErr.RetryConfigList))
			p.applyECHConfig(host, rule, echErr.RetryConfigList, "server-retry-configs")
			return echErr.RetryConfigList
		}
		p.tracef("[Upstream] Server RetryConfigList for %s matches current config; trying DNS", host)
	} else {
		p.tracef("[Upstream] No RetryConfigList in ECH rejection for %s; trying DNS", host)
	}

	// 2. DNS refresh via non-ECH DoH nodes.
	if newECH := p.refreshECHFromDNS(host, rule); len(newECH) > 0 {
		if !bytes.Equal(newECH, oldECH) {
			return newECH
		}
		p.tracef("[Upstream] DNS-refreshed ECH for %s is identical to rejected config", host)
	}
	return nil
}

// refreshECHFromDNS fetches a fresh ECH config list via safe DNS and applies it.
func (p *ProxyServer) refreshECHFromDNS(host string, rule Rule) []byte {
	if p.dohResolver == nil {
		p.tracef("[Upstream] ECH DNS refresh skipped for %s: no DoH resolver", host)
		return nil
	}

	lookupDomain := strings.TrimSpace(rule.ECHDiscoveryDomain)
	if lookupDomain == "" {
		lookupDomain = host
	}

	refreshCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	newECH, err := p.dohResolver.ResolveECHSafe(refreshCtx, lookupDomain)
	if err != nil || len(newECH) == 0 {
		p.tracef("[Upstream] ECH DNS refresh via safe nodes failed for %s (lookup=%s): %v; trying direct", host, lookupDomain, err)
		// Fallback: use first enabled node's URL directly (works even when all nodes have ECH)
		if nodes := p.rules.GetDNSNodes(); len(nodes) > 0 {
			for _, n := range nodes {
				if n.Enabled && n.URL != "" {
					directCtx, directCancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer directCancel()
					if ech, fetchErr := fetchECHDirect(directCtx, lookupDomain, n.URL); fetchErr == nil && len(ech) > 0 {
						p.tracef("[Upstream] ECH DNS refresh success via direct %s for %s (%d bytes)", n.URL, host, len(ech))
						p.applyECHConfig(host, rule, ech, "dns-refresh-direct")
						return ech
					}
					break
				}
			}
		}
		return nil
	}

	p.tracef("[Upstream] ECH DNS refresh success for %s (lookup=%s, %d bytes)", host, lookupDomain, len(newECH))
	p.applyECHConfig(host, rule, newECH, "dns-refresh")
	return newECH
}
