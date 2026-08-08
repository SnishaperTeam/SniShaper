import React, { useState, useEffect } from 'react';
import { Layers, Save } from '../lib/icons';
import { AddNAT64Profile, UpdateNAT64Profile } from '../api/bindings';
import { useTranslation } from '../i18n/I18nContext';
import {
  Box, Button, TextField, Stack, Typography, FormControlLabel, Switch,
} from '@mui/material';

interface NAT64ProfileFormProps {
  initialData?: any;
  onSuccess: () => void;
  onCancel: () => void;
}

const NAT64ProfileForm: React.FC<NAT64ProfileFormProps> = ({ initialData, onSuccess, onCancel }) => {
  const { t } = useTranslation();
  const [formData, setFormData] = useState<any>({
    id: '',
    name: '',
    prefix: ''
  });
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    if (initialData) {
      setFormData({
        id: initialData.id || '',
        name: initialData.name || '',
        prefix: initialData.prefix || ''
      });
    }
  }, [initialData]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!formData.name.trim() || !formData.prefix.trim()) return;

    setIsSubmitting(true);
    try {
      if (formData.id) {
        await UpdateNAT64Profile(formData);
      } else {
        await AddNAT64Profile(formData);
      }
      onSuccess();
    } catch (err) {
      console.error(err);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <form onSubmit={handleSubmit}>
      <Stack direction="column" spacing={3} sx={{ alignItems: 'stretch', color: 'text.primary' }}>
        <Box sx={{ p: 1.5, bgcolor: 'rgba(25, 118, 210, 0.05)', border: 1, borderColor: 'rgba(25, 118, 210, 0.2)', borderRadius: 2 }}>
          <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center' }}>
            <Layers size={24} color="primary.main" />
            <Stack direction="column" spacing={0.25}>
              <Typography variant="body2" sx={{ fontWeight: 'bold', textTransform: 'uppercase', letterSpacing: '0.05em', color: 'primary.main' }}>
                {formData.id ? t('proxies.edit_nat64') || '编辑 NAT64 配置' : t('proxies.add_nat64') || '添加 NAT64 配置'}
              </Typography>
              <Typography variant="caption" color="primary.main" sx={{ opacity: 0.7, fontWeight: 'medium' }}>
                {t('proxies.nat64_form_subtitle') || '配置独立映射规则前缀'}
              </Typography>
            </Stack>
          </Stack>
        </Box>

        <Stack direction="column" spacing={2} sx={{ alignItems: 'stretch' }}>
          <Box>
            <TextField
              label={t('proxies.nat64_form_name') || '配置名称'}
              size="small"
              required
              fullWidth
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              placeholder="例如：特定黑名单绕过"
              slotProps={{ input: { style: { borderRadius: 1 } }, inputLabel: { shrink: true } }}
            />
          </Box>

          <Box>
            <TextField
              label={t('proxies.nat64_form_prefix') || 'NAT64 前缀'}
              size="small"
              required
              fullWidth
              value={formData.prefix}
              onChange={(e) => setFormData({ ...formData, prefix: e.target.value })}
              placeholder="例如：64:ff9b::"
              slotProps={{ input: { style: { borderRadius: 1, fontFamily: 'mono' } }, inputLabel: { shrink: true } }}
            />
          </Box>
        </Stack>

        <Stack direction="row" spacing={1.5} sx={{ justifyContent: 'flex-end', pt: 2, borderTop: 1, borderColor: 'divider' }}>
          <Button
            variant="text"
            size="small"
            onClick={onCancel}
            sx={{ borderWidth: 1, borderColor: 'divider', borderRadius: 1, '&:hover': { bgcolor: 'action.hover' } }}
          >
            {t('common.cancel')}
          </Button>
          <Button
            type="submit"
            variant="contained"
            color="primary"
            size="small"
            startIcon={<Save size={14} />}
            disabled={isSubmitting}
          >
            {isSubmitting ? t('ech_form.probing') : t('common.save')}
          </Button>
        </Stack>
      </Stack>
    </form>
  );
};

export default NAT64ProfileForm;
