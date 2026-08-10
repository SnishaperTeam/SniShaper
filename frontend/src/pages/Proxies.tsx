import React, { useState, useEffect } from 'react';
import {
  Delete, Shield, Bolt, Lock, History, AddCircle, Public, Layers
} from '../lib/icons';
import {
  GetECHProfiles, DeleteECHProfile, GetNAT64Profiles, DeleteNAT64Profile, TestNAT64Profile,
  GetMigrationServer, SetMigrationServer, TestMigration
} from '../api/bindings';
import {
  Box, Typography, Button, IconButton, TextField, Grid, useColorScheme,
} from '@mui/material';
import Modal from '../components/Modal';
import ECHProfileForm from '../components/ECHProfileForm';
import NAT64ProfileForm from '../components/NAT64ProfileForm';
import { useTranslation } from '../i18n/I18nContext';
import { toast } from '../lib/toast';

const Proxies: React.FC = () => {
  const { t } = useTranslation();
  const { mode } = useColorScheme();
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
      toast.success(t('proxies.notifications.connect_success', { name: profile.name, ms }));
    } catch (err: any) {
      const errMsg = err?.message || String(err);
      setTestResults((prev) => ({ ...prev, [id]: t('common.failed') }));
      toast.error(t('proxies.notifications.connect_failed', { name: profile.name, error: errMsg }));
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
    if (confirm(t('proxies.delete_nat64_confirm'))) { await DeleteNAT64Profile(id); loadData(); }
  };
  const handleNAT64FormSuccess = () => { setIsNAT64ModalOpen(false); loadData(); };

  const handleSaveMigration = async () => {
    setMigrationSaving(true);
    try {
      await SetMigrationServer(migrationServerInput.trim());
      setMigrationServerState(migrationServerInput.trim());
      toast.success(t('common.success'));
    } catch (err: any) {
      toast.error(err?.message || String(err));
    } finally {
      setMigrationSaving(false);
    }
  };

  const handleTestMigration = async () => {
    const server = migrationServerInput.trim() || migrationServer;
    if (!server) {
      toast.error(t('proxies.notifications.migration_required'));
      return;
    }
    setMigrationTesting(true);
    try {
      const msg = await TestMigration(server);
      toast.success(msg || t('proxies.notifications.test_success'));
    } catch (err: any) {
      toast.error(err?.message || String(err));
    } finally {
      setMigrationTesting(false);
    }
  };

  const cardHoverSx = (enabled: boolean) => enabled
    ? { cursor: 'pointer', '&:hover': { boxShadow: 3, borderColor: 'primary.main' } }
    : { cursor: 'default' };

  return (
    <Box sx={{ px: 6, pt: 4, pb: 6, maxWidth: '5xl', mx: 'auto' }}>
      <Typography variant="h4" sx={{ fontWeight: 900, letterSpacing: '-0.02em' }}>{t('proxies.title')}</Typography>

      <Box sx={{ mt: 5, display: 'flex', flexDirection: 'column', gap: 2 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, px: 0.5, color: 'text.secondary' }}>
          <Public size={18} aria-hidden />
          <Typography variant="body2" sx={{ fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.05em', display: 'flex', alignItems: 'center', gap: 1 }}>
            {t('proxies.migration_service')}
          </Typography>
        </Box>

        <Box sx={{ p: 2.5, bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2, boxShadow: 1, display: 'flex', flexDirection: 'column', gap: 2 }}>
          <Typography variant="caption" sx={{ fontSize: 11, color: 'text.secondary' }}>{t('proxies.migration_service_desc')}</Typography>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
            <TextField
              type="text"
              size="small"
              value={migrationServerInput}
              onChange={(e) => setMigrationServerInput(e.target.value)}
              placeholder={t('proxies.migration_service_hint')}
              sx={{ flex: 1, '& input': { fontSize: '0.75rem', fontWeight: 500 } }}
            />
            <Button onClick={handleSaveMigration} disabled={migrationSaving} variant="outlined" size="small" sx={{ flexShrink: 0 }}>
              {migrationSaving ? '...' : t('common.save')}
            </Button>
            <Button onClick={handleTestMigration} disabled={migrationTesting} variant="outlined" size="small" sx={{ flexShrink: 0 }}>
              {migrationTesting ? '...' : '测试连接'}
            </Button>
          </Box>
        </Box>
      </Box>

      <Box sx={{ mt: 5, display: 'flex', flexDirection: 'column', gap: 2 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', px: 0.5 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, color: 'text.secondary' }}>
            <Shield size={18} aria-hidden />
            <Typography variant="body2" sx={{ fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.05em' }}>{t('proxies.ech_management')}</Typography>
          </Box>
          <Button onClick={handleAddProfile} variant="outlined" size="small" startIcon={<AddCircle size={14} />}>
            {t('proxies.add_ech')}
          </Button>
        </Box>

        <Grid container spacing={2}>
          {echProfiles.length === 0 ? (
            <Grid size={12}>
              <Box sx={{ py: 12, display: 'flex', flexDirection: 'column', alignItems: 'center', color: 'text.secondary', opacity: 0.7, bgcolor: 'background.paper', border: '1px dashed', borderColor: 'divider', borderRadius: 2 }}>
                <Lock size={32} strokeWidth={1.5} />
                <Typography variant="body2" sx={{ mt: 1.5 }}>{t('proxies.no_ech')}</Typography>
              </Box>
            </Grid>
          ) : (
            echProfiles.map((p) => (
              <Grid key={p.id} size={{ xs: 12, md: 6, xl: 4 }}>
                <Box
                  onClick={() => handleEditProfile(p)}
                  sx={{
                    display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 2,
                    p: 2.5, bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2,
                    boxShadow: 1, transition: 'all 0.2s',
                    ...cardHoverSx(true),
                  }}
                >
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, minWidth: 0 }}>
                    <Box sx={{ width: 40, height: 40, borderRadius: 2, bgcolor: 'rgba(34,197,94,0.1)', color: 'success.main', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
                      <Bolt size={18} fill="currentColor" aria-hidden />
                    </Box>
                    <Box sx={{ minWidth: 0 }}>
                      <Typography variant="body2" sx={{ fontWeight: 700, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{p.name}</Typography>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, mt: 0.25, color: 'text.secondary' }}>
                        <History size={10} aria-hidden />
                        <Typography variant="caption" sx={{ fontSize: 10, fontWeight: 700, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {p.auto_update ? t('proxies.auto_sync') : t('proxies.static_config')}
                        </Typography>
                      </Box>
                    </Box>
                  </Box>
                  <IconButton
                    size="small"
                    aria-label={t('proxies.delete_aria', { name: p.name })}
                    onClick={(e) => handleDeleteProfile(p.id, e)}
                    sx={{ color: 'text.secondary', flexShrink: 0, '&:hover': { color: 'error.main', bgcolor: 'action.hover' } }}
                  >
                    <Delete size={18} />
                  </IconButton>
                </Box>
              </Grid>
            ))
          )}
        </Grid>
      </Box>

      <Box sx={{ mt: 5, display: 'flex', flexDirection: 'column', gap: 2 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', px: 0.5 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, color: 'text.secondary' }}>
            <Public size={18} aria-hidden />
            <Typography variant="body2" sx={{ fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.05em' }}>{t('proxies.nat64_management')}</Typography>
          </Box>
          <Button onClick={handleAddNAT64} variant="outlined" size="small" startIcon={<AddCircle size={14} />}>
            {t('proxies.add_nat64')}
          </Button>
        </Box>

        <Grid container spacing={2}>
          {nat64Profiles.length === 0 ? (
            <Grid size={12}>
              <Box sx={{ py: 12, display: 'flex', flexDirection: 'column', alignItems: 'center', color: 'text.secondary', opacity: 0.7, bgcolor: 'background.paper', border: '1px dashed', borderColor: 'divider', borderRadius: 2 }}>
                <Layers size={32} strokeWidth={1.5} />
                <Typography variant="body2" sx={{ mt: 1.5 }}>{t('proxies.no_nat64')}</Typography>
              </Box>
            </Grid>
          ) : (
            nat64Profiles.map((p) => (
              <Grid key={p.id} size={{ xs: 12, md: 6, xl: 4 }}>
                <Box
                  onClick={() => handleEditNAT64(p)}
                  sx={{
                    display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 2,
                    p: 2.5, bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2,
                    boxShadow: 1, transition: 'all 0.2s',
                    ...cardHoverSx(true),
                  }}
                >
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, minWidth: 0 }}>
                    <Box sx={{ width: 40, height: 40, borderRadius: 2, bgcolor: 'rgba(11,123,255,0.1)', color: 'primary.main', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
                      <Public size={18} aria-hidden />
                    </Box>
                    <Box sx={{ minWidth: 0 }}>
                      <Typography variant="body2" sx={{ fontWeight: 700, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{p.name}</Typography>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, mt: 0.25, color: 'text.secondary', minWidth: 0 }}>
                        <Layers size={10} aria-hidden />
                        <Typography variant="caption" sx={{ fontSize: 10, fontWeight: 700, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          前缀：{p.prefix}
                        </Typography>
                      </Box>
                    </Box>
                  </Box>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexShrink: 0 }}>
                    {testResults[p.id] && (
                      <Box
                        component="span"
                        sx={{
                          fontSize: 10, fontWeight: 700, fontFamily: 'mono', px: 1, py: 0.25, borderRadius: 1, border: 1,
                          color: testResults[p.id] === '失败' ? 'error.main' : 'success.main',
                          borderColor: testResults[p.id] === '失败' ? 'rgba(239,68,68,0.2)' : 'rgba(34,197,94,0.2)',
                          bgcolor: testResults[p.id] === '失败' ? 'rgba(239,68,68,0.1)' : 'rgba(34,197,94,0.1)',
                        }}
                      >
                        {testResults[p.id]}
                      </Box>
                    )}
                    <Button
                      size="small"
                      variant="outlined"
                      disabled={testingMap[p.id]}
                      onClick={(e) => handleTestNAT64(p, e)}
                      sx={{ fontSize: 10, fontWeight: 700, minWidth: 0, flexShrink: 0 }}
                    >
                      {testingMap[p.id] ? '测试中...' : '测试连接'}
                    </Button>
                    <IconButton
                      size="small"
                      aria-label={t('proxies.delete_aria', { name: p.name })}
                      onClick={(e) => handleDeleteNAT64(p.id, e)}
                      sx={{ color: 'text.secondary', flexShrink: 0, '&:hover': { color: 'error.main', bgcolor: 'action.hover' } }}
                    >
                      <Delete size={16} />
                    </IconButton>
                  </Box>
                </Box>
              </Grid>
            ))
          )}
        </Grid>
      </Box>

      <Modal
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        title={editingProfile ? t('proxies.edit_ech') : t('proxies.probe_ech')}
        subtitle={editingProfile ? `${t('proxies.modifying')}: ${editingProfile.name || editingProfile.Name}` : t('proxies.probe_hint')}
        maxWidth="48rem"
      >
        <ECHProfileForm initialData={editingProfile} onSuccess={handleFormSuccess} onCancel={() => setIsModalOpen(false)} />
      </Modal>

      <Modal
        isOpen={isNAT64ModalOpen}
        onClose={() => setIsNAT64ModalOpen(false)}
        title={editingNAT64 ? t('proxies.edit_nat64') : t('proxies.add_nat64')}
        subtitle={editingNAT64 ? `${t('proxies.modifying')}: ${editingNAT64.name}` : t('proxies.nat64_form_subtitle')}
        maxWidth="48rem"
      >
        <NAT64ProfileForm initialData={editingNAT64} onSuccess={handleNAT64FormSuccess} onCancel={() => setIsNAT64ModalOpen(false)} />
      </Modal>
    </Box>
  );
};

export default Proxies;
