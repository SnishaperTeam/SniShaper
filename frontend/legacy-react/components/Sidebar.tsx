import React, { useMemo } from 'react';
import { NavLink } from 'react-router-dom';
import { Box, Drawer, IconButton, Tooltip, Typography, useColorScheme } from '@mui/material';
import { LightMode, DarkMode } from '@mui/icons-material';
import logoUrl from '../assets/logo.svg';
import {
  LayoutDashboard,
  ShieldCheck,
  Activity,
  FileText,
  Settings,
  Workflow,
  Globe,
  Antenna,
  Info,
  Zap,
} from '../lib/icons';
import { useTranslation } from '../i18n/I18nContext';

const DRAWER_WIDTH = 240;

const getNavItems = (t: any) => [
  { path: '/dashboard', label: t('sidebar.dashboard'), icon: LayoutDashboard },
  { path: '/proxies', label: t('sidebar.proxies'), icon: Globe },
  { path: '/rules', label: t('sidebar.rules'), icon: ShieldCheck },
  { path: '/routing', label: t('sidebar.routing'), icon: Workflow },
  { path: '/dns', label: t('sidebar.dns'), icon: Antenna },
  { path: '/evolution', label: t('evolution.title'), icon: Zap },
  { path: '/logs', label: t('sidebar.logs'), icon: FileText },
  { path: '/settings', label: t('sidebar.settings'), icon: Settings },
  { path: '/about', label: t('sidebar.about'), icon: Info },
];

const SidebarContent: React.FC = () => {
  const { t } = useTranslation();
  const { mode, setMode } = useColorScheme();
  const navItems = useMemo(() => getNavItems(t), [t]);

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, p: 2, borderBottom: 1, borderColor: 'divider' }}>
        <img src={logoUrl} alt="SniShaper" width={32} height={32} style={{ objectFit: 'contain' }} />
        <Typography
          variant="subtitle1"
          noWrap
          sx={{
            fontWeight: 700,
            letterSpacing: "0.12em"
          }}>
          SniShaper
        </Typography>
      </Box>

      <Box component="nav" aria-label={t('sidebar.nav_label')} sx={{ flexGrow: 1, display: 'flex', flexDirection: 'column', gap: 0.5, px: 1, py: 1, overflowY: 'auto' }}>
        {navItems.map((item) => (
          <NavLink key={item.path} to={item.path} style={{ textDecoration: 'none', color: 'inherit' }}>
            {({ isActive }) => (
              <Box
                sx={{
                  position: 'relative',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 1.5,
                  px: 1.5,
                  py: 1,
                  borderRadius: 1,
                  bgcolor: isActive ? 'primary.main' : 'transparent',
                  color: isActive ? 'primary.contrastText' : 'text.secondary',
                  transition: 'background-color 0.15s',
                  boxShadow: isActive ? 'inset 0 1px 0 rgba(255,255,255,0.1)' : 'none',
                  '&::before': isActive
                    ? {
                        content: '""',
                        position: 'absolute',
                        left: 0,
                        top: '50%',
                        transform: 'translateY(-50%)',
                        width: 3,
                        height: '60%',
                        borderRadius: 2,
                        bgcolor: 'primary.contrastText',
                        opacity: 0.85,
                      }
                    : undefined,
                  '&:hover': {
                    bgcolor: isActive ? 'primary.main' : 'action.hover',
                    color: isActive ? 'primary.contrastText' : 'text.primary',
                  },
                }}
              >
                <item.icon size={18} aria-hidden />
                <Typography
                  variant="body2"
                  sx={{
                    fontWeight: isActive ? 700 : 500,
                    textAlign: 'left'
                  }}>
                  {item.label}
                </Typography>
              </Box>
            )}
          </NavLink>
        ))}
      </Box>

      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 1, p: 1.5, borderTop: 1, borderColor: 'divider' }}>
        <Tooltip title={mode === 'light' ? t('sidebar.dark_mode') : t('sidebar.light_mode')}>
          <IconButton
            size="small"
            aria-label={mode === 'light' ? t('sidebar.dark_mode_aria') : t('sidebar.light_mode_aria')}
            onClick={() => setMode(mode === 'light' ? 'dark' : 'light')}
            color={mode === 'dark' ? 'primary' : 'default'}
          >
            {mode === 'light' ? <LightMode fontSize="small" /> : <DarkMode fontSize="small" />}
          </IconButton>
        </Tooltip>
        <Typography
          variant="caption"
          sx={{
            color: "text.secondary",
            minWidth: 48,
            textAlign: 'center'
          }}>
          {mode === 'dark' ? t('sidebar.dark_mode') : t('sidebar.light_mode')}
        </Typography>
      </Box>
    </Box>
  );
};

const Sidebar: React.FC = React.memo(() => {
  return (
    <Drawer
      variant="permanent"
      open
      sx={{
        width: DRAWER_WIDTH,
        flexShrink: 0,
        '& .MuiDrawer-paper': { width: DRAWER_WIDTH, boxSizing: 'border-box' },
      }}
    >
      <SidebarContent />
    </Drawer>
  );
});

export { SidebarContent };
export default Sidebar;
