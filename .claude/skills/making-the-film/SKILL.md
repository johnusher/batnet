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
5. **Publish**: the film lives on YouTube only (the owner, 2026-10-15: no
   large media in the repo, one branch). Upload the new cut by hand to the
   owner's channel (or with `video/upload_youtube.py` once its OAuth consent
   can reach that channel; see below), paste the description the script
   prints (title, chapters from the timeline, tags), then put the new video
   id in the README (the heading link, the thumbnail
   `https://img.youtube.com/vi/<id>/maxresdefault.jpg`, and the link in
   "The film") and in this file. `video/out/` is gitignored: the MP4 and the
   preview GIF stay local. History note: cut 1 sat on a `film` branch
   (deleted) and cut 2 briefly on `main`; both were taken out again.
   `upload_youtube.py` needs a consent through the Google OAuth client in
   `~/.config/google/`, whose consent screen is Internal to the Workspace
   account, not the personal channel; until that is changed, upload by hand
   and print the description with
   `python3 -c "import sys; sys.path.insert(0,'video'); import upload_youtube as u; print(u.DESCRIPTION.replace('{chapters}', u.chapters()))"`.
   The current video: https://youtu.be/XyFXxIppOpI (cut 3, uploaded by hand
   2026-10-25: the field scene, the measured phones and power, the voice fixes
   below). Cut 2 was https://youtu.be/zk5At_n8p_c (2026-10-15). The title,
   description and tags live in `upload_youtube.py`; they name LoRa, the
   kilometres and "no app to install" so the film is found by people looking
   for something longer-ranged than a phone-to-phone Bluetooth app.

## Rules

* **Credible**: every number on screen comes from the repo or a public
  record, and the knowledge bundle or the scene comment says where. The
  field scene (cut 3) draws on the test write-ups: 154 radios in one night
  across 29 by 20 km (`knowledge/tests/2026-10-22-one-night-on-mediumfast.md`),
  12 km read straight and nothing indoors against 26 near the roof
  (`knowledge/hardware/bench-pi7.md`), the farthest radio *heard* about 25 km
  away through two relays (`bench-pz7.md`; the reach test had not reached it
  when cut 3 was made: say *heard*, not *reached*, until a report says so),
  200 phones at once (`knowledge/tests/2026-10-23-hub-load-test.md`), the
  Pi 4 hub's 15 h 48 min on a 72 Wh bank. Its dots are a count: it never
  shows where a radio or a hub is. The Berlin
  figures (the night of 8/9 September 2025, two 110 kV pylons in Johannisthal,
  about 50,000 customers, about 60 hours; the six localities drawn dark) are
  the public reporting of the arson attack, in
  `knowledge/research/berlin-blackout-2025.md` (cut 1 wrongly said January
  and a cable tunnel: check a date before narrating it); the simulator figures
  (247 posts, 8,968 deliveries, 12 ms median, 0 missing) are `bang.json`'s
  report on its CI seed; re-run `go run ./cmd/batnetd -sim scenarios/bang.json`
  before changing them. Crowd sizes: Glastonbury about 210,000, Black Rock
  City about 80,000. No made-up test results, no promised features shown as
  working: what isn't built yet is said to be next.
* **The voice, joined up** (cut 3): each clip is spoken with the previous
  scene's line as `previous_text`, so a scene does not start on a sudden new
  pitch; never pass `next_text` (given the next line the voice runs its last
  word to the clip's very end, every time, and it sounds cut). `voice.py`
  checks a clip's last 120 ms and speaks it again if the voice is still
  sounding there (`tail_db`). The narrator starts `LEAD` (0.45 s) after the
  cut (`render.py`; the timeline's `lead`, which captions and cues follow),
  and each clip gets a 30 ms fade either end against clicks. Cut 2's
  narration sat on the cuts at full level, which played as sudden edits.
* **The narrator** is Klaus (ElevenLabs library voice, thick German accent,
  on purpose). The phonetic spellings in `script.json` are the accent, and
  the owner wants it thick (cut 2, 2026-10-15): th becomes z or s ("ze",
  "zis", "sousand", "sree", "nossing"), w becomes v ("ven", "vay", "netvork"),
  v becomes w ("willage", "ewery", "deliwered"), and becomes "unt", j becomes
  y ("Yanuary"), "WiFi" is "Vee-Fee", plus a "Ja" now and then. The caption
  stays plain English. The gags (cut 2): one line each from 'Allo 'Allo!
  (Michelle's "Listen very carefully", Madame Fanny's "flashing knobs",
  Crabtree's "Good moaning") and Dad's Army (Hodges' "Put that light out",
  "Don't tell him, Pike", Frazer's "We're doomed", Jones' "Don't panic",
  Wilson's "Do you think that's wise, sir?", Mainwaring's "Stupid boy"), each
  where the picture gives it a reason; keep the jokes light (one WW2 allusion per minute
  at most, never about victims, uniforms or the regime). British English
  otherwise. Numbers are spelled out in words so the voice reads them well.
* **Captions** are the narration text, chunked by sentence (`chunks()` in
  `scenes.js`); keep sentences under about 90 characters or the caption wraps
  to three lines.
* **Scene length** follows the clip: `min` and `pad` in `script.json`. A scene's
  animation must look right for any length a little above `min`: tie
  moments to seconds from the scene start, not to its end, except the fades.
* **Moments follow the words**: `cue(id, phrase)` in `scenes.js` gives the
  second at which the narrator reaches a phrase of the scene's `text` (the
  words are spread evenly over the clip, like the captions). Key a scene's
  moments to cues, computed once in `setup()`, so a re-written line does not
  leave the picture out of step. A phrase quoted in a cue must stay in the
  text; a missing one logs `no cue` and lands at 0 s.
* **Determinism**: every random choice goes through `rng(seed)`. The render
  pass and the sound events are computed from the same data, so a re-render
  is identical. Never use `Math.random()` except for the one-frame shake.
* **Secrets and people**: the ElevenLabs key stays in
  `~/.config/elevenlabs/api_key`; `video/out/` is ignored; no real names or
  precise places beyond the public ones above. The Berlin scene's map (tiles
  and outlines, OpenStreetMap) is fetched by `video/fetch_map.py` on the
  first render and stays out of the repo: gitleaks' `precise-coordinates`
  rule refuses it, rightly. Any new coordinate in the sources is rounded to
  two decimals (about a kilometre).
* **Cost**: one full voice pass is about 4,600 characters (about as many
  credits on Multilingual v2, half that on Flash); clips are cached by text,
  voice and the previous line, so a changed scene re-speaks itself and the
  one after it.

## When something is off

* A scene draws nothing or the render stops: run the preview, open the
  browser console; in render mode errors are posted to the terminal as
  `page: ERROR …`.
* "Chrome exited before the frames were done" at frame 0: a headless Chrome
  from a killed render is still running (`pgrep -fl chrome-profile`); kill
  it. Since cut 2 each run has its own profile directory, so this should not
  recur, but a killed render can still leave a Chrome behind.
* The render was killed part-way: the voice clips are cached, so the
  re-run costs no credits for unchanged lines.
* Frames render but the MP4 has no sound: check `video/out/mix.wav` exists
  and `events.json` is a list; `render.py --mux` re-mixes without re-rendering.
* Captions out of step with the voice: the words are spread evenly over the
  clip; a long pause in the voice shows as captions running ahead. Split
  the sentence or add a comma.
