---
description: Harness engineering rules, tools, and automation flows for Agent processing
---

# Harness Engineering Context & Workflows

## 핵심 문서 & 계약 (Core Documents & Contracts)
- `AGENTS.md`: Internal App Deployment Platform, domain rules, repo map, allowed/forbidden changes, `make backend-run`, `make frontend-run`, `make check` 등 명령 정리.
- **Docs**: `docs/internal-platform/openapi.yaml`, `domain-rules.md`, `acceptance-criteria.md` → 계약 중심으로 프론트/백/QA가 따를 기준.
- **Notion sync**: `scripts/sync_internals_notion.py` + Notion token in `config/mcporter.json` → playground page + "Harness Coordination" subpage.

## 툴/플러그인 (Tools & Plugins)
- **gstack / gstack harness**: Claude Code slash commands (`/plan-ceo-review`, `/review`, `/qa`) + `agent-browser`/`gstack`/`gws` evidence.
  - Primary repo: https://github.com/garrytan/gstack
- **gstack flows**: ClawFlows repo cloned under `/workspace/clawflows`. Use it to define multi-agent DAG for release/QA automation.
- **agent-browser**: `scripts/agent-browser-run.sh` + agent-browser CLI to capture snapshots, interact with UIs, replace Playwright.
- **Google Calendar**: `gws` commands and helper scripts (`gworkspace_create_event.sh`) using `GCAL_CALENDAR_ID` to register deadlines.
- **Notion**: Token stored in `config/mcporter.json` (`ntn…`). Scripts read it to sync docs and create Harness Coordination page.

## Channel + Role Mapping
- `#front-qa`: Frontend React QA via agent-browser + Claude prompts.
- `#back-qa`: Go backend QA via gstack + harness review.
- `#harness-ops`: Release monitor / ClawFlows orchestration + Notion/Discord updates.
- **Notion page**: `playground…/Harness Coordination` hosts overview for future agents.

## Automation Flows
- **Inputs**: PRD for internal deployment platform (Go backend / React frontend), OpenAPI/domain/acceptance docs, Makefile commands.
- **Flow**: Submit PRD → agent-browser snapshot → Claude `/plan-ceo-review` → gstack/Playwright QA → ClawFlows release monitor → Notion + Discord summary + Google Calendar alerts.
- **Post-update**: Notion sync script can be run after docs update to keep page fresh.

## Next Steps Ready
- **Integration**: Harness integration now focused on “contract-first” loops: front/back QA, release monitor, Notion briefs.
- **Automation Utilities**: Additional gluing (e.g., ClawFlows automation, agent-browser evidence, gws calendar events) is already scripted and can be triggered by future agents via provided scripts.
- **Memory/Context**: `gstack`/`clawflows`/`docs` + Notion sync + calendar automations are recorded in `memory/2026-03-30.md` & `memory/2026-03-31.md` so subsequent agents know history.
