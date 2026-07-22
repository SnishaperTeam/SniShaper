package cfpool

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type IPStats struct {
	IP            string    `json:"ip"`
	Latency       string    `json:"latency"`
	Failures      int       `json:"failures"`
	LastCheck     string    `json:"last_check"`
	LatencyVal    time.Duration
	LastCheckTime time.Time
}

type CloudflarePool struct {
	allIPs    map[string]*IPStats // Keep track of stats for all IPs
	activeIPs []*IPStats          // IPs that are considered healthy, sorted by latency
	mu        sync.RWMutex
	stopChan  chan struct{}
	running   bool
	wg        sync.WaitGroup // Track goroutine lifecycle

	// 最快 IP 缓存及 5分钟的过期时间
	bestIP       string
	bestIPExpire time.Time

	// IP 拉取新鲜度：记录上次成功拉取时间
	lastFetchTime time.Time
	nat64Prefix   string
}

func (p *CloudflarePool) SetNAT64Prefix(prefix string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nat64Prefix = prefix
}

func (p *CloudflarePool) getNAT64Prefix() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.nat64Prefix
}

func NewCloudflarePool(ips []string) *CloudflarePool {
	p := &CloudflarePool{
		allIPs:    make(map[string]*IPStats),
		activeIPs: make([]*IPStats, 0),
		stopChan:  make(chan struct{}),
	}
	p.UpdateIPs(ips)
	p.lastFetchTime = time.Now()
	return p
}

// SetLastFetchTime updates the last fetch timestamp after a successful API call.
func (p *CloudflarePool) SetLastFetchTime(t time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastFetchTime = t
}

// IsStale returns true if the IP pool was last fetched more than maxAge ago.
func (p *CloudflarePool) IsStale(maxAge time.Duration) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.lastFetchTime.IsZero() {
		return true
	}
	return time.Since(p.lastFetchTime) > maxAge
}

// HasIPs returns true if the pool has any IPs at all.
func (p *CloudflarePool) HasIPs() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.allIPs) > 0
}

func (p *CloudflarePool) Start() {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.stopChan = make(chan struct{})
	p.mu.Unlock()

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[CFPool] panic in health check: %v\n", r)
			}
		}()
		p.healthCheckLoop()
	}()
}

func (p *CloudflarePool) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	close(p.stopChan)
	p.mu.Unlock()

	p.wg.Wait() // Wait for goroutine to exit
}

func (p *CloudflarePool) UpdateIPs(ips []string) {
	p.mu.Lock()

	newMap := make(map[string]*IPStats)
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}

		if existing, ok := p.allIPs[ip]; ok {
			newMap[ip] = existing
		} else {
			newMap[ip] = &IPStats{IP: ip}
		}
	}
	p.allIPs = newMap

	// Re-filter activeIPs to remove deleted ones
	p.activeIPs = make([]*IPStats, 0)
	for _, stats := range p.allIPs {
		if stats.LatencyVal > 0 && stats.Failures < 3 {
			p.activeIPs = append(p.activeIPs, stats)
		}
	}
	sort.Slice(p.activeIPs, func(i, j int) bool {
		return p.activeIPs[i].LatencyVal < p.activeIPs[j].LatencyVal
	})
	p.mu.Unlock()

	// Trigger check when IP list is updated
	go p.checkAllIPs()
}

// GetTopIPs returns up to n best IPs.
func (p *CloudflarePool) GetTopIPs(n int) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	count := len(p.activeIPs)
	// Fallback to all IPs if no active ones
	if count == 0 {
		res := make([]string, 0, n)
		i := 0
		for ip := range p.allIPs {
			res = append(res, ip)
			i++
			if i >= n {
				break
			}
		}
		return res
	}

	if n > count {
		n = count
	}

	res := make([]string, n)
	for i := 0; i < n; i++ {
		res[i] = p.activeIPs[i].IP
	}
	return res
}

func (p *CloudflarePool) GetAllIPsWithStats() []*IPStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make([]*IPStats, 0, len(p.allIPs))
	for _, s := range p.allIPs {
		stats = append(stats, s)
	}

	sort.Slice(stats, func(i, j int) bool {
		// Put 0 latency (unchecked/failed) at end
		if stats[i].LatencyVal == 0 {
			return false
		}
		if stats[j].LatencyVal == 0 {
			return true
		}
		return stats[i].LatencyVal < stats[j].LatencyVal
	})

	return stats
}

func (p *CloudflarePool) healthCheckLoop() {
	// Initial check on startup
	go p.checkAllIPs()

	// Wait for stop signal
	<-p.stopChan
}

func (p *CloudflarePool) TriggerHealthCheck() {
	go p.checkAllIPs()
}

func (p *CloudflarePool) RemoveInvalidIPs() int {
	p.mu.Lock()
	count := 0
	for ip, stats := range p.allIPs {
		if stats.Failures >= 3 {
			delete(p.allIPs, ip)
			count++
		}
	}
	p.mu.Unlock()

	if count > 0 {
		p.rebuildActiveIPs()
	}
	return count
}

func (p *CloudflarePool) ReportFailure(ip string) {
	p.mu.Lock()
	if stats, ok := p.allIPs[ip]; ok {
		stats.Failures++
		stats.LatencyVal += 1000 * time.Millisecond // Penalize latency
		stats.Latency = stats.LatencyVal.String()

		// Trigger incremental check when failures reach threshold
		if stats.Failures >= 2 {
			go p.checkIncremental()
		}
	}
	// 清空最快 IP 缓存，迫使下一次连接重新竞速
	p.bestIP = ""
	p.bestIPExpire = time.Time{}
	p.mu.Unlock()
	p.rebuildActiveIPs()
}

func (p *CloudflarePool) ReportSuccess(ip string) {
	p.mu.Lock()
	if stats, ok := p.allIPs[ip]; ok {
		if stats.Failures > 0 {
			stats.Failures--
		}
	}
	p.mu.Unlock()
}

func (p *CloudflarePool) checkIncremental() {
	p.mu.RLock()
	ipsToCheck := make([]string, 0)
	now := time.Now()

	for ip, stats := range p.allIPs {
		if stats.Failures >= 2 ||
			now.Sub(stats.LastCheckTime) > 30*time.Minute {
			ipsToCheck = append(ipsToCheck, ip)
		}
	}
	p.mu.RUnlock()

	if len(ipsToCheck) == 0 {
		return
	}

	p.checkIPs(ipsToCheck)
}

func (p *CloudflarePool) checkIPs(ipsToCheck []string) {
	if len(ipsToCheck) == 0 {
		return
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // Concurrency limit

	for _, ip := range ipsToCheck {
		wg.Add(1)
		sem <- struct{}{}
		go func(targetIP string) {
			defer wg.Done()
			defer func() { <-sem }()

			latency, err := p.testIP(targetIP)

			p.mu.Lock()
			stats, ok := p.allIPs[targetIP]
			if ok {
				now := time.Now()
				stats.LastCheck = now.Format(time.RFC3339)
				stats.LastCheckTime = now
				if err != nil {
					stats.Failures++
					stats.LatencyVal = 0 // Invalid
					stats.Latency = ""
				} else {
					stats.Failures = 0
					stats.LatencyVal = latency
					stats.Latency = latency.String()
				}
			}
			p.mu.Unlock()
		}(ip)
	}
	wg.Wait()

	p.rebuildActiveIPs()
}

func (p *CloudflarePool) checkAllIPs() {
	p.mu.RLock()
	ipsToCheck := make([]string, 0, len(p.allIPs))
	for ip := range p.allIPs {
		ipsToCheck = append(ipsToCheck, ip)
	}
	p.mu.RUnlock()

	p.checkIPs(ipsToCheck)
}

func (p *CloudflarePool) rebuildActiveIPs() {
	p.mu.Lock()
	defer p.mu.Unlock()

	newActive := make([]*IPStats, 0)
	for _, stats := range p.allIPs {
		if stats.LatencyVal > 0 && stats.Failures < 3 {
			newActive = append(newActive, stats)
		}
	}

	sort.Slice(newActive, func(i, j int) bool {
		return newActive[i].LatencyVal < newActive[j].LatencyVal
	})

	p.activeIPs = newActive
}

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

func (p *CloudflarePool) testIP(ip string) (time.Duration, error) {
	prefix := p.getNAT64Prefix()
	mappedIP, ok := mapNAT64Addr(ip, prefix)
	if !ok {
		return 0, fmt.Errorf("native IPv6 %s excluded under NAT64 mode", ip)
	}

	dialer := &net.Dialer{Timeout: 3 * time.Second}
	start := time.Now()

	conn, err := dialer.Dial("tcp", net.JoinHostPort(mappedIP, "443"))
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	return time.Since(start), nil
}

// DialParallel 并发建连竞速并缓存最快 IP
func (p *CloudflarePool) DialParallel(ctx context.Context, network string, port string, nat64Prefix string) (net.Conn, string, error) {
	if port == "" {
		port = "443"
	}

	// 优先回退到全局前缀
	if nat64Prefix == "" {
		nat64Prefix = p.getNAT64Prefix()
	}

	p.mu.Lock()
	// 1. 如果缓存的 bestIP 未过期，优先进行单点连接
	if p.bestIP != "" && time.Now().Before(p.bestIPExpire) {
		best := p.bestIP
		p.mu.Unlock()

		mappedBest, ok := mapNAT64Addr(best, nat64Prefix)
		if ok {
			dialer := &net.Dialer{Timeout: 2 * time.Second}
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(mappedBest, port))
			if err == nil {
				return conn, net.JoinHostPort(mappedBest, port), nil
			}
		}

		// 失败了，清空 bestIP，准备开始竞速
		p.mu.Lock()
		p.bestIP = ""
		p.bestIPExpire = time.Time{}
	}

	// 2. 收集候选 IP 列表
	var activeIPs []string
	for _, ip := range p.activeIPs {
		activeIPs = append(activeIPs, ip.IP)
	}
	if len(activeIPs) == 0 {
		for ip := range p.allIPs {
			activeIPs = append(activeIPs, ip)
		}
	}
	p.mu.Unlock()

	// 过滤与映射候选 IP
	type mappedCandidate struct {
		rawIP    string
		mappedIP string
	}
	candidates := make([]mappedCandidate, 0, len(activeIPs))
	for _, ip := range activeIPs {
		mapped, ok := mapNAT64Addr(ip, nat64Prefix)
		if ok {
			candidates = append(candidates, mappedCandidate{rawIP: ip, mappedIP: mapped})
		}
	}

	if len(candidates) == 0 {
		return nil, "", errors.New("no valid Cloudflare IPs available in pool under NAT64 mode")
	}

	// 3. 开始并发拨号竞速 (Happy Eyeballs)
	type dialResult struct {
		conn     net.Conn
		mappedIP string
		rawIP    string
		err      error
	}

	resChan := make(chan dialResult, len(candidates))
	raceCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, c := range candidates {
		go func(item mappedCandidate) {
			dialer := &net.Dialer{Timeout: 3 * time.Second}
			conn, err := dialer.DialContext(raceCtx, network, net.JoinHostPort(item.mappedIP, port))
			select {
			case resChan <- dialResult{conn: conn, mappedIP: item.mappedIP, rawIP: item.rawIP, err: err}:
			case <-raceCtx.Done():
				if conn != nil {
					conn.Close()
				}
			}
		}(c)
	}

	var lastErr error
	success := false
	var winningConn net.Conn
	var winningAddr string
	var winningRawIP string

	for i := 0; i < len(candidates); i++ {
		select {
		case res := <-resChan:
			if res.err == nil && res.conn != nil {
				if !success {
					success = true
					winningConn = res.conn
					winningAddr = net.JoinHostPort(res.mappedIP, port)
					winningRawIP = res.rawIP
					cancel() // 取消其他竞速协程的拨号
				} else {
					res.conn.Close()
				}
			} else {
				if res.err != nil {
					lastErr = res.err
				}
			}
		case <-ctx.Done():
			return nil, "", ctx.Err()
		}
	}

	if success {
		// 4. 更新 bestIP 缓存
		p.mu.Lock()
		p.bestIP = winningRawIP
		p.bestIPExpire = time.Now().Add(5 * time.Minute)
		p.mu.Unlock()

		return winningConn, winningAddr, nil
	}

	if lastErr != nil {
		return nil, "", lastErr
	}
	return nil, "", errors.New("all Cloudflare pool IPs failed to connect under NAT64 mode")
}

type apiIPInfo struct {
	IP string `json:"ip"`
}

type cfApiResponse struct {
	Status bool                   `json:"status"`
	Code   int                    `json:"code"`
	Msg    string                 `json:"msg"`
	Info   map[string][]apiIPInfo `json:"info"`
}

func FetchCloudflareIPs(apiKey string) ([]string, error) {
	if apiKey == "" {
		apiKey = "o1zrmHAF"
	}
	url := fmt.Sprintf("https://www.wetest.vip/api/cf2dns/get_cloudflare_ip?key=%s&type=v4", apiKey)

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiRes cfApiResponse
	if err := json.Unmarshal(body, &apiRes); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if !apiRes.Status || apiRes.Code != 200 {
		return nil, fmt.Errorf("API error: %s (code %d)", apiRes.Msg, apiRes.Code)
	}

	var ips []string
	for _, list := range apiRes.Info {
		for _, item := range list {
			if item.IP != "" {
				ips = append(ips, item.IP)
			}
		}
	}

	return ips, nil
}
