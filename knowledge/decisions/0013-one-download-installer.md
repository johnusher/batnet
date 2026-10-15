---
type: Decision
title: "0013: One download to install (draft)"
description: People get batnet as one zip from a fixed link, no GitHub account needed; scripts inside open Raspberry Pi Imager on the batnet image. A one-line install covers Pis that already run Raspberry Pi OS.
tags: [installer, setup, release, usability]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T05:46:54Z }
---

# Context

The README is for the general public: most readers have never used GitHub or a
Raspberry Pi (user, 2026-10-15). Cloning a repo or typing commands on a Pi loses
them. Writing an SD card by hand (dd, picking a disk) can wipe the wrong drive.

# Decision (proposed)

* **Path A, download and install** (the default in the README): a release asset
  at a link that never changes,
  `https://github.com/johnusher/batnet/releases/latest/download/batnet.zip`.
  Downloading a release needs no GitHub account. Inside:
  * `batnet.img.xz`: Raspberry Pi OS Lite with batnet installed, arm64 (Zero 2 W, 3, 4, 5).
  * `Install on Windows.bat`, `Install on Mac.command`, `install-linux.sh`:
    each checks for Raspberry Pi Imager (offers its download page if missing)
    and opens it with the batnet image already chosen, so the person only picks
    the memory card in Imager's own window. No disk is written by our scripts.
  * `READ ME FIRST.txt`: the same steps as the README, in plain words.
* **Path B, GitHub or a Pi you already run**: on Raspberry Pi OS, one command
  fetches and runs `install.sh` from the latest release; developers clone the repo.
* No questions at install time. The house name, the first admin and the village
  key are set on the phone set-up page at first boot.

# Open

* Verify Raspberry Pi Imager's command-line option for a custom image list
  (`--repo`) on Windows, macOS and Linux.
* macOS Gatekeeper and Windows SmartScreen warnings on unsigned scripts: write
  the README steps for "open anyway".
* Image size and hosting limits for release assets.
* Sharing the village key between houses at set-up without typing a long key
  (QR code from the first house's admin screen?).
