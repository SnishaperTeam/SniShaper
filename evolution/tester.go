package evolution

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"runtime"
	"sync"
	"time"

	"snishaper/pkg/dohresolver"
	"snishaper/proxy"
)

type Tester struct {
	mu          sync.RWMutex
	currentTask *TestTask
	tempRules   map[string]*TempRule
	allResults  []DomainTestResult
	ruleManager *proxy.RuleManager
	proxyServer *proxy.ProxyServer
	dohResolver *dohresolver.FailoverResolver
	autoRouter  *proxy.AutoRouter
	logCallback func(string)
	stopChan    chan struct{}
}

func NewTester(ruleManager *proxy.RuleManager, proxyServer *proxy.ProxyServer, dohResolver *dohresolver.FailoverResolver, autoRouter *proxy.AutoRouter, logCallback func(string)) *Tester {
	return &Tester{
		ruleManager: ruleManager,
		proxyServer: proxyServer,
		dohResolver: dohResolver,
		autoRouter:  autoRouter,
		logCallback: logCallback,
		tempRules:   make(map[string]*TempRule),
		allResults:  make([]DomainTestResult, 0),
		stopChan:    make(chan struct{}),
	}
}

func (t *Tester) GetTempRule(ruleID string) *TempRule {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.tempRules[ruleID]
}

func (t *Tester) GetTempRules() []*TempRule {
	t.mu.RLock()
	defer t.mu.RUnlock()
	rules := make([]*TempRule, 0, len(t.tempRules))
	for _, rule := range t.tempRules {
		rules = append(rules, rule)
	}
	return rules
}

func (t *Tester) GetAllResults() []DomainTestResult {
	t.mu.RLock()
	defer t.mu.RUnlock()
	results := make([]DomainTestResult, len(t.allResults))
	copy(results, t.allResults)
	return results
}

func (t *Tester) MarkRuleApplied(ruleID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.tempRules[ruleID]; ok {
		delete(t.tempRules, ruleID)
		return true
	}
	return false
}

func (t *Tester) StartTest(domains []string, config TestConfig) (*TestTask, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.currentTask != nil && t.currentTask.Status == StatusRunning {
		return nil, fmt.Errorf("测试任务正在运行中")
	}

	// Clear historical data to prevent memory leak
	t.allResults = t.allResults[:0]
	t.tempRules = make(map[string]*TempRule)

	task := &TestTask{
		ID:        fmt.Sprintf("task-%d", time.Now().Unix()),
		Domains:   domains,
		Status:    StatusRunning,
		Config:    config,
		Results:   make([]DomainTestResult, 0),
		Progress:  0,
		Total:     len(domains),
		StartTime: time.Now(),
	}

	t.currentTask = task
	t.stopChan = make(chan struct{})

	go t.runTest(task)

	return task, nil
}

func (t *Tester) StopTest() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.currentTask != nil && t.currentTask.Status == StatusRunning {
		close(t.stopChan)
		t.currentTask.Status = StatusFailed
		t.currentTask.EndTime = time.Now()
	}
}

func (t *Tester) GetTask() *TestTask {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.currentTask
}

func (t *Tester) runTest(task *TestTask) {
	t.log("[TASK] 开始进化模式测试任务, 域名数量: %d, 并发数: %d", task.Total, task.Config.Concurrency)

	concurrency := task.Config.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, domain := range task.Domains {
		select {
		case <-t.stopChan:
			t.log("[TASK] 测试任务已停止")
			return
		default:
		}

		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := t.testDomain(d, task.Config)

			t.mu.Lock()
			task.Results = append(task.Results, result)
			t.allResults = append(t.allResults, result)
			task.Progress++
			if result.GeneratedRule != nil {
				t.tempRules[result.GeneratedRule.ID] = result.GeneratedRule
			}
			t.mu.Unlock()
		}(domain)
	}

	wg.Wait()

	t.mu.Lock()
	task.Status = StatusCompleted
	task.EndTime = time.Now()
	t.mu.Unlock()

	t.log("[TASK] 测试任务完成, 成功: %d, 失败: %d",
		countByStatus(task.Results, true),
		countByStatus(task.Results, false))
}

func (t *Tester) testDomain(domain string, config TestConfig) DomainTestResult {
	t.log("[DOMAIN] ========== 开始测试域名: %s ==========", domain)

	result := DomainTestResult{
		Domain:      domain,
		Timestamp:   time.Now(),
		StepResults: make([]StepResult, 0),
	}

	step0Result := t.testDirect(domain, config)
	result.StepResults = append(result.StepResults, step0Result)

	if step0Result.Success {
		result.Reachable = true
		result.Method = MethodDirect
		result.Delay = step0Result.Delay
		t.log("[DOMAIN] 域名 %s 测试完成: 可达=true, 方法=direct", domain)
		return result
	}

	step1Result, resolvedIPs, bestIP := t.testTCPing(domain, config)
	result.StepResults = append(result.StepResults, step1Result)
	result.ResolvedIPs = resolvedIPs
	result.BestIP = bestIP

	if !step1Result.Success {
		result.Reachable = false
		result.Error = "TCPing失败，所有IP不可达"
		t.log("[DOMAIN] 域名 %s 测试完成: 可达=false, 原因=TCPing失败", domain)
		return result
	}

	isCF := t.autoRouter.IsCloudflare(domain)
	result.IsCloudflare = isCF

	if isCF {
		t.log("[DOMAIN] 检测到域名 %s 为 Cloudflare 域名，开启 Cloudflare 专用测试流程 (ECH -> QUIC -> TLS分片)", domain)

		// 1. ECH 测试
		step5Result := t.testECH(domain, bestIP, config)
		result.StepResults = append(result.StepResults, step5Result)
		if step5Result.Success {
			result.Reachable = true
			result.Method = MethodECH
			result.Delay = step5Result.Delay
			result.GeneratedRule = GenerateRule(domain, MethodECH, "", true)
			result.GeneratedRule.UseCFPool = true
			t.log("[DOMAIN] 域名 %s 测试完成: 可达=true, 方法=ech (已开启cfpool)", domain)
			return result
		}

		// 2. QUIC 测试
		step6Result := t.testQUIC(domain, bestIP, config)
		result.StepResults = append(result.StepResults, step6Result)
		if step6Result.Success {
			result.Reachable = true
			result.Method = MethodQUIC
			result.Delay = step6Result.Delay
			result.GeneratedRule = GenerateRule(domain, MethodQUIC, "", false)
			result.GeneratedRule.UseCFPool = true
			t.log("[DOMAIN] 域名 %s 测试完成: 可达=true, 方法=quic (已开启cfpool)", domain)
			return result
		}

		// 3. TLS 分片测试
		step3Result := t.testTLSFragment(domain, bestIP, config)
		result.StepResults = append(result.StepResults, step3Result)
		if step3Result.Success {
			result.Reachable = true
			result.Method = MethodTLSFragment
			result.Delay = step3Result.Delay
			result.GeneratedRule = GenerateRule(domain, MethodTLSFragment, "", false)
			result.GeneratedRule.UseCFPool = true
			t.log("[DOMAIN] 域名 %s 测试完成: 可达=true, 方法=tls_fragment (已开启cfpool)", domain)
			return result
		}

	} else {
		t.log("[DOMAIN] 域名 %s 为常规域名，进行常规测试流程 (域前置 -> TLS分片 -> QUIC)", domain)

		// 1. 域前置测试
		step2Result, sni := t.testDomainFronting(domain, bestIP, config)
		result.StepResults = append(result.StepResults, step2Result)
		if step2Result.Success {
			result.Reachable = true
			result.Method = MethodDomainFronting
			result.Delay = step2Result.Delay
			result.GeneratedRule = GenerateRule(domain, MethodDomainFronting, sni, false)
			t.log("[DOMAIN] 域名 %s 测试完成: 可达=true, 方法=domain_fronting", domain)
			return result
		}

		// 2. TLS 分片测试
		step3Result := t.testTLSFragment(domain, bestIP, config)
		result.StepResults = append(result.StepResults, step3Result)
		if step3Result.Success {
			result.Reachable = true
			result.Method = MethodTLSFragment
			result.Delay = step3Result.Delay
			result.GeneratedRule = GenerateRule(domain, MethodTLSFragment, "", false)
			t.log("[DOMAIN] 域名 %s 测试完成: 可达=true, 方法=tls_fragment", domain)
			return result
		}

		// 3. QUIC 测试
		step6Result := t.testQUIC(domain, bestIP, config)
		result.StepResults = append(result.StepResults, step6Result)
		if step6Result.Success {
			result.Reachable = true
			result.Method = MethodQUIC
			result.Delay = step6Result.Delay
			result.GeneratedRule = GenerateRule(domain, MethodQUIC, "", false)
			t.log("[DOMAIN] 域名 %s 测试完成: 可达=true, 方法=quic", domain)
			return result
		}
	}

	result.Reachable = false
	result.Error = "所有测试方法均失败"
	t.log("[DOMAIN] 域名 %s 测试完成: 可达=false, 原因=所有方法失败", domain)
	return result
}

func (t *Tester) testDirect(domain string, config TestConfig) StepResult {
	t.log("[DIRECT] 开始直连测试: %s", domain)
	start := time.Now()

	dialer := &net.Dialer{Timeout: config.Timeout}
	// 直连测试使用普通的 tls 握手并强制限制 http/1.1
	// NOTE: InsecureSkipVerify is used here because this is a diagnostic test tool
	// that needs to check raw connectivity regardless of certificate validity.
	tlsConn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(domain, "443"), &tls.Config{
		ServerName:         domain,
		InsecureSkipVerify: true,
		NextProtos:         []string{"http/1.1"},
	})
	if err != nil {
		t.log("[DIRECT] 直连测试失败 (握手失败): %s, 错误: %v", domain, err)
		return StepResult{
			StepName:  "direct",
			Success:   false,
			Error:     err.Error(),
			Timestamp: start,
		}
	}

	delay, err := t.testHTTPOverConn(tlsConn, domain, config.Timeout)
	if err != nil {
		t.log("[DIRECT] 直连测试失败 (HTTP请求失败): %s, 错误: %v", domain, err)
		return StepResult{
			StepName:  "direct",
			Success:   false,
			Error:     err.Error(),
			Timestamp: start,
		}
	}

	t.log("[DIRECT] 直连测试成功: %s, 延迟: %v", domain, delay)
	return StepResult{
		StepName:  "direct",
		Success:   true,
		Delay:     delay,
		Timestamp: start,
	}
}

func (t *Tester) testTCPing(domain string, config TestConfig) (StepResult, []string, string) {
	t.log("[TCP-PING] 开始TCPing测试: %s", domain)
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ips, err := t.dohResolver.ResolveIPAddrs(ctx, domain)
	if err != nil {
		t.log("[TCP-PING] DNS解析失败: %s, 错误: %v", domain, err)
		return StepResult{
			StepName:  "tcp_ping",
			Success:   false,
			Error:     fmt.Sprintf("DNS解析失败: %v", err),
			Timestamp: start,
		}, nil, ""
	}

	t.log("[TCP-PING] 使用内置DNS解析: %s, IP列表: %v", domain, ips)

	var successIPs []string
	var bestIP string
	var bestDelay time.Duration

	for _, ip := range ips {
		pingResult := t.tcpingIP(ip.String(), config)
		if pingResult.Success {
			successIPs = append(successIPs, ip.String())
			if bestIP == "" || pingResult.Delay < bestDelay {
				bestIP = ip.String()
				bestDelay = pingResult.Delay
			}
		}
	}

	if len(successIPs) == 0 {
		t.log("[TCP-PING] TCPing测试失败: %s, 所有IP不可达", domain)
		return StepResult{
			StepName:  "tcp_ping",
			Success:   false,
			Error:     "所有IP不可达",
			Timestamp: start,
		}, ipsToStrings(ips), ""
	}

	t.log("[TCP-PING] TCPing测试完成: %s, 最优IP: %s, 延迟: %v", domain, bestIP, bestDelay)
	return StepResult{
		StepName:  "tcp_ping",
		Success:   true,
		Delay:     bestDelay,
		Timestamp: start,
	}, ipsToStrings(ips), bestIP
}

func (t *Tester) tcpingIP(ip string, config TestConfig) TCPingResult {
	successCount := 0
	var totalDelay time.Duration

	for i := 0; i < 4; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
		start := time.Now()
		dialer := &net.Dialer{}
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, "443"))
		cancel()

		if err == nil {
			conn.Close()
			successCount++
			totalDelay += time.Since(start)
		}
	}

	if successCount == 4 {
		return TCPingResult{
			IP:      ip,
			Success: true,
			Delay:   totalDelay / 4,
		}
	}

	if successCount >= 2 {
		for i := 0; i < 4; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
			start := time.Now()
			dialer := &net.Dialer{}
			conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, "443"))
			cancel()

			if err == nil {
				conn.Close()
				successCount++
				totalDelay += time.Since(start)
			}
		}

		if successCount >= 6 {
			return TCPingResult{
				IP:      ip,
				Success: true,
				Delay:   totalDelay / time.Duration(successCount),
			}
		}
	}

	return TCPingResult{
		IP:      ip,
		Success: false,
		Error:   fmt.Sprintf("连接不稳定，成功次数: %d/4", successCount),
	}
}

func (t *Tester) testDomainFronting(domain string, ip string, config TestConfig) (StepResult, string) {
	t.log("[DOMAIN-FRONTING] 开始域前置测试: %s (IP: %s)", domain, ip)
	start := time.Now()

	for _, sni := range config.DomainFrontingSNIs {
		t.log("[DOMAIN-FRONTING] 尝试SNI: %s", sni)

		rule := proxy.Rule{
			Mode:    "mitm",
			SniFake: sni,
			Enabled: true,
		}

		success, delay, err := t.testWithRule(domain, ip, rule, config)
		if success {
			t.log("[DOMAIN-FRONTING] SNI %s 测试成功: %s, 延迟: %v", sni, domain, delay)
			return StepResult{
				StepName:  "domain_fronting",
				Success:   true,
				Delay:     delay,
				Timestamp: start,
			}, sni
		}

		if err != nil {
			t.log("[DOMAIN-FRONTING] SNI %s 测试失败: %v", sni, err)
		}
	}

	return StepResult{
		StepName:  "domain_fronting",
		Success:   false,
		Error:     "所有SNI测试均失败",
		Timestamp: start,
	}, ""
}

func (t *Tester) testTLSFragment(domain string, ip string, config TestConfig) StepResult {
	t.log("[TLS-FRAGMENT] 开始TLS分片测试: %s (IP: %s)", domain, ip)
	start := time.Now()

	rule := proxy.Rule{
		Mode:    "tls-rf",
		Enabled: true,
	}

	success, delay, err := t.testWithRule(domain, ip, rule, config)
	if success {
		t.log("[TLS-FRAGMENT] TLS分片测试成功: %s, 延迟: %v", domain, delay)
		return StepResult{
			StepName:  "tls_fragment",
			Success:   true,
			Delay:     delay,
			Timestamp: start,
		}
	}

	t.log("[TLS-FRAGMENT] TLS分片测试失败: %s, 错误: %v", domain, err)
	return StepResult{
		StepName:  "tls_fragment",
		Success:   false,
		Error:     err.Error(),
		Timestamp: start,
	}
}

func (t *Tester) testECH(domain string, ip string, config TestConfig) StepResult {
	t.log("[ECH-TEST] 开始ECH测试: %s (IP: %s)", domain, ip)
	start := time.Now()

	rule := proxy.Rule{
		Mode:               "mitm",
		ECHEnabled:         true,
		ECHProfileID:       "legacy-cloudflare",
		ECHDiscoveryDomain: "crypto.cloudflare.com",
		Enabled:            true,
	}

	success, delay, err := t.testWithRule(domain, ip, rule, config)
	if success {
		t.log("[ECH-TEST] ECH测试成功: %s, 延迟: %v", domain, delay)
		return StepResult{
			StepName:  "ech",
			Success:   true,
			Delay:     delay,
			Timestamp: start,
		}
	}

	t.log("[ECH-TEST] ECH测试失败: %s, 错误: %v", domain, err)
	return StepResult{
		StepName:  "ech",
		Success:   false,
		Error:     err.Error(),
		Timestamp: start,
	}
}

func (t *Tester) testQUIC(domain string, ip string, config TestConfig) StepResult {
	t.log("[QUIC-TEST] 开始QUIC测试: %s (IP: %s)", domain, ip)
	start := time.Now()

	rule := proxy.Rule{
		Mode:    "quic",
		Enabled: true,
	}

	transport, err := t.proxyServer.NewQUICRoundTripper(domain, rule)
	if err != nil {
		t.log("[QUIC-TEST] 创建 HTTP/3 Transport 失败: %s, 错误: %v", domain, err)
		return StepResult{
			StepName:  "quic",
			Success:   false,
			Error:     err.Error(),
			Timestamp: start,
		}
	}
	defer transport.Close()

	client := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest("GET", "https://"+domain+"/", nil)
	if err != nil {
		return StepResult{
			StepName:  "quic",
			Success:   false,
			Error:     err.Error(),
			Timestamp: start,
		}
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Close = true

	resp, err := client.Do(req)
	if err != nil {
		t.log("[QUIC-TEST] QUIC HTTP/3 请求失败: %s, 错误: %v", domain, err)
		return StepResult{
			StepName:  "quic",
			Success:   false,
			Error:     err.Error(),
			Timestamp: start,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		t.log("[QUIC-TEST] QUIC HTTP/3 状态码不正确: %s, 状态码=%d", domain, resp.StatusCode)
		return StepResult{
			StepName:  "quic",
			Success:   false,
			Error:     fmt.Sprintf("HTTP status code: %d", resp.StatusCode),
			Timestamp: start,
		}
	}

	delay := time.Since(start)
	t.log("[QUIC-TEST] QUIC测试成功: %s, 状态码=%d, 延迟: %v", domain, resp.StatusCode, delay)
	return StepResult{
		StepName:  "quic",
		Success:   true,
		Delay:     delay,
		Timestamp: start,
	}
}

func (t *Tester) testWithRule(domain string, ip string, rule proxy.Rule, config TestConfig) (bool, time.Duration, error) {
	start := time.Now()

	dialCandidates := []string{net.JoinHostPort(ip, "443")}

	// 协商 http/1.1 便于在此基础上发起真 HTTP/1.1 请求
	conn, _, err := t.proxyServer.EstablishUpstreamConn(domain, rule, dialCandidates, "http/1.1")
	if err != nil {
		return false, 0, fmt.Errorf("建立连接失败: %v", err)
	}

	_, err = t.testHTTPOverConn(conn, domain, config.Timeout)
	if err != nil {
		return false, 0, err
	}

	delay := time.Since(start)
	t.log("[TEST] 真HTTP请求成功: %s, 延迟=%v", domain, delay)
	return true, delay, nil
}

func (t *Tester) log(format string, args ...interface{}) {
	msg := fmt.Sprintf("[%s] %s", time.Now().Format("2006-01-02T15:04:05-07:00"), fmt.Sprintf(format, args...))
	log.Print(msg)
	if t.logCallback != nil {
		t.logCallback(msg)
	}
}

func countByStatus(results []DomainTestResult, reachable bool) int {
	count := 0
	for _, r := range results {
		if r.Reachable == reachable {
			count++
		}
	}
	return count
}

func ipsToStrings(ips []net.IP) []string {
	result := make([]string, len(ips))
	for i, ip := range ips {
		result[i] = ip.String()
	}
	return result
}

func GetDefaultConcurrency() int {
	cpuCount := runtime.NumCPU()
	if cpuCount <= 0 {
		return 10
	}
	return cpuCount * 2
}

func (t *Tester) testHTTPOverConn(conn net.Conn, domain string, timeout time.Duration) (time.Duration, error) {
	start := time.Now()
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return conn, nil
		},
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return conn, nil
		},
		DisableKeepAlives: true,
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest("GET", "https://"+domain+"/", nil)
	if err != nil {
		conn.Close()
		return 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Close = true

	resp, err := client.Do(req)
	if err != nil {
		conn.Close()
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		conn.Close()
		return 0, fmt.Errorf("HTTP status code: %d", resp.StatusCode)
	}
	return time.Since(start), nil
}
