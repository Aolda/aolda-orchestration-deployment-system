# Internal App Deployment Platform - Agent Guide

이 문서는 코덱스(Codex) 및 타 개발 에이전트들이 본 저장소에 기여할 때 참조해야 할 워크로딩(Workloading) 규칙입니다.

## Repo Map
- `docs/`: PRD, `openapi.yaml`, 도메인 규칙, Phase 결정 문서, 미래 로드맵 등 에이전트가 코딩을 위해 **반드시 먼저 참고해야 할 계약(Contracts)**이 있습니다.
- `frontend/`: React + Vite 기반의 포털 웹플리케이션이 배치될 폴더입니다.
- `backend/`: Go `net/http` 표준 라이브러리 기반의 플랫폼 백엔드 API가 배치될 폴더입니다.
- `scripts/`: 하네스 파이프라인 연동(노션/Discord) 스크립트 모음입니다.

## Read Order
코딩을 시작하기 전에 아래 순서로 문서를 읽으세요.

1. `docs/internal-platform/prd.md`
2. `docs/domain-rules.md`
3. `docs/phase1-decisions.md`
4. `docs/future-phases-roadmap.md`
5. `docs/codex-phase1-runbook.md`
6. `docs/internal-platform/openapi.yaml`
7. `docs/acceptance-criteria.md`

현재 기준 구현 대상은 **Phase 1 MVP**입니다.
Phase 2, 3, 4는 `docs/future-phases-roadmap.md`를 설계 기준으로만 참고하고, 사용자가 명시적으로 요청하지 않는 한 Phase 1 계약을 깨면서 선반영하지 마세요.

## Allowed / Forbidden Changes
✅ **허용(Allowed)**:
- `openapi.yaml`에 정의된 규격에 맞춰 `backend/`와 `frontend/` 하위 코드를 새롭게 구현하거나 리팩토링하는 행위.
- `docs/domain-rules.md`에 정의된 아키텍처 규칙을 위반하지 않는 선의 데이터 모델 변경.

🚫 **금지(Forbidden)**:
- Kustomize 매니페스트 구조 등 GitOps와 매핑되는 **근간을 벗어나는 구조적 변경** (예: Flux 대신 ArgoCD 배포 로직 임의 작성)
- `openapi.yaml` 내용과 일치하지 않는 형태의 JSON Response 임의 변경.
- GitHub 기본 브랜치를 desired state의 source of truth로 두는 현재 설계를 별도 합의 없이 변경하는 행위
- Secret 평문을 Git에 저장하거나 Kubernetes 기본 `Secret` 생성으로 우회하는 행위

## Current Architectural Decisions
- 프로젝트 목록과 앱 메타데이터의 source of truth는 GitHub 기본 브랜치입니다.
- 프로젝트 목록은 `platform/projects.yaml`에서 읽습니다.
- 앱 ID는 deterministic 규칙 `{projectId}__{appName}`를 따릅니다.
- Phase 1의 Git write 모델은 direct push입니다. PR workflow는 Phase 3에서 확장합니다.
- Vault는 KV v2 기준이며, `staging -> git commit -> final -> cleanup` 흐름을 따릅니다.
- Flux 상태는 UI에서 `Unknown`, `Syncing`, `Synced`, `Degraded` 네 개만 노출합니다.
- 권한 최소 모델은 `viewer`, `deployer`, `admin` 입니다.

## Makefile Commands
로컬 개발 및 검증 시 아래 명령을 사용하세요. (수동 검증 또는 `#back-qa` / `#front-qa` 테스트 시)

- `make backend-run`: Go 백엔드 로컬 서버를 구동합니다.
- `make frontend-run`: React Vite 프론트엔드를 개발 서버 모드로 구동합니다.
- `make check`: 전역 컨벤션/린트/테스트를 수행합니다.

## Implementation Bias
- 새로운 기능을 추가할 때는 먼저 `openapi.yaml` 계약과 `acceptance-criteria.md` 시나리오를 맞추세요.
- 구현 중 모호함이 생기면 `docs/phase1-decisions.md`를 우선 기준으로 사용하세요.
- 미래 Phase 기능이 필요해 보이더라도, 현재 요청 범위가 Phase 1이면 구조적 준비만 하고 동작은 열지 마세요.
