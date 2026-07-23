import React, { useState, useEffect, useRef, useMemo, useCallback, useDeferredValue } from 'react';
import {
  FileText, Trash2, Pause, Play, Search, ChevronsUp, Radio, ArrowDown, Download
} from '../lib/icons';
import {
  ClearLogs, GetRecentLogs, IsLogCaptureEnabled,
  StartLogCapture, StopLogCapture
} from '../api/bindings';
import { Button } from '../components/ui/Button';
import { EmptyState } from '../components/ui/EmptyState';
import { cn } from '../lib/utils';
import { useTranslation } from '../i18n/I18nContext';

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

const LogLine: React.FC<{ line: string }> = React.memo(({ line }) => {
  const { time, msg, level } = parseLine(line);

  return (
    <div className={cn(
      'flex gap-2.5 px-4 py-1.5 font-mono text-[11px] leading-relaxed group hover:bg-background-hover transition-colors border-l-2',
      level === 'error' && 'border-l-red-500/70 bg-red-500/[0.03] text-red-500',
      level === 'warn' && 'border-l-amber-500/70 bg-amber-500/[0.03] text-amber-500',
      level === 'info' && 'border-l-transparent hover:border-l-border text-text-primary'
    )}>
      <span className="shrink-0 text-text-muted w-[60px] text-right tabular-nums flex-shrink-0">{time}</span>
      <span className={cn(
        'shrink-0 px-1.5 rounded text-[9px] font-black uppercase leading-[16px] h-[16px] text-center min-w-[34px] flex-shrink-0',
        level === 'error' && 'bg-red-500/20 text-red-500',
        level === 'warn' && 'bg-amber-500/20 text-amber-500',
        level === 'info' && 'bg-accent/10 text-accent'
      )}>
        {level === 'error' ? 'ERR' : level === 'warn' ? 'WRN' : 'INF'}
      </span>
      <span className="truncate group-hover:whitespace-normal group-hover:break-all flex-1 min-w-0">{msg}</span>
    </div>
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
    if (isAtBottom) {
      setNewSinceAway(0);
    } else if (newSinceAway > 0) {
      // counter incremented elsewhere
    }
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
          <div key={`d-${date}`} className="sticky top-0 z-10 px-4 py-1 bg-background-card/90 backdrop-blur border-b border-border/60 text-[10px] font-black uppercase tracking-widest text-text-muted">
            {date}
          </div>
        );
      }
      elements.push(<LogLine key={`l-${i}`} line={line} />);
    });
    return elements;
  }, [filteredLines, captureEnabled]);

  return (
    <div className="h-full flex flex-col p-6 animate-in fade-in duration-500 overflow-hidden">
      <header className="flex justify-between items-end mb-6 shrink-0">
        <h1 className="text-3xl font-black tracking-tighter">{t('logs.title')}</h1>
        <div className="flex gap-2">
          <Button onClick={handleToggleCapture} loading={isTogglingCapture}
            variant={captureEnabled ? 'danger' : 'primary'} size="sm" icon={<Radio size={14} />}>
            {captureEnabled ? t('logs.stop_capture') : t('logs.capture')}
          </Button>
          <Button onClick={() => setIsPaused(!isPaused)} disabled={!captureEnabled}
            variant={isPaused ? 'primary' : 'ghost'} size="sm" icon={isPaused ? <Play size={14} /> : <Pause size={14} />}>
            {isPaused ? t('logs.resume') : t('logs.pause')}
          </Button>
          <Button onClick={handleScrollTop} variant="ghost" size="sm" icon={<ChevronsUp size={14} />}>
            {t('logs.scroll_top')}
          </Button>
          <Button onClick={handleClear} variant="ghost" size="sm"
            className="hover:bg-danger/10 hover:text-danger" icon={<Trash2 size={14} />}>
            {t('logs.clear')}
          </Button>
          <Button onClick={handleExport} disabled={!captureEnabled || filteredLines.length === 0}
            variant="ghost" size="sm" icon={<Download size={14} />}>
            {t('logs.export')}
          </Button>
        </div>
      </header>

      <div className="mb-4 relative group shrink-0">
        <Search className="absolute left-4 top-1/2 -translate-y-1/2 text-text-muted group-focus-within:text-accent transition-colors" size={16} aria-hidden />
        <input type="text" value={search} onChange={(e) => setSearch(e.target.value)} placeholder={t('logs.search_placeholder')}
          aria-label={t('logs.search_placeholder')}
          className="w-full bg-background-card border border-border focus:border-accent/30 pl-11 pr-4 py-2.5 rounded-2xl text-xs outline-none transition-all font-medium" />
      </div>

      <div ref={scrollRef} onScroll={handleScroll} className="flex-1 bg-background-card border border-border rounded-2xl overflow-hidden shadow-inner flex flex-col relative">
        <div className="flex-1 overflow-y-auto overflow-x-hidden">
          {!captureEnabled ? (
            <div className="h-full flex flex-col items-center justify-center text-text-muted opacity-60 px-8 text-center">
              <Radio size={42} strokeWidth={1.5} aria-hidden />
              <span className="text-xs mt-4 font-black uppercase tracking-[0.2em]">Capture Disabled</span>
              <p className="mt-3 text-xs leading-relaxed max-w-md">{t('logs.capture_hint')}</p>
            </div>
          ) : filteredLines.length === 0 ? (
            <EmptyState icon={<FileText size={48} strokeWidth={1} />} title={t('logs.no_logs')} />
          ) : (
            renderedContent
          )}
        </div>
        {captureEnabled && !atBottom && (
          <button onClick={scrollToBottom}
            className="absolute bottom-3 left-1/2 -translate-x-1/2 z-20 flex items-center gap-2 px-4 py-2 rounded-full bg-accent text-white text-[11px] font-black uppercase tracking-wider shadow-lg hover:bg-accent/90 active:scale-95 transition-all animate-in slide-in-from-bottom-2 fade-in duration-200">
            <ArrowDown size={14} />
            {newSinceAway > 0 && <span className="bg-white/20 px-1.5 rounded text-[10px]">+{newSinceAway}</span>}
            最新
          </button>
        )}

        <div className="px-6 py-2 bg-background-hover/50 border-t border-border flex justify-between items-center shrink-0">
          <div className="flex gap-4 text-[9px] font-black uppercase tracking-widest text-text-muted">
            <div className="flex items-center gap-1">
              <div className={cn('w-1.5 h-1.5 rounded-full', captureEnabled ? 'bg-success animate-pulse' : 'bg-text-muted/40')} aria-hidden />
              {captureEnabled ? "CAPTURE ON" : "CAPTURE OFF"}
            </div>
            <div>BUFFER: {lines.length} LINES</div>
          </div>
          {captureEnabled && isPaused && (
            <div className="text-[9px] font-black text-accent bg-accent/10 px-2 py-0.5 rounded-full animate-bounce">REFRESH PAUSED</div>
          )}
        </div>
      </div>
    </div>
  );
};

export default Logs;
