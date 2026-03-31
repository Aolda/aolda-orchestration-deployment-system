# AODS Codex / Claude Handoff

이 저장소는 문서 계약이 강한 편이다. 구현 전에 추측하지 말고 아래 순서로 읽는다.

## Read First

1. `docs/internal-platform/prd.md`
2. `docs/domain-rules.md`
3. `docs/phase1-decisions.md`
4. `docs/future-phases-roadmap.md`
5. `docs/codex-phase1-runbook.md`
6. `docs/internal-platform/openapi.yaml`
7. `docs/acceptance-criteria.md`
8. `AGENTS.md`

## Current Default

현재 기본 구현 범위는 **Phase 1 MVP**다.

Phase 1의 핵심은 아래다.

* `GET /api/v1/projects`
* `GET /api/v1/projects/{projectId}/applications`
* `POST /api/v1/projects/{projectId}/applications`
* `POST /api/v1/applications/{applicationId}/deployments`
* `GET /api/v1/applications/{applicationId}/sync-status`
* `GET /api/v1/applications/{applicationId}/metrics`

## Non-Negotiables

* GitHub 기본 브랜치가 desired state의 source of truth다.
* 프로젝트 목록은 `platform/projects.yaml`에서 읽는다.
* Secret 평문은 Git에 저장하지 않는다.
* Vault는 KV v2를 사용하고 staging/final 경로를 분리한다.
* Backend는 Go `net/http` 표준 라이브러리만 사용한다.
* Frontend는 Mantine + CSS Modules 기준이다.
* Flux 상태는 `Unknown`, `Syncing`, `Synced`, `Degraded` 네 개만 UI에 노출한다.

## Phase Guardrail

Phase 2, 3, 4는 이미 `docs/future-phases-roadmap.md`에 정리돼 있다.

* 사용자가 명시적으로 요청하지 않으면 Phase 1을 구현한다.
* 미래 Phase를 위해 구조를 망가뜨리는 지름길을 쓰지 않는다.
* direct push 모델은 Phase 1/2에서 유지하고, PR mode는 Phase 3에서 추가한다.

## Good Starting Point

새 세션에서 구현을 시작한다면 우선순위는 이쪽이다.

1. `platform/projects.yaml` 계약 파일 추가
2. backend `GET /api/v1/projects` 구현
3. GitHub source reader 추상화
4. `GET /api/v1/projects/{projectId}/applications` 구현
5. 이후 create / deploy / sync / metrics 순서

## If Things Feel Ambiguous

우선순위는 항상 이 순서다.

1. `openapi.yaml`
2. `acceptance-criteria.md`
3. `phase1-decisions.md`
4. `domain-rules.md`
5. `future-phases-roadmap.md`

문서끼리 충돌하면 코드부터 맞추지 말고 문서를 먼저 고친다.
