#!/usr/bin/env python3
"""Compares how strongly two hubs hear the same radios straight (no relay).

  tools/hubcompare/compare.py pz7 pi7:/var/lib/batnet/antenna-links-2026-10-23.json

Each side is an ssh host, optionally with a links file on it (default: the
hub's live /var/lib/batnet/links.json). It reads the files and the hubs'
/hub/api/status, never meshtasticd, so the radios are not disturbed (B38).
Places come from both hubs' radios; distances and bearings from the first
hub's place. The output names real radios and places: keep it out of the repo.
"""
import json, math, statistics, subprocess, sys

LIVE = "/var/lib/batnet/links.json"


def ssh(host, cmd):
    return subprocess.run(["ssh", "-o", "ConnectTimeout=8", host, cmd], check=True, capture_output=True, text=True).stdout


def side(arg):
    host, _, path = arg.partition(":")
    links = json.loads(ssh(host, f"sudo cat {path or LIVE}"))
    status = json.loads(ssh(host, "curl -s -H 'X-Hub: 1' http://localhost/hub/api/status"))
    return host + (" (saved)" if path else ""), links, status


def geo(w, p):
    r = math.pi / 180
    x = (p["lon"] - w["lon"]) * r * math.cos(w["lat"] * r)
    y = (p["lat"] - w["lat"]) * r
    return 6371 * math.hypot(x, y), (math.degrees(math.atan2(x, y)) + 360) % 360


def main():
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    (na, la, sa), (nb, lb, sb) = side(sys.argv[1]), side(sys.argv[2])
    w = sa["settings"].get("where")
    nodes = {}
    for s in (sb, sa):  # the first hub's knowledge wins
        for n in s["radio"]["nodes"]:
            old = nodes.get(n["id"], {})
            nodes[n["id"]] = {**old, **{k: v for k, v in n.items() if v is not None}, "position": n.get("position") or old.get("position")}
    selves = {n["id"] for s in (sa, sb) for n in s["radio"]["nodes"] if n.get("self")}

    def snr(links, k):
        return statistics.median(links[k]["snrs"]) if k in links else None

    rows = []
    for k in set(la) | set(lb):
        if k in selves:
            continue  # the hubs hear each other in the same house
        n, p = nodes.get(k, {}), nodes.get(k, {}).get("position")
        d, b = geo(w, p) if (w and p) else (None, None)
        rows.append((n.get("shortName") or k[-4:], d, b, p and p.get("accuracyM"), snr(la, k), la.get(k, {}).get("count", 0), snr(lb, k), lb.get(k, {}).get("count", 0)))
    rows.sort(key=lambda r: (r[4] is None or r[6] is None, -(r[1] or 0)))
    print(f"{'radio':8} {'km':>5} {'bearing':>7} {'place ±m':>8}   {na[:16]:>16} n    {nb[:16]:>16} n    difference")
    for name, d, b, acc, s1, c1, s2, c2 in rows:
        f = lambda v: f"{v:6.2f} dB" if v is not None else "      -  "
        diff = f"{s1 - s2:+.2f} dB" if s1 is not None and s2 is not None else ""
        print(f"{name[:8]:8} {d and round(d, 1) or '':>5} {b and f'{b:.0f}°' or '':>7} {acc or '':>8}   {f(s1):>16} {c1:<4} {f(s2):>16} {c2:<4} {diff}")
    both = [r[4] - r[6] for r in rows if r[4] is not None and r[6] is not None]
    if both:
        print(f"\nHeard straight by both: {len(both)}; {na} is {statistics.median(both):+.2f} dB against {nb} (median).")
    print(f"Only {na}: {sum(r[6] is None for r in rows)}; only {nb}: {sum(r[4] is None for r in rows)}.")


main()
