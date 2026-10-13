import React, { useState, useEffect } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { Box, Button, Typography } from '@mui/material';
import { OpenURL, SetTutorialDone } from '../api/bindings';
import { useTranslation } from '../i18n/I18nContext';

const WIKI_URL = 'https://github.com/SnishaperTeam/SniShaper/wiki';

interface TutorialStep {
  route: string | null; // null = final step (centered card, no target)
  selector?: string;
  titleKey: string;
  textKey: string;
}

// Step keys are static and exist in every locale, so t() cannot miss.
const STEPS: TutorialStep[] = [
  { route: '/dashboard', selector: '[data-tut="proxy-toggle"]', titleKey: 'tutorial.s1.title', textKey: 'tutorial.s1.text' },
  { route: '/dashboard', selector: '[data-tut="sys-proxy"]', titleKey: 'tutorial.s2.title', textKey: 'tutorial.s2.text' },
  { route: '/rules', selector: '[data-tut="rules-title"]', titleKey: 'tutorial.s3.title', textKey: 'tutorial.s3.text' },
  { route: '/settings', selector: '[data-tut="cert-section"]', titleKey: 'tutorial.s4.title', textKey: 'tutorial.s4.text' },
  { route: '/settings', selector: '[data-tut="tun-section"]', titleKey: 'tutorial.s5.title', textKey: 'tutorial.s5.text' },
  { route: null, titleKey: 'tutorial.s6.title', textKey: 'tutorial.s6.text' },
];

interface TutorialProps {
  onFinish: () => void;
}

const Tutorial: React.FC<TutorialProps> = ({ onFinish }) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const [step, setStep] = useState(0);
  // undefined = locating, null = target not found (fallback to centered card)
  const [rect, setRect] = useState<DOMRect | null | undefined>(undefined);
  const current = STEPS[step];
  const isLast = step === STEPS.length - 1;

  const finish = async () => {
    try { await SetTutorialDone(); } catch { /* non-fatal */ }
    onFinish();
  };
  const next = () => {
    if (isLast) { void finish(); return; }
    setRect(undefined);
    setStep(step + 1);
  };

  // Suppress the first-run certificate modal while the tutorial is active —
  // the tutorial covers the certificate in its own step.
  useEffect(() => {
    sessionStorage.setItem('ca_modal_shown', 'true');
  }, []);

  // Navigate to the step's page, then locate and measure the target element.
  useEffect(() => {
    if (!current.route) return;
    if (location.pathname !== current.route) {
      navigate(current.route);
      return; // effect re-runs after the route changes
    }
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout>;
    const deadline = Date.now() + 4000;
    const locate = () => {
      if (cancelled) return;
      const el = current.selector ? document.querySelector(current.selector) : null;
      if (el) {
        (el as HTMLElement).scrollIntoView({ block: 'center' });
        requestAnimationFrame(() => {
          if (!cancelled) setRect((el as HTMLElement).getBoundingClientRect());
        });
        return;
      }
      if (Date.now() < deadline) {
        timer = setTimeout(locate, 200);
      } else {
        setRect(null);
      }
    };
    timer = setTimeout(locate, 80);
    return () => { cancelled = true; clearTimeout(timer); };
  }, [step, location.pathname, current, navigate]);

  // Keep the spotlight in sync when the window is resized.
  useEffect(() => {
    const onResize = () => {
      const el = current.selector ? document.querySelector(current.selector) : null;
      setRect(el ? (el as HTMLElement).getBoundingClientRect() : null);
    };
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, [current]);

  // Game-style advance: clicking the highlighted element moves to the next
  // step. The overlay itself is pointer-events:none, so the real button
  // receives the click and performs its real action.
  useEffect(() => {
    if (!current.selector) return;
    const onClick = (e: MouseEvent) => {
      const el = document.querySelector(current.selector as string);
      const node = e.target as Node;
      if (el && (node === el || el.contains(node))) next();
    };
    document.addEventListener('click', onClick, true);
    return () => document.removeEventListener('click', onClick, true);
  });

  const spot = rect && current.selector ? rect : null;
  const PAD = 8;
  const CALLOUT_W = 300;
  const calloutSx: React.CSSProperties = spot
    ? (() => {
        const below = spot.bottom + 250 < window.innerHeight;
        const left = Math.min(Math.max(spot.left, 12), Math.max(12, window.innerWidth - CALLOUT_W - 12));
        return below
          ? { top: spot.bottom + 14, left }
          : { bottom: window.innerHeight - spot.top + 14, left };
      })()
    : { top: '50%', left: '50%', transform: 'translate(-50%, -50%)' };

  return (
    <Box>
      {/* Full dim while transitioning / for the final step / when the target
          is missing. Blocks stray clicks (the callout sits above it). */}
      {!spot && <Box sx={{ position: 'fixed', inset: 0, zIndex: 2000, bgcolor: 'rgba(0,0,0,0.6)' }} />}

      {/* Spotlight: the giant box-shadow dims everything except the target;
          pointer-events:none keeps the target clickable. */}
      {spot && (
        <Box sx={{
          position: 'fixed',
          zIndex: 2000,
          pointerEvents: 'none',
          top: spot.top - PAD,
          left: spot.left - PAD,
          width: spot.width + PAD * 2,
          height: spot.height + PAD * 2,
          borderRadius: 2.5,
          border: '2px solid',
          borderColor: 'primary.main',
          boxShadow: '0 0 0 9999px rgba(0,0,0,0.6)',
          transition: 'top 0.3s ease, left 0.3s ease, width 0.3s ease, height 0.3s ease',
        }} />
      )}

      {/* Callout */}
      <Box sx={{
        position: 'fixed',
        zIndex: 2001,
        width: CALLOUT_W,
        ...calloutSx,
        bgcolor: 'background.paper',
        border: 1,
        borderColor: 'divider',
        borderRadius: 2,
        boxShadow: 8,
        p: 2.5,
        display: 'flex',
        flexDirection: 'column',
        gap: 1.5,
      }}>
        <Typography variant="overline" color="primary.main" sx={{ fontWeight: 700, lineHeight: 1 }}>
          {step + 1} / {STEPS.length}
        </Typography>
        <Typography variant="subtitle2" sx={{ fontWeight: 700 }}>
          {t(current.titleKey)}
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ lineHeight: 1.6 }}>
          {t(current.textKey)}
        </Typography>
        {isLast && (
          <Button
            fullWidth
            variant="outlined"
            size="small"
            onClick={() => { void OpenURL(WIKI_URL); }}
          >
            {t('tutorial.open_wiki')}
          </Button>
        )}
        <Box sx={{ display: 'flex', gap: 1 }}>
          {!isLast && (
            <Button size="small" onClick={() => { void finish(); }} sx={{ color: 'text.secondary', flex: 1 }}>
              {t('tutorial.skip')}
            </Button>
          )}
          <Button size="small" variant="contained" onClick={next} sx={{ flex: 1 }}>
            {isLast ? t('tutorial.done') : t('tutorial.next')}
          </Button>
        </Box>
      </Box>
    </Box>
  );
};

export default Tutorial;
