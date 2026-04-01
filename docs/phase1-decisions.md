# Phase 1 Decisions

이 문서는 PRD보다 한 단계 더 구현 가까운 계약을 적는다.
지금 당장 개발을 시작할 때 흔들리면 안 되는 결정을 여기서 고정한다.

## 1. Source of Truth

Phase 1의 authoritative source는 GitHub 기본 브랜치다.

* 프로젝트 목록: `platform/projects.yaml`
* 앱 메타데이터: `apps/{projectId}/{appName}/`
* sync 상태: Kubernetes API
* 메트릭: Prometheus API

별도 DB는 Phase 1에서 필수 아님이다. 필요해도 캐시나 작업 로그 용도로만 쓰고, 정답은 GitHub에서 읽는다.

## 2. 프로젝트 목록 API

`GET /api/v1/projects`

권장 저장 형식:

```yaml
projects:
  - id: project-a
    name: Project A
    description: Internal APIs
    namespace: project-a
    access:
      viewerGroups:
        - aods:project-a:view
      deployerGroups:
        - aods:project-a:deploy
      adminGroups:
        - aods:platform:admin
```

백엔드는 이 파일을 GitHub에서 읽고, 현재 사용자의 Keycloak claim 과 대조해 접근 가능한 프로젝트만 반환한다.

## 3. 앱 ID 규칙

DB를 쓰지 않으므로 `applicationId` 는 GitHub 경로에서 결정 가능해야 한다.

권장 규칙:

* GitHub 경로: `apps/{projectId}/{appName}`
* API ID: `{projectId}__{appName}`

전제:

* `projectId`, `appName` 은 DNS-1123 스타일 slug 를 사용한다.
* 구분자는 `__` 로 고정한다.

## 4. Git 쓰기 모델

Phase 1은 바로 쓰기로 간다.

* 대상: GitHub 기본 브랜치
* 방식: backend direct push
* 이유: 구현 단순성, 첫 파일럿 팀 속도

Phase 2 이후:

* PR 기반 변경 승인 흐름
* diff 검토 UI
* rollback 히스토리 고도화

### 4.1 개발 환경 실연동 순서

Phase 1 제품 계약은 GitHub + Vault + Kubernetes + Prometheus 연동을 포함한다.
하지만 **개발 환경에서의 실연동 순서**는 아래처럼 가져간다.

1. GitHub 기본 브랜치 direct push를 먼저 실연동한다.
2. Vault는 인터페이스와 경로 계약을 유지한 채 local adapter를 당분간 사용한다.
3. GitHub write/read 경로가 안정화된 뒤 Vault KV v2 adapter를 붙인다.

이 결정은 **제품 범위 축소가 아니다.**
개발 단계에서 실패 원인을 분리하고 vertical slice 를 빠르게 검증하기 위한 순서 조정이다.

주의:

* Secret 평문이 Git에 기록되면 안 된다는 계약은 그대로 유지한다.
* Vault 최종 경로 규칙 `secret/aods/apps/{projectId}/{appName}/prod` 는 지금부터 고정한다.
* local secret adapter는 dev fallback 일 뿐이며, 운영 연결 전에는 Vault real adapter로 교체해야 한다.

## 5. Vault 경로 규칙

Vault는 KV v2 기준으로 잡는다.

권장 최종 경로:

* 논리 경로: `secret/aods/apps/{projectId}/{appName}/prod`
* 실제 write API: `/v1/secret/data/aods/apps/{projectId}/{appName}/prod`
* metadata API: `/v1/secret/metadata/aods/apps/{projectId}/{appName}/prod`

왜 이렇게 자르나:

* `secret` 는 mount path
* `aods/apps/...` 는 논리 경로
* 프로젝트, 앱, 환경까지만 경로에 넣고 실제 secret key 들은 문서 body 안에 넣는다
* 경로 이름은 listing 에 보일 수 있으니 민감한 값은 넣지 않는다

권장 데이터 형식:

```json
{
  "DATABASE_URL": "...",
  "REDIS_URL": "...",
  "JWT_SECRET": "..."
}
```

개별 key 별로 경로를 쪼개지 않는 이유는:

* ExternalSecret 매핑이 단순해진다
* 경로에서 비밀 구조가 덜 드러난다
* 앱 단위 회수와 정리가 쉽다

## 6. Vault 임시 저장 모델

가능하다. 오히려 지금 상황에는 이쪽이 맞다.

권장 흐름:

1. 요청 수신
2. Vault 임시 경로에 저장
3. Git 커밋 시도
4. 성공 시 최종 경로에 재기록하고 임시 경로 삭제
5. 실패 시 임시 경로 유지, 정리 잡이 나중에 삭제

권장 임시 경로:

* `secret/aods/staging/{requestId}`

권장 메타데이터:

* `projectId`
* `appName`
* `status=pending_commit`
* `createdBy`
* `expiresAt`

권장 정리 방식:

* staging 경로에 `delete_version_after=24h` 같은 TTL 설정
* 성공 시 `kv metadata delete`
* 실패 건은 주기적 정리 잡으로 삭제

이렇게 하면 Git 실패 시에도 Secret 을 바로 잃지 않는다. 대신 staging 과 final 을 분리해야 ExternalSecret 이 잘못된 경로를 읽지 않는다.

## 7. Flux 상태 매핑

플랫폼 UI는 아래 4개 상태만 쓴다.

* `Unknown`
  - Kustomization 을 아직 못 찾음
  - 상태 조건이 아직 없음

* `Syncing`
  - `Reconciling=True`
  - 또는 `Ready=Unknown`

* `Synced`
  - `Ready=True`
  - reason = `ReconciliationSucceeded`

* `Degraded`
  - `Ready=False`
  - 또는 `Stalled=True`

Flux 내부 reason 은 더 많다. 예를 들면 build 실패, dependency 미준비, health check 실패가 있다. 이건 API `message` 에 그대로 담고, UI는 4개 상태만 보여주면 충분하다.

## 8. 공통 에러 스키마

권장 형식:

```json
{
  "error": {
    "code": "PROJECT_NOT_FOUND",
    "message": "Project project-a was not found.",
    "details": {
      "projectId": "project-a"
    },
    "requestId": "req_01H...",
    "retryable": false
  }
}
```

상태 코드 의미:

* `400`: 잘못된 입력, 지원하지 않는 전략, 필수값 누락
* `401`: 인증 실패 또는 토큰 없음
* `403`: 인증은 되었지만 프로젝트 권한 없음
* `404`: 프로젝트, 앱, deployment, Git 경로, K8s 리소스를 못 찾음
* `409`: 중복 앱, 이미 진행 중인 작업, 상태 충돌
* `500`: GitHub, Vault, Prometheus, Kubernetes 연동 실패 또는 서버 내부 오류

## 9. 권한 최소 모델

Phase 1은 이것만 있으면 된다.

* `viewer`
  - `GET /api/v1/projects`
  - `GET /api/v1/projects/{projectId}/applications`
  - `GET /api/v1/applications/{applicationId}/sync-status`
  - `GET /api/v1/applications/{applicationId}/metrics`

* `deployer`
  - `viewer` 전부
  - `POST /api/v1/projects/{projectId}/applications`
  - `POST /api/v1/applications/{applicationId}/deployments`

* `admin`
  - Phase 1에서는 최소 `deployer` 와 동일 취급
  - Phase 2부터 프로젝트 설정/권한 변경까지 확장 가능
