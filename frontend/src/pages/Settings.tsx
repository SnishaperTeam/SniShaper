import React, { useState, useEffect } from 'react';
import {
  Settings as SettingsIcon,
  Save,
  ShieldAlert,
  Download,
  Cloud,
  FolderOpen,
  RefreshCcw,
  Monitor,
  Anchor,
  HelpCircle,
  Cpu,
  Globe,
  BellRing,
  Activity,
  CloudLightning,
  Zap,
  Trash2,
  AlertCircle,
  Upload,
  Sun,
  Moon
} from 'lucide-react';
import {
  GetListenPort,
  SetListenPort,
  GetCloseToTray,
  SetCloseToTray,
  GetAutoStart,
  SetAutoStart,
  GetShowMainWindowOnAutoStart,
  SetShowMainWindowOnAutoStart,
  GetAutoEnableProxyOnAutoStart,
  SetAutoEnableProxyOnAutoStart,
  GetSocks5Port,
  SetSocks5Port,
  GetTUNConfig,
  UpdateTUNConfig,
  GetTUNStatus,
  OpenCertDir,
  RegenerateCert,
  GetCAInstallStatus,
  GetInstalledCerts,
  UninstallCert,
  ExportConfig,
  ImportConfigWithSummary,
  GetCloudflareConfig,
  UpdateCloudflareConfig,
  GetCloudflareIPStats,
  ForceFetchCloudflareIPs,
  TriggerCFHealthCheck,
  RemoveInvalidCFIPs,
  GetLanguage,
  SetLanguage
} from '../api/bindings';
import { toast } from '../lib/toast';
import { useTranslation } from '../i18n/I18nContext';

const SettingItem: React.FC<{
  title: React.ReactNode;
  desc?: React.ReactNode;
  icon?: React.ReactNode;
  children: React.ReactNode;
}> = ({ title, desc, icon, children }) => {
  const hasLeft = title || icon;
  return (
    <div className={`flex items-start gap-5 p-5 bg-background-card border border-border rounded-xl hover:border-accent/40 transition-all group ${hasLeft ? 'justify-between' : 'justify-center'}`}>
      {hasLeft && (
        <div className="flex flex-1 min-w-0 gap-4 items-center">
          <div className="w-10 h-10 rounded-2xl bg-background-hover flex items-center justify-center text-text-secondary group-hover:text-accent transition-colors shrink-0">
            {icon || <Activity size={20} />}
          </div>
          <div className="min-w-0">
            <h4 className="text-sm font-bold leading-snug whitespace-nowrap">{title}</h4>
            {desc && <p className="text-[11px] text-text-muted mt-0.5 leading-relaxed font-medium break-words">{desc}</p>}
          </div>
        </div>
      )}
      <div className={`shrink-0 ${hasLeft ? 'self-center' : ''}`}>
        {children}
      </div>
    </div>
  );
};

const StackedSettingItem: React.FC<{
  title: React.ReactNode;
  desc?: React.ReactNode;
  icon?: React.ReactNode;
  children: React.ReactNode;
}> = ({ title, desc, icon, children }) => (
  <div className="p-5 bg-background-card border border-border rounded-xl hover:border-accent/40 transition-all group">
    <div className="flex items-center gap-4 min-w-0">
      <div className="w-10 h-10 rounded-2xl bg-background-hover flex items-center justify-center text-text-secondary group-hover:text-accent transition-colors shrink-0">
        {icon || <Activity size={20} />}
      </div>
      <div className="min-w-0">
        <h4 className="text-sm font-bold leading-snug">{title}</h4>
        {desc && <p className="text-[11px] text-text-muted mt-0.5 leading-relaxed font-medium break-words">{desc}</p>}
      </div>
    </div>
    <div className="mt-4">
      {children}
    </div>
  </div>
);

interface SettingsProps {
  cache: any;
  onCacheUpdate: (patch: any) => void;
  theme: 'light' | 'dark';
  toggleTheme: () => void;
}

const Settings: React.FC<SettingsProps> = ({ cache, onCacheUpdate, theme, toggleTheme }) => {
  const { t, language, setLanguage: setI18nLanguage } = useTranslation();
  
  // Only keep local state for editable text inputs and toggle optimistic updates
  const [port, setPort] = useState(cache.port);
  const [socks5Port, setSocks5Port] = useState(cache.socks5Port ?? '8081');
  const [closeToTray, setCloseToTray] = useState(cache.closeToTray);
  const [autoStart, setAutoStart] = useState(cache.autoStart);
  const [showMainOnAutoStart, setShowMainOnAutoStart] = useState(cache.showMainOnAutoStart);
  const [autoEnableProxyOnAutoStart, setAutoEnableProxyOnAutoStart] = useState(cache.autoEnableProxyOnAutoStart);

  // Cloudflare Config
  const [cfConfig, setCfConfig] = useState<any>(cache.cfConfig);

  // Pure UI states
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isCheckingHealth, setIsCheckingHealth] = useState(false);
  const [isCertBusy, setIsCertBusy] = useState(false);

  // Read-only display data
  const tunConfig = cache.tunConfig;
  const tunStatus = cache.tunStatus;
  const caStatus = cache.caStatus;
  const installedCerts = cache.installedCerts || [];
  const ipStats = cache.ipStats || [];

  const parseLatencyMs = (latency: unknown) => {
    if (typeof latency === 'number') return latency;
    if (typeof latency !== 'string') return 0;
    const match = latency.match(/^(\d+(?:\.\d+)?)\s*(ns|us|µs|ms|s)?$/i);
    if (!match) return 0;
    const value = parseFloat(match[1]);
    const unit = (match[2] || 'ms').toLowerCase();
    if (unit === 's') return value * 1000;
    if (unit === 'us' || unit === 'µs') return value / 1000;
    if (unit === 'ns') return value / 1000000;
    return value;
  };

  const reloadCriticalData = async () => {
    try {
      const [tunCfg, tunState, cf, ca, certs, stats] = await Promise.all([
        GetTUNConfig(),
        GetTUNStatus(),
        GetCloudflareConfig(),
        GetCAInstallStatus(),
        GetInstalledCerts(),
        GetCloudflareIPStats()
      ]);

      if (cf) setCfConfig(cf);

      onCacheUpdate({
        tunConfig: tunCfg || cache.tunConfig,
        tunStatus: tunState || cache.tunStatus,
        cfConfig: cf || cache.cfConfig,
        caStatus: ca || cache.caStatus,
        installedCerts: certs || cache.installedCerts,
        ipStats: stats || cache.ipStats
      });
    } catch {
      // Silently ignore
    }
  };

  useEffect(() => {
    reloadCriticalData();
    TriggerCFHealthCheck().catch(console.error);
    const ipTimer = setInterval(async () => {
      const stats = await GetCloudflareIPStats();
      onCacheUpdate({ ipStats: stats || [] });
    }, 5000);
    return () => clearInterval(ipTimer);
  }, []);

  const handleSavePort = async () => {
    await SetListenPort(port);
    onCacheUpdate({ port });
    toast.success(t('proxies.notifications.updated'), `${t('settings.http_port')} ${port}`);
  };

  const handleSaveSocks5Port = async (val: string) => {
    const normalized = val.trim();
    if (!normalized) return;
    setSocks5Port(normalized);
    try {
      await SetSocks5Port(normalized);
      onCacheUpdate({ socks5Port: normalized });
      toast.success(t('proxies.notifications.updated'));
    } catch (err: any) {
      toast.error(t('common.failed'), String(err));
    }
  };

  const handleToggleTray = async (val: boolean) => {
    setCloseToTray(val);
    await SetCloseToTray(val);
    onCacheUpdate({ closeToTray: val });
    toast.success(t('proxies.notifications.updated'));
  };

  const handleToggleAutoStart = async (val: boolean) => {
    setAutoStart(val);
    try {
      await SetAutoStart(val);
      onCacheUpdate({ autoStart: val });
      toast.success(t('proxies.notifications.updated'));
    } catch (err: any) {
      setAutoStart(!val);
      toast.error(t('common.failed'), String(err));
    }
  };

  const handleToggleAutoEnableProxyOnAutoStart = async (val: boolean) => {
    setAutoEnableProxyOnAutoStart(val);
    try {
      await SetAutoEnableProxyOnAutoStart(val);
      onCacheUpdate({ autoEnableProxyOnAutoStart: val });
      toast.success(t('proxies.notifications.updated'));
    } catch (err: any) {
      setAutoEnableProxyOnAutoStart(!val);
      toast.error(t('common.failed'), String(err));
    }
  };

  const handleToggleShowMainWindowOnAutoStart = async (val: boolean) => {
    setShowMainOnAutoStart(val);
    try {
      await SetShowMainWindowOnAutoStart(val);
      onCacheUpdate({ showMainOnAutoStart: val });
      toast.success(t('proxies.notifications.updated'));
    } catch (err: any) {
      setShowMainOnAutoStart(!val);
      toast.error(t('common.failed'), String(err));
    }
  };

  const handleLanguageChange = async (lang: string) => {
    await SetLanguage(lang);
    setI18nLanguage(lang as any);
    onCacheUpdate({ language: lang });
    toast.success(t('common.success'));
  };

  const handleFetchIPs = async () => {
    setIsRefreshing(true);
    try {
      await ForceFetchCloudflareIPs();
      await reloadCriticalData();
    } finally {
      setIsRefreshing(false);
    }
  };

  const handleHealthCheck = async () => {
    setIsCheckingHealth(true);
    try {
      await TriggerCFHealthCheck();
      await reloadCriticalData();
      window.setTimeout(() => { void reloadCriticalData(); }, 1200);
      window.setTimeout(() => { void reloadCriticalData(); }, 3000);
      toast.info(t('common.loading'));
    } finally {
      window.setTimeout(() => setIsCheckingHealth(false), 1200);
    }
  };

  const handleRegenerateCert = async () => {
    setIsCertBusy(true);
    try {
      await RegenerateCert();
      await reloadCriticalData();
      toast.success(t('settings.ca_management.reset_success'));
    } catch (err: any) {
      toast.error(t('common.failed'), String(err));
    } finally {
      setIsCertBusy(false);
    }
  };

  const handleUninstallCert = async (token: string) => {
    if (!token) return;
    setIsCertBusy(true);
    try {
      await UninstallCert(token);
      await reloadCriticalData();
      toast.success(t('common.success'));
    } catch (err: any) {
      toast.error(t('common.failed'), String(err));
    } finally {
      setIsCertBusy(false);
    }
  };

  return (
    <div className="p-6 max-w-5xl mx-auto space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-700">
      <header className="flex justify-between items-end">
        <div>
          <h1 className="text-3xl font-black tracking-tighter">{t('settings.title')}</h1>
        </div>
      </header>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        {/* Proxy Base & Startup Section */}
        <div className="space-y-8">
          <section className="space-y-4">
            <div className="flex items-center gap-2 px-1 text-text-secondary">
              <Anchor size={18} />
              <h3 className="text-sm font-bold uppercase tracking-wider">{t('settings.tabs.general')}</h3>
            </div>

            <div className="space-y-4">
              <SettingItem
                title=""
                icon={<Monitor size={20} />}
              >
                <div className="flex flex-col gap-3">
                  <div className="flex items-center gap-2">
                    <span className="text-[10px] text-text-secondary font-bold w-12">{t('settings.http_port')}</span>
                    <input
                      type="number"
                      value={port}
                      onChange={(e) => setPort(parseInt(e.target.value))}
                      className="w-20 bg-background-soft border border-border px-3 py-1.5 rounded-xl text-sm font-bold focus:ring-2 focus:ring-accent outline-none"
                    />
                    <button onClick={handleSavePort} className="px-3 py-1.5 bg-accent/10 text-accent rounded-xl text-[11px] font-bold hover:bg-accent hover:text-white transition-all">{t('common.apply')}</button>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="text-[10px] text-text-secondary font-bold w-12">SOCKS5</span>
                    <input
                      type="text"
                      value={socks5Port}
                      onChange={(e) => setSocks5Port(e.target.value)}
                      onBlur={(e) => handleSaveSocks5Port(e.target.value)}
                      onKeyDown={(e) => { if (e.key === 'Enter') { (e.target as HTMLInputElement).blur(); } }}
                      className="w-20 bg-background-soft border border-border px-3 py-1.5 rounded-xl text-sm font-bold focus:ring-2 focus:ring-accent outline-none"
                      placeholder="8081"
                    />
                  </div>
                </div>
              </SettingItem>

              <SettingItem
                title={t('settings.min_to_tray.title')}
                desc={t('settings.min_to_tray.desc')}
                icon={<BellRing size={20} />}
              >
                <button
                  onClick={() => handleToggleTray(!closeToTray)}
                  className={`w-9 h-5 rounded-full transition-all relative ${closeToTray ? "bg-accent" : "bg-background-hover border border-border"}`}
                >
                  <div className={`absolute top-0.5 w-4 h-4 bg-white rounded-full shadow-sm transition-transform duration-200 ${closeToTray ? "translate-x-[18px] left-0" : "left-0.5"}`} />
                </button>
              </SettingItem>

              <SettingItem
                title={t('settings.language.title')}
                desc={t('settings.language.desc')}
                icon={<Globe size={20} />}
              >
                <div className="flex p-1 bg-background-soft rounded-xl border border-border">
                  <button
                    onClick={() => handleLanguageChange('zh')}
                    className={`px-3 py-1 text-[11px] font-bold rounded-lg transition-all ${language === 'zh' ? 'bg-accent text-white shadow-sm' : 'text-text-secondary hover:text-text-primary'}`}
                  >
                    中文
                  </button>
                  <button
                    onClick={() => handleLanguageChange('en')}
                    className={`px-3 py-1 text-[11px] font-bold rounded-lg transition-all ${language === 'en' ? 'bg-accent text-white shadow-sm' : 'text-text-secondary hover:text-text-primary'}`}
                  >
                    English
                  </button>
                </div>
              </SettingItem>

              <SettingItem
                title={t('settings.appearance.title')}
                desc={t('settings.appearance.desc')}
                icon={theme === 'light' ? <Sun size={20} /> : <Moon size={20} />}
              >
                <div className="flex p-1 bg-background-soft rounded-xl border border-border">
                  <button
                    onClick={() => theme === 'dark' && toggleTheme()}
                    className={`flex items-center gap-2 px-3 py-1.5 text-[11px] font-bold rounded-lg transition-all ${theme === 'light' ? 'bg-white text-accent shadow-sm' : 'text-text-secondary hover:text-text-primary'}`}
                  >
                    <Sun size={14} />
                    {t('settings.appearance.light')}
                  </button>
                  <button
                    onClick={() => theme === 'light' && toggleTheme()}
                    className={`flex items-center gap-2 px-3 py-1.5 text-[11px] font-bold rounded-lg transition-all ${theme === 'dark' ? 'bg-accent text-white shadow-sm' : 'text-text-secondary hover:text-text-primary'}`}
                  >
                    <Moon size={14} />
                    {t('settings.appearance.dark')}
                  </button>
                </div>
              </SettingItem>
            </div>
          </section>

          <section className="space-y-4">
            <div className="flex items-center gap-2 px-1 text-text-secondary">
              <Cpu size={18} />
              <h3 className="text-sm font-bold uppercase tracking-wider">{t('settings.tabs.startup')}</h3>
            </div>

            <div className="space-y-4">
              <SettingItem
                title={t('settings.auto_start.title')}
                desc={t('settings.auto_start.desc')}
                icon={<Cpu size={20} />}
              >
                <button
                  onClick={() => handleToggleAutoStart(!autoStart)}
                  className={`w-9 h-5 rounded-full transition-all relative ${autoStart ? "bg-accent" : "bg-background-hover border border-border"}`}
                >
                  <div className={`absolute top-0.5 w-4 h-4 bg-white rounded-full shadow-sm transition-transform duration-200 ${autoStart ? "translate-x-[18px] left-0" : "left-0.5"}`} />
                </button>
              </SettingItem>

              <SettingItem
                title={t('settings.auto_proxy.title')}
                desc={t('settings.auto_proxy.desc')}
                icon={<Activity size={20} />}
              >
                <button
                  onClick={() => handleToggleAutoEnableProxyOnAutoStart(!autoEnableProxyOnAutoStart)}
                  className={`w-9 h-5 rounded-full transition-all relative ${autoEnableProxyOnAutoStart ? "bg-accent" : "bg-background-hover border border-border"}`}
                >
                  <div className={`absolute top-0.5 w-4 h-4 bg-white rounded-full shadow-sm transition-transform duration-200 ${autoEnableProxyOnAutoStart ? "translate-x-[18px] left-0" : "left-0.5"}`} />
                </button>
              </SettingItem>

              <SettingItem
                title={t('settings.show_main.title')}
                desc={t('settings.show_main.desc')}
                icon={<Monitor size={20} />}
              >
                <button
                  onClick={() => handleToggleShowMainWindowOnAutoStart(!showMainOnAutoStart)}
                  className={`w-9 h-5 rounded-full transition-all relative ${showMainOnAutoStart ? "bg-accent" : "bg-background-hover border border-border"}`}
                >
                  <div className={`absolute top-0.5 w-4 h-4 bg-white rounded-full shadow-sm transition-transform duration-200 ${showMainOnAutoStart ? "translate-x-[18px] left-0" : "left-0.5"}`} />
                </button>
              </SettingItem>
            </div>
          </section>
        </div>

        {/* Security / Certs Section - Now side-by-side with Proxy Base */}
        <section className="space-y-4">
          <div className="flex items-center gap-2 px-1 text-text-secondary">
            <ShieldAlert size={18} />
            <h3 className="text-sm font-bold uppercase tracking-wider">{t('settings.tabs.security')}</h3>
          </div>

          <div className="grid grid-cols-1 gap-4">
            <SettingItem
              title={t('settings.ca_management.reset')}
              desc={t('settings.ca_management.reset_hint')}
              icon={<RefreshCcw size={20} />}
            >
              <button
                onClick={handleRegenerateCert}
                disabled={isCertBusy}
                className="px-4 py-2 border border-border rounded-xl text-xs font-bold hover:bg-background-hover transition-all disabled:opacity-60"
              >
                {isCertBusy ? t('ech_form.probing') : t('common.apply')}
              </button>
            </SettingItem>

            <SettingItem
              title={t('settings.ca_management.export')}
              desc={caStatus?.CertPath || undefined}
              icon={<FolderOpen size={20} />}
            >
              <button onClick={() => OpenCertDir()} className="flex items-center gap-2 px-4 py-2 bg-accent/5 text-accent rounded-xl text-xs font-bold hover:bg-accent/10 transition-all">
                {t('common.view')}
              </button>
            </SettingItem>
          </div>

          <StackedSettingItem
            title={t('settings.ca_management.title')}
            desc={caStatus?.Installed ? t('dashboard.cert_installed') : t('dashboard.cert_not_installed')}
            icon={<ShieldAlert size={20} />}
          >
            <div className="space-y-3">
              <div className={`text-[11px] font-bold ${caStatus?.Installed ? 'text-success' : 'text-text-muted'}`}>
                {caStatus?.Installed ? `${installedCerts.length} CERTS` : t('common.off')}
              </div>
              {installedCerts.length === 0 ? (
                <div className="rounded-xl border border-border/40 bg-background-card px-4 py-5 text-[11px] text-text-muted">
                  {t('proxies.no_ech')}
                </div>
              ) : (
                <div className="space-y-2 max-h-64 overflow-y-auto pr-1">
                  {installedCerts.map((cert: any) => (
                    <div key={cert.token} className="flex items-center justify-between gap-4 rounded-2xl border border-border/40 bg-background-card px-5 py-4">
                      <div className="min-w-0 flex-1 space-y-1">
                        <div className="text-xs font-bold break-all">{cert.subject}</div>
                        <div className="text-[10px] text-text-muted break-all">
                          {cert.storeLocation} / {cert.storeName} / {cert.thumbprint}
                        </div>
                      </div>
                      <button
                        onClick={() => handleUninstallCert(cert.token)}
                        disabled={isCertBusy}
                        className="shrink-0 inline-flex min-w-[92px] items-center justify-center gap-2 rounded-xl bg-danger/12 px-4 py-2 text-[11px] font-black text-danger shadow-[inset_0_0_0_1px_rgba(248,81,73,0.24)] hover:bg-danger/18 disabled:opacity-60"
                      >
                        <Trash2 size={12} />
                        {t('common.delete')}
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </StackedSettingItem>
        </section>

        {/* Cloudflare IP Shaper Section */}
        <section className="lg:col-span-2 space-y-4">
          <div className="flex items-center justify-between px-1 text-text-secondary">
            <div className="flex items-center gap-2">
              <CloudLightning size={18} />
              <h3 className="text-sm font-bold uppercase tracking-wider">{t('rules.form.cf_pool')}</h3>
            </div>
            <div className="flex gap-2">
              <button onClick={handleHealthCheck} className="text-[10px] font-black uppercase text-accent hover:underline disabled:opacity-50" disabled={isCheckingHealth}>
                {isCheckingHealth ? t('ech_form.probing') : t('dns.test')}
              </button>
            </div>
          </div>

          <div className="bg-background-card border border-border rounded-2xl overflow-hidden">
            <div className="grid grid-cols-1 md:grid-cols-5">
              <div className="md:col-span-1 p-6 border-r border-border flex flex-col justify-center items-center">
                <button 
                  onClick={handleFetchIPs} 
                  disabled={isRefreshing} 
                  className="w-full py-3 bg-accent text-white rounded-xl text-xs font-black shadow-lg shadow-accent/20 hover:scale-[1.02] active:scale-[0.98] transition-all flex items-center justify-center gap-2"
                >
                  {isRefreshing ? <RefreshCcw size={14} className="animate-spin" /> : <Download size={14} />}
                  <span>{t('settings.cf_pool.fetch_now')}</span>
                </button>
              </div>

              <div className="md:col-span-4 p-6 bg-background-soft/30">
                <div className="flex items-center justify-between mb-4 px-2">
                  <h4 className="text-[10px] font-black uppercase text-text-muted tracking-widest">IP POOL ({ipStats.length})</h4>
                  <Zap size={14} className="text-warning animate-pulse" />
                </div>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 max-h-[320px] overflow-y-auto px-2 pb-4 scrollbar-thin">
                  {ipStats.length === 0 ? (
                    <div className="col-span-full py-12 flex flex-col items-center justify-center text-text-muted opacity-40">
                      <AlertCircle size={32} />
                      <span className="text-[10px] font-bold uppercase mt-2">{t('rules.form.no_domains')}</span>
                    </div>
                  ) : (
                    ipStats.map((ip: any, i: number) => (
                      <div key={i} className="flex items-center justify-between p-3 bg-background-card border border-border/60 rounded-2xl shadow-sm hover:border-accent/30 transition-all group">
                        <div className="flex items-center gap-3">
                          <div className={`w-2 h-2 rounded-full ${parseLatencyMs(ip.latency) > 0 ? "bg-success shadow-[0_0_8px_rgba(34,197,94,0.5)]" : "bg-danger"}`} />
                          <span className="text-xs font-mono font-bold">{ip.ip}</span>
                        </div>
                        <span className={`text-[10px] font-black ${parseLatencyMs(ip.latency) > 0 && parseLatencyMs(ip.latency) < 200 ? "text-success" : "text-warning"}`}>
                          {ip.latency ? `${Math.round(parseLatencyMs(ip.latency))}ms` : "---"}
                        </span>
                      </div>
                    ))
                  )}
                </div>
              </div>
            </div>
          </div>
        </section>

      </div>
    </div>
  );
};

export default Settings;
