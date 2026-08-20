import React, { Suspense, lazy, useState, useEffect, useCallback, useRef, createContext, useContext } from 'react';
import { HashRouter as Router, Routes, Route, Navigate, useLocation } from 'react-router-dom';
import { Box, CssBaseline, ThemeProvider, AppBar, Toolbar, Typography, CircularProgress } from '@mui/material';
import { alpha } from '@mui/material/styles';
import { keyframes } from '@emotion/react';
import Sidebar from './components/Sidebar';
import WindowControls from './components/WindowControls';
import ToastProvider from './components/ToastProvider';
import { appTheme, defaultTheme, availableThemes } from './theme';
import {
  GetListenPort, GetCloseToTray, GetAutoStart,
  GetShowMainWindowOnAutoStart, GetAutoEnableProxyOnAutoStart, GetSocks5Enabled, GetSocks5Port,
  GetTUNConfig, GetTUNStatus, GetCloudflareConfig,
  GetCAInstallStatus, GetInstalledCerts, GetCloudflareIPStats,
  GetLanguage, GetTheme, SetTheme, GetIPv6Available, EventsOn
} from './api/bindings';
import { I18nProvider, useTranslation } from './i18n/I18nContext';
import { toast } from './lib/toast';
import logoUrl from './assets/logo.svg';

const Welcome = lazy(() => import('./pages/Welcome'));

const fadeIn = keyframes`from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: translateY(0); }`;

const Dashboard = lazy(() => import('./pages/Dashboard'));
const Proxies = lazy(() => import('./pages/Proxies'));
const Rules = lazy(() => import('./pages/Rules'));
const Routing = lazy(() => import('./pages/Routing'));
const Logs = lazy(() => import('./pages/Logs'));
const Settings = lazy(() => import('./pages/Settings'));
const DNS = lazy(() => import('./pages/DNS'));
const About = lazy(() => import('./pages/About'));
const Evolution = lazy(() => import('./pages/Evolution'));

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
  themeMode: string;
  ipv6Available: boolean;
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
  themeMode: 'dark',
  ipv6Available: true
};

const SettingsCtx = createContext<{ cache: SettingsCache; updateCache: (patch: Partial<SettingsCache>) => void }>({
  cache: defaultCache,
  updateCache: () => {}
});
export { SettingsCtx };

const App: React.FC = () => {
  const [settingsCache, setSettingsCache] = useState<SettingsCache>({
    ...defaultCache,
    language: localStorage.getItem('language') || '',
    themeMode: (localStorage.getItem('mui-mode') as any) || 'dark'
  });
  const [initialized, setInitialized] = useState(false);
  const [themeId, setThemeId] = useState<string>(() => localStorage.getItem('theme-id') || 'default');
  const activeTheme = availableThemes.find((th) => th.id === themeId)?.theme || defaultTheme;

  useEffect(() => {
    void import('./pages/Dashboard');
    void import('./pages/Proxies');
    void import('./pages/Rules');
    void import('./pages/Routing');
    void import('./pages/DNS');
    void import('./pages/Evolution');
    void import('./pages/Logs');
    void import('./pages/Settings');
    void import('./pages/About');
  }, []);

  useEffect(() => {
    const fastLoad = async () => {
      try {
        const [language, port, closeToTray, autoStart,
          showMainOnAutoStart, autoEnableProxyOnAutoStart, socks5Enabled, socks5Port, cfConfig, ipv6Available] = await Promise.all([
          GetLanguage(), GetListenPort(), GetCloseToTray(), GetAutoStart(),
          GetShowMainWindowOnAutoStart(), GetAutoEnableProxyOnAutoStart(), GetSocks5Enabled(), GetSocks5Port(), GetCloudflareConfig(), GetIPv6Available()
        ]);
        if (language) {
          localStorage.setItem('language', language as string);
        }
        setSettingsCache(prev => ({
          ...prev,
          language: (language as string) || '',
          port: port ?? prev.port,
          closeToTray: closeToTray ?? prev.closeToTray,
          autoStart: autoStart ?? prev.autoStart,
          showMainOnAutoStart: showMainOnAutoStart ?? prev.showMainOnAutoStart,
          autoEnableProxyOnAutoStart: autoEnableProxyOnAutoStart ?? prev.autoEnableProxyOnAutoStart,
          socks5Enabled: socks5Enabled ?? prev.socks5Enabled,
          socks5Port: (socks5Port as string) || '8081',
          cfConfig: cfConfig || prev.cfConfig,
          ipv6Available: typeof ipv6Available === 'boolean' ? ipv6Available : prev.ipv6Available,
        }));
      } catch { /* ignore */ }
      setInitialized(true);
    };
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
    const unlisten2 = EventsOn("app:state_changed", (state: any) => {
      if (state && typeof state.ipv6Available === 'boolean') {
        setSettingsCache(prev => ({ ...prev, ipv6Available: state.ipv6Available }));
      }
    });
    return () => {
      if (unlisten) unlisten();
      if (unlisten2) unlisten2();
    };
  }, []);

  const updateSettingsCache = useCallback((patch: Partial<SettingsCache>) => {
    setSettingsCache(prev => ({ ...prev, ...patch }));
  }, []);

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

  return (
    <ThemeProvider theme={activeTheme} defaultMode="system">
      <CssBaseline enableColorScheme />
      <I18nProvider initialLanguage={(settingsCache.language as any) || 'zh'}>
        <AppContent
          settingsCache={settingsCache}
          updateSettingsCache={updateSettingsCache}
          themeId={themeId}
          onThemeChange={(id: string) => { setThemeId(id); localStorage.setItem('theme-id', id); }}
        />
      </I18nProvider>
    </ThemeProvider>
  );
};

const routeFallback = (
  <Box sx={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center', animation: `${fadeIn} 0.25s ease` }}>
    <CircularProgress size={28} color="primary" />
  </Box>
);

const AppRoutes: React.FC<{ settingsCache: SettingsCache, updateSettingsCache: any, themeId: string, onThemeChange: (id: string) => void }> = ({ settingsCache, updateSettingsCache, themeId, onThemeChange }) => {
  const location = useLocation();
  return (
    <Box key={location.key} sx={{ flexGrow: 1, flexShrink: 1, minHeight: 0, minWidth: 0, display: 'flex', flexDirection: 'column', animation: `${fadeIn} 0.3s cubic-bezier(0.16, 1, 0.3, 1)` }}>
      <SettingsCtx.Provider value={{ cache: settingsCache, updateCache: updateSettingsCache }}>
        <Suspense fallback={routeFallback}>
          <Routes>
            <Route path="/" element={<Navigate to="/dashboard" replace />} />
            <Route path="/dashboard" element={<Dashboard />} />
            <Route path="/proxies" element={<Proxies />} />
            <Route path="/rules" element={<Rules />} />
            <Route path="/routing" element={<Routing />} />
            <Route path="/dns" element={<DNS />} />
            <Route path="/evolution" element={<Evolution />} />
            <Route path="/logs" element={<Logs />} />
            <Route path="/settings" element={<Settings cache={settingsCache} onCacheUpdate={updateSettingsCache} currentThemeId={themeId} onThemeChange={onThemeChange} />} />
            <Route path="/about" element={<About />} />
          </Routes>
        </Suspense>
      </SettingsCtx.Provider>
    </Box>
  );
};

const AppContent: React.FC<{ settingsCache: SettingsCache, updateSettingsCache: any, themeId: string, onThemeChange: (id: string) => void }> = ({ settingsCache, updateSettingsCache, themeId, onThemeChange }) => {
  const { t } = useTranslation();
  const [glowPos, setGlowPos] = useState({ x: 0.5, y: 0.3 });
  const glowRef = useRef<HTMLDivElement>(null);

  const handleGlowMove = useCallback((e: React.MouseEvent) => {
    const rect = e.currentTarget.getBoundingClientRect();
    requestAnimationFrame(() => {
      setGlowPos({
        x: (e.clientX - rect.left) / rect.width,
        y: (e.clientY - rect.top) / rect.height,
      });
    });
  }, []);

  const prevIpv6Ref = useRef<boolean | null>(null);
  useEffect(() => {
    const prev = prevIpv6Ref.current;
    prevIpv6Ref.current = settingsCache.ipv6Available;
    if (prev === null || prev === settingsCache.ipv6Available) return;
    if (settingsCache.ipv6Available) {
      toast.success(t('network.ipv6_restored'));
    } else {
      toast.error(t('network.ipv6_disabled'));
    }
  }, [settingsCache.ipv6Available, t]);

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
      <Box sx={{ display: 'flex', height: '100vh', width: '100vw', overflow: 'hidden', position: 'relative', userSelect: 'none' }}>
        <ToastProvider />
        <Sidebar />
        <Box
          ref={glowRef}
          onMouseMove={handleGlowMove}
          sx={{
            flexGrow: 1, display: 'flex', flexDirection: 'column', position: 'relative', bgcolor: 'background.default', minWidth: 0,
            '&::before': {
              content: '""', position: 'absolute', inset: 0, pointerEvents: 'none', zIndex: 0,
              backgroundImage: (theme: any) => {
                const line = theme.palette.mode === 'dark' ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.08)';
                const glow = theme.palette.mode === 'dark' ? 'rgba(255,255,255,0.1)' : alpha(theme.palette.primary.main, 0.12);
                return `
                  linear-gradient(${line} 1px, transparent 1px),
                  linear-gradient(90deg, ${line} 1px, transparent 1px),
                  radial-gradient(ellipse 800px 500px at ${glowPos.x * 100}% ${glowPos.y * 100}%, ${glow} 0%, transparent 70%)
                `;
              },
              backgroundSize: '48px 48px, 48px 48px, 100% 100%',
              opacity: 1,
            },
          }}>
          <AppBar position="fixed" sx={{
            zIndex: (theme) => theme.zIndex.drawer + 1,
            bgcolor: (theme) => alpha(theme.palette.background.paper, 0.64),
            backdropFilter: 'blur(20px) saturate(180%)',
            WebkitBackdropFilter: 'blur(20px) saturate(180%)',
            boxShadow: '0 1px 0 rgba(127,127,127,0.12)',
          }}>
            <Toolbar variant="dense" sx={{ '--wails-draggable': 'drag', gap: 1.5 }}>
              <Box
                component="img"
                src={logoUrl}
                alt="SniShaper"
                sx={{ width: 22, height: 22, objectFit: 'contain', flexShrink: 0 }}
              />
              <Typography
                variant="subtitle1"
                noWrap
                sx={{
                  fontWeight: 700,
                  letterSpacing: '0.1em',
                  color: 'text.primary',
                  opacity: 0.82,
                  userSelect: 'none',
                }}
              >
                SniShaper
              </Typography>
              <Box sx={{ flexGrow: 1 }} />
              <WindowControls />
            </Toolbar>
          </AppBar>
          <Box component="main" sx={{ flexGrow: 1, pt: 3, px: 3, pb: 0, overflow: 'hidden', display: 'flex', flexDirection: 'column', position: 'relative', zIndex: 1 }}>
            <Toolbar sx={{ visibility: 'hidden' }} />
            <AppRoutes
              settingsCache={settingsCache}
              updateSettingsCache={updateSettingsCache}
              themeId={themeId}
              onThemeChange={onThemeChange}
            />
          </Box>
        </Box>
      </Box>
    </Router>
  );
};

export default App;