#!/usr/bin/env python3
"""How closely a hub's temperature follows the weather.

    tools/temptrack/compare.py pz7                 # pull the hub's log, compare, write the page
    tools/temptrack/compare.py pz7 --station 00433 # another DWD station (00433: Berlin-Tempelhof)

The hub notes its CPU temperature every 5 minutes (deploy/pi/batnet-temp-log,
/var/lib/batnet/temperature.csv). This pulls that file over ssh, fetches the
hourly air temperature and sunshine of a public DWD weather station from
Bright Sky (api.brightsky.dev; only the station's number is sent, never the
hub's place), puts both on the hour and prints:

  - the gap: how much warmer the Pi is than the air, on average and its range;
  - how closely it follows (correlation) and by how much (slope: degrees of
    Pi per degree of air; 1 means it moves one for one, 0.3 that a loft or a
    box damps the weather to a third);
  - the lag: the hours by which the Pi trails the air, where it follows best;
  - the day's swing of each, day by day.

Everything lands in private/temptrack/<hub>/ (gitignored): the hub's log, the
weather, the joined hours (hours.csv) and a page with the two curves
(temptrack.html). The default station 00427 is Berlin Brandenburg (the
airport, south-east of the city); pick the one nearest the hub.
"""
import argparse
import csv
import datetime as dt
import json
import math
import pathlib
import statistics as st
import subprocess
import sys
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parents[2]


def pull(hub):
    out = subprocess.run(["ssh", hub, "sudo cat /var/lib/batnet/temperature.csv"],
                         capture_output=True, text=True, check=True).stdout
    rows = []
    for r in csv.DictReader(out.splitlines()):
        try:
            rows.append((dt.datetime.fromisoformat(r["time"].replace("Z", "+00:00")),
                         float(r["cpu_c"]), float(r["load1"])))
        except (KeyError, ValueError):
            continue
    return out, rows


def weather(station, start, end):
    """Hourly air temperature (°C) and sunshine (minutes) per UTC hour."""
    url = ("https://api.brightsky.dev/weather?dwd_station_id=%s&date=%s&last_date=%s&tz=Etc/UTC"
           % (station, start.strftime("%Y-%m-%dT%H:00"), end.strftime("%Y-%m-%dT%H:00")))
    req = urllib.request.Request(url, headers={"User-Agent": "batnet-temptrack"})
    with urllib.request.urlopen(req, timeout=30) as f:
        data = json.load(f)
    out = {}
    for w in data.get("weather", []):
        if w.get("temperature") is None:
            continue
        t = dt.datetime.fromisoformat(w["timestamp"]).astimezone(dt.timezone.utc)
        out[t] = (w["temperature"], w.get("sunshine"))
    name = next((s.get("station_name") for s in data.get("sources", []) if s.get("station_name")), station)
    return out, name


def hourly(rows):
    """The Pi's mean temperature and load per UTC hour (an hour with fewer than 6 readings is left out)."""
    by = {}
    for t, c, l in rows:
        by.setdefault(t.replace(minute=0, second=0, microsecond=0), []).append((c, l))
    return {h: (st.mean(c for c, _ in v), st.mean(l for _, l in v)) for h, v in by.items() if len(v) >= 6}


def corr(a, b):
    if len(a) < 3 or st.pstdev(a) == 0 or st.pstdev(b) == 0:
        return None
    ma, mb = st.mean(a), st.mean(b)
    return sum((x - ma) * (y - mb) for x, y in zip(a, b)) / (len(a) * st.pstdev(a) * st.pstdev(b))


def slope(x, y):
    if len(x) < 3 or st.pstdev(x) == 0:
        return None
    mx, my = st.mean(x), st.mean(y)
    return sum((a - mx) * (b - my) for a, b in zip(x, y)) / sum((a - mx) ** 2 for a in x)


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("hub", help="the hub's ssh name, e.g. pz7")
    ap.add_argument("--station", default="00427", help="DWD station id (default 00427, Berlin Brandenburg)")
    a = ap.parse_args()

    raw, rows = pull(a.hub)
    if not rows:
        sys.exit(f"{a.hub}: no temperature log yet (is batnet-temp-log.timer on?)")
    d = ROOT / "private" / "temptrack" / a.hub
    d.mkdir(parents=True, exist_ok=True)
    (d / "temperature.csv").write_text(raw)

    pi = hourly(rows)
    start, end = rows[0][0], rows[-1][0] + dt.timedelta(hours=1)
    air, station = weather(a.station, start, end)
    (d / "weather.json").write_text(json.dumps({t.isoformat(): v for t, v in sorted(air.items())}))

    hours = sorted(h for h in pi if h in air)
    with open(d / "hours.csv", "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["hour_utc", "pi_c", "load1", "air_c", "sunshine_min"])
        for h in hours:
            w.writerow([h.isoformat(), round(pi[h][0], 2), round(pi[h][1], 2), air[h][0], air[h][1]])

    span = (rows[-1][0] - rows[0][0]).total_seconds() / 3600
    print(f"{a.hub}: {len(rows)} readings over {span:.0f} hours, {len(hours)} full hours with weather from {station}")
    print(f"  the Pi now {rows[-1][1]:.1f} °C, between {min(r[1] for r in rows):.1f} and {max(r[1] for r in rows):.1f} °C")
    if len(hours) < 3:
        print("  too few hours to compare yet: run it again after a day or two")
        return
    p = [pi[h][0] for h in hours]
    o = [air[h][0] for h in hours]
    gap = [x - y for x, y in zip(p, o)]
    print(f"  air {min(o):.1f} to {max(o):.1f} °C; the Pi {st.mean(gap):.1f} °C warmer on average "
          f"({min(gap):.1f} to {max(gap):.1f})")
    lags = []
    for lag in range(0, 7):
        pairs = [(pi[h][0], air[h - dt.timedelta(hours=lag)][0]) for h in hours
                 if h - dt.timedelta(hours=lag) in air]
        r = corr([x for x, _ in pairs], [y for _, y in pairs])
        if r is not None:
            lags.append((r, lag, slope([y for _, y in pairs], [x for x, _ in pairs])))
    if lags:
        r0 = next((x for x in lags if x[1] == 0), None)
        best = max(lags)
        if r0:
            print(f"  same hour: correlation {r0[0]:.2f}, {r0[2]:.2f} °C of Pi per °C of air")
        if best[1]:
            print(f"  follows best {best[1]} h behind the air: correlation {best[0]:.2f}, {best[2]:.2f} °C per °C")
    rl = corr([pi[h][1] for h in hours], p)
    if rl is not None:
        print(f"  load against the Pi's temperature: correlation {rl:.2f} (load {min(pi[h][1] for h in hours):.2f} to {max(pi[h][1] for h in hours):.2f})")
    days = {}
    for h in hours:
        days.setdefault(h.date(), []).append((pi[h][0], air[h][0]))
    full = [(k, v) for k, v in sorted(days.items()) if len(v) >= 20]
    if full:
        print("  day's swing (max - min):")
        for k, v in full:
            ps, os_ = [x for x, _ in v], [y for _, y in v]
            print(f"    {k:%a %d %b}  air {max(os_) - min(os_):4.1f} °C   Pi {max(ps) - min(ps):4.1f} °C")
    page(d, a.hub, station, hours, pi, air)
    print(f"files: {d}")


def page(d, hub, station, hours, pi, air):
    data = [[h.isoformat(), round(pi[h][0], 2), air[h][0], air[h][1] or 0] for h in hours]
    html = """<!doctype html><meta charset="utf-8"><title>%(hub)s temperature</title>
<style>body{font:14px system-ui,sans-serif;margin:24px;color:#13201e}svg{width:100%%;height:auto}
.k{display:flex;gap:16px}.k i{display:inline-block;width:12px;height:3px;margin-right:6px;vertical-align:3px}</style>
<h2>%(hub)s: the Pi against the air (%(station)s)</h2>
<div class="k"><span><i style="background:#c0392b"></i>Pi CPU °C</span><span><i style="background:#2563a8"></i>air °C</span><span><i style="background:#f2c94c"></i>sunshine (min/h)</span></div>
<svg id="c" viewBox="0 0 1000 380"></svg>
<script>
const D=%(data)s, W=1000,H=380,m={l:40,r:10,t:10,b:30};
const t=D.map(d=>Date.parse(d[0])), lo=Math.floor(Math.min(...D.map(d=>Math.min(d[1],d[2])))-1), hi=Math.ceil(Math.max(...D.map(d=>Math.max(d[1],d[2])))+1);
const x=v=>m.l+(v-t[0])/(t[t.length-1]-t[0]||1)*(W-m.l-m.r), y=v=>m.t+(hi-v)/(hi-lo)*(H-m.t-m.b);
let g='';
for(let v=lo;v<=hi;v+=Math.max(1,Math.round((hi-lo)/8)))g+=`<line x1="${m.l}" x2="${W-m.r}" y1="${y(v)}" y2="${y(v)}" stroke="#ddd"/><text x="${m.l-6}" y="${y(v)+4}" text-anchor="end" font-size="11">${v}</text>`;
D.forEach((d,i)=>{if(d[3])g+=`<rect x="${x(t[i])}" y="${H-m.b-d[3]/60*40}" width="${Math.max(1,(W-m.l-m.r)/D.length)}" height="${d[3]/60*40}" fill="#f2c94c"/>`;
 const h=new Date(t[i]);if(h.getUTCHours()===0)g+=`<text x="${x(t[i])}" y="${H-8}" font-size="11">${h.toISOString().slice(5,10)}</text>`});
const line=(k,c)=>`<polyline fill="none" stroke="${c}" stroke-width="2" points="${D.map((d,i)=>x(t[i])+','+y(d[k])).join(' ')}"/>`;
document.getElementById('c').innerHTML=g+line(2,'#2563a8')+line(1,'#c0392b');
</script>""" % {"hub": hub, "station": station, "data": json.dumps(data)}
    (d / "temptrack.html").write_text(html)


if __name__ == "__main__":
    main()
