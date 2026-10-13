package singtun

import (
	"errors"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-tun"

	"snishaper/proxy"
)

// logCollector 线程安全地收集 logf 输出，供顺序断言。
type logCollector struct {
	mu    sync.Mutex
	lines []string
}

func (c *logCollector) logf(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, msg)
}

func (c *logCollector) all() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.lines...)
}

func (c *logCollector) indexOf(substr string) int {
	for i, line := range c.all() {
		if strings.Contains(line, substr) {
			return i
		}
	}
	return -1
}

// TestStartIsIdempotentWhenRunning 验证 US1 验收场景 3：
// 正常运行中重复 Start 必须幂等，不创建第二个设备实例。
func TestStartIsIdempotentWhenRunning(t *testing.T) {
	logs := &logCollector{}
	m := NewManager(nil, logs.logf)
	m.running = true

	if err := m.Start(proxy.TUNConfig{}, "127.0.0.1:1"); err != nil {
		t.Fatalf("repeated Start while running must succeed immediately, got %v", err)
	}
	if m.tun != nil || m.stack != nil || m.handler != nil {
		t.Fatal("idempotent Start must not create any resources")
	}
	if len(logs.all()) != 0 {
		t.Fatalf("idempotent Start must be a no-op, got logs: %v", logs.all())
	}
}

// TestCleanupRunsBeforeTunCreation 验证 US1 验收场景 1：
// 残留清理（含进程内残留资源释放与遗留网卡清理）必须先于 tun.New 执行，
// 且清理阶段把残留 Handler 释放干净。
func TestCleanupRunsBeforeTunCreation(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("darwin branch creates utun devices directly; factory injection covers windows/linux only")
	}

	logs := &logCollector{}
	m := NewManager(nil, logs.logf)

	var order []string
	var orderMu sync.Mutex
	record := func(step string) {
		orderMu.Lock()
		defer orderMu.Unlock()
		order = append(order, step)
	}

	m.cleanAdaptersFn = func(func(string)) int {
		record("adapter-cleanup")
		return 1
	}
	m.newTunFn = func(tun.Options) (tun.Tun, error) {
		record("tun-new")
		return nil, errors.New("no device in unit tests")
	}
	// 预置一个残留 Handler：清理阶段必须先把它释放掉再创建设备。
	m.handler = NewHandler("127.0.0.1:1", nil, func(string) {})

	err := m.Start(proxy.TUNConfig{}, "127.0.0.1:1")
	if err == nil {
		t.Fatal("Start must fail when tun creation fails")
	}

	orderMu.Lock()
	defer orderMu.Unlock()
	if len(order) != 2 || order[0] != "adapter-cleanup" || order[1] != "tun-new" {
		t.Fatalf("cleanup must run before tun creation, got order %v", order)
	}
	if m.handler != nil {
		t.Fatal("residual handler must be released by the cleanup stage")
	}
	if logs.indexOf("startup residual cleanup done") < 0 {
		t.Fatal("cleanup stage must log its completion with the removal count")
	}
}

// TestStartWaitsForOngoingRelease 验证 US1 验收场景 2：
// 上一次释放仍在进行时，再次 Start 必须等待其完成后再进入清理/创建流程，
// 不能在旧资源未清空时叠加创建新实例。
func TestStartWaitsForOngoingRelease(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("darwin branch creates utun devices directly; factory injection covers windows/linux only")
	}

	m := NewManager(nil, func(string) {})
	m.newTunFn = func(tun.Options) (tun.Tun, error) {
		return nil, errors.New("no device in unit tests")
	}
	m.cleanAdaptersFn = func(func(string)) int { return 0 }

	m.releasing.Store(true)
	done := make(chan error, 1)
	go func() { done <- m.Start(proxy.TUNConfig{}, "127.0.0.1:1") }()

	select {
	case <-done:
		t.Fatal("Start must wait while the previous release is still in progress")
	case <-time.After(200 * time.Millisecond):
	}

	m.releasing.Store(false)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Start must still surface the tun creation failure after waiting")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not proceed after the previous release finished")
	}
}

// fakeTun 实现 tun.Tun 接口，Close 行为可注入（挂死/计数），
// 供有界关闭与幂等测试使用。
type fakeTun struct {
	closeFn func() error
}

func (f *fakeTun) Read(p []byte) (int, error)           { return 0, io.EOF }
func (f *fakeTun) Write(p []byte) (int, error)          { return 0, io.EOF }
func (f *fakeTun) Name() (string, error)                { return "fake", nil }
func (f *fakeTun) Start() error                         { return nil }
func (f *fakeTun) Close() error                         { return f.closeFn() }
func (f *fakeTun) UpdateRouteOptions(tun.Options) error { return nil }

// TestStopIsBoundedWhenTunCloseHangs 验证 US2 验收场景 3（D6）：
// 底层设备 Close 挂死时，Stop 必须在 timeout+hardCap 到顶后返回，
// 完成其余清理且不冻结主流程。
func TestStopIsBoundedWhenTunCloseHangs(t *testing.T) {
	m := NewManager(nil, func(string) {})
	m.closeTimeout = 30 * time.Millisecond
	m.closeHardCap = 100 * time.Millisecond
	cleaned := 0
	m.cleanAdaptersFn = func(func(string)) int { cleaned++; return 0 }

	m.tun = &fakeTun{closeFn: func() error { select {} }}
	m.handler = NewHandler("127.0.0.1:1", nil, func(string) {})
	m.running = true

	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- m.Stop() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Stop must succeed even when tun.Close hangs, got %v", err)
		}
		if elapsed := time.Since(start); elapsed > 3*time.Second {
			t.Fatalf("Stop must be bounded even when tun.Close hangs, took %v", elapsed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return while tun.Close hung")
	}
	if m.tun != nil {
		t.Fatal("Stop must clear the tun reference even on hung Close")
	}
	if cleaned != 0 {
		t.Fatal("adapter cleanup must be skipped when the close leaked (handle race, D6)")
	}
}

// TestStopAndShutdownAreIdempotent 验证 FR-004：
// Stop/Shutdown 可重复调用，不报错也不重复清理。
func TestStopAndShutdownAreIdempotent(t *testing.T) {
	m := NewManager(nil, func(string) {})
	m.closeTimeout = time.Second
	m.closeHardCap = 2 * time.Second
	m.cleanAdaptersFn = func(func(string)) int { return 0 }

	// 空实例：直接返回成功。
	for i := 0; i < 2; i++ {
		if err := m.Stop(); err != nil {
			t.Fatalf("Stop on an idle manager must be a no-op, got %v", err)
		}
		if err := m.Shutdown(); err != nil {
			t.Fatalf("Shutdown on an idle manager must be a no-op, got %v", err)
		}
	}

	// 带资源的实例：第一次 Stop 清理一次，后续调用不再触碰资源。
	closeCount := 0
	m.tun = &fakeTun{closeFn: func() error { closeCount++; return nil }}
	m.handler = NewHandler("127.0.0.1:1", nil, func(string) {})
	m.running = true

	if err := m.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := m.Stop(); err != nil {
			t.Fatalf("repeated Stop must succeed, got %v", err)
		}
		if err := m.Shutdown(); err != nil {
			t.Fatalf("Shutdown after Stop must succeed, got %v", err)
		}
	}
	if closeCount != 1 {
		t.Fatalf("resources must be cleaned exactly once, tun closed %d times", closeCount)
	}
}
