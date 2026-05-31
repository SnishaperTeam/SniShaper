import React, { useState, useEffect, useRef, useMemo, useCallback } from 'react';
import { 
  FileText, 
  Trash2, 
  Pause, 
  Play, 
  Search,
  ChevronsUp,
  Radio
} from 'lucide-react';
import {
  ClearLogs,
  GetRecentLogs,
  IsLogCaptureEnabled,
  StartLogCapture,
  StopLogCapture
} from '../api/bindings';
import { useTranslation } from '../i18n/I18nContext';

const RE_ERROR = /error|failed|panic/i;
const RE_WARN = /warn/i;
const RE_LINE_PARSE = /^(\d{4}\/\d{2}\/\d{2}) (\d{2}:\d{2}:\d{2})(?:\.\d+)?\s+(.*)$/;

const parseLine = (text: string) => {
  const match = text.match(RE_LINE_PARSE);
  if (!match) return { time: '--:--:--', msg: text };
  return { time: match[2], msg: match[3] };
};

const LogLine: React.FC<{ line: string }> = React.memo(({ line }) => {
  const isError = RE_ERROR.test(line);
  const isWarn = RE_WARN.test(line);
  const { time, msg } = parseLine(line);

  return (
    <div className={`flex gap-3 px-4 py-1.5 border-b border-border/40 font-mono text-[11px] leading-relaxed group hover:bg-background-hover transition-colors ${
        isError ? 'bg-danger/5 text-danger' : isWarn ? 'bg-warning/5 text-warning' : 'text-text-primary'
    }`}>
      <span className="shrink-0 text-text-muted font-bold w-[70px]">{time}</span>
      <div className="flex gap-2 items-start flex-1 overflow-hidden">
        <span className={`shrink-0 px-1.5 rounded text-[9px] font-black uppercase mt-0.5 ${
            isError ? 'bg-danger text-white' : isWarn ? 'bg-warning text-white' : 'bg-accent/20 text-accent'
        }`}>
            {isError ? 'ERROR' : isWarn ? 'WARN' : 'INFO'}
        </span>
        <span className="truncate group-hover:whitespace-normal group-hover:break-all">{msg}</span>
      </div>
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
  const scrollRef = useRef<HTMLDivElement>(null);

  const fetchLogs = useCallback(async () => {
    const text = await GetRecentLogs(400);
    const newLines = text ? text.split('\n').filter(Boolean) : [];
    setLines(newLines);
  }, []);

  useEffect(() => {
    let mounted = true;

    const init = async () => {
      const enabled = await IsLogCaptureEnabled();
      if (!mounted) return;
      setCaptureEnabled(enabled);
      if (enabled) {
        await fetchLogs();
      }
    };

    void init();

    return () => {
      mounted = false;
    };
  }, []);

  useEffect(() => {
    if (!captureEnabled || isPaused) return;

    void fetchLogs();
    const interval = setInterval(() => {
      void fetchLogs();
    }, 1500);

    return () => clearInterval(interval);
  }, [captureEnabled, isPaused]);

  useEffect(() => {
    if (!isPaused && scrollRef.current) {
        scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [lines, isPaused]);

  const handleToggleCapture = async () => {
    if (isTogglingCapture) return;

    setIsTogglingCapture(true);
    try {
      if (captureEnabled) {
        await StopLogCapture();
        setCaptureEnabled(false);
      } else {
        await StartLogCapture();
        setCaptureEnabled(true);
        setIsPaused(false);
        await fetchLogs();
      }
    } finally {
      setIsTogglingCapture(false);
    }
  };

  const filteredLines = useMemo(
    () => lines.filter(l => l.toLowerCase().includes(search.toLowerCase())),
    [lines, search]
  );

  const handleClear = async () => {
    await ClearLogs();
    setLines([]);
  };

  const handleScrollTop = () => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = 0;
    }
  };

  return (
    <div className="h-full flex flex-col p-6 animate-in fade-in duration-500 overflow-hidden">
      <header className="flex justify-between items-end mb-6 shrink-0">
        <div>
           <h1 className="text-3xl font-black tracking-tighter">{t('logs.title')}</h1>
        </div>
        <div className="flex gap-2">
            <button 
                onClick={handleToggleCapture}
                disabled={isTogglingCapture}
                className={`flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-bold transition-all ${
                    captureEnabled ? "bg-danger text-white" : "bg-accent text-white"
                }`}
            >
                <Radio size={14} />
                {captureEnabled ? t('logs.stop_capture') : t('logs.capture')}
            </button>
            <button 
                onClick={() => setIsPaused(!isPaused)}
                disabled={!captureEnabled}
                className={`flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-bold transition-all ${
                    isPaused ? "bg-accent text-white" : "bg-background-hover text-text-secondary hover:text-accent"
                } ${!captureEnabled ? "opacity-50 cursor-not-allowed" : ""}`}
            >
                {isPaused ? <Play size={14} /> : <Pause size={14} />}
                {isPaused ? t('logs.resume') : t('logs.pause')}
            </button>
            <button
                onClick={handleScrollTop}
                className="flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-bold bg-background-hover text-text-secondary hover:text-accent transition-all"
            >
                <ChevronsUp size={14} />
                {t('logs.scroll_top')}
            </button>
            <button 
                onClick={handleClear}
                className="flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-bold bg-background-hover hover:bg-danger/10 hover:text-danger transition-all"
            >
                <Trash2 size={14} />
                {t('logs.clear')}
            </button>
        </div>
      </header>

      <div className="mb-4 relative group shrink-0">
          <Search className="absolute left-4 top-1/2 -translate-y-1/2 text-text-muted group-focus-within:text-accent transition-colors" size={16} />
          <input 
            type="text" 
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t('logs.search_placeholder')}
            className="w-full bg-background-card border border-border focus:border-accent/30 pl-11 pr-4 py-2.5 rounded-2xl text-xs focus:ring-0 outline-none transition-all font-medium"
          />
      </div>

      <div 
        ref={scrollRef}
        className="flex-1 bg-background-card border border-border rounded-2xl overflow-hidden shadow-inner flex flex-col"
      >
        <div className="flex-1 overflow-y-auto overflow-x-hidden">
            {!captureEnabled ? (
                <div className="h-full flex flex-col items-center justify-center text-text-muted opacity-60 px-8 text-center">
                    <Radio size={42} strokeWidth={1.5} />
                    <span className="text-xs mt-4 font-black uppercase tracking-[0.2em]">Capture Disabled</span>
                    <p className="mt-3 text-xs leading-relaxed max-w-md">
                      {t('logs.capture_hint')}
                    </p>
                </div>
            ) : filteredLines.length === 0 ? (
                <div className="h-full flex flex-col items-center justify-center text-text-muted opacity-40">
                    <FileText size={48} strokeWidth={1} />
                    <span className="text-xs mt-4 font-black uppercase tracking-[0.2em]">{t('logs.no_logs')}</span>
                </div>
            ) : (
                filteredLines.map((line, i) => <LogLine key={i} line={line} />)
            )}
        </div>
        
        <div className="px-6 py-2 bg-background-hover/50 border-t border-border flex justify-between items-center shrink-0">
             <div className="flex gap-4 text-[9px] font-black uppercase tracking-widest text-text-muted">
                <div className="flex items-center gap-1">
                  <div className={`w-1.5 h-1.5 rounded-full ${captureEnabled ? "bg-success animate-pulse" : "bg-text-muted/40"}`} />
                  {captureEnabled ? "CAPTURE ON" : "CAPTURE OFF"}
                </div>
                <div>BUFFER: {lines.length} LINES</div>
             </div>
             {captureEnabled && isPaused && (
                <div className="text-[9px] font-black text-accent bg-accent/10 px-2 py-0.5 rounded-full animate-bounce">
                    REFRESH PAUSED
                </div>
             )}
        </div>
      </div>
    </div>
  );
};

export default Logs;
