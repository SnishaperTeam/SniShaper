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
  HardDrive,
  CloudLightning,
  ShieldAlert,
  Search,
  Loader2,
  AlertCircle,
  Download,
  Lock
} from 'lucide-react';
import {
  GetProxyMode,
  IsProxyRunning,
  GetSystemProxyStatus,
  GetListenPort,
  StartProxy,
  StopProxy,
  EnableSystemProxy,
  DisableSystemProxy,
  GetStats,
  GetCAInstallStatus,
  OpenCAFile,
  InstallCA,
  EventsOn
} from '../api/bindings';
import Modal from '../components/Modal';

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

const Dashboard: React.FC = () => {
  const [proxyRunning, setProxyRunning] = useState(false);
  const [sysProxyEnabled, setSysProxyEnabled] = useState(false);
  const [proxyMode, setProxyMode] = useState('MITM');
  const [port, setPort] = useState(8080);
  const [isOperating, setIsOperating] = useState(false);

  // Real-time Stats
  const [downSpeed, setDownSpeed] = useState(0);
  const [upSpeed, setUpSpeed] = useState(0);

  // CA Status
  const [caStatus, setCaStatus] = useState<any>({ Installed: false, CertPath: '', Platform: 'windows' });
  const [showCertModal, setShowCertModal] = useState(false);
  const [isInstallingCert, setIsInstallingCert] = useState(false);

  const refresh = async () => {
    try {
      const [running, sysStatus, mode, p, ca] = await Promise.all([
        IsProxyRunning(),
        GetSystemProxyStatus(),
        GetProxyMode(),
        GetListenPort(),
        GetCAInstallStatus()
      ]);

      setProxyRunning(running);
      setSysProxyEnabled(sysStatus.Enabled);
      setProxyMode(mode.toUpperCase());
      setPort(p);
      setCaStatus(ca || { Installed: false });

      // Auto-show modal if not installed and not shown this session
      if (ca && !ca.Installed && !sessionStorage.getItem('ca_modal_shown')) {
        setShowCertModal(true);
        sessionStorage.setItem('ca_modal_shown', 'true');
      }
    } catch (e) {
      console.error("Dashboard refresh error:", e);
    }
  };

  useEffect(() => {
    refresh();
    const timer = setInterval(refresh, 5000); // UI state like proxy run status (slower is fine)

    // Real-time traffic stats from Go
    const unoff = EventsOn("app:traffic", (data: any) => {
      if (data) {
        setDownSpeed(data.down || 0);
        setUpSpeed(data.up || 0);
      }
    });

    return () => {
      clearInterval(timer);
      unoff();
    };
  }, []);

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
          <h1 className="text-3xl font-black tracking-tighter">主页</h1>
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
            {proxyRunning ? "停止代理" : "开启代理"}
          </button>
          <button
            onClick={handleToggleSysProxy}
            disabled={isOperating}
            className={`flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-bold transition-all border shadow-sm min-w-[120px] justify-center ${sysProxyEnabled
                ? "bg-success text-white border-success/30 shadow-success/10"
                : "bg-background-hover text-text-secondary border-border"
              } ${isOperating ? "opacity-70 cursor-not-allowed" : ""}`}
          >
            {isOperating ? <Loader2 size={16} className="animate-spin" /> : <Globe size={16} />}
            系统代理: {sysProxyEnabled ? "开" : "关"}
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        <DashboardCard title="核心状态" icon={<Cpu size={20} />}>
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <span className="text-text-secondary text-sm">运行状态</span>
              <span className={`px-2 py-0.5 rounded-lg text-[11px] font-black uppercase ${proxyRunning ? "bg-success/10 text-success" : "bg-danger/10 text-danger"}`}>
                {proxyRunning ? "Running" : "Stopped"}
              </span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-text-secondary text-sm">工作模式</span>
              <span className="font-bold text-accent">{proxyMode}</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-text-secondary text-sm">监听端口</span>
              <span className="font-bold">{port}</span>
            </div>
          </div>
        </DashboardCard>

        <DashboardCard title="实时流量" icon={<Activity size={20} />}>
          <div className="grid grid-cols-2 gap-4">
            <div className="p-3 bg-background-soft/50 rounded-2xl border border-border/40 min-w-0">
              <div className="flex items-center gap-1 text-success mb-1">
                <ArrowDownRight size={14} />
                <span className="text-[10px] font-black uppercase">Download</span>
              </div>
              <div className="text-lg font-black tabular-nums truncate flex items-baseline gap-1">
                {formatSpeed(downSpeed).split(' ')[0]} 
                <span className="text-[10px] text-text-muted font-bold uppercase">{formatSpeed(downSpeed).split(' ')[1]}</span>
              </div>
            </div>
            <div className="p-3 bg-background-soft/50 rounded-2xl border border-border/40 min-w-0">
              <div className="flex items-center gap-1 text-accent mb-1">
                <ArrowUpRight size={14} />
                <span className="text-[10px] font-black uppercase">Upload</span>
              </div>
              <div className="text-lg font-black tabular-nums truncate flex items-baseline gap-1">
                {formatSpeed(upSpeed).split(' ')[0]} 
                <span className="text-[10px] text-text-muted font-bold uppercase">{formatSpeed(upSpeed).split(' ')[1]}</span>
              </div>
            </div>
          </div>
        </DashboardCard>

        <DashboardCard title="证书状态" icon={<ShieldCheck size={20} />}>
          <div className="space-y-3">
            <div className={`flex items-center gap-2 p-2.5 rounded-2xl border border-transparent ${caStatus.Installed ? "bg-success/10 text-success shadow-[inset_0_0_0_1px_rgba(63,185,80,0.24)]" : "bg-danger/10 text-danger shadow-[inset_0_0_0_1px_rgba(248,81,73,0.22)]"}`}>
              {caStatus.Installed ? <ShieldCheck size={14} /> : <ShieldAlert size={14} />}
              <span className="text-xs font-bold truncate">
                {caStatus.Installed ? "根证书已安装" : "未安装根证书"}
              </span>
            </div>
            <div className="text-[10px] text-text-muted font-medium px-1 flex justify-between items-center">
              <span className="truncate max-w-[140px] opacity-60 text-[9px]" title={caStatus.CertPath}>{caStatus.CertPath || "路径待定"}</span>
              <button onClick={() => OpenCAFile()} className="flex items-center gap-1 text-accent hover:underline font-bold shrink-0">
                <Search size={10} /> 查看
              </button>
            </div>
          </div>
        </DashboardCard>

        <DashboardCard title="连接信息" icon={<ShieldCheck size={20} />} className="lg:col-span-1">
          <div className="space-y-3">
            <div className="flex items-center gap-2 p-2.5 bg-accent/10 border border-transparent rounded-2xl shadow-[inset_0_0_0_1px_rgba(47,129,247,0.24)]">
              <Zap size={14} className="text-accent" />
              <span className="text-sm font-bold text-accent truncate">127.0.0.1:{port}</span>
            </div>
            <div className="text-[11px] text-text-muted font-medium px-1 flex items-center justify-end">
              <span className="text-[9px] bg-background-hover px-1.5 py-0.5 rounded text-text-secondary uppercase">READY</span>
            </div>
          </div>
        </DashboardCard>
      </div>

      {/* Certificate Installation Modal */}
      <Modal
        isOpen={showCertModal}
        onClose={() => setShowCertModal(false)}
        title="安装根证书"
        maxWidth="max-w-md"
      >
        <div className="space-y-6 py-2">
          <div className="flex justify-center">
            <div className="w-20 h-20 bg-accent/10 rounded-full flex items-center justify-center text-accent animate-pulse">
              <Lock size={40} />
            </div>
          </div>

          <div className="text-center space-y-2">
            <h4 className="text-lg font-bold">需安装证书以启用绕过功能</h4>
          </div>

          <div className="bg-background-soft/50 border border-border rounded-2xl p-4 space-y-3">
            <div className="flex items-start gap-3">
              <div className="mt-0.5 text-warning group-hover:scale-110 transition-transform">
                <ShieldAlert size={16} />
              </div>
              <div className="space-y-1">
                <p className="text-[11px] font-bold">系统安全提示</p>
                <p className="text-[10px] text-text-muted leading-normal">
                  安装时 Windows 会弹出安全警告确认窗，请点击“是”以继续完成配置。
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
                  <span>正在安装...</span>
                </>
              ) : (
                <>
                  <Download size={18} />
                  <span>立即安装证书</span>
                </>
              )}
            </button>
            <button
              onClick={() => setShowCertModal(false)}
              className="w-full py-3 bg-background-hover text-text-secondary rounded-2xl text-xs font-bold hover:bg-background-soft transition-all"
            >
              稍后提醒我
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
};

export default Dashboard;
