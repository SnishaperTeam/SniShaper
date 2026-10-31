import React, { useState, useEffect, useCallback, useDeferredValue } from 'react';
import { Link, Search, Copy } from '../lib/icons';
import { GetActiveConnections } from '../api/bindings';
import { useTranslation } from '../i18n/I18nContext';
import {
  Box, Typography, TextField, InputAdornment, Table, TableBody, TableCell,
  TableContainer, TableHead, TableRow, Chip, Tooltip, CircularProgress,
} from '@mui/material';
import { alpha } from '@mui/material/styles';

// Wails 绑定按 Go JSON tag 序列化（proxy.ConnInfo），字段为 snake_case。
interface ConnInfo {
  id: number;
  protocol: string;
  host: string;
  dst_addr: string;
  rule: string;
  rule_domain: string;
  mode: string;
  source: string;
  state: string;
  started_at: string;
  bytes_up: number;
  bytes_down: number;
}

const formatBytes = (n: number): string => {
  if (!Number.isFinite(n) || n <= 0) return '0 B';
  const units = ['B', 'KiB', 'MiB', 'GiB'];
  let v = n;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
  return `${v >= 100 ? v.toFixed(0) : v.toFixed(1)} ${units[i]}`;
};

const formatAge = (startedAt: string, now: number, t: (k: string, params?: Record<string, string | number>) => string): string => {
  const start = Date.parse(startedAt);
  if (Number.isNaN(start)) return '—';
  const secs = Math.max(0, Math.floor((now - start) / 1000));
  if (secs < 60) return t('connections.age_seconds', { s: secs });
  if (secs < 3600) return t('connections.age_minutes', { m: Math.floor(secs / 60) });
  return t('connections.age_hours', { h: Math.floor(secs / 3600) });
};

const Connections: React.FC = () => {
  const { t } = useTranslation();
  const [conns, setConns] = useState<ConnInfo[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [search, setSearch] = useState('');
  const deferredSearch = useDeferredValue(search);
  const [now, setNow] = useState(Date.now());
  const [copiedId, setCopiedId] = useState<number | null>(null);

  const fetchConns = useCallback(async () => {
    try {
      const list = await GetActiveConnections();
      setConns(Array.isArray(list) ? (list as ConnInfo[]) : []);
    } catch {
      setConns([]);
    }
    setNow(Date.now());
    setLoaded(true);
  }, []);

  useEffect(() => {
    void fetchConns();
    const interval = setInterval(() => void fetchConns(), 2000);
    return () => clearInterval(interval);
  }, [fetchConns]);

  const filtered = conns.filter((c) => {
    const q = deferredSearch.trim().toLowerCase();
    if (!q) return true;
    return `${c.host} ${c.dst_addr} ${c.rule} ${c.mode} ${c.protocol}`.toLowerCase().includes(q);
  });

  const copyDetail = (c: ConnInfo) => {
    const text = JSON.stringify(c, null, 2);
    void navigator.clipboard.writeText(text).then(() => {
      setCopiedId(c.id);
      setTimeout(() => setCopiedId((prev) => (prev === c.id ? null : prev)), 1200);
    }).catch(() => {});
  };

  const totalUp = conns.reduce((s, c) => s + (c.bytes_up || 0), 0);
  const totalDown = conns.reduce((s, c) => s + (c.bytes_down || 0), 0);

  return (
    <Box sx={{ p: 3, height: '100%', overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}>
        <Link size={22} aria-hidden />
        <Typography variant="h5" component="h1" sx={{ fontWeight: 700 }}>{t('connections.title')}</Typography>
        <Chip
          size="small"
          label={t('connections.count', { n: conns.length })}
          sx={{ bgcolor: (theme: any) => alpha(theme.palette.primary.main, 0.12), color: 'primary.main', fontWeight: 700 }}
        />
        {conns.length > 0 && (
          <Typography variant="body2" color="text.secondary" sx={{ fontVariantNumeric: 'tabular-nums' }}>
            ↑ {formatBytes(totalUp)} · ↓ {formatBytes(totalDown)}
          </Typography>
        )}
        <Box sx={{ flexGrow: 1 }} />
        <TextField
          size="small"
          placeholder={t('connections.search_placeholder')}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          sx={{ width: 240 }}
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <Search size={16} aria-hidden />
                </InputAdornment>
              ),
            },
          }}
        />
      </Box>

      {!loaded ? (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
          <CircularProgress size={28} />
        </Box>
      ) : filtered.length === 0 ? (
        <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1, py: 8, color: 'text.secondary' }}>
          <Typography>{conns.length === 0 ? t('connections.empty') : t('connections.no_match')}</Typography>
        </Box>
      ) : (
        <TableContainer sx={{ flex: 1, overflow: 'auto' }}>
          <Table stickyHeader size="small" aria-label={t('connections.title')}>
            <TableHead>
              <TableRow>
                <TableCell>{t('connections.col_host')}</TableCell>
                <TableCell>{t('connections.col_dst')}</TableCell>
                <TableCell>{t('connections.col_rule')}</TableCell>
                <TableCell>{t('connections.col_mode')}</TableCell>
                <TableCell align="right">{t('connections.col_up')}</TableCell>
                <TableCell align="right">{t('connections.col_down')}</TableCell>
                <TableCell align="right">{t('connections.col_age')}</TableCell>
                <TableCell>{t('connections.col_state')}</TableCell>
                <TableCell padding="checkbox" aria-label={t('connections.copy_detail')} />
              </TableRow>
            </TableHead>
            <TableBody>
              {filtered.map((c) => (
                <TableRow key={c.id} hover>
                  <TableCell sx={{ maxWidth: 260, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={c.host || undefined}>
                    {c.host || '—'}
                  </TableCell>
                  <TableCell sx={{ maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontVariantNumeric: 'tabular-nums' }} title={c.dst_addr || undefined}>
                    {c.dst_addr || '—'}
                  </TableCell>
                  <TableCell>{c.rule || '—'}</TableCell>
                  <TableCell>{c.mode || '—'}</TableCell>
                  <TableCell align="right" sx={{ fontVariantNumeric: 'tabular-nums' }}>{formatBytes(c.bytes_up)}</TableCell>
                  <TableCell align="right" sx={{ fontVariantNumeric: 'tabular-nums' }}>{formatBytes(c.bytes_down)}</TableCell>
                  <TableCell align="right" sx={{ fontVariantNumeric: 'tabular-nums' }}>{formatAge(c.started_at, now, t)}</TableCell>
                  <TableCell>
                    <Chip
                      size="small"
                      label={c.state === 'active' ? t('connections.state_active') : (c.state || '—')}
                      color={c.state === 'active' ? 'success' : 'default'}
                      variant={c.state === 'active' ? 'filled' : 'outlined'}
                      sx={{ height: 20, fontSize: '0.7rem' }}
                    />
                  </TableCell>
                  <TableCell padding="checkbox">
                    <Tooltip title={t('connections.copy_detail')}>
                      <Box
                        component="span"
                        role="button"
                        tabIndex={0}
                        aria-label={t('connections.copy_detail')}
                        onClick={() => copyDetail(c)}
                        onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') copyDetail(c); }}
                        sx={{ cursor: 'pointer', display: 'inline-flex', color: copiedId === c.id ? 'success.main' : 'text.secondary', '&:hover': { color: 'primary.main' } }}
                      >
                        <Copy size={15} aria-hidden />
                      </Box>
                    </Tooltip>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
    </Box>
  );
};

export default Connections;
