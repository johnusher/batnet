---
name: making-the-film
description: Use when editing or re-rendering the batnet film (video/): changing the narration, a scene's animation, the voice, the sounds, or publishing a new cut to the GitHub release. Covers the pipeline (ElevenLabs voice, canvas scenes in headless Chrome, ffmpeg), what to check before a render, and the credibility rules for what the film claims.
---

# Making the batnet film

The film is code: `video/script.json` (what the narrator says, per scene),
`video/scenes.js` (what is drawn, per scene, as a pure function of time) and
`video/render.py` (voice, timeline, frames, sound, MP4). Nothing is edited in a
video editor. Read `video/README.md` first.

## The loop

1. **Change the words** in `script.json`, or **a scene** in `scenes.js`
   (one object per scene id: `setup()` builds deterministic data and pushes
   sound events; `draw(u, len)` draws second `u` of the scene).
2. **Draft**: `python3 video/render.py --draft` (720p, 15 fps, the cheap Flash
   voice; a few minutes). Look at frames in `video/out/frames/` with the Read
   tool before trusting the MP4: pick one frame per scene plus the moments
   that matter (a bubble, the bang, the dust front).
3. **Preview** in a browser with sound: `python3 video/render.py --preview`.
4. **Final**: `python3 video/render.py` (1080p, 30 fps, Multilingual v2).
5. **Publish**: the MP4 lives on the `film` branch of the repo (one file,
   no history worth keeping: `git checkout film && cp video/out/batnet.mp4 . &&
   git commit -am "cut N" && git push`), linked from the README as
   `https://github.com/johnusher/batnet/raw/film/batnet.mp4`. Never on `main`.
   A GitHub release (`gh release create film-vN video/out/batnet.mp4`) is the
   better home once the `gh` token has "Contents: write" on the repo; the
   fine-grained token of 2026-10-15 got HTTP 403 creating one.

## Rules

* **Credible**: every number on screen comes from the repo or a public
  record, and the knowledge bundle or the scene comment says where. The Berlin
  figures (8 January 2025, Treptow-Köpenick, about 50,000 homes, up to three
  days) are the public reporting of the arson attack; the simulator figures
  (247 posts, 8,968 deliveries, 12 ms median, 0 missing) are `bang.json`'s
  report on its CI seed; re-run `go run ./cmd/batnetd -sim scenarios/bang.json`
  before changing them. Crowd sizes: Glastonbury about 210,000, Black Rock
  City about 80,000. No made-up test results, no promised features shown as
  working: what isn't built yet is said to be next.
* **The narrator** is Klaus (ElevenLabs library voice, thick German accent,
  on purpose). The phonetic spellings in `script.json` ("ze", "ven", "haff")
  are the accent; keep the jokes light and few (one WW2 allusion per minute
  at most, never about victims, uniforms or the regime). British English
  otherwise. Numbers are spelled out in words so the voice reads them well.
* **Captions** are the narration text, chunked by sentence (`chunks()` in
  `scenes.js`); keep sentences under about 90 characters or the caption wraps
  to three lines.
* **Scene length** follows the clip: `min` and `pad` in `script.json`. A scene's
  animation must look right for any length a little above `min`: tie
  moments to seconds from the scene start, not to its end, except the fades.
* **Determinism**: every random choice goes through `rng(seed)`. The render
  pass and the sound events are computed from the same data, so a re-render
  is identical. Never use `Math.random()` except for the one-frame shake.
* **Secrets and people**: the ElevenLabs key stays in
  `~/.config/elevenlabs/api_key`; `video/out/` is ignored; no real names or
  precise places beyond the public ones above.
* **Cost**: one full voice pass is about 1,800 characters (about 800 credits
  on Multilingual v2, half that on Flash); clips are cached by text and voice,
  so only changed scenes are re-spoken.

## When something is off

* A scene draws nothing or the render stops: run the preview, open the
  browser console; in render mode errors are posted to the terminal as
  `page: ERROR …`.
* Frames render but the MP4 has no sound: check `video/out/mix.wav` exists
  and `events.json` is a list; `render.py --mux` re-mixes without re-rendering.
* Captions out of step with the voice: the words are spread evenly over the
  clip; a long pause in the voice shows as captions running ahead. Split
  the sentence or add a comma.
