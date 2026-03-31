# Domain Rules & Architecture Constraints

이 문서는 코덱스(Codex) 및 에이전트(Agent)가 애플리케이션 코드를 생성/수정할 때 지켜야 하는 **절대적인 도메인 제약 사항**입니다.

---

## 1. GitHub Source of Truth 규칙

Phase 1에서 프로젝트와 앱의 authoritative source는 **GitHub 기본 브랜치**다.

* 프로젝트 목록은 `platform/projects.yaml` 에서 읽는다.
* 앱 메타데이터는 `apps/{projectId}/{appName}/` 경로 구조에서 읽는다.
* Phase 1에서는 별도 DB를 source of truth로 두지 않는다.
* `applicationId` 는 GitHub 경로에서 결정 가능한 deterministic 값으로 유지한다. 권장 형식은 `{projectId}__{appName}` 이다.

---

## 2. GitOps Manifest 구조 규칙

플랫폼 백엔드는 사용자의 앱 생성 요청을 받으면 클러스터 API가 아닌 **GitHub에 Kustomize 폴더 구조를 커밋**해야 한다.

```yaml
platform:
  projects.yaml

apps:
  {projectId}:
    {appName}:
      base:
        kustomization.yaml
        deployment.yaml
        service.yaml
        virtualservice.yaml
        destinationrule.yaml
        externalsecret.yaml
      overlays:
        prod:
          kustomization.yaml
```

`platform/projects.yaml` 은 UI와 API가 참조하는 프로젝트 카탈로그다.

---

## 3. 쓰기/읽기/제어 흐름 규칙

GitOps 시스템이라고 해서 모든 흐름을 단방향 처리하지 마십시오.

* **Write 흐름 (Desired State)**: 매니페스트 렌더링 후 GitHub 기본 브랜치에 직접 커밋/푸시한다.
* **Metadata read 흐름**: 프로젝트 목록, 앱 목록, 배포 관련 메타데이터는 GitHub에서 읽는다.
* **Actual State read 흐름**: K8s `client-go` 로 API Server에 접근해 Flux `Kustomization` 상태를 읽는다.
* **제어 흐름 (Promote/Abort)**: Phase 2에서 `client-go` 기반으로 해당 앱의 `Rollout` CRD 를 패치한다.

Phase 1에서는 direct push가 기본이다. PR 기반 흐름은 Phase 2 이상에서 확장한다.

---

## 4. 보안 관련 제약 (Secret 금지, Vault 직접 통신)

* **금지 조항**: 백엔드는 Kubernetes 기본 `Secret` 리소스를 생성하면 안 된다.
* **Vault 통신 모델**: UI에서 입력한 Secret 값은 백엔드가 즉시 Vault KV v2에 저장한다.
* **Git 커밋 실패 허용 모델**: Secret 은 먼저 임시 경로에 저장하고, Git 커밋 성공 후 최종 경로로 확정한다.
* **Git 실패 후 정리**: Git 커밋이 실패하면 Secret 은 임시 경로에 남겨둘 수 있지만, TTL 또는 정리 잡으로 삭제 가능해야 한다.

권장 경로 규칙은 아래와 같다.

* 임시 경로: `secret/aods/staging/{requestId}`
* 최종 경로: `secret/aods/apps/{projectId}/{appName}/prod`

주의:

* Vault KV v2 는 API 경로에서 `/data/` 와 `/metadata/` 를 구분한다.
* 키 이름은 `/metadata/` 경로 listing 에 노출될 수 있으므로, 경로 세그먼트에는 민감한 정보를 넣지 않는다.
* Secret key/value 쌍은 개별 경로로 쪼개지 말고 앱 단위 문서 한 건으로 저장하는 것을 권장한다.

---

## 5. Flux 상태 매핑 규칙

플랫폼은 Flux `Kustomization.status.conditions` 를 아래 4개 UI 상태로 축약한다.

* `Unknown`: 리소스를 아직 찾지 못했거나 유효한 조건이 없다.
* `Syncing`: `Reconciling=True` 이거나 `Ready=Unknown` 이다.
* `Synced`: `Ready=True` 이고 reason 이 `ReconciliationSucceeded` 이다.
* `Degraded`: `Ready=False` 이거나 `Stalled=True` 이다.

실제 reason 문자열은 Flux가 제공하는 값을 그대로 로그와 message 필드에 보존하되, UI 표시는 위 4개만 사용한다.

---

## 6. 인증 및 권한 최소 모델

인증은 Keycloak MVP를 사용한다.

권한은 최소 아래 3단계를 가져야 한다.

* `viewer`: 프로젝트/앱 목록 조회, sync 상태 조회, 메트릭 조회
* `deployer`: `viewer` 권한 + 앱 생성 + 재배포
* `admin`: Phase 2 이상의 관리 기능용 예약 역할, Phase 1에서는 최소 `deployer` 이상 권한

백엔드는 Keycloak 토큰의 group 또는 role claim 을 읽어 프로젝트별 역할로 해석해야 한다.

---

## 7. 프론트엔드/백엔드 기본 컨벤션

* 프론트엔드 (Vite + React + Zustand)
  - **절대적 금지**: Tailwind CSS 및 인라인 스타일 사용 금지
  - UI 렌더링은 **Mantine** 컴포넌트를 호출하여 조립
  - 커스텀 디자인은 오직 **CSS Modules** 사용
  - API 통신은 `openapi.yaml` 계약을 먼저 기준으로 구현

* 백엔드 (Go 1.22+ Standard Library)
  - **절대적 금지**: Fiber, Gin, Echo 와 같은 서드파티 웹 프레임워크 사용 금지
  - 라우팅은 오직 `net/http` 의 Path Variable Mapping 방식 사용
  - 너무 깊은 레이어드 구조를 지양하고 `internal/application/`, `internal/project/` 처럼 **도메인 단위 Flat Pattern** 으로 구성
