const ROUTING_PRESETS = {
  'direct-geosite-cn': { name: 'China → direct', action: 'direct', domains: ['geosite:cn'] },
  'direct-geosite-private': { name: 'Private domains → direct', action: 'direct', domains: ['geosite:private'] },
  'direct-geoip-ru': { name: 'Russia IP → direct', action: 'direct', ips: ['geoip:ru'] },
  'direct-geoip-cn': { name: 'China IP → direct', action: 'direct', ips: ['geoip:cn'] },
  'proxy-geosite-google': { name: 'Google → proxy', action: 'proxy', domains: ['geosite:google'] },
  'proxy-geosite-youtube': { name: 'YouTube → proxy', action: 'proxy', domains: ['geosite:youtube'] },
  'proxy-geosite-telegram': { name: 'Telegram → proxy', action: 'proxy', domains: ['geosite:telegram'] },
  'block-ads': { name: 'Block ads', action: 'block', domains: ['geosite:category-ads-all'] },
  'block-tracking': { name: 'Block trackers', action: 'block', domains: ['geosite:category-ads'] },
};

const ROUTING_TEMPLATES = {
  'vpn-all': {
    name: 'Весь гостевой трафик через VPN',
    routing: {
      domain_strategy: 'IPIfNonMatch',
      rule_order: ['direct', 'proxy', 'block'],
      default_guest_action: 'proxy',
      bypass_private: true,
      bypass_vpn_hosts: true,
      rules: [],
    },
  },
  'split-cn-ru': {
    name: 'CN/RU напрямую, остальное VPN',
    routing: {
      domain_strategy: 'IPIfNonMatch',
      rule_order: ['direct', 'proxy', 'block'],
      default_guest_action: 'proxy',
      bypass_private: true,
      bypass_vpn_hosts: true,
      rules: [
        { name: 'CN sites', action: 'direct', domains: ['geosite:cn'], enabled: true },
        { name: 'RU IP', action: 'direct', ips: ['geoip:ru'], enabled: true },
      ],
    },
  },
  'split-with-ads-block': {
    name: 'CN direct + block ads + VPN',
    routing: {
      domain_strategy: 'IPIfNonMatch',
      rule_order: ['direct', 'block', 'proxy'],
      default_guest_action: 'proxy',
      bypass_private: true,
      bypass_vpn_hosts: true,
      rules: [
        { name: 'CN direct', action: 'direct', domains: ['geosite:cn'], enabled: true },
        { name: 'Block ads', action: 'block', domains: ['geosite:category-ads-all'], enabled: true },
      ],
    },
  },
};

const RoutingPage = {
  rules: [],
  meta: { use_balancer: false, selection_mode: 'single', active_nodes: 0 },

  splitMatchers(text) {
    return String(text || '').split(/[\n,]+/).map(s => s.trim()).filter(Boolean);
  },

  joinMatchers(list) {
    return (list || []).join('\n');
  },

  escapeAttr(s) {
    return String(s).replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;');
  },

  escapeHtml(s) {
    return String(s || '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  },

  newId() {
    return crypto.randomUUID ? crypto.randomUUID() : `r-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  },

  collectRouting() {
    return {
      domain_strategy: document.getElementById('domain_strategy').value,
      default_guest_action: document.getElementById('default_guest').value,
      rule_order: [
        document.getElementById('order_1').value,
        document.getElementById('order_2').value,
        document.getElementById('order_3').value,
      ],
      bypass_private: document.getElementById('bypass_private').checked,
      bypass_vpn_hosts: document.getElementById('bypass_vpn_hosts').checked,
      rules: this.rules.map(r => ({
        id: r.id,
        name: r.name,
        action: r.action,
        domains: r.domains || [],
        ips: r.ips || [],
        enabled: r.enabled !== false,
      })),
    };
  },

  setRuleOrder(order) {
    const o = order?.length ? order : ['direct', 'proxy', 'block'];
    document.getElementById('order_1').value = o[0] || 'direct';
    document.getElementById('order_2').value = o[1] || 'proxy';
    document.getElementById('order_3').value = o[2] || 'block';
    this.renderPipeline();
  },

  renderPipeline() {
    const order = [
      document.getElementById('order_1').value,
      document.getElementById('order_2').value,
      document.getElementById('order_3').value,
    ];
    const labels = {
      direct: typeof t === 'function' ? t('routing.action_direct') : 'Direct',
      proxy: typeof t === 'function' ? t('routing.action_proxy') : 'Proxy',
      block: typeof t === 'function' ? t('routing.action_block') : 'Block',
    };
    const el = document.getElementById('pipeline');
    el.innerHTML = order.map((a, i) => `
      <span class="pipeline-step action-${a}">${i + 1}. ${labels[a] || a}</span>
      ${i < order.length - 1 ? '<span class="pipeline-arrow">→</span>' : ''}
    `).join('');
  },

  actionBadge(action) {
    return `<span class="badge action-${action}">${action}</span>`;
  },

  renderRules() {
    const filter = document.getElementById('filter-action')?.value || 'all';
    const list = document.getElementById('rules-list');
    list.innerHTML = '';

    const items = this.rules.map((rule, idx) => ({ rule, idx }))
      .filter(({ rule }) => filter === 'all' || rule.action === filter);

    if (!items.length) {
      list.innerHTML = '<div class="empty-rules">Нет правил. Добавьте вручную или выберите пресет.</div>';
      return;
    }

    items.forEach(({ rule, idx }) => {
      const card = document.createElement('div');
      card.className = `rule-card ${rule.enabled === false ? 'disabled' : ''}`;
      card.innerHTML = `
        <div class="rule-card-head">
          <label class="rule-toggle"><input type="checkbox" data-k="enabled" data-i="${idx}" ${rule.enabled !== false ? 'checked' : ''}> Вкл</label>
          <input class="rule-name" data-k="name" data-i="${idx}" value="${this.escapeAttr(rule.name || '')}" placeholder="Имя правила">
          <select data-k="action" data-i="${idx}" class="rule-action">
            <option value="direct">direct</option>
            <option value="proxy">proxy</option>
            <option value="block">block</option>
          </select>
          <div class="rule-move">
            <button type="button" class="secondary" data-up="${idx}" title="Выше">↑</button>
            <button type="button" class="secondary" data-down="${idx}" title="Ниже">↓</button>
            <button type="button" class="secondary danger" data-del="${idx}" title="Удалить">×</button>
          </div>
        </div>
        <div class="rule-card-body grid-2">
          <div class="field">
            <label class="label">Domains (geosite:, domain:, full:, regexp:)</label>
            <textarea rows="3" data-k="domains" data-i="${idx}" placeholder="geosite:cn\ndomain:example.com">${this.escapeHtml(this.joinMatchers(rule.domains))}</textarea>
          </div>
          <div class="field">
            <label class="label">IPs (geoip:, CIDR)</label>
            <textarea rows="3" data-k="ips" data-i="${idx}" placeholder="geoip:ru\n192.168.0.0/16">${this.escapeHtml(this.joinMatchers(rule.ips))}</textarea>
          </div>
        </div>`;
      list.appendChild(card);
      card.querySelector('[data-k="action"]').value = rule.action || 'direct';
    });

    list.querySelectorAll('[data-k]').forEach(el => {
      el.onchange = el.oninput = () => this.syncRule(el);
    });
    list.querySelectorAll('[data-up]').forEach(btn => {
      btn.onclick = () => this.moveRule(parseInt(btn.dataset.up, 10), -1);
    });
    list.querySelectorAll('[data-down]').forEach(btn => {
      btn.onclick = () => this.moveRule(parseInt(btn.dataset.down, 10), 1);
    });
    list.querySelectorAll('[data-del]').forEach(btn => {
      btn.onclick = () => {
        this.rules.splice(parseInt(btn.dataset.del, 10), 1);
        this.renderRules();
        this.renderPreviewLocal();
      };
    });

    document.getElementById('rules-count').textContent = String(this.rules.length);
  },

  syncRule(el) {
    const idx = parseInt(el.dataset.i, 10);
    const k = el.dataset.k;
    const rule = this.rules[idx];
    if (!rule) return;
    if (k === 'enabled') rule.enabled = el.checked;
    else if (k === 'name') rule.name = el.value;
    else if (k === 'action') rule.action = el.value;
    else if (k === 'domains') rule.domains = this.splitMatchers(el.value);
    else if (k === 'ips') rule.ips = this.splitMatchers(el.value);
    this.renderPreviewLocal();
  },

  moveRule(idx, delta) {
    const j = idx + delta;
    if (j < 0 || j >= this.rules.length) return;
    [this.rules[idx], this.rules[j]] = [this.rules[j], this.rules[idx]];
    this.renderRules();
    this.renderPreviewLocal();
  },

  addRule(presetKey) {
    const base = { id: this.newId(), enabled: true, name: 'New rule', action: 'direct', domains: [], ips: [] };
    const preset = presetKey ? ROUTING_PRESETS[presetKey] : null;
    this.rules.push({ ...base, ...(preset || {}), id: this.newId() });
    this.renderRules();
    this.renderPreviewLocal();
  },

  applyTemplate(key) {
    const tpl = ROUTING_TEMPLATES[key];
    if (!tpl) return;
    if (this.rules.length && !confirm(`Применить шаблон «${tpl.name}»? Текущие правила будут заменены.`)) return;
    const r = JSON.parse(JSON.stringify(tpl.routing));
    r.rules = (r.rules || []).map(rule => ({ ...rule, id: this.newId() }));
    document.getElementById('domain_strategy').value = r.domain_strategy;
    document.getElementById('default_guest').value = r.default_guest_action;
    document.getElementById('bypass_private').checked = r.bypass_private !== false;
    document.getElementById('bypass_vpn_hosts').checked = r.bypass_vpn_hosts !== false;
    this.setRuleOrder(r.rule_order);
    this.rules = r.rules;
    this.renderRules();
    this.renderPreviewLocal();
  },

  renderPreview(preview) {
    const tbody = document.getElementById('preview-body');
    tbody.innerHTML = '';
    (preview || []).forEach(row => {
      const tr = document.createElement('tr');
      const matchers = [...(row.domains || []), ...(row.ips || [])].join(', ') || '—';
      tr.innerHTML = `
        <td>${row.index}</td>
        <td><span class="badge kind-${row.kind}">${row.kind}</span></td>
        <td>${this.escapeHtml(row.name)}</td>
        <td>${this.actionBadge(row.action)}</td>
        <td><code class="matchers">${this.escapeHtml(matchers)}</code></td>
        <td><code>${this.escapeHtml(row.outbound)}</code></td>
        <td class="label">${this.escapeHtml(row.inbound || '')}</td>`;
      tbody.appendChild(tr);
    });
  },

  renderPreviewLocal() {
    // Client-side quick count; server preview on save/load
    const order = [
      document.getElementById('order_1').value,
      document.getElementById('order_2').value,
      document.getElementById('order_3').value,
    ];
    document.getElementById('preview-note').textContent =
      `Локальный черновик: ${this.rules.filter(r => r.enabled !== false).length} правил, порядок групп ${order.join(' → ')}. Точная цепочка — после «Обновить preview».`;
  },

  renderMeta() {
    const el = document.getElementById('meta-info');
    const parts = [
      `Режим серверов: ${this.meta.selection_mode}`,
      `Активных nodes: ${this.meta.active_nodes}`,
      this.meta.use_balancer ? 'Proxy через balancer (multi + observatory)' : 'Proxy через один outbound',
    ];
    el.textContent = parts.join(' · ');
  },

  async load() {
    const data = await API.get('/api/routing');
    const r = data.routing || {};
    this.meta = {
      use_balancer: data.use_balancer,
      selection_mode: data.selection_mode,
      active_nodes: data.active_nodes,
    };
    document.getElementById('domain_strategy').value = r.domain_strategy || 'IPIfNonMatch';
    document.getElementById('default_guest').value = r.default_guest_action || 'proxy';
    document.getElementById('bypass_private').checked = r.bypass_private !== false;
    document.getElementById('bypass_vpn_hosts').checked = r.bypass_vpn_hosts !== false;
    this.setRuleOrder(r.rule_order);
    this.rules = JSON.parse(JSON.stringify(r.rules || []));
    this.renderRules();
    this.renderPreview(data.preview || []);
    this.renderMeta();
    this.renderPreviewLocal();
  },

  async refreshPreview() {
    const result = await API.send('PUT', '/api/routing', {
      routing: this.collectRouting(),
      apply: false,
    });
    this.renderPreview(result.preview || []);
    showMsg(document.getElementById('msg'), typeof t === 'function' ? t('routing.preview_updated') : 'Preview обновлён', true);
  },

  async save(apply) {
    const result = await API.send('PUT', '/api/routing', {
      routing: this.collectRouting(),
      apply,
    });
    this.renderPreview(result.preview || []);
    showMsg(document.getElementById('msg'), result.message, result.ok);
    if (result.ok) await this.load();
  },

  init() {
    ['order_1', 'order_2', 'order_3'].forEach(id => {
      document.getElementById(id).onchange = () => {
        this.renderPipeline();
        this.renderPreviewLocal();
      };
    });

    document.getElementById('filter-action').onchange = () => this.renderRules();
    document.getElementById('btn-add-rule').onclick = () => this.addRule();
    document.querySelectorAll('[data-preset]').forEach(btn => {
      btn.onclick = () => this.addRule(btn.dataset.preset);
    });
    document.querySelectorAll('[data-template]').forEach(btn => {
      btn.onclick = () => this.applyTemplate(btn.dataset.template);
    });
    document.getElementById('btn-preview').onclick = () => this.refreshPreview().catch(e => showMsg(document.getElementById('msg'), e.message));
    document.getElementById('btn-save').onclick = () => this.save(false).catch(e => showMsg(document.getElementById('msg'), e.message));
    document.getElementById('btn-save-apply').onclick = () => this.save(true).catch(e => showMsg(document.getElementById('msg'), e.message));

    this.load().catch(e => showMsg(document.getElementById('msg'), e.message));
  },
};
