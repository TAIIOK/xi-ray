const API = {
  async get(path) {
    const r = await fetch(path);
    if (!r.ok) throw new Error(await r.text());
    return r.json();
  },
  async send(method, path, body) {
    const r = await fetch(path, {
      method,
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body: body ? JSON.stringify(body) : undefined,
    });
    const data = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(data.error || r.statusText);
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

function setActiveNav() {
  const path = location.pathname.replace(/\/$/, '') || '/';
  document.querySelectorAll('nav a').forEach(a => {
    const href = a.getAttribute('href').replace(/\/$/, '') || '/';
    a.classList.toggle('active', href === path);
  });
}

document.addEventListener('DOMContentLoaded', setActiveNav);
