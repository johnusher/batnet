// The "real batnet code" view: batnet's Go code, compiled to WebAssembly,
// runs a scenario's village in virtual time; this file draws it.
(() => {
  'use strict';
  const $ = (s) => document.querySelector(s);
  const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const css = (v) => getComputedStyle(document.documentElement).getPropertyValue(v).trim();
  let ready = null, running = false, speed = 10, last = 0, state = null, selected = null, scenarios = [], scenarioJSON = null;
  const beams = [];

  function boot() {
    if (ready) return ready;
    if (location.protocol === 'file:') {
      ready = Promise.reject(new Error('The real engine loads batnet.wasm, which browsers block on file:// pages. Run: python3 -m http.server, then open http://localhost:8000.'));
      return ready;
    }
    ready = (async () => {
      const go = new Go();
      const res = await WebAssembly.instantiateStreaming(fetch('batnet.wasm'), go.importObject);
      go.run(res.instance);
      scenarios = await (await fetch('scenarios/index.json')).json();
    })();
    return ready;
  }

  async function loadScenario(file) {
    scenarioJSON = await (await fetch('scenarios/' + file)).text();
    const err = batnetSim.load(scenarioJSON);
    if (err) throw new Error(err);
    let about = '';
    try { about = JSON.parse(scenarioJSON).about || ''; } catch (e) {}
    $('#rv-about').textContent = about;
    selected = null; beams.length = 0;
    pull();
  }

  function pull() {
    state = JSON.parse(batnetSim.state());
    const now = performance.now();
    for (const f of state.frames || []) beams.push({ from: f.from, to: f.to, t0: now });
    if (beams.length > 400) beams.splice(0, beams.length - 400);
    renderPanels();
  }

  // ---------- map ----------
  function drawMap() {
    const cv = $('#rv-map'), ctx = cv.getContext('2d');
    const dpr = window.devicePixelRatio || 1, W = cv.clientWidth, H = cv.clientHeight;
    if (cv.width !== W * dpr) { cv.width = W * dpr; cv.height = H * dpr; }
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, W, H);
    if (!state || !state.nodes) return;
    const xs = state.nodes.map((n) => n.x), ys = state.nodes.map((n) => n.y);
    const minX = Math.min(...xs) - 60, maxX = Math.max(...xs) + 60, minY = Math.min(...ys) - 60, maxY = Math.max(...ys) + 60;
    const s = Math.min(W / (maxX - minX), H / (maxY - minY));
    const ox = (W - (maxX - minX) * s) / 2, oy = (H - (maxY - minY) * s) / 2;
    const px = (n) => [ox + (n.x - minX) * s, oy + (n.y - minY) * s];
    const byName = new Map(state.nodes.map((n) => [n.name, n]));
    ctx.lineWidth = 1.5; ctx.strokeStyle = css('--wifi'); ctx.globalAlpha = 0.45;
    for (const [a, b] of state.links || []) {
      const A = byName.get(a), B = byName.get(b);
      if (!A.on || !B.on) continue;
      const [x1, y1] = px(A), [x2, y2] = px(B);
      ctx.beginPath(); ctx.moveTo(x1, y1); ctx.lineTo(x2, y2); ctx.stroke();
    }
    ctx.globalAlpha = 1;
    const now = performance.now();
    ctx.strokeStyle = css('--wifi'); ctx.lineWidth = 2;
    for (let i = beams.length - 1; i >= 0; i--) {
      const b = beams[i], age = Math.max(0, (now - b.t0) / 1200);
      if (age > 1) { beams.splice(i, 1); continue; }
      const A = byName.get(b.from); if (!A) continue;
      const [x1, y1] = px(A);
      ctx.globalAlpha = 0.8 * (1 - age);
      for (const t of b.to) {
        const B = byName.get(t); if (!B) continue;
        const [x2, y2] = px(B), k = Math.min(1, age * 3);
        ctx.beginPath(); ctx.moveTo(x1, y1); ctx.lineTo(x1 + (x2 - x1) * k, y1 + (y2 - y1) * k); ctx.stroke();
      }
    }
    ctx.globalAlpha = 1;
    ctx.font = '600 11px ' + css('--f-mono'); ctx.textBaseline = 'middle';
    const labels = [];
    for (const n of state.nodes) {
      const [x, y] = px(n);
      const col = css('--wifi');
      if (n.kind === 'relay') {
        ctx.beginPath(); ctx.moveTo(x, y - 8); ctx.lineTo(x + 7, y + 5); ctx.lineTo(x - 7, y + 5); ctx.closePath();
        ctx.fillStyle = n.on ? css('--muted') : css('--panel'); ctx.fill();
        continue;
      }
      const r = 6 + Math.min(n.people, 10);
      ctx.fillStyle = css('--panel'); ctx.strokeStyle = n.on ? col : css('--muted'); ctx.lineWidth = n.name === selected ? 3.5 : 2.2;
      if (!n.on) ctx.setLineDash([3, 3]);
      ctx.beginPath(); ctx.roundRect(x - r, y - r, 2 * r, 2 * r, 3); ctx.fill(); ctx.stroke(); ctx.setLineDash([]);
      if (!n.on) { ctx.strokeStyle = css('--critical'); ctx.beginPath(); ctx.moveTo(x - 4, y - 4); ctx.lineTo(x + 4, y + 4); ctx.moveTo(x + 4, y - 4); ctx.lineTo(x - 4, y + 4); ctx.stroke(); }
      labels.push({ n, x, y, r });
    }
    const taken = labels.map((l) => [l.x - l.r, l.y - l.r, l.x + l.r, l.y + l.r]);
    const hits = (b) => taken.some((t) => b[0] < t[2] && b[2] > t[0] && b[1] < t[3] && b[3] > t[1]);
    ctx.lineJoin = 'round';
    for (const l of labels) {
      const w = ctx.measureText(l.n.name).width, h = 12, g = 4;
      const spots = [[l.x + l.r + g, l.y], [l.x - l.r - g - w, l.y], [l.x - w / 2, l.y + l.r + g + h / 2], [l.x - w / 2, l.y - l.r - g - h / 2]];
      const box = ([tx, ty]) => [tx - 2, ty - h / 2, tx + w + 2, ty + h / 2];
      const at = spots.find((sp) => !hits(box(sp)));
      if (!at) continue; // crowded: click the house to see its name
      taken.push(box(at));
      ctx.strokeStyle = css('--panel'); ctx.lineWidth = 3; ctx.strokeText(l.n.name, at[0], at[1]);
      ctx.fillStyle = l.n.on ? css('--ink') : css('--muted'); ctx.fillText(l.n.name, at[0], at[1]);
    }
    cv.onclick = (e) => {
      const rect = cv.getBoundingClientRect(), mx = e.clientX - rect.left, my = e.clientY - rect.top;
      let best = null, bd = 18;
      for (const n of state.nodes) { const [x, y] = px(n), d = Math.hypot(x - mx, y - my); if (d < bd) { bd = d; best = n; } }
      selected = best ? best.name : null; renderPanels();
    };
  }

  // ---------- panels ----------
  function renderPanels() {
    if (!state || !state.stats) return;
    const st = state.stats;
    $('#rv-clock').textContent = state.clock;
    $('#rv-progress').value = state.end ? state.now / state.end : 0;
    const ms = (v) => v >= 1000 ? (v / 1000).toFixed(1) + ' s' : Math.round(v) + ' ms';
    const tiles = [['Posts', st.posts, 'real signed entries'], ['Deliveries', st.deliveries, 'arrivals at other houses'],
      ['Delivery time', ms(st.p50), 'median · slowest 5% ' + ms(st.p95)], ['WiFi frames', st.frames, st.lost + ' lost to receivers'],
      ['Busiest channel', Math.round(st.channel * 100) + '%', 'share of a second on air']];
    $('#rv-tiles').innerHTML = tiles.map(([k, v, s]) => `<div class="tile"><div class="k">${k}</div><div class="v">${v}</div><div class="s">${s}</div></div>`).join('');
    const rep = JSON.parse(batnetSim.report());
    const done = state.now >= state.end;
    $('#rv-checks').innerHTML = (rep.Checks || []).map((c) => done
      ? `<li class="${c.Pass ? 'ok' : 'bad'}"><b>${c.Pass ? '✓' : '✗'} ${esc(c.Kind)}</b> ${esc(c.Detail)}</li>`
      : `<li><b>⋯ ${esc(c.Kind)}</b> judged when the scenario ends. So far: ${esc(c.Detail)}</li>`).join('') || '<li>No checks in this scenario.</li>';
    $('#rv-posts').innerHTML = (state.posts || []).slice().reverse().slice(0, 12).map((p) => `<li><b>${esc(p.house)}</b> ${p.prio >= 8 ? '<span class="chip warn">High</span> ' : ''}${esc(p.text)} <span class="note">· at ${p.arrived} other house${p.arrived === 1 ? '' : 's'}</span></li>`).join('');
    const n = selected && state.nodes.find((x) => x.name === selected);
    $('#rv-house').innerHTML = n ? `<h3>${esc(n.name)}</h3><p class="meta">${n.kind === 'relay' ? 'Relay' : esc(n.pi) + ' · ' + esc(n.wifi) + ' · ' + n.people + ' people'} · holds ${n.entries} entries · ${n.on ? (n.group >= 0 ? 'WiFi group ' + (n.group + 1) : '') : 'no power'}</p>
      ${n.kind === 'house' ? `<form id="rv-postform" style="display:flex;gap:6px;margin:8px 0"><input class="field" id="rv-text" placeholder="Post from ${esc(n.name)}" style="flex:1"><select class="field" id="rv-prio"><option value="5">Normal</option><option value="8">High</option><option value="2">Low</option></select><button class="btn primary">Post</button></form>` : ''}
      <button class="btn" id="rv-power">${n.on ? 'Cut the power' : 'Restore power'}</button>` : '<p class="note">Click a house or relay on the map.</p>';
    if (n) {
      $('#rv-power').onclick = () => { batnetSim.power(n.name, !n.on); pull(); };
      const f = $('#rv-postform');
      if (f) f.onsubmit = (e) => { e.preventDefault(); const t = $('#rv-text').value.trim(); if (t) { const err = batnetSim.post(n.name, t, +$('#rv-prio').value); if (err) alert(err); pull(); } };
    }
  }

  // ---------- loop ----------
  function frame(t) {
    if ($('#realview').hidden) { requestAnimationFrame(frame); return; }
    const dt = Math.min(t - (last || t), 100); last = t;
    if (running && state && state.now < state.end) { batnetSim.step(Math.round(dt * speed)); pull(); }
    drawMap();
    requestAnimationFrame(frame);
  }

  async function enter() {
    $('#rv-status').textContent = 'Loading batnet.wasm…';
    try {
      await boot();
      $('#rv-scenario').innerHTML = scenarios.map((f) => `<option>${esc(f)}</option>`).join('');
      await loadScenario(scenarios[0]);
      $('#rv-status').textContent = 'batnet\'s real Go code is running in your browser (WebAssembly).';
    } catch (e) { $('#rv-status').textContent = e.message; }
  }

  window.realEngine = { enter };
  document.addEventListener('DOMContentLoaded', () => {
    $('#rv-scenario').onchange = (e) => loadScenario(e.target.value).catch((err) => { $('#rv-status').textContent = err.message; });
    $('#rv-run').onclick = () => { running = !running; $('#rv-run').textContent = running ? 'Pause' : 'Run'; };
    $('#rv-restart').onclick = () => loadScenario($('#rv-scenario').value);
    for (const b of document.querySelectorAll('#rv-speeds button')) b.onclick = () => { speed = +b.dataset.speed; for (const x of document.querySelectorAll('#rv-speeds button')) x.setAttribute('aria-pressed', x === b); };
    requestAnimationFrame(frame);
  });
})();
