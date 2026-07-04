const API = {
  langHeaders() {
    const lang = typeof getLang === 'function' ? getLang() : 'ru';
    return { 'Accept-Language': lang };
  },
  async fetch(path, options = {}) {
    const timeoutMs = options.timeoutMs ?? 120000;
    const ctrl = new AbortController();
    const timer = setTimeout(() => ctrl.abort(), timeoutMs);
    try {
      return await fetch(path, {
        method: options.method || 'GET',
        headers: {
          ...this.langHeaders(),
          ...(options.headers || {}),
        },
        body: options.body,
        signal: ctrl.signal,
      });
    } catch (e) {
      if (e && e.name === 'AbortError') {
        const msg = typeof t === 'function' ? t('common.timeout') : 'Request timeout';
        throw new Error(msg);
      }
      throw e;
    } finally {
      clearTimeout(timer);
    }
  },
  async get(path, options = {}) {
    const r = await this.fetch(path, options);
    if (!r.ok) throw new Error(await r.text());
    return r.json();
  },
  async send(method, path, body, options = {}) {
    const r = await this.fetch(path, {
      ...options,
      method,
      headers: { ...(body ? { 'Content-Type': 'application/json' } : {}), ...(options.headers || {}) },
      body: body ? JSON.stringify(body) : undefined,
    });
    const data = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(translateApiMessage(data.error) || data.error || r.statusText);
    if (data.message) data.message = translateApiMessage(data.message);
    if (data.message_key && typeof t === 'function') {
      const tk = t('api.' + data.message_key);
      if (tk !== 'api.' + data.message_key) data.message = tk;
    }
    return data;
  },
};

function stateClass(state) {
  return `status-${state || 'stopped'}`;
}

function healthClass(h) {
  if (h === 'ok' || h === 'alive') return 'health-ok';
  if (h === 'dead') return 'health-dead';
  return 'health-unknown';
}

function fmtBytes(n) {
  if (!n) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  let i = 0;
  while (n >= 1024 && i < units.length - 1) { n /= 1024; i++; }
  return `${n.toFixed(1)} ${units[i]}`;
}

function showMsg(el, text, ok = false) {
  if (!el) return;
  el.className = 'msg ' + (ok ? 'ok' : 'error');
  el.textContent = text;
  el.hidden = !text;
}

function setButtonBusy(btn, busy, busyText) {
  if (!btn) return;
  if (busy) {
    if (!btn.dataset.origText) btn.dataset.origText = btn.textContent;
    btn.disabled = true;
    btn.classList.add('busy');
    btn.textContent = busyText || '…';
  } else {
    btn.disabled = false;
    btn.classList.remove('busy');
    if (btn.dataset.origText) btn.textContent = btn.dataset.origText;
    delete btn.dataset.origText;
  }
}

async function withButtonBusy(btn, busyText, fn) {
  setButtonBusy(btn, true, busyText);
  try {
    return await fn();
  } finally {
    setButtonBusy(btn, false);
  }
}

function parseVersionCore(v) {
  const core = String(v || '').replace(/^v/i, '').split(/[-+]/)[0];
  const parts = core.split('.').map(p => {
    const n = parseInt(p, 10);
    return Number.isFinite(n) ? n : NaN;
  });
  if (parts.length < 2 || parts.some(n => Number.isNaN(n))) return null;
  while (parts.length < 3) parts.push(0);
  return parts.slice(0, 3);
}

function versionNewer(a, b) {
  const va = parseVersionCore(a);
  const vb = parseVersionCore(b);
  if (!va) return false;
  if (!vb) return true;
  for (let i = 0; i < 3; i++) {
    if (va[i] !== vb[i]) return va[i] > vb[i];
  }
  return false;
}

function trimVersion(v) {
  return String(v || '').replace(/^v/i, '');
}

function updateAvailable(st) {
  if (!st) return null;
  const ver = st.available?.version || st.target_version;
  if (!ver) return null;
  return versionNewer(ver, st.current_version) ? ver : null;
}

function guestNetworkClass(gn) {
  if (!gn) return 'health-unknown';
  if (gn.ok && gn.is_guest) return 'health-ok';
  if (gn.is_guest) return 'health-unknown';
  return 'health-dead';
}

function guestNetworkNeedsConfirm(gn) {
  return !!(gn && (!gn.ok || !gn.is_guest));
}

function renderGuestNetworkStatus(containerEl, gn, escapeFn) {
  if (!containerEl) return;
  const esc = escapeFn || (s => String(s || ''));
  if (!gn || !gn.message) {
    containerEl.hidden = true;
    containerEl.innerHTML = '';
    return;
  }
  const cls = guestNetworkClass(gn);
  let html = `<div class="${cls}">${esc(gn.message)}</div>`;
  if (gn.detected_gateway) {
    const iface = gn.interface || 'br-guest';
    const subnet = gn.detected_subnet ? ` (${esc(gn.detected_subnet)})` : '';
    html += `<div class="label">${esc(iface)}: ${esc(gn.detected_gateway)}${subnet}</div>`;
  }
  if (gn.main_lan_subnet) {
    html += `<div class="label">Основная LAN: ${esc(gn.main_lan_subnet)}</div>`;
  }
  if (gn.warning && gn.warning !== gn.message) {
    html += `<div class="label health-unknown">${esc(gn.warning)}</div>`;
  }
  containerEl.innerHTML = html;
  containerEl.hidden = false;
}

function setActiveNav() {
  const path = location.pathname.replace(/\/$/, '') || '/';
  document.querySelectorAll('nav a').forEach(a => {
    const href = a.getAttribute('href').replace(/\/$/, '') || '/';
    a.classList.toggle('active', href === path);
  });
}

// setActiveNav is called from renderHeader in i18n.js
