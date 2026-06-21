import React, { useState, useEffect, useRef } from 'react';
import {
  Play,
  Square,
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
  CheckSquare,
  Timer,
  Wifi
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
  const logsEndRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = () => {
    logsEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  useEffect(() => {
    scrollToBottom();
  }, [tempRules]);

  const getMethodLabel = (method?: string): string => {
    const key = method || 'unreachable';
    return t(`evolution.methods.${key}`);
  };

  const getStepLabel = (stepName: string): string => {
    const key = stepName.replace(/-/g, '_');
    return t(`evolution.steps.${key}`) || stepName.replace(/_/g, ' ');
  };

  const handleStartTest = async () => {
    const domainList = domains.split('\n').map(d => d.trim()).filter(d => d.length > 0);

    if (domainList.length === 0) {
      toast.error(t('evolution.empty_rules'));
      return;
    }

    setIsOperating(true);
    setIsRunning(true);
    setProgress({ current: 0, total: domainList.length });

    try {
      await StartEvolutionTest(domainList, enableIPv6);
      toast.success(t('evolution.start_success', { count: domainList.length }));
    } catch (error: any) {
      toast.error(t('evolution.start_failed', { error: error?.message || error }));
      setIsRunning(false);
      setIsOperating(false);
    }
  };

  const handleStopTest = async () => {
    try {
      await StopEvolutionTest();
      setIsRunning(false);
      setIsOperating(false);
      toast.info(t('evolution.stop_success'));
    } catch (error: any) {
      toast.error(t('evolution.stop_failed', { error: error?.message || error }));
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
      toast.success(t('evolution.convert_success', { name: selectedRule.name }));
      setShowRuleModal(false);
      setSelectedRule(null);
    } catch (error: any) {
      toast.error(t('evolution.convert_failed', { error: error?.message || error }));
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
        console.error(t('evolution.status_failed'), e);
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
      toast.success(t('evolution.complete_success', { count: data.results?.length || 0 }));
      setActiveTab('results');
    };

    const unlisten1 = EventsOn('evolution:progress', handleProgress);
    const unlisten2 = EventsOn('evolution:complete', handleComplete);

    return () => {
      unlisten1?.();
      unlisten2?.();
    };
  }, []);

  const successCount = results.filter(r => r.reachable).length;
  const failCount = results.filter(r => !r.reachable).length;
  const progressPercent = progress.total > 0 ? (progress.current / progress.total) * 100 : 0;
  const domainCount = domains.split('\n').filter(d => d.trim().length > 0).length;

  const tabs = [
    { id: 'test' as TabType, label: t('evolution.tab_test'), icon: <Play className="w-4 h-4" />, count: undefined },
    { id: 'rules' as TabType, label: t('evolution.tab_rules'), icon: <FileText className="w-4 h-4" />, count: tempRules.length },
    { id: 'results' as TabType, label: t('evolution.tab_results'), icon: <Activity className="w-4 h-4" />, count: results.length },
  ];

  return (
    <div className="h-full flex flex-col bg-background-soft/30">
      {/* Header */}
      <div className="relative px-6 py-5 border-b border-border/60 bg-gradient-to-r from-background/80 via-background-card/60 to-background/80 backdrop-blur-md overflow-hidden">
        <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_top_right,_var(--accent-soft)_0%,_transparent_60%)] opacity-30" />
        <div className="relative flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-2.5 bg-accent/10 rounded-xl border border-accent/10">
              <Zap className="w-5 h-5 text-accent" />
            </div>
            <div>
              <h1 className="text-lg font-bold text-text-primary tracking-tight">{t('evolution.title')}</h1>
              <p className="text-xs text-text-muted mt-0.5">{t('evolution.subtitle')}</p>
            </div>
          </div>
          {results.length > 0 && (
            <div className="flex items-center gap-3">
              <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-success/10 border border-success/20">
                <CheckCircle2 className="w-3.5 h-3.5 text-success" />
                <span className="text-xs font-bold text-success">{successCount}</span>
              </div>
              <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-danger/10 border border-danger/20">
                <XCircle className="w-3.5 h-3.5 text-danger" />
                <span className="text-xs font-bold text-danger">{failCount}</span>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Tab Bar */}
      <div className="px-6 py-2 border-b border-border/60 bg-background/50">
        <div className="flex gap-1">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-bold transition-all duration-200 ${
                activeTab === tab.id
                  ? 'bg-accent/10 text-accent border border-accent/20 shadow-sm shadow-accent/5'
                  : 'text-text-secondary hover:text-text-primary hover:bg-background-soft/50 border border-transparent'
              }`}
            >
              {tab.icon}
              {tab.label}
              {tab.count !== undefined && tab.count > 0 && (
                <span className={`px-1.5 py-0.5 rounded-full text-[10px] font-bold min-w-[18px] text-center ${
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
            <div className="space-y-5">
              <div className="p-6 bg-background-card border border-border rounded-2xl shadow-sm hover:shadow-md transition-shadow">
                <div className="flex items-center gap-2 mb-4">
                  <div className="p-1.5 bg-accent/10 rounded-lg">
                    <Globe className="w-3.5 h-3.5 text-accent" />
                  </div>
                  <h2 className="text-sm font-bold text-text-secondary uppercase tracking-tight">{t('evolution.domain_list')}</h2>
                  <span className="text-xs text-text-muted font-mono ml-auto">
                    {t('evolution.domain_count', { count: domainCount })}
                  </span>
                </div>
                <textarea
                  value={domains}
                  onChange={(e) => setDomains(e.target.value)}
                  placeholder={t('evolution.placeholder')}
                  className="w-full h-36 px-4 py-3 bg-background-soft/30 dark:bg-zinc-900/50 border border-border/60 rounded-xl text-text-primary placeholder-text-muted/40 focus:outline-none focus:ring-2 focus:ring-accent/30 focus:border-accent/50 resize-none font-mono text-sm transition-all"
                  disabled={isRunning}
                />

                <div className="flex items-center justify-between mt-4">
                  <label className="flex items-center gap-2.5 cursor-pointer group">
                    <div className={`relative w-10 h-5 rounded-full transition-all duration-200 ${enableIPv6 ? 'bg-accent' : 'bg-slate-200 dark:bg-zinc-800'} border ${enableIPv6 ? 'border-accent/20' : 'border-slate-300 dark:border-zinc-700'}`}>
                      <div className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow-sm border border-slate-300/40 dark:border-zinc-600/40 transition-transform duration-200 ${enableIPv6 ? 'translate-x-5' : ''}`} />
                    </div>
                    <input
                      type="checkbox"
                      checked={enableIPv6}
                      onChange={(e) => setEnableIPv6(e.target.checked)}
                      className="sr-only"
                      disabled={isRunning}
                    />
                    <span className="text-sm text-text-secondary group-hover:text-text-primary transition-colors">{t('evolution.enable_ipv6')}</span>
                  </label>

                  <div className="flex items-center gap-2">
                    {isRunning ? (
                      <button
                        onClick={handleStopTest}
                        className="flex items-center gap-2 px-5 py-2.5 bg-danger/10 hover:bg-danger/20 text-danger border border-danger/20 rounded-xl text-sm font-bold transition-all"
                      >
                        <Square className="w-4 h-4" />
                        {t('evolution.stop_test')}
                      </button>
                    ) : (
                      <button
                        onClick={handleStartTest}
                        disabled={isOperating || domains.trim().length === 0}
                        className="flex items-center gap-2 px-5 py-2.5 bg-accent hover:bg-accent/90 disabled:opacity-40 disabled:cursor-not-allowed text-white rounded-xl text-sm font-bold transition-all shadow-sm hover:shadow-md hover:shadow-accent/20 active:scale-[0.98]"
                      >
                        {isOperating ? (
                          <Loader2 className="w-4 h-4 animate-spin" />
                        ) : (
                          <Play className="w-4 h-4" />
                        )}
                        {t('evolution.start_test')}
                      </button>
                    )}
                  </div>
                </div>

                {isRunning && (
                  <div className="mt-5 pt-4 border-t border-border/50">
                    <div className="flex items-center justify-between text-xs text-text-muted mb-2.5">
                      <span className="flex items-center gap-1.5 font-bold">
                        <Loader2 className="w-3.5 h-3.5 animate-spin text-accent" />
                        {t('evolution.test_progress')}
                      </span>
                      <span className="font-mono font-bold text-accent bg-accent/10 px-2 py-0.5 rounded-md text-[11px]">
                        {progress.current} / {progress.total}
                      </span>
                    </div>
                    <div className="relative w-full h-2.5 bg-background-soft border border-border/40 rounded-full overflow-hidden">
                      <div
                        className="absolute inset-y-0 left-0 rounded-full transition-all duration-500 ease-out"
                        style={{
                          width: `${progressPercent}%`,
                          background: 'linear-gradient(90deg, var(--accent) 0%, #a855f7 50%, var(--accent) 100%)',
                          backgroundSize: '200% 100%',
                          animation: 'shimmer 2s linear infinite',
                          boxShadow: '0 0 8px color-mix(in srgb, var(--accent) 50%, transparent)'
                        }}
                      />
                    </div>
                    {progress.current > 0 && (
                      <div className="mt-2.5 text-xs text-text-secondary text-center font-bold">
                        {t('evolution.testing_domain', { current: progress.current })}
                      </div>
                    )}
                  </div>
                )}
              </div>

              <div className="p-5 bg-background-card border border-border rounded-2xl shadow-sm">
                <div className="flex items-center gap-2 mb-3">
                  <div className="p-1.5 bg-accent/10 rounded-lg">
                    <Wifi className="w-3.5 h-3.5 text-accent" />
                  </div>
                  <h3 className="text-sm font-bold text-text-secondary uppercase tracking-tight">{t('evolution.test_flow')}</h3>
                </div>
                <div className="flex flex-wrap gap-2">
                  {[t('evolution.steps.direct'), t('evolution.steps.tcping'), t('evolution.steps.domain_fronting'), t('evolution.steps.tls_fragment'), t('evolution.steps.ech'), t('evolution.steps.quic')].map((step, i) => (
                    <React.Fragment key={step}>
                      <span className="text-xs px-2.5 py-1 bg-background-soft/80 border border-border/60 rounded-lg text-text-secondary font-medium">
                        {step}
                      </span>
                      {i < 5 && <ArrowRight className="w-3 h-3 text-text-muted/40 self-center" />}
                    </React.Fragment>
                  ))}
                </div>
                <p className="text-xs text-text-muted mt-3">
                  {t('evolution.test_flow_desc')}
                </p>
              </div>
            </div>
          </div>

          {/* Rules Tab */}
          <div style={{ display: activeTab === 'rules' ? 'block' : 'none' }}>
            <div className="space-y-4">
              {tempRules.length === 0 ? (
                <div className="p-6 bg-background-card border border-border rounded-2xl shadow-sm">
                  <div className="text-center py-14">
                    <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-background-soft/50 border border-border/50 flex items-center justify-center">
                      <FileText className="w-7 h-7 text-text-muted/30" />
                    </div>
                    <p className="text-sm font-medium text-text-muted">{t('evolution.empty_rules')}</p>
                    <p className="text-xs text-text-muted/60 mt-1.5">{t('evolution.empty_rules_desc')}</p>
                  </div>
                </div>
              ) : (
                <>
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <FileText className="w-4 h-4 text-accent" />
                      <span className="text-sm font-bold text-text-primary">{t('evolution.temp_rules')}</span>
                      <span className="text-xs text-text-muted font-mono bg-background-soft px-2 py-0.5 rounded-md">{tempRules.length}</span>
                    </div>
                    <div className="text-xs text-text-muted">
                      {t('evolution.applied_count', { applied: tempRules.filter(r => r.is_applied).length, total: tempRules.length })}
                    </div>
                  </div>
                  <div className="space-y-2">
                    {tempRules.map((rule) => (
                      <div
                        key={rule.id}
                        className="group p-4 bg-background-card border border-border rounded-xl hover:border-accent/30 hover:shadow-md transition-all duration-200"
                      >
                        <div className="flex items-center gap-4">
                          <div className={`p-2 rounded-lg ${getMethodBgColor(rule.method)} group-hover:scale-110 transition-transform`}>
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
                                  {t('evolution.applied')}
                                </span>
                              )}
                            </div>
                            <div className="flex items-center gap-3 mt-1 text-xs text-text-muted font-mono">
                              <span>{rule.domain}</span>
                              {rule.sni_fake && (
                                <>
                                  <span className="text-border">|</span>
                                  <span>{t('evolution.sni')}: {rule.sni_fake}</span>
                                </>
                              )}
                            </div>
                          </div>
                          {!rule.is_applied && (
                            <button
                              onClick={() => handleApplyRule(rule)}
                              className="flex items-center gap-1.5 px-4 py-2 bg-accent/10 hover:bg-accent/20 text-accent text-xs rounded-lg font-bold transition-all border border-accent/20 opacity-0 group-hover:opacity-100"
                            >
                              <ArrowRight className="w-3 h-3" />
                              {t('evolution.convert_to_rule')}
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
                  <div className="text-center py-14">
                    <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-background-soft/50 border border-border/50 flex items-center justify-center">
                      <Activity className="w-7 h-7 text-text-muted/30" />
                    </div>
                    <p className="text-sm font-medium text-text-muted">{t('evolution.empty_results')}</p>
                    <p className="text-xs text-text-muted/60 mt-1.5">{t('evolution.empty_results_desc')}</p>
                  </div>
                </div>
              ) : (
                <>
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Activity className="w-4 h-4 text-accent" />
                      <span className="text-sm font-bold text-text-primary">{t('evolution.test_results')}</span>
                      <span className="text-xs text-text-muted font-mono bg-background-soft px-2 py-0.5 rounded-md">{results.length}</span>
                    </div>
                    <div className="flex items-center gap-3 text-xs">
                      <span className="flex items-center gap-1 text-success font-bold">
                        <CheckCircle2 className="w-3.5 h-3.5" />
                        {t('evolution.success_count', { count: successCount })}
                      </span>
                      <span className="flex items-center gap-1 text-danger font-bold">
                        <XCircle className="w-3.5 h-3.5" />
                        {t('evolution.fail_count', { count: failCount })}
                      </span>
                    </div>
                  </div>
                  <div className="space-y-2">
                    {results.map((result, index) => (
                      <div
                        key={index}
                        className={`group p-4 border rounded-xl transition-all duration-200 ${
                          result.reachable
                            ? 'bg-background-card border-border hover:border-accent/30 hover:shadow-md'
                            : 'bg-danger/5 border-danger/20 hover:border-danger/40'
                        }`}
                      >
                        <div className="flex items-center gap-3">
                          <div className={`p-2 rounded-lg transition-transform group-hover:scale-110 ${
                            result.reachable ? getMethodBgColor(result.method) : 'bg-danger/10 border-danger/20'
                          }`}>
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
                                      {t('evolution.cloudflare')}
                                    </span>
                                  )}
                                </>
                              ) : (
                                <span className="text-[10px] px-2 py-0.5 rounded-full bg-danger/10 border border-danger/20 text-danger font-bold">
                                  {t('evolution.unreachable')}
                                </span>
                              )}
                            </div>
                            <div className="flex items-center gap-3 mt-1 text-xs text-text-muted">
                              {result.reachable ? (
                                <>
                                  <span className="flex items-center gap-1">
                                    <Timer className="w-3 h-3" />
                                    {t('evolution.delay', { ms: Math.round(result.delay / 1000000) })}
                                  </span>
                                  {result.best_ip && (
                                    <span className="font-mono">{result.best_ip}</span>
                                  )}
                                </>
                              ) : (
                                result.error && <span className="text-danger text-xs">{result.error}</span>
                              )}
                            </div>
                          </div>

                          <div className="text-[10px] text-text-muted font-mono shrink-0 opacity-60 group-hover:opacity-100 transition-opacity">
                            {new Date(result.timestamp).toLocaleTimeString()}
                          </div>
                        </div>

                        {result.step_results && result.step_results.length > 0 && (
                          <div className="mt-3 pt-3 border-t border-border/50">
                            <div className="flex flex-wrap gap-1.5">
                              {result.step_results.map((step, i) => (
                                <div
                                  key={i}
                                  className={`flex items-center gap-1 text-[10px] px-2 py-1 rounded-full border font-bold transition-colors ${
                                    step.success
                                      ? 'bg-success/10 border-success/20 text-success'
                                      : 'bg-danger/10 border-danger/20 text-danger'
                                  }`}
                                >
                                  {step.success ? <CheckCircle2 className="w-3 h-3" /> : <XCircle className="w-3 h-3" />}
                                  {getStepLabel(step.step_name)}
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
        title={t('evolution.confirm_title')}
      >
        {selectedRule && (
          <div className="space-y-4">
            <p className="text-sm text-text-secondary">
              {t('evolution.confirm_desc')}
            </p>
            <div className="p-4 bg-background-soft/50 border border-border rounded-xl">
              <div className="flex items-center gap-2 mb-2">
                <span className="text-sm font-bold text-text-primary">{selectedRule.name}</span>
                <span className={`text-[10px] px-2 py-0.5 rounded-full border font-bold ${getMethodBgColor(selectedRule.method)} ${getMethodColor(selectedRule.method)}`}>
                  {getMethodLabel(selectedRule.method)}
                </span>
              </div>
              <div className="text-xs text-text-muted font-mono">
                {t('evolution.domain')}: {selectedRule.domain}
                {selectedRule.sni_fake && <span> | {t('evolution.sni')}: {selectedRule.sni_fake}</span>}
              </div>
            </div>
            <div className="flex gap-3 justify-end">
              <button
                onClick={() => { setShowRuleModal(false); setSelectedRule(null); }}
                className="px-4 py-2 bg-background-soft border border-border text-text-secondary rounded-xl text-sm font-bold hover:bg-background-hover transition-all"
              >
                {t('common.cancel')}
              </button>
              <button
                onClick={confirmApplyRule}
                className="px-4 py-2 bg-accent hover:bg-accent/90 text-white rounded-xl text-sm font-bold transition-all shadow-sm hover:shadow-md"
              >
                {t('evolution.confirm_convert')}
              </button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default Evolution;
