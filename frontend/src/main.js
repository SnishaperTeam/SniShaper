import { StartProxy, StopProxy, IsProxyRunning, GetSiteGroups, AddSiteGroup, DeleteSiteGroup, UpdateSiteGroup, ExportConfig, ImportConfigWithSummary, GetCAInstallStatus, OpenCAFile, GetCACertPEM, GetSystemProxyStatus, EnableSystemProxy, DisableSystemProxy, RegenerateCert, ExportCert, GetListenPort, SetListenPort, SetProxyMode, GetProxyMode, GetRecentLogs, ClearLogs, ProxySelfCheck, GetProxyDiagnostics, GetCloudflareConfig, UpdateCloudflareConfig, TriggerCFHealthCheck, RemoveInvalidCFIPs, GetCloudflareIPStats, ForceFetchCloudflareIPs, GetServerConfig, UpdateServerConfig, InstallCA, GetECHProfiles, UpsertECHProfile, DeleteECHProfile, FetchECHConfig } from '../wailsjs/go/main/App.js';
import { WindowMinimise, WindowToggleMaximise, Quit } from '../wailsjs/runtime/runtime.js';

let isRunning = false;
let systemProxyEnabled = false;
let editingGroupId = null;
let loggingEnabled = true;
let backendLogPoll = null;
let rulesSearchQuery = '';
let rulesViewMode = 'mitm';
let echProfiles = [];

function getModeLabel(mode) {
    switch ((mode || '').toLowerCase()) {
    case 'server':
        return 'Server 节点';
    case 'mitm':
        return 'MITM';
    case 'transparent':
        return '透传';
    case 'tls-rf':
        return 'TLS 分片';
    default:
        return mode || '未知模式';
    }
}

window.windowMinimise = function () {
    WindowMinimise();
};

window.windowToggleMaximise = function () {
    WindowToggleMaximise();
};

window.windowCloseApp = function () {
    Quit();
};

function getWebsiteKey(group) {
    const website = (group.website || '').trim();
    if (website) return website;
    const name = (group.name || '').trim();
    if (name) return name;
    const firstDomain = (group.domains || [])[0] || '';
    return firstDomain.trim() || '未分组';
}

// 生成 Fake SNI 映射
window.generateFakeSNI = function() {
    const domainsInput = document.getElementById('input-domains').value;
    const snifakeInput = document.getElementById('input-snifake');
    
    if (!domainsInput.trim()) {
        addLog('warn', '请先填写域名列表');
        return;
    }
    
    // 获取第一个域名
    const firstDomain = domainsInput.split('\n').find(d => d.trim())?.trim();
    if (!firstDomain) {
        addLog('warn', '未找到有效域名');
        return;
    }
    
    // 生成映射：将 . 替换为 -，并添加 .mapped 后缀
    const mapped = firstDomain.replace(/\./g, '-');
    const fakeSNI = `${mapped}.mapped`;
    
    // 填充到输入框
    snifakeInput.value = fakeSNI;
    addLog('info', `已生成 Fake SNI: ${fakeSNI}`);
};


function updateStatus() {
    const statusEl = document.getElementById('proxy-status');
    const btnStart = document.getElementById('btn-start');
    const btnStop = document.getElementById('btn-stop');
    const btnSysProxy = document.getElementById('btn-sysproxy');
    const proxyMode = document.getElementById('proxy-mode');
    const mode = document.querySelector('input[name="mode"]:checked').value;

    proxyMode.textContent = getModeLabel(mode);

    if (isRunning) {
        statusEl.classList.add('running');
        statusEl.querySelector('.status-text').textContent = '运行中';
        btnStart.style.display = 'none';
        btnStop.style.display = 'inline-flex';
    } else {
        statusEl.classList.remove('running');
        statusEl.querySelector('.status-text').textContent = '已停止';
        btnStart.style.display = 'inline-flex';
        btnStop.style.display = 'none';
    }

    if (btnSysProxy) {
        btnSysProxy.textContent = `系统代理: ${systemProxyEnabled ? '开' : '关'}`;
        btnSysProxy.className = systemProxyEnabled ? 'btn btn-success' : 'btn btn-secondary';
    }
}

async function loadSystemProxyStatus() {
    try {
        const status = await GetSystemProxyStatus();
        systemProxyEnabled = status.enabled;
        updateStatus();
    } catch (err) {
        console.error('Load system proxy status error:', err);
    }
}

window.toggleSystemProxy = async function () {
    try {
        if (systemProxyEnabled) {
            await DisableSystemProxy();
            systemProxyEnabled = false;
            addLog('info', '系统代理已关闭');
        } else {
            // 如果代理未运行，先尝试启动代理
            if (!isRunning) {
                addLog('warn', '系统代理依赖本地代理服务，正在先启动代理...');
                const mode = document.querySelector('input[name="mode"]:checked').value;
                try {
                    await SetProxyMode(mode);
                    await StartProxy();
                    isRunning = true;
                    addLog('success', '代理服务器已启动');
                } catch (e) {
                    addLog('error', '前置代理启动失败: ' + e);
                    isRunning = false; // 确保状态复位
                    updateStatus();
                    return; // 终止后续开启系统代理的操作
                }
            }
            // 代理启动成功（或已运行），再设置系统代理
            const port = await GetListenPort();
            await EnableSystemProxy();
            systemProxyEnabled = true;
            addLog('info', `系统代理已开启 (127.0.0.1:${port})`);
        }
        updateStatus();
    } catch (err) {
        console.error('Toggle system proxy error:', err);
        addLog('error', '系统代理设置失败: ' + err);
        // 如果开启失败，可能需要回滚 systemProxyEnabled 状态?
        // updateStatus 会根据 systemProxyEnabled 渲染，还是保持现状比较安全
    }
};

window.startProxy = async function () {
    const mode = document.querySelector('input[name="mode"]:checked').value;

    if (mode === 'mitm') {
        try {
            const status = await GetCAInstallStatus();
            if (!status.Installed) {
                showCertModal();
                addLog('warn', '未检测到受信任 CA，仍尝试启动 MITM（浏览器可能证书告警）');
            }
        } catch (err) {
            console.error('Check cert status error:', err);
        }
    }

    try {
        await SetProxyMode(mode);
        await StartProxy();
        isRunning = true;
        addLog('info', '代理已启动');
    } catch (err) {
        console.error('Start proxy error:', err);
        addLog('error', '启动失败: ' + err);
        isRunning = false; // Reset state
    }
    updateStatus();
};

window.stopProxy = async function () {
    try {
        if (systemProxyEnabled) {
            await DisableSystemProxy();
            systemProxyEnabled = false;
            addLog('warn', '已自动关闭系统代理，避免断网');
        }
        await StopProxy();
        isRunning = false;
        addLog('info', '代理已停止');
    } catch (err) {
        console.error('Stop proxy error:', err);
        addLog('error', '停止失败: ' + err);
    }
    updateStatus();
};

function addLog(level, message) {
    if (!loggingEnabled) return;

    const container = document.getElementById('log-container');
    if (!container) return;

    const now = new Date();
    const timeStr = now.toTimeString().split(' ')[0];

    const entry = document.createElement('div');
    entry.className = 'log-entry';
    entry.innerHTML = `<span class="log-time">${timeStr}</span><span class="log-level ${level}">${level.toUpperCase()}</span><span>${message}</span>`;

    container.appendChild(entry);
    container.scrollTop = container.scrollHeight;

    if (container.children.length > 500) {
        container.removeChild(container.firstChild);
    }
}

window.showPage = function (pageId) {
    document.querySelectorAll('.page').forEach(p => p.style.display = 'none');
    document.getElementById('page-' + pageId).style.display = 'block';

    document.querySelectorAll('.nav-item').forEach(item => {
        item.classList.remove('active');
        if (item.dataset.page === pageId) {
            item.classList.add('active');
        }
    });

    if (pageId === 'settings') {
        loadCloudflareConfig();
    }
    if (pageId === 'cloudflare') {
        loadECHProfiles();
    }
    if (pageId === 'rules') {
        loadSiteGroups();
    }
    if (pageId === 'logs') {
        refreshBackendLogs();
        if (!backendLogPoll) {
            backendLogPoll = setInterval(refreshBackendLogs, 1200);
        }
    } else if (backendLogPoll) {
        clearInterval(backendLogPoll);
        backendLogPoll = null;
    }

    if (pageId === 'cloudflare') {
        loadECHProfiles();
    }
}

function guessLogLevel(line) {
    const s = line.toLowerCase();
    if (s.includes('error') || s.includes('failed') || s.includes('panic')) return 'error';
    if (s.includes('warn')) return 'warn';
    return 'info';
}

async function refreshBackendLogs() {
    const container = document.getElementById('log-container');
    if (!container) return;
    try {
        const text = await GetRecentLogs(400);
        const lines = (text || '').split('\n').filter(Boolean);
        container.innerHTML = '';
        if (lines.length === 0) {
            const entry = document.createElement('div');
            entry.className = 'log-entry';
            entry.innerHTML = `<span class="log-time">--:--:--</span><span class="log-level warn">WARN</span><span>后端日志为空：请求可能未进入代理，或日志接口未返回内容。</span>`;
            container.appendChild(entry);
            return;
        }
        lines.forEach(line => {
            const level = guessLogLevel(line);
            const entry = document.createElement('div');
            entry.className = 'log-entry';

            const time = document.createElement('span');
            time.className = 'log-time';
            time.textContent = '--:--:--';

            const levelEl = document.createElement('span');
            levelEl.className = `log-level ${level}`;
            levelEl.textContent = level.toUpperCase();

            const msg = document.createElement('span');
            msg.style.whiteSpace = 'pre-wrap';
            msg.textContent = line;

            entry.appendChild(time);
            entry.appendChild(levelEl);
            entry.appendChild(msg);
            container.appendChild(entry);
        });
        container.scrollTop = container.scrollHeight;

        const diag = await GetProxyDiagnostics();
        const ingressEl = document.getElementById('ingress-list');
        // Diagnostics cleanup: simplified
    } catch (err) {
        console.error('Refresh backend logs error:', err);
        container.innerHTML = '';
        const entry = document.createElement('div');
        entry.className = 'log-entry';
        entry.innerHTML = `<span class="log-time">--:--:--</span><span class="log-level error">ERROR</span><span>读取后端日志失败: ${String(err)}</span>`;
        container.appendChild(entry);
    }
}

async function loadSiteGroups() {
    try {
        const groups = await GetSiteGroups();
        const container = document.getElementById('rules-list');
        const query = rulesSearchQuery.trim().toLowerCase();

        if (!groups || groups.length === 0) {
            container.innerHTML = `
                <div class="empty-state">
                    <div class="empty-state-icon">-</div>
                    <div class="empty-state-text">暂无规则</div>
                    <div class="empty-state-hint">点击上方按钮添加</div>
                </div>
            `;
            return;
        }

        container.innerHTML = '';

        const buildModeColumn = (mode, title) => {
            const modeGroups = groups
                .filter(g => (g.mode || '').toLowerCase() === mode)
                .filter(g => {
                    if (!query) return true;
                    const haystack = [
                        g.name || '',
                        g.website || '',
                        g.upstream || '',
                        ...(g.domains || [])
                    ].join(' ').toLowerCase();
                    return haystack.includes(query);
                });

            const modeColumn = document.createElement('div');
            modeColumn.className = 'rules-column';

            const modeBlock = document.createElement('div');
            modeBlock.className = 'website-group rules-mode-block';

            const modeHeader = document.createElement('div');
            modeHeader.className = 'website-group-header';
            modeHeader.innerHTML = `
                <div class="website-group-title">${title}</div>
                <div class="website-group-count">${modeGroups.length} 条规则</div>
            `;
            modeBlock.appendChild(modeHeader);

            if (modeGroups.length === 0) {
                const empty = document.createElement('div');
                empty.className = 'rule-item';
                empty.innerHTML = `<div class="rule-info"><div class="rule-domains">暂无${title}规则</div></div>`;
                modeBlock.appendChild(empty);
                modeColumn.appendChild(modeBlock);
                return modeColumn;
            }

            const websiteMap = new Map();
            modeGroups.forEach(group => {
                const key = getWebsiteKey(group);
                if (!websiteMap.has(key)) {
                    websiteMap.set(key, []);
                }
                websiteMap.get(key).push(group);
            });

            Array.from(websiteMap.entries())
                .sort((a, b) => a[0].localeCompare(b[0], 'zh-Hans-CN'))
                .forEach(([website, websiteRules]) => {
                    const section = document.createElement('div');
                    section.className = 'website-group';

                    const header = document.createElement('div');
                    header.className = 'website-group-header';
                    const titleEl = document.createElement('div');
                    titleEl.className = 'website-group-title';
                    titleEl.textContent = website;

                    const tools = document.createElement('div');
                    tools.className = 'website-group-tools';

                    const countEl = document.createElement('div');
                    countEl.className = 'website-group-count';
                    countEl.textContent = `${websiteRules.length} 条规则`;

                    const addBtn = document.createElement('button');
                    addBtn.className = 'btn btn-secondary';
                    addBtn.textContent = '+ 本网站规则';
                    addBtn.onclick = () => window.showAddRuleModal({ website, mode });

                    tools.appendChild(countEl);
                    tools.appendChild(addBtn);
                    header.appendChild(titleEl);
                    header.appendChild(tools);
                    section.appendChild(header);

                    websiteRules.forEach(group => {
                        const item = document.createElement('div');
                        item.className = 'rule-item';
                        item.innerHTML = `
                            <div class="rule-info">
                                <div class="rule-name">${group.name || '未命名'}</div>
                                <div class="rule-domains">${(group.domains || []).join(', ')}</div>
                                <div class="rule-domains">${group.ech_enabled ? '<span style="color:var(--success)">ECH开启</span>' : ''} ${group.use_cf_pool ? '<span style="color:var(--primary)">优选IP</span>' : ''}</div>
                                <div class="rule-mode">${getModeLabel(group.mode)}${group.upstream ? ' → ' + (group.upstream.length > 40 ? group.upstream.substring(0, 40) + '...' : group.upstream) : ''}</div>
                            </div>
                            <div class="rule-actions">
                                <button class="btn btn-secondary" onclick="showEditRuleModal('${group.id}')">编辑</button>
                                <button class="btn btn-danger" onclick="deleteSiteGroup('${group.id}')">删除</button>
                            </div>
                        `;
                        section.appendChild(item);
                    });

                    modeBlock.appendChild(section);
                });

            modeColumn.appendChild(modeBlock);
            return modeColumn;
        };

        const title = rulesViewMode === 'server'
            ? 'Server 节点规则'
            : (rulesViewMode === 'transparent'
                ? '透传规则'
                : (rulesViewMode === 'tls-rf' ? 'TLS 分片规则' : 'MITM 规则'));
        container.appendChild(buildModeColumn(rulesViewMode, title));
    } catch (err) {
        console.error('Load site groups error:', err);
    }
}

window.deleteSiteGroup = async function (id) {
    try {
        await DeleteSiteGroup(id);
        addLog('info', '删除规则: ' + id);
        loadSiteGroups();
    } catch (err) {
        addLog('error', '删除失败: ' + err);
    }
};

window.showAddRuleModal = function () {
    let defaults = {};
    if (arguments.length > 0 && typeof arguments[0] === 'object' && arguments[0] !== null) {
        defaults = arguments[0];
    }
    editingGroupId = null;
    document.getElementById('modal-title').textContent = '添加规则';
    document.getElementById('input-name').value = '';
    document.getElementById('input-website').value = defaults.website || '';
    document.getElementById('input-domains').value = '';
    document.getElementById('input-mode').value = defaults.mode || 'server';
    document.getElementById('input-upstream').value = '';
    document.getElementById('input-snifake').value = '';
    document.getElementById('input-ech-domain').value = '';
    document.getElementById('input-utls-policy').value = '';
    document.getElementById('input-ech-enabled').checked = false;
    document.getElementById('input-ech-profile-id').value = '';
    document.getElementById('input-use-cf-pool').checked = false;
    document.getElementById('input-enabled').checked = true;
    updateECHProfileDropdown();
    document.getElementById('modal-overlay').style.display = 'flex';
};

window.showEditRuleModal = async function (id) {
    try {
        const groups = await GetSiteGroups();
        const group = groups.find(g => g.id === id);
        if (!group) {
            addLog('error', '找不到该规则');
            return;
        }

        editingGroupId = id;
        document.getElementById('modal-title').textContent = '编辑规则';
        document.getElementById('input-name').value = group.name || '';
        document.getElementById('input-website').value = group.website || '';
        document.getElementById('input-domains').value = (group.domains || []).join('\n');
        document.getElementById('input-mode').value = group.mode || 'mitm';
        document.getElementById('input-upstream').value = group.upstream || '';
        document.getElementById('input-snifake').value = group.sni_fake || '';
        document.getElementById('input-ech-domain').value = group.ech_domain || '';
        document.getElementById('input-utls-policy').value = group.utls_policy || '';
        document.getElementById('input-ech-enabled').checked = !!group.ech_enabled;
        
        await updateECHProfileDropdown();
        document.getElementById('input-ech-profile-id').value = group.ech_profile_id || '';
        
        document.getElementById('input-use-cf-pool').checked = !!group.use_cf_pool;
        document.getElementById('input-enabled').checked = group.enabled !== false;
        document.getElementById('modal-overlay').style.display = 'flex';
    } catch (err) {
        console.error('Edit rule error:', err);
        addLog('error', '加载规则失败: ' + err);
    }
};

window.closeModal = function () {
    document.getElementById('modal-overlay').style.display = 'none';
};

window.confirmModal = async function () {
    const name = document.getElementById('input-name').value;
    const website = document.getElementById('input-website').value.trim();
    const domains = document.getElementById('input-domains').value.split('\n').filter(d => d.trim());
    const mode = document.getElementById('input-mode').value;
    const upstream = document.getElementById('input-upstream').value;
    const snifake = document.getElementById('input-snifake').value;
    const echDomain = document.getElementById('input-ech-domain').value.trim();
    const utlsPolicy = document.getElementById('input-utls-policy').value;
    const echEnabled = document.getElementById('input-ech-enabled').checked;
    const echProfileId = document.getElementById('input-ech-profile-id').value;
    const useCfPool = document.getElementById('input-use-cf-pool').checked;
    const enabled = document.getElementById('input-enabled').checked;

    if (!name || domains.length === 0) {
        addLog('warn', '请填写名称和域名');
        return;
    }

    if (mode === 'transparent' && !upstream) {
        addLog('warn', '透传模式需要填写上游服务器地址');
        return;
    }

    try {
        const groupData = {
            name,
            website,
            domains,
            mode,
            upstream,
            sni_fake: snifake,
            ech_domain: echDomain,
            ech_profile_id: echProfileId,
            utls_policy: utlsPolicy,
            ech_enabled: echEnabled,
            use_cf_pool: useCfPool,
            enabled
        };

        if (editingGroupId) {
            groupData.id = editingGroupId;
            await UpdateSiteGroup(groupData);
            addLog('info', '更新规则: ' + name);
        } else {
            groupData.id = 'sg-' + Date.now();
            await AddSiteGroup(groupData);
            addLog('info', '添加规则: ' + name);
        }

        loadSiteGroups();
        closeModal();
    } catch (err) {
        addLog('error', '操作失败: ' + err);
    }
};

window.clearLogs = async function () {
    try {
        await ClearLogs();
        await refreshBackendLogs();
        addLog('info', '日志文件已清空');
    } catch (err) {
        addLog('error', '清空日志失败: ' + err);
    }
};

window.runProxySelfCheck = async function () {
    try {
        const result = await ProxySelfCheck();
        addLog('info', result || '自检完成');
        await refreshBackendLogs();
    } catch (err) {
        addLog('error', '代理自检失败: ' + err);
    }
};

window.exportConfig = async function () {
    try {
        const config = await ExportConfig();
        const blob = new Blob([config], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        const now = new Date();
        const stamp = `${now.getFullYear()}${String(now.getMonth() + 1).padStart(2, '0')}${String(now.getDate()).padStart(2, '0')}-${String(now.getHours()).padStart(2, '0')}${String(now.getMinutes()).padStart(2, '0')}${String(now.getSeconds()).padStart(2, '0')}`;
        a.download = `snishaper-rules-${stamp}.json`;
        a.click();
        URL.revokeObjectURL(url);
        addLog('info', '规则配置已导出');
    } catch (err) {
        addLog('error', '导出规则失败: ' + err);
    }
};

window.importConfig = function () {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = '.json';
    input.onchange = async (e) => {
        const file = e.target.files && e.target.files[0];
        if (!file) {
            addLog('warn', '未选择文件，已取消导入');
            return;
        }
        const reader = new FileReader();
        reader.onload = async (ev) => {
            try {
                const summary = await ImportConfigWithSummary(String(ev.target?.result || ''));
                addLog('info', `规则配置已导入: ${file.name} (新增 ${summary.added || 0}, 覆盖 ${summary.overwritten || 0}, 跳过 ${summary.skipped || 0})`);
                await loadSiteGroups();
                // Removed alert
            } catch (err) {
                addLog('error', '导入规则失败: ' + err);
            }
        };
        reader.onerror = () => {
            const msg = '读取文件失败';
            addLog('error', msg + ': ' + file.name);
        };
        reader.readAsText(file);
    };
    input.click();
};

window.regenerateCert = async function () {
    try {
        await RegenerateCert();
        addLog('info', '证书已重新生成，请重新安装到系统信任库');
    } catch (err) {
        addLog('error', '重新生成证书失败: ' + err);
    }
};

window.exportCert = async function () {
    try {
        const pem = await ExportCert();
        const blob = new Blob([pem], { type: 'application/x-pem-file' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = 'snishaper-ca.crt';
        a.click();
        URL.revokeObjectURL(url);
        addLog('info', '证书已导出');
    } catch (err) {
        addLog('error', '导出证书失败: ' + err);
    }
};

async function loadCloudflareRules() {
    try {
        const groups = await GetSiteGroups();
        const container = document.getElementById('cf-rules-container');
        if (!container) return;
        container.innerHTML = '';

        // Filter for enabled ECH rules
        const cfRules = (groups || []).filter(g => g.ech_enabled);

        if (cfRules.length === 0) {
            container.innerHTML = `
                <div style="text-align:center; padding: 40px; color: var(--text-secondary); background: var(--bg-dark); border-radius: 12px; border: 1px dashed var(--border);">
                    <div style="font-size: 24px; margin-bottom: 8px;"></div>
                    暂无 ECH 加速规则，在上方输入域名开始加速
                </div>`;
            return;
        }

        cfRules.forEach(group => {
            const card = document.createElement('div');
            card.className = 'card-item';

            let domains = (group.domains || []).join(', ');
            if (domains.length > 40) domains = domains.substring(0, 40) + '...';

            let ip = group.upstream || (group.use_cf_pool ? '全局优选池' : '自动');
            if (ip.length > 20) ip = ip.substring(0, 20) + '...';

            let echSource = group.ech_domain;
            const isDefaultECH = !echSource || echSource === 'crypto.cloudflare.com';

            card.innerHTML = `
                <div class="card-info">
                    <div class="card-title">${domains}</div>
                    <div class="card-meta">
                        <span class="card-badge">${ip}</span>
                        <span class="card-badge" style="${isDefaultECH ? 'opacity: 0.6;' : 'color: var(--accent);'}">
                            ECH: ${isDefaultECH ? '自动' : echSource}
                        </span>
                    </div>
                </div>
                <div style="display: flex; gap: 8px;">
                    <button class="btn btn-secondary btn-sm" onclick="showEditRuleModal('${group.id}')">编辑</button>
                    <button class="btn btn-danger btn-sm" onclick="deleteCfRule('${group.id}')">移除</button>
                </div>
            `;
            container.appendChild(card);
        });
    } catch (err) {
        console.error("Failed to load CF rules:", err);
        addLog('error', '加载 Cloudflare 规则失败: ' + err);
    }
}

// IP Pool Tagging Logic
let currentIpPool = [];
let currentIpStats = [];

async function loadCloudflareConfig() {
    try {
        const config = await GetCloudflareConfig();
        const dohEl = document.getElementById('setting-cf-doh');
        if (dohEl) dohEl.value = config.doh_url || '';

        const autoUpdateEl = document.getElementById('setting-cf-auto-update');
        if (autoUpdateEl) autoUpdateEl.checked = !!config.auto_update;

        const apiKeyEl = document.getElementById('setting-cf-api-key');
        if (apiKeyEl) apiKeyEl.value = config.api_key || '';

        // Load generic server config as well
        if (typeof GetServerConfig === 'function') {
            const serverConfig = await GetServerConfig();
            const serverHostEl = document.getElementById('setting-server-host');
            if (serverHostEl) serverHostEl.value = serverConfig.host || '';
            const serverAuthEl = document.getElementById('setting-server-auth');
            if (serverAuthEl) serverAuthEl.value = serverConfig.auth || '';
        }

        currentIpPool = config.preferred_ips || [];

        // Try to fetch real-time stats
        try {
            currentIpStats = await GetCloudflareIPStats() || [];
        } catch (e) {
            console.warn("Failed to get IP stats:", e);
            currentIpStats = [];
        }

        renderIpGrid();
    } catch (err) {
        console.error('Load CF config error:', err);
    }
}

let lastIpGridData = '';

function renderIpGrid() {
    const container = document.getElementById('ip-tag-container');
    if (!container) return;

    // 1. 差异检测：如果数据没变，跳过重绘以节省性能
    const rawData = {
        pool: [...currentIpPool].sort(),
        stats: currentIpStats.map(s => `${s.ip}:${s.latency}:${s.failures}`).sort()
    };
    const currentDataString = JSON.stringify(rawData);
    if (currentDataString === lastIpGridData) return;
    lastIpGridData = currentDataString;

    // 2. 执行渲染
    // Switch container class to grid
    container.className = 'ip-grid';
    container.innerHTML = '';

    // If we have stats, use them (they represent the active pool). 
    // If stats are empty (e.g. startup), fallback to config pool.
    // Merge: create a map of IP -> Stat
    const statsMap = {};
    if (currentIpStats && currentIpStats.length > 0) {
        currentIpStats.forEach(s => {
            statsMap[s.ip] = s;
        });
    }

    const displayIPs = new Set([...currentIpPool]);
    if (currentIpStats) {
        currentIpStats.forEach(s => displayIPs.add(s.ip));
    }

    const list = Array.from(displayIPs);

    if (list.length === 0) {
        // Remove grid class for empty state to center text
        container.className = 'tag-container';
        container.innerHTML = '<span style="color: var(--text-secondary); font-size: 13px; font-style: italic; padding: 20px;">池中暂无 IP，请在上方输入或点击“手动更新”</span>';
        return;
    }

    // 批量生成卡片，减少 Reflow
    const fragment = document.createDocumentFragment();

    // 计算统计信息
    let goodCount = 0;
    let fairCount = 0;
    let poorCount = 0;
    let checkingCount = 0;

    list.forEach((ip) => {
        const stat = statsMap[ip];
        if (!stat) {
            checkingCount++;
        } else if (stat.failures >= 3) {
            poorCount++;
        } else {
            const ms = Math.round(stat.latency / 1000000);
            if (ms > 0 && ms < 100) goodCount++;
            else if (ms > 0 && ms < 300) fairCount++;
            else if (ms > 0) poorCount++;
            else checkingCount++;
        }
    });

    // 添加统计汇总
    const summaryDiv = document.createElement('div');
    summaryDiv.className = 'ip-summary';
    summaryDiv.innerHTML = `
        <div class="ip-summary-item">
            <span class="ip-summary-count">${list.length}</span>
            <span class="ip-summary-label">总 IP</span>
        </div>
        <div class="ip-summary-item ip-summary-good">
            <span class="ip-summary-count">${goodCount}</span>
            <span class="ip-summary-label">优 (100ms)</span>
        </div>
        <div class="ip-summary-item ip-summary-fair">
            <span class="ip-summary-count">${fairCount}</span>
            <span class="ip-summary-label">中 (300ms)</span>
        </div>
        <div class="ip-summary-item ip-summary-poor">
            <span class="ip-summary-count">${poorCount}</span>
            <span class="ip-summary-label">差/失效</span>
        </div>
    `;
    container.appendChild(summaryDiv);

    list.forEach((ip) => {
        const stat = statsMap[ip];
        let latencyClass = 'checking';
        let latencyText = 'pending';

        if (stat) {
            if (stat.failures >= 3) {
                latencyClass = 'poor';
                latencyText = 'failed';
            } else {
                const ms = Math.round(stat.latency / 1000000); // ns to ms
                if (ms > 0) {
                    if (ms < 100) latencyClass = 'good';
                    else if (ms < 300) latencyClass = 'fair';
                    else latencyClass = 'poor';
                    latencyText = `${ms}ms`;
                }
            }
        }

        const card = document.createElement('div');
        card.className = 'ip-card';
        const isInPool = currentIpPool.includes(ip);
        card.innerHTML = `
            <div class="ip-address">${ip}</div>
            <div class="ip-meta">
                <span class="ip-latency ${latencyClass}">${latencyText}</span>
                ${isInPool ? '<span class="ip-pool-badge">池</span>' : ''}
            </div>
            <div class="ip-remove" onclick="removeIpTag('${ip}')" title="移除 IP">×</div>
        `;
        fragment.appendChild(card);
    });
    container.appendChild(fragment);
}

window.removeIpTag = async function (ip) {
    // Remove from config pool
    const idx = currentIpPool.indexOf(ip);
    if (idx !== -1) {
        currentIpPool.splice(idx, 1);
        await saveCloudflareConfig(); // Persist config removal
    }
    await loadCloudflareConfig();
    renderIpGrid();
};

// Helper for button loading state
async function withLoading(btnId, loadingText, action) {
    const btn = document.querySelector(`button[onclick="${btnId}()"]`) || document.getElementById(btnId);
    // Fallback: search by onclick attribute text if element not found by ID or Selector
    let targetBtn = btn;
    if (!targetBtn) {
        const buttons = document.querySelectorAll('button');
        for (let b of buttons) {
            if (b.getAttribute('onclick') && b.getAttribute('onclick').includes(btnId)) {
                targetBtn = b;
                break;
            }
        }
    }

    const originalText = targetBtn ? targetBtn.innerText : '';
    if (targetBtn) {
        targetBtn.classList.add('loading');
        targetBtn.innerText = loadingText;
    }

    try {
        await action();
    } finally {
        if (targetBtn) {
            targetBtn.classList.remove('loading');
            targetBtn.innerText = originalText;
        }
    }
}

async function saveCloudflareConfig() {
    const doh_url = document.getElementById('setting-cf-doh')?.value.trim();
    const auto_update = document.getElementById('setting-cf-auto-update')?.checked;
    const api_key = document.getElementById('setting-cf-api-key')?.value.trim();

    try {
        await UpdateCloudflareConfig({
            doh_url,
            preferred_ips: currentIpPool,
            auto_update: !!auto_update,
            api_key: api_key || ""
        });

        addLog('info', '配置已更新');
    } catch (err) {
        addLog('error', '保存配置失败: ' + err);
    }
}

function bindServerAuthToggle() {
    const authInput = document.getElementById('setting-server-auth');
    const toggleBtn = document.getElementById('toggle-server-auth');
    if (!authInput || !toggleBtn) return;

    const syncToggleLabel = () => {
        toggleBtn.textContent = authInput.type === 'password' ? '显示' : '隐藏';
    };

    toggleBtn.addEventListener('click', () => {
        authInput.type = authInput.type === 'password' ? 'text' : 'password';
        syncToggleLabel();
        authInput.focus();
    });

    syncToggleLabel();
}


window.loadECHDiscoveryDomains = async function () {
    try {
        const domains = await GetECHDiscoveryDomains();
        const container = document.getElementById('ech-discovery-domains');
        if (!container) return;

        if (!domains || domains.length === 0) {
            container.innerHTML = '暂无已开启 ECH Discovery 的域名';
        } else {
            container.innerHTML = `已开启 ECH Discovery 的域名: <strong>${domains.join(', ')}</strong>`;
        }
    } catch (err) {
        console.error('Failed to load ECH discovery domains:', err);
    }
};

// Removed duplicate saveServerConfig and addServerRule

async function saveIpPool() {
    await saveCloudflareConfig();
}

function initIpTagging() {
    const input = document.getElementById('setting-ip-input');
    const addBtn = document.getElementById('btn-add-ip');
    if (!input) return;

    const handleAdd = async () => {
        const val = input.value.trim();
        if (val && !currentIpPool.includes(val)) {
            // Simple IP validation
            if (/^(\d{1,3}\.){3}\d{1,3}$/.test(val) || val.includes(':')) {
                currentIpPool.push(val);
                input.value = '';
                await saveIpPool();
                await loadCloudflareConfig(); // Refresh stats too?
            } else {
                addLog('warn', '无效的 IP 格式');
            }
        }
    };

    input.addEventListener('keydown', async (e) => {
        if (e.key === 'Enter') {
            await handleAdd();
        }
    });

    if (addBtn) {
        addBtn.onclick = handleAdd;
    }
}

window.triggerCFHealthCheck = async function () {
    await withLoading('triggerCFHealthCheck', '测速中...', async () => {
        try {
            await TriggerCFHealthCheck();
            addLog('info', '已触发 Cloudflare IP 健康检查 (后台运行)');
            // Auto poll for updates
            for (let i = 0; i < 8; i++) {
                await new Promise(r => setTimeout(r, 1000));
                try {
                    // Silent update
                    const stats = await GetCloudflareIPStats() || [];
                    if (stats.length > 0) {
                        currentIpStats = stats;
                        renderIpGrid();
                    }
                } catch (e) { }
            }
            await loadCloudflareConfig();
        } catch (err) {
            addLog('error', '触发失败: ' + err);
        }
    });
};

window.removeInvalidCFIPs = async function () {
    await withLoading('removeInvalidCFIPs', '清理中...', async () => {
        try {
            const count = await RemoveInvalidCFIPs();
            addLog('info', `已清理 ${count} 个失效 IP`);
            await loadCloudflareConfig();
        } catch (err) {
            addLog('error', '清理失败: ' + err);
        }
    });
};

window.addCloudflareRule = async function () {
    const domainsText = document.getElementById('cf-input-domains').value.trim();
    if (!domainsText) {
        addLog('warn', "请输入目标域名列表");
        return;
    }

    const ipInput = document.getElementById('cf-input-ip').value.trim();
    const echDomainInput = document.getElementById('cf-input-ech-domain').value.trim();
    const echDomain = echDomainInput || 'crypto.cloudflare.com';

    // Split lines and filter empty
    const domains = domainsText.split('\n').map(d => d.trim()).filter(d => d);
    if (domains.length === 0) return;

    // Logic: Create one siteGroup containing all these domains.
    // Name it based on the first domain.
    const groupName = domains[0] + (domains.length > 1 ? ` 等${domains.length}个` : '');

    const echProfileId = document.getElementById('cf-input-ech-profile-id')?.value || '';
    
    const newGroup = {
        name: groupName,
        website: domains[0].split('.')[0],
        domains: domains,
        mode: "mitm",
        upstream: ipInput ? (ipInput.includes(':') ? ipInput : ipInput + ":443") : "",
        ech_enabled: true,
        ech_domain: echDomain,
        ech_profile_id: echProfileId,
        use_cf_pool: !ipInput, // If no specific IP entered, use pool
        sni_policy: "fake",    // Force fake SNI policy (ECH handles outer)
        utls_policy: "auto",
        enabled: true
    };

    try {
        await AddSiteGroup(newGroup);
        document.getElementById('cf-input-domains').value = '';
        document.getElementById('cf-input-ip').value = '';
        // Don't clear ech-domain, keep default or user choice

        loadCloudflareRules();
        addLog('info', `已添加 Cloudflare 规则: ${groupName}`);
    } catch (err) {
        addLog('error', "添加 Cloudflare 规则失败: " + err);
    }
};

window.deleteCfRule = async function (id) {
    if (!window.confirm("确定要删除此加速规则吗？")) return;
    try {
        await DeleteSiteGroup(id);
        loadCloudflareRules();
        addLog('info', '删除 Cloudflare 规则: ' + id);
    } catch (err) {
        addLog('error', "删除失败: " + err);
    }
};

window.loadCloudflareRules = loadCloudflareRules;

window.showCertModal = async function () {
    const modal = document.getElementById('cert-modal');
    const statusEl = document.getElementById('cert-install-status');
    const pathEl = document.getElementById('cert-path');
    const helpEl = document.getElementById('cert-help-text');

    try {
        const status = await GetCAInstallStatus();
        statusEl.textContent = status.Installed ? '已安装' : '未安装';
        statusEl.style.color = status.Installed ? 'var(--success)' : 'var(--danger)';
        pathEl.textContent = status.CertPath || 'N/A';
        helpEl.textContent = status.InstallHelp || '';
    } catch (err) {
        console.error('Get cert status error:', err);
        statusEl.textContent = '获取失败';
        pathEl.textContent = err.message;
    }

    modal.style.display = 'flex';
};

window.closeCertModal = function () {
    document.getElementById('cert-modal').style.display = 'none';
};

window.openCertFile = async function () {
    try {
        await OpenCAFile();
        addLog('info', '已打开证书文件');
    } catch (err) {
        console.error('Open cert file error:', err);
        addLog('error', '打开证书文件失败: ' + err);
    }
};

window.installCert = async function () {
    try {
        await InstallCA();
        addLog('info', '证书已自动安装到系统');
        const status = await GetCAInstallStatus();
        if (status.Installed) {
            document.getElementById('cert-install-status').textContent = '已安装';
            document.getElementById('cert-install-status').style.color = 'var(--success)';
        }
    } catch (err) {
        console.error('Install cert error:', err);
        addLog('error', '自动安装证书失败: ' + err);
    }
};

function updateThemeIcon(theme) {
    const toggleBtn = document.getElementById('theme-toggle');
    if (toggleBtn) {
        toggleBtn.setAttribute('aria-label', theme === 'dark' ? '切换到亮色' : '切换到暗色');
    }
}

async function checkCertAndPrompt() {
    const mode = document.querySelector('input[name="mode"]:checked').value;
    if (mode !== 'mitm') return;

    try {
        const status = await GetCAInstallStatus();
        if (!status.Installed) {
            showCertModal();
        }
    } catch (err) {
        console.error('Check cert status error:', err);
    }
}

document.addEventListener('DOMContentLoaded', async () => {
    const theme = localStorage.getItem('theme') || 'light';
    document.documentElement.setAttribute('data-theme', theme);
    updateThemeIcon(theme);

    document.getElementById('theme-toggle').addEventListener('click', () => {
        const currentTheme = document.documentElement.getAttribute('data-theme');
        const nextTheme = currentTheme === 'dark' ? 'light' : 'dark';
        document.documentElement.setAttribute('data-theme', nextTheme);
        localStorage.setItem('theme', nextTheme);
        updateThemeIcon(nextTheme);
    });

    document.querySelectorAll('.nav-item').forEach(item => {
        item.addEventListener('click', (e) => {
            if (item.id === 'theme-toggle') return;
            e.preventDefault();
            const page = item.dataset.page;
            showPage(page);
            if (page === 'cloudflare') loadCloudflareRules();
            if (page === 'settings') loadCloudflareConfig();
            if (page === 'server') loadCloudflareConfig();
        });
    });

    initIpTagging();
    loadCloudflareConfig();

    const rulesSearch = document.getElementById('rules-search');
    if (rulesSearch) {
        rulesSearch.addEventListener('input', () => {
            rulesSearchQuery = rulesSearch.value || '';
            const rulesPage = document.getElementById('page-rules');
            if (rulesPage && rulesPage.style.display !== 'none') {
                loadSiteGroups();
            }
        });
    }

    const modeServerBtn = document.getElementById('rules-mode-server');
    const modeMitmBtn = document.getElementById('rules-mode-mitm');
    const modeTransBtn = document.getElementById('rules-mode-transparent');
    const modeTLSRFBtn = document.getElementById('rules-mode-tls-rf');
    const updateRulesModeButtons = () => {
        if (modeServerBtn) modeServerBtn.classList.toggle('active', rulesViewMode === 'server');
        if (modeMitmBtn) modeMitmBtn.classList.toggle('active', rulesViewMode === 'mitm');
        if (modeTransBtn) modeTransBtn.classList.toggle('active', rulesViewMode === 'transparent');
        if (modeTLSRFBtn) modeTLSRFBtn.classList.toggle('active', rulesViewMode === 'tls-rf');
    };
    if (modeServerBtn) {
        modeServerBtn.addEventListener('click', () => {
            rulesViewMode = 'server';
            updateRulesModeButtons();
            loadSiteGroups();
        });
    }
    if (modeMitmBtn) {
        modeMitmBtn.addEventListener('click', () => {
            rulesViewMode = 'mitm';
            updateRulesModeButtons();
            loadSiteGroups();
        });
    }
    if (modeTransBtn) {
        modeTransBtn.addEventListener('click', () => {
            rulesViewMode = 'transparent';
            updateRulesModeButtons();
            loadSiteGroups();
        });
    }
    if (modeTLSRFBtn) {
        modeTLSRFBtn.addEventListener('click', () => {
            rulesViewMode = 'tls-rf';
            updateRulesModeButtons();
            loadSiteGroups();
        });
    }
    updateRulesModeButtons();

    document.querySelectorAll('input[name="mode"]').forEach(radio => {
        radio.addEventListener('change', async () => {
            updateStatus();
            try {
                await SetProxyMode(radio.value);
                addLog('info', '运行模式切换为: ' + (radio.value === 'mitm' ? 'MITM' : '透传'));
            } catch (err) {
                addLog('error', '模式切换失败: ' + err);
            }
            await checkCertAndPrompt();
        });
    });

    document.getElementById('modal-overlay').addEventListener('click', (e) => {
        if (e.target === document.getElementById('modal-overlay')) {
            closeModal();
        }
    });

    const portInput = document.getElementById('setting-port');
    if (portInput) {
        try {
            const port = await GetListenPort();
            portInput.value = port || 8080;
        } catch (err) {
            console.error('Get listen port error:', err);
        }
    }

    window.applyPort = async function() {
        const portInput = document.getElementById('setting-port');
        if (!portInput) return;
        
        const newPort = parseInt(portInput.value, 10);
        if (newPort >= 1 && newPort <= 65535) {
            try {
                await SetListenPort(newPort);
                document.getElementById('listen-port').textContent = newPort;
                addLog('info', '监听端口已设置为 ' + newPort);
            } catch (err) {
                addLog('error', '设置端口失败: ' + err);
                portInput.value = await GetListenPort();
            }
        } else {
            addLog('error', '端口号无效 (1-65535)');
        }
    };

    addLog('info', 'SniShaper 已就绪');

    try {
        isRunning = await IsProxyRunning();
        const backendMode = await GetProxyMode();
        if (backendMode === 'mitm' || backendMode === 'transparent' || backendMode === 'tls-rf') {
            const radio = document.querySelector(`input[name="mode"][value="${backendMode}"]`);
            if (radio) radio.checked = true;
        } else {
            const mode = document.querySelector('input[name="mode"]:checked').value;
            await SetProxyMode(mode);
        }
    } catch (err) {
        console.error('Init proxy mode error:', err);
    }

    updateStatus();

    await loadSystemProxyStatus();
    await checkCertAndPrompt();

    await loadCloudflareConfig();
    await loadCloudflareRules();
    await updateECHProfileDropdown();
    bindServerAuthToggle();
    
    document.getElementById('setting-cf-doh')?.addEventListener('change', saveCloudflareConfig);
});

window.saveServerConfig = async function () {
    const btn = document.getElementById('btn-save-server');
    if (!btn) return;
    const originalText = btn.innerHTML;

    try {
        btn.disabled = true;
        btn.innerHTML = '⌛ 保存中...';

        const serverHost = document.getElementById('setting-server-host')?.value.trim() || "";
        const serverAuth = document.getElementById('setting-server-auth')?.value.trim() || "";

        if (typeof UpdateServerConfig !== 'function') {
            throw new Error('后端绑定函数未找到');
        }

        await UpdateServerConfig(serverHost, serverAuth);

        btn.innerHTML = '已保存';
        addLog('success', 'Server 节点配置已刷新并存入 rules/config.json');

        setTimeout(() => {
            btn.disabled = false;
            btn.innerHTML = originalText;
        }, 1500);
    } catch (err) {
        btn.disabled = false;
        btn.innerHTML = originalText;
        addLog('error', 'Server 配置保存失败: ' + err);
        console.error(err);
    }
};

window.addServerRule = async function () {
    const input = document.getElementById('server-input-domains');
    if (!input) return;
    const domainsRaw = input.value.trim();
    if (!domainsRaw) {
        addLog('warn', '请先输入要加速的域名列表');
        return;
    }

    const domains = domainsRaw.split('\n').map(d => d.trim()).filter(d => d);
    if (domains.length === 0) return;

    try {
        const count = domains.length;
        addLog('info', `准备为 ${count} 个域名创建 Server 中转规则...`);

        for (const domain of domains) {
            let cleanDomain = domain.replace(/^https?:\/\//, '').split('/')[0].trim();
            if (!cleanDomain) continue;

            const echProfileId = document.getElementById('server-input-ech-profile-id')?.value || '';

            const groupData = {
                id: 'sg-srv-' + Date.now() + Math.floor(Math.random() * 1000),
                name: 'Server: ' + cleanDomain,
                website: 'server-batch',
                domains: [cleanDomain],
                mode: 'server',
                upstream: '',
                sni_fake: '',
                ech_domain: '',
                ech_profile_id: echProfileId,
                utls_policy: 'auto',
                ech_enabled: true,
                use_cf_pool: true,
                enabled: true
            };

            if (typeof AddSiteGroup === 'function') {
                await AddSiteGroup(groupData);
            }
        }

        addLog('success', `成功批量部署 ${count} 条加速规则！`);
        input.value = '';
        if (document.getElementById('page-rules').style.display !== 'none') {
            loadSiteGroups();
        }
    } catch (err) {
        addLog('error', '批量加速动作失败: ' + err);
    }
};

// --- ECH Profile Management ---
window.loadECHProfiles = async function() {
    try {
        const profiles = await GetECHProfiles();
        echProfiles = profiles || [];
        renderECHProfiles();
        await updateECHProfileDropdown();
    } catch (err) {
        console.error('Load ECH profiles error:', err);
        addLog('error', '加载 ECH 配置文件失败: ' + err);
    }
};

function renderECHProfiles() {
    const container = document.getElementById('ech-profiles-container');
    if (!container) return;

    if (echProfiles.length === 0) {
        container.innerHTML = '<div class="empty-state">暂无配置文件，点击上方按钮添加</div>';
        return;
    }

    container.innerHTML = '';
    echProfiles.forEach(p => {
        const badges = [];
        if (p.discovery_domain) {
            badges.push(`<span class="card-badge">发现域名: ${p.discovery_domain}</span>`);
        }
        if (p.doh_upstream) {
            badges.push(`<span class="card-badge">DoH: ${p.doh_upstream}</span>`);
        }
        if (p.auto_update) {
            badges.push('<span class="card-badge">自动更新</span>');
        }
        if (p.config) {
            badges.push(`<span class="card-badge" style="max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">Config: ${p.config.substring(0, 20)}...</span>`);
        } else {
            badges.push('<span class="card-badge">仅动态获取</span>');
        }
        const item = document.createElement('div');
        item.className = 'card-item';
        item.innerHTML = `
            <div class="card-info">
                <div class="card-title">${p.name}</div>
                <div class="card-meta">
                    ${badges.join('')}
                </div>
            </div>
            <div class="card-actions">
                <button class="btn btn-secondary btn-sm" onclick="showECHProfileModal('${p.id}')">编辑</button>
                <button class="btn btn-danger btn-sm" onclick="deleteECHProfile('${p.id}')">删除</button>
            </div>
        `;
        container.appendChild(item);
    });
}

window.showECHProfileModal = function(id) {
    const title = document.getElementById('ech-modal-title');
    const inputId = document.getElementById('ech-input-id');
    const inputName = document.getElementById('ech-input-name');
    const inputConfig = document.getElementById('ech-input-config');
    const inputDiscoveryDomain = document.getElementById('ech-input-discovery-domain');
    const inputDoHUpstream = document.getElementById('ech-input-doh-upstream');
    const inputAutoUpdate = document.getElementById('ech-input-auto-update');

    if (id) {
        const p = echProfiles.find(x => x.id === id);
        if (p) {
            title.textContent = '编辑 ECH 配置文件';
            inputId.value = p.id;
            inputName.value = p.name;
            inputConfig.value = p.config;
            inputDiscoveryDomain.value = p.discovery_domain || '';
            inputDoHUpstream.value = p.doh_upstream || '';
            inputAutoUpdate.checked = !!p.auto_update;
        }
    } else {
        title.textContent = '添加 ECH 配置文件';
        inputId.value = '';
        inputName.value = '';
        inputConfig.value = '';
        inputDiscoveryDomain.value = '';
        inputDoHUpstream.value = '';
        inputAutoUpdate.checked = false;
    }

    document.getElementById('ech-profile-modal').style.display = 'flex';
};

window.closeECHProfileModal = function() {
    document.getElementById('ech-profile-modal').style.display = 'none';
};

window.saveECHProfile = async function() {
    const id = document.getElementById('ech-input-id').value;
    const name = document.getElementById('ech-input-name').value.trim();
    const config = document.getElementById('ech-input-config').value.trim();
    const discoveryDomain = document.getElementById('ech-input-discovery-domain').value.trim();
    const dohUpstream = document.getElementById('ech-input-doh-upstream').value.trim();
    const autoUpdate = document.getElementById('ech-input-auto-update').checked;

    if (!name) {
        addLog('warn', '请填写配置名称');
        return;
    }

    if (!config && !autoUpdate) {
        addLog('warn', '请填写配置串，或启用自动更新');
        return;
    }

    if (autoUpdate && !discoveryDomain) {
        addLog('warn', '启用自动更新时请填写自动发现域名');
        return;
    }

    try {
        await UpsertECHProfile({
            id: id,
            name: name,
            config: config,
            discovery_domain: discoveryDomain,
            doh_upstream: dohUpstream,
            auto_update: autoUpdate
        });
        addLog('success', (id ? '更新' : '添加') + ' ECH 配置文件: ' + name);
        closeECHProfileModal();
        loadECHProfiles();
    } catch (err) {
        addLog('error', '保存 ECH 配置文件失败: ' + err);
    }
};

window.fetchECHConfigFromModal = async function() {
    const domain = document.getElementById('ech-input-discovery-domain').value.trim();
    const doh = document.getElementById('ech-input-doh-upstream').value.trim();
    const configArea = document.getElementById('ech-input-config');
    
    if (!domain) {
        addLog('warn', '请填写要解析的域名');
        return;
    }
    
    try {
        addLog('info', '正在解析 ECH 配置: ' + domain);
        const config = await FetchECHConfig(domain, doh);
        if (config) {
            configArea.value = config;
            addLog('success', 'ECH 配置解析成功: ' + domain);
        } else {
            addLog('warn', '未能解析到 ECH 配置: ' + domain);
        }
    } catch (err) {
        addLog('error', '解析 ECH 失败: ' + err);
    }
};

window.deleteECHProfile = async function(id) {
    if (!confirm('确定要删除此 ECH 配置文件吗？引用此配置的规则将失效。')) return;
    try {
        await DeleteECHProfile(id);
        addLog('info', '删除 ECH 配置文件: ' + id);
        loadECHProfiles();
    } catch (err) {
        addLog('error', '删除 ECH 配置文件失败: ' + err);
    }
};

async function updateECHProfileDropdown() {
    const selectors = ['input-ech-profile-id', 'cf-input-ech-profile-id', 'server-input-ech-profile-id'];
    
    try {
        const profiles = await GetECHProfiles();
        echProfiles = profiles || [];
    } catch (e) {
        console.error('Fetch profiles error:', e);
    }

    selectors.forEach(id => {
        const select = document.getElementById(id);
        if (!select) return;

        // Preserve selection
        const currentVal = select.value;
        
        select.innerHTML = '<option value="">(无 - 自动从 DoH fallback)</option>';
        echProfiles.forEach(p => {
            const opt = document.createElement('option');
            opt.value = p.id;
            opt.textContent = p.name;
            select.appendChild(opt);
        });

        select.value = currentVal;
    });
}
