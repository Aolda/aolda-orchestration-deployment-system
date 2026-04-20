# Frontend Worklist

이 문서는 AODS 프론트엔드에서 앞으로 해야 할 일을 에이전트와 사람이 빠르게 잡을 수 있도록 정리한 작업 목록이다.

기준:

* 최소 계약 문서는 `docs/internal-platform/openapi.yaml`, `docs/acceptance-criteria.md`, `docs/phase1-decisions.md`, `docs/domain-rules.md` 를 따른다.
* 실제 현재 구현 상태와 회귀 기준선은 `docs/current-implementation-status.md` 를 함께 본다.
* 페이지 구조와 UX 목표는 `docs/frontend-page-plan.md` 를 함께 본다.
* 이미 노출된 later-phase UI 는 current implementation baseline 일부로 유지한다.
* 사용자가 명시적으로 요청하지 않으면 완전히 새로운 future-scope UI 를 임의로 열지 않는다.

## 이미 구현된 것

아래는 다시 설계할 필요가 없는 항목이다.

* 로그인 게이트용 기본 화면
* 프로젝트 목록 조회
* 프로젝트 선택 기반 앱 카드 대시보드
* 앱 운영 센터 Drawer
* 메트릭 카드와 시계열 상세 표시
* 배포 실행 UI
* 배포 이력 테이블
* 앱 단위 rollback policy 조회/저장 UI
* 긴급 중단 / 직전 버전 롤백 UI
* 프로젝트 설정 Drawer
* 분리 중인 `ProjectsWorkspace`, `ChangesWorkspace`, `ClustersPage`, `MePage`

즉, 프론트 기본 뼈대는 있다.
앞으로의 작업은 대부분 `구조 정리`, `전역 IA 연결`, `later-phase 흐름 완성`, `운영 UX 보강` 쪽이다.

## Default Priority

별도 지시가 없으면 아래 우선순위를 따른다.

1. 현재 라이브 구조와 목표 IA의 불일치 해소
2. 권한/보호 환경 때문에 사용자가 오해할 수 있는 UX 정리
3. 이미 노출된 later-phase 흐름의 실제 화면 완성
4. 운영 관측성과 디버깅 UX 강화
5. 시각 polish 보다 상태/동작 정합성 우선

## Active Backlog

### 1. FE-01 프로젝트 메인 구조 이관

상태: `Recommended next`

이슈 제목:

* `refactor(frontend): move live project dashboard into ProjectsWorkspace and AppShell`

왜 필요한가:

* 현재 실제 화면은 `frontend/src/App.tsx` 에 몰려 있다.
* 코드상 목표 IA 는 `projects / changes / clusters / me` 로 분리돼 있지만, 라이브 UI 는 아직 이를 전면 사용하지 않는다.
* 이후 `Changes`, `Clusters`, `Me` 를 제대로 연결하려면 먼저 프로젝트 메인 구조를 분리된 workspace / shell 모델로 옮겨야 한다.

작업 범위:

* 현재 프로젝트 대시보드 레이아웃을 `ProjectsWorkspace` 기준으로 재구성한다.
* 전역 shell 을 `AppShell` 기준으로 연결한다.
* 기존 앱 운영 Drawer, 새 앱 생성 Drawer, 프로젝트 설정 Drawer 는 동작을 유지한다.
* `App.tsx` 의 인라인 레이아웃 책임을 줄인다.

주요 수정 위치:

* `frontend/src/App.tsx`
* `frontend/src/pages/projects/ProjectsWorkspace.tsx`
* `frontend/src/app/layout/AppShell.tsx`
* `frontend/src/app/layout/navigation.ts`
* `frontend/src/components/navigation/SidebarNav.tsx`
* `frontend/src/App.module.css`
* `frontend/src/app/layout/AppShell.module.css`

완료 조건:

* 프로젝트 화면이 `ProjectsWorkspace` 를 중심으로 렌더링된다.
* 프로젝트 개요 / 애플리케이션 / 변경 요청 / 운영 규칙 탭 책임이 구분된다.
* 기존 프로젝트 선택, 앱 선택, Drawer 열기 흐름이 깨지지 않는다.
* `npm run lint` 와 `npm run build` 가 통과한다.

예상 공수:

* `1.5 ~ 2.5일`

### 2. FE-02 Changes 페이지 독립화

상태: `Shell delivered, backend support needed for full completion`

이슈 제목:

* `feat(frontend): build first-class changes workspace with list, detail, diff, and actions`

왜 필요한가:

* 기존 `ChangesWorkspace` 는 설명용 컨테이너에 가까웠다.
* `Change` 리소스가 이미 존재하고 submit / approve / merge 도 있지만, 프론트에서는 아직 목록 기반 운영 흐름이 없다.
* `pull_request` 성격의 review-gated change flow 를 제품적으로 보여주려면 `Changes` 가 독립 화면이 되어야 한다.

작업 범위:

* 변경 요청 목록 영역을 만든다.
* 상태, 환경, 작성자 기준 필터/정렬 UX 를 만든다.
* 선택된 change 의 상세, diff preview, action bar 를 만든다.
* submit / approve / merge 액션을 UI 안에서 연결한다.
* 목록 API가 없더라도 세션 추적 + change ID 수동 불러오기 기반으로 first-class workspace shell 을 만든다.

주요 수정 위치:

* `frontend/src/pages/changes/ChangesWorkspace.tsx`
* `frontend/src/App.tsx`
* `frontend/src/types/api.ts`
* `frontend/src/api/client.ts`
* 필요 시 새 `frontend/src/pages/changes/*` 하위 컴포넌트

백엔드 의존성:

* 현재는 `POST /api/v1/projects/{projectId}/changes` 와 `GET /api/v1/changes/{changeId}` 는 있으나, `프로젝트별 change 목록 조회` API 는 없다.
* 전체 완성을 위해서는 `GET /api/v1/projects/{projectId}/changes` 또는 동등한 목록 API 가 필요하다.

완료 조건:

* 사용자가 프로젝트 기준으로 변경 요청 목록을 본다.
* 상태, 환경, 작성자, 최신순 기준 탐색이 가능하다.
* change 상세에서 diff preview 와 submit / approve / merge 액션이 가능하다.
* `pull_request` 환경은 direct deploy 보다 change flow CTA 가 우선 노출된다.
* 프론트 단독 검증 시에는 목록 API 부재를 명시하고 mocked shell 수준까지는 정리할 수 있다.

예상 공수:

* `프론트 쉘만 1일`
* `백엔드 포함 end-to-end 2.5 ~ 4일`

### 3. FE-03 권한 및 보호 환경 UX 적용

상태: `Delivered`

이슈 제목:

* `feat(frontend): add role-gated actions and protected-environment UX`

왜 필요한가:

* 현재 UI 는 많은 액션을 보여주지만, `viewer / deployer / admin` 차이와 `direct / pull_request` 차이가 충분히 드러나지 않는다.
* 사용자는 버튼이 보이면 쓸 수 있다고 기대하기 때문에, 권한/환경 규칙이 화면에서 먼저 드러나야 한다.

작업 범위:

* role 별 액션 visible/disabled 정책을 정한다.
* `viewer` 에게는 조회 전용 UX 를 보인다.
* `prod` 또는 `pull_request` 성격 환경에서는 direct deploy 대신 change request 유도 CTA 를 보여준다.
* destructive action 은 더 명시적인 설명과 confirm 흐름을 붙인다.

주요 수정 위치:

* `frontend/src/App.tsx`
* `frontend/src/pages/projects/ProjectsWorkspace.tsx`
* `frontend/src/pages/changes/ChangesWorkspace.tsx`
* `frontend/src/types/api.ts`
* 필요 시 공통 CTA / guard 컴포넌트

완료 조건:

* `viewer` 는 배포/중단/정책 저장 같은 액션을 직접 수행할 수 없다.
* `deployer` 와 `admin` 의 액션 가능 범위가 화면에 반영된다.
* 보호 환경에서는 direct deploy 버튼보다 change flow 진입이 우선된다.
* 사용 불가 액션은 이유가 텍스트로 보인다.

예상 공수:

* `1 ~ 1.5일`

### 4. FE-04 앱 운영 센터 관측 탭 강화

상태: `Delivered with placeholder diagnostics`

이슈 제목:

* `feat(frontend): strengthen application observability workspace with diagnostics entry points`

왜 필요한가:

* 현재 운영 센터는 metrics / events / deploy / history / rules 는 있으나, 관측 축이 분산돼 있고 logs 진입이 없다.
* 외부 사례 기준으로 배포 운영 화면은 metrics 뿐 아니라 events, history, failure context, logs 진입이 함께 있어야 한다.

작업 범위:

* 운영 센터 탭 구조를 `상태`, `배포`, `관측`, `이력`, `규칙` 수준으로 재정리한다.
* metrics 와 events 의 관계를 하나의 관측 맥락으로 묶는다.
* logs / diagnostics 가 아직 없으면 placeholder 와 contract hook 을 준비한다.
* 실패 배포의 메시지와 원인을 더 잘 보이게 한다.

주요 수정 위치:

* `frontend/src/App.tsx`
* `frontend/src/types/api.ts`
* `frontend/src/api/client.ts`
* 필요 시 새 observability 관련 컴포넌트

백엔드 의존성:

* 실제 logs / diagnostics 구현은 신규 API 계약이 필요할 가능성이 높다.

완료 조건:

* 운영 센터에서 `배포 상태`와 `관측 정보`의 역할이 분리된다.
* metrics, events, history 가 더 자연스럽게 탐색된다.
* 향후 logs API 를 붙일 자리가 화면 구조상 준비된다.

예상 공수:

* `프론트 구조 정리 1 ~ 1.5일`
* `로그 API 포함 시 3일 이상`

### 5. FE-05 공통 상태 화면 체계화

상태: `Delivered`

이슈 제목:

* `chore(frontend): standardize loading empty error and forbidden states across pages`

왜 필요한가:

* 현재는 성공 경로 중심으로 화면이 짜여 있고, 빈 상태나 권한 부족 상태는 페이지별 일관성이 약하다.
* 이 작업은 큰 기능보다 작아 보이지만, 실제 체감 완성도를 크게 올린다.

작업 범위:

* 공통 `loading / empty / partial / error / forbidden` 패턴을 만든다.
* 프로젝트 없음, 앱 없음, metrics 없음, policy 미조회, 권한 없음 케이스를 정리한다.
* 페이지별 placeholder 문구와 CTA 를 통일한다.

주요 수정 위치:

* `frontend/src/App.tsx`
* `frontend/src/components/ui/PageHeader.tsx`
* `frontend/src/components/ui/SurfaceCard.tsx`
* `frontend/src/pages/clusters/ClustersPage.tsx`
* `frontend/src/pages/me/MePage.tsx`
* 필요 시 새 상태 표시 컴포넌트

완료 조건:

* 주요 화면이 최소 5개 상태를 일관되게 가진다.
* forbidden 상태에서 왜 막혔는지 보인다.
* 빈 화면이 단순 공백이 아니라 다음 액션을 안내한다.

예상 공수:

* `0.5 ~ 1일`

## Ongoing Maintenance Tasks

### FE-06 유저저니 기반 품질 게이트

상태: `Delivered`

이슈 제목:

* `test(frontend): add user-story based Vitest gates for critical journeys`

왜 필요한가:

* 기존 프론트 검증은 사실상 `lint` 중심이라, 사용자가 실제로 밟는 흐름 회귀를 잡기 어려웠다.
* `App.tsx` 분리 작업이 진행 중인 상태라서, 화면 구조를 옮기는 동안에도 핵심 유저저니를 고정된 계약으로 묶어둘 필요가 있다.

작업 범위:

* `Vitest + React Testing Library` 테스트 하네스를 추가한다.
* 핵심 프론트 유저 스토리를 `frontend/user-stories.json` 과 `docs/frontend-user-journeys.md` 로 관리한다.
* 테스트 제목에 `[US-...]` 식별자를 넣고, `npm run test:stories` 로 누락 여부를 검사한다.
* 루트 `make check` 에 프론트 테스트 게이트를 포함한다.

주요 수정 위치:

* `frontend/package.json`
* `frontend/vite.config.ts`
* `frontend/src/testing/`
* `frontend/src/pages/projects/ProjectsWorkspace.test.tsx`
* `frontend/src/pages/changes/ChangesWorkspace.test.tsx`
* `frontend/user-stories.json`
* `docs/frontend-user-journeys.md`

완료 조건:

* 최소 핵심 화면(`ProjectsWorkspace`, `ChangesWorkspace`)의 critical story 가 테스트로 묶여 있다.
* 새 유저 흐름을 추가할 때 문서, story manifest, 테스트가 함께 갱신되는 규칙이 생긴다.
* `npm run lint`, `npm run test`, `npm run test:stories`, `npm run build` 가 통과한다.

예상 공수:

* `초기 구축 0.5 ~ 1일`

### A. API Change Discipline

항상 해야 하는 일:

* `openapi.yaml` 변경
* 프론트 `types/api.ts` 변경
* `api/client.ts` 변경
* 실제 연결 화면 변경
* lint / build 확인

이 항목은 별도 기능이 아니라 상시 규칙이다.

### B. Keep Plan And UI In Sync

항상 해야 하는 일:

* `docs/frontend-page-plan.md` 와 실제 화면 구조가 달라지면 같이 갱신한다.
* `docs/current-implementation-status.md` 에 영향을 주는 later-phase UI 진전이 있으면 함께 반영한다.
* 임시 버튼, placeholder 링크, 개발용 카피는 그대로 방치하지 않는다.

## Not A Default Task

아래는 명시적 요청 없이는 시작하지 않는다.

* 디자인 시스템 전면 재구축
* 로그/진단 API 없는 상태에서 fake log viewer 를 과하게 구현하는 일
* 전역 라우터 대수술만을 위한 대규모 프론트 재작성
* 범용 쿠버네티스 콘솔 같은 UI 확장

## Quick Start For Frontend Work

프론트 작업을 시작할 때는 아래 순서를 기본으로 따른다.

1. `docs/internal-platform/openapi.yaml`
2. `docs/acceptance-criteria.md`
3. `docs/phase1-decisions.md`
4. `docs/domain-rules.md`
5. `docs/current-implementation-status.md`
6. `docs/frontend-page-plan.md`
7. `docs/frontend-worklist.md`
8. `docs/agent-code-index.md`
9. `frontend/src/App.tsx`
