package proxy

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"

	utls "github.com/refraction-networking/utls"
)

func (p *ProxyServer) handleMITM(clientConn net.Conn, host string, rule Rule, dialCandidates []string, initialDialAddr string) {
	defer func() {
		if r := recover(); r != nil {
			p.tracef("[MITM] Panic: %v", r)
			_ = clientConn.Close()
		}
	}()

	p.tracef("[MITM] Handling %s with SNI: %s", host, rule.SniFake)

	if p.certGenerator == nil {
		p.tracef("[MITM] No cert generator, falling back to direct")
		p.directTunnel(clientConn, clientConn)
		return
	}

	caCert := p.certGenerator.GetCACert()
	caKey := p.certGenerator.GetCAKey()
	if caCert == nil || caKey == nil {
		p.tracef("[MITM] CA cert/key not available")
		clientConn.Close()
		return
	}

	orderedCandidates := make([]string, 0, len(dialCandidates)+1)
	if strings.TrimSpace(initialDialAddr) != "" {
		orderedCandidates = append(orderedCandidates, initialDialAddr)
	}
	for _, c := range dialCandidates {
		if strings.TrimSpace(c) == "" || c == initialDialAddr {
			continue
		}
		orderedCandidates = append(orderedCandidates, c)
	}

	var (
		upstreamRW       net.Conn
		upstreamProtocol string
		upstreamErr      error
		dialOnce         sync.Once
	)

	tlsConfig := &tls.Config{
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			dialOnce.Do(func() {
				initialALPN := "h2_h1"
				if len(hello.SupportedProtos) > 0 {
					hasH2 := false
					for _, proto := range hello.SupportedProtos {
						if proto == "h2" {
							hasH2 = true
							break
						}
					}
					if !hasH2 {
						initialALPN = "http/1.1"
					}
				} else {
					initialALPN = "http/1.1"
				}

				p.tracef("[MITM] Client supported ALPNs: %v, selected initialALPN: %s", hello.SupportedProtos, initialALPN)
				p.tracef("[MITM] Establishing upstream via candidates=%v", orderedCandidates)
				upstreamRW, upstreamProtocol, upstreamErr = p.establishUpstreamConn(host, rule, orderedCandidates, initialALPN)
			})

			if upstreamErr != nil {
				p.tracef("[MITM] Failed to establish upstream in callback: %v", upstreamErr)
				return nil, upstreamErr
			}

			if upstreamRW == nil {
				return nil, fmt.Errorf("no usable upstream connection established")
			}

			p.tracef("[MITM] Upstream negotiated protocol: %s", upstreamProtocol)

			clientSNI := normalizeHost(hello.ServerName)
			if clientSNI != "" && host != "" && clientSNI != host {
				log.Printf("[MITM] ClientHello SNI mismatch: connect_host=%s client_sni=%s remote=%s", host, clientSNI, hello.Conn.RemoteAddr())
			} else {
				log.Printf("[MITM] ClientHello: connect_host=%s client_sni=%s remote=%s", host, clientSNI, hello.Conn.RemoteAddr())
			}

			var clientNextProtos []string
			if upstreamProtocol != "" {
				if upstreamProtocol == "h2" {
					clientNextProtos = []string{"h2", "http/1.1"}
				} else {
					clientNextProtos = []string{upstreamProtocol}
				}
			} else {
				clientNextProtos = []string{"http/1.1"}
			}

			certHost := host
			if hello.ServerName != "" {
				certHost = hello.ServerName
			}
			cert, err := p.generateCert(certHost, caCert, caKey)
			if err != nil {
				log.Printf("[MITM] Generate cert failed: cert_host=%s err=%v", certHost, err)
				return nil, err
			}
			log.Printf("[MITM] Serving MITM cert: cert_host=%s alpn=%v next_protos=%v", certHost, hello.SupportedProtos, clientNextProtos)
			return &tls.Config{
				Certificates: []tls.Certificate{*cert},
				NextProtos:   clientNextProtos,
			}, nil
		},
	}

	clientTls := tls.Server(clientConn, tlsConfig)
	if err := clientTls.Handshake(); err != nil {
		p.tracef("[MITM] Client TLS handshake failed: %v", err)
		clientConn.Close()
		if upstreamRW != nil {
			upstreamRW.Close()
		}
		return
	}
	defer func() {
		if upstreamRW != nil {
			upstreamRW.Close()
		}
	}()

	clientALPN := clientTls.ConnectionState().NegotiatedProtocol
	p.tracef("[MITM] Client ALPN: %s, Upstream Protocol: %s", clientALPN, upstreamProtocol)

	p.directTunnel(clientTls, upstreamRW)
}

func (p *ProxyServer) generateCert(host string, caCert *x509.Certificate, caKey interface{}) (*tls.Certificate, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return nil, errors.New("empty host for cert generation")
	}

	p.certCacheMu.RLock()
	if cached, ok := p.certCache[host]; ok {
		p.certCacheMu.RUnlock()
		return cached, nil
	}
	p.certCacheMu.RUnlock()

	p.certCacheMu.Lock()
	defer p.certCacheMu.Unlock()

	if cached, ok := p.certCache[host]; ok {
		return cached, nil
	}

	if len(p.certCache) > 1000 {
		p.certCache = make(map[string]*tls.Certificate)
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key failed: %w", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("generate serial number failed: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"SniShaper"},
			CommonName:   host,
		},
		NotBefore: time.Now().Add(-24 * time.Hour),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{host}
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, caCert, &priv.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("create cert failed: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("marshal key failed: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("load keypair failed: %w", err)
	}

	p.certCache[host] = &cert
	return &cert, nil
}

func (p *ProxyServer) makeMITMTLSConfig(connectHost string, caCert *x509.Certificate, caKey interface{}, nextProtos []string, logPrefix string) *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		NextProtos: nextProtos,
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			name := info.ServerName
			if name == "" {
				name = connectHost
			}
			return p.generateCert(name, caCert, caKey)
		},
	}
}

func (p *ProxyServer) handleTransparent(clientConn, upstreamConn net.Conn, host string, rule Rule) {
	if upstreamConn == nil {
		p.tracef("[Tunnel] Dialing upstream directly for %s...", host)
		var err error

		defaultPort := "443"
		targetAddr := ensureAddrWithPort(host, defaultPort)
		dialCandidates := p.buildDialCandidates(context.Background(), normalizeHost(host), targetAddr, rule, rule.Mode)
		if len(dialCandidates) == 0 {
			dialCandidates = []string{targetAddr}
		}

		var lastErr error
		for _, cand := range dialCandidates {
			upstreamConn, err = p.dialWithRule(context.Background(), "tcp", cand, rule)
			if err == nil {
				break
			}
			lastErr = err
		}
		if err != nil {
			log.Printf("[Tunnel] Direct dial upstream failed for %s: %v", host, lastErr)
			clientConn.Close()
			return
		}
	}

	p.directTunnel(clientConn, upstreamConn)
}

func (p *ProxyServer) GetUConn(conn net.Conn, sni string, verifyName string, rule Rule, allowInsecure bool, alpn string, echConfig []byte) *utls.UConn {
	nextProtos := []string{"h2", "http/1.1"}
	if strings.EqualFold(strings.TrimSpace(alpn), "http/1.1") {
		nextProtos = []string{"http/1.1"}
	}

	verifyConn := buildVerifyConnection(verifyName, rule.CertVerify)

	var serverName string
	if len(echConfig) > 0 {
		serverName = verifyName
	} else {
		serverName = sni
		if serverName == "" {
			serverName = verifyName
		}
	}

	skipVerify := allowInsecure
	if rule.CertVerify.Mode != "" {
		skipVerify = true
	}

	if _, ok := p.certBypassMap.Load(normalizeHost(verifyName)); ok {
		skipVerify = true
		verifyConn = nil
	}

	if len(echConfig) > 0 {
		skipVerify = false
		verifyConn = nil
	}

	config := &utls.Config{
		ServerName:                     serverName,
		InsecureSkipVerify:             skipVerify,
		EncryptedClientHelloConfigList: echConfig,
		NextProtos:                     nextProtos,
		VerifyConnection:               verifyConn,
	}

	if len(echConfig) > 0 {
		config.InsecureServerNameToVerify = "*"
		// ECH 被服务器拒绝时，uTLS 会验证外层 public_name 证书。
		// 返回 nil 允许握手继续，以便提取 RetryConfigList 进行纠错重试。
		// CA 链验证仍由 uTLS 内部 RootCAs 保证安全性。
		config.EncryptedClientHelloRejectionVerify = func(cs utls.ConnectionState) error {
			return nil
		}
	}

	clientHelloID := chooseUTLSClientHelloID(alpn)
	uconn := utls.UClient(conn, config, utls.HelloCustom)
	if spec, err := utls.UTLSIdToSpec(clientHelloID); err == nil {
		rewriteUTLSALPN(&spec, nextProtos)
		if err := uconn.ApplyPreset(&spec); err == nil {
			return uconn
		}
	}
	uconn = utls.UClient(conn, config, clientHelloID)
	return uconn
}

func (p *ProxyServer) resolveRuleECHConfig(host string, rule Rule) []byte {
	if !rule.ECHEnabled {
		return nil
	}

	// 1. Runtime hot-patch from a previous ECH rejection (highest priority).
	if rule.ECHProfileID != "" {
		if v, ok := p.echRuntimeConfigs.Load(rule.ECHProfileID); ok {
			if b, ok := v.([]byte); ok && len(b) > 0 {
				return b
			}
		}
	}
	if host != "" {
		if v, ok := p.echRuntimeConfigs.Load("host:" + host); ok {
			if b, ok := v.([]byte); ok && len(b) > 0 {
				return b
			}
		}
	}

	p.mu.RLock()
	rules := p.rules
	p.mu.RUnlock()
	if rules == nil {
		return nil
	}

	// 2. Persisted ECH profile.
	if rule.ECHProfileID != "" {
		if bytes := rules.GetBinaryECHConfig(rule.ECHProfileID); len(bytes) > 0 {
			return bytes
		}
	}

	// 3. Live DNS discovery.
	lookupDomain := rule.ECHDiscoveryDomain
	if lookupDomain == "" {
		lookupDomain = host
	}

	if p.dohResolver != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if bytes, err := p.dohResolver.ResolveECH(ctx, lookupDomain); err == nil && len(bytes) > 0 {
			p.applyECHConfig(host, rule, bytes, "dns-discovery")
			return bytes
		}
	}

	return nil
}

// applyECHConfig stores a fresh ECH config list for immediate retries and optionally
// persists it into the linked ECH profile.
func (p *ProxyServer) applyECHConfig(host string, rule Rule, config []byte, source string) {
	if len(config) == 0 {
		return
	}
	if rule.ECHProfileID != "" {
		p.echRuntimeConfigs.Store(rule.ECHProfileID, append([]byte(nil), config...))
		p.UpdateECHProfileConfig(rule.ECHProfileID, config)
	}
	if host != "" {
		p.echRuntimeConfigs.Store("host:"+host, append([]byte(nil), config...))
	}
	p.tracef("[Upstream] Applied ECH config host=%s profile=%s source=%s len=%d", host, rule.ECHProfileID, source, len(config))
}

func (p *ProxyServer) NewQUICRoundTripper(host string, rule Rule) (*http3.Transport, error) {
	targetAddr := net.JoinHostPort(host, "443")
	dialCandidates := p.buildDialCandidates(context.Background(), host, targetAddr, rule, "quic")
	if len(dialCandidates) == 0 {
		dialCandidates = []string{targetAddr}
	}

	sniHost := chooseUpstreamSNI(host, rule)
	if sniHost == "" {
		sniHost = host
	}

	var echConfig []byte
	if rule.ECHEnabled {
		echConfig = p.resolveRuleECHConfig(host, rule)
	}

	innerSNI := host
	if len(echConfig) == 0 {
		innerSNI = sniHost
	}

	verifyConn := buildVerifyConnection(host, rule.CertVerify)
	tlsConfig := &tls.Config{
		ServerName:         innerSNI,
		NextProtos:         []string{"h3", "h3-29", "h3-32"},
		InsecureSkipVerify: true,
	}

	if len(echConfig) > 0 {
		tlsConfig.EncryptedClientHelloConfigList = echConfig
		tlsConfig.InsecureSkipVerify = false
		log.Printf("[QUIC] ECH enabled host=%s innerSNI=%s echLen=%d", host, innerSNI, len(echConfig))
	}

	if verifyConn != nil && len(echConfig) == 0 {
		tlsConfig.VerifyConnection = func(cs tls.ConnectionState) error {
			peer := make([]*x509.Certificate, len(cs.PeerCertificates))
			copy(peer, cs.PeerCertificates)
			return verifyConn(utls.ConnectionState{
				Version:                     cs.Version,
				HandshakeComplete:           cs.HandshakeComplete,
				DidResume:                   cs.DidResume,
				CipherSuite:                 cs.CipherSuite,
				NegotiatedProtocol:          cs.NegotiatedProtocol,
				NegotiatedProtocolIsMutual:  cs.NegotiatedProtocolIsMutual,
				ServerName:                  cs.ServerName,
				PeerCertificates:            peer,
				VerifiedChains:              cs.VerifiedChains,
				SignedCertificateTimestamps: cs.SignedCertificateTimestamps,
				OCSPResponse:                cs.OCSPResponse,
				TLSUnique:                   cs.TLSUnique,
				ECHAccepted:                 cs.ECHAccepted,
			})
		}
	}

	return &http3.Transport{
		TLSClientConfig: tlsConfig,
		QUICConfig: &quic.Config{
			HandshakeIdleTimeout: 10 * time.Second,
		},
		Dial: func(ctx context.Context, _ string, tlsCfg *tls.Config, cfg *quic.Config) (*quic.Conn, error) {
			var errs []string
			for _, candidate := range dialCandidates {
				conn, err := quic.DialAddr(ctx, candidate, tlsCfg, cfg)
				if err == nil {
					cs := conn.ConnectionState().TLS
					log.Printf("[QUIC] H3 dial success host=%s addr=%s sni=%s alpn=%s echAccepted=%v", host, candidate, tlsCfg.ServerName, cs.NegotiatedProtocol, cs.ECHAccepted)
					return conn, nil
				}
				errs = append(errs, fmt.Sprintf("%s: %v", candidate, err))
				log.Printf("[QUIC] H3 dial failed host=%s addr=%s err=%v", host, candidate, err)
			}
			if len(errs) == 0 {
				return nil, fmt.Errorf("no QUIC dial candidates for %s", host)
			}
			return nil, fmt.Errorf("all QUIC dial candidates failed for %s: %s", host, strings.Join(errs, "; "))
		},
	}, nil
}

func (p *ProxyServer) handleQUICMITM(clientConn net.Conn, host string, rule Rule) {
	defer clientConn.Close()
	log.Printf("[QUICMode] Handling %s via local H3 replay", host)

	if p.certGenerator == nil {
		log.Printf("[QUICMode] No cert generator available")
		return
	}
	caCert := p.certGenerator.GetCACert()
	caKey := p.certGenerator.GetCAKey()

	// TLS: offer h2 + http/1.1. Go's http.Server automatically switches
	// to HTTP/2 when ALPN selects "h2" — no extra config needed.
	tlsConfig := p.makeMITMTLSConfig(host, caCert, caKey, []string{"http/1.1"}, "[QUICMode]")
	clientTLS := tls.Server(clientConn, tlsConfig)
	if err := clientTLS.Handshake(); err != nil {
		log.Printf("[QUICMode] Client TLS handshake failed: %v", err)
		return
	}
	log.Printf("[QUICMode] TLS handshake OK for %s, negotiated proto: %s", host, clientTLS.ConnectionState().NegotiatedProtocol)

	quicTransport, err := p.NewQUICRoundTripper(host, rule)
	if err != nil {
		log.Printf("[QUICMode] Failed to create HTTP/3 transport: %v", err)
		return
	}
	defer quicTransport.Close()

	client := &http.Client{
		Transport: quicTransport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	srv := &http.Server{
		// No ReadTimeout/WriteTimeout — H2 connections are long-lived.
		// http.Server handles H2 framing, flow-control, GOAWAY automatically.
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			path := req.URL.EscapedPath()
			if path == "" || !strings.HasPrefix(path, "/") {
				path = "/" + strings.TrimPrefix(path, "/")
			}

			targetURL := "https://" + host + path
			if req.URL.RawQuery != "" {
				targetURL += "?" + req.URL.RawQuery
			}

			newReq, err := http.NewRequestWithContext(req.Context(), req.Method, targetURL, req.Body)
			if err != nil {
				http.Error(w, "Bad request", http.StatusInternalServerError)
				return
			}
			for k, vv := range req.Header {
				for _, v := range vv {
					newReq.Header.Add(k, v)
				}
			}
			removeHopByHopHeaders(newReq.Header)
			newReq.Host = host

			resp, err := client.Do(newReq)
			if err != nil {
				log.Printf("[QUICMode] Forwarding error method=%s host=%s target=%s err=%v", req.Method, host, targetURL, err)
				http.Error(w, "Proxy error", http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()

			// Forward response headers, stripping hop-by-hop and H2-invalid headers.
			removeHopByHopHeaders(resp.Header)
			for k, vv := range resp.Header {
				if strings.EqualFold(k, "Alt-Svc") {
					continue // don't advertise H3 to the browser
				}
				for _, v := range vv {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(resp.StatusCode)
			_, _ = io.Copy(w, resp.Body)
		}),
	}

	_ = srv.Serve(&singleConnListener{conn: clientTLS, done: make(chan struct{})})
}

// isHopByHopHeader reports headers that must not be forwarded across hops
// (RFC 7230 §6.1). HTTP/3 also rejects several of these.
func isHopByHopHeader(name string) bool {
	switch strings.ToLower(name) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
		"te", "trailers", "transfer-encoding", "upgrade", "proxy-connection":
		return true
	default:
		return false
	}
}

func removeHopByHopHeaders(h http.Header) {
	if h == nil {
		return
	}
	if c := h.Get("Connection"); c != "" {
		for _, f := range strings.Split(c, ",") {
			if name := strings.TrimSpace(f); name != "" {
				if !strings.EqualFold(name, "Upgrade") {
					h.Del(name)
				}
			}
		}
	}
	for _, name := range []string{"Connection", "Keep-Alive", "Proxy-Authenticate",
		"Proxy-Authorization", "Te", "Trailers", "Transfer-Encoding", "Upgrade"} {
		h.Del(name)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// oneConnListener exposes a single net.Conn to http.Server.Serve and keeps
// Accept blocked until that connection (or the listener) is closed. This
// prevents Serve from returning early and tearing down the request path.
type oneConnListener struct {
	addr      net.Addr
	ch        chan net.Conn
	closed    chan struct{}
	closeOnce sync.Once
}

func newOneConnListener(conn net.Conn) *oneConnListener {
	l := &oneConnListener{
		addr:   conn.LocalAddr(),
		ch:     make(chan net.Conn, 1),
		closed: make(chan struct{}),
	}
	l.ch <- &closeNotifyConn{
		Conn: conn,
		onClose: func() {
			l.signalClosed()
		},
	}
	return l
}

func (o *oneConnListener) signalClosed() {
	o.closeOnce.Do(func() {
		close(o.closed)
		// Drop the pending conn if Accept never took it.
		select {
		case c := <-o.ch:
			_ = c.Close()
		default:
		}
	})
}

func (o *oneConnListener) Accept() (net.Conn, error) {
	select {
	case c, ok := <-o.ch:
		if !ok {
			return nil, net.ErrClosed
		}
		return c, nil
	case <-o.closed:
		return nil, net.ErrClosed
	}
}

func (o *oneConnListener) Close() error {
	o.signalClosed()
	return nil
}

func (o *oneConnListener) Addr() net.Addr {
	if o.addr != nil {
		return o.addr
	}
	return &net.TCPAddr{}
}

// closeNotifyConn notifies the listener when the connection ends, so the
// second Accept can unblock and http.Server.Serve can return cleanly.
type closeNotifyConn struct {
	net.Conn
	onClose func()
	once    sync.Once
}

func (c *closeNotifyConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() {
		if c.onClose != nil {
			c.onClose()
		}
	})
	return err
}

type singleConnListener struct {
	conn     *tls.Conn
	accepted sync.Once
	done     chan struct{}
	doneOnce sync.Once
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	var accepted bool
	l.accepted.Do(func() { accepted = true })
	if accepted {
		go func() {
			buf := make([]byte, 32*1024)
			for {
				if _, err := l.conn.Read(buf); err != nil {
					l.doneOnce.Do(func() { close(l.done) })
					return
				}
			}
		}()
		return l.conn, nil
	}
	<-l.done
	return nil, io.EOF
}
func (l *singleConnListener) Close() error {
	l.doneOnce.Do(func() { close(l.done) })
	return nil
}
func (l *singleConnListener) Addr() net.Addr { return l.conn.LocalAddr() }



func (p *ProxyServer) ClearCertCache() {
	p.certCacheMu.Lock()
	defer p.certCacheMu.Unlock()
	p.certCache = make(map[string]*tls.Certificate)
}

func (p *ProxyServer) certCacheCleanup(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.ClearCertCache()
		case <-ctx.Done():
			return
		}
	}
}

func chooseUTLSClientHelloID(alpn string) utls.ClientHelloID {
	if strings.EqualFold(strings.TrimSpace(alpn), "http/1.1") {
		return utls.HelloFirefox_120
	}
	return utls.HelloChrome_120
}

func rewriteUTLSALPN(spec *utls.ClientHelloSpec, nextProtos []string) {
	if spec == nil {
		return
	}
	for _, ext := range spec.Extensions {
		if alpnExt, ok := ext.(*utls.ALPNExtension); ok {
			alpnExt.AlpnProtocols = append([]string(nil), nextProtos...)
			return
		}
	}
	spec.Extensions = append(spec.Extensions, &utls.ALPNExtension{
		AlpnProtocols: append([]string(nil), nextProtos...),
	})
}

func chooseUpstreamSNI(targetHost string, rule Rule) string {
	targetHost = normalizeHost(targetHost)
	hostAsToken := strings.Trim(targetHost, "[]")
	hostAsToken = strings.ReplaceAll(hostAsToken, ".", "-")
	hostAsToken = strings.ReplaceAll(hostAsToken, ":", "-")
	hostAsToken = strings.TrimSpace(hostAsToken)
	if hostAsToken == "" {
		hostAsToken = "g-cn"
	}
	resolvedUpstream := resolveRuleUpstream(targetHost, rule)

	switch strings.ToLower(strings.TrimSpace(rule.SniPolicy)) {
	case "none":
		return ""
	case "original":
		return targetHost
	case "fake":
		if strings.TrimSpace(rule.SniFake) != "" {
			return rule.SniFake
		}
		if rule.ECHEnabled {
			return targetHost
		}
		return hostAsToken
	case "upstream":
		if upstreamHost := firstUpstreamHost(targetHost, resolvedUpstream); upstreamHost != "" && !isLiteralIP(upstreamHost) {
			return upstreamHost
		}
		return targetHost
	}

	if strings.TrimSpace(rule.SniFake) != "" {
		return rule.SniFake
	}
	if resolvedUpstream != "" {
		if upstreamHost := firstUpstreamHost(targetHost, resolvedUpstream); upstreamHost != "" {
			if !isLiteralIP(upstreamHost) && upstreamHost != targetHost {
				return upstreamHost
			}
		}
	}
	return targetHost
}


