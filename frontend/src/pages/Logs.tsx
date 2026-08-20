import React, { useState, useEffect, useRef, useMemo, useCallback, useDeferredValue } from 'react';
import {
  FileText, Trash2, Pause, Play, Search, ChevronsUp, Radio, ArrowDown, Download
} from '../lib/icons';
import {
  ClearLogs, GetRecentLogs, IsLogCaptureEnabled,
  StartLogCapture, StopLogCapture
} from '../api/bindings';
import { useTranslation } from '../i18n/I18nContext';
import { Box, Typography, Button, TextField, InputAdornment } from '@mui/material';
import { alpha } from '@mui/material/styles';
import { keyframes } from '@emotion/react';

const RE_ERROR = /error|failed|panic/i;
const RE_WARN = /warn/i;
const RE_LINE_PARSE = /^(\d{4}\/\d{2}\/\d{2}) (\d{2}:\d{2}:\d{2})(?:\.\d+)?\s+(.*)$/;

type Parsed = { date: string; time: string; msg: string; level: 'error' | 'warn' | 'info' };

const parseLine = (text: string): Parsed => {
  const match = text.match(RE_LINE_PARSE);
  if (!match) return { date: '', time: '--:--:--', msg: text, level: 'info' };
  const msg = match[3];
  const level: Parsed['level'] = RE_ERROR.test(msg) ? 'error' : RE_WARN.test(msg) ? 'warn' : 'info';
  return { date: match[1], time: match[2], msg, level };
};

const pulse = keyframes`0%,100%{opacity:1}50%{opacity:0.4}`;
const bounce = keyframes`0%,100%{transform:translateY(0)}50%{transform:translateY(-2px)}`;

const LogLine: React.FC<{ line: string }> = React.memo(({ line }) => {
  const { time, msg, level } = parseLine(line);
  const { t } = useTranslation();

  const levelColor = level === 'error' ? 'error.main' : level === 'warn' ? 'warning.main' : 'text.primary';
  const borderColor = level === 'error' ? 'rgba(239,68,68,0.7)' : level === 'warn' ? 'rgba(245,158,11,0.7)' : 'transparent';
  const bgColor = level === 'error' ? 'rgba(239,68,68,0.03)' : level === 'warn' ? 'rgba(245,158,11,0.03)' : 'transparent';
  const badgeBg = level === 'error' ? 'rgba(239,68,68,0.2)' : level === 'warn' ? 'rgba(245,158,11,0.2)' : (theme: any) => alpha(theme.palette.primary.main, 0.1);
  const badgeColor = level === 'error' ? 'error.main' : level === 'warn' ? 'warning.main' : 'primary.main';

  return (
    <Box sx={{
      display: 'flex', gap: 1.5, px: 2.5, py: 1, fontSize: '0.75rem', lineHeight: 1.75,
      borderLeft: 3, borderLeftColor: borderColor, bgcolor: bgColor, color: levelColor,
      transition: 'background-color 0.15s', '&:hover': { bgcolor: 'action.hover' },
    }}>
      <Box component="span" sx={{ flexShrink: 0, color: 'text.secondary', width: 68, textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>{time}</Box>
      <Box component="span" sx={{
        flexShrink: 0, px: 0.75, borderRadius: 0.5, fontSize: '0.625rem', fontWeight: 900, textTransform: 'uppercase',
        lineHeight: '18px', height: 18, textAlign: 'center', minWidth: 38, bgcolor: badgeBg, color: badgeColor,
      }}>
        {level === 'error' ? t('logs.level_error') : level === 'warn' ? t('logs.level_warn') : t('logs.level_info')}
      </Box>
      <Box component="span" sx={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', '&:hover': { whiteSpace: 'normal', wordBreak: 'break-all' } }}>{msg}</Box>
    </Box>
  );
});

const Logs: React.FC = () => {
  const { t } = useTranslation();
  const [lines, setLines] = useState<string[]>([]);
  const [captureEnabled, setCaptureEnabled] = useState(false);
  const [isPaused, setIsPaused] = useState(false);
  const [isTogglingCapture, setIsTogglingCapture] = useState(false);
  const [search, setSearch] = useState('');
  const deferredSearch = useDeferredValue(search);
  const [atBottom, setAtBottom] = useState(true);
  const [newSinceAway, setNewSinceAway] = useState(0);
  const scrollRef = useRef<HTMLDivElement>(null);
  const prevRef = useRef<string>('');

  const handleScroll = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    const isAtBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 60;
    setAtBottom(isAtBottom);
  }, []);

  const scrollToBottom = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
    setAtBottom(true);
    setNewSinceAway(0);
  }, []);

  const fetchLogs = useCallback(async () => {
    const text = await GetRecentLogs(400);
    const key = text || '';
    if (key === prevRef.current) return;
    prevRef.current = key;
    setLines(key ? key.split('\n').filter(Boolean) : []);
    setNewSinceAway(prev => atBottom ? 0 : prev + 1);
  }, [atBottom]);

  useEffect(() => {
    let mounted = true;
    const init = async () => {
      const enabled = await IsLogCaptureEnabled();
      if (!mounted) return;
      setCaptureEnabled(enabled);
      if (enabled) await fetchLogs();
    };
    void init();
    return () => { mounted = false; };
  }, [fetchLogs]);

  useEffect(() => {
    if (!captureEnabled || isPaused) return;
    void fetchLogs();
    const interval = setInterval(() => void fetchLogs(), 1500);
    return () => clearInterval(interval);
  }, [captureEnabled, isPaused, fetchLogs]);

  useEffect(() => {
    if (!isPaused && atBottom && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [lines, isPaused, atBottom]);

  const handleToggleCapture = async () => {
    if (isTogglingCapture) return;
    setIsTogglingCapture(true);
    try {
      if (captureEnabled) { await StopLogCapture(); setCaptureEnabled(false); }
      else { await StartLogCapture(); setCaptureEnabled(true); setIsPaused(false); await fetchLogs(); }
    } finally { setIsTogglingCapture(false); }
  };

  const filteredLines = useMemo(
    () => lines.filter(l => l.toLowerCase().includes(deferredSearch.toLowerCase())),
    [lines, deferredSearch]
  );

  const handleClear = async () => { await ClearLogs(); setLines([]); prevRef.current = ''; };
  const handleScrollTop = () => { if (scrollRef.current) scrollRef.current.scrollTop = 0; };

  const handleExport = () => {
    const content = filteredLines.join('\n');
    const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `snishaper-logs-${new Date().toISOString().slice(0, 19).replace(/[T:]/g, '-')}.txt`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  const renderedContent = useMemo(() => {
    if (!captureEnabled || filteredLines.length === 0) return null;
    let lastDate = '';
    const elements: React.ReactNode[] = [];
    filteredLines.forEach((line, i) => {
      const { date } = parseLine(line);
      if (date && date !== lastDate) {
        lastDate = date;
        elements.push(
          <Box key={`d-${date}`} sx={{
            position: 'sticky', top: 0, zIndex: 10, px: 2.5, py: 0.75, bgcolor: 'background.paper',
            borderBottom: 1, borderColor: 'divider',
            fontSize: '0.6875rem', fontWeight: 900, textTransform: 'uppercase', letterSpacing: '0.1em', color: 'text.secondary',
          }}>
            {date}
          </Box>
        );
      }
      elements.push(<LogLine key={`l-${i}`} line={line} />);
    });
    return elements;
  }, [filteredLines, captureEnabled]);

  return (
    <Box sx={{ flexGrow: 1, minHeight: 0, display: 'flex', flexDirection: 'column', pt: 4, px: 3, pb: 0, overflow: 'hidden' }}>
      <Box sx={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'space-between', alignItems: { xs: 'flex-start', sm: 'flex-end' }, gap: 2, mb: 6, flexShrink: 0 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
          <Box sx={{ p: 1.25, borderRadius: 1.5, border: 1, color: 'primary.main', bgcolor: (theme) => alpha(theme.palette.primary.main, 0.1), borderColor: (theme) => alpha(theme.palette.primary.main, 0.1), display: 'flex' }}>
            <FileText size={20} />
          </Box>
          <Typography variant="h5" sx={{ fontWeight: 700, letterSpacing: '-0.02em' }}>{t('logs.title')}</Typography>
        </Box>
        <Box sx={{ display: 'flex', gap: 1 }}>
          <Button onClick={handleToggleCapture} loading={isTogglingCapture}
            color={captureEnabled ? 'error' : 'primary'} variant="contained" size="small" startIcon={<Radio size={14} />}>
            {captureEnabled ? t('logs.stop_capture') : t('logs.capture')}
          </Button>
          <Button onClick={() => setIsPaused(!isPaused)} disabled={!captureEnabled}
            variant={isPaused ? 'contained' : 'outlined'} size="small" startIcon={isPaused ? <Play size={14} /> : <Pause size={14} />}>
            {isPaused ? t('logs.resume') : t('logs.pause')}
          </Button>
          <Button onClick={handleScrollTop} variant="outlined" size="small" startIcon={<ChevronsUp size={14} />}>
            {t('logs.scroll_top')}
          </Button>
          <Button onClick={handleClear} variant="outlined" size="small"
            sx={{ '&:hover': { bgcolor: 'rgba(239,68,68,0.1)', color: 'error.main' } }} startIcon={<Trash2 size={14} />}>
            {t('logs.clear')}
          </Button>
          <Button onClick={handleExport} disabled={!captureEnabled || filteredLines.length === 0}
            variant="outlined" size="small" startIcon={<Download size={14} />}>
            {t('logs.export')}
          </Button>
        </Box>
      </Box>

      <Box sx={{ mb: 2.5, flexShrink: 0 }}>
        <TextField
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t('logs.search_placeholder')}
          aria-label={t('logs.search_placeholder')}
          fullWidth
          size="small"
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <Search size={16} aria-hidden />
                </InputAdornment>
              ),
            },
          }}
          sx={{ '& .MuiOutlinedInput-root': { borderRadius: 2, bgcolor: 'background.paper' } }}
        />
      </Box>

      <Box ref={scrollRef} onScroll={handleScroll} sx={{
        flex: 1, bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: '16px 16px 0 0',
        borderBottom: 0, overflow: 'hidden', display: 'flex', flexDirection: 'column', position: 'relative',
        boxShadow: 'inset 0 2px 4px 0 rgba(0,0,0,0.05)',
      }}>
        <Box sx={{ flex: 1, overflowY: 'auto', overflowX: 'hidden' }}>
          {!captureEnabled ? (
            <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', color: 'text.secondary', opacity: 0.6, px: 4, textAlign: 'center' }}>
              <Radio size={42} strokeWidth={1.5} aria-hidden />
              <Typography variant="caption" sx={{ mt: 2, fontWeight: 900, textTransform: 'uppercase', letterSpacing: '0.2em', fontSize: '0.75rem' }}>{t('logs.capture_disabled')}</Typography>
              <Typography variant="caption" sx={{ mt: 1.5, lineHeight: 1.6, maxWidth: 448, display: 'block' }}>{t('logs.capture_hint')}</Typography>
            </Box>
          ) : filteredLines.length === 0 ? (
            <Box sx={{ py: 12, display: 'flex', flexDirection: 'column', alignItems: 'center', color: 'text.secondary', opacity: 0.5 }}>
              <FileText size={48} strokeWidth={1} />
              <Typography variant="body2" sx={{ mt: 1.5 }}>{t('logs.no_logs')}</Typography>
            </Box>
          ) : (
            renderedContent
          )}
        </Box>
        {captureEnabled && !atBottom && (
          <Button
            onClick={scrollToBottom}
            variant="contained"
            color="primary"
            size="small"
            sx={{
              position: 'absolute', bottom: 12, left: '50%', transform: 'translateX(-50%)', zIndex: 20,
              borderRadius: '999px', fontSize: '0.6875rem', fontWeight: 900, textTransform: 'uppercase',
              letterSpacing: '0.05em', boxShadow: 8, '&:active': { transform: 'translateX(-50%) scale(0.95)' },
            }}
            startIcon={<ArrowDown size={14} />}
          >
            {newSinceAway > 0 && (
              <Box component="span" sx={{ bgcolor: 'rgba(255,255,255,0.2)', px: 0.75, borderRadius: 0.5, fontSize: '0.625rem' }}>+{newSinceAway}</Box>
            )}
            {t('logs.latest')}
          </Button>
        )}

        <Box sx={{ px: 3, py: 1.25, bgcolor: 'action.hover', borderTop: 1, borderColor: 'divider', display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexShrink: 0 }}>
          <Box sx={{ display: 'flex', gap: 2.5, fontSize: '0.625rem', fontWeight: 900, textTransform: 'uppercase', letterSpacing: '0.1em', color: 'text.secondary' }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
              <Box sx={{ width: 6, height: 6, borderRadius: '50%', bgcolor: captureEnabled ? 'success.main' : 'text.disabled', ...(captureEnabled ? { animation: `${pulse} 1.5s ease-in-out infinite` } : {}) }} aria-hidden />
              {captureEnabled ? t('logs.capture_on') : t('logs.capture_off')}
            </Box>
            <Box>{t('logs.buffer', { count: lines.length })}</Box>
          </Box>
          {captureEnabled && isPaused && (
            <Typography variant="caption" sx={{ fontWeight: 900, color: 'primary.main', bgcolor: (theme) => alpha(theme.palette.primary.main, 0.1), px: 1, py: 0.25, borderRadius: '999px', fontSize: '0.5625rem', animation: `${bounce} 1s ease-in-out infinite` }}>{t('logs.refresh_paused')}</Typography>
          )}
        </Box>
      </Box>
    </Box>
  );
};

export default Logs;
