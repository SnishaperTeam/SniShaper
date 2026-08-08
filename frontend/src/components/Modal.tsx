import React from 'react';
import { Modal as MuiModal, Box, Typography, IconButton } from '@mui/material';
import { X } from '../lib/icons';

interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  title: string;
  subtitle?: string;
  children: React.ReactNode;
  footer?: React.ReactNode;
  maxWidth?: string;
}

const Modal: React.FC<ModalProps> = ({
  isOpen,
  onClose,
  title,
  subtitle,
  children,
  footer,
  maxWidth = '42rem'
}) => {
  return (
    <MuiModal open={isOpen} onClose={onClose}>
      <Box
        sx={{
          position: 'absolute',
          top: '50%',
          left: '50%',
          transform: 'translate(-50%, -50%)',
          width: '100%',
          maxWidth,
          maxHeight: '85vh',
          display: 'flex',
          flexDirection: 'column',
          bgcolor: 'background.paper',
          border: 1,
          borderColor: 'divider',
          borderRadius: 2,
          boxShadow: 24,
          p: 3,
        }}
      >
        {title && (
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5, pr: 4 }}>
            <Typography variant="h5" sx={{ fontWeight: 'bold', letterSpacing: '-0.02em' }}>
              {title}
            </Typography>
            {subtitle && (
              <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5, fontWeight: 'medium' }}>
                {subtitle}
              </Typography>
            )}
          </Box>
        )}
        <Box sx={{ flexGrow: 1, py: 2, overflowY: 'auto' }}>
          {children}
        </Box>
        {footer && (
          <Box sx={{ pt: 2, borderTop: 1, borderColor: 'divider', display: 'flex', gap: 1, justifyContent: 'flex-end' }}>
            {footer}
          </Box>
        )}
        <IconButton
          aria-label="关闭对话框"
          size="small"
          onClick={onClose}
          sx={{ position: 'absolute', top: 8, right: 8 }}
        >
          <X size={16} />
        </IconButton>
      </Box>
    </MuiModal>
  );
};

export default Modal;
