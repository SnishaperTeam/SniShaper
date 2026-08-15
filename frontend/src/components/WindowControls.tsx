import React, { useCallback } from 'react';
import { Box, IconButton, Tooltip } from '@mui/material';
import { Minus, Square, X } from '../lib/icons';
import { useTranslation } from '../i18n/I18nContext';
import {
  HandleWindowClose,
  WindowMinimise,
  WindowToggleMaximise
} from "../api/bindings";

const WindowControls: React.FC = React.memo(() => {
  const { t } = useTranslation();
  const handleMinimise = useCallback(async () => {
    try {
      await WindowMinimise();
    } catch (e) {
      console.error("WindowMinimise failed:", e);
    }
  }, []);

  const handleToggleMaximise = useCallback(async () => {
    try {
      await WindowToggleMaximise();
    } catch (e) {
      console.error("WindowToggleMaximise failed:", e);
    }
  }, []);

  const handleClose = useCallback(async () => {
    try {
      await HandleWindowClose();
    } catch (e) {
      console.error("HandleWindowClose failed:", e);
    }
  }, []);

  return (
    <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', '--wails-draggable': 'no-drag' }}>
      <Tooltip title={t('window.minimize')}>
        <IconButton
          size="small"
          aria-label={t('window.minimize')}
          onClick={handleMinimise}
          sx={{ color: 'text.primary' }}
        >
          <Minus size={18} />
        </IconButton>
      </Tooltip>
      <Tooltip title={t('window.maximize_restore')}>
        <IconButton
          size="small"
          aria-label={t('window.maximize_restore')}
          onClick={handleToggleMaximise}
          sx={{ color: 'text.primary' }}
        >
          <Square size={18} />
        </IconButton>
      </Tooltip>
      <Tooltip title={t('window.close')}>
        <IconButton
          size="small"
          aria-label={t('window.close')}
          color="error"
          onClick={handleClose}
        >
          <X size={18} />
        </IconButton>
      </Tooltip>
    </Box>
  );
});

export default WindowControls;