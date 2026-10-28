#!/usr/bin/env python3
"""Who talks to whom on the mesh around a hub, and what its public channels
talk about, day by day: one interactive page.

  tools/meshstats/talk.py pz7            # pull, add to the archive, write the page
  tools/meshstats/talk.py pz7 --no-pull  # write the page from the archive only
  tools/meshstats/talk.py pz7 --import private/meshstats/pz7/*/meshtasticd.log.gz
                                         # add saved journals (plain or .gz) to the archive first

Each run reads the hub's panel status (radios, places, its last 200
messages), meshtasticd's journal since the last run (the bare headers of
garbled copies, which name sender and recipient; texts; traced routes) and
the packet trace when one runs (/var/log/batnet-trace/trace.json, every copy
with its header), never meshtasticd itself (B38). It adds them to
private/meshstats/<hub>/talk/archive.json, so the history grows with each
run, and writes talk.html beside it: a map of conversations, a strip of
days, the words people used day by day, and the messages behind them.
Private messages show by their headers only: who to whom and when, never
what. The page names real radios, places and words: it stays in private/.
"""
import datetime as dt
import gzip
import json
import os
import re
import sys
from collections import Counter, defaultdict

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from meshstats import STORE, VENDOR, parse_log, shoot, ssh  # noqa: E402

EVERYONE = 0xFFFFFFFF
TRACE = "/var/log/batnet-trace/trace.json"
TEXT = re.compile(r"\[Router\] Received text msg from=0x([0-9a-f]+), id=0x([0-9a-f]+), msg=(.*)$")
HOP = re.compile(r"0x([0-9a-f]+)(?: \((-?[\d.]+)dB\))?")
STOP = set("""
aber alle allem allen aller alles also auch auf aus bei beim bin bis bist bitte dann das dass dein deine dem den denn der des dich die dies diese
diesem diesen dieser dir doch dort durch ein eine einem einen einer eines euch euer fuer für gibt habe haben hast hat hatte hier hin ich ihr ihre
ihm ihn immer ist jetzt kann kein keine man mal mehr mein meine mich mir mit muss nach nicht noch nur oder ohne schon sehr sein seine sich sie
sind soll sowie über und uns unser unter viel vom von vor war waren was weil wenn wer wie wieder wir wird wo zum zur zwei gerade heute morgen
eben etwas grad gut ganz klar okay jemand jemanden leider ja nein danke hallo moin servus gruss gruß grüße liebe lieben
guten gute abend nacht tag tach mahlzeit test tests testing teste empfang empfangen angekommen kommt hört höre hörst
also and are but can for from had has have her here him his how its just like not now off one our out she that the their them then there these
they this too very was were what when where which who will with would you your yeah yes thanks hello
""".split())


def epoch(t):
    return dt.datetime.fromisoformat(t).timestamp()


def node_id(num):
    return "!%08x" % num


# ---- pull ----

def pull(host, arc):
    now = dt.datetime.now().astimezone()
    since = now - dt.timedelta(days=8)
    if arc.get("journalUntil"):
        since = max(since, dt.datetime.fromisoformat(arc["journalUntil"]) - dt.timedelta(minutes=5))
    status = json.loads(ssh(host, "curl -s -H 'X-Hub: 1' http://localhost/hub/api/status"))
    radio = status.get("radio", {})
    where = ssh(host, "sudo cat /var/lib/batnet/hub.json 2>/dev/null", check=False)
    if where.strip():
        arc["where"] = json.loads(where).get("where") or arc.get("where")
    arc["hub"] = radio.get("myNum", arc.get("hub"))
    arc["channels"] = {str(c["index"]): c.get("name") or ("MediumFast" if c["index"] == 0 else str(c["index"])) for c in radio.get("channels", [])}
    for n in radio.get("nodes", []):
        old = arc["nodes"].get(str(n["num"]), {})
        p = n.get("position") or {}
        arc["nodes"][str(n["num"])] = {
            "name": n.get("longName") or old.get("name") or n["id"], "short": n.get("shortName") or old.get("short", ""),
            "lat": p.get("lat", old.get("lat")), "lon": p.get("lon", old.get("lon")),
            "acc": p.get("accuracyM", old.get("acc", 0)) if p else old.get("acc", 0),
            "last": n.get("lastHeard") or old.get("last"), "hops": n.get("hops", old.get("hops")),
            "role": n.get("role") or old.get("role", ""), "hw": n.get("hardware") or old.get("hw", "")}
    for t in radio.get("texts", []):
        arc["texts"][str(t["id"])] = {"t": epoch(t["at"]), "from": t["from"], "to": t["to"], "ch": t.get("channel"),
                                      "text": t.get("text", ""), "reply": t.get("replyId") or None, "mine": bool(t.get("mine"))}
    lines = ssh(host, f"sudo journalctl -u meshtasticd --no-pager -o short-iso --since '{since:%Y-%m-%d %H:%M:%S}'").splitlines()
    add_journal(arc, lines)
    trace = ssh(host, f"sudo cat {TRACE} 2>/dev/null", check=False).splitlines()
    add_trace(arc, trace)
    arc["journalUntil"] = now.isoformat()
    print(f"{host}: {len(radio.get('nodes', []))} radios, {len(lines)} journal lines, {len(trace)} trace lines; "
          f"archive: {len(arc['packets'])} packets to one radio, {len(arc['texts'])} texts, {len(arc['routes'])} traced links")


def add_journal(arc, lines):
    """meshtasticd's journal (short-iso): headers of garbled copies, texts, traced routes."""
    log = parse_log(lines)
    ports = {(int(e["id"], 16), int(e["from"][1:], 16)): e["port"] for e in log["received"]}
    for e in log["garbled"]:
        to, frm, pid = int(e["to"][1:], 16), int(e["from"][1:], 16), int(e["id"], 16)
        if to == EVERYONE:
            continue
        port = ports.get((pid, frm))
        add_packet(arc, f"{frm:x}:{pid:x}", epoch(e["t"]), frm, to, "private" if port in (None, "text") else "machine")
    for line in lines:
        if m := TEXT.search(line):
            k = str(int(m[2], 16))
            if k not in arc["texts"]:  # older than the hub's last 200: no channel or recipient known
                arc["texts"][k] = {"t": epoch(line[:25]), "from": int(m[1], 16), "to": EVERYONE, "ch": None, "text": m[3], "reply": None, "mine": False}
    for t, route in log["routes"]:
        hops = [(int(h, 16), float(snr) if snr else None) for h, snr in HOP.findall(route)]
        for (a, _), (b, snr) in zip(hops, hops[1:]):
            if snr is not None:
                arc["routes"][f"{a:x}>{b:x}@{int(epoch(t))}"] = {"t": epoch(t), "a": a, "b": b, "snr": snr}


def add_trace(arc, trace):
    """meshtasticd's packet trace: every copy heard, with its header, and the decoded ones."""
    decoded = {}
    for line in trace:
        try:
            j = json.loads(line)
        except ValueError:
            continue
        if "payload" in j or "type" in j:
            decoded[(j.get("from"), j.get("id"))] = j
    for line in trace:
        try:
            j = json.loads(line)
        except ValueError:
            continue
        if "bytes" not in j or j.get("to") in (None, EVERYONE):
            continue
        d = decoded.get((j["from"], j["id"]))
        kind = "private" if not d or d.get("type") in ("text", "") else "machine"
        add_packet(arc, f"{j['from']:x}:{j['id']:x}", j.get("timestamp") or 0, j["from"], j["to"], kind, j.get("channel"))
    for d in decoded.values():
        if d.get("type") == "text" and d.get("to") == EVERYONE:
            text = d["payload"] if isinstance(d.get("payload"), str) else (d.get("payload") or {}).get("text", "")
            arc["texts"].setdefault(str(d["id"]), {"t": d.get("timestamp"), "from": d["from"], "to": EVERYONE, "ch": d.get("channel"),
                                                   "text": text, "reply": None, "mine": False})


def add_packet(arc, key, t, frm, to, kind, ch=None):
    old = arc["packets"].get(key)
    if old and (old["kind"] == "machine" or kind == old["kind"]):
        return
    arc["packets"][key] = {"t": t, "from": frm, "to": to, "kind": kind, **({"ch": ch} if ch is not None else {})}


# ---- the page's data ----

def words(text):
    """The words a message is about: German nouns and names (capitalised, not
    just at the start of a sentence) and long words; not fillers or verbs."""
    text = re.sub(r"https?://\S+|\S+\.(de|net|org|com)\S*|![A-Za-z]+", " ", text)  # links, and bot commands like !witz
    out = set()
    for m in re.finditer(r"[A-Za-zÄÖÜäöüß][A-Za-zÄÖÜäöüß\-]{3,}", text):
        w = m.group(0)
        start = not text[:m.start()].strip() or text[:m.start()].rstrip()[-1] in ".!?:"
        if w.lower() in STOP or w.startswith("-"):
            continue
        if (w[0].isupper() and not start and not w.isupper()) or len(w) >= 8:
            out.add(w.lower())
    return out


def build(host, arc, out):
    hub = arc.get("hub")
    nodes = arc["nodes"]
    local = lambda t: dt.datetime.fromtimestamp(t).astimezone()
    day = lambda t: local(t).strftime("%Y-%m-%d")
    # A garbled header can name radios that do not exist: keep a packet only
    # when both ends are radios the hub knows, or were named in three
    # packets or more (a header garbled the same way three times is unlikely).
    seen = Counter(n for p in arc["packets"].values() for n in (p["from"], p["to"]))
    known = {int(k) for k in nodes} | ({hub} if hub else set()) | {n for n, c in seen.items() if c >= 3}
    packets = [p for p in arc["packets"].values() if p["from"] in known and p["to"] in known and p["from"] != p["to"] and p["t"]]
    texts = sorted((dict(t, id=int(k)) for k, t in arc["texts"].items() if t.get("t")), key=lambda t: t["t"])
    byid = {t["id"]: t for t in texts}
    replies = []
    for t in texts:
        q = byid.get(t["reply"]) if t.get("reply") else None
        if q and q["from"] != t["from"] and t["to"] == EVERYONE:
            replies.append({"t": t["t"], "from": t["from"], "to": q["from"], "text": t["id"]})
    days = sorted({day(p["t"]) for p in packets} | {day(t["t"]) for t in texts})
    # Radios that appear, and the index the page uses for each
    used = Counter()
    for p in packets + replies:
        used[p["from"]] += 1
        used[p["to"]] += 1
    for t in texts:
        used[t["from"]] += 1
    ids = sorted(used, key=lambda n: -used[n])
    ix = {n: i for i, n in enumerate(ids)}
    where = arc.get("where") or {}
    radios = []
    for n in ids:
        r = nodes.get(str(n), {})
        me = n == hub
        lat, lon = (where.get("lat"), where.get("lon")) if me and where else (r.get("lat"), r.get("lon"))
        radios.append({"name": "this hub" if me else r.get("name") or node_id(n), "short": r.get("short", ""), "id": node_id(n),
                       "lat": lat, "lon": lon, "acc": 0 if me else r.get("acc") or 0, "hub": me,
                       "role": r.get("role", ""), "hw": r.get("hw", ""), "hops": r.get("hops"), "last": (r.get("last") or "")[:16].replace("T", " ")})
    kinds = {"private": 0, "machine": 1}
    P = [[int(p["t"]), ix[p["from"]], ix[p["to"]], kinds[p["kind"]]] for p in packets]
    R = [[int(r["t"]), ix[r["from"]], ix[r["to"]], r["text"]] for r in replies]
    ch = arc.get("channels", {})
    T = [{"id": t["id"], "t": int(t["t"]), "from": ix[t["from"]], "ch": ch.get(str(t["ch"]), "") if t["ch"] is not None else "",
          "text": t["text"], "mine": t["mine"], "reply": t.get("reply") if t.get("reply") in byid else None,
          "dm": t["to"] != EVERYONE} for t in texts]
    # Words people used, by day: counted by different senders, so one bot
    # repeating itself does not make a story.
    senders = defaultdict(lambda: defaultdict(set))
    where_used = defaultdict(list)
    for i, t in enumerate(T):
        if t["mine"] or t["dm"] or t["ch"] in ("Warnmeldung",):
            continue
        for w in words(t["text"]):
            senders[w][day(t["t"])].add(t["from"])
            where_used[w].append(i)
    def score(w):
        per = [len(senders[w].get(d, ())) for d in days]
        peak = max(per)
        rest = (sum(per) - peak) / max(len([x for x in per if x]) - 1, 1)
        return (peak, peak - rest, sum(per))
    top = sorted((w for w in senders if max(len(s) for s in senders[w].values()) >= 2), key=score, reverse=True)[:28]
    W = [{"w": w, "days": [len(senders[w].get(d, ())) for d in days], "msgs": where_used[w]} for w in top]
    traced = defaultdict(list)
    for r in arc["routes"].values():
        if r["a"] in ix and r["b"] in ix:
            traced[(ix[r["a"]], ix[r["b"]])].append(r["snr"])
    links = [[a, b, round(sorted(v)[len(v) // 2], 1), len(v)] for (a, b), v in traced.items()]
    data = {"host": host, "made": dt.datetime.now().astimezone().strftime("%a %d %b %H:%M"), "days": days, "radios": radios,
            "packets": P, "replies": R, "texts": T, "words": W, "links": links,
            "hub": [where.get("lat"), where.get("lon")] if where else None}
    page = (PAGE.replace("LEAFLET_CSS", open(os.path.join(VENDOR, "leaflet.css")).read())
            .replace("LEAFLET_JS", open(os.path.join(VENDOR, "leaflet.js")).read().replace("</script", "<\\/script"))
            .replace("DATA_JSON", json.dumps(data, ensure_ascii=False).replace("</", "<\\/")))
    open(os.path.join(out, "talk.html"), "w").write(page)
    print(f"{os.path.join(out, 'talk.html')}: {len(days)} days, {len(P)} packets to one radio, {len(R)} public replies, {len(T)} texts, {len(W)} words")


PAGE = r"""<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Who talks to whom</title>
<link rel="preconnect" href="https://fonts.googleapis.com"><link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Atkinson+Hyperlegible:wght@400;700&display=swap" rel="stylesheet">
<style>LEAFLET_CSS</style><style>
:root { color-scheme: light; --ink:#1d2826; --muted:#5e6b66; --rule:#d3d9d0; --paper:#f6f8f4; --card:#fff;
  --private:#3b4cc0; --public:#c27a00; --machine:#8f9a95; --hub:#0e7468; --hot:#1d2826; }
* { box-sizing:border-box }
body { margin:0; font:15px/1.5 "Atkinson Hyperlegible", system-ui, sans-serif; color:var(--ink); background:var(--paper); }
main { max-width:1500px; margin:0 auto; padding:16px; }
h1 { font-size:1.45rem; line-height:1.2; margin:0 } h2 { font-size:1.05rem; margin:0 0 .5rem }
.lede { color:var(--muted); margin:.3rem 0 1rem; max-width:75ch }
button { font:inherit; color:inherit; cursor:pointer }
:focus-visible { outline:3px solid var(--private); outline-offset:2px }
/* The strip of days: a column per day, its bars the day's traffic */
#days { display:flex; gap:6px; align-items:stretch; overflow-x:auto; padding-bottom:4px; margin-bottom:14px }
#days button { flex:1 0 76px; background:var(--card); border:1px solid var(--rule); border-radius:6px; padding:6px 8px 5px; text-align:left }
#days button[aria-pressed=true] { border-color:var(--ink); box-shadow:inset 0 0 0 1px var(--ink) }
#days .d { font-weight:700; display:block } #days .n { color:var(--muted); font-size:.8rem; display:block; white-space:nowrap }
#days .bars { display:flex; gap:2px; align-items:flex-end; height:34px; margin:4px 0 2px }
#days .bars i { flex:1; display:block; min-height:1px; border-radius:2px 2px 0 0 }
.grid { display:grid; grid-template-columns:minmax(0,1.7fr) minmax(320px,1fr); gap:16px; align-items:start }
@media (max-width:900px) { .grid { grid-template-columns:1fr } }
#m { height:70vh; min-height:420px; border:1px solid var(--rule); border-radius:6px; background:#e9ece6 }
.panel { background:var(--card); border:1px solid var(--rule); border-radius:6px; padding:12px 14px; }
.panel + .panel { margin-top:12px }
#side { max-height:70vh; overflow:auto }
.key { display:flex; flex-wrap:wrap; gap:4px 16px; font-size:.85rem; color:var(--muted); margin:.5rem 0 0 }
.key label { display:inline-flex; align-items:center; gap:6px; cursor:pointer }
.sw { display:inline-block; width:22px; height:4px; border-radius:2px }
#sel { font-size:.95rem } #sel .clear { background:none; border:0; color:var(--private); text-decoration:underline; padding:0 }
ol.convs { list-style:none; margin:0; padding:0 }
ol.convs li button { width:100%; display:grid; grid-template-columns:1fr auto; gap:8px; background:none; border:0; border-bottom:1px solid var(--rule); padding:5px 2px; text-align:left }
ol.convs li button:hover, ol.convs li button[aria-pressed=true] { background:#eef1ec }
.who { overflow:hidden; text-overflow:ellipsis; white-space:nowrap } .cnt { color:var(--muted); font-variant-numeric:tabular-nums; white-space:nowrap }
.noplace { color:var(--muted); font-size:.8rem }
.dot { display:inline-block; width:8px; height:8px; border-radius:50%; margin-right:6px; vertical-align:middle }
/* Words by day: the shade is how many different people used the word that day */
.words { margin-top:16px; display:grid; grid-template-columns:minmax(0,1.7fr) minmax(320px,1fr); gap:16px; align-items:start }
@media (max-width:900px) { .words { grid-template-columns:1fr } }
table.wg { border-collapse:separate; border-spacing:3px; width:100%; font-variant-numeric:tabular-nums }
table.wg th { font-weight:400; color:var(--muted); font-size:.8rem; text-align:center; white-space:nowrap }
table.wg th.w { text-align:right; color:var(--ink); font-size:.95rem; padding-right:6px }
table.wg th.w button { background:none; border:0; padding:0 2px; text-align:right }
table.wg tr[aria-selected=true] th.w button { font-weight:700; text-decoration:underline }
table.wg td { height:26px; border-radius:4px; text-align:center; font-size:.8rem; min-width:34px }
table.wg td.sel { box-shadow:inset 0 0 0 2px var(--ink) }
#msgs { max-height:560px; overflow:auto }
#msgs article { border-bottom:1px solid var(--rule); padding:7px 0 }
#msgs .meta { color:var(--muted); font-size:.82rem } #msgs .meta b { color:var(--ink) }
#msgs .rq { color:var(--muted); font-size:.85rem; border-left:3px solid var(--public); padding-left:6px; margin:2px 0 }
#msgs mark { background:#ffe9b8; color:inherit; padding:0 1px }
.empty { color:var(--muted) }
.lbl { font:700 11px "Atkinson Hyperlegible", system-ui, sans-serif; background:rgba(255,255,255,.88); border:0; box-shadow:none; padding:0 3px }
.leaflet-tooltip.lbl::before { display:none }
footer { color:var(--muted); font-size:.85rem; margin-top:18px; max-width:90ch }
@media (prefers-reduced-motion: reduce) { * { transition:none !important } }
</style>
<main>
<h1>Who talks to whom around the hub</h1>
<p class="lede" id="lede"></p>
<div id="days" role="group" aria-label="Days"></div>
<div class="grid">
  <div>
    <div id="m"></div>
    <div class="key" id="key">
      <label><input type="checkbox" data-k="private" checked><span class="sw" style="background:var(--private)"></span>private messages (who to whom only)</label>
      <label><input type="checkbox" data-k="public" checked><span class="sw" style="background:var(--public)"></span>public replies</label>
      <label><input type="checkbox" data-k="machine"><span class="sw" style="background:var(--machine)"></span>machine chatter (route checks, name exchanges)</label>
      <label><input type="checkbox" data-k="links"><span class="sw" style="background:#2f6b3a;height:2px"></span>who hears whom (traced)</label>
    </div>
  </div>
  <div id="side">
    <div class="panel" id="sel"></div>
    <div class="panel"><h2>Conversations</h2><ol class="convs" id="convs"></ol></div>
  </div>
</div>
<div class="words">
  <div class="panel"><h2>What the public channels talked about</h2>
    <p class="lede" style="margin:0 0 .6rem">Each square: how many different people used the word that day. A dark square on one day only is a story of that day. Pick a word to read the messages.</p>
    <table class="wg" id="wg"></table></div>
  <div class="panel"><h2 id="msgs-h">Messages</h2><div id="msgs"></div></div>
</div>
<footer>Private messages are encrypted: the hub sees only their envelope (sender, recipient, time), from copies it heard, many too weak to read in full. Each private message also sends a receipt back, so traffic both ways does not by itself mean an answer. Radios that share no place are in the list but not on the map. Made DATE_MADE from what this hub heard; it names real radios and stays private.</footer>
</main>
<script>LEAFLET_JS</script>
<script>
const D = DATA_JSON;
document.querySelector("footer").innerHTML = document.querySelector("footer").innerHTML.replace("DATE_MADE", D.made);
const $ = (id) => document.getElementById(id);
const el = (tag, attrs = {}, ...kids) => { const e = document.createElement(tag); for (const [k, v] of Object.entries(attrs)) { if (k === "text") e.textContent = v; else if (k === "html") e.innerHTML = v; else e.setAttribute(k, v); } for (const c of kids) if (c != null) e.append(c); return e; };
const COL = { private: "#3b4cc0", public: "#c27a00", machine: "#8f9a95" };
const dayOf = (t) => { const d = new Date(t * 1000); return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`; };
const dayName = (k, long) => new Date(k + "T12:00").toLocaleDateString([], long ? { weekday: "long", day: "numeric", month: "long" } : { weekday: "short", day: "numeric", month: "short" });
const when = (t) => new Date(t * 1000).toLocaleString([], { weekday: "short", day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });
const R = D.radios;
const name = (i) => R[i].name;
const placed = (i) => R[i].lat != null && R[i].acc <= 6000;
const pkDay = D.packets.map((p) => dayOf(p[0])), rpDay = D.replies.map((r) => dayOf(r[0])), txDay = D.texts.map((t) => dayOf(t.t));

// What is chosen: a day (or all), and at most one of a radio, a conversation, a word
const S = { day: null, radio: null, pair: null, word: null, show: { private: true, public: true, machine: false, links: false } };

// ---- the day strip
function drawDays() {
  const box = $("days"); box.replaceChildren();
  const per = D.days.map((d) => ({ p: pkDay.filter((x, i) => x === d && D.packets[i][3] === 0).length, m: pkDay.filter((x, i) => x === d && D.packets[i][3] === 1).length,
    t: txDay.filter((x, i) => x === d && !D.texts[i].dm).length }));
  const max = Math.max(1, ...per.flatMap((x) => [x.p, x.t * 3, x.m / 2]));
  const btn = (key, label, sub, bars) => { const b = el("button", { "aria-pressed": String(S.day === key) }, el("span", { class: "d", text: label }), bars, el("span", { class: "n", text: sub })); b.onclick = () => { S.day = key; S.pair = null; render(); }; return b; };
  box.append(btn(null, "All days", `${D.days.length} days`, el("span", { class: "bars" })));
  D.days.forEach((d, i) => {
    const x = per[i];
    const bars = el("span", { class: "bars", title: `${x.p} private, ${x.t} public messages, ${x.m} machine` },
      el("i", { style: `height:${100 * x.p / max}%;background:var(--private)` }), el("i", { style: `height:${100 * x.t * 3 / max}%;background:var(--public)` }),
      el("i", { style: `height:${100 * x.m / 2 / max}%;background:var(--machine)` }));
    box.append(btn(d, dayName(d), `${x.p} private, ${x.t} public`, bars));
  });
}

// ---- conversations for the chosen day
function convs() {
  const c = new Map();
  const add = (a, b, kind, t) => { const k = a < b ? `${a}-${b}` : `${b}-${a}`; let v = c.get(k); if (!v) c.set(k, v = { a: Math.min(a, b), b: Math.max(a, b), ab: 0, ba: 0, private: 0, public: 0, machine: 0, first: t, last: t, hours: new Array(24).fill(0) }); v[a < b ? "ab" : "ba"]++; v[kind]++; v.first = Math.min(v.first, t); v.last = Math.max(v.last, t); v.hours[new Date(t * 1000).getHours()]++; };
  D.packets.forEach((p, i) => { if ((!S.day || pkDay[i] === S.day)) { const k = p[3] ? "machine" : "private"; if (S.show[k]) add(p[1], p[2], k, p[0]); } });
  if (S.show.public) D.replies.forEach((r, i) => { if (!S.day || rpDay[i] === S.day) add(r[1], r[2], "public", r[0]); });
  return [...c.values()].map((v) => ({ ...v, n: v.private + v.public + v.machine, kind: v.private ? "private" : v.public ? "public" : "machine" }))
    .sort((x, y) => y.n - x.n);
}

// ---- the map
const map = L.map("m", { zoomSnap: 0.5 });
const esri = (svc, extra) => L.tileLayer(`https://server.arcgisonline.com/ArcGIS/rest/services/${svc}/MapServer/tile/{z}/{y}/{x}`, { maxZoom: 16, attribution: "Tiles © Esri, HERE, Garmin, © OpenStreetMap contributors", ...extra });
L.layerGroup([esri("Canvas/World_Light_Gray_Base"), esri("Canvas/World_Light_Gray_Reference", { pane: "shadowPane" })]).addTo(map);
L.control.scale({ imperial: false }).addTo(map);
const edgeLayer = L.layerGroup().addTo(map), linkLayer = L.layerGroup().addTo(map), dotLayer = L.layerGroup().addTo(map);
const pts = R.map((r, i) => (placed(i) ? [r.lat, r.lon] : null)).filter(Boolean);
if (pts.length) map.fitBounds(L.latLngBounds(pts).pad(0.05)); else map.setView([52.52, 13.4], 11);
function arc(a, b) { // a gentle curve, so two lines between neighbours do not hide each other
  const [x1, y1] = [a[1], a[0]], [x2, y2] = [b[1], b[0]], k = 0.18, mx = (x1 + x2) / 2 - (y2 - y1) * k, my = (y1 + y2) / 2 + (x2 - x1) * k;
  return Array.from({ length: 17 }, (_, i) => { const t = i / 16, u = 1 - t; return [u * u * y1 + 2 * u * t * my + t * t * y2, u * u * x1 + 2 * u * t * mx + t * t * x2]; });
}
function drawMap(list) {
  edgeLayer.clearLayers(); dotLayer.clearLayers(); linkLayer.clearLayers();
  const focus = S.radio ?? null, pair = S.pair;
  const deg = new Map();
  for (const v of list) { deg.set(v.a, (deg.get(v.a) || 0) + v.n); deg.set(v.b, (deg.get(v.b) || 0) + v.n); }
  if (S.show.links) for (const [a, b, snr, n] of D.links) if (placed(a) && placed(b))
    L.polyline([[R[a].lat, R[a].lon], [R[b].lat, R[b].lon]], { color: "#2f6b3a", weight: 1.5, opacity: 0.5, dashArray: "3 4" })
      .bindTooltip(`${name(a)} heard ${name(b)}: ${snr} dB, traced ${n}×`, { sticky: true }).addTo(linkLayer);
  for (const v of [...list].reverse()) {
    if (!placed(v.a) || !placed(v.b)) continue;
    const on = pair ? pair === `${v.a}-${v.b}` : focus == null || v.a === focus || v.b === focus;
    const line = L.polyline(arc([R[v.a].lat, R[v.a].lon], [R[v.b].lat, R[v.b].lon]), { color: COL[v.kind], weight: 1.5 + 1.7 * Math.log2(1 + v.n), opacity: on ? 0.8 : 0.08, lineCap: "round" })
      .bindTooltip(`<b>${esc(name(v.a))}</b> ⇄ <b>${esc(name(v.b))}</b><br>${v.ab} one way, ${v.ba} the other${v.public ? `, ${v.public} public ${v.public === 1 ? "reply" : "replies"}` : ""}<br>${when(v.first)} to ${when(v.last)}`, { sticky: true });
    line.on("click", () => { S.pair = `${v.a}-${v.b}`; S.radio = null; S.word = null; render(); });
    line.addTo(edgeLayer);
  }
  R.forEach((r, i) => {
    if (!placed(i)) return;
    const d = deg.get(i) || 0, posted = D.texts.some((t, j) => t.from === i && (!S.day || txDay[j] === S.day));
    if (!d && !posted && !r.hub) return;
    const rough = r.acc > 1500;
    const sel = focus === i || (pair && pair.split("-").map(Number).includes(i));
    const c = L.circleMarker([r.lat, r.lon], { radius: r.hub ? 8 : 3.5 + Math.sqrt(d) * 0.9, color: sel ? "#1d2826" : "#fff", weight: sel ? 2.5 : 1.2,
      fillColor: r.hub ? "#0e7468" : rough ? "#fff" : "#44524d", fillOpacity: 1, dashArray: rough ? "2 2" : null })
      .bindTooltip(esc(r.name), { direction: "top", className: "lbl", permanent: !!(r.hub || sel) });
    if (rough && !r.hub) c.setStyle({ color: "#44524d", weight: 1.5 });
    c.on("click", () => { S.radio = S.radio === i ? null : i; S.pair = null; S.word = null; render(); });
    c.addTo(dotLayer);
  });
}

// ---- side panel: what is chosen, and the conversations
const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));
function hoursBar(h) {
  const max = Math.max(1, ...h);
  return `<svg viewBox="0 0 240 40" width="100%" height="40" role="img" aria-label="By hour of the day">${h.map((v, i) => `<rect x="${i * 10 + 1}" y="${32 - 30 * v / max}" width="8" height="${Math.max(30 * v / max, v ? 1 : 0)}" fill="var(--private)"><title>${i}:00 ${v}</title></rect>`).join("")}<text x="0" y="40" font-size="8" fill="#5e6b66">0:00</text><text x="120" y="40" font-size="8" fill="#5e6b66">12:00</text><text x="222" y="40" font-size="8" fill="#5e6b66">23:00</text></svg>`;
}
function drawSide(list) {
  const sel = $("sel"), scope = S.day ? dayName(S.day, true) : "all days";
  const clear = () => { const b = el("button", { class: "clear", text: "Show everyone" }); b.onclick = () => { S.radio = S.pair = S.word = null; render(); }; return b; };
  if (S.pair) {
    const v = list.find((x) => `${x.a}-${x.b}` === S.pair) || convs().find((x) => `${x.a}-${x.b}` === S.pair);
    const [a, b] = S.pair.split("-").map(Number);
    sel.innerHTML = `<h2>${esc(name(a))} and ${esc(name(b))}</h2>` + (v ? `<p>${v.ab} packets from ${esc(name(a))}, ${v.ba} from ${esc(name(b))}${v.public ? `; ${v.public} public replies` : ""}, ${scope}; first ${when(v.first)}, last ${when(v.last)}.</p>${v.private + v.machine ? `<p class="noplace">When, by hour of the day:</p>${hoursBar(v.hours)}` : ""}` : `<p class="empty">Nothing between them on ${scope}.</p>`);
    sel.append(clear());
  } else if (S.radio != null) {
    const r = R[S.radio], mine = list.filter((v) => v.a === S.radio || v.b === S.radio);
    sel.innerHTML = `<h2>${esc(r.name)}</h2><p>${[r.short && `short name ${esc(r.short)}`, r.hw && esc(r.hw), r.role && esc(r.role), r.hops != null && `${r.hops} relays from the hub`, r.last && `last heard ${esc(r.last)}`].filter(Boolean).join(", ")}${placed(S.radio) ? (r.acc > 1500 ? `; its place is rough (±${(r.acc / 1000).toFixed(1)} km)` : "") : "; it shares no place"}.</p><p>${mine.length ? `In touch with ${mine.length} ${mine.length === 1 ? "radio" : "radios"} on ${scope}.` : `Wrote to no one on ${scope}.`}</p>`;
    sel.append(clear());
  } else if (S.word) {
    sel.innerHTML = `<h2>“${esc(S.word)}”</h2><p>The messages that use it are on the right, below; the people who wrote them are marked on the map.</p>`;
    sel.append(clear());
  } else {
    const priv = list.filter((v) => v.private).length, pub = list.filter((v) => v.public).length;
    const radios = new Set(list.flatMap((v) => [v.a, v.b]));
    sel.innerHTML = `<h2>${S.day ? dayName(S.day, true) : "All days"}</h2><p>${radios.size} radios in ${list.length} conversations: ${priv} with private messages, ${pub} with public replies. Pick a line, a dot or a conversation.</p>`;
  }
  const ol = $("convs"); ol.replaceChildren();
  const shown = (S.radio != null ? list.filter((v) => v.a === S.radio || v.b === S.radio) : list).slice(0, 60);
  if (!shown.length) ol.append(el("li", { class: "empty", text: "No conversations here." }));
  for (const v of shown) {
    const key = `${v.a}-${v.b}`, np = !placed(v.a) || !placed(v.b);
    const b = el("button", { "aria-pressed": String(S.pair === key) },
      el("span", { class: "who", html: `<span class="dot" style="background:${COL[v.kind]}"></span>${esc(name(v.a))} ⇄ ${esc(name(v.b))}${np ? ' <span class="noplace">(not on the map)</span>' : ""}` }),
      el("span", { class: "cnt", text: `${v.ab} / ${v.ba}` }));
    b.onclick = () => { S.pair = S.pair === key ? null : key; S.radio = null; S.word = null; render(); };
    ol.append(el("li", {}, b));
  }
}

// ---- words by day, and the messages
function shade(n, max) { if (!n) return "#f0f2ee"; const t = Math.min(1, n / Math.max(max, 3)); return `rgb(${Math.round(255 - t * 196)},${Math.round(236 - t * 160)},${Math.round(200 - t * 190)})`; }
function drawWords() {
  const t = $("wg"); t.replaceChildren();
  if (!D.words.length) { t.append(el("tr", {}, el("td", { class: "empty", text: "No word yet used by two people on one day." }))); return; }
  const max = Math.max(...D.words.flatMap((w) => w.days));
  t.append(el("tr", {}, el("th", {}), ...D.days.map((d) => el("th", { text: dayName(d) }))));
  for (const w of D.words) {
    const b = el("button", { text: w.w }); b.onclick = () => { S.word = S.word === w.w ? null : w.w; S.radio = S.pair = null; render(); };
    t.append(el("tr", { "aria-selected": String(S.word === w.w) }, el("th", { class: "w" }, b),
      ...w.days.map((n, i) => { const td = el("td", { text: n || "", title: `${w.w}: ${n} ${n === 1 ? "person" : "people"} on ${dayName(D.days[i], true)}`, style: `background:${shade(n, max)};color:${n / Math.max(max, 3) > 0.55 ? "#fff" : "#1d2826"}` }); if (S.day === D.days[i]) td.className = "sel"; return td; })));
  }
}
function drawMsgs() {
  const box = $("msgs"); box.replaceChildren();
  let list = D.texts.map((t, i) => ({ ...t, i })).filter((t) => !t.dm);
  let title = "Public messages";
  if (S.word) { const set = new Set(D.words.find((w) => w.w === S.word)?.msgs || []); list = list.filter((t) => set.has(t.i)); title = `Messages with “${S.word}”`; }
  else if (S.radio != null) { list = list.filter((t) => t.from === S.radio); title = `Public messages from ${name(S.radio)}`; }
  else if (S.pair) { const [a, b] = S.pair.split("-").map(Number); const ids = new Set(D.replies.filter((r) => (r[1] === a && r[2] === b) || (r[1] === b && r[2] === a)).map((r) => r[3])); list = list.filter((t) => ids.has(t.id) || ((t.from === a || t.from === b) && D.texts.some((x) => ids.has(x.id) && x.reply === t.id))); title = "Their public replies"; }
  if (S.day) { list = list.filter((t) => txDay[t.i] === S.day); title += `, ${dayName(S.day, true)}`; }
  $("msgs-h").textContent = `${title} (${list.length})`;
  if (!list.length) { box.append(el("p", { class: "empty", text: S.pair ? "They wrote no public replies to each other; their private messages cannot be read." : "No messages here." })); return; }
  const byId = new Map(D.texts.map((t) => [t.id, t]));
  const re = S.word ? new RegExp(`(${S.word.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")})`, "gi") : null;
  for (const t of list.slice().reverse().slice(0, 300)) {
    const q = t.reply && byId.get(t.reply);
    const a = el("article", {}, el("div", { class: "meta", html: `<b>${esc(name(t.from))}</b> ${when(t.t)}${t.ch ? ` on ${esc(t.ch)}` : ""}` }),
      q ? el("div", { class: "rq", text: `${name(q.from)}: ${q.text}` }) : null,
      el("div", { html: re ? esc(t.text).replace(re, "<mark>$1</mark>") : esc(t.text) }));
    box.append(a);
  }
}
function markWriters() { // a word chosen: ring the people who used it
  if (!S.word) return;
  const set = new Set(D.words.find((w) => w.w === S.word)?.msgs || []);
  const who = new Set(D.texts.filter((t, i) => set.has(i) && (!S.day || txDay[i] === S.day)).map((t) => t.from));
  for (const i of who) if (placed(i)) L.circleMarker([R[i].lat, R[i].lon], { radius: 10, color: "#c27a00", weight: 3, fill: false }).bindTooltip(esc(name(i)), { permanent: true, direction: "top", className: "lbl" }).addTo(dotLayer);
}
function lede() {
  const nP = D.packets.filter((p) => p[3] === 0).length;
  $("lede").textContent = `From ${dayName(D.days[0], true)} to ${dayName(D.days[D.days.length - 1], true)}: ${nP} private messages and receipts between ${new Set(D.packets.filter((p) => p[3] === 0).flatMap((p) => [p[1], p[2]])).size} radios, seen by their envelopes; ${D.replies.length} public replies; ${D.texts.filter((t) => !t.dm).length} public messages. Line thickness is how many.`;
}
function render() { const list = convs(); drawDays(); drawMap(list); markWriters(); drawSide(list); drawWords(); drawMsgs(); }
document.querySelectorAll("#key input").forEach((c) => { c.onchange = () => { S.show[c.dataset.k] = c.checked; render(); }; });
lede(); render();
</script></html>"""


def main():
    if len(sys.argv) < 2:
        sys.exit(__doc__)
    host = sys.argv[1]
    out = os.path.join(STORE, host, "talk")
    os.makedirs(out, exist_ok=True)
    path = os.path.join(out, "archive.json")
    arc = json.load(open(path)) if os.path.exists(path) else {}
    for k in ("nodes", "packets", "texts", "routes"):
        arc.setdefault(k, {})
    if "--import" in sys.argv:
        for f in sys.argv[sys.argv.index("--import") + 1:]:
            add_journal(arc, (gzip.open(f, "rt") if f.endswith(".gz") else open(f)).read().splitlines())
            print("imported", f)
    if "--no-pull" not in sys.argv:
        pull(host, arc)
    json.dump(arc, open(path, "w"))
    build(host, arc, out)
    shoot(out, "talk.html", "talk.png", "1500,1700")


if __name__ == "__main__":
    main()
