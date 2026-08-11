import React, { useState, useEffect } from 'react';
import {
  Workflow,
  ShieldCheck,
  Activity,
  Share2,
  RefreshCw,
  Power,
  Zap
} from '../lib/icons';
import {
  GetAutoRoutingConfig,
  UpdateAutoRoutingConfig,
  GetAutoRoutingStatus,
  RefreshGFWList,
  EventsOn
} from '../api/bindings';
import { toast } from '../lib/toast';
import { extractErrorMessage } from '../lib/utils';
import { useTranslation } from '../i18n/I18nContext';
import { Box, Typography, Button } from '@mui/material';
import { alpha } from '@mui/material/styles';
import { keyframes } from '@emotion/react';

interface FlowEntry {
  id: string;
  domain: string;
  mode: string;
  modeClass: string;
  modeDisplay: string;
}

const pulse = keyframes`0%,100%{opacity:1}50%{opacity:0.4}`;
const spin = keyframes`from{transform:rotate(0deg)}to{transform:rotate(360deg)}`;

const Routing: React.FC = () => {
  const { t } = useTranslation();
  const [flows, setFlows] = useState<FlowEntry[]>([]);
  const [config, setConfig] = useState<any>({ mode: '', gfwlist_url: '' });
  const [status, setStatus] = useState<any>({ enabled: false, domain_count: 0 });
  useEffect(() => {
    const loadData = async () => {
      const cfg = await GetAutoRoutingConfig();
      const s = await GetAutoRoutingStatus();
      setConfig(cfg);
      setStatus(s);
    };
    loadData();

    const unoff = EventsOn("app:route", (data: any) => {
        if (!data) return;
        const { domain, mode } = data;
        if (!domain || !mode) return;

        let modeClass = 'success.main';
        let modeDisplay = mode.toUpperCase();
        const lowerMode = mode.toLowerCase();

        if (lowerMode.includes('direct') || lowerMode.includes('tcp')) {
            modeClass = 'rgba(107,114,128,0.9)'; modeDisplay = 'DIRECT';
        }
        else if (lowerMode.includes('transparent')) modeClass = 'error.main';
        else if (lowerMode.includes('mitm') || lowerMode.includes('proxy')) modeClass = 'warning.main';

        setFlows(prev => {
            const newFlow: FlowEntry = {
                id: crypto.randomUUID(),
                domain,
                mode,
                modeClass,
                modeDisplay
            };
            return [newFlow, ...prev].slice(0, 40);
        });
    });

    return () => unoff();
  }, []);

  const handleSave = async () => {
    await UpdateAutoRoutingConfig(config);
    toast.success(t('routing.notifications.saved'), t('routing.notifications.saved_desc'));
  };

  const [refreshing, setRefreshing] = useState(false);

  const handleRefreshGFW = async () => {
    setRefreshing(true);
    try {
      await RefreshGFWList();
      const s = await GetAutoRoutingStatus();
      setStatus(s);
      toast.success(t('routing.notifications.updated'), t('routing.notifications.updated_desc', { count: s?.domain_count || 0 }));
    } catch (e: any) {
      toast.error(t('common.error'), extractErrorMessage(e));
    } finally {
      setRefreshing(false);
    }
  };

  return (
    <Box sx={{ pt: 4, pb: 6, px: 3, maxWidth: '5xl', mx: 'auto', display: 'flex', flexDirection: 'column', gap: 4 }}>
      <Typography variant="h5" sx={{ fontWeight: 700, letterSpacing: '-0.02em' }}>{t('routing.title')}</Typography>

      <Box sx={{ bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2, overflow: 'hidden', display: 'flex', flexDirection: 'column', height: 340 }}>
        <Box sx={{ px: 3, py: 2, borderBottom: 1, borderColor: 'divider', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, color: 'text.secondary', fontWeight: 700, fontSize: '0.75rem', textTransform: 'uppercase', letterSpacing: '0.1em' }}>
                <Box sx={{ display: 'inline-flex', color: 'success.main', animation: `${pulse} 1.5s ease-in-out infinite` }}><Activity size={14} /></Box>
                {t('routing.traffic_flow')}
            </Box>
        </Box>
        <Box sx={{ flex: 1, overflowY: 'auto', p: 2, display: 'flex', flexWrap: 'wrap', alignContent: 'flex-start', gap: 1.5, position: 'relative' }}>
            {flows.length === 0 && (
                <Box sx={{ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', color: 'text.secondary', opacity: 0.4 }}>
                    <Share2 size={40} strokeWidth={1} />
                    <Typography variant="caption" sx={{ mt: 1.5, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.1em' }}>{t('routing.waiting_traffic')}</Typography>
                </Box>
            )}
            {flows.map((flow) => (
                <Box
                    key={flow.id}
                    sx={{ display: 'flex', alignItems: 'center', gap: 1, px: 1.5, py: 0.75, bgcolor: 'action.hover', border: 1, borderColor: 'divider', borderRadius: '999px', boxShadow: 1 }}
                >
                    <Typography variant="caption" sx={{ fontWeight: 700, maxWidth: 150, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={flow.domain}>
                        {flow.domain}
                    </Typography>
                    <Typography variant="caption" sx={{ color: 'text.secondary', fontSize: '0.625rem' }}>➔</Typography>
                    <Typography variant="caption" sx={{ px: 1, py: 0.25, borderRadius: '999px', fontSize: '0.5625rem', fontWeight: 900, textTransform: 'uppercase', color: 'common.white', bgcolor: flow.modeClass, boxShadow: 1 }}>
                        {flow.modeDisplay}
                    </Typography>
                </Box>
            ))}
        </Box>
      </Box>

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', lg: '1fr 1fr' }, gap: 4 }}>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
             <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, px: 0.5 }}>
                <Box sx={{ display: 'inline-flex', color: 'primary.main' }}><Workflow size={18} /></Box>
                <Typography variant="body2" sx={{ fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.05em', color: 'text.secondary' }}>{t('routing.strategy')}</Typography>
             </Box>
             <Box sx={{ bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2, p: 2, display: 'flex', flexDirection: 'column', gap: 3 }}>
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
                    <Typography variant="caption" sx={{ fontSize: '0.625rem', fontWeight: 900, color: 'text.secondary', textTransform: 'uppercase', letterSpacing: '0.2em', px: 0.5 }}>{t('routing.mode_switch')}</Typography>
                    <Box sx={{ display: 'grid', gridTemplateColumns: { xs: 'repeat(2, 1fr)', sm: 'repeat(4, 1fr)' }, gap: 1.5 }}>
                        {[
                            { id: '', label: t('routing.modes.off'), icon: Power, color: 'text.secondary' },
                            { id: 'default', label: t('routing.modes.smart'), icon: Zap, color: 'success.main' }
                        ].map((opt) => {
                            const active = config.mode === opt.id;
                            return (
                                <Button
                                    key={opt.id}
                                    onClick={() => setConfig({ ...config, mode: opt.id })}
                                    variant="outlined"
                                    sx={{
                                        display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1, p: 1.5, borderRadius: 2,
                                        borderColor: active ? 'primary.main' : 'divider',
                                        bgcolor: active ? (theme) => alpha(theme.palette.primary.main, 0.05) : 'background.paper',
                                        color: active ? 'primary.main' : 'text.secondary',
                                        boxShadow: active ? 1 : 'none',
                                        '&:hover': { borderColor: active ? 'primary.main' : (theme) => alpha(theme.palette.primary.main, 0.3) },
                                    }}
                                >
                                    <Box sx={{ width: 28, height: 28, borderRadius: 1, bgcolor: 'action.hover', display: 'flex', alignItems: 'center', justifyContent: 'center', color: active ? 'primary.main' : opt.color }}>
                                        <opt.icon size={16} />
                                    </Box>
                                    <Typography variant="caption" sx={{ fontSize: '0.625rem', fontWeight: 900, textTransform: 'uppercase', letterSpacing: '0.1em' }}>{opt.label}</Typography>
                                </Button>
                            );
                        })}
                    </Box>
                </Box>

                <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderTop: 1, borderColor: 'divider', pt: 2 }}>
                    <Box>
                        <Typography variant="body2" sx={{ fontWeight: 700 }}>{t('routing.gfwlist_check')}</Typography>
                        <Typography variant="caption" sx={{ fontSize: '0.6875rem', color: 'text.secondary', display: 'block', mt: 0.25 }}>
                            {status.enabled ? t('routing.preloaded', { count: status.domain_count }) : t('routing.inactive')}
                        </Typography>
                    </Box>
                    <Button
                        onClick={handleRefreshGFW}
                        disabled={refreshing}
                        variant="outlined"
                        size="small"
                        sx={{
                            borderRadius: 2, fontSize: '0.625rem', fontWeight: 900, textTransform: 'uppercase',
                            letterSpacing: '0.1em', bgcolor: 'action.hover', borderColor: 'divider', color: 'text.secondary',
                            '&:hover': { bgcolor: 'primary.main', color: 'common.white', borderColor: 'primary.main' },
                        }}
                        startIcon={
                            <Box sx={{ display: 'inline-flex', animation: refreshing ? `${spin} 1s linear infinite` : 'none' }}><RefreshCw size={12} /></Box>
                        }
                    >
                        {refreshing ? t('ech_form.probing') : t('routing.update_list')}
                    </Button>
                </Box>
                <Button
                    onClick={handleSave}
                    variant="contained"
                    color="primary"
                    fullWidth
                    sx={{ py: 1.5, borderRadius: 2, fontWeight: 900, boxShadow: (theme) => `0 8px 16px ${alpha(theme.palette.primary.main, 0.2)}`, transition: 'transform 0.15s', '&:hover': { transform: 'scale(1.01)' }, '&:active': { transform: 'scale(0.99)' } }}
                >
                    {t('routing.save_apply')}
                </Button>
             </Box>
        </Box>

        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
             <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, px: 0.5 }}>
                <Box sx={{ display: 'inline-flex', color: 'success.main' }}><ShieldCheck size={18} /></Box>
                <Typography variant="body2" sx={{ fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.05em', color: 'text.secondary' }}>{t('routing.features')}</Typography>
             </Box>
             <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                {[
                    { icon: Zap, title: t('routing.feature_smart'), color: 'success.main', desc: t('routing.feature_smart_desc') },
                    { icon: Activity, title: t('routing.feature_priority'), color: 'secondary.main', desc: t('routing.feature_priority_desc') }
                ].map((item, i) => (
                    <Box key={i} sx={{ p: 2, bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: 2, display: 'flex', gap: 2, transition: 'border-color 0.15s', '&:hover': { borderColor: (theme) => alpha(theme.palette.primary.main, 0.4) } }}>
                        <Box sx={{ flexShrink: 0, width: 40, height: 40, borderRadius: 2, display: 'flex', alignItems: 'center', justifyContent: 'center', bgcolor: 'action.hover', color: item.color }}>
                            <item.icon size={20} />
                        </Box>
                        <Box>
                            <Typography variant="body2" sx={{ fontWeight: 700 }}>{item.title}</Typography>
                            <Typography variant="caption" sx={{ fontSize: '0.6875rem', color: 'text.secondary', display: 'block', mt: 0.5, lineHeight: 1.6, fontWeight: 500 }}>{item.desc}</Typography>
                        </Box>
                    </Box>
                ))}
             </Box>
        </Box>
      </Box>
    </Box>
  );
};

export default Routing;
