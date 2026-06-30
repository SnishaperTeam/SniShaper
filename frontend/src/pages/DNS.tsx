import React, { useState, useEffect } from 'react';
import {
  Plus, Antenna, Edit3, Trash2, ChevronUp, ChevronDown,
  Zap, CheckCircle2, Loader2, Shield, Globe, AlertCircle
} from '../lib/icons';
import {
  GetDNSNodes, AddDNSNode, UpdateDNSNode, DeleteDNSNode,
  SetDNSNodePriority, TestDNSNode
} from '../api/bindings';
import { Button } from '../components/ui/Button';
import { Toggle } from '../components/ui/Toggle';
import { EmptyState } from '../components/ui/EmptyState';
import Modal from '../components/Modal';
import { toast } from '../lib/toast';
import { cn, splitListInput, joinListInput } from '../lib/utils';
import { useTranslation } from '../i18n/I18nContext';

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
  const tags: { label: string; color: string }[] = [];
  if (node.ech_enabled) tags.push({ label: 'ECH', color: 'text-cyan-500 bg-cyan-500/10 border-cyan-500/20' });
  if (node.quic) tags.push({ label: 'QUIC', color: 'text-purple-500 bg-purple-500/10 border-purple-500/20' });
  if (node.sni) tags.push({ label: `SNI: ${node.sni}`, color: 'text-accent bg-accent/10 border-accent/20' });

  const vMode = node.cert_verify?.mode;
  if (vMode) {
    const modeLabels: Record<string, string> = {
      '': t('dns.modes.default'), 'strict_real': t('dns.modes.strict'),
      'allow_names': t('dns.modes.names'), 'allow_suffixes': t('dns.modes.suffixes'),
      'allow_spki': t('dns.modes.spki'), 'chain_only': t('dns.modes.chain')
    };
    tags.push({ label: `${t('dns.verify_mode')}: ${modeLabels[vMode] || vMode}`, color: 'text-warning bg-warning/10 border-warning/20' });
  }
  if (node.cert_verify?.allow_unknown_authority) tags.push({ label: t('dns.allow_unknown'), color: 'text-danger bg-danger/10 border-danger/20' });

  return (
    <div className="group flex items-center gap-4 py-4 px-6 bg-background-card hover:bg-background-hover border-b border-border/60 transition-colors">
      <div className="flex flex-col items-center gap-0.5 shrink-0">
        <button onClick={() => onMoveUp(node.id, index)} disabled={index === 0} className="p-0.5 rounded text-text-muted hover:text-accent disabled:opacity-20 disabled:cursor-default transition-colors" aria-label="上移">
          <ChevronUp size={14} />
        </button>
        <span className="text-[10px] font-black text-text-muted w-5 text-center">{index + 1}</span>
        <button onClick={() => onMoveDown(node.id, index)} disabled={index === total - 1} className="p-0.5 rounded text-text-muted hover:text-accent disabled:opacity-20 disabled:cursor-default transition-colors" aria-label="下移">
          <ChevronDown size={14} />
        </button>
      </div>

      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <div className={cn('w-2 h-2 rounded-full shrink-0', node.enabled ? 'bg-success shadow-[0_0_6px_rgba(34,197,94,0.4)]' : 'bg-text-muted/30')} />
          <h3 className="text-sm font-bold text-text-primary truncate">{node.name || t('common.unknown')}</h3>
        </div>
        <p className="text-[11px] text-text-muted font-mono mt-0.5 truncate">{node.url}</p>
        {tags.length > 0 && (
          <div className="flex gap-1.5 mt-1.5 flex-wrap">
            {tags.map((tag, i) => (
              <span key={i} className={`text-[9px] font-bold px-2 py-0.5 rounded-full border ${tag.color}`}>{tag.label}</span>
            ))}
          </div>
        )}
        {node.ips && node.ips.length > 0 && (
          <div className="flex gap-1.5 mt-1 flex-wrap">
            {node.ips.map((ip, i) => (
              <span key={i} className="text-[9px] font-mono bg-background-hover px-2 py-0.5 rounded border border-border/40 text-text-secondary">{ip}</span>
            ))}
          </div>
        )}
      </div>

      <div className="shrink-0 w-28 text-right">
        {isTesting ? (
          <div className="flex items-center justify-end gap-1.5 text-accent">
            <Loader2 size={14} className="animate-spin" />
            <span className="text-[10px] font-bold">{t('dns.test')}...</span>
          </div>
        ) : testResult ? (
          testResult.success ? (
            <div className="space-y-0.5">
              <div className="flex items-center justify-end gap-1 text-success">
                <CheckCircle2 size={12} />
                <span className="text-[10px] font-black">{testResult.latency}</span>
              </div>
              <div className="text-[9px] text-text-muted font-mono truncate">{testResult.ips?.[0]}</div>
            </div>
          ) : (
            <div className="flex items-center justify-end gap-1 text-danger" title={testResult.error}>
              <AlertCircle size={12} />
              <span className="text-[10px] font-bold">{testResult.error || t('common.failed')}</span>
            </div>
          )
        ) : null}
      </div>

      <div className="flex gap-1 shrink-0">
        <Button onClick={() => onTest(node.id)} loading={isTesting} variant="ghost" size="xs">{t('dns.test')}</Button>
        <button onClick={() => onEdit(node)} className="p-1.5 hover:bg-background-hover rounded text-text-secondary hover:text-accent transition-colors" aria-label="编辑">
          <Edit3 size={14} />
        </button>
        <button onClick={() => onDelete(node.id)} className="p-1.5 hover:bg-danger/10 rounded text-danger transition-colors" aria-label="删除">
          <Trash2 size={14} />
        </button>
      </div>
    </div>
  );
};

const DNSNodeForm: React.FC<{ initialData?: DNSNode | null; onSubmit: (data: any) => void }> = ({ initialData, onSubmit }) => {
  const { t } = useTranslation();
  const [form, setForm] = useState<any>({ ...defaultNode, ...initialData });
  const [ipInput, setIpInput] = useState((initialData?.ips || []).join('\n'));

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

  return (
    <form id="dns-form" onSubmit={handleSubmit} className="space-y-4 text-text-primary px-1 pb-2">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="space-y-1.5">
          <label className="text-[10px] font-black text-text-secondary uppercase tracking-widest px-1 flex items-center gap-1.5" htmlFor="dns-name">
            <Antenna size={10} className="text-accent" aria-hidden /> {t('dns.node_name')}
          </label>
          <input id="dns-name" type="text" required value={form.name} onChange={e => setForm({ ...form, name: e.target.value })}
            className="w-full bg-background-hover border border-border px-4 py-2 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none font-medium transition-all" />
        </div>
        <div className="space-y-1.5">
          <label className="text-[10px] font-black text-text-secondary uppercase tracking-widest px-1 flex items-center gap-1.5" htmlFor="dns-url">
            <Globe size={10} className="text-accent" aria-hidden /> {t('dns.doh_url')}
          </label>
          <input id="dns-url" type="text" required value={form.url} onChange={e => setForm({ ...form, url: e.target.value })}
            className="w-full bg-background-hover border border-border px-4 py-2 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none font-medium font-mono transition-all" />
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="space-y-1.5">
          <label className="text-[10px] font-black text-text-secondary uppercase tracking-widest px-1">{t('dns.sni_fake')}</label>
          <input type="text" value={form.sni || ''} onChange={e => setForm({ ...form, sni: e.target.value })}
            className="w-full bg-background-hover border border-border px-4 py-2 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none font-medium transition-all" />
        </div>
        <div className="space-y-1.5">
          <label className="text-[10px] font-black text-text-secondary uppercase tracking-widest px-1">{t('dns.bootstrap_ips')}</label>
          <textarea rows={2} value={ipInput} onChange={e => setIpInput(e.target.value)}
            className="w-full resize-none bg-background-hover border border-border px-4 py-2 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none font-medium font-mono transition-all" />
        </div>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-3 gap-3 p-4 bg-background-card border border-border rounded-2xl">
        {[
          { label: t('common.enabled'), field: 'enabled' },
          { label: 'ECH', field: 'ech_enabled' },
          { label: 'QUIC', field: 'quic' },
        ].map(({ label, field }) => (
          <label key={field} className="flex items-center justify-between rounded-2xl border border-border/40 px-4 py-3 cursor-pointer hover:border-accent/25 transition-all">
            <span className="text-[11px] font-bold text-text-primary">{label}</span>
            <Toggle checked={form[field]} onChange={(v) => setForm({ ...form, [field]: v })} />
          </label>
        ))}
      </div>

      <div className="space-y-6 pt-2 animate-in slide-in-from-top-2 fade-in duration-300">
        <div className="space-y-3 p-4 border border-warning/30 bg-background-card rounded-2xl relative">
          <div className="flex items-center gap-2 text-warning mb-2">
            <AlertCircle size={16} aria-hidden />
            <span className="text-xs font-bold uppercase tracking-wider">{t('dns.cert_policy')}</span>
          </div>
          <div className="space-y-2">
            <label className="text-[9px] font-bold text-text-muted">{t('dns.verify_mode')}</label>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-2">
              {CERT_VERIFY_MODES.map((mode) => {
                const active = (form.cert_verify?.mode || '') === mode.id;
                return (
                  <button key={mode.id || 'default'} type="button"
                    onClick={() => setForm({ ...form, cert_verify: { ...form.cert_verify, mode: mode.id } })}
                    className={cn('rounded-xl border px-3 py-3 text-left transition-all', active
                      ? 'border-warning/50 bg-warning/10 text-warning shadow-[inset_0_0_0_1px_rgba(210,153,34,0.14)]'
                      : 'border-border bg-background-hover/60 text-text-secondary hover:border-warning/30 hover:text-text-primary')}>
                    <div className="text-[11px] font-black tracking-wide">{mode.label}</div>
                  </button>
                );
              })}
            </div>
          </div>

          <label className={cn('flex w-full items-center justify-between rounded-2xl border px-4 py-3 transition-all cursor-pointer',
            form.cert_verify?.allow_unknown_authority ? 'border-warning/40 bg-warning/10' : 'border-border bg-background-hover/60 hover:border-warning/25')}>
            <div className="space-y-0.5 text-left">
              <div className="text-[11px] font-bold text-text-primary">{t('dns.allow_unknown')}</div>
            </div>
            <Toggle checked={form.cert_verify?.allow_unknown_authority} onChange={(v) => setForm({ ...form, cert_verify: { ...form.cert_verify, allow_unknown_authority: v } })} />
          </label>

          {(form.cert_verify?.mode === 'allow_names') && (
            <div className="space-y-1.5">
              <label className="text-[9px] font-bold text-text-muted">{t('dns.allow_names')}</label>
              <textarea rows={3} value={joinListInput(form.cert_verify?.names)}
                onChange={(e) => setForm({ ...form, cert_verify: { ...form.cert_verify, names: splitListInput(e.target.value) } })}
                className="w-full resize-none bg-background-card border border-border px-3 py-2 rounded-xl text-[11px] leading-relaxed outline-none focus:ring-2 focus:ring-warning" />
            </div>
          )}

          {(form.cert_verify?.mode === 'allow_suffixes') && (
            <div className="space-y-1.5">
              <label className="text-[9px] font-bold text-text-muted">{t('dns.allow_suffixes')}</label>
              <textarea rows={3} value={joinListInput(form.cert_verify?.suffixes)}
                onChange={(e) => setForm({ ...form, cert_verify: { ...form.cert_verify, suffixes: splitListInput(e.target.value) } })}
                className="w-full resize-none bg-background-card border border-border px-3 py-2 rounded-xl text-[11px] leading-relaxed outline-none focus:ring-2 focus:ring-warning" />
            </div>
          )}
        </div>
      </div>
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
    <div className="p-5 max-w-5xl mx-auto space-y-4 animate-in fade-in duration-500">
      <header className="flex justify-between items-center bg-background border border-border p-5 rounded-2xl shadow-sm">
        <h1 className="text-xl font-black tracking-tight">{t('dns.title')}</h1>
        <div className="flex gap-2">
          <Button onClick={handleTestAll} variant="outline" size="md" icon={<Zap size={14} />}>{t('dns.test_all')}</Button>
          <Button onClick={handleAdd} variant="primary" size="md" icon={<Plus size={16} strokeWidth={3} />}>{t('dns.add_node')}</Button>
        </div>
      </header>

      <div className="border border-border rounded-2xl overflow-hidden bg-background-card shadow-sm">
        {nodes.length === 0 ? (
          <EmptyState icon={<Antenna size={48} strokeWidth={1} />} title={t('dns.no_nodes')} className="py-24 grayscale" />
        ) : (
          nodes.map((node, idx) => (
            <DNSNodeItem key={node.id} node={node} index={idx} total={nodes.length}
              onEdit={handleEdit} onDelete={(id) => setPendingDeleteNode(nodes.find(n => n.id === id) || null)}
              onMoveUp={handleMoveUp} onMoveDown={handleMoveDown} onTest={handleTest}
              testResult={testResults[node.id]} isTesting={testingIds.has(node.id)} />
          ))
        )}
      </div>

      <Modal isOpen={isModalOpen} onClose={() => setIsModalOpen(false)} title={editingNode ? t('dns.edit_node') : t('dns.add_node')} maxWidth="max-w-2xl"
        footer={<>
          <Button type="button" onClick={() => setIsModalOpen(false)} variant="outline" size="md">{t('common.cancel')}</Button>
          <Button type="submit" form="dns-form" variant="primary" size="md" icon={<Shield size={16} />}>
            {editingNode ? t('dns.edit_node') : t('dns.add_node')}
          </Button>
        </>}>
        <DNSNodeForm initialData={editingNode} onSubmit={handleFormSubmit} />
      </Modal>

      <Modal isOpen={Boolean(pendingDeleteNode)} onClose={() => setPendingDeleteNode(null)} title={t('dns.delete_node')} subtitle={t('dns.delete_hint')} maxWidth="max-w-md"
        footer={<>
          <Button type="button" onClick={() => setPendingDeleteNode(null)} variant="outline" size="md">{t('common.cancel')}</Button>
          <Button type="button" onClick={handleDelete} variant="danger" size="md">{t('common.confirm')}</Button>
        </>}>
        <div className="text-sm text-text-secondary leading-relaxed">
          {t('common.delete')} <span className="mx-1 font-bold text-text-primary">{pendingDeleteNode?.name || t('common.unknown')}</span>{t('dns.delete_warning')}
        </div>
      </Modal>
    </div>
  );
};

export default DNS;
