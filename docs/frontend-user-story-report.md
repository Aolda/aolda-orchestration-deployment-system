# Frontend User Story Report

Generated at: `2026-04-13T07:23:27.517Z`

* Total critical stories: `8`
* Covered: `8`
* Uncovered: `0`

## US-NAV-001

* Screen: `SidebarNav`
* Persona: `viewer`
* Status: `covered`
* Story: 글로벌 사이드바는 프로젝트 메뉴 아래에 접근 가능한 프로젝트 목록을 노출한다.
* Tests: `frontend/src/components/navigation/SidebarNav.test.tsx`

## US-PROJ-001

* Screen: `ProjectsWorkspace`
* Persona: `viewer`
* Status: `covered`
* Story: 프로젝트를 선택하기 전에는 상세 운영 화면 진입 안내를 본다.
* Tests: `frontend/src/pages/projects/ProjectsWorkspace.test.tsx`

## US-PROJ-002

* Screen: `ProjectsWorkspace`
* Persona: `viewer`
* Status: `covered`
* Story: 프로젝트를 선택하면 운영 탭과 프로젝트 컨텍스트가 열린다.
* Tests: `frontend/src/pages/projects/ProjectsWorkspace.test.tsx`

## US-CHG-001

* Screen: `ChangesWorkspace`
* Persona: `viewer`
* Status: `covered`
* Story: 프로젝트 문맥이 없으면 change workspace 는 빈 상태와 CTA 제한을 먼저 보여준다.
* Tests: `frontend/src/pages/changes/ChangesWorkspace.test.tsx`

## US-CHG-002

* Screen: `ChangesWorkspace`
* Persona: `viewer`
* Status: `covered`
* Story: viewer 는 change draft 를 조회할 수 있지만 생성할 수는 없다.
* Tests: `frontend/src/pages/changes/ChangesWorkspace.test.tsx`

## US-CHG-003

* Screen: `ChangesWorkspace`
* Persona: `deployer`
* Status: `covered`
* Story: 운영자는 tracked changes 를 검색해 원하는 변경만 좁혀서 본다.
* Tests: `frontend/src/pages/changes/ChangesWorkspace.test.tsx`

## US-CHG-004

* Screen: `ChangesWorkspace`
* Persona: `deployer`
* Status: `covered`
* Story: 변경 요청 화면 첫 진입 시 가장 최근 tracked change 가 자동 선택된다.
* Tests: `frontend/src/pages/changes/ChangesWorkspace.test.tsx`

## US-CHG-005

* Screen: `ChangesWorkspace`
* Persona: `admin`
* Status: `covered`
* Story: pull_request 환경의 submitted change 는 관리자 승인 후에만 반영 가능하다.
* Tests: `frontend/src/pages/changes/ChangesWorkspace.test.tsx`

