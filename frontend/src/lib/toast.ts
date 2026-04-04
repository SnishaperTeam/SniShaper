export type ToastType = 'success' | 'error' | 'info';

export interface ToastPayload {
  id: string;
  type: ToastType;
  title: string;
  message?: string;
  duration?: number;
}

const TOAST_EVENT = 'app:toast';

const emitToast = (payload: Omit<ToastPayload, 'id'>) => {
  window.dispatchEvent(
    new CustomEvent<ToastPayload>(TOAST_EVENT, {
      detail: {
        id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
        duration: 2600,
        ...payload
      }
    })
  );
};

export const toast = {
  success: (title: string, message?: string, duration?: number) =>
    emitToast({ type: 'success', title, message, duration }),
  error: (title: string, message?: string, duration?: number) =>
    emitToast({ type: 'error', title, message, duration }),
  info: (title: string, message?: string, duration?: number) =>
    emitToast({ type: 'info', title, message, duration })
};

export { TOAST_EVENT };
