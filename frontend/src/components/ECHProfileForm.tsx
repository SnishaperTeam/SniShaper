import React, { useState, useEffect } from 'react';
import {
  Shield, Search, CheckCircle2, AlertCircle, Save
} from '../lib/icons';
import { UpsertECHProfile, FetchECHConfig, GetDNSNodes } from '../api/bindings';
import { useTranslation } from '../i18n/I18nContext';
import {
  Box, Button, TextField, Stack, Typography, LinearProgress,
  Alert, FormControlLabel, Switch,
} from '@mui/material';

interface ECHProfileFormProps {
  initialData?: any;
  onSuccess: () => void;
  onCancel: () => void;
}

const ECHProfileForm: React.FC<ECHProfileFormProps> = ({ initialData, onSuccess, onCancel }) => {
  const { t } = useTranslation();
  const [formData, setFormData] = useState<any>({
    id: '',
    name: '',
    discovery_domain: '',
    doh_upstream: '',
    config: '',
    auto_update: true
  });
  const [isFetching, setIsFetching] = useState(false);
  const [fetchError, setFetchError] = useState('');
  const [fetchSuccess, setFetchSuccess] = useState(false);

  useEffect(() => {
    const init = async () => {
      let currentData = initialData || {};

      if (!currentData.id && !currentData.doh_upstream) {
        try {
          const dnsNodes = await GetDNSNodes();
          if (dnsNodes && dnsNodes.length > 0) {
            const firstNode = dnsNodes.find((n: any) => n.enabled) || dnsNodes[0];
            if (firstNode && firstNode.url) {
              currentData.doh_upstream = firstNode.url;
            }
          }
        } catch (e) {
          console.error("Failed to fetch DNS nodes for ECH default", e);
        }

        if (!currentData.doh_upstream) {
          currentData.doh_upstream = 'https://cloudflare-dns.com/dns-query';
        }
      }

      setFormData((prev: any) => ({ ...prev, ...currentData }));
    };

    init();
  }, [initialData]);

  const handleFetch = async () => {
    if (!formData.discovery_domain) {
      setFetchError(t('ech_form.domain_placeholder'));
      return;
    }
    setIsFetching(true);
    setFetchError('');
    setFetchSuccess(false);

    try {
      const result = await FetchECHConfig(formData.discovery_domain, formData.doh_upstream);
      if (result && result.length > 10) {
        setFormData((prev: any) => ({ ...prev, config: result }));
        setFetchSuccess(true);
      } else {
        setFetchError(t('ech_form.probe_failed'));
      }
    } catch (e: any) {
      setFetchError(`${t('common.error')}: ${e.message || e}`);
    } finally {
      setIsFetching(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    await UpsertECHProfile(formData);
    onSuccess();
  };

  return (
    <form onSubmit={handleSubmit}>
      <Stack direction="column" spacing={3} sx={{ alignItems: 'stretch', color: 'text.primary' }}>
        <Box sx={{ p: 1.5, bgcolor: 'rgba(25, 118, 210, 0.05)', border: 1, borderColor: 'rgba(25, 118, 210, 0.2)', borderRadius: 2 }}>
          <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', mb: 1.5 }}>
            <Shield size={24} color="primary.main" />
            <Stack direction="column" spacing={0.25}>
              <Typography variant="body2" sx={{ fontWeight: 'bold', textTransform: 'uppercase', letterSpacing: '0.05em', color: 'primary.main' }}>
                {t('ech_form.title')}
              </Typography>
              <Typography variant="caption" color="primary.main" sx={{ opacity: 0.7, fontWeight: 'medium' }}>
                {t('ech_form.subtitle')}
              </Typography>
            </Stack>
          </Stack>

          <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center' }}>
            <TextField
              label={t('ech_form.domain_placeholder')}
              size="small"
              value={formData.discovery_domain}
              onChange={(e) => setFormData({ ...formData, discovery_domain: e.target.value })}
              placeholder={t('ech_form.domain_placeholder')}
              slotProps={{ input: { style: { borderRadius: 1 } }, inputLabel: { shrink: true } }}
            />
            <Button
              variant="contained"
              color="primary"
              size="small"
              startIcon={isFetching ? <LinearProgress sx={{ width: 24 }} /> : <Search size={14} />}
              onClick={handleFetch}
              disabled={isFetching}
            >
              {isFetching ? t('ech_form.probing') : t('ech_form.probe_and_resolve')}
            </Button>
          </Stack>

          {fetchError && (
            <Alert severity="error" sx={{ mb: 1, p: 1 }}>
              <AlertCircle size={20} />
              {fetchError}
            </Alert>
          )}
          {fetchSuccess && (
            <Alert severity="success" sx={{ mb: 1, p: 1 }}>
              <CheckCircle2 size={20} />
              {t('ech_form.probe_success')}
            </Alert>
          )}
        </Box>

        <Stack direction={{ xs: 'column', md: 'row' }} spacing={3} sx={{ pt: 1 }}>
          <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 0.75 }}>
            <TextField
              label={t('ech_form.config_name')}
              size="small"
              required
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              placeholder={t('ech_form.name_placeholder')}
              slotProps={{ input: { style: { borderRadius: 1 } }, inputLabel: { shrink: true } }}
            />
          </Box>
          <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 0.75 }}>
            <TextField
              label={t('ech_form.doh_source')}
              size="small"
              value={formData.doh_upstream}
              onChange={(e) => setFormData({ ...formData, doh_upstream: e.target.value })}
              slotProps={{ input: { style: { borderRadius: 1 } }, inputLabel: { shrink: true } }}
            />
          </Box>
        </Stack>

        <Box sx={{ pt: 0.75, display: 'flex', flexDirection: 'column', gap: 0.75 }}>
          <TextField
            label={t('ech_form.raw_content')}
            size="small"
            multiline
            rows={4}
            required
            value={formData.config}
            onChange={(e) => setFormData({ ...formData, config: e.target.value })}
            placeholder={t('ech_form.raw_placeholder')}
            slotProps={{ input: { style: { borderRadius: 1, fontSize: '0.75rem', fontFamily: 'mono' } }, inputLabel: { shrink: true } }}
          />
        </Box>

        <Box sx={{ pt: 0.5, display: 'flex', alignItems: 'center', gap: 1.5 }}>
          <FormControlLabel
            control={
              <Switch
                checked={formData.auto_update}
                onChange={(e) => setFormData({ ...formData, auto_update: e.target.checked })}
                color="primary"
                size="small"
              />
            }
            label={t('ech_form.auto_sync')}
          />
          <Typography variant="caption" color="text.secondary">
            {t('ech_form.auto_sync_hint')}
          </Typography>
        </Box>

        <Box sx={{ pt: 1.5, display: 'flex', justifyContent: 'flex-end', gap: 1.5, borderTop: 1, borderColor: 'divider' }}>
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
          >
            {t('common.save')}
          </Button>
        </Box>
      </Stack>
    </form>
  );
};

export default ECHProfileForm;
