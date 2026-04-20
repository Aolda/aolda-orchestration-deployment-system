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
5. `docs/current-baseline-runbook.md`
6. `docs/internal-platform/openapi.yaml`
7. `docs/acceptance-criteria.md`
8. `docs/current-implementation-status.md`
9. `docs/agent-code-index.md`
10. `docs/mistake-log.md`
11. `docs/user-feedback-log.md`

현재 기준 작업 기준선은 `docs/current-implementation-status.md`에 정리된 **current implementation baseline**입니다.
`docs/internal-platform/prd.md`, `docs/acceptance-criteria.md`, `docs/phase1-decisions.md`, `docs/domain-rules.md`는 여전히 최소 계약을 고정하는 문서이지만, 현재 레포를 `Phase 1 MVP only`로 가정하면 안 됩니다.
Phase 표기는 historical planning bucket 으로 남겨두되, 이미 노출된 later-phase 기능을 `아직 future scope`라는 이유로 축소하거나 꺼서는 안 됩니다. 반대로 사용자가 명시적으로 요청하지 않은 완전한 미래 기능을 새로 여는 것도 피하세요.

## Allowed / Forbidden Changes
✅ **허용(Allowed)**:
- `openapi.yaml`에 정의된 규격에 맞춰 `backend/`와 `frontend/` 하위 코드를 새롭게 구현하거나 리팩토링하는 행위.
- `docs/domain-rules.md`에 정의된 아키텍처 규칙을 위반하지 않는 선의 데이터 모델 변경.

🚫 **금지(Forbidden)**:
- Kustomize 매니페스트 구조 등 GitOps와 매핑되는 **근간을 벗어나는 구조적 변경** (예: Flux 대신 ArgoCD 배포 로직 임의 작성)
- `openapi.yaml` 내용과 일치하지 않는 형태의 JSON Response 임의 변경.
- GitHub 기본 브랜치를 desired state의 source of truth로 두는 현재 설계를 별도 합의 없이 변경하는 행위
- Secret 평문을 Git에 저장하거나 Kubernetes 기본 `Secret` 생성으로 우회하는 행위
- 실제 연동이 없는 상태에서 사용자에게 실제 상태처럼 보이는 mock, placeholder, synthetic runtime data 를 노출하는 행위

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
- `make install-self-hosted-kubeconfig`: self-hosted dev cluster kubeconfig 를 `~/.kube/aods-self-hosted.yaml` 경로로 설치합니다.
- `make frontend-run`: React Vite 프론트엔드를 개발 서버 모드로 구동합니다.
- `make check-backend`: `scripts/check-backend.sh`를 통해 백엔드 `go vet`, 커버리지 게이트, `go test -race`를 수행합니다.
- `make check-manifests`: 생성 템플릿을 기반으로 샘플 GitOps 산출물을 렌더링한 뒤 Kubernetes manifest 검증을 수행합니다. 클러스터가 있으면 server dry-run까지 수행합니다.
- `make check`: 백엔드 `make check-backend`, 프론트 `npm run lint` + `npm run build`를 순서대로 수행합니다.

`make backend-run`은 이제 mock/local 어댑터 기준이 아니라 self-hosted dev cluster 기준 실행 경로입니다.
기본 kubeconfig 경로는 `~/.kube/aods-self-hosted.yaml`이며, localhost Prometheus/Vault URL이 잡혀 있으면 `scripts/backend-run.sh`가 필요한 `kubectl port-forward`를 자동으로 엽니다.

## Implementation Bias
- 새로운 기능을 추가할 때는 먼저 `openapi.yaml` 계약과 `acceptance-criteria.md` 시나리오를 맞추세요.
- 구현 중 모호함이 생기면 `docs/phase1-decisions.md`를 우선 기준으로 사용하세요.
- 실제 수정 위치와 활성 코드 경로는 `docs/agent-code-index.md`를 먼저 확인하세요.
- 현재 코드가 문서상 baseline 보다 앞서 있는지 여부는 `docs/current-implementation-status.md`로 확인하세요.
- 사용자가 직접 준 제품/UX 피드백은 `docs/user-feedback-log.md`에 계속 누적해 다음 작업에서도 참고하세요.
- 외부 연동이 아직 없으면 fabricated success 나 합성 메트릭을 만들지 말고, `Unknown`, `Unavailable`, empty state 로 그대로 드러내세요.
- 미래 기능이 필요해 보이더라도, 현재 요청 범위를 넘는 완전한 새 future-scope 동작은 임의로 열지 마세요. 반대로 이미 노출된 기능을 오래된 Phase 1 가정 때문에 축소하지도 마세요.
