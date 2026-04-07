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

const DNS_OPTIONS = [
  { id: '', label: '默认', desc: '系统顺序' },
  { id: 'prefer_ipv4', label: 'IPv4 优先', desc: '优先 IPv4' },
  { id: 'prefer_ipv6', label: 'IPv6 优先', desc: '优先 IPv6' },
  { id: 'ipv4_only', label: '仅 IPv4', desc: '禁用 IPv6' },
  { id: 'ipv6_only', label: '仅 IPv6', desc: '禁用 IPv4' }
];

const CERT_VERIFY_MODES = [
  { id: '', label: '默认', desc: '按系统默认证书校验' },
  { id: 'strict_real', label: '严格', desc: '严格校验真实域名' },
  { id: 'allow_names', label: '名称', desc: '允许指定证书名称' },
  { id: 'allow_suffixes', label: '后缀', desc: '允许指定域名后缀' },
  { id: 'allow_spki', label: 'SPKI', desc: '允许指定公钥指纹' },
  { id: 'chain_only', label: '链路', desc: '只校验证书链' }
];

const createDefaultCertVerify = () => ({
  mode: '',
  names: [] as string[],
  suffixes: [] as string[],
  spki_sha256: [] as string[],
  allow_unknown_authority: false
});

const splitListInput = (value: string) =>
  value
    .split(/[\n,;]+/)
    .map((item) => item.trim())
    .filter(Boolean);

const joinListInput = (items: string[] | undefined) => (items || []).join('\n');

const normalizeCertVerify = (value: any) => {
  const next = createDefaultCertVerify();
  if (!value || typeof value !== 'object') {
    return next;
  }

  next.mode = String(value.mode || '').trim();
  next.names = Array.isArray(value.names)
    ? value.names
    : Array.isArray(value.allowed_names)
      ? value.allowed_names
      : [];
  next.suffixes = Array.isArray(value.suffixes)
    ? value.suffixes
    : Array.isArray(value.allowed_suffixes)
      ? value.allowed_suffixes
      : [];
  next.spki_sha256 = Array.isArray(value.spki_sha256)
    ? value.spki_sha256
    : Array.isArray(value.allowed_spki)
      ? value.allowed_spki
      : [];
  next.allow_unknown_authority = Boolean(value.allow_unknown_authority);
  return next;
};

const RuleForm: React.FC<RuleFormProps> = ({ initialData, onSuccess, onCancel }) => {
  const [formData, setFormData] = useState<any>({
    id: '',
    name: '',
    website: '',
    mode: 'mitm',
    upstream: '',
    domains: [] as string[],
    dns_mode: '',
    sni_fake: '',
    enabled: true,
    ech_enabled: false,
    ech_profile_id: '',
    ech_domain: '',
    use_cf_pool: false,
    cert_verify: createDefaultCertVerify()
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
      if (String(data.upstream || '').trim().toUpperCase() === 'DIRECT') {
        data.upstream = '';
      }
      data.cert_verify = normalizeCertVerify(data.cert_verify);
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
    const payload = {
      ...formData,
      cert_verify: normalizeCertVerify(formData.cert_verify),
      upstream: String(formData.upstream || '').trim().toUpperCase() === 'DIRECT'
        ? ''
        : String(formData.upstream || '').trim()
    };
    if (formData.id) {
      await UpdateSiteGroup(payload);
    } else {
      await AddSiteGroup(payload);
    }
    onSuccess();
  };

  const certVerifyMode = String(formData.cert_verify?.mode || '').trim();
  const setCertVerify = (patch: Record<string, any>) => setFormData({
    ...formData,
    cert_verify: {
      ...formData.cert_verify,
      ...patch
    }
  });
  const toggleBooleanField = (field: 'enabled' | 'ech_enabled' | 'use_cf_pool') =>
    setFormData({ ...formData, [field]: !formData[field] });

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
                        : "bg-background-card border-border/50 hover:border-accent/40"
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
          <div className="flex flex-wrap gap-2 max-h-[120px] overflow-y-auto p-3 bg-background-card border border-border/40 rounded-2xl custom-scrollbar">
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
                        placeholder="留空直连，或填写 1.2.3.4:443"
                        className="w-full bg-background-soft border border-border px-4 py-2 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none font-medium"
                    />
                </div>
                <div className="space-y-1.5">
                    <label className="text-[10px] font-black text-text-muted uppercase tracking-widest px-1">DNS 解析策略</label>
                    <div className="grid grid-cols-2 gap-2">
                      {DNS_OPTIONS.map((option) => {
                        const active = String(formData.dns_mode || '') === option.id;
                        return (
                          <button
                            key={option.id || 'default'}
                            type="button"
                            onClick={() => setFormData({ ...formData, dns_mode: option.id })}
                            className={`rounded-xl border px-3 py-3 text-left transition-all ${
                              active
                                ? 'border-accent/40 bg-accent/10 text-accent shadow-[inset_0_0_0_1px_rgba(47,129,247,0.14)]'
                                : 'border-border/40 bg-background-card text-text-secondary hover:border-accent/30 hover:text-text-primary'
                            }`}
                          >
                            <div className="text-[11px] font-black tracking-wide">{option.label}</div>
                            <div className="mt-1 text-[10px] leading-relaxed opacity-80">{option.desc}</div>
                          </button>
                        );
                      })}
                    </div>
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
                    <div className="space-y-2">
                      <button
                        type="button"
                        onClick={() => setFormData({ ...formData, ech_profile_id: '' })}
                        className={`w-full rounded-xl border px-4 py-3 text-left transition-all ${
                          !formData.ech_profile_id
                            ? 'border-accent/40 bg-accent/10 text-accent shadow-[inset_0_0_0_1px_rgba(47,129,247,0.14)]'
                            : 'border-border/40 bg-background-card text-text-secondary hover:border-accent/30 hover:text-text-primary'
                        }`}
                      >
                        <div className="text-[11px] font-black tracking-wide">自动</div>
                        <div className="mt-1 text-[10px] opacity-80">无固定 Profile，按 DoH 自动回退</div>
                      </button>
                      {echProfiles.length > 0 && (
                        <div className="grid grid-cols-1 gap-2 max-h-44 overflow-y-auto pr-1">
                          {echProfiles.map((p) => {
                            const active = formData.ech_profile_id === p.id;
                            return (
                              <button
                                key={p.id}
                                type="button"
                                onClick={() => setFormData({ ...formData, ech_profile_id: p.id })}
                                className={`rounded-xl border px-4 py-3 text-left transition-all ${
                                  active
                                    ? 'border-accent/40 bg-accent/10 text-accent shadow-[inset_0_0_0_1px_rgba(47,129,247,0.14)]'
                                    : 'border-border/40 bg-background-card text-text-secondary hover:border-accent/30 hover:text-text-primary'
                                }`}
                              >
                                <div className="text-[11px] font-black tracking-wide">{p.name}</div>
                                <div className="mt-1 text-[10px] opacity-80">{p.discovery_domain || '手动配置 ECH'}</div>
                              </button>
                            );
                          })}
                        </div>
                      )}
                    </div>
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-3 gap-3 p-4 bg-background-card border border-border/40 rounded-2xl">
                  <button
                    type="button"
                    onClick={() => toggleBooleanField('enabled')}
                    className={`flex items-center justify-between rounded-2xl border px-4 py-3 text-left transition-all ${
                      formData.enabled
                        ? 'border-accent/40 bg-accent/10'
                        : 'border-border/40 bg-background-soft/70 hover:border-accent/25'
                    }`}
                  >
                    <div className="space-y-0.5">
                      <div className="text-[11px] font-bold text-text-primary">开启规则</div>
                      <div className="text-[10px] text-text-muted">关闭后本规则不参与匹配</div>
                    </div>
                    <div className={`relative h-5 w-9 rounded-full transition-all ${formData.enabled ? 'bg-accent' : 'bg-background-hover border border-border/50'}`}>
                      <div className={`absolute top-0.5 h-4 w-4 rounded-full bg-white shadow-sm transition-transform duration-200 ${formData.enabled ? 'left-0 translate-x-[18px]' : 'left-0.5'}`} />
                    </div>
                  </button>
                  <button
                    type="button"
                    onClick={() => toggleBooleanField('ech_enabled')}
                    className={`flex items-center justify-between rounded-2xl border px-4 py-3 text-left transition-all ${
                      formData.ech_enabled
                        ? 'border-accent/40 bg-accent/10'
                        : 'border-border/40 bg-background-soft/70 hover:border-accent/25'
                    }`}
                  >
                    <div className="space-y-0.5">
                      <div className="text-[11px] font-bold text-text-primary flex items-center gap-1">
                        <Lock size={12} className="text-cyan-500" /> ECH 加密
                      </div>
                      <div className="text-[10px] text-text-muted">启用后尝试使用 ECH 连接</div>
                    </div>
                    <div className={`relative h-5 w-9 rounded-full transition-all ${formData.ech_enabled ? 'bg-accent' : 'bg-background-hover border border-border/50'}`}>
                      <div className={`absolute top-0.5 h-4 w-4 rounded-full bg-white shadow-sm transition-transform duration-200 ${formData.ech_enabled ? 'left-0 translate-x-[18px]' : 'left-0.5'}`} />
                    </div>
                  </button>
                  <button
                    type="button"
                    onClick={() => toggleBooleanField('use_cf_pool')}
                    className={`flex items-center justify-between rounded-2xl border px-4 py-3 text-left transition-all ${
                      formData.use_cf_pool
                        ? 'border-accent/40 bg-accent/10'
                        : 'border-border/40 bg-background-soft/70 hover:border-accent/25'
                    }`}
                  >
                    <div className="space-y-0.5">
                      <div className="text-[11px] font-bold text-text-primary">优选 IP 池</div>
                      <div className="text-[10px] text-text-muted">优先使用 Cloudflare 优选 IP</div>
                    </div>
                    <div className={`relative h-5 w-9 rounded-full transition-all ${formData.use_cf_pool ? 'bg-accent' : 'bg-background-hover border border-border/50'}`}>
                      <div className={`absolute top-0.5 h-4 w-4 rounded-full bg-white shadow-sm transition-transform duration-200 ${formData.use_cf_pool ? 'left-0 translate-x-[18px]' : 'left-0.5'}`} />
                    </div>
                  </button>
              </div>

              {/* Advanced Cert Verify */}
              <div className="space-y-3 p-4 border border-warning/20 bg-background-card rounded-2xl relative">
                  <div className="flex items-center gap-2 text-warning mb-2">
                      <AlertCircle size={16} />
                      <span className="text-xs font-bold uppercase tracking-wider">证书校验策略 (高级)</span>
                  </div>
                  <div className="space-y-2">
                    <label className="text-[9px] font-bold text-text-muted">验证模式</label>
                    <div className="grid grid-cols-2 md:grid-cols-3 gap-2">
                      {CERT_VERIFY_MODES.map((mode) => {
                        const active = certVerifyMode === mode.id;
                        return (
                          <button
                            key={mode.id || 'default'}
                            type="button"
                            onClick={() => setCertVerify({ mode: mode.id })}
                            className={`rounded-xl border px-3 py-3 text-left transition-all ${
                              active
                                ? 'border-warning/40 bg-warning/10 text-warning shadow-[inset_0_0_0_1px_rgba(210,153,34,0.14)]'
                                : 'border-border/40 bg-background-soft/70 text-text-secondary hover:border-warning/30 hover:text-text-primary'
                            }`}
                          >
                            <div className="text-[11px] font-black tracking-wide">{mode.label}</div>
                            <div className="mt-1 text-[10px] leading-relaxed opacity-80">{mode.desc}</div>
                          </button>
                        );
                      })}
                    </div>
                  </div>

                  <button
                    type="button"
                    onClick={() => setCertVerify({ allow_unknown_authority: !formData.cert_verify.allow_unknown_authority })}
                    className={`flex w-full items-center justify-between rounded-2xl border px-4 py-3 transition-all ${
                      formData.cert_verify.allow_unknown_authority
                        ? 'border-warning/30 bg-warning/10'
                        : 'border-border/40 bg-background-soft/70 hover:border-warning/25'
                    }`}
                  >
                    <div className="space-y-0.5 text-left">
                      <div className="text-[11px] font-bold text-text-primary">允许未知签发者</div>
                      <div className="text-[10px] text-text-muted">仅在你明确知道目标证书来源时启用</div>
                    </div>
                    <div className={`relative h-5 w-9 rounded-full transition-all ${formData.cert_verify.allow_unknown_authority ? 'bg-warning' : 'bg-background-hover border border-border/50'}`}>
                      <div className={`absolute top-0.5 h-4 w-4 rounded-full bg-white shadow-sm transition-transform duration-200 ${formData.cert_verify.allow_unknown_authority ? 'left-0 translate-x-[18px]' : 'left-0.5'}`} />
                    </div>
                  </button>

                  {certVerifyMode === 'allow_names' && (
                    <div className="space-y-1.5">
                      <label className="text-[9px] font-bold text-text-muted">允许名称列表</label>
                      <textarea
                        rows={4}
                        value={joinListInput(formData.cert_verify.names)}
                        onChange={(e) => setCertVerify({ names: splitListInput(e.target.value) })}
                        placeholder="每行一个域名，例如&#10;*.github.com&#10;github.com"
                        className="w-full resize-none bg-background-card border border-border px-3 py-2 rounded-xl text-[11px] leading-relaxed outline-none focus:ring-2 focus:ring-warning"
                      />
                    </div>
                  )}

                  {certVerifyMode === 'allow_suffixes' && (
                    <div className="space-y-1.5">
                      <label className="text-[9px] font-bold text-text-muted">允许后缀列表</label>
                      <textarea
                        rows={4}
                        value={joinListInput(formData.cert_verify.suffixes)}
                        onChange={(e) => setCertVerify({ suffixes: splitListInput(e.target.value) })}
                        placeholder="每行一个后缀，例如&#10;.github.com&#10;.cloudfront.net"
                        className="w-full resize-none bg-background-card border border-border px-3 py-2 rounded-xl text-[11px] leading-relaxed outline-none focus:ring-2 focus:ring-warning"
                      />
                    </div>
                  )}

                  {certVerifyMode === 'allow_spki' && (
                    <div className="space-y-1.5">
                      <label className="text-[9px] font-bold text-text-muted">允许 SPKI SHA256 列表</label>
                      <textarea
                        rows={4}
                        value={joinListInput(formData.cert_verify.spki_sha256)}
                        onChange={(e) => setCertVerify({ spki_sha256: splitListInput(e.target.value) })}
                        placeholder="每行一个 Base64 指纹"
                        className="w-full resize-none bg-background-card border border-border px-3 py-2 rounded-xl text-[11px] leading-relaxed outline-none focus:ring-2 focus:ring-warning font-mono"
                      />
                    </div>
                  )}
              </div>
          </div>
      )}
    </form>
  );
};

export default RuleForm;
