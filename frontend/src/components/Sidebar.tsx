import React, { useState, useEffect } from 'react';
import { NavLink } from 'react-router-dom';
import logoUrl from '../assets/logo.svg';
import { 
  LayoutDashboard, 
  ShieldCheck, 
  Activity, 
  FileText, 
  Settings, 
  Sun, 
  Moon,
  Workflow,
  Globe,
  ArrowDown,
  ArrowUp
} from 'lucide-react';
import { AreaChart, Area, ResponsiveContainer } from 'recharts';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { EventsOn } from '../api/bindings';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

const navItems = [
  { path: '/dashboard', label: '首页', icon: LayoutDashboard },
  { path: '/proxies', label: '代理', icon: Globe },
  { path: '/rules', label: '规则', icon: ShieldCheck },
  { path: '/routing', label: '分流', icon: Workflow },
  { path: '/logs', label: '日志', icon: FileText },
  { path: '/settings', label: '设置', icon: Settings },
];

const formatSpeed = (bytes: number) => {
  if (bytes < 1024) return `${Math.round(bytes)} B/s`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB/s`;
  return `${Math.round(bytes / (1024 * 1024))} MB/s`;
};

interface SidebarProps {
  theme: 'light' | 'dark';
  toggleTheme: () => void;
}

const Sidebar: React.FC<SidebarProps> = ({ theme, toggleTheme }) => {
  const [speedHistory, setSpeedHistory] = useState(Array.from({ length: 20 }, () => ({ down: 0, up: 0 })));
  const [currentSpeed, setCurrentSpeed] = useState({ down: 0, up: 0 });
  const chartGradientIdDown = React.useId();
  const chartGradientIdUp = React.useId();

  useEffect(() => {
    const unoff = EventsOn("app:traffic", (data: any) => {
        if (data) {
            const down = data.down || 0;
            const up = data.up || 0;
            setCurrentSpeed({ down, up });
            setSpeedHistory(prev => {
                const updated = [...prev.slice(1), { down, up }];
                return updated;
            });
        }
    });
    return () => unoff();
  }, []);

  return (
    <aside 
      className="w-[180px] h-full flex flex-col bg-background-card border-r border-border py-6 px-2 shadow-xl z-20 select-none overflow-hidden"
    >
      <div className="flex flex-col gap-6 mb-8 items-center">
        <div className="flex items-center">
          <div className="flex flex-col items-center gap-3">
            <img
              src={logoUrl}
              alt="SniShaper logo"
              className="w-14 h-14 object-contain drop-shadow-[0_10px_20px_rgba(33,150,243,0.22)]"
            />
            <span className="font-extrabold text-[11px] tracking-[0.2em] uppercase text-text-secondary">SniShaper</span>
          </div>
        </div>
      </div>

      <nav className="flex-1 space-y-1.5 px-1">
        {navItems.map((item) => (
          <NavLink
            key={item.path}
            to={item.path}
            className={({ isActive }) => cn(
              "flex items-center gap-4 pl-8 py-3 rounded-xl text-[13px] font-bold transition-all group",
              isActive 
                ? "bg-accent text-white shadow-lg shadow-accent/25" 
                : "text-text-secondary hover:bg-background-hover hover:text-text-primary"
            )}
          >
            <item.icon size={18} className={cn("transition-transform group-hover:scale-110 shrink-0")} />
            <span className="tracking-widest">{item.label}</span>
          </NavLink>
        ))}
      </nav>

      <div className="mt-auto space-y-4">
        {/* Real-time traffic wave chart (Clash Style) */}
        <div className="h-[76px] w-full px-2 pointer-events-none">
            <div className="h-full w-full bg-background-soft/40 rounded-xl overflow-hidden relative border border-border/50">
                <ResponsiveContainer width="100%" height="100%" className="-mt-1">
                    <AreaChart data={speedHistory} margin={{ top: 20, right: 0, left: 0, bottom: 0 }}>
                        <defs>
                            <linearGradient id={chartGradientIdDown} x1="0" y1="0" x2="0" y2="1">
                                <stop offset="5%" stopColor="#10b981" stopOpacity={0.3}/>
                                <stop offset="95%" stopColor="#10b981" stopOpacity={0}/>
                            </linearGradient>
                            <linearGradient id={chartGradientIdUp} x1="0" y1="0" x2="0" y2="1">
                                <stop offset="5%" stopColor="#f59e0b" stopOpacity={0.3}/>
                                <stop offset="95%" stopColor="#f59e0b" stopOpacity={0}/>
                            </linearGradient>
                        </defs>
                        <Area 
                            type="monotone" 
                            dataKey="up" 
                            stroke="#f59e0b" 
                            fillOpacity={1} 
                            fill={`url(#${chartGradientIdUp})`}
                            strokeWidth={1.5}
                            isAnimationActive={false}
                        />
                        <Area 
                            type="monotone" 
                            dataKey="down" 
                            stroke="#10b981" 
                            fillOpacity={1} 
                            fill={`url(#${chartGradientIdDown})`}
                            strokeWidth={1.5}
                            isAnimationActive={false}
                        />
                    </AreaChart>
                </ResponsiveContainer>
                
                <div className="absolute top-1.5 left-2 flex items-center gap-1 z-10">
                    <ArrowDown size={10} className="text-success" />
                    <span className="text-[10px] font-black text-text-primary drop-shadow-sm">{formatSpeed(currentSpeed.down)}</span>
                </div>
                <div className="absolute top-1.5 right-2 flex items-center gap-1 z-10">
                    <span className="text-[10px] font-black text-text-primary drop-shadow-sm">{formatSpeed(currentSpeed.up)}</span>
                    <ArrowUp size={10} className="text-warning" />
                </div>
            </div>
        </div>

        <button 
          type="button"
          onClick={toggleTheme}
          className="w-full flex items-center justify-center py-2.5 rounded-xl bg-background-hover border border-border text-text-secondary hover:text-accent transition-all outline-none focus:outline-none focus-visible:outline-none focus-visible:border-accent/40 focus-visible:ring-2 focus-visible:ring-accent/30"
        >
          {theme === 'light' ? <Moon size={18} /> : <Sun size={18} />}
        </button>
      </div>
    </aside>
  );
};

export default Sidebar;
