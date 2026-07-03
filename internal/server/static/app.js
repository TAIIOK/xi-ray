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

function setActiveNav() {
  const path = location.pathname.replace(/\/$/, '') || '/';
  document.querySelectorAll('nav a').forEach(a => {
    const href = a.getAttribute('href').replace(/\/$/, '') || '/';
    a.classList.toggle('active', href === path);
  });
}

document.addEventListener('DOMContentLoaded', setActiveNav);
