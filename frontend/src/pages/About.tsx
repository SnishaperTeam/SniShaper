import React, { useEffect, useState } from 'react';
import { Box, Button, Grid, Typography } from '@mui/material';
import { alpha } from '@mui/material/styles';
import { useTranslation } from '../i18n/I18nContext';
import {
  Globe, Link as LinkIcon, Users, Shield, Heart, RefreshCw,
  Download, Sparkles, Zap, Lock, Code2, GitBranch, Megaphone, Map, ExternalLink
} from '../lib/icons';
import logoUrl from '../assets/logo.svg';
import { GetAppVersion, CheckUpdate, OpenURL } from '../api/bindings';
import Modal from '../components/Modal';
import { toast } from '../lib/toast';

interface UpdateResult {
  has_update: boolean;
  latest_version: string;
  download_url: string;
  message: string;
  error_detail?: string;
}

const softBg = (token: string, opacity: number) => (theme: any) =>
  alpha(theme.palette[token.split('.')[0]][token.split('.')[1]], opacity);

const About: React.FC = () => {
  const { t } = useTranslation();
  const [version, setVersion] = useState<string>('1.29');
  const [checkingUpdate, setCheckingUpdate] = useState<boolean>(false);
  const [showUpdateModal, setShowUpdateModal] = useState<boolean>(false);
  const [updateInfo, setUpdateInfo] = useState<{ latestVersion: string; downloadURL: string } | null>(null);

  useEffect(() => {
    GetAppVersion().then((v) => { if (v) setVersion(v); }).catch(() => setVersion('1.29'));
  }, []);

  const handleOpenWebsite = () => OpenURL('https://github.com/SniShaper/SniShaper');
  const handleOpenGitHub = () => OpenURL('https://github.com/SniShaper/SniShaper');
  const handleOpenBeta = () => OpenURL('https://github.com/SniShaper/SniShaper/actions');
  const handleOpenAdaptation = () => OpenURL('https://github.com/SniShaper/SniShaper/issues/32');
  const handleOpenDevPlan = () => OpenURL('https://github.com/SniShaper/SniShaper/issues/36');

  const handleDownloadUpdate = () => {
    if (updateInfo) { OpenURL(updateInfo.downloadURL); setShowUpdateModal(false); }
  };

  const handleCheckUpdate = async () => {
    if (checkingUpdate) return;
    setCheckingUpdate(true);
    try {
      const result: UpdateResult = await CheckUpdate();
      switch (result.message) {
        case 'update_available':
          setUpdateInfo({ latestVersion: result.latest_version, downloadURL: result.download_url });
          setShowUpdateModal(true);
          break;
        case 'up_to_date':
          toast.success(t('about.up_to_date'), t('about.up_to_date_desc').replace('{version}', version));
          break;
        case 'dev_version':
          toast.info(t('about.dev_version'), t('about.dev_version_desc').replace('{version}', version).replace('{latestVersion}', result.latest_version));
          break;
        case 'check_failed':
        default:
          const errorKey = result.error_detail || 'check_failed';
          toast.error(t('about.check_failed'), t(`about.${errorKey}`) || t('about.check_failed_desc'));
          break;
      }
    } catch (error) {
      toast.error(t('about.check_failed'), t('about.check_failed_desc'));
    } finally { setCheckingUpdate(false); }
  };

  const features = [
    { icon: <Lock size={20} />, title: t('about.feature_ech'), desc: t('about.feature_ech_desc'), color: 'primary.main' },
    { icon: <Zap size={20} />, title: t('about.feature_fast'), desc: t('about.feature_fast_desc'), color: 'success.main' },
    { icon: <Code2 size={20} />, title: t('about.feature_open'), desc: t('about.feature_open_desc'), color: 'warning.main' },
  ];

  const communityCards = [
    { onClick: handleOpenAdaptation, icon: <Megaphone size={24} />, title: t('about.site_adaptation'), desc: t('about.site_adaptation_desc'), action: t('about.participate_now'), color: 'primary.main' },
    { onClick: handleOpenDevPlan, icon: <Map size={24} />, title: t('about.development_plan'), desc: t('about.development_plan_desc'), action: t('about.view_plan'), color: 'success.main' },
  ];

  const infoCards = [
    { icon: <Heart size={22} />, title: t('about.contributors'), value: 'mechrevo, dongzheyu, JetCPP-dongle', color: 'success.main', valueColor: 'text.primary' },
    { icon: <Users size={22} />, title: t('about.maintainers'), value: 'JetCPP Team, SniShaper Team', color: 'warning.main', valueColor: 'text.primary' },
    { icon: <Globe size={22} />, title: t('about.website'), value: 'github.com/SniShaper/SniShaper', color: 'primary.main', valueColor: 'primary.main', onClick: handleOpenWebsite },
    { icon: <GitBranch size={22} />, title: 'GitHub', value: 'SniShaper/SniShaper', color: 'text.primary', valueColor: 'text.primary', onClick: handleOpenGitHub },
    { icon: <Download size={22} />, title: t('about.latest_beta'), value: t('about.actions_build'), color: 'warning.main', valueColor: 'warning.main', onClick: handleOpenBeta },
  ];

  return (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column', overflowY: 'auto' }}>
      <Box sx={{ flex: 1, p: 4, maxWidth: '64rem', mx: 'auto', width: '100%' }}>
        <Box sx={{ position: 'relative', mb: 6, p: 4, borderRadius: 3, border: 1, borderColor: 'divider', overflow: 'hidden', background: (theme) => `linear-gradient(135deg, ${alpha(theme.palette.primary.main, 0.05)}, ${theme.palette.background.paper}, ${alpha(theme.palette.primary.main, 0.05)})` }}>
          <Box sx={{ position: 'absolute', inset: 0, opacity: 0.5, background: (theme) => `radial-gradient(ellipse at top right, ${alpha(theme.palette.primary.main, 0.08)}, transparent 50%)` }} />
          <Box sx={{ position: 'relative', display: 'flex', flexDirection: 'column', alignItems: 'center', textAlign: 'center' }}>
            <Box sx={{ position: 'relative', mb: 3 }}>
              <Box sx={{ position: 'absolute', inset: 0, borderRadius: '50%', bgcolor: (theme) => alpha(theme.palette.primary.main, 0.2), filter: 'blur(24px)' }} />
              <Box component="img" src={logoUrl} alt="SniShaper logo" sx={{ position: 'relative', width: 112, height: 112, objectFit: 'contain', filter: 'drop-shadow(0 10px 30px rgba(33, 150, 243, 0.3))' }} />
            </Box>
            <Typography variant="h1" sx={{ fontSize: '2.25rem', fontWeight: 900, color: 'text.primary', mb: 0.5, letterSpacing: '-0.025em' }}>SniShaper</Typography>
            <Typography sx={{ fontSize: '1.125rem', fontWeight: 500, color: 'text.secondary', mb: 2 }}>{t('about.title')}</Typography>
            <Box sx={{ display: 'inline-flex', alignItems: 'center', gap: 1, px: 2.5, py: 1.25, borderRadius: '999px', bgcolor: (theme) => alpha(theme.palette.primary.main, 0.1), border: 1, borderColor: (theme) => alpha(theme.palette.primary.main, 0.2), backdropFilter: 'blur(4px)' }}>
              <Shield size={16} aria-hidden />
              <Typography variant="body2" sx={{ fontWeight: 700, color: 'primary.main' }}>{t('about.version')}: {version}</Typography>
            </Box>
          </Box>
        </Box>

        <Box sx={{ mb: 5, p: 3, borderRadius: 2, bgcolor: 'background.paper', border: 1, borderColor: 'divider' }}>
          <Typography sx={{ color: 'text.secondary', textAlign: 'center', lineHeight: 1.625, fontSize: '0.9375rem' }}>{t('about.description')}</Typography>
        </Box>

        <Box sx={{ mb: 5 }}>
          <Typography variant="h2" sx={{ fontSize: '1.125rem', fontWeight: 700, color: 'text.primary', mb: 2.5, display: 'flex', alignItems: 'center', gap: 1 }}>
            <Box component="span" sx={{ display: 'inline-flex', color: 'primary.main' }}><Sparkles size={20} aria-hidden /></Box>
            {t('about.features')}
          </Typography>
          <Grid container spacing={2}>
            {features.map((f, i) => (
              <Grid key={i} size={{ xs: 12, md: 4 }}>
                <Box sx={{ height: '100%', p: 3, borderRadius: 2, bgcolor: 'background.paper', border: 1, borderColor: 'divider' }}>
                  <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', width: 44, height: 44, borderRadius: 2, bgcolor: softBg(f.color, 0.1), color: f.color }}>{f.icon}</Box>
                  <Typography sx={{ mt: 2, fontWeight: 700, color: 'text.primary' }}>{f.title}</Typography>
                  <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5, lineHeight: 1.625 }}>{f.desc}</Typography>
                </Box>
              </Grid>
            ))}
          </Grid>
        </Box>

        <Box sx={{ mb: 5 }}>
          <Typography variant="h2" sx={{ fontSize: '1.125rem', fontWeight: 700, color: 'text.primary', mb: 2.5, display: 'flex', alignItems: 'center', gap: 1 }}>
            <Box component="span" sx={{ display: 'inline-flex', color: 'error.main' }}><Heart size={20} aria-hidden /></Box>
            {t('about.community')}
          </Typography>
          <Grid container spacing={2.5}>
            {communityCards.map((c, i) => (
              <Grid key={i} size={{ xs: 12, md: 6 }}>
                <Box
                  role="button"
                  tabIndex={0}
                  onClick={c.onClick}
                  onKeyDown={(e) => e.key === 'Enter' && c.onClick()}
                  aria-label={c.title}
                  sx={{
                    p: 3,
                    borderRadius: 2,
                    cursor: 'pointer',
                    border: 1,
                    borderColor: 'divider',
                    transition: 'all 0.3s',
                    background: (theme: any) => `linear-gradient(135deg, ${alpha(theme.palette[c.color.split('.')[0]][c.color.split('.')[1]], 0.05)}, ${theme.palette.background.paper})`,
                    '--reveal': 0,
                    '--chip-bg': softBg(c.color, 0.1),
                    '&:hover': {
                      borderColor: softBg(c.color, 0.3),
                      boxShadow: 8,
                      transform: 'translateY(-2px)',
                      '--reveal': 1,
                      '--chip-bg': softBg(c.color, 0.15),
                    },
                  }}
                >
                  <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 2 }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', p: 1.5, borderRadius: 2, bgcolor: 'var(--chip-bg)', color: c.color, transition: 'background-color 0.3s' }}>{c.icon}</Box>
                    <Box sx={{ flex: 1 }}>
                      <Typography sx={{ fontSize: '1rem', fontWeight: 700, color: 'text.primary', mb: 0.75, display: 'flex', alignItems: 'center', gap: 1 }}>
                        {c.title}
                        <Box component="span" sx={{ display: 'inline-flex', color: 'text.secondary', opacity: 'var(--reveal, 0)', transition: 'opacity 0.3s' }}><ExternalLink size={14} /></Box>
                      </Typography>
                      <Typography sx={{ fontSize: '0.875rem', color: 'text.secondary', mb: 1.5, lineHeight: 1.625 }}>{c.desc}</Typography>
                      <Box component="span" sx={{ display: 'inline-flex', alignItems: 'center', gap: 0.75, fontSize: '0.875rem', fontWeight: 700, color: c.color }}>
                        {c.action}
                        <ExternalLink size={14} />
                      </Box>
                    </Box>
                  </Box>
                </Box>
              </Grid>
            ))}
          </Grid>
        </Box>

        <Grid container spacing={2.5} sx={{ mb: 5 }}>
          {infoCards.map((c, i) => (
            <Grid key={i} size={{ xs: 12, md: 6 }}>
              <Box
                {...(c.onClick ? { role: 'button', tabIndex: 0, onClick: c.onClick, onKeyDown: (e: React.KeyboardEvent) => e.key === 'Enter' && c.onClick() } : {})}
                sx={{
                  p: 2.5,
                  borderRadius: 2,
                  bgcolor: 'background.paper',
                  border: 1,
                  borderColor: 'divider',
                  transition: 'all 0.3s',
                  ...(c.onClick ? { cursor: 'pointer' } : {}),
                  '--chip-bg': softBg(c.color, 0.1),
                  '&:hover': {
                    borderColor: softBg(c.color, 0.3),
                    boxShadow: 8,
                    '--chip-bg': softBg(c.color, 0.15),
                  },
                }}
              >
                <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 2 }}>
                  <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', p: 1.5, borderRadius: 2, bgcolor: 'var(--chip-bg)', color: c.color, transition: 'background-color 0.3s' }}>{c.icon}</Box>
                  <Box sx={{ flex: 1 }}>
                    <Typography sx={{ fontSize: '0.75rem', fontWeight: 700, color: 'text.secondary', textTransform: 'uppercase', letterSpacing: '0.05em', mb: 0.75 }}>{c.title}</Typography>
                    <Typography sx={{ fontSize: '0.9375rem', fontWeight: 600, color: c.valueColor, lineHeight: 1.375 }}>{c.value}</Typography>
                  </Box>
                </Box>
              </Box>
            </Grid>
          ))}
        </Grid>

        <Box sx={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'center', gap: 2, mb: 6 }}>
          <Button onClick={handleCheckUpdate} variant="contained" size="large" loading={checkingUpdate} loadingPosition="start" startIcon={checkingUpdate ? undefined : <RefreshCw size={18} />}>
            {checkingUpdate ? t('about.checking') : t('about.check_update')}
          </Button>
          <Button onClick={handleOpenWebsite} variant="outlined" size="large" startIcon={<Globe size={18} />}>{t('about.website')}</Button>
          <Button onClick={handleOpenGitHub} variant="outlined" size="large" startIcon={<LinkIcon size={18} />}>GitHub</Button>
        </Box>

        <Box component="footer" sx={{ textAlign: 'center', pb: 2 }}>
          <Typography variant="caption" sx={{ display: 'block', fontSize: '0.75rem', color: 'text.secondary' }}>© 2025-2026 SniShaper. {t('about.rights_reserved')}</Typography>
          <Typography variant="caption" sx={{ display: 'block', fontSize: '0.6875rem', color: 'text.secondary', opacity: 0.6, mt: 1 }}>{t('about.made_with')} ❤️ {t('about.by_community')}</Typography>
        </Box>
      </Box>

      <Modal isOpen={showUpdateModal} onClose={() => setShowUpdateModal(false)} title={updateInfo ? t('about.update_available') : t('common.status')}>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          {updateInfo ? (
            <>
              <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1.5, p: 2, bgcolor: (theme) => alpha(theme.palette.primary.main, 0.1), border: 1, borderColor: (theme) => alpha(theme.palette.primary.main, 0.2), borderRadius: 2 }}>
                <Box component="span" sx={{ display: 'inline-flex', color: 'primary.main', flexShrink: 0, mt: 0.25 }}><Download size={20} aria-hidden /></Box>
                <Box>
                  <Typography sx={{ fontSize: '0.875rem', fontWeight: 700, color: 'text.primary', mb: 0.5 }}>{t('about.update_available')}</Typography>
                  <Typography sx={{ fontSize: '0.75rem', color: 'text.secondary' }}>{t('about.update_available_desc').replace('{version}', updateInfo.latestVersion)}</Typography>
                </Box>
              </Box>
              <Box sx={{ display: 'flex', gap: 1.5 }}>
                <Button onClick={handleDownloadUpdate} startIcon={<Download size={16} />} sx={{ flex: 1 }}>{t('common.confirm')}</Button>
                <Button onClick={() => setShowUpdateModal(false)} variant="outlined" sx={{ flex: 1 }}>{t('common.cancel')}</Button>
              </Box>
            </>
          ) : (
            <Box sx={{ textAlign: 'center', py: 2 }}>
              <Typography sx={{ fontSize: '0.875rem', color: 'text.secondary' }}>{t('about.up_to_date_desc').replace('{version}', version)}</Typography>
            </Box>
          )}
        </Box>
      </Modal>
    </Box>
  );
};

export default About;
