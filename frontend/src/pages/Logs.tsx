import React, { useState, useEffect, useRef } from 'react';
import { 
  FileText, 
  Trash2, 
  Download, 
  Pause, 
  Play, 
  Search,
  AlertCircle,
  Info,
  AlertTriangle
} from 'lucide-react';
import { GetRecentLogs, ClearLogs } from '../api/bindings';

const LogLine: React.FC<{ line: string }> = ({ line }) => {
  const isError = /error|failed|panic/i.test(line);
  const isWarn = /warn/i.test(line);
  
  const parseLine = (text: string) => {
    const match = text.match(/^(\d{4}\/\d{2}\/\d{2}) (\d{2}:\d{2}:\d{2})(?:\.\d+)?\s+(.*)$/);
    if (!match) return { time: '--:--:--', msg: text };
    return { time: match[2], msg: match[3] };
  };

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
};

const Logs: React.FC = () => {
  const [lines, setLines] = useState<string[]>([]);
  const [isPaused, setIsPaused] = useState(false);
  const [search, setSearch] = useState('');
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (isPaused) return;

    const fetchLogs = async () => {
      const text = await GetRecentLogs(400);
      if (text) {
        const newLines = text.split('\n').filter(Boolean);
        setLines(newLines);
      }
    };

    fetchLogs(); // Initial fetch
    const interval = setInterval(fetchLogs, 1500); // More frequent updates

    return () => clearInterval(interval);
  }, [isPaused]);

  useEffect(() => {
    if (!isPaused && scrollRef.current) {
        scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [lines, isPaused]);

  const filteredLines = lines.filter(l => l.toLowerCase().includes(search.toLowerCase()));

  const handleClear = async () => {
    await ClearLogs();
    setLines([]);
  };

  return (
    <div className="h-full flex flex-col p-6 animate-in fade-in duration-500 overflow-hidden">
      <header className="flex justify-between items-end mb-6 shrink-0">
        <div>
           <h1 className="text-3xl font-black tracking-tighter">系统日志</h1>
           <p className="text-text-muted mt-1 text-sm font-medium">诊断接口与后端运行轨迹</p>
        </div>
        <div className="flex gap-2">
            <button 
                onClick={() => setIsPaused(!isPaused)}
                className={`flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-bold transition-all ${
                    isPaused ? "bg-accent text-white" : "bg-background-hover text-text-secondary hover:text-accent"
                }`}
            >
                {isPaused ? <Play size={14} /> : <Pause size={14} />}
                {isPaused ? "继续渲染" : "暂停渲染"}
            </button>
            <button 
                onClick={handleClear}
                className="flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-bold bg-background-hover hover:bg-danger/10 hover:text-danger transition-all"
            >
                <Trash2 size={14} />
                清空日志
            </button>
        </div>
      </header>

      <div className="mb-4 relative group shrink-0">
          <Search className="absolute left-4 top-1/2 -translate-y-1/2 text-text-muted group-focus-within:text-accent transition-colors" size={16} />
          <input 
            type="text" 
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="搜索关键词、域、错误代码..."
            className="w-full bg-background-card border border-border focus:border-accent/30 pl-11 pr-4 py-2.5 rounded-2xl text-xs focus:ring-0 outline-none transition-all font-medium"
          />
      </div>

      <div 
        ref={scrollRef}
        className="flex-1 bg-background-card border border-border rounded-2xl overflow-hidden shadow-inner flex flex-col"
      >
        <div className="flex-1 overflow-y-auto overflow-x-hidden">
            {filteredLines.length === 0 ? (
                <div className="h-full flex flex-col items-center justify-center text-text-muted opacity-40">
                    <FileText size={48} strokeWidth={1} />
                    <span className="text-xs mt-4 font-black uppercase tracking-[0.2em]">No logs collected</span>
                </div>
            ) : (
                filteredLines.map((line, i) => <LogLine key={i} line={line} />)
            )}
        </div>
        
        <div className="px-6 py-2 bg-background-hover/50 border-t border-border flex justify-between items-center shrink-0">
             <div className="flex gap-4 text-[9px] font-black uppercase tracking-widest text-text-muted">
                <div className="flex items-center gap-1"><div className="w-1.5 h-1.5 bg-accent rounded-full animate-pulse" /> SYSTEM READY</div>
                <div>BUFFER: {lines.length} LINES</div>
             </div>
             {isPaused && (
                <div className="text-[9px] font-black text-accent bg-accent/10 px-2 py-0.5 rounded-full animate-bounce">
                    BUFFERING PAUSED
                </div>
             )}
        </div>
      </div>
    </div>
  );
};

export default Logs;
