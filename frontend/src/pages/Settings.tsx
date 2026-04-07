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
  Upload
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
  RemoveInvalidCFIPs
} from '../api/bindings';
import { toast } from '../lib/toast';

const SettingItem: React.FC<{
  title: React.ReactNode;
  desc?: React.ReactNode;
  icon?: React.ReactNode;
  children: React.ReactNode;
}> = ({ title, desc, icon, children }) => (
  <div className="flex items-start justify-between gap-5 p-5 bg-background-card border border-border rounded-xl hover:border-accent/40 transition-all group">
    <div className="flex flex-1 min-w-0 gap-4 items-center">
      <div className="w-10 h-10 rounded-2xl bg-background-hover flex items-center justify-center text-text-secondary group-hover:text-accent transition-colors shrink-0">
        {icon || <Activity size={20} />}
      </div>
      <div className="min-w-0">
        <h4 className="text-sm font-bold leading-snug">{title}</h4>
        {desc && <p className="text-[11px] text-text-muted mt-0.5 leading-relaxed font-medium break-words">{desc}</p>}
      </div>
    </div>
    <div className="shrink-0 self-center">
      {children}
    </div>
  </div>
);

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

const Settings: React.FC = () => {
  const [port, setPort] = useState(8080);
  const [closeToTray, setCloseToTray] = useState(false);
  const [autoStart, setAutoStart] = useState(false);
  const [showMainOnAutoStart, setShowMainOnAutoStart] = useState(true);
  const [autoEnableProxyOnAutoStart, setAutoEnableProxyOnAutoStart] = useState(false);

  // Cloudflare Config
  const [cfConfig, setCfConfig] = useState<any>({
    api_key: '',
    doh_url: 'https://1.1.1.1/dns-query',
    auto_update: true,
    warp_enabled: false,
    warp_endpoint: '162.159.199.2'
  });
  const [ipStats, setIpStats] = useState<any[]>([]);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isCheckingHealth, setIsCheckingHealth] = useState(false);
  const [caStatus, setCaStatus] = useState<any>({ Installed: false, CertPath: '', Platform: 'windows' });
  const [installedCerts, setInstalledCerts] = useState<any[]>([]);
  const [isCertBusy, setIsCertBusy] = useState(false);

  const loadIPStats = async () => {
    const stats = await GetCloudflareIPStats();
    setIpStats(stats || []);
  };

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

  const loadData = async () => {
    const [p, tray, autoStartEnabled, showMainEnabled, autoEnableProxyEnabled, cf, ca, certs] = await Promise.all([
      GetListenPort(),
      GetCloseToTray(),
      GetAutoStart(),
      GetShowMainWindowOnAutoStart(),
      GetAutoEnableProxyOnAutoStart(),
      GetCloudflareConfig(),
      GetCAInstallStatus(),
      GetInstalledCerts()
    ]);

    setPort(p);
    setCloseToTray(tray);
    setAutoStart(autoStartEnabled);
    setShowMainOnAutoStart(showMainEnabled);
    setAutoEnableProxyOnAutoStart(autoEnableProxyEnabled);
    setCaStatus(ca || { Installed: false, CertPath: '', Platform: 'windows' });
    setInstalledCerts(certs || []);
    setCfConfig(cf || {
      api_key: '',
      doh_url: 'https://1.1.1.1/dns-query',
      auto_update: true,
      warp_enabled: false,
      warp_endpoint: '162.159.199.2'
    });
    await loadIPStats();
  };

  useEffect(() => {
    loadData();
    const timer = setInterval(async () => {
      await loadIPStats();
    }, 5000);
    return () => clearInterval(timer);
  }, []);

  const handleSavePort = async () => {
    await SetListenPort(port);
    toast.success('端口已更新', `新的本地监听端口为 ${port}。`);
  };

  const handleToggleTray = async (val: boolean) => {
    setCloseToTray(val);
    await SetCloseToTray(val);
    toast.success('托盘行为已更新', val ? '关闭窗口时将最小化到托盘。' : '关闭窗口时将直接退出程序。');
  };

  const handleToggleAutoStart = async (val: boolean) => {
    setAutoStart(val);
    try {
      await SetAutoStart(val);
      toast.success('开机自启动已更新', val ? '系统登录后将自动启动 SniShaper。' : '已关闭开机自启动。');
    } catch (err: any) {
      setAutoStart(!val);
      toast.error('开机自启动更新失败', String(err));
    }
  };

  const handleToggleShowMainOnAutoStart = async (val: boolean) => {
    setShowMainOnAutoStart(val);
    try {
      await SetShowMainWindowOnAutoStart(val);
      toast.success('启动显示行为已更新', val ? '开机自启动时会显示主界面。' : '开机自启动时将直接在托盘中运行。');
    } catch (err: any) {
      setShowMainOnAutoStart(!val);
      toast.error('启动显示行为更新失败', String(err));
    }
  };

  const handleToggleAutoEnableProxyOnAutoStart = async (val: boolean) => {
    setAutoEnableProxyOnAutoStart(val);
    try {
      await SetAutoEnableProxyOnAutoStart(val);
      toast.success('启动代理行为已更新', val ? '开机自启动时会自动开启代理与系统代理。' : '开机自启动时将仅启动程序本身。');
    } catch (err: any) {
      setAutoEnableProxyOnAutoStart(!val);
      toast.error('启动代理行为更新失败', String(err));
    }
  };

  const handleSaveCF = async () => {
    await UpdateCloudflareConfig(cfConfig);
    await loadData();
    toast.success('Cloudflare 配置已保存');
  };

  const handleImport = async () => {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = '.json';
    input.onchange = async (e: any) => {
      const file = e.target.files[0];
      if (!file) return;
      const content = await file.text();
      try {
        const summary = await ImportConfigWithSummary(content);
        toast.success(
          '导入成功',
          `规则 +${summary.rules_added}，ECH 配置 +${summary.ech_profiles_added}，节点 +${summary.upstreams_added}。`,
          4200
        );
        loadData();
      } catch (err: any) {
        toast.error('导入失败', String(err));
      }
    };
    input.click();
  };

  const handleFetchIPs = async () => {
    setIsRefreshing(true);
    try {
      await ForceFetchCloudflareIPs();
      await loadData();
      toast.success('IP 池已刷新', 'Cloudflare 备选 IP 列表已更新。');
    } finally {
      setIsRefreshing(false);
    }
  };

  const handleToggleAutoUpdate = async () => {
    const nextConfig = { ...cfConfig, auto_update: !cfConfig.auto_update };
    setCfConfig(nextConfig);
    await UpdateCloudflareConfig(nextConfig);
    await loadData();
    toast.success('自动更新已切换', nextConfig.auto_update ? '将自动维护优选 IP。' : '已关闭自动维护优选 IP。');
  };

  const handleToggleWarpEnabled = async () => {
    const nextConfig = { ...cfConfig, warp_enabled: !cfConfig.warp_enabled };
    setCfConfig(nextConfig);
    await UpdateCloudflareConfig(nextConfig);
    await loadData();
    toast.success('WARP 状态已更新', nextConfig.warp_enabled ? '允许按需使用 WARP 上游。' : '已禁用 WARP 上游。');
  };

  const handleHealthCheck = async () => {
    setIsCheckingHealth(true);
    try {
      await TriggerCFHealthCheck();
      // The backend runs checks asynchronously, so refresh a few times
      // to surface updated latency values as soon as they land.
      await loadIPStats();
      window.setTimeout(() => { void loadIPStats(); }, 1200);
      window.setTimeout(() => { void loadIPStats(); }, 3000);
      toast.info('健康检查已启动', '后台会异步更新各个 IP 的延迟结果。');
    } finally {
      window.setTimeout(() => setIsCheckingHealth(false), 1200);
    }
  };

  const handleRegenerateCert = async () => {
    setIsCertBusy(true);
    try {
      await RegenerateCert();
      await loadData();
      toast.success('证书已重新安装', '新的根证书已重新生成并导入系统。');
    } catch (err: any) {
      toast.error('重新安装失败', String(err));
    } finally {
      setIsCertBusy(false);
    }
  };

  const handleUninstallCert = async (token: string) => {
    if (!token) return;
    setIsCertBusy(true);
    try {
      await UninstallCert(token);
      await loadData();
      toast.success('证书已卸载');
    } catch (err: any) {
      toast.error('卸载失败', String(err));
    } finally {
      setIsCertBusy(false);
    }
  };

  return (
    <div className="p-6 max-w-5xl mx-auto space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-700">
      <header className="flex justify-between items-end">
        <div>
          <h1 className="text-3xl font-black tracking-tighter">设置</h1>
        </div>
      </header>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        {/* Proxy Base Section */}
        <div className="space-y-8">
          <section className="space-y-4">
            <div className="flex items-center gap-2 px-1 text-text-secondary">
              <Anchor size={18} />
              <h3 className="text-sm font-bold uppercase tracking-wider">代理核心</h3>
            </div>

            <div className="space-y-4">
              <SettingItem
                title="本地端口"
                icon={<Monitor size={20} />}
              >
                <div className="flex gap-2">
                  <input
                    type="number"
                    value={port}
                    onChange={(e) => setPort(parseInt(e.target.value))}
                    className="w-20 bg-background-soft border border-border px-3 py-1.5 rounded-xl text-sm font-bold focus:ring-2 focus:ring-accent outline-none"
                  />
                  <button onClick={handleSavePort} className="px-3 py-1.5 bg-accent/10 text-accent rounded-xl text-[11px] font-bold hover:bg-accent hover:text-white transition-all">应用</button>
                </div>
              </SettingItem>

              <SettingItem
                title="最小化到托盘"
                desc="关闭主窗口时程序将在系统通知区域继续运行"
                icon={<BellRing size={20} />}
              >
                <button
                  onClick={() => handleToggleTray(!closeToTray)}
                  className={`w-9 h-5 rounded-full transition-all relative ${closeToTray ? "bg-accent" : "bg-background-hover border border-border"}`}
                >
                  <div className={`absolute top-0.5 w-4 h-4 bg-white rounded-full shadow-sm transition-transform duration-200 ${closeToTray ? "translate-x-[18px] left-0" : "left-0.5"}`} />
                </button>
              </SettingItem>
            </div>
          </section>

          <section className="space-y-4">
            <div className="flex items-center gap-2 px-1 text-text-secondary">
              <Cpu size={18} />
              <h3 className="text-sm font-bold uppercase tracking-wider">启动行为</h3>
            </div>

            <div className="space-y-4">
              <SettingItem
                title="开机自启动"
                desc="系统登录后自动启动 SniShaper"
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
                title="自启动时显示主界面"
                desc="关闭后仅显示托盘图标，主界面保持隐藏"
                icon={<SettingsIcon size={20} />}
              >
                <button
                  onClick={() => handleToggleShowMainOnAutoStart(!showMainOnAutoStart)}
                  className={`w-9 h-5 rounded-full transition-all relative ${showMainOnAutoStart ? "bg-accent" : "bg-background-hover border border-border"}`}
                >
                  <div className={`absolute top-0.5 w-4 h-4 bg-white rounded-full shadow-sm transition-transform duration-200 ${showMainOnAutoStart ? "translate-x-[18px] left-0" : "left-0.5"}`} />
                </button>
              </SettingItem>

              <SettingItem
                title="自启动时自动开启代理"
                desc="开机自启动拉起程序后，自动启动代理核心并打开系统代理"
                icon={<Activity size={20} />}
              >
                <button
                  onClick={() => handleToggleAutoEnableProxyOnAutoStart(!autoEnableProxyOnAutoStart)}
                  className={`w-9 h-5 rounded-full transition-all relative ${autoEnableProxyOnAutoStart ? "bg-accent" : "bg-background-hover border border-border"}`}
                >
                  <div className={`absolute top-0.5 w-4 h-4 bg-white rounded-full shadow-sm transition-transform duration-200 ${autoEnableProxyOnAutoStart ? "translate-x-[18px] left-0" : "left-0.5"}`} />
                </button>
              </SettingItem>
            </div>
          </section>
        </div>

        {/* Upstream / Warp Section */}
        <section className="space-y-4">
          <div className="flex items-center gap-2 px-1 text-text-secondary">
            <Cloud size={18} />
            <h3 className="text-sm font-bold uppercase tracking-wider">上游与 WARP</h3>
          </div>

          <div className="space-y-4">
            <StackedSettingItem
              title={<span className="whitespace-nowrap">上游 DoH</span>}
              icon={<Globe size={20} />}
            >
              <div className="flex items-start gap-2">
                <textarea
                  rows={2}
                  value={cfConfig.doh_url}
                  onChange={(e) => setCfConfig({ ...cfConfig, doh_url: e.target.value })}
                  className="flex-1 min-w-0 resize-none bg-background-soft border border-border px-3 py-2 rounded-xl text-xs font-bold leading-relaxed focus:ring-2 focus:ring-accent outline-none"
                />
                <button onClick={handleSaveCF} className="px-3 py-2 bg-accent/10 text-accent rounded-xl text-[11px] font-bold hover:bg-accent hover:text-white transition-all">应用</button>
              </div>
            </StackedSettingItem>

            <SettingItem
              title="启用 WARP"
              desc="禁用时，所有 WARP 规则将不会生效"
              icon={<Cloud size={20} />}
            >
              <button
                onClick={handleToggleWarpEnabled}
                className={`w-9 h-5 rounded-full transition-all relative ${cfConfig.warp_enabled ? "bg-success" : "bg-background-hover border border-border"}`}
              >
                <div className={`absolute top-0.5 w-4 h-4 bg-white rounded-full shadow-sm transition-transform duration-200 ${cfConfig.warp_enabled ? "translate-x-[18px] left-0" : "left-0.5"}`} />
              </button>
            </SettingItem>

            <StackedSettingItem
              title={<span className="whitespace-nowrap">WARP Endpoint</span>}
              icon={<CloudLightning size={20} />}
            >
              <div className="flex gap-2">
                <input
                  type="text"
                  value={cfConfig.warp_endpoint || ''}
                  onChange={(e) => setCfConfig({ ...cfConfig, warp_endpoint: e.target.value })}
                  className="flex-1 min-w-0 bg-background-soft border border-border px-3 py-2 rounded-xl text-xs font-bold focus:ring-2 focus:ring-accent outline-none"
                />
                <button onClick={handleSaveCF} className="px-3 py-2 bg-accent/10 text-accent rounded-xl text-[11px] font-bold hover:bg-accent hover:text-white transition-all">应用</button>
              </div>
            </StackedSettingItem>
          </div>
        </section>

        {/* Security / Certs Section */}
        <section className="lg:col-span-2 space-y-4">
          <div className="flex items-center gap-2 px-1 text-text-secondary">
            <ShieldAlert size={18} />
            <h3 className="text-sm font-bold uppercase tracking-wider">安全与证书</h3>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <SettingItem
              title="重新安装证书"
              desc="如浏览器证书报错，点此重新安装证书"
              icon={<RefreshCcw size={20} />}
            >
              <button
                onClick={handleRegenerateCert}
                disabled={isCertBusy}
                className="px-4 py-2 border border-border rounded-xl text-xs font-bold hover:bg-background-hover transition-all disabled:opacity-60"
              >
                {isCertBusy ? '处理中...' : '重新安装'}
              </button>
            </SettingItem>

            <SettingItem
              title="浏览根证书"
              desc={caStatus?.CertPath || undefined}
              icon={<FolderOpen size={20} />}
            >
              <button onClick={() => OpenCertDir()} className="flex items-center gap-2 px-4 py-2 bg-accent/5 text-accent rounded-xl text-xs font-bold hover:bg-accent/10 transition-all">
                打开目录
              </button>
            </SettingItem>
          </div>

          <StackedSettingItem
            title="已安装证书管理"
            desc={caStatus?.Installed ? '已检测到系统中存在 SniShaper 证书。' : '当前未检测到已安装的 SniShaper 证书。'}
            icon={<ShieldAlert size={20} />}
          >
            <div className="space-y-3">
              <div className={`text-[11px] font-bold ${caStatus?.Installed ? 'text-success' : 'text-text-muted'}`}>
                {caStatus?.Installed ? `已安装 ${installedCerts.length} 个证书项` : '未安装'}
              </div>
              {installedCerts.length === 0 ? (
                <div className="rounded-xl border border-border/40 bg-background-card px-4 py-5 text-[11px] text-text-muted">
                  暂无可管理的已安装证书。
                </div>
              ) : (
                <div className="space-y-2 max-h-64 overflow-y-auto pr-1">
                  {installedCerts.map((cert) => (
                    <div key={cert.token} className="flex items-center justify-between gap-4 rounded-2xl border border-border/40 bg-background-card px-5 py-4">
                      <div className="min-w-0 flex-1 space-y-1">
                        <div className="text-xs font-bold break-all">{cert.subject}</div>
                        <div className="text-[10px] text-text-muted break-all">
                          {cert.storeLocation} / {cert.storeName} / {cert.thumbprint}
                        </div>
                        <div className="text-[10px] text-text-muted">
                          到期时间: {cert.notAfter || '未知'}
                        </div>
                      </div>
                      <button
                        onClick={() => handleUninstallCert(cert.token)}
                        disabled={isCertBusy}
                        className="shrink-0 inline-flex min-w-[92px] items-center justify-center gap-2 rounded-xl bg-danger/12 px-4 py-2 text-[11px] font-black text-danger shadow-[inset_0_0_0_1px_rgba(248,81,73,0.24)] hover:bg-danger/18 disabled:opacity-60"
                      >
                        <Trash2 size={12} />
                        卸载
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
              <h3 className="text-sm font-bold uppercase tracking-wider">Cloudflare 优选 IP</h3>
            </div>
            <div className="flex gap-2">
              <button onClick={handleHealthCheck} className="text-[10px] font-black uppercase text-accent hover:underline disabled:opacity-50" disabled={isCheckingHealth}>
                {isCheckingHealth ? "检测中..." : "开始健康检查"}
              </button>
              <button onClick={async () => { await RemoveInvalidCFIPs(); await loadIPStats(); toast.success('已清理失效 IP'); }} className="text-[10px] font-black uppercase text-danger hover:underline">清理失效 IP</button>
            </div>
          </div>

          <div className="bg-background-card border border-border rounded-2xl overflow-hidden">
            <div className="grid grid-cols-1 md:grid-cols-3">
              <div className="p-8 border-r border-border space-y-6">
                <div className="space-y-4">
                  <div className="flex items-center justify-between p-1">
                    <span className="text-xs font-bold">自动更新优选 IP</span>
                    <button
                      onClick={handleToggleAutoUpdate}
                      className={`w-9 h-5 rounded-full transition-all relative ${cfConfig.auto_update ? "bg-success" : "bg-background-hover border border-border"}`}
                    >
                      <div className={`absolute top-0.5 w-4 h-4 bg-white rounded-full shadow-sm transition-transform duration-200 ${cfConfig.auto_update ? "translate-x-[18px] left-0" : "left-0.5"}`} />
                    </button>
                  </div>
                </div>
                <div className="flex gap-3">
                  <button onClick={handleFetchIPs} disabled={isRefreshing} className="flex-1 py-2.5 bg-accent text-white rounded-xl text-xs font-black shadow-lg shadow-accent/20 hover:scale-[1.02] transition-all flex items-center justify-center gap-2">
                    {isRefreshing ? <RefreshCcw size={16} className="animate-spin" /> : <Download size={16} />}
                    <span>立即更新备选 IP 池</span>
                  </button>
                </div>
              </div>

              <div className="md:col-span-2 p-6 bg-background-soft/30">
                <div className="flex items-center justify-between mb-4 px-2">
                  <h4 className="text-[10px] font-black uppercase text-text-muted tracking-widest">当前可用 IP 池 ({ipStats.length})</h4>
                  <Zap size={14} className="text-warning animate-pulse" />
                </div>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 max-h-[320px] overflow-y-auto px-2 pb-4 scrollbar-thin">
                  {ipStats.length === 0 ? (
                    <div className="col-span-full py-12 flex flex-col items-center justify-center text-text-muted opacity-40">
                      <AlertCircle size={32} />
                      <span className="text-[10px] font-bold uppercase mt-2">IP 池为空，请点击左侧下载</span>
                    </div>
                  ) : (
                    ipStats.map((ip, i) => (
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
