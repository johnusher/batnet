#!/usr/bin/env python3
"""Who is out there on the other presets? Listens on one preset after another
and counts the radios heard on each, then goes back to the home preset.

  survey.py run      the survey (as root on the hub; about an hour a preset)
  survey.py report   what each preset heard
  survey.py restore  back to the home preset and the zone's readings back, if
                     the survey stopped half-way (a guard: run it at boot and
                     after the survey's time; it does nothing while it runs)

It runs on the hub and talks to the panel API on localhost, never to
meshtasticd (B38). On each preset it says one hello that asks the radios in
range to answer (the quickest way to find who is there; once an hour is
mild). The zone's readings are kept aside before the first change and put
back after the last, so other presets' signals do not reach the map's zone.
The state file names real radios: keep it out of the repo.
"""
import argparse, json, os, shutil, subprocess, time, urllib.request
from datetime import datetime, timezone

HUB = "http://localhost/hub/api/"
STATE = "/var/lib/batnet/presetsurvey.json"
LINKS = "/var/lib/batnet/links.json"
# ShortTurbo's 500 kHz does not fit EU868's 869.4-869.65 MHz band.
PRESETS = "LongFast,LongModerate,LongSlow,MediumSlow,ShortSlow,ShortFast"


def api(path, body=None):
    req = urllib.request.Request(HUB + path, headers={"X-Hub": "1", "Content-Type": "application/json"},
                                 data=None if body is None else json.dumps(body).encode())
    try:
        with urllib.request.urlopen(req, timeout=20) as r:
            return json.load(r)
    except urllib.error.HTTPError as e:
        raise RuntimeError(json.load(e).get("error", str(e)))


def now():
    return datetime.now(timezone.utc).isoformat()


def say(*a):
    print(datetime.now().strftime("%H:%M:%S"), *a, flush=True)


def save(path, st):
    with open(path + ".new", "w") as f:
        json.dump(st, f, indent=1, ensure_ascii=False)
    os.replace(path + ".new", path)


def radio():
    try:
        return api("status")["radio"]
    except Exception:
        return {}


def switch(preset, wait=300):
    """Sets the preset and waits for the radio to come back on it."""
    if radio().get("preset") == preset and radio().get("ready"):
        return True
    for _ in range(3):
        try:
            api("radio", {"preset": preset})
            break
        except Exception as e:
            say("change refused:", e)
            time.sleep(20)
    end = time.time() + wait
    while time.time() < end:
        time.sleep(10)
        r = radio()
        if r.get("ready") and r.get("preset") == preset:
            return True
    return False


def frames(seq):
    out = api(f"traffic?since={seq}")
    fs = out.get("frames") or []
    return fs, max([seq] + [f["seq"] for f in fs])


def latest():
    """The newest frame's number: a poll returns at most 1000, oldest first."""
    seq = 0
    while True:
        fs, nxt = frames(seq)
        if not fs or nxt == seq:
            return seq
        seq = nxt


def run(a):
    st = {"started": now(), "home": a.home, "minutes": a.minutes, "presets": []}
    if os.path.exists(LINKS):
        shutil.copy2(LINKS, LINKS + ".survey")  # the zone's readings, put back at the end
    try:
        for p in a.presets.split(","):
            say("switching to", p)
            entry = {"preset": p, "frames": [], "hello": None}
            st["presets"].append(entry)
            if not switch(p):
                entry["error"] = "the radio did not come back on it"
                say(p, "failed")
                save(a.state, st)
                continue
            entry["from"] = now()
            seq = latest()  # start after what is already there
            time.sleep(60)
            try:
                api("hello", {})
                entry["hello"] = now()
            except Exception as e:
                entry["hello"] = f"not said: {e}"
            end = time.time() + a.minutes * 60
            while time.time() < end:
                time.sleep(min(60, max(1, end - time.time())))
                try:
                    fs, seq = frames(seq)
                except Exception:
                    continue
                for f in fs:
                    if f["kind"] in ("heard", "garbled"):
                        entry["frames"].append({k: f[k] for k in ("t", "kind", "from", "name", "to", "port", "encrypted", "snr", "rssi", "hops", "relay", "text", "lat", "lon", "info") if k in f})
                save(a.state, st)
            entry["to"] = now()
            say(p, len(entry["frames"]), "frames")
            save(a.state, st)
    finally:
        say("back to", a.home)
        ok = switch(a.home)
        st["back"] = now() if ok else "failed: run survey.py restore"
        save(a.state, st)
        if ok and os.path.exists(LINKS + ".survey"):
            subprocess.run(["systemctl", "stop", "batnet-hub"], check=False)
            shutil.copy2(LINKS + ".survey", LINKS)
            subprocess.run(["systemctl", "start", "batnet-hub"], check=False)
            say("zone readings put back")


def restore(a):
    """After a reboot or a crash half-way: back to the home preset, and the zone's readings back."""
    if subprocess.run(["systemctl", "is-active", "-q", "batnet-presetsurvey.service"]).returncode == 0:
        return say("the survey is still running")
    if radio().get("preset") != a.home:
        say("not on", a.home, "- switching back:", switch(a.home))
    try:
        with open(a.state) as f:
            st = json.load(f)
    except (FileNotFoundError, ValueError):
        return
    if not st.get("back", "").startswith("20") and os.path.exists(LINKS + ".survey"):  # none yet, or "failed: ..."
        subprocess.run(["systemctl", "stop", "batnet-hub"], check=False)
        shutil.copy2(LINKS + ".survey", LINKS)
        subprocess.run(["systemctl", "start", "batnet-hub"], check=False)
        st["back"] = now() + " (by restore)"
        save(a.state, st)
        say("zone readings put back")


def report(a):
    with open(a.state) as f:
        st = json.load(f)
    print(f"started {st['started'][:16]}, {st['minutes']} min a preset, back on {st['home']}: {st.get('back', 'not yet')}\n")
    print(f"{'preset':13} {'radios':>6} {'read':>5} {'locked':>6} {'unreadable':>10} {'straight':>8} {'median SNR':>10}")
    for e in st["presets"]:
        fs = e["frames"]
        heard = [f for f in fs if f["kind"] == "heard"]
        radios = {f["from"] for f in heard if f.get("from")}
        snrs = sorted(f["snr"] for f in heard if f.get("snr"))
        straight = {f["from"] for f in heard if f.get("hops") == 0}
        med = f"{snrs[len(snrs) // 2]:.2f} dB" if snrs else "-"
        print(f"{e['preset']:13} {len(radios):>6} {sum(not f.get('encrypted') for f in heard):>5} {sum(bool(f.get('encrypted')) for f in heard):>6}"
              f" {sum(f['kind'] == 'garbled' for f in fs):>10} {len(straight):>8} {med:>10}" + (f"  {e['error']}" if e.get("error") else ""))
    for e in st["presets"]:
        names = {}
        for f in e["frames"]:
            if f["kind"] == "heard" and f.get("from"):
                n = names.setdefault(f["from"], {"name": "", "n": 0, "hops": 99})
                n["name"] = n["name"] or f.get("name") or (f.get("info") or {}).get("longName", "")
                n["n"] += 1
                if f.get("hops") is not None and f["hops"] >= 0:
                    n["hops"] = min(n["hops"], f["hops"])
        if names:
            print(f"\n{e['preset']}: " + ", ".join(f"{v['name'] or k} ({v['n']}, {v['hops'] if v['hops'] < 99 else '?'} hops)"
                                                for k, v in sorted(names.items(), key=lambda x: -x[1]["n"])))


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("command", choices=["run", "report", "restore"])
    p.add_argument("--presets", default=PRESETS)
    p.add_argument("--minutes", type=float, default=60, help="listening time on each preset")
    p.add_argument("--home", default="MediumFast", help="the preset to go back to")
    p.add_argument("--state", default=STATE)
    a = p.parse_args()
    {"run": run, "report": report, "restore": restore}[a.command](a)


main()
