import type { Plugin } from 'vite';

// Dev-only mock backend for `npm run dev` in a plain browser.
// Enabled via SNISHAPER_MOCK=1 in vite.config.ts; never active in production builds.
// Mirrors the wire format of @wailsio/runtime's default HTTP transport:
// POST /wails/runtime {object:0, method:0, args:{methodName, args}} -> JSON value.

const mocks: Record<string, unknown> = {
  // ---- App bootstrap (App.tsx fastLoad / slowLoad) ----
  GetLanguage: 'zh',
  GetListenPort: 8080,
  GetListenAddress: '127.0.0.1',
  GetCloseToTray: true,
  GetAutoStart: false,
  GetAutoStartMethod: 'task',
  GetShowMainWindowOnAutoStart: true,
  GetAutoEnableProxyOnAutoStart: false,
  GetAutoEnableSysProxyOnAutoStart: true,
  GetAutoUpdateRules: true,
  GetCloudflareConfig: { api_key: '', auto_update: true, preferred_ips: ['104.16.1.1', '104.16.2.2'] },
  GetIPv6Available: true,
  GetTheme: 'default',
  GetTutorialDone: true,
  GetAppearanceMode: 'dark',
  GetNetworkRecoveryReport: { detected: false },

  // ---- Dashboard ----
  IsProxyRunning: true,
  GetProxyMode: 'rule',
  GetSystemProxyStatus: { enabled: true, server: '127.0.0.1:8080', managed: true },
  GetStats: { activeConnections: 12, totalRequests: 15234, blockedRequests: 89, trafficUp: 1024 * 1024 * 45, trafficDown: 1024 * 1024 * 512 },
  GetNotices: [],
  GetTUNStatus: { supported: true, running: false, enabled: false, message: '' },
  GetTUNConfig: { enabled: false, mtu: 9000, dns_hijack: true, auto_route: true, strict_route: true, stack: 'gvisor', adapterName: 'SniShaper', interfaceName: '', excludeInterfaces: [] },

  // ---- Certs ----
  GetCAInstallStatus: { Installed: true, CertPath: 'C:\\snishaper\\ca.pem', Platform: 'windows' },
  GetInstalledCerts: [],
  GetCACertPEM: '-----BEGIN CERTIFICATE-----\nMOCK\n-----END CERTIFICATE-----\n',

  // ---- Cloudflare ----
  GetCloudflareIPStats: [
    { ip: '104.16.1.1', latencyMs: 12, successRate: 99, lastCheck: '2026-11-27T23:00:00Z' },
    { ip: '104.16.2.2', latencyMs: 28, successRate: 96, lastCheck: '2026-11-27T23:00:00Z' },
  ],

  // ---- Proxies / Rules ----
  GetUpstreams: [
    { id: 'up-1', name: 'Direct', type: 'direct', enabled: true },
    { id: 'up-2', name: 'Cloudflare WARP', type: 'warp', enabled: false },
  ],
  GetSiteGroups: [
    {
      id: 'sg-1', name: 'Streaming', mode: 'ech', enabled: true, domains: ['netflix.com', 'youtube.com'],
      upstreamID: '', notes: 'ECH injection group',
    },
    {
      id: 'sg-2', name: 'Blocked sites', mode: 'fragment', enabled: true, domains: ['example.org'],
      upstreamID: 'up-2', notes: '',
    },
  ],
  GetRulesLayerInfo: {
    builtin: { editable: false, groups: 24, domains: 3821 },
    user: { editable: true, groups: 2, domains: 3 },
    upstream: { editable: false, groups: 0, domains: 0 },
  },

  // ---- Routing / DNS / NAT64 ----
  GetAutoRoutingConfig: { enabled: true, gfwlist: true, mode: 'auto' },
  GetAutoRoutingStatus: { lastUpdate: '2026-11-27T10:00:00Z', ruleCount: 1024, enabled: true },
  GetDNSNodes: [
    { id: 'dns-1', name: 'Cloudflare DoH', url: 'https://1.1.1.1/dns-query', sni: 'cloudflare-dns.com', ips: ['1.1.1.1'], echEnabled: true, echProfileID: '', echAutoUpdate: true, quic: true, enabled: true, certVerify: { mode: 'default', names: [], suffixes: [], spkiSHA256: '', allowUnknownAuthority: false } },
    { id: 'dns-2', name: 'Google DoH', url: 'https://8.8.8.8/dns-query', sni: 'dns.google', ips: ['8.8.8.8'], echEnabled: false, echProfileID: '', echAutoUpdate: false, quic: false, enabled: true, certVerify: { mode: 'default', names: [], suffixes: [], spkiSHA256: '', allowUnknownAuthority: false } },
  ],
  GetNAT64Profiles: [{ id: 'nat64-1', name: 'Default', prefix: '64:ff9b::/96', enabled: true }],

  // ---- Logs ----
  GetRecentLogs: '[info] mock log line 1\n[action] StartProxy called\n[success] proxy listening on 127.0.0.1:8080',
  GetAppRecentLogs: '[app] mock app log',
  GetCoreRecentLogs: '[core] mock core log',
  GetLogFiles: [{ name: '2026-11-27_23-00-00.log', size: 20480 }],
  IsLogCaptureEnabled: false,

  // ---- Connections viewer ----
  GetActiveConnections: [
    { id: 1, host: 'www.youtube.com', destIP: '104.16.1.1:443', rule: 'Streaming', mode: 'ech', up: 1024, down: 20480, state: 'active', process: 'chrome.exe' },
    { id: 2, host: 'api.github.com', destIP: '140.82.1.1:443', rule: 'DIRECT', mode: 'direct', up: 512, down: 4096, state: 'active', process: 'code.exe' },
  ],

  // ---- Profiles ----
  ListProfiles: [
    { name: 'Default', updated_at: '2026-11-27T22:00:00+08:00', active: true },
    { name: 'Work', updated_at: '2026-11-20T09:30:00+08:00', active: false },
  ],
  GetActiveProfile: 'Default',

  // ---- Diagnostics / misc ----
  GetProxyDiagnostics: { ok: true, listenAddr: '127.0.0.1:8080', reachable: true },
  ProxySelfCheck: { ok: true },
  GetPortOccupant: null,

  // ---- About / update ----
  GetAppVersion: '1.31.0',
  GetReleaseChannel: 'alpha',
  GetUpdateChannel: 'stable',
  GetDownloadSource: 'github',
  GetPendingUpdate: null,
  CheckUpdate: { hasUpdate: false, currentVersion: '1.31.0', latestVersion: '1.31.0' },
};

export function wailsMockPlugin(): Plugin {
  return {
    name: 'wails-runtime-mock',
    configureServer(server) {
      server.middlewares.use('/wails/runtime', (req, res) => {
        let body = '';
        req.on('data', (chunk: Buffer) => (body += chunk.toString('utf8')));
        req.on('end', () => {
          let methodName = '';
          try {
            const parsed = body ? JSON.parse(body) : null;
            methodName = String(parsed?.args?.methodName ?? '');
          } catch { /* fall through with empty name */ }
          const short = methodName.split('.').pop() ?? '';
          const value = short in mocks ? mocks[short] : null;
          res.statusCode = 200;
          res.setHeader('Content-Type', 'application/json');
          res.end(JSON.stringify(value));
        });
      });
    },
  };
}
