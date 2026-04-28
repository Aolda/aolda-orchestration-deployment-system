# User Feedback Log

이 문서는 사용자가 직접 준 제품 피드백, UX 수정 요청, 정보 구조 조정 요청을 계속 누적 기록하기 위한 전용 로그다.

목적은 세 가지다.

1. 사용자가 이미 말한 제품/UX 의도를 다음 작업에서도 잃지 않는다.
2. 한 번 반영한 피드백이 다시 되돌아가거나, 비슷한 중복 UI가 재등장하는 것을 줄인다.
3. 향후 화면 구조를 바꿀 때도 실제 사용자 취향과 운영 문맥을 참고한다.

`docs/mistake-log.md` 와의 차이:

* `mistake-log` 는 잘못된 판단, 회귀, 계약 위반, 운영 실수를 남긴다.
* 이 문서는 사용자가 원하는 방향, 불편했던 점, 개선 요청 자체를 남긴다.

## 언제 기록하나

아래와 같은 경우를 이 문서에 추가한다.

* 사용자가 특정 UI가 직관적이지 않다고 직접 말했을 때
* 정보 구조, 버튼 위치, 카피, 화면 위계에 대해 명시적으로 수정 방향을 줬을 때
* 이후 유사한 기능에도 반복 적용해야 할 취향/원칙을 전달했을 때
* 스크린샷, 영상, 화면 설명으로 구체 피드백을 줬을 때

## 기록 원칙

* 사용자의 표현을 가능한 한 보존하되, 후속 구현에 도움이 되게 짧게 정리한다.
* 피드백 그 자체와, 그 피드백이 의미하는 제품 의도를 같이 남긴다.
* 상태는 `new`, `applied`, `deferred`, `superseded` 중 하나로 기록한다.
* 관련 코드/문서 경로가 있으면 같이 남긴다.

## 기록 템플릿

```md
### YYYY-MM-DD - FEEDBACK-ID - 짧은 제목

* Area:
* User signal:
* Interpreted intent:
* Action:
* References:
* Status:
```

## Entries

### 2026-04-24 - FB-039 - Vault에 넣은 앱 환경 변수는 나중에 수정할 수 있어야 함

* Area: Frontend / Backend / Vault-backed Application Secrets
* User signal: `그 vault에 대해서 추후에 해당 넣은 env에 대해서 수정할 수 있는 페이지가 있어야할 것 같은데`
* Interpreted intent: 앱 생성 때 Vault에 저장한 `.env` 값은 일회성 입력으로 끝나면 안 되고, 운영 센터에서 값 노출 없이 키 추가/교체/삭제를 할 수 있어야 한다.
* Action: 앱 운영 센터에 환경 변수 탭을 추가하고, 백엔드에는 key-only 조회와 `set/delete` 기반 Vault Secret 갱신 API를 둔다.
* References: `frontend/src/App.tsx`, `backend/internal/application/service.go`, `docs/internal-platform/openapi.yaml`
* Status: applied

### 2026-04-18 - FB-038 - 외부 공개 단계는 멈춤이 아니라 실제 오류/준비 상태를 보여줘야 함

* Area: Frontend / Application Network Exposure
* User signal: `멈춰있는데 왜 그런거임? 오류가 생겼으면 오류가 생겼다 뭐 그런거 알려줘야하는거 아녀?`
* Interpreted intent: 외부 공개 처리 단계는 단순 진행도 UI가 아니라, 실제 클러스터 기준으로 `준비 중`, `준비 완료`, `오류`를 구분해서 설명해야 한다.
* Action: `LoadBalancer 준비` 단계를 Kubernetes Service/Event 기반 상태로 교체하고, 오류일 때는 주황/파랑 진행 표시 대신 경고 상태와 실제 오류 메시지를 노출하도록 정리한다.
* References: `frontend/src/App.tsx`, `backend/internal/kubernetes/real.go`, `docs/internal-platform/openapi.yaml`
* Status: applied

### 2026-04-17 - FB-036 - 운영 센터에서 직접 image tag 배포 입력 제거

* Area: Frontend / Operations Center
* User signal: `해당 이미지 변경에 대해서는 레포에서 진행하도록 하는게맞을 것 같아서 이거 관련해서 없애는게 좋지 않을까?`
* Interpreted intent: 운영 센터의 배포 탭은 직접 image tag 를 입력해 실행하는 곳이 아니라, 레포 기반 GitOps 반영 상태와 진행 상황을 확인하는 읽기 중심 화면이어야 한다.
* Action: 배포 탭에서 `TARGET IMAGE TAG` 입력과 직접 실행 버튼을 제거하고, 레포 반영 기준과 Flux 진행 상태를 안내하는 읽기 전용 구성으로 정리했다.
* References: `frontend/src/App.tsx`
* Status: applied

### 2026-04-17 - FB-037 - 기본 배포 상세에서는 빈 rollout 필드보다 실제 배포 정보가 먼저 보여야 함

* Area: Frontend / Deployment History
* User signal: `이거 실제로 어케 띄워져 있는지 뭔가 믿음이 안가긴 하네` / `걍 지금은 조절할 필요 없을 듯? 걍 배포 default 값으로 진행되게만 해서`
* Interpreted intent: 기본 배포에서는 rollout/canary 제어 필드보다 실제 환경, 이미지, 커밋, 시각 같은 확인 가능한 정보가 먼저 보여야 하며, 고급 필드는 카나리아 전략일 때만 드러나야 한다.
* Action: 배포 상세 패널을 기본 배포 중심으로 재구성하고, 빈 rollout/canary 필드는 숨기고 실제 배포 메타데이터를 우선 노출하도록 정리했다.
* References: `frontend/src/App.tsx`, `frontend/src/types/api.ts`
* Status: applied

### 2026-04-17 - FB-035 - GitHub 등록 첫 단계에서 서비스 ID와 앱 이름 수동 입력 제거

* Area: Frontend / Application Creation
* User signal: `이런 값 넣는건 또 있는데? 이런거 맞춰서 없애줘 알아서 파싱해서 넣어줘야하는거잖아`
* Interpreted intent: GitHub 연결 플로우 첫 단계에서는 사용자가 `repositoryServiceId` 나 앱 이름을 직접 채우면 안 되고, `aolda_deploy.json` 파싱 결과와 서비스 선택 단계가 단일 source of truth 여야 한다.
* Action: GitHub 등록 첫 단계에서 `저장소 내 서비스 ID`, `애플리케이션 이름` 입력을 제거하고, 이름 결정은 `설정 파일 확인` 단계의 자동 선택/수동 선택 결과에만 의존하도록 정리했다.
* References: `frontend/src/components/ApplicationWizard.tsx`, `frontend/src/components/ApplicationWizard.test.tsx`, `frontend/src/App.tsx`
* Status: applied

### 2026-04-13 - FB-001 - 글로벌 사이드바 아래에 프로젝트 목록 노출

* Area: Frontend / Global Navigation
* User signal: `프로젝트/변경 요청/클러스터/내 정보 아래에 프로젝트 아래에 내가 접근 가능한 프로젝트 목록이 보이게`
* Interpreted intent: 프로젝트 선택은 화면 본문이 아니라 글로벌 탐색 구조 안에서 계속 유지되어야 한다.
* Action: 글로벌 `SidebarNav`의 `프로젝트` 메뉴 아래에 접근 가능한 프로젝트 목록을 붙이고, 클릭 시 프로젝트 화면과 탭 컨텍스트를 바로 연다.
* References: `frontend/src/components/navigation/SidebarNav.tsx`, `frontend/src/app/layout/AppShell.tsx`, `frontend/src/App.tsx`
* Status: applied

### 2026-04-13 - FB-002 - 프로젝트 상단 정보 중복 제거

* Area: Frontend / Projects Overview
* User signal: `비슷한 게 두개가 있잖아? 이거에 대한 부분을 합쳐달라는거임 차피 같은 역할을 하니깐`
* Interpreted intent: 같은 프로젝트 헤더 정보는 한 번만 보여야 하고, 제목/설명/메타/주요 액션은 상단 한 군데로 모아야 한다.
* Action: TopBar와 중복되는 프로젝트 헤더 패널을 제거하고, 프로젝트 설정/새 애플리케이션 액션과 메타 배지를 상단으로 통합한다.
* References: `frontend/src/components/layout/TopBar.tsx`, `frontend/src/app/layout/AppShell.tsx`, `frontend/src/pages/projects/ProjectsWorkspace.tsx`, `frontend/src/App.tsx`
* Status: applied

### 2026-04-13 - FB-003 - 개요 화면은 한국어 중심의 운영 요약 페이지여야 함

* Area: Frontend / Projects Overview
* User signal: `어떤 부분을 내가 클릭 가능한건지, 뭘 할수 있을지 직관적으로 다가오지 않아 한국어로 작성해주던가` / `대표 애플리케이션 이렇게 하지말고 좀 지표 같은게 잘 보이게 해줘`
* Interpreted intent: 개요 화면은 영어 섞인 카드 모음이 아니라, 한국어 중심의 운영 요약 페이지여야 하고, 클릭 가능한 액션은 버튼/카피로 명확히 드러나야 한다.
* Action: 개요 섹션의 영어 라벨을 한국어로 바꾸고, 프로젝트 운영 요약 문장을 추가하고, 애플리케이션 카드에는 `운영 센터 열기` CTA를 명시하는 방향으로 정리 중이다.
* References: `frontend/src/App.tsx`, `frontend/src/App.module.css`
* Status: new

### 2026-04-13 - FB-004 - 사용자에게 보이는 목업 데이터 금지

* Area: Frontend / Backend / Working Rules
* User signal: `목업에 대해서 다 없애줘 이제는 목업은 사용하지 않아 목업 앞으로도 만들지마`
* Interpreted intent: 사용자에게 보이는 런타임 데이터와 상태 표현은 실제 연동 기준이어야 하며, 연동이 없으면 없는 상태를 그대로 드러내야 한다.
* Action: 로컬 fabricated metrics/sync/rollout 값을 제거하고, placeholder observability 패널을 실제 상태/empty state 기반 구성으로 교체했다. 에이전트 작업 지침에도 runtime mock 금지 규칙을 추가했다.
* References: `backend/internal/application/metrics_local.go`, `backend/internal/kubernetes/local.go`, `frontend/src/App.tsx`, `AGENTS.md`, `frontend/AGENTS.md`, `backend/AGENTS.md`, `docs/current-baseline-runbook.md`
* Status: applied

### 2026-04-13 - FB-005 - 변경 요청 진입점은 당분간 숨김

* Area: Frontend / Navigation
* User signal: `이거 일단 숨겨줘 지금은 필요없음`
* Interpreted intent: 현재 사용자 QA 흐름에서는 변경 요청 기능을 전면에 노출하지 말고, 프로젝트 탭과 글로벌 네비게이션에서 우선 감춰야 한다.
* Action: 프로젝트 상세 탭과 글로벌 사이드바에서 `변경 요청` 진입점을 숨기고, draft 생성 후 해당 섹션으로 자동 이동하던 흐름도 제거한다.
* References: `frontend/src/app/layout/navigation.ts`, `frontend/src/pages/projects/ProjectsWorkspace.tsx`, `frontend/src/App.tsx`
* Status: applied

### 2026-04-13 - FB-006 - 애플리케이션 탭은 앱별 간략 지표와 상세 진입점을 함께 보여야 함

* Area: Frontend / Applications Tab
* User signal: `이 탭에 그 애플리케이션 마다 지표를 볼 수 있게 간략히 표현할 수 있으면 좋을거 같은데 한눈에 애플리케이션들을 전부다 볼 수 있고 해당 특정 애플리케이션에 대해서 궁금하다 하면 클릭해서 볼 수 있게`
* Interpreted intent: 애플리케이션 탭은 단순 목록이 아니라, 앱별 현재 운영 상태를 빠르게 훑고 필요할 때만 상세 운영 화면으로 들어가는 2단 구조여야 한다.
* Action: 애플리케이션 카드에 최근 배포, 최근 상태, 간략 메트릭 요약과 상세 진입 CTA를 붙이는 방향으로 개선한다.
* References: `frontend/src/App.tsx`, `frontend/src/App.module.css`
* Status: applied

### 2026-04-13 - FB-007 - 새 프로젝트는 현재 프로젝트 상단이 아니라 글로벌 영역에 있어야 함

* Area: Frontend / Global Navigation
* User signal: `프로젝트에 들어갔는데 새 프로젝트가 있는거 자체가 이상함`
* Interpreted intent: `새 프로젝트`는 현재 프로젝트 컨텍스트 액션이 아니라, 글로벌 탐색 영역에서 열어야 하는 상위 작업이다.
* Action: 프로젝트 상단 액션에서 `새 프로젝트`를 제거하고, 사이드바의 프로젝트 영역 아래로 이동한다.
* References: `frontend/src/App.tsx`, `frontend/src/app/layout/AppShell.tsx`, `frontend/src/components/navigation/SidebarNav.tsx`
* Status: applied

### 2026-04-13 - FB-008 - 새 프로젝트 입력은 왼쪽 사이드바에서 바로 열려야 함

* Area: Frontend / Global Navigation
* User signal: `프로젝트 생성 버튼을 눌러도 그 왼쪽 사이드에서 나와서 해당 내용을 입력 가능하게는 안되나?`
* Interpreted intent: 글로벌 생성 액션은 메인 본문으로 밀어 넣지 말고, 사용자가 버튼을 누른 바로 그 사이드바 맥락에서 펼쳐서 입력하게 해야 한다.
* Action: 이후 사용자가 `왼쪽이 아니라 오른쪽에서 나오도록 즉 그 새애플리케이션 할때처럼`이라고 다시 명확히 수정했으므로, 이 요구는 우측 드로어 방식으로 대체됐다.
* References: `frontend/src/App.tsx`, `frontend/src/app/layout/AppShell.tsx`, `frontend/src/App.module.css`
* Status: superseded

### 2026-04-13 - FB-009 - 새 프로젝트 입력은 새 애플리케이션처럼 우측 드로어에서 열려야 함

* Area: Frontend / Global Navigation
* User signal: `아 미안 왼쪽이 아니라 오른쪽에서 나오도록 즉 그 새애플리케이션 할때처럼 말이지`
* Interpreted intent: `새 프로젝트`는 글로벌 액션이지만, 입력 경험은 현재 제품의 우측 드로어 패턴과 일치해야 한다.
* Action: 사이드바에는 `새 프로젝트` 버튼만 남기고, 실제 입력 폼은 `새 애플리케이션`과 같은 우측 드로어에서 열리도록 정리한다.
* References: `frontend/src/App.tsx`, `frontend/src/components/navigation/SidebarNav.tsx`
* Status: applied

### 2026-04-13 - FB-010 - 새 프로젝트 드로어 폭은 더 넓어야 함

* Area: Frontend / Global Navigation
* User signal: `이것도 오른쪽에서 나온 창이 더 컸으면 좋겠는데`
* Interpreted intent: 프로젝트 생성 드로어는 현재 폭이 좁아 보이므로, 입력과 안내 문구를 더 여유 있게 볼 수 있도록 넓혀야 한다.
* Action: `새 프로젝트` 우측 드로어 폭을 더 넓은 반응형 크기로 조정한다.
* References: `frontend/src/App.tsx`
* Status: applied

### 2026-04-13 - FB-011 - 프로젝트 삭제는 가능해야 하지만 공용 프로젝트는 보호되어야 함

* Area: Frontend / Backend / Project Settings
* User signal: `그 프로젝트에 대한 삭제도 진행될 수 있도록 해줘 단 공용 프로젝트는 삭제됨녀 안됨`
* Interpreted intent: 일반 프로젝트는 운영자가 정리할 수 있어야 하지만, 공용 프로젝트는 UI와 API 양쪽에서 삭제를 막아야 한다.
* Action: 프로젝트 설정 드로어에 삭제 액션과 확인 단계를 추가하고, 백엔드에는 `DELETE /api/v1/projects/{projectId}` 및 `shared` 네임스페이스 보호 규칙을 구현한다.
* References: `frontend/src/App.tsx`, `frontend/src/api/client.ts`, `backend/internal/project/service.go`, `backend/internal/project/source_local.go`, `docs/internal-platform/openapi.yaml`
* Status: applied

### 2026-04-14 - FB-012 - GitHub 기반 앱 등록은 저장소 선택과 반영 결과가 직관적으로 보여야 함

* Area: Frontend / Backend / Application Creation
* User signal: `그 깃헙을 기반으로해서 애플리케이션을 등록하는 그 플로우에 대해서 사용자 입장에서 더 잘 사용할 수 있게 만들고 싶은데 가능?`
* Interpreted intent: 앱 생성은 단순 폼 입력이 아니라, `빠른 생성`과 `GitHub 저장소 연결`을 구분하고, 어떤 저장소가 연결되는지와 생성 후 GitOps/Vault에 무엇이 반영되는지를 사전에 이해할 수 있어야 한다.
* Action: 새 애플리케이션 드로어를 저장소 연결 중심 위저드로 재구성하고, 저장소 선택, direct 환경만 허용하는 배포 설정, 단계별 검증, GitOps/Vault 반영 미리보기를 추가했다. 백엔드도 `repositoryId`가 현재 프로젝트에 연결된 저장소인지 검증하고 기본 `configPath`를 정규화하도록 보강했다.
* References: `frontend/src/components/ApplicationWizard.tsx`, `frontend/src/components/ApplicationWizard.test.tsx`, `frontend/src/App.tsx`, `backend/internal/application/service.go`, `backend/internal/application/service_test.go`, `docs/frontend-page-plan.md`
* Status: applied

### 2026-04-16 - FB-013 - Keycloak은 새 role 설계보다 기존 group hierarchy를 재사용해야 함

* Area: Backend / Auth / Documentation
* User signal: `이미 설정이 다 되어있어 ... Ajou_Univ/Aolda_Member/.../ops 에 권한을 줘야하는거고 Ajou_Univ/Aolda_Admin 에 대해서는 어드민 처럼 다 할 수 있게`
* Interpreted intent: AODS 권한 모델은 Keycloak 안에 이미 운영 중인 group 구조를 그대로 받아야 하며, agent 가 임의로 client role 체계를 새로 제안하거나 강제하면 안 된다.
* Action: backend/platform admin 판별을 환경변수 기반 global override 로 바꾸고, Keycloak group full path 기준 문서와 실행 예시를 추가했다. 앞으로 auth 작업은 `docs/keycloak-group-auth-model.md` 를 먼저 확인한다.
* References: `backend/internal/project/service.go`, `backend/internal/cluster/service.go`, `frontend/src/App.tsx`, `docs/keycloak-group-auth-model.md`
* Status: applied

### 2026-04-16 - FB-014 - Keycloak 장애 시에는 일반 비상 로그인으로도 진입 가능해야 함

* Area: Frontend / Auth / Local QA
* User signal: `keycloak 오류 시에도 들어갈 수 있게 일반적으로 치고 들어가는 것도 넣어줬으면 좋겠음`
* Interpreted intent: Keycloak 연동을 기본으로 유지하되, 로컬 QA나 장애 상황에서는 사용자가 브라우저에서 완전히 막히지 않도록 보조 진입 경로가 필요하다.
* Action: OIDC 로그인 화면에 로컬 비상 로그인 폼을 추가했고, 비상 세션일 때는 frontend가 Bearer 토큰 주입을 멈추고 backend dev fallback 계정으로 진입하도록 연결했다. 현재 로컬 실행 환경에서는 `admin/admin` 비상 로그인으로 접근 가능하다.
* References: `frontend/src/auth/oidc.ts`, `frontend/src/api/client.ts`, `frontend/src/App.tsx`, `.envrc`, `frontend/.env.local`
* Status: applied

### 2026-04-16 - FB-015 - 공용 프로젝트 첫 화면은 개요보다 애플리케이션이 먼저여야 함

* Area: Frontend / Projects Navigation
* User signal: `공용 프로젝트에 들어갔을 때 개요가 굳이 필요할까? 걍 애플리케이션 여러개 뜨게 하는게 나을 것 같은데 처음 화면으로`
* Interpreted intent: 공용 프로젝트는 운영 앱들을 빠르게 훑고 들어가는 허브 역할이 더 크므로, 첫 진입 탭은 개요보다 애플리케이션이 더 적합하다.
* Action: shared 네임스페이스 프로젝트를 선택하거나 자동 선택할 때 기본 탭을 `개요`가 아니라 `애플리케이션`으로 열리게 조정했다. 개요 탭은 유지하되 첫 진입 경로만 바꿨다.
* References: `frontend/src/App.tsx`
* Status: applied

### 2026-04-16 - FB-016 - 프로젝트 상세는 개요 대신 애플리케이션과 모니터링으로 분리되어야 함

* Area: Frontend / Projects Information Architecture
* User signal: `갯수나 sync 상태, 최근 배포 태그 최근 배포 상태등은 애플리케이션에 있어야되는거 같고 해당 cpu, 메모리 트래픽은 따로 탭을 하나 둬서`
* Interpreted intent: 프로젝트 화면은 메타 요약 중심의 `개요`보다, 운영자가 바로 쓰는 `애플리케이션`과 `모니터링` 중심 구조가 더 직관적이다.
* Action: 프로젝트 상세 탭을 `애플리케이션 / 모니터링 / 운영 규칙` 구조로 재편하고, `새 애플리케이션` 액션은 글로벌 헤더가 아니라 애플리케이션 탭 상단으로 고정했다.
* References: `frontend/src/App.tsx`, `frontend/src/pages/projects/ProjectsWorkspace.tsx`
* Status: applied

### 2026-04-16 - FB-017 - 운영 규칙 값은 물음표 도움말로 바로 설명되어야 함

* Area: Frontend / Project Rules
* User signal: `? 형태의 아이콘으로다가 가져온다음 내가 호버하면 딱 설명이 보이게 하는거지`
* Interpreted intent: 운영 규칙 값은 관리자에게도 의미 해석 비용이 있으므로, 필드별 정의를 즉시 확인할 수 있는 inline help가 필요하다.
* Action: 운영 규칙 탭의 저장소, 기본 정보, 운영 환경, 배포 정책 주요 라벨에 hover tooltip 기반 `?` 도움말을 추가했다.
* References: `frontend/src/App.tsx`
* Status: applied

### 2026-04-16 - FB-018 - 애플리케이션 상태 요약은 작은 배지보다 큰 카드형이어야 함

* Area: Frontend / Applications Tab
* User signal: `이거에 대해서 더 크게해서 한번에 잘 보이게 해줘 사용자 입장에서 솔직히 잘 보이지도 않음`
* Interpreted intent: 프로젝트 첫 화면의 상태 요약은 장식용 badge가 아니라, 운영자가 숫자를 바로 읽을 수 있는 크기의 요약 카드여야 한다.
* Action: 애플리케이션 탭 상단의 상태 요약을 작은 badge 줄에서 큰 stat card 그리드로 교체했다.
* References: `frontend/src/App.tsx`
* Status: applied

### 2026-04-16 - FB-019 - 프로젝트 설정은 열기만 하는 읽기 화면이 아니라 정책은 수정 가능해야 함

* Area: Frontend / Project Settings
* User signal: `프로젝트 설정은 열 수 있지만 이거 관련해서 변경은 불가능하네`
* Interpreted intent: 프로젝트 설정 드로어는 단순 조회 전용이면 의미가 약하므로, 적어도 운영 정책은 이 화면에서 바로 수정하고 저장할 수 있어야 한다.
* Action: 프로젝트 설정 드로어의 배포 정책 섹션을 편집형 폼으로 바꾸고, `PATCH /api/v1/projects/{projectId}/policies` 와 연결해 저장 가능하게 했다. 이름/namespace 같은 식별자는 별도 계약이 없으므로 읽기 전용으로 유지한다.
* References: `frontend/src/App.tsx`, `docs/internal-platform/openapi.yaml`
* Status: applied

### 2026-04-16 - FB-020 - 프로젝트 이름은 영문 slug이며 namespace와 동일해야 함

* Area: Frontend / Backend / Project Creation
* User signal: `프로젝트 이름을 무조건 영어로 가져가고 쿠버네티스 네임스페이스 생성 규칙이랑 동일하게 가져가자`
* Interpreted intent: 프로젝트 식별 체계는 사람이 보기 좋은 별도 이름과 기술적 namespace를 나누지 말고, 영문 소문자 slug 하나로 통일해야 한다.
* Action: 새 프로젝트 생성 입력을 slug 하나만 받도록 바꿨고, frontend는 이 값을 `id` 와 `name` 에 함께 넣는다. backend는 `id=name=namespace` 규칙을 검증하도록 강화했고, OpenAPI/가이드 문서도 함께 갱신했다.
* References: `frontend/src/App.tsx`, `backend/internal/project/service.go`, `docs/internal-platform/openapi.yaml`, `docs/frontend-swagger-guide.md`
* Status: applied

### 2026-04-16 - FB-021 - 새 프로젝트는 프로젝트 목록보다 먼저 보여야 함

* Area: Frontend / Global Navigation
* User signal: `새 프로젝트 생성에 대해서 음 어디로 옮길 수는 없나? 좀 애매한 위치에 있는거 같음 프로젝트 목록이 나오기 전에 있다던가`
* Interpreted intent: 프로젝트 섹션에서는 기존 목록을 보기 전에 `새 프로젝트` 생성 액션이 먼저 눈에 들어와야 탐색과 생성 흐름이 자연스럽다.
* Action: 글로벌 사이드바의 프로젝트 섹션에서 `새 프로젝트` 버튼을 프로젝트 목록 아래가 아니라 상단으로 이동시켰다.
* References: `frontend/src/components/navigation/SidebarNav.tsx`, `frontend/src/components/navigation/SidebarNav.module.css`
* Status: applied

### 2026-04-16 - FB-022 - 남겨 둘 공용 seed 프로젝트는 식별자 전부를 shared로 맞춰야 함

* Area: Backend / Platform Catalog / Seed Manifests
* User signal: `shared로 맞춰서 진행 ㄱ`
* Interpreted intent: 실제로 남겨 둘 공용 프로젝트는 표시 이름만 공용이 아니라, 프로젝트 ID, namespace, 앱 경로, 환경, 로컬 fallback 권한 기준까지 모두 `shared` 축으로 일관되게 맞아야 운영 중 혼선이 없다.
* Action: 실사용 `platform/projects.yaml` seed를 `shared` 기준으로 정리하고, 남아 있던 `apps/project-a/...`, 변경 요청 레코드, dev fallback 기본 권한, 관련 agent 문서를 함께 갱신한다.
* References: `platform/projects.yaml`, `apps/shared/`, `platform/changes/`, `backend/internal/core/config.go`, `docs/agent-code-index.md`
* Status: applied

### 2026-04-16 - FB-023 - OIDC 화면도 기본은 일반 로그인 폼이고 Keycloak은 보조 진입점이어야 함

* Area: Frontend / Login UX
* User signal: `이렇게 말고 그냥 노멀하게 아이디 비번치게 들어가게 해줘 밑에는 keycloak으로 로그인 이렇게 해주고`
* Interpreted intent: Keycloak fallback 계정이 있더라도 화면에 `비상 로그인` 톤을 전면 배치하면 일반 사용자 입장에서 어색하므로, 기본 로그인 폼을 먼저 보여주고 SSO는 보조 옵션으로 내려야 한다.
* Action: OIDC + fallback 로그인 화면을 일반 `아이디/비밀번호` 폼 우선 구조로 재배치하고, 상단에는 설명 문구 대신 `AODS` 타이틀을 크게 배치한 뒤 하단에 `Keycloak으로 로그인` 버튼만 둔다. 입력창에는 placeholder나 초기값을 넣지 않고, 로컬 로그인 기본 자격 증명은 `admin / qwe1356@`로 통일한다. 브라우저/비밀번호 매니저 자동완성으로 값이 다시 들어오지 않도록 로그인 폼에 자동완성 방지 속성도 추가한다.
* References: `frontend/src/App.tsx`
* Status: applied

### 2026-04-17 - FB-024 - 앱 생성은 가짜 연결 저장소 선택이 아니라 GitHub URL과 토큰만으로 끝나야 함

* Area: Frontend / Backend / Application Creation
* User signal: `걍 깃헙 링크랑 깃헙 토큰 주면 그 내부적으로 aolda_deploy.json이라는게 있다는 가정하여 알아서 쭉 연결되는걸 원했음`
* Interpreted intent: 앱 생성 UX는 프로젝트 카탈로그에 미리 저장소를 연결해 두는 방식보다, 사용자가 앱 만들 때 직접 GitHub URL과 토큰만 넣고 끝내는 흐름이어야 한다.
* Action: `새 애플리케이션` 위저드에서 `연결된 GitHub 저장소` 선택 UI를 제거하고 `GitHub URL + 토큰 + 선택적 서비스 ID + 설정 파일 경로` 기반으로 단순화했다. 백엔드는 `aolda_deploy.json`을 읽어 이미지/포트/레플리카/전략을 자동 채우고, GitHub 토큰은 앱 환경변수 Vault 경로와 분리된 별도 경로에 저장하도록 변경했다. 혼선을 줄이기 위해 `shared` seed 프로젝트의 가짜 `repositories` 항목도 제거했다.
* References: `frontend/src/components/ApplicationWizard.tsx`, `frontend/src/App.tsx`, `backend/internal/application/service.go`, `backend/internal/application/poller.go`, `backend/internal/vault/local.go`, `platform/projects.yaml`, `docs/internal-platform/openapi.yaml`
* Status: applied

### 2026-04-17 - FB-025 - GitHub 연결 방법은 별도 설명 없이도 화면 안에서 바로 이해되어야 함

* Area: Frontend / Application Creation
* User signal: `이 레포를 기반으로 연결하고 싶은데 이거 관련해서 뭔가 설명이 있으면 좋겠어 어떤 명령어나 어떤 페이지에서 토큰을 가져와서 어떤 값들을 넣으면 된다 이런식으로`
* Interpreted intent: GitHub 기반 앱 생성은 사용자가 문서나 대화 로그를 다시 찾지 않아도, 위저드 안에서 토큰 발급 위치와 입력값 규칙을 바로 이해할 수 있어야 한다.
* Action: GitHub 등록 단계 안에 `GitHub 연결 가이드` 카드를 추가해 fine-grained PAT 발급 페이지 링크, 필요한 권한, owner/repo 기준 추천값, `aolda_deploy.json` 체크 포인트를 함께 노출했다.
* References: `frontend/src/components/ApplicationWizard.tsx`
* Status: applied

### 2026-04-17 - FB-026 - GitHub 입력칸은 기본값이나 자동완성 값이 미리 들어가 있으면 안 됨

* Area: Frontend / Application Creation
* User signal: `GitHub 저장소 URL 같은 값들의 기본으로 들어가 있는 값들에 대해서도 비워줘 왜 이건 계속 차있는거임?`
* Interpreted intent: GitHub 저장소 URL과 토큰은 민감한 입력이므로, 사용자가 열었을 때 항상 빈 상태여야 하고 브라우저/비밀번호 매니저 자동완성도 최대한 막아야 한다.
* Action: GitHub 입력칸의 예시 URL을 `aods` 기준으로 바꾸고, 실제 입력 필드에는 고유 `name`, `autoComplete=off/new-password`, 자동완성 방지 속성, 숨김 더미 필드를 추가해 브라우저 autofill 개입을 줄였다.
* References: `frontend/src/components/ApplicationWizard.tsx`
* Status: applied

### 2026-04-17 - FB-027 - 설정 파일 포맷도 화면에서 바로 보고 예시를 받을 수 있어야 함

* Area: Frontend / Application Creation
* User signal: `aolda_deploy.json 에 대해서 포멧에 대해 알려주기도 해야하지 않을까? 예시 json 자체를 다운 받을 수 있게 하던가`
* Interpreted intent: GitHub 연결 가이드는 토큰/입력값 설명만으로 끝나면 부족하고, 실제로 필요한 `aolda_deploy.json` 구조까지 바로 보여줘야 한다.
* Action: GitHub 등록 단계 안에 `aolda_deploy.json 형식` 섹션을 추가해 필드 설명, 예시 JSON 미리보기, `예시 JSON 다운로드` 버튼을 넣었다. 예시는 현재 입력한 앱 이름/서비스 ID를 기준으로 동적으로 생성된다.
* References: `frontend/src/components/ApplicationWizard.tsx`
* Status: applied

### 2026-04-17 - FB-028 - 저장소 토큰과 이미지 레지스트리 토큰은 분리해서 받아야 함

* Area: Frontend / Backend / Application Creation / Runtime Deployment
* User signal: `repo token과 registry token을 분리해서 발급 받고 처리하는 방식까지 다 추가해서 적용 ㄱ`
* Interpreted intent: GitHub 저장소 설정 파일을 읽는 권한과 private 컨테이너 이미지를 pull 하는 권한은 보안 모델이 다르므로, 입력 UX와 backend 저장 구조도 분리되어야 한다.
* Action: 앱 생성 API와 위저드에 `registryServer`, `registryUsername`, `registryToken` 필드를 추가했다. backend는 저장소 토큰과 별도로 registry credential을 Vault 경로 `secret/aods/apps/{project}/{app}/registry` 에 저장하고, `dockerconfigjson` 기반 `ExternalSecret` 및 workload `imagePullSecrets`를 함께 생성한다. 이미지 접근 사전 검증도 registry credential을 사용하도록 확장했다.
* References: `frontend/src/components/ApplicationWizard.tsx`, `frontend/src/App.tsx`, `backend/internal/application/service.go`, `backend/internal/application/image_verifier.go`, `backend/internal/application/store_local.go`, `docs/internal-platform/openapi.yaml`
* Status: applied

### 2026-04-17 - FB-029 - 공개 GitHub 저장소는 토큰 없이도 바로 연결되어야 함

* Area: Frontend / Backend / API Contract / Application Creation
* User signal: `GitHub 연동을 하고 싶은뎅`
* Interpreted intent: GitHub 연결의 기본 경로는 public 저장소 기준이어야 하고, 토큰은 private 저장소나 rate-limit 회피가 필요한 경우에만 선택적으로 넣는 구조가 더 자연스럽다.
* Action: 앱 생성 Wizard에서 GitHub 저장소 토큰을 선택 입력으로 바꾸고, 안내 문구를 `public 저장소는 URL만, private 저장소는 토큰 추가` 기준으로 정리했다. `CreateApplicationRequest` OpenAPI 계약도 `repositoryUrl`만 필수로 완화했고, backend 회귀 테스트를 추가해 공개 저장소 descriptor가 무토큰으로도 동작함을 고정했다.
* References: `frontend/src/components/ApplicationWizard.tsx`, `frontend/src/components/ApplicationWizard.test.tsx`, `backend/internal/application/service_test.go`, `docs/internal-platform/openapi.yaml`
* Status: applied

### 2026-04-17 - FB-030 - 저장소 서비스 ID와 레지스트리 토큰 발급 방법은 새 탭 가이드로 바로 열려야 함

* Area: Frontend / Application Creation
* User signal: `레지스트리 토큰 생성이라던지 이런거 관련해서도 바로 링크로 연결되었으면 좋겠네` / `설명을 작성한 페이지를 하나 만들어서 해당 새로운 탭이 뜨면서`
* Interpreted intent: 위저드 안의 짧은 문구만으로는 이해가 부족하므로, 서비스 ID 예시와 GitHub/GHCR 토큰 발급 방법을 새 탭에서 바로 확인할 수 있어야 한다.
* Action: 앱 생성 Wizard의 GitHub 연결 가이드에 `설명 페이지 열기`, `GitHub 토큰 발급`, `GHCR 토큰 발급` 버튼을 추가했다. 단일 서비스 저장소는 이름/서비스 ID 없이도 다음 단계로 넘기도록 프론트 선행 차단을 제거했고, multi-service 저장소에서 backend가 반환하는 `repositoryServiceId` 오류는 한국어 안내 문구로 번역했다. 설명 문서는 정적 페이지 `frontend/public/application-source-guide.html`로 추가했다.
* References: `frontend/src/components/ApplicationWizard.tsx`, `frontend/src/App.tsx`, `frontend/public/application-source-guide.html`, `frontend/src/components/ApplicationWizard.test.tsx`
* Status: applied

### 2026-04-17 - FB-031 - 프로젝트 설정의 연결된 저장소는 실제 연결일 때만 보여야 함

* Area: Frontend / Backend / Project Settings
* User signal: `이거 실제로 연결된 저장소임? 아니면 걍 없애줘 그리고 실제로 있는 저장소만 연결할 수 있도록 처리`
* Interpreted intent: 프로젝트 설정에 seed 데이터나 데모 저장소가 실제 연결 상태처럼 보이면 신뢰를 해치므로, 현재 제품에서 실연동 보장이 안 되는 저장소 표기는 화면에서 제거해야 한다. 저장소 실존 검증은 실제 GitHub 연결 흐름인 애플리케이션 생성 단계에서만 수행되어야 한다.
* Action: 프로젝트 설정 Drawer/탭에서 `연결된 저장소` 섹션과 관련 fetch/state를 제거했다. 기본 서버 fixture와 실행 중 manifest repo에 남아 있던 fake repository seed도 삭제해 `/projects/{id}/repositories` 기본 응답이 빈 상태로 시작되도록 정리했다. 실제 저장소 연결 여부는 여전히 앱 생성 시 `aolda_deploy.json` 원격 조회 성공 여부로 검증된다.
* References: `frontend/src/App.tsx`, `backend/internal/server/server_test.go`, `backend/internal/server/server_git_test.go`, `/Users/ichanju/Desktop/aolda/AODS-manifest/platform/projects.yaml`
* Status: applied

### 2026-04-17 - FB-032 - 플랫폼 형태는 충분히 나왔고 이후는 디테일 정리 중심이어야 함

* Area: Product Direction / Documentation
* User signal: `플랫폼의 형태는 잘 나온거 같고 디테일한 부분들만 처리하면 될 것 같아서 이런 부분들에 대해서는 문서로 남겨줘`
* Interpreted intent: 큰 구조 변경을 계속 제안하기보다, 현재 합의된 플랫폼 형태를 문서로 고정하고 이후 작업은 디테일 polish, 실연동, 테스트, 가이드 정리에 집중해야 한다.
* Action: 현재 합의된 플랫폼 형태, 운영 모델, 비범위 항목, 남은 디테일 backlog 를 `docs/platform-shape-and-detail-backlog.md` 에 정리하고, baseline handoff 문서에서 함께 보도록 연결했다.
* References: `docs/platform-shape-and-detail-backlog.md`, `docs/current-baseline-runbook.md`
* Status: applied

### 2026-04-17 - FB-033 - GitHub와 이미지 가이드는 위저드 안보다 별도 설정 페이지에서 더 자세히 보여야 함

* Area: Frontend / Application Creation / Documentation UX
* User signal: `다 설정페이지로 넘겨서 자세히 작성해주고 아까 이미지 관련된 부분들에 대해서도 작성해주고`
* Interpreted intent: 앱 생성 위저드 안에 긴 설명을 계속 쌓는 대신, 저장소 연결, 이미지 Pull, 모노레포, 태그 운영 규칙은 별도 설정 가이드 페이지로 모으고 자세히 설명해야 한다.
* Action: 위저드 안의 긴 GitHub/JSON 설명 카드를 축약하고, `frontend/public/application-source-guide.html` 을 `AODS 연결 설정 페이지` 형태로 재작성했다. 새 페이지에는 저장소 설정, 레지스트리 설정, 프론트+백엔드 같은 레포 처리, `aolda_deploy.json` 구조, Vault 저장 경로, immutable tag 운영 원칙을 함께 정리했다.
* References: `frontend/src/components/ApplicationWizard.tsx`, `frontend/src/components/ApplicationWizard.test.tsx`, `frontend/public/application-source-guide.html`
* Status: applied

### 2026-04-17 - FB-034 - 생성 전에 descriptor를 읽고 서비스 수를 보여주는 단계가 필요함

* Area: Frontend / Backend / Application Creation
* User signal: `json을 보고 서비스가 몇개 인거 같다 각각의 env를 넣도록 하겠다 하는 단계가 있으면 좋겠음` / `ghcr.io 또는 docker hub 일거라 이거 관련해서 select로 선택 가능하게`
* Interpreted intent: 사용자는 앱 생성 전에 저장소 설정 파일을 실제로 읽어 서비스 구성을 먼저 확인하고 싶어 한다. 또한 레지스트리 주소는 자유 입력보다 `ghcr.io / docker.io / 직접 입력` 같이 선택 가능한 UX가 더 직관적이다.
* Action: 백엔드에 저장소 descriptor preview API를 추가하고, 프론트 위저드에 `설정 파일 확인` 단계를 넣어 서비스 수/이미지/포트/전략을 먼저 보여주도록 바꿨다. 여러 서비스면 여기서 원하는 `serviceId`를 선택할 수 있고, 단일 서비스면 실제 `serviceId`를 자동 저장한다. 레지스트리 주소도 select 기반으로 바꾸고 custom 입력은 별도 필드로 분리했다.
* References: `backend/internal/application/service.go`, `backend/internal/application/http.go`, `backend/internal/server/server.go`, `frontend/src/components/ApplicationWizard.tsx`, `frontend/src/api/client.ts`, `frontend/src/types/api.ts`
* Status: applied

### 2026-04-17 - FB-038 - 앱이 실제로 떠 있는지 믿을 수 있도록 컨테이너 로그를 바로 봐야 함

* Area: Frontend / Backend / Observability
* User signal: `해당 컨테이너에 대한 로그 같은것도 볼 수 있게 만들어줄 수 있나? 뭔가 해당 애플리케이션이 잘 띄워졌는지 이런거 보기가 어려운 것 같아서`
* Interpreted intent: 메트릭이나 Sync 상태만으로는 앱이 실제로 떠 있는지 신뢰하기 어려우므로, 운영 센터에서 최근 pod/container 로그를 직접 확인할 수 있어야 한다.
* Action: backend에 `GET /api/v1/applications/{applicationId}/logs` API를 추가해 최근 pod의 primary container 로그를 읽도록 하고, frontend 관측 탭에 `최근 컨테이너 로그` 카드를 추가해 pod 이름, 컨테이너 이름, Ready 여부, 재시작 횟수, 최근 로그 본문을 함께 보여준다.
* References: `backend/internal/application/service.go`, `backend/internal/application/http.go`, `backend/internal/kubernetes/real.go`, `backend/internal/server/server.go`, `frontend/src/App.tsx`, `frontend/src/api/client.ts`, `frontend/src/types/api.ts`, `docs/internal-platform/openapi.yaml`
* Status: applied

### 2026-04-17 - FB-039 - 관측 탭은 CPU 상세 표보다 pod/container 실시간 로그와 할당 대비 사용량이 더 중요함

* Area: Frontend / Backend / Observability
* User signal: `CPU Usage 상세 데이터 이건 뭐하는거임? 왜 이런 공간이 있는거지?` / `아예 제거 하는게 나은거 같은데 CPU usage 상세 데이터 로그쪽을 아까 내가 말했던 대로 고도화를 해주삼` / `파드 레벨 까지만 볼 수 있게 ... SSE 스트리밍 처리도 되었으면 좋겟고 pod/container는 직접 고를 수 있게` / `각 파드마다 할당되는 리소스에 비해 지금 얼마나 쓰는지를 지표에서 보여줄 수 있게`
* Interpreted intent: 관측 탭은 메트릭 상세 표를 늘어놓는 화면보다, 실제 운영자가 믿을 수 있는 런타임 증거인 `선택형 pod/container 로그` 와 `현재 usage vs request/limit` 를 먼저 보여주는 화면이어야 한다.
* Action: 앱 운영 센터 관측 탭에서 metric 상세 표를 제거하고, backend에 `logs/targets`, `logs/stream` API와 container resource status 계산을 추가했다. frontend는 pod/container selector, SSE 기반 실시간 로그 뷰어, 선택 컨테이너의 CPU/메모리 사용량 대비 request/limit 카드로 재구성했다.
* References: `backend/internal/application/types.go`, `backend/internal/application/service.go`, `backend/internal/application/http.go`, `backend/internal/kubernetes/real.go`, `backend/internal/server/server.go`, `frontend/src/App.tsx`, `frontend/src/api/client.ts`, `frontend/src/types/api.ts`, `docs/internal-platform/openapi.yaml`
* Status: applied

### 2026-04-17 - FB-040 - 리소스 할당은 저장소 작성자가 아니라 운영 admin이 관리해야 함

* Area: Backend / Frontend / Application Operations
* User signal: `해당 리소스를 할당하고 주는건 운영자가 해야할 것 같은데?` / `어드민 권한 가진 사람이 줘야할 것 같음 default는 정해주고`
* Interpreted intent: CPU/메모리 request/limit은 앱 저장소의 descriptor가 아니라 플랫폼 운영 영역에서 관리해야 하며, 새 앱은 서버 기본값으로 시작하고 이후 조정은 project admin만 할 수 있어야 한다.
* Action: application 모델과 manifest 렌더에 `resources.requests/limits`를 추가하고, 새 앱 생성 시 기본 리소스 할당을 자동 적용했다. 운영 센터 `운영 규칙` 탭에는 리소스 할당 카드를 추가했고, `PATCH /api/v1/applications/{applicationId}`에서 `resources` 수정은 project admin만 허용하도록 제한했다.
* References: `backend/internal/application/resources.go`, `backend/internal/application/service.go`, `backend/internal/application/store_local.go`, `backend/internal/application/store_support.go`, `frontend/src/App.tsx`, `frontend/src/types/api.ts`, `docs/internal-platform/openapi.yaml`
* Status: applied

### 2026-04-17 - FB-041 - Istio는 opt-in이고 서비스별 LoadBalancer on/off는 운영자가 직접 제어해야 함

* Area: Backend / Frontend / Network Exposure
* User signal: `istio를 잘 써보기 위해서 적용했던건데 쓸모가 없어져버렸네.. opt-in 모델로 두자` / `사용자가 각 서비스별로 lb을 켜고 끌 수 있게도 해주삼`
* Interpreted intent: Istio를 모든 앱에 강제로 붙이는 기본 모델은 현실 운영 흐름과 맞지 않는다. 기본은 일반 Kubernetes Service 경로로 두고, 필요한 앱만 Istio mesh를 켜며, 외부 노출은 앱 단위로 `LoadBalancer`를 직접 켜고 끄는 모델이 더 맞다.
* Action: application 모델과 metadata에 `meshEnabled`, `loadBalancerEnabled`를 추가했다. backend manifest 렌더는 `meshEnabled=true`일 때만 sidecar inject, VirtualService, DestinationRule, envoy metrics 포트를 생성하고, `loadBalancerEnabled=true`일 때만 Service를 `LoadBalancer` 타입으로 렌더한다. frontend 운영 규칙 탭에는 admin 전용 `Istio mesh 사용`, `LoadBalancer 노출` 스위치를 추가했고, 카나리아 배포에서는 `mesh=true`, `loadBalancer=false` 제약을 서버와 화면에서 함께 안내한다.
* References: `backend/internal/application/service.go`, `backend/internal/application/store_local.go`, `backend/internal/application/store_support.go`, `frontend/src/App.tsx`, `frontend/src/types/api.ts`, `docs/internal-platform/openapi.yaml`
* Status: applied

### 2026-04-18 - FB-042 - LoadBalancer는 비용/외부노출 영향이 큰 작업이므로 단순 스위치보다 요청 의미와 후속 절차를 더 명확히 보여줘야 함

* Area: Frontend / Network Exposure UX
* User signal: `어케 보면 꽤 비용이 큰 작업이라 ... 타입을 노출하고 floating ip가 받아지는 과정까지 뭔가 처리되는게 좀 잘 해줬으면`
* Interpreted intent: 현재 `LoadBalancer` 스위치는 외부 공개가 즉시 완료된 것처럼 오해를 줄 수 있다. 실제로는 `Service type=LoadBalancer` 요청, GitOps 반영, 클러스터 준비, floating IP / 외부 라우팅 연결이 순차적으로 이어지므로 운영자가 저장 전에 의미와 비용을 명확히 확인해야 한다.
* Action: frontend 운영 규칙 탭에서 `LoadBalancer 노출 요청`으로 문구를 조정하고, 활성화 시 확인 모달을 먼저 띄우도록 바꿨다. 모달에는 비용/외부 연결 영향, 후속 floating IP 확인 필요성을 명시했고, 화면 본문에는 `AODS -> Flux -> 클러스터 -> 네트워크` 단계 흐름을 보여주는 안내 카드를 추가했다. 애플리케이션 카드의 `LB 공개` 뱃지는 `LB 요청`으로 바꿔 의미를 더 정확히 맞췄다.
* References: `frontend/src/App.tsx`
* Status: applied

### 2026-04-18 - FB-043 - 트래픽 설정 저장 실패 시 generic invalid body 대신 stale backend 재시작 안내가 보여야 함

* Area: Frontend / Local Runtime UX
* User signal: `트래픽 설정 저장 실패 Request body is invalid. 라는데?`
* Interpreted intent: 로컬에서 프론트와 백엔드 스키마가 어긋난 상태면 사용자는 generic JSON 오류보다 “백엔드를 다시 시작해야 한다”는 직접적인 안내를 받아야 한다.
* Action: frontend `ApiError`에 server `details`를 실어 나르도록 보강하고, 애플리케이션 네트워크 설정 저장 실패 시 `unknown field "meshEnabled"` / `unknown field "loadBalancerEnabled"` 응답을 감지하면 백엔드 재시작 필요 메시지로 번역하도록 수정했다. 로컬 backend도 최신 코드로 재시작해 실제 `PATCH /api/v1/applications/{applicationId}` 저장이 정상 동작하는 것까지 확인했다.
* References: `frontend/src/api/client.ts`, `frontend/src/App.tsx`
* Status: applied

### 2026-04-24 - FB-044 - 모니터링은 보기만 하는 대시보드가 아니라 알림과 진단까지 포함해야 함

* Area: Backend / GitOps / Observability
* User signal: `지금 해당 레포지토리에서 올라온 서비스들에 대해 모니터링을 어케하고 있음?` / `이걸 보고 너가 생각하는 개선안은?` / `ㅇㅇ 그렇게 진행해줘봐 그럼`
* Interpreted intent: 현재 Prometheus metrics 조회와 UI polling만으로는 운영 플랫폼의 모니터링 기준이 부족하다. 앱별 alert rule, 프로젝트 단위 health snapshot, metrics가 비었을 때의 진단 정보가 필요하다.
* Action: 앱 manifest base에 `PrometheusRule` 생성을 추가하고, `GET /api/v1/projects/{projectId}/health`, `GET /api/v1/applications/{applicationId}/metrics/diagnostics` API 계약과 backend 구현, frontend API client 타입을 추가했다. 프로젝트 모니터링 refresh 는 앱별 `metrics/deployments` 루프 대신 project health snapshot 한 번으로 metrics/latest deployment 요약을 받도록 전환했다. 검증 진입점은 `make check-observability` 로 분리해 변경별 커버리지를 명확히 했다.
* References: `backend/internal/application/store_local.go`, `backend/internal/application/service.go`, `backend/internal/application/http.go`, `backend/internal/server/server.go`, `frontend/src/api/client.ts`, `frontend/src/types/api.ts`, `docs/internal-platform/openapi.yaml`
* Status: applied

### 2026-04-24 - FB-045 - `.env` 값도 Vault 기능으로 버전 관리하고 필요하면 복원할 수 있어야 함

* Area: Backend / Vault / Frontend Operations UX
* User signal: `해당 .env에 대해서 vault에 있는 기능을 사용해서 버전관리를 할 수 있게도 해줘봐`
* Interpreted intent: Git에는 ExternalSecret 참조만 남기고 실제 env 값은 Vault KV v2 version history로 추적해야 한다. UI는 값 노출 없이 버전 메타데이터를 보여주고, 특정 버전을 복원할 수 있어야 한다.
* Action: Vault KV v2 metadata 기반 version list / restore API와 운영 화면의 버전 히스토리, 복원 액션을 추가했다. 저장/복원 후 실행 중인 Pod에는 다음 rollout 또는 재시작부터 반영된다는 안내도 함께 노출한다.
* References: `backend/internal/application/service.go`, `backend/internal/vault/real.go`, `backend/internal/vault/local.go`, `frontend/src/App.tsx`, `docs/internal-platform/openapi.yaml`
* Status: applied

### 2026-04-24 - FB-046 - 애플리케이션별 `확인 불가` 상태에는 이유가 바로 보여야 함

* Area: Frontend / Observability UX
* User signal: `애플리케이션별에 대해서 확인 불가인데 왜 확인 불가인지 나왔으면 좋겠는데`
* Interpreted intent: 앱 카드가 `확인 불가` 배지만 보여주면 운영자는 Flux/Kubernetes/health snapshot 중 어디가 원인인지 알 수 없다. 이미 backend health signal 에 이유가 있으면 카드에서 바로 보여줘야 한다.
* Action: project health snapshot 의 sync/health signal message 를 애플리케이션별 카드에 노출했다.
* References: `frontend/src/App.tsx`
* Status: applied

### 2026-04-24 - FB-047 - 컨테이너 로그 실패는 실제 원인과 복구 흐름을 보여줘야 함

* Area: Frontend / Kubernetes Logs UX
* User signal: `Kubernetes에서 컨테이너 로그를 가져오지 못했습니다. 잠시 후 다시 시도하세요. 래 이것도 고쳐줘`
* Interpreted intent: 로그 조회 실패 화면이 generic 재시도 안내만 보여주면 운영자는 Kubernetes API 오류인지, rollout으로 선택 pod/container가 사라진 것인지 구분할 수 없다. 가능한 경우 실제 Kubernetes 원인을 보여주고 stale 로그 대상은 자동으로 다시 찾아야 한다.
* Action: 로그 스트림 API 에러에서 backend `details`를 보존하고, Kubernetes 404/not found 및 stale pod/container 오류를 감지하면 로그 대상을 자동 refresh하도록 처리했다. 그 외 Kubernetes 연동 오류는 실제 원인을 포함해 표시한다.
* References: `frontend/src/api/client.ts`, `frontend/src/App.tsx`
* Status: applied

### 2026-04-24 - FB-048 - 로그 화면에서 운영자가 직접 즉시 갱신할 수 있어야 함

* Area: Frontend / Kubernetes Logs UX
* User signal: `로그에 대해서 바로 업데이트 할 수 있는 버튼도 만들어줘`
* Interpreted intent: 자동 스트리밍만으로는 rollout 직후나 오류 복구 시 운영자가 현재 pod/container 상태를 즉시 다시 확인하기 어렵다. 로그 대상과 최근 로그를 수동으로 재조회하는 버튼이 필요하다.
* Action: Pod / Container 로그 카드에 `로그 업데이트` 버튼을 추가해 현재 로그 target을 다시 읽고 스트림을 새로 연결하도록 했다.
* References: `frontend/src/App.tsx`
* Status: applied

### 2026-04-24 - FB-049 - 클러스터 탭은 platform admin 전용이어야 함

* Area: Frontend / Navigation Authorization
* User signal: `클러스터 탭까지 admin-only로 잠겨줘 그냥`
* Interpreted intent: 클러스터 카탈로그는 일반 프로젝트 운영자에게 글로벌 탭으로 노출하지 않고, platform admin 권한에서만 접근 가능해야 한다.
* Action: 사이드바 글로벌 섹션을 권한별로 필터링해 비관리자에게 `클러스터` 탭을 숨기고, 비관리자 bootstrap에서는 클러스터 카탈로그 API도 호출하지 않도록 정리했다.
* References: `frontend/src/App.tsx`, `frontend/src/app/layout/AppShell.tsx`, `frontend/src/components/navigation/SidebarNav.tsx`
* Status: applied

### 2026-04-24 - FB-050 - platform admin 판정은 그룹이 아니라 role 기반이어야 함

* Area: Auth / Frontend / GitOps
* User signal: `/Ajou_Univ/Aolda_Admin 그룹 말고 걍 role 기반으로 만들어줘`
* Interpreted intent: Keycloak group path를 platform admin 기준으로 쓰면 조직 구조와 제품 권한이 강하게 결합된다. AODS 전역 admin은 `aods:platform:admin` 같은 명시적 role 문자열로 판정해야 한다.
* Action: backend/frontend platform admin authority, dev fallback groups, 배포 manifest, GitOps project adminGroups, 권한 문서와 테스트를 `aods:platform:admin` role 기준으로 정리했다.
* References: `.envrc`, `frontend/.env.local`, `deploy/aods-system/base/backend-deployment.yaml`, `deploy/aods-system/overlays/orbstack/patch-backend-deployment.yaml`, `../AODS-manifest/platform/projects.yaml`, `docs/keycloak-group-auth-model.md`
* Status: applied

### 2026-04-24 - FB-051 - Keycloak 운영자가 따라 할 설정 가이드가 필요함

* Area: Auth / Operations Documentation
* User signal: `운영하는 사람 입장에서는 keycloak에서 어떻게 해줘야하는지에 대해서도 가이드라인 작성 가능할까?`
* Interpreted intent: 기능 구현만으로는 운영 이관이 부족하다. Keycloak 운영자가 AODS client, client role, 사용자/group role 할당, token claim, 검증 및 장애 대응을 그대로 따라 할 수 있는 절차서가 필요하다.
* Action: role 기반 권한 모델에 맞춘 Keycloak 운영자 가이드를 추가하고, 기존 auth 모델 문서와 baseline runbook 에서 해당 문서를 참조하도록 연결했다.
* References: `docs/keycloak-operator-guide.md`, `docs/keycloak-group-auth-model.md`, `docs/current-baseline-runbook.md`
* Status: applied

### 2026-04-24 - FB-052 - Keycloak 사용자 배정은 group으로 관리해야 함

* Area: Auth / Operations Documentation
* User signal: `그룹으로 매핑하는게 나은거 아님? ... 그룹으로 관리하는게 좋은거같은데?`
* Interpreted intent: AODS 권한 판정은 role 기반으로 유지하되, 운영자가 사용자별로 role을 직접 붙이는 방식은 관리 비용이 크다. Keycloak group에 AODS client role을 매핑하고 사용자는 group membership 으로 관리하는 방식이 운영 모델에 맞다.
* Action: 후속 피드백에서 role 직접 할당 기준으로 다시 정리하기로 했으므로 폐기했다.
* References: `docs/keycloak-operator-guide.md`, `docs/keycloak-group-auth-model.md`, `docs/current-baseline-runbook.md`
* Status: superseded

### 2026-04-24 - FB-053 - Keycloak 사용자 배정도 role 기반 직접 할당으로 정리

* Area: Auth / Operations Documentation
* User signal: `아니다 걍 룰 기반으로 ㄱ`
* Interpreted intent: 운영 편의상 group 매핑을 기본으로 두기보다, 현재 AODS 권한 모델을 더 단순하게 유지하기 위해 Keycloak `aods` client role을 사용자에게 직접 할당하는 방식으로 가이드를 고정한다.
* Action: Keycloak 운영 가이드와 auth 모델 문서에서 group 기반 권장 흐름을 제거하고, 사용자별 role 직접 할당 절차를 기준으로 정리했다.
* References: `docs/keycloak-operator-guide.md`, `docs/keycloak-group-auth-model.md`, `docs/current-baseline-runbook.md`
* Status: applied

### 2026-04-24 - FB-054 - 저장소 sync 실패와 polling 주기 저장은 원인과 비용이 보여야 함

* Area: Frontend / Repository Sync UX
* User signal: `저장소 sync 실패 An unexpected integration error occurred. 래 그리고 주기에 대해서 저장하는 것도 너무 느린데 왜 그런겨?`
* Interpreted intent: 저장소 sync 실패가 generic integration error로 보이면 운영자는 Vault, token, GitHub 중 어디가 문제인지 알 수 없다. polling 주기 저장은 GitOps write가 필요한 작업이므로 같은 값을 반복 저장해 불필요하게 느려지면 안 된다.
* Action: 저장소 sync / polling 주기 저장 실패 알림에서 backend `details.error` 원인을 번역해 보여주고, 같은 polling 주기 저장은 버튼 비활성화와 no-op guard로 막았다. 주기 저장이 GitOps repo commit/push 이후 완료된다는 점도 화면에 명시했다.
* References: `frontend/src/App.tsx`
* Status: applied

### 2026-04-24 - FB-055 - LoadBalancer 처리 단계에서 외부 접속 진입점을 바로 열 수 있어야 함

* Area: Frontend / Network Exposure UX
* User signal: `그 lb 관련해서 열어주는 곳에 대해서 외부 인터넷 연결 이라는 곳을 만들고 버튼으로 클릭 시 https://itda.aoldacloud.com/login로 가도록 ㄱ`
* Interpreted intent: LoadBalancer 노출 절차를 확인하는 화면에서 운영자가 외부 인터넷 진입점을 바로 열어 실제 접속 상태를 확인할 수 있어야 한다.
* Action: LoadBalancer 외부 공개 처리 단계 아래에 `외부 인터넷 연결` 액션을 추가하고, 버튼 클릭 시 `https://itda.aoldacloud.com/login` 을 새 탭으로 열도록 연결했다.
* References: `frontend/src/App.tsx`
* Status: applied

### 2026-04-28 - FB-056 - 애플리케이션 생성은 토큰/연결 검증 흐름으로 자연스럽게 따라갈 수 있어야 함

* Area: Frontend / Application Creation DX
* User signal: `토큰을 생성하고, 해당 토큰에 대해서 차례대로 넣으면 바로 연결이 되도록 ... private면 토큰 생성으로 유도 하고 해당 레포 url이랑 토큰 넣으면 제대로 잘 접근되는지 확인하는 곳 넣고 그 다음 이미지 레지스토리지에 대해서도 이 과정을 단계별로 쭉 따라서 이어 갈 수 있게`
* Interpreted intent: 앱 생성 위저드가 긴 설명을 읽고 모든 값을 한 화면에 맞춰 넣는 구조면 private repository + private registry 조합에서 실수가 많다. 저장소 접근, 설정 파일 확인, 이미지 pull credential을 순서대로 확인하는 guided flow가 필요하다.
* Action: ApplicationWizard를 `등록 방식 -> GitHub 연결 -> 설정 파일 확인 -> 이미지 접근 -> 배포 설정 -> 비밀값 -> 최종 확인` 흐름으로 분리했다. public/private 저장소 선택, GitHub 토큰 생성 유도, 저장소 연결 확인 상태, 이미지/레지스트리 인증 단계, registry credential 저장 안내를 추가하고 관련 테스트를 갱신했다.
* References: `frontend/src/components/ApplicationWizard.tsx`, `frontend/src/components/ApplicationWizard.test.tsx`
* Status: applied

### 2026-04-28 - FB-057 - 앱 생성 중 입력된 정보로 이미지 pull 가능 여부를 확인해야 함

* Area: Frontend / Backend / Application Creation DX
* User signal: `그 중간에 이미지도 가져올 수 있는지 아닌지의 여부를 확인하고 싶은데 가능할라나? 입력된 정보 기반으로다가`
* Interpreted intent: 앱 생성 완료 후 backend preflight 에서야 image pull 실패를 알면 사용자는 다시 앞 단계로 돌아가야 한다. 이미지 접근 단계에서 현재 image, registry server, username/token 조합을 즉시 검증해 사용자가 credential 문제를 먼저 고칠 수 있어야 한다.
* Action: 프로젝트 deployer 전용 `POST /api/v1/projects/{projectId}/applications/image-access` API를 추가하고 기존 registry image verifier를 재사용하도록 했다. ApplicationWizard의 `이미지 접근` 단계에는 `이미지 접근 확인` 버튼과 성공/실패/입력 변경 상태를 추가했다.
* References: `backend/internal/application/http.go`, `backend/internal/application/service.go`, `backend/internal/server/server.go`, `frontend/src/components/ApplicationWizard.tsx`, `docs/internal-platform/openapi.yaml`
* Status: applied

### 2026-04-28 - FB-058 - AODS backend secret 저장소도 real Vault를 사용해야 함

* Area: Deployment / Vault / External Secrets
* User signal: `근본 해결은 AODS backend도 real Vault를 쓰게 바꿔줘 local 말고`
* Interpreted intent: ESO가 실제 Vault에서 읽는데 AODS backend가 local vault adapter에 쓰면 앱별 ExternalSecret이 값을 찾지 못한다. 배포 환경에서는 AODS backend와 ESO가 같은 Vault KV v2 mount를 보도록 맞춰야 한다.
* Action: AODS backend 기본 배포 manifest의 Vault 설정을 `local` 에서 real Vault token adapter로 전환하고, token은 `aods-backend-secrets/AODS_VAULT_TOKEN`에서 주입하도록 바꿨다. 로컬 orbstack overlay는 local vault adapter를 유지한다.
* References: `deploy/aods-system/base/backend-deployment.yaml`, `docs/current-baseline-runbook.md`
* Status: applied

## 운영 메모

앞으로 에이전트는 아래 순서를 기본으로 따른다.

1. 사용자가 직접 준 피드백을 이 문서에 먼저 남긴다.
2. 관련 화면/기능을 수정한다.
3. 반영되면 해당 항목의 `Action`과 `Status`를 갱신한다.

즉, 이 문서는 단순 메모가 아니라 다음 작업의 실제 입력값으로 사용한다.
