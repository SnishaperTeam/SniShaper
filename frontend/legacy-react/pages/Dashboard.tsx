import React, { useState, useEffect, useRef } from 'react';
import {
  Play,
  Square,
  Globe,
  Cpu,
  ShieldCheck,
  Zap,
  ShieldAlert,
  Search,
  Loader2,
  Download,
  Lock
} from '../lib/icons';
import {
  GetProxyMode,
  IsProxyRunning,
  GetSystemProxyStatus,
  GetListenPort,
  GetPortOccupant,
  KillPortOccupant,
  GetTUNConfig,
  GetTUNStatus,
  StartProxy,
  StartTUN,
  StopProxy,
  StopTUN,
  EnableSystemProxy,
  DisableSystemProxy,
  GetCAInstallStatus,
  OpenCAFile,
  InstallCA,
  EventsOn
} from '../api/bindings';
import Modal from '../components/Modal';
import { toast } from '../lib/toast';
import { formatSpeed, extractErrorMessage } from '../lib/utils';
import { useTranslation } from '../i18n/I18nContext';
import {
  Box,
  Typography,
  Button,
  Grid,
} from '@mui/material';

const Dashboard: React.FC = () => {
  const { t } = useTranslation();
  const [proxyRunning, setProxyRunning] = useState(false);
  const [sysProxyEnabled, setSysProxyEnabled] = useState(false);
  const [proxyMode, setProxyMode] = useState('MITM');
  const [port, setPort] = useState(8080);
  const [isOperating, setIsOperating] = useState(false);
  const operatingRef = useRef(false);
  const [isActive, setIsActive] = useState(true);
  const [isPageVisible, setIsPageVisible] = useState(true);
  const inactivityTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [tunConfig, setTunConfig] = useState<any>({ mtu: 9000, dns_hijack: true });
  const [tunStatus, setTunStatus] = useState<any>({
    supported: true, running: false, enabled: false, message: t('common.loading')
  });
  const [isTUNBusy, setIsTUNBusy] = useState(false);
  const [caStatus, setCaStatus] = useState<any>({ Installed: false, CertPath: '', Platform: 'windows' });
  const [showCertModal, setShowCertModal] = useState(false);
  const [isInstallingCert, setIsInstallingCert] = useState(false);
  const [portOccupant, setPortOccupant] = useState<{ port: number; pid: number; name: string } | null>(null);
  const [showKillDialog, setShowKillDialog] = useState(false);

  const refresh = async () => {
    try {
      const [running, sysStatus, m, p, ca, tunCfg, tunState] = await Promise.all([
        IsProxyRunning(),
        GetSystemProxyStatus(),
        GetProxyMode(),
        GetListenPort(),
        GetCAInstallStatus(),
        GetTUNConfig(),
        GetTUNStatus()
      ]);
      setProxyRunning(running);
      setSysProxyEnabled(sysStatus.Enabled);
      setProxyMode(m.toUpperCase());
      setPort(p);
      setCaStatus(ca || { Installed: false });
      const normalizedTunConfig = {
        mtu: Number(tunCfg?.mtu ?? tunCfg?.MTU ?? 9000),
        dns_hijack: Boolean(tunCfg?.dns_hijack ?? tunCfg?.DNSHijack ?? true),
      };
      const normalizedTunStatus = {
        supported: Boolean(tunState?.supported ?? tunState?.Supported),
        running: Boolean(tunState?.running ?? tunState?.Running),
        enabled: Boolean(tunState?.enabled ?? tunState?.Enabled),
        driver: String(tunState?.driver ?? tunState?.Driver ?? ''),
        message: String(tunState?.message ?? tunState?.Message ?? ''),
      };
      setTunConfig(normalizedTunConfig);
      setTunStatus(normalizedTunStatus);
      const statusPending = ca?.InstallHelp === '证书状态初始化中' || ca?.InstallHelp === '证书管理器未初始化';
      if (ca?.Installed) setShowCertModal(false);
      if (ca && !statusPending && !ca.Installed && !sessionStorage.getItem('ca_modal_shown')) {
        setShowCertModal(true);
        sessionStorage.setItem('ca_modal_shown', 'true');
      }
    } catch (e) {
      console.error("Dashboard refresh error:", e);
    }
  };

  useEffect(() => {
    const resetInactivityTimer = () => {
      setIsActive(true);
      if (inactivityTimer.current) clearTimeout(inactivityTimer.current);
      inactivityTimer.current = setTimeout(() => setIsActive(false), 60000);
    };
    window.addEventListener('mousemove', resetInactivityTimer);
    window.addEventListener('keydown', resetInactivityTimer);
    window.addEventListener('click', resetInactivityTimer);
    const handleVisibilityChange = () => setIsPageVisible(!document.hidden);
    document.addEventListener('visibilitychange', handleVisibilityChange);
    resetInactivityTimer();
    return () => {
      window.removeEventListener('mousemove', resetInactivityTimer);
      window.removeEventListener('keydown', resetInactivityTimer);
      window.removeEventListener('click', resetInactivityTimer);
      document.removeEventListener('visibilitychange', handleVisibilityChange);
      if (inactivityTimer.current) clearTimeout(inactivityTimer.current);
    };
  }, []);

  useEffect(() => {
    refresh();
    const getInterval = () => {
      if (!isPageVisible) return 60000;
      if (!isActive) return 30000;
      return 5000;
    };
    const timer = setInterval(refresh, getInterval());

    const unlisten = EventsOn("app:state_changed", (state: any) => {
      if (!state) return;
      if (typeof state.proxyRunning === 'boolean') setProxyRunning(state.proxyRunning);
      if (typeof state.systemProxyActive === 'boolean') setSysProxyEnabled(state.systemProxyActive);
      if (typeof state.proxyMode === 'string') setProxyMode(state.proxyMode.toUpperCase());
      if (typeof state.tunRunning === 'boolean') {
        setTunStatus((prev: any) => ({ ...prev, running: state.tunRunning, enabled: state.tunRunning }));
        if (state.tunRunning) setIsTUNBusy(false);
      }
      if (typeof state.tunMessage === 'string') {
        setTunStatus((prev: any) => ({ ...prev, message: state.tunMessage }));
        if (!state.tunRunning && state.tunMessage && state.tunMessage !== 'TUN is not running') {
          setIsTUNBusy(false);
          toast.error(t('dashboard.notifications.tun_failed'), state.tunMessage);
        }
      }
      setTimeout(refresh, 50);
    });

    return () => {
      clearInterval(timer);
      if (unlisten) unlisten();
    };
  }, [isActive, isPageVisible]);

  const handleToggleProxy = async () => {
    if (operatingRef.current) return;
    operatingRef.current = true;
    setIsOperating(true);
    try {
      if (proxyRunning) await StopProxy();
      else await StartProxy();
    } catch (err) {
      const msg = extractErrorMessage(err);
      console.error("[UI] Failed to toggle proxy:", err);
      if (!proxyRunning && /Only one usage|address already in use|occupied|forbidden by its access permissions|permission denied|access denied/i.test(msg)) {
        try {
          const occupant = await GetPortOccupant(port);
          if (occupant) {
            setPortOccupant(occupant);
            setShowKillDialog(true);
            refresh();
            return;
          }
        } catch { /* ignore */ }
      }
      toast.error(t('dashboard.notifications.proxy_toggle_failed'), msg);
    } finally {
      refresh();
      operatingRef.current = false;
      setIsOperating(false);
    }
  };

  const handleKillAndRetry = async () => {
    if (!portOccupant) return;
    setShowKillDialog(false);
    operatingRef.current = true;
    setIsOperating(true);
    try {
      await KillPortOccupant(portOccupant.pid);
      await new Promise(r => setTimeout(r, 400));
      await StartProxy();
    } catch (err) {
      console.error("[UI] Failed to kill and restart proxy:", err);
      toast.error(t('dashboard.notifications.kill_process_failed'), extractErrorMessage(err));
    } finally {
      setPortOccupant(null);
      refresh();
      operatingRef.current = false;
      setIsOperating(false);
    }
  };

  const handleToggleSysProxy = async () => {
    if (operatingRef.current) return;
    operatingRef.current = true;
    setIsOperating(true);
    try {
      if (sysProxyEnabled) await DisableSystemProxy();
      else await EnableSystemProxy();
    } catch (err) {
      console.error("[UI] Failed to toggle system proxy:", err);
      toast.error(t('dashboard.notifications.sys_proxy_failed'), extractErrorMessage(err));
    } finally {
      refresh();
      operatingRef.current = false;
      setIsOperating(false);
    }
  };

  const handleToggleTUN = async () => {
    if (isTUNBusy) return;
    setIsTUNBusy(true);
    const nextEnabled = !tunStatus.running;
    try {
      if (nextEnabled) {
        await StartTUN();
        toast.success(t('dashboard.notifications.tun_updated'), t('dashboard.notifications.tun_starting'));
      } else {
        await StopTUN();
        toast.success(t('dashboard.notifications.tun_updated'), t('dashboard.notifications.tun_stopped'));
        refresh();
        setIsTUNBusy(false);
      }
    } catch (err) {
      toast.error(t('dashboard.notifications.tun_failed'), extractErrorMessage(err));
      refresh();
      setIsTUNBusy(false);
    }
  };

  const handleInstallCA = async () => {
    setIsInstallingCert(true);
    try {
      await InstallCA();
      await new Promise(r => setTimeout(r, 2000));
      const ca = await GetCAInstallStatus();
      setCaStatus(ca || { Installed: false });
      if (ca?.Installed) setShowCertModal(false);
    } catch (err) {
      console.error("Failed to install CA:", err);
    } finally {
      setIsInstallingCert(false);
    }
  };

  return (
    <Box sx={{ pt: 4, pb: 6, maxWidth: '5xl', mx: 'auto' }}>
      <Box sx={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'space-between', alignItems: { xs: 'flex-start', sm: 'flex-end' }, gap: 2, mb: 6 }}>
        <Typography variant="h5" sx={{ fontWeight: 700, letterSpacing: '-0.02em' }}>
          {t('dashboard.title')}
        </Typography>
        <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1 }}>
          <Button
            onClick={handleToggleProxy}
            loading={isOperating}
            color={proxyRunning ? 'error' : 'primary'}
            size="small"
            startIcon={!isOperating && (proxyRunning ? <Square size={14} /> : <Play size={14} />)}
          >
            {!isOperating && (proxyRunning ? t('dashboard.proxy_stop') : t('dashboard.proxy_start'))}
          </Button>
          <Button
            onClick={handleToggleSysProxy}
            loading={isOperating}
            variant={sysProxyEnabled ? 'contained' : 'outlined'}
            color={sysProxyEnabled ? 'success' : undefined}
            size="small"
            startIcon={!isOperating && <Globe size={14} />}
          >
            {t('dashboard.sys_proxy')}: {sysProxyEnabled ? t('common.on') : t('common.off')}
          </Button>
          <Button
            onClick={handleToggleTUN}
            loading={isTUNBusy}
            disabled={isTUNBusy || !tunStatus.supported}
            variant={tunStatus.running ? 'contained' : 'outlined'}
            color={tunStatus.running ? 'warning' : undefined}
            size="small"
            startIcon={!isTUNBusy && <Globe size={14} />}
          >
            {t('dashboard.tun_status')}: {tunStatus.running ? t('common.on') : t('common.off')}
          </Button>
        </Box>
      </Box>

      <Grid container spacing={3}>
        <Grid size={{ xs: 12, md: 6 }}>
          <Box sx={{ p: 3, bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2, boxShadow: 1 }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mb: 2 }}>
              <Box sx={{ color: 'primary.main' }}><Cpu size={20} /></Box>
              <Typography variant="body2" sx={{ fontWeight: 'bold', color: 'text.secondary', letterSpacing: '-0.01em', textTransform: 'uppercase' }}>
                {t('dashboard.core_status')}
              </Typography>
            </Box>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <Typography variant="body2" color="text.secondary">{t('dashboard.run_status')}</Typography>
                <Box sx={{ px: 1, py: 0.25, borderRadius: 1, fontSize: '0.75rem', fontWeight: 'bold', textTransform: 'uppercase', bgcolor: proxyRunning ? 'success.main' : 'error.main', color: proxyRunning ? 'success.contrastText' : 'error.contrastText' }}>
                  {proxyRunning ? t('common.running') : t('common.stopped')}
                </Box>
              </Box>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <Typography variant="body2" color="text.secondary">{t('dashboard.work_mode')}</Typography>
                <Typography variant="body2" sx={{ fontWeight: 'bold', color: 'primary.main' }}>
                  {proxyMode}
                </Typography>
              </Box>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <Typography variant="body2" color="text.secondary">{t('dashboard.listen_port')}</Typography>
                <Typography variant="body2" sx={{ fontWeight: 'bold' }}>
                  {port}
                </Typography>
              </Box>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <Typography variant="body2" color="text.secondary">{t('dashboard.tun_status')}</Typography>
                <Box sx={{ px: 1, py: 0.25, borderRadius: 1, fontSize: '0.75rem', fontWeight: 'bold', textTransform: 'uppercase', bgcolor: tunStatus.running ? 'warning.main' : 'action.hover', color: tunStatus.running ? 'warning.contrastText' : 'text.secondary' }}>
                  {tunStatus.running ? t('common.running') : t('common.off')}
                </Box>
              </Box>
            </Box>
          </Box>
        </Grid>

        <Grid size={{ xs: 12, md: 6 }}>
          <Box sx={{ p: 3, bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2, boxShadow: 1 }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mb: 2 }}>
              <Box sx={{ color: 'primary.main' }}><ShieldCheck size={20} /></Box>
              <Typography variant="body2" sx={{ fontWeight: 'bold', color: 'text.secondary', letterSpacing: '-0.01em', textTransform: 'uppercase' }}>
                {t('dashboard.cert_status')}
              </Typography>
            </Box>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, p: 1.25, borderRadius: 2, border: 1, bgcolor: caStatus.Installed ? 'success.main' : 'error.main', color: caStatus.Installed ? 'success.contrastText' : 'error.contrastText', borderColor: caStatus.Installed ? 'success.light' : 'error.light' }}>
                {caStatus.Installed ? <ShieldCheck size={18} /> : <ShieldAlert size={18} />}
                <Typography variant="caption" sx={{ fontWeight: 'bold', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {caStatus.Installed ? t('dashboard.cert_installed') : t('dashboard.cert_not_installed')}
                </Typography>
              </Box>
              <Box sx={{ display: 'flex', fontSize: '0.75rem', color: 'text.secondary', fontWeight: 'medium', px: 1, justifyContent: 'space-between', alignItems: 'center' }}>
                <Typography noWrap sx={{ maxWidth: 140, opacity: 0.6, fontSize: '0.625rem' }} title={caStatus.CertPath}>
                  {caStatus.CertPath || t('dashboard.path_pending')}
                </Typography>
                <Button
                  variant="text"
                  size="small"
                  onClick={() => OpenCAFile()}
                  sx={{ gap: 0.5, color: 'primary.main', fontWeight: 'bold', '&:hover': { textDecoration: 'underline' } }}
                  startIcon={<Search size={14} />}
                >
                  {t('common.view')}
                </Button>
              </Box>
            </Box>
          </Box>
        </Grid>

        <Grid size={12}>
          <Box sx={{ p: 3, bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2, boxShadow: 1 }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mb: 2 }}>
              <Box sx={{ color: 'primary.main' }}><ShieldCheck size={20} /></Box>
              <Typography variant="body2" sx={{ fontWeight: 'bold', color: 'text.secondary', letterSpacing: '-0.01em', textTransform: 'uppercase' }}>
                {t('dashboard.conn_info')}
              </Typography>
            </Box>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, p: 1.25, bgcolor: 'primary.main', border: 1, borderColor: 'primary.main', borderRadius: 2 }}>
                <Zap size={18} color="primary.contrastText" aria-hidden />
                <Typography variant="body2" sx={{ fontWeight: 'bold', color: 'primary.contrastText', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  127.0.0.1:{port}
                </Typography>
              </Box>
              <Box sx={{ display: 'flex', fontSize: '0.75rem', color: 'text.secondary', fontWeight: 'medium', px: 1, justifyContent: 'flex-end', alignItems: 'center' }}>
                <Typography sx={{ fontSize: '0.625rem', bgcolor: 'action.hover', px: 1.5, py: 0.25, borderRadius: 1, textTransform: 'uppercase', color: 'text.secondary' }}>
                  {t('common.ready')}
                </Typography>
              </Box>
            </Box>
          </Box>
        </Grid>
      </Grid>

      <Modal isOpen={showCertModal} onClose={() => setShowCertModal(false)} title={t('dashboard.install_cert.title')} maxWidth="md">
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3, py: 1 }}>
          <Box sx={{ display: 'flex', justifyContent: 'center' }}>
            <Box sx={{ width: 56, height: 56, borderRadius: '50%', bgcolor: 'primary.main', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <Lock size={24} color="primary.contrastText" />
            </Box>
          </Box>
          <Box sx={{ textAlign: 'center' }}>
            <Typography variant="h5" sx={{ fontWeight: 'bold' }}>
              {t('dashboard.install_cert.subtitle')}
            </Typography>
          </Box>
          <Box sx={{ bgcolor: 'background.default', border: 1, borderColor: 'divider', borderRadius: 2, p: 2 }}>
            <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1.5 }}>
              <Box sx={{ mt: 0.5, color: 'warning.main' }}>
                <ShieldAlert size={20} />
              </Box>
              <Box>
                <Typography variant="body2" sx={{ fontWeight: 'bold' }}>
                  {t('dashboard.install_cert.security_alert')}
                </Typography>
                <Typography variant="caption" color="text.secondary" sx={{ lineHeight: 1.5, display: 'block' }}>
                  {t('dashboard.install_cert.security_desc')}
                </Typography>
              </Box>
            </Box>
          </Box>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, pt: 1 }}>
            <Button
              onClick={handleInstallCA}
              loading={isInstallingCert}
              variant="contained"
              fullWidth
              startIcon={!isInstallingCert && <Download size={16} />}
            >
              {isInstallingCert ? t('dashboard.install_cert.installing') : t('dashboard.install_cert.install_now')}
            </Button>
            <Button onClick={() => setShowCertModal(false)} variant="text" fullWidth>
              {t('dashboard.install_cert.remind_later')}
            </Button>
          </Box>
        </Box>
      </Modal>

      <Modal isOpen={showKillDialog} onClose={() => setShowKillDialog(false)} title={portOccupant && portOccupant.pid <= 0 ? t('dashboard.port_occupied.title_unavailable') : t('dashboard.port_occupied.title_occupied')} maxWidth="sm"
        footer={portOccupant && portOccupant.pid <= 0 ? (
          <Button type="button" onClick={() => setShowKillDialog(false)} variant="contained" size="small">
            {t('dashboard.port_occupied.ok')}
          </Button>
        ) : (
          <>
            <Button type="button" onClick={() => setShowKillDialog(false)} variant="outlined" size="small">
              {t('common.cancel')}
            </Button>
            <Button type="button" onClick={handleKillAndRetry} variant="contained" color="error" size="small" startIcon={<ShieldAlert size={16} />}>
              {t('dashboard.port_occupied.kill_and_retry')}
            </Button>
          </>
        )}>
        <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1.5 }}>
          <Box sx={{ mt: 0.5, color: 'error.main', flexShrink: 0 }}>
            <ShieldAlert size={22} />
          </Box>
          <Box>
            {portOccupant && portOccupant.pid <= 0 ? (
              <>
                <Typography variant="body2" sx={{ fontWeight: 'bold', color: 'text.primary' }}>
                  {t('dashboard.port_occupied.excluded_range', { port: portOccupant?.port, name: portOccupant?.name })}
                </Typography>
                <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.5, lineHeight: 1.6 }}>
                  {t('dashboard.port_occupied.excluded_desc')}
                </Typography>
              </>
            ) : (
              <>
                <Typography variant="body2" sx={{ fontWeight: 'bold', color: 'text.primary' }}>
                  {t('dashboard.port_occupied.occupied_by', { port: portOccupant?.port ?? '', name: portOccupant?.name || t('dashboard.port_occupied.unknown_process'), pid: portOccupant?.pid ?? '' })}
                </Typography>
                <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.5, lineHeight: 1.6 }}>
                  {t('dashboard.port_occupied.occupied_desc')}
                </Typography>
              </>
            )}
          </Box>
        </Box>
      </Modal>
    </Box>
  );
};

export default Dashboard;
