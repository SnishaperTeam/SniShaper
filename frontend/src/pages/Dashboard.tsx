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
  GetTUNConfig,
  GetTUNStatus,
  StartProxy,
  StartTUN,
  StopProxy,
  StopTUN,
  EnableSystemProxy,
  DisableSystemProxy,
  GetStats,
  GetCAInstallStatus,
  OpenCAFile,
  InstallCA
} from '../api/bindings';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import Modal from '../components/Modal';
import { toast } from '../lib/toast';
import { cn, formatSpeed, extractErrorMessage } from '../lib/utils';
import { useTranslation } from '../i18n/I18nContext';

const Dashboard: React.FC = () => {
  const { t } = useTranslation();
  const [proxyRunning, setProxyRunning] = useState(false);
  const [sysProxyEnabled, setSysProxyEnabled] = useState(false);
  const [proxyMode, setProxyMode] = useState('MITM');
  const [port, setPort] = useState(8080);
  const [isOperating, setIsOperating] = useState(false);
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

  const refresh = async () => {
    try {
      const [running, sysStatus, mode, p, ca, tunCfg, tunState] = await Promise.all([
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
      setProxyMode(mode.toUpperCase());
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
      return { running, sysStatus, mode, port: p, ca, tunCfg: normalizedTunConfig, tunState: normalizedTunStatus };
    } catch (e) {
      console.error("Dashboard refresh error:", e);
      return null;
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
    return () => clearInterval(timer);
  }, [isActive, isPageVisible]);

  const handleToggleProxy = async () => {
    if (isOperating) return;
    setIsOperating(true);
    try {
      if (proxyRunning) await StopProxy();
      else await StartProxy();
      await new Promise(r => setTimeout(r, 600));
      await refresh();
    } catch (err) { console.error("Failed to toggle proxy:", err);
    } finally { setIsOperating(false); }
  };

  const handleToggleSysProxy = async () => {
    if (isOperating) return;
    setIsOperating(true);
    try {
      if (sysProxyEnabled) await DisableSystemProxy();
      else await EnableSystemProxy();
      await new Promise(r => setTimeout(r, 800));
      await refresh();
    } catch (err) { console.error("Failed to toggle system proxy:", err);
    } finally { setIsOperating(false); }
  };

  const handleToggleTUN = async () => {
    if (isTUNBusy) return;
    setIsTUNBusy(true);
    const nextEnabled = !tunStatus.running;
    try {
      if (nextEnabled) await StartTUN();
      else await StopTUN();
      await new Promise(r => setTimeout(r, nextEnabled ? 1200 : 500));
      const state = await refresh();
      const running = Boolean(state?.tunState?.running);
      if (nextEnabled && !running) {
        toast.error(t('dashboard.notifications.tun_not_running'), String(state?.tunState?.message || 'TUN 配置已启用，但运行状态仍为关闭。'));
        return;
      }
      toast.success(t('dashboard.notifications.tun_updated'), nextEnabled ? 'TUN 已进入运行态。' : '已关闭 TUN 路径。');
    } catch (err) {
      await refresh();
      toast.error(t('dashboard.notifications.tun_failed'), extractErrorMessage(err));
    } finally { setIsTUNBusy(false); }
  };

  const handleInstallCA = async () => {
    setIsInstallingCert(true);
    try {
      await InstallCA();
      await new Promise(r => setTimeout(r, 2000));
      const ca = await GetCAInstallStatus();
      setCaStatus(ca || { Installed: false });
      if (ca?.Installed) setShowCertModal(false);
    } catch (err) { console.error("Failed to install CA:", err);
    } finally { setIsInstallingCert(false); }
  };

  return (
    <div className="px-6 pt-10 pb-6 max-w-5xl mx-auto space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-700">
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-end gap-4 mb-6">
        <div>
          <h1 className="text-3xl font-black tracking-tighter">{t('dashboard.title')}</h1>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            onClick={handleToggleProxy}
            loading={isOperating}
            variant={proxyRunning ? 'danger' : 'primary'}
            size="sm"
            icon={isOperating ? undefined : (proxyRunning ? <Square size={16} fill="white" /> : <Play size={16} fill="white" />)}
          >
            {proxyRunning ? t('dashboard.proxy_stop') : t('dashboard.proxy_start')}
          </Button>
          <Button
            onClick={handleToggleSysProxy}
            loading={isOperating}
            variant={sysProxyEnabled ? 'success' : 'outline'}
            size="sm"
            icon={isOperating ? undefined : <Globe size={16} />}
          >
            {t('dashboard.sys_proxy')}: {sysProxyEnabled ? t('common.on') : t('common.off')}
          </Button>
          <Button
            onClick={handleToggleTUN}
            loading={isTUNBusy}
            disabled={isTUNBusy || !tunStatus.supported}
            variant={tunStatus.running ? 'warning' : 'outline'}
            size="sm"
            icon={isTUNBusy ? undefined : <Globe size={16} />}
          >
            {t('dashboard.tun_status')}: {tunStatus.running ? t('common.on') : t('common.off')}
          </Button>
        </div>
      </div>

      <section className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        <Card title={t('dashboard.core_status')} icon={<Cpu size={20} />}>
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <span className="text-text-secondary text-sm">{t('dashboard.run_status')}</span>
              <span className={cn(
                "px-2 py-0.5 rounded-lg text-[11px] font-black uppercase",
                proxyRunning ? "bg-success/10 text-success" : "bg-danger/10 text-danger"
              )}>
                {proxyRunning ? t('common.running') : t('common.stopped')}
              </span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-text-secondary text-sm">{t('dashboard.work_mode')}</span>
              <span className="font-bold text-accent">{proxyMode}</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-text-secondary text-sm">{t('dashboard.listen_port')}</span>
              <span className="font-bold">{port}</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-text-secondary text-sm">{t('dashboard.tun_status')}</span>
              <span className={cn(
                "px-2 py-0.5 rounded-lg text-[11px] font-black uppercase",
                tunStatus.running ? "bg-warning/15 text-warning" : "bg-background-hover text-text-secondary"
              )}>
                {tunStatus.running ? t('common.running') : t('common.off')}
              </span>
            </div>
          </div>
        </Card>

        <Card title={t('dashboard.cert_status')} icon={<ShieldCheck size={20} />}>
          <div className="space-y-3">
            <div className={cn(
              "flex items-center gap-2 p-2.5 rounded-2xl border",
              caStatus.Installed
                ? "bg-success/10 text-success border-success/30"
                : "bg-danger/10 text-danger border-danger/30"
            )}>
              {caStatus.Installed ? <ShieldCheck size={14} /> : <ShieldAlert size={14} />}
              <span className="text-xs font-bold truncate">
                {caStatus.Installed ? t('dashboard.cert_installed') : t('dashboard.cert_not_installed')}
              </span>
            </div>
            <div className="text-[10px] text-text-muted font-medium px-1 flex justify-between items-center">
              <span className="truncate max-w-[140px] opacity-60 text-[9px]" title={caStatus.CertPath}>
                {caStatus.CertPath || t('dashboard.path_pending')}
              </span>
              <button onClick={() => OpenCAFile()} className="flex items-center gap-1 text-accent hover:underline font-bold shrink-0" aria-label="查看证书">
                <Search size={10} aria-hidden /> {t('common.view')}
              </button>
            </div>
          </div>
        </Card>

        <Card title={t('dashboard.conn_info')} icon={<ShieldCheck size={20} />} className="lg:col-span-1">
          <div className="space-y-3">
            <div className="flex items-center gap-2 p-2.5 bg-accent/10 border border-accent/20 rounded-2xl">
              <Zap size={14} className="text-accent" aria-hidden />
              <span className="text-sm font-bold text-accent truncate">127.0.0.1:{port}</span>
            </div>
            <div className="text-[11px] text-text-muted font-medium px-1 flex items-center justify-end">
              <span className="text-[9px] bg-background-hover px-1.5 py-0.5 rounded text-text-secondary uppercase">{t('common.ready')}</span>
            </div>
          </div>
        </Card>
      </section>

      <Modal isOpen={showCertModal} onClose={() => setShowCertModal(false)} title={t('dashboard.install_cert.title')} maxWidth="max-w-md">
        <div className="space-y-6 py-2">
          <div className="flex justify-center">
            <div className="w-20 h-20 bg-accent/10 rounded-full flex items-center justify-center text-accent animate-pulse">
              <Lock size={40} />
            </div>
          </div>
          <div className="text-center space-y-2">
            <h4 className="text-lg font-bold">{t('dashboard.install_cert.subtitle')}</h4>
          </div>
          <div className="bg-background-soft/50 border border-border rounded-2xl p-4 space-y-3">
            <div className="flex items-start gap-3">
              <div className="mt-0.5 text-warning group-hover:scale-110 transition-transform">
                <ShieldAlert size={16} />
              </div>
              <div className="space-y-1">
                <p className="text-[11px] font-bold">{t('dashboard.install_cert.security_alert')}</p>
                <p className="text-[10px] text-text-muted leading-normal">{t('dashboard.install_cert.security_desc')}</p>
              </div>
            </div>
          </div>
          <div className="flex flex-col gap-3 pt-2">
            <Button onClick={handleInstallCA} loading={isInstallingCert} icon={isInstallingCert ? undefined : <Download size={18} />} className="w-full">
              {isInstallingCert ? t('dashboard.install_cert.installing') : t('dashboard.install_cert.install_now')}
            </Button>
            <Button onClick={() => setShowCertModal(false)} variant="ghost" className="w-full">
              {t('dashboard.install_cert.remind_later')}
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
};

export default Dashboard;
