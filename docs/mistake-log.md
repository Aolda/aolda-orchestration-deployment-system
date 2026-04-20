# Mistake Log

이 문서는 AODS 저장소에서 작업하면서 발생한 잘못된 판단, 계약 위반 시도, 회귀, 문서 오해, 운영 실수를 기록하고 재발 방지 조치를 남기는 용도다.

목적은 두 가지다.

1. 같은 실수를 반복하지 않도록 맥락과 교정 사항을 남긴다.
2. 코드/문서/운영 판단에서 어디서 어긋났는지 빠르게 추적한다.

## 언제 기록하나

아래와 같은 경우를 이 문서에 남긴다.

* `openapi.yaml`, `domain-rules.md`, `phase1-decisions.md` 같은 계약 문서를 어긴 변경을 했거나 시도했을 때
* 테스트가 놓친 회귀나 배포 과정의 실수를 발견했을 때
* 문서가 실제 코드 상태와 달라 잘못된 구현 판단을 유도했을 때
* 동일한 유형의 실수가 반복되기 시작했을 때

## 기록 원칙

* 사람 비난보다 사실과 재발 방지에 집중한다.
* 가능하면 날짜, 영향 범위, 근본 원인, 후속 조치를 같이 남긴다.
* 관련 커밋, PR, 문서 경로가 있으면 함께 적는다.
* 사소한 오타보다 계약 위반, 회귀, 운영상 혼선을 우선 기록한다.

## 기록 템플릿

아래 템플릿을 복사해서 새로운 항목을 추가한다.

```md
### YYYY-MM-DD - 짧은 제목

* Area:
* Summary:
* Impact:
* Root cause:
* Fix:
* Prevention:
* References:
* Status:
```

## Entries

### 2026-04-18 - LoadBalancer 진행 상태를 sync-status로만 추정함

* Area: Frontend / Backend / Observability
* Summary: 외부 공개 처리 단계 UI가 실제 Kubernetes `Service` 상태를 읽지 않고 `sync-status`만으로 `LoadBalancer 준비` 단계를 추정하고 있었다.
* Impact: 실제로는 LoadBalancer IP가 이미 할당된 애플리케이션도 화면에서는 계속 준비 중처럼 보였고, 반대로 Warning 이벤트가 있어도 오류로 드러나지 않았다.
* Root cause: `LoadBalancer` 진행 상태를 별도 API 계약 없이 프론트에서 sync 상태 기반으로만 조합했다.
* Fix: `GET /api/v1/applications/{applicationId}/network-exposure` 응답을 추가하고, 백엔드가 `Internal / Pending / Provisioning / Ready / Error`를 Kubernetes `Service`와 `Event` 기준으로 계산하도록 수정했다.
* Prevention: 운영 단계 UI는 GitOps sync 여부와 실제 인프라 readiness를 분리해서 다룬다. 특히 네트워크, 로그, 메트릭처럼 런타임 의존성이 있는 값은 프론트 추정 로직만으로 상태를 만들지 않는다.
* References: `backend/internal/kubernetes/real.go`, `backend/internal/application/service.go`, `frontend/src/App.tsx`, `docs/internal-platform/openapi.yaml`
* Status: fixed

### 2026-04-12 - 실수 기록 위치 부재

* Area: Documentation
* Summary: 저장소 안에 잘못한 사항과 재발 방지 조치를 남기는 전용 문서가 없었다.
* Impact: 같은 종류의 문서 오해나 계약 위반 시도를 장기적으로 축적하고 되짚기 어려웠다.
* Root cause: 구현 계약 문서와 현재 상태 문서는 있었지만, 회고성 로그를 위한 명시적 위치가 정의되지 않았다.
* Fix: `docs/mistake-log.md`를 추가하고 핸드오프 문서에서 위치를 안내한다.
* Prevention: 이후 계약 위반, 회귀, 운영 혼선은 이 문서에 날짜 기준으로 누적 기록한다.
* References: `docs/mistake-log.md`, `CLAUDE.md`
* Status: Open for ongoing use

### 2026-04-16 - Keycloak 권한 모델을 새 role 체계로 먼저 가정함

* Area: Auth / Documentation
* Summary: 실제 운영 환경에는 이미 `Ajou_Univ/Aolda_Admin`, `Ajou_Univ/Aolda_Member/.../ops` 형태의 group hierarchy 가 있었는데, 초기에 client role 중심 모델을 먼저 가정해 설명과 예시를 만들었다.
* Impact: 사용자가 이미 가진 Keycloak 구조와 다른 설정을 다시 만들게 유도할 수 있었고, 프로젝트별 권한이 실제 어디서 관리되는지 혼선을 만들었다.
* Root cause: 토큰 claim 을 읽는 기술 구현과 실제 조직의 Keycloak 운영 모델을 분리해서 확인하지 않았다.
* Fix: platform admin 을 환경변수 기반 global authority 로 분리하고, `platform/projects.yaml` exact-match 규칙과 group full path 우선 전략을 별도 문서로 정리했다.
* Prevention: Keycloak/OIDC 작업을 시작할 때는 먼저 `기존 realm/group 구조가 이미 있는지`, `AODS 가 맞춰야 할 claim 문자열이 무엇인지`를 확인하고 `docs/keycloak-group-auth-model.md` 에 기준을 누적한다.
* References: `docs/keycloak-group-auth-model.md`, `docs/user-feedback-log.md`, `backend/internal/project/service.go`
* Status: fixed

### 2026-04-17 - dev CORS allowlist 를 5173 고정으로 가정함

* Area: Backend / Frontend QA
* Summary: 로컬 QA 중 frontend dev server 가 `5173` 대신 `5174` 로 뜨자, emergency login 이후 `/api/v1/me`, `/api/v1/projects`, `/api/v1/clusters` bootstrap 호출이 전부 CORS 로 막혔다.
* Impact: 로그인 자체는 성공했지만 실제 화면은 계속 로그인 페이지에 머물러 보였고, 로컬 dev QA 에서 서비스가 죽은 것처럼 오판할 수 있었다.
* Root cause: backend CORS middleware 가 요청 origin 을 평가하지 않고 `AODS_ALLOWED_ORIGIN=http://localhost:5173` 문자열만 그대로 응답했다. `AODS_ALLOW_DEV_FALLBACK=true` 가 auth fallback 에만 적용되고 dev-origin fallback 의미로는 연결되지 않았다.
* Fix: `WithCORS` 가 comma-separated origin 목록을 처리하고, `AODS_ALLOW_DEV_FALLBACK=true` 일 때는 `localhost` 와 `127.0.0.1` loopback origin 을 요청 origin 기준으로 허용하도록 수정했다. 동시에 회귀 테스트를 추가하고, patched backend + frontend 로 실제 브라우저 로그인 검증까지 수행했다.
* Prevention: 로컬 QA 에서는 `5173` 을 전제로 두지 말고 실제 브라우저 origin 을 확인한다. CORS 수정은 단위 테스트와 실제 브라우저 bootstrap 검증을 같이 남긴다. dev server 는 가능하면 `--strictPort` 로 띄우고, 실패 시 포트 fallback 이 곧바로 CORS 회귀로 이어지지 않게 backend 를 유지한다.
* References: `backend/internal/core/http.go`, `backend/internal/core/http_test.go`, `backend/internal/server/server.go`, `docs/current-baseline-runbook.md`
* Status: fixed

### 2026-04-18 - 로컬 backend-run 에서 orphan cleanup 워커가 로그인 부트스트랩을 막음

* Area: Backend / Local Runtime
* Summary: `scripts/backend-run.sh`로 띄운 로컬 백엔드에서 orphan Flux cleanup 워커가 managed Git repo 락을 점유한 상태가 되면 `/api/v1/projects`, `/api/v1/clusters`가 응답하지 않아 로그인 직후 화면이 무한 로딩에 머물렀다.
* Impact: `/api/v1/me`는 200이지만 실제 포털은 스피너만 보였고, 사용자는 로그인 자체가 망가진 것으로 보게 되었다.
* Root cause: 로컬 dogfooding 경로에서도 orphan cleanup 워커가 기본 활성화되어 있었다. 이 워커는 사용자 bootstrap 요청과 같은 단일 managed Git repo 락을 공유하므로, cleanup이 걸리면 로그인에 필요한 catalog read 전체가 같이 막힌다.
* Fix: `scripts/backend-run.sh`에서 `AODS_ORPHAN_FLUX_CLEANUP_INTERVAL`이 명시되지 않은 로컬 실행은 기본 `0`으로 내려 opt-in으로 전환했다. 현재 로컬 백엔드도 이 설정으로 재기동해 `/api/v1/projects`, `/api/v1/clusters` 응답과 로그인 경로를 복구했다.
* Prevention: 로컬 실행용 background maintenance worker는 source-of-truth read path와 같은 락을 잡을 때 기본 비활성화하거나 명시 opt-in으로 둔다. 이후 cleanup worker를 다시 켤 때는 lock contention 관측 로그 또는 timeout/skip 전략을 같이 넣고 QA한다.
* References: `scripts/backend-run.sh`, `backend/internal/application/orphan_cleanup.go`, `backend/internal/gitops/repository.go`
* Status: fixed
