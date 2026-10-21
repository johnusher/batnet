// batnet hub panel: polls /hub/api/status (and api/traffic) and draws them. Plain JS, no build step.
"use strict";

const $ = (id) => document.getElementById(id);
const screen = new URLSearchParams(location.search).has("screen");
if (screen) document.documentElement.classList.add("screen");
const BCAST = 0xffffffff;

let last = null;
let pin = "";
try { pin = sessionStorage.getItem("hubPIN") || ""; } catch (e) { /* private window */ }

// Per-viewer conveniences only: what was read, the traffic window.
function load(k, d) { try { const v = localStorage.getItem(k); return v == null ? d : JSON.parse(v); } catch (e) { return d; } }
function save(k, v) { try { localStorage.setItem(k, JSON.stringify(v)); } catch (e) { /* fine */ } }

const MODE_HELP = {
  off: "This hub works alone: the house app runs, nothing is shared over WiFi.",
  mock: "Four pretend houses run inside this hub and post now and then, so you can try the app without other Pis. Nothing is saved.",
  real: "batman-adv runs on the USB WiFi stick and the log syncs with other batnet Pis in range.",
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

function clock(t) { return new Date(t).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }); }
function clockSec(t) { return new Date(t).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" }); }

function duration(s) {
  const d = Math.floor(s / 86400), h = Math.floor((s % 86400) / 3600), m = Math.floor((s % 3600) / 60);
  return d ? `${d} d ${h} h` : h ? `${h} h ${m} min` : `${m} min`;
}

// Signal strength in four bars from LoRa SNR (dB): LoRa decodes down to about -20.
function bars(snr) {
  const n = snr >= 5 ? 4 : snr >= -2 ? 3 : snr >= -10 ? 2 : 1;
  const b = el("span", { class: "bars", "aria-hidden": "true" });
  for (let i = 0; i < 4; i++) b.append(el("i", { class: i < n ? "on" : "" }));
  return b;
}

const hex8 = (n) => (n >>> 0).toString(16).padStart(8, "0");
function nodeName(n) { return n.longName || n.shortName || n.id; }
function hash(s) { let h = 2166136261; for (const c of s) { h ^= c.charCodeAt(0); h = Math.imul(h, 16777619); } return h >>> 0; }
const minus = (x) => String(x).replace("-", "−");

// Who a node number or ID is, from the last status.
function byNum(num) { return ((last && last.radio && last.radio.nodes) || []).find((n) => n.num === num); }
function nameOfNum(num) { const n = byNum(num); return n ? nodeName(n) : "!" + hex8(num); }
function nameOfID(id) {
  if (!id || id === "everyone") return "everyone";
  const n = byNum(parseInt(id.slice(1), 16));
  return n && (n.longName || n.shortName) ? (n.longName || n.shortName) : id;
}

// ---- views ----
const VIEWS = ["radios", "messages", "traffic"];
let view = VIEWS.includes(location.hash.slice(1)) ? location.hash.slice(1) : "radios";

function setView(v) {
  view = v;
  for (const x of VIEWS) {
    $("view-" + x).hidden = x !== v;
    $("tab-" + x).setAttribute("aria-selected", String(x === v));
    $("tab-" + x).tabIndex = x === v ? 0 : -1;
  }
  history.replaceState(null, "", location.search + "#" + v);
  if (last) render(last);
  if (v === "traffic") drawTraffic();
}
for (const v of VIEWS) $("tab-" + v).addEventListener("click", () => setView(v));

// ---- the air ----
const SVGNS = "http://www.w3.org/2000/svg";
function svg(tag, attrs) { const e = document.createElementNS(SVGNS, tag); for (const [k, v] of Object.entries(attrs || {})) e.setAttribute(k, v); return e; }
const RINGS = [{ r: 52, label: "direct" }, { r: 96, label: "1 relay" }, { r: 140, label: "2+ relays" }];

function drawAir(nodes) {
  const p = $("plot");
  p.replaceChildren();
  for (const ring of RINGS) {
    p.append(svg("circle", { class: "ring", r: ring.r }));
    const t = svg("text", { class: "ringlabel", x: 4, y: -ring.r + 11 }); t.textContent = ring.label; p.append(t);
  }
  const pulse = svg("circle", { class: "pulse", r: 6, id: "pulse" });
  p.append(pulse, svg("circle", { class: "hubdot", r: 6 }));
  for (const n of nodes.filter((n) => !n.self)) {
    const ring = n.hops < 0 ? 2 : Math.min(n.hops, 2);
    const base = RINGS[ring].r - 22;
    // A stable angle per radio; inside its band, a stronger signal sits further in.
    const a = (hash(n.id) % 360) * Math.PI / 180;
    const inward = Math.max(0, Math.min(1, ((n.snr || -10) + 20) / 30));
    const r = base + 18 * (1 - inward);
    const x = Math.cos(a) * r, y = Math.sin(a) * r;
    const stale = Date.now() - new Date(n.lastHeard).getTime() > 2 * 3600e3;
    const cls = "node" + (n.batnet ? " bn" : ring > 0 ? " relayed" : "") + (stale ? " stale" : "");
    let shape;
    if (n.batnet) {
      const s = 7, pts = [];
      for (let i = 0; i < 6; i++) pts.push(`${(x + s * Math.cos(i * Math.PI / 3)).toFixed(1)},${(y + s * Math.sin(i * Math.PI / 3)).toFixed(1)}`);
      shape = svg("polygon", { class: cls, points: pts.join(" ") });
    } else {
      shape = svg("circle", { class: cls, cx: x.toFixed(1), cy: y.toFixed(1), r: 5 });
    }
    const title = svg("title"); title.textContent = `${nodeName(n)}, ${ago(n.lastHeard)}`;
    shape.append(title);
    const lbl = svg("text", { class: "lbl", x: (x + 9).toFixed(1), y: (y + 3.5).toFixed(1) });
    lbl.textContent = n.shortName || n.id.slice(-4);
    p.append(shape, lbl);
  }
}

// ---- messages ----
let conv = "pub";            // the open conversation: "pub" or a node number as a string
let replyTo = null;          // the message being answered
const started = new Set();   // conversations opened from the radio list, maybe still empty
const seen = new Set();      // messages already noticed (for the new-message flash)
let firstStatus = true;
const pageStart = Date.now();
const lastRead = load("hub.read", {});
let threadSig = "";

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
  if (conv !== id) { replyTo = null; threadSig = ""; }
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

function renderMessages(r, convs) {
  const total = convs.filter((c) => c.id !== "pub").reduce((s, c) => s + c.unread, 0);
  const pubUnread = convs[0].unread;
  const badge = $("badge");
  badge.hidden = !total && !pubUnread;
  badge.textContent = total ? String(total) : "";
  badge.className = "badge" + (total ? "" : " dot");

  const c = convs.find((x) => x.id === conv) || convs[0];
  conv = c.id;
  if (view === "messages" && document.visibilityState === "visible" && c.texts.length) {
    lastRead[c.id] = c.texts[c.texts.length - 1].at;
    c.unread = 0;
    save("hub.read", lastRead);
  }

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
    : `Only this radio gets it.${node ? ` Last heard ${ago(node.lastHeard)}, ${minus((node.snr || 0).toFixed(1))} dB, ${node.hops < 0 ? "?" : node.hops} relays.` : ""} Messages to one radio are encrypted between the two once they know each other's key.`;
  $("msg").placeholder = c.id === "pub" ? "Message to everyone in range" : `Message to ${c.name}`;
  $("thread-empty").hidden = c.texts.length > 0;
  $("thread-empty").textContent = c.id === "pub" ? "No messages heard yet." : "No messages yet. Write the first one below.";

  const sig = conv + "|" + c.texts.map((t) => textKey(t) + t.status).join(",") + "|" + (r.nodes || []).length;
  if (sig === threadSig) return;
  threadSig = sig;
  const list = $("thread");
  const atBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 40;
  const all = r.texts || [];
  list.replaceChildren(...c.texts.map((t) => {
    const li = el("li", { class: t.mine ? "mine" : "in" });
    if (!t.mine && c.id === "pub") li.append(el("span", { class: "who", text: nameOfNum(t.from) }));
    if (t.replyId) {
      const q = all.find((x) => x.id === t.replyId);
      li.append(el("span", { class: "quote", text: q ? `${q.mine ? "You" : nameOfNum(q.from)}: ${q.text}` : "An earlier message" }));
    }
    li.append(el("span", { class: "body", text: t.text }));
    const meta = el("span", { class: "meta" }, clock(t.at));
    if (!t.mine) meta.append(`, ${minus((t.snr || 0).toFixed(1))} dB, ${t.hops < 0 ? "?" : t.hops} relays${t.pki ? ", encrypted to this hub" : ""}`);
    const st = t.mine ? statusLine(t) : null;
    if (st) meta.append(" ", st);
    const rb = el("button", { class: "linkbtn", type: "button", text: "Reply" });
    rb.addEventListener("click", () => { replyTo = t; showReply(); $("msg").focus(); });
    meta.append(" ", rb);
    li.append(meta);
    return li;
  }));
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

// ---- drawing the status ----
function render(s) {
  last = s;
  $("house").textContent = s.system.model ? `${s.house}, on a ${s.system.model.replace(/ Model B Rev [\d.]+$/, "")}` : s.house;
  $("clock").textContent = clock(s.system.time);
  const r = s.radio;
  const onair = $("onair");
  if (!r) { onair.textContent = "No radio configured"; onair.className = "onair down"; }
  else if (!r.connected) { onair.textContent = "Radio offline: meshtasticd is not answering"; onair.className = "onair down"; }
  else if (!r.ready) { onair.textContent = "Radio connecting…"; onair.className = "onair"; }
  else if (r.region === "not set") { onair.textContent = "Radio on, but no region is set: it will not transmit"; onair.className = "onair down"; }
  else {
    const ch = (r.channels || []).find((c) => c.role === "primary");
    onair.textContent = `On air: ${r.region.replace("_", " ")} MHz, ${r.preset}, ${r.txPower} dBm${ch ? `, channel ${ch.name}` : ""}`;
    onair.className = "onair live";
  }

  const nodes = (r && r.nodes) || [];
  if (view === "radios") {
    drawAir(nodes);
    const others = nodes.filter((n) => !n.self);
    $("ncount").textContent = others.length ? `(${others.length})` : "";
    $("nodes-empty").hidden = others.length > 0;
    $("nodelist").replaceChildren(...nodes.map((n) => {
      const name = el("td", { class: "nm" }, nodeName(n));
      if (n.batnet) name.append(el("span", { class: "tag bn", text: "batnet" }));
      if (n.self) name.append(el("span", { class: "tag me", text: "this hub" }));
      name.append(el("span", { class: "sub", text: `${n.shortName || ""} ${n.id}${n.role ? ", " + n.role : ""}` }));
      const sig = n.self ? el("td", { text: "" }) : el("td", {}, bars(n.snr), `${minus((n.snr || 0).toFixed(1))} dB`,
        n.rssi ? el("span", { class: "sub", text: `${minus(n.rssi)} dBm` }) : null);
      const hops = el("td", { text: n.self ? "" : n.hops < 0 ? "?" : String(n.hops) });
      const heard = el("td", { text: n.self ? "" : ago(n.lastHeard) });
      if (!n.self && n.packets) heard.append(el("span", { class: "sub", text: `${n.packets} packet${n.packets > 1 ? "s" : ""}` }));
      const act = el("td", {});
      if (!n.self) {
        const b = el("button", { class: "btn ghost small", type: "button", text: "Message" });
        b.addEventListener("click", () => openConv(String(n.num)));
        act.append(b);
      }
      return el("tr", {}, name, sig, hops, heard, act);
    }));
  }

  if (r) {
    noticeNew(r);
    renderMessages(r, conversations(r));
  }
  const ready = r && r.ready && r.region !== "not set";
  for (const b of [$("hello"), document.querySelector("#send button")]) b.disabled = !ready;

  const m = s.mesh || {};
  for (const b of document.querySelectorAll(".side .modes button")) b.setAttribute("aria-checked", String(b.dataset.mode === m.mode));
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
      ? `Logging since ${clock(lg.started)}: ${lg.packets} packet${lg.packets === 1 ? "" : "s"} from ${lg.radios} radio${lg.radios === 1 ? "" : "s"}, ${lg.hellos} hello${lg.hellos === 1 ? "" : "s"} sent (${lg.helloMinutes ? `one every ${lg.helloMinutes} min` : "listening only"})${lg.unreadable ? `, ${lg.unreadable} heard too weak to read` : ""}`
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
    ["LoRa stick", sy.meshstick ? "plugged in" : "not found"]];
  for (const i of sy.interfaces || []) if (i.addrs && i.addrs.length) facts.push([i.name + (i.wifi ? " (WiFi)" : ""), i.addrs.join(", ")]);
  $("sys").replaceChildren(...facts.flatMap(([k, v]) => [el("dt", { text: k }), el("dd", { text: v })]));

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

const WHAT = { heard: (f) => (f.encrypted ? "encrypted" : f.port || "packet"), garbled: () => "unreadable", sent: (f) => `sent ${f.port === "nodeinfo" ? "hello" : f.port || ""}`, relayed: () => "passed on", busy: () => "channel busy" };
function frameText(f) {
  const parts = [`${clockSec(f.t)}: ${WHAT[f.kind] ? WHAT[f.kind](f) : f.kind}`];
  if (f.from) parts.push(`from ${f.name || nameOfID(f.from)}`);
  if (f.to) parts.push(`to ${nameOfID(f.to)}`);
  if (f.kind === "heard" || f.kind === "garbled") parts.push(`${minus(f.snr.toFixed(2))} dB${f.rssi ? `, ${minus(f.rssi)} dBm` : ""}`);
  if (f.hops >= 0 && f.kind !== "sent") parts.push(`${f.hops} relays`);
  if (f.relay) parts.push(`via …${f.relay}`);
  if (f.airMs) parts.push(`${f.airMs} ms on air`);
  return parts.join(", ");
}

let marks = [];
function drawTraffic() {
  const now = Date.now(), span = win * 1000, t0 = now - span;
  const fr = tr.frames.filter((f) => Date.parse(f.t) >= t0);
  const me = last && last.radio ? "!" + hex8(last.radio.myNum) : "";
  const self = last && last.radio ? (last.radio.nodes || []).find((n) => n.self) : null;
  for (const b of document.querySelectorAll(".win button")) b.setAttribute("aria-checked", String(Number(b.dataset.win) === win));
  $("tr-sub").textContent = `Everything this hub's radio picks up on ${tr.preset || "its preset"}, readable or not, over the last ${win === 3600 ? "hour" : win / 60 + " minutes"}.`;

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
  g.push(svg("text", { class: "ax", x: ml - 6, y: laneY + 4, "text-anchor": "end" }));
  g[g.length - 1].textContent = "this hub";
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
    const w = Math.max(6, (f.airMs || 0) * pxPerMs);
    let m, my;
    if (f.kind === "heard") {
      my = y(f.snr);
      m = svg("rect", { class: "m heard" + (f.encrypted ? " enc" : "") + (f.to === me ? " us" : "") + fresh, x: fx, y: my - 4, width: Math.max(w, 9), height: 8, rx: 4 });
    } else if (f.kind === "garbled") {
      my = y(f.snr);
      m = svg("circle", { class: "m garbled" + (f.to === me ? " us" : "") + fresh, cx: fx, cy: my, r: 4 });
    } else if (f.kind === "sent") {
      my = laneY;
      m = svg("rect", { class: "m sent" + fresh, x: fx, y: my - 4, width: w, height: 8, rx: 3 });
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
  const nowLine = svg("line", { class: "now", x1: W - mr, x2: W - mr, y1: laneY - 8, y2: bot });
  g.push(nowLine);
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
  const nodes = (last && last.radio && last.radio.nodes) || [];
  $("relays").replaceChildren(...(top.length ? top.map(([r, c]) => {
    const cands = nodes.filter((nd) => (nd.num & 0xff) === parseInt(r, 16) && !nd.self).map(nodeName).slice(0, 2);
    return el("li", {},
      el("span", { class: "rl", text: `…${r}` }),
      el("span", { class: "rbar" }, el("i", { style: `width:${(c / maxv * 100).toFixed(0)}%` })),
      el("span", { class: "rc", text: String(c) }),
      el("span", { class: "rn", text: cands.length ? `maybe ${cands.join(" or ")}` : "" }));
  }) : [el("li", { class: "empty", text: "Nothing relayed in this window." })]));
}

function drawFeed(fr) {
  const rows = fr.slice(-80).reverse();
  $("feed").replaceChildren(...rows.map((f) => el("tr", { class: f.kind + (f.encrypted ? " enc" : "") },
    el("td", { text: clockSec(f.t) }),
    el("td", { class: "what", text: WHAT[f.kind] ? WHAT[f.kind](f) : f.kind }),
    el("td", { text: f.from || f.to ? `${f.from ? f.name || nameOfID(f.from) : "this hub"} → ${nameOfID(f.to)}` : "" }),
    el("td", { text: f.kind === "heard" || f.kind === "garbled" ? `${minus(f.snr.toFixed(1))} dB${f.rssi ? `, ${minus(f.rssi)} dBm` : ""}` : "" }),
    el("td", { text: f.hops >= 0 && f.kind !== "sent" ? String(f.hops) : "" }),
    el("td", { text: f.relay ? "…" + f.relay : "" }),
    el("td", { text: f.airMs ? `${f.airMs} ms` : "" }))));
}

// Hover: the nearest mark within reach gets a tooltip.
$("timeline").addEventListener("pointermove", (ev) => {
  const T = $("timeline"), r = T.getBoundingClientRect(), vb = T.viewBox.baseVal;
  const px = (ev.clientX - r.left) * vb.width / r.width, py = (ev.clientY - r.top) * vb.height / r.height;
  let best = null, bd = 16;
  for (const m of marks) { const d = Math.hypot(m.x - px, m.y - py); if (d < bd) { bd = d; best = m; } }
  const tip = $("tip");
  if (!best) { tip.classList.remove("on"); return; }
  tip.textContent = frameText(best.f);
  tip.style.left = `${Math.min(Math.max(best.x * r.width / vb.width, 120), r.width - 120)}px`;
  tip.style.top = `${best.y * r.height / vb.height}px`;
  tip.classList.add("on");
});
$("timeline").addEventListener("pointerleave", () => $("tip").classList.remove("on"));
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
      if (p == null) throw new Error("Not sent: the buttons need the hub PIN.");
      pin = p;
      try { sessionStorage.setItem("hubPIN", pin); } catch (e) { /* fine */ }
      continue;
    }
    if (!res.ok) throw new Error(out.error || "The hub said no.");
    return out;
  }
  throw new Error("Wrong PIN.");
}

function note(msg, bad) { const n = $("send-note"); n.textContent = msg; n.className = "note" + (bad ? " bad" : ""); }

$("send").addEventListener("submit", async (ev) => {
  ev.preventDefault();
  const text = $("msg").value.trim();
  if (!text) return;
  const body = { text };
  if (conv !== "pub") body.to = "!" + hex8(Number(conv));
  if (replyTo && replyTo.id) body.replyId = String(replyTo.id);
  try {
    await act("send", body);
    $("msg").value = "";
    replyTo = null; showReply();
    note(conv === "pub" ? "Sent by radio. Radios in range get it within a few seconds." : "Sent by radio. The tick shows when it arrived.");
    poll();
  } catch (e) { note(e.message, true); }
});

$("hello").addEventListener("click", async () => {
  try {
    await act("hello");
    const p = $("pulse");
    p.classList.remove("go"); void p.getBBox(); p.classList.add("go");
    $("air-note").textContent = "Hello sent. Radios in range answer over the next minute or so.";
    $("air-note").className = "note";
  } catch (e) { $("air-note").textContent = e.message; $("air-note").className = "note bad"; }
});

$("logbtn").addEventListener("click", async () => {
  const on = !(last && last.log && last.log.on);
  try {
    await act("log", { on: String(on) });
    poll();
  } catch (e) { $("log-state").textContent = e.message; }
});

for (const b of document.querySelectorAll(".side .modes button")) {
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

// Keys for the hub's own keyboard: 1, 2, 3 switch views; / starts a message.
document.addEventListener("keydown", (ev) => {
  const typing = ["INPUT", "TEXTAREA"].includes(document.activeElement.tagName);
  if (ev.key === "Escape") { if (replyTo) { replyTo = null; showReply(); } else if (typing) document.activeElement.blur(); return; }
  if (typing || ev.ctrlKey || ev.metaKey || ev.altKey) return;
  if (ev.key === "/") { ev.preventDefault(); setView("messages"); $("msg").focus(); }
  const i = ["1", "2", "3"].indexOf(ev.key);
  if (i >= 0) setView(VIEWS[i]);
});
$("tab-radios").parentElement.addEventListener("keydown", (ev) => {
  if (ev.key !== "ArrowRight" && ev.key !== "ArrowLeft") return;
  const i = (VIEWS.indexOf(view) + (ev.key === "ArrowRight" ? 1 : VIEWS.length - 1)) % VIEWS.length;
  setView(VIEWS[i]); $("tab-" + VIEWS[i]).focus();
});
window.addEventListener("resize", () => { if (view === "traffic") drawTraffic(); });

setView(view);
poll();
pollTraffic();
setInterval(poll, 2000);
setInterval(pollTraffic, 2000);
setInterval(() => { if (view === "traffic") drawTraffic(); }, 1000);
