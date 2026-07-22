package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"snishaper/pkg/cfpool"
	"snishaper/pkg/dohresolver"

	"github.com/miekg/dns"
	"github.com/things-go/go-socks5"
	utls "github.com/refraction-networking/utls"
)

// tunnelBufPool provides reusable 128KB buffers for tunnel data copying
// to reduce memory allocation and GC pressure in high-concurrency scenarios.
var tunnelBufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 128*1024)
		return &buf
	},
}

type CertGenerator interface {
	GetCACert() *x509.Certificate
	GetCAKey() interface{}
	IsCAInstalled() bool
}

type connectResult struct {
	effectiveMode  string
	targetHost     string
	targetAddr     string
	rule           Rule
	dialCandidates []string
	dialAddr       string
	conn           net.Conn
}

type DNSNode struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	URL           string           `json:"url"`
	SNI           string           `json:"sni,omitempty"`
	IPs           []string         `json:"ips,omitempty"`
	ECHEnabled    bool             `json:"ech_enabled"`
	ECHProfileID  string           `json:"ech_profile_id,omitempty"`
	ECHAutoUpdate bool             `json:"ech_auto_update"`
	QUIC          bool             `json:"quic"`
	Enabled       bool             `json:"enabled"`
	CertVerify    CertVerifyConfig `json:"cert_verify,omitempty"`
}

type CloudflareConfig struct {
	PreferredIPs []string  `json:"preferred_ips"`
	AutoUpdate   bool      `json:"auto_update"`
	APIKey       string    `json:"api_key"`
	DNSNodes     []DNSNode `json:"dns_nodes,omitempty"`
}

type TUNConfig struct {
	Enabled     bool `json:"enabled"`
	MTU         int  `json:"mtu,omitempty"`
	DNSHijack   bool `json:"dns_hijack,omitempty"`
	AutoRoute   bool `json:"auto_route,omitempty"`
	StrictRoute bool `json:"strict_route,omitempty"`
}

type TUNStatus struct {
	Supported bool   `json:"supported"`
	Running   bool   `json:"running"`
	Enabled   bool   `json:"enabled"`
	Driver    string `json:"driver,omitempty"`
	Message   string `json:"message,omitempty"`
}

type ECHProfile struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Config          string `json:"config"`
	DiscoveryDomain string `json:"discovery_domain,omitempty"`
	DoHUpstream     string `json:"doh_upstream,omitempty"`
	AutoUpdate      bool   `json:"auto_update"`
}

type SettingsConfig struct {
	ListenPort                 string            `json:"listen_port"`
	Socks5Port                 string            `json:"socks5_port,omitempty"`
	CloseToTray                *bool             `json:"close_to_tray,omitempty"`
	AutoStart                  *bool             `json:"auto_start,omitempty"`
	ShowMainWindowOnAutoStart  *bool             `json:"show_main_window_on_auto_start,omitempty"`
	AutoEnableProxyOnAutoStart *bool             `json:"auto_enable_proxy_on_auto_start,omitempty"`
	AutoRouting                AutoRoutingConfig `json:"auto_routing,omitempty"`
	TUN                        TUNConfig         `json:"tun,omitempty"`
	Language                   string            `json:"language,omitempty"`
	Theme                      string            `json:"theme,omitempty"`
	CloudflareConfig           CloudflareConfig  `json:"cloudflare_config,omitempty"`
	Socks5Enabled              *bool             `json:"socks5_enabled,omitempty"`
	MigrationEnabled           *bool             `json:"migration_enabled,omitempty"`
	MigrationServer            string            `json:"migration_server,omitempty"`
}

type NAT64Profile struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
}

type Rule struct {
	Domain             string           `json:"domain"`
	Upstream           string           `json:"upstream,omitempty"`
	Upstreams          []string         `json:"upstreams,omitempty"`
	DNSMode            string           `json:"dns_mode,omitempty"`
	Mode               string           `json:"mode"` // "mitm", "transparent", "tls-rf", "quic", "direct"
	SniFake            string           `json:"sni_fake,omitempty"`
	ConnectPolicy      string           `json:"connect_policy,omitempty"`
	SniPolicy          string           `json:"sni_policy,omitempty"`
	Enabled            bool             `json:"enabled"`
	SiteID             string           `json:"site_id,omitempty"`
	ECHEnabled         bool             `json:"ech_enabled"`
	ECHProfileID       string           `json:"ech_profile_id,omitempty"`
	UseCFPool          bool             `json:"use_cf_pool"`
	ECHDiscoveryDomain string           `json:"ech_discovery_domain,omitempty"`
	ECHDoHUpstream     string           `json:"ech_doh_upstream,omitempty"`
	FallbackMode       string           `json:"fallback_mode,omitempty"`
	CertVerify         CertVerifyConfig `json:"cert_verify,omitempty"`
	ECHAutoUpdate      bool             `json:"ech_auto_update"`
	AutoRouted         bool             `json:"auto_routed,omitempty"`
	NAT64Enabled       bool             `json:"nat64_enabled"`
	NAT64ProfileID     string           `json:"nat64_profile_id,omitempty"`
}

type SiteGroup struct {
	ID                 string           `json:"id"`
	Name               string           `json:"name"`
	Mode               string           `json:"mode"`
	DNSMode            string           `json:"dns_mode,omitempty"`
	SniFake            string           `json:"sni_fake,omitempty"`
	ConnectPolicy      string           `json:"connect_policy,omitempty"`
	SniPolicy          string           `json:"sni_policy,omitempty"`
	Domains            []string         `json:"domains"`
	ECHEnabled         bool             `json:"ech_enabled"`
	ECHProfileID       string           `json:"ech_profile_id,omitempty"`
	UseCFPool          bool             `json:"use_cf_pool"`
	ECHDiscoveryDomain string           `json:"ech_discovery_domain,omitempty"`
	ECHDoHUpstream     string           `json:"ech_doh_upstream,omitempty"`
	FallbackMode       string           `json:"fallback_mode,omitempty"`
	CertVerify         CertVerifyConfig `json:"cert_verify,omitempty"`
	Website            string           `json:"website,omitempty"`
	Enabled            bool             `json:"enabled"`
	Upstream           string           `json:"upstream,omitempty"`
	Upstreams          []string         `json:"upstreams,omitempty"`
	ECHDomain          string           `json:"ech_domain,omitempty"`
	NAT64Enabled       bool             `json:"nat64_enabled"`
	NAT64ProfileID     string           `json:"nat64_profile_id,omitempty"`
}

type Upstream struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Type      string   `json:"type"` // "cf_preferred"
	Addresses []string `json:"addresses"`
	Enabled   bool     `json:"enabled"`
	Address   string   `json:"address,omitempty"`
}

type ProxyServer struct {
	startStopMu   sync.Mutex
	Server        *http.Server
	listenAddr    string
	socks5Addr    string
	rules         *RuleManager
	running       bool
	mode          string
	mu            sync.RWMutex
	certCacheMu   sync.RWMutex
	certCache     map[string]*tls.Certificate
	Fingerprint   string
	certGenerator CertGenerator
	dohResolver   *dohresolver.FailoverResolver
	cfPool        *cfpool.CloudflarePool
	transport     *http.Transport
	logCallback   func(string)
	bytesDown     int64
	bytesUp       int64
	certBypassMap sync.Map
	// echRuntimeConfigs holds hot-patched ECH configs after server rejection
	// (keyed by profile ID or "host:<name>"). Preferred over persisted profiles
	// so a retry can take effect even when profile save fails or ID is empty.
	echRuntimeConfigs sync.Map
	socks5Enabled     bool
	socks5Server      *socks5.Server
	socks5Tracker     *socks5ConnTracker

	// CF IP pool 刷新回调：当池过期时由 app 层注入
	cfRefreshCallback func()

	// migrationCache holds persistent session tickets for migration mode,
	// keyed by host name. Tickets are reused across requests until they fail.
	migrationCache *migrationSessionCache
}

type dohProxyAdapter struct {
	p *ProxyServer
}

func (a *dohProxyAdapter) DialWithRule(ctx context.Context, network, addr string, rule dohresolver.Rule) (net.Conn, error) {
	r := Rule{
		SniFake:       rule.SniFake,
		ECHEnabled:    rule.ECHEnabled,
		ECHProfileID:  rule.ECHProfileID,
		ECHAutoUpdate: rule.ECHAutoUpdate,
		CertVerify:    toProxyCertVerify(rule.CertVerify),
	}
	return a.p.dialWithRule(ctx, network, addr, r)
}

func (a *dohProxyAdapter) GetUConn(conn net.Conn, sni, verifyName string, rule dohresolver.Rule, allowUnknownAuthority bool, alpn string, ech []byte) *utls.UConn {
	r := Rule{
		SniFake:       rule.SniFake,
		ECHEnabled:    rule.ECHEnabled,
		ECHProfileID:  rule.ECHProfileID,
		ECHAutoUpdate: rule.ECHAutoUpdate,
		CertVerify:    toProxyCertVerify(rule.CertVerify),
	}
	return a.p.GetUConn(conn, sni, verifyName, r, allowUnknownAuthority, alpn, ech)
}

func (a *dohProxyAdapter) ResolveRuleECHConfig(host string, rule dohresolver.Rule) []byte {
	r := Rule{
		SniFake:       rule.SniFake,
		ECHEnabled:    rule.ECHEnabled,
		ECHProfileID:  rule.ECHProfileID,
		ECHAutoUpdate: rule.ECHAutoUpdate,
		CertVerify:    toProxyCertVerify(rule.CertVerify),
	}
	return a.p.resolveRuleECHConfig(host, r)
}

func (a *dohProxyAdapter) UpdateECHProfileConfig(profileID string, configBytes []byte) {
	a.p.UpdateECHProfileConfig(profileID, configBytes)
}

func NewProxyServer(addr string) *ProxyServer {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	p := &ProxyServer{
		listenAddr:  addr,
		certCache:   make(map[string]*tls.Certificate),
		mode:        "direct",
		transport:   transport,
	}

	p.dohResolver = dohresolver.NewFailoverResolver(&dohProxyAdapter{p: p}, func() []dohresolver.DNSNode {
		if p.rules == nil {
			return nil
		}
		var nodes []dohresolver.DNSNode
		for _, node := range p.rules.GetDNSNodes() {
			nodes = append(nodes, dohresolver.DNSNode{
				Name:          node.Name,
				URL:           node.URL,
				SNI:           node.SNI,
				IPs:           node.IPs,
				ECHEnabled:    node.ECHEnabled,
				ECHProfileID:  node.ECHProfileID,
				ECHAutoUpdate: node.ECHAutoUpdate,
				QUIC:          node.QUIC,
				Enabled:       node.Enabled,
				CertVerify: toDohCertVerify(node.CertVerify),
			})
		}
		return nodes
	})

	return p
}

func (p *ProxyServer) SetRuleManager(rm *RuleManager) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rules = rm
}

func (p *ProxyServer) SetCertGenerator(cg CertGenerator) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.certGenerator = cg
}

func (p *ProxyServer) SetLogCallback(cb func(string)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.logCallback = cb
}

func (p *ProxyServer) tracef(format string, args ...interface{}) {
	p.mu.RLock()
	cb := p.logCallback
	p.mu.RUnlock()
	if cb != nil {
		cb(fmt.Sprintf(format, args...))
	} else {
		log.Printf(format, args...)
	}
}

func (p *ProxyServer) UpdateCloudflareConfig(cfg CloudflareConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if cfg.AutoUpdate {
		p.updateCloudflareIPPoolLocked(cfg.PreferredIPs)
	}
}

func (p *ProxyServer) UpdateCloudflareIPPool(ips []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.updateCloudflareIPPoolLocked(ips)
}

func (p *ProxyServer) updateCloudflareIPPoolLocked(ips []string) {
	if p.cfPool == nil {
		p.cfPool = cfpool.NewCloudflarePool(ips)
		return
	}
	p.cfPool.UpdateIPs(ips)
}

func (p *ProxyServer) SetCFPoolFetchTime(t time.Time) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.cfPool != nil {
		p.cfPool.SetLastFetchTime(t)
	}
}

func (p *ProxyServer) GetCFPool() *cfpool.CloudflarePool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cfPool
}

// CFPoolUsable returns true when the CF pool exists, has IPs, and is not stale (>1 day).
// If stale, it fires an async refresh in the background.
func (p *ProxyServer) CFPoolUsable() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.cfPool == nil {
		return false
	}
	if !p.cfPool.HasIPs() {
		return false
	}
	if p.cfPool.IsStale(24 * time.Hour) {
		return false
	}
	return true
}

// maybeRefreshCFPoolAsync triggers an async IP refresh when the pool is stale.
func (p *ProxyServer) maybeRefreshCFPoolAsync() {
	p.mu.RLock()
	stale := p.cfPool == nil || !p.cfPool.HasIPs() || p.cfPool.IsStale(24*time.Hour)
	cb := p.cfRefreshCallback
	p.mu.RUnlock()
	if !stale || cb == nil {
		return
	}
	go cb()
}

func (p *ProxyServer) SetCFRefreshCallback(cb func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cfRefreshCallback = cb
}

func (p *ProxyServer) SetListenAddr(addr string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return fmt.Errorf("cannot change address while proxy is running")
	}
	p.listenAddr = addr
	return nil
}

func (p *ProxyServer) TriggerCFHealthCheck() {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.cfPool != nil {
		p.cfPool.TriggerHealthCheck()
	}
}

func (p *ProxyServer) RemoveInvalidCFIPs() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cfPool != nil {
		return p.cfPool.RemoveInvalidIPs()
	}
	return 0
}

func (p *ProxyServer) GetAllCFIPsWithStats() []*cfpool.IPStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.cfPool != nil {
		return p.cfPool.GetAllIPsWithStats()
	}
	return nil
}

func (p *ProxyServer) GetListenAddr() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.listenAddr
}

func (p *ProxyServer) GetDoHResolver() *dohresolver.FailoverResolver {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.dohResolver
}

func (p *ProxyServer) UpdateECHProfileConfig(profileID string, configBytes []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.rules != nil {
		p.rules.UpdateECHProfileConfig(profileID, configBytes)
	}
}

func (p *ProxyServer) GetMode() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mode
}

func (p *ProxyServer) SetMode(mode string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mode = mode
	return nil
}

func (p *ProxyServer) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

func (p *ProxyServer) Start() error {
	p.startStopMu.Lock()
	defer p.startStopMu.Unlock()

	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return nil
	}

	srv := &http.Server{
		Addr:         p.listenAddr,
		Handler:      http.HandlerFunc(p.handleRequest),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	listenAddr := p.listenAddr

	if p.cfPool != nil {
		p.cfPool.Start()
	}

	p.mu.Unlock()

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		if p.cfPool != nil {
			p.cfPool.Stop()
		}
		return fmt.Errorf("failed to listen on %s: %w", listenAddr, err)
	}

	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		_ = ln.Close()
		return nil
	}
	p.Server = srv
	p.running = true
	p.mu.Unlock()

	// Periodic cert cache cleanup (异步化运行，解决永久阻塞)
	go p.certCacheCleanup(context.Background())

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[Proxy] panic in HTTP server: %v", r)
			}
		}()
		log.Printf("[Proxy] HTTP server started on %s", listenAddr)

		tl := &trackingListener{
			Listener: ln,
			proxy:    p,
		}

		if err := srv.Serve(tl); err != nil && err != http.ErrServerClosed {
			log.Printf("[Proxy] HTTP server error: %v", err)
		}

		p.mu.Lock()
		if p.Server == srv {
			p.running = false
		}
		p.mu.Unlock()
	}()

	if p.socks5Enabled {
		p.startSocks5()
	}

	return nil
}

func (p *ProxyServer) Stop() error {
	p.startStopMu.Lock()
	defer p.startStopMu.Unlock()

	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return nil
	}
	p.running = false

	if p.socks5Tracker != nil {
		_ = p.socks5Tracker.Close()
		p.socks5Tracker = nil
	}

	if p.cfPool != nil {
		p.cfPool.Stop()
	}

	var err error
	if p.Server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err = p.Server.Shutdown(ctx)
		p.Server = nil
	}
	p.mu.Unlock()

	p.tracef("[Proxy] Server stopped")
	return err
}

func (p *ProxyServer) SetSocks5Addr(addr string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.socks5Addr = addr
}

func (p *ProxyServer) SetSocks5Enabled(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.socks5Enabled = enabled
	if p.running {
		if enabled {
			p.startSocks5()
		} else {
			if p.socks5Tracker != nil {
				_ = p.socks5Tracker.Close()
				p.socks5Tracker = nil
			}
		}
	}
}

func (p *ProxyServer) IsSocks5Enabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.socks5Enabled
}

type trackingListener struct {
	net.Listener
	proxy *ProxyServer
}

type statConn struct {
	net.Conn
	bytesDown *int64
	bytesUp   *int64
}

func (c *statConn) Read(p []byte) (n int, err error) {
	n, err = c.Conn.Read(p)
	if n > 0 {
		atomic.AddInt64(c.bytesUp, int64(n))
	}
	return n, err
}

func (c *statConn) Write(p []byte) (n int, err error) {
	n, err = c.Conn.Write(p)
	if n > 0 {
		atomic.AddInt64(c.bytesDown, int64(n))
	}
	return n, err
}

func (l *trackingListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &statConn{
		Conn:      conn,
		bytesDown: &l.proxy.bytesDown,
		bytesUp:   &l.proxy.bytesUp,
	}, nil
}

type bufferedReadConn struct {
	net.Conn
	reader io.Reader
}

func (c *bufferedReadConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func (c *bufferedReadConn) WriteTo(w io.Writer) (int64, error) {
	return io.Copy(w, c.reader)
}

func enableKeepAlive(conn net.Conn) {
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetKeepAlive(true)
		_ = tc.SetKeepAlivePeriod(30 * time.Second)
	}
}

func wrapHijackedConn(conn net.Conn, rw *bufio.ReadWriter) net.Conn {
	enableKeepAlive(conn)
	if rw == nil || rw.Reader == nil || rw.Reader.Buffered() == 0 {
		return conn
	}
	n := rw.Reader.Buffered()
	buffered := make([]byte, n)
	_, _ = rw.Reader.Read(buffered)

	return &bufferedReadConn{
		Conn:   conn,
		reader: io.MultiReader(bytes.NewReader(buffered), conn),
	}
}

type socks5ConnTracker struct {
	net.Listener
	conns sync.Map
}

func (l *socks5ConnTracker) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	l.conns.Store(conn.RemoteAddr().String(), conn)
	return &socks5TrackedConn{Conn: conn, tracker: l}, nil
}

func (l *socks5ConnTracker) unregister(addr string) {
	l.conns.Delete(addr)
}

func (l *socks5ConnTracker) getConn(addr string) net.Conn {
	if v, ok := l.conns.Load(addr); ok {
		return v.(net.Conn)
	}
	return nil
}

func (l *socks5ConnTracker) Close() error {
	err := l.Listener.Close()
	l.conns.Range(func(key, value any) bool {
		if conn, ok := value.(net.Conn); ok {
			_ = conn.Close()
		}
		return true
	})
	return err
}

type socks5TrackedConn struct {
	net.Conn
	tracker *socks5ConnTracker
}

func (c *socks5TrackedConn) Close() error {
	c.tracker.unregister(c.RemoteAddr().String())
	return c.Conn.Close()
}

func (p *ProxyServer) startSocks5() {
	p.socks5Server = p.newSocks5Server()
	socks5Ln, err := net.Listen("tcp", p.socks5Addr)
	if err != nil {
		log.Printf("[Proxy] Failed to listen SOCKS5 on %s: %v", p.socks5Addr, err)
		return
	}
	p.socks5Tracker = &socks5ConnTracker{Listener: socks5Ln}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[Proxy] panic in SOCKS5 server: %v", r)
			}
		}()
		log.Printf("[Proxy] SOCKS5 server started on %s", p.socks5Addr)
		if err := p.socks5Server.Serve(p.socks5Tracker); err != nil {
			log.Printf("[Proxy] SOCKS5 server error: %v", err)
		}
	}()
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func normalizeECHProfile(p *ECHProfile) {
	if p == nil {
		return
	}
	p.ID = strings.TrimSpace(p.ID)
	p.Name = strings.TrimSpace(p.Name)
	p.Config = strings.TrimSpace(p.Config)
	p.DiscoveryDomain = strings.TrimSpace(p.DiscoveryDomain)
	p.DoHUpstream = strings.TrimSpace(p.DoHUpstream)
}

func cleanWebsiteToken(token string) string {
	token = normalizeHost(token)
	token = strings.TrimPrefix(token, "*.")
	token = strings.TrimSuffix(token, "$")
	token = strings.Trim(token, "[]")
	if i := strings.Index(token, ":"); i >= 0 {
		token = token[:i]
	}
	return token
}

func tokenMatchesDomain(token, domain string) bool {
	token = cleanWebsiteToken(token)
	domain = cleanWebsiteToken(domain)
	if token == "" || domain == "" {
		return false
	}
	return token == domain || strings.HasSuffix(token, "."+domain)
}

func inferWebsiteFromSiteGroup(sg SiteGroup) string {
	tokens := []string{sg.Name, sg.Upstream, sg.SniFake}
	tokens = append(tokens, sg.Domains...)

	hasDomain := func(domains ...string) bool {
		for _, t := range tokens {
			for _, d := range domains {
				if tokenMatchesDomain(t, d) {
					return true
				}
			}
		}
		return false
	}

	switch {
	case hasDomain("google.com", "youtube.com", "gstatic.com", "googlevideo.com", "gvt1.com", "ytimg.com", "youtu.be", "ggpht.com"):
		return "google"
	case hasDomain("github.com", "githubusercontent.com", "githubassets.com", "github.io"):
		return "github"
	case hasDomain("telegram.org", "web.telegram.org", "cdn-telegram.org", "t.me", "telesco.pe", "tg.dev", "telegram.me"):
		return "telegram"
	case hasDomain("proton.me"):
		return "proton"
	case hasDomain("pixiv.net", "fanbox.cc", "pximg.net", "pixiv.org"):
		return "pixiv"
	case hasDomain("nyaa.si"):
		return "nyaa"
	case hasDomain("wikipedia.org", "wikimedia.org", "mediawiki.org", "wikibooks.org", "wikidata.org", "wikifunctions.org", "wikinews.org", "wikiquote.org", "wikisource.org", "wikiversity.org", "wikivoyage.org", "wiktionary.org"):
		return "wikipedia"
	case hasDomain("e-hentai.org", "exhentai.org", "ehgt.org", "hentaiverse.org", "ehwiki.org", "ehtracker.org"):
		return "ehentai"
	case hasDomain("facebook.com", "fbcdn.net", "instagram.com", "cdninstagram.com", "instagr.am", "ig.me", "whatsapp.com", "whatsapp.net"):
		return "meta"
	case hasDomain("twitter.com", "x.com", "t.co", "twimg.com"):
		return "x"
	case hasDomain("steamcommunity.com", "steampowered.com"):
		return "steam"
	case hasDomain("mega.nz", "mega.io", "mega.co.nz"):
		return "mega"
	case hasDomain("dailymotion.com"):
		return "dailymotion"
	case hasDomain("duckduckgo.com"):
		return "duckduckgo"
	case hasDomain("reddit.com", "redd.it", "redditmedia.com", "redditstatic.com"):
		return "reddit"
	case hasDomain("twitch.tv"):
		return "twitch"
	case hasDomain("bbc.com", "bbc.co.uk", "bbci.co.uk"):
		return "bbc"
	}

	for _, d := range sg.Domains {
		d = cleanWebsiteToken(d)
		if d == "" || d == "off" {
			continue
		}
		parts := strings.Split(d, ".")
		if len(parts) >= 2 {
			return parts[len(parts)-2]
		}
		return d
	}

	for _, t := range tokens {
		t = cleanWebsiteToken(t)
		if t == "" || t == "off" {
			continue
		}
		parts := strings.Split(t, ".")
		if len(parts) >= 2 {
			return parts[len(parts)-2]
		}
		return t
	}
	return "misc"
}

func resolveUpstreamHost(targetHost, upstream string) string {
	upstream = strings.TrimSpace(upstream)
	if upstream == "" {
		return ""
	}
	if strings.Contains(upstream, "$1") {
		firstLabel := targetHost
		if i := strings.Index(firstLabel, "."); i > 0 {
			firstLabel = firstLabel[:i]
		}
		upstream = strings.ReplaceAll(upstream, "$1", firstLabel)
	}
	return upstream
}

func resolveRuleUpstream(targetHost string, rule Rule) string {
	resolved := resolveUpstreamHost(targetHost, rule.Upstream)
	trimmed := strings.TrimSpace(resolved)
	if trimmed == "" && len(rule.Upstreams) > 0 {
		return strings.Join(rule.Upstreams, ",")
	}

	low := strings.ToLower(trimmed)
	if strings.HasPrefix(low, "$backend_ip") || strings.HasPrefix(low, "$upstream_host") || strings.HasPrefix(trimmed, "$") {
		if len(rule.Upstreams) > 0 {
			return strings.Join(rule.Upstreams, ",")
		}
		return net.JoinHostPort(targetHost, "443")
	}

	return resolved
}

func splitUpstreamCandidates(targetHost, upstream, defaultPort string) []string {
	resolved := resolveUpstreamHost(targetHost, upstream)
	if resolved == "" {
		return nil
	}
	// Support Chinese commas, semicolons, and spaces as delimiters
	resolved = strings.ReplaceAll(resolved, "，", ",")
	resolved = strings.ReplaceAll(resolved, ";", ",")
	resolved = strings.ReplaceAll(resolved, " ", ",")
	parts := strings.Split(resolved, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		addr := ensureAddrWithPort(p, defaultPort)
		if addr == "" {
			continue
		}
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		out = append(out, addr)
	}
	return out
}

func firstUpstreamHost(targetHost, upstream string) string {
	candidates := splitUpstreamCandidates(targetHost, upstream, "443")
	if len(candidates) == 0 {
		return ""
	}
	host, _, err := net.SplitHostPort(candidates[0])
	if err != nil {
		return normalizeHost(candidates[0])
	}
	return normalizeHost(host)
}

func hostMatchesDomain(host, domain string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	domain = strings.ToLower(strings.TrimSpace(domain))
	if host == "" || domain == "" {
		return false
	}
	domain = strings.TrimPrefix(domain, "*.")
	domain = strings.TrimSuffix(domain, "$")

	if strings.HasSuffix(domain, ".*") {
		base := strings.TrimSuffix(domain, ".*")
		if base == "" {
			return false
		}
		hostParts := strings.Split(host, ".")
		baseParts := strings.Split(base, ".")
		if len(hostParts) < len(baseParts)+1 {
			return false
		}
		for i := 0; i+len(baseParts) < len(hostParts); i++ {
			ok := true
			for j := 0; j < len(baseParts); j++ {
				if hostParts[i+j] != baseParts[j] {
					ok = false
					break
				}
			}
			if ok {
				return true
			}
		}
		return false
	}

	if host == domain {
		return true
	}
	return strings.HasSuffix(host, "."+domain)
}

func domainMatchScore(host, domain string) int {
	host = strings.ToLower(strings.TrimSpace(host))
	domain = strings.ToLower(strings.TrimSpace(domain))
	if host == "" || domain == "" {
		return -1
	}

	if strings.HasPrefix(domain, "~") {
		pattern := strings.TrimSpace(strings.TrimPrefix(domain, "~"))
		if pattern == "" {
			return -1
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return -1
		}
		if re.MatchString(host) {
			return 900 + len(pattern) // exact(1000+) > regex(900+) > suffix/exact-domain
		}
		return -1
	}

	domain = strings.TrimPrefix(domain, "*.")
	domain = strings.TrimSuffix(domain, "$")

	if strings.HasSuffix(domain, ".*") {
		base := strings.TrimSuffix(domain, ".*")
		if base == "" {
			return -1
		}
		hostParts := strings.Split(host, ".")
		baseParts := strings.Split(base, ".")
		if len(hostParts) < len(baseParts)+1 {
			return -1
		}
		for i := 0; i+len(baseParts) < len(hostParts); i++ {
			ok := true
			for j := 0; j < len(baseParts); j++ {
				if hostParts[i+j] != baseParts[j] {
					ok = false
					break
				}
			}
			if ok {
				return len(base)
			}
		}
		return -1
	}

	if host == domain {
		return len(domain) + 1000 // Prefer exact match over suffix match.
	}
	if strings.HasSuffix(host, "."+domain) {
		return len(domain)
	}
	return -1
}

func normalizeTUNConfig(cfg TUNConfig) TUNConfig {
	if cfg.MTU <= 0 {
		cfg.MTU = 9000
	}
	cfg.StrictRoute = false
	return cfg
}

func defaultDNSNodes() []DNSNode {
	return []DNSNode{}
}

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

func (p *ProxyServer) FetchECH(ctx context.Context, domain string, dohURL string) ([]byte, error) {
	if dohURL != "" {
		return fetchECHDirect(ctx, domain, dohURL)
	}
	if p.dohResolver == nil {
		return nil, fmt.Errorf("no DoH resolver available")
	}
	return p.dohResolver.ResolveECH(ctx, domain)
}

// fetchECHDirect sends a plain HTTPS DoH TypeHTTPS query to the given URL,
// bypassing configured DNS nodes. Used by ECH profile probing.
func fetchECHDirect(ctx context.Context, domain, dohURL string) ([]byte, error) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeHTTPS)
	buf, err := msg.Pack()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", dohURL, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.ContentLength = int64(len(buf))
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 SNIShaper/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DoH server returned status %d", resp.StatusCode)
	}

	respBuf, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, err
	}

	resMsg := new(dns.Msg)
	if err := resMsg.Unpack(respBuf); err != nil {
		return nil, err
	}

	for _, ans := range resMsg.Answer {
		if https, ok := ans.(*dns.HTTPS); ok {
			for _, opt := range https.Value {
				if ech, ok := opt.(*dns.SVCBECHConfig); ok {
					return ech.ECH, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("no ECH config found for %s", domain)
}
