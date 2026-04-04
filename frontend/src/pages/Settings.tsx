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
  OpenCertDir,
  RegenerateCert,
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
  title: string;
  desc?: string;
  icon?: React.ReactNode;
  children: React.ReactNode;
}> = ({ title, desc, icon, children }) => (
  <div className="flex items-center justify-between p-5 bg-background-card border border-border rounded-xl hover:border-accent/40 transition-all group">
    <div className="flex gap-4 items-center">
      <div className="w-10 h-10 rounded-2xl bg-background-hover flex items-center justify-center text-text-secondary group-hover:text-accent transition-colors">
        {icon || <Activity size={20} />}
      </div>
      <div>
        <h4 className="text-sm font-bold">{title}</h4>
        {desc && <p className="text-[11px] text-text-muted mt-0.5 leading-relaxed font-medium">{desc}</p>}
      </div>
    </div>
    <div className="shrink-0 ml-4">
      {children}
    </div>
  </div>
);

const Settings: React.FC = () => {
  const [port, setPort] = useState(8080);
  const [closeToTray, setCloseToTray] = useState(false);

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
    const p = await GetListenPort();
    const tray = await GetCloseToTray();
    const cf = await GetCloudflareConfig();

    setPort(p);
    setCloseToTray(tray);
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

  return (
    <div className="p-6 max-w-5xl mx-auto space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-700">
      <header className="flex justify-between items-end">
        <div>
          <h1 className="text-3xl font-black tracking-tighter">全局设置</h1>
        </div>
      </header>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        {/* Proxy Base Section */}
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

            <SettingItem
              title="上游 DOH"
              icon={<Globe size={20} />}
            >
              <div className="flex gap-2">
                <input
                  type="text"
                  value={cfConfig.doh_url}
                  onChange={(e) => setCfConfig({ ...cfConfig, doh_url: e.target.value })}
                  className="w-48 bg-background-soft border border-border px-3 py-1.5 rounded-xl text-xs font-bold focus:ring-2 focus:ring-accent outline-none"
                />
                <button onClick={handleSaveCF} className="px-3 py-1.5 bg-accent/10 text-accent rounded-xl text-[11px] font-bold hover:bg-accent hover:text-white transition-all">应用</button>
              </div>
            </SettingItem>

            <SettingItem
              title="启用 WARP"
              desc="关闭后不会按需拉起 WARP，也不会允许规则走 WARP 上游"
              icon={<Cloud size={20} />}
            >
              <button
                onClick={handleToggleWarpEnabled}
                className={`w-9 h-5 rounded-full transition-all relative ${cfConfig.warp_enabled ? "bg-success" : "bg-background-hover border border-border"}`}
              >
                <div className={`absolute top-0.5 w-4 h-4 bg-white rounded-full shadow-sm transition-transform duration-200 ${cfConfig.warp_enabled ? "translate-x-[18px] left-0" : "left-0.5"}`} />
              </button>
            </SettingItem>

            <SettingItem
              title="WARP Endpoint"
              desc="修改 WARP 连接出口地址，保存后会按当前启用状态生效"
              icon={<CloudLightning size={20} />}
            >
              <div className="flex gap-2">
                <input
                  type="text"
                  value={cfConfig.warp_endpoint || ''}
                  onChange={(e) => setCfConfig({ ...cfConfig, warp_endpoint: e.target.value })}
                  className="w-40 bg-background-soft border border-border px-3 py-1.5 rounded-xl text-xs font-bold focus:ring-2 focus:ring-accent outline-none"
                />
                <button onClick={handleSaveCF} className="px-3 py-1.5 bg-accent/10 text-accent rounded-xl text-[11px] font-bold hover:bg-accent hover:text-white transition-all">应用</button>
              </div>
            </SettingItem>
          </div>
        </section>

        {/* Security / Certs Section */}
        <section className="space-y-4">
          <div className="flex items-center gap-2 px-1 text-text-secondary">
            <ShieldAlert size={18} />
            <h3 className="text-sm font-bold uppercase tracking-wider">安全与证书</h3>
          </div>

          <div className="space-y-4">
            <SettingItem
              title="重新安装证书"
              desc="如浏览器证书报错，点此重新安装证书"
              icon={<RefreshCcw size={20} />}
            >
              <button onClick={() => { void RegenerateCert(); toast.success('证书任务已触发', '系统正在重新生成并安装证书。'); }} className="px-4 py-2 border border-border rounded-xl text-xs font-bold hover:bg-background-hover transition-all">重新安装</button>
            </SettingItem>

            <SettingItem
              title="浏览根证书"
              icon={<FolderOpen size={20} />}
            >
              <button onClick={() => OpenCertDir()} className="flex items-center gap-2 px-4 py-2 bg-accent/5 text-accent rounded-xl text-xs font-bold hover:bg-accent/10 transition-all">
                打开目录
              </button>
            </SettingItem>
          </div>
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
