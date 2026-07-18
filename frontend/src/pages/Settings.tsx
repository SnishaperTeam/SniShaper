import React, { useState, useEffect, useCallback } from 'react';
import {
  ShieldAlert,
  Download,
  FolderOpen,
  RefreshCcw,
  Monitor,
  Anchor,
  Cpu,
  Globe,
  BellRing,
  Activity,
  CloudLightning,
  Zap,
  Trash2,
  AlertCircle,
  Sun,
  Moon
} from '../lib/icons';
import {
  GetListenPort, SetListenPort, GetCloseToTray, SetCloseToTray,
  GetAutoStart, SetAutoStart, GetShowMainWindowOnAutoStart, SetShowMainWindowOnAutoStart,
  GetAutoEnableProxyOnAutoStart, SetAutoEnableProxyOnAutoStart,
  GetSocks5Port, SetSocks5Port, GetTUNConfig, UpdateTUNConfig, GetTUNStatus,
  OpenCertDir, RegenerateCert, GetCAInstallStatus, GetInstalledCerts,
  UninstallCert, ExportConfig, ImportConfigWithSummary,
  GetCloudflareConfig, UpdateCloudflareConfig, GetCloudflareIPStats,
  ForceFetchCloudflareIPs, TriggerCFHealthCheck, RemoveInvalidCFIPs,
  GetLanguage, SetLanguage
} from '../api/bindings';
import { SettingRow, StackedSettingRow } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Toggle } from '../components/ui/Toggle';
import { EmptyState } from '../components/ui/EmptyState';
import { toast } from '../lib/toast';
import { parseLatencyMs } from '../lib/utils';
import { useTranslation } from '../i18n/I18nContext';

interface SettingsProps {
  cache: any;
  onCacheUpdate: (patch: any) => void;
  theme: 'light' | 'dark';
  toggleTheme: () => void;
}

const Settings: React.FC<SettingsProps> = ({ cache, onCacheUpdate, theme, toggleTheme }) => {
  const { t, language, setLanguage: setI18nLanguage } = useTranslation();
  const [port, setPort] = useState(cache.port);
  const [socks5Port, setSocks5Port] = useState(cache.socks5Port ?? '8081');
  const [closeToTray, setCloseToTray] = useState(cache.closeToTray);
  const [autoStart, setAutoStart] = useState(cache.autoStart);
  const [showMainOnAutoStart, setShowMainOnAutoStart] = useState(cache.showMainOnAutoStart);
  const [autoEnableProxyOnAutoStart, setAutoEnableProxyOnAutoStart] = useState(cache.autoEnableProxyOnAutoStart);
  const [cfConfig, setCfConfig] = useState<any>(cache.cfConfig);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isCheckingHealth, setIsCheckingHealth] = useState(false);
  const [isCertBusy, setIsCertBusy] = useState(false);
  const tunConfig = cache.tunConfig;
  const tunStatus = cache.tunStatus;
  const caStatus = cache.caStatus;
  const installedCerts = cache.installedCerts || [];
  const [ipStats, setIpStats] = useState<any[]>(cache.ipStats || []);

  const reloadCriticalData = useCallback(async () => {
    try {
      const [tunCfg, tunState, cf, ca, certs, stats] = await Promise.all([
        GetTUNConfig(), GetTUNStatus(), GetCloudflareConfig(),
        GetCAInstallStatus(), GetInstalledCerts(), GetCloudflareIPStats()
      ]);
      if (cf) setCfConfig(cf);
      if (stats) setIpStats(stats);
      onCacheUpdate({
        tunConfig: tunCfg || cache.tunConfig, tunStatus: tunState || cache.tunStatus,
        cfConfig: cf || cache.cfConfig, caStatus: ca || cache.caStatus,
        installedCerts: certs || cache.installedCerts
      });
    } catch { /* ignore */ }
  }, [cache, onCacheUpdate]);

  useEffect(() => {
    reloadCriticalData();
    TriggerCFHealthCheck().catch(console.error);
    const ipTimer = setInterval(async () => {
      const stats = await GetCloudflareIPStats();
      if (stats) setIpStats(stats);
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
    } catch (err: any) { toast.error(t('common.failed'), String(err)); }
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
    } catch (err: any) { setAutoStart(!val); toast.error(t('common.failed'), String(err)); }
  };

  const handleToggleAutoEnableProxyOnAutoStart = async (val: boolean) => {
    setAutoEnableProxyOnAutoStart(val);
    try {
      await SetAutoEnableProxyOnAutoStart(val);
      onCacheUpdate({ autoEnableProxyOnAutoStart: val });
      toast.success(t('proxies.notifications.updated'));
    } catch (err: any) { setAutoEnableProxyOnAutoStart(!val); toast.error(t('common.failed'), String(err)); }
  };

  const handleToggleShowMainWindowOnAutoStart = async (val: boolean) => {
    setShowMainOnAutoStart(val);
    try {
      await SetShowMainWindowOnAutoStart(val);
      onCacheUpdate({ showMainOnAutoStart: val });
      toast.success(t('proxies.notifications.updated'));
    } catch (err: any) { setShowMainOnAutoStart(!val); toast.error(t('common.failed'), String(err)); }
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
      toast.success(t('settings.cf_pool.fetch_now'));
    } catch (err: any) { toast.error(t('common.failed'), String(err?.message || err));
    } finally { setIsRefreshing(false); }
  };

  const handleHealthCheck = async () => {
    setIsCheckingHealth(true);
    try {
      await TriggerCFHealthCheck();
      await reloadCriticalData();
      window.setTimeout(() => { void reloadCriticalData(); }, 1200);
      window.setTimeout(() => { void reloadCriticalData(); }, 3000);
      toast.info(t('common.loading'));
    } finally { window.setTimeout(() => setIsCheckingHealth(false), 1200); }
  };

  const handleRegenerateCert = async () => {
    setIsCertBusy(true);
    try {
      await RegenerateCert();
      await reloadCriticalData();
      toast.success(t('settings.ca_management.reset_success'));
    } catch (err: any) { toast.error(t('common.failed'), String(err));
    } finally { setIsCertBusy(false); }
  };

  const handleUninstallCert = async (token: string) => {
    if (!token) return;
    setIsCertBusy(true);
    try {
      await UninstallCert(token);
      await reloadCriticalData();
      toast.success(t('common.success'));
    } catch (err: any) { toast.error(t('common.failed'), String(err));
    } finally { setIsCertBusy(false); }
  };

  return (
    <div className="p-6 max-w-5xl mx-auto space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-700">
      <header>
        <h1 className="text-3xl font-black tracking-tighter">{t('settings.title')}</h1>
      </header>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        <div className="space-y-8">
          <section className="space-y-4">
            <div className="flex items-center gap-2 px-1 text-text-secondary">
              <Anchor size={18} aria-hidden />
              <h2 className="text-sm font-bold uppercase tracking-wider">{t('settings.tabs.general')}</h2>
            </div>

            <div className="space-y-4">
              <SettingRow icon={<Monitor size={20} />} title={t('settings.port_title')}>
                <div className="flex flex-col gap-3">
                  <div className="flex items-center gap-2">
                    <label htmlFor="http-port" className="text-[10px] text-text-secondary font-bold w-12">{t('settings.http_port')}</label>
                    <input
                      id="http-port"
                      type="number"
                      value={port}
                      onChange={(e) => setPort(parseInt(e.target.value))}
                      className="w-20 bg-background-soft border border-border px-3 py-1.5 rounded-xl text-sm font-bold focus:ring-2 focus:ring-accent outline-none"
                    />
                    <Button onClick={handleSavePort} size="xs">{t('common.apply')}</Button>
                  </div>
                  <div className="flex items-center gap-2">
                    <label htmlFor="socks5-port" className="text-[10px] text-text-secondary font-bold w-12">{t('settings.socks_port')}</label>
                    <input
                      id="socks5-port"
                      type="text"
                      value={socks5Port}
                      onChange={(e) => setSocks5Port(e.target.value)}
                      onBlur={(e) => handleSaveSocks5Port(e.target.value)}
                      onKeyDown={(e) => { if (e.key === 'Enter') (e.target as HTMLInputElement).blur(); }}
                      className="w-20 bg-background-soft border border-border px-3 py-1.5 rounded-xl text-sm font-bold focus:ring-2 focus:ring-accent outline-none"
                      placeholder="8081"
                    />
                  </div>
                </div>
              </SettingRow>

              <SettingRow title={t('settings.min_to_tray.title')} desc={t('settings.min_to_tray.desc')} icon={<BellRing size={20} />}>
                <Toggle checked={closeToTray} onChange={handleToggleTray} />
              </SettingRow>

              <SettingRow title={t('settings.language.title')} desc={t('settings.language.desc')} icon={<Globe size={20} />}>
                <div className="flex p-1 bg-background-soft rounded-xl border border-border" role="radiogroup" aria-label="选择语言">
                  {(['zh', 'en', 'ru'] as const).map((lang) => (
                    <button
                      key={lang}
                      onClick={() => handleLanguageChange(lang)}
                      role="radio"
                      aria-checked={language === lang}
                      className={`px-3 py-1 text-[11px] font-bold rounded-lg transition-all ${language === lang ? 'bg-accent text-white shadow-sm' : 'text-text-secondary hover:text-text-primary'}`}
                    >
                      {lang === 'zh' ? '中文' : lang === 'en' ? 'English' : 'Русский'}
                    </button>
                  ))}
                </div>
              </SettingRow>

              <SettingRow title={t('settings.appearance.title')} desc={t('settings.appearance.desc')} icon={theme === 'light' ? <Sun size={20} /> : <Moon size={20} />}>
                <div className="flex p-1 bg-background-soft rounded-xl border border-border" role="radiogroup" aria-label="选择主题">
                  <button
                    onClick={() => theme === 'dark' && toggleTheme()}
                    role="radio"
                    aria-checked={theme === 'light'}
                    className={`flex items-center gap-2 px-3 py-1.5 text-[11px] font-bold rounded-lg transition-all ${theme === 'light' ? 'bg-white text-accent shadow-sm' : 'text-text-secondary hover:text-text-primary'}`}
                  >
                    <Sun size={14} aria-hidden />
                    {t('settings.appearance.light')}
                  </button>
                  <button
                    onClick={() => theme === 'light' && toggleTheme()}
                    role="radio"
                    aria-checked={theme === 'dark'}
                    className={`flex items-center gap-2 px-3 py-1.5 text-[11px] font-bold rounded-lg transition-all ${theme === 'dark' ? 'bg-accent text-white shadow-sm' : 'text-text-secondary hover:text-text-primary'}`}
                  >
                    <Moon size={14} aria-hidden />
                    {t('settings.appearance.dark')}
                  </button>
                </div>
              </SettingRow>
            </div>
          </section>

          <section className="space-y-4">
            <div className="flex items-center gap-2 px-1 text-text-secondary">
              <Cpu size={18} aria-hidden />
              <h2 className="text-sm font-bold uppercase tracking-wider">{t('settings.tabs.startup')}</h2>
            </div>
            <div className="space-y-4">
              <SettingRow title={t('settings.auto_start.title')} desc={t('settings.auto_start.desc')} icon={<Cpu size={20} />}>
                <Toggle checked={autoStart} onChange={handleToggleAutoStart} />
              </SettingRow>
              <SettingRow title={t('settings.auto_proxy.title')} desc={t('settings.auto_proxy.desc')} icon={<Activity size={20} />}>
                <Toggle checked={autoEnableProxyOnAutoStart} onChange={handleToggleAutoEnableProxyOnAutoStart} />
              </SettingRow>
              <SettingRow title={t('settings.show_main.title')} desc={t('settings.show_main.desc')} icon={<Monitor size={20} />}>
                <Toggle checked={showMainOnAutoStart} onChange={handleToggleShowMainWindowOnAutoStart} />
              </SettingRow>
            </div>
          </section>
        </div>

        <section className="space-y-4">
          <div className="flex items-center gap-2 px-1 text-text-secondary">
            <ShieldAlert size={18} aria-hidden />
            <h2 className="text-sm font-bold uppercase tracking-wider">{t('settings.tabs.security')}</h2>
          </div>
          <div className="grid grid-cols-1 gap-4">
            <SettingRow title={t('settings.ca_management.reset')} desc={t('settings.ca_management.reset_hint')} icon={<RefreshCcw size={20} />}>
              <Button onClick={handleRegenerateCert} loading={isCertBusy} variant="outline" size="sm">
                {isCertBusy ? t('ech_form.probing') : t('common.apply')}
              </Button>
            </SettingRow>
            <SettingRow title={t('settings.ca_management.export')} desc={caStatus?.CertPath || undefined} icon={<FolderOpen size={20} />}>
              <Button onClick={() => OpenCertDir()} variant="ghost" size="sm" icon={<FolderOpen size={14} />}>
                {t('common.view')}
              </Button>
            </SettingRow>
          </div>

          <StackedSettingRow title={t('settings.ca_management.title')} desc={caStatus?.Installed ? t('dashboard.cert_installed') : t('dashboard.cert_not_installed')} icon={<ShieldAlert size={20} />}>
            <div className="space-y-3">
              <div className={`text-[11px] font-bold ${caStatus?.Installed ? 'text-success' : 'text-text-muted'}`}>
                {caStatus?.Installed ? `${installedCerts.length} CERTS` : t('common.off')}
              </div>
              {installedCerts.length === 0 ? (
                <EmptyState icon={<ShieldAlert size={32} />} title={t('proxies.no_ech')} />
              ) : (
                <div className="space-y-2 max-h-64 overflow-y-auto pr-1">
                  {installedCerts.map((cert: any) => (
                    <div key={cert.token} className="flex items-center justify-between gap-4 rounded-2xl border border-border/40 bg-background-card px-5 py-4">
                      <div className="min-w-0 flex-1 space-y-1">
                        <div className="text-xs font-bold break-all">{cert.subject}</div>
                        <div className="text-[10px] text-text-muted break-all">{cert.storeLocation} / {cert.storeName} / {cert.thumbprint}</div>
                      </div>
                      <Button onClick={() => handleUninstallCert(cert.token)} disabled={isCertBusy} variant="danger" size="sm" icon={<Trash2 size={12} />}>
                        {t('common.delete')}
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </StackedSettingRow>
        </section>

        <section className="lg:col-span-2 space-y-4">
          <div className="flex items-center justify-between px-1 text-text-secondary">
            <div className="flex items-center gap-2">
              <CloudLightning size={18} aria-hidden />
              <h2 className="text-sm font-bold uppercase tracking-wider">{t('rules.form.cf_pool')}</h2>
            </div>
            <Button onClick={handleHealthCheck} loading={isCheckingHealth} variant="ghost" size="xs" disabled={isCheckingHealth}>
              {isCheckingHealth ? t('ech_form.probing') : t('dns.test')}
            </Button>
          </div>

          <div className="bg-background-card border border-border rounded-2xl overflow-hidden">
            <div className="grid grid-cols-1 md:grid-cols-5">
              <div className="md:col-span-1 p-6 border-r border-border flex flex-col justify-center items-center">
                <Button onClick={handleFetchIPs} loading={isRefreshing} icon={isRefreshing ? undefined : <Download size={14} />} className="w-full">
                  {t('settings.cf_pool.fetch_now')}
                </Button>
              </div>
              <div className="md:col-span-4 p-6 bg-background-soft/30">
                <div className="flex items-center justify-between mb-4 px-2">
                  <h3 className="text-[10px] font-black uppercase text-text-muted tracking-widest">IP POOL ({ipStats.length})</h3>
                  <Zap size={14} className="text-warning animate-pulse" aria-hidden />
                </div>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 max-h-[320px] overflow-y-auto px-2 pb-4 scrollbar-thin">
                  {ipStats.length === 0 ? (
                    <EmptyState icon={<AlertCircle size={32} />} title={t('rules.form.no_domains')} className="col-span-full" />
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
