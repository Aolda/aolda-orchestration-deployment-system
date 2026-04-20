---
applyTo: "docs/**,AGENTS.md,CLAUDE.md"
---

# Documentation Instructions

When editing docs and instruction files:

* Treat `docs/internal-platform/openapi.yaml`, `docs/acceptance-criteria.md`, `docs/phase1-decisions.md`, and `docs/domain-rules.md` as contract documents.
* Keep the baseline as Phase 1 unless the task explicitly expands scope.
* If current code is ahead of baseline docs, reflect that in `docs/current-implementation-status.md` instead of silently widening the Phase 1 contract.
* Keep `docs/agent-code-index.md` current when code entrypoints or feature ownership move.
* Record meaningful mistakes, regressions, or recurring confusion in `docs/mistake-log.md`.
