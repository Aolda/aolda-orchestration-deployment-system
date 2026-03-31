# 내부 앱 배포 플랫폼 PRD

## 1. 문서 정보

* 문서명: 내부 앱 배포 플랫폼 PRD
* 버전: v0.3 (Architecture Refinement: Istio & Vault 반영)
* 작성일: 2026-03-31
* 상태: Draft
* 작성 목적: 컨테이너 이미지 기반으로 내부 서비스를 쉽게 배포하고, GitOps 방식으로 상태를 관리하며, 점진 배포와 기본 메트릭 관제를 제공하는 내부 플랫폼 정의

---

## 2. 배경 및 문제 정의

현재 쿠버네티스 기반 서비스 배포에는 다음과 같은 문제가 있다.

* 개발자가 배포를 위해 Kubernetes, Istio, Secret, Registry 인증, Rollout 전략까지 직접 알아야 한다.
* 팀마다 배포 방식이 달라 standard rollout, canary rollout, rollback 방식이 일관되지 않다.
* 배포 후 상태 확인이 여러 도구에 흩어져 있어 빠른 판단이 어렵다.
* 복구를 위해 현재 desired state를 명확히 파악하기 어렵거나, 변경 이력이 산발적으로 남는다.
* 운영팀이 반복적으로 매니페스트 생성, Secret 연결, 배포 지원을 수동 처리하게 된다.

이 문제를 해결하기 위해, 사용자는 포털에서 최소 입력만 제공하고 플랫폼은 이를 Git 기반 desired state로 기록한 뒤 클러스터에 자동 반영하는 내부 배포 플랫폼이 필요하다.

---

## 3. 제품 비전

개발자가 컨테이너 이미지와 배포에 필요한 최소 정보만 입력하면, 플랫폼이 이를 GitHub 기반 GitOps 방식으로 관리하고, 안전한 standard/canary 배포와 기본 메트릭 관측까지 한 번에 제공한다.

핵심 가치는 아래와 같다.

* **쉬운 배포**: YAML 없이 앱을 배포할 수 있다.
* **명확한 상태 관리**: GitHub를 desired state의 단일 기준으로 삼는다.
* **안전한 배포**: Argo Rollouts 기반 점진 배포와 rollback을 제공한다.
* **기본 관측 가능성**: Prometheus 기반 핵심 메트릭을 플랫폼에서 바로 확인한다.
* **복구 용이성**: Git에 정의된 상태를 기준으로 재현 및 복구가 가능하다.
* **보안 내재화**: 모든 시스템간 통신은 Istio Mesh로 보호되고, 모든 Secret은 플랫폼이 HashiCorp Vault에 등재시켜 Git에 평문이 노출되지 않는다.

---

## 4. 제품 목표

### 4.1 핵심 목표

1. 컨테이너 이미지 기반의 쉬운 앱 배포 제공
2. GitOps 기반 desired state 관리 제공
3. Standard / Canary 배포 전략 표준화
4. Prometheus 기반 메트릭 조회 제공
5. Vault 연동 기반 보안 시크릿 환경 변수 처리
6. 수동 운영 지원 비용 절감

### 4.2 비목표

다음 항목은 본 제품의 초기 범위에 포함하지 않는다.

* 범용 CI 빌드 시스템
* 복잡한 승인 워크플로우
* 소스코드 저장소 구조 자동 해석

---

## 5. 대상 사용자

### 5.1 1차 사용자

* 사내 백엔드 개발자
* 애플리케이션 운영 담당자

### 5.2 2차 사용자

* 플랫폼 엔지니어
* DevOps / SRE 담당자

---

## 6. 핵심 제품 원칙

1. **포털 중심 UX**

   * 사용자는 GitOps 세부 구현보다 앱 배포 경험에 집중한다.

2. **GitHub를 최종 desired state 원본으로 사용 (Read는 Kubernetes API로 보완)**

   * 플랫폼이 직접 클러스터에 배포 상태를 강제(Apply)하는 대신, Git에 desired state를 기록하고 Flux가 이를 동기화한다. (예외: 조회 및 긴급 제어 통신은 K8s API 사용)

3. **점진 배포는 전용 컨트롤러로 처리**

   * 트래픽 분할 및 Service Mesh 보안은 Istio가 담당하고, 배포 수명주기는 Argo Rollouts가 담당한다.

4. **플랫폼은 핵심 메트릭만 우선 제공**

5. **안전한 기본값을 강제**

   * readiness, 최소 replica, Vault 시크릿 강제 저장 시스템 등을 플랫폼이 제공한다.

---

## 7. 사용자 시나리오

### 7.1 신규 앱 배포

1. 사용자가 단순 인증으로 로그인한다.
2. 프로젝트를 선택하고 앱을 생성한다.
3. 이미지 명, 서비스 포트, 배포 전략을 선택한다.
4. 민감한 환경변수 값을 폼에 작성하면, 플랫폼이 이를 즉시 Vault API 호출을 통해 안전하게 저장한다.
5. 플랫폼은 GitHub 저장소에 앱 리소스 매니페스트(`Deployment`, `ExternalSecret`, `VirtualService` 등)를 생성/갱신한다.
6. Flux가 Git 변경사항을 클러스터에 반영하며, ExternalSecret 오퍼레이터가 Vault에서 값을 당겨온다.
7. 사용자는 플랫폼에서 Flux 동기화 상태 배지를 실시간 확인한다.

### 7.2 Canary 배포 중 장애 대응

1. 전략을 Canary로 배포 중이다.
2. Prometheus 메트릭 기준치를 이탈하자, 사용자가 수동 Abort를 누르거나 백엔드 헬스체크 정책에 의해 롤아웃이 멈춘다.
3. 플랫폼 백엔드가 Argo Rollout API(또는 K8s Client)에 Patch 명령을 날려 롤백(중단) 처리한다.
4. 이전 버전(Stable)로 트래픽이 완전히 전환된다.

---

## 8. 범위 정의

### 8.1 포함 범위

* 인증 관리, 앱/프로젝트 관리, 이미지 배포
* Vault를 통한 Secret 직접 삽입 폼 처리 및 ExternalSecret 연동 생성 기능
* Standard/Canary 배포 지원 (Argo Rollouts)
* Istio VirtualService 및 DestinationRule 기반 North-South 노출, 트래픽 분할 기능
* Flux 동기화 및 K8s API 기반 상태 모니터링
* Prometheus 메트릭 조회

### 8.2 제외 범위

* 빌드 파이프라인 (CI)
* 멀티클러스터 구조

---

## 9. 기능 요구사항

### 9.1 인증 및 권한 (유지)

### 9.2 앱 및 프로젝트 관리 (유지)

### 9.3 시크릿 보안 (Vault 강제 연동)
* 사용자는 Vault에 대해 알 필요 없이 플랫폼 UI 창에 민감한 KEY/VALUE를 넣는다.
* 플랫폼 백엔드가 Vault Token을 지니고 Vault Server에 해당 값을 저장한다.
* 백엔드는 GitOps 커밋 시값을 평문 저장하는 대신 `ExternalSecret` 리소스만 생성한다.

### 9.4 배포 전략 및 트래픽 라우팅
* 플랫폼은 Standard 및 Canary 배포를 제공하되, 뒤에서는 Istio `VirtualService` 트래픽 웨이팅 방식을 사용한다.
* 모든 앱 배포는 클러스터 내 mTLS 보안 구간을 보장받는다.

---

## 10. 비기능 요구사항

### 10.1 보안
* Secret 값 평문 저장은 철저히 배제(Vault 1차 필터링).
* Istio 기반 내부망 서비스 메시 통신.

---

## 11. 제안 아키텍처

### 11.1 아키텍처 스택 (확정판)
* Frontend: React / Vite
* Backend: Go (Fiber / Stdlib)
* GitOps: Flux
* Progressive Delivery: Argo Rollouts
* Traffic Management & Security: Istio Service Mesh
* Observability: Envoy Proxy, Prometheus
* Secrets: HashiCorp Vault & ExternalSecrets 연동

### 11.2 플랫폼 백엔드 연계 구조
* **GitOps 쓰기 통신**: 플랫폼 백엔드 -> GitHub (Kustomize 폴더 커밋) 
* **상태 읽기 / 제어 통신**: 플랫폼 백엔드 -> Kubernetes API Server (K8s Client-go 로 상태 GET/WATCH/PATCH 수행)
* **비밀값 관리 통신**: 플랫폼 백엔드 -> Vault API (Secret 값 즉시 삽입)
* **메트릭 통신**: 플랫폼 백엔드 -> Prometheus HTTP API

---

## 13. 리포지토리 구조 지침

앱별 GitOps 저장소 구조는 단순성과 diff 분석을 위해 Kustomize 기반으로 확립한다.

### 13.1 Kustomize 매니페스트 예시

```text
apps/
  project-a/
    my-app/
      base/
        kustomization.yaml
        deployment.yaml       # 혹은 rollout.yaml (Canary 시)
        service.yaml
        virtualservice.yaml   # Istio 라우팅
        destinationrule.yaml  # Istio 트래픽 제어 부분 규격
        externalsecret.yaml   # Vault 경로 참조 설정 리소스
      overlays/
        prod/
          kustomization.yaml
```

---

## 15. API 구조

### 앱 배포 및 시크릿

* POST `/api/v1/projects/{projectId}/applications` : 앱 생성 시 Env Variables 중 보안 Secret Array 폼데이터 추가 수용.
* PATCH `/api/v1/applications/{applicationId}`
* POST `/api/v1/applications/{applicationId}/deployments`

---

## 21. 최종 결론

본 제품은 **내부 배포 포털 + GitOps (Flux) 기반 desired state 관리 + Progressive Delivery (Argo Rollouts) + Service Mesh (Istio) + Secret Security (Vault)** 결합체로 정의한다.

개발자가 플랫폼 포털에서 버튼과 폼(form)입력만 하면, 플랫폼 백엔드는 GitHub, K8s API, Vault API의 중간 코디네이터 역할을 수행해 운영 오버헤드를 제로에 가깝게 줄여준다.
