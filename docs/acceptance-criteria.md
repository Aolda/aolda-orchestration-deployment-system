# Acceptance Criteria (인수 조건, BDD 형태)

이 문서는 `#front-qa` 및 `#back-qa` 에이전트(또는 개발자)가 플랫폼 구축 코드를 검증할 때 사용하는 공통 테스트 시나리오(Test Cases)입니다. 시스템 설계상 아래의 조건을 100% 통과해야 Phase 1 구현 완료로 간주합니다.

---

## Bounding Context: 인증 및 프로젝트 관리

### Scenario: 사용자가 권한 내의 앱 목록을 조회한다
- **Given** 사용자가 단순 인증(Keycloak MVP)을 마친 후 대시보드 화면에 접속해 있다.
- **When** 프로젝트 `project-a` 상세 뷰를 클릭한다.
- **Then** `project-a`에 소속된 어플리케이션 목록만 표시되어야 하며 화면 로딩 응답(`GET /api/v1/projects/{projectId}/applications`)에 실패가 없어야 한다.

---

## Bounding Context: 앱 배포 생성 (GitOps Sync)

### Scenario: 새 어플리케이션(Standard 배포 전략)을 생성한다
- **Given** 사용자가 `project-a` 화면에서 `앱 생성` 버튼을 누르고, 배포 전략을 `Standard`로 선택했다.
- **When** 앱 이름(`my-app`), 이미지 주소(`repo/my-app:v1`), 서비스 포트(`8080`)를 입력하고 "배포 실행"을 누른다.
- **Then** 백엔드는 상태 코드 `201`을 반환한다.
- **And** 백엔드는 내부적으로 대상 Git 저장소의 `apps/project-a/my-app/base/` 경로에 템플릿된 매니페스트(Kustomization 및 Deployment)를 생성하는 Commit을 발생시킨다.

### Scenario: GitOps 동기화 상태를 확인한다
- **Given** 앞선 시나리오에서 앱 생성이 갓 완료되었다.
- **When** 사용자가 "앱 상세 화면"으로 진입하여 "동기화 상태 새로고침"을 누른다 (`GET /api/v1/applications/{applicationId}/sync-status`).
- **Then** 화면 헤더의 상태 뱃지는 초기 `Unknown` 또는 `Syncing`에서, Flux가 반영을 마친 뒤 `Synced` (Ready) 로 즉각 렌더링되어야 한다.

---

## Bounding Context: 모니터링 및 메트릭 관제

### Scenario: 메트릭 정보 렌더링 (Prometheus 연동)
- **Given** 이미 생성된 앱 `my-app`이 1개 이상의 레플리카로 구동 중이다.
- **When** 앱 상세 화면 하단의 메트릭 패널로 스크롤을 내린다. (`GET /api/v1/applications/../metrics`)
- **Then** 백엔드는 Prometheus API에 Query하여 얻어온 결과인 `요청 수`, `에러율`, `응답 지연`, `메모리/CPU 사용량` 정보를 JSON으로 넘겨주고, 프론트엔드는 이를 스파크라인이나 차트 대시보드 형태로 사용자에게 보여줘야 한다.
- **And** 메트릭 API 호출 시 빈 값(Null) 발생 처리를 올바르게 핸들링하여 UI가 무너지지 않도록 해야 한다.

---

## Bounding Context: Progressive Delivery (Canary)

*Note: Phase 2 중심 요구사항이나, API/UI 뼈대는 Phase 1에서 구현한다.*

### Scenario: 카나리 배포 전략 구동 및 수동 승격 (Promote)
- **Given** 배포 전략을 `Canary`로 미리 설정한 앱에 새 버전(`repo/my-app:v2`) 배포 요청이 진입한다.
- **When** 점진적 배포 단계가 진행 중일 때, 사용자가 "수동 승격(Promote)" 버튼을 누른다.
- **Then** 백엔드는 Argo Rollout 상태를 한 단계 진행시키고, UI에서는 트래픽 비중(Canary % 증가율)이 상승한 모습을 시각적으로 표시해야 한다.
