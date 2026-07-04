const I18N_STORAGE_KEY = 'panel_lang';
const I18N_DEFAULT = 'ru';

let _locale = I18N_DEFAULT;
let _strings = {};

async function initI18n() {
  _locale = localStorage.getItem(I18N_STORAGE_KEY) || I18N_DEFAULT;
  if (_locale !== 'ru' && _locale !== 'en') _locale = I18N_DEFAULT;
  const resp = await fetch(`/static/locales/${_locale}.json`);
  if (!resp.ok) throw new Error('Failed to load locale');
  _strings = await resp.json();
  document.documentElement.lang = _locale;
  applyI18n();
  return _locale;
}

function getLang() {
  return _locale;
}

function setLang(lang) {
  if (lang !== 'ru' && lang !== 'en') return;
  localStorage.setItem(I18N_STORAGE_KEY, lang);
  location.reload();
}

function t(key, params) {
  const parts = key.split('.');
  let val = _strings;
  for (const p of parts) {
    if (val == null || typeof val !== 'object') {
      val = undefined;
      break;
    }
    val = val[p];
  }
  if (typeof val !== 'string') return key;
  if (!params) return val;
  return val.replace(/\{(\w+)\}/g, (_, k) => (params[k] != null ? String(params[k]) : `{${k}}`));
}

function applyI18n(root) {
  const scope = root || document;
  scope.querySelectorAll('[data-i18n]').forEach(el => {
    const key = el.getAttribute('data-i18n');
    if (key) el.textContent = t(key);
  });
  scope.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
    const key = el.getAttribute('data-i18n-placeholder');
    if (key) el.placeholder = t(key);
  });
  scope.querySelectorAll('[data-i18n-title]').forEach(el => {
    const key = el.getAttribute('data-i18n-title');
    if (key) el.title = t(key);
  });
  scope.querySelectorAll('title[data-i18n]').forEach(el => {
    const key = el.getAttribute('data-i18n');
    if (key) el.textContent = t(key);
  });
}

function translateApiMessage(msg) {
  if (!msg || typeof msg !== 'string') return msg;
  const key = 'api.' + msg;
  const translated = t(key);
  return translated === key ? msg : translated;
}

function renderHeader(activePage) {
  const el = document.getElementById('app-header');
  if (!el) return;
  const pages = [
    ['/', 'nav.dashboard'],
    ['/servers', 'nav.servers'],
    ['/subscriptions', 'nav.subscriptions'],
    ['/routing', 'nav.routing'],
    ['/settings', 'nav.settings'],
    ['/logs', 'nav.logs'],
  ];
  const path = location.pathname.replace(/\/$/, '') || '/';
  const navLinks = pages.map(([href, key]) => {
    const active = href === path ? ' class="active"' : '';
    return `<a href="${href}"${active} data-i18n="${key}">${t(key)}</a>`;
  }).join('');
  el.innerHTML = `
    <strong data-i18n="brand">${t('brand')}</strong>
    <div class="header-right">
      <nav>${navLinks}</nav>
      <div class="lang-switch">
        <button type="button" class="lang-btn${_locale === 'ru' ? ' active' : ''}" data-lang="ru">RU</button>
        <button type="button" class="lang-btn${_locale === 'en' ? ' active' : ''}" data-lang="en">EN</button>
      </div>
    </div>`;
  el.querySelectorAll('[data-lang]').forEach(btn => {
    btn.onclick = () => { if (btn.dataset.lang !== _locale) setLang(btn.dataset.lang); };
  });
}

function renderOnboardingHeader() {
  const el = document.getElementById('app-header');
  if (!el) return;
  el.innerHTML = `
    <strong data-i18n="onboarding.title">${t('onboarding.title')}</strong>
    <div class="header-right">
      <nav><a href="/logout" data-i18n="nav.logout">${t('nav.logout')}</a></nav>
      <div class="lang-switch">
        <button type="button" class="lang-btn${_locale === 'ru' ? ' active' : ''}" data-lang="ru">RU</button>
        <button type="button" class="lang-btn${_locale === 'en' ? ' active' : ''}" data-lang="en">EN</button>
      </div>
    </div>`;
  el.querySelectorAll('[data-lang]').forEach(btn => {
    btn.onclick = () => { if (btn.dataset.lang !== _locale) setLang(btn.dataset.lang); };
  });
}

function renderLoginLangSwitch() {
  const el = document.getElementById('login-lang');
  if (!el) return;
  el.innerHTML = `
    <button type="button" class="lang-btn${_locale === 'ru' ? ' active' : ''}" data-lang="ru">RU</button>
    <button type="button" class="lang-btn${_locale === 'en' ? ' active' : ''}" data-lang="en">EN</button>`;
  el.querySelectorAll('[data-lang]').forEach(btn => {
    btn.onclick = () => { if (btn.dataset.lang !== _locale) setLang(btn.dataset.lang); };
  });
}

function whenI18nReady(fn) {
  if (_strings && Object.keys(_strings).length) return Promise.resolve(fn());
  return initI18n().then(fn);
}
