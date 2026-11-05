'use strict';

/* rbackup-ui Web UI —— 原生 JS，无框架、无构建步骤。 */

const S = {
  csrf: '',
  version: '',
  readOnly: false,
  tokenFree: false,
  config: {},
  tasks: [],
  selected: new Set(),
  job: null,
  lastJob: null,
  previewToken: '',
  pendingConfirmAfterPreview: false,
  es: null,
  paused: false,
  stats: null,

  // 登录状态
  auth: { methods: [], totp: false, readOnly: false, version: '' },
  hasToken: false,
  hasPassword: false,
  mode: 'token', // token | password | none
  step: 'login', // login | totp
};

const $ = (id) => document.getElementById(id);
const el = (tag, cls) => { const n = document.createElement(tag); if (cls) n.className = cls; return n; };
const esc = (s) => String(s == null ? '' : s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

/* ---------- 主题（自动 / 亮 / 暗） ---------- */

const THEME_KEY = 'rbtui-theme';
const THEME_ICON = { auto: '🌓', light: '☀️', dark: '🌙' };
const THEME_NAME = { auto: '跟随系统', light: '亮色', dark: '暗色' };
const THEME_ORDER = ['auto', 'light', 'dark'];

function currentTheme() {
  try { return localStorage.getItem(THEME_KEY) || 'auto'; } catch (e) { return 'auto'; }
}

function applyTheme(mode) {
  const root = document.documentElement;
  if (mode === 'light' || mode === 'dark') root.dataset.theme = mode;
  else delete root.dataset.theme;
  try { localStorage.setItem(THEME_KEY, mode); } catch (e) { /* ignore */ }
  for (const btn of [$('themeBtn'), $('loginThemeBtn')]) {
    if (btn) {
      btn.textContent = THEME_ICON[mode];
      btn.title = '主题：' + THEME_NAME[mode] + '（点击切换）';
    }
  }
}

function cycleTheme() {
  applyTheme(THEME_ORDER[(THEME_ORDER.indexOf(currentTheme()) + 1) % THEME_ORDER.length]);
}

/* ---------- API ---------- */

async function api(method, path, body) {
  const headers = {};
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (S.csrf && method !== 'GET') headers['X-RB-CSRF'] = S.csrf;
  const res = await fetch(path, {
    method,
    headers,
    credentials: 'same-origin',
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  let data = null;
  if (text) { try { data = JSON.parse(text); } catch (_) { data = { error: { message: text } }; } }
  if (!res.ok) {
    const err = new Error((data && data.error && data.error.message) || ('HTTP ' + res.status));
    err.code = data && data.error && data.error.code;
    err.status = res.status;
    throw err;
  }
  return data;
}

/* ---------- 视图切换 ---------- */

function showLogin(msg) {
  $('loginView').classList.remove('hidden');
  $('appView').classList.add('hidden');
  if (msg) showLoginError(msg);
}

function showApp() {
  $('loginView').classList.add('hidden');
  $('appView').classList.remove('hidden');
}

function showLoginError(msg) {
  const n = $('loginError');
  n.textContent = msg || '';
  n.classList.toggle('hidden', !msg);
}

function showBanner(msg) {
  const b = $('banner');
  b.textContent = msg;
  b.classList.remove('hidden');
}

function hideBanner() { $('banner').classList.add('hidden'); }

function handleError(e) {
  if (e && e.status === 401) {
    showApp();
    showLogin('会话已失效，请重新登录');
    return;
  }
  showBanner((e && e.message) || '操作失败');
}

/* ---------- 登录（token / 密码 / TOTP） ---------- */

async function fetchAuthInfo() {
  try {
    const r = await api('GET', '/api/authinfo');
    S.auth = r;
  } catch (e) {
    S.auth = { methods: ['token'], totp: false };
  }
  S.hasToken = (S.auth.methods || []).includes('token');
  S.hasPassword = (S.auth.methods || []).includes('password');
  S.mode = S.hasPassword ? 'password' : (S.hasToken ? 'token' : 'none');
  S.version = S.auth.version || S.version;
  $('loginVersion').textContent = S.version || '';
  renderLoginForm();
}

function renderLoginForm() {
  const tokenFree = S.mode === 'none';
  $('passwordFields').classList.toggle('hidden', S.mode !== 'password');
  $('tokenFields').classList.toggle('hidden', tokenFree || S.mode !== 'token');
  $('totpFields').classList.add('hidden');
  $('loginBtn').textContent = tokenFree ? '进入' : '登录';
  $('switchAuthBtn').classList.toggle('hidden', !(S.hasPassword && S.hasToken));
  $('switchAuthBtn').textContent = S.mode === 'password' ? '改用 token 登录' : '改用用户名密码登录';

  let hint = '正在获取登录方式…';
  if (tokenFree) hint = '本机模式：无需凭据，点击下方按钮进入。';
  else if (S.mode === 'password') hint = S.auth.totp ? '用户名密码登录，随后需要 6 位动态码。' : '用户名密码登录。';
  else hint = '请输入访问 token。';
  $('loginHint').textContent = hint;
  $('loginFoot').textContent = '';
  S.step = 'login';
  showLoginError('');
}

function showTotpStep() {
  S.step = 'totp';
  $('passwordFields').classList.add('hidden');
  $('tokenFields').classList.add('hidden');
  $('totpFields').classList.remove('hidden');
  $('switchAuthBtn').classList.add('hidden');
  $('loginHint').textContent = '二次验证：输入认证器 App 中的 6 位动态码。';
  $('loginBtn').textContent = '验证';
  showLoginError('');
  $('totpInput').value = '';
  $('totpInput').focus();
}

function enterApp(r) {
  S.csrf = r.csrf || S.csrf;
  S.version = r.version || S.version;
  S.readOnly = !!r.readOnly;
  S.tokenFree = !!r.tokenFree;
  showApp();
  loadState().then(() => connectSSE()).catch(handleError);
}

$('loginForm').addEventListener('submit', async (ev) => {
  ev.preventDefault();
  showLoginError('');
  $('loginBtn').disabled = true;
  try {
    if (S.step === 'totp') {
      const code = ($('totpInput').value || '').trim();
      if (!/^[0-9]{6}$/.test(code)) throw new Error('请输入 6 位数字动态码');
      enterApp(await api('POST', '/api/session/totp', { code }));
      return;
    }
    if (S.mode === 'none') {
      enterApp(await api('POST', '/api/session', {}));
    } else if (S.mode === 'password') {
      const r = await api('POST', '/api/session', {
        username: $('userInput').value,
        password: $('passInput').value,
      });
      if (r.totpRequired) { showTotpStep(); return; }
      enterApp(r);
    } else {
      enterApp(await api('POST', '/api/session', { token: $('tokenInput').value }));
    }
  } catch (e) {
    showLoginError(e.message || '登录失败');
    if (e.code === 'pending_expired') { renderLoginForm(); }
    $('passInput').value = '';
    if (S.step === 'totp') $('totpInput').value = '';
  } finally {
    $('loginBtn').disabled = false;
  }
});

$('switchAuthBtn').addEventListener('click', () => {
  S.mode = S.mode === 'password' ? 'token' : 'password';
  renderLoginForm();
});

$('logoutBtn').addEventListener('click', async () => {
  try { await api('DELETE', '/api/session'); } catch (_) { /* ignore */ }
  if (S.es) { S.es.close(); S.es = null; }
  S.csrf = '';
  S.selected = new Set();
  showLogin();
  renderLoginForm();
});

/* ---------- 状态加载与渲染 ---------- */

async function loadState() {
  const s = await api('GET', '/api/state');
  S.version = s.version || S.version;
  S.readOnly = !!s.readOnly;
  if (s.csrf) S.csrf = s.csrf;
  S.config = s.config || {};
  S.tasks = s.tasks || [];
  S.job = s.job || null;
  S.lastJob = s.lastJob || null;
  const known = new Set(S.tasks.map((t) => t.name));
  S.selected = new Set([...S.selected].filter((n) => known.has(n)));
  if (s.lastJob && s.lastJob.kind === 'dryrun' && s.lastJob.state === 'done' && s.lastJob.previewToken) {
    S.previewToken = s.lastJob.previewToken;
  }
  renderAll();
  if (s.configError) showBanner('配置加载失败：' + s.configError); else hideBanner();
}

function renderAll() {
  $('version').textContent = S.version;
  renderMeta();
  renderInfo();
  renderTasks();
  renderJob();
  renderStats();
}

function renderMeta() {
  const c = S.config || {};
  $('meta').textContent = c.path ? ('配置: ' + c.path + (S.readOnly ? ' · 只读模式' : '')) : (S.readOnly ? '只读模式' : '');
}

function renderInfo() {
  const c = S.config || {};
  const items = [
    ['远端', (c.sshUser || '') + '@' + (c.host || '') + (c.sshPort ? ':' + c.sshPort : '')],
    ['门禁策略', c.policy || '-'],
    ['脚本', c.script || '-'],
    ['日志目录', c.logDir || '-'],
  ];
  const wrap = $('info');
  wrap.innerHTML = '';
  for (const [k, v] of items) {
    const n = el('span');
    n.innerHTML = '<b>' + esc(k) + '</b> ' + esc(v);
    wrap.appendChild(n);
  }
}

function gateText(t) {
  if (t.gate === 'require_mounted') return '需已挂载';
  if (t.gate === 'require_unmounted') return '需未挂载';
  return '—';
}

function dangerText(t) {
  if (!t.dangerous) return '';
  return [t.delete === 'yes' ? '删除' : '', t.removeSource === 'yes' ? '删源' : ''].filter(Boolean).join('+') || '危险';
}

function toggleTask(name, checked) {
  if (checked) S.selected.add(name); else S.selected.delete(name);
  renderButtons();
  syncSelectAll();
}

function syncSelectAll() {
  const all = $('selectAll');
  all.checked = S.tasks.length > 0 && S.selected.size === S.tasks.length;
}

function renderTasks() {
  const body = $('taskBody');
  body.innerHTML = '';
  const cards = $('taskCards');
  cards.innerHTML = '';
  $('emptyHint').classList.toggle('hidden', S.tasks.length > 0);
  const disabled = isBusy() || S.readOnly;

  for (const t of S.tasks) {
    /* --- 桌面表格行 --- */
    const tr = el('tr');
    const td1 = el('td', 'c-chk');
    const cb = el('input');
    cb.type = 'checkbox';
    cb.checked = S.selected.has(t.name);
    cb.disabled = disabled;
    cb.addEventListener('change', () => toggleTask(t.name, cb.checked));
    td1.appendChild(cb);
    tr.appendChild(td1);

    const tdName = el('td');
    tdName.textContent = t.name;
    tr.appendChild(tdName);

    const tdSrc = el('td', 'c-src');
    tdSrc.textContent = t.src || '';
    tdSrc.title = t.src || '';
    tr.appendChild(tdSrc);

    const tdDst = el('td', 'c-dst');
    tdDst.textContent = t.dst || '';
    tdDst.title = t.dst || '';
    tr.appendChild(tdDst);

    const tdGate = el('td', 'c-gate');
    tdGate.textContent = gateText(t);
    tr.appendChild(tdGate);

    const tdDanger = el('td', 'c-danger');
    if (t.dangerous) {
      const b = el('span', 'badge failed');
      b.textContent = dangerText(t);
      tdDanger.appendChild(b);
    } else {
      tdDanger.textContent = '—';
    }
    tr.appendChild(tdDanger);
    body.appendChild(tr);

    /* --- 手机端卡片 --- */
    const card = el('div', 'task-card');
    const ccb = el('input');
    ccb.type = 'checkbox';
    ccb.checked = S.selected.has(t.name);
    ccb.disabled = disabled;
    ccb.addEventListener('change', () => toggleTask(t.name, ccb.checked));
    const main = el('div', 'tc-main');
    const name = el('div', 'tc-name');
    name.textContent = t.name;
    main.appendChild(name);
    const src = el('div', 'tc-line');
    src.textContent = '源: ' + (t.src || '-');
    main.appendChild(src);
    const dst = el('div', 'tc-line');
    dst.textContent = '目标: ' + (t.dst || '-');
    main.appendChild(dst);
    const badges = el('div', 'badge-row');
    const gb = el('span', 'badge');
    gb.textContent = '门禁: ' + gateText(t);
    badges.appendChild(gb);
    if (t.dangerous) {
      const db = el('span', 'badge failed');
      db.textContent = dangerText(t);
      badges.appendChild(db);
    }
    main.appendChild(badges);
    card.appendChild(ccb);
    card.appendChild(main);
    card.addEventListener('click', (ev) => {
      if (ev.target === ccb) return;
      ccb.checked = !ccb.checked;
      toggleTask(t.name, ccb.checked);
    });
    cards.appendChild(card);
  }

  const all = $('selectAll');
  all.disabled = disabled;
  all.checked = S.tasks.length > 0 && S.selected.size === S.tasks.length;
  renderButtons();
}

function isBusy() {
  return !!(S.job && (S.job.state === 'running' || S.job.state === 'canceling'));
}

function renderButtons() {
  const busy = isBusy();
  const disabled = busy || S.readOnly;
  for (const id of ['runBtn', 'dryBtn', 'mountBtn', 'reloadBtn']) $(id).disabled = disabled;
  $('selectAll').disabled = disabled;
  $('cancelBtn').classList.toggle('hidden', !busy || S.readOnly);
  for (const cb of document.querySelectorAll('#taskBody input[type=checkbox], #taskCards input[type=checkbox]')) {
    cb.disabled = disabled;
  }
}

const JOB_STATE_TEXT = {
  running: '运行中', canceling: '取消中', done: '完成', canceled: '已取消', failed: '失败',
};

function renderJob() {
  const b = $('jobState');
  const j = S.job;
  b.className = 'badge';
  if (!j) { b.textContent = '空闲'; b.classList.add('idle'); }
  else {
    b.textContent = JOB_STATE_TEXT[j.state] || j.state;
    b.classList.add(j.state === 'failed' ? 'failed' : (j.state === 'done' ? 'done' : (isBusy() ? 'running' : 'idle')));
    if (isBusy() && j.currentTask) b.textContent += ' · ' + j.currentTask;
  }
  renderButtons();
}

/* ---------- 操作 ---------- */

$('selectAll').addEventListener('change', (ev) => {
  S.selected = ev.target.checked ? new Set(S.tasks.map((t) => t.name)) : new Set();
  renderTasks();
});

function selectedNames() { return S.tasks.filter((t) => S.selected.has(t.name)).map((t) => t.name); }

function selectedDangerous() { return S.tasks.filter((t) => S.selected.has(t.name) && t.dangerous); }

async function startJob(body) {
  try {
    const r = await api('POST', '/api/jobs', body);
    S.job = r.job;
    renderJob();
    hideBanner();
  } catch (e) {
    if (e.code === 'preview_required') {
      S.previewToken = '';
      S.pendingConfirmAfterPreview = true;
      showBanner('危险任务的预览已失效，已自动重新执行 Dry-run 预览…');
      await startJob({ tasks: selectedNames(), dryRun: true });
      return;
    }
    handleError(e);
  }
}

async function runSelected(dryRun) {
  const names = selectedNames();
  if (!names.length) { showBanner('请先选择任务'); return; }
  if (S.readOnly) { showBanner('当前为只读模式'); return; }
  const danger = selectedDangerous();
  if (!dryRun && danger.length) {
    if (!S.previewToken) {
      S.pendingConfirmAfterPreview = true;
      showBanner('选中的任务包含危险操作，先执行 Dry-run 预览…');
      await startJob({ tasks: names, dryRun: true });
      return;
    }
    openConfirm(names, danger);
    return;
  }
  await startJob({ tasks: names, dryRun: !!dryRun });
}

function openConfirm(names, danger) {
  $('modalTitle').textContent = '确认执行危险任务';
  const body = $('modalBody');
  body.innerHTML = '';
  const p1 = el('p');
  p1.textContent = '以下任务包含删除类操作，Dry-run 预览已成功：';
  body.appendChild(p1);
  const ul = el('ul', 'modal-list');
  for (const t of danger) {
    const li = el('li');
    const flags = [
      t.delete === 'yes' ? '--delete（删除目标端多余文件）' : '',
      t.removeSource === 'yes' ? '--remove-source-files（删除本地源文件）' : '',
    ].filter(Boolean).join('，');
    li.textContent = t.name + '：' + flags;
    ul.appendChild(li);
  }
  body.appendChild(ul);
  const p2 = el('p', 'hint small');
  p2.textContent = '确认后将立即执行，操作不可撤销。';
  body.appendChild(p2);
  $('modal').classList.remove('hidden');
  $('modalOk').onclick = async () => {
    $('modal').classList.add('hidden');
    await startJob({ tasks: names, dryRun: false, confirm: true, previewToken: S.previewToken });
  };
}

$('modalCancel').addEventListener('click', () => $('modal').classList.add('hidden'));
$('modal').addEventListener('click', (ev) => { if (ev.target === $('modal')) $('modal').classList.add('hidden'); });

$('runBtn').addEventListener('click', () => runSelected(false));
$('dryBtn').addEventListener('click', () => runSelected(true));
$('mountBtn').addEventListener('click', async () => {
  if (S.readOnly) return;
  try {
    const r = await api('POST', '/api/mountcheck', {});
    S.job = r.job;
    renderJob();
  } catch (e) { handleError(e); }
});
$('cancelBtn').addEventListener('click', async () => {
  if (!confirm('确定取消当前运行？')) return;
  try {
    const r = await api('POST', '/api/jobs/current/cancel', {});
    S.job = r.job;
    renderJob();
  } catch (e) { handleError(e); }
});
$('reloadBtn').addEventListener('click', () => loadState().catch(handleError));
$('pauseBtn').addEventListener('click', () => {
  S.paused = !S.paused;
  $('pauseBtn').textContent = S.paused ? '继续滚动' : '暂停滚动';
});
$('clearBtn').addEventListener('click', () => { $('log').textContent = ''; });

/* ---------- 日志与 SSE ---------- */

function appendLog(text, cls) {
  const log = $('log');
  const span = el('span', cls || '');
  span.textContent = text + '\n';
  log.appendChild(span);
  while (log.childNodes.length > 3000) log.removeChild(log.firstChild);
  if (!S.paused) log.scrollTop = log.scrollHeight;
}

function appendSep(text) {
  appendLog('─'.repeat(12) + ' ' + text + ' ' + '─'.repeat(12), 'sep');
}

function onLog(d) {
  const prefix = d.task && d.task !== '__mountcheck__' ? '[' + d.task + '] ' : '';
  appendLog(prefix + (d.line || ''));
}

function onTaskStart(d) {
  appendSep('任务 ' + d.task + '（' + d.index + '/' + d.total + (d.dryRun ? '，Dry-run' : '') + '）');
}

function onTaskResult(d) {
  const map = { success: '成功', failed: '失败', canceled: '已取消' };
  let line = '>>> ' + d.task + ' ' + (map[d.result] || d.result) + '  用时: ' + (d.duration || '-');
  if (d.stats) {
    line += '  文件: ' + d.stats.filesTransferred + '/' + d.stats.filesTotal +
      '  总大小: ' + d.stats.totalSize + '  数据: ' + d.stats.data +
      '  速率: ' + d.stats.rate;
  }
  appendLog(line, d.result === 'failed' ? 'err' : '');
  if (d.stats) S.stats = d.stats;
  if (d.exitCode) appendLog('退出码: ' + d.exitCode, d.exitCode === 0 ? '' : 'err');
}

function renderStats() {
  const f = $('stats');
  const parts = [];
  if (S.job) parts.push('任务: <b>' + esc(S.job.finished || 0) + '/' + esc(S.job.total || 0) + '</b>');
  if (S.stats) {
    parts.push('文件: <b>' + esc(S.stats.filesTransferred) + '/' + esc(S.stats.filesTotal) + '</b>');
    parts.push('总大小: <b>' + esc(S.stats.totalSize) + '</b>');
    parts.push('数据: <b>' + esc(S.stats.data) + '</b>');
    parts.push('速率: <b>' + esc(S.stats.rate) + '</b>');
    parts.push('rsync: ' + esc(S.stats.rsync) + ' · 列表: ' + esc(S.stats.list));
  } else {
    parts.push('等待任务…');
  }
  f.innerHTML = parts.join(' &nbsp;|&nbsp; ');
}

function onJobState(d) {
  if (d.kind === 'dryrun' && d.state === 'done' && d.previewToken) {
    S.previewToken = d.previewToken;
    appendLog('预览成功，可以确认执行危险任务。');
    if (S.pendingConfirmAfterPreview) {
      S.pendingConfirmAfterPreview = false;
      const names = selectedNames();
      const danger = selectedDangerous();
      if (names.length && danger.length) openConfirm(names, danger);
    }
  }
  if (d.state === 'done' || d.state === 'canceled' || d.state === 'failed') {
    S.job = S.job && S.job.id === d.jobId ? Object.assign({}, S.job, { state: d.state }) : S.job;
    loadState().catch(handleError);
  } else if (S.job && S.job.id === d.jobId) {
    S.job.state = d.state;
    renderJob();
  }
}

function onJobSummary(d) {
  S.stats = {
    filesTransferred: d.files, filesTotal: d.filesTotal, totalSize: d.totalSize,
    data: d.data, rate: d.rate, rsync: d.rsync, list: d.list,
  };
  renderStats();
}

function connectSSE() {
  if (S.es) S.es.close();
  const es = new EventSource('/api/events');
  S.es = es;
  es.onopen = () => hideBanner();
  es.onerror = () => showBanner('与服务器的连接中断，正在自动重连…');
  es.addEventListener('log', (e) => onLog(JSON.parse(e.data)));
  es.addEventListener('task_start', (e) => onTaskStart(JSON.parse(e.data)));
  es.addEventListener('task_result', (e) => onTaskResult(JSON.parse(e.data)));
  es.addEventListener('job_state', (e) => onJobState(JSON.parse(e.data)));
  es.addEventListener('job_summary', (e) => onJobSummary(JSON.parse(e.data)));
  es.addEventListener('reset', () => loadState().catch(handleError));
}

/* ---------- 启动 ---------- */

applyTheme(currentTheme());
$('themeBtn').addEventListener('click', cycleTheme);
$('loginThemeBtn').addEventListener('click', cycleTheme);
fetchAuthInfo();
