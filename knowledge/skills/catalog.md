---
type: Skill Catalog
title: batnet agent skills
description: The agent skills this repo ships in .claude/skills (shared with other agents via .agents/skills), and the ones planned per milestone.
tags: [skills, agents]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T06:28:18Z }
---

# How skills work here

A skill is a `SKILL.md` (Agent Skills format: `name`, `description` saying *when*
to use it) in `.claude/skills/<name>/`; `.agents/skills` points at the same folder
for Codex and other agents. Each skill is written test-first: run the task without
it, record what goes wrong, write the skill, re-run. Mechanical rules go in tools
(okflint, gitleaks), not skills — skills are for judgement.

# Shipped

None yet.

# Tried and not needed

| Skill | Why not |
|---|---|
| capturing-knowledge | Baseline test 2026-10-14: three agents given realistic tasks (a bench test full of personal data, a reversed decision, vendor and forum claims) with only AGENTS.md, [conventions](/conventions.md) and okflint all kept personal data out, attributed and sourced claims, superseded instead of editing the old decision, and updated index and log. Nothing for a skill to fix. Re-test if knowledge quality slips. |

# Replaced by a tool

| Would-be skill | Tool | Why a tool |
|---|---|---|
| screenshotting-the-mockup | `sh prototypes/village-sim/shoot.sh OUT.png 'JS' [light\|dark] [phone\|full]` | Mechanical. The one trap (2026-10-15): JavaScript injected with a `<script>` element can't reach the page's state, because the page code sits in an IIFE; errors then fail silently and the screenshot shows the default screen. The script uses the page's `window.__simEval` hook and prints errors. |

# Planned

These do **not exist yet**. Do not cite them as if they apply; once written, each moves to Shipped.

| Skill | Use when | Milestone |
|---|---|---|
| publishing-safely | making the repo public, pushing, or sharing logs, captures or screenshots — the judgement regexes miss (names and places in prose) | before going public |
| computing-lora-airtime | adding anything that transmits over LoRa; checks payload size and airtime against the village budget, with a Go calculator | M1 |
| checking-radio-rules | a feature transmits or scans: EU868 duty cycle and power, PMR446, listening law; records the rule in `knowledge/` | M1, M4 |
| cross-building-for-pi | building and deploying batnetd to Zero W / Zero 2 / 3 / 4 / 5 | M1 |
| provisioning-a-node | turning a fresh SD card into a house, relay or gateway, including the village key | M1 |
| running-a-mock-village | testing sync, partitions and rejoin with several fake houses on one machine (`go run ./cmd/batnetd -mock N`, plan 1) | M1 |
| debugging-the-mesh | nodes don't see each other, messages don't arrive, LoRa is silent: batctl, iw, meshtastic CLI, batnetd logs | M1 |
| testing-on-phones | checking captive portal, certificate warning, microphone and WebRTC on real iOS/Android | M1 spike |
| field-testing | planning and recording a range or deployment test without leaking locations | M1+ |
| benchmarking-speech | measuring speech-to-text/text-to-speech on each Pi model; raw vs denoised clips ([decision 0014](/decisions/0014-speech-pipeline.md)) | M2 |
| writing-for-the-public | writing the README, help pages or any text households read: plain words, every term explained, click-by-click steps, what doesn't work yet said plainly (the user's rule, 2026-10-15) | M1 |
| choosing-hardware | recommending a board, dongle or radio: check the driver's supported modes and meshtasticd's support status before naming a part. 2026-10-15 research found two traps: MT7921AU sticks have no mesh mode, and Berrybase's Waveshare "SX1262 LoRa HAT" is the UART version | M1 |
| measuring-power | bench power measurements per node and use case, feeding the [power budget](/research/power-budget.md) | M1 bench |
