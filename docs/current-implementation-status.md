# Current Implementation Status

이 문서는 `docs/internal-platform/openapi.yaml`, `backend/`, `frontend/`, `platform/`, 테스트 코드, 그리고 git 히스토리를 기준으로 현재 구현 상태를 정리한 코드 우선(status-by-code) 문서다.

목적은 두 가지다.

1. 현재 레포가 실제로 어디까지 구현됐는지 Phase 기준으로 오해 없이 전달한다.
2. `Phase 1만 구현된 저장소`라는 오래된 전제를 그대로 따라가며 이미 들어온 기능을 다시 미래 기능으로 취급하는 일을 막는다.

주의:

* 이 문서는 `기획 우선순위`가 아니라 `현재 코드 상태`를 설명한다.
* `docs/future-phases-roadmap.md` 는 여전히 설계 기준 문서다.
* 다만 roadmap 에 적힌 기능 중 일부는 이미 코드에 들어와 있으므로, roadmap 을 `아직 시작 안 한 일 목록`으로 읽으면 안 된다.

## 요약

현재 레포의 **active baseline** 은 더 이상 `Phase 1 MVP only` 가 아니다.
유지보수, QA, 회귀 판단 기준은 현재 코드와 UI에 이미 노출된 기능 전체다.

가장 정확한 읽기 방식은 아래와 같다.

* **Foundation deployment flow**: 완료
  - 프로젝트/앱 조회, 앱 생성, 재배포, sync status, metrics, Git/Vault/K8s/Prometheus 실연동 경로
* **Progressive delivery**: 상당 부분 구현됨
  - canary, rollout 상태, promote, abort, deployment history
* **Reviewed changes and environments**: 부분 구현됨
  - environments, reviewed change flow, project policy/repository 모델
* **Platform ops and guardrails**: 대부분 구현됨
  - cluster/project bootstrap, policy enforcement, rollback policy, auto rollback executor, cleanup workers

## Recent Stability Work

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
* Playwright 기준 `admin/admin` emergency login 후 프로젝트 메인 화면 정상 진입 확인
* 네트워크 기준 `/api/v1/me`, `/api/v1/projects`, `/api/v1/clusters` 포함 bootstrap 요청이 모두 `200 OK`
* `Origin: http://localhost:5174` 요청에 `Access-Control-Allow-Origin: http://localhost:5174` 응답 확인
* `Origin: http://malicious.example` 요청에는 allow-origin header 가 내려오지 않음을 확인
* `/api/v1/projects` 실측 응답 시간: 약 `0.000846s`

추가 관찰:

* 로그인 후 프로젝트/메트릭스 조회가 5초 주기 polling 으로 반복되므로, 다음 프론트 안정화 라운드에서는 과호출과 bundle size 최적화를 별도 점검할 필요가 있다.

아래 Phase 섹션은 historical roadmap bucket 과 현재 구현을 매핑해 설명하기 위한 것이다.

## Phase 1

코드 기준으로 보면 Phase 1 핵심 플로우는 구현되어 있고 테스트도 있다.

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

* 라우팅: [backend/internal/server/server.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server.go:141)
* 앱 생성/재배포 서비스: [backend/internal/application/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/service.go:207)
* manifest 및 metadata/deployment 기록: [backend/internal/application/store_local.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/store_local.go:69)
* Git mode 검증: [backend/internal/server/server_git_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server_git_test.go:16)
* 기본 Phase 1 시나리오 테스트: [backend/internal/server/server_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server_test.go:95)

판정:

* **구현됨**

## Phase 2

roadmap 기준의 Progressive Delivery 기능은 이미 여러 층에 들어와 있다.

이미 구현된 것:

* `Canary` 전략 허용
* `rollout.yaml` 생성
* canary service / Istio virtual service / destination rule 생성
* deployment history 조회
* deployment detail 조회
* `promote`
* `abort`
* rollout 상태 필드(`rolloutPhase`, `currentStep`, `canaryWeight`, `stableRevision`, `canaryRevision`)
* 프론트의 canary 상태/긴급 조치 UI

주요 근거:

* roadmap 정의: [docs/future-phases-roadmap.md](/Users/ichanju/Desktop/aolda/AODS/docs/future-phases-roadmap.md:63)
* OpenAPI endpoints: [docs/internal-platform/openapi.yaml](/Users/ichanju/Desktop/aolda/AODS/docs/internal-platform/openapi.yaml:230)
* patch/promote/abort/deployment 서비스: [backend/internal/application/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/service.go:298)
* rollout manifest 렌더링: [backend/internal/application/store_local.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/store_local.go:607)
* real Argo rollout adapter: [backend/internal/kubernetes/real.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/kubernetes/real.go:1014)
* local rollout adapter: [backend/internal/kubernetes/local.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/kubernetes/local.go:42)
* canary artifact 테스트: [backend/internal/server/server_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server_test.go:334)
* 프론트 rollout/abort/rollback UI: [frontend/src/App.tsx](/Users/ichanju/Desktop/aolda/AODS/frontend/src/App.tsx:349), [frontend/src/App.tsx](/Users/ichanju/Desktop/aolda/AODS/frontend/src/App.tsx:820)

주의할 점:

* 로컬 모드에서는 rollout 상태가 스텁으로 반환된다.
* 실운영 의미의 canary 상태와 promote/abort 는 Kubernetes/Argo Rollouts 연동이 켜져 있어야 실제 동작 의미가 생긴다.

판정:

* **상당 부분 구현됨**

## Phase 3

roadmap 기준의 Change Management / Environments 기능도 이미 적지 않게 들어와 있다.

이미 구현된 것:

* 프로젝트별 environments
* `direct` / `pull_request` write mode 필드
* repositories 조회
* `Change` 리소스 생성
* submit / approve / merge 상태 전이
* diff preview 데이터
* `admin` 승인 권한 분리
* 환경 전환 redeploy
* 환경별 cluster 연결

주요 근거:

* roadmap 정의: [docs/future-phases-roadmap.md](/Users/ichanju/Desktop/aolda/AODS/docs/future-phases-roadmap.md:208)
* catalog 구조: [platform/projects.yaml](/Users/ichanju/Desktop/aolda/AODS/platform/projects.yaml:13)
* project service의 environments/repositories/policies 모델: [backend/internal/project/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/project/service.go:37)
* change lifecycle: [backend/internal/change/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/change/service.go:35)
* later-phase routes: [backend/internal/server/server.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server.go:144)
* UI의 environments/repositories/policies fetch: [frontend/src/App.tsx](/Users/ichanju/Desktop/aolda/AODS/frontend/src/App.tsx:280)
* 프로젝트 탭 구조: [frontend/src/pages/projects/ProjectsWorkspace.tsx](/Users/ichanju/Desktop/aolda/AODS/frontend/src/pages/projects/ProjectsWorkspace.tsx:5)
* PR-mode change review 테스트: [backend/internal/server/server_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server_test.go:453)
* 환경 전환 redeploy 테스트: [backend/internal/server/server_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server_test.go:923)

중요한 한계:

* 현재 `pull_request` 는 **실제 GitHub Pull Request 생성 흐름이 아니다.**
* 현재 구현은 `Change` 객체로 승인 게이트를 둔 뒤, `merge` 시점에 실제 mutation 을 바로 적용한다.
* 즉, `PR mode` 라는 이름은 있지만 현재는 **review-gated direct apply** 에 더 가깝다.

관련 근거:

* `merge` 시 실제 앱/정책 mutation 호출: [backend/internal/change/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/change/service.go:135)
* 프론트도 “변경 요청 목록 없이 선택된 변경 객체 중심”이라고 명시: [frontend/src/pages/changes/ChangesWorkspace.tsx](/Users/ichanju/Desktop/aolda/AODS/frontend/src/pages/changes/ChangesWorkspace.tsx:17)

판정:

* **부분 구현됨**
* 실제 GitHub PR workflow 는 아직 미구현

## Phase 4

roadmap 기준의 cluster / policy / rollback guardrail 기능도 일부 들어와 있다.

이미 구현된 것:

* cluster catalog
* project policy read/update
* allowed environment / strategy / cluster target enforcement
* required probes 정책 반영
* rollback policy 저장/조회
* application events 저장/조회
* cluster-aware Flux bootstrap / child kustomization wiring
* platform admin cluster bootstrap API

주요 근거:

* roadmap 정의: [docs/future-phases-roadmap.md](/Users/ichanju/Desktop/aolda/AODS/docs/future-phases-roadmap.md:344)
* cluster route: [backend/internal/server/server.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server.go:142)
* cluster bootstrap service/http/store: [backend/internal/cluster/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/cluster/service.go:1), [backend/internal/cluster/http.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/cluster/http.go:1), [backend/internal/cluster/source_local.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/cluster/source_local.go:1), [backend/internal/cluster/source_git.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/cluster/source_git.go:1)
* policy 모델: [backend/internal/project/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/project/service.go:45)
* rollback/events API: [backend/internal/application/service.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/service.go:515)
* Flux cluster wiring: [backend/internal/application/flux_support.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/flux_support.go:39), [backend/internal/fluxscaffold/scaffold.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/fluxscaffold/scaffold.go:1)
* stale secret cleanup worker: [backend/internal/vault/cleanup.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/vault/cleanup.go:1), [backend/internal/vault/local.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/vault/local.go:79), [backend/internal/vault/real.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/vault/real.go:196)
* orphan Flux manifest cleanup worker: [backend/internal/application/orphan_cleanup.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/orphan_cleanup.go:1)
* cluster catalog UI: [frontend/src/pages/clusters/ClustersPage.tsx](/Users/ichanju/Desktop/aolda/AODS/frontend/src/pages/clusters/ClustersPage.tsx:10)
* rollback policy UI: [frontend/src/App.tsx](/Users/ichanju/Desktop/aolda/AODS/frontend/src/App.tsx:874)
* 정책/rollback/cleanup 관련 테스트: [backend/internal/server/server_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server_test.go:413), [backend/internal/server/server_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server_test.go:1229), [backend/internal/server/server_git_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server_git_test.go:1), [backend/internal/vault/local_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/vault/local_test.go:1), [backend/internal/vault/real_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/vault/real_test.go:117), [backend/internal/application/orphan_cleanup_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/orphan_cleanup_test.go:1)

중요한 한계:

* `autoRollbackEnabled` 와 rollback thresholds 는 이제 **저장/표시를 넘어서 poller 가 실제로 평가하고 자동 재배포까지 수행한다.**
* 다만 이 실행기는 프로젝트 정책, 앱 rollback policy, 이전 deployment history, metrics reader 가 모두 준비된 경우에만 동작한다.
* stale secret cleanup 과 orphan Flux child cleanup 은 이제 worker 로 처리된다.
* orphan cleanup 은 여전히 worker 기준의 안전한 child manifest 정리 역할을 맡고, 사용자 lifecycle 은 별도 archive/delete API 로 처리된다.
* self-service bootstrap 은 이제 cluster catalog 생성 + Flux scaffold 생성 + project catalog 생성까지 PoC 기준으로 가능하다.

관련 근거:

* `autoRollbackEnabled` 는 catalog/UI/schema 에 존재: [platform/projects.yaml](/Users/ichanju/Desktop/aolda/AODS/platform/projects.yaml:27), [frontend/src/App.tsx](/Users/ichanju/Desktop/aolda/AODS/frontend/src/App.tsx:1100), [docs/internal-platform/openapi.yaml](/Users/ichanju/Desktop/aolda/AODS/docs/internal-platform/openapi.yaml:1070)
* auto rollback executor: [backend/internal/application/poller.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/poller.go:19)
* rollback evaluation 및 재배포 트리거: [backend/internal/application/poller.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/poller.go:197)
* executor 회귀 테스트: [backend/internal/application/poller_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/poller_test.go:234)
* stale secret cleanup 회귀 테스트: [backend/internal/vault/local_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/vault/local_test.go:1), [backend/internal/vault/real_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/vault/real_test.go:117)
* orphan cleanup 회귀 테스트: [backend/internal/application/orphan_cleanup_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/application/orphan_cleanup_test.go:1)
* cluster/project bootstrap 및 lifecycle cleanup 회귀 테스트: [backend/internal/server/server_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server_test.go:95), [backend/internal/server/server_git_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/server/server_git_test.go:1)

판정:

* **대부분 구현됨**
* 정책 저장/노출, 자동 집행, stale secret cleanup, orphan Flux child cleanup, cluster/project bootstrap, archive/delete lifecycle cleanup 까지 PoC 기준으로 구현됐다

## 문서 해석 가이드

현재 문서는 아래처럼 읽어야 한다.

* [docs/internal-platform/prd.md](/Users/ichanju/Desktop/aolda/AODS/docs/internal-platform/prd.md) 와 [docs/acceptance-criteria.md](/Users/ichanju/Desktop/aolda/AODS/docs/acceptance-criteria.md) 는 여전히 **Phase 1 최소 계약 문서**다. current baseline 설명 문서가 아니다.
* [docs/internal-platform/openapi.yaml](/Users/ichanju/Desktop/aolda/AODS/docs/internal-platform/openapi.yaml) 는 이미 **Phase 1~4 통합 스펙**
* [docs/future-phases-roadmap.md](/Users/ichanju/Desktop/aolda/AODS/docs/future-phases-roadmap.md) 는 **미래 계획 + 이미 일부 구현된 목표**가 섞여 있는 상태
* [docs/current-baseline-runbook.md](/Users/ichanju/Desktop/aolda/AODS/docs/current-baseline-runbook.md) 는 현재 **current implementation baseline handoff 문서**다

## git 히스토리 근거

아래 커밋들이 later-phase 확장이 실제로 진행됐음을 보여준다.

* `68f5220` `feat: phase 2-4 플랫폼 운영 기능을 확장한다`
* `9e028b9` `fix: canary flux 동기화 대기 문제를 해소한다`
* `9d7907e` `fix: 변경 상태 전이와 정책 회귀 테스트를 보강한다`
* `1dcdf2c` `feat: 실연동 경로와 저장소 기반 자동화를 확장한다`

즉, 현재 mismatch 는 `코드는 확장됐는데 일부 handoff 문서가 Phase 1-only 설명을 유지한 것`에 가깝다.

## 다음 정리 원칙

앞으로 문서를 정리할 때는 아래 기준을 권장한다.

1. Phase 1 문서는 `minimum contract` 로 남긴다.
2. 현재 구현과 QA baseline 은 이 문서를 기준으로 설명한다.
3. `Phase 3 PR mode` 는 실제 GitHub PR 생성 전까지 `review-gated change flow` 라고 명확히 적는다.
4. `Phase 4 auto rollback` 는 executor 포함 상태로 적고, 남은 미구현 항목은 cleanup/self-service/bootstrap 계열로 분리해서 적는다.
