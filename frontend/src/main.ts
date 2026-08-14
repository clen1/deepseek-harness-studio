import './style.css';

import {
    BenchmarkRegistries,
    CancelTask,
    Deploy,
    GetStatus,
    OpenInstallDirectory,
    OpenService,
    SaveConfig,
    StartService,
    StopService,
    TestCurrentRegistry,
    Uninstall,
} from '../wailsjs/go/main/App';
import {EventsOn, Quit, WindowMinimise, WindowToggleMaximise} from '../wailsjs/runtime/runtime';
import type {main} from '../wailsjs/go/models';

type Status = main.Status;
type Config = main.Config;
type LogEntry = main.LogEntry;
type RegistryResult = main.RegistryResult;
type PageName = 'overview' | 'network' | 'logs';

const iconPaths: Record<string, string> = {
    dashboard: '<path d="M4 13h6V4H4v9Zm0 7h6v-4H4v4Zm10 0h6v-9h-6v9Zm0-16v4h6V4h-6Z"/>',
    network: '<circle cx="12" cy="12" r="3"/><path d="M5.6 8.8a7 7 0 0 1 12.8 0M2.5 5.8a11 11 0 0 1 19 0M8.7 16.8 12 20l3.3-3.2"/>',
    logs: '<path d="M5 4h14v16H5z"/><path d="m8 9 2 2-2 2m4 1h4"/>',
    play: '<path d="m8 5 11 7-11 7V5Z"/>',
    stop: '<rect x="6" y="6" width="12" height="12" rx="2"/>',
    rocket: '<path d="M14 5c3-3 6-2 6-2s1 3-2 6l-5 5-4-4 5-5Z"/><path d="m9 10-4 1-2 3 6 1m4-1 1 6 3-2 1-4M7 17l-2 2"/>',
    folder: '<path d="M3 6h7l2 2h9v11H3V6Z"/>',
    refresh: '<path d="M20 7v5h-5M4 17v-5h5"/><path d="M6.1 8A7 7 0 0 1 18 6l2 6M18 16a7 7 0 0 1-12 2l-2-6"/>',
    bolt: '<path d="m13 2-8 12h7l-1 8 8-12h-7l1-8Z"/>',
    shield: '<path d="M12 3 4 6v5c0 5 3.4 8.3 8 10 4.6-1.7 8-5 8-10V6l-8-3Z"/><path d="m9 12 2 2 4-4"/>',
    check: '<path d="m5 12 4 4L19 6"/>',
    copy: '<rect x="8" y="8" width="11" height="11" rx="2"/><path d="M16 8V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h3"/>',
    search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/>',
    close: '<path d="m6 6 12 12M18 6 6 18"/>',
    chevron: '<path d="m9 18 6-6-6-6"/>',
    globe: '<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18"/>',
    cpu: '<rect x="6" y="6" width="12" height="12" rx="2"/><path d="M9 1v5m6-5v5M9 18v5m6-5v5M1 9h5m-5 6h5m12-6h5m-5 6h5"/>',
    package: '<path d="m12 3 8 4-8 4-8-4 8-4Z"/><path d="m4 7 8 4 8-4v10l-8 4-8-4V7Zm8 4v10"/>',
    activity: '<path d="M3 12h4l2-7 4 14 2-7h6"/>',
    wand: '<path d="m15 4 5 5L9 20l-5-5L15 4Z"/><path d="m6 3 .5 2L8 6l-1.5.5L6 8l-.5-1.5L4 6l1.5-1L6 3Zm13 11 .5 1.5L21 16l-1.5.5L19 18l-.5-1.5L17 16l1.5-.5L19 14Z"/>',
    trash: '<path d="M4 7h16M9 7V4h6v3m3 0-1 14H7L6 7m4 4v6m4-6v6"/>',
};

const icon = (name: string, size = 18): string =>
    `<svg class="icon" width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${iconPaths[name] ?? ''}</svg>`;

const app = document.querySelector<HTMLDivElement>('#app')!;

app.innerHTML = `
<div class="window-shell">
    <header class="titlebar">
        <div class="titlebar-drag">
            <div class="app-mark">H</div>
            <span>Harness Studio</span>
            <span class="title-version">2.1</span>
        </div>
        <div class="window-actions">
            <button class="window-button" data-action="minimise" aria-label="最小化"><span></span></button>
            <button class="window-button" data-action="maximise" aria-label="最大化"><span class="square"></span></button>
            <button class="window-button close" data-action="quit" aria-label="关闭">${icon('close', 14)}</button>
        </div>
    </header>

    <div class="app-layout">
        <aside class="sidebar">
            <div class="brand-block">
                <div class="brand-title">Harness 安装助手</div>
                <div class="brand-subtitle"><span class="live-dot"></span><span id="sidebar-state">正在检查电脑</span></div>
            </div>

            <div class="system-card">
                <div class="system-card-label">这台电脑</div>
                <div class="system-card-main">
                    <strong id="platform-name">--</strong>
                    <span id="arch-name">--</span>
                </div>
                <div class="system-stats">
                    <div><strong id="install-stat">--</strong><span>安装</span></div>
                    <div><strong id="node-stat">--</strong><span>环境</span></div>
                    <div><strong id="service-stat">--</strong><span>运行</span></div>
                </div>
                <button class="ghost-light-button" data-action="refresh">${icon('refresh', 14)}重新检查</button>
            </div>

            <nav class="navigation" aria-label="主导航">
                <button class="nav-item active" data-page="overview">${icon('dashboard')}<span>一键安装</span><span class="nav-badge" id="overview-badge">1</span></button>
                <button class="nav-item" data-page="network">${icon('network')}<span>网络设置</span><span class="nav-status" id="network-dot"></span></button>
                <button class="nav-item" data-page="logs">${icon('logs')}<span>安装日志</span><span class="nav-badge muted" id="log-count">0</span></button>
            </nav>

            <div class="sidebar-tip">
                <div class="tip-icon">${icon('shield', 17)}</div>
                <div><strong>不知道怎么选？</strong><p>保持默认设置，直接点击安装即可。</p></div>
            </div>

            <div class="sidebar-footer">
                <span>DeepSeek Harness</span>
                <button data-action="open-folder">打开安装目录</button>
            </div>
        </aside>

        <main class="main-content">
            <header class="content-header">
                <div>
                    <p class="eyebrow" id="page-eyebrow">EASY INSTALL</p>
                    <h1 id="page-title">一键安装</h1>
                    <p id="page-subtitle">点击一次，自动完成下载、安装和打开</p>
                </div>
                <div class="header-actions">
                    <div class="status-pill" id="header-status"><span></span><b>检查中</b></div>
                    <button class="header-folder-button" data-action="open-folder" title="打开程序安装目录" aria-label="打开程序安装目录">${icon('folder', 16)}<span>安装目录</span></button>
                    <button class="icon-button" data-action="refresh" title="刷新状态">${icon('refresh')}</button>
                </div>
            </header>

            <div class="content-scroll">
                <section class="page active" data-page-view="overview">
                    <article class="deploy-hero">
                        <div class="hero-glow"></div>
                        <div class="hero-content">
                            <div class="hero-copy">
                                <div class="hero-label"><span>${icon('wand', 14)}</span>新手模式已开启</div>
                                <h2 id="hero-title">点击按钮，剩下的自动完成</h2>
                                <p id="hero-description">无需安装其他软件，无需输入命令。完成后会自动打开 DeepSeek Harness。</p>
                                <div class="hero-actions">
                                    <button class="primary-button light beginner-primary" data-action="deploy" id="install-button">${icon('rocket')}<span id="deploy-button-text">立即安装并打开</span></button>
                                    <button class="secondary-button dark hidden" data-action="service" id="service-button">${icon('play')}<span id="service-button-text">启动并打开</span></button>
                                    <button class="secondary-button dark hidden" data-action="cancel" id="cancel-button">${icon('close')}取消</button>
                                    <button class="quiet-button hidden" data-action="stop-service" id="stop-button">停止运行</button>
                                </div>
                                <button class="hero-help" data-page="network">安装失败或下载太慢？打开网络设置</button>
                            </div>
                            <div class="hero-orbit" aria-hidden="true">
                                <div class="orbit-ring ring-one"></div><div class="orbit-ring ring-two"></div>
                                <div class="core-logo">DS</div>
                                <span class="orbit-node node-a">${icon('cpu', 15)}</span>
                                <span class="orbit-node node-b">${icon('network', 15)}</span>
                                <span class="orbit-node node-c">${icon('package', 15)}</span>
                            </div>
                        </div>
                        <div class="progress-wrap" id="progress-wrap">
                            <div class="progress-meta"><span id="progress-title">等待安装</span><span id="progress-value">0%</span></div>
                            <div class="progress-track"><div class="progress-bar" id="progress-bar"></div></div>
                            <p id="progress-message">第一次安装通常需要几分钟，请保持网络连接。</p>
                        </div>
                    </article>

                    <div class="easy-summary">
                        <div class="easy-summary-title"><span>${icon('check', 15)}</span><div><strong>推荐设置已经准备好</strong><p>大多数用户无需修改，直接安装即可</p></div></div>
                        <div class="easy-summary-items">
                            <div><span>下载线路</span><strong id="registry-value">NPMirror</strong></div>
                            <div><span>网络方式</span><strong id="proxy-value">跟随系统</strong></div>
                            <div><span>安装位置</span><strong>自动管理</strong></div>
                        </div>
                        <button class="text-arrow" data-page="network">修改设置 ${icon('chevron', 14)}</button>
                        <span class="hidden" id="version-value">尚未安装</span><span class="hidden" id="version-hint"></span><span class="hidden" id="version-state"></span><span class="hidden" id="service-value"></span><span class="hidden" id="service-pulse"></span>
                    </div>

                    <article class="panel simple-flow">
                        <div class="panel-heading"><div><span class="section-kicker">3 SIMPLE STEPS</span><h3>安装过程</h3><p>全程自动执行，你只需要等待完成。</p></div><button class="link-button" data-page="logs">查看详细日志</button></div>
                        <div class="simple-steps" id="deploy-steps">
                            <div class="simple-step" data-step="network"><span class="step-index">1</span><div><strong>检查并下载</strong><p>自动选择适合这台电脑的文件</p></div><span class="step-state">等待</span></div>
                            <span class="step-line"></span>
                            <div class="simple-step" data-step="install"><span class="step-index">2</span><div><strong>自动安装</strong><p>准备运行环境和 Harness</p></div><span class="step-state">等待</span></div>
                            <span class="step-line"></span>
                            <div class="simple-step" data-step="open"><span class="step-index">3</span><div><strong>打开使用</strong><p>安装完成后自动打开页面</p></div><span class="step-state">等待</span></div>
                        </div>
                    </article>
                    <article class="install-location-card">
                        <span class="location-icon">${icon('folder', 19)}</span>
                        <div><strong>程序安装位置</strong><p id="install-path-value">正在读取安装目录…</p></div>
                        <div class="location-actions">
                            <button class="secondary-button" data-action="open-folder">${icon('folder', 15)}打开安装目录</button>
                            <button class="danger-button hidden" data-action="uninstall" id="uninstall-button">${icon('trash', 15)}一键卸载</button>
                        </div>
                    </article>
                    <div class="hidden" id="activity-list"></div>
                </section>

                <section class="page" data-page-view="network">
                    <div class="network-hero easy-network-head">
                        <div><span class="section-kicker">OPTIONAL SETTINGS</span><h2>通常无需修改</h2><p>只有安装失败或下载很慢时，才需要进入这里。</p></div>
                    </div>

                    <article class="easy-network-card">
                        <div class="easy-network-icon">${icon('wand', 22)}</div>
                        <div><strong>让软件自动选择</strong><p>同时测试多条下载线路，并自动使用响应最快的一条。</p></div>
                        <button class="primary-button" data-action="benchmark">${icon('bolt')}自动测速并选择</button>
                    </article>

                    <details class="advanced-settings" id="advanced-settings">
                        <summary><span>${icon('network', 17)}</span><div><strong>手动网络设置</strong><small>下载失败、公司网络或使用代理时展开</small></div>${icon('chevron', 16)}</summary>
                        <div class="advanced-settings-body">
                            <div class="advanced-heading"><strong>选择下载线路</strong><p>优先选择标有“推荐”或测速较快的线路。</p></div>
                            <div class="registry-grid" id="registry-grid"></div>

                            <article class="panel custom-registry-panel hidden" id="custom-registry-panel">
                                <div class="field-heading"><div><label for="custom-registry">自定义下载地址</label><p>仅供了解 npm Registry 的用户使用</p></div></div>
                                <div class="input-with-icon">${icon('globe')}<input id="custom-registry" type="url" placeholder="https://registry.example.com/" spellcheck="false"></div>
                            </article>

                            <article class="panel proxy-panel">
                                <div class="panel-heading"><div><h3>是否使用代理</h3><p>家庭网络通常选择“自动跟随电脑”。</p></div><span class="soft-badge" id="proxy-badge">自动跟随电脑</span></div>
                                <div class="choice-tabs" role="radiogroup" aria-label="代理设置">
                                    <button class="choice-tab" data-proxy="direct"><span class="choice-icon">${icon('bolt')}</span><span><strong>不使用代理</strong><small>直接连接下载线路</small></span><i></i></button>
                                    <button class="choice-tab active" data-proxy="system"><span class="choice-icon">${icon('activity')}</span><span><strong>自动跟随电脑</strong><small>推荐大多数用户选择</small></span><i></i></button>
                                    <button class="choice-tab" data-proxy="custom"><span class="choice-icon">${icon('network')}</span><span><strong>填写代理地址</strong><small>适合公司网络或代理软件</small></span><i></i></button>
                                </div>
                                <div class="proxy-input hidden" id="proxy-input-wrap">
                                    <label for="proxy-url">代理地址</label>
                                    <div class="input-with-icon">${icon('network')}<input id="proxy-url" type="url" placeholder="例如：http://127.0.0.1:7890" spellcheck="false"></div>
                                    <p>不知道代理地址时，请选择“自动跟随电脑”。</p>
                                </div>
                            </article>

                            <div class="hidden"><input id="auto-start" type="checkbox" checked><input id="auto-open" type="checkbox" checked></div>

                            <div class="sticky-actions">
                                <div id="network-feedback"><span class="status-mini"></span>修改后请点击保存</div>
                                <div><button class="secondary-button" data-action="test-current">测试能否下载</button><button class="primary-button" data-action="save-config">${icon('check')}保存并返回安装</button></div>
                            </div>
                        </div>
                    </details>
                </section>

                <section class="page logs-page" data-page-view="logs">
                    <article class="log-console">
                        <div class="console-header">
                            <div class="console-title"><span class="console-lights"><i></i><i></i><i></i></span><div><strong>运行日志</strong><small id="console-status">等待事件</small></div></div>
                            <div class="console-tools">
                                <label class="log-search">${icon('search', 15)}<input id="log-search" placeholder="筛选日志"></label>
                                <select id="log-level" aria-label="日志级别"><option value="all">全部级别</option><option value="success">成功</option><option value="warning">警告</option><option value="error">错误</option></select>
                                <button data-action="copy-logs" title="复制当前日志">${icon('copy', 16)}</button>
                                <button data-action="clear-logs" title="清空界面日志">${icon('close', 16)}</button>
                            </div>
                        </div>
                        <div class="console-body" id="console-body"><div class="terminal-empty"><span>&gt;_</span><p>部署或启动服务后，这里会实时显示输出。</p></div></div>
                        <div class="console-footer"><span><i class="live-dot"></i>事件流已连接</span><label><input id="auto-scroll" type="checkbox" checked>自动滚动</label><span id="visible-log-count">0 条</span></div>
                    </article>
                </section>
            </div>
        </main>
    </div>
</div>
<div class="toast-stack" id="toast-stack"></div>
`;

let currentStatus: Status | null = null;
let currentPage: PageName = 'overview';
let currentProxyMode = 'system';
let selectedRegistry = 'npmmirror';
let allLogs: LogEntry[] = [];
let benchmarkResults = new Map<string, RegistryResult>();
let pendingLogs: LogEntry[] = [];
let logFrame = 0;
let configInitialised = false;

const fallbackRegistries = [
    {id: 'npmmirror', name: 'NPMirror', url: 'https://registry.npmmirror.com/', description: '中国大陆推荐，通常延迟最低', recommended: true},
    {id: 'official', name: 'npm 官方', url: 'https://registry.npmjs.org/', description: '官方源，包同步最及时', recommended: false},
    {id: 'tencent', name: '腾讯云', url: 'https://mirrors.cloud.tencent.com/npm/', description: '腾讯云开源镜像站', recommended: false},
    {id: 'huawei', name: '华为云', url: 'https://repo.huaweicloud.com/repository/npm/', description: '华为云开源镜像站', recommended: false},
];

const $ = <T extends HTMLElement>(selector: string): T => document.querySelector<T>(selector)!;
const $$ = <T extends HTMLElement>(selector: string): T[] => Array.from(document.querySelectorAll<T>(selector));

function escapeHTML(value: string): string {
    return value.replace(/[&<>'"]/g, (character) => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;'}[character]!));
}

function registryName(id: string): string {
    if (id === 'custom') return '自定义镜像';
    return (currentStatus?.registries ?? fallbackRegistries).find((item) => item.id === id)?.name ?? id;
}

function proxyLabel(mode: string): string {
    return {direct: '不使用代理', system: '自动跟随电脑', custom: '填写代理地址'}[mode] ?? mode;
}

function serviceLabel(value: string): string {
    return {running: '运行中', external: '外部运行', stopped: '未运行'}[value] ?? value;
}

function setText(selector: string, value: string): void {
    const element = document.querySelector<HTMLElement>(selector);
    if (element) element.textContent = value;
}

function showPage(page: PageName): void {
    currentPage = page;
    const meta = {
        overview: ['EASY INSTALL', '一键安装', '点击一次，自动完成下载、安装和打开'],
        network: ['NETWORK SETTINGS', '网络设置', '安装失败或下载缓慢时，可以在这里调整'],
        logs: ['INSTALL DETAILS', '安装日志', '需要排查问题时查看详细安装信息'],
    }[page];
    $$('.nav-item').forEach((item) => item.classList.toggle('active', item.dataset.page === page));
    $$('.page').forEach((item) => item.classList.toggle('active', item.dataset.pageView === page));
    setText('#page-eyebrow', meta[0]);
    setText('#page-title', meta[1]);
    setText('#page-subtitle', meta[2]);
    if (page === 'logs') renderLogs();
}

function applyStatus(status: Status, syncForm = false): void {
    currentStatus = status;
    allLogs = mergeLogs(status.logs ?? []);
    setText('#platform-name', status.platform);
    setText('#arch-name', status.architecture.toUpperCase());
    setText('#install-path-value', status.installPath);
    setText('#install-stat', status.installed ? '就绪' : '待装');
    setText('#node-stat', status.nodeReady ? 'v22' : '待装');
    setText('#service-stat', status.service === 'stopped' ? '离线' : '在线');
    setText('#sidebar-state', status.service === 'stopped' ? (status.installed ? '已经安装，可以使用' : '电脑检查完成') : 'Harness 正在运行');
    setText('#version-value', status.installed ? `v${status.version}` : '尚未安装');
    setText('#version-hint', status.installed ? '已通过本地文件验证' : '点击一键部署开始');
    setText('#version-state', status.installed ? '已就绪' : '待部署');
    setText('#service-value', serviceLabel(status.service));
    setText('#registry-value', registryName(status.config.registryId));
    setText('#proxy-value', proxyLabel(status.config.proxyMode));
    setText('#overview-badge', status.job.phase === 'running' ? '···' : (status.installed ? '✓' : '1'));
    setText('#log-count', String(allLogs.length));

    const online = status.service !== 'stopped';
    $('#service-pulse').classList.toggle('online', online);
    const header = $('#header-status');
    header.className = `status-pill ${status.job.phase === 'running' ? 'ready' : (online ? 'online' : (status.installed ? 'ready' : ''))}`;
    header.querySelector('b')!.textContent = status.job.phase === 'running' ? (status.job.type === 'uninstall' ? '正在卸载' : '正在安装') : (online ? '可以使用' : (status.installed ? '可以启动' : '等待安装'));
    $('#network-dot').classList.toggle('online', status.config.registryUrl.length > 0);

    const serviceButton = $<HTMLButtonElement>('#service-button');
    const stopButton = $<HTMLButtonElement>('#stop-button');
    const serviceOnline = status.service !== 'stopped';
    serviceButton.dataset.action = serviceOnline ? 'open-service' : 'service';
    serviceButton.innerHTML = serviceOnline
        ? `${icon('play')}<span id="service-button-text">打开 Harness</span>`
        : `${icon('play')}<span id="service-button-text">启动并打开</span>`;
    serviceButton.classList.toggle('hidden', !status.installed || status.job.phase === 'running');
    stopButton.classList.toggle('hidden', !serviceOnline || status.job.phase === 'running');

    const deployButton = $('#install-button') as HTMLButtonElement;
    deployButton.disabled = status.job.phase === 'running';
    deployButton.classList.toggle('hidden', (status.job.type === 'uninstall' && status.job.phase === 'running') || (status.installed && status.job.phase !== 'running'));
    setText('#deploy-button-text', status.job.phase === 'running' ? '正在自动安装…' : '立即安装并打开');
    $('#cancel-button').classList.toggle('hidden', status.job.phase !== 'running' || status.job.type !== 'deploy');
    const uninstallButton = $<HTMLButtonElement>('#uninstall-button');
    uninstallButton.classList.toggle('hidden', (!status.installed && !status.nodeReady) || status.job.phase === 'running');
    uninstallButton.disabled = status.job.phase === 'running';
    applyJob(status.job);
    updateSteps(status);
    renderActivity();

    if (syncForm || !configInitialised) {
        syncConfigForm(status.config);
        configInitialised = true;
    }
    if (currentPage === 'logs') renderLogs();
}

function applyJob(job: Status['job']): void {
    const progress = Math.max(0, Math.min(100, job.progress || 0));
    let friendlyTitle = job.title || '等待安装';
    let friendlyMessage = job.message || '第一次安装通常需要几分钟，请保持网络连接。';
    const uninstalling = job.type === 'uninstall';
    if (uninstalling && job.phase === 'running') {
        friendlyTitle = job.title || '正在卸载';
        friendlyMessage = job.message || '正在安全移除 Harness 和运行环境。';
    } else if (job.phase === 'running') {
        if (progress < 8) {
            friendlyTitle = '正在检查这台电脑';
            friendlyMessage = '马上开始下载，请保持网络连接。';
        } else if (progress < 50) {
            friendlyTitle = '正在下载安装环境';
            friendlyMessage = '下载时间取决于当前网速，请耐心等待。';
        } else if (progress < 94) {
            friendlyTitle = '正在安装 DeepSeek Harness';
            friendlyMessage = '这一步需要处理较多文件，界面可以继续正常操作。';
        } else {
            friendlyTitle = '马上就好';
            friendlyMessage = '正在检查安装结果并准备打开。';
        }
    } else if (uninstalling && job.phase === 'success') {
        friendlyTitle = '卸载完成';
        friendlyMessage = '已移除 Harness 和运行环境，可随时重新安装。';
    } else if (job.phase === 'success') {
        friendlyTitle = '安装完成';
        friendlyMessage = 'DeepSeek Harness 已经可以使用。';
    }
    $('#progress-bar').style.width = `${progress}%`;
    setText('#progress-value', `${Math.round(progress)}%`);
    setText('#progress-title', friendlyTitle);
    setText('#progress-message', friendlyMessage);
    $('#progress-wrap').className = `progress-wrap ${job.phase}`;
    if (uninstalling && job.phase === 'running') {
        setText('#hero-title', '正在安全卸载，请稍等');
        setText('#hero-description', '运行服务会先停止，然后清理 Harness 和内置运行环境。');
    } else if (job.phase === 'running') {
        setText('#hero-title', '正在自动安装，请稍等');
        setText('#hero-description', '你可以查看下方进度，安装完成后会自动打开。');
    } else if (uninstalling && job.phase === 'success') {
        setText('#hero-title', '卸载完成，可以重新安装');
        setText('#hero-description', '网络和镜像设置已经保留，需要时点击按钮重新安装。');
    } else if (job.phase === 'success') {
        setText('#hero-title', '安装完成，可以开始使用');
        setText('#hero-description', '点击“打开 Harness”进入工作页面。');
    } else {
        setText('#hero-title', currentStatus?.installed ? '已经安装完成' : '点击按钮，剩下的自动完成');
        setText('#hero-description', currentStatus?.installed ? '点击“启动并打开”即可继续使用。' : '无需安装其他软件，无需输入命令。完成后会自动打开 DeepSeek Harness。');
    }
}

function updateSteps(status: Status): void {
    const progress = status.job.progress || 0;
    const marks = [
        {name: 'network', done: status.nodeReady || progress > 48, active: progress > 0 && progress <= 48},
        {name: 'install', done: status.installed || progress >= 94, active: progress > 48 && progress < 100},
        {name: 'open', done: status.service !== 'stopped', active: status.installed && status.service === 'stopped'},
    ];
    for (const mark of marks) {
        const row = $<HTMLElement>(`[data-step="${mark.name}"]`);
        row.classList.toggle('done', mark.done);
        row.classList.toggle('active', !mark.done && mark.active);
        row.querySelector('.step-state')!.textContent = mark.done ? '完成' : (mark.active ? '进行中' : '等待');
    }
}

function syncConfigForm(config: Config): void {
    selectedRegistry = config.registryId;
    currentProxyMode = config.proxyMode;
    $('#custom-registry').setAttribute('value', config.registryUrl);
    ($<HTMLInputElement>('#custom-registry')).value = config.registryUrl;
    ($<HTMLInputElement>('#proxy-url')).value = config.proxyUrl;
    ($<HTMLInputElement>('#auto-start')).checked = config.autoStart;
    ($<HTMLInputElement>('#auto-open')).checked = config.autoOpen;
    $<HTMLDetailsElement>('#advanced-settings').open = config.registryId === 'custom' || config.proxyMode === 'custom';
    updateProxyUI();
    renderRegistries();
}

function updateProxyUI(): void {
    $$<HTMLButtonElement>('[data-proxy]').forEach((button) => button.classList.toggle('active', button.dataset.proxy === currentProxyMode));
    $('#proxy-input-wrap').classList.toggle('hidden', currentProxyMode !== 'custom');
    setText('#proxy-badge', proxyLabel(currentProxyMode));
}

function renderRegistries(): void {
    if (!currentStatus) return;
    const cards = [...(currentStatus?.registries ?? fallbackRegistries), {id: 'custom', name: '自定义', url: '', description: '企业私有仓库或其他兼容镜像', recommended: false}];
    $('#registry-grid').innerHTML = cards.map((item) => {
        const result = benchmarkResults.get(item.id);
        const latency = result ? (result.ok ? `${result.latencyMs} ms` : '不可用') : '等待测速';
        const grade = result ? (result.ok && result.latencyMs < 600 ? 'fast' : (result.ok ? 'slow' : 'failed')) : '';
        return `<button class="registry-card ${selectedRegistry === item.id ? 'selected' : ''}" data-registry="${item.id}">
            <span class="registry-select">${selectedRegistry === item.id ? icon('check', 13) : ''}</span>
            <span class="registry-top"><span class="registry-logo ${item.id}">${item.id === 'custom' ? '+' : item.name.slice(0, 1).toUpperCase()}</span>${item.recommended ? '<i>推荐</i>' : ''}</span>
            <strong>${escapeHTML(item.name)}</strong><small>${escapeHTML(item.description)}</small>
            <span class="latency ${grade}"><i></i>${latency}</span>
        </button>`;
    }).join('');
    $('#custom-registry-panel').classList.toggle('hidden', selectedRegistry !== 'custom');
}

function readConfigForm(): Config {
    const preset = (currentStatus?.registries ?? fallbackRegistries).find((item) => item.id === selectedRegistry);
    return {
        registryId: selectedRegistry,
        registryUrl: selectedRegistry === 'custom' ? $<HTMLInputElement>('#custom-registry').value.trim() : (preset?.url ?? ''),
        proxyMode: currentProxyMode,
        proxyUrl: currentProxyMode === 'custom' ? $<HTMLInputElement>('#proxy-url').value.trim() : '',
        autoOpen: $<HTMLInputElement>('#auto-open').checked,
        autoStart: $<HTMLInputElement>('#auto-start').checked,
        installChannel: 'stable',
    } as Config;
}

function mergeLogs(incoming: LogEntry[]): LogEntry[] {
    const map = new Map<number, LogEntry>();
    for (const item of allLogs) map.set(item.id, item);
    for (const item of incoming) map.set(item.id, item);
    return Array.from(map.values()).sort((a, b) => a.id - b.id).slice(-800);
}

function queueLog(entry: LogEntry): void {
    pendingLogs.push(entry);
    if (logFrame) return;
    logFrame = requestAnimationFrame(() => {
        allLogs = mergeLogs(pendingLogs.splice(0));
        logFrame = 0;
        setText('#log-count', String(allLogs.length));
        renderActivity();
        if (currentPage === 'logs') renderLogs();
    });
}

function renderActivity(): void {
    const target = $('#activity-list');
    const items = allLogs.slice(-5).reverse();
    if (!items.length) {
        target.innerHTML = '<div class="empty-state">运行日志会显示在这里</div>';
        return;
    }
    target.innerHTML = items.map((item) => `<div class="activity-item"><span class="activity-dot ${item.level}"></span><div><strong>${escapeHTML(item.text)}</strong><p>${escapeHTML(item.source)} · ${item.time}</p></div></div>`).join('');
}

function filteredLogs(): LogEntry[] {
    const query = $<HTMLInputElement>('#log-search').value.trim().toLowerCase();
    const level = $<HTMLSelectElement>('#log-level').value;
    return allLogs.filter((item) => (level === 'all' || item.level === level) && (!query || `${item.source} ${item.text}`.toLowerCase().includes(query))).slice(-500);
}

function renderLogs(): void {
    const target = $('#console-body');
    const rows = filteredLogs();
    setText('#visible-log-count', `${rows.length} 条`);
    setText('#console-status', currentStatus?.job.phase === 'running' ? currentStatus.job.title : (currentStatus?.service === 'running' ? '服务正在运行' : '事件流已连接'));
    if (!rows.length) {
        target.innerHTML = '<div class="terminal-empty"><span>&gt;_</span><p>当前筛选条件下没有日志。</p></div>';
        return;
    }
    target.innerHTML = rows.map((item) => `<div class="log-row ${item.level}"><span class="log-time">${item.time}</span><span class="log-source">${escapeHTML(item.source)}</span><span class="log-message">${escapeHTML(item.text)}</span></div>`).join('');
    if ($<HTMLInputElement>('#auto-scroll').checked) target.scrollTop = target.scrollHeight;
}

function toast(message: string, type: 'success' | 'error' | 'info' = 'info'): void {
    const item = document.createElement('div');
    item.className = `toast ${type}`;
    item.innerHTML = `<span>${type === 'success' ? icon('check', 15) : (type === 'error' ? icon('close', 15) : icon('activity', 15))}</span><p>${escapeHTML(message)}</p>`;
    $('#toast-stack').appendChild(item);
    requestAnimationFrame(() => item.classList.add('show'));
    window.setTimeout(() => { item.classList.remove('show'); window.setTimeout(() => item.remove(), 220); }, 3500);
}

function setBusy(action: string, busy: boolean): void {
    const button = document.querySelector<HTMLButtonElement>(`[data-action="${action}"]`);
    if (!button) return;
    button.disabled = busy;
    button.classList.toggle('loading', busy);
}

async function refreshStatus(syncForm = false): Promise<void> {
    try {
        applyStatus(await GetStatus(), syncForm);
    } catch (error) {
        toast(String(error), 'error');
    }
}

async function saveConfiguration(showMessage = true): Promise<void> {
    const saved = await SaveConfig(readConfigForm());
    if (currentStatus) currentStatus.config = saved;
    if (showMessage) toast('网络与镜像配置已保存', 'success');
    setText('#network-feedback', '配置已保存并会应用到下一次操作');
    await refreshStatus(true);
}

async function runAction(action: string): Promise<void> {
    try {
        switch (action) {
            case 'minimise': WindowMinimise(); return;
            case 'maximise': WindowToggleMaximise(); return;
            case 'quit': Quit(); return;
            case 'refresh': await refreshStatus(); toast('状态已刷新', 'success'); return;
            case 'open-folder': await OpenInstallDirectory(); return;
            case 'uninstall':
                if (!window.confirm('确认一键卸载？\n\n将删除 Harness 和内置运行环境，保留 Harness Studio 软件与网络设置。')) return;
                await Uninstall();
                toast('卸载已经开始，请稍等', 'info');
                await refreshStatus();
                return;
            case 'open-service': await OpenService(); return;
            case 'deploy':
                await saveConfiguration(false);
                await Deploy();
                showPage('overview');
                toast('安装已经开始，请耐心等待', 'success');
                await refreshStatus();
                return;
            case 'cancel': await CancelTask(); toast('正在取消部署任务', 'info'); return;
            case 'service':
                if (currentStatus?.service === 'running') await StopService();
                else await StartService(true);
                await refreshStatus();
                return;
            case 'benchmark': {
                setBusy(action, true);
                setText('#network-feedback', '正在测试可用的下载线路…');
                const results = await BenchmarkRegistries();
                benchmarkResults = new Map(results.map((item) => [item.id, item]));
                renderRegistries();
                const available = results.filter((item) => item.ok).sort((a, b) => a.latencyMs - b.latencyMs);
                if (available.length) {
                    selectedRegistry = available[0].id;
                    renderRegistries();
                    await saveConfiguration(false);
                }
                setText('#network-feedback', available.length ? `已自动选择 ${registryName(available[0].id)}` : '当前没有检测到可用下载线路');
                toast(available.length ? `已选择最快线路：${registryName(available[0].id)}` : '自动测速没有通过', available.length ? 'success' : 'error');
                return;
            }
            case 'test-current': {
                await saveConfiguration(false);
                setBusy(action, true);
                const result = await TestCurrentRegistry();
                toast(result.ok ? `连接成功 · ${result.latencyMs} ms` : `连接失败 · ${result.message}`, result.ok ? 'success' : 'error');
                return;
            }
            case 'save-config': await saveConfiguration(); showPage('overview'); return;
            case 'stop-service': await StopService(); await refreshStatus(); toast('Harness 已停止运行', 'success'); return;
            case 'copy-logs':
                await navigator.clipboard.writeText(filteredLogs().map((item) => `[${item.time}] [${item.source}] ${item.text}`).join('\n'));
                toast('日志已复制', 'success'); return;
            case 'clear-logs':
                allLogs = [];
                renderLogs(); renderActivity(); setText('#log-count', '0');
                toast('已清空界面日志', 'success'); return;
        }
    } catch (error) {
        toast(String(error).replace(/^Error:\s*/, ''), 'error');
    } finally {
        setBusy(action, false);
    }
}

document.addEventListener('click', (event) => {
    const target = (event.target as HTMLElement).closest<HTMLElement>('[data-action], [data-page], [data-registry], [data-proxy]');
    if (!target) return;
    if (target.dataset.page) showPage(target.dataset.page as PageName);
    if (target.dataset.action) void runAction(target.dataset.action);
    if (target.dataset.registry) {
        selectedRegistry = target.dataset.registry;
        renderRegistries();
        setText('#network-feedback', `${registryName(selectedRegistry)} 已选中，保存后生效`);
    }
    if (target.dataset.proxy) {
        currentProxyMode = target.dataset.proxy;
        updateProxyUI();
        setText('#network-feedback', `${proxyLabel(currentProxyMode)} 已选中，保存后生效`);
    }
});

$('#log-search').addEventListener('input', renderLogs);
$('#log-level').addEventListener('change', renderLogs);
$('#auto-scroll').addEventListener('change', renderLogs);

const hasWailsRuntime = typeof (window as Window & {runtime?: {EventsOnMultiple?: unknown}}).runtime?.EventsOnMultiple === 'function';
if (hasWailsRuntime) {
    EventsOn('studio:log', (entry: LogEntry) => queueLog(entry));
    EventsOn('studio:job', (job: Status['job']) => {
        if (!currentStatus) return;
        currentStatus.job = job;
        applyJob(job);
        updateSteps(currentStatus);
        setText('#overview-badge', job.phase === 'running' ? '···' : (currentStatus.installed ? '✓' : '1'));
        $('#cancel-button').classList.toggle('hidden', job.phase !== 'running' || job.type !== 'deploy');
    });
    EventsOn('studio:status', (status: Status) => applyStatus(status));
    void refreshStatus(true);
    window.setInterval(() => void refreshStatus(), 5000);
} else {
    const preview = {
        installed: false, version: '', nodeReady: false, nodeVersion: '22.19.0', service: 'stopped', servicePid: 0,
        platform: 'Windows', architecture: 'amd64', installPath: 'C:\\Users\\用户名\\AppData\\Roaming\\HarnessStudio\\engine', serviceUrl: 'http://127.0.0.1:3080',
        job: {type: '', phase: 'idle', title: '等待安装', message: '第一次安装通常需要几分钟，请保持网络连接。', progress: 0, startedAt: 0, finishedAt: 0},
        config: {registryId: 'npmmirror', registryUrl: fallbackRegistries[0].url, proxyMode: 'system', proxyUrl: '', autoOpen: true, autoStart: true, installChannel: 'stable'},
        registries: fallbackRegistries, logs: [], downloadSupport: true,
    } as unknown as Status;
    applyStatus(preview, true);
}
