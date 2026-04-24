# Frontend User Journeys

이 문서는 AODS 프론트엔드에서 핵심 사용자 흐름을 문서와 테스트 양쪽에서 같이 관리하기 위한 기준선이다.

목표는 세 가지다.

1. 유저 스토리를 화면 단위 계약으로 고정한다.
2. 프론트 테스트가 단순 렌더링 확인이 아니라 실제 사용자 흐름을 검증하게 만든다.
3. 앞으로 새 기능을 붙일 때도 문서, story manifest, 테스트를 같이 갱신하게 강제한다.

기준 파일:

* 문서 기준: `docs/frontend-user-journeys.md`
* 기계 판독용 manifest: `frontend/user-stories.json`
* 테스트 공통 유틸: `frontend/src/testing/`

## 품질 게이트

프론트 유저저니 검증은 아래 순서를 기본으로 한다.

1. `cd frontend && npm run lint`
2. `cd frontend && npm run test`
3. `cd frontend && npm run test:stories`
4. 필요하면 `cd frontend && npm run test:stories:report`
5. 공유 가능한 산출물이 필요하면 `cd frontend && npm run test:stories:export`
6. `cd frontend && npm run build`

설명:

* `npm run test` 는 `Vitest + React Testing Library` 기반 화면 테스트를 돌린다.
* `npm run test:stories` 는 `frontend/user-stories.json` 의 critical story ID 가 실제 테스트 제목에 `[US-...]` 형식으로 들어있는지 검사한다.
* `npm run test:stories:report` 는 현재 critical story 목록, persona, 화면, 연결된 테스트 파일, 커버 상태를 사람이 읽기 쉬운 리포트로 출력한다.
* `npm run test:stories:export` 는 최신 상태를 파일로 남긴다.
  - Markdown: `docs/frontend-user-story-report.md`
  - JSON: `frontend/public/reports/frontend-user-story-report.json`
* 루트 `make check` 도 이 체인을 포함한다.

## 현재 Critical Stories

### Global Sidebar

* `US-NAV-001`
  - 글로벌 사이드바는 프로젝트 메뉴 아래에 접근 가능한 프로젝트 목록을 노출한다.
  - 현재 운영 모드에서는 공용 프로젝트(`shared`)만 노출하고, 새 프로젝트 액션은 프론트에서 숨긴다.
  - 테스트 위치: `frontend/src/components/navigation/SidebarNav.test.tsx`

### Projects Workspace

* `US-PROJ-001`
  - 프로젝트를 선택하기 전에는 상세 운영 화면 진입 안내를 본다.
  - 테스트 위치: `frontend/src/pages/projects/ProjectsWorkspace.test.tsx`
* `US-PROJ-002`
  - 프로젝트를 선택하면 운영 탭과 프로젝트 컨텍스트가 열린다.
  - 테스트 위치: `frontend/src/pages/projects/ProjectsWorkspace.test.tsx`

### Application Creation

* `US-APP-001`
  - 새 애플리케이션 생성은 기본으로 공개 저장소 기준의 GitHub 등록 흐름을 보여준다.
  - 테스트 위치: `frontend/src/components/ApplicationWizard.test.tsx`
* `US-APP-002`
  - GitHub 등록에서 저장소 URL 없이 다음 단계로 넘어가면 검증 메시지와 입력 포커스를 제공한다.
  - 테스트 위치: `frontend/src/components/ApplicationWizard.test.tsx`
* `US-APP-003`
  - 공개 저장소는 토큰 없이도 설정 파일 확인 단계로 이동할 수 있다.
  - 테스트 위치: `frontend/src/components/ApplicationWizard.test.tsx`
* `US-APP-004`
  - 레지스트리 사용자명과 토큰을 하나만 입력하면 생성 전 검증 메시지를 보여준다.
  - 테스트 위치: `frontend/src/components/ApplicationWizard.test.tsx`
* `US-APP-005`
  - 비밀값 입력칸은 브라우저 자동완성을 방지한다.
  - 테스트 위치: `frontend/src/components/ApplicationWizard.test.tsx`
* `US-APP-006`
  - 설정 파일 확인 단계에서 여러 서비스를 보여주고 원하는 `serviceId`를 선택할 수 있다.
  - 테스트 위치: `frontend/src/components/ApplicationWizard.test.tsx`
* `US-APP-007`
  - 빠른 생성 모드는 저장소 설정 파일 확인 단계를 건너뛰고 배포 설정으로 이동한다.
  - 테스트 위치: `frontend/src/components/ApplicationWizard.test.tsx`

### Changes Workspace

* `US-CHG-001`
  - 프로젝트 문맥이 없으면 change workspace 는 빈 상태와 CTA 제한을 먼저 보여준다.
  - 테스트 위치: `frontend/src/pages/changes/ChangesWorkspace.test.tsx`
* `US-CHG-002`
  - `viewer` 는 change draft 를 조회할 수 있지만 생성할 수는 없다.
  - 테스트 위치: `frontend/src/pages/changes/ChangesWorkspace.test.tsx`
* `US-CHG-003`
  - 운영자는 tracked changes 를 검색해 원하는 변경만 좁혀서 본다.
  - 테스트 위치: `frontend/src/pages/changes/ChangesWorkspace.test.tsx`
* `US-CHG-004`
  - 변경 요청 화면 첫 진입 시 가장 최근 tracked change 가 자동 선택된다.
  - 테스트 위치: `frontend/src/pages/changes/ChangesWorkspace.test.tsx`
* `US-CHG-005`
  - `pull_request` 환경의 submitted change 는 관리자 승인 후에만 반영 가능하다.
  - 테스트 위치: `frontend/src/pages/changes/ChangesWorkspace.test.tsx`

## 운영 규칙

새 프론트 기능을 추가하거나 기존 UX 를 바꾸면 아래 규칙을 따른다.

* 새 사용자 흐름이 생기면 `frontend/user-stories.json` 에 story ID 를 추가한다.
* 같은 변경에서 `docs/frontend-user-journeys.md` 도 같이 갱신한다.
* 대응 테스트 제목에는 반드시 `[US-...]` 식별자를 넣는다.
* 권한(`viewer`, `deployer`, `admin`) 또는 상태 전이(`Draft`, `Submitted`, `Approved`, `Merged`)가 바뀌는 흐름은 우선적으로 critical story 로 본다.

## 범위 메모

현재 초기 커버리지는 `ProjectsWorkspace`, `ApplicationWizard`, `ChangesWorkspace` 중심이다.

이유:

* `App.tsx` 는 아직 구조 이관 중이라 전체 앱 단위 테스트보다 화면 단위 계약 고정이 더 안전하다.
* 프로젝트 선택, 앱 생성, 변경 요청 생성/승인 흐름은 Phase 1 및 현재 later-phase shell 에서 회귀 영향이 크다.

즉, 앞으로 `App.tsx` 분리가 더 진행되면 `Clusters`, `Me`, 앱 운영 센터도 같은 방식으로 story gate 를 확장한다.
