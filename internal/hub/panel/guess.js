// guess.js works out where a radio that shares no place (or only a very rough
// one) probably is: the radios whose place is known and whose packets reach
// this hub the same way (through the same last relays, as many relays) are
// spread over a grid, and a shared rough area cuts it down. Tried on the
// roof hub's radios whose place is known, hiding each in turn: the light
// patch held the radio three times in four (knowledge/tests,
// 2026-10-25-guessing-where-radios-are). The panel draws it; node tests it.
"use strict";
const Guess = (() => {
  const CELL = 500;    // m, the grid's squares
  const SPREAD = 1500; // m, how far each known radio's place is smeared
  const EPS = 0.2;     // weight of a known radio that shares no last relay with it
  const HOPS = 2;      // how fast the weight falls with a different number of relays
  const EVEN = 0.1;    // share spread evenly, for a radio unlike any known one
  const MAXR = 60000;  // m, the farthest the grid reaches from the hub
  const M = 111320;    // m per degree of latitude

  // grid lays squares over the hub's surroundings, as far as the known radios
  // ({lat, lon, acc: the place's accuracy, m}) and a margin and the far
  // corners of rough areas (areas: {lat, lon, accuracyM}), and smears each
  // known radio's place over them once.
  function grid(hub, known, areas = []) {
    const kLon = M * Math.cos((hub.lat * Math.PI) / 180);
    const xy = (p) => [(p.lon - hub.lon) * kLon, (p.lat - hub.lat) * M];
    let r = 10000;
    for (const k of known) r = Math.max(r, Math.hypot(...xy(k)) + 3 * SPREAD);
    for (const a of areas) {
      const [x, y] = xy(a), h = a.accuracyM;
      r = Math.max(r, Math.hypot(Math.abs(x) + (h * kLon) / M, Math.abs(y) + h));
    }
    r = Math.min(r, MAXR);
    const n = 2 * Math.ceil(r / CELL);
    const g = { hub, kLon, n, r, inside: new Uint8Array(n * n), kernels: [] };
    let inside = 0;
    for (let i = 0; i < n; i++) for (let j = 0; j < n; j++) {
      const x = (j + 0.5) * CELL - r, y = r - (i + 0.5) * CELL;
      if (Math.hypot(x, y) <= r) { g.inside[i * n + j] = 1; inside++; }
    }
    g.insideCount = inside;
    for (const k of known) {
      // A rough place is the middle of a square of the grid Meshtastic
      // rounds to (13 bits, the apps' "approximate": 5.8 by 3.6 km here),
      // and many radios share one middle: the radio is anywhere in it.
      const [x, y] = xy(k), hy = k.acc || 0, hx = ((k.acc || 0) * kLon) / M;
      const wi = Math.ceil((3 * SPREAD + hy) / CELL), wj = Math.ceil((3 * SPREAD + hx) / CELL);
      const ci = Math.floor((r - y) / CELL), cj = Math.floor((x + r) / CELL);
      const idx = [], val = [];
      let sum = 0;
      for (let i = ci - wi; i <= ci + wi; i++) for (let j = cj - wj; j <= cj + wj; j++) {
        if (i < 0 || j < 0 || i >= n || j >= n || !g.inside[i * n + j]) continue;
        const dx = Math.max(Math.abs((j + 0.5) * CELL - r - x) - hx, 0), dy = Math.max(Math.abs(r - (i + 0.5) * CELL - y) - hy, 0);
        const v = Math.exp(-(dx * dx + dy * dy) / (2 * SPREAD * SPREAD));
        idx.push(i * n + j); val.push(v); sum += v;
      }
      if (sum > 0) g.kernels.push({ k, idx, val: val.map((v) => v / sum) });
    }
    return g;
  }

  // weight says how alike two radios reach the hub: the share of last relays
  // they have in common (cosine of the packet counts), and as many relays.
  function weight(a, b) {
    let dot = 0, na = 0, nb = 0;
    for (const v of Object.values(a.via || {})) na += v * v;
    for (const v of Object.values(b.via || {})) nb += v * v;
    for (const [r, v] of Object.entries(a.via || {})) dot += v * ((b.via || {})[r] || 0);
    const cos = na && nb ? dot / Math.sqrt(na * nb) : 0;
    const hops = a.hops >= 0 && b.hops >= 0 ? Math.exp(-((a.hops - b.hops) ** 2) / (2 * HOPS * HOPS)) : 1;
    return (EPS + cos) * hops;
  }

  // where returns the guess for one radio t ({via, hops, rough: {lat, lon,
  // accuracyM} or null}): how likely each square is, the levels holding half
  // and four in five of it, and the likeliest square.
  function where(g, t) {
    const n = g.n, p = new Float64Array(n * n);
    let wsum = 0;
    for (const kr of g.kernels) {
      const w = weight(t, kr.k);
      wsum += w;
      for (let q = 0; q < kr.idx.length; q++) p[kr.idx[q]] += w * kr.val[q];
    }
    const even = EVEN / g.insideCount;
    for (let q = 0; q < p.length; q++) p[q] = g.inside[q] ? (wsum ? ((1 - EVEN) * p[q]) / wsum : 0) + even : 0;
    if (t.rough) { // a shared rough area: the radio is in that square of the map, nowhere else
      // Squares the area's edge cuts count as in: half a square more each side.
      const hLat = t.rough.accuracyM / M + CELL / 2 / M, hLon = t.rough.accuracyM / M + CELL / 2 / g.kLon;
      for (let i = 0; i < n; i++) for (let j = 0; j < n; j++) {
        const c = center(g, i, j);
        if (Math.abs(c.lat - t.rough.lat) > hLat || Math.abs(c.lon - t.rough.lon) > hLon) p[i * n + j] = 0;
      }
    }
    let total = 0;
    for (const v of p) total += v;
    if (!total) return null; // a rough area wholly off the grid
    let best = 0;
    for (let q = 0; q < p.length; q++) { p[q] /= total; if (p[q] > p[best]) best = q; }
    const sorted = Float64Array.from(p).sort().reverse();
    const level = (mass) => { let c = 0; for (const v of sorted) { c += v; if (c >= mass) return v; } return 0; };
    return { p, half: level(0.5), most: level(0.8), peak: center(g, Math.floor(best / n), best % n) };
  }

  function center(g, i, j) {
    return { lat: g.hub.lat + (g.r - (i + 0.5) * CELL) / M, lon: g.hub.lon + ((j + 0.5) * CELL - g.r) / g.kLon };
  }

  // bounds are the grid's corners, south-west and north-east, for the map.
  function bounds(g) {
    return [[g.hub.lat - g.r / M, g.hub.lon - g.r / g.kLon], [g.hub.lat + g.r / M, g.hub.lon + g.r / g.kLon]];
  }

  // spread says where the guess lies from the hub: the distances holding the
  // middle four in five of it, the narrowest arc of the 16-point compass
  // holding four in five (from, to; null: no clear direction) and the light
  // patch's area, m².
  function spread(g, r) {
    const n = g.n, byD = [], sector = new Array(16).fill(0);
    let area = 0;
    for (let i = 0; i < n; i++) for (let j = 0; j < n; j++) {
      const v = r.p[i * n + j];
      if (!v) continue;
      const x = (j + 0.5) * CELL - g.r, y = g.r - (i + 0.5) * CELL;
      byD.push([Math.hypot(x, y), v]);
      sector[Math.round(((Math.atan2(x, y) * 180) / Math.PI + 360) / 22.5) % 16] += v;
      if (v >= r.most) area += CELL * CELL;
    }
    byD.sort((a, b) => a[0] - b[0]);
    let c = 0, near = 0, far = 0;
    for (const [d, v] of byD) {
      if (c < 0.1 && c + v >= 0.1) near = d;
      if (c < 0.9 && c + v >= 0.9) far = d;
      c += v;
    }
    let best = null;
    for (let s = 0; s < 16; s++) {
      let m = 0, len = 0;
      while (m < 0.8 && len < 16) m += sector[(s + len++) % 16];
      if (m >= 0.8 && (!best || len < best.len)) best = { s, len };
    }
    const clear = best && best.len <= 6; // at most 135° wide
    return { near, far, area, from: clear ? best.s : null, to: clear ? (best.s + best.len - 1) % 16 : null };
  }

  return { grid, where, bounds, spread, weight, CELL };
})();
if (typeof module !== "undefined") module.exports = Guess;
