---
type: Issue List
title: Ideas
description: Features to consider, from the 2026-10-22 review - nicer to use, more settings, village and radio tools, power, and offline AI hooks; not yet decided or ordered.
tags: [ideas, features, backlog, review]
status: draft
generated: { by: claude-code/claude-fable-5-1, at: 2026-10-22T22:30:00Z }
---

What could come after milestone 1, gathered in the 2026-10-22 review. Nothing
here is decided: the owner picks, and a pick becomes a line in [to do](todo.md)
(or a decision). The seven features already listed under *Features to consider*
in [to do](todo.md) (unread counts on the phone, delivered to N of M houses, reach
chips, admin health warnings, a Low batch over LoRa, scenario checks, change PIN)
are not repeated here. Bugs and limits live in [known problems](known-problems.md).

# Nicer to use: the house app

- [ ] **Roll call.** An admin asks "Everyone OK?"; each person taps *OK* or *Need help* (one tiny entry each); the People tab shows who answered, who has not, by house. The single most useful thing in a real power cut.
- [ ] **Quick replies and templates.** One tap for "I'm OK", "Need help at <house>", "Have spare: …", "Water point open": short, consistent, cheap on LoRa, easy to translate.
- [ ] **Pinned notices per group and for everyone.** An admin pins a message ("Pharmacy open 9-11"); it stays on top, and is what the notice-board screen ([0016](/decisions/0016-no-phone-mode.md)) rotates.
- [ ] **Reply with a quote, copy, and search** in a conversation; a day separator and *jump to today*.
- [ ] **Reactions** (👍 ✅ ❤) as tiny entries, batched over LoRa so they cost almost nothing, or WiFi-only.
- [ ] **Night theme** for bedside mode (dim, red-shifted, large), and a high-contrast theme.
- [ ] **A printable village sheet** from the app's help: this house, the hub's name, the channel, how to join, who the admins are. Paper works in a blackout.

# Nicer to use: the hub panel

- [ ] **Nicknames and notes on radios.** Call `!a1b2c3d4` "Tom, roof at the school" on this hub only; search by it.
- [ ] **Watch a radio.** Chime or blink when a chosen radio is heard again, or has been silent for N hours.
- [ ] **Tell the owner's phone about a private message**, wherever the phone is: a push through ntfy (self-hosted or ntfy.sh) with the sender's name only, never the words; the hub needs the internet for it. The away note ([0033](/decisions/0033-away-note.md)) holds the door meanwhile.
- [ ] **Statistics from the log.** Radios per day, the busiest hours, how the coverage zone changes over days: the log already has it all; draw it offline.
- [ ] **Replay a night.** Scrub through the log on the map: who was heard when, with what signal.
- [ ] **Export.** The radios heard as GPX or GeoJSON, a log as CSV, the coverage zone as a picture.
- [ ] **German panel.** The app speaks German; the panel only English.
- [ ] **A QR code for the house app's address**, next to the WiFi one, on the Home view.
- [ ] **Night watch mode** for the hub's screen: only new messages, large, dim; a key brings the panel back.
- [ ] **Touchscreen ergonomics**: an on-screen keyboard for the official display, bigger targets in the message box.

# Settings to add

- [ ] **Quiet hours for the hub's sound and LED** (the app has them per person; the hub chimes all night).
- [ ] **Per-conversation sound** on the panel: BerlinMesh chatter silent, direct messages loud.
- [ ] **Hellos by time of day**: never between 23:00 and 07:00.
- [ ] **Logging on battery**: start the log by itself when the mains goes; keep logs for N days or X MB.
- [ ] **Fade old radios after N hours** on the map (6 h is fixed today) and a toggle for the coverage zone.
- [ ] **Trusted networks** for PIN-free reading of the panel (the home LAN by name or subnet), everything else asks; fixes part of B52 and B54.
- [ ] **Change the hub PIN** from the screen; show PINs only after a button press (L14).
- [ ] **A read-only guest PIN** for the panel, so a neighbour can watch the map without being able to send as the hub or restart it.
- [ ] **Power rules**: switch to Low below a battery level once a HAT reports one; screen off at night; the hub's lights off.
- [ ] **Clock, units, language**: 12/24 h, km or miles, the panel's language.

# Admin and village

- [ ] **Backup to a USB stick** from the panel: the house log, keys and settings, encrypted with the admin PIN; and restore on a fresh Pi. Grab-and-go for a long outage.
- [ ] **Rename, merge and archive groups**; an admin's note on a person ("GP, house 12").
- [ ] **Delegate**: a second admin per house, so one phone's battery does not decide.
- [ ] **A chain of trust for the village.** Meshtastic gives one shared channel key and nothing above it ([research](research/meshtastic-security.md)): anyone holding the village key is anyone (B51). A decision on the admin's key as the root, houses admitted by its signature, people vouched for by their house, a lost phone or a stolen Pi revoked without re-keying the village, and person-to-person secrecy inside the village; the pattern to copy is Meshtastic's own direct messages and admin keys (a key pair per node, the public key announced, a shared secret per pair, admins as a short list of public keys).
- [ ] **A daily mesh health note** on Home, worked out from the log: "Last night: 154 radios, 7 % unreadable, most within 3 minutes of a hello" (what found B47 and B48 by hand).

# Radio and mesh

- [ ] **Range-test mode.** A walker's radio shares its position; the hub pairs each packet's place and signal and grows the coverage zone from measurements, not estimates (the bike ride did this by hand).
- [ ] **New neighbour found.** The first time a `batnet`-named radio is heard: a chime and a card, with *write to them*.
- [ ] **Mailbox for radios that were off.** A radio that comes back asks the hub for what it missed on the public channel (Meshtastic's store-and-forward does this; the hub has the messages). Airtime manners apply.
- [ ] **Official warnings for the hub's own area.** While the hub has the internet it fetches the national warnings (NINA from the BBK, warnung.bund.de, and DWD weather) for its own district only, keeps them, and shows them in the house app and the panel; when the internet goes, the last ones stay. Nothing on the air. Suggested 2026-10-25, after a bot's *Warnmeldung* channel on the Berlin mesh was asked, a second time, to stop posting warnings for all of Germany; such a bot needs the internet too, so it goes quiet in a wide blackout.
- [ ] **The queue and the duty meter on the panel** once the LoRa bridge is wired (plan 3b): what is waiting, what the hour's budget has left.
- [x] **Which way a radio is, from who passes it on.** Built 2026-10-25 as the map's *Guess where the others are*, for the hub's owner only (the hub PIN, even at home; the owner chose that over leaving rough areas alone): [test](/tests/2026-10-25-guessing-where-radios-are.md). Next: a second hub's sightings, and traceroutes or neighbour lists heard on the air, which name direct links.
- [ ] **A second radio stick as a roving listener** stepping through the EU presets (in [to do](todo.md)), drawn beside the main one.

# Power and hardware

- [ ] **A battery HAT that reports its charge** (in [to do](todo.md)) read over I2C in pure Go; the power rules above hang off it.
- [ ] **A USB GPS dongle as the clock** after a power cut (B39): read NMEA straight in Go, no gpsd; a position for the hub's own place too.
- [ ] **An e-paper display** for the lowest power: the last message and the radios heard, updated a few times an hour, readable in sunlight, no backlight.
- [ ] **Button gestures**: short press hello, long press own WiFi, very long press switch off; a buzzer pattern per message kind.

# AI and language hooks, offline first

The pattern exists already: the translator is a helper process (JSON lines over
stdin and stdout, started on first use, stopped when idle,
[0030](/decisions/0030-offline-translator.md)). A small language model fits the
same shape. On a Pi 4 a 0.5 to 1.5 billion parameter instruct model (Qwen 2.5,
Gemma 3 1B, quantised, on llama.cpp) runs at a few words a second and needs about
1 GB of memory: fine for a summary a few times a day, not for the Zero 2 W. Every
hook below is off by default and never sends a private message anywhere.

- [ ] **What happened while I slept.** A paragraph on Home from the night's log and messages: how many radios, what people talked about on the public channel, anything that looked urgent. Local model; a plain template version first, the model only polishes.
- [ ] **Make it fit.** A button under the message box: shorten what I wrote to one LoRa packet (about 200 bytes) in the same language, for me to check. Local model.
- [ ] **Urgent or not.** A tiny classifier (a word list first, a small model later) marks messages that ask for help, so the Home view and the LED show them first and the roll call picks them up. No generation, cheap, runs on anything.
- [ ] **Ask the help.** "How do I set the clock?" answered from the panel's own help and this knowledge bundle, offline: plain text search (BM25) first, a local model to phrase the answer only where it helps.
- [ ] **Voice to text and back** are already decided for milestone 2 ([0014](/decisions/0014-speech-pipeline.md): Moonshine on the Pi, the phone's own voice; Piper as the fallback).
- [ ] **A better model when the internet is there.** In normal times a cloud model (the Claude API, say) can do the same four jobs better, behind one setting, *Use the internet for AI when there is some*, off by default, with the public channel only and never direct messages. Grid-down there is no internet, so this is for the quiet months, not the emergency.
- [ ] **A hub assistant** on the panel: plain language to a fixed list of actions ("start the log", "switch to MediumFast"), each shown as the button it would press and confirmed before it runs. Never free-form commands; the polkit rule stays the boundary ([0031](/decisions/0031-root-actions-through-polkit.md)).
- [ ] **For agents and scripts**: name the panel's JSON API as stable and document it, and a `batnetctl` on the laptop (status, traffic, the log, send) so a developer, or a coding agent, can work the bench hub without the browser; the test write-ups in `tests/` would come straight from it.

# Developer and operations

- [ ] **CI**: gitleaks (a deferred minor since plan 1), `go test -race`, and a nightly run of the fuzz targets added in the 2026-10-22 review.
- [ ] **A self-test** from the panel: the speaker, the LED, the radio, the card's free space, the clock, each with a tick or a cross.
- [ ] **Update from the panel** when the internet is there: fetch a signed release (plan 6), verify, swap, restart; show what changed.
- [ ] **Diagnostics in the panel**, for a hub with no screen: fixed buttons behind the hub PIN, never a shell (the panel has no TLS): the last hour of the log, warnings since the start, disk and memory, restart the radio or the hub, and a USB reset of the radio stick (the only cure for [B74](/known-problems.md)). Suggested 2026-10-25.
- [ ] **The radio's shared position from the panel**: *where the hub stands* in Settings also sets the radio's fixed position (an admin `set_fixed_position`), with a choice of how rough it goes on the air (channel position precision: about 364 m, 730 m, 1.5 km or 2.9 km). Today that takes the command-line tool with batnetd stopped. Asked for 2026-10-25.
- [ ] **Crash notes**: a panic's trace saved in the log folder and shown under System after a restart.
