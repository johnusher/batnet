#!/usr/bin/env python3
"""Draw the radio diagrams for the README, in a light and a dark version.

    python3 docs/img/radio/make.py

Writes NAME-light.svg and NAME-dark.svg next to this file. Standard library
only. The numbers drawn here come from knowledge/research/eu868-radio-basics.md.
"""
import math
import os
import random

HERE = os.path.dirname(os.path.abspath(__file__))
FONT = "-apple-system, 'Segoe UI', Helvetica, Arial, sans-serif"

THEMES = {
    "light": dict(bg="#ffffff", fg="#1f2328", muted="#59636e", faint="#d1d9e0", panel="#f6f8fa",
                  hot="#d9480f", hotfill="#ffe8d9", wifi="#0969da", mobile="#8250df",
                  gadget="#1a7f37", alarm="#cf222e", sky="#ddf4ff", ground="#e9e2d0", tree="#2da44e",
                  bar="#d9480f"),
    "dark": dict(bg="#0d1117", fg="#e6edf3", muted="#9198a1", faint="#3d444d", panel="#151b23",
                 hot="#ff8c42", hotfill="#3d2414", wifi="#4493f8", mobile="#ab7df8",
                 gadget="#3fb950", alarm="#f85149", sky="#0f2233", ground="#2b2618", tree="#2ea043",
                 bar="#e8692a"),  # bars: inside the dark lightness band, unlike hot
}


class SVG:
    def __init__(self, w, h, t):
        self.w, self.h, self.t, self.parts = w, h, t, []

    def add(self, s):
        self.parts.append(s)

    def text(self, x, y, s, size=13, color="fg", anchor="start", weight="normal", italic=False):
        style = ' font-style="italic"' if italic else ""
        s = s.replace("&", "&amp;").replace("<", "&lt;")
        self.add(f'<text x="{x:.1f}" y="{y:.1f}" font-size="{size}" fill="{self.t[color]}" '
                 f'text-anchor="{anchor}" font-weight="{weight}"{style}>{s}</text>')

    def line(self, x1, y1, x2, y2, color="fg", width=1, dash=None):
        d = f' stroke-dasharray="{dash}"' if dash else ""
        self.add(f'<line x1="{x1:.1f}" y1="{y1:.1f}" x2="{x2:.1f}" y2="{y2:.1f}" '
                 f'stroke="{self.t[color]}" stroke-width="{width}"{d}/>')

    def rect(self, x, y, w, h, fill="panel", stroke=None, rx=0, opacity=1):
        s = f' stroke="{self.t[stroke]}"' if stroke else ""
        self.add(f'<rect x="{x:.1f}" y="{y:.1f}" width="{w:.1f}" height="{h:.1f}" rx="{rx}" '
                 f'fill="{self.t[fill]}" fill-opacity="{opacity}"{s}/>')

    def path(self, d, stroke="fg", width=1.5, fill="none", opacity=1, dash=None):
        f = self.t[fill] if fill != "none" else "none"
        da = f' stroke-dasharray="{dash}"' if dash else ""
        self.add(f'<path d="{d}" stroke="{self.t[stroke]}" stroke-width="{width}" fill="{f}" '
                 f'fill-opacity="{opacity}" stroke-linejoin="round" stroke-linecap="round"{da}/>')

    def save(self, name, theme, title):
        body = "\n".join(self.parts)
        svg = (f'<svg xmlns="http://www.w3.org/2000/svg" width="{self.w}" height="{self.h}" '
               f'viewBox="0 0 {self.w} {self.h}" font-family="{FONT}" role="img">\n'
               f'<title>{title}</title>\n'
               f'<rect width="100%" height="100%" rx="10" fill="{self.t["bg"]}"/>\n{body}\n</svg>\n')
        with open(os.path.join(HERE, f"{name}-{theme}.svg"), "w") as f:
            f.write(svg)


def dial(theme):
    """Where 869.5 MHz sits among the radios people know, on a log scale."""
    t = THEMES[theme]
    s = SVG(900, 330, t)
    lo, hi, x0, x1 = 60, 7000, 40, 860

    def x(f):
        return x0 + (math.log10(f) - math.log10(lo)) / (math.log10(hi) - math.log10(lo)) * (x1 - x0)

    s.text(450, 30, "Where batnet's long-range radio sits on the dial", 17, anchor="middle", weight="bold")
    axis = 170
    s.line(x0, axis, x1, axis, "muted", 1.5)
    for f, lab in ((100, "100 MHz"), (300, "300 MHz"), (1000, "1 GHz"), (3000, "3 GHz")):
        s.line(x(f), axis - 5, x(f), axis + 5, "muted")
        s.text(x(f), axis + 20, lab, 11, "muted", "middle")
    # bands: (lo, hi, label, color, row) - row < 0 above the axis, > 0 below
    bands = [
        (87.5, 108, "FM radio", "mobile", -1),
        (433.05, 434.79, "433 MHz gadgets:", "gadget", -2),
        (446.0, 446.2, "walkie-talkies (PMR446)", "gadget", 2),
        (791, 862, "4G mobile", "mobile", 2),
        (880, 960, "2G / 4G mobile", "mobile", 3),
        (1710, 1880, "mobile", "mobile", 2),
        (2400, 2483.5, "WiFi, Bluetooth,", "wifi", -1),
        (5150, 5875, "WiFi 5 GHz", "wifi", -2),
    ]
    for a, b, lab, col, row in bands:
        xa, xb = x(a), x(b)
        s.add(f'<rect x="{xa:.1f}" y="{axis - 9}" width="{max(xb - xa, 3):.1f}" height="18" '
              f'fill="{t[col]}" fill-opacity="0.75"/>')
        ty = axis - 22 - 30 * (abs(row) - 1) if row < 0 else axis + 42 + 26 * (row - 1)
        mid = (xa + xb) / 2
        s.line(mid, axis - 9 if row < 0 else axis + 9, mid, ty + (4 if row < 0 else -14), col, 1, "2 2")
        s.text(mid, ty, lab, 12, col, "middle")
    s.text(x(434), axis - 22 - 30 + 14, "car keys, weather stations", 11, "gadget", "middle")
    s.text(x(2440), axis - 22 + 14, "microwave ovens", 11, "wifi", "middle")
    # batnet's slot
    xb = x(869.525)
    s.add(f'<rect x="{xb - 2.5:.1f}" y="{axis - 30}" width="5" height="60" fill="{t["hot"]}"/>')
    s.line(xb, axis - 30, xb, 62, "hot", 1.5)
    s.text(xb, 56, "batnet + Meshtastic: 869.4–869.65 MHz", 14, "hot", "middle", "bold")
    # wavelength strip
    wy = 278
    s.text(x0, wy - 14, "Wavelength (the size of one radio wave):", 12, "muted")
    for f in (100, 434, 869.525, 2440, 5500):
        w = 299.792 / f
        lab = f"{w:.1f} m" if w >= 1 else f"{w * 100:.1f} cm"
        col = "hot" if f == 869.525 else "fg"
        s.line(x(f), wy - 4, x(f), wy + 4, col, 2)
        s.text(x(f), wy + 20, lab, 12, col, "middle", "bold" if col == "hot" else "normal")
    s.line(x0, wy, x1, wy, "faint", 1)
    s.text(450, 318, "Lower frequency means longer waves, which reach further and get through walls and trees better.",
           12, "muted", "middle", italic=True)
    s.save("dial", theme, "batnet's radio at 869.5 MHz compared with FM radio, walkie-talkies, mobile phones and WiFi")


def subbands(theme):
    """The EU 863-870 MHz short-range band: power and duty cycle per slice."""
    t = THEMES[theme]
    s = SVG(900, 400, t)
    x0, x1, f0, f1 = 50, 870, 863.0, 870.0

    def x(f):
        return x0 + (f - f0) / (f1 - f0) * (x1 - x0)

    s.text(450, 30, "Europe's 863–870 MHz band: free to use, with rules for each slice", 17, anchor="middle", weight="bold")
    base = 300

    def h(mw):
        return 26 + 44 * math.log10(mw)

    slices = [  # lo, hi, mW, duty, colour, label
        (863.0, 865.0, 10, "", "faint", "wireless mics"),
        (865.0, 868.0, 25, "1%", "faint", "general use"),
        (868.0, 868.6, 25, "1%", "gadget", ""),
        (868.6, 868.7, 10, "", "alarm", ""),
        (868.7, 869.2, 25, "0.1%", "faint", ""),
        (869.2, 869.4, 10, "", "alarm", ""),
        (869.4, 869.65, 500, "10%", "hot", ""),
        (869.65, 869.7, 25, "", "alarm", ""),
        (869.7, 870.0, 25, "", "faint", ""),
    ]
    for a, b, mw, duty, col, lab in slices:
        xa, xb, hh = x(a), x(b), h(mw)
        op = 0.9 if col in ("hot", "alarm") else 0.55
        s.add(f'<rect x="{xa + 0.5:.1f}" y="{base - hh:.1f}" width="{xb - xa - 1:.1f}" height="{hh:.1f}" '
              f'fill="{t[col]}" fill-opacity="{op}"/>')
        if lab:
            s.text((xa + xb) / 2, base - hh + 18, lab, 12, "fg", "middle")
            s.text((xa + xb) / 2, base - hh + 34, f"{mw} mW" + (f" · {duty}" if duty else ""), 11, "muted", "middle")
    s.line(x0, base, x1, base, "muted", 1.5)
    for f in range(863, 871):
        s.line(x(f), base, x(f), base + 6, "muted")
        s.text(x(f), base + 20, f"{f}", 11, "muted", "middle")
    s.text(x1, base + 20 + 16, "MHz", 11, "muted", "end")
    s.text(x0 - 8, base - h(500), "500 mW", 10, "muted", "end")
    s.text(x0 - 8, base - h(25) + 4, "25 mW", 10, "muted", "end")
    s.line(x0 - 4, base - h(500), x0, base - h(500), "muted")
    s.line(x0 - 4, base - h(25), x0, base - h(25), "muted")
    s.text(x0 - 8, base - h(500) - 18, "power", 10, "muted", "end")
    # call-outs
    xm = x(869.525)
    s.line(xm, base - h(500), xm, 72, "hot", 1)
    s.text(xm - 6, 66, "869.4–869.65 MHz: up to 500 mW, on the air 10% of the time", 13, "hot", "end", "bold")
    s.text(xm - 6, 84, "Meshtastic and batnet's radio. Only 0.25 MHz wide.", 12, "hot", "end")
    xl = x(868.3)
    s.line(xl, base - h(25), xl, 150, "gadget", 1, "2 2")
    s.text(xl - 6, 144, "868.1 / 868.3 / 868.5: LoRaWAN sensors", 12, "gadget", "end")
    s.text(xl - 6, 160, "(sensors and trackers, The Things Network)", 11, "muted", "end")
    s.add(f'<rect x="{x(866.0):.1f}" y="{base + 40}" width="12" height="12" fill="{t["alarm"]}" fill-opacity="0.9"/>')
    s.text(x(866.0) + 18, base + 51, "slices kept for alarms (smoke, burglar, social alarms)", 12, "muted")
    s.text(450, 388, "Simplified from CEPT's ERC Recommendation 70-03. Bar height: the power allowed; label: the share of time a radio may transmit.",
           11, "muted", "middle", italic=True)
    s.save("subbands", theme, "The EU 863-870 MHz band in slices; 869.4-869.65 MHz allows 500 mW at 10% duty cycle")


def chirps(theme):
    """LoRa symbols as chirps: preamble, sync, then data chirps that start at different heights."""
    t = THEMES[theme]
    s = SVG(900, 340, t)
    s.text(450, 30, "How LoRa sends data: chirps", 17, anchor="middle", weight="bold")
    top, bot, x, T = 70, 230, 70, 46
    s.rect(64, top - 6, 806, bot - top + 12, "panel", rx=6)
    s.text(56, top + 4, "869.65", 11, "muted", "end")
    s.text(56, bot + 4, "869.40", 11, "muted", "end")
    s.text(56, (top + bot) / 2 + 4, "MHz", 11, "muted", "end")

    def up(x, v=0.0, col="hot", w=2.2):
        y0 = bot - v * (bot - top)
        if v == 0:
            s.path(f"M{x:.1f},{bot} L{x + T:.1f},{top}", col, w)
            return
        xs = x + (1 - v) * T
        s.path(f"M{x:.1f},{y0:.1f} L{xs:.1f},{top}", col, w)
        s.path(f"M{xs:.1f},{top} L{xs:.1f},{bot}", col, 0.8, dash="2 3")
        s.path(f"M{xs:.1f},{bot} L{x + T:.1f},{y0:.1f}", col, w)
        s.add(f'<circle cx="{x:.1f}" cy="{y0:.1f}" r="3.5" fill="{t[col]}"/>')

    start = x
    for _ in range(6):
        up(x, col="muted")
        x += T
    pre_end = x
    for v in (0.375, 0.5):  # sync word symbols
        up(x, v, "mobile")
        x += T
    sync_end = x
    for i in range(2):
        s.path(f"M{x:.1f},{top} L{x + T:.1f},{bot}", "wifi", 2.2)
        x += T
    s.path(f"M{x:.1f},{top} L{x + T / 4:.1f},{top + (bot - top) / 4}", "wifi", 2.2)
    x += T / 4
    down_end = x
    rnd = random.Random(7)
    for _ in range(6):
        up(x, rnd.randrange(1, 32) / 32)
        x += T
    s.text(x + 8, (top + bot) / 2 + 4, "…", 18, "hot")
    for a, b, lab, col in ((start, pre_end, "preamble: “wake up, a message is coming”", "muted"),
                           (pre_end, sync_end, "network", "mobile"),
                           (sync_end, down_end, "start mark", "wifi"),
                           (down_end, x, "data: where each chirp starts is the data", "hot")):
        s.line(a + 2, bot + 18, b - 2, bot + 18, col, 1.5)
        s.text((a + b) / 2, bot + 36, lab, 12, col, "middle")
    s.text(450, 300, "Each chirp is a tone that slides up across the whole 250 kHz channel in 8.2 ms. On LongFast a chirp can start",
           12, "fg", "middle")
    s.text(450, 318, "at any of 2,048 places, so each one carries 11 bits. A receiver can pick these slides out even below the background hiss.",
           12, "fg", "middle")
    s.save("chirps", theme, "A LoRa transmission as chirps: preamble, sync word, down-chirps, then data chirps")


def antennas(theme):
    """Radiation patterns seen from the side: a low-gain whip and a high-gain collinear; a DIY quarter-wave."""
    t = THEMES[theme]
    s = SVG(900, 360, t)
    s.text(450, 30, "Antennas: gain is not extra power, it is the same power squeezed flatter", 17, anchor="middle", weight="bold")

    def panel(cx, title, sub, a, b, ant_h):
        cy = 175
        # pattern as two lobes (side view of a doughnut)
        for sgn in (-1, 1):
            pts = []
            for i in range(0, 181):
                th = math.radians(i)  # 0..180 from up to down
                r = abs(math.sin(th)) ** b
                px = cx + sgn * a * r * math.sin(th)
                py = cy - a * r * math.cos(th)
                pts.append(f"{px:.1f},{py:.1f}")
            s.path("M" + " L".join(pts) + " Z", "hot", 1.5, "hotfill", 0.9)
        s.line(cx, cy - ant_h / 2, cx, cy + ant_h / 2, "fg", 4)
        s.text(cx, 300, title, 14, "fg", "middle", "bold")
        s.text(cx, 318, sub, 12, "muted", "middle")

    panel(160, "Short whip, about 2 dBi", "the one in the box: good all round", 120, 1.0, 40)
    panel(470, "Long fibreglass mast, 6–8 dBi", "further over flat land, less up and down", 175, 6.0, 110)
    # DIY quarter wave ground plane
    cx, cy = 770, 175
    s.line(cx, cy, cx, cy - 92, "fg", 3)
    for dx, dy in ((-66, 66), (66, 66), (-30, 50), (30, 50)):
        s.line(cx, cy, cx + dx, cy + dy, "fg", 2)
    s.rect(cx - 10, cy - 4, 20, 10, "muted", rx=2)
    s.line(cx + 16, cy - 92, cx + 16, cy, "hot", 1)
    s.text(cx + 22, cy - 42, "8.2 cm", 13, "hot", weight="bold")
    s.text(cx + 22, cy - 26, "(a quarter wave)", 11, "hot")
    s.text(cx, cy + 92, "4 wires, about 8.6 cm, bent down 45°", 11, "muted", "middle")
    s.text(cx, 300, "Make your own", 14, "fg", "middle", "bold")
    s.text(cx, 318, "a quarter-wave ground plane on an SMA plug", 12, "muted", "middle")
    s.text(450, 348, "Seen from the side. The orange shape shows where the signal goes; the antenna stands upright in the middle.",
           11, "muted", "middle", italic=True)
    s.save("antennas", theme, "Side-view radiation patterns of a 2 dBi whip and a 6-8 dBi collinear, and a DIY quarter-wave ground plane")


def ranges(theme):
    """Typical ranges on a log scale, from a pocket radio in town to the record."""
    t = THEMES[theme]
    s = SVG(900, 350, t)
    s.text(450, 30, "How far does it reach? It depends on height and what is in the way", 17, anchor="middle", weight="bold")
    x0, x1, lo, hi = 300, 830, 0.3, 500

    def x(km):
        return x0 + (math.log10(km) - math.log10(lo)) / (math.log10(hi) - math.log10(lo)) * (x1 - x0)

    rows = [
        ("In a pocket, in a town", 0.5, 2),
        ("Node indoors, in a village", 1, 3),
        ("Antenna on the roof", 3, 10),
        ("Mast or hilltop, open view", 10, 50),
        ("Mountain to mountain", 100, 331),
    ]
    y = 70
    for lab, a, b in rows:
        s.text(x0 - 12, y + 14, lab, 13, "fg", "end")
        s.rect(x(a), y, x(b) - x(a), 20, "hot", rx=4, opacity=0.85)
        s.text(x(b) + 6, y + 15, f"{a:g}–{b:g} km" if b != 331 else "100+ km", 12, "muted")
        y += 38
    xr = x(331)
    s.line(xr, 60, xr, y - 10, "hot", 1.5, "3 3")
    s.text(xr, 56, "record: 331 km", 12, "hot", "middle", "bold")
    ya = y + 4
    s.line(x0, ya, x1, ya, "muted")
    for km in (0.5, 1, 2, 5, 10, 20, 50, 100, 200):
        s.line(x(km), ya, x(km), ya + 5, "muted")
        s.text(x(km), ya + 19, f"{km:g}", 11, "muted", "middle")
    s.text(x1, ya + 36, "kilometres", 11, "muted", "end")
    s.text(450, 338, "Rough guides, not promises: the reach of one radio. Messages can also hop from radio to radio.", 11, "muted", "middle", italic=True)
    s.save("range", theme, "Typical LoRa ranges from 0.5 km in town to 100+ km mountain to mountain; record 331 km")


def fresnel(theme):
    """Side view of a 2 km link: the Fresnel zone and what blocks it."""
    t = THEMES[theme]
    s = SVG(900, 330, t)
    s.text(450, 30, "Why height matters: the signal travels in a fat zone, not a thin line", 17, anchor="middle", weight="bold")
    gy = 270
    s.rect(20, 50, 860, gy - 50, "sky", rx=0, opacity=0.6)
    s.rect(20, gy, 860, 30, "ground")
    xa, xb = 90, 810
    ya = yb = 142
    # houses with masts
    for hx in (xa, xb):
        s.path(f"M{hx - 34},{gy} L{hx - 34},{gy - 60} L{hx},{gy - 88} L{hx + 34},{gy - 60} L{hx + 34},{gy} Z",
               "muted", 1.5, "panel")
        s.line(hx, gy - 88, hx, ya, "fg", 2.5)
        s.add(f'<circle cx="{hx}" cy="{ya}" r="4" fill="{t["hot"]}"/>')
    # Fresnel ellipse; radius 13 m halfway on 2 km at 869 MHz; draw 1 m = 4.2 px vertically
    cx, rx_, ry = (xa + xb) / 2, (xb - xa) / 2, 13 * 4.2
    s.add(f'<ellipse cx="{cx}" cy="{ya}" rx="{rx_}" ry="{ry:.1f}" fill="{t["hot"]}" fill-opacity="0.16" '
          f'stroke="{t["hot"]}" stroke-width="1.5" stroke-dasharray="5 4"/>')
    s.line(xa, ya, xb, ya, "hot", 1.5)
    # trees poking into the zone
    for tx, th in ((330, 120), (372, 135), (412, 112)):
        s.add(f'<ellipse cx="{tx}" cy="{gy - th + 34}" rx="26" ry="38" fill="{t["tree"]}" fill-opacity="0.85"/>')
        s.line(tx, gy - th + 60, tx, gy, "muted", 4)
    s.line(cx, ya, cx, ya + ry, "fg", 1)
    s.text(cx + 8, ya + ry / 2 + 4, "13 m", 13, "fg", weight="bold")
    s.text(cx, ya - ry - 10, "the first Fresnel zone, halfway along a 2 km link at 869 MHz", 12, "hot", "middle")
    s.text(296, gy - 46, "trees in the lower part", 12, "fg", "end")
    s.text(296, gy - 32, "weaken the signal", 12, "fg", "end")
    s.text(xa, gy + 20, "house A", 12, "fg", "middle")
    s.text(xb, gy + 20, "house B, 2 km away", 12, "fg", "middle")
    s.text(450, 318, "Keep most of the zone clear: raise the antenna above roofs and trees. A few metres of height beats more power.",
           12, "muted", "middle", italic=True)
    s.save("fresnel", theme, "A 2 km link between two houses: the Fresnel zone is 13 m deep halfway, and trees intrude on it")


def waterfall(theme):
    """An illustrated spectrogram of 867.8-870 MHz: who shares the band over 20 seconds."""
    t = THEMES[theme]
    s = SVG(900, 560, t)
    s.text(450, 30, "Sharing the band: 20 seconds of radio traffic around 868–870 MHz", 17, anchor="middle", weight="bold")
    x0, x1, f0, f1 = 60, 840, 867.8, 870.0
    y0, secs, pps = 60, 20, 19  # top of the waterfall, seconds shown, pixels per second

    def x(f):
        return x0 + (f - f0) / (f1 - f0) * (x1 - x0)

    def y(sec):
        return y0 + sec * pps

    s.add('<defs>'
          '<filter id="hiss" x="0" y="0" width="100%" height="100%">'
          '<feTurbulence type="fractalNoise" baseFrequency="0.9" numOctaves="2" seed="3"/>'
          '<feColorMatrix values="0 0 0 0 0.16  0 0 0 0 0.30  0 0 0 0 0.85  0 0 0 0.55 0"/></filter>'
          '<linearGradient id="burst" x1="0" x2="1" y1="0" y2="0">'
          '<stop offset="0" stop-color="#ffe14d" stop-opacity="0.35"/><stop offset="0.12" stop-color="#ffd23f"/>'
          '<stop offset="0.3" stop-color="#ff4d1a"/><stop offset="0.7" stop-color="#ff4d1a"/>'
          '<stop offset="0.88" stop-color="#ffd23f"/><stop offset="1" stop-color="#ffe14d" stop-opacity="0.35"/>'
          '</linearGradient></defs>')
    s.add(f'<rect x="{x0}" y="{y0}" width="{x1 - x0}" height="{secs * pps}" fill="#1c2f8f"/>')
    s.add(f'<rect x="{x0}" y="{y0}" width="{x1 - x0}" height="{secs * pps}" filter="url(#hiss)"/>')
    # the Meshtastic slot
    s.add(f'<rect x="{x(869.4):.1f}" y="{y0}" width="{x(869.65) - x(869.4):.1f}" height="{secs * pps}" '
          f'fill="none" stroke="{t["hot"]}" stroke-width="1.5" stroke-dasharray="4 3"/>')

    def burst(fc, bw_khz, start, dur, strength=1.0):
        a, b = x(fc - bw_khz / 2000), x(fc + bw_khz / 2000)
        s.add(f'<rect x="{a:.1f}" y="{y(start):.1f}" width="{b - a:.1f}" height="{max(dur * pps, 1.6):.1f}" '
              f'fill="url(#burst)" fill-opacity="{strength:.2f}"/>')

    # LoRaWAN sensors at 868.1/868.3/868.5, 125 kHz, mostly short (SF7-SF9), one slow one (SF12)
    rnd = random.Random(11)
    for _ in range(14):
        burst(rnd.choice((868.1, 868.3, 868.5)), 125, rnd.uniform(0, 19.5), rnd.choice((0.06, 0.1, 0.2, 0.4)),
              rnd.uniform(0.45, 1))
    burst(868.3, 125, 12.0, 1.5, 0.6)
    # wireless M-Bus meters at 868.95 MHz, narrow and short
    for st in (2.2, 7.9, 15.4):
        burst(868.95, 60, st, 0.08, 0.9)
    # an alarm sensor
    burst(869.25, 25, 9.6, 0.15, 1)
    # Meshtastic LongFast: a message, then two neighbours repeating it; other radios' packets
    for st, dur, k in ((1.0, 0.7, 1), (2.05, 0.7, 0.75), (3.2, 0.7, 0.5),  # one message, repeated twice
                       (6.4, 1.3, 0.9), (11.0, 0.55, 0.6), (14.3, 0.9, 1), (17.6, 0.55, 0.7)):
        burst(869.525, 250, st, dur, k)
    # a LoRaWAN gateway answering a sensor on the same frequency (125 kHz)
    burst(869.525, 125, 8.7, 0.25, 0.85)
    # axis
    for f in (868.0, 868.5, 869.0, 869.5, 870.0):
        s.line(x(f), y(secs), x(f), y(secs) + 6, "muted")
        s.text(x(f), y(secs) + 20, f"{f:.1f}", 11, "muted", "middle")
    s.text(x1, y(secs) + 36, "MHz", 11, "muted", "end")
    for sec in (0, 5, 10, 15, 20):
        s.line(x0 - 6, y(sec), x0, y(sec), "muted")
        s.text(x0 - 10, y(sec) + 4, f"{sec} s", 11, "muted", "end")
    s.text(x0 - 10, y0 - 10, "time ↓", 11, "muted", "end")
    # labels
    lab_y = y(secs) + 58
    for fc, line1, line2, col in ((868.3, "LoRaWAN sensors", "868.1 / .3 / .5", "gadget"),
                                  (868.95, "smart meters", "(wireless M-Bus)", "muted"),
                                  (869.25, "alarm", "", "alarm"),
                                  (869.525, "Meshtastic + batnet", "869.4–869.65", "hot")):
        s.line(x(fc), y(secs) + 26 if fc != 869.525 else y(secs) + 40, x(fc), lab_y - 14, col, 1, "2 2")
        s.text(x(fc), lab_y, line1, 12, col, "middle", "bold")
        if line2:
            s.text(x(fc), lab_y + 15, line2, 11, "muted", "middle")
    # call-outs on the right of the slot
    xr = x(869.65) + 10
    s.line(xr - 6, y(1.0), xr - 6, y(3.9), "fg", 1)
    s.text(xr, y(1.6), "one message,", 12, "fg")
    s.text(xr, y(1.6) + 15, "then two neighbours", 12, "fg")
    s.text(xr, y(1.6) + 30, "repeating it", 12, "fg")
    s.text(xr, y(8.85) + 4, "← a LoRaWAN gateway", 12, "fg")
    s.text(xr + 14, y(8.85) + 19, "uses this slot too", 12, "fg")
    s.text(450, 546, "An illustration drawn from the band rules, not a recording. Brighter means a stronger (closer) radio.",
           11, "muted", "middle", italic=True)
    s.save("waterfall", theme, "Illustrated waterfall of 867.8-870 MHz: LoRaWAN sensors, meters, an alarm and Meshtastic bursts sharing the band")


def dechirp(theme):
    """How a receiver reads a chirp: multiply by a falling chirp, then an FFT. Top: the wanted
    chirp collapses into one detector. Bottom: a steady interfering beep is scattered."""
    t = THEMES[theme]
    s = SVG(900, 470, t)
    s.text(450, 30, "How a receiver reads a chirp: multiply by a falling chirp, then sort by pitch", 17,
           anchor="middle", weight="bold")
    cols = (40, 260, 480, 700)
    pw, ph = 170, 120
    v = 61 / 250  # value 500 starts 61 kHz up the 250 kHz channel

    def frame(x, y):
        s.rect(x, y, pw, ph, "panel", rx=6)

    def ln(x, y, a, b, c, d, col, w=2.2, dash=None):  # a,b,c,d in 0..1 panel units, b/d from the bottom
        s.line(x + a * pw, y + ph - b * ph, x + c * pw, y + ph - d * ph, col, w, dash)

    def rising(x, y, start, col):
        xs = 1 - start
        ln(x, y, 0, start, xs, 1, col)
        ln(x, y, xs, 1, xs, 0, col, 0.8, "2 3")
        ln(x, y, xs, 0, 1, start, col)

    def falling(x, y, start, col):
        ln(x, y, 0, start, start, 0, col)
        if start < 1:
            ln(x, y, start, 0, start, 1, col, 0.8, "2 3")
            ln(x, y, start, 1, 1, start, col)

    def bars(x, y, spike, floor, col, seed):
        rnd = random.Random(seed)
        n = 48
        bw = pw / n
        for i in range(n):
            hgt = floor + rnd.uniform(0, 0.08)
            if spike is not None and i == spike:
                hgt = 0.92
            s.rect(x + i * bw + 0.5, y + ph - hgt * ph, bw - 1, hgt * ph,
                   "hot" if spike is not None and i == spike else col, opacity=0.9)

    def ops(y):
        for cx, sym in ((235, "×"), (455, "="), (675, "FFT")):
            s.text(cx, y + ph / 2 + 6, sym, 20 if sym != "FFT" else 13, "fg", "middle", "bold")

    rows = ((70, "The wanted chirp (value 500, starting at 61 kHz)"),
            (270, "A steady beep from another gadget"))
    for y, title in rows:
        s.text(cols[0], y - 10, title, 13, "fg", weight="bold")
        for x in cols:
            frame(x, y)
        ops(y)
    # top row
    y = 70
    rising(cols[0], y, v, "hot")
    falling(cols[1], y, 1.0, "wifi")
    ln(cols[2], y, 0, v, 1, v, "hot", 3)
    s.text(cols[2] + pw - 6, y + ph - v * ph - 8, "61 kHz, steady", 11, "hot", "end")
    bars(cols[3], y, 12, 0.05, "faint", 1)
    s.text(cols[3] + 12.5 * pw / 48 + 6, y + 16, "detector 500 lights up", 11, "hot")
    labels = ("what arrives", "the receiver's own falling chirp", "the slide cancels: one beep", "2,048 detectors, one lit")
    for x, lab in zip(cols, labels):
        s.text(x + pw / 2, y + ph + 18, lab, 11, "muted", "middle")
    # bottom row
    y = 270
    b = 180 / 250
    ln(cols[0], y, 0, b, 1, b, "alarm", 3)
    falling(cols[1], y, 1.0, "wifi")
    falling(cols[2], y, b, "alarm")
    bars(cols[3], y, None, 0.16, "alarm", 2)
    labels = ("a beep at 180 kHz", "the same falling chirp", "the beep becomes a falling chirp", "spread thinly over all of them")
    for x, lab in zip(cols, labels):
        s.text(x + pw / 2, y + ph + 18, lab, 11, "muted", "middle")
    s.text(450, 446, "Each panel: frequency up the side (0 to 250 kHz across the channel), time along the bottom (one 8.2 ms symbol).",
           11, "muted", "middle", italic=True)
    s.text(450, 462, "The FFT panels show detectors by pitch, drawn as 48 bars instead of 2,048.", 11, "muted", "middle", italic=True)
    s.save("dechirp", theme, "A receiver multiplies the incoming chirp by a falling chirp to get a steady tone, then an FFT lights one detector; an interfering beep is spread over all detectors")


def sync(theme):
    """Finding the timing and the crystal error: rising chirps shift one way, falling chirps the other."""
    t = THEMES[theme]
    s = SVG(900, 316, t)
    s.text(450, 30, "Finding the start: rising and falling chirps shift opposite ways", 17, anchor="middle", weight="bold")
    pw, ph = 380, 150
    for i, (x, kind, col) in enumerate(((50, "rising", "muted"), (470, "falling", "wifi"))):
        y = 60
        s.rect(x, y, pw, ph, "panel", rx=6)
        # the true chirp, and the receiver's window drawn late by a quarter of a symbol
        T = 0.22
        for k in range(2):
            x0 = x + k * pw / 2
            if kind == "rising":
                s.line(x0, y + ph, x0 + pw / 2, y, col, 2.2)
            else:
                s.line(x0, y, x0 + pw / 2, y + ph, col, 2.2)
        wx = x + T * pw / 2
        s.add(f'<rect x="{wx:.1f}" y="{y - 4}" width="{pw / 2:.1f}" height="{ph + 8}" fill="none" '
              f'stroke="{t["hot"]}" stroke-width="1.6" stroke-dasharray="5 4" rx="3"/>')
        s.text(wx + pw / 4, y + ph + 22, "the receiver's window, a little late", 11, "hot", "middle")
        what = ("Rising: a late window sees the chirp higher up", "→ the peak moves UP by the delay")
        if kind == "falling":
            what = ("Falling: a late window sees the chirp lower down", "→ the peak moves DOWN by the delay")
        s.text(x + pw / 2, y + ph + 44, what[0], 12, "fg", "middle", "bold")
        s.text(x + pw / 2, y + ph + 60, what[1], 12, "fg", "middle")
    s.text(450, 304, "A crystal that is off moves both peaks UP. Rising gives delay + crystal; falling gives crystal − delay: two sums, two unknowns.",
           11, "muted", "middle", italic=True)
    s.save("sync", theme, "Rising chirps shift up and falling chirps shift down when the receiver's window is late, which separates timing from crystal error")


def bar_up(s, x, y0, w, h, fill):
    """A bar standing on y0 with 4 px rounded top corners."""
    r = min(4, w / 2, h)
    s.add(f'<path d="M{x:.1f},{y0:.1f} V{y0 - h + r:.1f} Q{x:.1f},{y0 - h:.1f} {x + r:.1f},{y0 - h:.1f} '
          f'H{x + w - r:.1f} Q{x + w:.1f},{y0 - h:.1f} {x + w:.1f},{y0 - h + r:.1f} V{y0:.1f} Z" fill="{s.t[fill]}"/>')


def bar_right(s, x0, y, w, h, fill):
    """A bar from x0 to the right with 4 px rounded end corners."""
    r = min(4, h / 2, w)
    s.add(f'<path d="M{x0:.1f},{y:.1f} H{x0 + w - r:.1f} Q{x0 + w:.1f},{y:.1f} {x0 + w:.1f},{y + r:.1f} '
          f'V{y + h - r:.1f} Q{x0 + w:.1f},{y + h:.1f} {x0 + w - r:.1f},{y + h:.1f} H{x0:.1f} Z" fill="{s.t[fill]}"/>')


# Meshtastic's EU_868 presets: spreading factor, bandwidth (kHz), link budget (dB),
# and the time on air of one 150-byte packet (the LoRa airtime formula, header and
# CRC on, 16-symbol preamble; knowledge/research/lora-airtime-eu868.md).
PRESETS = [("Short Fast", 7, 250, 143, 0.127), ("Short Slow", 8, 250, 145.5, 0.223),
           ("Medium Fast", 9, 250, 148, 0.406), ("Medium Slow", 10, 250, 150.5, 0.751),
           ("Long Fast", 11, 250, 153, 1.378), ("Long Moderate", 11, 125, 156, 4.919)]


def presets(theme):
    """Why LongFast and MediumFast radios can't hear each other, and what each preset costs in airtime."""
    t = THEMES[theme]
    s = SVG(900, 576, t)
    s.text(450, 30, "The same frequency, different speeds: Meshtastic's settings in Europe", 17, anchor="middle", weight="bold")

    # Top: 8.2 ms of the channel, one LongFast chirp against four MediumFast chirps.
    top, bot, x0, x1 = 76, 186, 120, 840
    s.rect(x0, top, x1 - x0, bot - top, "panel", rx=6)
    s.text(x0 - 8, top + 10, "869.65", 11, "muted", "end")
    s.text(x0 - 8, bot, "869.40", 11, "muted", "end")
    s.text(x0 - 8, (top + bot) / 2 + 4, "MHz", 11, "muted", "end")
    s.path(f"M{x0},{bot} L{x1},{top}", "wifi", 2.6)
    q = (x1 - x0) / 4
    for k in range(4):
        s.path(f"M{x0 + k * q:.1f},{bot} L{x0 + (k + 1) * q:.1f},{top}", "bar", 2.6)
    for lx0, col, lab in ((x0, "bar", "Medium Fast: four quick chirps, 2 ms each"),
                          (x0 + 2.2 * q, "wifi", "Long Fast: one slow chirp, 8.2 ms")):
        s.line(lx0, top - 12, lx0 + 24, top - 12, col, 2.6)
        s.text(lx0 + 32, top - 8, lab, 12, "fg", "start", "bold")
    s.line(x0, bot + 14, x1, bot + 14, "muted")
    for i, ms in enumerate((0, 2, 4, 6, 8.2)):
        xx = x0 + (x1 - x0) * ms / 8.192
        s.line(xx, bot + 14, xx, bot + 19, "muted")
        s.text(xx, bot + 32, f"{ms:g} ms", 11, "muted", "middle")
    s.text(450, bot + 56, "Both slide across the same 250 kHz at 869.525 MHz. A receiver listens for one slope only, so a Long Fast radio",
           12, "fg", "middle")
    s.text(450, bot + 73, "and a Medium Fast radio side by side hear nothing of each other: not a word, not even as noise they can read.",
           12, "fg", "middle")

    # Bottom: time on air of one 150-byte message on each preset.
    y, lx, bx, bw = 306, 200, 330, 470
    s.text(450, y - 16, "How long one message (150 bytes) is on the air", 14, "fg", "middle", "bold")
    longest = PRESETS[-1][4]
    for name, sf, khz, budget, secs in PRESETS:
        s.text(lx, y + 15, name, 13, "fg", "end", "bold" if name in ("Medium Fast", "Long Fast") else "normal")
        s.text(lx + 10, y + 15, f"SF {sf}" + ("" if khz == 250 else f", {khz} kHz"), 11, "muted")
        fill = {"Medium Fast": "bar", "Long Fast": "wifi"}.get(name, "faint")
        bar_right(s, bx, y + 2, max(3, bw * secs / longest), 18, fill)
        note = {"Medium Fast": "  · Berlin", "Long Fast": "  · the default, most of Europe"}.get(name, "")
        s.text(bx + bw * secs / longest + 8, y + 15, f"{secs:.1f} s{note}" if secs >= 0.3 else f"{secs:.2f} s{note}", 12, "fg")
        y += 30
    s.text(450, y + 22, "Slower settings reach a little further (about 2.5 dB more per step) but fill the air for longer, so in a busy city",
           12, "fg", "middle")
    s.text(450, y + 39, "more messages collide. Each radio may send for 6 minutes an hour: about 260 such messages on Long Fast, 880 on Medium Fast.",
           12, "fg", "middle")
    s.save("presets", theme, "One LongFast chirp against four MediumFast chirps on the same channel, and the airtime of a 150-byte message on each EU preset")


# One night on MediumFast at the bench hub in Berlin, 2026-10-21 22:32 to
# 2026-10-22 07:14: packets read from other radios, by how many radios passed
# each one on (knowledge/tests/2026-10-22-one-night-on-mediumfast.md).
NIGHT_RELAYS = [28, 259, 428, 460, 264, 192, 97, 49]


def night(theme):
    """How many relays each packet the hub read had come through, one night in Berlin."""
    t = THEMES[theme]
    s = SVG(900, 400, t)
    s.text(450, 30, "One night in Berlin: how many radios passed each message on before the hub heard it", 16, anchor="middle", weight="bold")
    x0, x1, y0, ytop = 110, 840, 320, 70
    peak = 500
    for v in (0, 100, 200, 300, 400, 500):
        yy = y0 - (y0 - ytop) * v / peak
        s.line(x0, yy, x1, yy, "faint", 1 if v else 1.2)
        s.text(x0 - 10, yy + 4, f"{v}", 11, "muted", "end")
    s.text(x0 - 10, ytop - 14, "packets", 11, "muted", "end")
    slot = (x1 - x0) / len(NIGHT_RELAYS)
    w = slot - 2 - 22
    for i, n in enumerate(NIGHT_RELAYS):
        x = x0 + i * slot + 12
        bar_up(s, x, y0, w, (y0 - ytop) * n / peak, "bar")
        s.text(x + w / 2, y0 + 20, "direct" if i == 0 else f"{i}", 12, "fg", "middle")
        if i in (0, 3, 7):
            s.text(x + w / 2, y0 - (y0 - ytop) * n / peak - 8, f"{n}", 12, "fg", "middle", "bold")
    s.text((x0 + x1) / 2, y0 + 42, "radios it came through on the way", 12, "muted", "middle")
    s.text(450, 386, "1,788 packets read from 154 radios between 22:30 and 07:15, on Medium Fast. Only 28 came straight from the sender.",
           12, "fg", "middle")
    s.save("berlin-night", theme, "One night of packets at the Berlin hub by number of relays: 28 direct, most through 2 to 4 relays, 49 through 7")


# The radios the bench hub heard straight (no relay) on Medium Fast, from its
# panel on 2026-10-23: (bearing in degrees, distance in km, SNR in dB), rounded
# to 10 degrees and 1 km so the hub's place can't be worked back from them.
# RELAYED: radios in other directions heard only through relays, same rounding.
DIRECT = [(300, 4, -11), (310, 12, -12), (320, 2, -14.5), (330, 1, -12)]
RELAYED = [(0, 4), (50, 7), (200, 3)]
FLOOR_MF, PATH_EXP, SPREAD = -15, 3, 30  # as in internal/hub/panel/panel.js coverage()


def zone():
    """The panel's coverage(): each straight radio's reach, spread 30 degrees either side, tapering."""
    reach = [(b, max(d, d * 10 ** ((snr - FLOOR_MF) / (10 * PATH_EXP)))) for b, d, snr in DIRECT]
    out = []
    for a in range(0, 360, 2):
        m = 0
        for b, r in reach:
            off = abs((a - b + 540) % 360 - 180)
            if off < SPREAD:
                m = max(m, r * math.cos(off / SPREAD * math.pi / 2) ** 2)
        out.append((a, m))
    return reach, out


def area(pts):
    """Area in km² of a polygon given as (bearing, km) pairs, by the shoelace formula."""
    xy = [(r * math.sin(math.radians(a)), r * math.cos(math.radians(a))) for a, r in pts]
    return abs(sum(x0 * y1 - x1 * y0 for (x0, y0), (x1, y1) in zip(xy, xy[1:] + xy[:1]))) / 2


def directivity(theme):
    """A range circle against the zone the hub works out from the radios it hears straight."""
    t = THEMES[theme]
    s = SVG(900, 540, t)
    s.text(450, 30, "Which way does the hub hear? Not in a circle", 18, anchor="middle", weight="bold")
    s.text(450, 52, "The first hub in Berlin. Every radio it hears with no relay shows how far it reaches in that direction.",
           12, "muted", "middle")
    reach, petal = zone()
    far = max(r for _, r in reach)
    px = 150 / 16  # pixels per km

    def at(cx, cy, brg, km):
        return cx + km * px * math.sin(math.radians(brg)), cy - km * px * math.cos(math.radians(brg))

    def grid(cx, cy):
        for km in (5, 10, 15):
            s.add(f'<circle cx="{cx}" cy="{cy}" r="{km * px:.1f}" fill="none" stroke="{t["faint"]}" stroke-width="1"/>')
            x, y = at(cx, cy, 135, km)
            s.text(x + 3, y + 4, f"{km} km", 10, "muted")
        s.line(cx, cy - 16 * px, cx, cy + 16 * px, "faint", 1, "2 4")
        s.line(cx - 16 * px, cy, cx + 16 * px, cy, "faint", 1, "2 4")
        s.text(cx, cy - 16 * px - 6, "N", 12, "muted", "middle", "bold")

    def hub(cx, cy):
        s.rect(cx - 5, cy - 5, 10, 10, "fg", rx=2)
        s.text(cx + 9, cy + 16, "hub", 11, "fg")

    # Left: the usual guess, one range figure drawn all the way round.
    cx, cy = 215, 255
    s.add(f'<circle cx="{cx}" cy="{cy}" r="{far * px:.1f}" fill="{t["hotfill"]}" fill-opacity="0.85" '
          f'stroke="{t["hot"]}" stroke-width="2" stroke-dasharray="5 4"/>')
    grid(cx, cy)
    hub(cx, cy)
    s.text(cx, 430, "The usual guess: one range, all the way round", 14, "fg", "middle", "bold")
    s.text(cx, 448, f"{far:.0f} km in every direction: about {area([(a, far) for a in range(0, 360, 2)]):,.0f} km²",
           12, "muted", "middle")

    # Right: what the hub works out.
    cx, cy = 655, 270
    grid(cx, cy)
    pts = " L".join(f"{x:.1f},{y:.1f}" for x, y in (at(cx, cy, a, r) for a, r in petal))
    s.path("M" + pts + " Z", "hot", 2, "hotfill", 0.85)
    for b, d, _ in DIRECT:
        x, y = at(cx, cy, b, d)
        s.line(cx, cy, x, y, "hot", 1.5, "5 4")
        s.add(f'<circle cx="{x:.1f}" cy="{y:.1f}" r="5" fill="{t["fg"]}" stroke="{t["bg"]}" stroke-width="1.5"/>')
    for b, d in RELAYED:
        x, y = at(cx, cy, b, d)
        s.add(f'<circle cx="{x:.1f}" cy="{y:.1f}" r="5" fill="{t["bg"]}" stroke="{t["muted"]}" stroke-width="2"/>')
    hub(cx, cy)
    b, d, snr = DIRECT[1]
    x, y = at(cx, cy, b, d)
    s.text(x - 22, y - 44, f"heard {d} km away with", 11, "fg", "end")
    s.text(x - 22, y - 30, f"{snr - FLOOR_MF:.0f} dB to spare: could be {far:.0f} km", 11, "fg", "end")
    x, y = at(cx, cy, *RELAYED[2])
    s.text(x - 10, y + 4, f"{RELAYED[2][1]} km away, yet heard", 11, "muted", "end")
    s.text(x - 10, y + 18, "only through relays", 11, "muted", "end")
    s.text(cx, 430, "What batnet works out from the signal", 14, "fg", "middle", "bold")
    s.text(cx, 448, f"{far:.0f} km to the north-west, unknown elsewhere: about {area(petal):,.0f} km²",
           12, "muted", "middle")

    # Legend and source.
    y = 482
    s.add(f'<circle cx="190" cy="{y - 4}" r="5" fill="{t["fg"]}"/>')
    s.text(200, y, "heard straight, no relay", 12, "fg")
    s.add(f'<circle cx="375" cy="{y - 4}" r="5" fill="{t["bg"]}" stroke="{t["muted"]}" stroke-width="2"/>')
    s.text(385, y, "heard only through relays", 12, "fg")
    s.rect(566, y - 11, 16, 12, "hotfill", "hot", rx=2)
    s.text(588, y, "where it should hear a radio straight", 12, "fg")
    s.text(450, 512, "Readings of 23 October 2026 on Medium Fast, rounded to 10° and 1 km. The antenna is near the roof on one side of the house,",
           11, "muted", "middle", italic=True)
    s.text(450, 527, "so the house itself is probably in the way of the rest. A radio lower down or indoors is heard less far.",
           11, "muted", "middle", italic=True)
    s.save("directivity", theme, "A range circle of 15 km all round against the zone the hub works out: 15 km to the north-west only, from four radios heard with no relay")


def board(s, x, y, w_mm, h_mm, k, holes, header, chip, ports):
    """A Raspberry Pi board seen from above, to scale (k pixels per mm)."""
    t = s.t
    s.add(f'<rect x="{x}" y="{y}" width="{w_mm * k:.1f}" height="{h_mm * k:.1f}" rx="{3 * k:.1f}" '
          f'fill="{t["panel"]}" stroke="{t["gadget"]}" stroke-width="2"/>')
    for hx, hy in holes:
        s.add(f'<circle cx="{x + hx * k:.1f}" cy="{y + hy * k:.1f}" r="{1.4 * k:.1f}" fill="{t["bg"]}" stroke="{t["faint"]}"/>')
    hx0, hy0 = header
    for i in range(20):
        for j in range(2):
            s.add(f'<circle cx="{x + (hx0 + i * 2.54) * k:.1f}" cy="{y + (hy0 + j * 2.54) * k:.1f}" r="{0.6 * k:.1f}" fill="{t["muted"]}"/>')
    cx_, cy_, cw = chip
    s.rect(x + cx_ * k, y + cy_ * k, cw * k, cw * k, "faint", rx=2)
    for px_, py_, pw, ph in ports:
        s.rect(x + px_ * k, y + py_ * k, pw * k, ph * k, "muted", rx=2, opacity=0.6)


def two_pis(theme):
    """The two Pis batnet runs on in Berlin, to scale, with what each was tried with."""
    t = THEMES[theme]
    s = SVG(900, 400, t)
    s.text(450, 30, "Tested on both: a Raspberry Pi 4 and a Pi Zero 2 W", 18, anchor="middle", weight="bold")
    s.text(450, 52, "Drawn to scale. Both have run a hub in Berlin, with the same software and a radio on a USB stick.", 12, "muted", "middle")
    k = 2.6
    # Pi 4: 85 x 56 mm, holes 58 x 49 mm apart, USB and Ethernet on the right edge.
    board(s, 80, 80, 85, 56, k, [(3.5, 3.5), (61.5, 3.5), (3.5, 52.5), (61.5, 52.5)], (8, 2.3), (25, 25, 15),
          [(66, 2, 21, 15), (68, 21, 19, 14), (68, 40, 19, 14), (12, 50, 8, 8), (26, 50, 7, 8), (40, 50, 7, 8)])
    # Pi Zero 2 W: 65 x 30 mm, the same hole spacing along, ports on the long edge.
    board(s, 520, 148, 65, 30, k, [(3.5, 3.5), (61.5, 3.5), (3.5, 26.5), (61.5, 26.5)], (8, 2.3), (25, 12, 11),
          [(8, 25, 11, 6), (38, 25, 8, 6), (50, 25, 8, 6)])

    def facts(x, title, sub, rows, price):
        s.text(x, 255, title, 15, "fg", weight="bold")
        s.text(x, 273, sub, 12, "muted")
        for i, (ok, line) in enumerate(rows):
            yy = 300 + i * 20
            s.text(x, yy, "✓" if ok else "✗", 13, "gadget" if ok else "muted", weight="bold")
            s.text(x + 18, yy, line, 12, "fg" if ok else "muted")
        s.text(x, 300 + len(rows) * 20 + 6, price, 12, "hot", weight="bold")

    facts(80, "Pi 4, 2 GB memory", "on the air from 21 to 23 October 2026",
          [(True, "the radio, the app and the control panel"), (True, "200 phones at once, no errors"),
           (True, "a screen and the offline translator"), (True, "15 h 48 min on a 72 Wh power bank")],
          "about €106 for the 4 GB model")
    facts(520, "Pi Zero 2 W, 512 MB memory", "on the air since 23 October 2026, in a loft",
          [(True, "the radio, the app and the control panel"), (True, "200 phones at once, no errors"),
           (True, "offline maps of five areas"), (False, "too little memory for a screen or translator")],
          "about €19")
    s.save("two-pis", theme, "A Raspberry Pi 4 and a Pi Zero 2 W drawn to scale, with what each hub was tested with")


if __name__ == "__main__":
    for th in THEMES:
        for draw in (dial, subbands, chirps, antennas, ranges, fresnel, waterfall, dechirp, sync, presets, night,
                     directivity, two_pis):
            draw(th)
    print("wrote", sorted(f for f in os.listdir(HERE) if f.endswith(".svg")))
