---
type: Decision
title: "0011: MIT licence"
description: batnet's code, knowledge bundle and prototypes are released under the MIT licence.
tags: [licence, open-source]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T05:24:55Z }
verified: { by: human:johnusher, at: 2026-10-15T05:24:55Z }
---

# Context

The repository went public on GitHub. Without a licence, people can read the
code but may not legally reuse it, which works against a project meant to be
copied village to village.

# Decision

The whole repository is MIT-licensed ([LICENSE](https://github.com/johnusher/batnet/blob/main/LICENSE)).
The copyright line names "the batnet contributors" rather than a person, in
keeping with the [secret hygiene policy](/security/secret-hygiene.md).

# Consequences

Anyone may reuse, modify and sell batnet, including in closed products.
Dependencies must have MIT-compatible licences for the binary to ship under
MIT. meshtasticd (GPL-3.0) runs as a separate program that batnetd talks to
over a socket and is not linked into it; keep it that way.
