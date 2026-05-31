import React, { useState, useEffect, useRef } from 'react';
import {
  Play,
  Square,
  Globe,
  Cpu,
  ShieldCheck,
  Activity,
  ArrowUpRight,
  ArrowDownRight,
  Zap,
  ShieldAlert,
  Search,
  Loader2,
  Download,
  Lock
} from 'lucide-react';
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
  UpdateTUNConfig,
  GetStats,
  GetCAInstallStatus,
  OpenCAFile,
  InstallCA,
  EventsOn
} from '../api/bindings';
import Modal from '../components/Modal';
import { toast } from '../lib/toast';
import { useTranslation } from '../i18n/I18nContext';

const DashboardCard: React.FC<{
  title: string;
  icon: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}> = ({ title, icon, children, className }) => (
  <div className={`p-6 bg-background-card border border-border rounded-2xl shadow-sm hover:shadow-md transition-all ${className}`}>
    <div className="flex items-center gap-3 mb-4">
      <div className="text-accent">{icon}</div>
      <h3 className="text-[13px] font-bold text-text-secondary tracking-tight uppercase">{title}</h3>
    </div>
    {children}
  </div>
);

const formatSpeed = (bytes: number) => {
  if (bytes < 1024) return `${Math.round(bytes)} B/s`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB/s`;
  return `${Math.round(bytes / (1024 * 1024))} MB/s`;
};

const normalizeTUNStatus = (value: any) => ({
  supported: Boolean(value?.supported ?? value?.Supported),
  running: Boolean(value?.running ?? value?.Running),
  enabled: Boolean(value?.enabled ?? value?.Enabled),
  driver: String(value?.driver ?? value?.Driver ?? ''),
  message: String(value?.message ?? value?.Message ?? ''),
});

const normalizeTUNConfig = (value: any) => ({
  mtu: Number(value?.mtu ?? value?.MTU ?? 9000),
  dns_hijack: Boolean(value?.dns_hijack ?? value?.DNSHijack ?? true),
});

const extractErrorMessage = (err: any): string => {
  if (!err) return '';
  if (typeof err === 'string') return err;
  if (typeof err?.message === 'string' && err.message.trim()) return err.message;
  if (typeof err?.cause === 'string' && err.cause.trim()) return err.cause;
  if (typeof err?.cause?.message === 'string' && err.cause.message.trim()) return err.cause.message;
  try {
    return JSON.stringify(err);
  } catch {
    return String(err);
  }
};

const Dashboard: React.FC = () => {
  const { t } = useTranslation();
  const [proxyRunning, setProxyRunning] = useState(false);
  const [sysProxyEnabled, setSysProxyEnabled] = useState(false);
  const [proxyMode, setProxyMode] = useState('MITM');
  const [port, setPort] = useState(8080);
  const [isOperating, setIsOperating] = useState(false);
  const [isActive, setIsActive] = useState(true);
  const [isPageVisible, setIsPageVisible] = useState(true);
  const inactivityTimer = useRef<NodeJS.Timeout | null>(null);
  const [tunConfig, setTunConfig] = useState<any>({
    mtu: 9000,
    dns_hijack: true
  });
  const [tunStatus, setTunStatus] = useState<any>({
    supported: true,
    running: false,
    enabled: false,
    message: t('common.loading')
  });
  const [isTUNBusy, setIsTUNBusy] = useState(false);

  // Real-time Stats
  const [downSpeed, setDownSpeed] = useState(0);
  const [upSpeed, setUpSpeed] = useState(0);

  // CA Status
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
      const normalizedTunConfig = normalizeTUNConfig(tunCfg);
      const normalizedTunStatus = normalizeTUNStatus(tunState);

      setTunConfig(normalizedTunConfig);
      setTunStatus(normalizedTunStatus);

      const statusPending = ca?.InstallHelp === '证书状态初始化中' || ca?.InstallHelp === '证书管理器未初始化';

      if (ca?.Installed) {
        setShowCertModal(false);
      }

      // Auto-show modal only after certificate state is fully initialized.
      if (ca && !statusPending && !ca.Installed && !sessionStorage.getItem('ca_modal_shown')) {
        setShowCertModal(true);
        sessionStorage.setItem('ca_modal_shown', 'true');
      }

      return {
        running,
        sysStatus,
        mode,
        port: p,
        ca,
        tunCfg: normalizedTunConfig,
        tunState: normalizedTunStatus
      };
    } catch (e) {
      console.error("Dashboard refresh error:", e);
      return null;
    }
  };

  useEffect(() => {
    const resetInactivityTimer = () => {
      setIsActive(true);
      if (inactivityTimer.current) {
        clearTimeout(inactivityTimer.current);
      }
      inactivityTimer.current = setTimeout(() => {
        setIsActive(false);
      }, 60000); // 1 minute inactivity
    };

    window.addEventListener('mousemove', resetInactivityTimer);
    window.addEventListener('keydown', resetInactivityTimer);
    window.addEventListener('click', resetInactivityTimer);

    const handleVisibilityChange = () => {
      setIsPageVisible(!document.hidden);
    };
    document.addEventListener('visibilitychange', handleVisibilityChange);

    resetInactivityTimer();

    // Real-time traffic stats from Go
    const unoff = EventsOn("app:traffic", (data: any) => {
      if (data) {
        setDownSpeed(data.down || 0);
        setUpSpeed(data.up || 0);
      }
    });

    return () => {
      window.removeEventListener('mousemove', resetInactivityTimer);
      window.removeEventListener('keydown', resetInactivityTimer);
      window.removeEventListener('click', resetInactivityTimer);
      document.removeEventListener('visibilitychange', handleVisibilityChange);
      if (inactivityTimer.current) {
        clearTimeout(inactivityTimer.current);
      }
      unoff();
    };
  }, []);

  useEffect(() => {
    refresh();

    const getInterval = () => {
      if (!isPageVisible) return 60000; // Hidden: 60s
      if (!isActive) return 30000; // Inactive: 30s
      return 5000; // Active: 5s
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
    } catch (err) {
      console.error("Failed to toggle proxy:", err);
    } finally {
      setIsOperating(false);
    }
  };

  const handleToggleSysProxy = async () => {
    if (isOperating) return;
    setIsOperating(true);
    try {
      if (sysProxyEnabled) await DisableSystemProxy();
      else await EnableSystemProxy();
      await new Promise(r => setTimeout(r, 800)); // Win registry update takes time
      await refresh();
    } catch (err) {
      console.error("Failed to toggle system proxy:", err);
    } finally {
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
      } else {
        await StopTUN();
      }
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
    } finally {
      setIsTUNBusy(false);
    }
  };

  const handleInstallCA = async () => {
    setIsInstallingCert(true);
    try {
      await InstallCA();
      // Wait a bit for system to process
      await new Promise(r => setTimeout(r, 2000));
      const ca = await GetCAInstallStatus();
      setCaStatus(ca || { Installed: false });
      if (ca?.Installed) {
        setShowCertModal(false);
      }
    } catch (err) {
      console.error("Failed to install CA:", err);
    } finally {
      setIsInstallingCert(false);
    }
  };

  return (
    <div className="px-6 pt-10 pb-6 max-w-5xl mx-auto space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-700">
      <div className="flex justify-between items-end mb-6">
        <div>
          <h1 className="text-3xl font-black tracking-tighter">{t('dashboard.title')}</h1>
        </div>
        <div className="flex gap-3">
          <button
            onClick={handleToggleProxy}
            disabled={isOperating}
            className={`flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-bold transition-all shadow-lg min-w-[100px] justify-center ${proxyRunning
                ? "bg-danger text-white shadow-danger/20 hover:brightness-110"
                : "bg-accent text-white shadow-accent/20 hover:brightness-110"
              } ${isOperating ? "opacity-70 cursor-not-allowed" : ""}`}
          >
            {isOperating ? <Loader2 size={16} className="animate-spin" /> : (proxyRunning ? <Square size={16} fill="white" /> : <Play size={16} fill="white" />)}
            {proxyRunning ? t('dashboard.proxy_stop') : t('dashboard.proxy_start')}
          </button>
          <button
            onClick={handleToggleSysProxy}
            disabled={isOperating}
            className={`flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-bold transition-all border border-border shadow-sm min-w-[120px] justify-center ${sysProxyEnabled
                ? "bg-success text-white border-success/30 shadow-success/10"
                : "bg-background-hover text-text-secondary border-border"
              } ${isOperating ? "opacity-70 cursor-not-allowed" : ""}`}
          >
            {isOperating ? <Loader2 size={16} className="animate-spin" /> : <Globe size={16} />}
            {t('dashboard.sys_proxy')}: {sysProxyEnabled ? t('common.on') : t('common.off')}
          </button>
          <button
            onClick={handleToggleTUN}
            disabled={isTUNBusy || !tunStatus.supported}
            className={`flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-bold transition-all border border-border shadow-sm min-w-[118px] justify-center ${
              tunStatus.running
                ? "bg-warning text-white border-warning/30 shadow-warning/10"
                : "bg-background-hover text-text-secondary border-border"
            } ${(isTUNBusy || !tunStatus.supported) ? "opacity-70 cursor-not-allowed" : ""}`}
          >
            {isTUNBusy ? <Loader2 size={16} className="animate-spin" /> : <Globe size={16} />}
            {t('dashboard.tun_status')}: {tunStatus.running ? t('common.on') : t('common.off')}
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        <DashboardCard title={t('dashboard.core_status')} icon={<Cpu size={20} />}>
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <span className="text-text-secondary text-sm">{t('dashboard.run_status')}</span>
              <span className={`px-2 py-0.5 rounded-lg text-[11px] font-black uppercase ${proxyRunning ? "bg-success/10 text-success" : "bg-danger/10 text-danger"}`}>
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
              <span className={`px-2 py-0.5 rounded-lg text-[11px] font-black uppercase ${tunStatus.running ? "bg-warning/15 text-warning" : "bg-background-hover text-text-secondary"}`}>
                {tunStatus.running ? t('common.running') : t('common.off')}
              </span>
            </div>
          </div>
        </DashboardCard>

        <DashboardCard title={t('dashboard.realtime_traffic')} icon={<Activity size={20} />}>
          <div className="grid grid-cols-2 gap-4">
            <div className="p-3 bg-background-soft/50 rounded-2xl border border-border/40 min-w-0">
              <div className="flex items-center gap-1 text-success mb-1">
                <ArrowDownRight size={14} />
                <span className="text-[10px] font-black uppercase">{t('dashboard.download')}</span>
              </div>
              <div className="text-lg font-black tabular-nums truncate flex items-baseline gap-1">
                {formatSpeed(downSpeed).split(' ')[0]} 
                <span className="text-[10px] text-text-muted font-bold uppercase">{formatSpeed(downSpeed).split(' ')[1]}</span>
              </div>
            </div>
            <div className="p-3 bg-background-soft/50 rounded-2xl border border-border/40 min-w-0">
              <div className="flex items-center gap-1 text-accent mb-1">
                <ArrowUpRight size={14} />
                <span className="text-[10px] font-black uppercase">{t('dashboard.upload')}</span>
              </div>
              <div className="text-lg font-black tabular-nums truncate flex items-baseline gap-1">
                {formatSpeed(upSpeed).split(' ')[0]} 
                <span className="text-[10px] text-text-muted font-bold uppercase">{formatSpeed(upSpeed).split(' ')[1]}</span>
              </div>
            </div>
          </div>
        </DashboardCard>

        <DashboardCard title={t('dashboard.cert_status')} icon={<ShieldCheck size={20} />}>
          <div className="space-y-3">
            <div className={`flex items-center gap-2 p-2.5 rounded-2xl border border-transparent ${caStatus.Installed ? "bg-success/10 text-success shadow-[inset_0_0_0_1px_rgba(63,185,80,0.24)]" : "bg-danger/10 text-danger shadow-[inset_0_0_0_1px_rgba(248,81,73,0.22)]"}`}>
              {caStatus.Installed ? <ShieldCheck size={14} /> : <ShieldAlert size={14} />}
              <span className="text-xs font-bold truncate">
                {caStatus.Installed ? t('dashboard.cert_installed') : t('dashboard.cert_not_installed')}
              </span>
            </div>
            <div className="text-[10px] text-text-muted font-medium px-1 flex justify-between items-center">
              <span className="truncate max-w-[140px] opacity-60 text-[9px]" title={caStatus.CertPath}>{caStatus.CertPath || t('dashboard.path_pending')}</span>
              <button onClick={() => OpenCAFile()} className="flex items-center gap-1 text-accent hover:underline font-bold shrink-0">
                <Search size={10} /> {t('common.view')}
              </button>
            </div>
          </div>
        </DashboardCard>

        <DashboardCard title={t('dashboard.conn_info')} icon={<ShieldCheck size={20} />} className="lg:col-span-1">
          <div className="space-y-3">
            <div className="flex items-center gap-2 p-2.5 bg-accent/10 border border-transparent rounded-2xl shadow-[inset_0_0_0_1px_rgba(47,129,247,0.24)]">
              <Zap size={14} className="text-accent" />
              <span className="text-sm font-bold text-accent truncate">127.0.0.1:{port}</span>
            </div>
            <div className="text-[11px] text-text-muted font-medium px-1 flex items-center justify-end">
              <span className="text-[9px] bg-background-hover px-1.5 py-0.5 rounded text-text-secondary uppercase">{t('common.ready')}</span>
            </div>
          </div>
        </DashboardCard>
      </div>

      {/* Certificate Installation Modal */}
      <Modal
        isOpen={showCertModal}
        onClose={() => setShowCertModal(false)}
        title={t('dashboard.install_cert.title')}
        maxWidth="max-w-md"
      >
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
                <p className="text-[10px] text-text-muted leading-normal">
                  {t('dashboard.install_cert.security_desc')}
                </p>
              </div>
            </div>
          </div>

          <div className="flex flex-col gap-3 pt-2">
            <button
              onClick={handleInstallCA}
              disabled={isInstallingCert}
              className="w-full py-3 bg-accent text-white rounded-2xl text-sm font-black shadow-lg shadow-accent/20 hover:scale-[1.02] active:scale-[0.98] transition-all flex items-center justify-center gap-2"
            >
              {isInstallingCert ? (
                <>
                  <Loader2 size={18} className="animate-spin" />
                  <span>{t('dashboard.install_cert.installing')}</span>
                </>
              ) : (
                <>
                  <Download size={18} />
                  <span>{t('dashboard.install_cert.install_now')}</span>
                </>
              )}
            </button>
            <button
              onClick={() => setShowCertModal(false)}
              className="w-full py-3 bg-background-hover text-text-secondary rounded-2xl text-xs font-bold hover:bg-background-soft transition-all"
            >
              {t('dashboard.install_cert.remind_later')}
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
};

export default Dashboard;
