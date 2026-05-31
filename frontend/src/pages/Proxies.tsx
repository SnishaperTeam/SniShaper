import React, { useState, useEffect } from 'react';
import { 
  Trash2, 
  Shield, 
  Server, 
  Zap, 
  Lock, 
  History, 
  PlusCircle,
  Activity
} from 'lucide-react';
import { 
  GetServerConfig,
  UpdateServerConfig,
  GetECHProfiles,
  DeleteECHProfile,
  ProxySelfCheck,
  TestServerNode
} from '../api/bindings';
import Modal from '../components/Modal';
import ECHProfileForm from '../components/ECHProfileForm';
import { useTranslation } from '../i18n/I18nContext';

const ServiceCard: React.FC<{
  title: string;
  icon: React.ReactNode;
  status: React.ReactNode;
  children: React.ReactNode;
  action?: React.ReactNode;
}> = ({ title, icon, status, children, action }) => (
  <div className="bg-background-card border border-border rounded-2xl overflow-hidden shadow-sm flex flex-col hover:border-accent/30 transition-all">
    <div className="px-6 py-4 border-b border-border bg-background-soft/30 flex justify-between items-center shrink-0">
        <div className="flex items-center gap-3">
            <div className="text-accent">{icon}</div>
            <h3 className="text-sm font-black tracking-tight uppercase">{title}</h3>
        </div>
        <div className="flex items-center gap-3">
            {status}
            {action}
        </div>
    </div>
    <div className="p-6 flex-1 space-y-4">
        {children}
    </div>
  </div>
);

const Proxies: React.FC = () => {
  const { t } = useTranslation();
  const [serverConfig, setServerConfig] = useState<{ host: string, auth: string }>({ host: '', auth: '' });
  const [echProfiles, setEchProfiles] = useState<any[]>([]);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState('');
  
  // Modal state
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingProfile, setEditingProfile] = useState<any>(null);

  const loadData = async () => {
    const [e] = await Promise.all([
      GetECHProfiles()
    ]);
    setEchProfiles(e || []);
  };

  const loadServerConfig = async () => {
    const s = await GetServerConfig();
    setServerConfig({ host: s.host || '', auth: s.auth || '' });
  };

  useEffect(() => {
    loadData();
    loadServerConfig();
    const timer = setInterval(loadData, 3000);
    return () => clearInterval(timer);
  }, []);

  const handleAddProfile = () => {
    setEditingProfile(null);
    setIsModalOpen(true);
  };

  const handleEditProfile = (profile: any) => {
    setEditingProfile(profile);
    setIsModalOpen(true);
  };

  const handleDeleteProfile = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    if (confirm(t('proxies.delete_confirm'))) {
      await DeleteECHProfile(id);
      loadData();
    }
  };

  const handleFormSuccess = () => {
    setIsModalOpen(false);
    loadData();
  };

  const handleSaveServer = async () => {
    await UpdateServerConfig(serverConfig.host, serverConfig.auth);
    await loadServerConfig();
  };

  return (
    <div className="p-6 max-w-5xl mx-auto space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-700">
      <header className="flex justify-between items-end">
        <div>
           <h1 className="text-3xl font-black tracking-tighter">{t('proxies.title')}</h1>
        </div>
      </header>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">

        {/* Server Node Section */}
        <ServiceCard 
            title={t('proxies.server_node')} 
            icon={<Server size={20} />}
            status={
                <button 
                  onClick={async () => {
                    setTesting(true);
                    try {
                      const duration = await TestServerNode();
                      setTestResult(`${duration}ms`);
                    } catch (err: any) {
                      const msg = String(err).replace('Error: ', '');
                      setTestResult(msg || t('common.error'));
                      console.error("[TestNode]", err);
                    }
                    setTesting(false);
                    setTimeout(() => setTestResult(''), 3000);
                  }}
                  disabled={testing}
                  className="flex items-center gap-1.5 px-2 py-0.5 rounded-full bg-background-hover hover:text-accent transition-all group"
                  title={t('proxies.test_hint')}
                >
                  <Activity size={14} className={testing ? "animate-pulse" : "group-hover:scale-110 transition-transform"} />
                  <span className="text-[10px] font-black uppercase tracking-tighter">
                      {testing ? t('proxies.testing') : (testResult || t('proxies.test_conn'))}
                  </span>
                </button>
            }
        >
            <div className="space-y-4">
                <div className="space-y-1.5">
                    <label className="text-[10px] font-black text-text-muted uppercase tracking-widest px-1">{t('proxies.node_host')}</label>
                    <input 
                        type="text" 
                        value={serverConfig.host}
                        onChange={(e) => setServerConfig({...serverConfig, host: e.target.value})}
                        placeholder="proxy.yourdomain.workers.dev"
                        className="w-full bg-background-soft border border-border px-4 py-2.5 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none transition-all"
                    />
                </div>
                <div className="space-y-1.5">
                    <label className="text-[10px] font-black text-text-muted uppercase tracking-widest px-1">{t('proxies.auth_secret')}</label>
                    <input 
                        type="password" 
                        value={serverConfig.auth}
                        onChange={(e) => setServerConfig({...serverConfig, auth: e.target.value})}
                        placeholder={t('proxies.auth_placeholder')}
                        className="w-full bg-background-soft border border-border px-4 py-2.5 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none transition-all"
                    />
                </div>
                <button 
                    onClick={handleSaveServer}
                    className="w-full py-2.5 bg-accent/10 text-accent rounded-xl text-[13px] font-black hover:bg-accent hover:text-white transition-all mt-1"
                >
                    {t('proxies.save_node')}
                </button>
            </div>
        </ServiceCard>

        {/* ECH Profiles Management */}
        <section className="lg:col-span-2 space-y-4">
             <div className="flex items-center justify-between px-1">
                <div className="flex items-center gap-2 text-text-secondary">
                    <Shield size={18} />
                    <h3 className="text-sm font-bold uppercase tracking-wider">{t('proxies.ech_management')}</h3>
                </div>
                <button 
                  onClick={handleAddProfile}
                  className="flex items-center gap-1.5 text-xs font-bold text-accent hover:underline"
                >
                    <PlusCircle size={14} />
                    {t('proxies.add_ech')}
                </button>
             </div>

             <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
                {echProfiles.length === 0 ? (
                    <div className="col-span-full py-12 bg-background-card border border-dashed border-border rounded-2xl flex flex-col items-center justify-center text-text-muted opacity-50">
                        <Lock size={32} strokeWidth={1.5} />
                        <span className="text-[11px] font-bold uppercase tracking-widest mt-3">{t('proxies.no_ech')}</span>
                    </div>
                ) : (
                    echProfiles.map((p) => (
                        <div 
                          key={p.id} 
                          onClick={() => handleEditProfile(p)}
                          className="group p-5 bg-background-card border border-border rounded-2xl shadow-sm hover:shadow-md hover:border-accent/40 transition-all flex justify-between items-center cursor-pointer"
                        >
                            <div className="flex items-center gap-3 overflow-hidden">
                                <div className="w-10 h-10 rounded-2xl bg-success/10 text-success flex items-center justify-center shrink-0">
                                    <Zap size={18} fill="currentColor" className="opacity-80" />
                                </div>
                                <div className="overflow-hidden">
                                    <h4 className="text-sm font-bold truncate">{p.name}</h4>
                                    <div className="flex items-center gap-1.5 text-[10px] text-text-muted font-bold">
                                        <History size={10} />
                                        {p.auto_update ? t('proxies.auto_sync') : t('proxies.static_config')}
                                    </div>
                                </div>
                            </div>
                            <button className="p-2 text-text-muted hover:text-danger opacity-0 group-hover:opacity-100 transition-all" onClick={(e) => handleDeleteProfile(p.id, e)}>
                                <Trash2 size={18} />
                            </button>
                        </div>
                    ))
                )}
             </div>
        </section>
      </div>

      <Modal 
        isOpen={isModalOpen} 
        onClose={() => setIsModalOpen(false)} 
        title={editingProfile ? t('proxies.edit_ech') : t('proxies.probe_ech')}
        subtitle={editingProfile ? `${t('proxies.modifying')}: ${editingProfile.name || editingProfile.Name}` : t('proxies.probe_hint')}
        maxWidth="max-w-3xl"
      >
        <ECHProfileForm 
            initialData={editingProfile} 
            onSuccess={handleFormSuccess} 
            onCancel={() => setIsModalOpen(false)} 
        />
      </Modal>
    </div>
  );
};

export default Proxies;
