import { createTheme } from '@mui/material/styles';

const base = {
  typography: {
    fontFamily: '"Inter", "Roboto", "Helvetica", "Arial", sans-serif',
  },
  shape: { borderRadius: 8 },
  components: {
    MuiButton: { styleOverrides: { root: { textTransform: 'none' } } },
  },
};

const lightText = { primary: 'rgba(0,0,0,0.87)', secondary: 'rgba(0,0,0,0.6)' };
const darkText = { primary: '#ffffff', secondary: 'rgba(255,255,255,0.7)' };

export const defaultTheme = createTheme({
  ...base,
  colorSchemes: {
    light: {
      palette: {
        primary: { main: '#1976d2', light: '#42a5f5', dark: '#1565c0' },
        secondary: { main: '#9c27b0', light: '#ba68c8', dark: '#7b1fa2' },
        background: { default: '#ffffff', paper: '#f5f5f5' },
        text: lightText,
      },
    },
    dark: {
      palette: {
        primary: { main: '#90caf9', light: '#e3f2fd', dark: '#42a5f5' },
        secondary: { main: '#ce93d8', light: '#f3e5f5', dark: '#ab47bc' },
        background: { default: '#121212', paper: '#1e1e1e' },
        text: darkText,
      },
    },
  },
});

export const sniShaperTheme = createTheme({
  ...base,
  colorSchemes: {
    light: {
      palette: {
        primary: { main: '#0b7bff', light: '#3396ff', dark: '#0969da', contrastText: '#ffffff' },
        secondary: { main: '#9c27b0', light: '#ba68c8', dark: '#7b1fa2' },
        background: { default: '#ffffff', paper: '#f5f5f5' },
        text: lightText,
      },
    },
    dark: {
      palette: {
        primary: { main: '#0b7bff', light: '#3396ff', dark: '#0969da', contrastText: '#ffffff' },
        secondary: { main: '#ce93d8', light: '#f3e5f5', dark: '#ab47bc' },
        background: { default: '#0d1117', paper: '#161b22' },
        text: { primary: '#e6edf3', secondary: '#8b949e' },
      },
    },
  },
});

export const forestTheme = createTheme({
  ...base,
  colorSchemes: {
    light: {
      palette: {
        primary: { main: '#2e7d32', light: '#4caf50', dark: '#1b5e20', contrastText: '#ffffff' },
        secondary: { main: '#00695c', light: '#26a69a', dark: '#004d40' },
        background: { default: '#f7faf7', paper: '#ffffff' },
        text: lightText,
      },
    },
    dark: {
      palette: {
        primary: { main: '#66bb6a', light: '#81c784', dark: '#388e3c', contrastText: '#0b160c' },
        secondary: { main: '#26a69a', light: '#4db6ac', dark: '#00897b' },
        background: { default: '#0f1a12', paper: '#152219' },
        text: { primary: '#e4efe6', secondary: '#93a99a' },
      },
    },
  },
});

export const sunsetTheme = createTheme({
  ...base,
  colorSchemes: {
    light: {
      palette: {
        primary: { main: '#ed6c02', light: '#ff9800', dark: '#c25e00', contrastText: '#ffffff' },
        secondary: { main: '#6d4c41', light: '#8d6e63', dark: '#4e342e' },
        background: { default: '#fffaf4', paper: '#ffffff' },
        text: lightText,
      },
    },
    dark: {
      palette: {
        primary: { main: '#ff9800', light: '#ffb74d', dark: '#e65100', contrastText: '#1a0f00' },
        secondary: { main: '#a1887f', light: '#bcaaa4', dark: '#795548' },
        background: { default: '#1a1410', paper: '#211a14' },
        text: { primary: '#f2ece5', secondary: '#b3a89c' },
      },
    },
  },
});

export const oceanTheme = createTheme({
  ...base,
  colorSchemes: {
    light: {
      palette: {
        primary: { main: '#00897b', light: '#26a69a', dark: '#00695c', contrastText: '#ffffff' },
        secondary: { main: '#0277bd', light: '#29b6f6', dark: '#01579b' },
        background: { default: '#f4faf9', paper: '#ffffff' },
        text: lightText,
      },
    },
    dark: {
      palette: {
        primary: { main: '#4db6ac', light: '#80cbc4', dark: '#26a69a', contrastText: '#0a1a18' },
        secondary: { main: '#4fc3f7', light: '#81d4fa', dark: '#0288d1' },
        background: { default: '#0d1a19', paper: '#132322' },
        text: { primary: '#e0efed', secondary: '#8fb0ac' },
      },
    },
  },
});

export const graphiteTheme = createTheme({
  ...base,
  colorSchemes: {
    light: {
      palette: {
        primary: { main: '#546e7a', light: '#78909c', dark: '#37474f', contrastText: '#ffffff' },
        secondary: { main: '#8d6e63', light: '#a1887f', dark: '#6d4c41' },
        background: { default: '#f7f8fa', paper: '#ffffff' },
        text: lightText,
      },
    },
    dark: {
      palette: {
        primary: { main: '#90a4ae', light: '#b0bec5', dark: '#607d8b', contrastText: '#101418' },
        secondary: { main: '#a1887f', light: '#bcaaa4', dark: '#795548' },
        background: { default: '#121418', paper: '#1a1d22' },
        text: { primary: '#e8eaee', secondary: '#9aa1ab' },
      },
    },
  },
});

export const availableThemes = [
  { id: 'default', nameKey: 'settings.theme.default', theme: defaultTheme },
  { id: 'sniShaper', nameKey: 'settings.theme.sniShaper', theme: sniShaperTheme },
  { id: 'forest', nameKey: 'settings.theme.forest', theme: forestTheme },
  { id: 'sunset', nameKey: 'settings.theme.sunset', theme: sunsetTheme },
  { id: 'ocean', nameKey: 'settings.theme.ocean', theme: oceanTheme },
  { id: 'graphite', nameKey: 'settings.theme.graphite', theme: graphiteTheme },
];

export const appTheme = defaultTheme;
