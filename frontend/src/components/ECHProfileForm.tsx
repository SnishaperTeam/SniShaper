import React, { useState, useEffect } from 'react';
import { 
  Shield, 
  Search, 
  Loader2, 
  CheckCircle2,
  AlertCircle,
  Save
} from 'lucide-react';
import { UpsertECHProfile, FetchECHConfig } from '../api/bindings';

interface ECHProfileFormProps {
  initialData?: any;
  onSuccess: () => void;
  onCancel: () => void;
}

const ECHProfileForm: React.FC<ECHProfileFormProps> = ({ initialData, onSuccess, onCancel }) => {
  const [formData, setFormData] = useState<any>({
    id: '',
    name: '',
    discovery_domain: '',
    doh_upstream: 'https://1.1.1.1/dns-query',
    config: '',
    auto_update: true
  });
  const [isFetching, setIsFetching] = useState(false);
  const [fetchError, setFetchError] = useState('');
  const [fetchSuccess, setFetchSuccess] = useState(false);

  useEffect(() => {
    if (initialData) {
      setFormData(initialData);
    }
  }, [initialData]);

  const handleFetch = async () => {
    if (!formData.discovery_domain) {
      setFetchError('需先输入域名以进行探测');
      return;
    }
    setIsFetching(true);
    setFetchError('');
    setFetchSuccess(false);

    try {
      const result = await FetchECHConfig(formData.discovery_domain, formData.doh_upstream);
      if (result && result.length > 10) {
        setFormData((prev: any) => ({ ...prev, config: result }));
        setFetchSuccess(true);
      } else {
        setFetchError('探测失败: 未找到有效的 ECH 配置');
      }
    } catch (e: any) {
      setFetchError(`探测出错: ${e.message || e}`);
    } finally {
      setIsFetching(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    await UpsertECHProfile(formData);
    onSuccess();
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-6 text-text-primary">
      <div className="space-y-4 p-4 bg-accent/5 border border-accent/20 rounded-2xl">
          <div className="flex gap-3 items-center">
            <Shield className="text-accent" size={20} />
            <div>
                <h4 className="text-xs font-black uppercase tracking-widest text-accent">ECH 借力探测</h4>
                <p className="text-[10px] text-accent/70 font-medium">输入目标域名，系统将通过 DoH 自动获取最新的 ECH 配置 (ECHConfigList)</p>
            </div>
          </div>
          
          <div className="flex gap-3">
              <div className="flex-1 space-y-1">
                  <input 
                    type="text" 
                    value={formData.discovery_domain}
                    onChange={(e) => setFormData({...formData, discovery_domain: e.target.value})}
                    placeholder="例如: cloudflare.com"
                    className="w-full bg-background-card border border-border px-4 py-2 rounded-xl text-xs focus:ring-1 focus:ring-accent outline-none font-medium text-text-primary"
                  />
              </div>
              <button 
                type="button"
                onClick={handleFetch}
                disabled={isFetching}
                className="px-4 py-2 bg-accent text-white rounded-xl text-xs font-black shadow-lg shadow-accent/20 flex items-center gap-2 disabled:opacity-50 transition-all hover:scale-105"
              >
                {isFetching ? <Loader2 size={14} className="animate-spin" /> : <Search size={14} />}
                {isFetching ? "探测中..." : "开始探测"}
              </button>
          </div>

          {fetchError && (
              <div className="flex items-center gap-2 text-danger text-[10px] font-bold px-1">
                  <AlertCircle size={12} />
                  {fetchError}
              </div>
          )}
          {fetchSuccess && (
              <div className="flex items-center gap-2 text-success text-[10px] font-bold px-1 animate-in slide-in-from-left-2">
                  <CheckCircle2 size={12} />
                  已成功获取 ECH 配置片段
              </div>
          )}
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6 pt-2">
        <div className="space-y-1.5">
          <label className="text-[10px] font-black text-text-muted uppercase tracking-widest px-1">配置文件名称</label>
          <input 
            type="text" 
            required
            value={formData.name}
            onChange={(e) => setFormData({...formData, name: e.target.value})}
            placeholder="自定义名称"
            className="w-full bg-background-soft border border-border px-4 py-3 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none font-medium transition-all"
          />
        </div>
        <div className="space-y-1.5">
          <label className="text-[10px] font-black text-text-muted uppercase tracking-widest px-1">DoH 查询地址</label>
          <input 
            type="text" 
            value={formData.doh_upstream}
            onChange={(e) => setFormData({...formData, doh_upstream: e.target.value})}
            className="w-full bg-background-soft border border-border px-4 py-3 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none font-medium transition-all"
          />
        </div>
      </div>

      <div className="space-y-1.5">
          <label className="text-[10px] font-black text-text-muted uppercase tracking-widest px-1">ECH 配置原始内容 (Raw Base64 / Hex)</label>
          <textarea 
            rows={4}
            required
            value={formData.config}
            onChange={(e) => setFormData({...formData, config: e.target.value})}
            placeholder="粘贴 ECHConfigList 或通过上方探测自动填入"
            className="w-full bg-background-soft border border-border px-4 py-3 rounded-xl text-xs focus:ring-2 focus:ring-accent outline-none font-mono leading-relaxed transition-all"
          />
      </div>

      <div className="flex items-center gap-3 p-1">
          <button 
            type="button"
            onClick={() => setFormData({...formData, auto_update: !formData.auto_update})}
            className={`w-10 h-5 rounded-full transition-all relative ${formData.auto_update ? "bg-accent" : "bg-gray-400/20"}`}
          >
            <div className={`absolute top-0.5 w-4 h-4 bg-white rounded-full transition-all ${formData.auto_update ? "left-5.5" : "left-0.5"}`} />
          </button>
          <div>
            <div className="text-xs font-bold">后台自动静默更新</div>
            <p className="text-[10px] text-text-muted">启用后 SniShaper 将定期重新探测并刷新过期配置</p>
          </div>
      </div>

      <div className="flex justify-end gap-3 pt-4 border-t border-border/50">
          <button 
            type="button"
            onClick={onCancel}
            className="px-6 py-2.5 rounded-xl border border-border text-text-secondary font-bold hover:bg-background-hover transition-all"
          >
            取消
          </button>
          <button 
            type="submit"
            className="px-8 py-2.5 rounded-xl bg-accent text-white font-black shadow-lg shadow-accent/20 hover:scale-[1.02] active:scale-[0.98] transition-all flex items-center gap-2"
          >
            <Save size={18} />
            {formData.id ? "更新 Profile" : "保存 Profile"}
          </button>
      </div>
    </form>
  );
};

export default ECHProfileForm;
