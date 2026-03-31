# Acceptance Criteria (인수 조건, BDD 형태)

이 문서는 Phase 1 MVP 구현 완료 여부를 검증하기 위한 공통 테스트 시나리오다.

핵심 질문은 하나다.

> 내부 사용자가 포털만으로 프로젝트를 고르고, 앱을 배포하고, 다시 배포하고, 상태를 확인할 수 있는가?

---

## Bounding Context: 인증 및 프로젝트 목록

### Scenario: 사용자가 권한 내 프로젝트 목록을 조회한다
- **Given** 사용자가 단순 인증(Keycloak MVP)을 마친 후 대시보드 화면에 접속해 있다.
- **When** 사용자가 `GET /api/v1/projects` 를 호출한다.
- **Then** 응답은 사용자가 접근 가능한 프로젝트만 포함해야 한다.
- **And** 각 항목에는 최소 프로젝트 이름, 설명, 네임스페이스, 역할이 포함되어야 한다.

---

## Bounding Context: 프로젝트 앱 목록

### Scenario: 사용자가 권한 내의 앱 목록을 조회한다
- **Given** 사용자가 권한이 있는 프로젝트 `project-a` 를 선택했다.
- **When** 사용자가 `GET /api/v1/projects/{projectId}/applications` 를 호출한다.
- **Then** `project-a` 에 소속된 어플리케이션 목록만 표시되어야 한다.
- **And** 각 항목에는 최소 앱 이름, 이미지, 배포 전략, 최근 sync 상태가 보여야 한다.

### Scenario: 권한 없는 프로젝트 앱 목록을 조회하려고 한다
- **Given** 사용자가 프로젝트 `project-b` 에 대한 권한이 없다.
- **When** 사용자가 `GET /api/v1/projects/project-b/applications` 를 호출한다.
- **Then** 백엔드는 상태 코드 `403` 을 반환해야 한다.
- **And** 공통 에러 스키마를 따라 응답해야 한다.

---

## Bounding Context: 앱 생성 (Standard 배포)

### Scenario: 새 어플리케이션(Standard 전략)을 생성한다
- **Given** 사용자가 `project-a` 화면에서 `앱 생성` 버튼을 눌렀다.
- **When** 앱 이름(`my-app`), 이미지 주소(`repo/my-app:v1`), 서비스 포트(`8080`), Secret 값 배열을 입력하고 생성 요청을 보낸다.
- **Then** 백엔드는 상태 코드 `201` 을 반환해야 한다.
- **And** 백엔드는 GitOps 저장소의 `apps/project-a/my-app/base/` 경로에 템플릿된 리소스 파일셋을 생성하는 커밋을 발생시켜야 한다.
- **And** 생성 파일셋에는 `kustomization.yaml`, `deployment.yaml`, `service.yaml`, `virtualservice.yaml`, `destinationrule.yaml`, `externalsecret.yaml` 이 포함되어야 한다.
- **And** Secret 평문 값은 Git 커밋이나 생성된 매니페스트에 포함되면 안 된다.
- **And** Secret 값은 먼저 Vault 임시 경로에 저장되고, Git 커밋 성공 후 최종 경로로 확정되어야 한다.

### Scenario: viewer 권한 사용자가 앱 생성을 시도한다
- **Given** 사용자가 `project-a` 에 대해 `viewer` 권한만 가지고 있다.
- **When** 사용자가 `POST /api/v1/projects/project-a/applications` 를 호출한다.
- **Then** 백엔드는 상태 코드 `403` 을 반환해야 한다.
- **And** 공통 에러 스키마를 따라 응답해야 한다.

---

## Bounding Context: GitOps 동기화 상태

### Scenario: 사용자가 앱의 sync 상태를 확인한다
- **Given** 앱 생성 또는 재배포 직후 앱 상세 화면에 진입했다.
- **When** 사용자가 `GET /api/v1/applications/{applicationId}/sync-status` 를 호출하거나 화면에서 새로고침을 누른다.
- **Then** 화면 헤더의 상태 뱃지는 `Unknown`, `Syncing`, `Synced`, `Degraded` 중 하나로 렌더링되어야 한다.
- **And** Flux 반영 완료 후에는 `Synced` 상태를 표시해야 한다.

---

## Bounding Context: Standard 재배포

### Scenario: 사용자가 새 이미지 태그로 재배포한다
- **Given** 이미 생성된 앱 `my-app` 이 존재한다.
- **When** 사용자가 `POST /api/v1/applications/{applicationId}/deployments` 로 새 이미지 태그(`v2`)를 전달한다.
- **Then** 백엔드는 상태 코드 `201` 을 반환해야 한다.
- **And** 백엔드는 기존 앱의 GitOps 리소스를 갱신하는 커밋을 발생시켜야 한다.
- **And** 사용자는 이후 sync 상태가 다시 갱신되는 것을 앱 상세 화면에서 확인할 수 있어야 한다.

---

## Bounding Context: 모니터링 및 메트릭 관제

### Scenario: 메트릭 정보를 렌더링한다
- **Given** 앱 `my-app` 이 1개 이상의 레플리카로 구동 중이다.
- **When** 사용자가 앱 상세 화면에서 `GET /api/v1/applications/{applicationId}/metrics` 를 호출한다.
- **Then** 백엔드는 요청 수, 에러율, 응답 지연, CPU 사용량, 메모리 사용량 정보를 JSON으로 반환해야 한다.
- **And** 프론트엔드는 이를 카드 또는 차트 형태로 렌더링해야 한다.
- **And** 일부 메트릭 값이 비어 있어도 UI는 깨지지 않아야 한다.

---

## Bounding Context: 공통 에러 응답

### Scenario: 잘못된 요청 형식을 보낸다
- **Given** 사용자가 필수 필드 없는 앱 생성 요청을 보낸다.
- **When** 백엔드가 요청을 검증한다.
- **Then** 백엔드는 상태 코드 `400` 을 반환해야 한다.
- **And** 응답 본문은 공통 에러 스키마의 `code`, `message`, `requestId` 를 포함해야 한다.

### Scenario: 중복 앱을 생성하려고 한다
- **Given** `project-a/my-app` 경로에 이미 앱이 존재한다.
- **When** 같은 이름으로 다시 앱 생성을 요청한다.
- **Then** 백엔드는 상태 코드 `409` 을 반환해야 한다.
- **And** 응답 본문은 공통 에러 스키마를 따라야 한다.

---

## Phase 1 제외 항목

아래 항목은 이번 인수 조건에 포함하지 않는다.

* Canary 배포 실행
* Promote / Abort 동작
* 메트릭 기준 자동 rollback

이 항목들은 Phase 2 인수 조건에서 별도로 다룬다.
