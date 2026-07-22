import React, { useState, useEffect } from 'react';
import {
  Trash2, Shield, Zap, Lock, History, PlusCircle, Globe, Layers
} from '../lib/icons';
import {
  GetECHProfiles, DeleteECHProfile, GetNAT64Profiles, DeleteNAT64Profile, TestNAT64Profile,
  GetMigrationServer, SetMigrationServer, TestMigration
} from '../api/bindings';
import { Button } from '../components/ui/Button';
import { EmptyState } from '../components/ui/EmptyState';
import Modal from '../components/Modal';
import ECHProfileForm from '../components/ECHProfileForm';
import NAT64ProfileForm from '../components/NAT64ProfileForm';
import { useTranslation } from '../i18n/I18nContext';
import { cn } from '../lib/utils';
import { toast } from '../lib/toast';

const Proxies: React.FC = () => {
  const { t } = useTranslation();
  const [echProfiles, setEchProfiles] = useState<any[]>([]);
  const [nat64Profiles, setNat64Profiles] = useState<any[]>([]);
  
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingProfile, setEditingProfile] = useState<any>(null);

  const [isNAT64ModalOpen, setIsNAT64ModalOpen] = useState(false);
  const [editingNAT64, setEditingNAT64] = useState<any>(null);

  const [testingMap, setTestingMap] = useState<Record<string, boolean>>({});
  const [testResults, setTestResults] = useState<Record<string, string>>({});

  const [migrationServer, setMigrationServerState] = useState('');
  const [migrationServerInput, setMigrationServerInput] = useState('');
  const [migrationSaving, setMigrationSaving] = useState(false);
  const [migrationTesting, setMigrationTesting] = useState(false);

  const loadData = async () => {
    const [e, n, ms] = await Promise.all([GetECHProfiles(), GetNAT64Profiles(), GetMigrationServer()]);
    setEchProfiles(e || []);
    setNat64Profiles(n || []);
    setMigrationServerState(String(ms || ''));
    setMigrationServerInput(String(ms || ''));
  };

  const handleTestNAT64 = async (profile: any, e: React.MouseEvent) => {
    e.stopPropagation();
    const id = profile.id;
    setTestingMap((prev) => ({ ...prev, [id]: true }));
    setTestResults((prev) => ({ ...prev, [id]: '' }));
    try {
      const ms = await TestNAT64Profile(profile.prefix);
      setTestResults((prev) => ({ ...prev, [id]: `${ms} ms` }));
      toast.success(`${profile.name}: 连接成功，延迟 ${ms} ms`);
    } catch (err: any) {
      const errMsg = err?.message || String(err);
      setTestResults((prev) => ({ ...prev, [id]: '失败' }));
      toast.error(`${profile.name}: 连接失败: ${errMsg}`);
    } finally {
      setTestingMap((prev) => ({ ...prev, [id]: false }));
    }
  };

  useEffect(() => {
    loadData();
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

  const handleAddNAT64 = () => { setEditingNAT64(null); setIsNAT64ModalOpen(true); };
  const handleEditNAT64 = (profile: any) => { setEditingNAT64(profile); setIsNAT64ModalOpen(true); };
  const handleDeleteNAT64 = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    if (confirm(t('proxies.delete_nat64_confirm') || '确定要删除此 NAT64 配置吗？')) { await DeleteNAT64Profile(id); loadData(); }
  };
  const handleNAT64FormSuccess = () => { setIsNAT64ModalOpen(false); loadData(); };

  const handleSaveMigration = async () => {
    setMigrationSaving(true);
    try {
      await SetMigrationServer(migrationServerInput.trim());
      setMigrationServerState(migrationServerInput.trim());
      toast.success(t('common.success') || '已保存');
    } catch (err: any) {
      toast.error(err?.message || String(err));
    } finally {
      setMigrationSaving(false);
    }
  };

  const handleTestMigration = async () => {
    const server = migrationServerInput.trim() || migrationServer;
    if (!server) {
      toast.error('请输入迁移服务地址');
      return;
    }
    setMigrationTesting(true);
    try {
      const msg = await TestMigration(server);
      toast.success(msg || '测试成功');
    } catch (err: any) {
      toast.error(err?.message || String(err));
    } finally {
      setMigrationTesting(false);
    }
  };

  return (
    <div className="p-6 max-w-5xl mx-auto space-y-12 animate-in fade-in slide-in-from-bottom-4 duration-700">
      <header>
        <h1 className="text-3xl font-black tracking-tighter">{t('proxies.title')}</h1>
      </header>

      {/* 连接迁移服务 */}
      <section className="space-y-4">
        <div className="flex items-center justify-between px-1">
          <div className="flex items-center gap-2 text-text-secondary">
            <Globe size={18} aria-hidden />
            <h2 className="text-sm font-bold uppercase tracking-wider">{t('proxies.migration_service') || '连接迁移服务'}</h2>
          </div>
        </div>

        <div className="p-5 bg-background-card border border-border rounded-2xl shadow-sm space-y-4">
          <p className="text-[11px] text-text-muted">{t('proxies.migration_service_desc') || '配置连接迁移服务地址'}</p>
          <div className="flex items-center gap-3">
            <input
              type="text"
              value={migrationServerInput}
              onChange={(e) => setMigrationServerInput(e.target.value)}
              placeholder={t('proxies.migration_service_hint') || '输入迁移服务 API 地址'}
              className="flex-1 bg-background-hover border border-border px-4 py-2 rounded-xl text-[11px] font-medium focus:ring-2 focus:ring-accent outline-none transition-all placeholder:text-text-muted/40"
            />
            <Button
              onClick={handleSaveMigration}
              disabled={migrationSaving}
              variant="ghost"
              size="sm"
            >
              {migrationSaving ? '...' : (t('common.save') || '保存')}
            </Button>
            <Button
              onClick={handleTestMigration}
              disabled={migrationTesting}
              variant="ghost"
              size="sm"
            >
              {migrationTesting ? '...' : '测试连接'}
            </Button>
          </div>
        </div>
      </section>

      {/* ECH 配置管理 */}
      <section className="space-y-4">
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

      {/* NAT64 配置管理 */}
      <section className="space-y-4">
        <div className="flex items-center justify-between px-1">
          <div className="flex items-center gap-2 text-text-secondary">
            <Globe size={18} aria-hidden />
            <h2 className="text-sm font-bold uppercase tracking-wider">{t('proxies.nat64_management') || 'NAT64 配置管理'}</h2>
          </div>
          <Button onClick={handleAddNAT64} variant="ghost" size="sm" icon={<PlusCircle size={14} />}>
            {t('proxies.add_nat64') || '添加 NAT64 配置'}
          </Button>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {nat64Profiles.length === 0 ? (
            <EmptyState
              icon={<Layers size={32} strokeWidth={1.5} />}
              title={t('proxies.no_nat64') || '暂无 NAT64 配置'}
              className="col-span-full py-12 bg-background-card border border-dashed border-border rounded-2xl"
            />
          ) : (
            nat64Profiles.map((p) => (
              <div
                key={p.id}
                onClick={() => handleEditNAT64(p)}
                className="group p-5 bg-background-card border border-border rounded-2xl shadow-sm hover:shadow-md hover:border-accent/40 transition-all flex justify-between items-center cursor-pointer"
              >
                <div className="flex items-center gap-3 overflow-hidden">
                  <div className="w-10 h-10 rounded-2xl bg-accent/10 text-accent flex items-center justify-center shrink-0">
                    <Globe size={18} className="opacity-80" aria-hidden />
                  </div>
                  <div className="overflow-hidden">
                    <h3 className="text-sm font-bold truncate">{p.name}</h3>
                    <div className="flex items-center gap-1.5 text-[10px] text-text-muted font-bold truncate">
                      <Layers size={10} aria-hidden />
                      前缀：{p.prefix}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  {testResults[p.id] && (
                    <span className={cn(
                      "text-[10px] font-bold px-2 py-0.5 rounded-lg font-mono border",
                      testResults[p.id] === '失败'
                        ? "bg-danger/10 border-danger/20 text-danger"
                        : "bg-success/10 border-success/20 text-success"
                    )}>
                      {testResults[p.id]}
                    </span>
                  )}
                  <button
                    disabled={testingMap[p.id]}
                    onClick={(e) => handleTestNAT64(p, e)}
                    className={cn(
                      "text-[10px] font-bold px-2.5 py-1 rounded-xl transition-all border shrink-0",
                      testingMap[p.id]
                        ? "bg-background-soft border-border text-text-muted cursor-not-allowed"
                        : "bg-accent/5 hover:bg-accent/15 border-accent/20 text-accent hover:border-accent/40"
                    )}
                  >
                    {testingMap[p.id] ? '测试中...' : '测试连接'}
                  </button>
                  <button
                    className="p-2 text-text-muted hover:text-danger opacity-0 group-hover:opacity-100 transition-all shrink-0"
                    onClick={(e) => handleDeleteNAT64(p.id, e)}
                    aria-label={`删除 ${p.name}`}
                  >
                    <Trash2 size={16} />
                  </button>
                </div>
              </div>
            ))
          )}
        </div>
      </section>

      {/* ECH Modal */}
      <Modal
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        title={editingProfile ? t('proxies.edit_ech') : t('proxies.probe_ech')}
        subtitle={editingProfile ? `${t('proxies.modifying')}: ${editingProfile.name || editingProfile.Name}` : t('proxies.probe_hint')}
        maxWidth="max-w-3xl"
      >
        <ECHProfileForm initialData={editingProfile} onSuccess={handleFormSuccess} onCancel={() => setIsModalOpen(false)} />
      </Modal>

      {/* NAT64 Modal */}
      <Modal
        isOpen={isNAT64ModalOpen}
        onClose={() => setIsNAT64ModalOpen(false)}
        title={editingNAT64 ? t('proxies.edit_nat64') || '编辑 NAT64 配置' : t('proxies.add_nat64') || '添加 NAT64 配置'}
        subtitle={editingNAT64 ? `${t('proxies.modifying') || '正在修改'}: ${editingNAT64.name}` : t('proxies.nat64_form_subtitle') || '配置独立映射规则前缀'}
        maxWidth="max-w-3xl"
      >
        <NAT64ProfileForm initialData={editingNAT64} onSuccess={handleNAT64FormSuccess} onCancel={() => setIsNAT64ModalOpen(false)} />
      </Modal>
    </div>
  );
};

export default Proxies;
