import React, { useCallback } from 'react';
import { Box, IconButton, Tooltip } from '@mui/material';
import { HorizontalRule, CropSquare, Close } from '@mui/icons-material';
import {
  HandleWindowClose,
  WindowMinimise,
  WindowToggleMaximise
} from "../api/bindings";

const WindowControls: React.FC = React.memo(() => {
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
      <Tooltip title="最小化">
        <IconButton
          size="small"
          aria-label="最小化"
          onClick={handleMinimise}
          sx={{ color: 'text.primary' }}
        >
          <HorizontalRule fontSize="small" />
        </IconButton>
      </Tooltip>
      <Tooltip title="最大化/还原">
        <IconButton
          size="small"
          aria-label="最大化/还原"
          onClick={handleToggleMaximise}
          sx={{ color: 'text.primary' }}
        >
          <CropSquare fontSize="small" />
        </IconButton>
      </Tooltip>
      <Tooltip title="关闭">
        <IconButton
          size="small"
          aria-label="关闭"
          color="error"
          onClick={handleClose}
        >
          <Close fontSize="small" />
        </IconButton>
      </Tooltip>
    </Box>
  );
});

export default WindowControls;