import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { alpha } from '@mui/material/styles';
import { Box, Typography, Button, IconButton, Chip, CircularProgress, Tooltip } from '@mui/material';
import { Megaphone, History, RefreshCw, ExternalLink, X, Info } from '../lib/icons';
import Modal from './Modal';
import { GetNotices, OpenURL } from '../api/bindings';
import { useTranslation } from '../i18n/I18nContext';

const READ_KEY = 'snishaper.notices.read_ids';

const loadReadIds = (): string[] => {
  try {
    const raw = localStorage.getItem(READ_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed.filter((item) => typeof item === 'string') : [];
  } catch {
    return [];
  }
};

const saveReadIds = (ids: string[]) => {
  try {
    localStorage.setItem(READ_KEY, JSON.stringify(ids.slice(-400)));
  } catch {
    // storage disabled
  }
};

const levelColor = (level: string): 'info' | 'warning' | 'error' => {
  if (level === 'critical') return 'error';
  if (level === 'important') return 'warning';
  return 'info';
};

const isExpired = (notice: any): boolean => {
  if (!notice?.expires_at) return false;
  const ts = Date.parse(String(notice.expires_at));
  return Number.isFinite(ts) && ts <= Date.now();
};

const formatTime = (value: string, locale: string): string => {
  if (!value) return '';
  const ts = Date.parse(value);
  if (!Number.isFinite(ts)) return value;
  const tag = locale === 'zh' ? 'zh-CN' : locale === 'ru' ? 'ru-RU' : 'en-US';
  return new Date(ts).toLocaleString(tag, { hour12: false });
};

const NoticeCenter: React.FC = () => {
  const { t, language } = useTranslation();
  const [payload, setPayload] = useState<any>(null);
  const [loading, setLoading] = useState(false);
  const [showHistory, setShowHistory] = useState(false);
  const [readIds, setReadIds] = useState<string[]>(() => loadReadIds());

  const load = useCallback(async (force: boolean) => {
    setLoading(true);
    try {
      const data = await GetNotices(force);
      setPayload(data || null);
    } catch (err) {
      console.error('[notices] load failed:', err);
      setPayload((prev: any) => prev || { notices: [], error: String(err) });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load(false);
  }, [load]);

  const notices = useMemo<any[]>(() => (Array.isArray(payload?.notices) ? payload.notices : []), [payload]);

  const pick = useCallback(
    (notice: any, field: 'title' | 'body'): string => {
      const suffix = language === 'zh' ? '' : `_${language}`;
      const localized = suffix ? notice?.[`${field}${suffix}`] : '';
      return localized || notice?.[field] || '';
    },
    [language]
  );

  const unread = useMemo(
    () => notices.filter((notice) => notice?.id && !readIds.includes(notice.id) && !isExpired(notice)),
    [notices, readIds]
  );

  const banner = unread[0] || null;

  const markRead = useCallback(
    (ids: string[]) => {
      if (!ids.length) return;
      setReadIds((prev) => {
        const next = Array.from(new Set([...prev, ...ids]));
        saveReadIds(next);
        return next;
      });
    },
    []
  );

  const openHistory = () => {
    setShowHistory(true);
    markRead(notices.map((notice) => notice?.id).filter(Boolean));
  };

  const openLink = (url: string) => {
    if (!url) return;
    OpenURL(url).catch((err: any) => console.error('[notices] open link failed:', err));
  };

  const activeCount = notices.filter((notice) => !isExpired(notice)).length;

  return (
    <>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, mb: 2 }}>
        {banner && (
          <Box
            sx={{
              display: 'flex',
              alignItems: 'flex-start',
              gap: 1.5,
              p: 1.5,
              pl: 2,
              borderRadius: 2,
              border: 1,
              borderColor: (theme) => alpha(theme.palette[levelColor(banner.level)].main, 0.35),
              bgcolor: (theme) => alpha(theme.palette[levelColor(banner.level)].main, 0.08),
              position: 'relative',
              overflow: 'hidden',
              '&::before': {
                content: '""',
                position: 'absolute',
                left: 0,
                top: 0,
                bottom: 0,
                width: 3,
                bgcolor: `${levelColor(banner.level)}.main`,
              },
            }}
          >
            <Box sx={{ color: `${levelColor(banner.level)}.main`, mt: 0.25, flexShrink: 0 }}>
              <Megaphone size={18} />
            </Box>
            <Box sx={{ minWidth: 0, flexGrow: 1 }}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
                <Typography variant="body2" sx={{ fontWeight: 700, letterSpacing: '-0.01em' }}>
                  {pick(banner, 'title')}
                </Typography>
                {banner.pinned && (
                  <Chip label={t('notices.pinned')} size="small" color={levelColor(banner.level)} variant="outlined" sx={{ height: 18, fontSize: 10 }} />
                )}
                {unread.length > 1 && (
                  <Typography variant="caption" sx={{ color: 'text.secondary' }}>
                    {t('notices.more_unread', { count: unread.length - 1 })}
                  </Typography>
                )}
              </Box>
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{
                  display: '-webkit-box',
                  WebkitLineClamp: 2,
                  WebkitBoxOrient: 'vertical',
                  overflow: 'hidden',
                  lineHeight: 1.6,
                  mt: 0.5,
                  whiteSpace: 'pre-wrap',
                }}
              >
                {pick(banner, 'body')}
              </Typography>
              {banner.link && (
                <Button
                  size="small"
                  variant="text"
                  onClick={() => openLink(banner.link)}
                  sx={{ mt: 0.5, px: 0.5, minWidth: 0, fontSize: 11, fontWeight: 700 }}
                  endIcon={<ExternalLink size={12} />}
                >
                  {banner.link_text || t('notices.link_default')}
                </Button>
              )}
            </Box>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, flexShrink: 0 }}>
              <Tooltip title={t('notices.view_all')} arrow>
                <Button
                  size="small"
                  variant="text"
                  onClick={openHistory}
                  sx={{ fontSize: 11, fontWeight: 700, whiteSpace: 'nowrap' }}
                  startIcon={<History size={14} />}
                >
                  {t('notices.view_all')}
                </Button>
              </Tooltip>
              <Tooltip title={t('notices.mark_read')} arrow>
                <IconButton size="small" onClick={() => markRead([banner.id])} aria-label={t('notices.mark_read')}>
                  <X size={14} />
                </IconButton>
              </Tooltip>
            </Box>
          </Box>
        )}

        {!banner && notices.length > 0 && (
          <Box sx={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button
              size="small"
              variant="text"
              onClick={openHistory}
              sx={{ fontSize: 11, color: 'text.secondary', gap: 0.5 }}
              startIcon={<Megaphone size={13} />}
            >
              {t('notices.history_entry', { count: activeCount })}
            </Button>
          </Box>
        )}

        {payload?.stale && (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, color: 'warning.main' }}>
            <Info size={13} />
            <Typography variant="caption">{t('notices.stale')}</Typography>
          </Box>
        )}
      </Box>

      <Modal
        isOpen={showHistory}
        onClose={() => setShowHistory(false)}
        title={t('notices.title')}
        subtitle={t('notices.subtitle', { count: notices.length })}
        maxWidth="46rem"
        footer={
          <>
            <Button
              size="small"
              variant="text"
              onClick={() => load(true)}
              disabled={loading}
              startIcon={loading ? <CircularProgress size={12} /> : <RefreshCw size={14} />}
            >
              {t('notices.refresh')}
            </Button>
            <Button size="small" variant="contained" onClick={() => setShowHistory(false)}>
              {t('notices.close')}
            </Button>
          </>
        }
      >
        {notices.length === 0 ? (
          <Box sx={{ py: 6, textAlign: 'center', color: 'text.secondary' }}>
            <Megaphone size={28} />
            <Typography variant="body2" sx={{ mt: 1.5 }}>
              {loading ? t('common.loading') : t('notices.empty')}
            </Typography>
          </Box>
        ) : (
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
            {notices.map((notice) => {
              const expired = isExpired(notice);
              const color = levelColor(notice.level);
              return (
                <Box
                  key={notice.id}
                  sx={{
                    p: 1.75,
                    pl: 2,
                    borderRadius: 2,
                    border: 1,
                    borderColor: 'divider',
                    borderLeft: 3,
                    borderLeftColor: expired ? 'divider' : `${color}.main`,
                    bgcolor: 'background.default',
                    opacity: expired ? 0.55 : 1,
                    position: 'relative',
                  }}
                >
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
                    <Typography variant="body2" sx={{ fontWeight: 700, letterSpacing: '-0.01em' }}>
                      {pick(notice, 'title')}
                    </Typography>
                    {notice.pinned && !expired && (
                      <Chip label={t('notices.pinned')} size="small" color={color} variant="outlined" sx={{ height: 18, fontSize: 10 }} />
                    )}
                    {expired && (
                      <Chip label={t('notices.expired')} size="small" variant="outlined" sx={{ height: 18, fontSize: 10 }} />
                    )}
                    <Box sx={{ flexGrow: 1 }} />
                    <Typography variant="caption" color="text.secondary" sx={{ fontVariantNumeric: 'tabular-nums' }}>
                      {formatTime(notice.published_at, language)}
                    </Typography>
                  </Box>
                  <Typography
                    variant="body2"
                    color="text.secondary"
                    sx={{ mt: 0.75, lineHeight: 1.7, whiteSpace: 'pre-wrap', fontSize: '0.8125rem' }}
                  >
                    {pick(notice, 'body')}
                  </Typography>
                  {notice.link && (
                    <Button
                      size="small"
                      variant="text"
                      onClick={() => openLink(notice.link)}
                      sx={{ mt: 0.5, px: 0.5, minWidth: 0, fontSize: 11, fontWeight: 700 }}
                      endIcon={<ExternalLink size={12} />}
                    >
                      {notice.link_text || t('notices.link_default')}
                    </Button>
                  )}
                </Box>
              );
            })}
          </Box>
        )}
      </Modal>
    </>
  );
};

export default NoticeCenter;
