import React, { Suspense, lazy, useState, useEffect, createContext, useContext } from 'react';
import { HashRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import Sidebar from './components/Sidebar';
import WindowControls from './components/WindowControls';
import ToastProvider from './components/ToastProvider';
import {
  GetListenPort, GetCloseToTray, GetAutoStart,
  GetShowMainWindowOnAutoStart, GetAutoEnableProxyOnAutoStart,
  GetTUNConfig, GetTUNStatus, GetCloudflareConfig,
  GetCAInstallStatus, GetInstalledCerts, GetCloudflareIPStats
} from './api/bindings';

const Dashboard = lazy(() => import('./pages/Dashboard'));
const Proxies = lazy(() => import('./pages/Proxies'));
const Rules = lazy(() => import('./pages/Rules'));
const Routing = lazy(() => import('./pages/Routing'));
const Logs = lazy(() => import('./pages/Logs'));
const Settings = lazy(() => import('./pages/Settings'));

// Global settings cache — read once at app startup, shared across all pages
interface SettingsCache {
  port: number;
  closeToTray: boolean;
  autoStart: boolean;
  showMainOnAutoStart: boolean;
  autoEnableProxyOnAutoStart: boolean;
  tunConfig: any;
  tunStatus: any;
  cfConfig: any;
  caStatus: any;
  installedCerts: any[];
  ipStats: any[];
}

const defaultCache: SettingsCache = {
  port: 8080, closeToTray: false, autoStart: false,
  showMainOnAutoStart: true, autoEnableProxyOnAutoStart: false,
  tunConfig: { enabled: false, stack: 'gvisor', mtu: 9000, dns_hijack: true, auto_route: true, strict_route: true },
  tunStatus: { supported: true, running: false, enabled: false, stack: 'gvisor', message: '' },
  cfConfig: { api_key: '', doh_url: 'https://1.1.1.1/dns-query', auto_update: true, warp_enabled: false, warp_endpoint: '162.159.199.2' },
  caStatus: { Installed: false, CertPath: '', Platform: 'windows' },
  installedCerts: [], ipStats: []
};

const SettingsCtx = createContext<{ cache: SettingsCache; updateCache: (patch: Partial<SettingsCache>) => void }>({
  cache: defaultCache,
  updateCache: () => {}
});

const App: React.FC = () => {
  const [theme, setTheme] = useState<'light' | 'dark'>('light');
  const [settingsCache, setSettingsCache] = useState<SettingsCache>(defaultCache);

  // Load settings once on app startup
  useEffect(() => {
    const load = async () => {
      try {
        const [port, closeToTray, autoStart, showMainOnAutoStart, autoEnableProxyOnAutoStart,
          tunConfig, tunStatus, cfConfig, caStatus, installedCerts, ipStats] = await Promise.all([
            GetListenPort(), GetCloseToTray(), GetAutoStart(),
            GetShowMainWindowOnAutoStart(), GetAutoEnableProxyOnAutoStart(),
            GetTUNConfig(), GetTUNStatus(), GetCloudflareConfig(),
            GetCAInstallStatus(), GetInstalledCerts(), GetCloudflareIPStats()
          ]);
        setSettingsCache({
          port, closeToTray, autoStart, showMainOnAutoStart, autoEnableProxyOnAutoStart,
          tunConfig: tunConfig || defaultCache.tunConfig,
          tunStatus: tunStatus || defaultCache.tunStatus,
          cfConfig: cfConfig || defaultCache.cfConfig,
          caStatus: caStatus || defaultCache.caStatus,
          installedCerts: installedCerts || [],
          ipStats: ipStats || []
        });
      } catch { /* non-blocking */ }
    };
    load();
  }, []);

  const updateSettingsCache = (patch: Partial<SettingsCache>) => {
    setSettingsCache(prev => ({ ...prev, ...patch }));
  };

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    if (theme === 'dark') {
      document.documentElement.classList.add('dark');
    } else {
      document.documentElement.classList.remove('dark');
    }
  }, [theme]);

  useEffect(() => {
    const shouldAllowNativeMenu = (target: EventTarget | null) => {
      if (!(target instanceof HTMLElement)) return false;
      return Boolean(target.closest('input, textarea, [contenteditable="true"], [data-native-contextmenu="true"]'));
    };

    const handleContextMenu = (event: MouseEvent) => {
      if (shouldAllowNativeMenu(event.target)) return;
      event.preventDefault();
    };

    window.addEventListener('contextmenu', handleContextMenu);
    return () => window.removeEventListener('contextmenu', handleContextMenu);
  }, []);

  const toggleTheme = () => setTheme(prev => prev === 'light' ? 'dark' : 'light');
  const routeFallback = (
    <div className="h-full min-h-[240px] flex items-center justify-center text-sm text-text-secondary">
      正在加载页面...
    </div>
  );

  return (
    <Router>
      <div className="flex h-screen w-screen overflow-hidden bg-background select-none relative">
        <ToastProvider />
        <Sidebar theme={theme} toggleTheme={toggleTheme} />
        
        <main className="flex-1 min-w-0 bg-background-soft/30 backdrop-blur-sm relative flex flex-col">
          <header className="h-10 shrink-0 border-b border-border/60 bg-background/70 backdrop-blur-md flex items-center justify-between pl-4 pr-3">
            <div
              className="flex-1 h-full"
              style={{ "--wails-draggable": "drag" } as React.CSSProperties}
            />
            <WindowControls />
          </header>

          <div className="flex-1 overflow-y-auto overflow-x-hidden">
            <SettingsCtx.Provider value={{ cache: settingsCache, updateCache: updateSettingsCache }}>
              <Suspense fallback={routeFallback}>
                <Routes>
                  <Route path="/" element={<Navigate to="/dashboard" replace />} />
                  <Route path="/dashboard" element={<Dashboard />} />
                  <Route path="/proxies" element={<Proxies />} />
                  <Route path="/rules" element={<Rules />} />
                  <Route path="/routing" element={<Routing />} />
                  <Route path="/logs" element={<Logs />} />
                  <Route path="/settings" element={<Settings cache={settingsCache} onCacheUpdate={updateSettingsCache} />} />
                </Routes>
              </Suspense>
            </SettingsCtx.Provider>
          </div>
        </main>
      </div>
    </Router>
  );
};

export default App;
