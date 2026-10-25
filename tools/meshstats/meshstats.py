#!/usr/bin/env python3
"""Weekly statistics of what a hub hears, kept as snapshots to follow trends.

  tools/meshstats/meshstats.py pull pz7      # a snapshot: private/meshstats/pz7/<date>/
  tools/meshstats/meshstats.py report pz7    # the latest snapshot against the ones before

pull reads the hub's panel API (status), its saved files (zone readings,
sightings) and meshtasticd's journal since the last snapshot (at most eight
days), never meshtasticd itself (B38). report writes, beside the snapshot:
summary.md (plain words, the things worth a look, the trends) and
report.html, one page that works on its own: the summary, a map with layers
(rough squares, who hands each radio's packets on, traced links, radios heard
straight; names on hover), the flows and the traffic by hour; report.png is a
picture of it, at twice the screen's resolution. They name real radios and
places: everything stays in private/ (gitignored). The skill
.claude/skills/weekly-mesh-stats says how to read them and what goes into
the knowledge bundle.
"""
import datetime as dt
import functools
import gzip
import http.server
import json
import math
import os
import re
import shutil
import statistics as st
import subprocess
import sys
import threading
from collections import Counter, defaultdict

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
STORE = os.path.join(ROOT, "private", "meshstats")
VENDOR = os.path.join(ROOT, "internal", "hub", "panel", "vendor")
DATA = "/var/lib/batnet"
SECTORS = ["N", "NE", "E", "SE", "S", "SW", "W", "NW"]
WORDS = {"N": "north", "NE": "north-east", "E": "east", "SE": "south-east", "S": "south", "SW": "south-west", "W": "west", "NW": "north-west"}


def ssh(host, cmd, check=True):
    return subprocess.run(["ssh", "-o", "ConnectTimeout=10", host, cmd], check=check, capture_output=True, text=True).stdout


# ---- pull ----

def snapshots(host):
    d = os.path.join(STORE, host)
    return sorted(x for x in os.listdir(d) if re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{4}", x)) if os.path.isdir(d) else []


def pull(host):
    now = dt.datetime.now().astimezone()
    prev = snapshots(host)
    since = now - dt.timedelta(days=8)
    if prev:
        since = max(since, dt.datetime.strptime(prev[-1], "%Y-%m-%dT%H%M").astimezone())
    out = os.path.join(STORE, host, now.strftime("%Y-%m-%dT%H%M"))
    os.makedirs(out, exist_ok=True)
    status = ssh(host, "curl -s -H 'X-Hub: 1' http://localhost/hub/api/status")
    status = json.loads(status)
    status.pop("pins", None)  # the screen's PINs: not needed, not kept
    json.dump(status, open(os.path.join(out, "status.json"), "w"))
    for f in ("links.json", "sightings.json", "hub.json"):
        b = ssh(host, f"sudo cat {DATA}/{f} 2>/dev/null", check=False)
        if b.strip():
            if f == "hub.json":  # only where the hub stands
                b = json.dumps({"where": json.loads(b).get("where")})
            open(os.path.join(out, f), "w").write(b)
    log = ssh(host, f"sudo journalctl -u meshtasticd --no-pager -o short-iso --since '{since.strftime('%Y-%m-%d %H:%M:%S')}'")
    with gzip.open(os.path.join(out, "meshtasticd.log.gz"), "wt") as fh:
        fh.write(log)
    json.dump({"host": host, "at": now.isoformat(), "since": since.isoformat()}, open(os.path.join(out, "meta.json"), "w"))
    print(f"{out}: {len(status.get('radio', {}).get('nodes', []))} radios, {log.count(chr(10))} log lines since {since:%d %b %H:%M}")
    return out


# ---- reading a snapshot ----

def load(path):
    j = lambda f, d=None: json.load(open(os.path.join(path, f))) if os.path.exists(os.path.join(path, f)) else d
    s = {"path": path, "meta": j("meta.json", {}), "status": j("status.json", {}), "links": j("links.json", {}),
         "sightings": j("sightings.json", {}), "where": (j("hub.json", {}) or {}).get("where")}
    s["log"] = parse_log(gzip.open(os.path.join(path, "meshtasticd.log.gz"), "rt").read().splitlines())
    return s


GARBLED = re.compile(r"Ignore received packet due to error=(-?\d+) \(maybe id=0x([0-9a-f]+) fr=0x([0-9a-f]+) to=0x([0-9a-f]+) flags=0x([0-9a-f]+) rxSNR=(-?[\d.]+) rxRSSI=(-?\d+) .*relay=0x([0-9a-f]+)")
RECEIVED = re.compile(r"\[Router\] Received (\S+) from=0x([0-9a-f]+), id=0x([0-9a-f]+)")
HOP = re.compile(r"0x([0-9a-f]+)(?: \((-?[\d.]+)dB\))?")


def parse_log(lines):
    """Each line of meshtasticd's journal (short-iso) that says something about the air."""
    out = {"garbled": [], "received": [], "relayed": [], "routes": []}
    route_next = None
    for line in lines:
        t = line[:25] if len(line) > 25 and line[4] == "-" else None
        if route_next and "-->" in line:  # the route itself, on the next line
            out["routes"].append((route_next, line.split(": ", 1)[-1].strip()))
            route_next = None
            continue
        route_next = None
        if not t:
            continue
        if m := GARBLED.search(line):
            flags = int(m[5], 16)
            start, limit = flags >> 5, flags & 7
            out["garbled"].append({"t": t, "id": m[2], "from": "!%08x" % int(m[3], 16), "hops": start - limit if start else None,
                                   "snr": float(m[6]), "rssi": int(m[7]), "relay": "%02x" % (int(m[8], 16) & 0xFF), "mqtt": bool(flags & 0x10)})
        elif m := RECEIVED.search(line):
            out["received"].append({"t": t, "port": m[1], "from": "!%08x" % int(m[2], 16), "id": m[3]})
        elif "Rebroadcast received message" in line:
            out["relayed"].append(t)
        elif "Route traced:" in line or "Route back traced:" in line:
            route_next = t
    return out


def ts(t):
    return dt.datetime.fromisoformat(t)


def km(a, b):
    r = math.pi / 180
    x = (b["lon"] - a["lon"]) * r * math.cos(a["lat"] * r)
    y = (b["lat"] - a["lat"]) * r
    return 6371 * math.hypot(x, y), (math.degrees(math.atan2(x, y)) + 360) % 360


def sector(b):
    return SECTORS[int(((b + 22.5) % 360) // 45)]


# ---- the figures ----

def figures(s):
    nodes = [n for n in s["status"].get("radio", {}).get("nodes", [])]
    me = next((n for n in nodes if n.get("self")), {})
    w = s["where"] or (me.get("position") and {"lat": me["position"]["lat"], "lon": me["position"]["lon"]})
    meta = s["meta"]
    since, at = ts(meta["since"]), ts(meta["at"])
    log = s["log"]
    first = min((ts(e["t"]) for k in ("received", "garbled") for e in log[k][:1]), default=since)
    since = max(since, first)  # the journal may not reach back to the last snapshot
    # A garbled copy's header may be garbled too: trust it only when a
    # readable copy of the same packet names the same sender.
    read = {(e["id"], e["from"]) for e in log["received"]}
    sure = [e for e in log["garbled"] if (e["id"], e["from"]) in read]
    hours = max((at - since).total_seconds() / 3600, 1)
    others = [n for n in nodes if not n.get("self")]
    heard = [n for n in others if n.get("lastHeard") and ts(n["lastHeard"]) >= since]
    f = {"since": since.isoformat(), "at": meta["at"], "hours": round(hours, 1), "radios_heard": len(heard), "radios_known": len(others)}
    byid = {n["id"]: n for n in nodes}
    name = lambda i: (byid.get(i, {}).get("longName") or byid.get(i, {}).get("shortName") or i)

    # Places and Meshtastic's grid
    placed = [n for n in heard if n.get("position")]
    bits = Counter()
    for n in placed:
        a = n["position"].get("accuracyM", 0)
        bits[round(31 - math.log2(a / 111320 / 1e-7)) if a else "exact"] += 1
    f["placed"] = len(placed)
    f["precision"] = {str(k): v for k, v in sorted(bits.items(), key=lambda kv: str(kv[0]))}
    sect, far = Counter(), []
    for n in placed:
        if not w:
            break
        d, b = km(w, n["position"])
        if n["position"].get("accuracyM", 0) / 1000 > d / 3:
            sect["rough"] += 1
            continue
        sect[sector(b)] += 1
        far.append((d, n))
    f["directions"] = dict(sect)
    far.sort(key=lambda x: -x[0])
    f["farthest"] = [{"name": name(n["id"]), "km": round(d, 1), "dir": sector(km(w, n["position"])[1]), "hops": n.get("hops")} for d, n in far[:5]]

    # Straight: the zone's readings (the antenna's health)
    straight = []
    for i, l in s["links"].items():
        if not l.get("at") or ts(l["at"]) < since:
            continue
        n = byid.get(i, {})
        d = km(w, n["position"]) if w and n.get("position") and n["position"].get("accuracyM", 0) < 1000 else None
        straight.append({"id": i, "name": name(i), "snr": round(st.median(l["snrs"]), 2), "count": l["count"],
                         "km": round(d[0], 1) if d else None, "dir": sector(d[1]) if d else None})
    straight.sort(key=lambda x: -x["snr"])
    f["straight"] = straight

    # Traffic: readable, garbled and passed on, by hour of day and by day
    byhour = defaultdict(Counter)
    byday = defaultdict(Counter)
    for kind in ("received", "garbled"):
        for e in log[kind]:
            t = ts(e["t"])
            byhour[t.hour][kind] += 1
            byday[t.strftime("%a %d %b")][kind] += 1
    for t in log["relayed"]:
        byhour[ts(t).hour]["relayed"] += 1
        byday[ts(t).strftime("%a %d %b")]["relayed"] += 1
    f["per_hour"] = {h: dict(byhour[h]) for h in range(24)}
    f["per_day"] = {d: dict(c) for d, c in byday.items()}
    nr, ng = len(log["received"]), len(log["garbled"])
    f["readable_share"] = round(nr / (nr + ng), 3) if nr + ng else None
    f["packets_per_hour"] = round((nr + ng) / hours, 1)
    f["relayed_per_hour"] = round(len(log["relayed"]) / hours, 1)
    f["ports"] = dict(Counter(e["port"] for e in log["received"]).most_common(8))
    # The noise floor: RSSI - SNR of the weak copies, by hour (rises with interference)
    noise = defaultdict(list)
    for e in log["garbled"]:
        if e["snr"] < 0:
            noise[ts(e["t"]).hour].append(e["rssi"] - e["snr"])
    f["noise_floor"] = round(st.median([v for vs in noise.values() for v in vs]), 1) if noise else None
    f["noise_by_hour"] = {h: round(st.median(v), 1) for h, v in sorted(noise.items())}
    f["mqtt_copies"] = sum(e["mqtt"] for e in sure)

    # Who carries what: last relays of the sightings (or of the garbled copies)
    via = Counter()
    for g in s["sightings"].values():
        for r, c in g["via"].items():
            via[r] += c
    if not via:
        for e in sure:
            via["direct" if e["hops"] == 0 else e["relay"]] += 1
    direct = {x["id"][-2:]: x["name"] for x in straight}
    total = sum(via.values()) or 1
    f["carriers"] = [{"relay": r, "name": "straight, no relay" if r == "direct" else direct.get(r, "…" + r), "share": round(c / total, 3)} for r, c in via.most_common(8)]

    # Traced routes heard: direct links between radios, with signal
    links = defaultdict(list)
    for t, line in log["routes"]:
        hops = [("!%08x" % int(h, 16), float(snr) if snr else None) for h, snr in HOP.findall(line)]
        for (a, _), (b, snr) in zip(hops, hops[1:]):
            if snr is not None:
                links[(a, b)].append(snr)
    f["traced_links"] = len(links)
    mine = me.get("id")
    f["traced_to_us"] = sorted(({"from": name(a), "snr": round(st.median(v), 1)} for (a, b), v in links.items() if b == mine), key=lambda x: -x["snr"])
    s["_links"] = links

    # Oddities: radios close by that reach the hub only the long way
    odd = []
    for n in placed:
        if not w:
            break
        d, b = km(w, n["position"])
        g = s["sightings"].get(n["id"], {})
        hops = g.get("hops", n.get("hops"))
        rough = n["position"].get("accuracyM", 0) / 1000
        if d - rough < 5 and hops is not None and hops >= 4:
            odd.append({"name": name(n["id"]), "km": f"{max(d - rough, 0):.1f}-{d + rough:.1f}", "fewest_relays": hops, "heard": n.get("lastHeard", "")[:16]})
    f["near_but_far"] = odd
    s["_w"], s["_byid"] = w, byid
    return f


# ---- charts ----

def bars_svg(per_hour, title):
    W, H, pad = 720, 220, 30
    top = max((sum(c.values()) for c in per_hour.values()), default=1) or 1
    bw = (W - 2 * pad) / 24
    cols = {"received": "#1b5e8c", "garbled": "#9aa7a1", "relayed": "#b8321e"}
    out = [f'<svg viewBox="0 0 {W} {H + 40}" xmlns="http://www.w3.org/2000/svg" font-family="sans-serif" font-size="11"><text x="{pad}" y="14" font-weight="700">{title}</text>']
    for h in range(24):
        c, y = per_hour.get(h, per_hour.get(str(h), {})), H
        for k in ("received", "garbled", "relayed"):
            v = c.get(k, 0)
            hh = (H - 30) * v / top
            y -= hh
            out.append(f'<rect x="{pad + h * bw + 1:.1f}" y="{y:.1f}" width="{bw - 2:.1f}" height="{hh:.1f}" fill="{cols[k]}"><title>{h}:00 {k} {v}</title></rect>')
        out.append(f'<text x="{pad + h * bw + bw / 2:.1f}" y="{H + 14}" text-anchor="middle">{h}</text>')
    lx = pad
    for k, lbl in (("received", "read"), ("garbled", "too weak to read"), ("relayed", "passed on by the hub")):
        out.append(f'<rect x="{lx}" y="{H + 24}" width="10" height="10" fill="{cols[k]}"/><text x="{lx + 14}" y="{H + 33}">{lbl}</text>')
        lx += 160
    return "".join(out) + "</svg>"


def flows_svg(s):
    """Where packets come from (by direction) and which neighbour brings them."""
    w, byid = s["_w"], s["_byid"]
    straight = {x["id"][-2:]: x["name"] for x in s["fig"]["straight"]}
    flow = Counter()
    for i, g in s["sightings"].items():
        n = byid.get(i)
        if not n or n.get("self"):
            continue
        p = n.get("position")
        if p and w:
            d, b = km(w, p)
            src = "rough place" if p.get("accuracyM", 0) / 1000 > d / 3 else WORDS[sector(b)]
        else:
            src = "no place"
        for r, c in g["via"].items():
            dst = "straight" if r == "direct" else straight.get(r, "others")
            flow[(src, dst[:28])] += c
    if not flow:
        return "<p>No sightings yet.</p>"
    ins, outs = Counter(), Counter()
    for (a, b), c in flow.items():
        ins[a] += c
        outs[b] += c
    left, right = [k for k, _ in ins.most_common()], [k for k, _ in outs.most_common()]
    total = sum(flow.values())
    W, H, gap, nw = 900, 460, 8, 14
    scale = (H - gap * max(len(left), len(right))) / total

    def stack(keys, side):
        y, pos = 10, {}
        for k in keys:
            v = sum(c for (a, b), c in flow.items() if (a if side == 0 else b) == k)
            pos[k] = [y, y, v * scale]
            y += v * scale + gap
        return pos

    L, R = stack(left, 0), stack(right, 1)
    x0, x1 = 170, W - 260
    pal = ["#1b5e8c", "#0e7468", "#b8321e", "#8a5a00", "#6b3fa0", "#2f6b3a", "#5e6b66", "#a0507a", "#44524d"]
    col = {k: pal[i % len(pal)] for i, k in enumerate(right)}
    out = [f'<svg viewBox="0 0 {W} {H + 20}" xmlns="http://www.w3.org/2000/svg" font-family="sans-serif" font-size="12">']
    for (a, b), c in sorted(flow.items(), key=lambda kv: (left.index(kv[0][0]), right.index(kv[0][1]))):
        h = c * scale
        ya, yb = L[a][1], R[b][1]
        L[a][1] += h
        R[b][1] += h
        mx = (x0 + x1) / 2
        out.append(f'<path d="M{x0},{ya} C{mx},{ya} {mx},{yb} {x1},{yb} L{x1},{yb + h} C{mx},{yb + h} {mx},{ya + h} {x0},{ya + h} Z" fill="{col[b]}" opacity="0.45"><title>{a} → {b}: {c}</title></path>')
    for k, (y, _, h) in L.items():
        out.append(f'<rect x="{x0 - nw}" y="{y}" width="{nw}" height="{max(h, 1)}" fill="#44524d"/><text x="{x0 - nw - 6}" y="{y + h / 2 + 4}" text-anchor="end">{k} ({100 * sum(c for (a, _), c in flow.items() if a == k) / total:.0f}%)</text>')
    for k, (y, _, h) in R.items():
        out.append(f'<rect x="{x1}" y="{y}" width="{nw}" height="{max(h, 1)}" fill="{col[k]}"/><text x="{x1 + nw + 6}" y="{y + h / 2 + 4}">{k} ({100 * sum(c for (_, b), c in flow.items() if b == k) / total:.0f}%)</text>')
    out.append(f'<text x="{x0 - nw}" y="{H + 16}" font-weight="700">where it comes from</text><text x="{x1 + nw}" y="{H + 16}" font-weight="700" text-anchor="end">the neighbour that hands it on</text>')
    return "".join(out) + "</svg>"


PAGE = """<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Hub statistics</title><style>LEAFLET_CSS</style><style>
:root { --ink:#1d2826; --muted:#5e6b66; --rule:#c5cdc2; --paper:#f6f8f4; }
body { margin:0; font:15px/1.45 system-ui,sans-serif; color:var(--ink); background:var(--paper); }
main { max-width:1500px; margin:0 auto; padding:16px; }
.top { display:grid; grid-template-columns:minmax(0,1fr) minmax(0,1.6fr); gap:20px; align-items:start; }
@media (max-width:900px) { .top { grid-template-columns:1fr; } }
#m { height:78vh; min-height:420px; border:1px solid var(--rule); border-radius:6px; }
h1 { font-size:1.3rem; margin:0 0 .5rem } h2 { font-size:1.05rem; margin:1.2rem 0 .4rem }
ul { padding-left:1.1rem; margin:.3rem 0 } li { margin:.25rem 0 }
table { border-collapse:collapse; font-size:.9rem } td,th { border-bottom:1px solid var(--rule); padding:2px 8px; text-align:left }
figure { margin:20px 0; background:#fff; border:1px solid var(--rule); border-radius:6px; padding:10px; }
figure svg { width:100%; max-width:1000px; height:auto; display:block; margin:0 auto } figcaption { color:var(--muted); font-size:.9rem; margin-top:6px }
.lbl { font:600 11px system-ui,sans-serif; background:rgba(255,255,255,.85); border:0; box-shadow:none; padding:0 3px; }
.leaflet-tooltip.lbl::before { display:none }
.key { font-size:.85rem; color:var(--muted) } .key span { display:inline-block; width:22px; height:3px; vertical-align:middle; margin:0 4px 0 10px }
</style><main><div class="top"><section>SUMMARY</section><section><div id="m"></div>
<p class="key">Click a radio for its details; hover a line. Layers top right.<br>
Traced links (dashed, heard by the second radio):<span style="background:#2f6b3a"></span>above −5 dB<span style="background:#8a5a00"></span>−5 to −10<span style="background:#b8321e"></span>below −10.
Faint lines: a radio's packets to the neighbour that handed them to the hub, coloured by that neighbour, thicker for more.</p></section></div>
<figure>FLOWS<figcaption>Where the packets come from (by direction from the hub, for radios with a clear enough place) and which neighbour hands them to the hub. Hover a band for its count.</figcaption></figure>
<figure>TRAFFIC<figcaption>Every copy heard, by hour of the day: read, too weak to read, and passed on by the hub.</figcaption></figure>
</main><script>LEAFLET_JS</script><script>
const D = DATA;
const m = L.map("m").fitBounds(D.bounds, { padding: [20, 20] });
L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", { maxZoom: 18, attribution: "© OpenStreetMap" }).addTo(m);
const snrCol = (v) => (v > -5 ? "#2f6b3a" : v > -10 ? "#8a5a00" : "#b8321e");
const layers = { squares: L.layerGroup(), funnels: L.layerGroup().addTo(m), links: L.layerGroup().addTo(m),
  straight: L.layerGroup().addTo(m), radios: L.layerGroup().addTo(m), names: L.layerGroup() };
const byRadio = {};
for (const f of D.funnels) {
  const l = L.polyline([f.a, f.b], { color: f.col, weight: 1 + Math.log2(1 + f.n), opacity: 0.3 })
    .bindTooltip(`${f.from} → ${f.via}: ${f.n} ${f.n === 1 ? "copy" : "copies"}`, { sticky: true }).addTo(layers.funnels);
  (byRadio[f.from] = byRadio[f.from] || []).push(l);
}
for (const l of D.links) L.polyline([l.a, l.b], { color: snrCol(l.snr), weight: 3, opacity: 0.9, dashArray: "6 5" })
  .bindTooltip(`${l.from} → ${l.to}: heard at ${l.snr.toFixed(1)} dB (${l.n} traced)`, { sticky: true }).addTo(layers.links);
for (const s of D.straight) L.polyline([D.hub, s.at], { color: "#0e7468", weight: 4, opacity: 0.7 })
  .bindTooltip(`${s.name}: heard straight, ${s.snr} dB median, ${s.count} packets`, { sticky: true }).addTo(layers.straight);
for (const r of D.radios) {
  if (r.h > 0.0004) L.rectangle([[r.lat - r.h, r.lon - r.h], [r.lat + r.h, r.lon + r.h]], { weight: 0.6, color: "#5e6b66", fillOpacity: 0.04, interactive: false }).addTo(layers.squares);
  const c = L.circleMarker([r.lat, r.lon], { radius: r.relay ? 7 : 4.5, color: "#fff", weight: 1.5, fillColor: r.relay ? r.col : "#44524d", fillOpacity: 1 })
    .bindTooltip(r.name, { direction: "top", className: "lbl" }).bindPopup(r.info).addTo(layers.radios);
  c.on("mouseover", () => (byRadio[r.name] || []).forEach((l) => l.setStyle({ opacity: 0.95, weight: 4 })));
  c.on("mouseout", () => (byRadio[r.name] || []).forEach((l) => l.setStyle({ opacity: 0.3, weight: 2 })));
  L.marker([r.lat, r.lon], { icon: L.divIcon({ className: "", html: "", iconSize: [0, 0] }), interactive: false })
    .bindTooltip(r.name, { permanent: true, direction: "right", offset: [5, 0], className: "lbl" }).addTo(layers.names);
}
L.circleMarker(D.hub, { radius: 9, color: "#fff", weight: 2, fillColor: "#0e7468", fillOpacity: 1 }).addTo(m)
  .bindTooltip("this hub", { permanent: true, direction: "right", offset: [8, 0], className: "lbl" });
L.control.layers(null, { "Rough squares (where a radio could be)": layers.squares, "Who hands each radio's packets on": layers.funnels,
  "Traced links": layers.links, "Heard straight": layers.straight, "Radios": layers.radios, "Show all names": layers.names }, { collapsed: false }).addTo(m);
L.control.scale({ imperial: false }).addTo(m);
</script></html>"""


def md_html(text):
    """The summary's few Markdown forms as HTML."""
    esc = lambda t: re.sub(r"\*\*(.+?)\*\*", r"<b>\1</b>", t.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;"))
    out, inlist, table = [], False, []
    for line in text.splitlines() + [""]:
        if line.startswith("|"):
            if not set(line) <= set("|-"):
                table.append("<tr>" + "".join(f"<td>{esc(c.strip())}</td>" for c in line.strip("|").split("|")) + "</tr>")
            continue
        if table:
            out.append("<table>" + "".join(table) + "</table>")
            table = []
        if line.startswith("- "):
            if not inlist:
                out.append("<ul>")
                inlist = True
            out.append(f"<li>{esc(line[2:])}</li>")
            continue
        if inlist:
            out.append("</ul>")
            inlist = False
        if line.startswith("# "):
            out.append(f"<h1>{esc(line[2:])}</h1>")
        elif line.startswith("## "):
            out.append(f"<h2>{esc(line[3:])}</h2>")
        elif line.strip():
            out.append(f"<p>{esc(line)}</p>")
    return "\n".join(out)


def report_page(s, path, text):
    """One self-contained page: the summary, the map (vector, with layers), the flows and the traffic."""
    w, byid, f = s["_w"], s["_byid"], s["fig"]
    straight = {x["id"][-2:]: x for x in f["straight"]}
    pal = ["#1b5e8c", "#b8321e", "#8a5a00", "#6b3fa0", "#2f6b3a", "#a0507a", "#0e7468"]
    rcol = {r: pal[i % len(pal)] for i, r in enumerate(sorted(straight))}
    at = lambda n: [n["position"]["lat"], n["position"]["lon"]]
    good = lambda n: n.get("position") and n["position"].get("accuracyM", 0) <= 3000  # a rougher place would draw a line to nowhere
    nm = lambda n: n.get("longName") or n.get("shortName") or n["id"]
    hub = [w["lat"], w["lon"]]
    funnels = []
    for i, g in s["sightings"].items():
        n = byid.get(i)
        if not n or not good(n) or n.get("self"):
            continue
        for r, c in g["via"].items():
            relay = byid.get(straight[r]["id"]) if r in straight else None
            if r == "direct":
                funnels.append({"a": at(n), "b": hub, "n": c, "col": "#0e7468", "from": nm(n), "via": "straight to the hub"})
            elif relay and good(relay):
                funnels.append({"a": at(n), "b": at(relay), "n": c, "col": rcol[r], "from": nm(n), "via": nm(relay)})
    mine = next((n["id"] for n in byid.values() if n.get("self")), None)
    links = []
    for (a, b), v in s["_links"].items():
        pa = hub if a == mine else (at(byid[a]) if a in byid and good(byid[a]) else None)
        pb = hub if b == mine else (at(byid[b]) if b in byid and good(byid[b]) else None)
        if pa and pb:
            links.append({"a": pa, "b": pb, "snr": st.median(v), "n": len(v), "from": nm(byid.get(a, {"id": a})), "to": "this hub" if b == mine else nm(byid.get(b, {"id": b}))})
    straight_lines = [{"at": at(byid[x["id"]]), "name": x["name"], "snr": x["snr"], "count": x["count"]} for x in f["straight"] if x["id"] in byid and good(byid[x["id"]])]
    radios = []
    for n in byid.values():
        if not good(n) or n.get("self"):
            continue
        p, g = n["position"], s["sightings"].get(n["id"], {})
        d, b = km(w, p)
        a = p.get("accuracyM", 0) / 1000
        via = ", ".join(f"{'straight' if r == 'direct' else straight[r]['name'] if r in straight else '…' + r} {c}" for r, c in sorted(g.get("via", {}).items(), key=lambda kv: -kv[1])[:4])
        info = (f"<b>{nm(n)}</b><br>{(f'{max(d - a, 0):.1f} to {d + a:.1f} km' if a > 0.2 else f'{d:.1f} km')} {WORDS[sector(b)]}"
                f"{f', a rough square ±{a:.1f} km' if a > 0.2 else ''}<br>fewest relays: {g.get('hops', n.get('hops', '?'))}"
                f"{'<br>handed on by: ' + via if via else ''}<br>last heard {n.get('lastHeard', '')[:16].replace('T', ' ')}")
        radios.append({"lat": p["lat"], "lon": p["lon"], "h": p.get("accuracyM", 0) / 111320, "name": nm(n), "info": info,
                       "relay": n["id"][-2:] in straight and n["id"] == straight[n["id"][-2:]]["id"], "col": rcol.get(n["id"][-2:])})
    pts = [[r["lat"], r["lon"]] for r in radios if km(w, r)[0] < 40] + [hub]
    bounds = [[min(p[0] for p in pts), min(p[1] for p in pts)], [max(p[0] for p in pts), max(p[1] for p in pts)]]
    data = {"hub": hub, "funnels": funnels, "links": links, "straight": straight_lines, "radios": radios, "bounds": bounds}
    page = (PAGE.replace("LEAFLET_CSS", open(os.path.join(VENDOR, "leaflet.css")).read())
            .replace("LEAFLET_JS", open(os.path.join(VENDOR, "leaflet.js")).read().replace("</script", "<\\/script"))
            .replace("SUMMARY", md_html(text)).replace("FLOWS", flows_svg(s)).replace("TRAFFIC", bars_svg(f["per_hour"], "Copies heard by hour of the day"))
            .replace("DATA", json.dumps(data).replace("</", "<\\/")))
    open(os.path.join(path, "report.html"), "w").write(page)


def shoot(path, page, png, size):
    chrome = os.environ.get("CHROME", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome")
    if not os.path.exists(chrome):
        chrome = shutil.which("chromium") or shutil.which("google-chrome")
    if not chrome:
        print("no Chrome: skipping", png)
        return
    handler = functools.partial(http.server.SimpleHTTPRequestHandler, directory=path)
    handler.log_message = lambda *a: None
    srv = http.server.ThreadingHTTPServer(("127.0.0.1", 0), handler)
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    subprocess.run([chrome, "--headless=new", "--hide-scrollbars", f"--window-size={size}", "--force-device-scale-factor=2", "--virtual-time-budget=15000",
                    f"--screenshot={os.path.join(path, png)}", f"http://127.0.0.1:{srv.server_address[1]}/{page}"], capture_output=True)
    srv.shutdown()


# ---- the summary ----

def summary(f, before):
    """Plain words: the figures, the trends against the snapshots before, and what looks odd."""
    L = [f"# What the hub heard, {ts(f['since']):%d %b %H:%M} to {ts(f['at']):%d %b %H:%M} ({f['hours']:.0f} hours)", ""]
    b = before[-1] if before else None
    def trend(key, fmt="{:.0f}", better="up"):
        if not b or b.get(key) is None or f.get(key) is None:
            return ""
        d = f[key] - b[key]
        if abs(d) < 1e-9:
            return " (same as last time)"
        good = (d > 0) == (better == "up")
        return f" ({'+' if d > 0 else ''}{fmt.format(d)} since last time{'' if better is None else ', better' if good else ', worse'})"
    L += [f"- **Radios heard:** {f['radios_heard']}{trend('radios_heard')}, {f['placed']} with a place.",
          f"- **Packets heard:** {f['packets_per_hour']} an hour{trend('packets_per_hour', '{:.1f}', None)}; "
          f"**readable:** {100 * (f['readable_share'] or 0):.0f}%{trend('readable_share', '{:+.1%}')}; the hub passed on {f['relayed_per_hour']} an hour.",
          f"- **Noise floor:** {f['noise_floor']} dBm{trend('noise_floor', '{:.1f}', 'down')} (lower is quieter; a rise means interference near the hub).",
          f"- **Heard straight:** {len(f['straight'])} radios{(' (' + str(len(b['straight'])) + ' last time)') if b else ''}: " +
          ", ".join(f"{x['name']} {x['snr']} dB" + (f" ({x['km']} km {x['dir']})" if x['km'] else "") for x in f["straight"][:6]) + ".",
          "- **Directions** (radios with a clear enough place): " + ", ".join(f"{WORDS.get(k, k)} {v}" for k, v in sorted(f["directions"].items(), key=lambda kv: -kv[1])) + ".",
          "- **Who brings the packets:** " + ", ".join(f"{c['name']} {100 * c['share']:.0f}%" for c in f["carriers"][:5]) + ".",
          f"- **Traced routes heard:** {f['traced_links']} links; straight to this hub: " + (", ".join(f"{x['from']} {x['snr']} dB" for x in f["traced_to_us"][:5]) or "none") + ".",
          ""]
    # What to look at
    notes = []
    if b:
        old = {x["id"]: x for x in b["straight"]}
        for x in f["straight"]:
            if x["id"] in old and x["snr"] - old[x["id"]]["snr"] <= -3:
                notes.append(f"{x['name']} reaches the hub {old[x['id']]['snr'] - x['snr']:.1f} dB weaker than last time: the antenna, its cable or that radio.")
        lost = [old[i]["name"] for i in old if i not in {x["id"] for x in f["straight"]}]
        if lost:
            notes.append("No longer heard straight: " + ", ".join(lost) + ".")
    dirs = {x["dir"] for x in f["straight"] if x["dir"]}
    if len(dirs) == 1 and sum(1 for x in f["straight"] if x["dir"]) >= 3:
        notes.append(f"Every radio heard straight with a clear place is to the {WORDS[dirs.pop()]}: the antenna may be blind the other ways.")
    if f["carriers"] and f["carriers"][0]["relay"] != "direct" and f["carriers"][0]["share"] > 0.5:
        notes.append(f"{f['carriers'][0]['name']} brings more than half of everything: if it goes off, the hub goes nearly deaf.")
    if f["readable_share"] is not None and f["readable_share"] < 0.4:
        notes.append(f"Only {100 * f['readable_share']:.0f}% of the packets heard could be read: many arrive at the edge of what the preset decodes.")
    for o in f["near_but_far"][:5]:
        notes.append(f"{o['name']} is {o['km']} km away but came through {o['fewest_relays']} relays at best: indoors, low, or behind something.")
    if f["mqtt_copies"]:
        notes.append(f"{f['mqtt_copies']} copies came through the internet (MQTT) and back onto the air.")
    if f["per_hour"]:
        busy = max(range(24), key=lambda h: sum(f["per_hour"][h].values()))
        quiet = min(range(24), key=lambda h: sum(f["per_hour"][h].values()))
        notes.append(f"Busiest hour {busy}:00, quietest {quiet}:00.")
    L += ["## Worth a look", ""] + [f"- {n}" for n in notes] + [""]
    if before:
        L += ["## Over the weeks", "", "| Snapshot | Radios | Straight | Readable | Noise floor | Packets/h |", "|---|---|---|---|---|---|"]
        for x in before[-7:] + [f]:
            L.append(f"| {ts(x['at']):%d %b} | {x['radios_heard']} | {len(x['straight'])} | {100 * (x['readable_share'] or 0):.0f}% | {x['noise_floor']} | {x['packets_per_hour']} |")
    return "\n".join(L) + "\n"


def report(host, snap=None):
    snaps = snapshots(host)
    if not snaps:
        sys.exit(f"no snapshots of {host}: run pull first")
    snap = snap or snaps[-1]
    path = os.path.join(STORE, host, snap)
    s = load(path)
    s["fig"] = f = figures(s)
    json.dump(f, open(os.path.join(path, "figures.json"), "w"), indent=1)
    before = [json.load(open(os.path.join(STORE, host, x, "figures.json"))) for x in snaps if x < snap and os.path.exists(os.path.join(STORE, host, x, "figures.json"))]
    text = summary(f, before)
    open(os.path.join(path, "summary.md"), "w").write(text)
    report_page(s, path, text)
    for old in ("traffic.html", "flows.html", "map.html", "leaflet.js", "leaflet.css", "traffic.png", "flows.png", "map.png"):
        if os.path.exists(os.path.join(path, old)):
            os.remove(os.path.join(path, old))
    shoot(path, "report.html", "report.png", "1500,1900")  # a picture of the page, for a mail
    print(text)
    print("files:", path)


if __name__ == "__main__":
    if len(sys.argv) < 3 or sys.argv[1] not in ("pull", "report"):
        sys.exit(__doc__)
    if sys.argv[1] == "pull":
        pull(sys.argv[2])
    else:
        report(sys.argv[2], sys.argv[3] if len(sys.argv) > 3 else None)
