import React, { useState, useEffect } from 'react';
import {
  Plus, Antenna, Edit3, Trash2, ChevronUp, ChevronDown,
  Zap, CheckCircle2, Loader2, Shield, Globe, AlertCircle
} from '../lib/icons';
import {
  GetDNSNodes, AddDNSNode, UpdateDNSNode, DeleteDNSNode,
  SetDNSNodePriority, TestDNSNode
} from '../api/bindings';
import Modal from '../components/Modal';
import { toast } from '../lib/toast';
import { splitListInput, joinListInput } from '../lib/utils';
import { useTranslation } from '../i18n/I18nContext';
import {
  Box, Typography, Button, IconButton, TextField, Switch,
  useColorScheme,
} from '@mui/material';

interface CertVerifyConfig {
  mode: string; names: string[]; suffixes: string[]; spki_sha256: string[]; allow_unknown_authority: boolean;
}
interface DNSNode {
  id: string; name: string; url: string; sni?: string; ips?: string[];
  ech_enabled: boolean; ech_profile_id?: string; quic: boolean;
  cert_verify: CertVerifyConfig; enabled: boolean;
}

const createDefaultCertVerify = () => ({ mode: '', names: [], suffixes: [], spki_sha256: [], allow_unknown_authority: false });
const defaultNode: Partial<DNSNode> = { name: '', url: '', sni: '', ips: [], ech_enabled: false, ech_profile_id: '', quic: false, cert_verify: createDefaultCertVerify(), enabled: true };

const DNSNodeItem: React.FC<{
  node: DNSNode; index: number; total: number;
  onEdit: (node: DNSNode) => void; onDelete: (id: string) => void;
  onMoveUp: (id: string, idx: number) => void; onMoveDown: (id: string, idx: number) => void;
  onTest: (id: string) => void; testResult: any; isTesting: boolean;
}> = ({ node, index, total, onEdit, onDelete, onMoveUp, onMoveDown, onTest, testResult, isTesting }) => {
  const { t } = useTranslation();
  const { mode } = useColorScheme();
  const hoverBg = mode === 'light' ? 'rgba(0,0,0,0.04)' : 'rgba(255,255,255,0.08)';

  const tags: { label: string; color: string; bg: string; border: string }[] = [];
  if (node.ech_enabled) tags.push({ label: 'ECH', color: 'cyan.main', bg: 'rgba(6,182,212,0.1)', border: 'rgba(6,182,212,0.2)' });
  if (node.quic) tags.push({ label: 'QUIC', color: 'secondary.main', bg: 'rgba(168,85,247,0.1)', border: 'rgba(168,85,247,0.2)' });
  if (node.sni) tags.push({ label: `SNI: ${node.sni}`, color: 'primary.main', bg: 'rgba(11,123,255,0.1)', border: 'rgba(11,123,255,0.2)' });

  const vMode = node.cert_verify?.mode;
  if (vMode) {
    const modeLabels: Record<string, string> = {
      '': t('dns.modes.default'), 'strict_real': t('dns.modes.strict'),
      'allow_names': t('dns.modes.names'), 'allow_suffixes': t('dns.modes.suffixes'),
      'allow_spki': t('dns.modes.spki'), 'chain_only': t('dns.modes.chain')
    };
    tags.push({ label: `${t('dns.verify_mode')}: ${modeLabels[vMode] || vMode}`, color: 'warning.main', bg: 'rgba(210,153,34,0.1)', border: 'rgba(210,153,34,0.2)' });
  }
  if (node.cert_verify?.allow_unknown_authority) tags.push({ label: t('dns.allow_unknown'), color: 'error.main', bg: 'rgba(239,68,68,0.1)', border: 'rgba(239,68,68,0.2)' });

  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, py: 1.5, px: 2, borderBottom: 1, borderColor: 'divider', transition: 'background-color 0.2s', '&:hover': { bgcolor: hoverBg } }}>
      <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 0.25, flexShrink: 0 }}>
        <IconButton size="small" aria-label={t('dns.move_up')} onClick={() => onMoveUp(node.id, index)} disabled={index === 0} sx={{ color: 'text.secondary', '&:hover': { color: 'primary.main' }, '&.Mui-disabled': { opacity: 0.2 } }}>
          <ChevronUp size={14} />
        </IconButton>
        <Typography variant="caption" sx={{ fontWeight: 900, color: 'text.secondary', width: 20, textAlign: 'center' }}>{index + 1}</Typography>
        <IconButton size="small" aria-label={t('dns.move_down')} onClick={() => onMoveDown(node.id, index)} disabled={index === total - 1} sx={{ color: 'text.secondary', '&:hover': { color: 'primary.main' }, '&.Mui-disabled': { opacity: 0.2 } }}>
          <ChevronDown size={14} />
        </IconButton>
      </Box>

      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Box sx={{ width: 8, height: 8, borderRadius: '50%', flexShrink: 0, bgcolor: node.enabled ? 'success.main' : 'text.disabled', boxShadow: node.enabled ? '0 0 6px rgba(34,197,94,0.4)' : 'none' }} />
          <Typography variant="body2" sx={{ fontWeight: 'bold', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{node.name || t('common.unknown')}</Typography>
        </Box>
        <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'mono', display: 'block', mt: 0.5, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{node.url}</Typography>
        {tags.length > 0 && (
          <Box sx={{ display: 'flex', gap: 0.75, mt: 0.75, flexWrap: 'wrap' }}>
            {tags.map((tag, i) => (
              <Typography key={i} variant="caption" sx={{ fontSize: 9, fontWeight: 700, px: 1, py: 0.25, borderRadius: '999px', border: 1, color: tag.color, bgcolor: tag.bg, borderColor: tag.border, lineHeight: 1.6 }}>
                {tag.label}
              </Typography>
            ))}
          </Box>
        )}
        {node.ips && node.ips.length > 0 && (
          <Box sx={{ display: 'flex', gap: 0.75, mt: 0.5, flexWrap: 'wrap' }}>
            {node.ips.map((ip, i) => (
              <Typography key={i} variant="caption" sx={{ fontSize: 9, fontFamily: 'mono', bgcolor: hoverBg, px: 1, py: 0.25, borderRadius: 1, border: 1, borderColor: 'divider', color: 'text.secondary' }}>
                {ip}
              </Typography>
            ))}
          </Box>
        )}
      </Box>

      <Box sx={{ flexShrink: 0, width: 112, display: 'flex', justifyContent: 'flex-end' }}>
        {isTesting ? (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, color: 'primary.main' }}>
            <Loader2 size={14} />
            <Typography variant="caption" sx={{ fontWeight: 'bold' }}>{t('dns.test')}...</Typography>
          </Box>
        ) : testResult ? (
          testResult.success ? (
            <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: 0.25 }}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, color: 'success.main' }}>
                <CheckCircle2 size={12} />
                <Typography variant="caption" sx={{ fontWeight: 900 }}>{testResult.latency}</Typography>
              </Box>
              <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'mono', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 112 }}>{testResult.ips?.[0]}</Typography>
            </Box>
          ) : (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, color: 'error.main' }} title={testResult.error}>
              <AlertCircle size={12} />
              <Typography variant="caption" sx={{ fontWeight: 'bold' }}>{testResult.error || t('common.failed')}</Typography>
            </Box>
          )
        ) : null}
      </Box>

      <Box sx={{ display: 'flex', gap: 0.5, flexShrink: 0 }}>
        <Button size="small" variant="text" onClick={() => onTest(node.id)} disabled={isTesting}>{t('dns.test')}</Button>
        <IconButton size="small" aria-label={t('common.edit')} onClick={() => onEdit(node)} sx={{ color: 'text.secondary', '&:hover': { bgcolor: hoverBg, color: 'primary.main' } }}>
          <Edit3 size={14} />
        </IconButton>
        <IconButton size="small" aria-label={t('common.delete')} color="error" onClick={() => onDelete(node.id)}>
          <Trash2 size={14} />
        </IconButton>
      </Box>
    </Box>
  );
};

const DNSNodeForm: React.FC<{ initialData?: DNSNode | null; onSubmit: (data: any) => void }> = ({ initialData, onSubmit }) => {
  const { t } = useTranslation();
  const [form, setForm] = useState<any>({ ...defaultNode, ...initialData });
  const [ipInput, setIpInput] = useState((initialData?.ips || []).join('\n'));
  const { mode } = useColorScheme();
  const hoverBg = mode === 'light' ? 'rgba(0,0,0,0.04)' : 'rgba(255,255,255,0.08)';

  const CERT_VERIFY_MODES = [
    { id: '', label: t('dns.modes.default'), desc: t('dns.mode_descs.default') },
    { id: 'strict_real', label: t('dns.modes.strict'), desc: t('dns.mode_descs.strict') },
    { id: 'allow_names', label: t('dns.modes.names'), desc: t('dns.mode_descs.names') },
    { id: 'allow_suffixes', label: t('dns.modes.suffixes'), desc: t('dns.mode_descs.suffixes') },
    { id: 'allow_spki', label: t('dns.modes.spki'), desc: t('dns.mode_descs.spki') },
    { id: 'chain_only', label: t('dns.modes.chain'), desc: t('dns.mode_descs.chain') }
  ];

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const ips = splitListInput(ipInput);
    onSubmit({ ...form, ips });
  };

  const inputSx = {
    '& .MuiOutlinedInput-root': {
      bgcolor: hoverBg,
      '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: 'primary.main' },
    },
  };

  return (
    <form id="dns-form" onSubmit={handleSubmit}>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, gap: 3 }}>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            <Typography variant="caption" sx={{ fontWeight: 900, color: 'text.secondary', textTransform: 'uppercase', letterSpacing: '0.1em' }}>
              <Box component="span" sx={{ display: 'inline-flex', alignItems: 'center', gap: 0.5, mr: 0.5 }}>
                <Antenna size={10} color="primary.main" />
              </Box>
              {t('dns.node_name')}
            </Typography>
            <TextField id="dns-name" type="text" required size="small" value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} sx={inputSx} />
          </Box>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            <Typography variant="caption" sx={{ fontWeight: 900, color: 'text.secondary', textTransform: 'uppercase', letterSpacing: '0.1em' }}>
              <Box component="span" sx={{ display: 'inline-flex', alignItems: 'center', gap: 0.5, mr: 0.5 }}>
                <Globe size={10} color="primary.main" />
              </Box>
              {t('dns.doh_url')}
            </Typography>
            <TextField id="dns-url" type="text" required size="small" value={form.url} onChange={e => setForm({ ...form, url: e.target.value })} sx={{ ...inputSx, '& input': { fontFamily: 'mono' } }} />
          </Box>
        </Box>

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, gap: 3 }}>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            <Typography variant="caption" sx={{ fontWeight: 900, color: 'text.secondary', textTransform: 'uppercase', letterSpacing: '0.1em' }}>{t('dns.sni_fake')}</Typography>
            <TextField type="text" size="small" value={form.sni || ''} onChange={e => setForm({ ...form, sni: e.target.value })} sx={inputSx} />
          </Box>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            <Typography variant="caption" sx={{ fontWeight: 900, color: 'text.secondary', textTransform: 'uppercase', letterSpacing: '0.1em' }}>{t('dns.bootstrap_ips')}</Typography>
            <TextField multiline rows={2} size="small" value={ipInput} onChange={e => setIpInput(e.target.value)} sx={{ ...inputSx, '& textarea': { fontFamily: 'mono' } }} />
          </Box>
        </Box>

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(3, 1fr)' }, gap: 1.5, p: 2, bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2 }}>
          {[
            { label: t('common.enabled'), field: 'enabled' },
            { label: 'ECH', field: 'ech_enabled' },
            { label: 'QUIC', field: 'quic' },
          ].map(({ label, field }) => (
            <Box key={field} sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderRadius: 2, border: 1, borderColor: 'divider', px: 2, py: 1.5, cursor: 'pointer', '&:hover': { borderColor: 'primary.main' }, transition: 'all 0.2s' }}>
              <Typography variant="caption" sx={{ fontWeight: 'bold' }}>{label}</Typography>
              <Switch size="small" checked={Boolean(form[field])} onChange={(e) => setForm({ ...form, [field]: e.target.checked })} />
            </Box>
          ))}
        </Box>

        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3, pt: 1 }}>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, p: 2, border: 1, borderColor: 'warning.main', bgcolor: 'background.paper', borderRadius: 2, position: 'relative' }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, color: 'warning.main', mb: 1 }}>
              <AlertCircle size={16} />
              <Typography variant="caption" sx={{ fontWeight: 'bold', textTransform: 'uppercase', letterSpacing: '0.05em' }}>{t('dns.cert_policy')}</Typography>
            </Box>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
              <Typography variant="caption" sx={{ fontWeight: 'bold', color: 'text.secondary' }}>{t('dns.verify_mode')}</Typography>
              <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(3, 1fr)' }, gap: 1 }}>
                {CERT_VERIFY_MODES.map((cm) => {
                  const active = (form.cert_verify?.mode || '') === cm.id;
                  return (
                    <Button key={cm.id || 'default'} type="button" size="small"
                      onClick={() => setForm({ ...form, cert_verify: { ...form.cert_verify, mode: cm.id } })}
                      sx={{
                        justifyContent: 'flex-start',
                        textAlign: 'left',
                        borderRadius: 1,
                        border: 1,
                        px: 1.5,
                        py: 1,
                        transition: 'all 0.2s',
                        bgcolor: active ? 'rgba(210,153,34,0.1)' : hoverBg,
                        borderColor: active ? 'warning.main' : 'divider',
                        color: active ? 'warning.main' : 'text.secondary',
                        '&:hover': { borderColor: 'warning.main', color: 'text.primary', bgcolor: active ? 'rgba(210,153,34,0.1)' : hoverBg },
                      }}
                    >
                      <Typography variant="caption" sx={{ fontWeight: 900, letterSpacing: '0.05em' }}>{cm.label}</Typography>
                    </Button>
                  );
                })}
              </Box>
            </Box>

            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderRadius: 2, border: 1, px: 2, py: 1.5, transition: 'all 0.2s', cursor: 'pointer', bgcolor: form.cert_verify?.allow_unknown_authority ? 'rgba(210,153,34,0.1)' : hoverBg, borderColor: form.cert_verify?.allow_unknown_authority ? 'warning.main' : 'divider', '&:hover': { borderColor: 'warning.main' } }}>
              <Typography variant="caption" sx={{ fontWeight: 'bold' }}>{t('dns.allow_unknown')}</Typography>
              <Switch size="small" checked={Boolean(form.cert_verify?.allow_unknown_authority)} onChange={(e) => setForm({ ...form, cert_verify: { ...form.cert_verify, allow_unknown_authority: e.target.checked } })} color="warning" />
            </Box>

            {(form.cert_verify?.mode === 'allow_names') && (
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                <Typography variant="caption" sx={{ fontWeight: 'bold', color: 'text.secondary' }}>{t('dns.allow_names')}</Typography>
                <TextField multiline rows={3} size="small" value={joinListInput(form.cert_verify?.names)}
                  onChange={(e) => setForm({ ...form, cert_verify: { ...form.cert_verify, names: splitListInput(e.target.value) } })}
                  sx={inputSx} />
              </Box>
            )}

            {(form.cert_verify?.mode === 'allow_suffixes') && (
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                <Typography variant="caption" sx={{ fontWeight: 'bold', color: 'text.secondary' }}>{t('dns.allow_suffixes')}</Typography>
                <TextField multiline rows={3} size="small" value={joinListInput(form.cert_verify?.suffixes)}
                  onChange={(e) => setForm({ ...form, cert_verify: { ...form.cert_verify, suffixes: splitListInput(e.target.value) } })}
                  sx={inputSx} />
              </Box>
            )}
          </Box>
        </Box>
      </Box>
    </form>
  );
};

const DNS: React.FC = () => {
  const { t } = useTranslation();
  const [nodes, setNodes] = useState<DNSNode[]>([]);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingNode, setEditingNode] = useState<DNSNode | null>(null);
  const [pendingDeleteNode, setPendingDeleteNode] = useState<DNSNode | null>(null);
  const [testResults, setTestResults] = useState<Record<string, any>>({});
  const [testingIds, setTestingIds] = useState<Set<string>>(new Set());

  const loadData = async () => { setNodes((await GetDNSNodes()) || []); };
  useEffect(() => { loadData(); }, []);

  const handleAdd = () => { setEditingNode(null); setIsModalOpen(true); };
  const handleEdit = (node: DNSNode) => { setEditingNode(node); setIsModalOpen(true); };
  const handleFormSubmit = async (data: any) => {
    if (editingNode?.id) await UpdateDNSNode({ ...data, id: editingNode.id });
    else await AddDNSNode(data);
    setIsModalOpen(false);
    await loadData();
    toast.success(editingNode ? t('dns.notifications.updated') : t('dns.notifications.added'));
  };
  const handleDelete = async () => {
    if (!pendingDeleteNode?.id) return;
    await DeleteDNSNode(pendingDeleteNode.id);
    setPendingDeleteNode(null);
    await loadData();
    toast.success(t('dns.notifications.deleted'));
  };
  const handleMoveUp = async (id: string, idx: number) => { if (idx <= 0) return; await SetDNSNodePriority(id, idx - 1); await loadData(); };
  const handleMoveDown = async (id: string, idx: number) => { if (idx >= nodes.length - 1) return; await SetDNSNodePriority(id, idx + 1); await loadData(); };
  const handleTest = async (id: string) => {
    setTestingIds(prev => new Set(prev).add(id));
    try { const result = await TestDNSNode(id); setTestResults(prev => ({ ...prev, [id]: result })); }
    catch (err: any) { setTestResults(prev => ({ ...prev, [id]: { success: false, error: String(err) } })); }
    finally { setTestingIds(prev => { const s = new Set(prev); s.delete(id); return s; }); }
  };
  const handleTestAll = async () => { for (const node of nodes) { void handleTest(node.id); } };

  return (
    <Box sx={{ px: 6, pt: 4, pb: 6, maxWidth: '5xl', mx: 'auto' }}>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', bgcolor: 'background.paper', border: 1, borderColor: 'divider', p: 2.5, borderRadius: 2 }}>
        <Typography variant="h4" sx={{ fontWeight: 900, letterSpacing: '-0.02em' }}>{t('dns.title')}</Typography>
        <Box sx={{ display: 'flex', gap: 1 }}>
          <Button onClick={handleTestAll} variant="outlined" size="small" startIcon={<Zap size={14} />}>{t('dns.test_all')}</Button>
          <Button onClick={handleAdd} variant="contained" color="primary" size="small" startIcon={<Plus size={16} strokeWidth={3} />}>{t('dns.add_node')}</Button>
        </Box>
      </Box>

      <Box sx={{ border: 1, borderColor: 'divider', borderRadius: 2, overflow: 'hidden', bgcolor: 'background.paper', mt: 4 }}>
        {nodes.length === 0 ? (
          <Box sx={{ py: 12, color: 'text.secondary', opacity: 0.5, display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
            <Antenna size={48} strokeWidth={1} />
            <Typography variant="body2" sx={{ mt: 1.5 }}>{t('dns.no_nodes')}</Typography>
          </Box>
        ) : (
          nodes.map((node, idx) => (
            <DNSNodeItem key={node.id} node={node} index={idx} total={nodes.length}
              onEdit={handleEdit} onDelete={(id) => setPendingDeleteNode(nodes.find(n => n.id === id) || null)}
              onMoveUp={handleMoveUp} onMoveDown={handleMoveDown} onTest={handleTest}
              testResult={testResults[node.id]} isTesting={testingIds.has(node.id)} />
          ))
        )}
      </Box>

      <Modal isOpen={isModalOpen} onClose={() => setIsModalOpen(false)} title={editingNode ? t('dns.edit_node') : t('dns.add_node')} maxWidth="2xl"
        footer={<>
          <Button type="button" onClick={() => setIsModalOpen(false)} variant="outlined" size="small">{t('common.cancel')}</Button>
          <Button type="submit" form="dns-form" variant="contained" color="primary" size="small" startIcon={<Shield size={16} />}>
            {editingNode ? t('dns.edit_node') : t('dns.add_node')}
          </Button>
        </>}>
        <DNSNodeForm initialData={editingNode} onSubmit={handleFormSubmit} />
      </Modal>

      <Modal isOpen={Boolean(pendingDeleteNode)} onClose={() => setPendingDeleteNode(null)} title={t('dns.delete_node')} subtitle={t('dns.delete_hint')} maxWidth="md"
        footer={<>
          <Button type="button" onClick={() => setPendingDeleteNode(null)} variant="outlined" size="small">{t('common.cancel')}</Button>
          <Button type="button" onClick={handleDelete} variant="contained" color="error" size="small">{t('common.confirm')}</Button>
        </>}>
        <Typography variant="body2" color="text.secondary">
          {t('common.delete')} <Box component="span" sx={{ mx: 0.5, fontWeight: 'bold', color: 'text.primary' }}>{pendingDeleteNode?.name || t('common.unknown')}</Box>{t('dns.delete_warning')}
        </Typography>
      </Modal>
    </Box>
  );
};

export default DNS;
