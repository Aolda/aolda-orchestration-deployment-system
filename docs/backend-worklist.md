# Backend Worklist

이 문서는 AODS 백엔드에서 앞으로 해야 할 일을 에이전트와 사람이 빠르게 잡을 수 있도록 정리한 작업 목록이다.

기준:

* 최소 계약 문서는 `docs/internal-platform/openapi.yaml`, `docs/acceptance-criteria.md`, `docs/phase1-decisions.md`, `docs/domain-rules.md` 를 따른다.
* 실제 현재 구현 상태와 회귀 기준선은 `docs/current-implementation-status.md` 를 함께 본다.
* 이미 노출된 later-phase 기능은 current implementation baseline 일부로 유지한다.
* 사용자가 명시적으로 요청하지 않으면 완전히 새로운 future-scope 기능을 임의로 열지 않는다.

## 이미 구현된 것

아래는 다시 만들 필요가 없는 항목이다.

* 프로젝트 목록 조회
* 프로젝트 앱 목록 조회
* 앱 생성
* Secret staging/finalize 처리
* GitOps manifest 생성
* Flux child `Kustomization` wiring
* 재배포
* sync status 조회
* metrics 조회
* canary 전략, deployment history, promote, abort
* change 리소스와 submit / approve / merge 상태 전이
* project policy read/update
* rollback policy 저장/조회
* application events 저장/조회
* poller 기반 auto rollback executor

즉, 백엔드 기본 뼈대는 이미 있다. 앞으로의 작업은 대부분 `남은 공백 메우기`, `실제 운영 흐름 강화`, `문서 계약과 구현 정렬` 쪽이다.

## Default Priority

별도 지시가 없으면 아래 우선순위를 따른다.

1. 최소 계약과 current implementation baseline 을 함께 지키는 회귀 방지
2. 운영 안전성 관련 cleanup / hardening
3. 이미 노출된 later-phase 기능의 실제 구현 공백 메우기
4. 새 later-phase 기능 확장

## Recently Completed

### Stale Secret Cleanup Worker

상태: `Implemented`

무엇이 들어갔나:

* local vault 와 real vault 모두에서 stale staging secret cleanup 지원
* 서버 시작 시 cleanup worker 자동 실행
* staging secret 에 `createdAt` 메타데이터 기록
* local/real cleanup 테스트 추가

주요 수정 위치:

* `backend/internal/vault/local.go`
* `backend/internal/vault/real.go`
* `backend/internal/vault/cleanup.go`
* `backend/internal/core/config.go`
* `backend/cmd/server/main.go`

### Orphan Flux Manifest Cleanup Worker

상태: `Implemented`

무엇이 들어갔나:

* AODS 가 생성한 Flux child `Kustomization` 만 대상으로 orphan cleanup 수행
* 현재 `platform/projects.yaml` + 앱 metadata 기준으로 기대 경로를 계산해 잘못된 cluster child manifest 제거
* 앱이 이미 삭제된 경우 남은 child manifest 제거
* catalog 에 없는 프로젝트의 기존 앱 manifest 나 수동(manual) manifest 는 보수적으로 유지
* 서버 시작 시 cleanup worker 자동 실행
* orphan cleanup 회귀 테스트 추가

주요 수정 위치:

* `backend/internal/application/orphan_cleanup.go`
* `backend/internal/application/orphan_cleanup_test.go`
* `backend/internal/core/config.go`
* `backend/cmd/server/main.go`

### Cluster Bootstrap API

상태: `Implemented`

무엇이 들어갔나:

* `platform admin` 권한으로 cluster catalog entry 생성 가능
* `platform/clusters.yaml` write 와 Flux bootstrap scaffold 생성을 같은 store 흐름에서 처리
* local mode 와 git mode 둘 다 지원
* cluster bootstrap 회귀 테스트와 git mode 회귀 테스트 추가

주요 수정 위치:

* `backend/internal/cluster/service.go`
* `backend/internal/cluster/http.go`
* `backend/internal/cluster/source_local.go`
* `backend/internal/cluster/source_git.go`
* `backend/internal/fluxscaffold/scaffold.go`
* `backend/internal/server/server_test.go`
* `backend/internal/server/server_git_test.go`

### Project Bootstrap API

상태: `Implemented`

무엇이 들어갔나:

* `platform admin` 권한으로 project catalog entry 생성 가능
* access / environment / policy 기본값을 bootstrap 시 자동 채움
* project create 시 참조 cluster Flux scaffold 를 보장
* local mode 와 git mode 둘 다 지원
* project bootstrap 회귀 테스트와 git mode 회귀 테스트 추가

주요 수정 위치:

* `backend/internal/project/service.go`
* `backend/internal/project/http.go`
* `backend/internal/project/source_local.go`
* `backend/internal/project/source_git.go`
* `backend/internal/server/server_test.go`
* `backend/internal/server/server_git_test.go`

### Application Archive/Delete Lifecycle

상태: `Implemented`

무엇이 들어갔나:

* application archive API 추가
* application delete API 추가
* archive 시 active desired state 와 Flux child wiring 제거, `.aods` history 는 유지
* delete 시 앱 디렉터리와 final Vault secret 제거
* local mode 와 git mode 둘 다 지원
* archive/delete 회귀 테스트 추가

주요 수정 위치:

* `backend/internal/application/service.go`
* `backend/internal/application/http.go`
* `backend/internal/application/store_local.go`
* `backend/internal/application/store_git.go`
* `backend/internal/application/store_support.go`
* `backend/internal/vault/local.go`
* `backend/internal/vault/real.go`
* `backend/internal/server/server_test.go`
* `backend/internal/server/server_git_test.go`

## Active Backlog

### 1. Real GitHub PR Workflow For `pull_request`

상태: `Partially implemented`

왜 필요한가:

* 현재 `pull_request` write mode 는 실제 GitHub Pull Request 생성 흐름이 아니다.
* 지금 구현은 `Change` 승인 후 `merge` 시점에 mutation 을 바로 적용하는 review-gated direct apply 에 가깝다.
* 이름과 실제 동작이 다르기 때문에 later-phase 목표와 현재 구현을 구분해서 메워야 한다.

추천 작업:

* `pull_request` mode 에서 branch 생성, commit, PR 생성, merge 결과 반영 흐름을 설계한다.
* direct push 와 PR mode 가 공존하도록 유지한다.
* `Change` 와 `Deployment` 의 역할을 계속 분리한다.

주요 수정 위치:

* `backend/internal/change/service.go`
* `backend/internal/change/store_git.go`
* `backend/internal/gitops/repository.go`
* `backend/internal/project/service.go`
* `backend/internal/server/server_test.go`

검증:

* `pull_request` mode 에서 direct apply 하지 않는 테스트
* 승인 전/후 상태 전이 테스트
* GitHub 연동 경계에 대한 adapter 테스트

주의:

* 이 작업은 later-phase scope 다. 사용자가 명시적으로 요청할 때만 구현한다.

### 2. Full Self-Service Bootstrap

상태: `Implemented for PoC`

왜 필요한가:

* cluster bootstrap API 와 project bootstrap API 는 들어갔다.
* 현재 PoC 기준에서는 cluster 와 project 를 platform admin 이 순차적으로 bootstrap 하면 된다.

추천 작업:

* 향후에는 cluster + project 를 한 트랜잭션 UX 로 묶는 onboarding flow 를 설계한다.
* 필요 시 권한 그룹 provisioning 연계를 붙인다.

주요 수정 위치:

* `backend/internal/project/`
* `backend/internal/application/flux_support.go`
* `backend/internal/cluster/`
* `backend/internal/gitops/repository.go`

### 3. Full Delete/Archive Cleanup Flow

상태: `Implemented for PoC`

왜 필요한가:

* orphan Flux child manifest cleanup 과 별도로, 사용자 API 에서 archive/delete 를 직접 호출할 수 있어야 한다.
* 현재 PoC 기준으로는 archive 와 delete 경계를 분리해 구현했다.

추천 작업:

* 향후 restore workflow 가 필요하면 archive 복구 API 를 별도 설계한다.
* delete 이후 external integration audit trail 을 남길지 검토한다.

주요 수정 위치:

* `backend/internal/application/service.go`
* `backend/internal/application/store_local.go`
* `backend/internal/application/store_git.go`
* `backend/internal/application/store_support.go`

주의:

* 현재 archive 는 `.aods` history 를 유지하고, delete 는 hard delete 다.
* restore / trash bin 같은 복구 UX 는 아직 없다.

## Ongoing Maintenance Tasks

### A. API Change Discipline

항상 해야 하는 일:

* `openapi.yaml` 변경
* 해당 `http.go` / `service.go` 변경
* 프론트 `types/api.ts` / `api/client.ts` 변경
* `backend/internal/server/server_test.go` 보강

이 항목은 별도 기능이 아니라 상시 규칙이다.

### B. Remove Stale Comments And Drift

항상 해야 하는 일:

* 현재 구현과 맞지 않는 TODO, 주석, handoff 문구를 그대로 두지 않는다.
* 특히 later-phase 구현 여부가 달라졌으면 `docs/current-implementation-status.md` 와 관련 지침 문서를 같이 갱신한다.

예시:

* `backend/internal/application/service.go` 의 오래된 repository field 관련 주석은 제거 대상이었다.

## Not A Default Task

아래는 명시적 요청 없이는 시작하지 않는다.

* actual GitHub PR workflow 구현
* restore workflow
* external approval / IAM provisioning 연계 bootstrap
* 멀티클러스터 UX 확장
* 범용 쿠버네티스 콘솔 성격의 기능 추가

## Quick Start For Backend Work

백엔드 작업을 시작할 때는 아래 순서를 기본으로 따른다.

1. `docs/internal-platform/openapi.yaml`
2. `docs/acceptance-criteria.md`
3. `docs/phase1-decisions.md`
4. `docs/domain-rules.md`
5. `docs/current-implementation-status.md`
6. `docs/backend-worklist.md`
7. `docs/agent-code-index.md`
8. `backend/AGENTS.md`
