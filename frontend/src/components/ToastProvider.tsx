import React, { useEffect, useState } from 'react';
import { Box, Stack, IconButton, Typography, Paper } from '@mui/material';
import { keyframes } from '@emotion/react';
import { CheckCircle, Error, Info, X } from '../lib/icons';
import { TOAST_EVENT, type ToastPayload } from '../lib/toast';
import { useTranslation } from '../i18n/I18nContext';

const ICONS = {
  success: CheckCircle,
  error: Error,
  info: Info
} as const;

const slideIn = keyframes`
  from { opacity: 0; transform: translateX(24px) scale(0.96); }
  to { opacity: 1; transform: translateX(0) scale(1); }
`;

const ToastProvider: React.FC = () => {
  const [toasts, setToasts] = useState<ToastPayload[]>([]);
  const { t } = useTranslation();

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
    <Box
      sx={{
        position: 'fixed',
        right: 12,
        top: 60,
        zIndex: 9999,
        width: 'min(360px, calc(100vw - 2rem))',
        pointerEvents: 'none',
      }}
    >
      <Stack direction="column" spacing={1.5}>
        {toasts.map((toast) => {
          const Icon = ICONS[toast.type];
          const color = toast.type;
          return (
            <Paper
              key={toast.id}
              elevation={12}
              sx={{
                pointerEvents: 'auto',
                overflow: 'hidden',
                borderRadius: 2,
                border: 1,
                borderColor: 'divider',
                p: 1.5,
                display: 'flex',
                alignItems: 'flex-start',
                gap: 1.5,
                bgcolor: (theme) =>
                  `color-mix(in srgb, ${theme.palette.background.paper} 82%, transparent)`,
                backdropFilter: 'blur(16px) saturate(180%)',
                WebkitBackdropFilter: 'blur(16px) saturate(180%)',
                boxShadow: (theme) => `0 8px 32px ${theme.palette.mode === 'dark' ? 'rgba(0,0,0,0.4)' : 'rgba(0,0,0,0.08)'}`,
                animation: `${slideIn} 0.25s cubic-bezier(0.16, 1, 0.3, 1)`,
              }}
            >
              <Box
                sx={{
                  mt: 0.25,
                  width: 36,
                  height: 36,
                  flexShrink: 0,
                  borderRadius: 1.5,
                  border: 1,
                  borderColor: `${color}.light`,
                  bgcolor: (theme) => `color-mix(in srgb, ${theme.palette[color].main} 14%, transparent)`,
                  color: `${color}.main`,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                }}
              >
                <Icon size={20} color="inherit" />
              </Box>
              <Box sx={{ minWidth: 0, flexGrow: 1 }}>
                <Typography
                  variant="body2"
                  sx={{
                    fontWeight: 600,
                    color: "text.primary"
                  }}>
                  {toast.title}
                </Typography>
                {toast.message && (
                  <Typography variant="caption" sx={{ mt: 0.5, lineHeight: 1.6, color: 'text.secondary', display: 'block' }}>
                    {toast.message}
                  </Typography>
                )}
              </Box>
              <IconButton
                aria-label={t('toast.close_notification')}
                size="small"
                onClick={() => setToasts((prev) => prev.filter((item) => item.id !== toast.id))}
              >
                <X size={18} />
              </IconButton>
            </Paper>
          );
        })}
      </Stack>
    </Box>
  );
};

export default ToastProvider;
