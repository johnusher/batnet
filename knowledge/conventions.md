---
type: Convention
title: Knowledge conventions
description: How batnet records knowledge - OKF v0.2 plus a small set of local types, trust and freshness rules.
tags: [okf, knowledge, conventions]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
sources:
  - id: okf-spec
    resource: https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md
    title: Open Knowledge Format v0.2
    author: team:google-cloud
---

# The format

`knowledge/` is an OKF v0.2 bundle.[^okf-spec] One concept per `.md` file, a YAML
frontmatter block with a required `type`, a structured markdown body. Every
directory has an `index.md` listing its concepts with their one-line
`description`; the root `log.md` records changes newest first. `tools/okflint`
checks all of this and runs in the pre-commit hook.

# Types used here

| `type` | Directory | One file is |
|---|---|---|
| `Vision` | `/` | the purpose and scope |
| `Convention` | `/` | a rule for how we work |
| `Glossary` | `/` | term definitions |
| `Decision` | `decisions/` | one architecture decision record, numbered `NNNN-slug.md` |
| `Research Note` | `research/` | what we found about one question, with sources |
| `Hardware` | `hardware/` | one class of board or radio and what it can do |
| `Test Report` | `tests/` | one bench or field test: setup, results, what it settles, what was not recorded |
| `Policy` | `security/` | a rule that protects people or secrets |
| `Skill Catalog` | `skills/` | the agent skills, shipped and planned |
| `Issue List` | root | known problems: bugs, limits, deferred review findings, things to measure; with status |

A new type is fine when none fits; add it to this table.

# Fields we always fill

* `title`, `description` (one sentence, reused by index.md), `tags`.
* `generated: { by, at }` — actor per OKF §7: `claude-code/<model>` for an agent,
  `human:<github-handle>` for a person.
* `status` — `draft` while a decision is proposed or a note is unchecked,
  `stable` once agreed, `deprecated` when superseded (link the replacement).
* `sources` with an `id` on each, cited in the body as footnotes `[^id]`, for
  every claim that came from outside the repo.
* `stale_after` on anything that goes out of date: prices, firmware and
  library versions, benchmark numbers, regulations. Six months by default.

# Trust

* An agent writes a concept: no `verified` → **unverified**.
* A person agrees with it (a decision they chose, a note they checked):
  add `verified: { by: human:<handle>, at: … }` → **human-reviewed**.
* A bench or field test confirms a number: write it up in `tests/`
  (`YYYY-MM-DD-slug.md`), then add `verified: { by: process:<test-file-slug>, at: … }`
  to the concepts it confirms and link the write-up.
* Vendor claims stay attributed to the vendor in the body ("RAK says…").

# What never goes in

See [secret hygiene](/security/secret-hygiene.md): no keys, PSKs, passwords,
real locations, real names, node IDs, MACs or private hostnames.
