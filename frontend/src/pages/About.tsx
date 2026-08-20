import React, { useEffect, useState, useRef, useCallback } from 'react';
import { Box, Button, Grid, Typography, LinearProgress } from '@mui/material';
import { alpha } from '@mui/material/styles';
import { useTranslation } from '../i18n/I18nContext';
import {
  Globe, Link as LinkIcon, Users, Shield, Heart, RefreshCw,
  Download, Sparkles, Zap, Lock, Code2, GitBranch, Megaphone, Map, ExternalLink,
  FolderOpen, AlertCircle
} from '../lib/icons';
import logoUrl from '../assets/logo.svg';
import { GetAppVersion, GetReleaseChannel, GetPendingUpdate, CheckUpdate, OpenURL, DownloadUpdateAsset, InstallUpdateAsset, EventsOn } from '../api/bindings';
import Modal from '../components/Modal';
import { toast } from '../lib/toast';

interface UpdateAsset {
  name: string;
  size: number;
  download_url: string;
  kind: string;
}

interface UpdateResult {
  has_update: boolean;
  latest_version: string;
  channel: string;
  release_name: string;
  assets: UpdateAsset[];
  download_url: string;
  message: string;
  error_detail?: string;
}

const softBg = (token: string, opacity: number) => (theme: any) =>
  alpha(theme.palette[token.split('.')[0]][token.split('.')[1]], opacity);

const fmtSize = (bytes: number) => {
  if (!bytes || bytes <= 0) return '---';
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${bytes} B`;
};

const fmtSpeed = (bps: number) => {
  if (!bps || bps <= 0) return '';
  if (bps >= 1024 * 1024) return `${(bps / 1024 / 1024).toFixed(1)} MB/s`;
  if (bps >= 1024) return `${(bps / 1024).toFixed(0)} KB/s`;
  return `${bps.toFixed(0)} B/s`;
};

const About: React.FC = () => {
  const { t } = useTranslation();
  const [version, setVersion] = useState<string>('1.29');
  const [checkingUpdate, setCheckingUpdate] = useState<boolean>(false);
  const [showUpdateModal, setShowUpdateModal] = useState<boolean>(false);
  const [showRestartPrompt, setShowRestartPrompt] = useState<boolean>(false);
  const [updateInfo, setUpdateInfo] = useState<UpdateResult | null>(null);
  const [downloadProgress, setDownloadProgress] = useState<number>(0);
  const [downloadSpeed, setDownloadSpeed] = useState<number>(0);
  const [downloadReceived, setDownloadReceived] = useState<number>(0);
  const [downloadTotal, setDownloadTotal] = useState<number>(0);
  const [pendingUpdate, setPendingUpdate] = useState<string | null>(null);
  const [releaseChannel, setReleaseChannel] = useState<string>('stable');
  const downloadingRef = useRef<string | null>(null);

  const refreshPending = useCallback(async () => {
    try {
      const p = await GetPendingUpdate();
      setPendingUpdate(p || null);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => {
    GetAppVersion().then((v) => { if (v) setVersion(v); }).catch(() => setVersion('1.29'));
  }, []);

  useEffect(() => {
    GetReleaseChannel().then((c) => { if (c) setReleaseChannel(c); }).catch(() => {});
  }, []);

  useEffect(() => {
    refreshPending();
  }, [refreshPending]);

  useEffect(() => {
    const off = EventsOn('update:download_progress', (data: any) => {
      if (data && downloadingRef.current && data.asset_name === downloadingRef.current) {
        setDownloadProgress(Math.round(data.percent || 0));
        setDownloadSpeed(data.speed || 0);
        setDownloadReceived(data.received || 0);
        setDownloadTotal(data.total || 0);
      }
    });
    return off;
  }, []);

  const channelMap: Record<string, { key: string; color: string }> = {
    alpha: { key: 'about.channel_alpha', color: 'error.main' },
    beta: { key: 'about.channel_beta', color: 'warning.main' },
    rc: { key: 'about.channel_rc', color: 'primary.main' },
    stable: { key: 'about.channel_stable', color: 'success.main' },
  };
  const channelConf = channelMap[releaseChannel] || channelMap.stable;

  const handleOpenWebsite = () => OpenURL('https://jetcpp.dpdns.org');
  const handleOpenGitHub = () => OpenURL('https://github.com/SniShaper/SniShaper');
  const handleOpenBeta = () => OpenURL('https://github.com/SniShaper/SniShaper/actions');
  const handleOpenAdaptation = () => OpenURL('https://github.com/SniShaper/SniShaper/issues/95');
  const handleOpenDevPlan = () => OpenURL('https://github.com/SniShaper/SniShaper/issues/36');

  const handleCheckUpdate = async () => {
    if (checkingUpdate) return;
    setCheckingUpdate(true);
    try {
      const result: UpdateResult = await CheckUpdate();
      switch (result.message) {
        case 'update_available':
          setUpdateInfo(result);
          setDownloadProgress(0);
          setShowUpdateModal(true);
          break;
        case 'up_to_date':
          toast.success(t('about.up_to_date'), t('about.up_to_date_desc').replace('{version}', version));
          break;
        case 'dev_version':
          toast.info(t('about.dev_version'), t('about.dev_version_desc').replace('{version}', version).replace('{latestVersion}', result.latest_version));
          break;
        case 'no_release_found':
          toast.info(t('about.no_release_found'), t('about.no_release_found_desc'));
          break;
        case 'check_failed':
        default: {
          const errorKey = result.error_detail || 'check_failed';
          const errText = t(`about.${errorKey}`);
          const errorDesc = errText === `about.${errorKey}` ? t('about.check_failed_desc') : errText;
          toast.error(t('about.check_failed'), errorDesc);
          break;
        }
      }
    } catch (error) {
      toast.error(t('about.check_failed'), t('about.check_failed_desc'));
    } finally { setCheckingUpdate(false); }
  };

  const handleInstall = async (localPath: string) => {
    if (!localPath) return;
    try {
      await InstallUpdateAsset(localPath);
      const is7z = localPath.toLowerCase().endsWith('.7z');
      setShowRestartPrompt(false);
      setPendingUpdate(null);
      setShowUpdateModal(false);
      toast.success(is7z ? t('about.install_7z_started') : t('about.install_started'));
    } catch (err: any) {
      const msg = String(err?.message || err);
      const keyMap: Record<string, string> = {
        sevenzip_missing: t('about.sevenzip_missing'),
        dir_not_writable: t('about.dir_not_writable'),
        extract_failed: t('about.extract_failed'),
        bad_archive: t('about.bad_archive'),
      };
      toast.error(t('about.install_failed'), keyMap[msg] || msg);
    }
  };

  const handleDownload = async (asset: UpdateAsset) => {
    if (downloadingRef.current) return;
    downloadingRef.current = asset.name;
    setDownloadProgress(0);
    try {
      const result = await DownloadUpdateAsset(asset.download_url);
      await refreshPending();
      toast.success(t('about.download_done'), asset.name);
      if (asset.kind === '7z') {
        setShowRestartPrompt(true);
      } else {
        await handleInstall(result?.local_path || '');
      }
    } catch (err: any) {
      toast.error(t('about.download_failed'), String(err?.message || err));
    } finally {
      downloadingRef.current = null;
    }
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
    { icon: <Globe size={22} />, title: t('about.website'), value: 'jetcpp.dpdns.org', color: 'primary.main', valueColor: 'primary.main', onClick: handleOpenWebsite },
    { icon: <GitBranch size={22} />, title: 'GitHub', value: 'SniShaper/SniShaper', color: 'text.primary', valueColor: 'text.primary', onClick: handleOpenGitHub },
    { icon: <Download size={22} />, title: t('about.latest_beta'), value: t('about.actions_build'), color: 'warning.main', valueColor: 'warning.main', onClick: handleOpenBeta },
  ];

  return (
    <Box sx={{ flexGrow: 1, minHeight: 0, width: '100%', display: 'flex', flexDirection: 'column', overflowY: 'auto', overflowX: 'hidden' }}>
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
            <Box sx={{ display: 'inline-flex', alignItems: 'center', gap: 1.5, px: 2.5, py: 1.25, borderRadius: '999px', bgcolor: (theme) => alpha(theme.palette.primary.main, 0.1), border: 1, borderColor: (theme) => alpha(theme.palette.primary.main, 0.2) }}>
              <Shield size={16} aria-hidden />
              <Typography variant="body2" sx={{ fontWeight: 700, color: 'primary.main' }}>{t('about.version')}: {version}</Typography>
              <Box component="span" sx={{ px: 1, py: 0.25, borderRadius: '999px', fontSize: '0.6875rem', fontWeight: 700, letterSpacing: '0.03em', color: channelConf.color, bgcolor: (theme) => {
                const parts = channelConf.color.split('.');
                return alpha((theme.palette as any)[parts[0]]?.[parts[1]] ?? theme.palette.primary.main, 0.12);
              } }}>
                {t(channelConf.key)}
              </Box>
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
                    <Box sx={{ flex: 1, minWidth: 0 }}>
                      <Typography sx={{ fontSize: '1rem', fontWeight: 700, color: 'text.primary', mb: 0.75, display: 'flex', alignItems: 'center', gap: 1, overflow: 'hidden', textOverflow: 'ellipsis', maxWidth: '100%' }}>
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
                  <Box sx={{ flex: 1, minWidth: 0 }}>
                    <Typography sx={{ fontSize: '0.75rem', fontWeight: 700, color: 'text.secondary', textTransform: 'uppercase', letterSpacing: '0.05em', mb: 0.75 }}>{c.title}</Typography>
                    <Typography sx={{ fontSize: '0.9375rem', fontWeight: 600, color: c.valueColor, lineHeight: 1.375 }}>{c.value}</Typography>
                  </Box>
                </Box>
              </Box>
            </Grid>
          ))}
        </Grid>

        <Box sx={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'center', gap: 2, mb: 6 }}>
          {pendingUpdate ? (
            <Button onClick={() => handleInstall(pendingUpdate)} variant="contained" size="large" startIcon={<RefreshCw size={18} />}>
              {t('about.restart_to_update')}
            </Button>
          ) : (
            <Button onClick={handleCheckUpdate} variant="contained" size="large" loading={checkingUpdate} loadingPosition="start" startIcon={checkingUpdate ? undefined : <RefreshCw size={18} />}>
              {checkingUpdate ? t('about.checking') : t('about.check_update')}
            </Button>
          )}
          <Button onClick={handleOpenWebsite} variant="outlined" size="large" startIcon={<Globe size={18} />}>{t('about.website')}</Button>
          <Button onClick={handleOpenGitHub} variant="outlined" size="large" startIcon={<LinkIcon size={18} />}>GitHub</Button>
        </Box>

        <Box component="footer" sx={{ textAlign: 'center', pb: 2 }}>
          <Typography variant="caption" sx={{ display: 'block', fontSize: '0.75rem', color: 'text.secondary' }}>© 2025-2026 SniShaper. {t('about.rights_reserved')}</Typography>
          <Typography variant="caption" sx={{ display: 'block', fontSize: '0.6875rem', color: 'text.secondary', opacity: 0.6, mt: 1 }}>{t('about.made_with')} ❤️ {t('about.by_community')}</Typography>
        </Box>
      </Box>

      <Modal isOpen={showUpdateModal} onClose={() => setShowUpdateModal(false)} title={t('about.update_available')}>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          {updateInfo && (
            <>
              <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1.5, p: 2, bgcolor: (theme) => alpha(theme.palette.primary.main, 0.1), border: 1, borderColor: (theme) => alpha(theme.palette.primary.main, 0.2), borderRadius: 2 }}>
                <Box component="span" sx={{ display: 'inline-flex', color: 'primary.main', flexShrink: 0, mt: 0.25 }}><Download size={20} aria-hidden /></Box>
                <Box sx={{ minWidth: 0 }}>
                  <Typography sx={{ fontSize: '0.875rem', fontWeight: 700, color: 'text.primary', mb: 0.5 }}>{t('about.update_available')}</Typography>
                  <Typography sx={{ fontSize: '0.75rem', color: 'text.secondary', wordBreak: 'break-all' }}>
                    {updateInfo.release_name || updateInfo.latest_version}
                  </Typography>
                  <Typography sx={{ fontSize: '0.6875rem', color: 'text.secondary', mt: 0.5, textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                    {updateInfo.channel} · {updateInfo.latest_version}
                  </Typography>
                </Box>
              </Box>

              {updateInfo.assets && updateInfo.assets.length > 0 ? (
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                  <Typography variant="caption" sx={{ fontWeight: 'bold', color: 'text.secondary', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                    {t('about.choose_download')}
                  </Typography>
                  {updateInfo.assets.map((asset) => {
                    const is7z = asset.kind === '7z';
                    const isDownloading = downloadingRef.current === asset.name;
                    return (
                      <Box key={asset.name} sx={{ p: 1.5, display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1.5, border: 1, borderColor: 'divider', borderRadius: 1.5, bgcolor: 'background.default' }}>
                        <Box sx={{ display: 'flex', gap: 1.5, alignItems: 'center', minWidth: 0, flex: 1 }}>
                          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', width: 36, height: 36, borderRadius: 1, flexShrink: 0, bgcolor: is7z ? 'rgba(234,179,8,0.12)' : 'rgba(33,150,243,0.12)', color: is7z ? 'warning.main' : 'primary.main' }}>
                            {is7z ? <FolderOpen size={18} /> : <Download size={18} />}
                          </Box>
                          <Box sx={{ minWidth: 0 }}>
                            <Typography sx={{ fontSize: '0.75rem', fontWeight: 'bold', color: 'text.primary', wordBreak: 'break-all', display: 'block' }}>
                              {asset.name}
                            </Typography>
                            <Typography variant="caption" sx={{ fontSize: '0.6875rem', color: 'text.secondary', display: 'block' }}>
                              {t(is7z ? 'about.asset_7z' : 'about.asset_exe')} · {t('about.asset_size')}: {fmtSize(asset.size)}
                            </Typography>
                          </Box>
                        </Box>
                        {isDownloading ? (
                          <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: 0.5, minWidth: 120, flexShrink: 0 }}>
                            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, width: '100%' }}>
                              <LinearProgress variant="determinate" value={downloadProgress} sx={{ flex: 1 }} />
                              <Typography variant="caption" sx={{ fontSize: '0.6875rem', color: 'text.secondary', flexShrink: 0 }}>
                                {downloadProgress}%
                              </Typography>
                            </Box>
                            <Typography variant="caption" sx={{ fontSize: '0.6875rem', color: 'text.secondary', display: 'flex', gap: 1 }}>
                              <Box component="span">{fmtSize(downloadReceived)} / {fmtSize(downloadTotal)}</Box>
                              {downloadSpeed > 0 && <Box component="span" sx={{ color: 'primary.main' }}>{fmtSpeed(downloadSpeed)}</Box>}
                            </Typography>
                          </Box>
                        ) : (
                          <Button size="small" variant="outlined" startIcon={<Download size={14} />} sx={{ flexShrink: 0 }} onClick={() => handleDownload(asset)}>
                            {t('about.download')}
                          </Button>
                        )}
                      </Box>
                    );
                  })}
                </Box>
              ) : (
                <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1, py: 2, color: 'text.secondary' }}>
                  <AlertCircle size={28} />
                  <Typography sx={{ fontSize: '0.8125rem', fontWeight: 700 }}>{t('about.no_assets')}</Typography>
                  <Typography sx={{ fontSize: '0.75rem', color: 'text.secondary', textAlign: 'center' }}>{t('about.no_assets_desc')}</Typography>
                  <Button size="small" variant="outlined" sx={{ mt: 1 }} startIcon={<ExternalLink size={14} />} onClick={() => OpenURL('https://github.com/SniShaper/SniShaper/releases')}>
                    {t('about.open_github')}
                  </Button>
                </Box>
              )}

              <Box sx={{ display: 'flex', gap: 1.5, justifyContent: 'flex-end' }}>
                <Button onClick={() => setShowUpdateModal(false)} variant="outlined">{t('common.cancel')}</Button>
              </Box>
            </>
          )}
        </Box>
      </Modal>

      <Modal isOpen={showRestartPrompt} onClose={() => setShowRestartPrompt(false)} title={t('about.download_done')}>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          <Typography sx={{ fontSize: '0.8125rem', color: 'text.secondary', lineHeight: 1.625 }}>{t('about.restart_prompt_desc')}</Typography>
          <Box sx={{ display: 'flex', gap: 1.5, justifyContent: 'flex-end' }}>
            <Button variant="contained" onClick={() => pendingUpdate && handleInstall(pendingUpdate)}>{t('about.restart_now')}</Button>
            <Button variant="outlined" onClick={() => setShowRestartPrompt(false)}>{t('about.restart_later')}</Button>
          </Box>
        </Box>
      </Modal>
    </Box>
  );
};

export default About;
