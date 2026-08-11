import React, { useState, useEffect, useRef, useContext } from 'react';
import {
  Play,
  Stop,
  Globe,
  Activity,
  Bolt,
  Shield,
  Public as NetworkIcon,
  Error as ErrorIcon,
  Sync,
  CheckCircle,
  Cancel,
  ArrowForward,
  Description,
  CheckSquare,
  Timer,
  Wifi
} from '../lib/icons';
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
import { SettingsCtx } from '../App';
import { Box, Typography, Button, TextField, Switch, LinearProgress } from '@mui/material';
import { alpha, keyframes } from '@mui/material/styles';

const spin = keyframes`
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
`;

const shimmer = keyframes`
  0% { background-position: 0% 0%; }
  100% { background-position: 200% 0%; }
`;

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
    case 'direct': return <Globe size={16} />;
    case 'domain_fronting': return <Shield size={16} />;
    case 'tls_fragment': return <NetworkIcon size={16} />;
    case 'ech': return <Bolt size={16} />;
    case 'quic': return <Activity size={16} />;
    default: return <ErrorIcon size={16} />;
  }
};

const getMethodColors = (method?: string) => {
  switch (method) {
    case 'direct': return { color: 'success.main', bg: 'rgba(34,197,94,0.1)', border: 'rgba(34,197,94,0.2)' };
    case 'domain_fronting': return { color: 'warning.main', bg: 'rgba(234,88,12,0.1)', border: 'rgba(234,88,12,0.2)' };
    case 'tls_fragment': return { color: 'primary.main', bg: (theme: any) => alpha(theme.palette.primary.main, 0.1), border: (theme: any) => alpha(theme.palette.primary.main, 0.2) };
    case 'ech': return { color: 'secondary.main', bg: (theme: any) => alpha(theme.palette.secondary.main, 0.1), border: (theme: any) => alpha(theme.palette.secondary.main, 0.2) };
    case 'quic': return { color: '#06b6d4', bg: 'rgba(6,182,212,0.1)', border: 'rgba(6,182,212,0.2)' };
    default: return { color: 'error.main', bg: 'rgba(239,68,68,0.1)', border: 'rgba(239,68,68,0.2)' };
  }
};

const Evolution: React.FC = () => {
  const { t } = useTranslation();
  const { cache } = useContext(SettingsCtx);
  const ipv6Available = cache.ipv6Available !== false;
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
    const label = t(`evolution.methods.${key}`);
    return label === `evolution.methods.${key}` ? key : label;
  };

  const getStepLabel = (stepName: string): string => {
    const key = stepName.replace(/-/g, '_');
    const label = t(`evolution.steps.${key}`);
    return label === `evolution.steps.${key}` ? stepName.replace(/_/g, ' ') : label;
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
    { id: 'test' as TabType, label: t('evolution.tab_test'), icon: <Play size={16} />, count: undefined },
    { id: 'rules' as TabType, label: t('evolution.tab_rules'), icon: <Description size={16} />, count: tempRules.length },
    { id: 'results' as TabType, label: t('evolution.tab_results'), icon: <Activity size={16} />, count: results.length },
  ];

  const flowSteps = [t('evolution.steps.direct'), t('evolution.steps.tcping'), t('evolution.steps.domain_fronting'), t('evolution.steps.tls_fragment'), t('evolution.steps.ech'), t('evolution.steps.quic')];

  return (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column', bgcolor: 'background.default' }}>
      <Box sx={{ position: 'relative', mt: 3, px: 3, py: 2.5, maxWidth: '5xl', width: '100%', mx: 'auto', border: 1, borderColor: 'divider', borderRadius: 2, bgcolor: 'background.paper', overflow: 'hidden' }}>
        <Box sx={{ position: 'absolute', inset: 0, background: (theme) => `radial-gradient(ellipse at top right, ${alpha(theme.palette.primary.main, 0.08)} 0%, transparent 60%)`, opacity: 0.3, pointerEvents: 'none' }} />
        <Box sx={{ position: 'relative', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
            <Box sx={{ p: 1.25, borderRadius: 1.5, border: 1, color: 'primary.main', bgcolor: (theme) => alpha(theme.palette.primary.main, 0.1), borderColor: (theme) => alpha(theme.palette.primary.main, 0.1), display: 'flex' }}>
              <Bolt size={20} />
            </Box>
            <Box>
              <Typography variant="h5" sx={{ fontWeight: 700, letterSpacing: '-0.02em' }}>{t('evolution.title')}</Typography>
              <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.25 }}>{t('evolution.subtitle')}</Typography>
            </Box>
          </Box>
          {results.length > 0 && (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, px: 1.5, py: 0.75, borderRadius: 2, color: 'success.main', bgcolor: 'rgba(34,197,94,0.1)', border: 1, borderColor: 'rgba(34,197,94,0.2)' }}>
                <CheckCircle size={14} />
                <Typography variant="caption" sx={{ fontWeight: 900 }}>{successCount}</Typography>
              </Box>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, px: 1.5, py: 0.75, borderRadius: 2, color: 'error.main', bgcolor: 'rgba(239,68,68,0.1)', border: 1, borderColor: 'rgba(239,68,68,0.2)' }}>
                <Cancel size={14} />
                <Typography variant="caption" sx={{ fontWeight: 900 }}>{failCount}</Typography>
              </Box>
            </Box>
          )}
        </Box>
      </Box>

      <Box sx={{ mt: 1.5, px: 1.5, py: 1, maxWidth: '5xl', width: '100%', mx: 'auto', border: 1, borderColor: 'divider', borderRadius: 2, bgcolor: 'background.paper' }}>
        <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap' }}>
          {tabs.map((tab) => {
            const active = activeTab === tab.id;
            return (
              <Button
                key={tab.id}
                size="small"
                onClick={() => setActiveTab(tab.id)}
                startIcon={tab.icon}
                sx={{
                  borderRadius: 2,
                  fontWeight: 900,
                  fontSize: '0.875rem',
                  transition: 'all 0.2s',
                  color: active ? 'primary.main' : 'text.secondary',
                  bgcolor: active ? (theme) => alpha(theme.palette.primary.main, 0.1) : 'transparent',
                  border: 1,
                  borderColor: active ? (theme) => alpha(theme.palette.primary.main, 0.2) : 'transparent',
                  '&:hover': active ? { bgcolor: (theme) => alpha(theme.palette.primary.main, 0.1) } : { color: 'text.primary', bgcolor: 'action.hover' },
                }}
              >
                {tab.label}
                {tab.count !== undefined && tab.count > 0 && (
                  <Box component="span" sx={{ ml: 0.5, px: 0.75, py: 0.25, borderRadius: '999px', fontSize: 10, fontWeight: 900, minWidth: 18, textAlign: 'center', color: active ? 'primary.main' : 'text.secondary', bgcolor: active ? (theme) => alpha(theme.palette.primary.main, 0.2) : 'action.hover' }}>
                    {tab.count}
                  </Box>
                )}
              </Button>
            );
          })}
        </Box>
      </Box>

      <Box sx={{ flexGrow: 1, overflowY: 'auto', minHeight: 0 }}>
        <Box sx={{ p: 3, maxWidth: '5xl', mx: 'auto' }}>
          <Box sx={{ display: activeTab === 'test' ? 'block' : 'none' }}>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2.5 }}>
              <Box sx={{ p: 3, bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2, boxShadow: 1, transition: 'box-shadow 0.3s', '&:hover': { boxShadow: 3 } }}>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
                  <Box sx={{ p: 0.75, borderRadius: 1, color: 'primary.main', bgcolor: (theme) => alpha(theme.palette.primary.main, 0.1), display: 'flex' }}>
                    <Globe size={14} />
                  </Box>
                  <Typography variant="body2" sx={{ fontWeight: 900, color: 'text.secondary', textTransform: 'uppercase', letterSpacing: '0.01em' }}>{t('evolution.domain_list')}</Typography>
                  <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace', ml: 'auto' }}>
                    {t('evolution.domain_count', { count: domainCount })}
                  </Typography>
                </Box>
                <TextField
                  multiline
                  rows={6}
                  fullWidth
                  value={domains}
                  onChange={(e) => setDomains(e.target.value)}
                  placeholder={t('evolution.placeholder')}
                  disabled={isRunning}
                  slotProps={{ input: { sx: { fontFamily: 'monospace', fontSize: '0.875rem', p: 2 } } }}
                  sx={{ '& .MuiOutlinedInput-root': { bgcolor: 'action.hover', borderRadius: 1.5 } }}
                />

                <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mt: 2 }}>
                  <Box component="label" sx={{ display: 'flex', alignItems: 'center', gap: 1.25, cursor: ipv6Available ? 'pointer' : 'not-allowed', opacity: ipv6Available ? 1 : 0.5 }}>
                    <Switch
                      checked={enableIPv6}
                      onChange={(e) => setEnableIPv6(e.target.checked)}
                      disabled={isRunning || !ipv6Available}
                      size="small"
                    />
                    <Typography variant="body2" color="text.secondary">
                      {t('evolution.enable_ipv6')}
                      {!ipv6Available && <Box component="span" sx={{ ml: 1, fontSize: 10, color: 'error.main', fontWeight: 900 }}>({t('network.ipv6_disabled_title')})</Box>}
                    </Typography>
                  </Box>

                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    {isRunning ? (
                      <Button variant="outlined" color="error" size="small" startIcon={<Stop size={16} />} onClick={handleStopTest} sx={{ fontWeight: 900, borderRadius: 1.5 }}>
                        {t('evolution.stop_test')}
                      </Button>
                    ) : (
                      <Button variant="contained" size="small" startIcon={isOperating ? <Sync size={16} style={{ animation: `${spin} 1s linear infinite` }} /> : <Play size={16} />} onClick={handleStartTest} disabled={isOperating || domains.trim().length === 0} sx={{ fontWeight: 900, borderRadius: 1.5 }}>
                        {t('evolution.start_test')}
                      </Button>
                    )}
                  </Box>
                </Box>

                {isRunning && (
                  <Box sx={{ mt: 2.5, pt: 2, borderTop: 1, borderColor: 'divider' }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 1.25 }}>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, color: 'text.secondary' }}>
                        <Box sx={{ display: 'flex', color: 'primary.main', animation: `${spin} 1s linear infinite` }}>
                          <Sync size={14} />
                        </Box>
                        <Typography variant="caption" sx={{ fontWeight: 900 }}>{t('evolution.test_progress')}</Typography>
                      </Box>
                      <Typography variant="caption" sx={{ fontFamily: 'monospace', fontWeight: 900, color: 'primary.main', bgcolor: (theme) => alpha(theme.palette.primary.main, 0.1), px: 1, py: 0.25, borderRadius: 0.75, fontSize: 11 }}>
                        {progress.current} / {progress.total}
                      </Typography>
                    </Box>
                    <LinearProgress
                      variant="determinate"
                      value={progressPercent}
                      sx={{
                        height: 10,
                        borderRadius: '999px',
                        bgcolor: 'action.hover',
                        border: 1,
                        borderColor: 'divider',
                        '& .MuiLinearProgress-bar': {
                          borderRadius: '999px',
                          background: (theme) => `linear-gradient(90deg, ${theme.palette.primary.main}, ${theme.palette.primary.light}, ${theme.palette.primary.main})`,
                          backgroundSize: '200% 100%',
                          animation: `${shimmer} 2s linear infinite`,
                        },
                      }}
                    />
                    {progress.current > 0 && (
                      <Typography variant="caption" color="text.secondary" sx={{ mt: 1.25, display: 'block', textAlign: 'center', fontWeight: 900 }}>
                        {t('evolution.testing_domain', { current: progress.current })}
                      </Typography>
                    )}
                  </Box>
                )}
              </Box>

              <Box sx={{ p: 2.5, bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2, boxShadow: 1 }}>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1.5 }}>
                  <Box sx={{ p: 0.75, borderRadius: 1, color: 'primary.main', bgcolor: (theme) => alpha(theme.palette.primary.main, 0.1), display: 'flex' }}>
                    <Wifi size={14} />
                  </Box>
                  <Typography variant="body2" sx={{ fontWeight: 900, color: 'text.secondary', textTransform: 'uppercase', letterSpacing: '0.01em' }}>{t('evolution.test_flow')}</Typography>
                </Box>
                <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, alignItems: 'center' }}>
                  {flowSteps.map((step, i) => (
                    <React.Fragment key={step}>
                      <Typography variant="caption" sx={{ px: 1.25, py: 0.5, bgcolor: 'action.hover', border: 1, borderColor: 'divider', borderRadius: 1, color: 'text.secondary', fontWeight: 500 }}>
                        {step}
                      </Typography>
                      {i < 5 && (
                        <Box component="span" sx={{ display: 'inline-flex', alignSelf: 'center', color: 'text.secondary', opacity: 0.4 }}>
                          <ArrowForward size={12} />
                        </Box>
                      )}
                    </React.Fragment>
                  ))}
                </Box>
                <Typography variant="caption" color="text.secondary" sx={{ mt: 1.5, display: 'block' }}>
                  {t('evolution.test_flow_desc')}
                </Typography>
              </Box>
            </Box>
          </Box>

          <Box sx={{ display: activeTab === 'rules' ? 'block' : 'none' }}>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
              {tempRules.length === 0 ? (
                <Box sx={{ p: 3, bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2, boxShadow: 1 }}>
                  <Box sx={{ textAlign: 'center', py: 7 }}>
                    <Box sx={{ width: 64, height: 64, mx: 'auto', mb: 2, borderRadius: 2, bgcolor: 'action.hover', border: 1, borderColor: 'divider', display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'text.disabled', opacity: 0.5 }}>
                      <Description size={28} />
                    </Box>
                    <Typography variant="body2" color="text.secondary" sx={{ fontWeight: 500 }}>{t('evolution.empty_rules')}</Typography>
                    <Typography variant="caption" sx={{ mt: 0.75, color: 'text.disabled' }}>{t('evolution.empty_rules_desc')}</Typography>
                  </Box>
                </Box>
              ) : (
                <>
                  <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <Box sx={{ color: 'primary.main', display: 'flex' }}>
                        <Description size={16} />
                      </Box>
                      <Typography variant="body2" sx={{ fontWeight: 900 }}>{t('evolution.temp_rules')}</Typography>
                      <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace', bgcolor: 'action.hover', px: 1, py: 0.25, borderRadius: 0.75 }}>{tempRules.length}</Typography>
                    </Box>
                    <Typography variant="caption" color="text.secondary">
                      {t('evolution.applied_count', { applied: tempRules.filter(r => r.is_applied).length, total: tempRules.length })}
                    </Typography>
                  </Box>
                  <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                    {tempRules.map((rule) => {
                      const mc = getMethodColors(rule.method);
                      return (
                        <Box key={rule.id} sx={{ p: 2, bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 1.5, transition: 'all 0.2s', '&:hover': { borderColor: (theme) => alpha(theme.palette.primary.main, 0.3), boxShadow: 2 } }}>
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                            <Box sx={{ p: 1, borderRadius: 1, color: mc.color, bgcolor: mc.bg, border: 1, borderColor: mc.border, display: 'flex', transition: 'transform 0.2s', '&:hover': { transform: 'scale(1.1)' } }}>
                              {getMethodIcon(rule.method)}
                            </Box>
                            <Box sx={{ flex: 1, minWidth: 0 }}>
                              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                <Typography variant="body2" sx={{ fontWeight: 900, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{rule.name}</Typography>
                                <Typography variant="caption" sx={{ fontSize: 10, px: 1, py: 0.25, borderRadius: '999px', border: 1, fontWeight: 900, color: mc.color, bgcolor: mc.bg, borderColor: mc.border }}>
                                  {getMethodLabel(rule.method)}
                                </Typography>
                                {rule.is_applied && (
                                  <Typography variant="caption" sx={{ fontSize: 10, px: 1, py: 0.25, borderRadius: '999px', border: 1, fontWeight: 900, color: 'success.main', bgcolor: 'rgba(34,197,94,0.1)', borderColor: 'rgba(34,197,94,0.2)', display: 'inline-flex', alignItems: 'center', gap: 0.5 }}>
                                    <CheckSquare size={12} />
                                    {t('evolution.applied')}
                                  </Typography>
                                )}
                              </Box>
                              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mt: 0.5 }}>
                                <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>{rule.domain}</Typography>
                                {rule.sni_fake && (
                                  <>
                                    <Typography variant="caption" sx={{ color: 'divider' }}>|</Typography>
                                    <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>{t('evolution.sni')}: {rule.sni_fake}</Typography>
                                  </>
                                )}
                              </Box>
                            </Box>
                            {!rule.is_applied && (
                              <Button size="small" startIcon={<ArrowForward size={12} />} onClick={() => handleApplyRule(rule)} sx={{ fontSize: 12, fontWeight: 900, borderRadius: 1, color: 'primary.main', bgcolor: (theme) => alpha(theme.palette.primary.main, 0.1), border: 1, borderColor: (theme) => alpha(theme.palette.primary.main, 0.2), '&:hover': { bgcolor: (theme) => alpha(theme.palette.primary.main, 0.2) } }}>
                                {t('evolution.convert_to_rule')}
                              </Button>
                            )}
                          </Box>
                        </Box>
                      );
                    })}
                  </Box>
                </>
              )}
            </Box>
          </Box>

          <Box sx={{ display: activeTab === 'results' ? 'block' : 'none' }}>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
              {results.length === 0 ? (
                <Box sx={{ p: 3, bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2, boxShadow: 1 }}>
                  <Box sx={{ textAlign: 'center', py: 7 }}>
                    <Box sx={{ width: 64, height: 64, mx: 'auto', mb: 2, borderRadius: 2, bgcolor: 'action.hover', border: 1, borderColor: 'divider', display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'text.disabled', opacity: 0.5 }}>
                      <Activity size={28} />
                    </Box>
                    <Typography variant="body2" color="text.secondary" sx={{ fontWeight: 500 }}>{t('evolution.empty_results')}</Typography>
                    <Typography variant="caption" sx={{ mt: 0.75, color: 'text.disabled' }}>{t('evolution.empty_results_desc')}</Typography>
                  </Box>
                </Box>
              ) : (
                <>
                  <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <Box sx={{ color: 'primary.main', display: 'flex' }}>
                        <Activity size={16} />
                      </Box>
                      <Typography variant="body2" sx={{ fontWeight: 900 }}>{t('evolution.test_results')}</Typography>
                      <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace', bgcolor: 'action.hover', px: 1, py: 0.25, borderRadius: 0.75 }}>{results.length}</Typography>
                    </Box>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, color: 'success.main' }}>
                        <CheckCircle size={14} />
                        <Typography variant="caption" sx={{ fontWeight: 900 }}>{t('evolution.success_count', { count: successCount })}</Typography>
                      </Box>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, color: 'error.main' }}>
                        <Cancel size={14} />
                        <Typography variant="caption" sx={{ fontWeight: 900 }}>{t('evolution.fail_count', { count: failCount })}</Typography>
                      </Box>
                    </Box>
                  </Box>
                  <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                    {results.map((result, index) => {
                      const mc = getMethodColors(result.method);
                      return (
                        <Box
                          key={index}
                          sx={{
                            p: 2,
                            border: 1,
                            borderRadius: 1.5,
                            transition: 'all 0.2s',
                            ...(result.reachable
                              ? { bgcolor: 'background.paper', borderColor: 'divider', '&:hover': { borderColor: (theme) => alpha(theme.palette.primary.main, 0.3), boxShadow: 2 } }
                              : { bgcolor: 'rgba(239,68,68,0.05)', borderColor: 'rgba(239,68,68,0.2)', '&:hover': { borderColor: 'rgba(239,68,68,0.4)' } }),
                          }}
                        >
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
                            <Box sx={{ p: 1, borderRadius: 1, display: 'flex', transition: 'transform 0.2s', '&:hover': { transform: 'scale(1.1)' }, color: result.reachable ? mc.color : 'error.main', bgcolor: result.reachable ? mc.bg : 'rgba(239,68,68,0.1)', border: 1, borderColor: result.reachable ? mc.border : 'rgba(239,68,68,0.2)' }}>
                              {getMethodIcon(result.method)}
                            </Box>

                            <Box sx={{ flex: 1, minWidth: 0 }}>
                              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
                                <Typography variant="body2" sx={{ fontWeight: 900 }}>{result.domain}</Typography>
                                {result.reachable ? (
                                  <>
                                    <Typography variant="caption" sx={{ fontSize: 10, px: 1, py: 0.25, borderRadius: '999px', border: 1, fontWeight: 900, color: mc.color, bgcolor: mc.bg, borderColor: mc.border }}>
                                      {getMethodLabel(result.method)}
                                    </Typography>
                                    {result.is_cloudflare && (
                                      <Typography variant="caption" sx={{ fontSize: 10, px: 1, py: 0.25, borderRadius: '999px', border: 1, fontWeight: 900, color: 'warning.main', bgcolor: 'rgba(234,88,12,0.1)', borderColor: 'rgba(234,88,12,0.2)' }}>
                                        {t('evolution.cloudflare')}
                                      </Typography>
                                    )}
                                  </>
                                ) : (
                                  <Typography variant="caption" sx={{ fontSize: 10, px: 1, py: 0.25, borderRadius: '999px', border: 1, fontWeight: 900, color: 'error.main', bgcolor: 'rgba(239,68,68,0.1)', borderColor: 'rgba(239,68,68,0.2)' }}>
                                    {t('evolution.unreachable')}
                                  </Typography>
                                )}
                              </Box>
                              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mt: 0.5 }}>
                                {result.reachable ? (
                                  <>
                                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, color: 'text.secondary' }}>
                                      <Timer size={12} />
                                      <Typography variant="caption" color="text.secondary">{t('evolution.delay', { ms: Math.round(result.delay / 1000000) })}</Typography>
                                    </Box>
                                    {result.best_ip && <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>{result.best_ip}</Typography>}
                                  </>
                                ) : (
                                  result.error && <Typography variant="caption" sx={{ color: 'error.main' }}>{result.error}</Typography>
                                )}
                              </Box>
                            </Box>

                            <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace', flexShrink: 0, opacity: 0.6, transition: 'opacity 0.2s', '&:hover': { opacity: 1 } }}>
                              {new Date(result.timestamp).toLocaleTimeString()}
                            </Typography>
                          </Box>

                          {result.step_results && result.step_results.length > 0 && (
                            <Box sx={{ mt: 1.5, pt: 1.5, borderTop: 1, borderColor: 'divider' }}>
                              <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.75 }}>
                                {result.step_results.map((step, i) => (
                                  <Box key={i} sx={{ display: 'flex', alignItems: 'center', gap: 0.5, fontSize: 10, px: 1, py: 0.5, borderRadius: '999px', border: 1, fontWeight: 900, color: step.success ? 'success.main' : 'error.main', bgcolor: step.success ? 'rgba(34,197,94,0.1)' : 'rgba(239,68,68,0.1)', borderColor: step.success ? 'rgba(34,197,94,0.2)' : 'rgba(239,68,68,0.2)' }}>
                                    {step.success ? <CheckCircle size={12} /> : <Cancel size={12} />}
                                    {getStepLabel(step.step_name)}
                                  </Box>
                                ))}
                              </Box>
                            </Box>
                          )}
                        </Box>
                      );
                    })}
                  </Box>
                </>
              )}
            </Box>
          </Box>
        </Box>
      </Box>

      <Modal
        isOpen={showRuleModal}
        onClose={() => { setShowRuleModal(false); setSelectedRule(null); }}
        title={t('evolution.confirm_title')}
      >
        {selectedRule && (
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            <Typography variant="body2" color="text.secondary">
              {t('evolution.confirm_desc')}
            </Typography>
            <Box sx={{ p: 2, bgcolor: 'action.hover', border: 1, borderColor: 'divider', borderRadius: 1.5 }}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                <Typography variant="body2" sx={{ fontWeight: 900 }}>{selectedRule.name}</Typography>
                <Typography variant="caption" sx={{ fontSize: 10, px: 1, py: 0.25, borderRadius: '999px', border: 1, fontWeight: 900, color: getMethodColors(selectedRule.method).color, bgcolor: getMethodColors(selectedRule.method).bg, borderColor: getMethodColors(selectedRule.method).border }}>
                  {getMethodLabel(selectedRule.method)}
                </Typography>
              </Box>
              <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
                {t('evolution.domain')}: {selectedRule.domain}
                {selectedRule.sni_fake && <Box component="span"> | {t('evolution.sni')}: {selectedRule.sni_fake}</Box>}
              </Typography>
            </Box>
            <Box sx={{ display: 'flex', gap: 1.5, justifyContent: 'flex-end' }}>
              <Button variant="outlined" size="small" onClick={() => { setShowRuleModal(false); setSelectedRule(null); }} sx={{ fontWeight: 900, borderRadius: 1.5 }}>
                {t('common.cancel')}
              </Button>
              <Button variant="contained" size="small" onClick={confirmApplyRule} sx={{ fontWeight: 900, borderRadius: 1.5 }}>
                {t('evolution.confirm_convert')}
              </Button>
            </Box>
          </Box>
        )}
      </Modal>
    </Box>
  );
};

export default Evolution;
