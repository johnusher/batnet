// The batnet film, drawn on a canvas. Every frame is a pure function of the
// time t (seconds from the start), so headless Chrome can render it frame by
// frame (?render=1) and a browser can preview it in real time (no query).
// Scene lengths come from out/timeline.json, written by render.py from the
// narration clips. Coordinates below are in a 1920x1080 space; the canvas is
// scaled to the timeline's size.
(async () => {
  'use strict';
  const RENDER = new URLSearchParams(location.search).has('render');
  const post = (path, body) => fetch(path, { method: 'POST', body }).catch(() => {});
  const log = (...a) => { console.log(...a); if (RENDER) post('/log', a.join(' ')); };
  window.addEventListener('error', (e) => log('ERROR', e.message, e.filename, e.lineno));

  const tl = await (await fetch('out/timeline.json')).json();
  const W = 1920, H = 1080, S = tl.width / W;
  const cv = document.getElementById('c');
  cv.width = tl.width; cv.height = tl.height;
  const ctx = cv.getContext('2d');

  const images = {};
  await Promise.all(['app-sign-in', 'app-conversation', 'village-sim-dark'].map((n) => new Promise((ok) => {
    const im = new Image(); im.onload = () => { images[n] = im; ok(); }; im.onerror = () => { log('no image', n); ok(); }; im.src = 'assets/' + n + '.png';
  })));

  // ---------- small helpers ----------
  const clamp = (x, a = 0, b = 1) => Math.max(a, Math.min(b, x));
  const lerp = (a, b, t) => a + (b - a) * t;
  const easeOut = (t) => 1 - Math.pow(1 - clamp(t), 3);
  const easeIn = (t) => Math.pow(clamp(t), 3);
  const easeInOut = (t) => { t = clamp(t); return t < .5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2; };
  const ramp = (u, a, b) => clamp((u - a) / (b - a)); // 0 before a, 1 after b
  const rng = (seed) => () => { seed |= 0; seed = seed + 0x6D2B79F5 | 0; let x = Math.imul(seed ^ seed >>> 15, 1 | seed); x ^= x + Math.imul(x ^ x >>> 7, 61 | x); return ((x ^ x >>> 14) >>> 0) / 4294967296; };
  const C = { bg: '#0a0f1c', ink: '#eef2f6', sub: '#9aa6b4', wifi: '#6fb6ea', lora: '#f2a541', alert: '#ee4b3a', warm: '#ffd27a', ok: '#5bd49a' };
  const FONT = '"Helvetica Neue", Helvetica, Arial, system-ui, sans-serif';
  const MONO = '"SF Mono", Menlo, Consolas, monospace';
  const font = (px, w = 500, f = FONT) => { ctx.font = `${w} ${px}px ${f}`; };

  function rr(x, y, w, h, r) { ctx.beginPath(); ctx.roundRect(x, y, w, h, r); }
  function glow(x, y, r, color, a = 1) {
    const g = ctx.createRadialGradient(x, y, 0, x, y, r);
    g.addColorStop(0, color); g.addColorStop(1, 'rgba(0,0,0,0)');
    ctx.globalAlpha = a; ctx.fillStyle = g; ctx.beginPath(); ctx.arc(x, y, r, 0, Math.PI * 2); ctx.fill(); ctx.globalAlpha = 1;
  }
  // an expanding ring: born at t0, lives `life` seconds, grows to rmax
  function ring(x, y, t, t0, life, rmax, color, width = 3) {
    const k = (t - t0) / life; if (k < 0 || k > 1) return;
    ctx.globalAlpha = (1 - k) * 0.9; ctx.strokeStyle = color; ctx.lineWidth = width * (1 - k * 0.6);
    ctx.beginPath(); ctx.arc(x, y, 6 + rmax * easeOut(k), 0, Math.PI * 2); ctx.stroke(); ctx.globalAlpha = 1;
  }
  function pulse(x, y, t, t0, color, period = 1.6, rmax = 70, n = 3) {
    for (let i = 0; i < n; i++) ring(x, y, t, t0 + i * period / n, period, rmax, color, 2.5);
  }
  function text(s, x, y, px, color = C.ink, align = 'left', w = 500, f = FONT) {
    font(px, w, f); ctx.fillStyle = color; ctx.textAlign = align; ctx.textBaseline = 'middle'; ctx.fillText(s, x, y);
  }
  function wrap(s, maxW) {
    const words = s.split(' '), lines = []; let line = '';
    for (const w of words) { const t = line ? line + ' ' + w : w; if (ctx.measureText(t).width > maxW && line) { lines.push(line); line = w; } else line = t; }
    if (line) lines.push(line); return lines;
  }
  function house(x, y, s, color, lit = true) { // a small house icon, base at (x, y)
    ctx.fillStyle = color; ctx.beginPath(); ctx.moveTo(x - s, y); ctx.lineTo(x - s, y - s * .9); ctx.lineTo(x, y - s * 1.6); ctx.lineTo(x + s, y - s * .9); ctx.lineTo(x + s, y); ctx.closePath(); ctx.fill();
    if (lit) { ctx.fillStyle = C.warm; ctx.fillRect(x - s * .3, y - s * .7, s * .6, s * .45); }
  }
  function node(x, y, t, t0, color = C.lora, s = 9) { // a batnet node: a square with a dot, and rings when born
    const k = easeOut(ramp(t, t0, t0 + .5)); if (k <= 0) return;
    ctx.fillStyle = color; ctx.globalAlpha = k; rr(x - s, y - s, 2 * s, 2 * s, 3); ctx.fill(); ctx.fillStyle = '#111'; ctx.beginPath(); ctx.arc(x, y, s * .35, 0, 7); ctx.fill(); ctx.globalAlpha = 1;
    ring(x, y, t, t0, 1.4, 60, color);
  }
  function phone(x, y, w, h, color = '#1a2230') { rr(x, y, w, h, w * .12); ctx.fillStyle = color; ctx.fill(); ctx.strokeStyle = '#3a4656'; ctx.lineWidth = 3; ctx.stroke(); }
  function bubble(s, x, y, t, t0, opts = {}) { // a chat bubble popping at (x, y) above the point
    const k = easeOut(ramp(t, t0, t0 + .35)); if (k <= 0) return;
    const px = opts.px || 22, pad = 12; font(px, 500); const tw = ctx.measureText(s).width, w = tw + pad * 2, h = px + pad * 1.4;
    y = Math.min(y, H - 230); const bx = clamp(x - w / 2, 20, W - w - 20), by = y - h - 16;
    ctx.save(); ctx.translate(x, y); ctx.scale(k, k); ctx.translate(-x, -y);
    ctx.globalAlpha = opts.alpha == null ? 1 : opts.alpha;
    ctx.fillStyle = opts.fill || '#f3f6f9'; rr(bx, by, w, h, 10); ctx.fill();
    ctx.beginPath(); ctx.moveTo(x - 7, by + h); ctx.lineTo(x + 7, by + h); ctx.lineTo(x, by + h + 9); ctx.fill();
    text(s, bx + pad, by + h / 2, px, opts.color || '#10151c');
    ctx.restore(); ctx.globalAlpha = 1;
  }
  function dot(ax, ay, bx, by, k, color, r = 5) { if (k < 0 || k > 1) return; const x = lerp(ax, bx, k), y = lerp(ay, by, k); glow(x, y, r * 4, color, .5); ctx.fillStyle = color; ctx.beginPath(); ctx.arc(x, y, r, 0, 7); ctx.fill(); }
  function vignette() { const g = ctx.createRadialGradient(W / 2, H / 2, H * .35, W / 2, H / 2, H * .95); g.addColorStop(0, 'rgba(0,0,0,0)'); g.addColorStop(1, 'rgba(0,0,0,.55)'); ctx.fillStyle = g; ctx.fillRect(0, 0, W, H); }
  function fade(a) { if (a <= 0) return; ctx.globalAlpha = clamp(a); ctx.fillStyle = '#000'; ctx.fillRect(0, 0, W, H); ctx.globalAlpha = 1; }
  function label(s, sub) { // top-left scene label
    text(s.toUpperCase(), 64, 62, 22, C.sub, 'left', 600); ctx.font = `600 22px ${FONT}`; const w = ctx.measureText(s.toUpperCase()).width;
    ctx.fillStyle = C.lora; ctx.fillRect(64, 82, Math.min(w, 180), 3); if (sub) text(sub, 64, 110, 20, C.sub);
  }

  // ---------- captions: the narration, in chunks, words appearing as spoken ----------
  function chunks(s) {
    const parts = s.match(/[^.!?:]+[.!?:]+["']?\s*|[^.!?:]+$/g) || [s];
    const out = []; let cur = '';
    for (const p of parts) { if ((cur + p).length > 92 && cur) { out.push(cur.trim()); cur = p; } else cur += p; }
    if (cur.trim()) out.push(cur.trim());
    return out.flatMap((c) => c.length > 110 ? c.split(/,\s/).reduce((acc, piece, i, arr) => { const last = acc[acc.length - 1]; if (last != null && (last + ', ' + piece).length <= 100) acc[acc.length - 1] = last + ', ' + piece; else acc.push(piece + (i < arr.length - 1 && !piece.endsWith(',') ? ',' : '')); return acc; }, []) : [c]);
  }
  function caption(scene, u) {
    if (scene.id === 'title') return;
    const cs = chunks(scene.caption || scene.text), total = cs.reduce((n, c) => n + c.length, 0), speech = scene.speech || 5;
    let at = 0, cur = cs[cs.length - 1], start = 0, len = 1;
    for (const c of cs) { const d = speech * c.length / total; if (u < at + d) { cur = c; start = at; len = d; break; } at += d; start = at; }
    if (u > speech + 2.5 && scene.id !== 'close') { ctx.globalAlpha = clamp(1 - (u - speech - 2.5) / .6); }
    const words = cur.split(' '), shown = Math.min(words.length, Math.ceil((u - start) / len * (words.length + 1)));
    font(34, 500); const lines = wrap(words.slice(0, shown).join(' '), 1500);
    const y0 = H - 118 - (lines.length - 1) * 44;
    lines.forEach((ln, i) => {
      const y = y0 + i * 44, w = ctx.measureText(ln).width;
      ctx.fillStyle = 'rgba(0,0,0,.62)'; rr(W / 2 - w / 2 - 18, y - 24, w + 36, 48, 8); ctx.fill();
      text(ln, W / 2, y, 34, '#ffffff', 'center');
    });
    ctx.globalAlpha = 1;
  }

  // ---------- the scenes ----------
  const events = []; // {t, kind, gain}: sounds, collected once in setup
  const scenes = {};
  const sceneStart = (id) => (tl.scenes.find((s) => s.id === id) || { start: 0 }).start;
  const sceneLen = (id) => { const s = tl.scenes.find((x) => x.id === id); return s ? s.end - s.start : 1; };
  const ev = (id, u, kind, gain) => events.push({ t: +(sceneStart(id) + u).toFixed(3), kind, gain });

  // --- title ---
  scenes.title = {
    setup() { const r = rng(1); this.lights = Array.from({ length: 220 }, () => ({ x: r() * W, y: r() * H, f: r() * 6, a: .2 + r() * .5 })); ev('title', 1.0, 'swell', -14); },
    draw(u, len) {
      ctx.fillStyle = C.bg; ctx.fillRect(0, 0, W, H);
      const dim = 1 - easeIn(ramp(u, len - 1.6, len - .3));
      for (const l of this.lights) { const fl = .7 + .3 * Math.sin(u * 3 + l.f); ctx.fillStyle = C.warm; ctx.globalAlpha = l.a * fl * dim * ramp(u, 0, 1); ctx.beginPath(); ctx.arc(l.x, l.y, 1.6, 0, 7); ctx.fill(); }
      ctx.globalAlpha = 1;
      const cx = W / 2, cy = H / 2 - 60;
      const houses = [[cx, cy + 110], [cx - 230, cy + 150], [cx + 250, cy + 140], [cx - 90, cy + 190], [cx + 110, cy + 200]];
      houses.forEach(([x, y], i) => { const t0 = .4 + i * .35; const k = ramp(u, t0, t0 + .4); if (k > 0) { ctx.globalAlpha = k; house(x, y, 18, '#2c3a4e'); ctx.globalAlpha = 1; ring(x, y - 18, u, t0, 1.6, 90, C.wifi, 2); } });
      ctx.strokeStyle = C.wifi; ctx.lineWidth = 1.5; ctx.globalAlpha = .5 * ramp(u, 1.6, 2.4);
      for (let i = 1; i < houses.length; i++) { ctx.beginPath(); ctx.moveTo(houses[0][0], houses[0][1] - 18); ctx.lineTo(houses[i][0], houses[i][1] - 18); ctx.stroke(); }
      ctx.globalAlpha = 1;
      const k = easeOut(ramp(u, 1.1, 2.1));
      ctx.globalAlpha = k; ctx.save(); ctx.translate(cx, cy - 80); ctx.scale(.9 + .1 * k, .9 + .1 * k);
      font(170, 700); ctx.letterSpacing = '-4px'; ctx.fillStyle = C.ink; ctx.textAlign = 'center'; ctx.textBaseline = 'middle'; ctx.fillText('batnet', 0, 0); ctx.letterSpacing = '0px'; ctx.restore();
      ctx.globalAlpha = ramp(u, 2.3, 3.1); text('When the grid goes down, the village keeps talking.', cx, cy + 30, 40, C.sub, 'center', 400); ctx.globalAlpha = 1;
      vignette();
    },
  };

  // --- Berlin: a city map that goes dark, then starts talking ---
  const berlin = {
    tunnel: [150, 1010], blocks: [], nodes: [], links: [], msgs: [], phones: [],
    setup() {
      const r = rng(7); const cols = 13, rows = 7, bw = (W - 80) / cols, bh = (H - 200) / rows;
      for (let i = 0; i < cols; i++) for (let j = 0; j < rows; j++) {
        if ((i === 5 && j >= 2 && j <= 3) || (i === 6 && j === 3)) continue; // a park
        const x = 40 + i * bw + 12 + r() * 6, y = 140 + j * bh + 12 + r() * 6, w = bw - 28 - r() * 10, h = bh - 28 - r() * 10;
        const d = Math.hypot(x + w / 2 - this.tunnel[0], y + h / 2 - this.tunnel[1]) / 1900;
        const win = []; for (let a = 0; a < Math.floor(w / 14); a++) for (let b = 0; b < Math.floor(h / 16); b++) if (r() < .72) win.push([x + 8 + a * 14, y + 9 + b * 16, r()]);
        this.blocks.push({ x, y, w, h, dark: 2.6 + d * 3.8 + r() * .4, win, i, j });
      }
      // nodes: every third block, at a window
      this.blocks.forEach((b, i) => { if (i % 3 === 1) this.nodes.push({ x: b.x + b.w * .5, y: b.y + b.h * .55, t0: 18.2 + (i / 3) * .13 + r() * .1, b }); });
      for (let a = 0; a < this.nodes.length; a++) for (let b = a + 1; b < this.nodes.length; b++) {
        const d = Math.hypot(this.nodes[a].x - this.nodes[b].x, this.nodes[a].y - this.nodes[b].y);
        if (d < 420) this.links.push([a, b, d]);
      }
      this.phones = this.blocks.filter((_, i) => i % 7 === 3).slice(0, 9).map((b) => ({ x: b.x + b.w * .25, y: b.y + b.h * .3, t0: 7.5 + r() * 2.5 }));
      const texts = ['Generator at number 14: phones charge 9 to 11', 'Pharmacy on the corner open till 20:00', 'Insulin needed, block C, anyone with a cold box?'];
      texts.forEach((s, k) => {
        const from = [4, 17, 26][k] % this.nodes.length, t0 = 20.4 + k * 2.3;
        // breadth-first over the links: hop times
        const at = new Map([[from, t0]]); const q = [from];
        while (q.length) { const a = q.shift(); for (const [p, qq] of this.links) { const o = p === a ? qq : qq === a ? p : -1; if (o >= 0 && !at.has(o)) { at.set(o, at.get(a) + .22); q.push(o); } } }
        const hops = []; for (const [p, qq] of this.links) { const ta = at.get(p), tb = at.get(qq); if (ta == null || tb == null) continue; if (Math.abs(ta - tb) > .2 && Math.abs(ta - tb) < .3) hops.push(ta < tb ? [p, qq, ta] : [qq, p, tb]); }
        hops.sort((x, y) => x[2] - y[2]);
        hops.slice(0, 14).forEach((h, i) => { if (i % 2 === 0) ev('berlin2', h[2] - 18, 'blip', -22 - i); });
        this.msgs.push({ s, from, t0, hops, at });
      });
      ev('berlin', 2.05, 'boom', -8);
    },
    map(ug) {
      ctx.fillStyle = '#0d1220'; ctx.fillRect(0, 0, W, H);
      // the river
      ctx.strokeStyle = '#132238'; ctx.lineWidth = 70; ctx.beginPath(); ctx.moveTo(-50, 300); ctx.bezierCurveTo(500, 200, 900, 700, W + 50, 560); ctx.stroke();
      ctx.fillStyle = '#121a2a';
      for (const b of this.blocks) { rr(b.x, b.y, b.w, b.h, 4); ctx.fill(); }
      for (const b of this.blocks) {
        const flick = ug > b.dark - .5 && ug < b.dark ? (Math.sin(ug * 60 + b.i) > 0 ? 1 : .3) : 1;
        const on = ug < b.dark ? flick : 0;
        for (const [x, y, f] of b.win) {
          if (on > 0) { ctx.fillStyle = C.warm; ctx.globalAlpha = (.55 + .45 * f) * on; ctx.fillRect(x, y, 5, 6); }
          else { ctx.fillStyle = '#1b2333'; ctx.globalAlpha = 1; ctx.fillRect(x, y, 5, 6); }
        }
      }
      ctx.globalAlpha = 1;
      if (ug < 3.2) { const g = ctx.createRadialGradient(W / 2, H / 2, 0, W / 2, H / 2, 1000); g.addColorStop(0, 'rgba(255,210,122,.10)'); g.addColorStop(1, 'rgba(0,0,0,0)'); ctx.fillStyle = g; ctx.fillRect(0, 0, W, H); }
      // the fire at the cable tunnel
      const [tx, ty] = this.tunnel; const f = ramp(ug, 1.9, 2.1) * (1 - ramp(ug, 9, 12));
      if (f > 0) { glow(tx, ty, 260, 'rgba(255,120,40,.85)', f * (0.7 + .3 * Math.sin(ug * 20))); ctx.fillStyle = '#ffb347'; ctx.globalAlpha = f; ctx.beginPath(); ctx.arc(tx, ty, 9, 0, 7); ctx.fill(); ctx.globalAlpha = 1; text('cable tunnel', tx + 24, ty - 8, 20, '#ffb347'); }
      ring(tx, ty, ug, 2.0, 2.8, 1900, 'rgba(255,140,60,.6)', 5);
    },
  };
  scenes.berlin = {
    setup() { berlin.setup(); },
    draw(u, len) {
      berlin.map(u);
      for (const p of berlin.phones) { const k = ramp(u, p.t0, p.t0 + .5) * (1 - ramp(u, 19, 20.5)); if (k <= 0) continue; ctx.globalAlpha = k; phone(p.x - 14, p.y - 26, 28, 52, '#222b38'); text('no service', p.x, p.y - 44, 15, C.alert, 'center', 600); ctx.strokeStyle = C.alert; ctx.lineWidth = 3; ctx.beginPath(); ctx.moveTo(p.x - 9, p.y - 10); ctx.lineTo(p.x + 9, p.y + 10); ctx.moveTo(p.x + 9, p.y - 10); ctx.lineTo(p.x - 9, p.y + 10); ctx.stroke(); ctx.globalAlpha = 1; }
      // the numbers
      const stat = (v, unit, y, t0) => { const k = ramp(u, t0, t0 + 2.2); if (k <= 0) return; ctx.globalAlpha = Math.min(1, k * 4) * (1 - ramp(u, len - .4, len)); const n = typeof v === 'number' ? Math.round(easeOut(k) * v).toLocaleString('en-GB') : v; text(n, W - 100, y, 92, C.ink, 'right', 700, MONO); text(unit, W - 100, y + 62, 26, C.sub, 'right', 500); ctx.globalAlpha = 1; };
      stat(50000, 'homes dark', 330, 11); stat('3 days', 'without power or heating', 520, 13.6); stat('0 bars', 'mobile masts out after a few hours', 710, 15.4);
      label('Berlin, Treptow-Köpenick', '8 January 2025: an arson attack on two high-voltage cables');
      vignette(); fade(1 - ramp(u, 0, .6));
    },
  };
  scenes.berlin2 = {
    setup() {},
    draw(u, len) {
      const ug = u + 18; berlin.map(ug);
      ctx.strokeStyle = C.wifi;
      for (const [a, b] of berlin.links) { const na = berlin.nodes[a], nb = berlin.nodes[b]; const k = ramp(ug, Math.max(na.t0, nb.t0) + .3, Math.max(na.t0, nb.t0) + .9); if (k <= 0) continue; ctx.globalAlpha = .35 * k; ctx.lineWidth = 1.5; ctx.beginPath(); ctx.moveTo(na.x, na.y); ctx.lineTo(nb.x, nb.y); ctx.stroke(); }
      ctx.globalAlpha = 1;
      for (const n of berlin.nodes) { node(n.x, n.y, ug, n.t0, C.wifi, 8); if (ug > n.t0 + 1) pulse(n.x, n.y, ug, n.t0 + 1, 'rgba(111,182,234,.35)', 3.2, 150, 1); }
      for (const m of berlin.msgs) {
        for (const [a, b, ta] of m.hops) { const na = berlin.nodes[a], nb = berlin.nodes[b]; dot(na.x, na.y, nb.x, nb.y, (ug - ta) / .22, C.wifi, 4); const ka = ramp(ug, ta + .22, ta + .32) * (1 - ramp(ug, ta + .5, ta + 1.4)); if (ka > 0) glow(nb.x, nb.y, 40, 'rgba(111,182,234,.9)', ka); }
        const nf = berlin.nodes[m.from]; bubble(m.s, nf.x, nf.y - 10, ug, m.t0, { alpha: 1 - ramp(ug, m.t0 + 4.5, m.t0 + 5.2) });
      }
      label('Berlin, the same night', 'every third house with a node');
      vignette(); fade(ramp(u, len - .5, len));
    },
  };

  // --- how it works ---
  scenes.how = {
    setup() { const r = rng(3); this.qr = Array.from({ length: 21 * 21 }, () => r() < .45); ev('how', 2.6, 'swell', -16); ev('how', 21.5, 'swell', -14); for (let i = 0; i < 6; i++) ev('how', 14.2 + i * .9, 'blip', -24); },
    pi(x, y, s = 1) { ctx.save(); ctx.translate(x, y); ctx.scale(s, s); ctx.fillStyle = '#2f7d4f'; rr(-85, -55, 170, 110, 8); ctx.fill(); ctx.fillStyle = '#1b1f24'; rr(-60, -25, 54, 54, 4); ctx.fill(); ctx.fillStyle = '#c9ccd1'; for (let i = 0; i < 20; i++) { ctx.fillRect(-80 + i * 8, -52, 4, 10); } ctx.fillStyle = '#8a95a3'; rr(10, -20, 30, 22, 2); ctx.fill(); rr(45, -20, 30, 22, 2); ctx.fill(); rr(10, 10, 30, 22, 2); ctx.fill(); rr(45, 10, 30, 22, 2); ctx.fill(); ctx.fillStyle = C.ok; ctx.beginPath(); ctx.arc(-70, 42, 4, 0, 7); ctx.fill(); ctx.restore(); },
    draw(u, len) {
      ctx.fillStyle = C.bg; ctx.fillRect(0, 0, W, H);
      const A = 1 - ramp(u, 11.6, 12.4); // the house close-up
      if (A > 0) {
        ctx.globalAlpha = A;
        const hx = 640, hy = 760; ctx.strokeStyle = '#3a4a60'; ctx.lineWidth = 6; ctx.beginPath(); ctx.moveTo(hx - 380, hy); ctx.lineTo(hx - 380, hy - 360); ctx.lineTo(hx, hy - 600); ctx.lineTo(hx + 380, hy - 360); ctx.lineTo(hx + 380, hy); ctx.closePath(); ctx.stroke();
        ctx.fillStyle = '#243041'; ctx.fillRect(hx - 220, hy - 190, 440, 14); ctx.fillRect(hx - 200, hy - 176, 14, 176); ctx.fillRect(hx + 186, hy - 176, 14, 176);
        const k1 = easeOut(ramp(u, .3, 1.0)); ctx.globalAlpha = A * k1; this.pi(hx - 40, hy - 250, 1); ctx.fillStyle = '#1b222d'; rr(hx + 70, hy - 230, 50, 110, 10); ctx.fill(); ctx.strokeStyle = '#555'; ctx.lineWidth = 3; ctx.beginPath(); ctx.moveTo(hx + 45, hy - 250); ctx.lineTo(hx + 95, hy - 250); ctx.lineTo(hx + 95, hy - 232); ctx.stroke();
        text('Raspberry Pi', hx - 40, hy - 330, 24, C.sub, 'center', 600); text('power bank', hx + 95, hy - 110, 20, C.sub, 'center', 600);
        if (u > 2.4) for (let i = 0; i < 4; i++) ring(hx - 40, hy - 250, u, 2.4 + i * .55, 2.2, 520, C.wifi, 3);
        const k2 = ramp(u, 3.4, 4); ctx.globalAlpha = A * k2; rr(hx - 220, hy - 480, 440, 48, 24); ctx.fillStyle = '#152a3c'; ctx.fill(); text('open WiFi  ·  batnet · Miller House', hx, hy - 456, 24, C.wifi, 'center', 600);
        ctx.globalAlpha = A;
        // the phone with the real app
        const k3 = easeOut(ramp(u, 5.8, 6.8)); if (k3 > 0) { const px = W - 540 + (1 - k3) * 300, py = 150; ctx.globalAlpha = A * k3; phone(px, py, 400, 800, '#0e1218'); const a2 = ramp(u, 8.8, 9.6); const im1 = images['app-sign-in'], im2 = images['app-conversation']; ctx.save(); rr(px + 12, py + 12, 376, 776, 36); ctx.clip(); if (im1) { ctx.globalAlpha = A * k3 * (1 - a2); ctx.drawImage(im1, px + 12, py + 12, 376, 770); } if (im2) { ctx.globalAlpha = A * k3 * a2; ctx.drawImage(im2, px + 12, py + 12, 376, 770); } ctx.restore(); ctx.globalAlpha = A * k3; text('no app store · no account · no company', px + 200, py + 840, 22, C.sub, 'center', 500); }
        ctx.globalAlpha = 1;
      }
      const B = ramp(u, 11.8, 12.6) * (1 - ramp(u, 26.4, 27.2)); // the village: mesh, then the hills and LoRa
      if (B > 0) {
        ctx.globalAlpha = B;
        // hills on the right from 20 s
        const hk = easeOut(ramp(u, 19.6, 21));
        if (hk > 0) { ctx.fillStyle = '#16301f'; ctx.beginPath(); ctx.moveTo(900, H); for (let x = 900; x <= W; x += 20) ctx.lineTo(x, H - hk * (260 + 170 * Math.sin((x - 900) / 330) + 60 * Math.sin((x - 900) / 90))); ctx.lineTo(W, H); ctx.fill(); }
        const hs = [[260, 700], [470, 640], [690, 720], [560, 850], [360, 880], [800, 600]];
        hs.forEach(([x, y], i) => { const t0 = 12.2 + i * .3; const k = ramp(u, t0, t0 + .4); if (k > 0) { ctx.globalAlpha = B * k; house(x, y, 26, '#2c3a4e'); node(x, y - 22, u, t0 + .2, C.wifi, 7); } });
        ctx.globalAlpha = B * .5 * ramp(u, 13.6, 14.4); ctx.strokeStyle = C.wifi; ctx.lineWidth = 2;
        const pairs = [[0, 1], [1, 2], [2, 3], [3, 4], [4, 0], [1, 5], [2, 5], [0, 3]];
        for (const [a, b] of pairs) { ctx.beginPath(); ctx.moveTo(hs[a][0], hs[a][1] - 22); ctx.lineTo(hs[b][0], hs[b][1] - 22); ctx.stroke(); }
        ctx.globalAlpha = B;
        pairs.forEach(([a, b], i) => { const t0 = 14.2 + i * .9; dot(hs[a][0], hs[a][1] - 22, hs[b][0], hs[b][1] - 22, (u - t0) / .5, C.wifi); });
        text('WiFi mesh between neighbours', 530, 520, 26, C.wifi, 'center', 600);
        // the far house over the hill, by LoRa
        const fk = ramp(u, 20.8, 21.6); if (fk > 0) { const fx = 1720, fy = H - 330; ctx.globalAlpha = B * fk; house(fx, fy, 26, '#2c3a4e'); node(fx, fy - 22, u, 21.2, C.lora, 8); text('3 km, over the hill', fx, fy + 40, 22, C.lora, 'center', 600); ctx.globalAlpha = B;
          const gx = hs[5][0], gy = hs[5][1] - 22; ctx.setLineDash([10, 14]); ctx.strokeStyle = C.lora; ctx.lineWidth = 3; ctx.globalAlpha = B * .8 * ramp(u, 21.4, 22.2); ctx.beginPath(); ctx.moveTo(gx, gy); ctx.quadraticCurveTo((gx + fx) / 2, 120, fx, fy - 22); ctx.stroke(); ctx.setLineDash([]);
          for (let i = 0; i < 3; i++) { const k = ((u - 21.8 - i * 1.4) / 1.6); if (k > 0 && k < 1) { const x = (1 - k) * (1 - k) * gx + 2 * (1 - k) * k * (gx + fx) / 2 + k * k * fx, y = (1 - k) * (1 - k) * gy + 2 * (1 - k) * k * 120 + k * k * (fy - 22); glow(x, y, 26, C.lora, .6); ctx.fillStyle = C.lora; ctx.beginPath(); ctx.arc(x, y, 6, 0, 7); ctx.fill(); } }
          ctx.globalAlpha = B * ramp(u, 22, 22.6); text('LoRa radio · 868 MHz · a few kilometres', 1180, 150, 26, C.lora, 'center', 600);
          node(gx, gy, u, 20.9, C.lora, 8);
        }
        ctx.globalAlpha = 1;
      }
      const Q = ramp(u, 26.6, 27.4) * (1 - ramp(u, len - .5, len)); // the QR village key
      if (Q > 0) {
        ctx.globalAlpha = Q; const px = W / 2 - 200, py = 110; phone(px, py, 400, 800, '#0e1218');
        const qx = px + 60, qy = py + 160, cs = 280 / 21; ctx.fillStyle = '#fff'; ctx.fillRect(qx - 14, qy - 14, 308, 308);
        ctx.fillStyle = '#000'; this.qr.forEach((on, i) => { if (on) ctx.fillRect(qx + (i % 21) * cs, qy + Math.floor(i / 21) * cs, cs, cs); });
        for (const [fx, fy] of [[0, 0], [14, 0], [0, 14]]) { ctx.fillStyle = '#000'; ctx.fillRect(qx + fx * cs, qy + fy * cs, 7 * cs, 7 * cs); ctx.fillStyle = '#fff'; ctx.fillRect(qx + (fx + 1) * cs, qy + (fy + 1) * cs, 5 * cs, 5 * cs); ctx.fillStyle = '#000'; ctx.fillRect(qx + (fx + 2) * cs, qy + (fy + 2) * cs, 3 * cs, 3 * cs); }
        text('Add a house', px + 200, py + 90, 30, C.ink, 'center', 700); text('scan once, from the admin’s phone', px + 200, py + 520, 22, C.sub, 'center'); text('village key · 2 minutes', px + 200, py + 560, 20, C.lora, 'center', 600);
        const sk = easeOut(ramp(u, 28.4, 29.2)); if (sk > 0) { ctx.save(); ctx.translate(px + 200, py + 690); ctx.rotate(-.12); ctx.scale(1.3 - .3 * sk, 1.3 - .3 * sk); ctx.globalAlpha = Q * sk; ctx.strokeStyle = C.alert; ctx.lineWidth = 4; rr(-170, -30, 340, 60, 8); ctx.stroke(); text('NO ENIGMA REQUIRED', 0, 0, 26, C.alert, 'center', 800); ctx.restore(); }
        ctx.globalAlpha = 1;
      }
      label('How it works', 'one Pi per house');
      vignette(); fade(1 - ramp(u, 0, .5));
    },
  };

  // --- Glastonbury: hills, tents, pocket nodes; a lost child; a bang ---
  const glasto = {
    tents: [], nodes: [], links: [], reports: [], bang: [420, 640], stage: [700, 470],
    hill(x, k) { return [H - 230 + 70 * Math.sin(x / 700 + k) + 40 * Math.sin(x / 260 + k * 2), H - 350 + 90 * Math.sin(x / 520 + k + 1) + 30 * Math.sin(x / 150), H - 470 + 110 * Math.sin(x / 900 + k + 2) + 50 * Math.sin(x / 300 + 1)]; },
    setup() {
      const r = rng(11);
      for (let i = 0; i < 420; i++) { const x = r() * (W + 600) - 300, layer = Math.floor(r() * 3); const y = this.hill(x, 0)[layer] + r() * 40 + 10; this.tents.push({ x, y, layer, c: ['#e35c5c', '#5aa9e6', '#f2c14e', '#7bd389', '#d87be6', '#f0f0f0'][Math.floor(r() * 6)], s: 9 + r() * 8 - layer * 2 }); }
      this.tents.sort((a, b) => b.layer - a.layer || a.y - b.y);
      this.nodes = [[700, 575], [300, 720], [1100, 690], [1500, 600], [1750, 790], [950, 860], [480, 880]].map(([x, y], i) => ({ x, y, t0: 4.2 + i * .45 }));
      for (let a = 0; a < this.nodes.length; a++) for (let b = a + 1; b < this.nodes.length; b++) if (Math.hypot(this.nodes[a].x - this.nodes[b].x, this.nodes[a].y - this.nodes[b].y) < 620) this.links.push([a, b]);
      // the alert: from the stage node (0), one hop every .45 s
      const at = new Map([[0, 12.9]]); const q = [0];
      while (q.length) { const a = q.shift(); for (const [p, qq] of this.links) { const o = p === a ? qq : qq === a ? p : -1; if (o >= 0 && !at.has(o)) { at.set(o, at.get(a) + .45); q.push(o); } } }
      this.alertAt = at;
      ev('lostchild', .5, 'alert', -8); for (const [, t] of at) if (t > 12.9) ev('lostchild', t - 12.4 + .2, 'blip', -22);
      // the bang at 18.2, the reports from 19 on, nearest node first
      const lines = ['Bang by the west field', 'Loud bang, smoke near the cider bus', 'Something exploded?', 'Big bang, west side', 'Fireworks? Sounded wrong', 'Bang near the gate, kids crying', 'Heard it from the Park', 'Smoke by the west field', 'Bang, everyone OK here', 'Bang! Anyone hurt?', 'West field, flames', 'That was not fireworks', 'Bang near Pennard Hill', 'Smoke drifting east', 'Medics needed west field', 'Loud bang, 2 min ago', 'Bang by the fence', 'All fine at the Pyramid'];
      lines.forEach((s, i) => { const d = 60 + i * 55, ang = r() * Math.PI * 2; const x = clamp(this.bang[0] + Math.cos(ang) * d * 2.2, 60, W - 60), y = clamp(this.bang[1] + Math.sin(ang) * d * .7, 520, H - 160); let best = 0, bd = 1e9; this.nodes.forEach((n, k) => { const dd = Math.hypot(n.x - x, n.y - y); if (dd < bd) { bd = dd; best = k; } }); this.reports.push({ s, x, y, t0: 19.2 + i * .33 + r() * .15, n: best }); });
      ev('bang', .5, 'boom', 0); this.reports.forEach((rp, i) => { if (i % 3 === 0) ev('bang', rp.t0 - 17.7 + .1, 'blip', -24); });
    },
    base(ug, shake = 0) {
      const sky = ctx.createLinearGradient(0, 0, 0, H); sky.addColorStop(0, '#1a2250'); sky.addColorStop(.55, '#6a4f7a'); sky.addColorStop(.8, '#e07a4a'); sky.addColorStop(1, '#f2b26b'); ctx.fillStyle = sky; ctx.fillRect(0, 0, W, H);
      glow(1500, 500, 420, 'rgba(255,200,120,.35)');
      ctx.save(); if (shake) ctx.translate((Math.random() - .5) * shake, (Math.random() - .5) * shake);
      const pan = ug * 6;
      const cols = ['#0f2a1f', '#1a4a2e', '#2b6a3e'];
      for (let L = 2; L >= 0; L--) { ctx.fillStyle = cols[L]; ctx.beginPath(); ctx.moveTo(0, H); for (let x = 0; x <= W; x += 16) ctx.lineTo(x, this.hill(x + pan * (3 - L) * .3, 0)[L]); ctx.lineTo(W, H); ctx.fill();
        if (L === 1) { // the Pyramid stage on the middle hill
          const sx = this.stage[0], sy = this.hill(sx + pan * .6, 0)[1] - 80; this.stage[1] = sy; ctx.fillStyle = '#d9d9d9'; ctx.beginPath(); ctx.moveTo(sx - 150, sy + 90); ctx.lineTo(sx, sy - 120); ctx.lineTo(sx + 150, sy + 90); ctx.closePath(); ctx.fill(); ctx.fillStyle = '#9fa5ad'; ctx.beginPath(); ctx.moveTo(sx - 150, sy + 90); ctx.lineTo(sx, sy - 120); ctx.lineTo(sx, sy + 90); ctx.closePath(); ctx.fill();
          for (let i = 0; i < 5; i++) { ctx.globalAlpha = .18; ctx.fillStyle = ['#ff5bd6', '#5bd4ff', '#ffe95b'][i % 3]; ctx.beginPath(); ctx.moveTo(sx, sy - 100); const a = -Math.PI / 2 + Math.sin(ug * .8 + i) * .7; ctx.lineTo(sx + Math.cos(a) * 1200 - 60, sy - 100 + Math.sin(a) * 1200); ctx.lineTo(sx + Math.cos(a) * 1200 + 60, sy - 100 + Math.sin(a) * 1200); ctx.fill(); ctx.globalAlpha = 1; }
          const r2 = rng(5); for (let i = 0; i < 700; i++) { const px = sx - 280 + r2() * 560, d = Math.abs(px - sx) / 280; ctx.fillStyle = `rgba(20,20,30,${.5 + r2() * .5})`; ctx.fillRect(px, this.hill(px + pan * .6, 0)[1] + 4 + r2() * (70 - 50 * d), 3, 3); }
        }
        for (const t of this.tents) { if (t.layer !== L) continue; const x = t.x - pan * (3 - L) * .3; ctx.fillStyle = t.c; ctx.beginPath(); ctx.moveTo(x - t.s, t.y); ctx.lineTo(x, t.y - t.s * 1.1); ctx.lineTo(x + t.s, t.y); ctx.closePath(); ctx.fill(); }
      }
      ctx.restore();
    },
    nodesAndLinks(ug) {
      ctx.setLineDash([8, 10]); ctx.strokeStyle = C.lora; ctx.lineWidth = 2.5;
      for (const [a, b] of this.links) { const na = this.nodes[a], nb = this.nodes[b]; const k = ramp(ug, Math.max(na.t0, nb.t0) + .3, Math.max(na.t0, nb.t0) + 1); if (k <= 0) continue; ctx.globalAlpha = .6 * k; ctx.beginPath(); ctx.moveTo(na.x, na.y); ctx.quadraticCurveTo((na.x + nb.x) / 2, Math.min(na.y, nb.y) - 120, nb.x, nb.y); ctx.stroke(); }
      ctx.setLineDash([]); ctx.globalAlpha = 1;
      for (const n of this.nodes) { node(n.x, n.y, ug, n.t0, C.lora, 9); if (ug > n.t0 + 1) pulse(n.x, n.y, ug, n.t0 + 1, 'rgba(242,165,65,.35)', 4, 120, 1); }
    },
    signal(ug) { // the mobile network giving up, top right
      const bars = 4 - Math.floor(ramp(ug, 2, 5.5) * 4.99); const x = W - 120, y = 70;
      for (let i = 0; i < 4; i++) { ctx.fillStyle = i < bars ? C.ink : '#4a4f5a'; ctx.fillRect(x + i * 16, y + 28 - i * 8, 11, 8 + i * 8); }
      text(bars === 0 ? 'No service' : 'EE', x - 12, y + 18, 20, bars === 0 ? C.alert : C.ink, 'right', 600);
    },
  };
  scenes.glasto = {
    setup() { glasto.setup(); for (let i = 0; i < 4; i++) ev('glasto', 4.6 + i * .5, 'blip', -22); },
    draw(u, len) { glasto.base(u); glasto.nodesAndLinks(u); glasto.signal(u); label('Glastonbury', 'Worthy Farm, Somerset: 210,000 people on a hill'); vignette(); fade(1 - ramp(u, 0, .6)); },
  };
  scenes.lostchild = {
    setup() {},
    draw(u) {
      const ug = u + 12.4; glasto.base(ug); glasto.nodesAndLinks(ug); glasto.signal(ug);
      for (const [i, t] of glasto.alertAt) { const n = glasto.nodes[i]; ring(n.x, n.y, ug, t, 1.2, 160, C.alert, 4); const k = ramp(ug, t, t + .3); if (k > 0) { ctx.fillStyle = C.alert; ctx.globalAlpha = k; rr(n.x - 34, n.y - 58, 68, 22, 5); ctx.fill(); text('ALERT', n.x, n.y - 47, 14, '#fff', 'center', 800); ctx.globalAlpha = 1; } }
      const s0 = glasto.nodes[0]; bubble('⚠ Lost child, red jacket, about 6, last seen by the Pyramid', s0.x, s0.y - 60, ug, 12.6, { fill: C.alert, color: '#fff' });
      const last = Math.max(...glasto.alertAt.values()); const k = ramp(ug, last + .3, last + .8); if (k > 0) { ctx.globalAlpha = k; text('every camp alerted', W - 100, 200, 30, C.alert, 'right', 700); text(`${Math.round((last - 12.9) * 12 + 5)} s`, W - 100, 250, 56, C.ink, 'right', 700, MONO); ctx.globalAlpha = 1; }
      label('Glastonbury', 'a High message from the welfare tent'); vignette();
    },
  };
  scenes.bang = {
    setup() {},
    draw(u, len) {
      const ug = u + 17.7; const [bx, by] = glasto.bang; const since = ug - 18.2;
      glasto.base(ug, since > 0 && since < .6 ? 26 * (1 - since / .6) : 0); glasto.nodesAndLinks(ug); glasto.signal(ug);
      if (since > 0) { glow(bx, by, 320 * Math.min(1, since * 3), 'rgba(255,230,180,.95)', clamp(1 - since / .5)); ring(bx, by, ug, 18.2, 1.2, 900, 'rgba(255,220,160,.9)', 10); ring(bx, by, ug, 18.3, 2, 1400, 'rgba(255,200,120,.5)', 6);
        // smoke: drifting discs
        const r = rng(9); for (let i = 0; i < 26; i++) { const age = since - i * .12; if (age < 0) continue; ctx.fillStyle = `rgba(60,60,70,${clamp(.35 - age * .04)})`; ctx.beginPath(); ctx.arc(bx + (r() - .3) * 40 + age * (18 + r() * 10), by - age * (26 + r() * 18), 20 + age * 14, 0, 7); ctx.fill(); }
        glow(bx, by, 60 + Math.min(240, Math.max(0, since - 1) * 30), 'rgba(238,75,58,.55)', clamp((since - 1) / 2) * .9); // the reports' heat
      }
      for (const rp of glasto.reports) { const n = glasto.nodes[rp.n]; dot(rp.x, rp.y - 20, n.x, n.y, (ug - rp.t0 - .3) / .6, C.wifi, 4); bubble(rp.s, rp.x, rp.y, ug, rp.t0, { px: 19, alpha: 1 - ramp(ug, rp.t0 + 3.5, rp.t0 + 4.5) }); }
      const n = glasto.reports.filter((rp) => ug > rp.t0).length; if (n > 0) { text(`${n} reports`, W - 100, 200, 30, C.ink, 'right', 700); text('all converge on the west field', W - 100, 240, 22, C.sub, 'right'); }
      const mk = ramp(ug, 23.6, 27); if (mk > 0) { const s0 = glasto.nodes[0]; const x = lerp(s0.x, bx + 30, easeInOut(mk)), y = lerp(s0.y + 20, by + 10, easeInOut(mk)); ctx.fillStyle = '#fff'; rr(x - 16, y - 16, 32, 32, 6); ctx.fill(); ctx.fillStyle = C.alert; ctx.fillRect(x - 3, y - 11, 6, 22); ctx.fillRect(x - 11, y - 3, 22, 6); text('medics', x, y + 30, 18, '#fff', 'center', 700); }
      label('Glastonbury', 'a bang by the west field, 23:40'); vignette(); fade(ramp(u, len - .5, len));
    },
  };

  // --- Burning Man: Black Rock City from above, eight nodes, a whiteout ---
  scenes.burningman = {
    setup() {
      const r = rng(13); this.camps = []; const cx = 960, cy = 905; this.c = [cx, cy];
      for (let ri = 0; ri < 11; ri++) { const rad = 330 + ri * 38; for (let a = -150; a <= -30; a += 1.3 + r() * 2) { const ang = a * Math.PI / 180; if (r() < .8) this.camps.push({ x: cx + Math.cos(ang) * rad, y: cy + Math.sin(ang) * rad, w: 4 + r() * 6, h: 4 + r() * 5, c: `rgba(${90 + r() * 80 | 0},${70 + r() * 60 | 0},${50 + r() * 40 | 0},1)` }); } }
      this.nodes = [-140, -120, -100, -85, -70, -55, -40, -130].map((a, i) => { const ang = a * Math.PI / 180, rad = i === 7 ? 360 : 520 + (i % 3) * 80; return { x: cx + Math.cos(ang) * rad, y: cy + Math.sin(ang) * rad, t0: 2.2 + i * .35 }; });
      this.alertAt = this.nodes.map((n, i) => 9.2 + i * .28);
      this.dust = Array.from({ length: 900 }, () => ({ x: r(), y: r(), s: 1 + r() * 3, v: .6 + r() * .8 }));
      ev('burningman', 2.4, 'swell', -14); ev('burningman', 9.2, 'alert', -8); for (let i = 1; i < 8; i++) ev('burningman', 9.2 + i * .28, 'blip', -22);
    },
    draw(u, len) {
      const [cx, cy] = this.c; const z = 1 + .12 * easeInOut(ramp(u, 0, len));
      ctx.fillStyle = '#cdb78d'; ctx.fillRect(0, 0, W, H);
      ctx.save(); ctx.translate(cx, cy); ctx.scale(z, z); ctx.translate(-cx, -cy);
      const r = rng(21); ctx.fillStyle = 'rgba(0,0,0,.05)'; for (let i = 0; i < 2500; i++) ctx.fillRect(r() * W, r() * H, 2, 2);
      // the streets: arcs and radials from 2:00 to 10:00 round the Man
      ctx.strokeStyle = 'rgba(110,90,60,.7)'; for (let ri = 0; ri < 12; ri++) { ctx.lineWidth = ri === 0 ? 6 : 2; ctx.beginPath(); ctx.arc(cx, cy, 300 + ri * 38, -150 * Math.PI / 180, -30 * Math.PI / 180); ctx.stroke(); }
      for (let a = -150; a <= -30; a += 7.5) { const ang = a * Math.PI / 180; ctx.lineWidth = a % 30 === 0 ? 3 : 1.2; ctx.beginPath(); ctx.moveTo(cx + Math.cos(ang) * 300, cy + Math.sin(ang) * 300); ctx.lineTo(cx + Math.cos(ang) * 730, cy + Math.sin(ang) * 730); ctx.stroke(); }
      for (const c of this.camps) { ctx.fillStyle = c.c; ctx.fillRect(c.x - c.w / 2, c.y - c.h / 2, c.w, c.h); }
      // the Man, the temple, the trash fence
      ctx.fillStyle = '#4a3a2a'; ctx.beginPath(); ctx.arc(cx, cy, 10, 0, 7); ctx.fill(); ctx.fillRect(cx - 2, cy - 30, 4, 30); ctx.fillRect(cx - 14, cy - 24, 28, 4); text('the Man', cx, cy + 30, 18, '#4a3a2a', 'center', 600);
      ctx.setLineDash([6, 10]); ctx.strokeStyle = 'rgba(90,70,50,.6)'; ctx.lineWidth = 2; ctx.beginPath(); ctx.moveTo(cx - 900, cy - 60); ctx.lineTo(cx - 560, cy - 780); ctx.lineTo(cx + 560, cy - 780); ctx.lineTo(cx + 900, cy - 60); ctx.stroke(); ctx.setLineDash([]);
      // LoRa coverage: each node reaches the whole city
      for (const n of this.nodes) { const k = easeOut(ramp(u, n.t0, n.t0 + 1.8)); if (k > 0) { ctx.fillStyle = 'rgba(242,165,65,.07)'; ctx.beginPath(); ctx.arc(n.x, n.y, 560 * k, 0, 7); ctx.fill(); ctx.strokeStyle = 'rgba(242,165,65,.35)'; ctx.lineWidth = 2; ctx.stroke(); } }
      for (const n of this.nodes) node(n.x, n.y, u, n.t0, C.lora, 10);
      // the whiteout: a wall of dust from the north-west, and the alert ahead of it
      const dk = ramp(u, 7.5, 15.5); const front = -500 + dk * 2900; // the front's x at y = 0, sloping
      if (dk > 0) { ctx.save(); const g = ctx.createLinearGradient(front - 500, 0, front + 150, 0); g.addColorStop(0, 'rgba(226,206,160,1)'); g.addColorStop(.7, 'rgba(226,206,160,.85)'); g.addColorStop(1, 'rgba(226,206,160,0)'); ctx.translate(0, 0); ctx.transform(1, 0, -.35, 1, 0, 0); ctx.fillStyle = g; ctx.fillRect(-3000, -100, front + 3150, H + 200); ctx.restore();
        for (const d of this.dust) { const x = ((d.x * 2200 - 1000 + u * 160 * d.v) % 2200) - 300 + front - 2200 * .5; const y = d.y * H; if (x < front + 150 - y * .35) { ctx.fillStyle = `rgba(240,225,190,${.5 + .5 * d.v - .3})`; ctx.fillRect(x, y, d.s * 2, d.s); } } }
      ctx.restore();
      this.alertAt.forEach((t, i) => { const n = this.nodes[i]; const sx = cx + (n.x - cx) * z, sy = cy + (n.y - cy) * z; ring(sx, sy, u, t, 1.4, 240, C.alert, 5); const k = ramp(u, t, t + .3); if (k > 0) { ctx.globalAlpha = k; ctx.fillStyle = C.alert; rr(sx - 36, sy - 60, 72, 24, 6); ctx.fill(); text('ALERT', sx, sy - 48, 15, '#fff', 'center', 800); ctx.globalAlpha = 1; } });
      const n0 = this.nodes[0]; bubble('⚠ Whiteout coming from the north-west. Stay in camp.', cx + (n0.x - cx) * z, cy + (n0.y - cy) * z - 62, u, 9.0, { fill: C.alert, color: '#fff' });
      ctx.fillStyle = '#3a2d20'; ctx.fillRect(W - 420, H - 160, 300, 4); ctx.fillRect(W - 420, H - 170, 4, 14); ctx.fillRect(W - 120, H - 170, 4, 14); text('4 km', W - 270, H - 185, 22, '#3a2d20', 'center', 700);
      const ak = ramp(u, 11.6, 12.2); if (ak > 0) { ctx.globalAlpha = ak; text('alert delivered: 8 nodes, 80,000 people', W - 100, 200, 28, '#3a2d20', 'right', 700); ctx.globalAlpha = 1; }
      ctx.fillStyle = 'rgba(58,45,32,.9)'; text('BLACK ROCK CITY', 64, 62, 22, '#3a2d20', 'left', 800); ctx.fillStyle = '#3a2d20'; ctx.fillRect(64, 82, 180, 3); text('Nevada: 80,000 people, 2:00 to 10:00 round the Man', 64, 110, 20, '#5a4a38');
      const vg = ctx.createRadialGradient(W / 2, H / 2, H * .4, W / 2, H / 2, H); vg.addColorStop(0, 'rgba(0,0,0,0)'); vg.addColorStop(1, 'rgba(60,40,20,.45)'); ctx.fillStyle = vg; ctx.fillRect(0, 0, W, H);
      fade(1 - ramp(u, 0, .6)); fade(ramp(u, len - .5, len));
    },
  };

  // --- proof: the simulator with the real code ---
  scenes.proof = {
    setup() {
      const r = rng(17); this.houses = []; const cs = [[500, 380], [1130, 330]];
      for (let i = 0; i < 40; i++) { const [cx, cy] = cs[i % 2]; const a = r() * 7, d = 60 + r() * 230; this.houses.push({ x: cx + Math.cos(a) * d * 1.3, y: cy + Math.sin(a) * d, r: 2 + r() * 4 }); }
      this.links = []; for (let a = 0; a < 40; a++) for (let b = a + 1; b < 40; b++) if (Math.hypot(this.houses[a].x - this.houses[b].x, this.houses[a].y - this.houses[b].y) < 150) this.links.push([a, b]);
      this.school = [760, 370];
      this.beams = Array.from({ length: 160 }, (_, i) => { const [a, b] = this.links[Math.floor(r() * this.links.length)]; return { a, b, t0: 1.8 + i * .055 + r() * .05 }; });
      ev('proof', 1.5, 'boom', -16); for (let i = 0; i < 8; i++) ev('proof', 2 + i * .5, 'blip', -26);
    },
    draw(u, len) {
      ctx.fillStyle = C.bg; ctx.fillRect(0, 0, W, H);
      ctx.strokeStyle = 'rgba(111,182,234,.25)'; ctx.lineWidth = 1; for (const [a, b] of this.links) { ctx.beginPath(); ctx.moveTo(this.houses[a].x, this.houses[a].y); ctx.lineTo(this.houses[b].x, this.houses[b].y); ctx.stroke(); }
      const [sx, sy] = this.school; ring(sx, sy, u, 1.5, 1.6, 420, C.warm, 6); glow(sx, sy, 90, 'rgba(255,210,122,.8)', clamp(1 - (u - 1.5) / 1.5));
      for (const h of this.houses) { ctx.fillStyle = '#243041'; rr(h.x - 9, h.y - 9, 18, 18, 3); ctx.fill(); ctx.fillStyle = C.wifi; ctx.beginPath(); ctx.arc(h.x, h.y, 2.5, 0, 7); ctx.fill(); }
      ctx.strokeStyle = C.wifi; ctx.lineWidth = 2.5;
      for (const b of this.beams) { const k = (u - b.t0) / .4; if (k < 0 || k > 1) continue; const A = this.houses[b.a], B = this.houses[b.b]; ctx.globalAlpha = 1 - k; ctx.beginPath(); ctx.moveTo(A.x, A.y); ctx.lineTo(lerp(A.x, B.x, Math.min(1, k * 2)), lerp(A.y, B.y, Math.min(1, k * 2))); ctx.stroke(); }
      ctx.globalAlpha = 1;
      text('bang.json: 40 houses, everyone near the school posts within 30 s', 64, 160, 22, C.sub, 'left');
      // the tiles
      const tile = (x, y, v, unit, t0, fmt = (n) => Math.round(n).toLocaleString('en-GB')) => { const k = ramp(u, t0, t0 + 2); if (k <= 0) return; ctx.globalAlpha = Math.min(1, k * 3); ctx.fillStyle = '#121a2a'; rr(x, y, 330, 120, 12); ctx.fill(); text(typeof v === 'number' ? fmt(easeOut(k) * v) : v, x + 24, y + 48, 48, C.ink, 'left', 700, MONO); text(unit, x + 24, y + 94, 20, C.sub); ctx.globalAlpha = 1; };
      tile(1480, 160, 247, 'posts, real signed entries', 2.2); tile(1480, 300, 8968, 'deliveries', 2.6); tile(1480, 440, 12, 'ms median delivery', 3.0, (n) => Math.round(n)); tile(1480, 580, 0, 'missing  ·  0 late', 3.4);
      const ck = ramp(u, 7.6, 8.2); if (ck > 0) { ctx.globalAlpha = ck; ctx.fillStyle = '#10301f'; rr(1480, 720, 330, 64, 12); ctx.fill(); text('✓  CI: six scenarios pass', 1500, 752, 24, C.ok, 'left', 700); ctx.globalAlpha = 1; }
      const im = images['village-sim-dark']; const wk = easeOut(ramp(u, 5.6, 6.6)); if (im && wk > 0) { const w = 560, h = w * im.height / im.width, x = 320 + (wk - 1) * 200, y = H - h - 190; ctx.globalAlpha = wk; ctx.shadowColor = 'rgba(0,0,0,.6)'; ctx.shadowBlur = 40; ctx.drawImage(im, x, y, w, h); ctx.shadowBlur = 0; text('the simulator, in your browser: johnusher.github.io/batnet', x + w / 2, y + h + 26, 20, C.sub, 'center'); ctx.globalAlpha = 1; }
      label('The real code, simulated', 'batnetd -sim: the same Go code the Pi runs, in virtual time, on every push'); vignette(); fade(1 - ramp(u, 0, .5)); fade(ramp(u, len - .5, len));
    },
  };

  // --- close ---
  scenes.close = {
    setup() { const r = rng(23); this.lights = Array.from({ length: 160 }, () => ({ x: r() * W, y: r() * H, f: r() * 6, a: .15 + r() * .4 })); ev('close', 5.2, 'swell', -14); },
    draw(u, len) {
      ctx.fillStyle = C.bg; ctx.fillRect(0, 0, W, H);
      for (const l of this.lights) { ctx.fillStyle = C.wifi; ctx.globalAlpha = l.a * (.7 + .3 * Math.sin(u * 2 + l.f)) * ramp(u, 0, 2); ctx.beginPath(); ctx.arc(l.x, l.y, 1.5, 0, 7); ctx.fill(); }
      ctx.globalAlpha = 1;
      const cx = W / 2; ctx.globalAlpha = easeOut(ramp(u, .2, 1.2)); font(150, 700); ctx.letterSpacing = '-3px'; ctx.fillStyle = C.ink; ctx.textAlign = 'center'; ctx.textBaseline = 'middle'; ctx.fillText('batnet', cx, 330); ctx.letterSpacing = '0px';
      ctx.globalAlpha = ramp(u, 1.2, 2); text('Open source (MIT)  ·  github.com/johnusher/batnet', cx, 460, 34, C.ink, 'center', 500);
      ctx.globalAlpha = ramp(u, 2.0, 2.8); text('Try the simulator in your browser: johnusher.github.io/batnet', cx, 520, 28, C.sub, 'center');
      ctx.globalAlpha = ramp(u, 3.0, 3.8); text('Next: the field. Three to five houses, two radios, one bang.', cx, 575, 28, C.sub, 'center');
      const hs = [[cx - 320, 760], [cx - 160, 800], [cx, 740], [cx + 160, 800], [cx + 320, 760]];
      hs.forEach(([x, y], i) => { const t0 = 4.4 + i * .3; const k = ramp(u, t0, t0 + .4); if (k > 0) { ctx.globalAlpha = k; house(x, y, 22, '#2c3a4e'); node(x, y - 20, u, t0 + .2, i === 2 ? C.lora : C.wifi, 7); } });
      ctx.globalAlpha = .5 * ramp(u, 5.6, 6.2); ctx.strokeStyle = C.wifi; ctx.lineWidth = 2; for (let i = 1; i < hs.length; i++) { ctx.beginPath(); ctx.moveTo(hs[i - 1][0], hs[i - 1][1] - 20); ctx.lineTo(hs[i][0], hs[i][1] - 20); ctx.stroke(); }
      ctx.globalAlpha = 1; vignette(); fade(1 - ramp(u, 0, .5)); fade(ramp(u, len - 1.6, len - .2));
    },
  };

  for (const s of tl.scenes) if (scenes[s.id] && scenes[s.id].setup) scenes[s.id].setup();
  events.sort((a, b) => a.t - b.t);

  // ---------- one frame ----------
  function draw(t) {
    ctx.setTransform(S, 0, 0, S, 0, 0);
    const s = tl.scenes.find((x) => t >= x.start && t < x.end) || tl.scenes[tl.scenes.length - 1];
    const u = t - s.start, len = s.end - s.start;
    ctx.fillStyle = '#000'; ctx.fillRect(0, 0, W, H);
    const sc = scenes[s.id];
    if (sc) sc.draw(u, len); else { text(s.id, W / 2, H / 2, 60, C.sub, 'center'); }
    caption(s, u);
  }

  // ---------- render every frame to the local server, or preview in real time ----------
  if (RENDER) {
    const total = Math.ceil(tl.total * tl.fps);
    for (let i = 0; i < total; i++) {
      draw(i / tl.fps);
      const blob = await new Promise((ok) => cv.toBlob(ok, 'image/png'));
      await fetch('/frame/' + i, { method: 'POST', body: blob });
    }
    await post('/events', JSON.stringify(events));
    await post('/done', '');
  } else {
    const ui = document.getElementById('ui'); ui.hidden = false;
    let playing = false, t = 0, last = 0; const audio = {};
    for (const s of tl.scenes) { const a = new Audio('out/voice/' + s.clip); audio[s.id] = a; }
    const list = document.getElementById('scenes'); for (const s of tl.scenes) { const a = document.createElement('a'); a.textContent = s.id; a.onclick = () => { t = s.start; for (const x of Object.values(audio)) x.pause(); }; list.append(a); }
    document.getElementById('play').onclick = () => { playing = !playing; document.getElementById('play').textContent = playing ? 'Pause' : 'Play'; if (!playing) for (const x of Object.values(audio)) x.pause(); };
    document.addEventListener('keydown', (e) => { if (e.key === ' ') document.getElementById('play').click(); });
    let cur = null;
    function frame(now) {
      const dt = Math.min(.1, (now - last) / 1000); last = now;
      if (playing) { t += dt; if (t >= tl.total) { t = 0; } }
      const s = tl.scenes.find((x) => t >= x.start && t < x.end);
      if (playing && s && cur !== s.id) { cur = s.id; const a = audio[s.id]; a.currentTime = Math.max(0, t - s.start); a.play().catch(() => {}); }
      if (!playing) cur = null;
      draw(t); document.getElementById('clock').textContent = t.toFixed(1) + ' s · ' + (s ? s.id : '');
      requestAnimationFrame(frame);
    }
    requestAnimationFrame(frame);
  }
})();
