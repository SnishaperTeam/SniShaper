import React, { useState } from 'react';
import { Box, Button, Typography } from '@mui/material';
import { alpha } from '@mui/material/styles';
import logoUrl from '../assets/logo.svg';
import { SetLanguage } from '../api/bindings';
import { useTranslation } from '../i18n/I18nContext';
import { Languages } from '../lib/icons';

interface WelcomeProps {
  onComplete: (lang: string) => void;
}

const languages = [
  { id: 'zh' as const, label: '简体中文', sub: 'Chinese' },
  { id: 'en' as const, label: 'English', sub: '英语' },
  { id: 'ru' as const, label: 'Русский', sub: '俄语' },
];

const Welcome: React.FC<WelcomeProps> = ({ onComplete }) => {
  const { setLanguage, t } = useTranslation();
  const [selected, setSelected] = useState<'zh' | 'en' | 'ru'>('zh');

  const handleStart = async () => {
    await SetLanguage(selected);
    localStorage.setItem('language', selected);
    setLanguage(selected);
    onComplete(selected);
  };

  return (
    <Box sx={{
      position: 'fixed',
      inset: 0,
      zIndex: 100,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      bgcolor: 'background.default',
      p: 3,
    }}>
      <Box sx={{
        width: '100%',
        maxWidth: '28rem',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        textAlign: 'center',
        gap: 4,
        '@keyframes welcomeFadeZoom': {
          '0%': { opacity: 0, transform: 'scale(0.95)' },
          '100%': { opacity: 1, transform: 'scale(1)' },
        },
        animation: 'welcomeFadeZoom 0.5s ease',
      }}>
        <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1.5 }}>
          <Box component="img" src={logoUrl} alt="logo" sx={{
            width: 80,
            height: 80,
            mb: 3,
            filter: 'drop-shadow(0 25px 50px rgba(0, 0, 0, 0.25))',
            '@keyframes welcomePulse': {
              '0%, 100%': { opacity: 1 },
              '50%': { opacity: 0.5 },
            },
            animation: 'welcomePulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite',
          }} />
          <Typography variant="h1" sx={{ fontSize: '1.875rem', fontWeight: 700, color: 'text.primary', letterSpacing: '-0.025em' }}>
            {t('welcome.title')}
          </Typography>
          <Typography color="text.secondary">{t('welcome.subtitle')}</Typography>
        </Box>

        <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 1.5, width: '100%' }}>
          {languages.map((lang) => {
            const active = selected === lang.id;
            return (
              <Button
                key={lang.id}
                onClick={() => { setSelected(lang.id); setLanguage(lang.id); }}
                sx={{
                  flexDirection: 'column',
                  py: 2,
                  borderRadius: 2,
                  border: 2,
                  transition: 'all 0.2s',
                  ...(active
                    ? {
                        borderColor: 'primary.main',
                        bgcolor: (theme) => alpha(theme.palette.primary.main, 0.05),
                        color: 'primary.main',
                        boxShadow: 1,
                        '&:hover': { bgcolor: (theme) => alpha(theme.palette.primary.main, 0.05) },
                      }
                    : {
                        borderColor: 'divider',
                        bgcolor: (theme) => alpha(theme.palette.background.paper, 0.5),
                        color: 'text.secondary',
                        '&:hover': { borderColor: (theme) => alpha(theme.palette.primary.main, 0.4) },
                      }),
                }}
              >
                <Box component="span" sx={{ display: 'flex', alignItems: 'center', gap: 1, fontSize: '1.125rem', fontWeight: 500, mb: 0.5 }}>
                  <Languages size={18} />
                  {lang.label}
                </Box>
                <Box component="span" sx={{ fontSize: '0.75rem', opacity: 0.6 }}>{lang.sub}</Box>
              </Button>
            );
          })}
        </Box>

        <Button
          fullWidth
          onClick={handleStart}
          sx={{
            py: 1.75,
            borderRadius: 2,
            fontWeight: 600,
            boxShadow: (theme) => `0 10px 15px -3px ${alpha(theme.palette.primary.main, 0.2)}`,
            transition: 'all 0.2s',
            '&:hover': { bgcolor: (theme) => alpha(theme.palette.primary.main, 0.9) },
            '&:active': { transform: 'scale(0.98)' },
          }}
        >
          {t('welcome.start')}
        </Button>
      </Box>
    </Box>
  );
};

export default Welcome;
