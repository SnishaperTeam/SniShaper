package singtun

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"

	"snishaper/pkg/dohresolver"
	"snishaper/pkg/netiface"
	"snishaper/proxy"
)

type Manager struct {
	mu             sync.Mutex
	tun            tun.Tun
	stack          tun.Stack
	handler        *Handler
	options        tun.Options
	running        bool
	releasing      atomic.Bool
	resolver       *dohresolver.FailoverResolver
	logf           func(string)
	logger         *slog.Logger
	ifaceConfig    netiface.Config
	networkMonitor tun.NetworkUpdateMonitor
	ifaceMonitor   tun.DefaultInterfaceMonitor

	// 测试注入点（lifecycle_test.go）：设备创建工厂与残留网卡清理函数。
	// nil 时使用真实实现（tun.New / cleanupStaleAdapters），生产路径不受影响。
	newTunFn        func(options tun.Options) (tun.Tun, error)
	cleanAdaptersFn func(logf func(string)) int
}

func NewManager(resolver *dohresolver.FailoverResolver, logf func(string)) *Manager {
	return &Manager{
		resolver: resolver,
		logf:     logf,
		logger:   newBridgedLogger(logf),
	}
}

func (m *Manager) Start(cfg proxy.TUNConfig, proxyAddr string) (err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	// creating 标记本次 Start 是否已开始创建资源：只有创建中途失败才需要
	// releaseLocked 回滚。残留清理阶段失败（如等待上次释放超时）时尚未创建
	// 任何新资源，且可能仍有并发释放在进行，此时不能叠加 releaseLocked
	// （会与在途释放竞争同一批资源、提前清掉 releasing 标记）。
	creating := false
	defer func() {
		if err == nil || !creating {
			return
		}
		m.releaseLocked()
	}()

	// [US1] 显式残留清理阶段（设计决策 D7）：先清理上次运行的残留资源与
	// 遗留虚拟网卡，全部完成才开始创建新设备。反复启停或崩溃后重启时，
	// 残留不再累积到设备创建阶段才暴露。
	var alreadyRunning bool
	alreadyRunning, err = m.cleanupResidualsLocked()
	if err != nil || alreadyRunning {
		return err
	}
	creating = true

	netiface.InvalidateCache()
	m.ifaceConfig = cfg.InterfaceConfig()

	mtu := cfg.MTU
	if mtu <= 0 {
		mtu = 9000
	}

	m.options = tun.Options{
		Name:        "SniShaper",
		MTU:         uint32(mtu),
		Inet4Address: []netip.Prefix{
			netip.MustParsePrefix(fakeIPv4Prefix),
		},
		Inet4Gateway: netip.MustParseAddr(tunGateway4),
		Inet6Address: []netip.Prefix{
			netip.MustParsePrefix(fakeIPv6Prefix),
		},
		Inet6Gateway: netip.MustParseAddr(tunGateway6),
		AutoRoute:    cfg.AutoRoute,
		StrictRoute:  cfg.StrictRoute,
		DNSAddress: []netip.Addr{
			netip.MustParseAddr(tunDNS4),
			netip.MustParseAddr(tunDNS6),
		},
		EXP_DisableDNSHijack: false,
		Inet4RouteExcludeAddress: append(
			[]netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")},
			routeExcludePrefixes(cfg, false, m.logf)...,
		),
		Inet6RouteExcludeAddress: append(
			[]netip.Prefix{netip.MustParsePrefix("::1/128")},
			routeExcludePrefixes(cfg, true, m.logf)...,
		),
		Logger: &singTunLogger{m.logger},
	}

	stageStart := time.Now()
	ifaceFinder := control.NewDefaultInterfaceFinder()
	if err := ifaceFinder.Update(); err != nil {
		m.logger.Error("failed to update interface finder", "error", err)
	}
	m.options.InterfaceFinder = ifaceFinder
	m.logger.Info("start: interface finder updated", "elapsed", time.Since(stageStart).String())

	networkMonitor, err := tun.NewNetworkUpdateMonitor(&singTunLogger{m.logger})
	if err != nil {
		m.logger.Warn("failed to create network monitor", "error", err)
	} else {
		if err := networkMonitor.Start(); err != nil {
			m.logger.Warn("failed to start network monitor", "error", err)
			networkMonitor.Close()
		} else {
			m.networkMonitor = networkMonitor
			ifaceMonitor, err := tun.NewDefaultInterfaceMonitor(networkMonitor, &singTunLogger{m.logger}, tun.DefaultInterfaceMonitorOptions{
				InterfaceFinder: ifaceFinder,
			})
			if err != nil {
				m.logger.Warn("failed to create interface monitor", "error", err)
			} else {
				if err := ifaceMonitor.Start(); err != nil {
					m.logger.Warn("failed to start interface monitor", "error", err)
					ifaceMonitor.Close()
					networkMonitor.Close()
					m.networkMonitor = nil
				} else {
					m.ifaceMonitor = ifaceMonitor
					m.options.InterfaceMonitor = ifaceMonitor
				}
			}
		}
	}

	tunStart := time.Now()
	if runtime.GOOS == "darwin" {
		var lastErr error
		m.tun = nil
		for i := 0; i < 128; i++ {
			m.options.Name = fmt.Sprintf("utun%d", i)
			t, e := tun.New(m.options)
			if e == nil {
				m.tun = t
				m.logger.Info("created macOS TUN interface", "name", m.options.Name)
				break
			}
			lastErr = e
		}
		if m.tun == nil {
			return fmt.Errorf("create tun failed (tried utun0..utun127): %w", lastErr)
		}
	} else {
		newTun := m.newTunFn
		if newTun == nil {
			// tun.New 在 Windows 上返回具体类型 *tun.WinTun，这里包一层
			// 转成 tun.Tun 接口，与注入的测试工厂签名一致。
			newTun = func(options tun.Options) (tun.Tun, error) {
				return tun.New(options)
			}
		}
		if m.tun, err = newTunWithRetry(m.options, m.logf, newTun); err != nil {
			return err
		}
	}
	m.logger.Info("start: tun.New completed", "elapsed", time.Since(tunStart).String())

	m.handler = NewHandler(proxyAddr, m.resolver, m.logf)
	m.handler.SetInterfaceConfig(cfg.InterfaceConfig())

	stackStart := time.Now()
	m.stack, err = tun.NewStack("gvisor", tun.StackOptions{
		Context:    context.Background(),
		Tun:        m.tun,
		TunOptions: m.options,
		Handler:    m.handler,
		UDPTimeout: 60 * time.Second,
		Logger:     &singTunLogger{m.logger},
	})
	if err != nil {
		return fmt.Errorf("create stack failed: %w", err)
	}
	m.logger.Info("start: NewStack completed", "elapsed", time.Since(stackStart).String())

	routeStart := time.Now()
	if err = m.tun.Start(); err != nil {
		return fmt.Errorf("start tun failed: %w", err)
	}
	m.logger.Info("start: tun.Start (routes+dns) completed", "elapsed", time.Since(routeStart).String())

	stackUpStart := time.Now()
	if err = m.stack.Start(); err != nil {
		return fmt.Errorf("start stack failed: %w", err)
	}
	m.logger.Info("start: stack.Start completed", "elapsed", time.Since(stackUpStart).String())

	m.running = true
	netiface.InvalidateCache()
	m.logger.Info("TUN started, running=true")
	return nil
}

// cleanupResidualsLocked 是 Start 的显式残留清理阶段（US1，设计决策 D7）。
//
// 顺序：等待上一次释放完成（waitReleasingLocked，12s 上限维持）→ 若进程内
// 仍有残留资源（上次 Start 回滚未尽 / Stop 关闭超时遗留）执行一次
// releaseLocked → 清理本项目遗留的虚拟网卡 → 输出阶段耗时与移除数量日志。
//
// 强杀进程后的遗留网卡没有进程内状态可回收，只能在此用系统 API 移除；
// matchesSniShaperAdapter 的严格匹配保证绝不误删 sing-box / mihomo 的设备。
// 返回 alreadyRunning=true 表示等待释放期间另一路 Start 已完成启动，
// 调用方应直接视为启动成功（幂等语义）。
func (m *Manager) cleanupResidualsLocked() (alreadyRunning bool, err error) {
	start := time.Now()
	if err := m.waitReleasingLocked(); err != nil {
		return false, err
	}
	if m.running {
		return true, nil
	}
	if m.hasResidualsLocked() {
		m.logger.Warn("startup residual cleanup: previous instance resources still present, releasing")
		m.releaseLocked()
	}
	clean := m.cleanAdaptersFn
	if clean == nil {
		clean = cleanupStaleAdapters
	}
	removed := clean(m.logf)
	m.logger.Info("startup residual cleanup done",
		"removed_adapters", removed,
		"elapsed", time.Since(start).String())
	return false, nil
}

// hasResidualsLocked 报告进程内是否仍有上次实例的资源（与 Stop 的
// 残留判断保持同一口径：不能只看 running，Start 中途失败时 running
// 仍为 false 但资源可能已分配）。
func (m *Manager) hasResidualsLocked() bool {
	return m.tun != nil || m.stack != nil || m.handler != nil ||
		m.ifaceMonitor != nil || m.networkMonitor != nil
}

func newTunWithRetry(options tun.Options, logf func(string), newTun func(tun.Options) (tun.Tun, error)) (tun.Tun, error) {
	maxRetry := 3
	// 遗留网卡的首次清理已提升为 Start 的显式阶段（cleanupResidualsLocked，US1）；
	// 这里仅保留创建失败重试之间的防御性清理，移除数量只进日志。
	var lastErr error
	for i := 0; i < maxRetry; i++ {
		attemptStart := time.Now()
		t, err := newTun(options)
		if err == nil {
			return t, nil
		}
		lastErr = err
		if time.Since(attemptStart) < time.Second {
			return nil, fmt.Errorf("create tun failed: %w", err)
		}
		logf("[sing-tun] tun.New slow failure, retrying " + fmt.Sprint(i+1) + "/" + fmt.Sprint(maxRetry) + ": " + err.Error())
		cleanupStaleAdapters(logf)
	}
	return nil, fmt.Errorf("create tun failed after %d attempts: %w", maxRetry, lastErr)
}

func (m *Manager) waitReleasingLocked() error {
	deadline := time.Now().Add(12 * time.Second)
	for m.releasing.Load() {
		if time.Now().After(deadline) {
			return fmt.Errorf("previous TUN release still in progress, try again later")
		}
		m.mu.Unlock()
		time.Sleep(100 * time.Millisecond)
		m.mu.Lock()
	}
	return nil
}

func (m *Manager) releaseLocked() {
	m.releasing.Store(true)
	defer m.releasing.Store(false)

	// 顺序要求：先摘掉数据面（stack），再关 handler，最后才关设备。
	// 反过来先关 handler，stack 仍会把新流量派发进来，track 被 closed 拒绝后
	// 连接当场被关，客户端看到的是无意义的快速 RST 而不是干净的停止。
	if m.stack != nil {
		start := time.Now()
		m.logger.Info("release: closing stack (detaches dispatcher)")
		m.stack.Close()
		m.stack = nil
		m.logger.Info("release: stack closed", "elapsed", time.Since(start).String())
	}
	if m.handler != nil {
		start := time.Now()
		m.logger.Info("release: releasing handler")
		m.handler.Release()
		m.logger.Info("release: handler released", "elapsed", time.Since(start).String())
		m.handler = nil
	}
	if m.tun != nil {
		start := time.Now()
		m.logger.Info("release: closing tun")
		if closeWithTimeout("tun", m.tun.Close, 10*time.Second, m.logf) {
			dumpGoroutines(m.logf)
		}
		m.tun = nil
		m.logger.Info("release: tun close stage done", "elapsed", time.Since(start).String())
		cleanupStaleAdapters(m.logf)
	}
	if m.ifaceMonitor != nil {
		start := time.Now()
		m.logger.Info("release: closing interface monitor")
		m.ifaceMonitor.Close()
		m.ifaceMonitor = nil
		m.options.InterfaceMonitor = nil
		m.logger.Info("release: interface monitor closed", "elapsed", time.Since(start).String())
	}
	if m.networkMonitor != nil {
		start := time.Now()
		m.logger.Info("release: closing network monitor")
		m.networkMonitor.Close()
		m.networkMonitor = nil
		m.logger.Info("release: network monitor closed", "elapsed", time.Since(start).String())
	}
	invalidateIPv6Egress()
	m.running = false
}

// closeWithTimeout 在独立 goroutine 中执行 shutdown，最多等待 timeout。
// 超时后仍会等待该 goroutine 真正结束再返回：调用方随后要清理 Wintun 适配器，
// 而适配器删除会与仍在运行的 Close 抢占同一个设备句柄。
// 返回值表示是否发生过超时（供调用方决定是否 dump goroutine）。
func closeWithTimeout(name string, shutdown func() error, timeout time.Duration, logf func(string)) bool {
	done := make(chan struct{})
	start := time.Now()
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logf("[sing-tun] release: " + name + " close panicked: " + fmt.Sprint(r))
			}
			close(done)
		}()
		if err := shutdown(); err != nil {
			logf("[sing-tun] release: " + name + " close error: " + err.Error())
		}
	}()
	timedOut := false
	select {
	case <-done:
		logf("[sing-tun] release: " + name + " closed in " + time.Since(start).String())
		return false
	case <-time.After(timeout):
		timedOut = true
		logf("[sing-tun] release: " + name + " close timed out after " + timeout.String() + ", waiting for it to finish")
	}
	// 超时不是放弃：必须等 Close 真正返回，否则随后的适配器删除会与它竞争。
	// 这里不再设上限——Close 泄漏属于必须暴露的故障，不该被静默吞掉。
	<-done
	logf("[sing-tun] release: " + name + " close finally returned after " + time.Since(start).String())
	return timedOut
}

func dumpGoroutines(logf func(string)) {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	s := string(buf[:n])
	const chunkSize = 16 * 1024
	const capSize = 128 * 1024
	for i := 0; i < len(s) && i < capSize; i += chunkSize {
		end := i + chunkSize
		if end > len(s) {
			end = len(s)
		}
		logf("[sing-tun] goroutine dump (" + fmt.Sprint(i/chunkSize+1) + "): " + s[i:end])
	}
	if len(s) > capSize {
		logf("[sing-tun] goroutine dump truncated at " + fmt.Sprint(capSize) + " bytes (total " + fmt.Sprint(len(s)) + ")")
	}
}

func routeExcludePrefixes(cfg proxy.TUNConfig, ipv6 bool, logf func(string)) []netip.Prefix {
	ipv4Prefixes, ipv6Prefixes := cfg.RouteExcludePrefixes()
	source := ipv4Prefixes
	selfPrefix := fakeIPv4Prefix
	if ipv6 {
		source = ipv6Prefixes
		selfPrefix = fakeIPv6Prefix
	}
	self, err := netip.ParsePrefix(selfPrefix)
	if err != nil {
		return source
	}
	out := make([]netip.Prefix, 0, len(source))
	skipped := 0
	for _, prefix := range source {
		// 用户把 TUN 自身的 fake-ip 网段写进排除列表时，该网段流量不再进 TUN，
		// DNS 劫持随之失效且无任何报错，表现为"开了 TUN 就完全没网"。
		// 这里直接丢弃这类前缀并留下日志，其余配置照常生效。
		if prefix.Overlaps(self) {
			skipped++
			continue
		}
		out = append(out, prefix)
	}
	if skipped > 0 && logf != nil {
		logf("[sing-tun] ignored " + fmt.Sprint(skipped) + " route exclude entr(ies) overlapping TUN prefix " + selfPrefix)
	}
	return out
}

// Stop 停止 TUN 并释放全部资源。这是可重复调用的运行时停止，不是终态。
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 不能只看 m.running：Start 中途失败时 m.running 仍为false，
	// 但 tun/stack/handler 可能已经分配。必须按资源是否残留来决定是否清理。
	if !m.running && m.tun == nil && m.stack == nil && m.handler == nil &&
		m.ifaceMonitor == nil && m.networkMonitor == nil {
		return nil
	}

	m.releaseLocked()
	netiface.InvalidateCache()
	m.logger.Info("TUN stopped")
	return nil
}

// Shutdown 是进程退出路径上的终态销毁。与 Stop 的区别是会一并关闭 Handler，
// 使残留的连接即使在 Stop 之后才被派发也无法再生效。
func (m *Manager) Shutdown() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.handler != nil {
		m.handler.Close()
	}
	m.releaseLocked()
	netiface.InvalidateCache()
	m.logger.Info("TUN shutdown complete")
	return nil
}

func (m *Manager) Status() proxy.TUNStatus {
	status := proxy.TUNStatus{
		Supported: true,
		Running:   false,
		Enabled:   false,
		Driver:    "sing-tun",
		Message:   "TUN is not running",
	}

	// releasing 期间设备正在被销毁，任何快照都不再有意义。
	if m.releasing.Load() {
		return status
	}
	if !m.mu.TryLock() {
		// Start/Stop 正在持锁跑：tun.New 在 Windows 上可能耗时 15s 以上，
		// 此时若一律报 "not running"，前端会把一次正常启动显示成启动失败。
		// 忙状态单独措辞，避免与真实失败混淆。
		status.Message = "TUN is starting or stopping"
		return status
	}
	defer m.mu.Unlock()

	status.Running = m.running
	status.Enabled = m.running
	if m.running {
		status.Message = "TUN is running with sing-tun driver"
	}
	return status
}
