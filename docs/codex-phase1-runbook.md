# Codex Phase 1 Runbook

이 문서는 새 Codex 세션이 AODS Phase 1 작업을 바로 이어받기 위한 handoff 파일이다.

숨겨진 세션 메모리를 기대하지 말고, 이 파일과 계약 문서를 기준으로 이어서 작업한다.

## 1. Current State

현재 기준:

* 브랜치: `main`
* 기본 구현 범위: **Phase 1 MVP**
* source of truth: **GitHub 기본 브랜치**
* 프로젝트 목록: `platform/projects.yaml`
* 앱 ID 규칙: `{projectId}__{appName}`
* Git write 모델: Phase 1은 `direct push`
* Secret 처리: Vault KV v2, `staging -> git commit -> final -> cleanup`

## 2. Read Order

반드시 아래 순서로 읽는다.

1. `AGENTS.md`
2. `CLAUDE.md`
3. `.agents/workflows/harness-engineering.md`
4. `docs/internal-platform/prd.md`
5. `docs/domain-rules.md`
6. `docs/phase1-decisions.md`
7. `docs/future-phases-roadmap.md`
8. `docs/internal-platform/openapi.yaml`
9. `docs/acceptance-criteria.md`

## 3. What Has Already Been Decided

이미 잠긴 결정:

* `GET /api/v1/projects` 는 Phase 1에 포함된다.
* 권한 최소 모델은 `viewer`, `deployer`, `admin` 이다.
* Flux UI 상태는 `Unknown`, `Syncing`, `Synced`, `Degraded` 네 개만 쓴다.
* Phase 2는 canary / promote / abort / rollout visibility 다.
* Phase 3은 PR mode / environments / audit / approval 다.
* Phase 4는 policy / auto rollback / cluster scale 다.

이 결정은 문서로 고정돼 있다. 추측으로 바꾸지 않는다.

## 4. Codex Runtime Settings

권장 실행 설정:

* Sandbox: `workspace-write`
* Approval policy: `on-failure`
* Network access: `enabled`
* Plan mode: `off`

이유:

* 코드 수정과 로컬 테스트는 막지 않는다.
* 진짜로 막히는 외부 연동에서만 승인 또는 개입을 요청한다.

## 5. Execution Prompt

새 Codex 세션에서 아래 프롬프트를 그대로 사용해도 된다.

```text
AODS Phase 1 MVP를 사용자 개입 최소화로 끝까지 진행해라.

반드시 먼저 아래 문서를 이 순서로 읽어라.
1. AGENTS.md
2. CLAUDE.md
3. .agents/workflows/harness-engineering.md
4. docs/internal-platform/prd.md
5. docs/domain-rules.md
6. docs/phase1-decisions.md
7. docs/future-phases-roadmap.md
8. docs/internal-platform/openapi.yaml
9. docs/acceptance-criteria.md

작업 목표:
- Phase 1 MVP를 구현한다.
- harness engineering은 orchestration, verification, QA 보조 용도로만 사용한다.
- product runtime이 harness-only env나 local file에 직접 의존하지 않게 한다.
- 사용자가 명시적으로 요청하지 않는 한 Phase 2 이상 기능은 구현하지 않는다.

작업 순서:
1. platform/projects.yaml 계약 파일 추가
2. backend에서 GitHub source reader abstraction 작성
3. local file-backed implementation으로 GET /api/v1/projects 구현
4. GET /api/v1/projects/{projectId}/applications 구현
5. POST /api/v1/projects/{projectId}/applications 구현
6. POST /api/v1/applications/{applicationId}/deployments 구현
7. GET /api/v1/applications/{applicationId}/sync-status 구현
8. GET /api/v1/applications/{applicationId}/metrics 구현
9. frontend Phase 1 화면 연결
10. 테스트와 검증 추가, make check 통과 시도

운영 규칙:
- GitHub, Vault, Kubernetes, Prometheus credential이 없으면 interface와 fake/local adapter를 만들고 계속 진행한다.
- 실제 외부 연동이 없다는 이유로 작업을 멈추지 말고, mock/fake/local reader로 계약을 먼저 맞춘다.
- 문서 계약과 충돌하거나 destructive action이 필요하거나 실제 credential이 반드시 필요한 마지막 연결 단계가 아니면 사용자에게 질문하지 말고 진행한다.
- 각 큰 단계 후 make check를 실행하고 실패 원인을 수정한다.
- openapi 계약과 acceptance criteria를 구현 기준으로 삼는다.
- Secret 평문을 Git에 저장하지 않는다.
- Go backend는 net/http만 사용한다.
- frontend는 Mantine + CSS Modules만 사용한다.

중간 보고는 짧게 하고, 구현과 검증을 우선하라.
```

## 6. First Commands

세션 시작 후 권장 첫 명령:

```bash
bash scripts/doctor.sh
make check
```

환경이 덜 깔려 있더라도, missing integration 때문에 구현을 멈추지 않는다.

## 7. Implementation Order

가장 작은 vertical slice 순서:

1. `platform/projects.yaml`
2. backend `GET /api/v1/projects`
3. backend `GET /api/v1/projects/{projectId}/applications`
4. backend create app
5. backend deploy
6. backend sync status
7. backend metrics
8. frontend project list
9. frontend app list
10. frontend create/app detail wiring

## 8. When To Stop And Ask

아래 경우만 멈춘다.

* destructive command 필요
* 실제 GitHub/Vault/K8s/Prometheus credential 없이는 마지막 연결 검증이 불가능
* 문서끼리 실제 충돌 발견
* 외부 side effect 가 생기는 단계

그 외에는 계속 진행한다.

## 9. Stronger Persistence

이 파일은 저장소에 남는 handoff 용도다.

더 강한 지속성을 원하면:

* 이 문서들과 계약 문서를 커밋한다.
* 구현은 새 브랜치에서 진행한다.

하지만 커밋 전에도, 이 파일이 있으면 다음 Codex 세션은 문맥을 충분히 이어받을 수 있다.
