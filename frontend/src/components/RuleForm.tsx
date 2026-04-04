import React, { useState, useEffect } from 'react';
import { 
  Zap, 
  Globe, 
  ShieldCheck, 
  Monitor, 
  Server,
  Cloud,
  ChevronDown,
  ChevronUp,
  Plus,
  Trash2,
  Lock,
  Settings,
  AlertCircle
} from 'lucide-react';
import { AddSiteGroup, UpdateSiteGroup, GetECHProfiles } from '../api/bindings';

interface RuleFormProps {
  initialData?: any;
  onSuccess: () => void;
  onCancel: () => void;
}

const MODES = [
  { id: 'mitm', label: 'MITM', icon: <Zap size={14} />, desc: '修改 SNI 或注入 ECH 配置' },
  { id: 'server', label: 'Server', icon: <Server size={14} />, desc: '连接上游反代' },
  { id: 'tls-rf', label: 'TLS 分片', icon: <Monitor size={14} />, desc: 'TLS 分片' },
  { id: 'warp', label: 'WARP', icon: <Cloud size={14} />, desc: '使用 Cloudflare WARP Masque 连接上游' },
  { id: 'quic', label: 'QUIC', icon: <Zap size={14} />, desc: 'QUIC 分片混淆' },
  { id: 'transparent', label: '透传', icon: <Monitor size={14} />, desc: '不解密 TLS，仅透传到目标或指定上游' }
];

const RuleForm: React.FC<RuleFormProps> = ({ initialData, onSuccess, onCancel }) => {
  const [formData, setFormData] = useState<any>({
    id: '',
    name: '',
    website: '',
    mode: 'mitm',
    upstream: 'DIRECT',
    domains: [] as string[],
    dns_mode: '',
    sni_fake: '',
    enabled: true,
    ech_enabled: false,
    ech_profile_id: '',
    ech_domain: '',
    use_cf_pool: false,
    cert_verify: {
      mode: '',
      allow_unknown_authority: false,
      allowed_names: [],
      allowed_suffixes: [],
      allowed_spki: []
    }
  });
  const [domainInput, setDomainInput] = useState('');
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [echProfiles, setEchProfiles] = useState<any[]>([]);

  useEffect(() => {
    const loadProfiles = async () => {
      const ps = await GetECHProfiles();
      setEchProfiles(ps || []);
    };
    loadProfiles();

    if (initialData) {
      // Ensure nested objects exist to avoid crashes
      const data = { ...initialData };
      if (!data.cert_verify) {
        data.cert_verify = { mode: '', allow_unknown_authority: false, allowed_names: [], allowed_suffixes: [], allowed_spki: [] };
      }
      setFormData(data);
    }
  }, [initialData]);

  const handleAddDomain = () => {
    if (!domainInput.trim()) return;
    const split = domainInput.split(/[\s,;]+/).filter(Boolean);
    setFormData((prev: any) => ({
      ...prev,
      domains: [...new Set([...(prev.domains || []), ...split])]
    }));
    setDomainInput('');
  };

  const handleRemoveDomain = (idx: number) => {
    setFormData((prev: any) => ({
      ...prev,
      domains: prev.domains.filter((_: any, i: number) => i !== idx)
    }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (formData.id) {
      await UpdateSiteGroup(formData);
    } else {
      await AddSiteGroup(formData);
    }
    onSuccess();
  };

  return (
    <form id="rule-form" onSubmit={handleSubmit} className="space-y-4 text-text-primary px-1 pb-2">
      {/* Basic Info Container */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="space-y-1.5">
          <label className="text-[10px] font-black text-text-muted uppercase tracking-widest px-1 flex items-center gap-1.5">
            <Zap size={10} className="text-accent" /> 描述名称
          </label>
          <input 
            type="text" 
            required
            value={formData.name}
            onChange={(e) => setFormData({...formData, name: e.target.value})}
            placeholder="例如: Google 服务"
            className="w-full bg-background-soft border border-border px-4 py-2 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none font-medium transition-all"
          />
        </div>
        <div className="space-y-1.5">
          <label className="text-[10px] font-black text-text-muted uppercase tracking-widest px-1 flex items-center gap-1.5">
             <Settings size={10} className="text-accent" /> 网站分组 (Website)
          </label>
          <input 
            type="text" 
            value={formData.website}
            onChange={(e) => setFormData({...formData, website: e.target.value})}
            placeholder="google"
            className="w-full bg-background-soft border border-border px-4 py-2 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none font-medium transition-all"
          />
        </div>
      </div>

      {/* Mode Selection Grid */}
      <div className="space-y-3">
          <label className="text-[10px] font-black text-text-muted uppercase tracking-widest px-1">代理模式</label>
          <div className="grid grid-cols-2 lg:grid-cols-3 gap-3">
              {MODES.map((m) => (
                  <div 
                    key={m.id}
                    onClick={() => setFormData({...formData, mode: m.id})}
                    className={`p-3 rounded-2xl border transition-all cursor-pointer flex flex-col gap-1 items-start relative overflow-hidden group ${
                        formData.mode === m.id 
                        ? "bg-accent/10 border-accent shadow-sm" 
                        : "bg-background-soft/40 border-border hover:border-accent/40"
                    }`}
                  >
                        <div className="flex items-center gap-2 z-10">
                            <div className={formData.mode === m.id ? "text-accent" : "text-text-muted"}>{m.icon}</div>
                            <span className={`text-[12px] font-black ${formData.mode === m.id ? "text-accent" : "text-text-primary"}`}>{m.label}</span>
                        </div>
                        <span className="text-[9px] text-text-muted font-medium leading-tight z-10">{m.desc}</span>
                        {formData.mode === m.id && <div className="absolute -right-2 -bottom-2 opacity-10 text-accent transform rotate-12">{m.icon}</div>}
                  </div>
              ))}
          </div>
      </div>

      {/* Domain List Management */}
      <div className="space-y-3">
          <label className="text-[10px] font-black text-text-muted uppercase tracking-widest px-1">匹配域名清单 (支持通配符)</label>
          <div className="relative group">
              <input 
                type="text" 
                value={domainInput}
                onChange={(e) => setDomainInput(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && (e.preventDefault(), handleAddDomain())}
                placeholder="输入域名后按回车，支持批量空格或逗号分割..."
                className="w-full bg-background-soft border border-border px-4 py-2 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none font-medium pr-12 transition-all placeholder:text-text-muted/40"
              />
              <button 
                type="button"
                onClick={handleAddDomain}
                className="absolute right-2 top-1.5 p-1.5 rounded-lg bg-accent text-white shadow-lg shadow-accent/20 hover:scale-105 active:scale-95 transition-all"
              >
                <Plus size={20} />
              </button>
          </div>
          <div className="flex flex-wrap gap-2 max-h-[120px] overflow-y-auto p-3 bg-background-soft/40 border border-border/50 rounded-2xl custom-scrollbar">
              {(!formData.domains || formData.domains.length === 0) ? (
                  <span className="text-[11px] text-text-muted italic px-2">尚未添加任何匹配域名</span>
              ) : (
                  formData.domains.map((d: any, i: number) => (
                      <div key={i} className="flex items-center gap-1.5 px-3 py-1 bg-background-card border border-border rounded-full text-[11px] font-bold group hover:border-danger/40 transition-all shadow-sm">
                          {d}
                          <button 
                            type="button" 
                            onClick={() => handleRemoveDomain(i)}
                            className="text-text-muted hover:text-danger opacity-0 group-hover:opacity-100 transition-opacity"
                          >
                              <Trash2 size={12} />
                          </button>
                      </div>
                  ))
              )}
          </div>
      </div>

      {/* Advanced Settings Toggle */}
      <div className="pt-2">
          <button 
            type="button"
            onClick={() => setShowAdvanced(!showAdvanced)}
            className="flex items-center gap-2 text-accent text-xs font-black uppercase tracking-[0.15em] hover:opacity-80 transition-opacity"
          >
            {showAdvanced ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
            {showAdvanced ? "折叠高级设置" : "展开高级设置"}
          </button>
      </div>

      {showAdvanced && (
          <div className="space-y-6 pt-2 animate-in slide-in-from-top-2 fade-in duration-300">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div className="space-y-1.5">
                    <label className="text-[10px] font-black text-text-muted uppercase tracking-widest px-1">上游服务器 (Upstream)</label>
                    <input 
                        type="text" 
                        value={formData.upstream}
                        onChange={(e) => setFormData({...formData, upstream: e.target.value})}
                        placeholder="DIRECT 或 1.2.3.4:443"
                        className="w-full bg-background-soft border border-border px-4 py-2 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none font-medium"
                    />
                </div>
                <div className="space-y-1.5">
                    <label className="text-[10px] font-black text-text-muted uppercase tracking-widest px-1">DNS 解析策略</label>
                    <select 
                        value={formData.dns_mode}
                        onChange={(e) => setFormData({...formData, dns_mode: e.target.value})}
                        className="w-full bg-background-soft border border-border px-4 py-2 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none font-medium appearance-none"
                    >
                        <option value="">默认 / 系统顺序</option>
                        <option value="prefer_ipv4">优先 IPv4</option>
                        <option value="prefer_ipv6">优先 IPv6</option>
                        <option value="ipv4_only">仅 IPv4</option>
                        <option value="ipv6_only">仅 IPv6</option>
                    </select>
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div className="space-y-1.5">
                    <label className="text-[10px] font-black text-text-muted uppercase tracking-widest px-1">Fake SNI 映射</label>
                    <input 
                        type="text" 
                        value={formData.sni_fake}
                        onChange={(e) => setFormData({...formData, sni_fake: e.target.value})}
                        placeholder="例如: github-com.mapped"
                        className="w-full bg-background-soft border border-border px-4 py-2 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none font-medium"
                    />
                </div>
                <div className="space-y-1.5">
                    <label className="text-[10px] font-black text-text-muted uppercase tracking-widest px-1">ECH 配置文件 (Profile)</label>
                    <select 
                        value={formData.ech_profile_id}
                        onChange={(e) => setFormData({...formData, ech_profile_id: e.target.value})}
                        className="w-full bg-background-soft border border-border px-4 py-2 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none font-medium appearance-none"
                    >
                        <option value="">(无 - 自动从 DoH fallback)</option>
                        {echProfiles.map(p => (
                            <option key={p.id} value={p.id}>{p.name}</option>
                        ))}
                    </select>
                </div>
              </div>

              <div className="flex flex-wrap gap-4 p-4 bg-background-soft/30 border border-border/40 rounded-2xl">
                  <label className="flex items-center gap-2 cursor-pointer group">
                      <input 
                        type="checkbox" 
                        checked={formData.enabled}
                        onChange={(e) => setFormData({...formData, enabled: e.target.checked})}
                        className="w-4 h-4 rounded border-border text-accent focus:ring-accent bg-background-card"
                      />
                      <span className="text-xs font-bold text-text-secondary group-hover:text-text-primary transition-colors">开启规则</span>
                  </label>
                  <label className="flex items-center gap-2 cursor-pointer group">
                      <input 
                        type="checkbox" 
                        checked={formData.ech_enabled}
                        onChange={(e) => setFormData({...formData, ech_enabled: e.target.checked})}
                        className="w-4 h-4 rounded border-border text-accent focus:ring-accent bg-background-card"
                      />
                      <span className="text-xs font-bold text-text-secondary group-hover:text-text-primary transition-colors flex items-center gap-1">
                          <Lock size={12} className="text-cyan-500" /> ECH 加密
                      </span>
                  </label>
                  <label className="flex items-center gap-2 cursor-pointer group">
                      <input 
                        type="checkbox" 
                        checked={formData.use_cf_pool}
                        onChange={(e) => setFormData({...formData, use_cf_pool: e.target.checked})}
                        className="w-4 h-4 rounded border-border text-accent focus:ring-accent bg-background-card"
                      />
                      <span className="text-xs font-bold text-text-secondary group-hover:text-text-primary transition-colors">优选 IP 池</span>
                  </label>
              </div>

              {/* Advanced Cert Verify */}
              <div className="space-y-3 p-4 border border-warning/20 bg-warning/5 rounded-2xl relative">
                  <div className="flex items-center gap-2 text-warning mb-2">
                      <AlertCircle size={16} />
                      <span className="text-xs font-bold uppercase tracking-wider">证书校验策略 (高级)</span>
                  </div>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <div className="space-y-1.5">
                          <label className="text-[9px] font-bold text-text-muted">验证模式</label>
                          <select 
                             value={formData.cert_verify.mode}
                             onChange={(e) => setFormData({...formData, cert_verify: {...formData.cert_verify, mode: e.target.value}})}
                             className="w-full bg-background-card border border-border px-3 py-2 rounded-lg text-xs"
                          >
                                <option value="">默认安全校验</option>
                                <option value="allow_names">允许证书名称列表</option>
                                <option value="allow_suffixes">允许后缀匹配</option>
                                <option value="allow_spki">允许 SPKI 指纹</option>
                                <option value="chain_only">仅校验链路完整性</option>
                          </select>
                      </div>
                      <div className="flex items-center h-full pt-4">
                          <label className="flex items-center gap-2 cursor-pointer">
                              <input 
                                type="checkbox" 
                                checked={formData.cert_verify.allow_unknown_authority}
                                onChange={(e) => setFormData({...formData, cert_verify: {...formData.cert_verify, allow_unknown_authority: e.target.checked}})}
                                className="w-4 h-4 rounded border-border text-warning focus:ring-warning bg-background-card"
                              />
                              <span className="text-[11px] font-bold text-text-secondary">允许未知签发者 (跳过 CA 验证)</span>
                          </label>
                      </div>
                  </div>
              </div>
          </div>
      )}
    </form>
  );
};

export default RuleForm;
