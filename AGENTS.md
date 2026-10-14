# AGENTS.md — batnet

Instructions for coding agents (Claude Code, Codex, others) working in this repo.

## What this is

batnet (working name) turns Raspberry Pis into grid-down community comms: a Pi
per house with an open hotspot and a WhatsApp-like web app, houses linked by a
batman-adv WiFi mesh and by Meshtastic LoRa. Start with
[`knowledge/vision.md`](knowledge/vision.md). The project is in the **design
phase**: no product code yet; specs go in `docs/specs/`.

## Where things are

| Path | What |
|---|---|
| `knowledge/` | The knowledge bundle (OKF v0.2): vision, decisions, research, hardware, security, skills. Read its `index.md` first. |
| `docs/specs/` | Design specs, one per milestone, written after the design is agreed |
| `.claude/skills/` | Agent skills (`.agents/skills` points here). Catalogue: `knowledge/skills/catalog.md` |
| `tools/okflint/` | Checks the knowledge bundle (`cd tools/okflint && go run . ../../knowledge`) |
| `.githooks/` | Pre-commit: gitleaks + okflint. Enable once per clone: `git config core.hooksPath .githooks` |

## Rules

1. **Knowledge goes in `knowledge/`, in OKF form.** Use the `capturing-knowledge`
   skill for any research finding, decision, hardware note or test result.
   Don't leave findings only in chat or commit messages.
2. **Nothing secret or personal is committed.** This repo is public. See
   `knowledge/security/secret-hygiene.md`. Never commit keys, PSKs, passwords,
   real names, precise locations, node IDs, MACs or private hostnames — use
   placeholders (`Anna · Miller House`, `!deadbeef`, `anna@example.org`).
   Never bypass the pre-commit hook (`--no-verify`).
3. **Decisions are records.** A change of direction gets a new decision that
   supersedes the old one (`status: deprecated`, linked) — never a silent edit.
4. **Go, pure Go for batnetd** (`CGO_ENABLED=0`); heavy work (speech, codecs,
   SDR) runs in helper processes. See decision 0005.
5. **Radio rules matter.** Anything that transmits or scans must respect EU868
   duty cycle and power limits and the activity-only scanner rule (decision 0009).
