# 내부 앱 배포 플랫폼 PRD

## 1. 문서 정보

* 문서명: 내부 앱 배포 플랫폼 PRD
* 버전: v0.5
* 작성일: 2026-03-31
* 상태: Draft
* 작성 목적: 내부 앱 배포 플랫폼의 **Phase 1 MVP 범위와 구현 계약**을 명확히 정의한다.

---

## 2. 문제 정의

현재 내부 서비스 배포에는 아래 문제가 있다.

* 개발자가 Kubernetes, Istio, Secret 처리, Registry, GitOps 구조를 모두 알아야 한다.
* 팀마다 배포 방식이 달라 standard rollout, rollback, 상태 확인 방식이 일관되지 않다.
* 운영팀이 매니페스트 생성, Secret 연결, 배포 지원을 반복적으로 수동 처리한다.
* 배포 후 현재 desired state와 actual state를 빠르게 대조하기 어렵다.

이 플랫폼의 첫 목표는 "개발자가 이미지와 최소 설정만 넣으면, 앱 하나를 안전하게 올리고 상태를 확인할 수 있게 만드는 것"이다.

---

## 3. 제품 비전

장기적으로는 내부 배포 포털이 GitOps, Secret 보안, 점진 배포, 관측을 하나로 묶는 진입점이 된다.

하지만 **Phase 1 MVP**는 더 좁다.

> 이미 빌드된 컨테이너 이미지 하나를 포털에서 등록하고, GitOps로 반영하고, 동기화 상태와 기본 메트릭을 확인할 수 있어야 한다.

이게 되면 첫 내부 팀이 더 이상 YAML, Vault UI, `kubectl`을 직접 만지지 않고도 앱을 한 번 배포해볼 수 있다.

---

## 4. 대상 사용자

### 4.1 1차 사용자

* 사내 백엔드 개발자
* 애플리케이션 운영 담당자

### 4.2 2차 사용자

* 플랫폼 엔지니어
* DevOps / SRE 담당자

---

## 5. Phase 1 MVP 목표

1. 사용자가 권한 내 프로젝트 목록을 조회할 수 있다.
2. 프로젝트 단위로 애플리케이션 목록을 조회할 수 있다.
3. 사용자가 앱 이름, 이미지, 서비스 포트, Secret 값을 입력해 **Standard 배포 앱**을 생성할 수 있다.
4. 플랫폼이 GitOps 저장소에 Kustomize 구조를 생성하고 커밋한다.
5. Secret 평문은 Git에 남기지 않고 Vault에 저장하며, Git에는 `ExternalSecret`만 남긴다.
6. 사용자가 앱 상세 화면에서 Flux sync 상태를 확인할 수 있다.
7. 사용자가 앱 상세 화면에서 핵심 메트릭을 확인할 수 있다.
8. 사용자가 새 이미지 태그로 Standard 재배포를 실행할 수 있다.

---

## 6. Phase 1 비목표

다음은 이번 MVP에 포함하지 않는다.

* 실제 Canary rollout 실행
* Promote / Abort 버튼의 동작 구현
* 메트릭 기반 자동 rollback
* 복잡한 승인 워크플로우
* 범용 CI 빌드 시스템
* 멀티클러스터 지원
* 저장소 구조 자동 해석

이 항목들은 **Phase 2 이후**로 미룬다.

---

## 7. 사용자 시나리오

### 7.1 프로젝트 진입

1. 사용자가 로그인한다.
2. 플랫폼은 현재 사용자 권한에 맞는 프로젝트 목록을 보여준다.
3. 사용자는 접근 가능한 프로젝트 하나를 선택한다.

### 7.2 신규 앱 배포

1. 사용자가 프로젝트를 선택한다.
2. 앱 이름, 이미지 주소, 서비스 포트, Secret 값을 입력한다.
3. 플랫폼 백엔드는 Secret 값을 먼저 Vault에 임시 저장한다.
4. 플랫폼 백엔드는 GitOps 저장소의 `apps/{projectId}/{appName}/` 아래에 Kustomize 리소스를 생성하고 기본 브랜치에 직접 커밋한다.
5. Git 커밋이 성공하면 Secret을 최종 경로로 확정하고 임시 경로를 정리한다.
6. Flux가 변경사항을 클러스터에 동기화한다.
7. 사용자는 플랫폼에서 sync 상태와 기본 메트릭을 확인한다.

### 7.3 Standard 재배포

1. 이미 생성된 앱이 있다.
2. 사용자가 새 이미지 태그로 재배포를 요청한다.
3. 플랫폼 백엔드는 GitOps overlay 또는 매니페스트를 갱신하는 커밋을 생성한다.
4. 사용자는 sync 상태가 다시 `Syncing`에서 `Synced`로 바뀌는 것을 확인한다.

---

## 8. Phase 1 포함 범위

### 8.1 프론트엔드

* 프로젝트 목록 화면
* 프로젝트별 앱 목록 화면
* 앱 생성 폼
* 앱 상세 화면
* sync 상태 배지
* 기본 메트릭 패널

### 8.2 백엔드 API

* 현재 사용자 조회
* 프로젝트 목록 조회
* 프로젝트별 앱 목록 조회
* 앱 생성
* 앱 재배포
* sync 상태 조회
* 메트릭 조회

### 8.3 인프라 연동

* GitOps 저장소 직접 커밋
* Vault Secret 저장
* Kubernetes API를 통한 Flux sync 상태 조회
* Prometheus API를 통한 메트릭 조회

---

## 9. Phase 2 예정 범위

Phase 1이 파일럿 팀에서 실제로 쓰이기 시작하면 아래를 순서대로 연다.

* Canary 전략 노출
* Argo Rollouts 기반 Promote / Abort
* 트래픽 비중 시각화
* 메트릭 기반 수동/자동 rollback 보조
* direct push에서 PR 기반 변경 승인 흐름으로 확장

Phase 2의 핵심은 "배포를 되게 만드는 것"이 아니라 "배포를 더 안전하게 만드는 것"이다. 순서를 뒤집지 않는다.

Phase 2, 3, 4의 상세 로드맵과 handoff guardrail 은 `docs/future-phases-roadmap.md` 를 기준으로 한다.

---

## 10. 기능 요구사항

### 10.1 프로젝트 목록 조회

* 사용자는 권한 내 프로젝트 목록을 조회할 수 있어야 한다.
* 프로젝트 목록의 source of truth는 GitHub 저장소 내 프로젝트 카탈로그 파일이다.
* 각 프로젝트 항목에는 최소한 프로젝트 이름, 설명, 네임스페이스, 사용자 역할이 포함되어야 한다.

### 10.2 앱 목록 조회

* 사용자는 프로젝트별 앱 목록을 조회할 수 있어야 한다.
* 목록에는 최소한 앱 이름, 현재 이미지, 배포 전략, 최근 sync 상태가 표시되어야 한다.
* 앱/배포 메타데이터의 authoritative source는 GitHub 기본 브랜치다.

### 10.3 앱 생성

* 사용자는 앱 이름, 설명, 이미지, 서비스 포트, Secret 배열을 입력할 수 있어야 한다.
* Phase 1의 배포 전략은 `Standard`만 허용한다.
* 백엔드는 앱 생성 시 GitOps 저장소에 표준 템플릿 리소스를 생성해야 한다.
* Git 커밋 방식은 Phase 1에서 기본 브랜치 직접 푸시를 사용한다.

### 10.4 Secret 처리

* Secret 값은 백엔드가 Vault KV v2에 먼저 임시 저장해야 한다.
* Git에는 Secret 평문이 절대 기록되면 안 된다.
* Git 커밋 성공 후 Secret은 최종 경로로 확정된다.
* Git 커밋 실패 시 Secret은 임시 경로에 남겨두고 TTL 또는 정리 작업으로 제거해야 한다.
* GitOps 저장소에는 `ExternalSecret` 리소스만 생성된다.

### 10.5 앱 재배포

* 사용자는 새 이미지 태그를 입력해 Standard 재배포를 요청할 수 있어야 한다.
* 백엔드는 GitOps 저장소를 갱신하는 커밋을 생성해야 한다.

### 10.6 상태 조회

* 백엔드는 Kubernetes API에서 Flux `Kustomization` 상태를 읽어 sync 상태를 제공해야 한다.
* UI는 `Unknown`, `Syncing`, `Synced`, `Degraded` 상태를 명확히 보여줘야 한다.
* 상태 매핑 규칙은 `docs/phase1-decisions.md` 의 Flux 섹션을 따른다.

### 10.7 메트릭 조회

* 백엔드는 Prometheus에서 요청 수, 에러율, 응답 지연, CPU/메모리 사용량을 조회해야 한다.
* 일부 메트릭이 비어 있어도 UI가 깨지면 안 된다.

### 10.8 인증 및 권한

* 인증은 Keycloak MVP를 사용한다.
* 권한은 최소 `viewer`, `deployer`, `admin` 개념을 가져야 한다.
* `viewer` 는 조회만 가능하고, `deployer` 는 앱 생성/재배포까지 가능해야 한다.
* `admin` 은 Phase 2 이상의 프로젝트 관리 권한 확장을 위해 예약하되, Phase 1에서는 최소한 `deployer` 이상 권한으로 취급한다.

---

## 11. 비기능 요구사항

### 11.1 보안

* Secret 평문 저장 금지
* Vault를 통한 Secret 직접 저장 강제
* Vault 경로에는 민감한 값이 드러나지 않도록 비민감 식별자만 사용한다.

### 11.2 운영성

* GitHub 기본 브랜치를 desired state의 단일 기준으로 사용한다.
* 프로젝트 목록과 앱 메타데이터 모두 GitHub에서 읽는다.
* 읽기와 제어는 Kubernetes API를 보조 채널로 사용한다.

### 11.3 구현 제약

* Frontend: React + Vite + Mantine + CSS Modules
* Backend: Go 1.22+ `net/http` 표준 라이브러리
* 백엔드는 도메인 단위 Flat 구조를 사용한다.

---

## 12. 제안 아키텍처

### 12.1 아키텍처 스택

* Frontend: React / Vite
* Backend: Go `net/http`
* GitOps: Flux
* Traffic / Security: Istio Service Mesh
* Observability: Prometheus
* Secrets: HashiCorp Vault + ExternalSecrets

### 12.2 플랫폼 백엔드 연계 구조

* **Metadata read 흐름**: 플랫폼 백엔드 -> GitHub 기본 브랜치
* **Manifest write 흐름**: 플랫폼 백엔드 -> GitHub 기본 브랜치 직접 커밋
* **Actual state read 흐름**: 플랫폼 백엔드 -> Kubernetes API Server
* **Secret 흐름**: 플랫폼 백엔드 -> Vault API
* **Metric 흐름**: 플랫폼 백엔드 -> Prometheus HTTP API

---

## 13. GitOps 저장소 구조

Phase 1에서는 GitHub 저장소 안에 아래 두 영역을 사용한다.

```text
platform/
  projects.yaml
  flux/
    bootstrap/
      {clusterId}/
        kustomization.yaml
        root-kustomization.yaml
    clusters/
      {clusterId}/
        kustomization.yaml
        applications/
          {projectId}-{appName}.yaml

apps/
  {projectId}/
    {appName}/
      base/
        kustomization.yaml
        deployment.yaml
        service.yaml
        virtualservice.yaml
        destinationrule.yaml
        externalsecret.yaml
      overlays/
        prod/
          kustomization.yaml
```

`platform/projects.yaml` 은 프로젝트 목록의 source of truth다.
`platform/flux/bootstrap/{clusterId}` 는 cluster bootstrap 용 Flux root manifest 위치다.
`platform/flux/clusters/{clusterId}` 는 Flux root 가 읽는 child `Kustomization` 집합이다.

`Canary` 동작은 Phase 2로 미루더라도, 구조는 이후 확장을 방해하지 않는 형태로 시작한다.

---

## 14. Phase 1 API 범위

* `GET /api/v1/me`
* `GET /api/v1/projects`
* `GET /api/v1/projects/{projectId}/applications`
* `POST /api/v1/projects/{projectId}/applications`
* `POST /api/v1/applications/{applicationId}/deployments`
* `GET /api/v1/applications/{applicationId}/sync-status`
* `GET /api/v1/applications/{applicationId}/metrics`

`PATCH /api/v1/applications/{applicationId}` 와 Canary 관련 제어 API는 Phase 2에서 다시 연다.

---

## 15. 성공 기준

Phase 1 MVP 완료 기준은 아래와 같다.

1. 사용자가 권한 내 프로젝트 목록을 볼 수 있다.
2. 내부 파일럿 팀이 포털에서 앱 하나를 생성할 수 있다.
3. 생성 과정에서 Secret 평문이 Git에 남지 않는다.
4. 플랫폼이 GitOps 커밋을 만들고 Flux sync 상태를 표시한다.
5. 사용자가 새 이미지 태그로 재배포를 다시 실행할 수 있다.
6. 사용자가 기본 메트릭을 플랫폼 안에서 확인할 수 있다.

이 여섯 가지가 되면, 그 다음부터 Canary를 붙여도 늦지 않다.
