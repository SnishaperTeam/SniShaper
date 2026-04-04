import React, { useEffect, useState } from 'react';
import { CheckCircle2, AlertCircle, Info, X } from 'lucide-react';
import { TOAST_EVENT, type ToastPayload } from '../lib/toast';

const ICONS = {
  success: CheckCircle2,
  error: AlertCircle,
  info: Info
} as const;

const STYLES = {
  success: 'border-success/30 bg-success/10 text-success',
  error: 'border-danger/30 bg-danger/10 text-danger',
  info: 'border-accent/30 bg-accent/10 text-accent'
} as const;

const ToastProvider: React.FC = () => {
  const [toasts, setToasts] = useState<ToastPayload[]>([]);

  useEffect(() => {
    const handleToast = (event: Event) => {
      const customEvent = event as CustomEvent<ToastPayload>;
      const payload = customEvent.detail;
      setToasts((prev) => [...prev, payload]);

      window.setTimeout(() => {
        setToasts((prev) => prev.filter((toast) => toast.id !== payload.id));
      }, payload.duration ?? 2600);
    };

    window.addEventListener(TOAST_EVENT, handleToast);
    return () => window.removeEventListener(TOAST_EVENT, handleToast);
  }, []);

  if (toasts.length === 0) return null;

  return (
    <div className="pointer-events-none fixed right-4 top-4 z-[80] flex w-[min(360px,calc(100vw-2rem))] flex-col gap-3">
      {toasts.map((toast) => {
        const Icon = ICONS[toast.type];
        return (
          <div
            key={toast.id}
            className="pointer-events-auto overflow-hidden rounded-2xl border border-border bg-background-card/95 shadow-2xl backdrop-blur-md animate-in fade-in slide-in-from-top-2 duration-300"
          >
            <div className="flex items-start gap-3 p-4">
              <div className={`mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border ${STYLES[toast.type]}`}>
                <Icon size={18} />
              </div>
              <div className="min-w-0 flex-1">
                <div className="text-sm font-bold text-text-primary">{toast.title}</div>
                {toast.message && <div className="mt-1 text-xs leading-relaxed text-text-secondary">{toast.message}</div>}
              </div>
              <button
                type="button"
                onClick={() => setToasts((prev) => prev.filter((item) => item.id !== toast.id))}
                className="rounded-lg p-1.5 text-text-muted transition-colors hover:bg-background-hover hover:text-text-primary"
                aria-label="关闭提示"
              >
                <X size={14} />
              </button>
            </div>
          </div>
        );
      })}
    </div>
  );
};

export default ToastProvider;
