// lorabatnet hub panel: polls /hub/api/status (and api/traffic) and draws them. Plain JS, no build step.
"use strict";

const $ = (id) => document.getElementById(id);
const screen = new URLSearchParams(location.search).has("screen");
if (screen) document.documentElement.classList.add("screen");
const BCAST = 0xffffffff;

let last = null;
let pin = "";
try { pin = sessionStorage.getItem("hubPIN") || ""; } catch (e) { /* private window */ }

// Per-viewer conveniences only: what was read, the traffic window, the colours.
function load(k, d) { try { const v = localStorage.getItem(k); return v == null ? d : JSON.parse(v); } catch (e) { return d; } }
function save(k, v) { try { localStorage.setItem(k, JSON.stringify(v)); } catch (e) { /* fine */ } }

const MODE_HELP = {
  off: "This hub works alone: the house app runs, nothing is shared over WiFi.",
  mock: "Four pretend houses run inside this hub and post now and then, so you can try the app without other Pis. Nothing is saved.",
  real: "batman-adv runs on the USB WiFi stick and the log syncs with other batnet Pis in range.",
};

// What each preset is for (EU, 250 kHz unless said). Rates and link budgets: Meshtastic's blog, April 2025.
// floor: the weakest signal (SNR, dB) a radio still reads on each preset - the
// LoRa limit for its spreading factor, less the 2.5 dB radios manage below it.
const PRESETS = {
  LongSlow: { what: "Furthest and slowest. Few people use it.", rate: "125 kHz", floor: -22.5 },
  LongModerate: { what: "Far and slow.", rate: "125 kHz", floor: -20 },
  LongFast: { what: "Meshtastic's default. Far, but a message takes about a second on the air, so a busy mesh clogs.", rate: "1.07 kbit/s, 153 dB", floor: -20 },
  MediumSlow: { what: "A middle way.", rate: "1.95 kbit/s, 150.5 dB", floor: -17.5 },
  MediumFast: { what: "What Berlin's mesh uses. Three times faster than LongFast, a little less range.", rate: "3.52 kbit/s, 148 dB", floor: -15 },
  ShortSlow: { what: "Short range, fast: very dense meshes.", rate: "6.25 kbit/s, 145.5 dB", floor: -12.5 },
  ShortFast: { what: "Short range, faster: events, a few streets.", rate: "10.94 kbit/s, 143 dB", floor: -10 },
  ShortTurbo: { what: "Fastest and shortest, 500 kHz wide.", rate: "21.88 kbit/s, 140 dB", floor: -10 },
};

function el(tag, attrs, ...kids) {
  const e = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (k === "class") e.className = v; else if (k === "text") e.textContent = v; else e.setAttribute(k, v);
  }
  for (const k of kids) if (k != null) e.append(k);
  return e;
}

function ago(t) {
  const d = (Date.now() - new Date(t).getTime()) / 1000;
  if (!isFinite(d) || new Date(t).getFullYear() < 2000) return "never";
  if (d < 45) return "just now";
  if (d < 3600) return Math.round(d / 60) + " min ago";
  if (d < 86400) return Math.round(d / 3600) + " h ago";
  return Math.round(d / 86400) + " days ago";
}
const clock = (t) => new Date(t).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
const clockSec = (t) => new Date(t).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
function duration(s) {
  const d = Math.floor(s / 86400), h = Math.floor((s % 86400) / 3600), m = Math.floor((s % 3600) / 60);
  return d ? `${d} d ${h} h` : h ? `${h} h ${m} min` : `${m} min`;
}
const minus = (x) => String(x).replace("-", "−");
const db = (x) => `${minus((x || 0).toFixed(1))} dB`;
const km = (m) => (m >= 1000 ? `${(m / 1000).toFixed(m >= 10000 ? 0 : 1)} km` : `${Math.round(m)} m`);
function distance(a, b) {
  const R = 6371000, r = Math.PI / 180;
  const h = Math.sin((b.lat - a.lat) * r / 2) ** 2 + Math.cos(a.lat * r) * Math.cos(b.lat * r) * Math.sin((b.lon - a.lon) * r / 2) ** 2;
  return 2 * R * Math.asin(Math.sqrt(h));
}

// Signal strength in four bars from LoRa SNR (dB).
function bars(snr) {
  const n = snr >= 5 ? 4 : snr >= -2 ? 3 : snr >= -10 ? 2 : 1;
  const b = el("span", { class: "bars", "aria-hidden": "true" });
  for (let i = 0; i < 4; i++) b.append(el("i", { class: i < n ? "on" : "" }));
  return b;
}

const hex8 = (n) => (n >>> 0).toString(16).padStart(8, "0");
const nodeName = (n) => n.longName || n.shortName || n.id;
function hash(s) { let h = 2166136261; for (const c of s) { h ^= c.charCodeAt(0); h = Math.imul(h, 16777619); } return h >>> 0; }
const nodes = () => (last && last.radio && last.radio.nodes) || [];
const byNum = (num) => nodes().find((n) => n.num === num);
function nameOfNum(num) { const n = byNum(num); return n ? nodeName(n) : "!" + hex8(num); }
function nameOfID(id) {
  if (!id || id === "everyone") return "everyone";
  const n = byNum(parseInt(id.slice(1), 16));
  return n ? nodeName(n) : id;
}
const where = () => (last && last.settings && last.settings.where) || null;
const css = (v) => getComputedStyle(document.documentElement).getPropertyValue(v).trim();
function setNote(id, msg, bad) { $(id).textContent = msg; $(id).className = "note" + (bad ? " bad" : ""); }

// ---- ?redact: screenshots for the public README ----
// Real people's radio names, node IDs, places and words never go into the
// repo (knowledge/security/secret-hygiene.md). With ?redact the panel swaps
// them, consistently, for placeholders before drawing: names from a list,
// IDs from a hash, places moved by a kilometre or two, message texts for
// stock lines, the hub's own addresses for examples. The shape of the mesh
// (how many radios, their signal, relays and timing) stays real.
const redact = new URLSearchParams(location.search).has("redact");
const R_NAMES = ["Anna", "Ben", "Cleo", "Dev", "Emil", "Fritzi", "Gül", "Hanna", "Ilja", "Jonas", "Kemal", "Lena", "Mira", "Noah",
  "Olga", "Paul", "Quinn", "Rosa", "Sami", "Tara", "Uli", "Vera", "Wim", "Xenia", "Yusuf", "Zoe"];
const R_LINES = ["Water is back on at our end.", "Anyone have a spare 12 V battery?", "Radio check from the south side, 73.",
  "Testing a new antenna on the balcony.", "Heard you loud and clear.", "Good morning, mesh!", "Who has power in Wedding?"];
const rh = (x, salt) => hash(salt + String(x));
const rNum = (num) => (num === BCAST || num == null ? num : (rh(num, "n") | 0x10000000) >>> 0);
const rID = (id) => (!id || id === "everyone" || id[0] !== "!" ? id : "!" + hex8(rNum(parseInt(id.slice(1), 16))));
const rSeen = new Map(); // real number -> its place in the list, so no two radios share a name
function rName(num, role) {
  if (!rSeen.has(num)) rSeen.set(num, rSeen.size);
  const i = rSeen.get(num), who = R_NAMES[i % R_NAMES.length], round = Math.floor(i / R_NAMES.length);
  if (/ROUTER|REPEATER/.test(role || "")) return { long: `Rooftop repeater ${i + 1}`, short: `R${i + 1}` };
  return { long: `${who}'s radio${round ? " " + (round + 1) : ""}`, short: who.slice(0, 2).toUpperCase() + (round ? round + 1 : "") };
}
function rPlace(p, num, km) { // moved by up to km kilometres, the same way every time
  if (!p || p.lat == null) return p;
  const h = rh(num, "pos"), a = (h % 360) * Math.PI / 180, d = km * (0.5 + ((h >>> 9) % 100) / 200);
  return { ...p, lat: +(p.lat + d / 111 * Math.cos(a)).toFixed(3), lon: +(p.lon + d / 68 * Math.sin(a)).toFixed(3) };
}
function redactStatus(s) {
  const r = s.radio;
  if (r) {
    const me = r.myNum;
    for (const n of (r.nodes || []).slice().sort((a, b) => rh(a.num, "o") - rh(b.num, "o"))) rName(n.num, n.role); // a stable order
    for (const n of r.nodes || []) {
      const nm = n.self ? { long: "lorabatnet 🐰🦇", short: "🦇" } : rName(n.num, n.role);
      n.longName = nm.long; n.shortName = nm.short;
      if (n.position) n.position = rPlace(n.position, n.num, 1.5);
      n.num = rNum(n.num); n.id = "!" + hex8(n.num);
    }
    r.myNum = rNum(me);
    (r.texts || []).forEach((t, i) => { t.from = rNum(t.from); t.to = rNum(t.to); t.text = R_LINES[(rh(t.id, "t") + i) % R_LINES.length]; });
  }
  if (s.settings && s.settings.where) s.settings.where = rPlace(s.settings.where, 1, 2.5);
  s.house = "Miller House";
  if (s.appURL) s.appURL = "http://hub.local/";
  if (s.devices) {
    for (const d of s.devices.devices || []) { d.name = `A ${d.kind || "device"}`; d.detail = ""; }
    for (const e of s.devices.events || []) { e.device.name = `A ${e.device.kind || "device"}`; e.device.detail = ""; }
  }
  if (s.maps) for (const a of s.maps.areas || []) if (a.id === "here" && a.bounds) { // the box is centred on the hub
    const c = rPlace({ lat: (a.bounds[1] + a.bounds[3]) / 2, lon: (a.bounds[0] + a.bounds[2]) / 2 }, 2, 2.5), h = (a.bounds[3] - a.bounds[1]) / 2, w = (a.bounds[2] - a.bounds[0]) / 2;
    a.bounds = [c.lon - w, c.lat - h, c.lon + w, c.lat + h];
  }
  const sy = s.system;
  sy.hostname = "hub";
  for (const i of sy.interfaces || []) i.addrs = (i.addrs || []).map((a) => (a.startsWith("10.42.") || a.startsWith("172.27.") ? a : "192.168.1.20/24"));
  const w = s.wifi;
  if (w) {
    w.ssid = "Home WiFi"; w.connection = "Home WiFi"; w.ownSSID = "batnet-hub";
    for (const n of w.saved || []) { n.ssid = "Home WiFi"; n.name = "Home WiFi"; }
  }
  return s;
}
function redactTraffic(o) {
  for (const f of o.frames) {
    const from = f.from ? parseInt(f.from.slice(1), 16) : null;
    f.from = rID(f.from); f.to = rID(f.to);
    if (f.name) f.name = rName(from, "").long;
    if (f.text) f.text = R_LINES[rh(f.seq, "t") % R_LINES.length];
    if (f.relay) f.relay = hex8(rh(f.relay, "relay")).slice(-2);
    if (f.lat != null) { const p = rPlace({ lat: f.lat, lon: f.lon }, from, 1.5); f.lat = p.lat; f.lon = p.lon; }
    const i = f.info;
    if (i) {
      if (i.longName) { const nm = rName(from, i.role); i.longName = nm.long; i.shortName = nm.short; }
      if (i.route) i.route = i.route.map(rID);
      if (i.neighbors) i.neighbors = i.neighbors.map((x) => ({ ...x, id: rID(x.id) }));
    }
  }
  return o;
}

// ---- views ----
const VIEWS = ["home", "messages", "map", "radios", "traffic", "settings", "system", "help"];
let view = VIEWS.includes(location.hash.slice(1)) ? location.hash.slice(1) : "home";

function setView(v) {
  view = v;
  for (const x of VIEWS) {
    $("view-" + x).hidden = x !== v;
    $("tab-" + x).setAttribute("aria-selected", String(x === v));
    $("tab-" + x).tabIndex = x === v ? 0 : -1;
  }
  history.replaceState(null, "", location.search + "#" + v);
  if (v === "settings") formsDirty.clear();
  homeSig = ""; threadSig = "";
  if (last) render(last);
  if (v === "traffic") drawTraffic();
  if (v === "map") showMap();
  if (v === "help") loadHelp();
  if (v === "system") sysSig = "";
}
for (const v of VIEWS) $("tab-" + v).addEventListener("click", () => setView(v));
for (const b of document.querySelectorAll("[data-go]")) b.addEventListener("click", () => setView(b.dataset.go));

// ---- the radar: rings by relays ----
const SVGNS = "http://www.w3.org/2000/svg";
function svg(tag, attrs) { const e = document.createElementNS(SVGNS, tag); for (const [k, v] of Object.entries(attrs || {})) e.setAttribute(k, v); return e; }
const RINGS = [{ r: 52, label: "direct" }, { r: 96, label: "1 relay" }, { r: 140, label: "2+ relays" }];

function drawAir(p, list) {
  const pulsing = p.querySelector(".pulse.go");
  p.replaceChildren();
  for (const ring of RINGS) {
    p.append(svg("circle", { class: "ring", r: ring.r }));
    const t = svg("text", { class: "ringlabel", x: 4, y: -ring.r + 11 }); t.textContent = ring.label; p.append(t);
  }
  p.append(pulsing || svg("circle", { class: "pulse", r: 6 }), svg("circle", { class: "hubdot", r: 6 }));
  for (const n of list.filter((n) => !n.self)) {
    const ring = n.hops < 0 ? 2 : Math.min(n.hops, 2);
    const base = RINGS[ring].r - 22;
    // A stable angle per radio; inside its band, a stronger signal sits further in.
    const a = (hash(n.id) % 360) * Math.PI / 180;
    const inward = Math.max(0, Math.min(1, ((n.snr || -10) + 20) / 30));
    const r = base + 18 * (1 - inward);
    const x = Math.cos(a) * r, y = Math.sin(a) * r;
    const stale = Date.now() - new Date(n.lastHeard).getTime() > 2 * 3600e3;
    const cls = "node" + (n.batnet ? " bn" : ring > 0 ? " relayed" : "") + (stale ? " stale" : "");
    const g = svg("g", { class: "hit", tabindex: "0", role: "button", "aria-label": `${nodeName(n)}, ${ago(n.lastHeard)}` });
    g.append(svg("circle", { class: "target", cx: x.toFixed(1), cy: y.toFixed(1), r: 12 }));
    if (n.batnet) {
      const s = 7, pts = [];
      for (let i = 0; i < 6; i++) pts.push(`${(x + s * Math.cos(i * Math.PI / 3)).toFixed(1)},${(y + s * Math.sin(i * Math.PI / 3)).toFixed(1)}`);
      g.append(svg("polygon", { class: cls, points: pts.join(" ") }));
    } else {
      g.append(svg("circle", { class: cls, cx: x.toFixed(1), cy: y.toFixed(1), r: 5 }));
    }
    const lbl = svg("text", { class: "lbl", x: (x + 9).toFixed(1), y: (y + 3.5).toFixed(1) });
    lbl.textContent = n.shortName || n.id.slice(-4);
    const title = svg("title"); title.textContent = nodeName(n);
    g.append(lbl, title);
    g.addEventListener("click", () => showRadio(n.num));
    g.addEventListener("keydown", (ev) => { if (ev.key === "Enter" || ev.key === " ") { ev.preventDefault(); showRadio(n.num); } });
    p.append(g);
  }
}

// ---- details: a radio, or one packet ----
let detailNum = null, detailPos = null;
function openDetail(title, sayText, facts, { num = null, pos = null } = {}) {
  $("detail-title").textContent = title;
  $("detail-says").textContent = sayText || "";
  $("detail-says").hidden = !sayText;
  $("detail-facts").replaceChildren(...facts.filter(([, v]) => v != null && v !== "").flatMap(([k, v]) => [el("dt", { text: k }), el("dd", { text: String(v) })]));
  detailNum = num; detailPos = pos;
  $("detail-msg").hidden = num == null;
  $("detail-map").hidden = !pos;
  const d = $("detail");
  if (!d.open) d.showModal();
}
$("detail-msg").addEventListener("click", () => { $("detail").close(); openConv(String(detailNum)); });
$("detail-map").addEventListener("click", () => {
  $("detail").close();
  const p = detailPos;
  setView("map");
  setTimeout(() => { if (map && p) { mapFitted = true; map.setView([p.lat, p.lon], Math.max(map.getZoom(), 14)); } }, 50);
});

function placeText(p) {
  if (!p) return "";
  let s = `${p.lat.toFixed(4)}, ${p.lon.toFixed(4)}`;
  if (p.accuracyM) s += `, within about ${km(p.accuracyM)}`;
  const w = where();
  if (w) s += `; ${km(distance(w, p))} from this hub`;
  return s;
}

// farGuess says roughly how far a radio with no shared place is, from what
// the hub has seen of radios whose place is known: heard straight, by its
// signal against theirs (the map zone's fading, 30 dB per tenfold distance);
// through relays, by how far radios as many relays away mostly are.
function farGuess(n) {
  const w = where();
  if (!w) return "";
  const placed = nodes().filter((x) => x.position && !x.self && (x.position.accuracyM || 0) < distance(w, x.position) / 2);
  if (n.link) {
    const ds = placed.filter((x) => x.link).map((x) => distance(w, x.position) * 10 ** ((x.link.snr - n.link.snr) / (10 * PATH_EXP))).sort((a, b) => a - b);
    if (!ds.length) return "";
    const lo = ds[0], hi = ds[ds.length - 1];
    return `${hi > 2 * lo ? `somewhere from ${km(lo)} to ${km(hi)}` : `about ${km(ds[ds.length >> 1])}`}, from how strongly it reaches this hub straight against ${ds.length === 1 ? "one radio" : `${ds.length} radios`} whose place is known (they sit higher or lower, hence the spread); which way is not known`;
  }
  if (n.hops > 0) {
    const ds = placed.filter((x) => x.hops === n.hops).map((x) => distance(w, x.position)).sort((a, b) => a - b);
    if (ds.length < 3) return "";
    const q = (f) => ds[Math.floor(f * (ds.length - 1))];
    return `radios ${n.hops} ${n.hops === 1 ? "relay" : "relays"} away that share their place are mostly ${km(q(0.25))} to ${km(q(0.75))} from here (${ds.length} of them)`;
  }
  return "";
}

function showRadio(num) {
  const n = byNum(num);
  if (!n) return;
  openDetail(nodeName(n), [n.shortName, n.hardware, n.role].filter(Boolean).join(", "), [
    ["ID", n.id],
    // Of the last packet only: after relays it is the last relay's signal, not this radio's.
    ["Signal", n.self ? "" : `${db(n.snr)}${n.rssi ? `, ${minus(n.rssi)} dBm` : ""}${n.hops > 0 ? ` (last packet, from the last of its ${n.hops} relays, not from this radio)` : ""}`],
    ["Straight to this hub", n.self || !n.link ? "" : `${db(n.link.snr)} (median), ${n.link.count} ${n.link.count === 1 ? "packet" : "packets"}, last ${ago(n.link.at)}`],
    ["Relays", n.self ? "" : n.hops < 0 ? "not known" : n.hops === 0 ? "none: heard directly" : String(n.hops)],
    ["Last heard", n.self ? "" : ago(n.lastHeard)],
    ["Packets heard", n.self ? "" : n.packets],
    ["Battery", n.battery != null ? (n.battery > 100 ? "on mains power" : `${n.battery} %`) : ""],
    ["Its channel in use", n.chanUtil != null ? `${n.chanUtil.toFixed(1)} %` : ""],
    ["Where", n.position ? `${placeText(n.position)} (shared ${ago(n.position.at)})` : "has not shared it"],
    ["How far, roughly", n.self || (n.position && where()) ? "" : farGuess(n)],
    ["Runs batnet", n.batnet ? "yes" : ""],
  ], { num: n.self ? null : n.num, pos: n.position });
}

const PORTS = { nodeinfo: "who it is", position: "position", telemetry: "telemetry", text: "message", routing: "receipt",
  traceroute: "route trace", neighborinfo: "neighbours", "map-report": "map report", "store-forward": "store and forward" };
const WHAT = {
  heard: (f) => (f.encrypted ? "encrypted" : PORTS[f.port] || f.port || "packet"),
  garbled: () => "unreadable",
  sent: (f) => (f.port === "nodeinfo" ? "hello sent" : f.port === "text" ? "message sent" : `sent ${f.port || ""}`),
  relayed: () => "passed on", busy: () => "channel busy",
};

// says puts a packet's contents in a few words.
function says(f) {
  const i = f.info || {};
  if (f.kind === "garbled" || f.kind === "relayed" || f.kind === "busy") return "";
  if (f.text) return `“${f.text}”`;
  if (f.lat != null) return placeText({ lat: f.lat, lon: f.lon, accuracyM: i.accuracyM });
  switch (f.port) {
    case "nodeinfo": return [i.longName && `${i.longName}${i.shortName ? ` (${i.shortName})` : ""}`, i.hardware, i.role].filter(Boolean).join(", ");
    case "telemetry": return [i.battery != null && (i.battery > 100 ? "mains power" : `battery ${i.battery} %`), i.voltage && `${i.voltage} V`,
      i.channelUse != null && `channel ${i.channelUse} % busy`, i.temperatureC != null && `${i.temperatureC} °C`, i.humidity != null && `${i.humidity} % humidity`,
      i.pressureHPa != null && `${i.pressureHPa} hPa`].filter(Boolean).join(", ");
    case "traceroute": return (i.route || []).length ? `via ${i.route.map(nameOfID).join(" → ")}` : "a route trace";
    case "neighborinfo": return (i.neighbors || []).length ? `hears ${i.neighbors.map((x) => nameOfID(x.id)).join(", ")}` : "";
    case "routing": return i.error ? `not delivered: ${i.error}` : "delivered";
  }
  return "";
}

const EXPLAIN = {
  encrypted: "Sent on a channel this hub has no key for (another group's private channel). The radio can read who sent it and where it goes, not what it says.",
  garbled: "Heard, but damaged on the way: too weak to read, or it collided with another packet. A strong signal that still fails usually means a collision.",
  relayed: "This hub's radio passed someone else's packet on, as every Meshtastic radio does, so it travels further.",
  busy: "The radio had something to send but someone else was on the air, so it waited.",
};

function showFrame(f) {
  const i = f.info || {};
  const pos = f.lat != null ? { lat: f.lat, lon: f.lon, accuracyM: i.accuracyM } : null;
  const what = WHAT[f.kind] ? WHAT[f.kind](f) : f.kind;
  const explain = f.kind === "heard" && f.encrypted ? EXPLAIN.encrypted : EXPLAIN[f.kind] || "";
  const from = f.from ? parseInt(f.from.slice(1), 16) : null;
  const facts = [["Time", clockSec(f.t)],
    ["From", f.from ? `${f.name || nameOfID(f.from)} (${f.from})` : f.kind === "sent" || f.kind === "relayed" ? "this hub" : ""],
    ["To", f.to ? nameOfID(f.to) : ""],
    ["Signal", f.kind === "heard" || f.kind === "garbled" ? `${db(f.snr)}${f.rssi ? `, ${minus(f.rssi)} dBm` : ""}` : ""],
    ["Relays on the way", f.hops >= 0 && f.kind !== "sent" ? String(f.hops) : ""], ["Last radio before ours", f.relay ? `…${f.relay}` : ""],
    ["Time on the air", f.airMs ? `about ${f.airMs} ms` : ""], ["Size", f.bytes ? `about ${f.bytes} bytes` : ""]];
  if (f.alt != null) facts.push(["Height", `${f.alt} m`]);
  if (i.sats) facts.push(["GPS satellites", i.sats]);
  if (i.uptimeS) facts.push(["Its radio running for", duration(i.uptimeS)]);
  if (i.airtime != null) facts.push(["Its own airtime", `${i.airtime} %`]);
  const n = from != null ? byNum(from) : null;
  openDetail(what[0].toUpperCase() + what.slice(1), [says(f), explain].filter(Boolean).join(" "), facts,
    { num: n && !n.self ? from : null, pos });
}

// ---- messages ----
let conv = "pub";            // the open conversation: "pub", a group channel "ch1".., or a node number as a string
let replyTo = null;          // the message being answered
const started = new Set();   // conversations opened from elsewhere, maybe still empty
const seen = new Set();      // messages already noticed (for the new-message flash)
let firstStatus = true;
const pageStart = Date.now();
const lastRead = load("hub.read", {});
let threadSig = "", homeSig = "";

const readTime = (c) => (lastRead[c] ? Date.parse(lastRead[c]) : pageStart);
const textKey = (t) => `${t.id}:${t.from}:${t.at}`;

function conversations(r) {
  const me = r.myNum;
  const out = new Map([["pub", { id: "pub", num: BCAST, texts: [] }]]);
  for (const t of r.texts || []) {
    let id;
    if (t.to === BCAST) id = t.channel ? "ch" + t.channel : "pub"; // a group channel of its own
    else if (t.mine) id = String(t.to);
    else if (t.to === me) id = String(t.from);
    else continue;
    if (!out.has(id)) out.set(id, { id, num: Number(id), texts: [] });
    out.get(id).texts.push(t);
  }
  for (const ch of r.channels || []) {
    const id = "ch" + ch.index;
    if (ch.index > 0 && !out.has(id)) out.set(id, { id, num: BCAST, texts: [] });
  }
  for (const id of started) if (!out.has(id)) out.set(id, { id, num: Number(id), texts: [] });
  for (const c of out.values()) {
    c.channel = c.id.startsWith("ch") ? Number(c.id.slice(2)) : 0;
    const chName = ((r.channels || []).find((x) => x.index === c.channel) || {}).name || `channel ${c.channel}`;
    c.name = c.id === "pub" ? "Everyone nearby" : c.channel ? `${chName} channel` : nameOfNum(c.num);
    c.lastAt = c.texts.length ? Date.parse(c.texts[c.texts.length - 1].at) : 0;
    c.unread = c.texts.filter((t) => !t.mine && Date.parse(t.at) > readTime(c.id)).length;
  }
  const rank = (c) => (c.id === "pub" ? 0 : c.channel ? 1 : 2); // everyone, the group channels, then people
  return [...out.values()].sort((a, b) => rank(a) - rank(b) || (rank(a) === 1 ? a.channel - b.channel : b.lastAt - a.lastAt));
}

function openConv(id) {
  if (/^\d+$/.test(id)) started.add(id);
  if (conv !== id) { replyTo = null; showReply(); }
  conv = id;
  setView("messages");
  $("msg").focus();
}

function statusLine(t) {
  switch (t.status) {
    case "sending": return el("span", { class: "st", text: "Sending…" });
    case "passed on": return el("span", { class: "st ok", text: "✓ passed on by a radio nearby" });
    case "delivered": return el("span", { class: "st ok", text: "✓✓ delivered" });
    case "failed": return el("span", { class: "st bad", text: `Not delivered: ${t.detail || "no answer"}` });
    default: return null;
  }
}

function bubble(t, all, { who, reply }) {
  const li = el("li", { class: t.mine ? "mine" : "in" });
  if (who) {
    const wb = el("button", { class: "who linkbtn", type: "button", text: who, title: "Who is this, and where?" });
    wb.addEventListener("click", () => showRadio(t.from));
    li.append(wb);
  }
  if (t.replyId) {
    const q = all.find((x) => x.id === t.replyId);
    li.append(el("span", { class: "quote", text: q ? `${q.mine ? "You" : nameOfNum(q.from)}: ${q.text}` : "An earlier message" }));
  }
  li.append(el("span", { class: "body", text: t.text }));
  const tk = textKey(t), tr = translations.get(tk);
  if (tr) li.append(el("span", { class: "tr-line", text: tr.error ? tr.error : `${tr.text} (machine translation from ${tr.fromName})` }));
  const meta = el("span", { class: "meta" }, clock(t.at));
  if (!t.mine) meta.append(`, ${db(t.snr)}, ${t.hops < 0 ? "?" : t.hops} relays${t.pki ? ", encrypted to this hub" : ""}`);
  const st = t.mine ? statusLine(t) : null;
  if (st) meta.append(" ", st);
  if (!t.mine && !tr && canTranslate(t.text)) {
    if (autoTranslate()) translateText(t);
    else {
      const tb = el("button", { class: "linkbtn", type: "button", text: "Translate" });
      tb.addEventListener("click", (ev) => { ev.stopPropagation(); tb.disabled = true; tb.textContent = "Translating…"; translateText(t); });
      meta.append(" ", tb);
    }
  }
  if (reply) {
    const rb = el("button", { class: "linkbtn", type: "button", text: "Reply" });
    rb.addEventListener("click", () => { replyTo = t; showReply(); $("msg").focus(); });
    meta.append(" ", rb);
  }
  li.append(meta);
  return li;
}

function renderMessages(r, convs) {
  // Messages to this hub and on its group channels count; the open channel only shows a dot.
  const total = convs.filter((c) => c.id !== "pub").reduce((s, c) => s + c.unread, 0);
  const badge = $("badge");
  badge.hidden = !total && !convs[0].unread;
  badge.textContent = total ? String(total) : "";
  badge.className = "badge" + (total ? "" : " dot");

  const c = convs.find((x) => x.id === conv) || convs[0];
  conv = c.id;
  if (view === "messages" && document.visibilityState === "visible" && c.texts.length) {
    lastRead[c.id] = c.texts[c.texts.length - 1].at;
    c.unread = 0;
    save("hub.read", lastRead);
    if (last && last.unread) markSeen();
  }
  if (view !== "messages") return;

  $("convs").replaceChildren(...convs.map((x) => {
    const lastT = x.texts[x.texts.length - 1];
    const b = el("button", { class: "conv" + (x.id === conv ? " on" : ""), type: "button", "aria-current": String(x.id === conv) },
      el("span", { class: "cn", text: x.name }),
      x.unread ? el("span", { class: "badge", text: String(x.unread) }) : null,
      el("span", { class: "cp", text: lastT ? `${lastT.mine ? "You: " : x.num === BCAST ? nameOfNum(lastT.from) + ": " : ""}${lastT.text}` : x.id === "pub" ? "The open channel" : x.channel ? "A group channel" : "No messages yet" }));
    b.addEventListener("click", () => openConv(x.id));
    return el("li", {}, b);
  }));

  const node = c.num === BCAST ? null : byNum(c.num);
  if (node) {
    const nb = el("button", { class: "linkbtn", type: "button", text: c.name, title: "Who is this, and where?" });
    nb.addEventListener("click", () => showRadio(node.num));
    $("thread-name").replaceChildren(nb);
  } else $("thread-name").textContent = c.name;
  $("thread-sub").textContent = c.id === "pub"
    ? `Meshtastic's open ${r.preset || ""} channel: anyone with a radio nearby can read it. Don't send anything private.`
    : c.channel ? `A group channel: radios that have its name and password read it. Its password is often public, so don't send anything private.`
    : `Only this radio gets it.${node ? ` Last heard ${ago(node.lastHeard)}, ${db(node.snr)}, ${node.hops < 0 ? "?" : node.hops} relays.` : ""} Messages to one radio are encrypted between the two once they know each other's key.`;
  $("msg").placeholder = c.id === "pub" ? "Message to everyone in range" : c.channel ? `Message to the ${c.name}` : `Message to ${c.name}`;
  $("thread-empty").hidden = c.texts.length > 0;
  $("thread-empty").textContent = c.num === BCAST ? "No messages heard yet." : "No messages yet. Write the first one below.";

  const sig = conv + "|" + c.texts.map((t) => textKey(t) + t.status).join(",") + "|" + nodes().length + "|" + translations.size;
  if (sig === threadSig) return;
  threadSig = sig;
  const list = $("thread");
  const atBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 40;
  list.replaceChildren(...c.texts.map((t) => bubble(t, r.texts || [], { who: !t.mine && c.num === BCAST ? nameOfNum(t.from) : "", reply: true })));
  if (atBottom) list.scrollTop = list.scrollHeight;
}

function showReply() {
  $("replying").hidden = !replyTo;
  if (replyTo) $("replying-text").textContent = `Replying to ${replyTo.mine ? "your message" : nameOfNum(replyTo.from)}: “${replyTo.text.slice(0, 60)}${replyTo.text.length > 60 ? "…" : ""}”`;
}
$("reply-cancel").addEventListener("click", () => { replyTo = null; showReply(); });

// A new message to this hub alone: a flash at the top until it is opened.
let flashConv = null, flashTimer = 0;
function noticeNew(r) {
  for (const t of r.texts || []) {
    const k = textKey(t);
    if (seen.has(k)) continue;
    seen.add(k);
    if (firstStatus || t.mine || t.to !== r.myNum) continue;
    const id = String(t.from);
    if (view === "messages" && conv === id) continue;
    flashConv = id;
    $("flash-text").textContent = `New message from ${nameOfNum(t.from)}: “${t.text.slice(0, 50)}${t.text.length > 50 ? "…" : ""}”`;
    $("flash").hidden = false;
    clearTimeout(flashTimer);
    flashTimer = setTimeout(() => { $("flash").hidden = true; }, 30000);
  }
  firstStatus = false;
}
$("flash-open").addEventListener("click", () => { $("flash").hidden = true; if (flashConv) openConv(flashConv); });

// ---- home: the simple first screen ----
function renderHome(s, convs) {
  const r = s.radio;
  const others = nodes().filter((n) => !n.self);
  const hour = others.filter((n) => Date.now() - Date.parse(n.lastHeard) < 3600e3).length;
  const unread = convs.filter((c) => c.id !== "pub").reduce((a, c) => a + c.unread, 0);
  const hl = $("headline");
  hl.replaceChildren(el("span", { class: "big", text: String(hour) }), ` radio${hour === 1 ? "" : "s"} heard in the last hour`);
  if (unread) {
    const b = el("button", { class: "linkbtn", type: "button", text: `${unread} new message${unread === 1 ? "" : "s"} for you` });
    b.addEventListener("click", () => { const c = convs.find((x) => x.id !== "pub" && x.unread); if (c) openConv(c.id); });
    hl.append(". ", b);
  }
  hl.append(".");
  const lg = s.log, sy = s.system;
  const parts = [];
  if (r && r.ready) parts.push(`Radio on ${r.preset}${r.txEnabled ? "" : ", listening only"}`);
  if (lg) parts.push(lg.on ? `logging since ${clock(lg.started)}` : "not logging");
  if (sy.tempC) parts.push(`the Pi at ${sy.tempC.toFixed(0)} °C`);
  $("health").textContent = parts.length ? parts.join(", ") + "." : "";

  drawAir($("plot-home"), nodes());
  const recent = [];
  for (const c of convs) for (const t of c.texts) recent.push({ t, c });
  recent.sort((a, b) => Date.parse(a.t.at) - Date.parse(b.t.at));
  const show = recent.slice(-5);
  $("home-empty").hidden = show.length > 0;
  const sig = show.map((x) => textKey(x.t) + x.t.status).join(",") + "|" + translations.size;
  if (sig === homeSig) return;
  homeSig = sig;
  $("home-thread").replaceChildren(...show.map(({ t, c }) => {
    const who = t.mine ? (c.id === "pub" ? "You, to everyone" : `You, to ${c.channel ? "the " : ""}${c.name}`) : `${nameOfNum(t.from)}${c.id === "pub" ? "" : c.channel ? `, on the ${c.name}` : ", to you"}`;
    const li = bubble(t, (r && r.texts) || [], { who, reply: false });
    li.classList.add("click");
    li.addEventListener("click", () => openConv(c.id));
    return li;
  }));
}

// ---- the map ----
let map = null, mapLayer = null, mapFitted = false, placing = false;
function showMap() {
  if (!window.L) { $("map-note").textContent = "The map could not load."; $("map-note").hidden = false; return; }
  if (!map) {
    map = L.map("map-canvas", { zoomControl: true });
    const tiles = L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 18, referrerPolicy: "strict-origin-when-cross-origin",
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
    }).addTo(map);
    let errors = 0;
    tiles.on("tileerror", () => {
      if (++errors < 3 || offline.size) return;
      $("map-note").textContent = "No streets: the online map needs the internet. Get a map to keep on the hub under System, Offline maps. The radios are still in the right places.";
      $("map-note").hidden = false;
    });
    mapLayer = L.layerGroup().addTo(map);
    map.setView([52.52, 13.405], 11);
    map.on("click", async (ev) => {
      if (!placing) return;
      placing = false;
      $("map-canvas").classList.remove("placing");
      try {
        await act("settings", { lat: ev.latlng.lat.toFixed(5), lon: ev.latlng.lng.toFixed(5) });
        setNote("place-note", "Saved. Distances now count from there.");
        poll();
      } catch (e) { setNote("place-note", e.message, true); }
    });
  }
  setTimeout(() => { map.invalidateSize(); drawMap(); }, 0);
}
$("place-hub").addEventListener("click", () => {
  if (!map) return;
  placing = true;
  $("map-canvas").classList.add("placing");
  setNote("place-note", "Click the map where this hub stands.");
});

// coverage works out where the hub should hear a radio straight. Each radio
// heard straight at distance d with SNR s could be d * 10^((s - floor) / 30)
// away and still be read: a signal fades about 30 dB for each tenfold
// distance among houses (path-loss exponent 3). That reach spreads 30° either
// side of its direction, tapering; no such radio in a direction, no zone.
const PATH_EXP = 3, SPREAD = 30;
function bearing(a, b) {
  const r = Math.PI / 180, dl = (b.lon - a.lon) * r;
  const y = Math.sin(dl) * Math.cos(b.lat * r);
  const x = Math.cos(a.lat * r) * Math.sin(b.lat * r) - Math.sin(a.lat * r) * Math.cos(b.lat * r) * Math.cos(dl);
  return (Math.atan2(y, x) / r + 360) % 360;
}
const COMPASS = ["north", "north-east", "east", "south-east", "south", "south-west", "west", "north-west"];
function coverage(w, placed) {
  const out = { evidence: [], shape: null, far: null };
  const floor = (PRESETS[(last && last.radio && last.radio.preset) || ""] || {}).floor;
  if (!w || floor == null) return out;
  for (const n of placed) {
    if (!n.link) continue;
    const d = distance(w, n.position);
    if (d < 50 || (n.position.accuracyM || 0) > d / 2) continue; // too close or too rough to say which way
    const reach = Math.min(Math.max(d, d * 10 ** ((n.link.snr - floor) / (10 * PATH_EXP))), 60000);
    out.evidence.push({ n, d, reach, brg: bearing(w, n.position) });
  }
  if (!out.evidence.length) return out;
  out.far = out.evidence.reduce((a, b) => (b.reach > a.reach ? b : a));
  out.far.dir = COMPASS[Math.round(out.far.brg / 45) % 8];
  const r = Math.PI / 180, kmLat = 111320, kmLon = 111320 * Math.cos(w.lat * r);
  out.shape = [];
  for (let a = 0; a < 360; a += 2) {
    let m = 0;
    for (const e of out.evidence) {
      const off = Math.abs(((a - e.brg + 540) % 360) - 180);
      if (off < SPREAD) m = Math.max(m, e.reach * Math.cos((off / SPREAD) * Math.PI / 2) ** 2);
    }
    out.shape.push([w.lat + (m * Math.cos(a * r)) / kmLat, w.lon + (m * Math.sin(a * r)) / kmLon]);
  }
  return out;
}

// Offline maps (decision 0032): the areas the hub keeps, drawn over the
// online tiles, so they show with or without the internet.
const offline = new Map(); // id:build:at -> layer
function syncOfflineMaps() {
  const want = new Map(((last && last.maps && last.maps.areas) || []).filter((a) => a.have).map((a) => [`${a.id}:${a.build}:${a.at}`, a]));
  for (const [k, l] of offline) if (!want.has(k)) { map.removeLayer(l); offline.delete(k); }
  if (!window.protomapsL) return;
  for (const [k, a] of want) {
    if (offline.has(k)) continue;
    const [w, so, e, n] = a.bounds;
    offline.set(k, protomapsL.leafletLayer({
      url: `maps/${a.id}.pmtiles?v=${encodeURIComponent(a.at)}`, flavor: "light", lang: "en", // dark mode inverts the whole tile pane, as for the online tiles
      bounds: L.latLngBounds([so, w], [n, e]), maxDataZoom: 15,
      attribution: '<a href="https://protomaps.com">Protomaps</a> &copy; <a href="https://openstreetmap.org/copyright">OpenStreetMap</a>',
    }).addTo(map));
  }
  if (offline.size) $("map-note").hidden = true;
}

function drawMap() {
  if (!map || view !== "map") return;
  syncOfflineMaps();
  mapLayer.clearLayers();
  const w = where();
  const placed = nodes().filter((n) => n.position && !n.self);
  const pts = [];
  const ink = css("--dot"), teal = css("--batnet"), sheet = css("--sheet");
  for (const n of placed) {
    const p = [n.position.lat, n.position.lon];
    pts.push(p);
    const col = n.batnet ? teal : ink;
    const old = Date.now() - Date.parse(n.position.at) > 6 * 3600e3;
    if (n.position.accuracyM > 60) {
      L.circle(p, { radius: n.position.accuracyM, color: col, weight: 1, opacity: old ? 0.3 : 0.6, fillOpacity: 0.06, interactive: false }).addTo(mapLayer);
    }
    const m = L.circleMarker(p, { radius: 7, color: sheet, weight: 2, fillColor: col, fillOpacity: old ? 0.45 : 1 }).addTo(mapLayer);
    // An element, not a string: Leaflet takes a string label as HTML, and the short name is the radio's own choice.
    m.bindTooltip(el("span", { text: n.shortName || n.id.slice(-4) }), { permanent: true, direction: "right", offset: [6, 0], className: "maplbl" });
    m.on("click", () => showRadio(n.num));
  }
  // Where the hub hears: the zone, and a dashed line to each radio behind it.
  const z = coverage(w, placed);
  if (z.shape) L.polygon(z.shape, { color: css("--signal"), weight: 2, dashArray: "4 4", fillOpacity: 0.22, interactive: false }).addTo(mapLayer);
  for (const e of z.evidence) {
    L.polyline([[w.lat, w.lon], [e.n.position.lat, e.n.position.lon]], { color: css("--signal"), weight: 2, dashArray: "6 6", interactive: false }).addTo(mapLayer);
  }
  $("map-reach").textContent = !w ? "Mark where this hub is to see where it hears."
    : z.evidence.length ? `Shaded: where the hub should hear a radio straight, with no relay, worked out from how strongly ${z.evidence.length === 1 ? "one radio reaches" : `${z.evidence.length} radios reach`} it (dashed lines): up to ${km(z.far.reach)} to the ${z.far.dir}. For a radio placed like those; one lower down or indoors is heard less far. Blank directions are not known yet.`
    : "No radio with a known place has reached the hub straight yet, so no zone.";
  if (w) {
    pts.push([w.lat, w.lon]);
    L.marker([w.lat, w.lon], { icon: L.divIcon({ className: "hubmark", html: "<span>this hub</span>", iconSize: null }), keyboard: false }).addTo(mapLayer);
  }
  if (!mapFitted && pts.length) { map.fitBounds(L.latLngBounds(pts).pad(0.2), { maxZoom: 14 }); mapFitted = true; }

  $("mcount").textContent = placed.length ? `(${placed.length})` : "";
  $("map-empty").hidden = placed.length > 0;
  const sorted = placed.slice().sort((a, b) => (w ? distance(w, a.position) - distance(w, b.position) : Date.parse(b.position.at) - Date.parse(a.position.at)));
  $("maplist").replaceChildren(...sorted.map((n) => {
    const b = el("button", { type: "button", class: "conv" },
      el("span", { class: "cn", text: nodeName(n) }),
      el("span", { class: "cp", text: `${w ? km(distance(w, n.position)) + " away, " : ""}${n.position.accuracyM > 60 ? `rough area (${km(n.position.accuracyM)}), ` : ""}shared ${ago(n.position.at)}` }));
    b.addEventListener("click", () => { map.setView([n.position.lat, n.position.lon], Math.max(map.getZoom(), 14)); showRadio(n.num); });
    return el("li", {}, b);
  }));
}

// ---- settings ----
const formsDirty = new Set();
for (const f of ["radio-form", "owner-form", "hub-form"]) $(f).addEventListener("input", () => formsDirty.add(f));
const bytes = (s) => new TextEncoder().encode(s).length;
function counts() {
  $("long-count").textContent = `${bytes($("set-long").value)} of 39 bytes`;
  const sb = bytes($("set-short").value);
  $("short-count").textContent = `${sb} of 4 bytes`;
  $("short-count").className = "hint" + (sb > 4 ? " bad" : "");
}
$("set-long").addEventListener("input", counts);
$("set-short").addEventListener("input", counts);
$("set-short").removeAttribute("maxlength"); // bytes, not letters: an emoji is one letter and four bytes

function renderSettings(s) {
  const r = s.radio, me = nodes().find((n) => n.self);
  if (r && r.ready && !formsDirty.has("radio-form")) {
    $("preset-list").replaceChildren(...(s.presets || []).map((p) => {
      const info = PRESETS[p] || { what: "", rate: "" };
      const inp = el("input", { type: "radio", name: "preset", value: p });
      inp.checked = p === r.preset;
      return el("label", { class: "preset" + (p === r.preset ? " now" : "") }, inp,
        el("span", { class: "pn" }, p, p === r.preset ? el("span", { class: "tag me", text: "now" }) : null),
        el("span", { class: "pw", text: info.what }), el("span", { class: "pr", text: info.rate }));
    }));
    $("set-hops").value = String(r.hopLimit || 3);
    $("set-power").value = String(r.txPower || 27);
    $("set-power").max = String(s.maxTxPower || 27);
    $("set-tx").checked = !!r.txEnabled;
  }
  if (me && !formsDirty.has("owner-form")) {
    $("set-long").value = me.longName || "";
    $("set-short").value = me.shortName || "";
    counts();
  }
  if (s.settings && !formsDirty.has("hub-form")) {
    const h = s.settings.helloMinutes, v = h < 0 ? "-1" : h === 0 ? "5" : String(h);
    if (![...$("set-hello").options].some((o) => o.value === v)) $("set-hello").append(el("option", { value: v, text: `every ${h} minutes` }));
    $("set-hello").value = v;
    $("set-lat").value = s.settings.where ? String(s.settings.where.lat) : "";
    $("set-lon").value = s.settings.where ? String(s.settings.where.lon) : "";
  }
}

$("radio-form").addEventListener("submit", async (ev) => {
  ev.preventDefault();
  const r = last && last.radio;
  const sel = document.querySelector('input[name="preset"]:checked');
  const body = { hopLimit: $("set-hops").value, txPower: $("set-power").value, txEnabled: String($("set-tx").checked) };
  if (sel && r && sel.value !== r.preset) {
    if (!confirm(`Switch the radio to ${sel.value}? It will then only hear radios on ${sel.value}.`)) return;
    body.preset = sel.value;
  }
  try {
    await act("radio", body);
    formsDirty.delete("radio-form");
    setNote("radio-note", "Sent to the radio. It restarts and is back in about 10 s.");
  } catch (e) { setNote("radio-note", e.message, true); }
});

$("owner-form").addEventListener("submit", async (ev) => {
  ev.preventDefault();
  try {
    await act("owner", { long: $("set-long").value, short: $("set-short").value });
    formsDirty.delete("owner-form");
    setNote("owner-note", "Renamed. Radios nearby see the new name the next time this hub says hello.");
  } catch (e) { setNote("owner-note", e.message, true); }
});

$("hub-form").addEventListener("submit", async (ev) => {
  ev.preventDefault();
  try {
    await act("settings", { helloMinutes: $("set-hello").value, lat: $("set-lat").value.trim(), lon: $("set-lon").value.trim() });
    formsDirty.delete("hub-form");
    setNote("hub-note", "Saved.");
    poll();
  } catch (e) { setNote("hub-note", e.message, true); }
});
$("forget-place").addEventListener("click", async () => {
  try {
    await act("settings", { lat: "", lon: "" });
    formsDirty.delete("hub-form");
    setNote("hub-note", "Forgotten.");
    poll();
  } catch (e) { setNote("hub-note", e.message, true); }
});

function applyTheme(t) {
  if (t) document.documentElement.dataset.theme = t; else delete document.documentElement.dataset.theme;
  for (const b of document.querySelectorAll(".theme button")) b.setAttribute("aria-checked", String(b.dataset.theme === (t || "")));
  if (map) drawMap();
}
for (const b of document.querySelectorAll(".theme button")) b.addEventListener("click", () => { save("hub.theme", b.dataset.theme); applyTheme(b.dataset.theme); });
applyTheme(load("hub.theme", ""));

// ---- drawing the status ----
function render(s) {
  last = s;
  const me = nodes().find((n) => n.self);
  const name = (me && me.longName) || "lorabatnet";
  $("hubname").textContent = name;
  document.title = `${name} hub`;
  $("house").textContent = s.system.model ? `${s.house}, on a ${s.system.model.replace(/ Model B Rev [\d.]+$/, "")}` : s.house;
  $("clock").textContent = clock(s.system.time);
  const r = s.radio;
  const onair = $("onair");
  if (!r) { onair.textContent = "No radio configured"; onair.className = "onair down"; }
  else if (!r.connected) { onair.textContent = "Radio offline: meshtasticd is not answering"; onair.className = "onair down"; }
  else if (!r.ready) { onair.textContent = "Radio connecting…"; onair.className = "onair"; }
  else if (r.region === "not set") { onair.textContent = "Radio on, but no region is set: it will not transmit"; onair.className = "onair down"; }
  else {
    onair.textContent = `On air: ${r.region.replace("_", " ")} MHz, ${r.preset}, ${r.txEnabled ? `${r.txPower} dBm` : "listening only"}`;
    onair.className = "onair live";
  }

  const convs = r ? conversations(r) : [];
  if (r) { noticeNew(r); renderMessages(r, convs); }
  if (view === "home") renderHome(s, convs);
  if (view === "map") drawMap();
  if (view === "radios") renderRadios();
  const ready = r && r.ready && r.region !== "not set";
  for (const b of [$("hello"), document.querySelector("#send button"), document.querySelector("#home-send button")]) b.disabled = !ready;
  if (view === "settings") renderHubSettings(s);
  if (view === "system") renderSystem(s);
  if (view === "help") fillHelp();
  renderKitGlobal(s);
}

function renderRadios() {
  drawAir($("plot"), nodes());
  const others = nodes().filter((n) => !n.self);
  $("ncount").textContent = others.length ? `(${others.length})` : "";
  $("nodes-empty").hidden = others.length > 0;
  $("nodelist").replaceChildren(...nodes().map((n) => {
    const name = el("td", { class: "nm" }, nodeName(n));
    if (n.batnet) name.append(el("span", { class: "tag bn", text: "batnet" }));
    if (n.self) name.append(el("span", { class: "tag me", text: "this hub" }));
    if (n.position && !n.self) name.append(el("span", { class: "tag pos", text: "on the map" }));
    name.append(el("span", { class: "sub", text: [n.shortName, n.hardware, n.role].filter(Boolean).join(", ") || n.id }));
    const sig = n.self ? el("td", { text: "" }) : el("td", {}, bars(n.snr), db(n.snr), n.rssi ? el("span", { class: "sub", text: `${minus(n.rssi)} dBm` }) : null);
    const hops = el("td", { text: n.self ? "" : n.hops < 0 ? "?" : String(n.hops) });
    const heard = el("td", { text: n.self ? "" : ago(n.lastHeard) });
    if (!n.self && n.packets) heard.append(el("span", { class: "sub", text: `${n.packets} packet${n.packets > 1 ? "s" : ""}` }));
    const actions = el("td", { class: "acts" });
    const info = el("button", { class: "btn ghost small", type: "button", text: "Info" });
    info.addEventListener("click", () => showRadio(n.num));
    actions.append(info);
    if (!n.self) {
      const b = el("button", { class: "btn ghost small", type: "button", text: "Message" });
      b.addEventListener("click", () => openConv(String(n.num)));
      actions.append(b);
    }
    return el("tr", {}, name, sig, hops, heard, actions);
  }));
}

function renderHubSettings(s) {
  renderSettings(s);
  const r = s.radio, m = s.mesh || {};
  for (const b of document.querySelectorAll(".mesh button")) b.setAttribute("aria-checked", String(b.dataset.mode === m.mode));
  $("mode-help").textContent = MODE_HELP[m.mode] || "";
  const st = $("mesh-state");
  st.textContent = m.state + (m.detail ? `. ${m.detail}` : "");
  st.className = "state" + (m.ok ? "" : " bad");
  $("peers").replaceChildren(...(m.peers || []).map((p) => el("li", {}, p.name, p.detail ? el("span", { class: "sub", text: `, ${p.detail}` }) : null)));

  const lg = s.log;
  const lb = $("logbtn");
  lb.disabled = !lg;
  if (lg) {
    lb.textContent = lg.on ? "Stop logging" : "Start logging";
    lb.className = "btn" + (lg.on ? " onair-btn" : "");
    $("log-state").textContent = lg.on
      ? `Logging since ${clock(lg.started)}: ${lg.packets} packet${lg.packets === 1 ? "" : "s"} from ${lg.radios} radio${lg.radios === 1 ? "" : "s"}, ${lg.hellos} hello${lg.hellos === 1 ? "" : "s"} sent (${lg.helloMinutes ? `one every ${lg.helloMinutes} min` : "none"})${lg.unreadable ? `, ${lg.unreadable} heard too weak to read` : ""}`
      : "Not logging.";
    $("log-state").className = "state" + (lg.error ? " bad" : "");
    if (lg.error) $("log-state").textContent += ` Error: ${lg.error}`;
    const q = s.local ? "" : `?pin=${encodeURIComponent(pin)}`;
    $("logfiles").replaceChildren(...(lg.files || []).slice(0, 5).map((f) => el("li", {},
      el("a", { href: `logs/${f.name}${q}`, download: f.name, text: f.name.replace(/^log-|\.jsonl$/g, "") }),
      el("span", { class: "sub", text: `, ${(f.size / 1024).toFixed(0)} KB` }))));
  }

  const sy = s.system;
  let url = s.appURL || location.origin + "/";
  if (/^http:\/\/(127\.|localhost|\[::1\])/.test(url)) { // on the screen: the address phones use instead
    const lan = (sy.interfaces || []).find((i) => i.name !== "bat0" && i.addrs && i.addrs.length);
    if (lan) url = `http://${lan.addrs[0].split("/")[0]}/`;
  }
  const app = [["Address", url], ["Log entries", String(m.entries || 0)]];
  if (s.pins) app.push(["Admin PIN", s.pins.admin, "pin"], ["Hub PIN", s.pins.hub, "pin"]);
  $("app").replaceChildren(...app.flatMap(([k, v, c]) => [el("dt", { text: k }), el("dd", { class: c || "", text: v })]));
}

function renderSysFacts(s) {
  const sy = s.system, r = s.radio;
  const facts = [["Temperature", sy.tempC ? `${sy.tempC.toFixed(0)} °C` : "?"],
    ["Memory free", sy.memTotalMB ? `${sy.memAvailMB} of ${sy.memTotalMB} MB` : "?"],
    ["Card free", sy.diskSizeGB ? `${sy.diskFreeGB.toFixed(1)} of ${sy.diskSizeGB.toFixed(0)} GB` : "?"],
    ["Running for", sy.uptime ? duration(sy.uptime) : "?"],
    ["LoRa stick", sy.meshstick ? "plugged in" : "not found"],
    ["Radio firmware", r && r.firmware ? `meshtasticd ${r.firmware}` : ""]];
  for (const i of sy.interfaces || []) if (i.addrs && i.addrs.length) facts.push([i.name + (i.wifi ? " (WiFi)" : ""), i.addrs.join(", ")]);
  $("sys").replaceChildren(...facts.filter(([, v]) => v).flatMap(([k, v]) => [el("dt", { text: k }), el("dd", { text: v })]));
}

let asking = false, locked = false;
async function poll() {
  try {
    const res = await fetch("api/status", { cache: "no-store", headers: pin ? { "X-Hub-PIN": pin } : {} });
    locked = res.status === 401 || res.status === 429;
    if (locked) {
      // The hub's own WiFi is open to neighbours: reading the panel needs the PIN too.
      // A PIN that was refused is dropped, so the polls don't keep guessing with it.
      if (res.status === 401 && pin) { pin = ""; try { sessionStorage.removeItem("hubPIN"); } catch (e) { /* fine */ } }
      $("onair").textContent = "This hub's WiFi is open to neighbours: the panel needs the hub PIN.";
      $("onair").className = "onair down";
      if (!asking && res.status === 401) {
        asking = true;
        const p = await askPIN();
        asking = false;
        if (p != null) { pin = p; try { sessionStorage.setItem("hubPIN", pin); } catch (e) { /* fine */ } poll(); }
      }
      return;
    }
    if (!res.ok) throw new Error(res.status);
    const st = await res.json();
    render(redact ? redactStatus(st) : st);
  } catch (e) {
    $("onair").textContent = "The hub is not answering. Retrying…";
    $("onair").className = "onair down";
  }
}

// ---- traffic ----
const tr = { seq: 0, frames: [], preset: "", limit: -17.5, drawnSeq: 0 };
let win = load("hub.win", 600);
if (![120, 600, 3600].includes(win)) win = 600;

async function pollTraffic() {
  if (locked) return;
  try {
    const res = await fetch(`api/traffic?since=${tr.seq}`, { cache: "no-store", headers: pin ? { "X-Hub-PIN": pin } : {} });
    if (!res.ok) return;
    const o = await res.json();
    if (redact) redactTraffic(o);
    if (o.seq < tr.seq) { tr.seq = 0; tr.frames = []; tr.drawnSeq = 0; return pollTraffic(); } // the hub restarted
    tr.seq = o.seq; tr.preset = o.preset; tr.limit = o.limit;
    tr.frames.push(...o.frames);
    const cut = Date.now() - 3600e3;
    while (tr.frames.length && Date.parse(tr.frames[0].t) < cut) tr.frames.shift();
    if (view === "traffic") drawTraffic();
  } catch (e) { /* next time */ }
}

function frameText(f) {
  const parts = [`${clockSec(f.t)}: ${WHAT[f.kind] ? WHAT[f.kind](f) : f.kind}`];
  if (f.from) parts.push(`from ${f.name || nameOfID(f.from)}`);
  if (f.to) parts.push(`to ${nameOfID(f.to)}`);
  const s = says(f);
  if (s) parts.push(s);
  if (f.kind === "heard" || f.kind === "garbled") parts.push(db(f.snr));
  return parts.join(", ");
}

let marks = [];
function drawTraffic() {
  const now = Date.now(), span = win * 1000, t0 = now - span;
  const fr = tr.frames.filter((f) => Date.parse(f.t) >= t0);
  const me = last && last.radio ? "!" + hex8(last.radio.myNum) : "";
  const self = nodes().find((n) => n.self);
  for (const b of document.querySelectorAll(".win button")) b.setAttribute("aria-checked", String(Number(b.dataset.win) === win));
  $("tr-sub").textContent = `Everything this hub's radio picks up on ${tr.preset || "its preset"}, readable or not, over the last ${win === 3600 ? "hour" : win / 60 + " minutes"}. Click a packet for what it says.`;

  const n = (k, p) => fr.filter((f) => f.kind === k && (!p || p(f))).length;
  const stats = [["Readable", n("heard", (f) => !f.encrypted)], ["Encrypted", n("heard", (f) => f.encrypted)], ["Unreadable", n("garbled")],
    ["Sent", n("sent")], ["Passed on", n("relayed")], ["Busy", n("busy")]];
  if (self && self.chanUtil != null) stats.push(["Channel in use", `${self.chanUtil.toFixed(1)} %`]);
  if (self && self.airUtilTx != null) stats.push(["Our airtime", `${self.airUtilTx.toFixed(1)} %`]);
  $("tr-stats").replaceChildren(...stats.map(([k, v]) => el("div", {}, el("dt", { text: k }), el("dd", { text: String(v) }))));

  // The timeline: a lane for this hub on top, everything heard below by SNR.
  const T = $("timeline");
  const W = Math.max(320, T.clientWidth || 900), H = Math.max(160, T.clientHeight || 260);
  T.setAttribute("viewBox", `0 0 ${W} ${H}`);
  const ml = 50, mr = 12, laneY = 16, top = 40, bot = H - 22;
  const x = (t) => ml + (t - t0) / span * (W - ml - mr);
  const hi = 10, lo = -25, y = (s) => top + (hi - Math.max(lo, Math.min(hi, s))) / (hi - lo) * (bot - top);
  const g = [];
  const lane = svg("text", { class: "ax", x: ml - 6, y: laneY + 4, "text-anchor": "end" }); lane.textContent = "this hub"; g.push(lane);
  g.push(svg("line", { class: "lane", x1: ml, x2: W - mr, y1: laneY + 12, y2: laneY + 12 }));
  for (const s of [10, 0, -10, -20]) {
    g.push(svg("line", { class: "grid", x1: ml, x2: W - mr, y1: y(s), y2: y(s) }));
    const t = svg("text", { class: "ax", x: ml - 6, y: y(s) + 4, "text-anchor": "end" }); t.textContent = `${minus(s)} dB`; g.push(t);
  }
  g.push(svg("line", { class: "limit", x1: ml, x2: W - mr, y1: y(tr.limit), y2: y(tr.limit) }));
  const lt = svg("text", { class: "ax lim", x: W - mr - 4, y: y(tr.limit) - 5, "text-anchor": "end" });
  lt.textContent = `readable down to about ${minus(tr.limit)} dB`; g.push(lt);
  const step = win === 120 ? 30e3 : win === 600 ? 120e3 : 600e3;
  for (let t = Math.ceil(t0 / step) * step; t <= now; t += step) {
    const tx = svg("text", { class: "ax", x: x(t), y: H - 5, "text-anchor": "middle" });
    tx.textContent = win === 120 ? clockSec(t) : clock(t); g.push(tx);
  }
  marks = [];
  const pxPerMs = (W - ml - mr) / span;
  for (const f of fr) {
    const fx = x(Date.parse(f.t)), fresh = f.seq > tr.drawnSeq ? " fresh" : "";
    const w = Math.max(9, (f.airMs || 0) * pxPerMs);
    let m, my;
    if (f.kind === "heard") {
      my = y(f.snr);
      m = svg("rect", { class: "m heard" + (f.encrypted ? " enc" : "") + (f.to === me ? " us" : "") + fresh, x: fx, y: my - 4, width: w, height: 8, rx: 4 });
    } else if (f.kind === "garbled") {
      my = y(f.snr);
      m = svg("circle", { class: "m garbled" + (f.to === me ? " us" : "") + fresh, cx: fx, cy: my, r: 4 });
    } else if (f.kind === "sent") {
      my = laneY;
      m = svg("rect", { class: "m sent" + fresh, x: fx, y: my - 4, width: w, height: 8, rx: 4 });
    } else if (f.kind === "relayed") {
      my = laneY;
      m = svg("circle", { class: "m relayed" + fresh, cx: fx, cy: my, r: 4 });
    } else {
      my = laneY;
      m = svg("line", { class: "m busy", x1: fx, x2: fx, y1: my - 6, y2: my + 6 });
    }
    g.push(m);
    marks.push({ x: fx + (f.kind === "heard" || f.kind === "sent" ? Math.min(w, 20) / 2 : 0), y: my, f });
  }
  g.push(svg("line", { class: "now", x1: W - mr, x2: W - mr, y1: laneY - 8, y2: bot }));
  T.replaceChildren(...g);
  tr.drawnSeq = tr.seq;

  drawPerMinute(now);
  drawRelays(fr);
  drawFeed(fr);
}

function drawPerMinute(now) {
  const P = $("perminute");
  const W = Math.max(260, P.clientWidth || 400), H = 90, n = 30, gap = 2;
  P.setAttribute("viewBox", `0 0 ${W} ${H}`);
  const bins = Array.from({ length: n }, () => ({ ok: 0, bad: 0, ours: 0 }));
  const end = Math.ceil(now / 60e3) * 60e3;
  for (const f of tr.frames) {
    const i = n - 1 - Math.floor((end - Date.parse(f.t)) / 60e3);
    if (i < 0 || i >= n) continue;
    if (f.kind === "heard") bins[i].ok++; else if (f.kind === "garbled") bins[i].bad++; else if (f.kind === "sent" || f.kind === "relayed") bins[i].ours++;
  }
  const maxv = Math.max(5, ...bins.map((b) => b.ok + b.bad + b.ours));
  const bw = (W - 30) / n, base = H - 16, scale = (base - 4) / maxv;
  const g = [];
  const lbl = svg("text", { class: "ax", x: 0, y: 10 }); lbl.textContent = String(maxv); g.push(lbl);
  bins.forEach((b, i) => {
    let yy = base;
    for (const [k, v] of [["ok", b.ok], ["bad", b.bad], ["ours", b.ours]]) {
      if (!v) continue;
      const h = v * scale;
      const r = svg("rect", { class: "pm " + k, x: 26 + i * bw, y: yy - h, width: Math.max(1, bw - gap), height: Math.max(1, h - (yy < base ? gap : 0)) });
      const t = svg("title"); t.textContent = `${clock(end - (n - i) * 60e3)}: ${b.ok} readable, ${b.bad} unreadable, ${b.ours} sent or passed on`;
      r.append(t); g.push(r);
      yy -= h;
    }
  });
  const a = svg("text", { class: "ax", x: 26, y: H - 3 }); a.textContent = "30 min ago"; g.push(a);
  const z = svg("text", { class: "ax", x: W, y: H - 3, "text-anchor": "end" }); z.textContent = "now"; g.push(z);
  P.replaceChildren(...g);
}

function drawRelays(fr) {
  const counts = new Map();
  for (const f of fr) if (f.relay && (f.kind === "heard" || f.kind === "garbled")) counts.set(f.relay, (counts.get(f.relay) || 0) + 1);
  const top = [...counts.entries()].sort((a, b) => b[1] - a[1]).slice(0, 8);
  const maxv = top.length ? top[0][1] : 1;
  $("relays").replaceChildren(...(top.length ? top.map(([r, c]) => {
    const cands = redact ? [] : nodes().filter((nd) => (nd.num & 0xff) === parseInt(r, 16) && !nd.self).map(nodeName).slice(0, 2);
    return el("li", {},
      el("span", { class: "rl", text: `…${r}` }),
      el("span", { class: "rbar" }, el("i", { style: `width:${(c / maxv * 100).toFixed(0)}%` })),
      el("span", { class: "rc", text: String(c) }),
      el("span", { class: "rn", text: cands.length ? `maybe ${cands.join(" or ")}` : "" }));
  }) : [el("li", { class: "empty", text: "Nothing relayed in this window." })]));
}

function drawFeed(fr) {
  const rows = fr.slice(-80).reverse();
  $("feed").replaceChildren(...rows.map((f) => {
    const row = el("tr", { class: "click " + f.kind + (f.encrypted ? " enc" : ""), tabindex: "0" },
      el("td", { text: clockSec(f.t) }),
      el("td", { class: "what", text: WHAT[f.kind] ? WHAT[f.kind](f) : f.kind }),
      el("td", { text: f.from || f.to ? `${f.from ? f.name || nameOfID(f.from) : "this hub"} → ${nameOfID(f.to)}` : "" }),
      el("td", { class: "says", text: says(f) }),
      el("td", { text: f.kind === "heard" || f.kind === "garbled" ? `${db(f.snr)}${f.rssi ? `, ${minus(f.rssi)} dBm` : ""}` : "" }),
      el("td", { text: f.hops >= 0 && f.kind !== "sent" ? String(f.hops) : "" }),
      el("td", { text: f.relay ? "…" + f.relay : "" }));
    row.addEventListener("click", () => showFrame(f));
    row.addEventListener("keydown", (ev) => { if (ev.key === "Enter") showFrame(f); });
    return row;
  }));
}

// Hover and click on the timeline: the nearest mark within reach.
function nearest(ev) {
  const T = $("timeline"), r = T.getBoundingClientRect(), vb = T.viewBox.baseVal;
  const px = (ev.clientX - r.left) * vb.width / r.width, py = (ev.clientY - r.top) * vb.height / r.height;
  let best = null, bd = 16;
  for (const m of marks) { const d = Math.hypot(m.x - px, m.y - py); if (d < bd) { bd = d; best = m; } }
  return { best, r, vb };
}
$("timeline").addEventListener("pointermove", (ev) => {
  const { best, r, vb } = nearest(ev);
  const tip = $("tip");
  $("timeline").style.cursor = best ? "pointer" : "";
  if (!best) { tip.classList.remove("on"); return; }
  tip.textContent = frameText(best.f);
  tip.style.left = `${Math.min(Math.max(best.x * r.width / vb.width, 120), r.width - 120)}px`;
  tip.style.top = `${best.y * r.height / vb.height}px`;
  tip.classList.add("on");
});
$("timeline").addEventListener("pointerleave", () => $("tip").classList.remove("on"));
$("timeline").addEventListener("click", (ev) => { const { best } = nearest(ev); if (best) showFrame(best.f); });
for (const b of document.querySelectorAll(".win button")) {
  b.addEventListener("click", () => { win = Number(b.dataset.win); save("hub.win", win); drawTraffic(); });
}

// ---- actions ----
function askPIN() {
  return new Promise((resolve) => {
    const d = $("pin");
    $("pinin").value = "";
    d.returnValue = "";
    d.onclose = () => resolve(d.returnValue === "ok" ? $("pinin").value : null);
    d.showModal();
  });
}

async function act(path, body) {
  for (let tries = 0; tries < 3; tries++) {
    const headers = { "Content-Type": "application/json", "X-Hub": "1" };
    if (pin) headers["X-Hub-PIN"] = pin;
    const res = await fetch("api/" + path, { method: "POST", headers, body: JSON.stringify(body || {}) });
    const out = await res.json().catch(() => ({}));
    if (res.status === 401 && !(last && last.local)) {
      const p = await askPIN();
      if (p == null) throw new Error("Not done: the buttons need the hub PIN.");
      pin = p;
      try { sessionStorage.setItem("hubPIN", pin); } catch (e) { /* fine */ }
      continue;
    }
    if (!res.ok) throw new Error((out.error || "The hub said no.").replace(/^meshtastic: /, ""));
    return out;
  }
  throw new Error("Wrong PIN.");
}

async function sendText(input, noteId, body) {
  const text = input.value.trim();
  if (!text) return false;
  try {
    await act("send", { ...body, text });
    input.value = "";
    setNote(noteId, body.to ? "Sent by radio. The tick shows when it arrived." : "Sent by radio. Radios in range get it within a few seconds.");
    poll();
    return true;
  } catch (e) { setNote(noteId, e.message, true); return false; }
}

$("send").addEventListener("submit", async (ev) => {
  ev.preventDefault();
  const body = {};
  if (conv.startsWith("ch")) body.channel = conv.slice(2);
  else if (conv !== "pub") body.to = "!" + hex8(Number(conv));
  if (replyTo && replyTo.id) body.replyId = String(replyTo.id);
  if (await sendText($("msg"), "send-note", body)) { replyTo = null; showReply(); }
});
$("home-send").addEventListener("submit", (ev) => { ev.preventDefault(); sendText($("home-msg"), "home-note", {}); });

$("hello").addEventListener("click", async () => {
  try {
    await act("hello");
    const p = $("plot-home").querySelector(".pulse");
    if (p) { p.classList.remove("go"); void p.getBBox(); p.classList.add("go"); }
    setNote("air-note", "Hello sent. Radios in range answer over the next minute or so.");
  } catch (e) { setNote("air-note", e.message, true); }
});

$("logbtn").addEventListener("click", async () => {
  const on = !(last && last.log && last.log.on);
  try {
    await act("log", { on: String(on) });
    poll();
  } catch (e) { $("log-state").textContent = e.message; }
});

for (const b of document.querySelectorAll(".mesh button")) {
  b.addEventListener("click", async () => {
    const mode = b.dataset.mode;
    if (last && last.mesh && last.mesh.mode === mode) return;
    if (!confirm(`Switch the WiFi mesh to ${b.textContent}? The hub restarts; this page is back in a few seconds.`)) return;
    try {
      await act("mode", { mode });
      $("mesh-state").textContent = "Restarting…";
    } catch (e) { $("mesh-state").textContent = e.message; }
  });
}

// ---- the translator (decision 0030) ----
const translations = new Map(); // message key -> {text, fromName} or {error}
const translating = new Set();
const pairs = () => (last && last.translate && last.translate.pairs) || [];
const transTo = () => (last && last.settings && last.settings.translateTo) || "en";
const autoTranslate = () => !!(last && last.settings && last.settings.translateAuto);
// A quick look for a language the hub can translate into transTo(): German words, or letters English doesn't use.
const DE = new Set("der die das und ist nicht ich du wir ihr sie es ein eine einen mit von zu auf für bei hat haben wer was wo wie noch schon auch aber oder wenn bitte danke hallo gibt jemand kein keine wasser heute morgen hier sind bin war mein uns im am zum zur nach gut ja nein mal wieder alles kann muss".split(" "));
function guessLang(text) {
  const words = text.toLowerCase().match(/[\p{L}']+/gu) || [];
  let de = 0, en = 0;
  for (const w of words) { if (DE.has(w)) de++; if (/[äöüß]/.test(w)) de++; if (["the", "and", "is", "you", "we", "have", "what", "anyone", "please", "thanks"].includes(w)) en++; }
  return de > en ? "de" : en > de ? "en" : "";
}
function canTranslate(text) {
  const to = transTo(), from = guessLang(text);
  return from && from !== to && pairs().includes(`${from}-${to}`);
}
async function translateText(t) {
  const k = textKey(t);
  if (translating.has(k) || translations.has(k)) return;
  translating.add(k);
  try {
    const out = await act("translate", { text: t.text, from: guessLang(t.text) });
    translations.set(k, out);
  } catch (e) { translations.set(k, { error: `Not translated: ${e.message}` }); }
  translating.delete(k);
  threadSig = ""; homeSig = "";
  if (last) render(last);
}
// What you write, into German, for you to check before sending.
$("msg-tr").addEventListener("click", async () => {
  const text = $("msg").value.trim();
  if (!text) return;
  $("msg-tr").disabled = true;
  try {
    const out = await act("translate", { text, from: "en", to: "de" });
    $("msg").value = out.text;
    setNote("send-note", "Translated by the hub. Check it, then send.");
  } catch (e) { setNote("send-note", e.message, true); }
  $("msg-tr").disabled = false;
});

let seenAt = 0;
function markSeen() {
  if (Date.now() - seenAt < 3000) return;
  seenAt = Date.now();
  fetch("api/seen", { method: "POST", headers: { "Content-Type": "application/json", "X-Hub": "1" }, body: "{}" }).catch(() => {});
}

// ---- things every view shows: the sleeping screen, reduced mode, plug events ----
let lastEventAt = null, toastTimer = 0;
function toast(msg) {
  $("toast").textContent = msg;
  $("toast").hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { $("toast").hidden = true; }, 6000);
}
function renderKitGlobal(s) {
  const pw = s.power, root = document.documentElement;
  root.classList.toggle("reduced", !!(pw && pw.reduced));
  const asleep = !!(screen && pw && !pw.screenOn);
  root.classList.toggle("asleep", asleep);
  $("asleep").hidden = !asleep;
  const w = s.wifi;
  $("sys-badge").hidden = !((w && w.mode === "own") || (pw && pw.underVoltage));
  $("msg-tr").hidden = !pairs().includes("en-de");
  const ev = s.devices && s.devices.events && s.devices.events[0];
  if (ev) {
    if (lastEventAt && ev.at !== lastEventAt && screen) toast(`${ev.plugged ? "Plugged in" : "Unplugged"}: ${ev.device.name}`);
    lastEventAt = ev.at;
  } else if (lastEventAt === null) lastEventAt = "";
}

// ---- the System view ----
let sysSig = "";
const sysDirty = new Set();
for (const f of ["wifi-form", "sound-form", "light-form", "tr-form"]) $(f).addEventListener("input", () => sysDirty.add(f));
const POWER_HELP = {
  high: "Everything at full speed; the screen stays on. For mains power.",
  medium: "The CPU up to 1 GHz, WiFi power saving, the screen sleeps after 10 minutes without a key and wakes for a message.",
  low: "The CPU at 600 MHz, WiFi power saving, the Pi's lights off, a reduced screen that sleeps after 2 minutes and wakes only for a key. The radio, sound and LED keep working.",
};
const GPIO = [17, 27, 22, 23, 24, 25, 5, 6, 12, 13, 16, 19, 20, 21, 26, 4, 18];
const PIN = { 4: 7, 5: 29, 6: 31, 12: 32, 13: 33, 16: 36, 17: 11, 18: 12, 19: 35, 20: 38, 21: 40, 22: 15, 23: 16, 24: 18, 25: 22, 26: 37, 27: 13 };
for (const id of ["lt-led", "lt-btn"]) {
  $(id).append(el("option", { value: "-1", text: "none" }), ...GPIO.map((g) => el("option", { value: String(g), text: `GPIO ${g} (pin ${PIN[g]})` })));
}

function mmss(sec) { return `${Math.floor(sec / 60)}:${String(sec % 60).padStart(2, "0")}`; }

function wifiText(w) {
  if (!w.available) return ["The hub's WiFi can't be managed here (no NetworkManager).", "bad"];
  switch (w.mode) {
    case "home": return [`On the home WiFi “${w.ssid || w.connection}”${w.signal ? `, signal ${w.signal} %` : ""}.`, "good"];
    case "own": return [`The hub's own WiFi is on${w.startedBy === "auto" ? " (started by itself: no home WiFi)" : w.startedBy === "button" ? " (from the button)" : ""}: ${w.clients} device${w.clients === 1 ? "" : "s"} on it.`, "good"];
    case "switching": return ["Switching the WiFi…", ""];
  }
  if (w.waiting) return ["No WiFi. The hub waits for the home WiFi, as you chose; start its own WiFi here when you need it.", "bad"];
  if (w.fallbackIn >= 0) return [`No WiFi. The hub starts its own in ${mmss(w.fallbackIn)} unless the home WiFi comes back.`, "bad"];
  return ["No WiFi.", "bad"];
}

function drawQR(box, text) {
  box.replaceChildren();
  if (!window.qrcode || !text) return;
  qrcode.stringToBytes = qrcode.stringToBytesFuncs["UTF-8"];
  const q = qrcode(0, "M");
  q.addData(text);
  q.make();
  const n = q.getModuleCount();
  let d = "";
  for (let r = 0; r < n; r++) for (let c = 0; c < n; c++) if (q.isDark(r, c)) d += `M${c} ${r}h1v1h-1z`;
  const s = svg("svg", { viewBox: `0 0 ${n} ${n}`, "shape-rendering": "crispEdges" });
  s.append(svg("path", { d, fill: "#000" }));
  box.append(s);
}
const wifiEsc = (v) => v.replace(/([\\;,:"])/g, "\\$1");
const wifiQR = (w) => `WIFI:T:${w.open ? "nopass" : "WPA"};S:${wifiEsc(w.ownSSID)};${w.open ? "" : `P:${wifiEsc(w.password || "")};`};`;

function renderWiFi(s) {
  const w = s.wifi;
  $("wifi-card").hidden = !w;
  if (!w) return;
  const [txt, cls] = wifiText(w);
  $("wifi-state").textContent = txt + (w.error ? ` ${w.error}` : "");
  $("wifi-state").className = "state " + (w.error ? "bad" : cls);
  if (w.note && !$("wifi-note").dataset.mine) setNote("wifi-note", w.note);
  const own = w.mode === "own";
  $("wifi-own").hidden = own;
  $("wifi-home").hidden = !own && w.mode !== "switching";
  $("wifi-own").disabled = $("wifi-home").disabled = w.mode === "switching" || !w.available;
  // The hub's own WiFi: its name, password and a QR code; the password only on the hub's screen.
  const box = $("own-box");
  box.hidden = !own;
  if (own) {
    const host = (s.system.hostname || "this-hub");
    const facts = [["WiFi name", w.ownSSID]];
    if (w.open) facts.push(["Password", "none: open to neighbours"]);
    else facts.push(["Password", w.password || "shown on the hub's own screen", w.password ? "pin" : ""]);
    facts.push(["Then open", `http://${w.addr}/hub/  or  http://${host}.local/hub/`], ["The house app", `http://${w.addr}/`]);
    $("own-facts").replaceChildren(...facts.flatMap(([k, v, c]) => [el("dt", { text: k }), el("dd", { class: c || "", text: v })]));
    if (w.open || w.password) drawQR($("wifi-qr"), wifiQR(w)); else $("wifi-qr").replaceChildren();
    $("wifi-qr").hidden = !(w.open || w.password);
    $("wifi-print").hidden = !(w.open || w.password);
  }
  if (!sysDirty.has("wifi-form")) {
    $("wifi-auto").checked = w.auto;
    $("wifi-after").value = String(w.afterMinutes);
    $("wifi-neigh").checked = w.open;
    $("wifi-ssid").value = w.ownSSID;
  }
  $("wifi-saved").replaceChildren(...(w.saved.length ? w.saved.map((n) => {
    const li = el("li", { class: n.active ? "on" : "" }, n.ssid || n.name, n.default ? el("span", { class: "sub", text: " (joined first)" }) : null,
      n.active ? el("span", { class: "sub", text: ", in use" }) : null);
    if (!n.default) { const b = el("button", { class: "btn ghost small", type: "button", text: "Make it the default" }); b.addEventListener("click", () => wifiAct({ op: "default", name: n.name }, "Done: the hub joins it first from now on.")); li.append(b); }
    const f = el("button", { class: "btn ghost small", type: "button", text: "Forget" });
    f.addEventListener("click", () => { if (confirm(`Forget ${n.ssid || n.name}? The hub won't join it again until you give its password.`)) wifiAct({ op: "forget", name: n.name }, "Forgotten."); });
    li.append(f);
    return li;
  }) : [el("li", { class: "empty", text: "None yet." })]));
}

async function wifiAct(body, ok) {
  $("wifi-note").dataset.mine = "1";
  setNote("wifi-note", body.op === "own" ? "Starting the hub's own WiFi…" : body.op === "home" ? "Going back to the home WiFi…" : body.op === "join" ? `Joining ${body.ssid}… (up to 40 s)` : "Working…");
  try {
    await act("wifi", body);
    setNote("wifi-note", ok);
    poll();
  } catch (e) {
    // A device on the WiFi that just changed loses the hub for a moment: say where to find it.
    setNote("wifi-note", /fetch|network/i.test(e.message) ? "The hub's WiFi changed: if this page stops, join the network it is on now." : e.message, !/fetch|network/i.test(e.message));
  }
  setTimeout(() => { delete $("wifi-note").dataset.mine; }, 15000);
}
$("wifi-own").addEventListener("click", () => {
  if (!(last && last.local) && !confirm("Start the hub's own WiFi? This device loses the hub until it joins the hub's WiFi (its name and password are on the hub's screen).")) return;
  wifiAct({ op: "own" }, "The hub's own WiFi is on.");
});
$("wifi-home").addEventListener("click", () => wifiAct({ op: "home" }, "Back to the home WiFi."));
$("wifi-form").addEventListener("submit", async (ev) => {
  ev.preventDefault();
  const neigh = $("wifi-neigh").checked;
  if (neigh && !(last && last.wifi && last.wifi.open) && !confirm("Let neighbours in? The hub's own WiFi then has no password: anyone in range can join it and use the house app.")) return;
  try {
    await act("wifi", { op: "settings", auto: String($("wifi-auto").checked), afterMinutes: $("wifi-after").value, neighbours: String(neigh), ssid: $("wifi-ssid").value.trim() });
    sysDirty.delete("wifi-form");
    setNote("wifi-form-note", "Saved.");
    poll();
  } catch (e) { setNote("wifi-form-note", e.message, true); }
});
$("wifi-scan").addEventListener("click", async () => {
  $("wifi-scan").disabled = true;
  $("wifi-nearby").replaceChildren(el("li", { class: "empty", text: "Looking…" }));
  try {
    const out = await act("wifi", { op: "scan" });
    $("wifi-nearby").replaceChildren(...(out.networks.length ? out.networks.map((n) => {
      const li = el("li", { class: n.inUse ? "on" : "" }, n.ssid, el("span", { class: "sub", text: `, ${n.signal} %${n.security ? ", " + n.security : ", open"}${n.inUse ? ", in use" : n.saved ? ", known" : ""}` }));
      if (!n.inUse) {
        const b = el("button", { class: "btn ghost small", type: "button", text: n.saved ? "Join" : "Join…" });
        b.addEventListener("click", () => {
          if (n.saved) { wifiAct({ op: "join", ssid: n.ssid }, `On ${n.ssid}.`); return; }
          openJoin(n.ssid, !n.security);
        });
        li.append(b);
      }
      return li;
    }) : [el("li", { class: "empty", text: "No networks in range." })]));
  } catch (e) { $("wifi-nearby").replaceChildren(el("li", { class: "empty", text: e.message })); }
  $("wifi-scan").disabled = false;
});
function openJoin(ssid, open) {
  $("join-form").hidden = false;
  $("join-ssid").value = ssid || "";
  $("join-pass").value = "";
  $("join-pass").placeholder = open ? "open network: no password" : "";
  (ssid ? $("join-pass") : $("join-ssid")).focus();
}
$("join-other").addEventListener("click", () => openJoin("", false));
$("join-cancel").addEventListener("click", () => { $("join-form").hidden = true; });
$("join-form").addEventListener("submit", (ev) => {
  ev.preventDefault();
  const ssid = $("join-ssid").value.trim();
  $("join-form").hidden = true;
  wifiAct({ op: "join", ssid, password: $("join-pass").value, hidden: String($("join-hidden").checked) }, `On ${ssid}. The hub joins it by itself from now on.`);
  $("join-pass").value = "";
});
$("wifi-print").addEventListener("click", () => {
  const w = last && last.wifi;
  if (!w) return;
  const card = el("div", { id: "wifi-print-card" }, el("h2", { text: `${$("hubname").textContent}: its own WiFi` }),
    el("p", { text: `WiFi name: ${w.ownSSID}` }), el("p", { text: w.open ? "No password" : `Password: ${w.password}` }),
    el("div", { class: "qr", id: "print-qr" }), el("p", { text: `Then open http://${w.addr}/` }));
  document.body.append(card);
  drawQR($("print-qr"), wifiQR(w));
  document.body.classList.add("wifi-card-print");
  window.print();
  document.body.classList.remove("wifi-card-print");
  card.remove();
});

function renderSound(s) {
  const so = s.sound;
  $("sound-form").hidden = !so;
  if (!so) return;
  const st = so.settings;
  const where = so.output === "jack" ? "the audio jack" : so.output === "usb" ? `a USB speaker (${so.card})` : "";
  $("sound-state").textContent = !where ? "No sound output found." : `Sound goes to ${where}.${so.output === "jack" ? " The Pi can't tell whether a speaker is plugged into the jack: press Play a test sound." : ""}${so.quietNow ? " Quiet hours now." : ""}${so.error ? " " + so.error : ""}`;
  $("sound-state").className = "state" + (!where || so.error ? " bad" : "");
  $("snd-speak-hint").textContent = so.canSpeak ? "" : "(espeak-ng is not installed)";
  $("snd-speak").disabled = $("snd-read").disabled = !so.canSpeak;
  if (sysDirty.has("sound-form")) return;
  $("snd-on").checked = st.on; $("snd-out").value = st.output; $("snd-vol").value = String(st.volume);
  $("snd-vol-v").textContent = `${st.volume} %`;
  for (const [id, k] of [["snd-direct", "direct"], ["snd-public", "public"], ["snd-replies", "replies"], ["snd-delivered", "delivered"], ["snd-speak", "speak"], ["snd-read", "readAloud"], ["snd-quiet", "quiet"]]) $(id).checked = st[k];
  $("snd-from").value = st.quietFrom; $("snd-to").value = st.quietTo;
}
$("snd-vol").addEventListener("input", () => { $("snd-vol-v").textContent = `${$("snd-vol").value} %`; });
$("sound-form").addEventListener("submit", async (ev) => {
  ev.preventDefault();
  const b = (id) => String($(id).checked);
  try {
    await act("sound", { on: b("snd-on"), output: $("snd-out").value, volume: $("snd-vol").value, direct: b("snd-direct"), public: b("snd-public"),
      replies: b("snd-replies"), delivered: b("snd-delivered"), speak: b("snd-speak"), readAloud: b("snd-read"), quiet: b("snd-quiet"),
      quietFrom: $("snd-from").value, quietTo: $("snd-to").value });
    sysDirty.delete("sound-form");
    setNote("sound-note", "Saved.");
    poll();
  } catch (e) { setNote("sound-note", e.message, true); }
});
$("snd-test").addEventListener("click", async () => {
  try { await act("sound", { op: "test" }); setNote("sound-note", "Playing a chime and a sentence. Nothing heard? Check the speaker's power, its plug and the volume."); }
  catch (e) { setNote("sound-note", e.message, true); }
});

function renderLight(s) {
  const l = s.light;
  $("light-form").hidden = !l;
  if (!l) return;
  const st = l.settings, parts = [];
  parts.push(st.led < 0 ? "No LED." : l.ledOK ? `LED on GPIO ${st.led} (pin ${l.ledPin})${l.blinking ? ", blinking: unread messages" : ""}.` : `LED on GPIO ${st.led}: not available.`);
  parts.push(st.button < 0 ? "No button." : l.buttonOK ? `Button on GPIO ${st.button} (pin ${l.buttonPin})${new Date(l.lastPress).getFullYear() > 2000 ? `, last pressed ${ago(l.lastPress)}` : ""}.` : `Button on GPIO ${st.button}: not available.`);
  $("light-state").textContent = parts.join(" ") + (l.error ? ` ${l.error}` : "");
  $("light-state").className = "state" + (l.error ? " bad" : "");
  if (sysDirty.has("light-form")) return;
  $("lt-led").value = String(st.led); $("lt-btn").value = String(st.button);
  $("lt-keep").checked = st.keepBlinking; $("lt-public").checked = st.public;
}
$("light-form").addEventListener("submit", async (ev) => {
  ev.preventDefault();
  try {
    await act("light", { led: $("lt-led").value, button: $("lt-btn").value, keepBlinking: String($("lt-keep").checked), public: String($("lt-public").checked) });
    sysDirty.delete("light-form");
    setNote("light-note", "Saved.");
    poll();
  } catch (e) { setNote("light-note", e.message, true); }
});
$("lt-test").addEventListener("click", async () => {
  try { await act("light", { op: "test" }); setNote("light-note", "Five blinks. Nothing? Check the LED's legs (long leg to the pin side) and the resistor."); }
  catch (e) { setNote("light-note", e.message, true); }
});

function renderPower(s) {
  const pw = s.power;
  if (!pw) return;
  for (const b of document.querySelectorAll(".power button")) b.setAttribute("aria-checked", String(b.dataset.power === pw.settings.mode));
  $("power-help").textContent = POWER_HELP[pw.settings.mode] || "";
  $("pw-reduced").checked = pw.reduced;
  $("pw-reduced").disabled = pw.settings.mode === "low";
  const parts = [];
  parts.push(pw.noScreen ? "No screen plugged in." : pw.sleepsIn < 0 ? "The screen stays on." : !pw.screenOn ? "The screen is asleep." : `The screen sleeps in ${mmss(pw.sleepsIn)} without a key.`);
  if (pw.uvKnown) parts.push(pw.underVoltage ? "The power supply is too weak right now: use a better power bank or a shorter, thicker cable." : pw.uvCount ? `The power supply was too weak ${pw.uvCount} time${pw.uvCount === 1 ? "" : "s"}, last ${ago(pw.uvLast)}.` : "The power supply is good.");
  if (s.mesh && s.mesh.mode === "real") parts.push("The WiFi mesh stick uses about 1 W: switch the mesh off in Settings if you don't need it.");
  $("power-state").textContent = parts.join(" ") + (pw.error ? ` ${pw.error}` : "");
  $("power-state").className = "state" + (pw.underVoltage || pw.error ? " bad" : "");
}
for (const b of document.querySelectorAll(".power button")) {
  b.addEventListener("click", async () => {
    try { await act("power", { mode: b.dataset.power }); setNote("power-note", `Power mode ${b.textContent}.`); poll(); }
    catch (e) { setNote("power-note", e.message, true); }
  });
}
$("pw-reduced").addEventListener("change", async () => {
  try { await act("power", { reduced: String($("pw-reduced").checked) }); poll(); }
  catch (e) { setNote("power-note", e.message, true); }
});

function renderTranslator(s) {
  const t = s.translate;
  $("tr-form").hidden = !t;
  if (!t) return;
  const names = { de: "German", en: "English", uk: "Ukrainian", ru: "Russian", pl: "Polish", tr: "Turkish", ar: "Arabic", fr: "French" };
  const nm = (c) => names[c] || c;
  $("tr-state").textContent = !t.installed ? "The translator is not installed on this hub (deploy/pi/translate-setup.sh)." :
    t.pairs.length ? `Installed: ${t.pairs.map((p) => p.split("-").map(nm).join(" → ")).join(", ")}.${t.loaded ? " Loaded and ready." : " It loads in a few seconds on first use."}${t.error ? " " + t.error : ""}` : "No languages installed.";
  $("tr-state").className = "state" + (!t.installed || t.error ? " bad" : "");
  if (sysDirty.has("tr-form")) return;
  const targets = [...new Set(t.pairs.map((p) => p.split("-")[1]))];
  if (!targets.includes("en")) targets.unshift("en");
  $("tr-to").replaceChildren(...targets.map((c) => el("option", { value: c, text: nm(c) })));
  $("tr-to").value = transTo();
  $("tr-auto").checked = autoTranslate();
}
$("tr-form").addEventListener("submit", async (ev) => {
  ev.preventDefault();
  try {
    await act("settings", { translateTo: $("tr-to").value, translateAuto: String($("tr-auto").checked) });
    sysDirty.delete("tr-form");
    translations.clear();
    setNote("tr-note", "Saved.");
    poll();
  } catch (e) { setNote("tr-note", e.message, true); }
});

const DEV_ICON = { screen: "🖥", keyboard: "⌨", mouse: "🖱", touch: "👆", sound: "🔊", lora: "📡", wifi: "📶", gps: "🛰", storage: "💾", usb: "🔌" };
function renderDevices(s) {
  const d = s.devices;
  if (!d) return;
  $("devlist").replaceChildren(...(d.devices.length ? d.devices.map((x) => el("li", {}, `${DEV_ICON[x.kind] || "•"} ${x.name}`, x.detail ? el("span", { class: "sub", text: `, ${x.detail}` }) : null))
    : [el("li", { class: "empty", text: "Nothing found." })]));
  $("devevents").replaceChildren(...((d.events || []).length ? d.events.slice(0, 6).map((e) => el("li", {}, `${e.plugged ? "Plugged in" : "Unplugged"}: ${e.device.name}`, el("span", { class: "sub", text: `, ${ago(e.at)}` })))
    : [el("li", { class: "empty", text: "Nothing plugged in or out since the hub started." })]));
}

function renderSystem(s) {
  renderWiFi(s); renderSound(s); renderLight(s); renderPower(s); renderTranslator(s); renderDevices(s); renderSysFacts(s); renderMaps(s);
  $("sys-term").hidden = !s.local;
  // A device with the right time can set the hub's clock (after a power cut without internet).
  const off = Date.now() - Date.parse(s.system.time);
  const wrong = !s.local && Math.abs(off) > 120e3;
  $("clock-line").hidden = $("sys-clock").hidden = !wrong;
  if (wrong) $("clock-line").textContent = `The hub's clock is ${duration(Math.abs(off) / 1000)} ${off > 0 ? "behind" : "ahead of"} this device's.`;
}
const MB = (b) => `${Math.max(1, Math.round(b / 1048576))} MB`;
function renderMaps(s) {
  const m = s.maps;
  $("maps-card").hidden = !m;
  if (!m) return;
  const busy = !!m.getting;
  const getting = busy ? m.areas.find((a) => a.id === m.getting) : null;
  $("maps-state").textContent = !m.tool ? "The tool that fetches maps is not installed: run setup.sh again with internet."
    : busy ? `Fetching ${getting ? getting.name : m.getting}: ${MB(m.soFar || 0)} so far. A city takes a minute or two.`
    : m.areas.some((a) => a.have) ? "The map shows these streets with no internet." : "No map kept yet: the map's streets need the internet.";
  $("maps-state").className = "state" + (m.error ? " bad" : "");
  $("maps-err").hidden = !m.error;
  $("maps-err").textContent = m.error ? `Not fetched: ${m.error}` : "";
  const sig = JSON.stringify(m);
  if (sig === renderMaps.sig) return;
  renderMaps.sig = sig;
  $("maplist-off").replaceChildren(...m.areas.map((a) => {
    const btn = (label, op, ask) => {
      const b = el("button", { class: "btn ghost small", type: "button", text: label });
      b.disabled = busy || !m.tool && op === "get";
      b.addEventListener("click", async () => {
        if (ask && !confirm(ask)) return;
        try { await act("maps", { op, area: a.id }); setNote("maps-note", op === "get" ? `Fetching ${a.name}…` : `${a.name} removed.`); poll(); }
        catch (e) { setNote("maps-note", e.message, true); }
      });
      return b;
    };
    return el("li", {}, el("span", { text: a.name }),
      el("span", { class: "sub", text: a.have ? `, ${MB(a.bytes)}, map of ${a.build.slice(0, 4)}-${a.build.slice(4, 6)}-${a.build.slice(6)}` : a.id === "here" ? ", about 55 km across" : "" }),
      " ", a.have ? btn("Update", "get") : btn("Get", "get"), a.have ? btn("Remove", "remove", `Remove the map of ${a.name}? The map then needs the internet there.`) : null);
  }));
}
async function sysAct(body, ask) {
  if (ask && !confirm(ask)) return;
  try { const out = await act("system", body); setNote("sys-note", out.note || "Done."); }
  catch (e) { setNote("sys-note", e.message, true); }
}
$("sys-term").addEventListener("click", () => sysAct({ op: "terminal" }));
$("sys-reboot").addEventListener("click", () => sysAct({ op: "reboot" }, "Restart the hub? The radio and the panel are back in about a minute."));
$("sys-off").addEventListener("click", () => sysAct({ op: "poweroff" }, "Switch the hub off? It only comes back when its power is unplugged and plugged in again."));
$("sys-clock").addEventListener("click", () => sysAct({ op: "clock", ms: String(Date.now()) }));

// ---- help: a page of its own in the binary, so it works offline ----
let helpLoaded = false;
async function loadHelp() {
  if (!helpLoaded) {
    try {
      const res = await fetch("help.html", { cache: "no-store" });
      if (!res.ok) throw new Error(res.status);
      $("help-body").innerHTML = await res.text(); // our own file, served by the hub
      helpLoaded = true;
    } catch (e) { $("help-body").textContent = "The help could not load: " + e.message; return; }
    for (const b of document.querySelectorAll(".toc button")) b.addEventListener("click", () => $(b.dataset.to).scrollIntoView({ behavior: "smooth", block: "start" }));
    $("help-print").addEventListener("click", () => window.print());
    $("help-find").addEventListener("input", () => {
      const q = $("help-find").value.trim().toLowerCase();
      const has = (e) => e.textContent.toLowerCase().includes(q);
      for (const s of document.querySelectorAll(".hs")) {
        const whole = !q || has(s.querySelector("h2"));
        let any = whole;
        for (const item of s.querySelectorAll("p, li, tbody tr:not(.grp), dt")) {
          // a term and its explanation show or hide together
          const pair = item.tagName === "DT" ? [item, item.nextElementSibling] : [item];
          const hit = whole || pair.some(has);
          for (const x of pair) x.classList.toggle("miss", !hit);
          any = any || hit;
        }
        s.hidden = !any;
      }
    });
  }
  fillHelp();
}

// fillHelp puts this hub's own name and addresses into the help.
function fillHelp() {
  if (!helpLoaded || !last) return;
  const sy = last.system, r = last.radio, me = nodes().find((n) => n.self);
  const lan = (sy.interfaces || []).find((i) => i.name !== "bat0" && i.addrs && i.addrs.length);
  const ip = lan ? lan.addrs[0].split("/")[0] : "";
  const vals = { name: (me && me.longName) || "this hub", preset: (r && r.preset) || "its preset", host: sy.hostname || "this-hub",
    url: ip ? `http://${ip}/hub/` : `http://${sy.hostname || "this-hub"}.local/hub/`, local: `http://${sy.hostname || "this-hub"}.local/hub/` };
  for (const e of document.querySelectorAll("[data-fill]")) if (vals[e.dataset.fill]) e.textContent = vals[e.dataset.fill];
}

// Keys for the hub's own keyboard: 1 to 8 switch views; / starts a message.
document.addEventListener("keydown", (ev) => {
  const typing = ["INPUT", "TEXTAREA", "SELECT"].includes(document.activeElement.tagName);
  if (ev.key === "Escape") { if (replyTo) { replyTo = null; showReply(); } else if (typing) document.activeElement.blur(); return; }
  if (typing || ev.ctrlKey || ev.metaKey || ev.altKey || $("detail").open || $("pin").open) return;
  if (ev.key === "/") { ev.preventDefault(); setView("messages"); $("msg").focus(); }
  const i = ["1", "2", "3", "4", "5", "6", "7", "8"].indexOf(ev.key);
  if (i >= 0) setView(VIEWS[i]);
});
document.querySelector(".tabs").addEventListener("keydown", (ev) => {
  if (ev.key !== "ArrowRight" && ev.key !== "ArrowLeft") return;
  const i = (VIEWS.indexOf(view) + (ev.key === "ArrowRight" ? 1 : VIEWS.length - 1)) % VIEWS.length;
  setView(VIEWS[i]); $("tab-" + VIEWS[i]).focus();
});
window.addEventListener("resize", () => { if (view === "traffic") drawTraffic(); if (map && view === "map") map.invalidateSize(); });

setView(view);
// The refresh slows down in low power and while the hub's screen sleeps (decision 0029).
function every() {
  const pw = last && last.power;
  if (!pw) return 2000;
  return 1000 * (screen ? pw.pollSeconds : Math.min(pw.pollSeconds, pw.settings.mode === "low" ? 5 : 2));
}
(async function loop() { await poll(); setTimeout(loop, every()); })();
(async function loopTraffic() { await pollTraffic(); setTimeout(loopTraffic, every()); })();
setInterval(() => { if (view === "traffic" && !document.documentElement.classList.contains("asleep")) drawTraffic(); }, 1000);
