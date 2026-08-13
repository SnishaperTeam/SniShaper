import React, { useState, useEffect, useCallback } from 'react';
import {
  ShieldAlert, Download, FolderOpen, RefreshCcw, Monitor, Anchor,
  Cpu, Globe, BellRing, Activity, CloudLightning, Zap, Trash2,
  AlertCircle, Sun, Moon, Wifi, FileText, Settings as SettingsIcon
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
  GetLanguage, SetLanguage,
  GetIPv6Available, RefreshIPv6Check,
  GetLogFiles, OpenLogFile, CleanOldLogs,
  GetUpdateChannel, SetUpdateChannel,
  GetDownloadSource, SetDownloadSource,
  GetCustomDownloadSource, SetCustomDownloadSource,
  MeasureDownloadSources
} from '../api/bindings';
import {
  Box, Button, TextField, Select, MenuItem, FormControl, InputLabel, Switch,
  FormControlLabel, Tooltip, Typography, Stack, Grid, Badge, Divider, CircularProgress,
} from '@mui/material';
import { alpha } from '@mui/material/styles';
import { toast } from '../lib/toast';
import { parseLatencyMs } from '../lib/utils';
import { useTranslation } from '../i18n/I18nContext';
import { useColorScheme } from '@mui/material';
import { availableThemes } from '../theme';

interface SettingsProps {
  cache: any;
  onCacheUpdate: (patch: any) => void;
  currentThemeId: string;
  onThemeChange: (id: string) => void;
}

const SectionHeader = ({ icon, label, action }: { icon: React.ReactNode; label: string; action?: React.ReactNode }) => (
  <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', px: 1 }}>
    <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
      {icon}
      <Typography variant="body2" sx={{ fontWeight: 'bold', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
        {label}
      </Typography>
    </Stack>
    {action}
  </Box>
);

const SettingRowInline = ({ icon, title, desc, children }: { icon: React.ReactNode; title: string; desc?: string; children: React.ReactNode }) => (
  <Box sx={{ p: 2.5, bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2, display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 3 }}>
    <Stack direction="row" spacing={1.5} sx={{ flex: 1, minWidth: 0 }}>
      <Box sx={{ width: 40, height: 40, borderRadius: 1, bgcolor: 'primary.main', color: 'primary.contrastText', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
        {icon}
      </Box>
      <Box sx={{ minWidth: 0, py: 0.25 }}>
        <Typography variant="body2" sx={{ fontWeight: 'bold', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '100%' }}>{title}</Typography>
        {desc && <Typography variant="caption" color="text.secondary" sx={{ display: 'block' }}>{desc}</Typography>}
      </Box>
    </Stack>
    <Box sx={{ flexShrink: 0, outline: '1px solid transparent' }}>
      {children}
    </Box>
  </Box>
);

const StackedRow = ({ icon, title, desc, children }: { icon: React.ReactNode; title: string; desc?: string; children: React.ReactNode }) => (
  <Box sx={{ p: 2.5, bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2 }}>
    <Stack direction="row" spacing={1.5} sx={{ mb: desc ? 1.5 : 0, alignItems: 'center' }}>
      <Box sx={{ width: 40, height: 40, borderRadius: 1, bgcolor: 'primary.main', color: 'primary.contrastText', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
        {icon}
      </Box>
      <Box>
        <Typography variant="body2" sx={{ fontWeight: 'bold', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '100%' }}>{title}</Typography>
        {desc && <Typography variant="caption" color="text.secondary" sx={{ display: 'block' }}>{desc}</Typography>}
      </Box>
    </Stack>
    {children}
  </Box>
);

const UPDATE_CHANNELS = [
  { value: 'alpha', label: 'Alpha' },
  { value: 'beta', label: 'Beta' },
  { value: 'rc', label: 'Release Candidate' },
  { value: 'stable', label: 'Stable' },
];

const DOWNLOAD_SOURCES = [
  { value: 'direct', labelKey: 'settings.download_source.direct' },
  { value: 'down.mxw.qzz.io', label: 'down.mxw.qzz.io' },
  { value: 'gh-proxy.org', label: 'gh-proxy.org' },
  { value: 'v4.gh-proxy.org', label: 'v4.gh-proxy.org' },
  { value: 'v6.gh-proxy.org', label: 'v6.gh-proxy.org' },
  { value: 'cdn.gh-proxy.org', label: 'cdn.gh-proxy.org' },
  { value: 'axisnow.gh-proxy.org', label: 'axisnow.gh-proxy.org' },
  { value: 'custom', labelKey: 'settings.download_source.custom' },
];

const Settings: React.FC<SettingsProps> = ({ cache, onCacheUpdate, currentThemeId, onThemeChange }) => {
  const { t, language, setLanguage: setI18nLanguage } = useTranslation();
  const { mode, setMode } = useColorScheme();
  const [port, setPort] = useState(String(cache.port ?? ''));
  const [socks5Port, setSocks5Port] = useState(String(cache.socks5Port ?? '8081'));
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
  const [isIpv6Checking, setIsIpv6Checking] = useState(false);
  const ipv6Available = cache.ipv6Available !== false;
  const [updateChannel, setUpdateChannel] = useState<string>(cache.updateChannel || 'stable');
  const [downloadSource, setDownloadSource] = useState<string>(cache.downloadSource || 'down.mxw.qzz.io');
  const [customSource, setCustomSource] = useState<string>(cache.customDownloadSource || '');
  const [sourceResults, setSourceResults] = useState<any[]>([]);
  const [measuring, setMeasuring] = useState<boolean>(false);

  const measureSources = async () => {
    if (measuring) return;
    setMeasuring(true);
    try {
      const results = await MeasureDownloadSources();
      setSourceResults(Array.isArray(results) ? results : []);
    } catch (err: any) {
      toast.error(t('common.failed'), String(err));
    } finally {
      setMeasuring(false);
    }
  };

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
    } catch {
      /* ignore */
    }
  }, [cache, onCacheUpdate]);

  useEffect(() => {
    reloadCriticalData();
    TriggerCFHealthCheck().catch(console.error);
    GetUpdateChannel().then((c) => {
      if (c) {
        setUpdateChannel(c);
        onCacheUpdate({ updateChannel: c });
      }
    }).catch(() => {});
    GetDownloadSource().then((s) => {
      if (s) {
        setDownloadSource(s);
        onCacheUpdate({ downloadSource: s });
      }
    }).catch(() => {});
    GetCustomDownloadSource().then((s) => {
      if (s) {
        setCustomSource(s);
        onCacheUpdate({ customDownloadSource: s });
      }
    }).catch(() => {});
    const ipTimer = setInterval(async () => {
      const stats = await GetCloudflareIPStats();
      if (stats) setIpStats(stats);
    }, 5000);
    return () => clearInterval(ipTimer);
  }, []);

  const handleChannelChange = async (value: string) => {
    const prev = updateChannel;
    setUpdateChannel(value);
    try {
      await SetUpdateChannel(value);
      onCacheUpdate({ updateChannel: value });
      toast.success(t('settings.notifications.updated'));
    } catch (err: any) {
      setUpdateChannel(prev);
      toast.error(t('common.failed'), String(err));
    }
  };

  const handleDownloadSourceChange = async (value: string) => {
    const prev = downloadSource;
    setDownloadSource(value);
    try {
      await SetDownloadSource(value);
      onCacheUpdate({ downloadSource: value });
      toast.success(t('settings.notifications.updated'));
    } catch (err: any) {
      setDownloadSource(prev);
      toast.error(t('common.failed'), String(err));
    }
  };

  const handleCustomSourceSave = async () => {
    const prev = customSource;
    try {
      await SetCustomDownloadSource(customSource.trim());
      onCacheUpdate({ customDownloadSource: customSource.trim() });
      toast.success(t('settings.notifications.updated'));
    } catch (err: any) {
      setCustomSource(prev);
      toast.error(t('common.failed'), String(err));
    }
  };

  const handleSavePort = async () => {
    const parsed = parseInt(port, 10);
    if (!port || Number.isNaN(parsed) || parsed < 1 || parsed > 65535) {
      setPort(String(cache.port ?? ''));
      toast.error(t('common.failed'), t('settings.notifications.port_invalid'));
      return;
    }
    await SetListenPort(parsed);
    onCacheUpdate({ port: parsed });
    toast.success(t('settings.notifications.updated'), `${t('settings.http_port')} ${parsed}`);
  };

  const handleSaveSocks5Port = async (val: string) => {
    const normalized = val.trim();
    const parsed = parseInt(normalized, 10);
    if (!normalized || Number.isNaN(parsed) || parsed < 1 || parsed > 65535) {
      setSocks5Port(String(cache.socks5Port ?? '8081'));
      toast.error(t('common.failed'), t('settings.notifications.port_invalid'));
      return;
    }
    setSocks5Port(normalized);
    try {
      await SetSocks5Port(normalized);
      onCacheUpdate({ socks5Port: normalized });
      toast.success(t('settings.notifications.updated'));
    } catch (err: any) { toast.error(t('common.failed'), String(err)); }
  };

  const handleToggleTray = async (val: boolean) => {
    setCloseToTray(val);
    await SetCloseToTray(val);
    onCacheUpdate({ closeToTray: val });
    toast.success(t('settings.notifications.updated'));
  };

  const handleToggleAutoStart = async (val: boolean) => {
    setAutoStart(val);
    try {
      await SetAutoStart(val);
      onCacheUpdate({ autoStart: val });
      toast.success(t('settings.notifications.updated'));
    } catch (err: any) { setAutoStart(!val); toast.error(t('common.failed'), String(err)); }
  };

  const handleToggleAutoEnableProxyOnAutoStart = async (val: boolean) => {
    setAutoEnableProxyOnAutoStart(val);
    try {
      await SetAutoEnableProxyOnAutoStart(val);
      onCacheUpdate({ autoEnableProxyOnAutoStart: val });
      toast.success(t('settings.notifications.updated'));
    } catch (err: any) { setAutoEnableProxyOnAutoStart(!val); toast.error(t('common.failed'), String(err)); }
  };

  const handleToggleShowMainWindowOnAutoStart = async (val: boolean) => {
    setShowMainOnAutoStart(val);
    try {
      await SetShowMainWindowOnAutoStart(val);
      onCacheUpdate({ showMainOnAutoStart: val });
      toast.success(t('settings.notifications.updated'));
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

  const handleRefreshIPv6 = async () => {
    setIsIpv6Checking(true);
    try {
      const available = await RefreshIPv6Check();
      onCacheUpdate({ ipv6Available: available === true });
      toast[available ? 'success' : 'error'](available ? t('network.ipv6_ok') : t('network.ipv6_disabled_title'), t('network.ipv6_ok_desc'));
    } catch (err: any) { toast.error(t('common.failed'), String(err));
    } finally { setIsIpv6Checking(false); }
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

  const [logFiles, setLogFiles] = useState<any[]>([]);
  const [isCleaningLogs, setIsCleaningLogs] = useState(false);

  const loadLogFiles = useCallback(async () => {
    try {
      const files = await GetLogFiles();
      setLogFiles(files || []);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => { void loadLogFiles(); }, [loadLogFiles]);

  const handleViewLog = (name: string) => { OpenLogFile(name); };

  const handleCleanLogs = async () => {
    setIsCleaningLogs(true);
    try {
      const removed = await CleanOldLogs();
      await loadLogFiles();
      toast.success(t('settings.logs.clean_done', { count: removed ?? 0 }));
    } catch (err: any) { toast.error(t('common.failed'), String(err));
    } finally { setIsCleaningLogs(false); }
  };

  const fmtLogDate = (name: string) => name.replace(/\.log$/i, '').replace('_', ' ');
  const fmtLogSize = (bytes: number) =>
    bytes >= 1024 * 1024 ? `${(bytes / 1024 / 1024).toFixed(1)} MB`
      : bytes >= 1024 ? `${(bytes / 1024).toFixed(0)} KB`
        : `${bytes} B`;

  const bgColor = 'action.hover';

  const channelKey = 'settings.update_channel.' + updateChannel;
  const channelDesc = t(channelKey) === channelKey ? t('settings.update_channel.stable') : t(channelKey);

  return (
    <Box sx={{ pt: 4, pb: 6, width: '100%', display: 'flex', flexDirection: 'column' }}>
      <Box sx={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'space-between', alignItems: { xs: 'flex-start', sm: 'flex-end' }, gap: 2, mb: 6 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
          <Box sx={{ p: 1.25, borderRadius: 1.5, border: 1, color: 'primary.main', bgcolor: (theme) => alpha(theme.palette.primary.main, 0.1), borderColor: (theme) => alpha(theme.palette.primary.main, 0.1), display: 'flex' }}>
            <SettingsIcon size={20} />
          </Box>
          <Typography variant="h5" sx={{ fontWeight: 700, letterSpacing: '-0.02em' }}>
            {t('settings.title')}
          </Typography>
        </Box>
      </Box>

      <Grid container spacing={3}>
        <Grid size={{ xs: 12, lg: 6 }}>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
            <SectionHeader icon={<Anchor size={18} />} label={t('settings.tabs.general')} />

            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
              <SettingRowInline icon={<Monitor size={18} />} title={t('settings.port_title')}>
                <Stack direction="row" spacing={1.5} sx={{ alignItems: 'flex-start' }}>
                  <Stack direction="row" spacing={1} sx={{ alignItems: 'flex-start' }}>
                    <TextField
                      label={t('settings.http_port')}
                      type="text"
                      size="small"
                      value={port}
                      onChange={(e) => setPort(e.target.value.replace(/\D/g, '').slice(0, 5))}
                      sx={{ width: 88, '& input': { fontSize: '0.875rem', textAlign: 'center' } }}
                    />
                    <Button size="small" variant="contained" color="primary" onClick={handleSavePort}>
                      {t('common.apply')}
                    </Button>
                  </Stack>
                  <Stack direction="row" spacing={1} sx={{ alignItems: 'flex-start' }}>
                    <TextField
                      label={t('settings.socks_port')}
                      type="text"
                      size="small"
                      value={socks5Port}
                      onChange={(e) => setSocks5Port(e.target.value.replace(/\D/g, '').slice(0, 5))}
                      onBlur={(e) => handleSaveSocks5Port(e.target.value)}
                      onKeyDown={(e) => { if (e.key === 'Enter') (e.target as HTMLInputElement).blur(); }}
                      sx={{ width: 88, '& input': { fontSize: '0.875rem', textAlign: 'center' } }}
                    />
                    <Button size="small" variant="contained" color="primary" onClick={() => handleSaveSocks5Port(socks5Port)}>
                      {t('common.apply')}
                    </Button>
                  </Stack>
                </Stack>
              </SettingRowInline>

              <SettingRowInline title={t('settings.min_to_tray.title')} desc={t('settings.min_to_tray.desc')} icon={<BellRing size={18} />}>
                <FormControlLabel
                  control={
                    <Switch checked={closeToTray} onChange={(e) => handleToggleTray(e.target.checked)} />
                  }
                  label={t('settings.min_to_tray.title')}
                  sx={{ gap: 1, marginLeft: 0, marginRight: 0 }}
                />
              </SettingRowInline>

              <SettingRowInline title={t('settings.language.title')} desc={t('settings.language.desc')} icon={<Globe size={18} />}>
                <Stack direction="row" spacing={0.5} sx={{ alignItems: 'center', bgcolor: bgColor, border: 1, borderColor: 'divider', borderRadius: 2, p: 0.5 }} role="radiogroup" aria-label="选择语言">
                  {(['zh', 'en', 'ru'] as const).map((lang) => (
                    <Box
                      key={lang}
                      component="button"
                      type="button"
                      sx={{
                        px: 1.5,
                        py: 0.5,
                        fontSize: '0.75rem',
                        fontWeight: 'bold',
                        borderRadius: 1,
                        border: 'none',
                        cursor: 'pointer',
                        color: language === lang ? '#fff' : 'text.secondary',
                        bgcolor: language === lang ? 'primary.main' : 'transparent',
                        '&:hover': { color: 'text.primary' },
                      }}
                      onClick={() => handleLanguageChange(lang)}
                      role="radio"
                      aria-checked={language === lang}
                    >
                      {lang === 'zh' ? '中文' : lang === 'en' ? 'English' : 'Русский'}
                    </Box>
                  ))}
                </Stack>
              </SettingRowInline>

              <SettingRowInline title={t('settings.appearance.title')} desc={t('settings.appearance.desc')} icon={mode === 'light' ? <Sun size={18} /> : <Moon size={18} />}>
                <Stack direction="row" spacing={0.5} sx={{ alignItems: 'center', bgcolor: bgColor, border: 1, borderColor: 'divider', borderRadius: 2, p: 0.5 }} role="radiogroup" aria-label={t('settings.select_theme')}>
                  <Box
                    component="button"
                    type="button"
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 0.25,
                      px: 1.5,
                      py: 0.75,
                      fontSize: '0.75rem',
                      fontWeight: 'bold',
                      borderRadius: 1,
                      border: 'none',
                      cursor: 'pointer',
                      color: mode === 'light' ? '#fff' : 'text.secondary',
                      bgcolor: mode === 'light' ? 'primary.main' : 'transparent',
                      '&:hover': { color: 'text.primary' },
                    }}
                    onClick={() => mode === 'dark' && setMode('light')}
                    role="radio"
                    aria-checked={mode === 'light'}
                  >
                    <Sun size={14} />
                    {t('settings.appearance.light')}
                  </Box>
                  <Box
                    component="button"
                    type="button"
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 0.25,
                      px: 1.5,
                      py: 0.75,
                      fontSize: '0.75rem',
                      fontWeight: 'bold',
                      borderRadius: 1,
                      border: 'none',
                      cursor: 'pointer',
                      color: mode === 'dark' ? '#fff' : 'text.secondary',
                      bgcolor: mode === 'dark' ? 'primary.main' : 'transparent',
                      '&:hover': { color: 'text.primary' },
                    }}
                    onClick={() => mode === 'light' && setMode('dark')}
                    role="radio"
                    aria-checked={mode === 'dark'}
                  >
                    <Moon size={14} />
                    {t('settings.appearance.dark')}
                  </Box>
                </Stack>
              </SettingRowInline>

              <SettingRowInline title={t('settings.theme.label')} desc={t('settings.theme.desc')} icon={<Monitor size={18} />}>
                <Box sx={{ width: 220, flexShrink: 0 }}>
                  <FormControl fullWidth size="small">
                    <InputLabel id="theme-select-label">{t('settings.theme.label')}</InputLabel>
                    <Select
                      labelId="theme-select-label"
                      id="theme-select"
                      value={currentThemeId}
                      label={t('settings.theme.label')}
                      onChange={(e) => onThemeChange(e.target.value)}
                    >
                      {availableThemes.map((th) => (
                        <MenuItem key={th.id} value={th.id}>
                          {t(th.nameKey)}
                        </MenuItem>
                      ))}
                    </Select>
                  </FormControl>
                </Box>
              </SettingRowInline>

              <SettingRowInline title={t('settings.update_channel.title')} desc={channelDesc} icon={<Download size={18} />}>
                <Box sx={{ width: 240, flexShrink: 0 }}>
                  <FormControl fullWidth size="small">
                    <InputLabel id="update-channel-label">{t('settings.update_channel.title')}</InputLabel>
                    <Select
                      labelId="update-channel-label"
                      id="update-channel-select"
                      value={updateChannel}
                      label={t('settings.update_channel.title')}
                      onChange={(e) => handleChannelChange(e.target.value)}
                    >
                      {UPDATE_CHANNELS.map((ch) => (
                        <MenuItem key={ch.value} value={ch.value}>
                          {ch.label}
                        </MenuItem>
                      ))}
                    </Select>
                  </FormControl>
                </Box>
              </SettingRowInline>

              <SettingRowInline title={t('settings.download_source.title')} desc={t('settings.download_source.desc')} icon={<Globe size={18} />}>
                <Box sx={{ width: 320, flexShrink: 0 }}>
                  <FormControl fullWidth size="small">
                    <InputLabel id="download-source-label">{t('settings.download_source.title')}</InputLabel>
                    <Select
                      labelId="download-source-label"
                      id="download-source-select"
                      value={downloadSource}
                      label={t('settings.download_source.title')}
                      onChange={(e) => handleDownloadSourceChange(e.target.value)}
                    >
                      {DOWNLOAD_SOURCES.map((src) => {
                        let label = src.value;
                        if (src.label) {
                          label = src.label;
                        } else if (src.labelKey) {
                          const translated = t(src.labelKey);
                          if (translated !== src.labelKey) label = translated;
                        }
                        return (
                          <MenuItem key={src.value} value={src.value}>
                            {label}
                          </MenuItem>
                        );
                      })}
                    </Select>
                  </FormControl>
                  {downloadSource === 'custom' && (
                    <TextField
                      size="small"
                      fullWidth
                      sx={{ mt: 1 }}
                      placeholder={t('settings.download_source.custom_placeholder')}
                      value={customSource}
                      onChange={(e) => setCustomSource(e.target.value)}
                      onBlur={handleCustomSourceSave}
                    />
                  )}
                  <Button size="small" sx={{ mt: 1 }} startIcon={<Activity size={14} />} onClick={measureSources} disabled={measuring}>
                    {measuring ? t('settings.download_source.measuring') : t('settings.download_source.measure')}
                  </Button>
                  {sourceResults.length > 0 && (
                    <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5, mt: 1 }}>
                      {sourceResults.map((r) => (
                        <Box
                          key={r.name}
                          component="span"
                          sx={{
                            px: 0.75, py: 0.25, borderRadius: '999px', fontSize: '0.6875rem',
                            bgcolor: (theme) => alpha(theme.palette[r.ok ? 'success' : 'error'].main, 0.12),
                            color: r.ok ? 'success.main' : 'error.main',
                          }}
                        >
                          {r.name} {r.ok ? `${r.latency_ms}ms` : t('settings.download_source.unreachable')}
                        </Box>
                      ))}
                    </Box>
                  )}
                </Box>
              </SettingRowInline>
            </Box>
          </Box>

          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, mt: 1.5 }}>
            <SectionHeader icon={<Cpu size={18} />} label={t('settings.tabs.startup')} />

            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
              <SettingRowInline title={t('settings.auto_start.title')} desc={t('settings.auto_start.desc')} icon={<Cpu size={18} />}>
                <FormControlLabel
                  control={<Switch checked={autoStart} onChange={(e) => handleToggleAutoStart(e.target.checked)} />}
                  label={t('settings.auto_start.title')}
                  sx={{ gap: 1, marginLeft: 0, marginRight: 0 }}
                />
              </SettingRowInline>
              <SettingRowInline title={t('settings.auto_proxy.title')} desc={t('settings.auto_proxy.desc')} icon={<Activity size={18} />}>
                <FormControlLabel
                  control={<Switch checked={autoEnableProxyOnAutoStart} onChange={(e) => handleToggleAutoEnableProxyOnAutoStart(e.target.checked)} />}
                  label={t('settings.auto_proxy.title')}
                  sx={{ gap: 1, marginLeft: 0, marginRight: 0 }}
                />
              </SettingRowInline>
              <SettingRowInline title={t('settings.show_main.title')} desc={t('settings.show_main.desc')} icon={<Monitor size={18} />}>
                <FormControlLabel
                  control={<Switch checked={showMainOnAutoStart} onChange={(e) => handleToggleShowMainWindowOnAutoStart(e.target.checked)} />}
                  label={t('settings.show_main.title')}
                  sx={{ gap: 1, marginLeft: 0, marginRight: 0 }}
                />
              </SettingRowInline>
            </Box>
          </Box>
        </Grid>

        <Grid size={{ xs: 12, lg: 6 }}>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
            <SectionHeader icon={<ShieldAlert size={18} />} label={t('settings.tabs.security')} />

            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
              <SettingRowInline title={t('settings.ca_management.reset')} desc={t('settings.ca_management.reset_hint')} icon={<RefreshCcw size={18} />}>
                <Button
                  size="small"
                  variant="outlined"
                  startIcon={isCertBusy ? <CircularProgress size={20} /> : null}
                  onClick={handleRegenerateCert}
                >
                  {isCertBusy ? t('ech_form.probing') : t('common.apply')}
                </Button>
              </SettingRowInline>
              <SettingRowInline title={t('settings.ca_management.export')} desc={caStatus?.CertPath || undefined} icon={<FolderOpen size={18} />}>
                <Button
                  size="small"
                  variant="text"
                  startIcon={<FolderOpen size={16} />}
                  onClick={() => OpenCertDir()}
                >
                  {t('common.view')}
                </Button>
              </SettingRowInline>
            </Box>

            <StackedRow title={t('settings.ca_management.title')} desc={caStatus?.Installed ? t('dashboard.cert_installed') : t('dashboard.cert_not_installed')} icon={<ShieldAlert size={18} />}>
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
                <Typography variant="body2" sx={{ fontWeight: 'bold', color: caStatus?.Installed ? 'success.main' : 'text.secondary' }}>
                  {caStatus?.Installed ? t('settings.certs_count', { count: installedCerts.length }) : t('common.off')}
                </Typography>
                {installedCerts.length === 0 ? (
                  <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', py: 2, color: 'text.secondary', opacity: 0.5 }}>
                    <ShieldAlert size={32} />
                    <Typography variant="caption" sx={{ mt: 0.5 }}>{t('proxies.no_ech')}</Typography>
                  </Box>
                ) : (
                  <Box sx={{ mt: 1, maxHeight: 320, overflowY: 'auto' }}>
                    {installedCerts.map((cert: any) => (
                      <Box key={cert.token} sx={{ p: 1, display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1.5, border: 1, borderColor: 'divider', borderRadius: 1, bgcolor: 'background.default', mb: 0.5 }}>
                        <Box sx={{ minWidth: 0, flex: 1 }}>
                          <Typography variant="caption" sx={{ fontWeight: 'bold', display: 'block', wordBreak: 'break-all' }}>
                            {cert.subject}
                          </Typography>
                          <Typography variant="caption" sx={{ fontSize: '0.75rem', color: 'text.secondary', display: 'block', wordBreak: 'break-all' }}>
                            {cert.storeLocation} / {cert.storeName} / {cert.thumbprint}
                          </Typography>
                        </Box>
                        <Button
                          size="small"
                          variant="outlined"
                          color="error"
                          disabled={isCertBusy}
                          onClick={() => handleUninstallCert(cert.token)}
                        >
                          <Trash2 size={16} />
                          {t('common.delete')}
                        </Button>
                      </Box>
                    ))}
                  </Box>
                )}
              </Box>
            </StackedRow>
          </Box>
        </Grid>

        <Grid size={12}>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
            <SectionHeader icon={<Wifi size={18} />} label={t('network.ipv6_title')}
              action={<Button size="small" variant="text" disabled={isIpv6Checking} onClick={handleRefreshIPv6}>
                {isIpv6Checking ? t('network.ipv6_checking') : t('network.ipv6_refresh')}
              </Button>}
            />

            {ipv6Available ? (
              <Box sx={{ p: 2, bgcolor: mode === 'light' ? 'rgba(34,197,94,0.1)' : 'rgba(34,197,94,0.15)', border: 1, borderColor: mode === 'light' ? 'rgba(34,197,94,0.3)' : 'rgba(34,197,94,0.4)', borderRadius: 2, display: 'flex', alignItems: 'flex-start', gap: 2 }}>
                <Box sx={{ width: 40, height: 40, borderRadius: 1, bgcolor: 'success.main', color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
                  <Wifi size={16} />
                </Box>
                <Box>
                  <Typography variant="body2" sx={{ fontWeight: 'bold', color: 'success.main' }}>
                    {t('network.ipv6_ok')}
                  </Typography>
                  <Typography variant="caption" color="text.secondary" sx={{ lineHeight: 1.5, display: 'block' }}>
                    {t('network.ipv6_ok_desc')}
                  </Typography>
                </Box>
              </Box>
            ) : (
              <Box sx={{ p: 2, bgcolor: mode === 'light' ? 'rgba(239,68,68,0.1)' : 'rgba(239,68,68,0.15)', border: 1, borderColor: mode === 'light' ? 'rgba(239,68,68,0.3)' : 'rgba(239,68,68,0.4)', borderRadius: 2, display: 'flex', alignItems: 'flex-start', gap: 2 }}>
                <Box sx={{ width: 40, height: 40, borderRadius: 1, bgcolor: 'error.main', color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
                  <AlertCircle size={16} />
                </Box>
                <Box>
                  <Typography variant="body2" sx={{ fontWeight: 'bold', color: 'error.main' }}>
                    {t('network.ipv6_disabled_title')}
                  </Typography>
                  <Typography variant="caption" color="text.secondary" sx={{ lineHeight: 1.5, display: 'block' }}>
                    {t('network.ipv6_disabled_desc')}
                  </Typography>
                </Box>
              </Box>
            )}
          </Box>
        </Grid>

        <Grid size={12}>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
            <SectionHeader icon={<CloudLightning size={18} />} label={t('rules.form.cf_pool')}
              action={<Button size="small" variant="text" disabled={isCheckingHealth} onClick={handleHealthCheck}>
                {isCheckingHealth ? t('ech_form.probing') : t('dns.test')}
              </Button>}
            />

            <Box sx={{ bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2, overflow: 'hidden' }}>
              <Grid container columns={12} spacing={1}>
                <Grid size={{ xs: 12, md: 2 }}>
                  <Box sx={{ p: 2, borderRight: { md: 1 }, borderColor: 'divider', display: 'flex', flexDirection: 'column', justifyContent: 'center', alignItems: 'center', height: '100%' }}>
                    <Button
                      color="primary"
                      size="small"
                      startIcon={isRefreshing ? <CircularProgress size={20} /> : <Download size={16} />}
                      onClick={handleFetchIPs}
                    >
                      {isRefreshing ? undefined : t('settings.cf_pool.fetch_now')}
                    </Button>
                  </Box>
                </Grid>
                <Grid size={{ xs: 12, md: 10 }}>
                  <Box sx={{ p: 2 }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 1, px: 1 }}>
                      <Typography variant="caption" sx={{ fontWeight: 'bold', textTransform: 'uppercase', color: 'text.secondary', letterSpacing: '0.05em' }}>
                        {t('settings.ip_pool', { count: ipStats.length })}
                      </Typography>
                      <Zap size={16} color="warning.main" />
                    </Box>
                    <Grid container columns={{ xs: 1, sm: 2 }} spacing={1} sx={{ maxHeight: 400, overflowY: 'auto', px: 1, pb: 2 }}>
                      {ipStats.length === 0 ? (
                        <Grid size={12}>
                          <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', py: 2, color: 'text.secondary', opacity: 0.5 }}>
                            <AlertCircle size={32} />
                            <Typography variant="caption" sx={{ mt: 0.5 }}>{t('rules.form.no_domains')}</Typography>
                          </Box>
                        </Grid>
                      ) : (
                        ipStats.map((ip: any, i: number) => {
                          const latency = parseLatencyMs(ip.latency);
                          const ok = latency > 0;
                          return (
                            <Grid key={i} size={{ xs: 12, sm: 6 }}>
                              <Box sx={{ p: 1, display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1.5, border: 1, borderColor: 'divider', borderRadius: 1, bgcolor: 'background.default', '&:hover': { borderColor: 'primary.main' }, transition: 'all 0.15s' }}>
                                <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', minWidth: 0 }}>
                                  <Box sx={{ width: 8, height: 8, borderRadius: '50%', flexShrink: 0, bgcolor: ok ? 'success.main' : 'error.main', boxShadow: ok ? '0 0 8px rgba(34,197,94,0.5)' : undefined }} />
                                  <Typography variant="caption" sx={{ fontFamily: 'monospace', fontWeight: 'bold', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                                    {ip.ip}
                                  </Typography>
                                </Box>
                                <Typography variant="caption" sx={{ fontWeight: 'bold', color: ok && latency < 200 ? 'success.main' : 'warning.main' }}>
                                  {ip.latency ? `${Math.round(latency)}ms` : '---'}
                                </Typography>
                              </Box>
                            </Grid>
                          );
                        })
                      )}
                    </Grid>
                  </Box>
                </Grid>
              </Grid>
            </Box>
          </Box>
        </Grid>

        <Grid size={12}>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
            <SectionHeader icon={<FileText size={18} />} label={t('settings.tabs.logs')}
              action={<Button size="small" variant="outlined" disabled={isCleaningLogs} onClick={handleCleanLogs}>
                <Box sx={{ display: 'flex', gap: 0.5, alignItems: 'center' }}>
                  <Trash2 size={16} />
                  {t('settings.logs.clean')}
                </Box>
              </Button>}
            />

            <StackedRow title={t('settings.logs.title')} desc={t('settings.logs.desc')} icon={<FileText size={18} />}>
              {logFiles.length === 0 ? (
                <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', py: 2, color: 'text.secondary', opacity: 0.5 }}>
                  <FileText size={32} />
                  <Typography variant="caption" sx={{ mt: 0.5 }}>{t('settings.logs.empty')}</Typography>
                </Box>
              ) : (
                <Box sx={{ mt: 1, maxHeight: 360, overflowY: 'auto' }}>
                  {logFiles.map((f: any, i: number) => (
                    <Box key={f.name} sx={{ p: 1, display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1.5, border: 1, borderColor: 'divider', borderRadius: 1, bgcolor: 'background.default', mb: 0.5 }}>
                      <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', fontSize: '0.75rem', fontWeight: 'bold', wordBreak: 'break-all', minWidth: 0, flex: 1 }}>
                        {fmtLogDate(f.name)}
                        {i === 0 && (
                          <Badge color="primary" sx={{ borderRadius: '50%' }}>
                            {t('settings.logs.current')}
                          </Badge>
                        )}
                      </Box>
                      <Typography variant="caption" sx={{ fontSize: '0.75rem', color: 'text.secondary', flexShrink: 0 }}>
                        {fmtLogSize(f.size || 0)}
                      </Typography>
                      <Button size="small" variant="text" onClick={() => handleViewLog(f.name)} sx={{ flexShrink: 0 }}>
                        <Box sx={{ display: 'flex', gap: 0.5, alignItems: 'center' }}>
                          <FolderOpen size={16} />
                          {t('settings.logs.view')}
                        </Box>
                      </Button>
                    </Box>
                  ))}
                </Box>
              )}
            </StackedRow>
          </Box>
        </Grid>
      </Grid>
    </Box>
  );
};

export default Settings;
