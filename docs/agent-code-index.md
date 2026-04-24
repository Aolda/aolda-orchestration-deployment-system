# Agent Code Index

이 문서는 AODS 저장소에서 에이전트가 기능을 추가하거나 수정할 때, 어떤 계약을 먼저 보고 어떤 파일을 열어야 하는지 빠르게 찾기 위한 코드 인덱스다.

목표는 두 가지다.

1. 기능별 진입점과 수정 경로를 빠르게 찾게 한다.
2. 앞으로의 기능 개발에서도 Phase 1 계약과 아키텍처 규칙을 계속 적용하게 한다.

## 필수 작업 순서

기능 개발이나 리팩토링을 시작할 때는 아래 순서를 기본으로 따른다.

1. `docs/internal-platform/openapi.yaml`
2. `docs/acceptance-criteria.md`
3. `docs/phase1-decisions.md`
4. `docs/domain-rules.md`
5. `docs/current-implementation-status.md`
6. `docs/agent-code-index.md`
7. `docs/mistake-log.md`
8. `docs/user-feedback-log.md`

판단 원칙:

* 최소 계약 문서는 여전히 **Phase 1 문서군**이다.
* 하지만 회귀와 QA 기준선은 `docs/current-implementation-status.md` 에 정리된 **current implementation baseline** 이다.
* 사용자가 명시적으로 요청하지 않으면 완전히 새로운 future-scope 기능을 임의로 열지 않는다.
* 이미 코드와 UI에 노출된 later-phase 기능은 `아직 future scope` 라는 이유로 축소하거나 숨기지 않는다.
* 실제 수정 위치는 `docs/current-implementation-status.md` 와 이 문서를 같이 본다.

## 항상 적용되는 규칙

아래 규칙은 기능 종류와 무관하게 유지한다.

* GitHub 기본 브랜치가 desired state 의 source of truth 다.
* 프로젝트 목록은 `platform/projects.yaml` 에서 읽는다.
* 앱 메타데이터는 `apps/{projectId}/{appName}/` 경로에서 읽는다.
* 앱 ID 는 `{projectId}__{appName}` 규칙을 유지한다.
* Secret 평문을 Git에 저장하지 않는다.
* Kubernetes 기본 `Secret` 리소스로 우회하지 않는다.
* Vault 는 KV v2 와 `staging -> git commit -> final -> cleanup` 흐름을 유지한다.
* GitHub 접근 토큰은 앱 `.env` 시크릿과 같은 Vault 경로에 저장하지 않는다. 저장소 접근용 토큰은 별도 경로에 분리 저장한다.
* Flux 상태는 UI에 `Unknown`, `Syncing`, `Synced`, `Degraded` 네 개만 노출한다.
* 백엔드는 Go `net/http` 표준 라이브러리 기준이다.
* 프론트엔드는 Mantine + CSS Modules 기준이다.
* `openapi.yaml` 과 다른 JSON response shape 를 임의로 만들지 않는다.

## 계약 및 소스 오브 트루스 파일

### API 와 제품 계약

* `docs/internal-platform/openapi.yaml`
  - API request/response 를 바꾸면 가장 먼저 수정한다.
  - 백엔드 핸들러, 프론트 타입, API 클라이언트보다 우선한다.
* `docs/acceptance-criteria.md`
  - 기능이 충족해야 할 시나리오 기준이다.
  - 테스트 케이스를 추가할 때 시나리오 기준으로 맞춘다.
* `docs/phase1-decisions.md`
  - 구현 중 애매한 판단의 기본 결론을 담고 있다.
* `docs/domain-rules.md`
  - 절대 제약 사항이다. GitOps 구조, Vault 모델, 권한 모델, 프론트/백엔드 컨벤션이 여기에 묶여 있다.
* `docs/keycloak-group-auth-model.md`
  - 현재 저장소에서 권장하는 Keycloak role 기반 연동 모델이다.
  - auth 작업에서는 platform admin 을 group path 로 되돌리기 전에 이 문서를 먼저 확인한다.
* `docs/keycloak-operator-guide.md`
  - Keycloak 운영자가 AODS client role, 사용자 role 할당, token claim, 장애 대응을 설정하는 절차서다.
  - 운영 안내를 작성하거나 배포 전 권한 검증을 할 때 이 문서를 기준으로 삼는다.
* `docs/current-implementation-status.md`
  - 실제 코드가 어디까지 들어와 있는지 보는 현실 체크 문서다.
* `docs/backend-worklist.md`
  - 백엔드에서 아직 남아 있는 구현/운영 backlog 를 정리한 문서다.
  - backend 작업에서는 이 문서를 같이 본다.
* `docs/frontend-swagger-guide.md`
  - 프론트가 Swagger/OpenAPI 계약을 실제 `types/api.ts`, `api/client.ts` 와 어떻게 연결하는지 정리한 문서다.
  - frontend 작업에서는 이 문서를 같이 본다.
* `docs/frontend-page-plan.md`
  - 프론트 페이지 구조, 라이브 화면, 분리 중인 화면, 기획 공백을 정리한 문서다.
* `docs/frontend-worklist.md`
  - 프론트에서 아직 남아 있는 구조/UX backlog 를 정리한 문서다.
  - frontend 작업에서는 이 문서를 같이 본다.
* `docs/frontend-user-journeys.md`
  - 프론트 핵심 유저 스토리와 테스트 규칙을 정리한 문서다.
  - 사용자 흐름을 바꾸는 작업에서는 이 문서를 같이 본다.
* `docs/mistake-log.md`
  - 계약 위반 시도, 회귀, 문서 오해, 운영 혼선을 기록한다.
* `docs/user-feedback-log.md`
  - 사용자가 직접 준 제품/UX 피드백과 그에 대한 반영 상태를 누적 기록한다.
  - UI/제품 흐름 수정 전에 먼저 확인하고, 새 피드백을 받으면 이어서 적는다.

### 플랫폼 카탈로그와 출력 경로

* `platform/projects.yaml`
  - 프로젝트, 접근 권한, 환경, 저장소, 정책의 source of truth 다.
  - 현재 기본 seed 프로젝트는 `shared` 하나만 유지한다. 새 정리 작업에서 `project-a` 같은 예전 seed 식별자를 실데이터에 되살리지 않는다.
  - 프로젝트 정책이나 환경 모델을 바꾸면 이 파일 구조와 `backend/internal/project/` 를 같이 본다.
* `platform/clusters.yaml`
  - 클러스터 카탈로그의 source of truth 다.
* `apps/{projectId}/{appName}/`
  - 앱 manifest 산출물 위치다.
  - 보통은 백엔드 store 를 통해 생성되므로, 기능 변경은 이 디렉터리보다 `backend/internal/application/store_*.go` 에서 처리한다.
* `deploy/aods-system/`
  - AODS 자체 배포 manifest 다.
  - 제품 API/포털 기능 개발과는 별도이므로 섞어서 수정하지 않는다.

## 백엔드 인덱스

### 부트스트랩과 라우팅

* `backend/cmd/server/main.go`
  - 서버 시작점이다.
  - config 로딩, git preflight, OIDC preflight, handler 생성, poller 시작이 여기서 일어난다.
* `backend/internal/server/server.go`
  - 의존성 조립과 라우팅 테이블의 중심이다.
  - 새 API endpoint 를 추가할 때 최종적으로 여기에 route 가 연결돼야 한다.

### 공통 코어 계층

* `backend/internal/core/config.go`
  - 런타임 환경변수와 모드 전환 규칙이 있다.
  - Git, Vault, Kubernetes, Prometheus, OIDC 설정을 건드릴 때 먼저 본다.
* `backend/internal/core/auth.go`
* `backend/internal/core/auth_oidc.go`
* `backend/internal/core/identity.go`
  - 현재 사용자 해석과 OIDC 권한 모델 진입점이다.
* `backend/internal/core/http.go`
  - 공통 JSON 응답, 에러 응답, request id, CORS 쪽을 본다.
* `backend/internal/core/secrets.go`
  - Vault staging/final 경로 규칙이 있다.

### 프로젝트 카탈로그와 정책

* `backend/internal/project/service.go`
  - 프로젝트 접근 권한, 환경, 저장소, 정책 모델의 중심이다.
  - `viewer`, `deployer`, `admin` 권한 해석과 project bootstrap 입력 정규화도 여기서 본다.
* `backend/internal/project/http.go`
  - 프로젝트 목록, bootstrap, 환경, 저장소, 정책 API 핸들러다.
* `backend/internal/project/source_local.go`
* `backend/internal/project/source_git.go`
  - `platform/projects.yaml` 읽기/쓰기와 project bootstrap scaffold 보장 경로다.

### 애플리케이션 생성/배포/상태/메트릭

* `backend/internal/application/service.go`
  - 가장 중요한 도메인 서비스다.
  - 앱 생성, archive/delete, 배포, sync 상태, metrics, rollout 제어, rollback policy, events 가 여기서 조합된다.
  - 정책 검증, image 검증, secret staging/finalize, change flow 우회 여부도 여기서 본다.
  - 현재 GitHub 기반 앱 생성은 `repositoryId`보다 `repositoryUrl + repositoryToken + aolda_deploy.json` 흐름이 우선이며, descriptor hydrate 와 저장소 토큰 분리 저장 로직도 여기서 본다.
* `backend/internal/application/http.go`
  - 애플리케이션 관련 API 핸들러다.
* `backend/internal/application/types.go`
  - request/response 와 도메인 모델 정의가 모여 있다.
* `backend/internal/application/store_local.go`
* `backend/internal/application/store_git.go`
  - 앱 manifest 와 deployment metadata 를 실제 파일/Git 에 쓰는 구현이다.
  - manifest 구조를 바꾸는 기능과 archive/delete lifecycle cleanup 은 대체로 여기까지 내려와야 한다.
* `backend/internal/application/store_support.go`
  - store 공통 보조 로직이다.
* `backend/internal/application/flux_support.go`
  - Flux 관련 보조 로직이다.
* `backend/internal/fluxscaffold/scaffold.go`
  - cluster bootstrap 과 app wiring 이 공유하는 Flux scaffold 렌더링/정리 로직이다.
* `backend/internal/application/orphan_cleanup.go`
  - AODS 가 생성한 Flux child manifest orphan cleanup worker 와 정리 기준이 있다.
* `backend/internal/application/metrics_local.go`
* `backend/internal/application/metrics_prometheus.go`
  - 메트릭 read 경로다.
* `backend/internal/application/poller.go`
  - 자동 업데이트 폴러다.
  - legacy project repository 연결과, 앱 메타데이터에 직접 저장된 `repositoryUrl`/`repositoryTokenPath` 기반 sync 둘 다 여기서 처리한다.
* `backend/internal/application/image_verifier.go`
  - 이미지 검증 경로다.

### 변경 관리와 승인 흐름

* `backend/internal/change/service.go`
  - change draft/submitted/approved/merged 상태 전이와 실제 반영 호출이 있다.
* `backend/internal/change/http.go`
  - change API 핸들러다.
* `backend/internal/change/store_local.go`
* `backend/internal/change/store_git.go`
  - 변경 레코드 저장 구현이다.

### 클러스터와 외부 연동 어댑터

* `backend/internal/cluster/service.go`
* `backend/internal/cluster/http.go`
* `backend/internal/cluster/source_local.go`
* `backend/internal/cluster/source_git.go`
  - 클러스터 카탈로그 조회/생성과 Flux bootstrap scaffold write 경로다.
* `backend/internal/gitops/repository.go`
  - Git read/write 어댑터다.
* `backend/internal/kubernetes/local.go`
* `backend/internal/kubernetes/real.go`
  - sync status, rollout 제어, pod metrics 의 local/real 구현이다.
* `backend/internal/vault/local.go`
* `backend/internal/vault/real.go`
  - Secret staging/finalize/delete 저장 구현이다.

### 백엔드 테스트 위치

* `backend/internal/server/server_test.go`
  - API 계약 중심의 통합 테스트 성격이 강하다.
  - endpoint 추가나 응답 shape 변경 시 가장 먼저 같이 본다.
* `backend/internal/server/server_git_test.go`
  - git mode 경로 검증용이다.
* `backend/internal/application/service_test.go`
* `backend/internal/project/service_test.go`
* `backend/internal/gitops/repository_test.go`
* `backend/internal/kubernetes/real_test.go`
* `backend/internal/vault/real_test.go`
* `backend/internal/application/orphan_cleanup_test.go`
  - 도메인 및 어댑터 단위 검증이다.

## 프론트엔드 인덱스

### 실제 활성 진입점

* `frontend/src/main.tsx`
  - React 진입점과 Mantine theme bootstrap 이다.
* `frontend/src/App.tsx`
  - 현재 실제 동작하는 메인 포털 조립 지점이다.
  - 프로젝트 목록, 앱 상세, metrics, rollout 액션, 정책 drawer 같은 실사용 흐름이 여기에 집중돼 있다.
  - 새 UI 기능을 붙일 때 라우터 기반 구조를 가정하지 말고 먼저 이 파일이 현재 활성 경로인지 확인한다.

주의:

* `frontend/src/pages/` 와 `frontend/src/app/layout/` 아래에 분리된 컴포넌트가 있지만, 현재 앱 전체가 완전히 그 구조로 이관된 상태는 아니다.
* 특히 `App.tsx` 에 레거시 인라인 스타일이 남아 있다. 새 기능에서는 이를 더 확산하지 말고, 가능하면 CSS Modules 쪽으로 정리하면서 확장한다.

### 프론트엔드 계약 계층

* `frontend/src/types/api.ts`
  - 프론트엔드가 믿는 API 타입 계약이다.
  - `openapi.yaml` 변경 후 가장 먼저 맞춰야 하는 파일 중 하나다.
* `frontend/src/api/client.ts`
  - fetch 래퍼와 모든 API 메서드가 있다.
  - 새 endpoint 추가 시 프론트에서 가장 먼저 진입하는 파일이다.
* `frontend/src/auth/oidc.ts`
  - OIDC 인증 경로다.

### 프론트엔드 UI 조각

* `frontend/src/components/ApplicationWizard.tsx`
  - 앱 생성 위저드다.
  - create request 필드가 바뀌면 거의 반드시 여기와 `types/api.ts` 를 같이 바꿔야 한다.
* `frontend/src/components/ui/`
* `frontend/src/components/layout/`
* `frontend/src/components/navigation/`
  - 재사용 UI 조각들이다.
* `frontend/src/pages/projects/ProjectsWorkspace.tsx`
* `frontend/src/pages/changes/ChangesWorkspace.tsx`
* `frontend/src/pages/clusters/ClustersPage.tsx`
* `frontend/src/pages/me/MePage.tsx`
  - 분리 중인 화면 단위 컴포넌트들이다.
  - 새 기능이 페이지 분리에 맞다면 여기로 옮기되, 실제 연결 지점은 `App.tsx` 에서 확인한다.
* `frontend/user-stories.json`
  - 프론트 critical user story manifest 다.
  - 새 사용자 흐름이나 권한/상태 규칙을 추가하면 이 파일도 같이 갱신한다.
* `frontend/src/testing/`
  - 프론트 공통 테스트 유틸과 setup 이 있는 위치다.
  - 새 화면 테스트는 여기 규칙을 재사용한다.

### 프론트엔드 스타일과 검증

* `frontend/src/App.module.css`
* `frontend/src/app/layout/AppShell.module.css`
* `frontend/src/index.css`
  - 스타일 진입점이다.
* `frontend/package.json`
  - 프론트 검증 스크립트는 `lint`, `test`, `test:stories`, `build`, `check` 중심이다.
  - 루트 `make check` 는 백엔드 `go vet ./...` + `go test ./...`, 프론트 `npm run check` 를 함께 수행한다.
* `frontend/scripts/check-user-story-coverage.mjs`
  - `frontend/user-stories.json` 의 critical story 들이 실제 테스트 제목에 매핑됐는지 검사한다.

## 기능별 수정 경로

### 새 API endpoint 추가

아래 순서로 본다.

1. `docs/internal-platform/openapi.yaml`
2. `backend/internal/server/server.go`
3. 대상 도메인의 `backend/internal/*/http.go`
4. 대상 도메인의 `backend/internal/*/service.go`
5. 필요 시 `store_*.go`, `source_*.go`, `kubernetes/*.go`, `vault/*.go`
6. `frontend/src/types/api.ts`
7. `frontend/src/api/client.ts`
8. `frontend/src/App.tsx` 또는 실제 연결된 화면 컴포넌트
9. `backend/internal/server/server_test.go`

### 프로젝트 정책/환경/권한 변경

아래 파일을 같이 본다.

* `platform/projects.yaml`
* `backend/internal/project/service.go`
* `backend/internal/project/http.go`
* `backend/internal/project/source_*.go`
* `frontend/src/types/api.ts`
* `frontend/src/api/client.ts`
* `frontend/src/App.tsx`

### 앱 생성 필드나 manifest 구조 변경

아래 파일을 같이 본다.

* `docs/internal-platform/openapi.yaml`
* `backend/internal/application/types.go`
* `backend/internal/application/http.go`
* `backend/internal/application/service.go`
* `backend/internal/application/store_local.go`
* `backend/internal/application/store_git.go`
* `backend/internal/core/secrets.go`
* `frontend/src/types/api.ts`
* `frontend/src/components/ApplicationWizard.tsx`
* `frontend/src/api/client.ts`
* `backend/internal/server/server_test.go`

### 배포 상태, rollout, promote/abort, metrics 변경

아래 파일을 같이 본다.

* `backend/internal/application/service.go`
* `backend/internal/application/http.go`
* `backend/internal/kubernetes/local.go`
* `backend/internal/kubernetes/real.go`
* `backend/internal/application/metrics_local.go`
* `backend/internal/application/metrics_prometheus.go`
* `frontend/src/types/api.ts`
* `frontend/src/api/client.ts`
* `frontend/src/App.tsx`

### Secret / Vault 흐름 변경

아래 파일을 같이 본다.

* `docs/domain-rules.md`
* `docs/phase1-decisions.md`
* `backend/internal/core/secrets.go`
* `backend/internal/application/service.go`
* `backend/internal/vault/local.go`
* `backend/internal/vault/real.go`

### Git write / GitOps 동작 변경

아래 파일을 같이 본다.

* `docs/domain-rules.md`
* `backend/internal/gitops/repository.go`
* `backend/internal/application/store_git.go`
* `backend/internal/project/source_git.go`
* `backend/internal/change/store_git.go`

## 기능 개발 체크리스트

기능 개발을 마치기 전에 아래를 확인한다.

* `openapi.yaml` 과 실제 응답 shape 가 일치하는가
* 최소 계약을 깨는 임의의 미래 기능을 무단으로 열지 않았는가
* 이미 노출된 later-phase 동작을 잘못된 baseline 가정 때문에 축소하거나 제거하지 않았는가
* `platform/projects.yaml` 또는 GitOps 경로 규칙을 깨지 않았는가
* Secret 이 Git 에 들어가거나 K8s `Secret` 으로 우회되지 않았는가
* 프론트 타입과 API client 가 백엔드 응답과 같이 갱신됐는가
* `server_test.go` 또는 도메인 테스트를 같이 보강했는가
* 실수나 문서 오해가 있었다면 `docs/mistake-log.md` 에 남겼는가
