const API = {
  langHeaders() {
    const lang = typeof getLang === 'function' ? getLang() : 'ru';
    return { 'Accept-Language': lang };
  },
  async get(path) {
    const r = await fetch(path, { headers: this.langHeaders() });
    if (!r.ok) throw new Error(await r.text());
    return r.json();
  },
  async send(method, path, body) {
    const r = await fetch(path, {
      method,
      headers: {
        ...this.langHeaders(),
        ...(body ? { 'Content-Type': 'application/json' } : {}),
      },
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

function trimVersion(v) {
  return String(v || '').replace(/^v/i, '');
}

function updateAvailable(st) {
  if (!st) return null;
  const ver = st.available?.version || st.target_version;
  if (!ver) return null;
  return trimVersion(ver) !== trimVersion(st.current_version) ? ver : null;
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
