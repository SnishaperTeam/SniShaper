import React, { useState, useEffect, useContext } from 'react';
import {
  Zap,
  Globe,
  Monitor,
  ChevronDown,
  ChevronUp,
  Plus,
  Trash2,
  Lock,
  Settings,
  AlertCircle
} from '../lib/icons';
import { Box, Typography, TextField, Switch, IconButton, Button } from '@mui/material';
import { alpha } from '@mui/material/styles';
import { AddSiteGroup, UpdateSiteGroup, GetECHProfiles, GetNAT64Profiles } from '../api/bindings';
import { useTranslation } from '../i18n/I18nContext';
import { SettingsCtx } from '../App';
import { toast } from '../lib/toast';

interface RuleFormProps {
  initialData?: any;
  onSuccess: () => void;
  onCancel: () => void;
}

const normalizeCertVerify = (value: any) => {
  const next = {
    mode: '',
    names: [] as string[],
    suffixes: [] as string[],
    spki_sha256: [] as string[],
    allow_unknown_authority: false
  };
  if (!value || typeof value !== 'object') {
    return next;
  }

  next.mode = String(value.mode || '').trim();
  next.names = Array.isArray(value.names)
    ? value.names
    : Array.isArray(value.allowed_names)
      ? value.allowed_names
      : [];
  next.suffixes = Array.isArray(value.suffixes)
    ? value.suffixes
    : Array.isArray(value.allowed_suffixes)
      ? value.allowed_suffixes
      : [];
  next.spki_sha256 = Array.isArray(value.spki_sha256)
    ? value.spki_sha256
    : Array.isArray(value.allowed_spki)
      ? value.allowed_spki
      : [];
  next.allow_unknown_authority = Boolean(value.allow_unknown_authority);
  return next;
};

const RuleForm: React.FC<RuleFormProps> = ({ initialData, onSuccess, onCancel }) => {
  const { t } = useTranslation();
  const { cache } = useContext(SettingsCtx);
  const ipv6Available = cache.ipv6Available !== false;
  const ipv6Option = (id: string) => id === 'prefer_ipv6' || id === 'ipv6_only';

  const MODES = [
    { id: 'mitm', label: 'MITM', icon: <Zap size={14} />, desc: t('rules.modes.mitm') },
    { id: 'tls-rf', label: t('rules.display.fragment'), icon: <Monitor size={14} />, desc: t('rules.modes.tls-rf') },
    { id: 'quic', label: 'QUIC', icon: <Zap size={14} />, desc: t('rules.modes.quic') },
    { id: 'transparent', label: t('rules.display.transparent'), icon: <Monitor size={14} />, desc: t('rules.modes.transparent') },
    { id: 'migration', label: t('rules.display.migration'), icon: <Globe size={14} />, desc: t('rules.modes.migration') }
  ];

  const DNS_OPTIONS = [
    { id: '', label: t('rules.dns_options.default'), desc: '' },
    { id: 'prefer_ipv4', label: t('rules.dns_options.prefer_ipv4'), desc: t('rules.dns_options.prefer_ipv4') },
    { id: 'prefer_ipv6', label: t('rules.dns_options.prefer_ipv6'), desc: t('rules.dns_options.prefer_ipv6') },
    { id: 'ipv4_only', label: t('rules.dns_options.ipv4_only'), desc: t('rules.dns_options.ipv4_only') },
    { id: 'ipv6_only', label: t('rules.dns_options.ipv6_only'), desc: t('rules.dns_options.ipv6_only') }
  ];

  const CERT_VERIFY_MODES = [
    { id: 'strict_real', label: t('dns.modes.strict'), desc: t('dns.mode_descs.strict') },
    { id: 'allow_names', label: t('dns.modes.names'), desc: t('dns.mode_descs.names') },
    { id: 'allow_suffixes', label: t('dns.modes.suffixes'), desc: t('dns.mode_descs.suffixes') },
    { id: 'allow_spki', label: t('dns.modes.spki'), desc: t('dns.mode_descs.spki') },
    { id: 'chain_only', label: t('dns.modes.chain'), desc: t('dns.mode_descs.chain') },
    { id: '', label: t('rules.cert_policy.ignore'), desc: t('rules.cert_policy.ignore_hint') }
  ];

  const [formData, setFormData] = useState<any>({
    id: '',
    name: '',
    website: '',
    mode: 'mitm',
    upstream: '',
    domains: [] as string[],
    dns_mode: '',
    sni_fake: '',
    enabled: true,
    ech_enabled: false,
    ech_profile_id: '',
    ech_domain: '',
    use_cf_pool: false,
    nat64_enabled: false,
    nat64_profile_id: '',
    cert_verify: {
      mode: '',
      names: [],
      suffixes: [],
      spki_sha256: [],
      allow_unknown_authority: false
    }
  });
  const [domainInput, setDomainInput] = useState('');
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [echProfiles, setEchProfiles] = useState<any[]>([]);
  const [nat64Profiles, setNat64Profiles] = useState<any[]>([]);

  useEffect(() => {
    const loadProfiles = async () => {
      const [ps, ns] = await Promise.all([GetECHProfiles(), GetNAT64Profiles()]);
      setEchProfiles(ps || []);
      setNat64Profiles(ns || []);
    };
    loadProfiles();

    if (initialData) {
      const data = { ...initialData };
      if (String(data.upstream || '').trim().toUpperCase() === 'DIRECT') {
        data.upstream = '';
      }
      data.cert_verify = normalizeCertVerify(data.cert_verify);
      data.nat64_enabled = Boolean(data.nat64_enabled);
      data.nat64_profile_id = String(data.nat64_profile_id || '');
      setFormData(data);
    }
  }, [initialData]);

  const handleAddDomain = () => {
    if (!domainInput.trim()) return;
    const split = domainInput.split(/[\s,;]+/).filter(Boolean);
    setFormData((prev: any) => ({
      ...prev,
      domains: [...new Set([...(prev.domains || []), ...split])]
    }));
    setDomainInput('');
  };

  const handleRemoveDomain = (idx: number) => {
    setFormData((prev: any) => ({
      ...prev,
      domains: prev.domains.filter((_: any, i: number) => i !== idx)
    }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const payload = {
      ...formData,
      cert_verify: normalizeCertVerify(formData.cert_verify),
      upstream: String(formData.upstream || '').trim().toUpperCase() === 'DIRECT'
        ? ''
        : String(formData.upstream || '').trim()
    };
    try {
      if (formData.id) {
        await UpdateSiteGroup(payload);
      } else {
        await AddSiteGroup(payload);
      }
      toast.success(t('rules.notifications.save_success'));
      onSuccess();
    } catch (err: any) {
      const detail = err && typeof err.message === 'string' && err.message ? err.message : err ? String(err) : '';
      toast.error(t('rules.notifications.save_error'), detail || t('rules.notifications.save_error_hint'));
    }
  };

  const certVerifyMode = String(formData.cert_verify?.mode || '').trim();
  const setCertVerify = (patch: Record<string, any>) => setFormData({
    ...formData,
    cert_verify: {
      ...formData.cert_verify,
      ...patch
    }
  });
  const toggleBooleanField = (field: 'enabled' | 'ech_enabled' | 'use_cf_pool' | 'nat64_enabled') =>
    setFormData({ ...formData, [field]: !formData[field] });

  const currentMode = String(formData.mode || '').trim().toLowerCase();
  const showSniFake = ['mitm', 'quic', 'migration'].includes(currentMode);
  const showEchConfig = ['mitm', 'quic'].includes(currentMode);
  const showCertVerify = ['mitm', 'quic', 'migration'].includes(currentMode);
  const showNat64Config = ['mitm', 'quic', 'tls-rf', 'transparent', 'migration'].includes(currentMode);
  const showCfPool = ['mitm', 'transparent', 'tls-rf', 'quic'].includes(currentMode);

  const splitListInput = (value: string) =>
    value
      .split(/[\n,;]+/)
      .map((item) => item.trim())
      .filter(Boolean);

  const joinListInput = (items: string[] | undefined) => (items || []).join('\n');

  const inputRootSx = {
    bgcolor: 'action.hover',
    '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: 'primary.main' },
  };
  const inputSx = { '& .MuiOutlinedInput-root': inputRootSx };

  const sectionLabelSx = {
    fontWeight: 900,
    color: 'text.secondary',
    textTransform: 'uppercase',
    letterSpacing: '0.1em',
    px: 1,
  } as const;

  const cardSx = (active: boolean, opts?: { warn?: boolean; disabled?: boolean; row?: boolean; inactiveBg?: string }) => {
    const row = opts?.row;
    const disabled = opts?.disabled;
    const warn = opts?.warn;
    const accent = (theme: any) => (warn ? theme.palette.warning.main : theme.palette.primary.main);
    return {
      borderRadius: row ? 2 : 1.5,
      border: 1,
      ...(row ? { px: 2, py: 1.5 } : { p: 1.5 }),
      display: 'flex',
      flexDirection: (row ? 'row' : 'column') as 'row' | 'column',
      alignItems: (row ? 'center' : 'flex-start') as 'center' | 'flex-start',
      justifyContent: row ? 'space-between' : undefined,
      gap: row ? 1 : 0.5,
      textAlign: 'left' as const,
      cursor: disabled ? 'not-allowed' : 'pointer',
      transition: 'all 0.2s',
      opacity: disabled ? 0.5 : 1,
      bgcolor: (theme: any) => active ? alpha(accent(theme), 0.1) : (opts?.inactiveBg || theme.palette.background.paper),
      borderColor: (theme: any) => active ? accent(theme) : theme.palette.divider,
      color: (theme: any) => active ? accent(theme) : theme.palette.text.secondary,
      '&:hover': disabled ? {} : {
        borderColor: (theme: any) => active ? accent(theme) : alpha(accent(theme), 0.6),
        bgcolor: (theme: any) => active ? alpha(accent(theme), 0.1) : (opts?.inactiveBg || 'action.hover'),
        color: (theme: any) => active ? accent(theme) : theme.palette.text.primary,
      },
    };
  };

  return (
    <Box component="form" id="rule-form" onSubmit={handleSubmit} sx={{ display: 'flex', flexDirection: 'column', gap: 2, color: 'text.primary', px: 1, pb: 1 }}>
      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, gap: 3 }}>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75 }}>
          <Typography variant="caption" sx={{ ...sectionLabelSx, display: 'flex', alignItems: 'center', gap: 0.75 }}>
            <Box component="span" sx={{ display: 'inline-flex', color: 'primary.main' }}><Zap size={10} /></Box>
            {t('rules.form.name')}
          </Typography>
          <TextField
            type="text"
            required
            size="small"
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            placeholder={t('rules.form.name_placeholder')}
            sx={inputSx}
          />
        </Box>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75 }}>
          <Typography variant="caption" sx={{ ...sectionLabelSx, display: 'flex', alignItems: 'center', gap: 0.75 }}>
            <Box component="span" sx={{ display: 'inline-flex', color: 'primary.main' }}><Settings size={10} /></Box>
            {t('rules.form.website')}
          </Typography>
          <TextField
            type="text"
            size="small"
            value={formData.website}
            onChange={(e) => setFormData({ ...formData, website: e.target.value })}
            placeholder={t('rules.form.website_placeholder')}
            sx={inputSx}
          />
        </Box>
      </Box>

      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
        <Typography variant="caption" sx={sectionLabelSx}>{t('rules.form.mode')}</Typography>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: 'repeat(2, 1fr)', lg: 'repeat(3, 1fr)' }, gap: 1.5 }}>
          {MODES.map((m) => {
            const active = formData.mode === m.id;
            return (
              <Box key={m.id} onClick={() => setFormData({ ...formData, mode: m.id })} sx={{ position: 'relative', overflow: 'hidden', ...cardSx(active) }}>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                  <Box component="span" sx={{ display: 'inline-flex', color: active ? 'primary.main' : 'text.secondary' }}>{m.icon}</Box>
                  <Typography variant="caption" sx={{ fontSize: 12, fontWeight: 900, color: active ? 'primary.main' : 'text.primary', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '100%' }}>{m.label}</Typography>
                </Box>
                <Typography variant="caption" sx={{ fontSize: 9, color: 'text.secondary', fontWeight: 'medium', lineHeight: 1.4, overflow: 'hidden', textOverflow: 'ellipsis', maxWidth: '100%' }}>{m.desc}</Typography>
                {active && (
                  <Box component="span" sx={{ position: 'absolute', right: -8, bottom: -8, opacity: 0.1, color: 'primary.main', transform: 'rotate(12deg)', display: 'inline-flex' }}>{m.icon}</Box>
                )}
              </Box>
            );
          })}
        </Box>
      </Box>

      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
        <Typography variant="caption" sx={sectionLabelSx}>{t('rules.form.domains')}</Typography>
        <Box sx={{ position: 'relative' }}>
          <TextField
            type="text"
            size="small"
            value={domainInput}
            onChange={(e) => setDomainInput(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && (e.preventDefault(), handleAddDomain())}
            placeholder={t('rules.form.domain_placeholder')}
            fullWidth
            sx={{ '& .MuiOutlinedInput-root': { ...inputRootSx, pr: 6 } }}
          />
          <IconButton
            type="button"
            size="small"
            onClick={handleAddDomain}
            aria-label={t('rules.form.add_domain')}
            sx={{ position: 'absolute', right: 8, top: '50%', transform: 'translateY(-50%)', bgcolor: 'primary.main', color: 'primary.contrastText', boxShadow: 3, p: 1, '&:hover': { bgcolor: 'primary.dark' } }}
          >
            <Plus size={20} />
          </IconButton>
        </Box>
        <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, maxHeight: 120, overflowY: 'auto', p: 1.5, bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2 }}>
          {(!formData.domains || formData.domains.length === 0) ? (
            <Typography variant="caption" sx={{ fontSize: 11, color: 'text.secondary', fontStyle: 'italic', px: 1 }}>{t('rules.form.no_domains')}</Typography>
          ) : (
            formData.domains.map((d: any, i: number) => (
              <Box key={i} sx={{ display: 'flex', alignItems: 'center', gap: 0.5, px: 1.25, py: 0.5, bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: '999px', boxShadow: 1, transition: 'all 0.2s', '&:hover': { borderColor: 'error.main' } }}>
                <Typography variant="caption" sx={{ fontSize: 11, fontWeight: 'bold' }}>{d}</Typography>
                <IconButton type="button" size="small" onClick={() => handleRemoveDomain(i)} aria-label={t('rules.form.remove_domain')} sx={{ p: 0.25, color: 'text.secondary', '&:hover': { color: 'error.main' } }}>
                  <Trash2 size={12} />
                </IconButton>
              </Box>
            ))
          )}
        </Box>
      </Box>

      <Box sx={{ pt: 1 }}>
        <Button
          type="button"
          variant="text"
          size="small"
          onClick={() => setShowAdvanced(!showAdvanced)}
          sx={{ color: 'primary.main', fontWeight: 900, textTransform: 'uppercase', letterSpacing: '0.15em', fontSize: '0.75rem', display: 'inline-flex', alignItems: 'center', gap: 1, '&:hover': { opacity: 0.8 } }}
        >
          {showAdvanced ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
          {showAdvanced ? t('rules.form.advanced_hide') : t('rules.form.advanced_show')}
        </Button>
      </Box>

      {showAdvanced && (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3, pt: 1 }}>
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, gap: 3 }}>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75 }}>
              <Typography variant="caption" sx={sectionLabelSx}>{t('rules.form.upstream')}</Typography>
              <TextField
                type="text"
                size="small"
                value={formData.upstream}
                onChange={(e) => setFormData({ ...formData, upstream: e.target.value })}
                placeholder={t('rules.form.upstream_placeholder')}
                sx={inputSx}
              />
            </Box>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75 }}>
              <Typography variant="caption" sx={sectionLabelSx}>{t('rules.form.dns_policy')}</Typography>
              <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' }, gap: 1 }}>
                {DNS_OPTIONS.map((option) => {
                  const active = String(formData.dns_mode || '') === option.id;
                  const disabled = !ipv6Available && ipv6Option(option.id);
                  return (
                    <Box key={option.id || 'default'} onClick={() => !disabled && setFormData({ ...formData, dns_mode: option.id })} sx={cardSx(active, { disabled })}>
                      <Typography variant="caption" sx={{ fontWeight: 900, letterSpacing: '0.05em' }}>{option.label}</Typography>
                      <Typography variant="caption" sx={{ fontSize: 10, lineHeight: 1.6, opacity: 0.8 }}>{disabled ? t('network.ipv6_disabled_title') : option.desc}</Typography>
                    </Box>
                  );
                })}
              </Box>
            </Box>
          </Box>

          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, gap: 3 }}>
            {showSniFake && (
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75 }}>
                <Typography variant="caption" sx={sectionLabelSx}>{t('dns.sni_fake')}</Typography>
                <TextField
                  type="text"
                  size="small"
                  value={formData.sni_fake}
                  onChange={(e) => setFormData({ ...formData, sni_fake: e.target.value })}
                  placeholder={t('rules.form.placeholder_mapped')}
                  sx={inputSx}
                />
              </Box>
            )}
            {showEchConfig && (
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75 }}>
                <Typography variant="caption" sx={sectionLabelSx}>{t('proxies.ech_management')}</Typography>
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                  <Box onClick={() => setFormData({ ...formData, ech_profile_id: '' })} sx={cardSx(!formData.ech_profile_id)}>
                    <Typography variant="caption" sx={{ fontWeight: 900, letterSpacing: '0.05em' }}>{t('rules.form.ech_auto')}</Typography>
                    <Typography variant="caption" sx={{ fontSize: 10, opacity: 0.8 }}>{t('rules.form.ech_auto_hint')}</Typography>
                  </Box>
                  {echProfiles.length > 0 && (
                    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, maxHeight: 176, overflowY: 'auto', pr: 0.5 }}>
                      {echProfiles.map((p) => {
                        const active = formData.ech_profile_id === p.id;
                        return (
                          <Box key={p.id} onClick={() => setFormData({ ...formData, ech_profile_id: p.id })} sx={cardSx(active)}>
                            <Typography variant="caption" sx={{ fontWeight: 900, letterSpacing: '0.05em' }}>{p.name}</Typography>
                            <Typography variant="caption" sx={{ fontSize: 10, opacity: 0.8 }}>{p.discovery_domain || t('rules.form.manual_ech')}</Typography>
                          </Box>
                        );
                      })}
                    </Box>
                  )}
                </Box>
              </Box>
            )}
            {showNat64Config && formData.nat64_enabled && (
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75, mt: 2 }}>
                <Typography variant="caption" sx={sectionLabelSx}>{t('rules.form.nat64_profile')}</Typography>
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                  {nat64Profiles.length === 0 ? (
                    <Typography variant="caption" sx={{ fontSize: 10, color: 'text.secondary', fontWeight: 'bold', px: 1 }}>
                      {t('proxies.no_nat64')}
                    </Typography>
                  ) : (
                    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, maxHeight: 176, overflowY: 'auto', pr: 0.5 }}>
                      {nat64Profiles.map((p) => {
                        const active = formData.nat64_profile_id === p.id;
                        const disabled = !ipv6Available;
                        return (
                          <Box key={p.id} onClick={() => ipv6Available && setFormData({ ...formData, nat64_profile_id: p.id })} sx={cardSx(active, { disabled })}>
                            <Typography variant="caption" sx={{ fontWeight: 900, letterSpacing: '0.05em' }}>{p.name}</Typography>
                            <Typography variant="caption" sx={{ fontSize: 10, opacity: 0.8 }}>{t('rules.form.prefix_label', { prefix: p.prefix })}</Typography>
                          </Box>
                        );
                      })}
                    </Box>
                  )}
                </Box>
              </Box>
            )}
          </Box>

          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, gap: 1.5, p: 2, bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2 }}>
            <Box onClick={() => toggleBooleanField('enabled')} sx={cardSx(formData.enabled, { row: true, inactiveBg: 'action.hover' })}>
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5, minWidth: 0 }}>
                <Typography variant="caption" sx={{ fontWeight: 'bold', color: 'text.primary', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '100%', py: 0.25 }}>{t('rules.form.enable_rule')}</Typography>
                <Typography variant="caption" sx={{ fontSize: 10, color: 'text.secondary' }}>{t('rules.form.enable_hint')}</Typography>
              </Box>
              <Switch size="small" checked={Boolean(formData.enabled)} />
            </Box>
            {showEchConfig && (
              <Box onClick={() => toggleBooleanField('ech_enabled')} sx={cardSx(formData.ech_enabled, { row: true, inactiveBg: 'action.hover' })}>
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5, minWidth: 0 }}>
                  <Typography variant="caption" sx={{ fontWeight: 'bold', color: 'text.primary', display: 'flex', alignItems: 'center', gap: 0.5, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '100%', py: 0.25 }}>
                    <Box component="span" sx={{ display: 'inline-flex', color: '#06b6d4' }}><Lock size={12} /></Box>
                    {t('rules.form.ech_enable')}
                  </Typography>
                  <Typography variant="caption" sx={{ fontSize: 10, color: 'text.secondary' }}>{t('rules.form.ech_hint')}</Typography>
                </Box>
                <Switch size="small" checked={Boolean(formData.ech_enabled)} />
              </Box>
            )}
            {showNat64Config && (
              <Box onClick={() => ipv6Available && toggleBooleanField('nat64_enabled')} sx={cardSx(formData.nat64_enabled, { row: true, inactiveBg: 'action.hover', disabled: !ipv6Available })}>
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5, minWidth: 0 }}>
                  <Typography variant="caption" sx={{ fontWeight: 'bold', color: 'text.primary', display: 'flex', alignItems: 'center', gap: 0.5, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '100%', py: 0.25 }}>
                    <Box component="span" sx={{ display: 'inline-flex', color: 'secondary.main' }}><Globe size={12} /></Box>
                    {t('rules.form.nat64_enable')}
                  </Typography>
                  <Typography variant="caption" sx={{ fontSize: 10, color: 'text.secondary' }}>{!ipv6Available ? t('network.ipv6_disabled_title') : t('rules.form.nat64_hint')}</Typography>
                </Box>
                <Switch size="small" checked={Boolean(formData.nat64_enabled)} disabled={!ipv6Available} />
              </Box>
            )}
            {showCfPool && (
              <Box onClick={() => toggleBooleanField('use_cf_pool')} sx={cardSx(formData.use_cf_pool, { row: true, inactiveBg: 'action.hover' })}>
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5, minWidth: 0 }}>
                  <Typography variant="caption" sx={{ fontWeight: 'bold', color: 'text.primary', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '100%', py: 0.25 }}>{t('rules.form.cf_pool')}</Typography>
                  <Typography variant="caption" sx={{ fontSize: 10, color: 'text.secondary' }}>{t('rules.form.cf_pool_hint')}</Typography>
                </Box>
                <Switch size="small" checked={Boolean(formData.use_cf_pool)} />
              </Box>
            )}
          </Box>

          {showCertVerify && (
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, p: 2, border: 1, borderColor: 'warning.main', bgcolor: 'background.paper', borderRadius: 2, position: 'relative' }}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, color: 'warning.main', mb: 1 }}>
                <AlertCircle size={16} />
                <Typography variant="caption" sx={{ fontWeight: 'bold', textTransform: 'uppercase', letterSpacing: '0.05em' }}>{t('dns.cert_policy')}</Typography>
              </Box>
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                <Typography variant="caption" sx={{ fontSize: 9, fontWeight: 'bold', color: 'text.secondary' }}>{t('dns.verify_mode')}</Typography>
                <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(3, 1fr)' }, gap: 1 }}>
                  {CERT_VERIFY_MODES.map((cm) => {
                    const active = certVerifyMode === cm.id;
                    return (
                      <Box key={cm.id || 'default'} onClick={() => setCertVerify({ mode: cm.id })} sx={cardSx(active, { warn: true, inactiveBg: 'action.hover' })}>
                        <Typography variant="caption" sx={{ fontWeight: 900, letterSpacing: '0.05em' }}>{cm.label}</Typography>
                        <Typography variant="caption" sx={{ fontSize: 10, lineHeight: 1.6, opacity: 0.8 }}>{cm.desc}</Typography>
                      </Box>
                    );
                  })}
                </Box>
              </Box>

              <Box onClick={() => setCertVerify({ allow_unknown_authority: !formData.cert_verify.allow_unknown_authority })} sx={cardSx(Boolean(formData.cert_verify.allow_unknown_authority), { warn: true, row: true, inactiveBg: 'action.hover' })}>
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5, minWidth: 0 }}>
                  <Typography variant="caption" sx={{ fontWeight: 'bold', color: 'text.primary', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '100%', py: 0.25 }}>{t('dns.allow_unknown')}</Typography>
                  <Typography variant="caption" sx={{ fontSize: 10, color: 'text.secondary' }}>{t('rules.form.cert_verify_warn')}</Typography>
                </Box>
                <Switch size="small" color="warning" checked={Boolean(formData.cert_verify.allow_unknown_authority)} />
              </Box>

              {certVerifyMode === 'allow_names' && (
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75 }}>
                  <Typography variant="caption" sx={{ fontSize: 9, fontWeight: 'bold', color: 'text.secondary' }}>{t('dns.allow_names')}</Typography>
                  <TextField
                    multiline
                    rows={4}
                    size="small"
                    value={joinListInput(formData.cert_verify.names)}
                    onChange={(e) => setCertVerify({ names: splitListInput(e.target.value) })}
                    placeholder={t('rules.form.placeholder_domains')}
                    sx={{ ...inputSx, '& textarea': { fontSize: '0.75rem', lineHeight: 1.6, resize: 'none' } }}
                  />
                </Box>
              )}

              {certVerifyMode === 'allow_suffixes' && (
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75 }}>
                  <Typography variant="caption" sx={{ fontSize: 9, fontWeight: 'bold', color: 'text.secondary' }}>{t('dns.allow_suffixes')}</Typography>
                  <TextField
                    multiline
                    rows={4}
                    size="small"
                    value={joinListInput(formData.cert_verify.suffixes)}
                    onChange={(e) => setCertVerify({ suffixes: splitListInput(e.target.value) })}
                    placeholder={t('rules.form.placeholder_suffixes')}
                    sx={{ ...inputSx, '& textarea': { fontSize: '0.75rem', lineHeight: 1.6, resize: 'none' } }}
                  />
                </Box>
              )}

              {certVerifyMode === 'allow_spki' && (
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75 }}>
                  <Typography variant="caption" sx={{ fontSize: 9, fontWeight: 'bold', color: 'text.secondary' }}>{t('rules.form.allow_spki_list')}</Typography>
                  <TextField
                    multiline
                    rows={4}
                    size="small"
                    value={joinListInput(formData.cert_verify.spki_sha256)}
                    onChange={(e) => setCertVerify({ spki_sha256: splitListInput(e.target.value) })}
                    placeholder={t('rules.form.placeholder_spki')}
                    sx={{ ...inputSx, '& textarea': { fontSize: '0.75rem', lineHeight: 1.6, resize: 'none', } }}
                  />
                </Box>
              )}
            </Box>
          )}
        </Box>
      )}
    </Box>
  );
};

export default RuleForm;
