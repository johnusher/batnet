# AGENTS.md — batnet

Instructions for coding agents (Claude Code, Codex, others) working in this repo.

## What this is

batnet (working name) turns Raspberry Pis into grid-down community comms: a Pi
per house with an open hotspot and a WhatsApp-like web app, houses linked by a
batman-adv WiFi mesh and by Meshtastic LoRa. Start with
[`knowledge/vision.md`](knowledge/vision.md). The milestone 1 design is approved ([spec](docs/specs/2026-10-15-m1-design.md));
implementation follows the plans in `docs/plans/`.

## Where things are

| Path | What |
|---|---|
| `knowledge/` | The knowledge bundle (OKF v0.2): vision, decisions, research, hardware, security, skills. Read its `index.md` first. |
| `prototypes/` | Throwaway design prototypes. `village-sim/`: the in-browser village model + web app mockup (`sim.js` runs in node too) |
| `docs/specs/` | Design specs, one per milestone, written after the design is agreed. Milestone 1: `2026-10-15-m1-design.md` (approved) |
| `docs/plans/` | Implementation plans, one per subsystem of a milestone |
| `cmd/batnetd/` | The node daemon. `-hub` runs a house hub on a Pi (the app, the control panel on `/hub/`, the radio, the WiFi mesh off/mock/real); `-mock N` runs a pretend village; `-sim` judges scenarios |
| `internal/` | Go packages: `hlc`, `ledger` (+ `sqlstore`), `seal`, `meshsync`, `mock`, `sim` (virtual time, the WiFi world model, villages of real nodes, scenarios, reports, the page API), `state`, `auth`, `web` (the house app), `lora`, `prio`, `meshtastic` (meshtasticd's TCP API, hand-decoded protobuf), `udplink` (meshsync over bat0), `hub` (the control panel, the radios' places hubs share (`places.go`), and plan 5a's power-cut kit: own WiFi, sound, LED and button, power modes, devices; offline maps), `gpio` (GPIO lines in pure Go), `translate` (the offline translator's client) |
| `deploy/pi/` | Turning a Pi into a hub: `deploy.sh <ssh-host>` from a laptop runs `setup.sh` on the Pi (clean-up, meshtasticd + Meshstick, batman-adv, kiosk, systemd units, watchdogs; the polkit rule, power modes, the own-WiFi firewall, the translator via `translate-setup.sh`; a Pi under 1 GB, like a Zero 2 W, gets no kiosk and no translator). The bench hubs: `knowledge/hardware/bench-pi7.md`, `bench-pz7.md` (a Zero 2 W) |
| `scenarios/` | Village scenarios (JSON) run by `batnetd -sim`, CI and the simulator's real engine |
| `cmd/batnetwasm/` | The real code for the simulator page (`GOOS=js GOARCH=wasm`) |
| `docs/img/` | Screenshots for the README; take new ones with `prototypes/village-sim/shoot.sh` (the simulator) and `docs/img/hub/shoot.sh <hub>` (a real hub's panel, with `?redact` swapping real names, IDs and places for placeholders) |
| `video/` | The film as code: `script.json` (narration), `scenes.js` (canvas scenes), `render.py` (ElevenLabs voice, headless Chrome frames, ffmpeg). The film itself is on YouTube only (linked from the README); no media in the repo. Skill: `.claude/skills/making-the-film/` |
| `.claude/skills/` | Agent skills (`.agents/skills` points here). Catalogue, including planned skills that do not exist yet: `knowledge/skills/catalog.md` |
| `tools/okflint/` | Checks the knowledge bundle (`cd tools/okflint && go run . ../../knowledge`) |
| `tools/hubload/` | Load test for a hub's app: pretend phones that sign up, hold the live stream, read and post (`main.go`), and a sampler for the hub (`sample.sh`) |
| `tools/hubcompare/` | `compare.py`: how strongly two hubs hear the same radios straight, from their saved readings and panel status (never meshtasticd); prints real names and places, keep its output out of the repo |
| `.githooks/` | Pre-commit: gitleaks + okflint. Enable once per clone: `git config core.hooksPath .githooks` |

## Rules

1. **Knowledge goes in `knowledge/`, in OKF form**, following
   [`knowledge/conventions.md`](knowledge/conventions.md): any research finding,
   decision, hardware note or test result. Don't leave findings only in chat or
   commit messages. Run okflint before committing (the hook does too).
2. **Nothing secret or personal is committed.** This repo is public. See
   `knowledge/security/secret-hygiene.md`. Never commit keys, PSKs, passwords,
   real names, precise locations, node IDs, MACs or private hostnames — use
   placeholders (`Anna · Miller House`, `!deadbeef`, `anna@example.org`).
   Never bypass the pre-commit hook (`--no-verify`).
3. **Decisions are records.** A change of direction gets a new decision that
   supersedes the old one (`status: deprecated`, linked) — never a silent edit.
4. **Go, pure Go for batnetd** (`CGO_ENABLED=0`); heavy work (speech, codecs,
   SDR) runs in helper processes. See decision 0005.
5. **Keep the known-problems list current.** Any bug you find, review finding
   you defer, or limit you accept goes in
   [`knowledge/known-problems.md`](knowledge/known-problems.md) with where it
   came from; move it to Resolved when fixed. The order of work is
   [`knowledge/todo.md`](knowledge/todo.md): tick items off as you finish them.
6. **Radio rules matter.** Anything that transmits or scans must respect EU868
   duty cycle and power limits and the activity-only scanner rule (decision 0009).
7. **`cp` here means: commit and push this repo.** The session's record is
   [`knowledge/log.md`](knowledge/log.md) (plus the test write-ups and to-do);
   batnet keeps its own records and writes nothing into other projects' repos.
