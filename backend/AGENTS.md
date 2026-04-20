# Backend Agent Guide

이 문서는 `backend/` 와 함께 `platform/`, `apps/` 쪽을 수정하는 작업에서 우선 적용되는 백엔드 전용 지침이다.

## Read First

작업 전에 아래 문서를 먼저 확인한다.

1. `docs/internal-platform/openapi.yaml`
2. `docs/acceptance-criteria.md`
3. `docs/phase1-decisions.md`
4. `docs/domain-rules.md`
5. `docs/current-implementation-status.md`
6. `docs/backend-worklist.md`
7. `docs/agent-code-index.md`
8. 루트 `AGENTS.md`

## Non-Negotiables

* 백엔드는 Go `net/http` 표준 라이브러리만 사용한다.
* `openapi.yaml` 과 다른 request/response shape 를 임의로 만들지 않는다.
* GitHub 기본 브랜치가 desired state 의 source of truth 다.
* 프로젝트 목록은 `platform/projects.yaml` 에서 읽는다.
* 앱 메타데이터는 `apps/{projectId}/{appName}/` 에서 읽고 쓴다.
* 앱 ID 는 `{projectId}__{appName}` 규칙을 유지한다.
* Secret 평문을 Git에 저장하지 않는다.
* Kubernetes 기본 `Secret` 리소스로 우회하지 않는다.
* Vault 는 `staging -> git commit -> final -> cleanup` 흐름을 유지한다.
* Flux 상태는 `Unknown`, `Syncing`, `Synced`, `Degraded` 네 개만 외부에 노출한다.
* 실제 연동이 없으면 fabricated sync, rollout, metrics 값을 만들어내지 않는다.
* 최소 계약 문서는 여전히 Phase 1 문서군이지만, 회귀 기준선은 `docs/current-implementation-status.md` 의 current implementation baseline 이다.
* 사용자가 명시적으로 요청하지 않으면 새 미래 기능을 임의로 열지 않는다.
* 이미 노출된 later-phase 동작을 `아직 future scope` 라는 이유로 축소하거나 제거하지 않는다.

## Where To Edit

* 서버 시작점: `backend/cmd/server/main.go`
* 라우팅과 의존성 조립: `backend/internal/server/server.go`
* 공통 설정/인증/HTTP 유틸: `backend/internal/core/`
* 프로젝트 카탈로그/정책: `backend/internal/project/`
* 앱 생성/배포/메트릭/상태/rollout: `backend/internal/application/`
* 변경 관리 흐름: `backend/internal/change/`
* 클러스터 카탈로그: `backend/internal/cluster/`
* Git 어댑터: `backend/internal/gitops/repository.go`
* Kubernetes 연동: `backend/internal/kubernetes/`
* Vault 연동: `backend/internal/vault/`

## Editing Rules

* 새 API endpoint 를 추가하면 `openapi.yaml`, `server.go`, 해당 도메인 `http.go`, `service.go`, 프론트 타입/클라이언트, 테스트까지 같이 맞춘다.
* 새 route 는 `backend/internal/server/server.go` 에 등록한다.
* 공통 에러 응답은 가능한 한 `backend/internal/core/http.go` 의 기존 방식과 `phase1-decisions.md` 의 에러 스키마를 따른다.
* GitOps manifest 구조 변경은 `application/store_local.go`, `application/store_git.go`, `gitops/repository.go` 를 같이 확인한다.
* 정책이나 권한 변경은 `platform/projects.yaml` 과 `backend/internal/project/` 를 같이 수정한다.
* Secret 경로 규칙을 바꿀 때는 `backend/internal/core/secrets.go`, `backend/internal/application/service.go`, `backend/internal/vault/` 를 같이 본다.
* 문서와 코드가 충돌하면 코드를 먼저 합리화하지 말고 문서 계약부터 확인한다.
* 연동이 비어 있으면 `Unknown`, empty response, 명시적 오류로 처리하고 가짜 성공 상태를 반환하지 않는다.

## Validation

백엔드 변경 후 기본 검증은 아래 순서를 권장한다.

1. `cd backend && go test ./...`
2. 루트에서 `make check-backend`
   - 현재 `scripts/check-backend.sh` 가 백엔드 `go vet ./...`, 전체 커버리지 하한선, `go test -race ./...` 를 함께 검증한다.
3. 루트에서 `make check`
   - 전체 저장소 기준으로는 `make check-backend` 뒤에 프론트 `npm run lint` + `npm run build` 가 이어진다.
4. GitOps manifest 템플릿이나 scaffold 를 건드렸다면 `make check-manifests`
   - 샘플 산출물을 실제 생성 경로로 렌더링하고 schema validation, 가능하면 Kubernetes server dry-run 까지 확인한다.

추가로 아래 경우는 더 엄격하게 본다.

* 라우트나 응답 shape 변경: `backend/internal/server/server_test.go`
* Git mode 변경: `backend/internal/server/server_git_test.go`
* Vault/Kubernetes/Git 어댑터 변경: 해당 패키지 테스트

현재 남은 백엔드 backlog 는 `docs/backend-worklist.md` 를 먼저 본다.

## Common Pitfalls

* `platform/` 나 `apps/` 산출물을 직접 덮어쓰기 전에 store 레이어에서 생성되는지 먼저 본다.
* Phase 2, 3, 4 endpoint 가 이미 있어도, 요청 범위가 Phase 1이면 later phase behavior 를 확장하지 않는다.
* `openapi.yaml` 을 안 고치고 핸들러나 타입만 바꾸는 실수를 피한다.
* Secret 값이 로그, fixture, 테스트 데이터, manifest 에 남지 않게 주의한다.
