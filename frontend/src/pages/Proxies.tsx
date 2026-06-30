import React, { useState, useEffect } from 'react';
import {
  Trash2, Shield, Server, Zap, Lock, History, PlusCircle, Activity
} from '../lib/icons';
import {
  GetServerConfig, UpdateServerConfig, GetECHProfiles,
  DeleteECHProfile, TestServerNode
} from '../api/bindings';
import { ServiceCard } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { EmptyState } from '../components/ui/EmptyState';
import Modal from '../components/Modal';
import ECHProfileForm from '../components/ECHProfileForm';
import { useTranslation } from '../i18n/I18nContext';

const Proxies: React.FC = () => {
  const { t } = useTranslation();
  const [serverConfig, setServerConfig] = useState<{ host: string, auth: string }>({ host: '', auth: '' });
  const [echProfiles, setEchProfiles] = useState<any[]>([]);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState('');
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingProfile, setEditingProfile] = useState<any>(null);

  const loadData = async () => {
    const [e] = await Promise.all([GetECHProfiles()]);
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

  const handleAddProfile = () => { setEditingProfile(null); setIsModalOpen(true); };
  const handleEditProfile = (profile: any) => { setEditingProfile(profile); setIsModalOpen(true); };
  const handleDeleteProfile = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    if (confirm(t('proxies.delete_confirm'))) { await DeleteECHProfile(id); loadData(); }
  };
  const handleFormSuccess = () => { setIsModalOpen(false); loadData(); };

  const handleSaveServer = async () => {
    await UpdateServerConfig(serverConfig.host, serverConfig.auth);
    await loadServerConfig();
  };

  return (
    <div className="p-6 max-w-5xl mx-auto space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-700">
      <header>
        <h1 className="text-3xl font-black tracking-tighter">{t('proxies.title')}</h1>
      </header>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        <ServiceCard
          title={t('proxies.server_node')}
          icon={<Server size={20} />}
          status={
            <Button
              onClick={async () => {
                setTesting(true);
                try {
                  const duration = await TestServerNode();
                  setTestResult(`${duration}ms`);
                } catch (err: any) {
                  const msg = String(err).replace('Error: ', '');
                  setTestResult(msg || t('common.error'));
                }
                setTesting(false);
                setTimeout(() => setTestResult(''), 3000);
              }}
              loading={testing}
              variant="ghost"
              size="xs"
              icon={<Activity size={14} />}
            >
              {testing ? t('proxies.testing') : (testResult || t('proxies.test_conn'))}
            </Button>
          }
        >
          <div className="space-y-4">
            <div className="space-y-1.5">
              <label className="text-[10px] font-black text-text-muted uppercase tracking-widest px-1" htmlFor="node-host">{t('proxies.node_host')}</label>
              <input
                id="node-host"
                type="text"
                value={serverConfig.host}
                onChange={(e) => setServerConfig({...serverConfig, host: e.target.value})}
                placeholder="proxy.yourdomain.workers.dev"
                className="w-full bg-background-soft border border-border px-4 py-2.5 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none transition-all"
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-[10px] font-black text-text-muted uppercase tracking-widest px-1" htmlFor="auth-secret">{t('proxies.auth_secret')}</label>
              <input
                id="auth-secret"
                type="password"
                value={serverConfig.auth}
                onChange={(e) => setServerConfig({...serverConfig, auth: e.target.value})}
                placeholder={t('proxies.auth_placeholder')}
                className="w-full bg-background-soft border border-border px-4 py-2.5 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none transition-all"
              />
            </div>
            <Button onClick={handleSaveServer} variant="primary" className="w-full">{t('proxies.save_node')}</Button>
          </div>
        </ServiceCard>

        <section className="lg:col-span-2 space-y-4">
          <div className="flex items-center justify-between px-1">
            <div className="flex items-center gap-2 text-text-secondary">
              <Shield size={18} aria-hidden />
              <h2 className="text-sm font-bold uppercase tracking-wider">{t('proxies.ech_management')}</h2>
            </div>
            <Button onClick={handleAddProfile} variant="ghost" size="sm" icon={<PlusCircle size={14} />}>
              {t('proxies.add_ech')}
            </Button>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
            {echProfiles.length === 0 ? (
              <EmptyState
                icon={<Lock size={32} strokeWidth={1.5} />}
                title={t('proxies.no_ech')}
                className="col-span-full py-12 bg-background-card border border-dashed border-border rounded-2xl"
              />
            ) : (
              echProfiles.map((p) => (
                <div
                  key={p.id}
                  onClick={() => handleEditProfile(p)}
                  className="group p-5 bg-background-card border border-border rounded-2xl shadow-sm hover:shadow-md hover:border-accent/40 transition-all flex justify-between items-center cursor-pointer"
                >
                  <div className="flex items-center gap-3 overflow-hidden">
                    <div className="w-10 h-10 rounded-2xl bg-success/10 text-success flex items-center justify-center shrink-0">
                      <Zap size={18} fill="currentColor" className="opacity-80" aria-hidden />
                    </div>
                    <div className="overflow-hidden">
                      <h3 className="text-sm font-bold truncate">{p.name}</h3>
                      <div className="flex items-center gap-1.5 text-[10px] text-text-muted font-bold">
                        <History size={10} aria-hidden />
                        {p.auto_update ? t('proxies.auto_sync') : t('proxies.static_config')}
                      </div>
                    </div>
                  </div>
                  <button
                    className="p-2 text-text-muted hover:text-danger opacity-0 group-hover:opacity-100 transition-all"
                    onClick={(e) => handleDeleteProfile(p.id, e)}
                    aria-label={`删除 ${p.name}`}
                  >
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
        <ECHProfileForm initialData={editingProfile} onSuccess={handleFormSuccess} onCancel={() => setIsModalOpen(false)} />
      </Modal>
    </div>
  );
};

export default Proxies;
