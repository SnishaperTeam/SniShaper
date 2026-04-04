import React, { useState, useEffect } from 'react';
import { 
  Plus, 
  Search, 
  Filter, 
  ExternalLink, 
  Edit3, 
  Trash2, 
  Zap, 
  Globe, 
  Activity,
  ShieldCheck
} from 'lucide-react';
import { 
  GetSiteGroups, 
  DeleteSiteGroup,
  ExportConfig
} from '../api/bindings';
import Modal from '../components/Modal';
import RuleForm from '../components/RuleForm';
import { toast } from '../lib/toast';

const FILTER_MODES = ['ALL', 'MITM', 'SERVER', 'TLS-RF', 'QUIC', 'WARP', 'TRANSPARENT'] as const;

const normalizeMode = (value: unknown) => String(value || '').trim().toLowerCase();

const getEffectiveMode = (group: any) => {
  const mode = normalizeMode(group?.mode);
  const upstream = normalizeMode(group?.upstream);

  if (mode === 'warp' || upstream === 'warp') return 'WARP';
  if (mode === 'quic') return 'QUIC';
  if (mode === 'tls-rf') return 'TLS-RF';
  if (mode === 'server') return 'SERVER';
  if (mode === 'mitm') return 'MITM';
  if (mode === 'transparent') return 'TRANSPARENT';
  return mode ? mode.toUpperCase() : 'DIRECT';
};

const getModeDisplay = (mode: string) => {
  switch (mode) {
    case 'TRANSPARENT':
      return '透传';
    case 'DIRECT':
      return '直连';
    default:
      return mode;
  }
};

const RuleItem: React.FC<{
  group: any;
  onEdit: (group: any) => void;
  onDelete: (id: string) => void;
}> = ({ group, onEdit, onDelete }) => {
  const modeColors: Record<string, string> = {
    'server': 'text-purple-600 dark:text-purple-400',
    'mitm': 'text-amber-600 dark:text-amber-400',
    'transparent': 'text-danger',
    'quic': 'text-success',
    'tls-rf': 'text-blue-600 dark:text-blue-400',
    'warp': 'text-accent'
  };

  const getEffectiveUpstream = (group: any) => {
    const upstream = String(group.upstream || '').trim();
    if (upstream && upstream.toUpperCase() !== 'DIRECT') return upstream;
    
    // Fallback labels based on specialized logic
    if (group.mode === 'warp') return 'WARP TUNNEL';
    if (group.use_cf_pool) return 'CF IP POOL';
    if (group.mode === 'server') return 'REMOTE SERVER';
    return 'DOH 动态解析';
  };

  const effectiveMode = getEffectiveMode(group);
  const modeKey = normalizeMode(group?.mode) || normalizeMode(effectiveMode);

  return (
    <div className="group flex items-center gap-4 py-3 px-6 bg-background-card hover:bg-background-hover border-b border-border/60 transition-colors">
      <div className="flex-1 min-w-0 flex items-center gap-6">
        <div className="w-1/3 min-w-0">
            <h3 className="text-sm font-bold text-text-primary truncate">{group.name || '未命名规则'}</h3>
            <p className="text-[10px] text-text-muted font-medium truncate uppercase tracking-wider">{group.website || 'Default'}</p>
        </div>
        
        <div className="flex-1 min-w-0 hidden md:block">
            <div className="flex gap-2 overflow-hidden items-center">
                {(group.domains || []).slice(0, 4).map((d: string, i: number) => (
                    <span key={i} className="text-[9px] bg-background-soft px-2 py-0.5 rounded border border-border/40 text-text-secondary whitespace-nowrap font-mono">
                        {d}
                    </span>
                ))}
                {(group.domains || []).length > 4 && (
                     <span className="text-[10px] text-text-muted font-bold px-1 opacity-50">+{ (group.domains || []).length - 4 }</span>
                )}
            </div>
        </div>
      </div>

      <div className="w-32 shrink-0 flex flex-col items-end px-3 border-r border-border/30 mr-2">
          <span className={`text-[10px] font-black uppercase tracking-widest ${modeColors[modeKey] || 'text-text-muted'}`}>
              {getModeDisplay(effectiveMode)}
          </span>
          <div className="flex items-center gap-1.5 text-[10px] text-text-muted font-bold truncate max-w-full">
            <Activity size={10} className="text-success" />
            <span className="truncate uppercase">{getEffectiveUpstream(group)}</span>
          </div>
      </div>

      <div className="flex gap-1.5 opacity-0 group-hover:opacity-100 transition-all">
          <button onClick={() => onEdit(group)} className="p-1.5 hover:bg-background-hover rounded text-text-secondary hover:text-accent" title="编辑"><Edit3 size={15} /></button>
          <button onClick={() => onDelete(group.id)} className="p-1.5 hover:bg-danger/10 rounded text-danger" title="删除"><Trash2 size={15} /></button>
      </div>
    </div>
  );
};

const Rules: React.FC = () => {
  const [groups, setGroups] = useState<any[]>([]);
  const [search, setSearch] = useState('');
  const [filterMode, setFilterMode] = useState('ALL');
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingGroup, setEditingGroup] = useState<any>(null);
  const [pendingDeleteGroup, setPendingDeleteGroup] = useState<any>(null);

  const loadData = async () => {
    try {
        const data = await GetSiteGroups();
        setGroups(data || []);
    } catch (e) {
        console.error("Failed to load site groups:", e);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const handleAdd = () => {
    setEditingGroup(null);
    setIsModalOpen(true);
  };

  const handleEdit = (group: any) => {
    setEditingGroup(group);
    setIsModalOpen(true);
  };

  const handleDelete = async (id: string) => {
    const target = groups.find((group) => group.id === id);
    setPendingDeleteGroup(target || { id });
  };

  const groupedResults = React.useMemo(() => {
    const filtered = groups.filter(g => {
      const matchesSearch = (
        (g.name || '') + 
        (g.website || '') + 
        (g.domains || []).join('')
      ).toLowerCase().includes(search.toLowerCase());
      
      const matchesMode = filterMode === 'ALL' || getEffectiveMode(g) === filterMode;
      return matchesSearch && matchesMode;
    });

    const groups_map: Record<string, any[]> = {};
    filtered.forEach(item => {
      const key = item.website || 'Others';
      if (!groups_map[key]) groups_map[key] = [];
      groups_map[key].push(item);
    });

    return Object.keys(groups_map).sort((a, b) => {
      if (a === 'Others') return 1;
      if (b === 'Others') return -1;
      return a.localeCompare(b);
    }).map(key => ({
      title: key,
      items: groups_map[key]
    }));
  }, [groups, search, filterMode]);

  return (
    <div className="p-5 max-w-5xl mx-auto space-y-4 animate-in fade-in duration-500">
      <header className="flex justify-between items-center bg-background border border-border p-5 rounded-2xl shadow-sm">
        <div>
           <h1 className="text-xl font-black tracking-tight">网站规则</h1>
        </div>
        <div className="flex gap-2">
            <button 
                onClick={async () => {
                    const cfg = await ExportConfig();
                    if (cfg) {
                        await navigator.clipboard.writeText(cfg);
                        toast.success("配置已复制", "现在可以直接粘贴分享。");
                    }
                }}
                className="p-2 rounded-xl bg-background-hover border border-border hover:text-accent transition-colors shadow-sm"
                title="导出并复制配置"
            >
                <ExternalLink size={16} />
            </button>
            <button 
                onClick={handleAdd}
                className="flex items-center gap-2 px-4 py-2 rounded-xl bg-accent text-white font-bold shadow-md shadow-accent/20 hover:scale-[1.02] active:scale-[0.98] transition-all"
            >
                <Plus size={16} strokeWidth={3} />
                <span className="text-sm">添加规则</span>
            </button>
        </div>
      </header>

      <div className="sticky top-0 z-10 space-y-3 pt-2 bg-background/50 backdrop-blur-md pb-2">
          <div className="flex items-center gap-1.5 p-1 bg-background-card border border-border rounded-xl w-fit shadow-sm">
              {FILTER_MODES.map((m) => (
                  <button
                      key={m}
                      onClick={() => setFilterMode(m)}
                      className={`px-4 py-1.5 rounded-lg text-[10px] font-black tracking-widest uppercase transition-all ${
                          filterMode === m 
                          ? "bg-accent text-white shadow-md shadow-accent/20" 
                          : "text-text-secondary hover:bg-background-hover"
                      }`}
                  >
                      {m === 'TLS-RF' ? '分片' : m === 'TRANSPARENT' ? '透传' : m}
                  </button>
              ))}
          </div>

          <div className="relative group">
              <Search className="absolute left-4 top-1/2 -translate-y-1/2 text-text-muted transition-colors group-focus-within:text-accent" size={16} />
              <input 
                  type="text" 
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder="搜索规则、分类或域名..."
                  className="w-full bg-background-card border border-border focus:border-accent/40 pl-11 pr-4 py-3 rounded-xl text-sm outline-none transition-all shadow-sm"
              />
          </div>
      </div>

      <div className="space-y-8 pb-10">
        {groupedResults.length === 0 ? (
            <div className="border border-border rounded-2xl overflow-hidden bg-background-card shadow-sm py-24 flex flex-col items-center justify-center text-text-muted opacity-30 grayscale">
                <Filter size={48} strokeWidth={1} />
                <span className="text-xs mt-4 font-black uppercase tracking-[0.2em]">No Results Found</span>
            </div>
        ) : (
            groupedResults.map((group) => (
              <div key={group.title} className="space-y-2">
                  <div className="flex items-center gap-2 px-2">
                      <div className="w-1.5 h-4 bg-accent rounded-full"></div>
                      <h2 className="text-[11px] font-black uppercase tracking-[0.2em] text-text-secondary flex items-center gap-2">
                        {group.title}
                        <span className="text-[9px] bg-background-card border border-border px-1.5 py-0.5 rounded text-text-muted font-bold">{group.items.length}</span>
                      </h2>
                      <div className="flex-1 h-[1px] bg-gradient-to-r from-border/60 to-transparent"></div>
                  </div>
                  <div className="border border-border rounded-2xl overflow-hidden bg-background-card shadow-sm divide-y divide-border/40">
                      {group.items.map((item) => (
                        <RuleItem 
                            key={item.id} 
                            group={item} 
                            onEdit={handleEdit} 
                            onDelete={handleDelete} 
                        />
                      ))}
                  </div>
              </div>
            ))
        )}
      </div>

      <Modal 
        isOpen={isModalOpen} 
        onClose={() => setIsModalOpen(false)} 
        title={editingGroup ? "编辑规则配置" : "创建新规则"}
        maxWidth="max-w-2xl"
        footer={(
            <>
                <button 
                    type="button"
                    onClick={() => setIsModalOpen(false)}
                    className="px-6 py-2 rounded-xl border border-border text-text-secondary font-bold hover:bg-background-hover transition-all text-xs"
                >
                    取消
                </button>
                <button 
                    type="submit"
                    form="rule-form"
                    className="px-8 py-2 rounded-xl bg-accent text-white font-black shadow-lg shadow-accent/20 hover:scale-[1.02] active:scale-[0.98] transition-all flex items-center gap-2 text-xs"
                >
                    <ShieldCheck size={16} />
                    {editingGroup ? "更新规则" : "保存规则"}
                </button>
            </>
        )}
      >
        <RuleForm 
            initialData={editingGroup} 
            onSuccess={() => { setIsModalOpen(false); loadData(); }} 
            onCancel={() => setIsModalOpen(false)} 
        />
      </Modal>

      <Modal
        isOpen={Boolean(pendingDeleteGroup)}
        onClose={() => setPendingDeleteGroup(null)}
        title="删除规则"
        subtitle="这会立刻从当前规则集中移除该条目。"
        maxWidth="max-w-md"
        footer={(
          <>
            <button
              type="button"
              onClick={() => setPendingDeleteGroup(null)}
              className="px-5 py-2 rounded-xl border border-border text-text-secondary font-bold hover:bg-background-hover transition-all text-xs"
            >
              取消
            </button>
            <button
              type="button"
              onClick={async () => {
                if (!pendingDeleteGroup?.id) return;
                await DeleteSiteGroup(pendingDeleteGroup.id);
                setPendingDeleteGroup(null);
                await loadData();
                toast.success('规则已删除');
              }}
              className="px-6 py-2 rounded-xl bg-danger text-white font-black transition-all text-xs"
            >
              确认删除
            </button>
          </>
        )}
      >
        <div className="text-sm text-text-secondary leading-relaxed">
          将要删除
          <span className="mx-1 font-bold text-text-primary">{pendingDeleteGroup?.name || '未命名规则'}</span>
          ，删除后不会自动恢复。
        </div>
      </Modal>
    </div>
  );
};

export default Rules;
