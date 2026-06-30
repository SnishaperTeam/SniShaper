import React, { useState, useEffect, useRef } from 'react';
import { 
  Workflow, 
  ShieldCheck, 
  Activity,
  Share2,
  RefreshCw,
  Power,
  Zap
} from '../lib/icons';
import { 
  GetAutoRoutingConfig, 
  UpdateAutoRoutingConfig, 
  GetAutoRoutingStatus, 
  RefreshGFWList,
  EventsOn
} from '../api/bindings';
import { toast } from '../lib/toast';
import { useTranslation } from '../i18n/I18nContext';

interface FlowEntry {
  id: string;
  domain: string;
  mode: string;
  modeClass: string;
  modeDisplay: string;
}

const Routing: React.FC = () => {
  const { t } = useTranslation();
  const [flows, setFlows] = useState<FlowEntry[]>([]);
  const [config, setConfig] = useState<any>({ mode: '', gfwlist_url: '' });
  const [status, setStatus] = useState<any>({ enabled: false, domain_count: 0 });
  useEffect(() => {
    const loadData = async () => {
      const cfg = await GetAutoRoutingConfig();
      const s = await GetAutoRoutingStatus();
      setConfig(cfg);
      setStatus(s);
    };
    loadData();

    const unoff = EventsOn("app:route", (data: any) => {
        if (!data) return;
        const { domain, mode } = data;
        if (!domain || !mode) return;
        
        let modeClass = 'bg-success/80'; 
        let modeDisplay = mode.toUpperCase();
        const lowerMode = mode.toLowerCase();
        
        if (lowerMode.includes('server')) modeClass = 'bg-purple-600';
        else if (lowerMode.includes('direct') || lowerMode.includes('tcp')) { 
            modeClass = 'bg-gray-500'; modeDisplay = 'DIRECT'; 
        }
        else if (lowerMode.includes('transparent')) modeClass = 'bg-red-500';
        else if (lowerMode.includes('mitm') || lowerMode.includes('proxy')) modeClass = 'bg-yellow-600';
        
        setFlows(prev => {
            const newFlow: FlowEntry = {
                id: crypto.randomUUID(),
                domain,
                mode,
                modeClass,
                modeDisplay
            };
            return [newFlow, ...prev].slice(0, 40);
        });
    });

    return () => unoff();
  }, []);

  const handleSave = async () => {
    await UpdateAutoRoutingConfig(config);
    toast.success(t('routing.notifications.saved'), t('routing.notifications.saved_desc'));
  };

  const handleRefreshGFW = async () => {
    await RefreshGFWList();
    const s = await GetAutoRoutingStatus();
    setStatus(s);
    toast.success(t('routing.notifications.updated'), t('routing.notifications.updated_desc', { count: s?.domain_count || 0 }));
  };

  return (
    <div className="p-6 max-w-5xl mx-auto space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-700">

      {/* Real-time Flow Canvas (Clash Verge connection-like) */}
      <div className="bg-background-card border border-border rounded-2xl overflow-hidden shadow-sm flex flex-col h-[340px]">
        <div className="px-6 py-4 border-b border-border bg-background-soft/30 flex justify-between items-center">
            <div className="flex items-center gap-2 text-text-secondary font-bold text-xs uppercase tracking-widest">
                <Activity size={14} className="text-success animate-pulse" />
                {t('routing.traffic_flow')}
            </div>
        </div>
        <div className="flex-1 overflow-y-auto p-4 flex flex-wrap content-start gap-3 relative">
            {flows.length === 0 && (
                <div className="absolute inset-0 flex flex-col items-center justify-center text-text-muted opacity-40">
                    <Share2 size={40} strokeWidth={1} />
                    <span className="text-xs mt-3 font-bold uppercase tracking-widest">{t('routing.waiting_traffic')}</span>
                </div>
            )}
            {flows.map((flow) => (
                <div 
                    key={flow.id} 
                    className="flex items-center gap-2 px-3 py-1.5 bg-background-hover border border-border rounded-full animate-in zoom-in slide-in-from-top-2 duration-300 shadow-sm"
                >
                    <span className="text-xs font-bold max-w-[150px] truncate" title={flow.domain}>
                        {flow.domain}
                    </span>
                    <span className="text-text-muted text-[10px]">➔</span>
                    <span className={`px-2 py-0.5 rounded-full text-[9px] font-black text-white uppercase shadow-sm ${flow.modeClass}`}>
                        {flow.modeDisplay}
                    </span>
                </div>
            ))}
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        <section className="space-y-4">
             <div className="flex items-center gap-2 px-1">
                <Workflow size={18} className="text-accent" />
                <h3 className="text-sm font-bold uppercase tracking-wider text-text-secondary">{t('routing.strategy')}</h3>
             </div>
             <div className="bg-background-card border border-border rounded-2xl p-4 space-y-6">
                <div className="space-y-3">
                    <label className="text-[10px] font-black text-text-muted uppercase tracking-[0.2em] px-1">{t('routing.mode_switch')}</label>
                    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                        {[
                            { id: '', label: t('routing.modes.off'), icon: Power, color: 'text-text-muted' },
                            { id: 'default', label: t('routing.modes.smart'), icon: Zap, color: 'text-success' },
                            { id: 'server', label: t('routing.modes.server'), icon: Activity, color: 'text-purple-500' }
                        ].map((opt) => (
                            <button
                                key={opt.id}
                                onClick={() => setConfig({...config, mode: opt.id})}
                                className={`flex flex-col items-center gap-2 p-3 rounded-2xl border transition-all ${
                                    config.mode === opt.id 
                                        ? "bg-accent/5 border-accent text-accent shadow-sm" 
                                        : "bg-background-soft border-border/60 text-text-secondary hover:border-accent/30"
                                }`}
                            >
                                <div className={`w-7 h-7 rounded-lg bg-background-hover flex items-center justify-center ${config.mode === opt.id ? 'text-accent' : opt.color}`}>
                                    <opt.icon size={16} />
                                </div>
                                <span className="text-[10px] font-black uppercase tracking-widest">{opt.label}</span>
                            </button>
                        ))}
                    </div>
                </div>

                <div className="flex items-center justify-between border-t border-border/50 pt-4">
                    <div>
                        <div className="text-sm font-bold">{t('routing.gfwlist_check')}</div>
                        <div className="text-[11px] text-text-muted mt-0.5">
                            {status.enabled ? t('routing.preloaded', { count: status.domain_count }) : t('routing.inactive')}
                        </div>
                    </div>
                    <button 
                        onClick={handleRefreshGFW}
                        className="flex items-center gap-2 px-5 py-2.5 rounded-2xl text-[10px] font-black uppercase tracking-widest bg-background-hover hover:bg-accent hover:text-white transition-all shadow-sm"
                    >
                        <RefreshCw size={12} />
                        {t('routing.update_list')}
                    </button>
                </div>
                <button 
                    onClick={handleSave}
                    className="w-full py-3 bg-accent text-white rounded-2xl font-black shadow-lg shadow-accent/20 hover:scale-[1.01] active:scale-[0.99] transition-all"
                >
                    {t('routing.save_apply')}
                </button>
             </div>
        </section>

        <section className="space-y-4">
             <div className="flex items-center gap-2 px-1">
                <ShieldCheck size={18} className="text-success" />
                <h3 className="text-sm font-bold uppercase tracking-wider text-text-secondary">{t('routing.features')}</h3>
             </div>
             <div className="grid grid-cols-1 gap-4">
                {[
                    { icon: Zap, title: t('routing.feature_smart'), color: "text-success", desc: t('routing.feature_smart_desc') },
                    { icon: Activity, title: t('routing.feature_priority'), color: "text-purple-500", desc: t('routing.feature_priority_desc') }
                ].map((item, i) => (
                    <div key={i} className="p-4 bg-background-card border border-border rounded-xl flex gap-4 hover:border-accent/40 transition-colors">
                        <div className={`shrink-0 w-10 h-10 rounded-2xl flex items-center justify-center bg-background-hover ${item.color}`}>
                            <item.icon size={20} />
                        </div>
                        <div>
                            <h4 className="text-sm font-bold">{item.title}</h4>
                            <p className="text-[11px] text-text-muted mt-1 leading-relaxed font-medium">{item.desc}</p>
                        </div>
                    </div>
                ))}
             </div>
        </section>
      </div>
    </div>
  );
};

export default Routing;
