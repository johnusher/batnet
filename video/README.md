# The batnet film

A five-and-a-half-minute animated explainer: a Berlin blackout on the real map, how a house node works,
why it has two radios (and how a voice becomes text for the slow one), what it
costs, the questions people ask, Glastonbury (a lost child, a bang that
everyone reports), the simulator that runs the real code, and the first two
real hubs on the air in Berlin (what they heard, on the whole city's map). Narrated
by Klaus, whose German accent is part of the act, with a few lines borrowed
from 'Allo 'Allo! and Dad's Army.

The film is code, so it can be re-cut: the words live in `script.json`, the
pictures in `scenes.js`, and `render.py` turns them into an MP4 for YouTube.

```
python3 video/render.py --draft    # 720p, quick: a look at a change
python3 video/render.py --preview  # real-time preview with sound in a browser
python3 video/render.py            # the final 1080p MP4 at video/out/batnet.mp4
```

What it needs: Python 3, ffmpeg, Google Chrome, and an ElevenLabs key in
`~/.config/elevenlabs/api_key` (or `$ELEVENLABS_API_KEY`). Everything under
`video/out/` is a cache (voice clips, frames, the mix, the film) and is not
committed.

## How it is made

| Step | Where | What |
|---|---|---|
| Narration | `voice.py` | ElevenLabs text-to-speech, one clip per scene, spoken with the previous scene's line as context, cached by text, voice settings and that context; a clip whose voice is still sounding at its very end is spoken again |
| Timeline | `render.py` | each scene lasts a short lead (the picture first), its clip, then `pad` seconds, at least `min` |
| Map | `fetch_map.py`, once | OpenStreetMap tiles of Berlin and the outlines of the dark localities, into gitignored `assets/berlin*` (the renderer runs it when they are missing) |
| Frames | `scenes.js` in headless Chrome | a canvas draws second `t` of the film; a scene's moments are keyed to phrases of its narration (`cue()`); every frame is posted to a local server |
| Sound | `render.py` | synthesised blips, a boom, an alert chime and a quiet synth bed, placed at the events the scenes report; narration after each scene's lead, with a short fade either end; loudness-normalised |
| Film | ffmpeg | H.264 high profile, AAC 48 kHz, faststart: what YouTube wants |

The current cut is on YouTube: see the README's "The film".
To edit it, the agent skill `.claude/skills/making-the-film/` has the loop and
the rules (every number on screen must be true and traceable).

## Assets

`assets/` holds three screenshots of the real app and the simulator, copied
from `docs/img/`, so the phone in the film shows the actual app.
