# AODS Codex / Claude Handoff

이 저장소는 문서 계약이 강한 편이다. 구현 전에 추측하지 말고 아래 순서로 읽는다.

## Read First

1. `docs/internal-platform/prd.md`
2. `docs/domain-rules.md`
3. `docs/phase1-decisions.md`
4. `docs/future-phases-roadmap.md`
5. `docs/current-baseline-runbook.md`
6. `docs/internal-platform/openapi.yaml`
7. `docs/acceptance-criteria.md`
8. `AGENTS.md`
9. `docs/current-implementation-status.md`
10. `docs/agent-code-index.md`
11. `docs/mistake-log.md`

## Current Default

현재 기준 계약 문서는 여전히 Phase 1 문서군이지만, 유지/QA 기준선은 **current implementation baseline** 이다.

다만 현재 코드베이스는 이미 Phase 2, 3, 4의 일부 기능을 포함한다.
현재 구현 상태는 `docs/current-implementation-status.md` 를 우선 참고한다.
수정할 코드 위치와 기능별 진입점은 `docs/agent-code-index.md` 를 우선 참고한다.
잘못한 사항과 재발 방지 기록은 `docs/mistake-log.md` 에 남긴다.

개발 런타임 기본값도 고정돼 있다.

* `make backend-run` 은 self-hosted dev cluster 기준으로 동작한다.
* 기본 kubeconfig 는 `~/.kube/aods-self-hosted.yaml` 이다.
* Prometheus 와 Vault 가 localhost URL 로 설정돼 있으면 `scripts/backend-run.sh` 가 필요한 port-forward 를 자동으로 연다.
* mock, placeholder, synthetic runtime data 를 dev 기본 경로로 되돌리지 않는다.
* 이미 코드와 UI에 노출된 later-phase 기능은 현재 baseline 일부로 보고 회귀 없이 유지한다.

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

* 사용자가 명시적으로 요청하지 않으면 최소 계약을 깨는 새 미래 기능을 임의로 열지 않는다.
* 이미 노출된 later-phase 기능을 `아직 future phase` 라는 이유로 축소하거나 제거하지 않는다.
* 미래 Phase를 위해 구조를 망가뜨리는 지름길을 쓰지 않는다.
* direct push 모델은 Phase 1/2에서 유지하고, PR mode는 Phase 3에서 추가한다.

## Good Starting Point

새 세션에서 구현을 시작한다면 우선순위는 이쪽이다.

1. `docs/current-implementation-status.md` 로 현재 baseline 을 파악
2. `docs/agent-code-index.md` 로 실제 활성 코드 경로 확인
3. 이미 노출된 API/UI/실연동 경로의 regression 부터 점검
4. 최소 계약 문서(`openapi.yaml`, `acceptance-criteria.md`, `phase1-decisions.md`)와의 불일치 수정
5. 이후 남은 구현 공백 또는 QA 이슈를 우선순위대로 정리

## If Things Feel Ambiguous

우선순위는 항상 이 순서다.

1. `openapi.yaml`
2. `acceptance-criteria.md`
3. `phase1-decisions.md`
4. `domain-rules.md`
5. `future-phases-roadmap.md`

문서끼리 충돌하면 코드부터 맞추지 말고 문서를 먼저 고친다.
