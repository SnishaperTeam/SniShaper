import React, { Suspense, lazy, useState, useEffect, createContext, useContext } from 'react';
import { HashRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import Sidebar from './components/Sidebar';
import WindowControls from './components/WindowControls';
import ToastProvider from './components/ToastProvider';
import {
  GetListenPort, GetCloseToTray, GetAutoStart,
  GetShowMainWindowOnAutoStart, GetAutoEnableProxyOnAutoStart, GetSocks5Enabled, GetSocks5Port,
  GetTUNConfig, GetTUNStatus, GetCloudflareConfig,
  GetCAInstallStatus, GetInstalledCerts, GetCloudflareIPStats,
  GetLanguage, GetTheme, SetTheme, EventsOn
} from './api/bindings';
import { I18nProvider, useTranslation } from './i18n/I18nContext';

const Welcome = lazy(() => import('./pages/Welcome'));

const Dashboard = lazy(() => import('./pages/Dashboard'));
const Proxies = lazy(() => import('./pages/Proxies'));
const Rules = lazy(() => import('./pages/Rules'));
const Routing = lazy(() => import('./pages/Routing'));
const Logs = lazy(() => import('./pages/Logs'));
const Settings = lazy(() => import('./pages/Settings'));
const DNS = lazy(() => import('./pages/DNS'));
const About = lazy(() => import('./pages/About'));

// Global settings cache — read once at app startup, shared across all pages
interface SettingsCache {
  port: number;
  closeToTray: boolean;
  autoStart: boolean;
  showMainOnAutoStart: boolean;
  autoEnableProxyOnAutoStart: boolean;
  socks5Enabled: boolean;
  socks5Port: string;
  tunConfig: any;
  tunStatus: any;
  cfConfig: any;
  caStatus: any;
  installedCerts: any[];
  ipStats: any[];
  language: string;
  theme: string;
}

const defaultCache: SettingsCache = {
  port: 8080, closeToTray: false, autoStart: false,
  showMainOnAutoStart: true, autoEnableProxyOnAutoStart: false,
  socks5Enabled: false,
  socks5Port: '8081',
  tunConfig: { enabled: false, mtu: 9000, dns_hijack: true, auto_route: true, strict_route: true },
  tunStatus: { supported: true, running: false, enabled: false, message: '' },
  cfConfig: { api_key: '', auto_update: true },
  caStatus: { Installed: false, CertPath: '', Platform: 'windows' },
  installedCerts: [], ipStats: [],
  language: '',
  theme: 'dark'
};

const SettingsCtx = createContext<{ cache: SettingsCache; updateCache: (patch: Partial<SettingsCache>) => void }>({
  cache: defaultCache,
  updateCache: () => {}
});

const App: React.FC = () => {
  const [theme, setTheme] = useState<'light' | 'dark'>((localStorage.getItem('theme') as any) || 'dark');
  const [settingsCache, setSettingsCache] = useState<SettingsCache>({
    ...defaultCache,
    language: localStorage.getItem('language') || '',
    theme: (localStorage.getItem('theme') as any) || 'dark'
  });
  const [initialized, setInitialized] = useState(false);

  useEffect(() => {
    // Phase 1: Load fast settings (language, theme, port) — pure file reads, < 5ms
    // Render immediately so there's zero loading screen.
    const fastLoad = async () => {
      try {
        const [language, savedTheme, port, closeToTray, autoStart,
          showMainOnAutoStart, autoEnableProxyOnAutoStart, socks5Enabled, socks5Port, cfConfig] = await Promise.all([
          GetLanguage(), GetTheme(), GetListenPort(), GetCloseToTray(), GetAutoStart(),
          GetShowMainWindowOnAutoStart(), GetAutoEnableProxyOnAutoStart(), GetSocks5Enabled(), GetSocks5Port(), GetCloudflareConfig()
        ]);
        const resolvedTheme = (savedTheme as any) || 'dark';
        setTheme(resolvedTheme);
        localStorage.setItem('theme', resolvedTheme);
        if (language) {
          localStorage.setItem('language', language as string);
        }
        setSettingsCache(prev => ({
          ...prev,
          language: (language as string) || '',
          theme: resolvedTheme,
          port: port ?? prev.port,
          closeToTray: closeToTray ?? prev.closeToTray,
          autoStart: autoStart ?? prev.autoStart,
          showMainOnAutoStart: showMainOnAutoStart ?? prev.showMainOnAutoStart,
          autoEnableProxyOnAutoStart: autoEnableProxyOnAutoStart ?? prev.autoEnableProxyOnAutoStart,
          socks5Enabled: socks5Enabled ?? prev.socks5Enabled,
          socks5Port: (socks5Port as string) || '8081',
          cfConfig: cfConfig || prev.cfConfig,
        }));
      } catch { /* ignore */ }
      setInitialized(true);
    };

    // Phase 2: Load slow settings (cert scan via PowerShell) — in background after render
    const slowLoad = async () => {
      try {
        const [tunConfig, tunStatus, caStatus, installedCerts, ipStats] = await Promise.all([
          GetTUNConfig(), GetTUNStatus(),
          GetCAInstallStatus(), GetInstalledCerts(), GetCloudflareIPStats()
        ]);
        setSettingsCache(prev => ({
          ...prev,
          tunConfig: tunConfig || prev.tunConfig,
          tunStatus: tunStatus || prev.tunStatus,
          caStatus: caStatus || prev.caStatus,
          installedCerts: installedCerts || [],
          ipStats: ipStats || [],
        }));
      } catch { /* ignore */ }
    };

    fastLoad().then(() => slowLoad());

    const unlisten = EventsOn("app:state", (state: any) => {
      if (!state) return;
      const updates: Partial<SettingsCache> = {};
      if (typeof state.listenPort === 'number') {
        updates.port = state.listenPort;
      }
      if (typeof state.socks5Port === 'string') {
        updates.socks5Port = state.socks5Port;
      }
      if (typeof state.socks5Enabled === 'boolean') {
        updates.socks5Enabled = state.socks5Enabled;
      }
      if (Object.keys(updates).length > 0) {
        setSettingsCache(prev => ({ ...prev, ...updates }));
      }
    });

    return () => {
      if (unlisten) unlisten();
    };
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

  const toggleTheme = () => {
    const next = theme === 'light' ? 'dark' : 'light';
    setTheme(next);
    SetTheme(next);
    localStorage.setItem('theme', next);
    updateSettingsCache({ theme: next });
  };
  return (
    <I18nProvider initialLanguage={(settingsCache.language as any) || 'zh'}>
      <AppContent 
        settingsCache={settingsCache} 
        updateSettingsCache={updateSettingsCache} 
        theme={theme} 
        toggleTheme={toggleTheme} 
      />
    </I18nProvider>
  );
};

const AppContent: React.FC<{ settingsCache: SettingsCache, updateSettingsCache: any, theme: any, toggleTheme: any }> = ({ settingsCache, updateSettingsCache, theme, toggleTheme }) => {
  const { t } = useTranslation();

  const routeFallback = <div className="h-full" />;

  if (!settingsCache.language) {
    return (
      <Suspense fallback={routeFallback}>
        <Welcome onComplete={(lang) => {
          updateSettingsCache({ language: lang });
        }} />
      </Suspense>
    );
  }

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
                  <Route path="/dns" element={<DNS />} />
                  <Route path="/logs" element={<Logs />} />
                  <Route path="/settings" element={<Settings cache={settingsCache} onCacheUpdate={updateSettingsCache} theme={theme} toggleTheme={toggleTheme} />} />
                  <Route path="/about" element={<About />} />
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
