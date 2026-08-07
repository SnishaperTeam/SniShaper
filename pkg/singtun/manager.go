package singtun

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"

	"snishaper/pkg/dohresolver"
	"snishaper/proxy"
)

// Manager 管理 sing-tun TUN 接口
type Manager struct {
	mu          sync.Mutex
	tun         tun.WinTun
	stack       tun.Stack
	handler     *Handler
	options     tun.Options
	running     bool
	resolver    *dohresolver.FailoverResolver
	logf        func(string)
}

// NewManager 创建新的 TUN 管理器
func NewManager(resolver *dohresolver.FailoverResolver, logf func(string)) *Manager {
	return &Manager{
		resolver: resolver,
		logf:     logf,
	}
}

// Start 启动 TUN
func (m *Manager) Start(cfg proxy.TUNConfig, proxyAddr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	// 1. 构建 TUN 选项
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
			// 用 TUN 网段内的非自身地址（198.18.0.2），原因：
			// 1. 查询路由进 TUN → gvisor 劫持（UDP:53）→ Handler 生成 fake-ip；
			// 2. 不能用 127.0.0.1（被 Inet4RouteExcludeAddress 排除，走 loopback），
			//    也不能用 198.18.0.1（TUN 自身地址，OS 本地投递，不经过 TUN）；
			// 3. 不能用公共 DNS（如 1.1.1.1）：Chrome 检测到系统 DNS 是公共 DNS 会
			//    自动启用 Secure DNS (DoH)，查询走 443 绕过 fake-ip 劫持，且 DoH
			//    域名被 MITM 规则处理后上游超时。私有地址不触发 DoH。
			netip.MustParseAddr("198.18.0.2"),
			netip.MustParseAddr("fd65:198:18::2"),
		},
		// 启用 DNS 劫持，使用 fake-ip 模式
		EXP_DisableDNSHijack: false,
		// 自环防护：排除 loopback
		Inet4RouteExcludeAddress: []netip.Prefix{
			netip.MustParsePrefix("127.0.0.0/8"),
		},
		Inet6RouteExcludeAddress: []netip.Prefix{
			netip.MustParsePrefix("::1/128"),
		},
		Logger: &singTunLogger{m.logf},
	}

	// 创建 InterfaceMonitor (sing-tun 需要)
	if cfg.AutoRoute {
		ifaceFinder := control.NewDefaultInterfaceFinder()
		if err := ifaceFinder.Update(); err != nil {
			m.logf("[sing-tun] failed to update interface finder: " + err.Error())
		}
		m.options.InterfaceFinder = ifaceFinder

		networkMonitor, err := tun.NewNetworkUpdateMonitor(&singTunLogger{m.logf})
		if err != nil {
			m.logf("[sing-tun] failed to create network monitor: " + err.Error())
		} else {
			if err := networkMonitor.Start(); err != nil {
				m.logf("[sing-tun] failed to start network monitor: " + err.Error())
			}
			ifaceMonitor, err := tun.NewDefaultInterfaceMonitor(networkMonitor, &singTunLogger{m.logf}, tun.DefaultInterfaceMonitorOptions{
				InterfaceFinder: ifaceFinder,
			})
			if err != nil {
				m.logf("[sing-tun] failed to create interface monitor: " + err.Error())
			} else {
				if err := ifaceMonitor.Start(); err != nil {
					m.logf("[sing-tun] failed to start interface monitor: " + err.Error())
				}
				m.options.InterfaceMonitor = ifaceMonitor
			}
		}
	}

	// 2. 创建 TUN 接口
	var err error
	m.tun, err = tun.New(m.options)
	if err != nil {
		return fmt.Errorf("create tun failed: %w", err)
	}

	// 3. 创建 Handler
	m.handler = NewHandler(proxyAddr, m.resolver, m.logf)

	// 4. 创建网络栈
	m.stack, err = tun.NewStack("gvisor", tun.StackOptions{
		Context:    context.Background(),
		Tun:        m.tun,
		TunOptions: m.options,
		Handler:    m.handler,
		UDPTimeout: 60 * time.Second,
		Logger:     &singTunLogger{m.logf},
	})
	if err != nil {
		m.tun.Close()
		return fmt.Errorf("create stack failed: %w", err)
	}

	// 5. 启动 TUN 接口 (添加路由)
	if err := m.tun.Start(); err != nil {
		m.tun.Close()
		return fmt.Errorf("start tun failed: %w", err)
	}

	// 6. 启动网络栈
	if err := m.stack.Start(); err != nil {
		m.tun.Close()
		return fmt.Errorf("start stack failed: %w", err)
	}

	m.running = true
	m.logf("[sing-tun] TUN started, running=true")
	return nil
}

// Stop 停止 TUN
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	if m.stack != nil {
		m.stack.Close()
	}
	if m.tun != nil {
		m.tun.Close()
	}

	m.running = false
	m.logf("[sing-tun] TUN stopped")
	return nil
}

// Status 获取 TUN 状态
func (m *Manager) Status() proxy.TUNStatus {
	m.mu.Lock()
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
