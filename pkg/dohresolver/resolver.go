package dohresolver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/miekg/dns"
	utls "github.com/refraction-networking/utls"
)

type CertVerifyConfig struct {
	Mode                  string   `json:"mode,omitempty"`
	Names                 []string `json:"names,omitempty"`
	Suffixes              []string `json:"suffixes,omitempty"`
	SPKISHA256            []string `json:"spki_sha256,omitempty"`
	AllowUnknownAuthority bool     `json:"allow_unknown_authority,omitempty"`
}

type DNSNode struct {
	Name          string           `json:"name"`
	URL           string           `json:"url"`
	SNI           string           `json:"sni"`
	ECHEnabled    bool             `json:"ech_enabled"`
	ECHProfileID  string           `json:"ech_profile_id"`
	QUIC          bool             `json:"quic"`
	IPs           []string         `json:"ips"`
	CertVerify    CertVerifyConfig `json:"cert_verify"`
	ECHAutoUpdate bool             `json:"ech_auto_update"`
	Enabled       bool             `json:"enabled"`
}

type Rule struct {
	SniFake       string
	ECHEnabled    bool
	ECHProfileID  string
	CertVerify    CertVerifyConfig
	ECHAutoUpdate bool
}

// ProxyServer 抽象接口，表达 DoH 模块对代理底层网络传输和安全状态的依赖
type ProxyServer interface {
	DialWithRule(ctx context.Context, network, addr string, rule Rule) (net.Conn, error)
	GetUConn(conn net.Conn, sni, verifyName string, rule Rule, allowUnknownAuthority bool, alpn string, ech []byte) *utls.UConn
	ResolveRuleECHConfig(host string, rule Rule) []byte
	UpdateECHProfileConfig(profileID string, configBytes []byte)
}

type dnsCacheEntry struct {
	ips       []string
	expiresAt time.Time
	staleAt   time.Time
}

type FailoverResolver struct {
	proxy    ProxyServer
	getNodes func() []DNSNode
	dnsCache sync.Map // domain -> *dnsCacheEntry
	echCache sync.Map // domain -> []byte (ECH config hot-patched by server retry)
}

func NewFailoverResolver(proxy ProxyServer, getNodes func() []DNSNode) *FailoverResolver {
	return &FailoverResolver{
		proxy:    proxy,
		getNodes: getNodes,
	}
}

func (r *FailoverResolver) getNodeClient(ctx context.Context, node DNSNode) (*http.Client, error) {
	parsedURL, err := url.Parse(node.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid node URL: %w", err)
	}
	host := parsedURL.Hostname()

	rule := Rule{
		SniFake:       node.SNI,
		ECHEnabled:    node.ECHEnabled,
		ECHProfileID:  node.ECHProfileID,
		CertVerify:    node.CertVerify,
		ECHAutoUpdate: node.ECHAutoUpdate,
	}

	tr := &http.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// 收集拨号候选 IP
			var dialCandidates []string
			if len(node.IPs) > 0 {
				port := parsedURL.Port()
				if port == "" {
					port = "443"
				}
				for _, ip := range node.IPs {
					dialCandidates = append(dialCandidates, net.JoinHostPort(ip, port))
				}
			} else {
				dialCandidates = []string{net.JoinHostPort(host, "443")}
			}

			var dialConn net.Conn
			var dialErr error
			for _, cand := range dialCandidates {
				dialConn, dialErr = r.proxy.DialWithRule(ctx, network, cand, rule)
				if dialErr == nil && dialConn != nil {
					break
				}
			}
			if dialErr != nil || dialConn == nil {
				return nil, dialErr
			}

			echBytes := r.proxy.ResolveRuleECHConfig(host, rule)
			verifyName := host
			if rule.SniFake != "" {
				verifyName = rule.SniFake
			}

			// 通过代理系统的 GetUConn 进行 uTLS 混淆客户端指纹和 ECH 注入
			uconn := r.proxy.GetUConn(
				dialConn,
				rule.SniFake,
				verifyName,
				rule,
				node.CertVerify.AllowUnknownAuthority,
				"http/1.1",
				echBytes,
			)

			if err := uconn.HandshakeContext(ctx); err != nil {
				dialConn.Close()
				return nil, err
			}
			return uconn, nil
		},
		ResponseHeaderTimeout: 5 * time.Second,
	}

	return &http.Client{Transport: tr, Timeout: 10 * time.Second}, nil
}

func (r *FailoverResolver) exchangeNode(ctx context.Context, node *DNSNode, msg *dns.Msg) (*dns.Msg, error) {
	return r.exchangeNodeWithRetry(ctx, node, msg, true)
}

func (r *FailoverResolver) exchangeNodeWithRetry(ctx context.Context, node *DNSNode, msg *dns.Msg, allowRetry bool) (*dns.Msg, error) {
	client, err := r.getNodeClient(ctx, *node)
	if err != nil {
		return nil, err
	}

	msg = msg.Copy()
	msg.Id = 0

	buf, err := msg.Pack()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", node.URL, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	if node.SNI != "" {
		u, _ := url.Parse(node.URL)
		if u != nil && isLiteralIP(u.Hostname()) {
			req.Host = node.SNI
		}
	}

	req.ContentLength = int64(len(buf))
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 SNIShaper/1.0")

	resp, err := client.Do(req)
	if err != nil {
		// 阻断自愈：ECH 被拒时优先使用服务端 RetryConfigList，其次安全 DNS 刷新后重试
		if allowRetry && node.ECHEnabled && node.ECHAutoUpdate {
			parsedURL, _ := url.Parse(node.URL)
			if parsedURL != nil {
				host := parsedURL.Hostname()
				var echErr *utls.ECHRejectionError
				if errors.As(err, &echErr) {
					log.Printf("[DOH] ECH rejected for %s (retryConfigs=%d), recovering...", host, len(echErr.RetryConfigList))
					if recovered := r.recoverNodeECH(ctx, node, host, echErr); recovered {
						return r.exchangeNodeWithRetry(ctx, node, msg, false)
					}
					log.Printf("[DOH] ECH recovery failed for %s", host)
				} else {
					// 非明确 ECH 拒绝时仍尝试 DNS 刷新（兼容被包装的错误）
					log.Printf("[DOH] Handshake failed for %s (%v), attempting ECH DNS refresh...", host, err)
					if newECH, refreshErr := r.ResolveECHSafe(ctx, host); refreshErr == nil && len(newECH) > 0 {
						log.Printf("[DOH] Successfully refreshed ECH for %s (%d bytes). Syncing and retrying...", host, len(newECH))
						if node.ECHProfileID != "" {
							r.proxy.UpdateECHProfileConfig(node.ECHProfileID, newECH)
						}
						r.echCache.Store(host, append([]byte(nil), newECH...))
						return r.exchangeNodeWithRetry(ctx, node, msg, false)
					}
				}
			}
		}
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

	return resMsg, nil
}

func (r *FailoverResolver) Exchange(ctx context.Context, msg *dns.Msg) (*dns.Msg, error) {
	if r.getNodes == nil {
		return nil, fmt.Errorf("getNodes callback not configured")
	}
	nodes := r.getNodes()

	var activeNodes []DNSNode
	for _, n := range nodes {
		if !n.Enabled {
			continue
		}
		activeNodes = append(activeNodes, n)
	}

	var lastErr error
	if len(activeNodes) > 0 {
		primary := &activeNodes[0]
		resp, primaryErr := r.exchangeNode(ctx, primary, msg)
		if primaryErr == nil && resp != nil {
			return resp, nil
		}
		lastErr = primaryErr

		if len(activeNodes) > 1 {
			type result struct {
				msg *dns.Msg
				err error
			}
			resChan := make(chan result, len(activeNodes)-1)
			ctxCancel, cancel := context.WithCancel(ctx)
			defer cancel()

			for i, node := range activeNodes[1:] {
				go func(idx int, n DNSNode) {
					m, e := r.exchangeNode(ctxCancel, &activeNodes[idx+1], msg)
					if e == nil && m != nil {
						resChan <- result{m, nil}
						cancel()
					} else {
						resChan <- result{nil, e}
					}
				}(i, node)
			}

			for i := 0; i < len(activeNodes)-1; i++ {
				select {
				case res := <-resChan:
					if res.err == nil {
						return res.msg, nil
					}
					lastErr = res.err
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("all DNS nodes failed to exchange: %w", lastErr)
	}
	return nil, fmt.Errorf("all DNS nodes failed to exchange (no active nodes)")
}

func (r *FailoverResolver) TestNode(ctx context.Context, node DNSNode) ([]string, error) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn("cloudflare.com"), dns.TypeA)

	resp, err := r.exchangeNode(ctx, &node, msg)
	if err != nil {
		return nil, err
	}

	var ips []string
	for _, ans := range resp.Answer {
		if a, ok := ans.(*dns.A); ok {
			ips = append(ips, a.A.String())
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no A records returned")
	}
	return ips, nil
}

func (r *FailoverResolver) ResolveIPs(ctx context.Context, domain string) ([]string, error) {
	now := time.Now()

	if val, ok := r.dnsCache.Load(domain); ok {
		entry := val.(*dnsCacheEntry)
		if now.Before(entry.expiresAt) {
			return entry.ips, nil
		}
		if now.Before(entry.staleAt) {
			go func(dom string) {
				bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, _ = r.resolveAndCache(bgCtx, dom)
			}(domain)
			return entry.ips, nil
		}
	}

	return r.resolveAndCache(ctx, domain)
}

func (r *FailoverResolver) resolveAndCache(ctx context.Context, domain string) ([]string, error) {
	ipAddrs, err := r.ResolveIPAddrs(ctx, domain)
	if err != nil {
		return nil, err
	}

	ips := make([]string, 0, len(ipAddrs))
	for _, ip := range ipAddrs {
		if ip == nil {
			continue
		}
		ips = append(ips, ip.String())
	}

	if len(ips) > 0 {
		now := time.Now()
		entry := &dnsCacheEntry{
			ips:       ips,
			expiresAt: now.Add(2 * time.Minute),
			staleAt:   now.Add(24 * time.Hour),
		}
		r.dnsCache.Store(domain, entry)
	}

	return ips, nil
}

func (r *FailoverResolver) ResolveIPAddrs(ctx context.Context, domain string) ([]net.IP, error) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeA)

	resp, err := r.Exchange(ctx, msg)
	if err != nil {
		return nil, err
	}

	var ips []net.IP
	for _, ans := range resp.Answer {
		if a, ok := ans.(*dns.A); ok {
			ips = append(ips, a.A)
		}
	}
	return ips, nil
}

func isLiteralIP(host string) bool {
	return net.ParseIP(host) != nil
}

// ResolveECH fetches the ECH config for a domain via TypeHTTPS (65)
func (r *FailoverResolver) ResolveECH(ctx context.Context, domain string) ([]byte, error) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeHTTPS)

	resp, err := r.Exchange(ctx, msg)
	if err != nil {
		return nil, err
	}

	for _, ans := range resp.Answer {
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

// ExchangeSafe issues the DNS query ONLY using nodes that don't have ECH enabled.
func (r *FailoverResolver) ExchangeSafe(ctx context.Context, msg *dns.Msg) (*dns.Msg, error) {
	if r.getNodes == nil {
		return nil, fmt.Errorf("getNodes callback not configured")
	}
	nodes := r.getNodes()

	var safeNodes []DNSNode
	for _, n := range nodes {
		if n.Enabled && !n.ECHEnabled {
			safeNodes = append(safeNodes, n)
		}
	}

	if len(safeNodes) == 0 {
		return nil, fmt.Errorf("no safe (non-ECH) DNS nodes available for fallback")
	}

	for _, node := range safeNodes {
		resp, err := r.exchangeNodeWithRetry(ctx, &node, msg, false)
		if err == nil {
			return resp, nil
		}
	}

	return nil, fmt.Errorf("all safe DNS resolution attempts failed")
}

// recoverNodeECH attempts to recover from an ECH rejection by applying retry configs
// provided by the server. Returns true if a usable ECH config was found and cached.
func (r *FailoverResolver) recoverNodeECH(ctx context.Context, node *DNSNode, host string, echErr *utls.ECHRejectionError) bool {
	rawCfg := echErr.RetryConfigList
	if len(rawCfg) == 0 {
		return false
	}
	if node.ECHProfileID != "" {
		r.proxy.UpdateECHProfileConfig(node.ECHProfileID, rawCfg)
	}
	r.echCache.Store(host, append([]byte(nil), rawCfg...))
	log.Printf("[DOH] Applied server retry ECH config for %s (%d bytes)", host, len(rawCfg))
	return true
}

// ResolveECHSafe fetches the ECH config for a domain via standard nodes (no ECH).
func (r *FailoverResolver) ResolveECHSafe(ctx context.Context, domain string) ([]byte, error) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeHTTPS)

	resp, err := r.ExchangeSafe(ctx, msg)
	if err != nil {
		return nil, err
	}

	for _, ans := range resp.Answer {
		if https, ok := ans.(*dns.HTTPS); ok {
			for _, opt := range https.Value {
				if ech, ok := opt.(*dns.SVCBECHConfig); ok {
					return ech.ECH, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("no ECH config found via safe source for %s", domain)
}
