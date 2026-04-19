import React, { Suspense, lazy, useState, useEffect } from 'react';
import { HashRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import Sidebar from './components/Sidebar';
import WindowControls from './components/WindowControls';
import ToastProvider from './components/ToastProvider';
import Settings from './pages/Settings';

const Dashboard = lazy(() => import('./pages/Dashboard'));
const Proxies = lazy(() => import('./pages/Proxies'));
const Rules = lazy(() => import('./pages/Rules'));
const Routing = lazy(() => import('./pages/Routing'));
const Logs = lazy(() => import('./pages/Logs'));

const App: React.FC = () => {
  const [theme, setTheme] = useState<'light' | 'dark'>('light');

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    if (theme === 'dark') {
      document.documentElement.classList.add('dark');
    } else {
      document.documentElement.classList.remove('dark');
    }
  }, [theme]);

  useEffect(() => {
    const shouldAllowNativeMenu = (target: EventTarget | null) => {
      if (!(target instanceof HTMLElement)) return false;
      return Boolean(target.closest('input, textarea, [contenteditable="true"], [data-native-contextmenu="true"]'));
    };

    const handleContextMenu = (event: MouseEvent) => {
      if (shouldAllowNativeMenu(event.target)) return;
      event.preventDefault();
    };

    window.addEventListener('contextmenu', handleContextMenu);
    return () => window.removeEventListener('contextmenu', handleContextMenu);
  }, []);

  const toggleTheme = () => setTheme(prev => prev === 'light' ? 'dark' : 'light');
  const routeFallback = (
    <div className="h-full min-h-[240px] flex items-center justify-center text-sm text-text-secondary">
      正在加载页面...
    </div>
  );

  return (
    <Router>
      <div className="flex h-screen w-screen overflow-hidden bg-background select-none relative">
        <ToastProvider />
        <Sidebar theme={theme} toggleTheme={toggleTheme} />
        
        <main className="flex-1 min-w-0 bg-background-soft/30 backdrop-blur-sm relative flex flex-col">
          <header className="h-10 shrink-0 border-b border-border/60 bg-background/70 backdrop-blur-md flex items-center justify-between pl-4 pr-3">
            <div
              className="flex-1 h-full"
              style={{ "--wails-draggable": "drag" } as React.CSSProperties}
            />
            <WindowControls />
          </header>

          <div className="flex-1 overflow-y-auto overflow-x-hidden">
            <Suspense fallback={routeFallback}>
              <Routes>
                <Route path="/" element={<Navigate to="/dashboard" replace />} />
                <Route path="/dashboard" element={<Dashboard />} />
                <Route path="/proxies" element={<Proxies />} />
                <Route path="/rules" element={<Rules />} />
                <Route path="/routing" element={<Routing />} />
                <Route path="/logs" element={<Logs />} />
                <Route path="/settings" element={<Settings />} />
              </Routes>
            </Suspense>
          </div>
        </main>
      </div>
    </Router>
  );
};

export default App;
