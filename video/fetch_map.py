"""Fetch the Berlin scene's map: OpenStreetMap tiles and the outlines of the localities that went
dark in September 2025. Run once (render.py runs it when the files are missing):

    python3 video/fetch_map.py

The files land in video/assets/berlin/ and video/assets/berlin-map.json, which are gitignored: they
are OpenStreetMap data (ODbL, (c) OpenStreetMap contributors), fetched fresh rather than kept in the
repo, and the repo's secret-hygiene check rightly refuses files full of precise coordinates. The
outlines are public administrative boundaries; the fire's position is rounded to about a kilometre.
Facts and sources: knowledge/research/berlin-blackout-2025.md.
"""
import json
import math
import os
import time
import urllib.parse
import urllib.request
from pathlib import Path

HERE = Path(__file__).parent
TILES = HERE / "assets" / "berlin"
DATA = HERE / "assets" / "berlin-map.json"
UA = {"User-Agent": "batnet-film/1.0 (https://github.com/johnusher/batnet; one-off fetch of ~80 tiles)"}
Z, WEST, NORTH, COLS, ROWS = 12, 13.08, 52.62, 12, 7
FIRE = [13.50, 52.45]  # the Koenigsheide pylons, Johannisthal, to about a kilometre
DARK = ["Johannisthal", "Adlershof", "Niederschöneweide", "Altglienicke", "Grünau", "Rudow"]


def tile_x(lon: float) -> float:
    return (lon + 180) / 360 * 2 ** Z


def tile_y(lat: float) -> float:
    r = math.radians(lat)
    return (1 - math.log(math.tan(r) + 1 / math.cos(r)) / math.pi) / 2 * 2 ** Z


def fetch_tiles() -> tuple[int, int]:
    TILES.mkdir(parents=True, exist_ok=True)
    x0, y0 = int(tile_x(WEST)), int(tile_y(NORTH))
    n = 0
    for x in range(x0, x0 + COLS):
        for y in range(y0, y0 + ROWS):
            p = TILES / f"{Z}-{x}-{y}.png"
            if p.exists():
                continue
            req = urllib.request.Request(f"https://tile.openstreetmap.org/{Z}/{x}/{y}.png", headers=UA)
            p.write_bytes(urllib.request.urlopen(req, timeout=30).read())
            n += 1
            time.sleep(0.25)  # the tile usage policy: no bulk downloads; this is 84 tiles, once
    print(f"tiles: {n} fetched, {COLS * ROWS} in {TILES}")
    return x0, y0


def outline(query: str, threshold: float) -> list:
    """The largest outer ring of a Nominatim result, simplified, as [lon, lat] pairs."""
    q = urllib.parse.urlencode({"q": query, "format": "json", "polygon_geojson": 1, "polygon_threshold": threshold, "limit": 1})
    req = urllib.request.Request("https://nominatim.openstreetmap.org/search?" + q, headers=UA)
    r = json.loads(urllib.request.urlopen(req, timeout=30).read())
    time.sleep(1.2)  # Nominatim: at most one request a second
    if not r:
        raise SystemExit(f"fetch_map: no result for {query!r}")
    g = r[0]["geojson"]
    polys = [g["coordinates"]] if g["type"] == "Polygon" else g["coordinates"]
    ring = max(polys, key=lambda p: len(p[0]))[0]
    print(f"outline: {query} -> {r[0]['display_name'][:50]} ({len(ring)} points)")
    return [[round(a, 5), round(b, 5)] for a, b in ring]


def main() -> None:
    x0, y0 = fetch_tiles()
    if DATA.exists():
        print(f"outlines: {DATA} exists")
        return
    data = {"z": Z, "x0": x0, "y0": y0, "cols": COLS, "rows": ROWS, "tile": 256, "fire": FIRE,
            "source": "OpenStreetMap tiles (standard style) and outlines via Nominatim; (c) OpenStreetMap contributors, ODbL",
            "berlin": outline("Berlin, Deutschland", 0.004),
            "district": outline("Treptow-Köpenick, Berlin", 0.001),
            "dark": {n: outline(f"{n}, Berlin, Deutschland", 0.0007) for n in DARK}}
    DATA.write_text(json.dumps(data))
    print(f"outlines: {DATA}")


if __name__ == "__main__":
    main()
