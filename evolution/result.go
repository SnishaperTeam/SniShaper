package evolution

import (
	"time"
)

type TestStatus string

const (
	StatusPending   TestStatus = "pending"
	StatusRunning   TestStatus = "running"
	StatusCompleted TestStatus = "completed"
	StatusFailed    TestStatus = "failed"
)

type TestMethod string

const (
	MethodDirect         TestMethod = "direct"
	MethodDomainFronting TestMethod = "domain_fronting"
	MethodTLSFragment    TestMethod = "tls_fragment"
	MethodECH            TestMethod = "ech"
	MethodQUIC           TestMethod = "quic"
)

type StepResult struct {
	StepName  string        `json:"step_name"`
	Success   bool          `json:"success"`
	Delay     time.Duration `json:"delay"`
	Error     string        `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

type DomainTestResult struct {
	Domain        string        `json:"domain"`
	Reachable     bool          `json:"reachable"`
	Method        TestMethod    `json:"method,omitempty"`
	ResolvedIPs   []string      `json:"resolved_ips,omitempty"`
	BestIP        string        `json:"best_ip,omitempty"`
	IsCloudflare  bool          `json:"is_cloudflare"`
	Delay         time.Duration `json:"delay"`
	Error         string        `json:"error,omitempty"`
	Timestamp     time.Time     `json:"timestamp"`
	StepResults   []StepResult  `json:"step_results"`
	GeneratedRule *TempRule     `json:"generated_rule,omitempty"`
}

type TempRule struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Domain      string    `json:"domain"`
	Mode        string    `json:"mode"`
	SniFake     string    `json:"sni_fake,omitempty"`
	ECHEnabled  bool      `json:"ech_enabled"`
	Method      TestMethod `json:"method"`
	CreatedAt   time.Time `json:"created_at"`
	IsApplied   bool      `json:"is_applied"`
	UseCFPool   bool      `json:"use_cf_pool"`
}

type TestConfig struct {
	EnableIPv6        bool          `json:"enable_ipv6"`
	DomainFrontingSNIs []string     `json:"domain_fronting_snis"`
	Timeout           time.Duration `json:"timeout"`
	Concurrency       int           `json:"concurrency"`
}

func DefaultTestConfig() TestConfig {
	return TestConfig{
		EnableIPv6:         false,
		DomainFrontingSNIs: []string{"apple.com", "app-com"},
		Timeout:            3 * time.Second,
		Concurrency:        10,
	}
}

type TestTask struct {
	ID        string            `json:"id"`
	Domains   []string          `json:"domains"`
	Status    TestStatus        `json:"status"`
	Config    TestConfig        `json:"config"`
	Results   []DomainTestResult `json:"results"`
	StartTime time.Time         `json:"start_time,omitempty"`
	EndTime   time.Time         `json:"end_time,omitempty"`
	Progress  int               `json:"progress"`
	Total     int               `json:"total"`
}

type TCPingResult struct {
	IP       string        `json:"ip"`
	Success  bool          `json:"success"`
	Delay    time.Duration `json:"delay"`
	Error    string        `json:"error,omitempty"`
}
