package proxy

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func (p *ProxyServer) handleRequest(w http.ResponseWriter, req *http.Request) {
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	matchHost := normalizeHost(host)
	mode := p.GetMode()
	rule := p.rules.matchRule(matchHost, mode)
	if rule.SiteID != "" {
		p.rules.incrementRuleHit(rule.SiteID)
	}

	p.tracef("[Proxy] Request: %s -> %s (match: %s, runtime-mode: %s, rule-mode: %s)", req.Method, host, matchHost, mode, rule.Mode)

	switch req.Method {
	case http.MethodConnect:
		p.handleConnect(w, req, rule)
	default:
		p.handleHTTP(w, req, rule)
	}
}

func (p *ProxyServer) handleConnect(w http.ResponseWriter, req *http.Request, rule Rule) {
	targetAuthority := req.URL.Host
	if targetAuthority == "" {
		targetAuthority = req.Host
	}
	targetHost := normalizeHost(targetAuthority)
	targetAddr := ensureAddrWithPort(targetAuthority, "443")

	cr := p.prepareConnect(targetHost, targetAddr, rule)

	if cr.effectiveMode == "direct" {
		p.directConnect(w, req)
		return
	}



	if cr.effectiveMode == "quic" {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "Hijack not supported", http.StatusInternalServerError)
			return
		}
		clientConn, rw, err := hijacker.Hijack()
		if err != nil {
			log.Printf("[Connect] QUIC hijack failed: %v", err)
			return
		}
		if _, err := rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			clientConn.Close()
			return
		}
		if err := rw.Flush(); err != nil {
			clientConn.Close()
			return
		}
		clientConn = wrapHijackedConn(clientConn, rw)
		_ = clientConn.SetDeadline(time.Time{})
		p.handleQUICMITM(clientConn, cr.targetHost, cr.rule)
		return
	}

	if cr.effectiveMode == "migration" {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "Hijack not supported", http.StatusInternalServerError)
			return
		}
		clientConn, rw, err := hijacker.Hijack()
		if err != nil {
			log.Printf("[Connect] Migration hijack failed: %v", err)
			return
		}
		clientConn = wrapHijackedConn(clientConn, rw)
		_ = clientConn.SetDeadline(time.Time{})
		_, targetPort, _ := net.SplitHostPort(cr.targetAddr)
		if targetPort == "" {
			targetPort = "443"
		}
		p.handleMigration(clientConn, cr.targetHost, targetPort, cr.rule)
		return
	}

	if err := p.dialUpstream(cr); err != nil {
		http.Error(w, "Failed to connect to upstream", http.StatusBadGateway)
		p.tracef("[Connect] All upstream connect attempts failed: %v, error: %v", cr.dialCandidates, err)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijack not supported", http.StatusInternalServerError)
		cr.conn.Close()
		return
	}

	clientConn, rw, err := hijacker.Hijack()
	if err != nil {
		log.Printf("[Connect] Hijack failed: %v", err)
		cr.conn.Close()
		return
	}
	if _, err := rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		log.Printf("[Connect] Write 200 failed: %v", err)
		clientConn.Close()
		cr.conn.Close()
		return
	}
	if err := rw.Flush(); err != nil {
		log.Printf("[Connect] Flush 200 failed: %v", err)
		clientConn.Close()
		cr.conn.Close()
		return
	}
	clientConn = wrapHijackedConn(clientConn, rw)
	_ = clientConn.SetDeadline(time.Time{})
	_ = cr.conn.SetDeadline(time.Time{})

	switch cr.effectiveMode {
	case "mitm":
		p.handleMITM(clientConn, cr.targetHost, cr.rule, cr.dialCandidates, cr.dialAddr)
	case "tls-rf":
		p.handleTLSFragment(clientConn, cr.conn, cr.targetHost, cr.rule)
	default:
		p.handleTransparent(clientConn, cr.conn, cr.targetHost, cr.rule)
	}
}

func (p *ProxyServer) directConnect(w http.ResponseWriter, req *http.Request) {
	targetAuthority := req.URL.Host
	if targetAuthority == "" {
		targetAuthority = req.Host
	}
	targetAddr := ensureAddrWithPort(targetAuthority, "443")

	log.Printf("[Direct] Connecting to %s", targetAddr)

	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	conn, err := dialer.Dial("tcp", targetAddr)
	if err != nil {
		http.Error(w, "Failed to connect", http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijack not supported", http.StatusInternalServerError)
		conn.Close()
		return
	}

	clientConn, rw, err := hijacker.Hijack()
	if err != nil {
		conn.Close()
		return
	}
	if _, err := rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		clientConn.Close()
		conn.Close()
		return
	}
	if err := rw.Flush(); err != nil {
		clientConn.Close()
		conn.Close()
		return
	}
	clientConn = wrapHijackedConn(clientConn, rw)
	_ = clientConn.SetDeadline(time.Time{})
	_ = conn.SetDeadline(time.Time{})

	// 双向复制数据
	var wg sync.WaitGroup
	wg.Add(2)

	buf1 := tunnelBufPool.Get().(*[]byte)
	buf2 := tunnelBufPool.Get().(*[]byte)

	go func() {
		defer wg.Done()
		defer tunnelBufPool.Put(buf1)
		io.CopyBuffer(conn, clientConn, *buf1)
		halfClose(conn)
	}()
	go func() {
		defer wg.Done()
		defer tunnelBufPool.Put(buf2)
		io.CopyBuffer(clientConn, conn, *buf2)
		halfClose(clientConn)
	}()
	wg.Wait()
	clientConn.Close()
	conn.Close()
}

func (p *ProxyServer) handleHTTP(w http.ResponseWriter, req *http.Request, rule Rule) {
	newReq := req.Clone(req.Context())
	newReq.RequestURI = ""
	newReq.Header.Del("Proxy-Connection")

	if newReq.URL.Scheme == "" {
		if req.TLS != nil {
			newReq.URL.Scheme = "https"
		} else {
			newReq.URL.Scheme = "http"
		}
	}
	if newReq.URL.Host == "" {
		newReq.URL.Host = req.Host
	}
	if newReq.Host == "" {
		newReq.Host = req.Host
	}
	if newReq.Host == "" {
		newReq.Host = newReq.URL.Host
	}

	if (rule.Mode == "mitm" || rule.Mode == "quic") && newReq.URL.Scheme == "http" {
		httpsURL := *newReq.URL
		httpsURL.Scheme = "https"
		if httpsURL.Host == "" {
			httpsURL.Host = req.Host
		}
		http.Redirect(w, req, httpsURL.String(), http.StatusMovedPermanently)
		return
	}

	if rule.Mode == "direct" {
		resp, err := p.transport.RoundTrip(newReq)
		if err != nil {
			log.Printf("[HTTP] Direct proxy failed: %v", err)
			http.Error(w, "Failed to proxy", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		return
	}

	transport := http.RoundTripper(p.transport)
	if rule.Upstream != "" {
		defaultPort := "80"
		if strings.EqualFold(newReq.URL.Scheme, "https") {
			defaultPort = "443"
		}
		candidates := p.buildDialCandidates(req.Context(), normalizeHost(newReq.Host), ensureAddrWithPort(newReq.URL.Host, defaultPort), rule, rule.Mode)
		if len(candidates) > 0 {
			newReq.URL.Host = candidates[0]
		}
	} else {
		defaultPort := "80"
		if strings.EqualFold(newReq.URL.Scheme, "https") {
			defaultPort = "443"
		}
		targetAddr := ensureAddrWithPort(newReq.URL.Host, defaultPort)
		dialCandidates := p.buildDialCandidates(req.Context(), normalizeHost(newReq.Host), targetAddr, rule, rule.Mode)
		if len(dialCandidates) > 0 && dialCandidates[0] != targetAddr {
			t := p.transport.Clone()
			candidateSet := dedupeDialCandidates(dialCandidates)
			t.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
				var lastErr error
				for _, candidate := range candidateSet {
					conn, err := p.dialWithRule(ctx, network, candidate, rule)
					if err == nil {
						return conn, nil
					}
					lastErr = err
				}
				return nil, lastErr
			}
			transport = t
		}
	}

	resp, err := transport.RoundTrip(newReq)
	if err != nil {
		log.Printf("[HTTP] Proxy failed: %v", err)
		http.Error(w, "Failed to connect to upstream", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (p *ProxyServer) directTunnel(clientConn, upstreamConn net.Conn) {
	p.tracef("[Tunnel] Starting direct tunnel")
	var wg sync.WaitGroup
	wg.Add(2)

	buf1 := tunnelBufPool.Get().(*[]byte)
	buf2 := tunnelBufPool.Get().(*[]byte)

	go func() {
		defer wg.Done()
		defer tunnelBufPool.Put(buf1)
		n, err := io.CopyBuffer(upstreamConn, clientConn, *buf1)
		p.tracef("[Tunnel] Client -> Upstream: %d bytes, err: %v", n, err)
		halfClose(upstreamConn)
	}()
	go func() {
		defer wg.Done()
		defer tunnelBufPool.Put(buf2)
		n, err := io.CopyBuffer(clientConn, upstreamConn, *buf2)
		p.tracef("[Tunnel] Upstream -> Client: %d bytes, err: %v", n, err)
		halfClose(clientConn)
	}()
	wg.Wait()
	clientConn.Close()
	upstreamConn.Close()
	p.tracef("[Tunnel] Tunnel closed")
}

func (p *ProxyServer) GetStats() (int64, int64, int64) {
	return atomic.LoadInt64(&p.bytesDown), atomic.LoadInt64(&p.bytesUp), 0
}
