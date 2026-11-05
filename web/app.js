'use strict';

/* rbackup-ui Web UI —— 原生 JS，无框架、无构建步骤。 */

/* ==================== i18n ==================== */

const I18N = {
  zh: {
    'app.security': '安全', 'app.phoneLogin': '手机登录', 'app.reload': '重载', 'app.logout': '退出',
    'login.user': '用户名', 'login.password': '密码', 'login.token': 'token',
    'login.totpHint': '二次验证：输入认证器 App 的 6 位动态码，或输入一个备份码。',
    'login.totpPlaceholder': '6 位动态码 / 备份码',
    'login.submit': '登录', 'login.enterBtn': '进入',
    'login.switchToToken': '改用 token 登录', 'login.switchToPassword': '改用用户名密码登录',
    'login.hint.loading': '正在获取登录方式…', 'login.hint.none': '本机模式：无需凭据，点击下方按钮进入。',
    'login.hint.password': '用户名密码登录。', 'login.hint.password2fa': '用户名密码登录，随后需要 6 位动态码或备份码。',
    'login.hint.token': '请输入访问 token。', 'login.hint.totp': '二次验证：输入认证器 App 中的 6 位动态码，或输入一个备份码。',
    'login.err.totpFormat': '请输入 6 位动态码或 10 位备份码',
    'lang.title': '切换语言', 'theme.title': '切换主题（自动/亮/暗）',
    'info.remote': '远端', 'info.policy': '门禁策略', 'info.script': '脚本', 'info.logDir': '日志目录',
    'tasks.title': '任务列表', 'tasks.selectAll': '全选', 'tasks.run': '运行选中', 'tasks.dryRun': 'Dry-run 预览',
    'tasks.mountCheck': '挂载检查', 'tasks.cancel': '取消运行',
    'tasks.colName': '任务名', 'tasks.colSrc': '源', 'tasks.colDst': '目标', 'tasks.colGate': '门禁', 'tasks.colDanger': '危险',
    'tasks.empty': '配置中没有任务，或配置加载失败。',
    'tasks.gate.require_mounted': '需已挂载', 'tasks.gate.require_unmounted': '需未挂载', 'tasks.gate.none': '—',
    'tasks.danger.delete': '删除', 'tasks.danger.removeSource': '删源', 'tasks.danger.generic': '危险',
    'log.title': '运行日志', 'log.pause': '暂停滚动', 'log.resume': '继续滚动', 'log.clear': '清屏',
    'job.idle': '空闲', 'job.running': '运行中', 'job.canceling': '取消中', 'job.done': '完成', 'job.canceled': '已取消', 'job.failed': '失败',
    'stats.waiting': '等待任务…', 'stats.tasks': '任务', 'stats.files': '文件', 'stats.totalSize': '总大小',
    'stats.data': '数据', 'stats.rate': '速率',
    'confirm.title': '确认执行', 'confirm.ok': '确认执行',
    'confirm.dangerIntro': '以下任务包含删除类操作，Dry-run 预览已成功：',
    'confirm.dangerNote': '确认后将立即执行，操作不可撤销。',
    'common.cancel': '取消', 'common.confirm': '确认', 'common.done': '完成', 'common.close': '关闭',
    'sec.title': '安全',
    'sec.status.off': '服务端未启用 2FA 策略（启动参数需加 --web-2fa totp）。',
    'sec.status.unconfigured': '2FA 策略已开启，但尚未绑定认证器。',
    'sec.status.enabled': '2FA 已启用 · 剩余备份码：{n} 个',
    'sec.setup': '设置 2FA', 'sec.regen': '重新生成备份码', 'sec.disable': '关闭 2FA',
    'sec.setupHint': '用手机 Authenticator App 扫描二维码，然后输入 App 里显示的 6 位动态码完成绑定。',
    'sec.manualSecret': '无法扫码？手动输入密钥：', 'sec.codePlaceholder': '6 位动态码', 'sec.confirmSetup': '确认绑定',
    'sec.verifyHint.regen': '输入当前动态码或一个备份码，确认后旧的备份码全部失效：',
    'sec.verifyHint.disable': '输入当前动态码或一个备份码，确认后关闭 2FA：',
    'sec.verifyPlaceholder': '动态码 / 备份码',
    'sec.backupOneTime': '新的备份码（只显示这一次，请立刻保存到安全的地方；每个只能用一次）：',
    'sec.copyCodes': '复制', 'sec.ok.setup': '2FA 已启用', 'sec.ok.regen': '备份码已重新生成', 'sec.ok.disable': '2FA 已关闭',
    'phone.title': '手机登录',
    'phone.hint': '用手机扫描二维码，或把链接发到手机打开（同一网络）。链接一次性使用，过期自动失效。',
    'phone.copy': '复制', 'phone.new': '新链接', 'phone.expires': '链接有效期：', 'phone.expired': '已过期', 'phone.copied': '链接已复制',
    'phone.err': '生成链接失败',
    'banner.configFail': '配置加载失败：', 'banner.sse': '与服务器的连接中断，正在自动重连…',
    'banner.readonly': '当前为只读模式', 'banner.pickTask': '请先选择任务',
    'banner.previewRequired': '危险任务的预览已失效，已自动重新执行 Dry-run 预览…',
    'banner.needPreview': '选中的任务包含危险操作，先执行 Dry-run 预览…',
    'banner.sessionExpired': '会话已失效，请重新登录', 'banner.fail': '操作失败',
    'banner.2faOff': '当前未启用两步验证（2FA）。启用后登录更安全，建议现在设置。',
    'banner.2faEnable': '立即启用', 'banner.2faLater': '稍后',
    'toast.copied': '已复制',
    'err.invalid_credentials': '用户名或密码不正确', 'err.invalid_token': 'token 不正确',
    'err.invalid_code': '动态码或备份码不正确', 'err.code_reused': '该动态码已被使用，请等待下一个',
    'err.pending_expired': '验证会话已过期，请重新登录', 'err.rate_limited': '尝试过于频繁，请稍后再试',
    'err.invalid_link': '链接无效、已过期或已被使用', 'err.csrf': '请求被拒绝（CSRF）',
    'err.2fa_disabled': '服务端未启用 2FA', 'err.already_configured': '2FA 已配置',
    'err.invalid_secret': 'secret 格式错误', 'err.too_many_links': '一次性链接过多，请稍后再试',
    'err.unauthorized': '未登录或会话已失效', 'err.network': '无法连接服务器', 'err.internal': '服务器内部错误',
  },
  en: {
    'app.security': 'Security', 'app.phoneLogin': 'Phone login', 'app.reload': 'Reload', 'app.logout': 'Logout',
    'login.user': 'Username', 'login.password': 'Password', 'login.token': 'Token',
    'login.totpHint': 'Two-factor: enter the 6-digit code from your authenticator app, or a backup code.',
    'login.totpPlaceholder': '6-digit code / backup code',
    'login.submit': 'Sign in', 'login.enterBtn': 'Enter',
    'login.switchToToken': 'Use token instead', 'login.switchToPassword': 'Use username/password instead',
    'login.hint.loading': 'Detecting sign-in methods…', 'login.hint.none': 'Local mode: no credentials needed. Click below to enter.',
    'login.hint.password': 'Sign in with username and password.',
    'login.hint.password2fa': 'Sign in with username/password, then a 6-digit code or backup code.',
    'login.hint.token': 'Enter the access token.', 'login.hint.totp': 'Two-factor: enter the 6-digit code from your authenticator app, or a backup code.',
    'login.err.totpFormat': 'Enter a 6-digit code or a 10-character backup code',
    'lang.title': 'Switch language', 'theme.title': 'Switch theme (auto/light/dark)',
    'info.remote': 'Remote', 'info.policy': 'Mount policy', 'info.script': 'Script', 'info.logDir': 'Log dir',
    'tasks.title': 'Tasks', 'tasks.selectAll': 'Select all', 'tasks.run': 'Run selected', 'tasks.dryRun': 'Dry-run preview',
    'tasks.mountCheck': 'Mount check', 'tasks.cancel': 'Cancel run',
    'tasks.colName': 'Task', 'tasks.colSrc': 'Source', 'tasks.colDst': 'Destination', 'tasks.colGate': 'Gate', 'tasks.colDanger': 'Danger',
    'tasks.empty': 'No tasks found, or the config failed to load.',
    'tasks.gate.require_mounted': 'must be mounted', 'tasks.gate.require_unmounted': 'must be unmounted', 'tasks.gate.none': '—',
    'tasks.danger.delete': 'delete', 'tasks.danger.removeSource': 'remove source', 'tasks.danger.generic': 'danger',
    'log.title': 'Run log', 'log.pause': 'Pause scroll', 'log.resume': 'Resume scroll', 'log.clear': 'Clear',
    'job.idle': 'Idle', 'job.running': 'Running', 'job.canceling': 'Canceling', 'job.done': 'Done', 'job.canceled': 'Canceled', 'job.failed': 'Failed',
    'stats.waiting': 'Waiting for tasks…', 'stats.tasks': 'Tasks', 'stats.files': 'Files', 'stats.totalSize': 'Total size',
    'stats.data': 'Data', 'stats.rate': 'Rate',
    'confirm.title': 'Confirm run', 'confirm.ok': 'Run now',
    'confirm.dangerIntro': 'Selected tasks contain destructive operations. Dry-run preview succeeded:',
    'confirm.dangerNote': 'They will run immediately and cannot be undone.',
    'common.cancel': 'Cancel', 'common.confirm': 'Confirm', 'common.done': 'Done', 'common.close': 'Close',
    'sec.title': 'Security',
    'sec.status.off': '2FA policy is disabled on the server (start with --web-2fa totp).',
    'sec.status.unconfigured': '2FA policy is enabled, but no authenticator is bound yet.',
    'sec.status.enabled': '2FA enabled · backup codes left: {n}',
    'sec.setup': 'Set up 2FA', 'sec.regen': 'Regenerate backup codes', 'sec.disable': 'Disable 2FA',
    'sec.setupHint': 'Scan the QR code with your authenticator app, then enter the 6-digit code to finish binding.',
    'sec.manualSecret': 'Cannot scan? Enter the secret manually:', 'sec.codePlaceholder': '6-digit code', 'sec.confirmSetup': 'Confirm',
    'sec.verifyHint.regen': 'Enter the current code or a backup code. All old backup codes will be invalidated:',
    'sec.verifyHint.disable': 'Enter the current code or a backup code to disable 2FA:',
    'sec.verifyPlaceholder': 'Code / backup code',
    'sec.backupOneTime': 'New backup codes (shown only once — save them somewhere safe; each can be used once):',
    'sec.copyCodes': 'Copy', 'sec.ok.setup': '2FA enabled', 'sec.ok.regen': 'Backup codes regenerated', 'sec.ok.disable': '2FA disabled',
    'phone.title': 'Phone login',
    'phone.hint': 'Scan the QR code with your phone, or open the link on a device on the same network. The link is single-use and expires.',
    'phone.copy': 'Copy', 'phone.new': 'New link', 'phone.expires': 'Link expires in:', 'phone.expired': 'expired', 'phone.copied': 'Link copied',
    'phone.err': 'Failed to create link',
    'banner.configFail': 'Config failed to load: ', 'banner.sse': 'Connection to server lost, reconnecting…',
    'banner.readonly': 'Read-only mode', 'banner.pickTask': 'Select at least one task',
    'banner.previewRequired': 'Preview expired; re-running Dry-run preview…',
    'banner.needPreview': 'Selection contains destructive operations; running Dry-run preview first…',
    'banner.sessionExpired': 'Session expired, please sign in again', 'banner.fail': 'Operation failed',
    'banner.2faOff': 'Two-factor authentication (2FA) is not enabled. Enable it now for better security.',
    'banner.2faEnable': 'Enable now', 'banner.2faLater': 'Later',
    'toast.copied': 'Copied',
    'err.invalid_credentials': 'Incorrect username or password', 'err.invalid_token': 'Invalid token',
    'err.invalid_code': 'Invalid code or backup code', 'err.code_reused': 'This code was already used; wait for the next one',
    'err.pending_expired': 'Verification session expired; sign in again', 'err.rate_limited': 'Too many attempts, try again later',
    'err.invalid_link': 'Link is invalid, expired, or already used', 'err.csrf': 'Request rejected (CSRF)',
    'err.2fa_disabled': '2FA is not enabled on the server', 'err.already_configured': '2FA is already configured',
    'err.invalid_secret': 'Invalid secret format', 'err.too_many_links': 'Too many one-time links; try again later',
    'err.unauthorized': 'Not signed in or session expired', 'err.network': 'Cannot reach the server', 'err.internal': 'Internal server error',
  },
};

const LANG_KEY = 'rbtui-lang';

function currentLang() {
  try {
    const v = localStorage.getItem(LANG_KEY);
    if (v === 'zh' || v === 'en') return v;
  } catch (e) { /* ignore */ }
  return (navigator.language || '').toLowerCase().startsWith('zh') ? 'zh' : 'en';
}

function t(key, params) {
  const dict = I18N[S.lang] || I18N.zh;
  let s = dict[key];
  if (s === undefined) s = (I18N.zh[key] !== undefined ? I18N.zh[key] : key);
  if (params) {
    for (const k of Object.keys(params)) s = s.split('{' + k + '}').join(String(params[k]));
  }
  return s;
}

function applyI18n() {
  document.documentElement.lang = S.lang === 'zh' ? 'zh-CN' : 'en';
  document.querySelectorAll('[data-i18n]').forEach((n) => { n.textContent = t(n.getAttribute('data-i18n')); });
  document.querySelectorAll('[data-i18n-ph]').forEach((n) => { n.placeholder = t(n.getAttribute('data-i18n-ph')); });
  document.querySelectorAll('[data-i18n-title]').forEach((n) => { n.title = t(n.getAttribute('data-i18n-title')); });
  for (const id of ['langBtn', 'loginLangBtn']) {
    const b = $(id);
    if (b) b.textContent = S.lang === 'zh' ? 'EN' : '中';
  }
  for (const b of [$('themeBtn'), $('loginThemeBtn')]) {
    if (b) b.title = t('theme.title') + ' · ' + THEME_NAME[currentTheme()];
  }
}

function setLang(lang) {
  S.lang = lang;
  try { localStorage.setItem(LANG_KEY, lang); } catch (e) { /* ignore */ }
  applyI18n();
  refreshViewTexts();
}

function toggleLang() { setLang(S.lang === 'zh' ? 'en' : 'zh'); }

/* ==================== 状态 ==================== */

const S = {
  csrf: '', version: '', readOnly: false, tokenFree: false, lang: 'zh',
  config: {}, tasks: [], selected: new Set(), job: null, lastJob: null,
  previewToken: '', pendingConfirmAfterPreview: false, es: null, paused: false, stats: null,
  auth: { methods: [], totp: false, readOnly: false, version: '' },
  hasToken: false, hasPassword: false, mode: 'token', step: 'login',
  twofa: null, setup: null, verifyAction: '', backupCodes: [],
  phone: null, phoneTimer: null, enable2faDismissed: false,
};

const $ = (id) => document.getElementById(id);
const el = (tag, cls) => { const n = document.createElement(tag); if (cls) n.className = cls; return n; };
const esc = (s) => String(s == null ? '' : s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const show = (id) => $(id).classList.remove('hidden');
const hide = (id) => $(id).classList.add('hidden');

/* ==================== 主题 ==================== */

const THEME_KEY = 'rbtui-theme';
const THEME_ICON = { auto: '🌓', light: '☀️', dark: '🌙' };
const THEME_NAME = { auto: 'Auto', light: 'Light', dark: 'Dark' };
const THEME_NAME_ZH = { auto: '自动', light: '亮色', dark: '暗色' };
const THEME_ORDER = ['auto', 'light', 'dark'];

function currentTheme() {
  try { return localStorage.getItem(THEME_KEY) || 'auto'; } catch (e) { return 'auto'; }
}

function applyTheme(mode) {
  const root = document.documentElement;
  if (mode === 'light' || mode === 'dark') root.dataset.theme = mode;
  else delete root.dataset.theme;
  try { localStorage.setItem(THEME_KEY, mode); } catch (e) { /* ignore */ }
  const name = S.lang === 'zh' ? THEME_NAME_ZH[mode] : THEME_NAME[mode];
  for (const btn of [$('themeBtn'), $('loginThemeBtn')]) {
    if (btn) { btn.textContent = THEME_ICON[mode]; btn.title = t('theme.title') + ' · ' + name; }
  }
}

function cycleTheme() { applyTheme(THEME_ORDER[(THEME_ORDER.indexOf(currentTheme()) + 1) % THEME_ORDER.length]); }

/* ==================== API ==================== */

async function api(method, path, body) {
  const headers = {};
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (S.csrf && method !== 'GET') headers['X-RB-CSRF'] = S.csrf;
  let res;
  try {
    res = await fetch(path, { method, headers, credentials: 'same-origin', body: body !== undefined ? JSON.stringify(body) : undefined });
  } catch (e) {
    const err = new Error(t('err.network')); err.code = 'network'; throw err;
  }
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

function errText(e) {
  const d = I18N[S.lang] || I18N.zh;
  if (e && e.code && d['err.' + e.code]) return d['err.' + e.code];
  return (e && e.message) || t('banner.fail');
}

/* ==================== 通用 UI ==================== */

let toastTimer = null;
function showToast(msg) {
  const n = $('toast');
  n.textContent = msg;
  n.classList.remove('hidden');
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => n.classList.add('hidden'), 2400);
}

function showBanner(msg) { const b = $('banner'); b.textContent = msg; b.classList.remove('hidden'); }
function hideBanner() { $('banner').classList.add('hidden'); }

function showLoginError(msg) {
  const n = $('loginError');
  n.textContent = msg || '';
  n.classList.toggle('hidden', !msg);
}

function showLogin() { show('loginView'); hide('appView'); }
function showApp() { hide('loginView'); show('appView'); }

function handleError(e) {
  if (e && e.status === 401) {
    showLogin();
    showLoginError(t('banner.sessionExpired'));
    if (S.es) { S.es.close(); S.es = null; }
    return;
  }
  showBanner(errText(e));
}

function refreshViewTexts() {
  if (!$('appView').classList.contains('hidden')) renderAll();
  else renderLoginForm();
  if (!$('securityModal').classList.contains('hidden')) render2FA();
  if (!$('phoneModal').classList.contains('hidden')) updatePhoneCountdown();
}

/* ==================== 登录 ==================== */

async function fetchAuthInfo() {
  try {
    S.auth = await api('GET', '/api/authinfo');
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
  hide('totpFields');
  $('loginBtn').textContent = tokenFree ? t('login.enterBtn') : t('login.submit');
  const sw = $('switchAuthBtn');
  sw.classList.toggle('hidden', !(S.hasPassword && S.hasToken));
  sw.textContent = S.mode === 'password' ? t('login.switchToToken') : t('login.switchToPassword');

  let hint = t('login.hint.loading');
  if (tokenFree) hint = t('login.hint.none');
  else if (S.mode === 'password') hint = S.auth.totp ? t('login.hint.password2fa') : t('login.hint.password');
  else hint = t('login.hint.token');
  $('loginHint').textContent = hint;
  $('loginFoot').textContent = '';
  S.step = 'login';
  showLoginError('');
}

function showTotpStep() {
  S.step = 'totp';
  hide('passwordFields'); hide('tokenFields'); show('totpFields'); hide('switchAuthBtn');
  $('loginHint').textContent = t('login.hint.totp');
  $('loginBtn').textContent = t('login.submit');
  showLoginError('');
  const inp = $('totpInput');
  inp.value = '';
  inp.focus();
}

function normalizeBackupInput(v) { return String(v || '').toUpperCase().replace(/[\s-]/g, ''); }

function enterApp(r) {
  S.csrf = r.csrf || S.csrf;
  S.version = r.version || S.version;
  S.readOnly = !!r.readOnly;
  S.tokenFree = !!r.tokenFree;
  showApp();
  loadState().then(() => { check2FAState(); connectSSE(); }).catch(handleError);
}

$('loginForm').addEventListener('submit', async (ev) => {
  ev.preventDefault();
  showLoginError('');
  $('loginBtn').disabled = true;
  try {
    if (S.step === 'totp') {
      const raw = ($('totpInput').value || '').trim();
      const isTotp = /^[0-9]{6}$/.test(raw);
      const isBackup = /^[A-Z0-9]{10}$/.test(normalizeBackupInput(raw));
      if (!isTotp && !isBackup) throw Object.assign(new Error(t('login.err.totpFormat')), { code: 'local_totp_format' });
      enterApp(await api('POST', '/api/session/totp', { code: raw }));
      return;
    }
    if (S.mode === 'none') {
      enterApp(await api('POST', '/api/session', {}));
    } else if (S.mode === 'password') {
      const r = await api('POST', '/api/session', {
        username: $('userInput').value.trim(),
        password: $('passInput').value,
      });
      if (r.totpRequired) { showTotpStep(); return; }
      enterApp(r);
    } else {
      enterApp(await api('POST', '/api/session', { token: $('tokenInput').value.trim() }));
    }
  } catch (e) {
    if (e.code === 'local_totp_format') { showLoginError(e.message); return; }
    showLoginError(errText(e));
    if (e.code === 'pending_expired') renderLoginForm();
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

/* 手机扫码登录：URL 形如 /#t=<一次性 token> */
async function maybePhoneLogin() {
  const m = /[#&]t=([A-Za-z0-9_-]+)/.exec(location.hash || '');
  if (!m) return false;
  const token = m[1];
  try { history.replaceState(null, '', location.pathname + location.search); } catch (e) { /* ignore */ }
  try {
    const r = await api('POST', '/api/session/phone', { token });
    enterApp(r);
    return true;
  } catch (e) {
    showLogin();
    showLoginError(errText(e));
    return false;
  }
}

/* ==================== 2FA 提示（未启用时提示开启） ==================== */

async function check2FAState() {
  try {
    S.twofa = await api('GET', '/api/2fa');
  } catch (e) {
    hide('enable2faBanner');
    return;
  }
  const st = S.twofa || {};
  if (st.policy && !st.configured && !S.enable2faDismissed) {
    $('enable2faText').textContent = t('banner.2faOff');
    $('enable2faBtn').textContent = t('banner.2faEnable');
    $('enable2faLater').textContent = t('banner.2faLater');
    show('enable2faBanner');
  } else {
    hide('enable2faBanner');
  }
}

$('enable2faBtn').addEventListener('click', async () => {
  hide('enable2faBanner');
  try { await openSecurity(); await start2FASetup(); } catch (e) { /* ignore */ }
});
$('enable2faLater').addEventListener('click', () => {
  S.enable2faDismissed = true;
  hide('enable2faBanner');
});

/* ==================== 状态渲染 ==================== */

async function loadState() {
  const s = await api('GET', '/api/state');
  S.version = s.version || S.version;
  S.readOnly = !!s.readOnly;
  if (s.csrf) S.csrf = s.csrf;
  S.config = s.config || {};
  S.tasks = s.tasks || [];
  S.job = s.job || null;
  S.lastJob = s.lastJob || null;
  const known = new Set(S.tasks.map((t2) => t2.name));
  S.selected = new Set([...S.selected].filter((n) => known.has(n)));
  if (s.lastJob && s.lastJob.kind === 'dryrun' && s.lastJob.state === 'done' && s.lastJob.previewToken) {
    S.previewToken = s.lastJob.previewToken;
  }
  renderAll();
  if (s.configError) showBanner(t('banner.configFail') + s.configError); else hideBanner();
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
  const mode = S.readOnly ? ' · ' + t('banner.readonly') : '';
  $('meta').textContent = c.path ? (c.path + mode) : (S.readOnly ? t('banner.readonly') : '');
}

function renderInfo() {
  const c = S.config || {};
  const items = [
    [t('info.remote'), (c.sshUser || '') + '@' + (c.host || '') + (c.sshPort ? ':' + c.sshPort : '')],
    [t('info.policy'), c.policy || '-'],
    [t('info.script'), c.script || '-'],
    [t('info.logDir'), c.logDir || '-'],
  ];
  const wrap = $('info');
  wrap.innerHTML = '';
  for (const [k, v] of items) {
    const n = el('span');
    n.innerHTML = '<b>' + esc(k) + '</b> ' + esc(v);
    wrap.appendChild(n);
  }
}

function gateText(task) {
  if (task.gate === 'require_mounted') return t('tasks.gate.require_mounted');
  if (task.gate === 'require_unmounted') return t('tasks.gate.require_unmounted');
  return t('tasks.gate.none');
}

function dangerText(task) {
  if (!task.dangerous) return '';
  const parts = [];
  if (task.delete === 'yes') parts.push(t('tasks.danger.delete'));
  if (task.removeSource === 'yes') parts.push(t('tasks.danger.removeSource'));
  return parts.length ? parts.join('+') : t('tasks.danger.generic');
}

function isBusy() { return !!(S.job && (S.job.state === 'running' || S.job.state === 'canceling')); }

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
  const body = $('taskBody'); body.innerHTML = '';
  const cards = $('taskCards'); cards.innerHTML = '';
  $('emptyHint').classList.toggle('hidden', S.tasks.length > 0);
  const disabled = isBusy() || S.readOnly;

  for (const task of S.tasks) {
    const tr = el('tr');
    const td1 = el('td', 'c-chk');
    const cb = el('input');
    cb.type = 'checkbox'; cb.checked = S.selected.has(task.name); cb.disabled = disabled;
    cb.addEventListener('change', () => toggleTask(task.name, cb.checked));
    td1.appendChild(cb); tr.appendChild(td1);
    for (const v of [task.name, task.src || '', task.dst || '', gateText(task)]) {
      const td = el('td');
      td.textContent = v; td.title = v;
      tr.appendChild(td);
    }
    const tdDanger = el('td', 'c-danger');
    if (task.dangerous) { const b = el('span', 'badge failed'); b.textContent = dangerText(task); tdDanger.appendChild(b); }
    else tdDanger.textContent = t('tasks.gate.none');
    tr.appendChild(tdDanger);
    body.appendChild(tr);

    const card = el('div', 'task-card');
    const ccb = el('input');
    ccb.type = 'checkbox'; ccb.checked = S.selected.has(task.name); ccb.disabled = disabled;
    ccb.addEventListener('change', () => toggleTask(task.name, ccb.checked));
    const main = el('div', 'tc-main');
    const name = el('div', 'tc-name'); name.textContent = task.name; main.appendChild(name);
    const src = el('div', 'tc-line'); src.textContent = t('tasks.colSrc') + ': ' + (task.src || '-'); main.appendChild(src);
    const dst = el('div', 'tc-line'); dst.textContent = t('tasks.colDst') + ': ' + (task.dst || '-'); main.appendChild(dst);
    const badges = el('div', 'badge-row');
    const gb = el('span', 'badge'); gb.textContent = gateText(task); badges.appendChild(gb);
    if (task.dangerous) { const db = el('span', 'badge failed'); db.textContent = dangerText(task); badges.appendChild(db); }
    main.appendChild(badges);
    card.appendChild(ccb); card.appendChild(main);
    card.addEventListener('click', (ev) => {
      if (ev.target === ccb) return;
      ccb.checked = !ccb.checked;
      toggleTask(task.name, ccb.checked);
    });
    cards.appendChild(card);
  }

  const all = $('selectAll');
  all.disabled = disabled;
  all.checked = S.tasks.length > 0 && S.selected.size === S.tasks.length;
  renderButtons();
}

function renderButtons() {
  const disabled = isBusy() || S.readOnly;
  for (const id of ['runBtn', 'dryBtn', 'mountBtn', 'reloadBtn']) $(id).disabled = disabled;
  $('selectAll').disabled = disabled;
  $('cancelBtn').classList.toggle('hidden', !isBusy() || S.readOnly);
  for (const cb of document.querySelectorAll('#taskBody input[type=checkbox], #taskCards input[type=checkbox]')) cb.disabled = disabled;
}

function renderJob() {
  const b = $('jobState');
  const j = S.job;
  b.className = 'badge';
  if (!j) { b.textContent = t('job.idle'); b.classList.add('idle'); }
  else {
    b.textContent = t('job.' + j.state) || j.state;
    b.classList.add(j.state === 'failed' ? 'failed' : (j.state === 'done' ? 'done' : (isBusy() ? 'running' : 'idle')));
    if (isBusy() && j.currentTask) b.textContent += ' · ' + j.currentTask;
  }
  renderButtons();
}

/* ==================== 操作 ==================== */

$('selectAll').addEventListener('change', (ev) => {
  S.selected = ev.target.checked ? new Set(S.tasks.map((t2) => t2.name)) : new Set();
  renderTasks();
});

function selectedNames() { return S.tasks.filter((t2) => S.selected.has(t2.name)).map((t2) => t2.name); }
function selectedDangerous() { return S.tasks.filter((t2) => S.selected.has(t2.name) && t2.dangerous); }

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
      showBanner(t('banner.previewRequired'));
      await startJob({ tasks: selectedNames(), dryRun: true });
      return;
    }
    handleError(e);
  }
}

async function runSelected(dryRun) {
  const names = selectedNames();
  if (!names.length) { showBanner(t('banner.pickTask')); return; }
  if (S.readOnly) { showBanner(t('banner.readonly')); return; }
  const danger = selectedDangerous();
  if (!dryRun && danger.length) {
    if (!S.previewToken) {
      S.pendingConfirmAfterPreview = true;
      showBanner(t('banner.needPreview'));
      await startJob({ tasks: names, dryRun: true });
      return;
    }
    openConfirm(names, danger);
    return;
  }
  await startJob({ tasks: names, dryRun: !!dryRun });
}

function openConfirm(names, danger) {
  $('modalTitle').textContent = t('confirm.title');
  const body = $('modalBody');
  body.innerHTML = '';
  const p1 = el('p'); p1.textContent = t('confirm.dangerIntro'); body.appendChild(p1);
  const ul = el('ul', 'modal-list');
  for (const task of danger) {
    const li = el('li');
    const flags = [];
    if (task.delete === 'yes') flags.push('--delete');
    if (task.removeSource === 'yes') flags.push('--remove-source-files');
    li.textContent = task.name + ': ' + flags.join(', ');
    ul.appendChild(li);
  }
  body.appendChild(ul);
  const p2 = el('p', 'hint small'); p2.textContent = t('confirm.dangerNote'); body.appendChild(p2);
  show('modal');
  $('modalOk').onclick = async () => {
    hide('modal');
    await startJob({ tasks: names, dryRun: false, confirm: true, previewToken: S.previewToken });
  };
}

$('modalCancel').addEventListener('click', () => hide('modal'));
$('modal').addEventListener('click', (ev) => { if (ev.target === $('modal')) hide('modal'); });

$('runBtn').addEventListener('click', () => runSelected(false));
$('dryBtn').addEventListener('click', () => runSelected(true));
$('mountBtn').addEventListener('click', async () => {
  if (S.readOnly) return;
  try { S.job = (await api('POST', '/api/mountcheck', {})).job; renderJob(); } catch (e) { handleError(e); }
});
$('cancelBtn').addEventListener('click', async () => {
  if (!confirm(t('tasks.cancel') + '?')) return;
  try { S.job = (await api('POST', '/api/jobs/current/cancel', {})).job; renderJob(); } catch (e) { handleError(e); }
});
$('reloadBtn').addEventListener('click', () => loadState().catch(handleError));
$('pauseBtn').addEventListener('click', () => {
  S.paused = !S.paused;
  $('pauseBtn').textContent = S.paused ? t('log.resume') : t('log.pause');
});
$('clearBtn').addEventListener('click', () => { $('log').textContent = ''; });

/* ==================== 日志与 SSE ==================== */

function appendLog(text, cls) {
  const log = $('log');
  const span = el('span', cls || '');
  span.textContent = text + '\n';
  log.appendChild(span);
  while (log.childNodes.length > 3000) log.removeChild(log.firstChild);
  if (!S.paused) log.scrollTop = log.scrollHeight;
}

function appendSep(text) { appendLog('─'.repeat(12) + ' ' + text + ' ' + '─'.repeat(12), 'sep'); }

function onLog(d) {
  const prefix = d.task && d.task !== '__mountcheck__' ? '[' + d.task + '] ' : '';
  appendLog(prefix + (d.line || ''));
}
function onTaskStart(d) { appendSep(d.task + ' (' + d.index + '/' + d.total + (d.dryRun ? ', dry-run' : '') + ')'); }
function onTaskResult(d) {
  let line = '>>> ' + d.task + ' ' + d.result + '  ' + (d.duration || '-');
  if (d.stats) {
    line += '  ' + t('stats.files') + ': ' + d.stats.filesTransferred + '/' + d.stats.filesTotal +
      '  ' + t('stats.totalSize') + ': ' + d.stats.totalSize +
      '  ' + t('stats.data') + ': ' + d.stats.data +
      '  ' + t('stats.rate') + ': ' + d.stats.rate;
  }
  appendLog(line, d.result === 'failed' ? 'err' : '');
  if (d.stats) S.stats = d.stats;
  if (d.exitCode) appendLog('exit: ' + d.exitCode, d.exitCode === 0 ? '' : 'err');
}

function renderStats() {
  const f = $('stats');
  const parts = [];
  if (S.job) parts.push(t('stats.tasks') + ': <b>' + esc(S.job.finished || 0) + '/' + esc(S.job.total || 0) + '</b>');
  if (S.stats) {
    parts.push(t('stats.files') + ': <b>' + esc(S.stats.filesTransferred) + '/' + esc(S.stats.filesTotal) + '</b>');
    parts.push(t('stats.totalSize') + ': <b>' + esc(S.stats.totalSize) + '</b>');
    parts.push(t('stats.data') + ': <b>' + esc(S.stats.data) + '</b>');
    parts.push(t('stats.rate') + ': <b>' + esc(S.stats.rate) + '</b>');
  } else {
    parts.push(t('stats.waiting'));
  }
  f.innerHTML = parts.join(' &nbsp;|&nbsp; ');
}

function onJobState(d) {
  if (d.kind === 'dryrun' && d.state === 'done' && d.previewToken) {
    S.previewToken = d.previewToken;
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
    data: d.data, rate: d.rate,
  };
  renderStats();
}

function connectSSE() {
  if (S.es) S.es.close();
  const es = new EventSource('/api/events');
  S.es = es;
  es.onopen = () => hideBanner();
  es.onerror = () => showBanner(t('banner.sse'));
  es.addEventListener('log', (e) => onLog(JSON.parse(e.data)));
  es.addEventListener('task_start', (e) => onTaskStart(JSON.parse(e.data)));
  es.addEventListener('task_result', (e) => onTaskResult(JSON.parse(e.data)));
  es.addEventListener('job_state', (e) => onJobState(JSON.parse(e.data)));
  es.addEventListener('job_summary', (e) => onJobSummary(JSON.parse(e.data)));
  es.addEventListener('reset', () => loadState().catch(handleError));
}

/* ==================== 安全中心（2FA 自助管理） ==================== */

function showSecError(msg) {
  const n = $('secError');
  n.textContent = msg || '';
  n.classList.toggle('hidden', !msg);
}

function hideSecPanels() {
  hide('secSetup'); hide('secVerify'); hide('secBackupResult');
  showSecError('');
}

function render2FA() {
  hideSecPanels();
  const st = S.twofa || {};
  const configured = !!st.configured;
  $('secSetupBtn').classList.toggle('hidden', !st.policy || configured);
  $('secRegenBtn').classList.toggle('hidden', !configured);
  $('secDisableBtn').classList.toggle('hidden', !configured);
  if (!st.policy) $('secStatus').textContent = t('sec.status.off');
  else if (!configured) $('secStatus').textContent = t('sec.status.unconfigured');
  else $('secStatus').textContent = t('sec.status.enabled', { n: st.backupCodes });
}

async function refresh2FA() {
  S.twofa = await api('GET', '/api/2fa');
  render2FA();
  check2FAState();
}

async function openSecurity() {
  show('securityModal');
  hideSecPanels();
  try { await refresh2FA(); } catch (e) { showSecError(errText(e)); }
}

function showBackupCodes(codes) {
  S.backupCodes = codes || [];
  const list = $('secBackupList');
  list.innerHTML = '';
  for (const c of S.backupCodes) { const li = el('li'); li.textContent = c; list.appendChild(li); }
  show('secBackupResult');
}

async function start2FASetup() {
  showSecError('');
  try {
    const r = await api('POST', '/api/2fa/setup', {});
    S.setup = r;
    $('secQr').src = '/api/qr.png?data=' + encodeURIComponent(r.uri);
    $('secSecret').textContent = (r.secret.match(/.{1,4}/g) || [r.secret]).join(' ');
    $('secCode').value = '';
    hideSecPanels();
    show('secSetup');
    $('secCode').focus();
  } catch (e) { showSecError(errText(e)); }
}

async function confirm2FASetup() {
  const code = ($('secCode').value || '').trim();
  if (!/^[0-9]{6}$/.test(code)) { showSecError(t('sec.codePlaceholder')); return; }
  showSecError('');
  try {
    const r = await api('POST', '/api/2fa/setup/confirm', { secret: S.setup.secret, code });
    hide('secSetup');
    showBackupCodes(r.backupCodes);
    showToast(t('sec.ok.setup'));
    S.enable2faDismissed = true;
    S.twofa = await api('GET', '/api/2fa');
    render2FA();
    showBackupCodes(r.backupCodes);
    check2FAState();
  } catch (e) { showSecError(errText(e)); }
}

function askVerify(action) {
  S.verifyAction = action;
  hideSecPanels();
  $('secVerifyHint').textContent = action === 'regen' ? t('sec.verifyHint.regen') : t('sec.verifyHint.disable');
  $('secVerifyCode').value = '';
  show('secVerify');
  $('secVerifyCode').focus();
}

async function confirmVerify() {
  const code = ($('secVerifyCode').value || '').trim();
  if (!code) { showSecError(t('sec.verifyPlaceholder')); return; }
  showSecError('');
  try {
    if (S.verifyAction === 'regen') {
      const r = await api('POST', '/api/2fa/backup-codes', { code });
      hideSecPanels();
      showBackupCodes(r.backupCodes);
      showToast(t('sec.ok.regen'));
    } else {
      await api('POST', '/api/2fa/disable', { code });
      hideSecPanels();
      showToast(t('sec.ok.disable'));
      S.enable2faDismissed = true;
    }
    S.twofa = await api('GET', '/api/2fa');
    render2FA();
    if (S.verifyAction === 'regen') showBackupCodes(S.backupCodes);
    check2FAState();
  } catch (e) { showSecError(errText(e)); }
}

$('secSetupBtn').addEventListener('click', start2FASetup);
$('secSetupCancel').addEventListener('click', hideSecPanels);
$('secSetupConfirm').addEventListener('click', confirm2FASetup);
$('secRegenBtn').addEventListener('click', () => askVerify('regen'));
$('secDisableBtn').addEventListener('click', () => askVerify('disable'));
$('secVerifyCancel').addEventListener('click', hideSecPanels);
$('secVerifyOk').addEventListener('click', confirmVerify);
$('secBackupCopy').addEventListener('click', async () => {
  try { await navigator.clipboard.writeText(S.backupCodes.join('\n')); showToast(t('toast.copied')); }
  catch (e) { showToast(t('banner.fail')); }
});
$('secBackupDone').addEventListener('click', async () => {
  hide('secBackupResult');
  try { S.twofa = await api('GET', '/api/2fa'); render2FA(); } catch (e) { /* ignore */ }
});
$('secClose').addEventListener('click', () => hide('securityModal'));
$('securityModal').addEventListener('click', (ev) => { if (ev.target === $('securityModal')) hide('securityModal'); });
$('securityBtn').addEventListener('click', openSecurity);

/* ==================== 手机扫码登录 ==================== */

function clearPhoneTimer() {
  if (S.phoneTimer) { clearInterval(S.phoneTimer); S.phoneTimer = null; }
}

function updatePhoneCountdown() {
  if (!S.phone) return;
  const left = Math.max(0, Math.ceil((S.phone.until - Date.now()) / 1000));
  $('phoneExpiry').textContent = left > 0 ? (left + 's') : t('phone.expired');
  if (left <= 0) clearPhoneTimer();
}

function showSecPhoneError(msg) {
  const n = $('phoneError');
  n.textContent = msg || '';
  n.classList.toggle('hidden', !msg);
}

async function newPhoneLink() {
  clearPhoneTimer();
  showSecPhoneError('');
  try {
    const r = await api('POST', '/api/phone-login', {});
    S.phone = { url: r.url, until: Date.now() + (r.expiresIn || 120) * 1000 };
    $('phoneQr').src = '/api/qr.png?data=' + encodeURIComponent(r.url);
    $('phoneUrl').textContent = r.url;
    updatePhoneCountdown();
    S.phoneTimer = setInterval(updatePhoneCountdown, 1000);
  } catch (e) {
    showSecPhoneError(errText(e));
  }
}

async function openPhone() { show('phoneModal'); await newPhoneLink(); }

$('phoneBtn').addEventListener('click', openPhone);
$('phoneNew').addEventListener('click', newPhoneLink);
$('phoneCopy').addEventListener('click', async () => {
  if (!S.phone) return;
  try { await navigator.clipboard.writeText(S.phone.url); showToast(t('phone.copied')); }
  catch (e) { showToast(t('banner.fail')); }
});
$('phoneClose').addEventListener('click', () => { clearPhoneTimer(); hide('phoneModal'); });
$('phoneModal').addEventListener('click', (ev) => { if (ev.target === $('phoneModal')) { clearPhoneTimer(); hide('phoneModal'); } });

/* ==================== 启动 ==================== */

(async function init() {
  S.lang = currentLang();
  applyI18n();
  applyTheme(currentTheme());
  $('themeBtn').addEventListener('click', cycleTheme);
  $('loginThemeBtn').addEventListener('click', cycleTheme);
  $('langBtn').addEventListener('click', toggleLang);
  $('loginLangBtn').addEventListener('click', toggleLang);

  const fromPhone = await maybePhoneLogin();
  if (!fromPhone) await fetchAuthInfo();
})();
