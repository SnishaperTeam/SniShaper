package singtun

import (
	"context"
	"fmt"
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
	ifaceConfig    netiface.Config
	networkMonitor tun.NetworkUpdateMonitor
	ifaceMonitor   tun.DefaultInterfaceMonitor
}

func NewManager(resolver *dohresolver.FailoverResolver, logf func(string)) *Manager {
	return &Manager{
		resolver: resolver,
		logf:     logf,
	}
}

func (m *Manager) Start(cfg proxy.TUNConfig, proxyAddr string) (err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}
	if err = m.waitReleasingLocked(); err != nil {
		return err
	}

	released := false
	defer func() {
		if err == nil || released {
			return
		}
		m.releaseLocked()
	}()

	netiface.InvalidateCache()
	m.ifaceConfig = cfg.InterfaceConfig()

	mtu := cfg.MTU
	if mtu <= 0 {
		mtu = 9000
	}

	m.options = tun.Options{
		Name: "SniShaper",
		MTU:  uint32(mtu),
		Inet4Address: []netip.Prefix{
			netip.MustParsePrefix("198.18.0.1/16"),
		},
		Inet4Gateway: netip.MustParseAddr("198.18.0.1"),
		Inet6Address: []netip.Prefix{
			netip.MustParsePrefix("fd65:198:18::1/64"),
		},
		Inet6Gateway: netip.MustParseAddr("fd65:198:18::1"),
		AutoRoute:    cfg.AutoRoute,
		StrictRoute:  cfg.StrictRoute,
		DNSServers: []netip.Addr{
			netip.MustParseAddr("198.18.0.2"),
			netip.MustParseAddr("fd65:198:18::2"),
		},
		EXP_DisableDNSHijack: false,
		Inet4RouteExcludeAddress: append(
			[]netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")},
			routeExcludePrefixes(cfg, false)...,
		),
		Inet6RouteExcludeAddress: append(
			[]netip.Prefix{netip.MustParsePrefix("::1/128")},
			routeExcludePrefixes(cfg, true)...,
		),
		Logger: &singTunLogger{m.logf},
	}

	if cfg.AutoRoute {
		stageStart := time.Now()
		ifaceFinder := control.NewDefaultInterfaceFinder()
		if err := ifaceFinder.Update(); err != nil {
			m.logf("[sing-tun] failed to update interface finder: " + err.Error())
		}
		m.options.InterfaceFinder = ifaceFinder
		m.logf("[sing-tun] start: interface finder updated in " + time.Since(stageStart).String())

		networkMonitor, err := tun.NewNetworkUpdateMonitor(&singTunLogger{m.logf})
		if err != nil {
			m.logf("[sing-tun] failed to create network monitor: " + err.Error())
		} else {
			if err := networkMonitor.Start(); err != nil {
				m.logf("[sing-tun] failed to start network monitor: " + err.Error())
				networkMonitor.Close()
			} else {
				m.networkMonitor = networkMonitor
				ifaceMonitor, err := tun.NewDefaultInterfaceMonitor(networkMonitor, &singTunLogger{m.logf}, tun.DefaultInterfaceMonitorOptions{
					InterfaceFinder: ifaceFinder,
				})
				if err != nil {
					m.logf("[sing-tun] failed to create interface monitor: " + err.Error())
				} else {
					if err := ifaceMonitor.Start(); err != nil {
						m.logf("[sing-tun] failed to start interface monitor: " + err.Error())
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
	}

	tunStart := time.Now()
	if runtime.GOOS == "darwin" {
		var lastErr error
		for i := 0; i < 128; i++ {
			m.options.Name = fmt.Sprintf("utun%d", i)
			t, e := tun.New(m.options)
			if e == nil {
				m.tun = t
				m.logf("[sing-tun] created macOS TUN interface: " + m.options.Name)
				break
			}
			lastErr = e
		}
		if m.tun == nil {
			return fmt.Errorf("create tun failed (tried utun0..utun127): %w", lastErr)
		}
	} else {
		if m.tun, err = newTunWithRetry(m.options, m.logf); err != nil {
			return err
		}
	}
	m.logf("[sing-tun] start: tun.New completed in " + time.Since(tunStart).String())

	m.handler = NewHandler(proxyAddr, m.resolver, m.logf)
	m.handler.SetInterfaceConfig(cfg.InterfaceConfig())

	stackStart := time.Now()
	m.stack, err = tun.NewStack("gvisor", tun.StackOptions{
		Context:    context.Background(),
		Tun:        m.tun,
		TunOptions: m.options,
		Handler:    m.handler,
		UDPTimeout: 60 * time.Second,
		Logger:     &singTunLogger{m.logf},
	})
	if err != nil {
		return fmt.Errorf("create stack failed: %w", err)
	}
	m.logf("[sing-tun] start: NewStack completed in " + time.Since(stackStart).String())

	routeStart := time.Now()
	if err = m.tun.Start(); err != nil {
		return fmt.Errorf("start tun failed: %w", err)
	}
	m.logf("[sing-tun] start: tun.Start (routes+dns) completed in " + time.Since(routeStart).String())

	stackUpStart := time.Now()
	if err = m.stack.Start(); err != nil {
		return fmt.Errorf("start stack failed: %w", err)
	}
	m.logf("[sing-tun] start: stack.Start completed in " + time.Since(stackUpStart).String())

	m.running = true
	released = true
	netiface.InvalidateCache()
	m.logf("[sing-tun] TUN started, running=true")
	return nil
}

func newTunWithRetry(options tun.Options, logf func(string)) (tun.Tun, error) {
	maxRetry := 3
	var lastErr error
	for i := 0; i < maxRetry; i++ {
		attemptStart := time.Now()
		t, err := tun.New(options)
		if err == nil {
			return t, nil
		}
		lastErr = err
		if time.Since(attemptStart) < time.Second {
			return nil, fmt.Errorf("create tun failed: %w", err)
		}
		logf("[sing-tun] tun.New slow failure, retrying " + fmt.Sprint(i+1) + "/" + fmt.Sprint(maxRetry) + ": " + err.Error())
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

	if m.handler != nil {
		m.handler.Close()
	}
	if m.stack != nil {
		start := time.Now()
		m.logf("[sing-tun] release: closing stack (detaches dispatcher)")
		m.stack.Close()
		m.stack = nil
		m.logf("[sing-tun] release: stack closed in " + time.Since(start).String())
	}
	if m.tun != nil {
		start := time.Now()
		m.logf("[sing-tun] release: closing tun")
		if closeWithTimeout("tun", m.tun.Close, 10*time.Second, m.logf) {
			dumpGoroutines(m.logf)
		}
		m.tun = nil
		m.logf("[sing-tun] release: tun close stage done in " + time.Since(start).String())
	}
	if m.ifaceMonitor != nil {
		start := time.Now()
		m.logf("[sing-tun] release: closing interface monitor")
		m.ifaceMonitor.Close()
		m.ifaceMonitor = nil
		m.options.InterfaceMonitor = nil
		m.logf("[sing-tun] release: interface monitor closed in " + time.Since(start).String())
	}
	if m.networkMonitor != nil {
		start := time.Now()
		m.logf("[sing-tun] release: closing network monitor")
		m.networkMonitor.Close()
		m.networkMonitor = nil
		m.logf("[sing-tun] release: network monitor closed in " + time.Since(start).String())
	}
	m.handler = nil
	m.running = false
}

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
	select {
	case <-done:
		logf("[sing-tun] release: " + name + " closed in " + time.Since(start).String())
		return false
	case <-time.After(timeout):
		logf("[sing-tun] release: " + name + " close timed out after " + timeout.String() + ", continuing")
		return true
	}
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

func routeExcludePrefixes(cfg proxy.TUNConfig, ipv6 bool) []netip.Prefix {
	ipv4Prefixes, ipv6Prefixes := cfg.RouteExcludePrefixes()
	if ipv6 {
		return ipv6Prefixes
	}
	return ipv4Prefixes
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.releaseLocked()
	netiface.InvalidateCache()
	m.logf("[sing-tun] TUN stopped")
	return nil
}

func (m *Manager) Status() proxy.TUNStatus {
	if m.releasing.Load() || !m.mu.TryLock() {
		return proxy.TUNStatus{
			Supported: true,
			Running:   false,
			Enabled:   false,
			Driver:    "sing-tun",
			Message:   "TUN is not running",
		}
	}

	defer m.mu.Unlock()

	status := proxy.TUNStatus{
		Supported: true,
		Running:   m.running,
		Enabled:   m.running,
		Driver:    "sing-tun",
	}

	if !m.running {
		status.Message = "TUN is not running"
	} else {
		status.Message = "TUN is running with sing-tun driver"
	}

	return status
}
