import React, { useState } from 'react';
import { Box, Button, Typography } from '@mui/material';
import { alpha } from '@mui/material/styles';
import logoUrl from '../assets/logo.svg';
import { OpenURL, SetTutorialDone } from '../api/bindings';
import { useTranslation } from '../i18n/I18nContext';

const WIKI_URL = 'https://github.com/SnishaperTeam/SniShaper/wiki';
// Step index is bounded by construction (0..STEP_COUNT-1) and every
// tutorial.sN.{title,text} key exists in all locales, so t() cannot miss.
const STEP_COUNT = 6;

interface TutorialProps {
  onFinish: () => void;
}

const Tutorial: React.FC<TutorialProps> = ({ onFinish }) => {
  const { t } = useTranslation();
  const [step, setStep] = useState(0);
  const isLast = step === STEP_COUNT - 1;

  const finish = async () => {
    try { await SetTutorialDone(); } catch { /* non-fatal */ }
    onFinish();
  };

  return (
    <Box sx={{
      position: 'fixed',
      inset: 0,
      zIndex: 100,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      bgcolor: (theme) => alpha(theme.palette.background.default, 0.88),
      backdropFilter: 'blur(10px)',
      WebkitBackdropFilter: 'blur(10px)',
      p: 3,
    }}>
      <Box sx={{
        width: '100%',
        maxWidth: '30rem',
        bgcolor: 'background.paper',
        border: 1,
        borderColor: 'divider',
        borderRadius: 3,
        boxShadow: 8,
        p: 4,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        textAlign: 'center',
        gap: 2.5,
        '@keyframes welcomeFadeZoom': {
          '0%': { opacity: 0, transform: 'scale(0.95)' },
          '100%': { opacity: 1, transform: 'scale(1)' },
        },
        animation: 'welcomeFadeZoom 0.4s ease',
      }}>
        <Box component="img" src={logoUrl} alt="logo" sx={{ width: 56, height: 56, objectFit: 'contain' }} />
        <Typography variant="h6" sx={{ fontWeight: 700 }}>
          {t(`tutorial.s${step + 1}.title`)}
        </Typography>
        <Typography color="text.secondary" sx={{ minHeight: 72 }}>
          {t(`tutorial.s${step + 1}.text`)}
        </Typography>

        <Box sx={{ display: 'flex', gap: 1, justifyContent: 'center', mt: 0.5 }}>
          {Array.from({ length: STEP_COUNT }, (_, i) => (
            <Box
              key={i}
              onClick={() => setStep(i)}
              role="button"
              aria-label={`${i + 1} / ${STEP_COUNT}`}
              sx={{
                width: 8,
                height: 8,
                borderRadius: '50%',
                cursor: 'pointer',
                bgcolor: i === step ? 'primary.main' : 'action.disabled',
                transition: 'background-color 0.2s',
              }}
            />
          ))}
        </Box>

        {isLast && (
          <Button fullWidth variant="outlined" onClick={() => { void OpenURL(WIKI_URL); }}>
            {t('tutorial.open_wiki')}
          </Button>
        )}

        <Box sx={{ display: 'flex', gap: 1.5, width: '100%' }}>
          <Button fullWidth onClick={() => { void finish(); }} sx={{ color: 'text.secondary' }}>
            {t('tutorial.skip')}
          </Button>
          {step > 0 && (
            <Button fullWidth variant="outlined" disabled={isLast} onClick={() => setStep(step - 1)}>
              {t('tutorial.back')}
            </Button>
          )}
          <Button
            fullWidth
            variant="contained"
            onClick={() => { if (isLast) { void finish(); } else { setStep(step + 1); } }}
          >
            {isLast ? t('tutorial.done') : t('tutorial.next')}
          </Button>
        </Box>
      </Box>
    </Box>
  );
};

export default Tutorial;
