package proxy

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"
	"unsafe"
)

// ---------------------------------------------------------------------------
// BlindSessionCache – per-host session ticket injection for migration mode
// ---------------------------------------------------------------------------

type migrationSessionCache struct {
	mu         sync.Mutex
	sessions   map[string]*tls.ClientSessionState
	activeHost string
}

func newMigrationSessionCache() *migrationSessionCache {
	return &migrationSessionCache{sessions: make(map[string]*tls.ClientSessionState)}
}

func (c *migrationSessionCache) setActiveHost(host string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activeHost = host
}

func (c *migrationSessionCache) setSession(host string, s *tls.ClientSessionState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessions[host] = s
}

func (c *migrationSessionCache) deleteSession(host string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.sessions, host)
}

func (c *migrationSessionCache) Get(sessionKey string) (*tls.ClientSessionState, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := sessionKey
	if c.activeHost != "" {
		key = c.activeHost
	}
	s, ok := c.sessions[key]
	return s, ok
}

func (c *migrationSessionCache) Put(sessionKey string, cs *tls.ClientSessionState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessions[sessionKey] = cs
}

// ---------------------------------------------------------------------------
// API response & session state reconstruction
// ---------------------------------------------------------------------------

type migrationSessionResponse struct {
	Ticket      string `json:"ticket"`
	Secret      string `json:"secret"`
	CipherSuite uint16 `json:"cipher_suite"`
	Version     uint16 `json:"version"`
	TargetIP    string `json:"target_ip"`
}

func setUnexportedField[T any](obj any, fieldName string, value T) {
	if f := reflect.ValueOf(obj).Elem().FieldByName(fieldName); f.IsValid() {
		*(*T)(unsafe.Pointer(f.UnsafeAddr())) = value
	}
}

func buildMigrationSessionState(resp migrationSessionResponse) (*tls.ClientSessionState, error) {
	ticketBytes, err := hex.DecodeString(resp.Ticket)
	if err != nil {
		return nil, fmt.Errorf("decode ticket: %w", err)
	}
	secretBytes, err := hex.DecodeString(resp.Secret)
	if err != nil {
		return nil, fmt.Errorf("decode secret: %w", err)
	}

	state := &tls.SessionState{}
	setUnexportedField(state, "secret", secretBytes)
	setUnexportedField(state, "version", resp.Version)
	setUnexportedField(state, "cipherSuite", resp.CipherSuite)
	setUnexportedField(state, "isClient", true)
	setUnexportedField(state, "createdAt", uint64(time.Now().Unix()))
	setUnexportedField(state, "extMasterSecret", true)

	// Populate PeerCertificates with a dummy self-signed cert to avoid panics.
	if dummy, err := generateMigrationDummyCert(); err == nil {
		if parsed, err := x509.ParseCertificate(dummy.Certificate[0]); err == nil {
			setUnexportedField(state, "peerCertificates", []*x509.Certificate{parsed})
		}
	}

	return tls.NewResumptionState(ticketBytes, state)
}

func generateMigrationDummyCert() (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return tls.Certificate{}, err
	}
	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{Organization: []string{"SniShaper Migration"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: priv}, nil
}

// ---------------------------------------------------------------------------
// handleMigration – main entry point for migration mode
// ---------------------------------------------------------------------------

func (p *ProxyServer) handleMigration(clientConn net.Conn, host, port string, rule Rule) {
	defer func() {
		if r := recover(); r != nil {
			p.tracef("[Migration] Panic: %v", r)
			clientConn.Close()
		}
	}()

	if !p.rules.GetMigrationEnabled() {
		p.tracef("[Migration] Service disabled")
		_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\nMigration service is disabled"))
		clientConn.Close()
		return
	}
	migrationServer := strings.TrimSpace(p.rules.GetMigrationServer())
	if migrationServer == "" {
		p.tracef("[Migration] Server URL not configured")
		_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\nMigration server URL not configured"))
		clientConn.Close()
		return
	}

	p.tracef("[Migration] Handling %s:%s (sni_fake=%q)", host, port, rule.SniFake)

	// 1. Resolve target IP locally
	resolvedIP := p.resolveMigrationIP(host, rule)
	if resolvedIP == "" {
		p.tracef("[Migration] Cannot resolve %s", host)
		_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\nDNS resolution failed"))
		clientConn.Close()
		return
	}
	p.tracef("[Migration] %s -> %s", host, resolvedIP)

	// 2. Self-healing loop (max 2 attempts)
	// Use persistent cache across requests, create if nil
	if p.migrationCache == nil {
		p.migrationCache = newMigrationSessionCache()
	}
	cache := p.migrationCache
	var upstreamConn net.Conn

	for attempt := 1; attempt <= 2; attempt++ {
		p.tracef("[Migration] Attempt %d/2", attempt)

		// 2a. Check cache first, only fetch from API if no valid ticket
		var targetIP string
		cache.setActiveHost(host)
		if cachedState, ok := cache.Get(host); ok && cachedState != nil {
			p.tracef("[Migration] Using cached ticket for %s", host)
		} else {
			// Fetch ticket from remote API
			_, fetchedIP, apiErr := p.fetchMigrationTicket(cache, host, port, migrationServer, resolvedIP)
			if apiErr != nil {
				p.tracef("[Migration] Ticket fetch failed: %v", apiErr)
				cache.setActiveHost("")
				if attempt == 2 {
					_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\nFailed to acquire session ticket"))
					clientConn.Close()
					return
				}
				continue
			}
			targetIP = fetchedIP
		}
		cache.setActiveHost("")

		dialIP := targetIP
		if dialIP == "" {
			dialIP = resolvedIP
		}

		// 2b. Dial target directly
		connectAddr := net.JoinHostPort(dialIP, port)
		rawConn, err := p.dialWithRule(context.Background(), "tcp", connectAddr, rule)
		if err != nil {
			p.tracef("[Migration] Dial %s failed: %v", connectAddr, err)
			cache.deleteSession(host)
			if attempt == 2 {
				_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\nTCP connect failed"))
				clientConn.Close()
				return
			}
			continue
		}

		// 2c. TLS handshake with session resumption
		cache.setActiveHost(host)
		tlsConfig := &tls.Config{
			ServerName:         rule.SniFake,
			MinVersion:         tls.VersionTLS12,
			MaxVersion:         tls.VersionTLS12,
			ClientSessionCache: cache,
			InsecureSkipVerify: true,
		}

		rawConn.SetDeadline(time.Now().Add(10 * time.Second))
		tlsConn := tls.Client(rawConn, tlsConfig)
		handshakeErr := tlsConn.Handshake()
		cache.setActiveHost("")

		if handshakeErr != nil {
			p.tracef("[Migration] TLS handshake failed: %v", handshakeErr)
			tlsConn.Close()
			rawConn.Close()
			cache.deleteSession(host)
			if attempt == 2 {
				_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\nTLS handshake failed"))
				clientConn.Close()
				return
			}
			continue
		}

		rawConn.SetDeadline(time.Time{})
		cs := tlsConn.ConnectionState()
		p.tracef("[Migration] Handshake OK DidResume=%v", cs.DidResume)

		// DidResume=false just means session ticket was not accepted;
		// the TLS handshake still succeeded and the connection is usable.
		// Following blindtls convention: treat any successful handshake as OK.
		upstreamConn = tlsConn
		break
	}

	if upstreamConn == nil {
		_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\nMigration failed"))
		clientConn.Close()
		return
	}

	// 3. 200 to browser
	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		clientConn.Close()
		upstreamConn.Close()
		return
	}

	// 4. MITM with browser
	if p.certGenerator != nil {
		caCert := p.certGenerator.GetCACert()
		caKey := p.certGenerator.GetCAKey()
		if caCert != nil && caKey != nil {
			mitmCert, err := p.generateCert(host, caCert, caKey)
			if err == nil {
				browserTLS := tls.Server(clientConn, &tls.Config{
					Certificates: []tls.Certificate{*mitmCert},
				})
				if err := browserTLS.Handshake(); err == nil {
					p.directTunnel(browserTLS, upstreamConn)
					return
				}
			}
		}
	}

	// Fallback: raw tunnel (browser sees upstream TLS as-is)
	p.directTunnel(clientConn, upstreamConn)
}

// resolveMigrationIP resolves the target host to an IP.
// Priority: rule upstream IP > local DNS resolution.
func (p *ProxyServer) resolveMigrationIP(host string, rule Rule) string {
	// 1. If rule has an upstream, use that IP directly
	if rule.Upstream != "" {
		upstream := strings.TrimSpace(rule.Upstream)
		if upstream != "" && strings.ToUpper(upstream) != "DIRECT" {
			// upstream may be "ip:port" or just "ip"
			ip, _, err := net.SplitHostPort(upstream)
			if err != nil {
				ip = upstream // no port, treat as raw IP/hostname
			}
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	// 2. Fallback to DNS resolution
	candidates := p.resolveDomainCandidates(context.Background(), host, "443", rule.DNSMode)
	for _, c := range candidates {
		ip, _, err := net.SplitHostPort(c)
		if err == nil && ip != "" {
			return ip
		}
	}
	// Fallback to system resolver
	ips, err := net.LookupIP(host)
	if err == nil && len(ips) > 0 {
		return ips[0].String()
	}
	return ""
}

// fetchMigrationTicket retrieves session credentials from the remote API.
func (p *ProxyServer) fetchMigrationTicket(cache *migrationSessionCache, host, port, server, resolvedIP string) (*tls.ClientSessionState, string, error) {
	target := net.JoinHostPort(host, port)
	apiURL := fmt.Sprintf("%s?target=%s&ip=%s", server, target, resolvedIP)
	p.tracef("[Migration] GET %s", apiURL)

	httpClient := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, "", fmt.Errorf("API %d: %s", resp.StatusCode, string(body))
	}

	var sessionResp migrationSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&sessionResp); err != nil {
		return nil, "", err
	}

	cs, err := buildMigrationSessionState(sessionResp)
	if err != nil {
		return nil, "", err
	}
	cache.setSession(host, cs)
	return cs, sessionResp.TargetIP, nil
}
