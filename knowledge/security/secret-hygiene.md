---
type: Policy
title: Secret hygiene for a public repo
description: What must never be committed, where it lives instead, and the checks that block it.
tags: [security, secrets, privacy, public-repo]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
---

# Never committed

| Kind | Examples | Lives instead in |
|---|---|---|
| Keys | village key, Meshtastic PSKs, node ed25519 keys, TLS private keys | generated on each node at install, `/etc/batnet/secrets/` (0600) |
| Credentials | admin passwords, WiFi passwords, SSH keys, API tokens | the node, or the operator's password manager |
| Personal data | real names, phone numbers, emails, home addresses | nowhere in the repo; use "Anna · Miller House", `anna@example.org` |
| Location | GPS coordinates finer than ~1 km, photos with EXIF | `private/` (gitignored) or a separate private repo |
| Device identifiers | Meshtastic node IDs (`!xxxxxxxx`), MAC addresses, serials, callsigns | `private/`; docs use `!deadbeef`, `aa:bb:cc:dd:ee:ff` |
| Private infrastructure | home hostnames, SSH aliases, tunnel URLs, IPs of personal servers | the operator's own machine config |
| Raw field data | SDR captures, audio, logs with any of the above | `private/` or `field/raw/` (gitignored) |

# How it is enforced

1. **Config split.** `batnet.example.yaml` is committed; the real `batnet.yaml` and
   any `*.local.*` file are gitignored. Code reads secrets from `/etc/batnet/secrets/`, never from the config file.
2. **Generated, not shared.** The installer generates keys on the node. The village
   key moves between houses by QR code on screen, never through the repo.
3. **Pre-commit hook** (`.githooks/pre-commit`, enabled with
   `git config core.hooksPath .githooks`): gitleaks with the default rules plus
   batnet rules (PSKs, node IDs, precise coordinates, MACs, emails) in `.gitleaks.toml`.
4. **CI** runs the same gitleaks scan on every push; GitHub secret scanning and
   push protection are switched on when the repo goes public.
5. **Before going public**, scan the full history: `gitleaks git --log-opts=--all`.
   History is forever: a secret found there means rotating it and rewriting history before publishing.
6. **Agent notes stay out.** Session memory and personal instructions live outside the repo (`~/.claude`); the repo's `AGENTS.md` holds project rules only.

# If something leaks

Treat it as compromised: rotate it (new village key, new PSK, new password) first,
then remove it from history. Deleting the file in a new commit is not enough.
