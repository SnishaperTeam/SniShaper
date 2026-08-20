import React, { useState, useEffect } from 'react';
import {
  Plus, Search, Filter, Edit, OpenInNew, Trash2, Activity, Zap
} from '../lib/icons';
import {
  GetSiteGroups, DeleteSiteGroup, ExportConfig
} from '../api/bindings';
import {
  Box, Typography, Button, IconButton, TextField, InputAdornment,
} from '@mui/material';
import { alpha } from '@mui/material/styles';
import Modal from '../components/Modal';
import RuleForm from '../components/RuleForm';
import { toast } from '../lib/toast';
import { useTranslation } from '../i18n/I18nContext';

const FILTER_MODES = ['ALL', 'MITM', 'TLS-RF', 'QUIC', 'TRANSPARENT', 'MIGRATION'] as const;

const normalizeMode = (value: unknown) => String(value || '').trim().toLowerCase();

const getEffectiveMode = (group: any) => {
  const mode = normalizeMode(group?.mode);
  if (mode === 'quic') return 'QUIC';
  if (mode === 'tls-rf') return 'TLS-RF';
  if (mode === 'mitm') return 'MITM';
  if (mode === 'transparent') return 'TRANSPARENT';
  if (mode === 'migration') return 'MIGRATION';
  return mode ? mode.toUpperCase() : 'DIRECT';
};

const RuleItem: React.FC<{ group: any; onEdit: (group: any) => void; onDelete: (id: string) => void }> = ({ group, onEdit, onDelete }) => {
  const { t } = useTranslation();

  const getModeDisplay = (mode: string) => {
    switch (mode) {
      case 'TRANSPARENT': return t('rules.display.transparent');
      case 'DIRECT': return t('rules.display.direct');
      case 'TLS-RF': return t('rules.display.fragment');
      case 'MIGRATION': return t('rules.display.migration');
      default: return mode;
    }
  };

  const modeColors: Record<string, string> = {
    'mitm': 'warning.main',
    'transparent': 'error.main',
    'quic': 'success.main',
    'tls-rf': 'primary.main',
    'migration': 'secondary.main'
  };

  const getEffectiveUpstream = (group: any) => {
    const upstream = String(group.upstream || '').trim();
    if (upstream && upstream.toUpperCase() !== 'DIRECT') return upstream;
    if (group.use_cf_pool) return t('rules.display.cf_ip_pool');
    return t('rules.display.doh_dynamic');
  };

  const effectiveMode = getEffectiveMode(group);
  const modeKey = normalizeMode(group?.mode) || normalizeMode(effectiveMode);

  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, py: 1.5, px: 3, bgcolor: 'background.paper', borderBottom: 1, borderColor: 'divider', transition: 'background-color 0.2s', '&:hover': { bgcolor: 'action.hover' }, minWidth: 0 }}>
      <Box sx={{ flex: 1, minWidth: 0, display: 'flex', alignItems: 'center', gap: 3, flexWrap: 'wrap' }}>
        <Box sx={{ width: { xs: '100%', md: '33%' }, minWidth: 0, flexShrink: 0 }}>
          <Typography variant="body2" sx={{ fontWeight: 700, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '100%', py: 0.25, outline: '1px solid transparent' }}>{group.name || t('common.unknown')}</Typography>
          <Typography variant="caption" sx={{ fontSize: 10, color: 'text.secondary', fontWeight: 500, textTransform: 'uppercase', letterSpacing: '0.05em', display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '100%', py: 0.25, outline: '1px solid transparent' }}>{group.website || t('rules.display.default_group')}</Typography>
        </Box>
        <Box sx={{ flex: 1, minWidth: 0, display: { xs: 'none', md: 'flex' }, alignItems: 'center', gap: 1, overflow: 'hidden' }}>
          {(group.domains || []).slice(0, 4).map((d: string, i: number) => (
            <Typography key={i} variant="caption" sx={{ fontSize: 9, bgcolor: 'action.hover', px: 1, py: 0.25, borderRadius: 1, border: 1, borderColor: 'divider', color: 'text.secondary', whiteSpace: 'nowrap', }}>{d}</Typography>
          ))}
          {(group.domains || []).length > 4 && (
            <Typography variant="caption" sx={{ fontSize: 10, color: 'text.secondary', fontWeight: 700, px: 0.5, opacity: 0.5 }}>+{(group.domains || []).length - 4}</Typography>
          )}
        </Box>
      </Box>
      <Box sx={{ width: 128, flexShrink: 0, display: 'flex', flexDirection: 'column', alignItems: 'flex-end', px: 1.5, borderRight: 1, borderColor: 'divider', mr: 1 }}>
        <Typography variant="caption" sx={{ fontSize: 10, fontWeight: 900, textTransform: 'uppercase', letterSpacing: '0.1em', color: modeColors[modeKey] || 'text.secondary', py: 0.25, outline: '1px solid transparent' }}>
          {getModeDisplay(effectiveMode)}
        </Typography>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, mt: 0.25, color: 'text.secondary', maxWidth: '100%', minWidth: 0 }}>
          <Box component="span" sx={{ color: 'success.main', display: 'inline-flex', flexShrink: 0 }}>
            <Activity size={10} aria-hidden />
          </Box>
          <Typography variant="caption" sx={{ fontSize: 10, fontWeight: 700, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', textTransform: 'uppercase', maxWidth: '100%', py: 0.25, outline: '1px solid transparent' }}>{getEffectiveUpstream(group)}</Typography>
        </Box>
      </Box>
      <Box sx={{ display: 'flex', gap: 0.75, flexShrink: 0 }}>
        <IconButton size="small" aria-label={t('rules.edit_rule_aria')} onClick={() => onEdit(group)} sx={{ color: 'text.secondary', '&:hover': { bgcolor: 'action.hover', color: 'primary.main' } }}>
          <Edit size={15} />
        </IconButton>
        <IconButton size="small" aria-label={t('rules.delete_rule_aria')} color="error" onClick={() => onDelete(group.id)}>
          <Trash2 size={15} />
        </IconButton>
      </Box>
    </Box>
  );
};

const Rules: React.FC = () => {
  const { t } = useTranslation();
  const [groups, setGroups] = useState<any[]>([]);
  const [search, setSearch] = useState('');
  const [filterMode, setFilterMode] = useState('ALL');
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingGroup, setEditingGroup] = useState<any>(null);
  const [pendingDeleteGroup, setPendingDeleteGroup] = useState<any>(null);

  const loadData = async () => {
    try { setGroups((await GetSiteGroups()) || []); }
    catch (e) { console.error("Failed to load site groups:", e); }
  };

  useEffect(() => { loadData(); }, []);

  const handleAdd = () => { setEditingGroup(null); setIsModalOpen(true); };
  const handleEdit = (group: any) => { setEditingGroup(group); setIsModalOpen(true); };
  const handleDelete = async (id: string) => {
    const target = groups.find((group) => group.id === id);
    setPendingDeleteGroup(target || { id });
  };
  const handleExport = async () => {
    const cfg = await ExportConfig();
    if (cfg) {
      await navigator.clipboard.writeText(cfg);
      toast.success(t('rules.copy_success'), t('rules.copy_hint'));
    }
  };
  const handleDeleteConfirm = async () => {
    if (!pendingDeleteGroup?.id) return;
    try {
      await DeleteSiteGroup(pendingDeleteGroup.id);
      setPendingDeleteGroup(null);
      await loadData();
      toast.success(t('rules.notifications.deleted'));
    } catch (err: any) {
      const detail = err && typeof err.message === 'string' && err.message ? err.message : err ? String(err) : '';
      toast.error(t('rules.notifications.delete_error'), detail || t('rules.notifications.save_error_hint'));
    }
  };

  const OTHERS_KEY = 'Others';
  const groupedResults = React.useMemo(() => {
    const filtered = groups.filter(g => {
      const matchesSearch = ((g.name || '') + (g.website || '') + (g.domains || []).join(''))
        .toLowerCase().includes(search.toLowerCase());
      const matchesMode = filterMode === 'ALL' || getEffectiveMode(g) === filterMode;
      return matchesSearch && matchesMode;
    });
    const groups_map: Record<string, any[]> = {};
    filtered.forEach(item => {
      const key = item.website || OTHERS_KEY;
      if (!groups_map[key]) groups_map[key] = [];
      groups_map[key].push(item);
    });
    return Object.keys(groups_map).sort((a, b) => {
      if (a === OTHERS_KEY) return 1;
      if (b === OTHERS_KEY) return -1;
      return a.localeCompare(b);
    }).map(key => ({ title: key === OTHERS_KEY ? t('rules.display.others') : key, items: groups_map[key] }));
  }, [groups, search, filterMode]);

  return (
    <Box sx={{ pt: 2, width: '100%', flexGrow: 1, minHeight: 0, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      <Box sx={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'space-between', alignItems: { xs: 'flex-start', sm: 'flex-end' }, gap: 2, mb: 2, flexShrink: 0 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
          <Box sx={{ p: 1.25, borderRadius: 1.5, border: 1, color: 'primary.main', bgcolor: (theme) => alpha(theme.palette.primary.main, 0.1), borderColor: (theme) => alpha(theme.palette.primary.main, 0.1), display: 'flex' }}>
            <Filter size={20} />
          </Box>
          <Typography variant="h5" sx={{ fontWeight: 700, letterSpacing: '-0.02em' }}>{t('rules.title')}</Typography>
        </Box>
        <Box sx={{ display: 'flex', gap: 1 }}>
          <Button onClick={handleExport} variant="outlined" size="small" aria-label={t('rules.export_aria')} sx={{ minWidth: 0, px: 1 }}>
            <OpenInNew size={16} />
          </Button>
          <Button onClick={handleAdd} variant="contained" color="primary" size="small" startIcon={<Plus size={16} strokeWidth={3} />}>
            {t('rules.add_rule')}
          </Button>
        </Box>
      </Box>

      <Box sx={{ flexShrink: 0, display: 'flex', flexDirection: 'column', gap: 1.5, pb: 2 }}>
        <Box sx={{ display: 'inline-flex', alignItems: 'center', gap: 0.75, p: 0.5, border: 1, borderColor: 'divider', borderRadius: 2, width: 'fit-content', flexWrap: 'wrap', minWidth: 0 }} role="tablist" aria-label={t('rules.filter_mode_aria')}>
          {FILTER_MODES.map((m) => {
            const active = filterMode === m;
            return (
              <Button
                key={m}
                onClick={() => setFilterMode(m)}
                role="tab"
                aria-selected={active}
                size="small"
                sx={{
                  px: 2, py: 0.5, borderRadius: 1.5, fontSize: 10, fontWeight: 900, letterSpacing: '0.05em',
                  textTransform: 'uppercase', minWidth: 0, color: active ? 'primary.main' : 'text.secondary',
                  bgcolor: active ? (theme) => alpha(theme.palette.primary.main, 0.12) : 'transparent',
                  boxShadow: active ? (theme) => `0 4px 12px ${alpha(theme.palette.primary.main, 0.15)}` : 'none',
                  '&:hover': { bgcolor: active ? (theme) => alpha(theme.palette.primary.main, 0.12) : 'action.hover' },
                }}
              >
                {m === 'TLS-RF' ? t('rules.display.fragment') : m === 'TRANSPARENT' ? t('rules.display.transparent') : m === 'MIGRATION' ? t('rules.display.migration') : m}
              </Button>
            );
          })}
        </Box>
        <TextField
          fullWidth
          size="small"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t('rules.search_placeholder')}
          aria-label={t('rules.search_placeholder')}
          slotProps={{ input: { startAdornment: <InputAdornment position="start"><Search size={16} /></InputAdornment> } }}
        />
      </Box>

      <Box sx={{ flexGrow: 1, minHeight: 0, overflowY: 'auto', overflowX: 'hidden', display: 'flex', flexDirection: 'column', gap: 4, pb: 4 }}>
        {groupedResults.length === 0 ? (
          <Box sx={{ py: 24, display: 'flex', flexDirection: 'column', alignItems: 'center', color: 'text.secondary', bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2, boxShadow: 1 }}>
            <Filter size={48} strokeWidth={1} />
            <Typography variant="body2" sx={{ mt: 1.5 }}>{t('rules.no_results')}</Typography>
          </Box>
        ) : (
          groupedResults.map((group) => (
            <Box key={group.title} sx={{ display: 'flex', flexDirection: 'column', gap: 1, flexShrink: 0 }}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, px: 1 }}>
                <Box sx={{ width: 6, height: 16, bgcolor: 'primary.main', borderRadius: '999px' }} aria-hidden />
                <Typography variant="caption" sx={{ fontSize: 11, fontWeight: 900, textTransform: 'uppercase', letterSpacing: '0.2em', color: 'text.secondary', display: 'flex', alignItems: 'center', gap: 1 }}>
                  {group.title}
                  <Box
                    component="span"
                    sx={{
                      fontSize: 10,
                      bgcolor: (theme) => alpha(theme.palette.text.secondary, 0.12),
                      px: 0.75,
                      py: 0.1,
                      borderRadius: 1,
                      color: 'text.secondary',
                      fontWeight: 700
                    }}
                  >
                    {group.items.length}
                  </Box>
                </Typography>
              </Box>
              <Box sx={{ border: 1, borderColor: 'divider', borderRadius: 2, overflow: 'hidden', bgcolor: 'background.paper', boxShadow: 1 }}>
                {group.items.map((item: any) => (
                  <RuleItem key={item.id} group={item} onEdit={handleEdit} onDelete={handleDelete} />
                ))}
              </Box>
            </Box>
          ))
        )}
      </Box>

      <Modal isOpen={isModalOpen} onClose={() => setIsModalOpen(false)} title={editingGroup ? t('rules.edit_rule') : t('rules.create_rule')} maxWidth="42rem"
        footer={<>
          <Button type="button" onClick={() => setIsModalOpen(false)} variant="outlined" size="small">{t('common.cancel')}</Button>
          <Button type="submit" form="rule-form" variant="contained" color="primary" size="small" startIcon={<Zap size={16} />}>{editingGroup ? t('common.save') : t('common.save')}</Button>
        </>}>
        <RuleForm initialData={editingGroup} onSuccess={() => { setIsModalOpen(false); loadData(); }} onCancel={() => setIsModalOpen(false)} />
      </Modal>

      <Modal isOpen={Boolean(pendingDeleteGroup)} onClose={() => setPendingDeleteGroup(null)} title={t('rules.delete_rule')} subtitle={t('rules.delete_hint')} maxWidth="28rem"
        footer={<>
          <Button type="button" onClick={() => setPendingDeleteGroup(null)} variant="outlined" size="small">{t('common.cancel')}</Button>
          <Button type="button" onClick={handleDeleteConfirm} variant="contained" color="error" size="small">{t('common.confirm')}</Button>
        </>}>
        <Typography variant="body2" color="text.secondary">
          {t('common.delete')}<Box component="span" sx={{ mx: 0.5, fontWeight: 'bold', color: 'text.primary' }}>{pendingDeleteGroup?.name || t('common.unknown')}</Box>{t('rules.delete_warning')}
        </Typography>
      </Modal>
    </Box>
  );
};

export default Rules;