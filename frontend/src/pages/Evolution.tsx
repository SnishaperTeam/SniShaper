import React, { useState, useEffect, useRef } from 'react';
import {
  Play,
  Square,
  Plus,
  Globe,
  Activity,
  Zap,
  Shield,
  Network,
  AlertCircle,
  Loader2,
  CheckCircle2,
  XCircle,
  ArrowRight,
  FileText,
  CheckSquare
} from 'lucide-react';
import {
  StartEvolutionTest,
  StopEvolutionTest,
  ApplyEvolutionRule,
  GetEvolutionTestStatus,
  EventsOn
} from '../api/bindings';
import Modal from '../components/Modal';
import { toast } from '../lib/toast';
import { useTranslation } from '../i18n/I18nContext';

interface TestResult {
  domain: string;
  reachable: boolean;
  method?: string;
  resolved_ips?: string[];
  best_ip?: string;
  is_cloudflare: boolean;
  delay: number;
  error?: string;
  timestamp: string;
  step_results: StepResult[];
  generated_rule?: TempRule;
}

interface StepResult {
  step_name: string;
  success: boolean;
  delay: number;
  error?: string;
  timestamp: string;
}

interface TempRule {
  id: string;
  name: string;
  domain: string;
  mode: string;
  sni_fake?: string;
  ech_enabled: boolean;
  method: string;
  created_at: string;
  is_applied: boolean;
}

type TabType = 'test' | 'rules' | 'results';

const getMethodIcon = (method?: string) => {
  switch (method) {
    case 'direct': return <Globe className="w-4 h-4" />;
    case 'domain_fronting': return <Shield className="w-4 h-4" />;
    case 'tls_fragment': return <Network className="w-4 h-4" />;
    case 'ech': return <Zap className="w-4 h-4" />;
    case 'quic': return <Activity className="w-4 h-4" />;
    default: return <AlertCircle className="w-4 h-4" />;
  }
};

const getMethodColor = (method?: string) => {
  switch (method) {
    case 'direct': return 'text-success';
    case 'domain_fronting': return 'text-amber-600 dark:text-amber-400';
    case 'tls_fragment': return 'text-blue-600 dark:text-blue-400';
    case 'ech': return 'text-purple-600 dark:text-purple-400';
    case 'quic': return 'text-cyan-600 dark:text-cyan-400';
    default: return 'text-danger';
  }
};

const getMethodBgColor = (method?: string) => {
  switch (method) {
    case 'direct': return 'bg-success/10 border-success/20';
    case 'domain_fronting': return 'bg-amber-500/10 border-amber-500/20';
    case 'tls_fragment': return 'bg-blue-500/10 border-blue-500/20';
    case 'ech': return 'bg-purple-500/10 border-purple-500/20';
    case 'quic': return 'bg-cyan-500/10 border-cyan-500/20';
    default: return 'bg-danger/10 border-danger/20';
  }
};

const getMethodLabel = (method?: string) => {
  switch (method) {
    case 'direct': return '直连';
    case 'domain_fronting': return '域前置';
    case 'tls_fragment': return 'TLS分片';
    case 'ech': return 'ECH';
    case 'quic': return 'QUIC';
    default: return '不可达';
  }
};

const Evolution: React.FC = () => {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<TabType>('test');
  const [domains, setDomains] = useState<string>('');
  const [enableIPv6, setEnableIPv6] = useState(false);
  const [isRunning, setIsRunning] = useState(false);
  const [results, setResults] = useState<TestResult[]>([]);
  const [progress, setProgress] = useState({ current: 0, total: 0 });
  const [tempRules, setTempRules] = useState<TempRule[]>([]);
  const [isOperating, setIsOperating] = useState(false);
  const [showRuleModal, setShowRuleModal] = useState(false);
  const [selectedRule, setSelectedRule] = useState<TempRule | null>(null);
  const [currentTestingDomain, setCurrentTestingDomain] = useState<string>('');
  const [testLogs, setTestLogs] = useState<string[]>([]);
  const logsEndRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = () => {
    logsEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  useEffect(() => {
    scrollToBottom();
  }, [testLogs]);

  const handleStartTest = async () => {
    const domainList = domains.split('\n').map(d => d.trim()).filter(d => d.length > 0);

    if (domainList.length === 0) {
      toast.error('请输入至少一个域名');
      return;
    }

    setIsOperating(true);
    setIsRunning(true);
    setTestLogs([]);
    setProgress({ current: 0, total: domainList.length });

    try {
      await StartEvolutionTest(domainList, enableIPv6);
      toast.success(`开始测试 ${domainList.length} 个域名`);
    } catch (error: any) {
      toast.error(`启动测试失败: ${error?.message || error}`);
      setIsRunning(false);
      setIsOperating(false);
    }
  };

  const handleStopTest = async () => {
    try {
      await StopEvolutionTest();
      setIsRunning(false);
      setIsOperating(false);
      toast.info('测试已停止');
    } catch (error: any) {
      toast.error(`停止测试失败: ${error?.message || error}`);
    }
  };

  const handleApplyRule = async (rule: TempRule) => {
    setSelectedRule(rule);
    setShowRuleModal(true);
  };

  const confirmApplyRule = async () => {
    if (!selectedRule) return;
    try {
      await ApplyEvolutionRule(selectedRule.id);
      setTempRules(prev => prev.filter(r => r.id !== selectedRule.id));
      toast.success(`规则 "${selectedRule.name}" 已转为正式规则`);
      setShowRuleModal(false);
      setSelectedRule(null);
    } catch (error: any) {
      toast.error(`应用规则失败: ${error?.message || error}`);
    }
  };

  useEffect(() => {
    const fetchStatus = async () => {
      try {
        const status: any = await GetEvolutionTestStatus();
        if (status && status.status !== 'idle') {
          setIsRunning(status.status === 'running');
          setProgress({
            current: status.progress || 0,
            total: status.total || 0
          });
          if (status.results) {
            setResults(status.results);
          }
          if (status.temp_rules) {
            setTempRules(status.temp_rules);
          } else if (status.results) {
            const newRules = status.results
              .filter((r: any) => r.generated_rule)
              .map((r: any) => r.generated_rule);
            setTempRules(newRules);
          }
        }
      } catch (e) {
        console.error("获取进化模式状态失败:", e);
      }
    };

    fetchStatus();

    const handleProgress = (data: any) => {
      setProgress({
        current: data.progress,
        total: data.total
      });
      if (data.results) {
        setResults(data.results);
      }
      if (data.temp_rules) {
        setTempRules(data.temp_rules);
      }
    };

    const handleComplete = (data: any) => {
      setIsRunning(false);
      setIsOperating(false);
      if (data.results) {
        setResults(data.results);
        const newRules = data.results
          .filter((r: TestResult) => r.generated_rule)
          .map((r: TestResult) => r.generated_rule!);
        setTempRules(newRules);
      }
      toast.success(`测试完成，共 ${data.results?.length || 0} 个域名`);
      setActiveTab('results');
    };

    const unlisten1 = EventsOn('evolution:progress', handleProgress);
    const unlisten2 = EventsOn('evolution:complete', handleComplete);

    return () => {
      unlisten1?.();
      unlisten2?.();
    };
  }, []);

  const tabs = [
    { id: 'test' as TabType, label: '测试', icon: <Play className="w-4 h-4" />, count: undefined },
    { id: 'rules' as TabType, label: '规则', icon: <FileText className="w-4 h-4" />, count: tempRules.length },
    { id: 'results' as TabType, label: '结果', icon: <Activity className="w-4 h-4" />, count: results.length },
  ];

  return (
    <div className="h-full flex flex-col bg-background-soft/30">
      {/* Header */}
      <div className="px-6 py-4 border-b border-border/60 bg-background/70 backdrop-blur-md">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-accent/10 rounded-xl">
            <Zap className="w-5 h-5 text-accent" />
          </div>
          <div>
            <h1 className="text-lg font-bold text-text-primary">进化模式</h1>
            <p className="text-xs text-text-muted">自动测试域名连通性并生成最优规则</p>
          </div>
        </div>
      </div>

      {/* Tab Bar */}
      <div className="px-6 py-2 border-b border-border/60 bg-background/50">
        <div className="flex gap-1">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-bold transition-all ${
                activeTab === tab.id
                  ? 'bg-accent/10 text-accent border border-accent/20'
                  : 'text-text-secondary hover:text-text-primary hover:bg-background-soft/50'
              }`}
            >
              {tab.icon}
              {tab.label}
              {tab.count !== undefined && tab.count > 0 && (
                <span className={`px-1.5 py-0.5 rounded-full text-[10px] font-bold ${
                  activeTab === tab.id ? 'bg-accent/20 text-accent' : 'bg-background-soft text-text-muted'
                }`}>
                  {tab.count}
                </span>
              )}
            </button>
          ))}
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto">
        <div className="p-6 max-w-5xl mx-auto">
          {/* Test Tab */}
          <div style={{ display: activeTab === 'test' ? 'block' : 'none' }}>
            <div className="space-y-6">
              <div className="p-6 bg-background-card border border-border rounded-2xl shadow-sm">
                <div className="flex items-center gap-2 mb-4">
                  <Globe className="w-4 h-4 text-accent" />
                  <h2 className="text-sm font-bold text-text-secondary uppercase tracking-tight">域名列表</h2>
                </div>
                <textarea
                  value={domains}
                  onChange={(e) => setDomains(e.target.value)}
                  placeholder="example.com&#10;test.com&#10;demo.org"
                  className="w-full h-40 px-4 py-3 bg-background-soft/50 border border-border rounded-xl text-text-primary placeholder-text-muted focus:outline-none focus:ring-2 focus:ring-accent/30 focus:border-accent/50 resize-none font-mono text-sm transition-all"
                  disabled={isRunning}
                />

                <div className="flex items-center justify-between mt-4">
                  <label className="flex items-center gap-2 cursor-pointer group">
                    <div className={`w-10 h-5 rounded-full transition-all duration-200 ${enableIPv6 ? 'bg-accent border border-accent/20' : 'bg-slate-200 dark:bg-zinc-800 border border-slate-300 dark:border-zinc-700'}`}>
                      <div className={`w-4 h-4 rounded-full bg-white shadow-sm border border-slate-300/40 dark:border-zinc-600/40 transition-transform mt-0.5 ${enableIPv6 ? 'translate-x-5' : 'translate-x-0.5'}`} />
                    </div>
                    <input
                      type="checkbox"
                      checked={enableIPv6}
                      onChange={(e) => setEnableIPv6(e.target.checked)}
                      className="sr-only"
                      disabled={isRunning}
                    />
                    <span className="text-sm text-text-secondary group-hover:text-text-primary transition-colors">启用 IPv6</span>
                  </label>

                  {!isRunning ? (
                    <button
                      onClick={handleStartTest}
                      disabled={isOperating}
                      className="flex items-center gap-2 px-5 py-2.5 bg-accent hover:bg-accent/90 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-xl text-sm font-bold transition-all shadow-sm hover:shadow-md"
                    >
                      {isOperating ? (
                        <Loader2 className="w-4 h-4 animate-spin" />
                      ) : (
                        <Play className="w-4 h-4" />
                      )}
                      开始测试
                    </button>
                  ) : (
                    <button
                      onClick={handleStopTest}
                      className="flex items-center gap-2 px-5 py-2.5 bg-danger hover:bg-danger/90 text-white rounded-xl text-sm font-bold transition-all shadow-sm hover:shadow-md"
                    >
                      <Square className="w-4 h-4" />
                      停止测试
                    </button>
                  )}
                </div>

                {isRunning && (
                  <div className="mt-5 pt-4 border-t border-border/50">
                    <div className="flex items-center justify-between text-xs text-text-muted mb-2">
                      <span className="flex items-center gap-1.5 font-bold">
                        <Loader2 className="w-3.5 h-3.5 animate-spin text-accent" />
                        测试进度
                      </span>
                      <span className="font-mono font-bold text-accent bg-accent/10 px-2 py-0.5 rounded-md text-[11px]">{progress.current} / {progress.total}</span>
                    </div>
                    <div className="w-full h-3 bg-background-soft border border-border/40 rounded-full overflow-hidden shadow-inner relative">
                      <div
                        className="h-full rounded-full transition-all duration-500 ease-out animate-shimmer"
                        style={{
                          width: `${progress.total > 0 ? (progress.current / progress.total) * 100 : 0}%`,
                          background: 'linear-gradient(90deg, var(--accent) 0%, #a855f7 50%, var(--accent) 100%)',
                          boxShadow: '0 0 10px color-mix(in srgb, var(--accent) 45%, transparent)'
                        }}
                      />
                    </div>
                    {progress.current > 0 && (
                      <div className="mt-2 text-xs text-text-secondary text-center font-bold animate-pulse">
                        正在测试第 {progress.current} 个域名...
                      </div>
                    )}
                  </div>
                )}
              </div>

              <div className="p-6 bg-background-card border border-border rounded-2xl shadow-sm">
                <h3 className="text-sm font-bold text-text-secondary uppercase tracking-tight mb-3">测试说明</h3>
                <div className="space-y-2 text-xs text-text-muted">
                  <p>• 测试将依次尝试：直连 → TCPing → 域前置 → TLS分片 → ECH → QUIC</p>
                  <p>• 成功后会自动生成临时规则，可在「规则」页面查看并转为正式规则</p>
                  <p>• 测试结果可在「结果」页面查看详细信息</p>
                </div>
              </div>
            </div>
          </div>

          {/* Rules Tab */}
          <div style={{ display: activeTab === 'rules' ? 'block' : 'none' }}>
            <div className="space-y-4">
              {tempRules.length === 0 ? (
                <div className="p-6 bg-background-card border border-border rounded-2xl shadow-sm">
                  <div className="text-center py-16">
                    <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-background-soft/50 border border-border/50 flex items-center justify-center">
                      <FileText className="w-7 h-7 text-text-muted opacity-30" />
                    </div>
                    <p className="text-sm text-text-muted">暂无生成的规则</p>
                    <p className="text-xs text-text-muted/60 mt-1">完成测试后，成功的域名将生成临时规则</p>
                  </div>
                </div>
              ) : (
                <>
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <FileText className="w-4 h-4 text-accent" />
                      <span className="text-sm font-bold text-text-primary">临时规则</span>
                      <span className="text-xs text-text-muted font-mono">{tempRules.length} 条</span>
                    </div>
                    <div className="text-xs text-text-muted">
                      已应用: {tempRules.filter(r => r.is_applied).length} / {tempRules.length}
                    </div>
                  </div>
                  <div className="space-y-2">
                    {tempRules.map((rule) => (
                      <div
                        key={rule.id}
                        className="p-4 bg-background-card border border-border rounded-xl hover:border-accent/30 transition-all shadow-sm"
                      >
                        <div className="flex items-center gap-4">
                          <div className={`p-2 rounded-lg ${getMethodBgColor(rule.method)}`}>
                            <span className={getMethodColor(rule.method)}>{getMethodIcon(rule.method)}</span>
                          </div>
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2">
                              <span className="text-sm font-bold text-text-primary truncate">{rule.name}</span>
                              <span className={`text-[10px] px-2 py-0.5 rounded-full border font-bold ${getMethodBgColor(rule.method)} ${getMethodColor(rule.method)}`}>
                                {getMethodLabel(rule.method)}
                              </span>
                              {rule.is_applied && (
                                <span className="text-[10px] px-2 py-0.5 rounded-full bg-success/10 border border-success/20 text-success font-bold flex items-center gap-1">
                                  <CheckSquare className="w-3 h-3" />
                                  已应用
                                </span>
                              )}
                            </div>
                            <div className="text-xs text-text-muted mt-0.5 font-mono">
                              {rule.domain}
                              {rule.sni_fake && <span className="ml-2">• SNI: {rule.sni_fake}</span>}
                            </div>
                          </div>
                          {!rule.is_applied && (
                            <button
                              onClick={() => handleApplyRule(rule)}
                              className="flex items-center gap-1.5 px-4 py-2 bg-accent/10 hover:bg-accent/20 text-accent text-xs rounded-lg font-bold transition-all border border-accent/20"
                            >
                              <ArrowRight className="w-3 h-3" />
                              转为正式规则
                            </button>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                </>
              )}
            </div>
          </div>

          {/* Results Tab */}
          <div style={{ display: activeTab === 'results' ? 'block' : 'none' }}>
            <div className="space-y-4">
              {results.length === 0 ? (
                <div className="p-6 bg-background-card border border-border rounded-2xl shadow-sm">
                  <div className="text-center py-16">
                    <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-background-soft/50 border border-border/50 flex items-center justify-center">
                      <Activity className="w-7 h-7 text-text-muted opacity-30" />
                    </div>
                    <p className="text-sm text-text-muted">暂无测试结果</p>
                    <p className="text-xs text-text-muted/60 mt-1">在「测试」页面输入域名并开始测试</p>
                  </div>
                </div>
              ) : (
                <>
                  <div className="flex items-center gap-2">
                    <Activity className="w-4 h-4 text-accent" />
                    <span className="text-sm font-bold text-text-primary">测试结果</span>
                    <span className="text-xs text-text-muted font-mono">{results.length} 条</span>
                    <span className="ml-auto text-xs text-text-muted">
                      成功: {results.filter(r => r.reachable).length} | 失败: {results.filter(r => !r.reachable).length}
                    </span>
                  </div>
                  <div className="space-y-2">
                    {results.map((result, index) => (
                      <div
                        key={index}
                        className={`p-4 border rounded-xl transition-all ${
                          result.reachable
                            ? 'bg-background-card border-border hover:border-accent/30 shadow-sm'
                            : 'bg-danger/5 border-danger/20'
                        }`}
                      >
                        <div className="flex items-center gap-3">
                          <div className={`p-2 rounded-lg ${result.reachable ? getMethodBgColor(result.method) : 'bg-danger/10 border-danger/20'}`}>
                            <span className={result.reachable ? getMethodColor(result.method) : 'text-danger'}>
                              {getMethodIcon(result.method)}
                            </span>
                          </div>

                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2 flex-wrap">
                              <span className="text-sm font-bold text-text-primary">{result.domain}</span>
                              {result.reachable ? (
                                <>
                                  <span className={`text-[10px] px-2 py-0.5 rounded-full border font-bold ${getMethodBgColor(result.method)} ${getMethodColor(result.method)}`}>
                                    {getMethodLabel(result.method)}
                                  </span>
                                  {result.is_cloudflare && (
                                    <span className="text-[10px] px-2 py-0.5 rounded-full bg-orange-500/10 border border-orange-500/20 text-orange-500 font-bold">
                                      Cloudflare
                                    </span>
                                  )}
                                </>
                              ) : (
                                <span className="text-[10px] px-2 py-0.5 rounded-full bg-danger/10 border border-danger/20 text-danger font-bold">
                                  不可达
                                </span>
                              )}
                            </div>
                            <div className="flex items-center gap-4 mt-1 text-xs text-text-muted">
                              {result.reachable && (
                                <>
                                  <span>延迟: {Math.round(result.delay / 1000000)}ms</span>
                                  {result.best_ip && <span>IP: {result.best_ip}</span>}
                                </>
                              )}
                              {!result.reachable && result.error && (
                                <span className="text-danger">{result.error}</span>
                              )}
                            </div>
                          </div>

                          <div className="text-[10px] text-text-muted font-mono shrink-0">
                            {new Date(result.timestamp).toLocaleTimeString()}
                          </div>
                        </div>

                        {result.step_results && result.step_results.length > 0 && (
                          <div className="mt-3 pt-3 border-t border-border/50">
                            <div className="flex flex-wrap gap-1.5">
                              {result.step_results.map((step, i) => (
                                <div
                                  key={i}
                                  className={`flex items-center gap-1 text-[10px] px-2 py-1 rounded-full border font-bold ${
                                    step.success
                                      ? 'bg-success/10 border-success/20 text-success'
                                      : 'bg-danger/10 border-danger/20 text-danger'
                                  }`}
                                >
                                  {step.success ? <CheckCircle2 className="w-3 h-3" /> : <XCircle className="w-3 h-3" />}
                                  {step.step_name.replace('_', ' ')}
                                </div>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                </>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Rule Confirmation Modal */}
      <Modal
        isOpen={showRuleModal}
        onClose={() => { setShowRuleModal(false); setSelectedRule(null); }}
        title="确认转为正式规则"
      >
        {selectedRule && (
          <div className="space-y-4">
            <p className="text-sm text-text-secondary">
              确定要将以下规则转为正式规则并写入配置吗？
            </p>
            <div className="p-4 bg-background-soft/50 border border-border rounded-xl">
              <div className="flex items-center gap-2 mb-2">
                <span className="text-sm font-bold text-text-primary">{selectedRule.name}</span>
                <span className={`text-[10px] px-2 py-0.5 rounded-full border font-bold ${getMethodBgColor(selectedRule.method)} ${getMethodColor(selectedRule.method)}`}>
                  {getMethodLabel(selectedRule.method)}
                </span>
              </div>
              <div className="text-xs text-text-muted font-mono">
                域名: {selectedRule.domain}
                {selectedRule.sni_fake && <span> • SNI: {selectedRule.sni_fake}</span>}
              </div>
            </div>
            <div className="flex gap-3 justify-end">
              <button
                onClick={() => { setShowRuleModal(false); setSelectedRule(null); }}
                className="px-4 py-2 bg-background-soft border border-border text-text-secondary rounded-xl text-sm font-bold hover:bg-background-hover transition-all"
              >
                取消
              </button>
              <button
                onClick={confirmApplyRule}
                className="px-4 py-2 bg-accent hover:bg-accent/90 text-white rounded-xl text-sm font-bold transition-all shadow-sm"
              >
                确认转换
              </button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default Evolution;
