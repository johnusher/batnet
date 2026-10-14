---
type: Research Note
title: Open Knowledge Format and agent knowledge practice
description: Why this bundle follows Google's OKF v0.2, what the spec requires, and the practices taken from it.
tags: [okf, knowledge, agents, skills]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
sources:
  - id: okf-spec
    resource: https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md
    title: Open Knowledge Format v0.2
    author: team:google-cloud
  - id: okf-blog
    resource: https://cloud.google.com/blog/products/data-analytics/how-the-open-knowledge-format-can-improve-data-sharing
    title: Introducing the Open Knowledge Format
    author: team:google-cloud
  - id: agentskills
    resource: https://agentskills.io/specification
    title: Agent Skills specification
---

# OKF in brief

* Released by Google Cloud in June 2026 (v0.1), now v0.2.[^okf-blog][^okf-spec]
* A bundle is a directory of markdown files with YAML frontmatter; `type` is the only required field; `index.md` and `log.md` are reserved.[^okf-spec]
* v0.2 adds provenance (`sources` with ids cited as footnotes), trust (`generated`, `verified`, tiers by `human:` actors), lifecycle (`status`, `stale_after`) and attested computations.[^okf-spec]
* Consumers must tolerate unknown types, unknown keys and broken links.[^okf-spec]

# Practices batnet takes

* The rules in [knowledge conventions](/conventions.md), enforced by `tools/okflint`.
* Agent skills follow the Agent Skills format (`SKILL.md` with `name` and `description`), readable by Claude Code, Codex and others.[^agentskills] Catalogue: [skills](/skills/).
* Decisions as numbered records, research notes with sources and an expiry, field results recorded as `verified` entries.
