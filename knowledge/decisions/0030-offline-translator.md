---
type: Decision
title: "0030: An offline translator on the hub (Argos models on CTranslate2)"
description: Messages in German (and later other languages) are translated to English on the hub itself, with no internet, by Argos Translate's free models run on CTranslate2 in a helper process; a Translate link under each message, a button to translate what you write, and an optional automatic mode.
tags: [hub, translation, offline, helper-process, accessibility]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-21T23:30:00Z }
---

# Context

Berlin's mesh speaks German and English. The owner asked for "a simple
translator, all offline of course, can be basic", German to English for a
start. The messages are short (a Meshtastic text is at most about 230 bytes).
batnetd is pure Go ([0005](/decisions/0005-pure-go-core.md)): heavy work runs
in helper processes.

# Decision

* **Argos Translate's models** (OpenNMT, one model per language pair, about
  100 MB, free licences) **run on CTranslate2 and SentencePiece** in a small
  Python helper, `batnet-translate`, in its own virtual environment
  (`/opt/batnet-translate`). Not the `argostranslate` package itself: it pulls in
  stanza and PyTorch, hundreds of MB, for sentence splitting a radio message
  doesn't need.
* batnetd starts the helper on the first translation, talks JSON lines over its
  stdin and stdout, and stops it after a few idle minutes ([0029](/decisions/0029-hub-power-modes.md)).
* **German↔English is installed by default**; other pairs are one command
  (`translate-setup.sh uk-en`) and appear by themselves.
* **On the panel:** a *Translate* link under each message the hub guesses is not
  in English (a word list, offline), the translation under the original, marked
  as a machine translation; a button that turns what you have written into
  German before you send it, for you to check; *Translate automatically* (off by
  default).

# Consequences

* 464 MB on the SD card, about 400 MB of memory with both directions loaded,
  and 0.5 to 1.1 s per message on a Pi 4 with two threads (the
  [translator test](/tests/2026-10-21-offline-translator.md)); the helper is
  stopped when idle to give the memory back.
* Machine translation of slang, abbreviations and typos is rough; the original
  stays on screen.
* A Pi Zero 2 W (512 MB) would struggle; the pocket node does without.
