#!/usr/bin/env python3
"""How far can a hub reach? Sends one friendly private message to a distant
radio at a time, and keeps what became of each.

  reachtest.py run      one round: note what became of the earlier messages,
                        then send the next one (run it every 10 minutes)
  reachtest.py report   what became of every message so far

It runs on the hub itself and talks only to the hub's panel API on localhost
(no PIN needed there), never to meshtasticd (B38). Each radio gets one message,
farthest first: radios with a known place at least --min-km away, then radios
with no known place that came through at least two relays; none that came
through more relays than the hub's own messages may take (its hop limit). A radio with no key
yet (error 39) is asked for its name card by the hub, which sends the message
again once the key comes. The state file names real radios: keep it out of
the repo.
"""
import argparse, json, math, os, urllib.request
from datetime import datetime, timedelta, timezone

HUB = "http://localhost/hub/api/"
STATE = "/var/lib/batnet/reachtest.json"

WITH_KM = ("Hallo! 👋 Ich teste gerade, wie weit mein Meshtastic-Funk reicht. Du bist etwa {km} km entfernt – "
           "wenn das ankommt, freue ich mich sehr über eine kurze Antwort. Liebe Grüße!")
NO_KM = ("Hallo! 👋 Ich teste gerade, wie weit mein Meshtastic-Funk reicht. Wenn das bei dir ankommt, "
         "freue ich mich sehr über eine kurze Antwort. Liebe Grüße!")


def api(path, body=None):
    req = urllib.request.Request(HUB + path, headers={"X-Hub": "1", "Content-Type": "application/json"},
                                 data=None if body is None else json.dumps(body).encode())
    try:
        with urllib.request.urlopen(req, timeout=15) as r:
            return json.load(r)
    except urllib.error.HTTPError as e:
        raise RuntimeError(json.load(e).get("error", str(e)))


def when(s):
    return datetime.fromisoformat(s.replace("Z", "+00:00")) if s else None


def km(a, b):
    r = math.pi / 180
    x = (b["lon"] - a["lon"]) * r * math.cos(a["lat"] * r)
    y = (b["lat"] - a["lat"]) * r
    return 6371 * math.hypot(x, y)


def load(path):
    try:
        with open(path) as f:
            return json.load(f)
    except FileNotFoundError:
        return {"tried": {}}


def save(path, st):
    with open(path + ".new", "w") as f:
        json.dump(st, f, indent=1, ensure_ascii=False)
    os.replace(path + ".new", path)


def update(st, status):
    """Notes what became of each message: the hub's receipt, the radio's key, its answers."""
    radio = status["radio"]
    me = radio["myNum"]
    nodes = {n["id"]: n for n in radio["nodes"]}
    texts = radio.get("texts") or []
    for rid, t in st["tried"].items():
        num, sent = int(rid[1:], 16), when(t["sent"])
        for x in texts:
            if x.get("mine") and x["to"] == num and x["text"] == t["text"] and abs((when(x["at"]) - sent).total_seconds()) < 120:
                t["status"], t["code"] = x.get("status"), x.get("code") or 0
                t["detail"], t["keyAsked"] = x.get("detail", ""), x.get("keyAsked", False)
        answers = [x for x in texts if x["from"] == num and when(x["at"]) > sent]
        t["answers"] = [{"at": x["at"], "text": x["text"], "toUs": x["to"] == me} for x in answers]
        n = nodes.get(rid, {})
        if n.get("key") and not t.get("keyBefore") and not t.get("keyCame"):
            t["keyCame"] = datetime.now(timezone.utc).isoformat()  # its name card answered ours: it heard us
        t["lastHeard"], t["hopsNow"] = n.get("lastHeard"), n.get("hops")


def candidates(st, status, min_km, within_h):
    radio = status["radio"]
    here = status["settings"].get("where") or next((n["position"] for n in radio["nodes"] if n.get("self") and n.get("position")), None)
    since = datetime.now(timezone.utc) - timedelta(hours=within_h)
    hop = radio.get("hopLimit") or 3  # a message goes through this many relays at most
    placed, unplaced = [], []
    for n in radio["nodes"]:
        if n.get("self") or n.get("batnet") or n["id"] in st["tried"] or not n.get("lastHeard") or when(n["lastHeard"]) < since \
                or (n.get("hops") or 0) > hop:
            continue
        p = n.get("position")
        if p and here:
            d = km(here, p)
            if d >= min_km:
                placed.append((d, n))
        elif not p and (n.get("hops") or 0) >= 2:
            unplaced.append((None, n))
    placed.sort(key=lambda x: -x[0])
    unplaced.sort(key=lambda x: (-x[1]["hops"], -when(x[1]["lastHeard"]).timestamp()))
    return placed + unplaced


def message(d):
    if d is None:
        return NO_KM
    return WITH_KM.format(km=round(d) if d >= 10 else f"{d:.1f}".replace(".", ","))


def run(a):
    st = load(a.state)
    status = api("status")
    update(st, status)
    if len(st["tried"]) >= a.max:
        save(a.state, st)
        return print(f"done: {len(st['tried'])} radios tried")
    todo = candidates(st, status, a.min_km, a.within)
    if not todo:
        save(a.state, st)
        return print("no radio left to try")
    d, n = todo[0]
    text = message(d)
    assert len(text.encode()) <= 200, len(text.encode())
    try:
        sent = api("send", {"to": n["id"], "text": text})
    except RuntimeError as e:
        save(a.state, st)
        return print(f"not sent to {n['id']}: {e}; next round tries again")
    st["tried"][n["id"]] = {"name": n.get("longName") or n.get("shortName") or "", "km": d and round(d, 1),
                            "placeFrom": (n.get("position") or {}).get("from", ""), "accuracyM": (n.get("position") or {}).get("accuracyM"),
                            "hopsBefore": n.get("hops"), "keyBefore": bool(n.get("key")), "sent": sent["at"], "text": text,
                            "status": sent.get("status"), "answers": []}
    save(a.state, st)
    print(f"sent to {n['id']} {st['tried'][n['id']]['name']!r}" + (f" at {d:.1f} km" if d else ""))


def report(a):
    st = load(a.state)
    try:
        update(st, api("status"))
    except Exception as e:
        print(f"(hub not reached: {e}; the last saved state)")
    rows = sorted(st["tried"].values(), key=lambda t: t["sent"])
    print(f"{'sent':16} {'radio':18} {'km':>5} {'hops':>4} {'key':>9}  {'outcome':34} answers")
    for t in rows:
        key = "had" if t.get("keyBefore") else ("came" if t.get("keyCame") else "none yet")
        out = t.get("status") or "?"
        if t.get("code"):
            out += f" ({t['detail']}, {t['code']})"
        ans = "; ".join(("" if x["toUs"] else "(to all) ") + x["text"] for x in t.get("answers", []))
        print(f"{t['sent'][:16]:16} {t['name'][:18]:18} {t['km'] or '':>5} {t['hopsBefore'] if t['hopsBefore'] is not None else '':>4} {key:>9}  {out[:34]:34} {ans}")
    reached = [t for t in rows if t.get("status") == "delivered" or t.get("keyCame") or t.get("answers")]
    print(f"\n{len(rows)} tried; {len(reached)} reached (delivered, its name card came back, or it answered);"
          f" farthest reached: {max((t['km'] or 0 for t in reached), default=0)} km")


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("command", choices=["run", "report"])
    p.add_argument("--state", default=STATE)
    p.add_argument("--min-km", type=float, default=8)
    p.add_argument("--within", type=float, default=48, help="only radios heard in the last this many hours")
    p.add_argument("--max", type=int, default=30, help="radios to try in all")
    a = p.parse_args()
    (run if a.command == "run" else report)(a)


main()
