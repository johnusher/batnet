// batnet app: plain JavaScript, no framework, served by the house Pi.
// Every piece of text from the server goes into the page as text, never as HTML.

const app = document.getElementById('app');
const state = {
  me: null, house: '', conv: null, tab: 'chats', unread: {}, loc: false, prio: 'normal', alert: null,
  events: null, names: {}, pendingTimer: null, retry: 1000, bedside: null,
};

// ---------- words: English and German ----------

const T = {
  en: {
    tagline: 'No internet: this connects you to the houses around you',
    signIn: 'Sign in', createAccount: 'Create an account', yourName: 'Your name', choosePin: 'Choose a 6-digit PIN', yourPin: 'Your 6-digit PIN',
    askToJoin: 'Ask to join', haveAccount: 'I already have an account', anyHouse: 'You can sign in at any house in the village.',
    approvesNew: (h) => 'Someone in ' + h + ' approves new accounts, so strangers walking past can\'t join.',
    waiting: 'Waiting for approval', notApproved: 'Not approved',
    waitingText: (n, h) => 'Thanks, ' + n + '. ' + h + '\'s admin needs to let you in. This page updates by itself.',
    deniedText: (n, h) => 'Sorry, ' + n + ': ' + h + '\'s admin did not approve this account. Ask them in person.',
    signOut: 'Sign out', chats: 'Chats', people: 'People', settings: 'Settings', everyone: 'Everyone', noMessages: 'No messages yet',
    message: 'Message', normal: 'Normal', high: '❗ High', low: 'Low', attachLoc: 'Attach my location', send: 'Send', back: 'Back',
    waitingToSend: 'waiting to send', failed: 'not sent', sendAsNormal: 'Send as Normal', sendAgain: 'Send again', delete: 'Delete', alertFrom: (n) => '⚠ Alert from ' + n, someone: 'Someone', theAdmin: 'the house admin',
    houseGps: 'house GPS', phoneGps: 'phone GPS', noLocation: 'Couldn\'t get your location. Send without it, or try again outside.',
    noGeo: 'This browser has no location.', groups: 'Groups', join: 'Join', leave: 'Leave', everyoneIn: 'Everyone in the village', directMessage: 'Direct message',
    you: 'You', notifications: 'Notifications', speaker: 'Announce my messages on the house speaker', speakerSub: 'Says who a message is for and who sent it, never what it says',
    quiet: 'Quiet hours, 22:00–07:00', quietSub: 'Only High messages and alerts are announced', locked: 'A locked phone can\'t ring without the internet. Keep batnet open, or use bedside mode.',
    bedside: 'Bedside mode', bedsideSub: 'Keeps the screen on and dim while charging, so the phone can ring at night', bedsideOn: 'batnet will ring for new messages',
    bedsideNoLock: 'Keep the screen on yourself: this browser can\'t.', tapToLeave: 'Tap to leave', display: 'Display', language: 'Language', auto: 'Automatic',
    largeText: 'Large text', on: 'On', off: 'Off', joinRequests: 'Join requests', noRequests: 'No requests.', approve: 'Approve', deny: 'Deny',
    alertEveryone: 'Alert everyone', alertHelp: 'For emergencies only. It goes ahead of everything else and shows at the top of every phone.',
    sendAlert: 'Send alert to everyone', alertSent: 'Alert sent.', help: 'How batnet works', alertBanner: '⚠ Alert', dismiss: 'Dismiss',
    helpText: [
      'batnet works without the internet. Your house\'s Pi links to the houses around it by WiFi, and to houses further away by radio.',
      'Your browser warned that this page is "not private" once. That\'s expected without the internet: the connection is still encrypted.',
      'Keep batnet open to hear new messages: a locked phone can\'t ring. At night, use bedside mode.',
      'Normal is for most messages. High goes first over the radio and alerts people: you have 3 an hour. Low can wait.',
      'Messages you write out of range wait on your phone and send by themselves when you\'re back.',
      'Turn on the house speaker in Settings to have the house Pi announce messages for you.',
    ],
    retryIn: (s) => ' (try again in ' + s + ' s)',
  },
  de: {
    tagline: 'Kein Internet: Das verbindet dich mit den Häusern um dich herum',
    signIn: 'Anmelden', createAccount: 'Konto anlegen', yourName: 'Dein Name', choosePin: 'Wähle eine 6-stellige PIN', yourPin: 'Deine 6-stellige PIN',
    askToJoin: 'Beitritt anfragen', haveAccount: 'Ich habe schon ein Konto', anyHouse: 'Du kannst dich in jedem Haus im Dorf anmelden.',
    approvesNew: (h) => 'Jemand in ' + h + ' gibt neue Konten frei, damit Fremde im Vorbeigehen nicht beitreten können.',
    waiting: 'Warten auf Freigabe', notApproved: 'Nicht freigegeben',
    waitingText: (n, h) => 'Danke, ' + n + '. Die Verwaltung von ' + h + ' muss dich freigeben. Diese Seite aktualisiert sich von selbst.',
    deniedText: (n, h) => 'Tut uns leid, ' + n + ': Die Verwaltung von ' + h + ' hat dieses Konto nicht freigegeben. Frag sie persönlich.',
    signOut: 'Abmelden', chats: 'Chats', people: 'Leute', settings: 'Einstellungen', everyone: 'Alle', noMessages: 'Noch keine Nachrichten',
    message: 'Nachricht', normal: 'Normal', high: '❗ Wichtig', low: 'Niedrig', attachLoc: 'Meinen Standort anhängen', send: 'Senden', back: 'Zurück',
    waitingToSend: 'wartet auf Versand', failed: 'nicht gesendet', sendAsNormal: 'Als Normal senden', sendAgain: 'Erneut senden', delete: 'Löschen', alertFrom: (n) => '⚠ Alarm von ' + n, someone: 'Jemand', theAdmin: 'der Hausverwaltung',
    houseGps: 'Haus-GPS', phoneGps: 'Handy-GPS', noLocation: 'Standort nicht gefunden. Sende ohne, oder versuch es draußen noch einmal.',
    noGeo: 'Dieser Browser kennt keinen Standort.', groups: 'Gruppen', join: 'Beitreten', leave: 'Verlassen', everyoneIn: 'Alle im Dorf', directMessage: 'Direktnachricht',
    you: 'Du', notifications: 'Benachrichtigungen', speaker: 'Meine Nachrichten über den Hauslautsprecher ansagen', speakerSub: 'Sagt, für wen eine Nachricht ist und von wem, nie was drinsteht',
    quiet: 'Ruhezeit, 22:00–07:00', quietSub: 'Nur wichtige Nachrichten und Alarme werden angesagt', locked: 'Ein gesperrtes Handy kann ohne Internet nicht klingeln. Lass batnet offen, oder nutze den Nachtmodus.',
    bedside: 'Nachtmodus', bedsideSub: 'Hält den Bildschirm beim Laden an und gedimmt, damit das Handy nachts klingeln kann', bedsideOn: 'batnet klingelt bei neuen Nachrichten',
    bedsideNoLock: 'Lass den Bildschirm selbst an: Dieser Browser kann das nicht.', tapToLeave: 'Tippen zum Verlassen', display: 'Anzeige', language: 'Sprache', auto: 'Automatisch',
    largeText: 'Große Schrift', on: 'An', off: 'Aus', joinRequests: 'Beitrittsanfragen', noRequests: 'Keine Anfragen.', approve: 'Freigeben', deny: 'Ablehnen',
    alertEveryone: 'Alle alarmieren', alertHelp: 'Nur für Notfälle. Geht allem anderen vor und erscheint oben auf jedem Handy.',
    sendAlert: 'Alarm an alle senden', alertSent: 'Alarm gesendet.', help: 'So funktioniert batnet', alertBanner: '⚠ Alarm', dismiss: 'Ausblenden',
    helpText: [
      'batnet funktioniert ohne Internet. Der Pi deines Hauses verbindet sich per WLAN mit den Häusern in der Nähe und per Funk mit weiter entfernten.',
      'Dein Browser hat einmal gewarnt, diese Seite sei „nicht privat“. Das ist ohne Internet normal: Die Verbindung ist trotzdem verschlüsselt.',
      'Lass batnet offen, um neue Nachrichten zu hören: Ein gesperrtes Handy kann nicht klingeln. Nachts hilft der Nachtmodus.',
      'Normal ist für die meisten Nachrichten. Wichtig geht per Funk zuerst und alarmiert: du hast 3 pro Stunde. Niedrig kann warten.',
      'Nachrichten, die du außer Reichweite schreibst, warten auf deinem Handy und gehen von selbst raus, sobald du zurück bist.',
      'Schalte in den Einstellungen den Hauslautsprecher ein, dann sagt der Pi deine Nachrichten an.',
    ],
    retryIn: (s) => ' (in ' + s + ' s erneut versuchen)',
  },
};

function store(key, value) {
  try {
    if (value === undefined) return localStorage.getItem('batnet.' + key);
    localStorage.setItem('batnet.' + key, value);
  } catch (e) { return null; }
  return null;
}

function lang() {
  const chosen = store('lang');
  if (chosen === 'en' || chosen === 'de') return chosen;
  return (navigator.language || 'en').toLowerCase().startsWith('de') ? 'de' : 'en';
}

function t(key, ...args) {
  const v = (T[lang()] || T.en)[key] ?? T.en[key] ?? key;
  return typeof v === 'function' ? v(...args) : v;
}

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
  document.documentElement.lang = lang();
  document.body.classList.toggle('large', store('large') === '1');
}

// flash shows an error for a moment at the top of the page, so a button that
// failed (out of range, refused) never just does nothing (B29).
function flash(msg) {
  let el = document.getElementById('flash');
  if (!el) {
    el = h('div', { id: 'flash', class: 'flash', role: 'alert' });
    document.body.append(el);
  }
  el.textContent = msg;
  el.hidden = false;
  clearTimeout(state.flashTimer);
  state.flashTimer = setTimeout(() => { el.hidden = true; }, 6000);
}

// act runs a button's request and flashes its error instead of swallowing it.
async function act(f) {
  try { await f(); } catch (x) { flash(x.message); }
}

// loadMe fetches who is signed in, and the village's latest alert with it.
async function loadMe() {
  state.me = await api('/api/me');
  pickAlert(state.me && state.me.alert);
}

// pickAlert shows an alert unless it is the one the person already dismissed.
function pickAlert(a) {
  if (!a) return;
  if (store('dismissedAlert') === a.id) return;
  if (!state.alert || a.ts.Wall >= state.alert.ts.Wall) state.alert = a;
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
  const d = new Date((ts && ts.Wall) || 0);
  return d.toLocaleTimeString(lang(), { hour: '2-digit', minute: '2-digit' });
}

function initials(name) {
  return String(name).split(/\s+/).map((w) => w[0] || '').join('').slice(0, 2).toUpperCase();
}

function newClientId() {
  const b = new Uint8Array(6); // 8 characters: short on the radio, and only compared within one account
  crypto.getRandomValues(b);
  return btoa(String.fromCharCode(...b)).replace(/\+/g, '-').replace(/\//g, '_');
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
  let at = audio.currentTime;
  for (const f of notes) {
    const o = audio.createOscillator(), g = audio.createGain();
    o.frequency.value = f;
    g.gain.value = kind === 'normal' ? 0.08 : 0.2;
    o.connect(g).connect(audio.destination);
    o.start(at);
    o.stop(at + 0.15);
    at += 0.2;
  }
  if (navigator.vibrate) navigator.vibrate(kind === 'normal' ? 80 : [200, 100, 200]);
}

// ---------- the outbox: messages written out of range wait here (IndexedDB) ----------

const outbox = {
  db: null,
  mem: [], // used when IndexedDB is unavailable (private windows): lost on reload
  async open() {
    if (this.db || !window.indexedDB) return this.db;
    this.db = await new Promise((resolve) => {
      let req;
      try { req = indexedDB.open('batnet', 1); } catch (e) { return resolve(null); }
      req.onupgradeneeded = () => req.result.createObjectStore('outbox', { keyPath: 'clientId' });
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => resolve(null);
    });
    return this.db;
  },
  async tx(mode, f) {
    const db = await this.open();
    if (!db) return undefined;
    return new Promise((resolve) => {
      const tr = db.transaction('outbox', mode);
      const req = f(tr.objectStore('outbox'));
      tr.oncomplete = () => resolve(req && req.result);
      tr.onerror = () => resolve(undefined);
    });
  },
  async all() {
    const got = await this.tx('readonly', (s) => s.getAll());
    return (got === undefined ? this.mem.slice() : got).sort((a, b) => a.made - b.made);
  },
  async put(item) {
    if ((await this.tx('readwrite', (s) => s.put(item))) === undefined) {
      this.mem = this.mem.filter((x) => x.clientId !== item.clientId).concat([item]);
    }
  },
  async remove(id) {
    await this.tx('readwrite', (s) => s.delete(id));
    this.mem = this.mem.filter((x) => x.clientId !== id);
  },
  // the outbox is per phone, and a phone can be shared: only its writer sees or sends an item
  mine(item) { return !!state.me && item.account === state.me.account; },
  async add(item) {
    item.account = state.me.account;
    item.made = Date.now();
    item.status = 'waiting';
    await this.put(item);
    this.flush();
  },
  busy: false,
  again: false, // a message was written while a round was sending: go round once more (B27)
  async flush() {
    if (!state.me || state.me.status !== 'approved') return;
    if (this.busy) { this.again = true; return; }
    this.busy = true;
    try {
      do {
        this.again = false;
        for (const item of await this.all()) {
          if (item.status === 'failed' || !this.mine(item)) continue;
          try {
            await api('/api/post', { conv: item.conv, text: item.text, prio: item.prio, loc: item.loc, clientId: item.clientId });
            await this.remove(item.clientId);
          } catch (e) {
            if (e.status === 401) { this.again = false; break; } // signed out: keep it until the person signs in again
            if (!e.status || e.status >= 500) { this.again = false; break; } // out of range or the Pi is busy: try later
            // refused (e.g. 429, the High limit): mark it with the reason and carry on with the rest
            item.status = 'failed';
            item.error = e.message;
            await this.put(item);
          }
        }
      } while (this.again);
    } finally {
      this.busy = false;
    }
    if (state.conv) refreshOutboxBubbles();
  },
};
window.addEventListener('online', () => outbox.flush());
setInterval(() => outbox.flush(), 15000);

// ---------- start ----------

async function boot() {
  try { state.house = (await api('/api/house')).house; } catch (e) { state.house = ''; }
  document.title = 'batnet · ' + state.house;
  try { await loadMe(); } catch (e) { state.me = null; }
  route();
  outbox.flush();
}

function route() {
  if (state.bedside) return bedside();
  if (!state.me) return landing();
  if (state.me.status !== 'approved') return pending();
  listen();
  if (state.conv) return conversation();
  if (state.tab === 'people') return people();
  if (state.tab === 'settings') return settings();
  if (state.tab === 'help') return help();
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
      await loadMe();
      route();
      outbox.flush();
    } catch (x) {
      err.textContent = x.retryAfter ? x.message + t('retryIn', x.retryAfter) : x.message;
    }
  };
  show(
    h('header', {}, h('h1', {}, 'batnet · ' + state.house, h('span', { class: 'sub' }, t('tagline')))),
    h('form', { class: 'pad', onsubmit: submit },
      h('h2', {}, mode === 'signup' ? t('createAccount') : t('signIn')),
      h('label', {}, t('yourName'), name),
      h('label', {}, mode === 'signup' ? t('choosePin') : t('yourPin'), pin),
      err,
      h('button', { class: 'btn', type: 'submit' }, mode === 'signup' ? t('askToJoin') : t('signIn')),
      h('button', { class: 'btn ghost', type: 'button', onclick: () => landing(mode === 'signup' ? 'login' : 'signup') },
        mode === 'signup' ? t('haveAccount') : t('createAccount')),
      h('p', { class: 'muted' }, mode === 'signup' ? t('approvesNew', state.house) : t('anyHouse')),
      h('button', { class: 'btn ghost', type: 'button', onclick: () => help(true) }, t('help'))));
}

function pending() {
  const denied = state.me.status === 'denied';
  const home = state.me.house || state.house;
  show(
    h('header', {}, h('h1', {}, 'batnet · ' + state.house)),
    h('div', { class: 'pad' },
      h('h2', {}, denied ? t('notApproved') : t('waiting')),
      h('p', {}, denied ? t('deniedText', state.me.name, home) : t('waitingText', state.me.name, home)),
      h('button', { class: 'btn ghost', onclick: logout }, t('signOut'))));
  clearTimeout(state.pendingTimer);
  if (denied) return;
  state.pendingTimer = setTimeout(async () => {
    try { await loadMe(); } catch (e) { state.me = null; }
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
  state.tab = 'chats';
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
    if (m.alert) pickAlert(m);
    if (!mine) beep(m.alert ? 'alert' : m.prio >= 8 ? 'high' : 'normal');
    if (state.bedside) { if (m.alert) bedside(); return; } // the dim screen shows the alert's text too (B28)
    if (state.conv === key) appendBubble(m); // add it in place: rebuilding would wipe what the person is typing
    else {
      if (!mine) state.unread[key] = (state.unread[key] || 0) + 1;
      if (!state.conv) route();
    }
  });
  state.events.addEventListener('state', () => { if (!state.conv && !state.bedside) route(); });
  state.events.addEventListener('open', () => {
    state.retry = 1000;
    outbox.flush();
    if (!state.conv && !state.bedside) route(); // catch up on anything missed while disconnected
  });
  state.events.addEventListener('error', () => {
    if (state.events && state.events.readyState === EventSource.CLOSED) {
      // the browser gave up (e.g. the Pi restarted and the session is gone): check, then reconnect with backoff
      state.events = null;
      setTimeout(async () => {
        try { await loadMe(); } catch (e) { state.me = null; }
        state.retry = Math.min(state.retry * 2, 60000);
        route();
      }, state.retry);
    }
  });
}

// ---------- signed in ----------

function tabs() {
  const tab = (k, label) => h('button', { 'aria-current': state.tab === k ? 'page' : false, onclick: () => { state.tab = k; state.conv = null; route(); } }, label);
  return h('nav', { class: 'tabs' }, tab('chats', t('chats')), tab('people', t('people')), tab('settings', t('settings')));
}

function alertBar() {
  const a = state.alert;
  if (!a) return null;
  const dismiss = () => { store('dismissedAlert', a.id); state.alert = null; route(); };
  return h('div', { class: 'alertbar', role: 'alert' },
    h('span', { class: 'main' }, h('b', {}, t('alertBanner') + ' · '), a.text, ' ', h('span', { class: 'small' }, time(a.ts))),
    h('button', { class: 'close', type: 'button', 'aria-label': t('dismiss'), title: t('dismiss'), onclick: dismiss }, '×'));
}

async function chats() {
  let convs = [];
  let dir = { groups: [] };
  try {
    [convs, dir] = await Promise.all([api('/api/conversations'), api('/api/directory')]);
  } catch (e) { if (e.status === 401) return logout(); }
  for (const c of convs) if (c.last && c.last.alert) pickAlert(c.last);
  const keys = new Set(convs.map((c) => c.key));
  const rows = convs.map((c) => convRow(c.key, c.key === 'everyone' ? t('everyone') : c.name, c.last));
  if (!keys.has('everyone')) rows.unshift(convRow('everyone', t('everyone'), null));
  for (const g of dir.groups) if (!keys.has('group:' + g.id)) rows.push(convRow('group:' + g.id, g.name, null));
  show(
    h('header', {}, h('h1', {}, t('chats'), h('span', { class: 'sub' }, state.me.name + ' · ' + state.house))),
    alertBar(),
    h('div', { class: 'grow' }, rows),
    tabs());
}

function convRow(key, name, last) {
  const n = state.unread[key] || 0;
  return h('button', { class: 'row', onclick: () => { state.conv = key; state.unread[key] = 0; conversation(); } },
    h('span', { class: 'av' }, key === 'everyone' ? 'All' : initials(name)),
    h('span', { class: 'main' }, h('span', { class: 'title' }, name), h('div', { class: 'last' }, last ? last.text : t('noMessages'))),
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
  const title = state.conv === 'everyone' ? t('everyone')
    : state.conv.startsWith('group:') ? ((dir.groups.find((g) => 'group:' + g.id === state.conv) || {}).name || state.conv)
    : (names[state.conv.slice(3)] || t('directMessage'));
  const list = h('div', { class: 'msgs' }, msgs.map((m) => bubble(m, names)));
  const err = h('p', { class: 'err', role: 'alert' });
  const text = h('input', { type: 'text', placeholder: t('message'), maxlength: 2000, 'aria-label': t('message') });
  const locBtn = h('button', { type: 'button', class: 'icon' + (state.loc ? ' on' : ''), 'aria-pressed': String(state.loc), title: t('attachLoc'), 'aria-label': t('attachLoc'),
    onclick: () => { state.loc = !state.loc; locBtn.classList.toggle('on', state.loc); locBtn.setAttribute('aria-pressed', String(state.loc)); } }, '📍');
  const prio = h('select', { 'aria-label': 'Priority', onchange: (e) => { state.prio = e.target.value; if (state.prio === 'high') { state.loc = true; locBtn.classList.add('on'); } } },
    h('option', { value: 'normal' }, t('normal')), h('option', { value: 'high' }, t('high')), h('option', { value: 'low' }, t('low')));
  prio.value = state.prio;
  const send = async (e) => {
    e.preventDefault();
    if (!text.value.trim()) return;
    err.textContent = '';
    const item = { clientId: newClientId(), conv: state.conv, text: text.value.trim(), prio: state.prio };
    if (state.loc) {
      try { item.loc = await position(); } catch (x) { err.textContent = x.message; return; }
    }
    text.value = '';
    state.prio = 'normal';
    state.loc = false;
    prio.value = 'normal';
    locBtn.classList.remove('on');
    await outbox.add(item);
    refreshOutboxBubbles();
  };
  show(
    h('header', {}, h('button', { class: 'back', 'aria-label': t('back'), onclick: () => { state.conv = null; route(); } }, '‹'), h('h1', {}, title)),
    list,
    err,
    h('form', { class: 'compose', onsubmit: send }, prio, locBtn, text, h('button', { class: 'icon on', type: 'submit', 'aria-label': t('send') }, '➤')));
  await refreshOutboxBubbles();
  list.scrollTop = list.scrollHeight;
}

// the outbox's items for this conversation, shown as bubbles marked "waiting to send"
async function refreshOutboxBubbles() {
  const list = document.querySelector('.msgs');
  if (!list) return;
  const items = await outbox.all(); // read first, then redraw in one go: two refreshes at once mustn't draw twice
  for (const el of list.querySelectorAll('.outbox')) el.remove();
  for (const item of items) {
    if (item.conv !== state.conv || !outbox.mine(item)) continue;
    const failed = item.status === 'failed';
    list.append(h('div', { class: 'bubble me outbox' + (failed ? ' failed' : '') },
      h('div', {}, item.text),
      h('div', { class: 'meta' }, failed ? t('failed') + (item.error ? ' · ' + item.error : '') : t('waitingToSend')),
      failed ? h('div', { class: 'acts' },
        h('button', { class: 'btn ghost', type: 'button', onclick: async () => {
          item.status = 'waiting'; item.prio = item.prio === 'high' ? 'normal' : item.prio; delete item.error;
          await outbox.put(item); refreshOutboxBubbles(); outbox.flush();
        } }, item.prio === 'high' ? t('sendAsNormal') : t('sendAgain')),
        h('button', { class: 'btn ghost', type: 'button', onclick: async () => { await outbox.remove(item.clientId); refreshOutboxBubbles(); } }, t('delete'))) : null));
  }
}

function appendBubble(m) {
  const list = document.querySelector('.msgs');
  if (!list) return conversation();
  const atBottom = list.scrollTop + list.clientHeight >= list.scrollHeight - 30;
  list.insertBefore(bubble(m, state.names), list.querySelector('.outbox'));
  if (atBottom) list.scrollTop = list.scrollHeight;
}

function bubble(m, names) {
  const mine = m.from === state.me.account;
  const cls = 'bubble' + (mine ? ' me' : '') + (m.alert ? ' alert' : m.prio >= 8 ? ' high' : '');
  return h('div', { class: cls },
    m.alert ? h('div', { class: 'who' }, t('alertFrom', names[m.from] || t('theAdmin')))
      : !mine && !m.audience.startsWith('dm:') ? h('div', { class: 'who' }, names[m.from] || t('someone')) : null,
    h('div', {}, m.text),
    m.loc ? h('div', { class: 'loc' }, '📍 ' + m.loc.lat.toFixed(4) + ', ' + m.loc.lon.toFixed(4) + ' (±' + Math.round(m.loc.acc) + ' m, ' + (m.loc.source === 'node' ? t('houseGps') : t('phoneGps')) + ')') : null,
    h('div', { class: 'meta' }, (m.prio >= 8 && !m.alert ? t('high') + ' · ' : '') + time(m.ts)));
}

function position() {
  return new Promise((resolve, reject) => {
    if (!navigator.geolocation) return reject(new Error(t('noGeo')));
    navigator.geolocation.getCurrentPosition(
      (p) => resolve({ lat: p.coords.latitude, lon: p.coords.longitude, acc: Math.min(10000, p.coords.accuracy || 0), source: 'phone' }),
      () => reject(new Error(t('noLocation'))),
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
  rows.push(h('div', { class: 'section' }, t('groups')));
  for (const g of dir.groups) {
    rows.push(h('div', { class: 'row' },
      h('span', { class: 'av' }, initials(g.name)),
      h('button', { class: 'main title plain', onclick: () => { state.conv = 'group:' + g.id; conversation(); } }, g.name),
      h('button', { class: 'btn ghost', onclick: () => act(async () => { await api('/api/groups/join', { group: g.id }); people(); }) }, t('join')),
      h('button', { class: 'btn ghost', onclick: () => act(async () => { await api('/api/groups/leave', { group: g.id }); people(); }) }, t('leave'))));
  }
  show(h('header', {}, h('h1', {}, t('people'), h('span', { class: 'sub' }, t('everyoneIn')))), h('div', { class: 'grow' }, rows), tabs());
}

function setRow(title, sub, value, onclick) {
  return h('button', { class: 'row', onclick }, h('span', { class: 'main' }, h('span', { class: 'title' }, title), sub ? h('div', { class: 'last' }, sub) : null), h('span', { class: 'muted' }, value));
}

const nextLang = { auto: 'en', en: 'de', de: 'auto' };

async function settings() {
  const me = state.me;
  const toggle = (field) => () => act(async () => {
    await api('/api/settings', { [field]: !me[field] });
    await loadMe();
    settings();
  });
  const parts = [
    h('div', { class: 'section' }, t('you')),
    setRow(me.name + ' · ' + (me.house || state.house), null, t('signOut'), logout),
    h('div', { class: 'section' }, t('notifications')),
    setRow(t('speaker'), t('speakerSub'), me.speaker ? t('on') : t('off'), toggle('speaker')),
    setRow(t('quiet'), t('quietSub'), me.quietHours ? t('on') : t('off'), toggle('quietHours')),
    setRow(t('bedside'), t('bedsideSub'), '›', () => { state.bedside = { lock: null }; bedside(); }),
    h('div', { class: 'pad' }, h('p', { class: 'muted' }, t('locked'))),
    h('div', { class: 'section' }, t('display')),
    setRow(t('language'), null, { en: 'English', de: 'Deutsch' }[store('lang')] || t('auto'), () => {
      store('lang', nextLang[store('lang') || 'auto']);
      settings();
    }),
    setRow(t('largeText'), null, store('large') === '1' ? t('on') : t('off'), () => { store('large', store('large') === '1' ? '0' : '1'); settings(); }),
    setRow(t('help'), null, '›', () => { state.tab = 'help'; help(); }),
  ];
  if (me.admin) {
    let pend = [];
    try { pend = await api('/api/admin/pending'); } catch (e) { pend = []; }
    parts.push(h('div', { class: 'section' }, t('joinRequests')));
    if (!pend.length) parts.push(h('div', { class: 'pad' }, h('p', { class: 'muted' }, t('noRequests'))));
    for (const p of pend) {
      parts.push(h('div', { class: 'row' }, h('span', { class: 'av' }, initials(p.name)), h('span', { class: 'main title' }, p.name),
        h('button', { class: 'btn', onclick: () => act(async () => { await api('/api/admin/approve', { account: p.id }); settings(); }) }, t('approve')),
        h('button', { class: 'btn ghost', onclick: () => act(async () => { await api('/api/admin/deny', { account: p.id }); settings(); }) }, t('deny'))));
    }
    const alertText = h('textarea', { rows: 2, maxlength: 2000, 'aria-label': t('alertEveryone') });
    const alertMsg = h('p', { class: 'muted', role: 'status' });
    parts.push(h('div', { class: 'section' }, t('alertEveryone')),
      h('div', { class: 'pad' }, h('p', { class: 'muted' }, t('alertHelp')), alertText, alertMsg,
        h('button', { class: 'btn danger', onclick: async () => {
          if (!alertText.value.trim()) return;
          try {
            await api('/api/post', { conv: 'everyone', text: alertText.value, prio: 'alert', clientId: newClientId() });
            alertText.value = '';
            alertMsg.textContent = t('alertSent');
          } catch (x) { alertMsg.textContent = x.message; }
        } }, t('sendAlert'))));
  }
  show(h('header', {}, h('h1', {}, t('settings'))), h('div', { class: 'grow' }, parts), tabs());
}

function help(signedOut) {
  show(
    h('header', {}, h('button', { class: 'back', 'aria-label': t('back'), onclick: () => { if (signedOut) landing(); else { state.tab = 'settings'; route(); } } }, '‹'), h('h1', {}, t('help'))),
    h('div', { class: 'pad' }, t('helpText').map((p) => h('p', {}, p))),
    signedOut ? null : tabs());
}

// ---------- bedside mode: screen on and dim, so the phone can ring at night ----------

async function keepAwake() {
  if (!state.bedside) return;
  try {
    if (navigator.wakeLock) state.bedside.lock = await navigator.wakeLock.request('screen');
  } catch (e) { state.bedside.lock = null; }
}
document.addEventListener('visibilitychange', () => { if (state.bedside && document.visibilityState === 'visible') keepAwake(); });

function bedside() {
  const clock = h('div', { class: 'clock' });
  const tick = () => { clock.textContent = new Date().toLocaleTimeString(lang(), { hour: '2-digit', minute: '2-digit' }); };
  tick();
  clearInterval(state.bedsideTimer);
  state.bedsideTimer = setInterval(tick, 10000);
  const leave = () => {
    clearInterval(state.bedsideTimer);
    if (state.bedside && state.bedside.lock) state.bedside.lock.release().catch(() => {});
    state.bedside = null;
    route();
  };
  show(h('div', { class: 'bedside', role: 'button', 'aria-label': t('tapToLeave'), onclick: leave },
    clock, h('p', {}, t('bedsideOn')), navigator.wakeLock ? null : h('p', {}, t('bedsideNoLock')), h('p', { class: 'small' }, t('tapToLeave')), alertBar()));
  keepAwake();
}

boot();
