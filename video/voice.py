#!/usr/bin/env python3
"""Narration for the batnet film: ElevenLabs text-to-speech, one clip per
scene, cached by what was said and how. Run through render.py; on its own:

    python3 video/voice.py            # speak every scene that isn't cached
    python3 video/voice.py --draft    # the cheaper Flash model, for previews

The key is read from $ELEVENLABS_API_KEY, else ~/.config/elevenlabs/api_key
(one line, chmod 600, never in the repo). Each clip costs about one credit per
character; the whole film is about 1,800 characters.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import subprocess
import sys
import urllib.request
from pathlib import Path

HERE = Path(__file__).resolve().parent
OUT = HERE / "out"
VOICE_DIR = OUT / "voice"


def api_key() -> str:
    k = os.environ.get("ELEVENLABS_API_KEY", "").strip()
    if not k:
        f = Path.home() / ".config" / "elevenlabs" / "api_key"
        if f.exists():
            k = f.read_text().strip()
    if not k:
        sys.exit("voice: no ElevenLabs key: export ELEVENLABS_API_KEY or write ~/.config/elevenlabs/api_key")
    return k


def load_script() -> dict:
    return json.loads((HERE / "script.json").read_text())


def clip_path(scene: dict, voice: dict, model: str, prev: str = "", nxt: str = "") -> Path:
    key = json.dumps({"t": scene["text"], "v": voice["voice_id"], "m": model, "s": voice.get("settings", {}), "p": prev, "n": nxt}, sort_keys=True)
    return VOICE_DIR / f"{scene['id']}-{hashlib.sha1(key.encode()).hexdigest()[:10]}.mp3"


def duration(path: Path) -> float:
    out = subprocess.check_output(["ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", str(path)])
    return float(out.decode().strip())


def tail_db(path: Path, seconds: float = 0.12) -> float:
    """The loudest sample in the clip's last `seconds`, in dBFS: a clip whose voice is still sounding
    at the very end was cut short by the synthesis (cut 2's Berlin line ended at -33 dB)."""
    d = duration(path)
    out = subprocess.run(["ffmpeg", "-v", "info", "-ss", str(max(0, d - seconds)), "-t", str(seconds), "-i", str(path),
                          "-af", "volumedetect", "-f", "null", "-"], capture_output=True, text=True).stderr
    m = re.search(r"max_volume: ([-\d.]+) dB", out)
    return float(m.group(1)) if m else -99.0


def speak(scene: dict, voice: dict, model: str, key: str, prev: str = "", nxt: str = "") -> Path:
    """One clip for the scene. `prev` and `nxt` are the neighbouring scenes' lines: ElevenLabs uses
    it for the flow of the voice across clips (previous_text), so a scene does not start on a
    sudden new pitch. It is part of the cache key, so a re-written neighbour re-speaks."""
    path = clip_path(scene, voice, model, prev, nxt)
    if path.exists():
        return path
    VOICE_DIR.mkdir(parents=True, exist_ok=True)
    body = {"text": scene["text"], "model_id": model, "voice_settings": voice.get("settings", {})}
    if prev:
        body["previous_text"] = prev
    if nxt:
        body["next_text"] = nxt
    for attempt in range(3):
        req = urllib.request.Request(
            f"https://api.elevenlabs.io/v1/text-to-speech/{voice['voice_id']}?output_format=mp3_44100_128",
            data=json.dumps(body).encode(), method="POST",
            headers={"xi-api-key": key, "Content-Type": "application/json", "Accept": "audio/mpeg"})
        with urllib.request.urlopen(req, timeout=120) as r:
            audio = r.read()
            cost = r.headers.get("character-cost", "?")
        path.write_bytes(audio)
        tail = tail_db(path)
        print(f"voice: {scene['id']:11s} {len(scene['text']):4d} chars, cost {cost}, {duration(path):.1f} s, tail {tail:.0f} dB")
        if tail < -40:  # the last word has died away: not cut short
            break
        print(f"voice: {scene['id']} ends while still sounding ({tail:.0f} dB): spoken again")
    return path


def neighbours(scenes: list) -> list:
    """Each scene with the text spoken before it (and after it: none. Given the next line, the voice
    runs its last word to the clip's very end with no decay, every time: the clip then sounds cut.)"""
    return [(s, scenes[i - 1]["text"] if i else "", "") for i, s in enumerate(scenes)]


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--draft", action="store_true", help="use the cheaper, faster draft model")
    args = ap.parse_args()
    sc = load_script()
    voice = sc["voice"]
    model = voice["draft_model_id"] if args.draft else voice["model_id"]
    key = api_key()
    for scene, prev, nxt in neighbours(sc["scenes"]):
        speak(scene, voice, model, key, prev, nxt)


if __name__ == "__main__":
    main()
