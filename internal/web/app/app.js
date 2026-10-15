// batnet app: plain JavaScript, no framework, served by the house Pi.
// Every piece of text from the server goes into the page as text, never as HTML.

const app = document.getElementById('app');
const state = { me: null, house: '', conv: null, tab: 'chats', unread: {}, loc: false, prio: 'normal', alert: null, events: null, names: {}, pendingTimer: null, retry: 1000 };

// ---------- helpers ----------

function esc(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

// h builds an element: h('button', {class: 'btn', onclick: f}, 'Send')
function h(tag, attrs, ...kids) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (v === false || v == null) continue;
    if (k.startsWith('on')) el.addEventListener(k.slice(2), v);
    else if (k === 'class') el.className = v;
    else el.setAttribute(k, v === true ? '' : v);
  }
  for (const kid of kids.flat()) {
    if (kid == null || kid === false) continue;
    el.append(kid instanceof Node ? kid : document.createTextNode(String(kid)));
  }
  return el;
}

function show(...els) {
  app.replaceChildren(...els.filter((e) => e != null && e !== false));
}

async function api(path, body) {
  const opts = { credentials: 'same-origin', headers: {} };
  if (body !== undefined) {
    opts.method = 'POST';
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(path, opts);
  let data = null;
  try { data = await res.json(); } catch (e) { data = null; }
  if (!res.ok) {
    const err = new Error((data && data.error) || 'Something went wrong.');
    err.status = res.status;
    err.retryAfter = data && data.retryAfter;
    throw err;
  }
  return data;
}

function time(ts) {
  const d = new Date(ts.Wall || 0);
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

function initials(name) {
  return String(name).split(/\s+/).map((w) => w[0] || '').join('').slice(0, 2).toUpperCase();
}

// ---------- sound (only while the app is open: a locked phone can't ring) ----------

let audio = null;
function unlockAudio() {
  if (!audio) {
    try { audio = new (window.AudioContext || window.webkitAudioContext)(); } catch (e) { audio = null; }
  }
}
document.addEventListener('pointerdown', unlockAudio, { once: true });

function beep(kind) {
  if (!audio) return;
  const notes = kind === 'alert' ? [880, 660, 880, 660, 880] : kind === 'high' ? [880, 880] : [660];
  let t = audio.currentTime;
  for (const f of notes) {
    const o = audio.createOscillator(), g = audio.createGain();
    o.frequency.value = f;
    g.gain.value = kind === 'normal' ? 0.08 : 0.2;
    o.connect(g).connect(audio.destination);
    o.start(t);
    o.stop(t + 0.15);
    t += 0.2;
  }
  if (navigator.vibrate) navigator.vibrate(kind === 'normal' ? 80 : [200, 100, 200]);
}

// ---------- start ----------

async function boot() {
  try { state.house = (await api('/api/house')).house; } catch (e) { state.house = ''; }
  document.title = 'batnet · ' + state.house;
  try {
    state.me = await api('/api/me');
  } catch (e) {
    state.me = null;
  }
  route();
}

function route() {
  if (!state.me) return landing();
  if (state.me.status !== 'approved') return pending();
  listen();
  if (state.conv) return conversation();
  if (state.tab === 'people') return people();
  if (state.tab === 'settings') return settings();
  return chats();
}

// ---------- signed out ----------

function landing(mode) {
  const err = h('p', { class: 'err', role: 'alert' });
  const name = h('input', { type: 'text', autocomplete: 'name', maxlength: 40, required: true });
  const pin = h('input', { type: 'password', inputmode: 'numeric', pattern: '[0-9]{6}', maxlength: 6, autocomplete: mode === 'signup' ? 'new-password' : 'current-password', required: true });
  const submit = async (e) => {
    e.preventDefault();
    err.textContent = '';
    try {
      await api(mode === 'signup' ? '/api/signup' : '/api/login', { name: name.value, pin: pin.value });
      state.me = await api('/api/me');
      route();
    } catch (x) {
      err.textContent = x.retryAfter ? x.message + ' (' + x.retryAfter + ' s)' : x.message;
    }
  };
  show(
    h('header', {}, h('h1', {}, 'batnet · ' + state.house, h('span', { class: 'sub' }, 'No internet: this connects you to the houses around you'))),
    h('form', { class: 'pad', onsubmit: submit },
      h('h2', {}, mode === 'signup' ? 'Create an account' : 'Sign in'),
      h('label', {}, 'Your name', name),
      h('label', {}, mode === 'signup' ? 'Choose a 6-digit PIN' : 'Your 6-digit PIN', pin),
      err,
      h('button', { class: 'btn', type: 'submit' }, mode === 'signup' ? 'Ask to join' : 'Sign in'),
      h('button', { class: 'btn ghost', type: 'button', onclick: () => landing(mode === 'signup' ? 'login' : 'signup') },
        mode === 'signup' ? 'I already have an account' : 'Create an account'),
      h('p', { class: 'muted' }, mode === 'signup'
        ? 'Someone in ' + state.house + ' approves new accounts, so strangers walking past can\'t join.'
        : 'You can sign in at any house in the village.')));
}

function pending() {
  const denied = state.me.status === 'denied';
  show(
    h('header', {}, h('h1', {}, 'batnet · ' + state.house)),
    h('div', { class: 'pad' },
      h('h2', {}, denied ? 'Not approved' : 'Waiting for approval'),
      h('p', {}, denied
        ? 'Sorry, ' + state.me.name + ': ' + (state.me.house || state.house) + '\'s admin did not approve this account. Ask them in person.'
        : 'Thanks, ' + state.me.name + '. ' + (state.me.house || state.house) + '\'s admin needs to let you in. This page updates by itself.'),
      h('button', { class: 'btn ghost', onclick: logout }, 'Sign out')));
  clearTimeout(state.pendingTimer);
  if (denied) return;
  state.pendingTimer = setTimeout(async () => {
    try { state.me = await api('/api/me'); } catch (e) { state.me = null; }
    route();
  }, 5000);
}

async function logout() {
  clearTimeout(state.pendingTimer);
  try { await api('/api/logout', {}); } catch (e) { /* signed out anyway */ }
  if (state.events) state.events.close();
  state.events = null;
  state.me = null;
  state.conv = null;
  route();
}

// ---------- live events ----------

function listen() {
  if (state.events) return;
  state.events = new EventSource('/api/events');
  state.events.addEventListener('message', (ev) => {
    let m;
    try { m = JSON.parse(ev.data); } catch (e) { return; }
    const mine = m.from === state.me.account;
    const key = m.audience.startsWith('dm:') ? (mine ? m.audience : 'dm:' + m.from) : m.audience;
    if (m.alert) state.alert = m;
    if (!mine) beep(m.alert ? 'alert' : m.prio >= 8 ? 'high' : 'normal');
    if (state.conv === key) appendBubble(m); // add it in place: rebuilding would wipe what the person is typing
    else {
      if (!mine) state.unread[key] = (state.unread[key] || 0) + 1;
      if (!state.conv) route();
    }
  });
  state.events.addEventListener('state', () => { if (!state.conv) route(); });
  state.events.addEventListener('open', () => {
    state.retry = 1000;
    if (!state.conv) route(); // catch up on anything missed while disconnected
  });
  state.events.addEventListener('error', () => {
    if (state.events && state.events.readyState === EventSource.CLOSED) {
      // the browser gave up (e.g. the Pi restarted and the session is gone): check, then reconnect with backoff
      state.events = null;
      setTimeout(async () => {
        try { state.me = await api('/api/me'); } catch (e) { state.me = null; }
        state.retry = Math.min(state.retry * 2, 60000);
        route();
      }, state.retry);
    }
  });
}

function appendBubble(m) {
  const list = document.querySelector('.msgs');
  if (!list) return conversation();
  const atBottom = list.scrollTop + list.clientHeight >= list.scrollHeight - 30;
  list.append(bubble(m, state.names));
  if (atBottom) list.scrollTop = list.scrollHeight;
}

// ---------- signed in ----------

function tabs() {
  const tab = (k, label) => h('button', { 'aria-current': state.tab === k ? 'page' : false, onclick: () => { state.tab = k; state.conv = null; route(); } }, label);
  return h('nav', { class: 'tabs' }, tab('chats', 'Chats'), tab('people', 'People'), tab('settings', 'Settings'));
}

async function chats() {
  let convs = [];
  let dir = { groups: [] };
  try {
    [convs, dir] = await Promise.all([api('/api/conversations'), api('/api/directory')]);
  } catch (e) { if (e.status === 401) return logout(); }
  const keys = new Set(convs.map((c) => c.key));
  const rows = convs.map((c) => convRow(c.key, c.name, c.last));
  if (!keys.has('everyone')) rows.unshift(convRow('everyone', 'Everyone', null));
  const alertBar = state.alert ? h('div', { class: 'alertbar', role: 'alert' }, h('b', {}, '⚠ Alert · '), state.alert.text) : null;
  show(
    h('header', {}, h('h1', {}, 'Chats', h('span', { class: 'sub' }, state.me.name + ' · ' + state.house))),
    alertBar,
    h('div', { class: 'grow' }, rows),
    tabs());
}

function convRow(key, name, last) {
  const n = state.unread[key] || 0;
  return h('button', { class: 'row', onclick: () => { state.conv = key; state.unread[key] = 0; conversation(); } },
    h('span', { class: 'av' }, key === 'everyone' ? 'All' : initials(name)),
    h('span', { class: 'main' }, h('span', { class: 'title' }, name), h('div', { class: 'last' }, last ? last.text : 'No messages yet')),
    n ? h('span', { class: 'badge' }, n) : null);
}

async function conversation() {
  let msgs = [], dir = { houses: [], groups: [] };
  try {
    [msgs, dir] = await Promise.all([api('/api/messages?conv=' + encodeURIComponent(state.conv)), api('/api/directory')]);
  } catch (e) { if (e.status === 401) return logout(); }
  const names = {};
  for (const house of dir.houses) for (const p of house.people) names[p.id] = p.name + ' · ' + house.name;
  state.names = names;
  const title = state.conv === 'everyone' ? 'Everyone'
    : state.conv.startsWith('group:') ? ((dir.groups.find((g) => 'group:' + g.id === state.conv) || {}).name || state.conv)
    : (names[state.conv.slice(3)] || 'Direct message');
  const list = h('div', { class: 'msgs' }, msgs.map((m) => bubble(m, names)));
  const err = h('p', { class: 'err', role: 'alert' });
  const text = h('input', { type: 'text', placeholder: 'Message', maxlength: 2000, 'aria-label': 'Message' });
  const prio = h('select', { 'aria-label': 'Priority', onchange: (e) => { state.prio = e.target.value; if (state.prio === 'high') { state.loc = true; locBtn.classList.add('on'); } } },
    h('option', { value: 'normal' }, 'Normal'), h('option', { value: 'high' }, '❗ High'), h('option', { value: 'low' }, 'Low'),
    state.me.admin && state.conv === 'everyone' ? h('option', { value: 'alert' }, '⚠ Alert') : null);
  prio.value = state.prio;
  const locBtn = h('button', { type: 'button', class: 'icon' + (state.loc ? ' on' : ''), 'aria-pressed': String(state.loc), title: 'Attach my location', onclick: () => { state.loc = !state.loc; locBtn.classList.toggle('on', state.loc); locBtn.setAttribute('aria-pressed', String(state.loc)); } }, '📍');
  const send = async (e) => {
    e.preventDefault();
    if (!text.value.trim()) return;
    err.textContent = '';
    const body = { conv: state.conv, text: text.value, prio: state.prio };
    try {
      if (state.loc) body.loc = await position();
      await api('/api/post', body);
      text.value = '';
      state.prio = 'normal';
      state.loc = false;
      prio.value = 'normal';
      locBtn.classList.remove('on');
    } catch (x) {
      err.textContent = x.message;
    }
  };
  show(
    h('header', {}, h('button', { class: 'back', 'aria-label': 'Back', onclick: () => { state.conv = null; route(); } }, '‹'), h('h1', {}, title)),
    list,
    err,
    h('form', { class: 'compose', onsubmit: send }, prio, locBtn, text, h('button', { class: 'icon on', type: 'submit', 'aria-label': 'Send' }, '➤')));
  list.scrollTop = list.scrollHeight;
}

function bubble(m, names) {
  const mine = m.from === state.me.account;
  const cls = 'bubble' + (mine ? ' me' : '') + (m.alert ? ' alert' : m.prio >= 8 ? ' high' : '');
  return h('div', { class: cls },
    m.alert ? h('div', { class: 'who' }, '⚠ Alert from ' + (names[m.from] || 'the house admin'))
      : !mine && !m.audience.startsWith('dm:') ? h('div', { class: 'who' }, names[m.from] || 'Someone') : null,
    h('div', {}, m.text),
    m.loc ? h('div', { class: 'loc' }, '📍 ' + m.loc.lat.toFixed(4) + ', ' + m.loc.lon.toFixed(4) + ' (±' + Math.round(m.loc.acc) + ' m, ' + (m.loc.source === 'node' ? 'house GPS' : 'phone GPS') + ')') : null,
    h('div', { class: 'meta' }, (m.prio >= 8 && !m.alert ? '❗ High · ' : '') + time(m.ts)));
}

function position() {
  return new Promise((resolve, reject) => {
    if (!navigator.geolocation) return reject(new Error('This browser has no location.'));
    navigator.geolocation.getCurrentPosition(
      (p) => resolve({ lat: p.coords.latitude, lon: p.coords.longitude, acc: Math.min(10000, p.coords.accuracy || 0), source: 'phone' }),
      () => reject(new Error('Couldn\'t get your location. Send without it, or try again outside.')),
      { enableHighAccuracy: true, timeout: 60000, maximumAge: 60000 });
  });
}

async function people() {
  let dir = { houses: [], groups: [] };
  try { dir = await api('/api/directory'); } catch (e) { if (e.status === 401) return logout(); }
  const rows = [];
  for (const house of dir.houses) {
    rows.push(h('div', { class: 'section' }, house.name + ' · ' + house.people.length));
    for (const p of house.people) {
      if (p.id === state.me.account) continue;
      rows.push(h('button', { class: 'row', onclick: () => { state.conv = 'dm:' + p.id; conversation(); } },
        h('span', { class: 'av' }, initials(p.name)), h('span', { class: 'main' }, h('span', { class: 'title' }, p.name + (p.admin ? ' · admin' : '')))));
    }
  }
  rows.push(h('div', { class: 'section' }, 'Groups'));
  for (const g of dir.groups) {
    rows.push(h('div', { class: 'row' },
      h('span', { class: 'av' }, initials(g.name)),
      h('button', { class: 'main title', style: 'border:0;background:none;text-align:left', onclick: () => { state.conv = 'group:' + g.id; conversation(); } }, g.name),
      h('button', { class: 'btn ghost', onclick: async () => { await api('/api/groups/join', { group: g.id }); people(); } }, 'Join'),
      h('button', { class: 'btn ghost', onclick: async () => { await api('/api/groups/leave', { group: g.id }); people(); } }, 'Leave')));
  }
  show(h('header', {}, h('h1', {}, 'People', h('span', { class: 'sub' }, 'Everyone in the village'))), h('div', { class: 'grow' }, rows), tabs());
}

async function settings() {
  const parts = [
    h('div', { class: 'section' }, 'You'),
    h('div', { class: 'pad' }, h('p', {}, state.me.name + ' · ' + (state.me.house || state.house)), h('button', { class: 'btn ghost', onclick: logout }, 'Sign out')),
    h('div', { class: 'section' }, 'Notifications'),
    h('div', { class: 'pad' }, h('p', { class: 'muted' }, 'A locked phone can\'t ring without the internet. Keep batnet open to hear new messages.')),
  ];
  if (state.me.admin) {
    let pend = [];
    try { pend = await api('/api/admin/pending'); } catch (e) { pend = []; }
    parts.push(h('div', { class: 'section' }, 'Join requests'));
    if (!pend.length) parts.push(h('div', { class: 'pad' }, h('p', { class: 'muted' }, 'No requests.')));
    for (const p of pend) {
      parts.push(h('div', { class: 'row' }, h('span', { class: 'av' }, initials(p.name)), h('span', { class: 'main title' }, p.name),
        h('button', { class: 'btn', onclick: async () => { await api('/api/admin/approve', { account: p.id }); settings(); } }, 'Approve'),
        h('button', { class: 'btn ghost', onclick: async () => { await api('/api/admin/deny', { account: p.id }); settings(); } }, 'Deny')));
    }
  }
  show(h('header', {}, h('h1', {}, 'Settings')), h('div', { class: 'grow' }, parts), tabs());
}

boot();
