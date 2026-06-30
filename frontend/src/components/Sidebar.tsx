import React, { useState, useEffect, useMemo, useCallback } from 'react';
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
  ArrowUp,
  Antenna,
  Info,
  Zap,
  Menu,
  X
} from '../lib/icons';
import { AreaChart, Area, ResponsiveContainer } from 'recharts';
import { cn, formatSpeed } from '../lib/utils';
import { EventsOn } from '../api/bindings';
import { useTranslation } from '../i18n/I18nContext';

const getNavItems = (t: any) => [
  { path: '/dashboard', label: t('sidebar.dashboard'), icon: LayoutDashboard },
  { path: '/proxies', label: t('sidebar.proxies'), icon: Globe },
  { path: '/rules', label: t('sidebar.rules'), icon: ShieldCheck },
  { path: '/routing', label: t('sidebar.routing'), icon: Workflow },
  { path: '/dns', label: t('sidebar.dns'), icon: Antenna },
  { path: '/evolution', label: '进化模式', icon: Zap },
  { path: '/logs', label: t('sidebar.logs'), icon: FileText },
  { path: '/settings', label: t('sidebar.settings'), icon: Settings },
  { path: '/about', label: t('sidebar.about'), icon: Info },
];

interface SidebarProps {
  theme: 'light' | 'dark';
  toggleTheme: () => void;
}

const Sidebar: React.FC<SidebarProps> = React.memo(({ theme, toggleTheme }) => {
  const { t } = useTranslation();
  const navItems = useMemo(() => getNavItems(t), [t]);
  const [sidebarOpen, setSidebarOpen] = useState(false);
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
            setSpeedHistory(prev => [...prev.slice(1), { down, up }]);
        }
    });
    return () => unoff();
  }, []);

  useEffect(() => {
    if (!sidebarOpen) return;
    const handleEsc = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setSidebarOpen(false);
    };
    window.addEventListener('keydown', handleEsc);
    return () => window.removeEventListener('keydown', handleEsc);
  }, [sidebarOpen]);

  const closeSidebar = useCallback(() => setSidebarOpen(false), []);

  const sidebarContent = (
    <>
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

      <nav className="flex-1 space-y-1.5" aria-label="主导航">
        {navItems.map((item) => (
          <NavLink
            key={item.path}
            to={item.path}
            onClick={closeSidebar}
            className={({ isActive }) => cn(
              "flex flex-col items-center gap-1 px-4 py-3 rounded-xl text-[13px] font-bold transition-all group",
              isActive
                ? "bg-accent text-white shadow-lg shadow-accent/25"
                : "text-text-secondary hover:bg-background-hover hover:text-text-primary"
            )}
          >
            <item.icon size={18} className={cn("transition-transform group-hover:scale-110 shrink-0")} aria-hidden />
            <span className="tracking-widest text-center">{item.label}</span>
          </NavLink>
        ))}
      </nav>

      <div className="mt-auto space-y-4">
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
                  <Area type="monotone" dataKey="up" stroke="#f59e0b" fillOpacity={1} fill={`url(#${chartGradientIdUp})`} strokeWidth={1.5} isAnimationActive={false} />
                  <Area type="monotone" dataKey="down" stroke="#10b981" fillOpacity={1} fill={`url(#${chartGradientIdDown})`} strokeWidth={1.5} isAnimationActive={false} />
                </AreaChart>
              </ResponsiveContainer>
              <div className="absolute top-1.5 left-2 flex items-center gap-1 z-10">
                <ArrowDown size={10} className="text-success" aria-hidden />
                <span className="text-[10px] font-black text-text-primary drop-shadow-sm">{formatSpeed(currentSpeed.down)}</span>
              </div>
              <div className="absolute top-1.5 right-2 flex items-center gap-1 z-10">
                <span className="text-[10px] font-black text-text-primary drop-shadow-sm">{formatSpeed(currentSpeed.up)}</span>
                <ArrowUp size={10} className="text-warning" aria-hidden />
              </div>
            </div>
        </div>

        <button
          type="button"
          onClick={toggleTheme}
          aria-label={theme === 'light' ? '切换到暗色模式' : '切换到亮色模式'}
          className="w-full flex items-center justify-center py-2.5 rounded-xl bg-background-hover border border-border text-text-secondary hover:text-accent transition-all"
        >
          {theme === 'light' ? <Moon size={18} aria-hidden /> : <Sun size={18} aria-hidden />}
        </button>
      </div>
    </>
  );

  return (
    <>
      {/* Mobile hamburger button */}
      <button
        type="button"
        onClick={() => setSidebarOpen(true)}
        aria-label="打开菜单"
        className="lg:hidden fixed top-3 left-3 z-30 p-2 rounded-xl bg-background-card border border-border shadow-md"
      >
        <Menu size={20} />
      </button>

      {/* Mobile overlay */}
      {sidebarOpen && (
        <div
          className="lg:hidden fixed inset-0 z-30 bg-background/60 backdrop-blur-sm animate-in fade-in duration-200"
          onClick={closeSidebar}
          aria-hidden
        />
      )}

      {/* Sidebar: desktop always visible, mobile as drawer */}
      <aside
        className={cn(
          "h-full flex flex-col bg-background-card border-r border-border py-6 px-3 w-48 shadow-xl z-40 select-none overflow-hidden",
          "lg:relative lg:translate-x-0",
          "fixed inset-y-0 left-0 transition-transform duration-300",
          sidebarOpen ? "translate-x-0" : "-translate-x-full lg:translate-x-0"
        )}
        aria-label="侧边栏导航"
      >
        {/* Mobile close button */}
        <button
          type="button"
          onClick={closeSidebar}
          aria-label="关闭菜单"
          className="lg:hidden absolute top-3 right-3 p-1.5 rounded-lg hover:bg-background-hover transition-colors"
        >
          <X size={18} />
        </button>
        {sidebarContent}
      </aside>
    </>
  );
});

export default Sidebar;
