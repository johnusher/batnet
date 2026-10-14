---
type: Skill Catalog
title: batnet agent skills
description: The agent skills this repo ships in .claude/skills (shared with other agents via .agents/skills), and the ones planned per milestone.
tags: [skills, agents]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
---

# How skills work here

A skill is a `SKILL.md` (Agent Skills format: `name`, `description` saying *when*
to use it) in `.claude/skills/<name>/`; `.agents/skills` points at the same folder
for Codex and other agents. Each skill is written test-first: run the task without
it, record what goes wrong, write the skill, re-run. Mechanical rules go in tools
(okflint, gitleaks), not skills — skills are for judgement.

# Shipped

| Skill | Use when |
|---|---|
| [capturing-knowledge](../../.claude/skills/capturing-knowledge/SKILL.md) | adding or changing anything in `knowledge/`: a research finding, decision, hardware note, test result |

# Planned

| Skill | Use when | Milestone |
|---|---|---|
| publishing-safely | making the repo public, pushing, or sharing logs, captures or screenshots — the judgement regexes miss (names and places in prose) | before going public |
| computing-lora-airtime | adding anything that transmits over LoRa; checks payload size and airtime against the village budget, with a Go calculator | M1 |
| checking-radio-rules | a feature transmits or scans: EU868 duty cycle and power, PMR446, listening law; records the rule in `knowledge/` | M1, M4 |
| cross-building-for-pi | building and deploying batnetd to Zero W / Zero 2 / 3 / 4 / 5 | M1 |
| provisioning-a-node | turning a fresh SD card into a house, relay or gateway, including the village key | M1 |
| running-a-mock-village | testing sync, partitions and rejoin with several fake houses on one machine | M1 |
| debugging-the-mesh | nodes don't see each other, messages don't arrive, LoRa is silent: batctl, iw, meshtastic CLI, batnetd logs | M1 |
| testing-on-phones | checking captive portal, certificate warning, microphone and WebRTC on real iOS/Android | M1 spike |
| field-testing | planning and recording a range or deployment test without leaking locations | M1+ |
| benchmarking-speech | measuring speech-to-text/text-to-speech on each Pi model | M2 |
