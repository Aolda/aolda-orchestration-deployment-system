# Current Implementation Status

이 문서는 `docs/internal-platform/openapi.yaml`, `backend/`, `frontend/`, `platform/`, 테스트 코드, 그리고 git 히스토리를 기준으로 현재 구현 상태를 정리한 코드 우선(status-by-code) 문서다.

목적은 두 가지다.

1. 현재 레포가 실제로 어디까지 구현됐는지 코드 기준으로 오해 없이 전달한다.
2. Phase 1/2/3/4를 앞으로 구현할 단계로 잘못 읽고 이미 들어온 기능을 다시 미래 기능으로 취급하는 일을 막는다.

주의:

* 이 문서는 `기획 우선순위`가 아니라 `현재 코드 상태`를 설명한다.
* Phase 표기는 이제 active delivery gate 가 아니라 historical planning bucket 이다.
* `docs/future-phases-roadmap.md` 는 현재 구조가 왜 이렇게 확장됐는지 설명하는 배경 문서로 읽는다.
* 현재 작업 기준은 `post-phase product hardening` 이다. 이미 코드와 UI에 노출된 기능은 모두 regression baseline 이다.

## 요약

현재 레포의 **active baseline** 은 Phase 1/2/3/4 이후의 통합 플랫폼 상태다.
유지보수, QA, 회귀 판단 기준은 현재 코드와 UI에 이미 노출된 기능 전체다.

가장 정확한 읽기 방식은 아래와 같다.

* **Foundation deployment flow**: baseline
  - 프로젝트/앱 조회, 앱 생성, 재배포, sync status, metrics, Git/Vault/K8s/Prometheus 실연동 경로
* **Progressive delivery**: baseline
  - canary, rollout 상태, promote, abort, deployment history
* **Reviewed changes and environments**: baseline
  - environments, review-gated change flow, project policy/repository 모델, diff preview, submit/approve/merge
* **Platform ops and guardrails**: baseline
  - cluster/project bootstrap, policy enforcement, rollback policy, auto rollback executor, cleanup workers
* **Frontend operations console**: baseline
  - AppShell 기반 전역 섹션, Projects/Changes/Clusters/Me workspace, 앱 운영 Drawer, logs/diagnostics, LoadBalancer workflow

주의할 점:

* `pull_request` write mode 는 현재 제품에서 **review-gated change flow** 로 동작한다. 실제 GitHub Pull Request 생성/머지는 현재 baseline 의 필수 의미가 아니다.
* 일부 frontend 기능은 `frontend/src/app/appConfig.ts` feature flag 로 숨겨져 있을 수 있다. 코드/API가 있더라도 현재 product surface 에 노출됐는지는 flag 를 같이 확인한다.
* `Standard` 전략명은 legacy alias 로 보고, 현재 OpenAPI/backend 계약의 전략명은 `Rollout` / `Canary` 다. 프론트 노출 전략은 `frontend/src/app/appConfig.ts` 의 `supportedDeploymentStrategies` 를 함께 확인한다.

## Recent Stability Work

### 2026-05-12 - Argo CD app-of-apps 배포 진입점 추가

이번 작업에서는 AODS 자체를 Argo CD root application 하나로 배포할 수 있는 app-of-apps scaffold 를 추가했다.

무엇을 바꿨는가:

* `deploy/argocd/aods-root.yaml` 이 `aods-root` root `Application` 으로 `deploy/argocd/apps` 아래 child app 을 관리한다.
* `deploy/argocd/apps/aods-system.yaml` 은 AODS 런타임을 `deploy/aods-system/overlays/argocd` 에서 배포한다.
* Argo CD overlay 는 backend/frontend `Service` 를 명시적으로 `ClusterIP` 로 고정해 NodePort/LoadBalancer 포트를 새로 점유하지 않는다.
* DB 는 기본 app-of-apps 에 포함하지 않고, 기존 DB 를 `aods-backend-secrets` 의 `AODS_MARIADB_DSN` 또는 `AODS_APPLICATION_CATALOG_DSN` 으로 주입하는 방식을 기준으로 둔다.

주요 근거:

* root app: [deploy/argocd/aods-root.yaml](/Users/ichanju/Desktop/aolda/AODS/deploy/argocd/aods-root.yaml:1)
* child app: [deploy/argocd/apps/aods-system.yaml](/Users/ichanju/Desktop/aolda/AODS/deploy/argocd/apps/aods-system.yaml:1)
* Argo CD overlay: [deploy/aods-system/overlays/argocd/kustomization.yaml](/Users/ichanju/Desktop/aolda/AODS/deploy/aods-system/overlays/argocd/kustomization.yaml:1)
* 운영 안내: [deploy/argocd/README.md](/Users/ichanju/Desktop/aolda/AODS/deploy/argocd/README.md:1)

주의:

* `AODS_GIT_REMOTE`, `AODS_VAULT_TOKEN`, 선택 `AODS_MARIADB_DSN`, 선택 `AODS_APPLICATION_CATALOG_DSN` 은 Secret 평문이므로 Git에 저장하지 않는다.
* app-of-apps 는 DB 를 새로 띄우지 않으므로, 이미 운영 중인 PostgreSQL/MariaDB 와 포트/서비스 충돌을 만들지 않는다.

### 2026-05-12 - MariaDB 기반 배포 operation queue 기준선 추가

이번 작업에서는 `POST /api/v1/applications/{applicationId}/deployments` 경로에 durable operation queue 를 추가했다.

무엇을 바꿨는가:

* `AODS_MARIADB_DSN` 이 설정된 경우 재배포 요청은 MariaDB `aods_deployment_operations` 에 먼저 저장된다.
* deployment worker 는 `SELECT ... FOR UPDATE SKIP LOCKED` 로 due operation 을 선점한다.
* Git desired state write 는 operation lock table 의 lease 로 repo/branch 단위 single-writer 처리를 한다.
* operation 상태 전이는 `version`, `lease_owner`, `lease_until` 을 기준으로 보호한다.
* DB 미설정 환경은 기존 동기 배포 경로를 유지한다.
* deployment history 는 Git 기록과 아직 Git에 쓰이지 않은 `Queued / Running / Retrying / Failed` operation 을 병합해서 보여준다.
* `AODS_APPLICATION_CATALOG_DSN` 이 설정된 경우 앱 목록 UI read model 은 PostgreSQL 또는 MariaDB projection 을 먼저 읽고, background projector 가 GitHub 기본 브랜치 내용을 주기적으로 반영한다.

주요 근거:

* operation worker/types: [backend/internal/application/deployment_operations.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/deployment_operations.go:1)
* MariaDB store: [backend/internal/application/deployment_operations_mariadb.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/deployment_operations_mariadb.go:1)
* service enqueue/execute split: [backend/internal/application/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/service.go:417)
* dependency wiring: [backend/internal/server/server.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server.go:1)

주의:

* GitHub 기본 브랜치가 desired state 의 source of truth 라는 규칙은 유지된다.
* PostgreSQL/MariaDB projection 은 UI read model 이며, MariaDB 는 durable command log, retry queue, Git write 직렬화 coordinator 로도 사용될 수 있다.

### 2026-04-24 - 모니터링 알림/진단 API 기준선 추가

이번 작업에서는 기존 `ServiceMonitor + metrics 조회 + UI polling` 구조에 운영 알림과 진단 기준선을 추가했다.

무엇을 바꿨는가:

* 앱 GitOps base 산출물에 `PrometheusRule` 을 추가 생성한다.
* 기본 alert 는 높은 5xx 비율, 높은 p95 latency, pod restart, pod 비정상 상태, Flux degraded 상태를 포함한다.
* `GET /api/v1/projects/{projectId}/health` 로 앱별 sync/metrics health signal 을 한 번에 조회한다.
* project health snapshot 은 앱별 metric series 와 최신 deployment 요약도 포함하므로, 프론트 프로젝트 모니터링 refresh 가 앱별 `metrics/deployments` 반복 호출을 피할 수 있다.
* `GET /api/v1/applications/{applicationId}/metrics/diagnostics` 로 expected scrape target 과 metric series 수집 상태를 진단한다.
* frontend API client/type 에 새 endpoint 계약을 연결하고, 프로젝트 모니터링 polling 을 snapshot 기반으로 전환했다.

주요 근거:

* alert manifest 렌더링: [backend/internal/application/store_local.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/store_local.go:1081)
* health/diagnostics 서비스: [backend/internal/application/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/service.go:689)
* routes: [backend/internal/server/server.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server.go:210)
* API 계약: [docs/internal-platform/openapi.yaml](/Users/ichanju/Desktop/aolda/AODS/docs/internal-platform/openapi.yaml:356)

검증:

* `make check-observability` 통과
  - backend route/store regression: `PrometheusRule`, project health, metrics diagnostics
  - backend domain compile guard: `application`, `vault`
  - manifest validation: generated `ServiceMonitor` / `PrometheusRule`
  - frontend lint/build: health snapshot API client/type wiring
* `go test ./internal/application ./internal/server` 통과
* `make check` 통과

검증 매트릭스:

| 변경 | 검증 위치 | 확인 내용 |
| --- | --- | --- |
| `prometheusrule.yaml` 생성 | `backend/internal/server/server_test.go`, `backend/internal/server/server_git_test.go`, `scripts/validate-manifests.sh` | local/git mode 앱 생성 시 파일 생성, alert 이름 포함, rendered manifest schema 통과 |
| project health snapshot | `backend/internal/server/server_test.go`, `frontend/src/App.tsx` build | `metrics`, `latestDeployment`, `signals` 응답 포함 및 프론트 프로젝트 모니터링 refresh 연결 |
| metrics diagnostics | `backend/internal/server/server_test.go`, `frontend/src/api/client.ts` build | scrape target, status, series diagnostics 응답 및 API client 타입 연결 |
| PrometheusRule CRD 검증 | `ci/kubeconform-schemas/prometheusrule-*.json`, `ci/crds/validation-crds.yaml` | kubeconform/server dry-run 하네스가 새 CRD를 인식 |

### 2026-04-17 - 백엔드 CORS dev-port 회귀 수정 및 실런타임 검증

이번 안정화 작업에서는 `frontend-run` 이 `5173` 대신 `5174` 같은 fallback port 로 뜰 때,
OIDC fallback 로그인 이후 bootstrap API 호출이 전부 CORS 로 막히는 실회귀를 수정했다.

무엇을 바꿨는가:

* backend CORS middleware 가 단일 `AODS_ALLOWED_ORIGIN` 문자열만 그대로 내려주던 동작을 수정했다.
* `AODS_ALLOWED_ORIGIN` 은 이제 comma-separated origin 목록을 처리한다.
* `AODS_ALLOW_DEV_FALLBACK=true` 인 개발 모드에서는 `localhost` 와 `127.0.0.1` loopback origin 을 요청 origin 기준으로 허용한다.
* 허용되지 않은 외부 origin 은 `Access-Control-Allow-Origin` 을 받지 못하도록 유지했다.
* 회귀 방지용 CORS 단위 테스트를 추가했다.

주요 근거:

* CORS origin 선택 로직: [backend/internal/core/http.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/core/http.go:42)
* CORS 회귀 테스트: [backend/internal/core/http_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/core/http_test.go:9)
* server wiring: [backend/internal/server/server.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server.go:206)

검증:

* `make check` 통과
* backend total coverage: `42.5%`
* patched backend 를 `:28081`, frontend 를 `:5175` 로 띄워 실제 브라우저 로그인 검증 수행
* Playwright 기준 emergency login 후 프로젝트 메인 화면 정상 진입 확인. 현재 프론트 로컬 기본 자격 증명은 `admin / qwe1356@` 이다.
* 네트워크 기준 `/api/v1/me`, `/api/v1/projects`, `/api/v1/clusters` 포함 bootstrap 요청이 모두 `200 OK`
* `Origin: http://localhost:5174` 요청에 `Access-Control-Allow-Origin: http://localhost:5174` 응답 확인
* `Origin: http://malicious.example` 요청에는 allow-origin header 가 내려오지 않음을 확인
* `/api/v1/projects` 실측 응답 시간: 약 `0.000846s`

추가 관찰:

* 로그인 후 프로젝트/메트릭스 조회는 `frontend/src/app/appConfig.ts` 의 polling interval 을 따른다. 현재 기본값은 15초이며, 다음 프론트 안정화 라운드에서는 과호출과 bundle size 최적화를 별도 점검할 필요가 있다.

아래 섹션은 더 이상 Phase 진행률을 판정하지 않고, 현재 baseline 의 주요 축을 코드 위치와 함께 정리한다.

## Deployment Foundation

현재 기본 배포 플로우는 baseline 이다.

포함된 것:

* 프로젝트 목록 조회
* 프로젝트 앱 목록 조회
* 앱 생성
* Secret staging/finalize 처리
* GitOps manifest 생성
* Flux child `Kustomization` wiring
* 재배포
* sync status 조회
* metrics 조회

주요 근거:

* 라우팅: [backend/internal/server/server.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server.go:239)
* 앱 생성/재배포 서비스: [backend/internal/application/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/service.go:180)
* manifest 및 metadata/deployment 기록: [backend/internal/application/store_local.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/store_local.go:69)
* Git mode 검증: [backend/internal/server/server_git_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server_git_test.go:16)
* 기본 시나리오 테스트: [backend/internal/server/server_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server_test.go:619)

## Progressive Delivery

Progressive delivery 는 현재 baseline 이다.

포함된 것:

* `Rollout` / `Canary` 전략
* `rollout.yaml` 생성
* canary service / Istio virtual service / destination rule 생성
* deployment history 조회
* deployment detail 조회
* `promote`
* `abort`
* rollout 상태 필드(`rolloutPhase`, `currentStep`, `canaryWeight`, `stableRevision`, `canaryRevision`)
* 프론트의 canary 상태/승격 UI

주요 근거:

* OpenAPI endpoints: [docs/internal-platform/openapi.yaml](/Users/ichanju/Desktop/aolda/AODS/docs/internal-platform/openapi.yaml:446)
* patch/promote/abort/deployment 서비스: [backend/internal/application/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/service.go:617)
* rollout manifest 렌더링: [backend/internal/application/store_local.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/store_local.go:913)
* real Argo rollout adapter: [backend/internal/kubernetes/real.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/kubernetes/real.go:1800)
* local rollout adapter: [backend/internal/kubernetes/local.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/kubernetes/local.go:42)
* canary API 테스트: [backend/internal/server/server_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server_test.go:1239)

운영 메모:

* 로컬 모드에서는 rollout 상태가 local adapter 기준으로 반환된다.
* 실운영 의미의 canary 상태와 promote/abort 는 Kubernetes/Argo Rollouts 연동이 켜져 있어야 실제 동작 의미가 생긴다.
* 긴급 중단/직전 버전 롤백 UI 는 코드에 있지만 `frontend/src/app/appConfig.ts` 의 `showEmergencyActionControls` flag 로 현재 product surface 에서는 숨겨질 수 있다.

## Reviewed Changes And Environments

Change / environment / policy 흐름은 현재 baseline 이다.

포함된 것:

* 프로젝트별 environments
* `direct` / `pull_request` write mode 필드
* repositories 조회
* `Change` 리소스 생성
* submit / approve / merge 상태 전이
* diff preview 데이터
* `admin` 승인 권한 분리
* 환경 전환 redeploy
* 환경별 cluster 연결
* frontend `ChangesWorkspace` 의 필터, 상세, diff preview, submit/approve/merge 액션

주요 근거:

* catalog 구조: [platform/projects.yaml](/Users/ichanju/Desktop/aolda/AODS/platform/projects.yaml:1)
* project service의 environments/repositories/policies 모델: [backend/internal/project/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/project/service.go:37)
* change lifecycle: [backend/internal/change/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/change/service.go:35)
* routes: [backend/internal/server/server.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server.go:248)
* 프로젝트 탭 구조: [frontend/src/pages/projects/ProjectsWorkspace.tsx](/Users/ichanju/Desktop/aolda/AODS/frontend/src/pages/projects/ProjectsWorkspace.tsx:1)
* 변경 요청 workspace: [frontend/src/pages/changes/ChangesWorkspace.tsx](/Users/ichanju/Desktop/aolda/AODS/frontend/src/pages/changes/ChangesWorkspace.tsx:1)
* change flow 테스트: [backend/internal/server/server_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server_test.go:1585)

운영 메모:

* 현재 `pull_request` 는 review-gated change flow 를 뜻한다.
* `merge` 는 승인 조건을 확인한 뒤 실제 앱/정책 mutation 을 적용한다. 이 동작은 baseline 이며, 실제 GitHub Pull Request 생성/머지와 동일한 뜻으로 해석하지 않는다.
* 프로젝트별 change 목록 API 는 아직 별도 endpoint 가 없으므로, 프론트는 현재 세션에서 생성했거나 수동으로 불러온 change 를 sessionStorage 기준으로 추적한다.

## Platform Ops And Guardrails

cluster / policy / rollback / cleanup guardrail 은 현재 baseline 이다.

포함된 것:

* cluster catalog
* project policy read/update
* allowed environment / strategy / cluster target enforcement
* required probes 정책 반영
* rollback policy 저장/조회
* application events 저장/조회
* cluster-aware Flux bootstrap / child kustomization wiring
* platform admin cluster bootstrap API
* platform admin project bootstrap API
* application archive/delete lifecycle
* stale secret cleanup worker
* orphan Flux manifest cleanup worker
* poller 기반 auto rollback executor

주요 근거:

* cluster route: [backend/internal/server/server.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server.go:242)
* cluster bootstrap service/http/store: [backend/internal/cluster/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/cluster/service.go:1), [backend/internal/cluster/http.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/cluster/http.go:1)
* project bootstrap service/http/store: [backend/internal/project/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/project/service.go:130), [backend/internal/project/http.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/project/http.go:75)
* policy 모델: [backend/internal/project/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/project/service.go:45)
* rollback/events API: [backend/internal/application/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/service.go:1120)
* auto rollback executor: [backend/internal/application/poller.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/poller.go:19)
* Flux cluster wiring: [backend/internal/application/flux_support.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/flux_support.go:39), [backend/internal/fluxscaffold/scaffold.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/fluxscaffold/scaffold.go:1)
* stale secret cleanup worker: [backend/internal/vault/cleanup.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/vault/cleanup.go:1)
* orphan Flux manifest cleanup worker: [backend/internal/application/orphan_cleanup.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/orphan_cleanup.go:1)
* cluster catalog UI: [frontend/src/pages/clusters/ClustersPage.tsx](/Users/ichanju/Desktop/aolda/AODS/frontend/src/pages/clusters/ClustersPage.tsx:1)

운영 메모:

* `autoRollbackEnabled` 와 rollback thresholds 는 저장/표시를 넘어 poller 가 평가하고 자동 재배포까지 수행한다.
* 실행기는 프로젝트 정책, 앱 rollback policy, 이전 deployment history, metrics reader 가 모두 준비된 경우에 동작한다.
* rollback policy UI, service mesh UI, 새 프로젝트 UI, 긴급 조치 UI 일부는 feature flag 로 숨겨질 수 있다. API/서비스 baseline 과 현재 노출 surface 를 구분해서 본다.

## Observability And Runtime Evidence

관측성도 현재 baseline 이다.

포함된 것:

* 앱별 `ServiceMonitor`
* 앱별 `PrometheusRule`
* project health snapshot
* metrics diagnostics
* application events
* 최근 컨테이너 로그 snapshot
* pod/container log target 선택
* SSE 기반 log stream
* container resource usage vs request/limit 표시

주요 근거:

* metrics/diagnostics/logs routes: [backend/internal/server/server.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server.go:273)
* diagnostics service: [backend/internal/application/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/service.go:1136)
* logs service: [backend/internal/application/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/service.go:1434)
* Kubernetes pod log reader: [backend/internal/kubernetes/real.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/kubernetes/real.go:677)
* frontend log UI: [frontend/src/App.tsx](/Users/ichanju/Desktop/aolda/AODS/frontend/src/App.tsx:3952)
* operations route test: [backend/internal/server/server_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server_test.go:1286)

운영 메모:

* Kubernetes/Prometheus 가 연결되지 않은 환경에서는 fabricated runtime data 를 만들지 않고 empty/unknown 상태로 드러낸다.
* 로그 조회 실패는 stale pod/container, Kubernetes API 미설정, integration error 를 구분해 표시한다.

## 문서 해석 가이드

현재 문서는 아래처럼 읽어야 한다.

* [docs/internal-platform/prd.md](/Users/ichanju/Desktop/aolda/AODS/docs/internal-platform/prd.md) 와 [docs/acceptance-criteria.md](/Users/ichanju/Desktop/aolda/AODS/docs/acceptance-criteria.md) 는 여전히 **Phase 1 최소 계약 문서**다. current baseline 설명 문서가 아니다.
* [docs/internal-platform/openapi.yaml](/Users/ichanju/Desktop/aolda/AODS/docs/internal-platform/openapi.yaml) 는 현재 active API contract 다.
* [docs/future-phases-roadmap.md](/Users/ichanju/Desktop/aolda/AODS/docs/future-phases-roadmap.md) 는 historical roadmap / 설계 배경 문서다.
* [docs/current-baseline-runbook.md](/Users/ichanju/Desktop/aolda/AODS/docs/current-baseline-runbook.md) 는 현재 **current implementation baseline handoff 문서**다

## git 히스토리 근거

아래 커밋들이 current baseline 이 historical Phase 1 범위를 넘어 확장됐음을 보여준다.

* `68f5220` `feat: phase 2-4 플랫폼 운영 기능을 확장한다`
* `9e028b9` `fix: canary flux 동기화 대기 문제를 해소한다`
* `9d7907e` `fix: 변경 상태 전이와 정책 회귀 테스트를 보강한다`
* `1dcdf2c` `feat: 실연동 경로와 저장소 기반 자동화를 확장한다`
* `27f09a2` `chore: harden deployment ops and coverage gate`
* `fadd335` `refactor: split app shell components`

즉, 현재 mismatch 는 `코드는 post-phase baseline 으로 확장됐는데 일부 handoff 문서가 Phase 진행 중 설명을 유지한 것`에 가깝다.

## 다음 정리 원칙

앞으로 문서를 정리할 때는 아래 기준을 권장한다.

1. Phase 1 문서는 `historical minimum contract` 로 남긴다.
2. 현재 구현과 QA baseline 은 이 문서를 기준으로 설명한다.
3. `pull_request` write mode 는 `review-gated change flow` 라고 명확히 적는다.
4. 기능이 코드에 있지만 feature flag 로 숨겨진 경우 `implemented but hidden from current product surface` 라고 구분한다.
5. 앞으로의 작업은 새 phase 구현이 아니라 hardening, 운영 UX, 실연동 신뢰도, 코드 구조 정리로 표현한다.
