# AODS Repository Instructions

이 저장소는 문서 계약이 강한 내부 배포 플랫폼이다. 작업 전에는 아래 순서를 우선 확인한다.

1. `docs/internal-platform/openapi.yaml`
2. `docs/acceptance-criteria.md`
3. `docs/phase1-decisions.md`
4. `docs/domain-rules.md`
5. `docs/current-implementation-status.md`
6. `docs/agent-code-index.md`
7. 루트 `AGENTS.md`

기본 규칙:

* 기본 구현 baseline 은 **Phase 1 MVP** 다.
* 사용자가 명시적으로 요청하지 않으면 Phase 2, 3, 4 동작을 임의로 확장하지 않는다.
* GitHub 기본 브랜치가 desired state 의 source of truth 다.
* 프로젝트 목록은 `platform/projects.yaml` 에서 읽는다.
* 앱 ID 는 `{projectId}__{appName}` 규칙을 유지한다.
* Secret 평문을 Git에 저장하지 않는다.
* Kubernetes 기본 `Secret` 리소스로 우회하지 않는다.
* Flux 상태는 `Unknown`, `Syncing`, `Synced`, `Degraded` 네 개만 UI에 노출한다.
* 백엔드는 Go `net/http` 표준 라이브러리만 사용한다.
* 프론트엔드는 Mantine + CSS Modules 기준이다.
* `openapi.yaml` 과 다른 API response shape 를 임의로 만들지 않는다.

작업 원칙:

* API 변경 시 `openapi.yaml`, 백엔드 핸들러/서비스, 프론트 타입/클라이언트, 테스트를 함께 갱신한다.
* 실제 수정 위치와 활성 코드 경로는 `docs/agent-code-index.md` 를 먼저 본다.
* 문서와 코드가 충돌하면 코드를 먼저 합리화하지 말고 문서 계약부터 확인한다.
* 계약 위반 시도, 회귀, 문서 오해는 `docs/mistake-log.md` 에 기록한다.
