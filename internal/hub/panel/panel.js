// batnet hub panel: polls /hub/api/status and draws it. Plain JS, no build step.
"use strict";

const $ = (id) => document.getElementById(id);
const screen = new URLSearchParams(location.search).has("screen");
if (screen) document.documentElement.classList.add("screen");

let last = null;
let pin = "";
try { pin = sessionStorage.getItem("hubPIN") || ""; } catch (e) { /* private window */ }

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

function clock(t) {
  return new Date(t).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

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

function nodeName(n) { return n.longName || n.shortName || n.id; }

function hash(s) { let h = 2166136261; for (const c of s) { h ^= c.charCodeAt(0); h = Math.imul(h, 16777619); } return h >>> 0; }

// ---- the air ----
const SVGNS = "http://www.w3.org/2000/svg";
function svg(tag, attrs) { const e = document.createElementNS(SVGNS, tag); for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v); return e; }
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
  const others = nodes.filter((n) => !n.self);
  for (const n of others) {
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
    const title = svg("title", {}); title.textContent = `${nodeName(n)}, ${ago(n.lastHeard)}`;
    shape.append(title);
    const lbl = svg("text", { class: "lbl", x: (x + 9).toFixed(1), y: (y + 3.5).toFixed(1) });
    lbl.textContent = n.shortName || n.id.slice(-4);
    p.append(shape, lbl);
  }
}

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
  drawAir(nodes);
  const others = nodes.filter((n) => !n.self);
  $("ncount").textContent = others.length ? `(${others.length})` : "";
  $("nodes-empty").hidden = others.length > 0;
  const tb = $("nodelist");
  tb.replaceChildren(...nodes.map((n) => {
    const name = el("td", { class: "nm" }, nodeName(n));
    if (n.batnet) name.append(el("span", { class: "tag bn", text: "batnet" }));
    if (n.self) name.append(el("span", { class: "tag me", text: "this hub" }));
    name.append(el("span", { class: "sub", text: `${n.shortName || ""} ${n.id}${n.role ? ", " + n.role : ""}` }));
    const sig = n.self ? el("td", { text: "" }) : el("td", {}, bars(n.snr), `${(n.snr || 0).toFixed(1)} dB`,
      n.rssi ? el("span", { class: "sub", text: `${n.rssi} dBm` }) : null);
    const hops = el("td", { text: n.self ? "" : n.hops < 0 ? "?" : String(n.hops) });
    const heard = el("td", { text: n.self ? "" : ago(n.lastHeard) });
    if (!n.self && n.packets) heard.append(el("span", { class: "sub", text: `${n.packets} packet${n.packets > 1 ? "s" : ""}` }));
    return el("tr", {}, name, sig, hops, heard);
  }));

  const names = new Map(nodes.map((n) => [n.num, n.shortName || n.longName || n.id]));
  const texts = ((r && r.texts) || []).filter((t) => t.channel === 0 && t.to === 0xffffffff);
  $("texts-empty").hidden = texts.length > 0;
  const list = $("texts");
  const atBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 30;
  list.replaceChildren(...texts.map((t) => el("li", { class: t.mine ? "mine" : "" },
    el("span", { class: "t", text: clock(t.at) }),
    el("span", { class: "who", text: t.mine ? "this hub" : names.get(t.from) || "!" + t.from.toString(16) }),
    el("span", { class: "body", text: t.text }))));
  if (atBottom) list.scrollTop = list.scrollHeight;
  const ready = r && r.ready && r.region !== "not set";
  for (const b of [$("hello"), document.querySelector(".onair-btn")]) b.disabled = !ready;

  const m = s.mesh || {};
  for (const b of document.querySelectorAll(".modes button")) b.setAttribute("aria-checked", String(b.dataset.mode === m.mode));
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
      ? `Logging since ${clock(lg.started)}: ${lg.packets} packet${lg.packets === 1 ? "" : "s"} from ${lg.radios} radio${lg.radios === 1 ? "" : "s"}, ${lg.hellos} hello${lg.hellos === 1 ? "" : "s"} sent${lg.unreadable ? `, ${lg.unreadable} heard too weak to read` : ""}`
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
  try {
    await act("send", { text });
    $("msg").value = "";
    note("Sent by radio. Radios in range get it within a few seconds.");
    poll();
  } catch (e) { note(e.message, true); }
});

$("hello").addEventListener("click", async () => {
  try {
    await act("hello");
    const p = $("pulse");
    p.classList.remove("go"); void p.getBBox(); p.classList.add("go");
    note("Hello sent. Radios in range answer over the next minute or so.");
  } catch (e) { note(e.message, true); }
});

$("logbtn").addEventListener("click", async () => {
  const on = !(last && last.log && last.log.on);
  try {
    await act("log", { on: String(on) });
    poll();
  } catch (e) { $("log-state").textContent = e.message; }
});

for (const b of document.querySelectorAll(".modes button")) {
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

document.addEventListener("keydown", (ev) => {
  if (ev.key === "/" && document.activeElement !== $("msg")) { ev.preventDefault(); $("msg").focus(); }
  if (ev.key === "Escape" && document.activeElement === $("msg")) $("msg").blur();
});

poll();
setInterval(poll, 2000);
