#!/usr/bin/env python3
"""Renders the batnet film: narration (ElevenLabs), a timeline, frames drawn by
scenes.js in headless Chrome, sound effects, and ffmpeg to an MP4 for YouTube.

    python3 video/render.py             # everything: voice, timeline, frames, mux
    python3 video/render.py --draft     # 720p, 15 fps, the cheaper voice model: a quick look
    python3 video/render.py --mux       # only re-mix audio into the frames already rendered
    python3 video/render.py --preview   # serve video/ and open the real-time preview in a browser

Output: video/out/batnet.mp4 (and batnet-draft.mp4). Everything under video/out
is a cache and is not committed. Needs ffmpeg and Google Chrome (or set $CHROME).
"""
from __future__ import annotations

import argparse
import http.server
import json
import math
import os
import shutil
import socketserver
import struct
import subprocess
import sys
import threading
import time
import wave
import webbrowser
from pathlib import Path

import voice

HERE = Path(__file__).resolve().parent
OUT = HERE / "out"
PORT = 8766
LEAD = 0.45  # seconds of picture before the narrator starts a scene: a clip on the cut itself sounds like an edit

CHROME_CANDIDATES = [
    os.environ.get("CHROME", ""),
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    "/Applications/Chromium.app/Contents/MacOS/Chromium",
    shutil.which("google-chrome") or "",
    shutil.which("chromium") or "",
    shutil.which("chromium-browser") or "",
]


def chrome() -> str:
    for c in CHROME_CANDIDATES:
        if c and Path(c).exists():
            return c
    sys.exit("render: no Chrome found; set $CHROME to the browser binary")


# ---------- timeline ----------

def build_timeline(draft: bool) -> dict:
    sc = voice.load_script()
    v = sc["voice"]
    model = v["draft_model_id"] if draft else v["model_id"]
    key = voice.api_key()
    t = 0.0
    scenes = []
    for s, prev, nxt in voice.neighbours(sc["scenes"]):
        clip = voice.speak(s, v, model, key, prev, nxt)
        d = voice.duration(clip)
        lead = float(s.get("lead", LEAD))
        length = max(float(s.get("min", 0)), lead + d + float(s.get("pad", 0.5)))
        scenes.append({"id": s["id"], "start": round(t, 3), "end": round(t + length, 3), "lead": lead, "speech": round(d, 3),
                       "text": s["text"], "caption": s.get("caption", s["text"]), "clip": clip.name})
        t += length
    tl = {"fps": 15 if draft else 30, "width": 1280 if draft else 1920, "height": 720 if draft else 1080,
          "total": round(t, 3), "scenes": scenes}
    OUT.mkdir(exist_ok=True)
    (OUT / "timeline.json").write_text(json.dumps(tl, indent=1))
    print(f"timeline: {len(scenes)} scenes, {t:.1f} s")
    return tl


# ---------- frames: a local server the page posts to ----------

class State:
    frames = 0
    events: list = []
    done = threading.Event()
    frames_dir = OUT / "frames"


def make_handler(state: State):
    class H(http.server.SimpleHTTPRequestHandler):
        def __init__(self, *a, **kw):
            super().__init__(*a, directory=str(HERE), **kw)

        def log_message(self, *a):
            pass

        def end_headers(self):
            self.send_header("Access-Control-Allow-Origin", "*")
            self.send_header("Cache-Control", "no-store")
            super().end_headers()

        def do_POST(self):
            n = int(self.headers.get("Content-Length", 0))
            body = self.rfile.read(n)
            if self.path.startswith("/frame/"):
                i = int(self.path.rsplit("/", 1)[1])
                (state.frames_dir / f"{i:06d}.png").write_bytes(body)
                state.frames = max(state.frames, i + 1)
            elif self.path == "/events":
                state.events = json.loads(body)
            elif self.path == "/done":
                state.done.set()
            elif self.path == "/log":
                print("page:", body.decode(errors="replace"))
            self.send_response(204)
            self.end_headers()

    return H


def serve(state: State) -> socketserver.TCPServer:
    socketserver.TCPServer.allow_reuse_address = True
    srv = socketserver.ThreadingTCPServer(("127.0.0.1", PORT), make_handler(state))
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    return srv


def render_frames(tl: dict) -> list:
    state = State()
    if state.frames_dir.exists():
        shutil.rmtree(state.frames_dir)
    state.frames_dir.mkdir(parents=True)
    srv = serve(state)
    # a profile of its own per run: a Chrome left over from a killed render holds the old one,
    # and the new Chrome would exit at once with "the profile is in use"
    profile = OUT / f"chrome-profile-{os.getpid()}"
    cmd = [chrome(), "--headless=new", "--disable-gpu", "--hide-scrollbars", "--no-first-run",
           f"--user-data-dir={profile}", f"--window-size={tl['width']},{tl['height']}",
           f"http://127.0.0.1:{PORT}/scenes.html?render=1"]
    total = int(math.ceil(tl["total"] * tl["fps"]))
    print(f"frames: {total} at {tl['width']}x{tl['height']}, {tl['fps']} fps")
    p = subprocess.Popen(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        last = -1
        t0 = time.time()
        while not state.done.wait(2):
            if state.frames != last:
                last = state.frames
                print(f"\r  {state.frames}/{total} ({time.time() - t0:.0f} s)", end="", flush=True)
            if p.poll() is not None:
                sys.exit("\nrender: Chrome exited before the frames were done")
            if time.time() - t0 > 3600:
                sys.exit("\nrender: gave up after an hour")
        print(f"\r  {state.frames}/{total} frames in {time.time() - t0:.0f} s")
    finally:
        p.terminate()
        try:
            p.wait(10)
        except subprocess.TimeoutExpired:
            p.kill()
        srv.shutdown()
        shutil.rmtree(profile, ignore_errors=True)
    (OUT / "events.json").write_text(json.dumps(state.events))
    return state.events


# ---------- sound effects and a music bed, synthesised ----------

RATE = 48000


def tone(seconds: float, f: float, kind: str) -> list:
    n = int(RATE * seconds)
    out = []
    for i in range(n):
        t = i / RATE
        env = math.exp(-t * (6 if kind == "blip" else 2.2))
        if kind == "blip":  # a soft radio chirp: rises a little, dies fast
            s = math.sin(2 * math.pi * (f + 400 * t) * t) * env * 0.5
        elif kind == "boom":  # a low thud with a noise burst
            s = (math.sin(2 * math.pi * (f * (1 - 0.6 * min(1, t * 3))) * t) * env
                 + (hash((i * 2654435761) & 0xFFFFFFFF) % 2000 / 1000 - 1) * math.exp(-t * 12) * 0.5) * 0.9
        elif kind == "alert":  # two rising notes
            s = (math.sin(2 * math.pi * f * t) + 0.6 * math.sin(2 * math.pi * f * 1.5 * t)) * env * 0.4
        else:  # a swell
            s = math.sin(2 * math.pi * f * t) * min(1, t * 4) * math.exp(-t * 1.2) * 0.4
        out.append(s)
    return out


def music_bed(seconds: float) -> list:
    """A slow synth pad: two detuned fifths, breathing, with a pulse every bar.
    Quiet under the narration; disable with --no-music."""
    n = int(RATE * seconds)
    out = []
    chords = [(110.0, 164.81, 220.0), (98.0, 146.83, 196.0), (87.31, 130.81, 174.61), (98.0, 146.83, 196.0)]
    for i in range(n):
        t = i / RATE
        bar = int(t / 8) % len(chords)
        fade = min(1, t / 6, max(0, (seconds - t) / 6))
        breathe = 0.6 + 0.4 * math.sin(2 * math.pi * t / 9)
        s = 0.0
        for k, f in enumerate(chords[bar]):
            s += math.sin(2 * math.pi * f * t) * (0.5 if k == 0 else 0.35)
            s += math.sin(2 * math.pi * f * 1.004 * t) * 0.2
        pulse = math.exp(-((t % 2) * 5)) * 0.25 * math.sin(2 * math.pi * 55 * t)
        out.append((s * 0.22 * breathe + pulse) * fade)
    return out


def write_wav(path: Path, samples: list) -> None:
    with wave.open(str(path), "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(RATE)
        w.writeframes(b"".join(struct.pack("<h", int(max(-1, min(1, s)) * 32000)) for s in samples))


def build_audio(tl: dict, events: list, music: bool) -> Path:
    """Mixes narration (at each scene's start), sound effects (at the page's
    events) and the bed into one 48 kHz stereo track, loudness-normalised."""
    sfx_dir = OUT / "sfx"
    sfx_dir.mkdir(exist_ok=True)
    kinds = {"blip": (0.25, 880), "boom": (1.6, 70), "alert": (0.7, 660), "swell": (2.0, 220)}
    for kind, (sec, f) in kinds.items():
        p = sfx_dir / f"{kind}.wav"
        if not p.exists():
            write_wav(p, tone(sec, f, kind))
    inputs, filters, labels = [], [], []
    k = 0

    def add(path: Path, at: float, gain_db: float, fade: float = 0):
        nonlocal k
        inputs.extend(["-i", str(path)])
        # a short fade either end of a voice clip: no click where the synthesis starts or stops mid-sample
        f = f"afade=t=in:d=0.03,afade=t=out:st={max(0, fade - 0.04):.3f}:d=0.04," if fade else ""
        filters.append(f"[{k}:a]aresample={RATE},aformat=channel_layouts=stereo,{f}volume={gain_db}dB,adelay={int(at * 1000)}|{int(at * 1000)}[a{k}]")
        labels.append(f"[a{k}]")
        k += 1

    for s in tl["scenes"]:
        add(voice.VOICE_DIR / s["clip"], s["start"] + s.get("lead", 0), 0, s["speech"])
    counts: dict = {}
    for e in events:
        kind = e.get("kind")
        if kind in kinds and counts.get(kind, 0) < 400:
            counts[kind] = counts.get(kind, 0) + 1
            add(sfx_dir / f"{kind}.wav", float(e["t"]), float(e.get("gain", -14)))
    if music:
        bed = OUT / "bed.wav"
        if not bed.exists() or abs(voice.duration(bed) - tl["total"]) > 1:
            write_wav(bed, music_bed(tl["total"] + 0.5))
        add(bed, 0, -9)
    mix = OUT / "mix.wav"
    graph = ";".join(filters) + f";{''.join(labels)}amix=inputs={k}:normalize=0:dropout_transition=0,loudnorm=I=-15:TP=-1.5:LRA=11[out]"
    subprocess.run(["ffmpeg", "-y", "-v", "error", *inputs, "-filter_complex", graph, "-map", "[out]", "-t", str(tl["total"]), str(mix)], check=True)
    return mix


# ---------- the film ----------

def mux(tl: dict, audio: Path, draft: bool) -> Path:
    out = OUT / ("batnet-draft.mp4" if draft else "batnet.mp4")
    subprocess.run(["ffmpeg", "-y", "-v", "error", "-framerate", str(tl["fps"]), "-i", str(OUT / "frames" / "%06d.png"),
                    "-i", str(audio), "-c:v", "libx264", "-preset", "slow" if not draft else "veryfast", "-crf", "18" if not draft else "23",
                    "-pix_fmt", "yuv420p", "-profile:v", "high", "-level", "4.1", "-movflags", "+faststart",
                    "-c:a", "aac", "-b:a", "192k", "-ar", "48000", "-shortest", str(out)], check=True)
    print(f"film: {out} ({out.stat().st_size / 1e6:.1f} MB, {tl['total']:.0f} s)")
    return out


# ---------- the README's preview: a few seconds of the film as a GIF, with a play button ----------

PREVIEW_CLIPS = [("berlin", .36), ("berlin2", .45), ("how", .25), ("radio", .72), ("cost", .55), ("questions", .4), ("glasto", .6), ("bang", .3), ("proof", .35), ("field", .8)]


def play_button_png(w: int, h: int, path: Path) -> None:
    """A transparent PNG with a dark disc and a white play triangle in the middle (pure Python: this
    ffmpeg has no drawtext, and we don't want a font or PIL dependency for one button)."""
    import zlib
    cx, cy, R = w / 2, h / 2, min(w, h) * .115
    rows = []
    for y in range(h):
        row = bytearray([0])
        for x in range(w):
            dx, dy = x - cx, y - cy
            d = (dx * dx + dy * dy) ** .5
            r, g, b, a = 0, 0, 0, 0
            if d < R:
                a = 175 if d < R - 3 else int(175 * (R - d) / 3)
                # the triangle: left edge at -R*.35, tip at +R*.5
                tx = dx + R * .08
                if -R * .38 <= tx <= R * .5 and abs(dy) <= (R * .5 - tx) * .72:
                    r, g, b, a = 255, 255, 255, 255
            row += bytes((r, g, b, a))
        rows.append(bytes(row))
    raw = b"".join(rows)
    def chunk(t, d): return len(d).to_bytes(4, "big") + t + d + zlib.crc32(t + d).to_bytes(4, "big")
    png = b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 6, 0, 0, 0)) + chunk(b"IDAT", zlib.compress(raw, 9)) + chunk(b"IEND", b"")
    path.write_bytes(png)


def preview_gif(film: Path, tl: dict) -> Path:
    """One second from each of a few scenes, 800 px wide at 10 fps, a play button on top.
    GitHub's README cannot embed a video from a branch, so this GIF links to the MP4."""
    out = OUT / "preview.gif"
    by = {s["id"]: s for s in tl["scenes"]}
    segs = [by[i]["start"] + (by[i]["end"] - by[i]["start"]) * at for i, at in PREVIEW_CLIPS if i in by]
    button = OUT / "play.png"
    if not button.exists():
        play_button_png(800, 450, button)
    parts = "".join(f"[0:v]trim=start={t:.2f}:duration=1,setpts=PTS-STARTPTS,scale=800:-2[c{k}];" for k, t in enumerate(segs))
    chain = "".join(f"[c{k}]" for k in range(len(segs)))
    graph = (f"{parts}{chain}concat=n={len(segs)}:v=1:a=0[v];[v][1:v]overlay=0:0,fps=10,split[s0][s1];"
             f"[s0]palettegen=max_colors=128:stats_mode=diff[p];[s1][p]paletteuse=dither=bayer:bayer_scale=5:diff_mode=rectangle")
    subprocess.run(["ffmpeg", "-y", "-v", "error", "-i", str(film), "-i", str(button), "-filter_complex", graph, "-loop", "0", str(out)], check=True)
    print(f"preview: {out} ({out.stat().st_size / 1e6:.1f} MB)")
    return out


def main() -> None:
    ap = argparse.ArgumentParser(description="Render the batnet film.")
    ap.add_argument("--draft", action="store_true", help="720p, 15 fps, the cheaper voice model")
    ap.add_argument("--mux", action="store_true", help="only mix the audio into frames already rendered")
    ap.add_argument("--no-music", action="store_true", help="narration and effects only")
    ap.add_argument("--preview", action="store_true", help="open the real-time preview (needs a timeline: run once first)")
    ap.add_argument("--gif", action="store_true", help="only remake the README's preview GIF from the rendered MP4")
    args = ap.parse_args()
    if args.preview:
        srv = serve(State())
        webbrowser.open(f"http://127.0.0.1:{PORT}/scenes.html")
        print(f"preview at http://127.0.0.1:{PORT}/scenes.html (Ctrl-C to stop)")
        try:
            while True:
                time.sleep(3600)
        except KeyboardInterrupt:
            srv.shutdown()
        return
    if args.gif:
        preview_gif(OUT / "batnet.mp4", json.loads((OUT / "timeline.json").read_text()))
        return
    if not (HERE / "assets" / "berlin-map.json").exists():
        subprocess.run([sys.executable, str(HERE / "fetch_map.py")], check=True)  # the Berlin scene's map, once
    tl = build_timeline(args.draft) if not args.mux else json.loads((OUT / "timeline.json").read_text())
    if args.mux:
        events = json.loads((OUT / "events.json").read_text()) if (OUT / "events.json").exists() else []
    else:
        events = render_frames(tl)
    audio = build_audio(tl, events, not args.no_music)
    film = mux(tl, audio, args.draft)
    if not args.draft:
        preview_gif(film, tl)


if __name__ == "__main__":
    main()
