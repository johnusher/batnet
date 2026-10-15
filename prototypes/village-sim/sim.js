// batnet village simulator - the model. No DOM; runs in the browser and in node.
// A discrete-event simulation of houses on a batman-adv WiFi mesh and Meshtastic
// LoRa, following knowledge/decisions/0007 (replicated log) and 0008 (LoRa budget).
// Every number here is an estimate; see the assumptions list at the bottom.
(function (root) {
  'use strict';

  // ---------- radio and hardware parameters ----------
  const PRESETS = {
    ShortFast: { sf: 7, bw: 250e3, cr: 1, range: 1200, label: 'ShortFast · SF7' },
    MediumFast: { sf: 9, bw: 250e3, cr: 1, range: 2000, label: 'MediumFast · SF9' },
    LongFast: { sf: 11, bw: 250e3, cr: 1, range: 3000, label: 'LongFast · SF11' },
  };
  const WIFI_RANGE = { none: 0, builtin: 120, dongle: 350, pole: 450 };
  const P2P_RANGE = 3500;
  const PI = {
    'Zero W': { phones: 4, stt: 'none', builtinRange: 80, note: 'Single core. Fine as a relay or LoRa gateway; too slow to serve a house.' },
    'Zero 2 W': { phones: 8, stt: 'basic', builtinRange: 100, note: 'Small houses only. Speech-to-text limited to small models.' },
    'Pi 3': { phones: 15, stt: 'basic', builtinRange: 120, note: 'Works as a house for a family. Small speech models.' },
    'Pi 4': { phones: 40, stt: 'good', builtinRange: 120, note: 'Good house node. Hardware video encoding.' },
    'Pi 5': { phones: 60, stt: 'best', builtinRange: 120, note: 'Best house node and speech-to-text host.' },
  };
  const DUTY_LIMIT_MS = 360000; // 10% of an hour, EU 869.4-869.65 MHz sub-band
  const HOUR = 3600000, MIN = 60000;
  const LORA_OVERHEAD = 26; // Meshtastic header + protobuf + encryption, bytes
  const FRAG_MAX = 200;
  const LORA_TEXT_MAX = 600;

  // house names: seven friends first, then surnames from Hitchcock films, Bond villains and British prime ministers
  const SURNAMES = ['Usher', 'Shemie', 'Dittrich', 'Green', 'Willett', 'Seigner', 'Chasserot', 'Bates', 'Crane', 'Ferguson', 'Elster', 'Thornhill', 'Kendall', 'Jefferies', 'Fremont', 'Danvers', 'Daniels', 'Goldfinger', 'Walpole', 'Blofeld', 'Pitt', 'Klebb', 'Peel', 'Largo', 'Disraeli', 'Scaramanga', 'Gladstone', 'Stromberg', 'Salisbury', 'Drax', 'Balfour', 'Khan', 'Asquith', 'Zorin', 'Baldwin', 'Sanchez', 'MacDonald', 'Trevelyan', 'Chamberlain', 'Onatopp', 'Churchill', 'Carver', 'Attlee', 'King', 'Eden', 'Graves', 'Macmillan', 'Silva', 'Wilson', 'Safin', 'Heath', 'Grant', 'Callaghan', 'Thatcher', 'Major', 'Blair', 'Brown', 'May', 'Starmer', 'Sunak'];
  // the first ten from Hitchcock films, then Bond villains and British prime ministers, alternating
  const FIRST = ['Norman Bates', 'Marion Crane', 'Scottie Ferguson', 'Madeleine Elster', 'Roger Thornhill', 'Eve Kendall', 'L.B. Jefferies', 'Lisa Fremont', 'Mrs Danvers', 'Melanie Daniels', 'Auric Goldfinger', 'Robert Walpole', 'Ernst Blofeld', 'William Pitt', 'Dr. No', 'Robert Peel', 'Rosa Klebb', 'Benjamin Disraeli', 'Emilio Largo', 'William Gladstone', 'Red Grant', 'Lord Salisbury', 'Scaramanga', 'Arthur Balfour', 'Oddjob', 'Herbert Asquith', 'Karl Stromberg', 'Lloyd George', 'Jaws', 'Stanley Baldwin', 'Hugo Drax', 'Ramsay MacDonald', 'Kamal Khan', 'Neville Chamberlain', 'Max Zorin', 'Winston Churchill', 'Franz Sanchez', 'Clement Attlee', 'Alec Trevelyan', 'Anthony Eden', 'Xenia Onatopp', 'Harold Macmillan', 'Elliot Carver', 'Harold Wilson', 'Elektra King', 'Edward Heath', 'Renard', 'James Callaghan', 'Gustav Graves', 'Margaret Thatcher', 'Le Chiffre', 'John Major', 'Dominic Greene', 'Tony Blair', 'Raoul Silva', 'Gordon Brown', 'Mr. Big', 'Theresa May', 'Lyutsifer Safin', 'Keir Starmer'];
  const GROUPS = [
    { id: 'g:medical', name: 'Medical' },
    { id: 'g:food', name: 'Food' },
    { id: 'g:water', name: 'Water' },
    { id: 'g:childcare', name: 'Childcare' },
    { id: 'g:repairs', name: 'Repairs' },
  ];
  const PHRASES = {
    everyone: ['Water point at the church runs again until 18:00.', 'Anyone have spare AA batteries? We can swap candles.', 'Road to the bridge is blocked by a fallen tree, use the farm track.', 'Community kitchen at the school from 12:00, bring a bowl.', 'Generator at the fire station charges phones 9-11.', 'Radio says power back in some districts tomorrow, nothing official yet.', 'Lost dog, brown, answers to Pepper. Last seen near the pond.', 'Please boil tap water until further notice.'],
    'g:medical': ['Insulin needs to stay cold - who has a running fridge?', 'First aid kit restocked, two bandage rolls left.', 'Can someone check on the older couple at the end of the lane?', 'Nurse available at the school hall 14:00-16:00.'],
    'g:food': ['Bread here, 12 loaves, come by before 10.', 'We have eggs to share, about two dozen.', 'Freezer is thawing, cooking it all tonight, come eat.', 'Potatoes and onions to trade for rice.'],
    'g:water': ['Rain barrels full, bring containers.', 'Filter pump works, can do 20 litres an hour.', 'Spring by the old mill tested clear.'],
    'g:childcare': ['Can take three kids this afternoon, games and lunch.', 'Story time at 16:00 by the fire pit.', 'Need someone to watch my two for an hour.'],
    'g:repairs': ['Roof tarp needed on the north side, anyone with a ladder?', 'Chainsaw available, have fuel for an hour.', 'Fixed the hand pump, it needed a new washer.'],
    dm: ['Are you OK?', 'Can you bring the ladder tomorrow?', 'Kids are fine, staying in.', 'Coming over at 3.', 'Do you still have candles?', 'Thanks for the water!', 'Is your phone charged? Call me when you can.', 'Meet at the crossroads at noon?'],
    public: ['Anyone on this channel near the reservoir?', 'Testing from the hill, how do you hear me?', 'Checkpoint at the main road, be patient.', 'Shelter open at the sports hall.'],
  };

  // ---------- small utilities ----------
  function mulberry32(seed) {
    let a = seed >>> 0;
    return function () {
      a = (a + 0x6D2B79F5) >>> 0;
      let t = a;
      t = Math.imul(t ^ (t >>> 15), t | 1);
      t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  }
  const dist = (a, b) => Math.hypot(a.x - b.x, a.y - b.y);
  const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));

  // LoRa time on air (Semtech formula), explicit header, CRC on, 16-symbol preamble.
  function airtimeMs(payloadBytes, p) {
    const tsym = (Math.pow(2, p.sf) / p.bw) * 1000;
    const de = tsym > 16 ? 1 : 0;
    const num = 8 * payloadBytes - 4 * p.sf + 28 + 16;
    const sym = 8 + Math.max(Math.ceil(num / (4 * (p.sf - 2 * de))) * (p.cr + 4), 0);
    return (16 + 4.25) * tsym + sym * tsym;
  }

  class Heap {
    constructor() { this.a = []; }
    get size() { return this.a.length; }
    peek() { return this.a[0]; }
    push(e) {
      const a = this.a; a.push(e);
      let i = a.length - 1;
      while (i > 0) { const p = (i - 1) >> 1; if (a[p].t <= a[i].t) break; [a[p], a[i]] = [a[i], a[p]]; i = p; }
    }
    pop() {
      const a = this.a; if (!a.length) return null;
      const top = a[0], last = a.pop();
      if (a.length) {
        a[0] = last; let i = 0; const n = a.length;
        for (;;) {
          const l = 2 * i + 1, r = l + 1; let m = i;
          if (l < n && a[l].t < a[m].t) m = l;
          if (r < n && a[r].t < a[m].t) m = r;
          if (m === i) break;
          [a[i], a[m]] = [a[m], a[i]]; i = m;
        }
      }
      return top;
    }
  }

  const SCENARIOS = {
    village: { label: 'Village · 12 houses, mixed links', houses: 12, width: 1400, height: 900, hamlets: 3, spread: 140, loraShare: 0.5, dongleShare: 0.5, relays: 2, meshUsers: 3, sizeMin: 1, sizeMax: 6, chat: 3, preset: 'LongFast', outages: false },
    street: { label: 'Street · 6 houses, all WiFi', houses: 6, width: 500, height: 200, hamlets: 1, spread: 120, loraShare: 0.2, dongleShare: 1, relays: 0, meshUsers: 1, sizeMin: 2, sizeMax: 5, chat: 4, preset: 'LongFast', outages: false },
    farms: { label: 'Farms · 8 houses, LoRa only', houses: 8, width: 3200, height: 2200, hamlets: 8, spread: 30, loraShare: 1, dongleShare: 0.3, relays: 0, meshUsers: 4, sizeMin: 2, sizeMax: 7, chat: 3, preset: 'LongFast', outages: false },
    stress: { label: 'Stress · 40 houses, chatty', houses: 40, width: 1800, height: 1200, hamlets: 6, spread: 160, loraShare: 0.4, dongleShare: 0.5, relays: 4, meshUsers: 6, sizeMin: 1, sizeMax: 8, chat: 8, preset: 'LongFast', outages: false },
    storm: { label: 'Storm · village with power cuts', houses: 14, width: 1400, height: 900, hamlets: 3, spread: 150, loraShare: 0.5, dongleShare: 0.5, relays: 2, meshUsers: 3, sizeMin: 1, sizeMax: 6, chat: 4, preset: 'LongFast', outages: true },
  };

  // ---------- the simulation ----------
  function createSim(userCfg) {
    const cfg = Object.assign({ seed: 7, piMix: { 'Zero 2 W': 0.15, 'Pi 3': 0.2, 'Pi 4': 0.4, 'Pi 5': 0.25 }, rebroadcast: 1, meshChatter: 1 }, SCENARIOS.village, userCfg || {});
    const rnd = mulberry32(cfg.seed);
    const pick = (arr) => arr[Math.floor(rnd() * arr.length)];
    const randInt = (lo, hi) => lo + Math.floor(rnd() * (hi - lo + 1));
    const gauss = () => { let u = 0, v = 0; while (u === 0) u = rnd(); while (v === 0) v = rnd(); return Math.sqrt(-2 * Math.log(u)) * Math.cos(2 * Math.PI * v); };
    const expo = (ratePerMs) => -Math.log(1 - rnd()) / ratePerMs;

    const S = {
      cfg, now: 8 * HOUR, start: 8 * HOUR, nodes: [], houses: [], people: [], personById: new Map(), groups: GROUPS.map((g) => Object.assign({ members: new Set() }, g)),
      links: [], clusters: [], msgs: [], msgById: new Map(), queue: [], chan: { busyUntil: 0, log: [], pumpAt: Infinity },
      heap: new Heap(), nextMsgId: 1, nextPid: 0, deliveries: [], series: { util: [], queue: [], pending: [] }, uiEvents: [], airLog: [], signups: [],
      counters: { sent: 0, loraTx: 0, loraRetries: 0, truncated: 0, bgAir: 0, msgAir: 0 },
    };
    const preset = () => PRESETS[cfg.preset] || PRESETS.LongFast;
    const schedule = (t, fn) => S.heap.push({ t, fn });

    // ----- village generation -----
    function pickPi() {
      const r = rnd(); let acc = 0;
      for (const [k, w] of Object.entries(cfg.piMix)) { acc += w; if (r < acc) return k; }
      return 'Pi 4';
    }
    function addPerson(house, name) {
      const p = { id: 'p' + S.nextPid, name: name || FIRST[S.nextPid % FIRST.length], houseId: house.id, groups: new Set() };
      for (const g of S.groups) if (rnd() < 0.25) { g.members.add(p.id); p.groups.add(g.id); }
      S.nextPid++; S.people.push(p); S.personById.set(p.id, p); house.people.push(p.id);
      return p;
    }
    const centres = [];
    for (let i = 0; i < cfg.hamlets; i++) centres.push({ x: cfg.width * (0.12 + 0.76 * rnd()), y: cfg.height * (0.15 + 0.7 * rnd()) });
    for (let i = 0; i < cfg.houses; i++) {
      const c = centres[i % centres.length];
      const name = SURNAMES[i % SURNAMES.length] + (i >= SURNAMES.length ? ' ' + Math.floor(i / SURNAMES.length + 1) : '');
      const h = {
        id: 'h' + i, kind: 'house', name, x: clamp(c.x + gauss() * cfg.spread, 20, cfg.width - 20), y: clamp(c.y + gauss() * cfg.spread, 20, cfg.height - 20),
        pi: pickPi(), wifi: rnd() < cfg.dongleShare ? 'dongle' : 'builtin', lora: rnd() < cfg.loraShare, p2p: null,
        online: true, people: [], has: new Set(), air: [], cluster: -1, offlineSince: null,
      };
      S.nodes.push(h); S.houses.push(h);
      const size = randInt(cfg.sizeMin, cfg.sizeMax);
      for (let k = 0; k < size; k++) addPerson(h);
      h.admin = h.people[0];
    }
    if (!S.houses.some((h) => h.lora) && S.houses.length) S.houses[0].lora = true;
    // relays: Zero 2 W on a pole between two houses that the built-in radios cannot bridge
    for (let i = 0, tries = 0; i < cfg.relays && tries < 200; tries++) {
      const a = pick(S.houses), b = pick(S.houses);
      if (a === b) continue;
      const d = dist(a, b);
      if (d < 180 || d > 800) continue;
      S.nodes.push({ id: 'r' + i, kind: 'relay', name: 'Relay ' + (i + 1), x: (a.x + b.x) / 2, y: (a.y + b.y) / 2, pi: 'Zero 2 W', wifi: 'pole', lora: false, online: true, has: new Set(), air: [], cluster: -1 });
      i++;
    }
    for (let i = 0; i < cfg.meshUsers; i++) {
      S.nodes.push({ id: 'm' + i, kind: 'mesh', name: FIRST[(i * 7 + 3) % FIRST.length] + "'s Meshtastic", x: cfg.width * rnd(), y: cfg.height * rnd(), lora: true, online: true, air: [], cluster: -1, heard: [], relayed: 0 });
    }
    const nodeById = new Map(S.nodes.map((n) => [n.id, n]));
    S.nodeById = nodeById;

    // ----- topology -----
    function wifiRange(n) {
      if (n.kind === 'mesh') return 0;
      if (n.wifi === 'builtin') return (PI[n.pi] || PI['Pi 4']).builtinRange;
      return WIFI_RANGE[n.wifi] || 0;
    }
    function recompute() {
      const bat = S.nodes.filter((n) => n.kind !== 'mesh');
      const adj = new Map(bat.map((n) => [n.id, []]));
      S.links = [];
      for (let i = 0; i < bat.length; i++) {
        for (let j = i + 1; j < bat.length; j++) {
          const a = bat[i], b = bat[j];
          const d = dist(a, b);
          const r = Math.min(wifiRange(a), wifiRange(b));
          let type = null, q = 0;
          if (r > 0 && d <= r) { type = 'wifi'; q = clamp(1 - Math.pow(d / r, 2), 0.05, 1); }
          else if ((a.p2p === b.id || b.p2p === a.id) && d <= P2P_RANGE) { type = 'p2p'; q = 0.9; }
          if (!type) continue;
          const up = a.online && b.online;
          S.links.push({ a, b, d, q, type, up, tq: Math.round(255 * q) });
          if (up) { adj.get(a.id).push(b); adj.get(b.id).push(a); }
        }
      }
      S.adj = adj;
      S.clusters = [];
      for (const n of S.nodes) n.cluster = -1;
      for (const n of bat) {
        if (!n.online || n.cluster >= 0) continue;
        const id = S.clusters.length, members = [], stack = [n];
        n.cluster = id;
        while (stack.length) {
          const x = stack.pop(); members.push(x);
          for (const y of adj.get(x.id)) if (y.cluster < 0) { y.cluster = id; stack.push(y); }
        }
        members.sort((p, q) => (p.id < q.id ? -1 : 1));
        const gw = members.filter((m) => m.lora).sort((p, q) => (p.id < q.id ? -1 : 1))[0] || null;
        S.clusters.push({ id, members, gateway: gw, letter: String.fromCharCode(65 + (id % 26)) + (id >= 26 ? Math.floor(id / 26) : '') });
      }
      syncClusters();
    }
    function loraReach(a, b) {
      if (!a.online || !b.online || !a.lora || !b.lora || a === b) return null;
      const R = preset().range, d = dist(a, b);
      if (d <= R) return { hops: 1, d };
      for (const c of S.nodes) {
        if (c === a || c === b || !c.lora || !c.online) continue;
        if (dist(a, c) <= R && dist(c, b) <= R) return { hops: 2, d: Math.max(dist(a, c), dist(c, b)) };
      }
      return null;
    }
    const lossAt = (d) => clamp(0.02 + 0.3 * Math.pow(d / preset().range, 3), 0, 0.6);

    function bfsHops(src) {
      const hops = new Map([[src.id, 0]]), qu = [src];
      while (qu.length) {
        const x = qu.shift();
        for (const y of S.adj.get(x.id) || []) if (!hops.has(y.id)) { hops.set(y.id, hops.get(x.id) + 1); qu.push(y); }
      }
      return hops;
    }

    // ----- the replicated log -----
    function recordDelivery(m, house, t, path) {
      m.deliv.set(house.id, { t, path });
      if (house.id !== m.fromHouse) S.deliveries.push({ t, lat: t - m.t, path, prio: m.prio || 'normal' });
    }
    function arrive(entry, m, t, path) {
      const cl = S.clusters[entry.cluster];
      if (!cl) return;
      const hops = bfsHops(entry);
      for (const k of cl.members) {
        if (k.has.has(m.id)) continue;
        const tt = k === entry ? t : t + 40 + 25 * (hops.get(k.id) || 1);
        k.has.add(m.id);
        if (k.kind === 'house' && m.targets.has(k.id) && !m.deliv.has(k.id)) {
          const p = k === entry ? path : path === 'local' ? 'wifi' : path;
          recordDelivery(m, k, tt, p);
        }
      }
      if (path !== 'lora' && cl.gateway && needsLoRa(m, cl)) enqueueMsg(cl.gateway, m, false);
    }
    // Send over LoRa only when a target house sits in another online cluster whose gateway our gateway can reach.
    function needsLoRa(m, cl) {
      if (m.kind === 'public') return !m.loraBy.size;
      const gw = cl.gateway;
      if (!gw || m.loraBy.has(gw.id)) return false;
      for (const hid of m.targets) {
        const h = nodeById.get(hid);
        if (!h.online || h.cluster === cl.id || h.has.has(m.id)) continue;
        const other = S.clusters[h.cluster];
        if (other && other.gateway && loraReach(gw, other.gateway)) return true;
      }
      return false;
    }
    function syncClusters() {
      for (const cl of S.clusters) {
        const union = new Set();
        for (const k of cl.members) for (const id of k.has) union.add(id);
        for (const k of cl.members) {
          for (const id of union) {
            if (k.has.has(id)) continue;
            k.has.add(id);
            const m = S.msgById.get(id);
            if (m && k.kind === 'house' && m.targets.has(k.id) && !m.deliv.has(k.id)) recordDelivery(m, k, S.now + 400, 'sync');
          }
        }
        if (cl.gateway) for (const id of union) { const m = S.msgById.get(id); if (m && m.via !== 'lora' && needsLoRa(m, cl)) enqueueMsg(cl.gateway, m, false); }
      }
    }

    function loraPayload(m) {
      const text = m.text.length > LORA_TEXT_MAX ? m.text.slice(0, LORA_TEXT_MAX) : m.text;
      const truncated = m.text.length > LORA_TEXT_MAX;
      let bytes;
      if (m.kind === 'everyone' || m.kind === 'public') bytes = 18 + text.length; // plain Meshtastic text "Norman Bates·Usher: ..."
      else bytes = 12 + Math.ceil(text.length * 0.65); // compact envelope + compressed text
      const frags = Math.ceil(bytes / FRAG_MAX);
      let dur = 0;
      for (let i = 0; i < frags; i++) dur += airtimeMs(Math.min(FRAG_MAX, bytes - i * FRAG_MAX) + LORA_OVERHEAD, preset());
      return { bytes, frags, dur, truncated };
    }
    function enqueueMsg(gw, m, retry) {
      if (m.inQueue.has(gw.id)) return;
      const pl = loraPayload(m);
      if (pl.truncated && !m.truncated) { m.truncated = true; S.counters.truncated++; }
      m.loraBy.add(gw.id); m.inQueue.add(gw.id);
      S.queue.push({ kind: 'msg', gw, m, dur: pl.dur, frags: pl.frags, retry, t: S.now });
      if (retry) S.counters.loraRetries++;
      pump();
    }
    function enqueueBg(node, kind, bytes) {
      S.queue.push({ kind, gw: node, dur: airtimeMs(bytes + LORA_OVERHEAD, preset()), frags: 1, t: S.now });
      pump();
    }
    function dutyAllowAt(node, dur) {
      const cut = S.now - HOUR;
      while (node.air.length && node.air[0].t < cut) node.air.shift();
      let used = 0;
      for (const e of node.air) used += e.dur;
      if (used + dur <= DUTY_LIMIT_MS) return S.now;
      for (const e of node.air) { used -= e.dur; if (used + dur <= DUTY_LIMIT_MS) return e.t + HOUR; }
      return S.now + HOUR;
    }
    // Priority 0-9 for the radio queue. People pick Low / Normal / High (house admins also Alert);
    // the network adds: direct messages +1, public posts -1, resends -1, and waiting raises a message
    // one step per 20 minutes, never past 7, so Normal can't age into High but Low can't starve.
    const PRIO_BASE = { low: 2, normal: 5, high: 8, alert: 9 };
    function effPrio(it) {
      let p;
      if (it.kind === 'msg') {
        const m = it.m;
        p = PRIO_BASE[m.prio || 'normal'] + (m.kind === 'dm' ? 1 : 0) - (m.kind === 'public' ? 1 : 0) - (it.retry ? 1 : 0);
        if (m.prio === 'high' || m.prio === 'alert') return Math.min(9, Math.max(8, p));
        p = Math.min(p, 7);
      } else p = it.kind === 'beacon' ? 6 : 1; // sync beacons keep recovery working; directory and housekeeping can wait
      const aged = p + Math.floor((S.now - it.t) / (20 * MIN));
      return Math.max(0, Math.min(Math.max(p, 7), aged));
    }
    S.effPrio = effPrio;
    function pump() {
      if (S.chan.busyUntil > S.now) return;
      // Each radio sends its own highest-priority item first (oldest first within a level);
      // a radio at its duty limit must not hold up the others.
      if (S.queue.length > 1) {
        for (const it of S.queue) it.ep = effPrio(it);
        S.queue.sort((a, b) => b.ep - a.ep || a.t - b.t);
      }
      let wake = Infinity;
      const blocked = new Set();
      for (let i = 0; i < S.queue.length; i++) {
        const it = S.queue[i];
        if (blocked.has(it.gw)) continue;
        if (!it.gw.online) {
          S.queue.splice(i--, 1);
          if (it.m) { it.m.inQueue.delete(it.gw.id); it.m.loraBy.delete(it.gw.id); }
          continue;
        }
        const at = dutyAllowAt(it.gw, it.dur);
        if (at <= S.now) { S.queue.splice(i, 1); transmit(it); return; }
        blocked.add(it.gw);
        wake = Math.min(wake, at);
      }
      if (wake < Infinity && wake < S.chan.pumpAt) { S.chan.pumpAt = wake; schedule(wake, () => { S.chan.pumpAt = Infinity; pump(); }); }
    }
    function transmit(it) {
      const loraNodes = S.nodes.filter((n) => n.lora && n.online).length;
      const rb = loraNodes > 2 ? cfg.rebroadcast : 0;
      const total = it.dur * (1 + rb);
      S.chan.busyUntil = S.now + total;
      S.chan.log.push({ t: S.now, dur: total });
      it.gw.air.push({ t: S.now, dur: it.dur });
      S.counters.loraTx++;
      it.log = { t: S.now, from: it.gw.name, mesh: it.gw.kind === 'mesh', kind: it.kind, channel: channelOf(it), dur: total, text: it.kind === 'msg' && it.m.kind === 'public' ? it.m.text : null, heard: 0 };
      S.airLog.push(it.log);
      if (S.airLog.length > 50) S.airLog.shift();
      if (it.kind === 'msg') { S.counters.msgAir += total; it.m.inQueue.delete(it.gw.id); } else S.counters.bgAir += total;
      S.uiEvents.push({ type: 'lora', node: it.gw, t: S.now, kind: it.kind });
      if (S.uiEvents.length > 200) S.uiEvents.shift();
      schedule(S.now + total, () => { receive(it); pump(); });
    }
    // Public posts go out on Meshtastic's open LongFast channel; everything else on batnet's encrypted channel.
    function channelOf(it) { return it.kind === 'msg' && it.m.kind === 'public' ? 'public' : 'private'; }
    function receive(it) {
      // Every radio in direct range hears the packet, Meshtastic users included. They read public posts
      // and pass batnet's encrypted packets on without being able to read them.
      const R = preset().range, heardBy = [];
      for (const r of S.nodes) {
        if (r === it.gw || !r.lora || !r.online || dist(it.gw, r) > R) continue;
        heardBy.push(r.id);
        if (r.kind !== 'mesh') continue;
        if (channelOf(it) === 'public') { if (!r.heard.some((x) => x.id === it.m.id)) r.heard.push({ id: it.m.id, t: S.now, name: senderLabel(it.m), text: it.m.text }); if (r.heard.length > 80) r.heard.shift(); }
        else r.relayed++;
      }
      it.log.heard = heardBy.length;
      S.uiEvents.push({ type: 'rx', node: it.gw, to: heardBy, channel: channelOf(it), t: S.now });
      if (it.kind === 'beacon') return recover(it.gw);
      if (it.kind !== 'msg') return;
      const m = it.m;
      for (const r of S.nodes) {
        if (r === it.gw || r.kind === 'mesh') continue;
        const reach = loraReach(it.gw, r);
        if (!reach) continue;
        const pOk = Math.pow(1 - lossAt(reach.d) - (reach.hops === 2 ? 0.03 : 0), it.frags);
        if (rnd() < pOk && !r.has.has(m.id)) { if (!m.via) m.via = 'lora'; arrive(r, m, S.now, 'lora'); }
      }
    }
    // A gateway's vector beacon lets every other reachable gateway re-send what this cluster is missing.
    function recover(g) {
      const C = S.clusters[g.cluster];
      if (!C || !g.online) return;
      const inC = new Set(C.members.map((m) => m.id));
      for (const D of S.clusters) {
        if (D === C || !D.gateway || !loraReach(g, D.gateway)) continue;
        const h = D.gateway;
        let sent = 0;
        for (let i = S.msgs.length - 1; i >= 0 && sent < 8; i--) {
          const m = S.msgs[i];
          if (g.has.has(m.id) || !h.has.has(m.id) || m.inQueue.has(h.id)) continue;
          let wanted = m.kind === 'public';
          for (const hid of m.targets) if (inC.has(hid)) { wanted = true; break; }
          if (!wanted) continue;
          enqueueMsg(h, m, true); sent++;
        }
      }
    }

    // ----- traffic -----
    function makeText(kind) {
      let t = pick(PHRASES[kind] || PHRASES.dm);
      if (kind === 'everyone' && rnd() < 0.06) { while (t.length < 700) t += ' ' + pick(PHRASES.everyone); }
      return t;
    }
    function send(personId, kind, to, text, prio) {
      const p = S.personById.get(personId);
      const house = nodeById.get(p.houseId);
      if (!house.online) return null;
      const targets = new Set();
      if (kind === 'dm') { const q = S.personById.get(to); targets.add(q.houseId); targets.add(p.houseId); }
      else if (kind === 'group') { const g = S.groups.find((x) => x.id === to); for (const pid of g.members) targets.add(S.personById.get(pid).houseId); targets.add(p.houseId); }
      else for (const h of S.houses) targets.add(h.id);
      const m = { id: S.nextMsgId++, t: S.now, from: personId, fromHouse: house.id, kind, to, text, prio: prio || 'normal', targets, deliv: new Map(), loraBy: new Set(), inQueue: new Set(), truncated: false, via: null };
      S.msgs.push(m); S.msgById.set(m.id, m); S.counters.sent++;
      m.deliv.set(house.id, { t: S.now, path: 'local' });
      arrive(house, m, S.now, 'local');
      return m;
    }
    function senderLabel(m) {
      if (m.fromName) return m.fromName;
      const p = S.personById.get(m.from);
      return p ? p.name + ' · ' + nodeById.get(p.houseId).name : '?';
    }
    function sendPublicFromMesh(node, text) {
      const own = text != null;
      text = own ? text : pick(PHRASES.public);
      node.heard.push({ id: S.nextMsgId, t: S.now, name: node.name, text, own });
      const m = { id: S.nextMsgId++, t: S.now, from: null, fromName: node.name, fromHouse: null, kind: 'public', to: null, text, targets: new Set(S.houses.map((h) => h.id)), deliv: new Map(), loraBy: new Set([node.id]), inQueue: new Set([node.id]), truncated: false, via: 'lora' };
      S.msgs.push(m); S.msgById.set(m.id, m);
      S.queue.push({ kind: 'msg', gw: node, m, dur: loraPayload(m).dur, frags: 1, retry: false, t: S.now });
      pump();
    }
    function trafficLoop() {
      const online = S.people.filter((p) => nodeById.get(p.houseId).online);
      if (online.length && cfg.chat > 0) {
        const p = pick(online);
        // people mark about 1 message in 20 High and 1 in 8 Low
        const r2 = rnd(), prio = r2 < 0.05 ? 'high' : r2 < 0.175 ? 'low' : 'normal';
        const r = rnd();
        if (r < 0.55) {
          let q = pick(S.people), guard = 0;
          while ((q === p || (q.houseId === p.houseId && rnd() < 0.7)) && guard++ < 20) q = pick(S.people);
          if (q !== p) send(p.id, 'dm', q.id, makeText('dm'), prio);
        } else if (r < 0.85 && p.groups.size) {
          const g = pick([...p.groups]);
          send(p.id, 'group', g, makeText(g), prio);
        } else if (rnd() < 0.2) send(p.id, 'public', null, makeText('public'));
        else send(p.id, 'everyone', null, makeText('everyone'), prio);
      }
      // about one message in six is spoken: a voice message whose transcript is what travels by radio
      const last = S.msgs[S.msgs.length - 1];
      if (last && last.from && last.t === S.now && !last.voice && last.kind !== 'public' && rnd() < 0.17) last.voice = { secs: 3 + Math.round(rnd() * 12) };
      const rate = (Math.max(online.length, 1) * Math.max(cfg.chat, 0.01)) / HOUR;
      schedule(S.now + expo(rate), trafficLoop);
    }

    // ----- background LoRa: vector beacons, directory cards, Meshtastic housekeeping -----
    function periodic(node, every, fn) {
      const loop = () => { fn(); schedule(S.now + every * (0.9 + 0.2 * rnd()), loop); };
      schedule(S.now + every * rnd(), loop);
    }
    for (const n of S.nodes) {
      if (n.kind === 'mesh') continue;
      periodic(n, 15 * MIN, () => { const cl = S.clusters[n.cluster]; if (n.online && cl && cl.gateway === n) enqueueBg(n, 'beacon', Math.min(200, 8 + 4 * S.houses.length)); });
      periodic(n, 30 * MIN, () => { const cl = S.clusters[n.cluster]; if (n.online && cl && cl.gateway === n) enqueueBg(n, 'directory', 40); });
      periodic(n, 30 * MIN, () => { if (n.online && n.lora) enqueueBg(n, 'housekeeping', 50); });
    }
    for (const n of S.nodes.filter((x) => x.kind === 'mesh')) {
      periodic(n, 30 * MIN, () => enqueueBg(n, 'housekeeping', 50));
      const chatter = () => { if (cfg.meshChatter > 0) sendPublicFromMesh(n); schedule(S.now + expo(Math.max(cfg.meshChatter, 0.01) / HOUR), chatter); };
      schedule(S.now + expo(Math.max(cfg.meshChatter, 0.01) / HOUR), chatter);
    }
    // power cuts
    function outageLoop() {
      if (cfg.outages) {
        const h = pick(S.houses.filter((x) => x.online));
        if (h) { setOnline(h.id, false); schedule(S.now + (5 + 35 * rnd()) * MIN, () => setOnline(h.id, true)); }
      }
      schedule(S.now + expo(1 / (12 * MIN)), outageLoop);
    }
    // sign-up requests for the admins
    function signupLoop() {
      const h = pick(S.houses.filter((x) => x.online));
      if (h) S.signups.push({ id: 's' + S.signups.length, houseId: h.id, name: pick(FIRST), auto: true, t: S.now, state: 'waiting' });
      schedule(S.now + expo(1 / (40 * MIN)), signupLoop);
    }

    // ----- metrics -----
    function busyFraction(windowMs) {
      const from = S.now - windowMs;
      while (S.chan.log.length && S.chan.log[0].t + S.chan.log[0].dur < S.now - 15 * MIN) S.chan.log.shift();
      let busy = 0;
      for (const e of S.chan.log) busy += Math.max(0, Math.min(e.t + e.dur, S.now) - Math.max(e.t, from));
      return busy / windowMs;
    }
    function pendingCount() {
      let n = 0;
      const cut = S.now - 8 * HOUR;
      for (let i = S.msgs.length - 1; i >= 0 && S.msgs[i].t > cut; i--) {
        const m = S.msgs[i];
        for (const hid of m.targets) if (!m.deliv.has(hid)) n++;
      }
      return n;
    }
    function sampleLoop() {
      const keep = 12 * 60;
      const push = (arr, v) => { arr.push({ t: S.now, v }); if (arr.length > keep) arr.shift(); };
      push(S.series.util, busyFraction(10 * MIN));
      push(S.series.queue, S.queue.filter((q) => q.kind === 'msg').length);
      push(S.series.pending, pendingCount());
      const cut = S.now - 2 * HOUR;
      while (S.deliveries.length && S.deliveries[0].t < cut) S.deliveries.shift();
      if (S.msgs.length > 6000) { // forget the oldest entries so memory stays bounded
        const drop = S.msgs.splice(0, S.msgs.length - 5000);
        for (const m of drop) { S.msgById.delete(m.id); for (const n of S.nodes) if (n.has) n.has.delete(m.id); }
      }
      schedule(S.now + MIN, sampleLoop);
    }
    function pct(arr, p) {
      if (!arr.length) return null;
      const s = arr.slice().sort((a, b) => a - b);
      return s[Math.min(s.length - 1, Math.floor(p * s.length))];
    }
    function stats() {
      const hourAgo = S.now - HOUR;
      const recent = S.deliveries.filter((d) => d.t >= hourAgo && d.t <= S.now);
      const by = (path) => recent.filter((d) => d.path === path).map((d) => d.lat);
      const wifi = by('wifi'), lora = by('lora'), sync = by('sync');
      const sentHour = S.msgs.filter((m) => m.t >= hourAgo && m.from).length;
      const fresh = S.msgs.filter((m) => m.t >= hourAgo - 10 * MIN && m.t < S.now - 10 * MIN && m.from);
      let tot = 0, ok = 0;
      for (const m of fresh) for (const hid of m.targets) { if (hid === m.fromHouse) continue; tot++; const d = m.deliv.get(hid); if (d && d.t - m.t <= 10 * MIN) ok++; }
      return {
        sentHour, within10: tot ? ok / tot : null,
        wifiP50: pct(wifi, 0.5), wifiP95: pct(wifi, 0.95), loraP50: pct(lora, 0.5), loraP95: pct(lora, 0.95), syncN: sync.length,
        loraN: lora.length, wifiN: wifi.length,
        loraByPrio: Object.fromEntries(['high', 'normal', 'low'].map((k) => { const xs = recent.filter((d) => d.path === 'lora' && d.prio === k).map((d) => d.lat); return [k, { n: xs.length, p50: pct(xs, 0.5), p95: pct(xs, 0.95) }]; })),
        util: busyFraction(10 * MIN), queue: S.queue.filter((q) => q.kind === 'msg').length, pending: pendingCount(),
      };
    }

    // ----- control -----
    function setOnline(id, on) {
      const n = nodeById.get(id);
      if (!n || n.online === on) return;
      n.online = on;
      n.offlineSince = on ? null : S.now;
      recompute();
      pump();
    }
    function step(toT, maxEvents) {
      let n = 0;
      const cap = maxEvents || 50000;
      while (S.heap.size && S.heap.peek().t <= toT && n < cap) {
        const e = S.heap.pop();
        S.now = Math.max(S.now, e.t);
        e.fn(); n++;
      }
      if (n < cap) S.now = Math.max(S.now, toT);
      return n;
    }
    function setPreset(name) { cfg.preset = name; recompute(); }
    function edit(id, patch) { const n = nodeById.get(id); Object.assign(n, patch); recompute(); pump(); }

    recompute();
    schedule(S.now + 1000, trafficLoop);
    schedule(S.now + 5 * MIN, outageLoop);
    schedule(S.now + 3 * MIN, signupLoop);
    schedule(S.now + MIN, sampleLoop);

    function approve(signupId, accept) {
      const s = S.signups.find((x) => x.id === signupId);
      if (!s || s.state !== 'waiting') return null;
      s.state = accept ? 'approved' : 'declined';
      if (accept) return addPerson(nodeById.get(s.houseId), s.auto ? s.name : s.name.split(' ')[0]);
      return null;
    }

    const meshPost = (id, text) => { const n = nodeById.get(id); if (n && n.kind === 'mesh') sendPublicFromMesh(n, text); };
    return { S, cfg, step, send, meshPost, setOnline, setPreset, edit, stats, loraReach, approve, recompute, preset, airtimeMs: (b) => airtimeMs(b + LORA_OVERHEAD, preset()) };
  }

  const ASSUMPTIONS = [
    'WiFi reach: Pi built-in radio ~80-120 m, USB dongle ~350 m, relay on a pole ~450 m, point-to-point unit up to 3.5 km. Link quality falls with the square of distance.',
    'LoRa reach per preset: ShortFast 1.2 km, MediumFast 2 km, LongFast 3 km, plus one rebroadcast hop. Packet loss 2% close in, rising towards 30% at the edge.',
    'Airtime from the Semtech formula with Meshtastic settings (250 kHz, CR 4/5, 16-symbol preamble) and ~26 bytes of Meshtastic overhead per packet.',
    'Each packet is heard again once as a Meshtastic rebroadcast when more than two LoRa nodes are on air.',
    'Each radio may use 10% of any hour (360 s), the EU 869.4-869.65 MHz rule.',
    'Everyone-channel posts go as plain text (~18 bytes of name prefix); DMs and group posts as a 12-byte envelope with text compressed to ~65%. Over LoRa text is cut at 600 characters and split into 200-byte fragments.',
    'Background LoRa: a 15-min vector beacon and 30-min directory card per WiFi group, and 30-min Meshtastic housekeeping per radio.',
    'A beacon makes other gateways re-send up to 8 missed messages. WiFi groups merge their logs at once when they connect.',
    'People send at a steady rate: 55% direct messages, 30% group posts, 15% everyone-channel posts; 6% of everyone posts are over 600 characters.',
  ];

  const api = { createSim, PRESETS, PI, SCENARIOS, WIFI_RANGE, airtimeMs, ASSUMPTIONS, HOUR, MIN };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.SimCore = api;
})(typeof window !== 'undefined' ? window : globalThis);
