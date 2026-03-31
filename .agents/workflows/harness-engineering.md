---
description: Harness engineering intent, boundaries, and workflow guidance for agents
---

# Harness Engineering Strategy

## Purpose
- This document explains how agents should use harness engineering in this repository.
- It is a strategy and operating note, not proof that every referenced tool or script is already installed in the current workspace.
- For concrete bootstrap steps, use `docs/harness-setup.md`.

## Core Contracts
- `AGENTS.md`
- `docs/internal-platform/openapi.yaml`
- `docs/internal-platform/prd.md`
- `docs/domain-rules.md`
- `docs/acceptance-criteria.md`

## Current Repository Baseline
- The product code lives in `backend/` and `frontend/`.
- Contract documents live in `docs/`.
- Harness-specific directories such as `scripts/`, `config/`, or `memory/` may be added as the harness is bootstrapped.
- Agents must verify that a path, script, or CLI exists before treating it as available.

## Harness Scope
- Use harness engineering to accelerate MVP planning, QA, release coordination, and external sync work.
- Keep harness concerns separate from product runtime concerns.
- Do not make the core application depend on harness-only environment variables or operator-only local files.

## Planned Harness Integrations
- Notion sync:
  Read tokens from `config/mcporter.json` once a sync script is added under `scripts/`.
- Google Calendar:
  Use `GCAL_CALENDAR_ID` once calendar helper scripts or `gws` wrappers are added.
- Browser QA:
  Prefer `agent-browser` when the team has it installed. Use another approved browser automation path only when needed.
- Multi-agent orchestration:
  Use `gstack` or ClawFlows only when those tools are available in the local environment.

## Operating Rules For Agents
- Treat `AGENTS.md` and the `docs/` contracts as source of truth for product behavior.
- Treat missing harness tools as setup gaps, not as reasons to change product contracts.
- Clearly distinguish between:
  Required product setup, required harness setup, and optional operator tooling.
- When adding harness automation, prefer small scripts in `scripts/` plus companion setup docs.
- When a document mentions future automation, mark it as planned until the supporting files actually exist.

## Long-Term Maintenance Rule
- The repository is expected to outlive the initial harness-driven MVP phase.
- Harness utilities should therefore remain removable or replaceable without forcing major changes to `backend/`, `frontend/`, or the API contracts.
- Any future agent should be able to understand:
  what is product code, what is operator tooling, and what is still planned work.
