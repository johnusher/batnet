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


def clip_path(scene: dict, voice: dict, model: str) -> Path:
    key = json.dumps({"t": scene["text"], "v": voice["voice_id"], "m": model, "s": voice.get("settings", {})}, sort_keys=True)
    return VOICE_DIR / f"{scene['id']}-{hashlib.sha1(key.encode()).hexdigest()[:10]}.mp3"


def duration(path: Path) -> float:
    out = subprocess.check_output(["ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", str(path)])
    return float(out.decode().strip())


def speak(scene: dict, voice: dict, model: str, key: str) -> Path:
    path = clip_path(scene, voice, model)
    if path.exists():
        return path
    VOICE_DIR.mkdir(parents=True, exist_ok=True)
    body = json.dumps({"text": scene["text"], "model_id": model, "voice_settings": voice.get("settings", {})}).encode()
    req = urllib.request.Request(
        f"https://api.elevenlabs.io/v1/text-to-speech/{voice['voice_id']}?output_format=mp3_44100_128",
        data=body, method="POST",
        headers={"xi-api-key": key, "Content-Type": "application/json", "Accept": "audio/mpeg"})
    with urllib.request.urlopen(req, timeout=120) as r:
        audio = r.read()
        cost = r.headers.get("character-cost", "?")
    path.write_bytes(audio)
    print(f"voice: {scene['id']:11s} {len(scene['text']):4d} chars, cost {cost}, {duration(path):.1f} s")
    return path


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--draft", action="store_true", help="use the cheaper, faster draft model")
    args = ap.parse_args()
    sc = load_script()
    voice = sc["voice"]
    model = voice["draft_model_id"] if args.draft else voice["model_id"]
    key = api_key()
    for scene in sc["scenes"]:
        speak(scene, voice, model, key)


if __name__ == "__main__":
    main()
