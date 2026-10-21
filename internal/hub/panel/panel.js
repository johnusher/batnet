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
const PRESETS = {
  LongSlow: { what: "Furthest and slowest. Few people use it.", rate: "125 kHz" },
  LongModerate: { what: "Far and slow.", rate: "125 kHz" },
  LongFast: { what: "Meshtastic's default. Far, but a message takes about a second on the air, so a busy mesh clogs.", rate: "1.07 kbit/s, 153 dB" },
  MediumSlow: { what: "A middle way.", rate: "1.95 kbit/s, 150.5 dB" },
  MediumFast: { what: "What Berlin's mesh uses. Three times faster than LongFast, a little less range.", rate: "3.52 kbit/s, 148 dB" },
  ShortSlow: { what: "Short range, fast: very dense meshes.", rate: "6.25 kbit/s, 145.5 dB" },
  ShortFast: { what: "Short range, faster: events, a few streets.", rate: "10.94 kbit/s, 143 dB" },
  ShortTurbo: { what: "Fastest and shortest, 500 kHz wide.", rate: "21.88 kbit/s, 140 dB" },
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

// ---- views ----
const VIEWS = ["home", "messages", "map", "radios", "traffic", "settings"];
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

function showRadio(num) {
  const n = byNum(num);
  if (!n) return;
  openDetail(nodeName(n), [n.shortName, n.hardware, n.role].filter(Boolean).join(", "), [
    ["ID", n.id],
    ["Signal", n.self ? "" : `${db(n.snr)}${n.rssi ? `, ${minus(n.rssi)} dBm` : ""}`],
    ["Relays", n.self ? "" : n.hops < 0 ? "not known" : n.hops === 0 ? "none: heard directly" : String(n.hops)],
    ["Last heard", n.self ? "" : ago(n.lastHeard)],
    ["Packets heard", n.self ? "" : n.packets],
    ["Battery", n.battery != null ? (n.battery > 100 ? "on mains power" : `${n.battery} %`) : ""],
    ["Its channel in use", n.chanUtil != null ? `${n.chanUtil.toFixed(1)} %` : ""],
    ["Where", n.position ? `${placeText(n.position)} (shared ${ago(n.position.at)})` : "has not shared it"],
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
let conv = "pub";            // the open conversation: "pub" or a node number as a string
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
    if (t.to === BCAST) { if (t.channel !== 0) continue; id = "pub"; }
    else if (t.mine) id = String(t.to);
    else if (t.to === me) id = String(t.from);
    else continue;
    if (!out.has(id)) out.set(id, { id, num: Number(id), texts: [] });
    out.get(id).texts.push(t);
  }
  for (const id of started) if (!out.has(id)) out.set(id, { id, num: Number(id), texts: [] });
  for (const c of out.values()) {
    c.name = c.id === "pub" ? "Everyone nearby" : nameOfNum(c.num);
    c.lastAt = c.texts.length ? Date.parse(c.texts[c.texts.length - 1].at) : 0;
    c.unread = c.texts.filter((t) => !t.mine && Date.parse(t.at) > readTime(c.id)).length;
  }
  return [...out.values()].sort((a, b) => (a.id === "pub" ? -1 : b.id === "pub" ? 1 : b.lastAt - a.lastAt));
}

function openConv(id) {
  if (id !== "pub") started.add(id);
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
  if (who) li.append(el("span", { class: "who", text: who }));
  if (t.replyId) {
    const q = all.find((x) => x.id === t.replyId);
    li.append(el("span", { class: "quote", text: q ? `${q.mine ? "You" : nameOfNum(q.from)}: ${q.text}` : "An earlier message" }));
  }
  li.append(el("span", { class: "body", text: t.text }));
  const meta = el("span", { class: "meta" }, clock(t.at));
  if (!t.mine) meta.append(`, ${db(t.snr)}, ${t.hops < 0 ? "?" : t.hops} relays${t.pki ? ", encrypted to this hub" : ""}`);
  const st = t.mine ? statusLine(t) : null;
  if (st) meta.append(" ", st);
  if (reply) {
    const rb = el("button", { class: "linkbtn", type: "button", text: "Reply" });
    rb.addEventListener("click", () => { replyTo = t; showReply(); $("msg").focus(); });
    meta.append(" ", rb);
  }
  li.append(meta);
  return li;
}

function renderMessages(r, convs) {
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
  }
  if (view !== "messages") return;

  $("convs").replaceChildren(...convs.map((x) => {
    const lastT = x.texts[x.texts.length - 1];
    const b = el("button", { class: "conv" + (x.id === conv ? " on" : ""), type: "button", "aria-current": String(x.id === conv) },
      el("span", { class: "cn", text: x.name }),
      x.unread ? el("span", { class: "badge", text: String(x.unread) }) : null,
      el("span", { class: "cp", text: lastT ? `${lastT.mine ? "You: " : x.id === "pub" ? nameOfNum(lastT.from) + ": " : ""}${lastT.text}` : x.id === "pub" ? "The open channel" : "No messages yet" }));
    b.addEventListener("click", () => openConv(x.id));
    return el("li", {}, b);
  }));

  const node = c.id === "pub" ? null : byNum(c.num);
  $("thread-name").textContent = c.name;
  $("thread-sub").textContent = c.id === "pub"
    ? `Meshtastic's open ${r.preset || ""} channel: anyone with a radio nearby can read it. Don't send anything private.`
    : `Only this radio gets it.${node ? ` Last heard ${ago(node.lastHeard)}, ${db(node.snr)}, ${node.hops < 0 ? "?" : node.hops} relays.` : ""} Messages to one radio are encrypted between the two once they know each other's key.`;
  $("msg").placeholder = c.id === "pub" ? "Message to everyone in range" : `Message to ${c.name}`;
  $("thread-empty").hidden = c.texts.length > 0;
  $("thread-empty").textContent = c.id === "pub" ? "No messages heard yet." : "No messages yet. Write the first one below.";

  const sig = conv + "|" + c.texts.map((t) => textKey(t) + t.status).join(",") + "|" + nodes().length;
  if (sig === threadSig) return;
  threadSig = sig;
  const list = $("thread");
  const atBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 40;
  list.replaceChildren(...c.texts.map((t) => bubble(t, r.texts || [], { who: !t.mine && c.id === "pub" ? nameOfNum(t.from) : "", reply: true })));
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
  const sig = show.map((x) => textKey(x.t) + x.t.status).join(",");
  if (sig === homeSig) return;
  homeSig = sig;
  $("home-thread").replaceChildren(...show.map(({ t, c }) => {
    const who = t.mine ? (c.id === "pub" ? "You, to everyone" : `You, to ${c.name}`) : `${nameOfNum(t.from)}${c.id === "pub" ? "" : ", to you"}`;
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
      if (++errors < 3) return;
      $("map-note").textContent = "No map tiles: they need the internet. The radios are still in the right places.";
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

function drawMap() {
  if (!map || view !== "map") return;
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
    m.bindTooltip(n.shortName || n.id.slice(-4), { permanent: true, direction: "right", offset: [6, 0], className: "maplbl" });
    m.on("click", () => showRadio(n.num));
  }
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
  const facts = [["Temperature", sy.tempC ? `${sy.tempC.toFixed(0)} °C` : "?"],
    ["Memory free", sy.memTotalMB ? `${sy.memAvailMB} of ${sy.memTotalMB} MB` : "?"],
    ["Card free", sy.diskSizeGB ? `${sy.diskFreeGB.toFixed(1)} of ${sy.diskSizeGB.toFixed(0)} GB` : "?"],
    ["Running for", sy.uptime ? duration(sy.uptime) : "?"],
    ["LoRa stick", sy.meshstick ? "plugged in" : "not found"],
    ["Radio firmware", r && r.firmware ? `meshtasticd ${r.firmware}` : ""]];
  for (const i of sy.interfaces || []) if (i.addrs && i.addrs.length) facts.push([i.name + (i.wifi ? " (WiFi)" : ""), i.addrs.join(", ")]);
  $("sys").replaceChildren(...facts.filter(([, v]) => v).flatMap(([k, v]) => [el("dt", { text: k }), el("dd", { text: v })]));

  let url = s.appURL || location.origin + "/";
  if (/^http:\/\/(127\.|localhost|\[::1\])/.test(url)) { // on the screen: the address phones use instead
    const lan = (sy.interfaces || []).find((i) => i.name !== "bat0" && i.addrs && i.addrs.length);
    if (lan) url = `http://${lan.addrs[0].split("/")[0]}/`;
  }
  const app = [["Address", url], ["Log entries", String(m.entries || 0)]];
  if (s.pins) app.push(["Admin PIN", s.pins.admin, "pin"], ["Hub PIN", s.pins.hub, "pin"]);
  $("app").replaceChildren(...app.flatMap(([k, v, c]) => [el("dt", { text: k }), el("dd", { class: c || "", text: v })]));
}

async function poll() {
  try {
    const res = await fetch("api/status", { cache: "no-store" });
    if (!res.ok) throw new Error(res.status);
    render(await res.json());
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
  try {
    const res = await fetch(`api/traffic?since=${tr.seq}`, { cache: "no-store" });
    if (!res.ok) return;
    const o = await res.json();
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
    const cands = nodes().filter((nd) => (nd.num & 0xff) === parseInt(r, 16) && !nd.self).map(nodeName).slice(0, 2);
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
  if (conv !== "pub") body.to = "!" + hex8(Number(conv));
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

// Keys for the hub's own keyboard: 1 to 6 switch views; / starts a message.
document.addEventListener("keydown", (ev) => {
  const typing = ["INPUT", "TEXTAREA", "SELECT"].includes(document.activeElement.tagName);
  if (ev.key === "Escape") { if (replyTo) { replyTo = null; showReply(); } else if (typing) document.activeElement.blur(); return; }
  if (typing || ev.ctrlKey || ev.metaKey || ev.altKey || $("detail").open || $("pin").open) return;
  if (ev.key === "/") { ev.preventDefault(); setView("messages"); $("msg").focus(); }
  const i = ["1", "2", "3", "4", "5", "6"].indexOf(ev.key);
  if (i >= 0) setView(VIEWS[i]);
});
document.querySelector(".tabs").addEventListener("keydown", (ev) => {
  if (ev.key !== "ArrowRight" && ev.key !== "ArrowLeft") return;
  const i = (VIEWS.indexOf(view) + (ev.key === "ArrowRight" ? 1 : VIEWS.length - 1)) % VIEWS.length;
  setView(VIEWS[i]); $("tab-" + VIEWS[i]).focus();
});
window.addEventListener("resize", () => { if (view === "traffic") drawTraffic(); if (map && view === "map") map.invalidateSize(); });

setView(view);
poll();
pollTraffic();
setInterval(poll, 2000);
setInterval(pollTraffic, 2000);
setInterval(() => { if (view === "traffic") drawTraffic(); }, 1000);
