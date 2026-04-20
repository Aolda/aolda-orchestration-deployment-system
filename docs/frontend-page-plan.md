# Frontend Page Plan

이 문서는 AODS 프론트엔드의 페이지 단위 기획안을 **현재 코드 기준 + 외부 제품 레퍼런스 반영 기준**으로 정리한 문서다.

목적은 세 가지다.

1. 현재 사용 중인 화면 구조를 문서로 고정한다.
2. `App.tsx` 에 몰려 있는 라이브 UI와 `pages/` 아래 분리 중인 화면의 역할을 구분한다.
3. 외부 사례를 참고해, 지금 문서가 단순 현황 요약이 아니라 **더 타당한 페이지 구조 제안**이 되게 한다.
4. 다음 프론트 작업에서 "지금 실제로 어떤 페이지가 있고, 무엇이 아직 미완성인지"를 빠르게 판단하게 한다.

주의:

* 이 문서는 디자인 시안 문서가 아니다.
* 이 문서는 `frontend/src/App.tsx`, `frontend/src/pages/`, `frontend/src/components/`, `frontend/src/types/api.ts` 를 기준으로 만든 **코드 우선 기획 문서**다.
* 제품 계약은 여전히 `docs/internal-platform/prd.md`, `docs/acceptance-criteria.md`, `docs/internal-platform/openapi.yaml` 이 우선이다.
* 구조 판단에는 아래 공식 문서의 패턴을 참고했다.
  * GitHub deployment environments
  * Render dashboard / metrics
  * Argo CD application health / history / rollback 성격
  * Qovery observability / deployment statuses
  * Portainer environment catalog / access model

## 0. 외부 레퍼런스에서 확인한 공통 패턴

공식 문서 기준으로 보면 배포 플랫폼 UI는 대체로 아래 패턴을 따른다.

1. **목록 -> 상세 -> 액션** 구조가 분명하다.
2. 앱 또는 서비스 상세 안에서 **상태, 배포, 관측, 이력, 설정**이 함께 다뤄진다.
3. `environment` 는 단순 라벨이 아니라 보호 규칙과 승인 흐름의 단위다.
4. metrics 만으로 운영하지 않고, **logs / events / history** 를 함께 보여준다.
5. role 과 approval rule 이 화면 액션 가능 여부에 직접 연결된다.

AODS 프론트도 이 패턴을 따르는 편이 맞다.
다만 현재 구현은 아직 `metrics + deploy + policy` 쪽이 더 강하고, `logs + change review + role-gated action` 쪽은 약하다.

## 1. 현재 프론트 구조 요약

현재 라이브 프론트는 **라우터 기반 멀티 페이지 앱**이라기보다, 아래 구조에 가깝다.

* 메인 진입점: `frontend/src/App.tsx`
* 현재 실제 사용자 흐름:
  * 로그인 화면
  * 프로젝트 대시보드
  * 애플리케이션 운영 센터 Drawer
  * 새 애플리케이션 생성 Drawer
  * 프로젝트 설정 Drawer
* 분리 중인 화면 컴포넌트:
  * `frontend/src/pages/projects/ProjectsWorkspace.tsx`
  * `frontend/src/pages/changes/ChangesWorkspace.tsx`
  * `frontend/src/pages/clusters/ClustersPage.tsx`
  * `frontend/src/pages/me/MePage.tsx`
* 분리 중인 레이아웃 셸:
  * `frontend/src/app/layout/AppShell.tsx`
  * `frontend/src/app/layout/navigation.ts`

즉, **페이지 설계는 일부 분리돼 있지만 실제 운영 화면은 아직 `App.tsx` 중심 단일 조립 구조**다.

## 2. 전역 IA

### 2.1 현재 라이브 IA

코드상 전역 섹션 목표는 아래 네 개다.

1. `projects`
2. `changes`
3. `clusters`
4. `me`

이 정의는 `frontend/src/app/layout/navigation.ts` 에 있다.

다만 현재 라이브 진입점인 `frontend/src/App.tsx` 는 이 전역 IA를 아직 전면 사용하지 않는다.
실제 사용자 경험은 `프로젝트 중심 대시보드 + 우측 Drawer 운영 패턴` 이다.

### 2.2 권장 목표 IA

외부 사례를 반영하면, 목표 IA는 아래처럼 읽히는 것이 더 자연스럽다.

1. `Projects`
   * 프로젝트 목록
   * 프로젝트 개요
   * 애플리케이션 목록
   * 변경 요청
   * 운영 규칙 / 설정
2. `Changes`
   * 변경 요청 목록
   * 상세 diff
   * 제출 / 승인 / 반영 액션
3. `Clusters`
   * 클러스터 카탈로그
   * 기본 상태 / 용도 / 연결 프로젝트
4. `Me`
   * 사용자 정보
   * 그룹
   * 접근 가능한 프로젝트
   * 권한 범위

즉, 전역 IA는 유지하되 각 섹션이 **왜 top-level 인지**가 화면에서 분명해야 한다.

## 3. 현재 라이브 화면

### 3.1 로그인 화면

목적:

* 플랫폼 진입 전 인증 게이트 역할

현재 동작:

* `admin / admin` 하드코딩 로그인 폼
* 실패 시 인라인 에러 메시지 노출

현재 구현 위치:

* `frontend/src/App.tsx` 의 `LoginForm`

향후 정리 방향:

* OIDC 로그인 화면으로 교체
* 개발용 fallback 과 실제 인증 UI를 명시적으로 분리

### 3.2 프로젝트 대시보드

목적:

* 프로젝트 선택
* 앱 목록 확인
* 프로젝트 단위 운영 현황 파악
* 새 애플리케이션 생성 진입
* 프로젝트 설정/저장소/운영 정책 확인 진입

핵심 레이아웃:

* 상단 헤더
  * 브랜드
  * 현재 사용자
  * 로그아웃
* 마스트헤드
  * "플랫폼 운영 현황" 요약 카피
* 프로젝트 선택 탭
* 앱 카드 그리드
* 우측 Insights / Shortcuts 사이드 패널

핵심 데이터:

* `GET /api/v1/me`
* `GET /api/v1/projects`
* `GET /api/v1/projects/{projectId}/applications`
* `GET /api/v1/projects/{projectId}/environments`
* `GET /api/v1/projects/{projectId}/repositories`
* `GET /api/v1/projects/{projectId}/policies`
* 각 앱별 metrics 집계 기반 project insight

핵심 액션:

* 프로젝트 전환
* 앱 선택
* 새 애플리케이션 생성 Drawer 열기
* 프로젝트 설정 Drawer 열기

현재 구현 위치:

* `frontend/src/App.tsx`

주의:

* 현재 메인 경로는 `ProjectsWorkspace` 기반 탭 모델로 이관되었다.
* 다만 상세 하위 탭의 데이터 흐름은 여전히 `App.tsx` 조립 중심이라 추가 분리가 더 필요하다.

권장 보강:

* 현재 카드형 앱 목록은 유지하되, 프로젝트 상세 구조를 `ProjectsWorkspace` 기준 탭 모델로 이관한다.
* 접근 가능한 프로젝트 목록은 글로벌 좌측 사이드바의 `프로젝트` 메뉴 아래에 노출하고, 선택된 프로젝트의 탭은 메인 상단에서 바로 보이게 한다.
* 우측 인사이트 패널은 좋지만, 장기적으로는 `Overview` 탭의 일부로 흡수하는 편이 정보 구조상 더 자연스럽다.
* 프로젝트 단위 health 는 `앱 수 / sync 상태 / 최근 실패 변경 / 최근 배포`를 함께 보여주는 것이 좋다.

### 3.3 애플리케이션 운영 센터 Drawer

목적:

* 선택한 앱의 운영 상태를 실시간으로 보고
* 배포를 시작하고
* 이력을 확인하고
* 롤백 규칙과 긴급 조치를 수행하는 작업 공간

표현 방식:

* 우측 대형 Drawer
* 제목: `{앱명} 운영 센터`

탭 구조:

1. `상태`
2. `배포`
3. `관측`
4. `이력`
5. `규칙`

현재 구현은 이 구조로 정리되었다.
다만 `logs / diagnostics` 는 아직 실제 API가 아니라 placeholder 진입점 수준이다.

#### 상태

목적:

* 현재 sync 상태와 최근 배포 상태 확인
* 보호 환경 여부와 현재 역할 기준 액션 범위 확인
* rollout 진행 상황을 요약해서 확인

핵심 데이터:

* `GET /api/v1/applications/{applicationId}/sync-status`
* `GET /api/v1/applications/{applicationId}/deployments`

주요 UX 규칙:

* viewer / deployer / admin 역할 차이가 상태 화면에서도 드러나야 함
* 보호 환경이면 direct deploy 대신 change flow 유도 메시지가 먼저 보여야 함
* 최근 실패 배포나 sync 메시지는 현재 상태 요약에 먼저 노출하는 편이 좋다

#### 관측

목적:

* CPU, Memory, Traffic, Latency P95 확인
* metric range 전환
* 상세 시계열 테이블 확인
* 최근 시스템 이벤트 확인
* 장기적으로 logs / diagnostics 로 확장될 관측 진입점 유지

핵심 데이터:

* `GET /api/v1/applications/{applicationId}/metrics`
* `GET /api/v1/applications/{applicationId}/events`

주요 UX 규칙:

* 5분 / 15분 / 1시간 범위 전환
* 카드 클릭으로 상세 metric 선택
* 데이터가 비어 있어도 카드와 상세 영역이 깨지지 않아야 함
* metrics 와 events 는 같은 관측 맥락 안에서 탐색되게 한다

남은 보강:

* 현재는 metrics 와 events 만 있다.
* 배포 플랫폼 운영 화면이라면 장기적으로 `logs` 와 `실패 원인` 진입도 필요하다.
* 목표는 `metrics + events + logs + current rollout state` 를 하나의 관측 축으로 묶는 것이다.

#### 배포 제어

목적:

* 새 이미지 태그 배포 시작
* 현재 배포 진행 상태 확인

핵심 데이터:

* `POST /api/v1/applications/{applicationId}/deployments`
* `GET /api/v1/applications/{applicationId}/sync-status`
* `GET /api/v1/applications/{applicationId}/deployments`

주요 UX 규칙:

* 태그 입력 후 배포 실행
* 진행률은 아래 3단계로 단순화해 보여줌
  * Git Commit Pushed
  * Flux Syncing
  * Canary Monitoring

현재 구현:

* environment 선택과 change review 진입이 들어갔다.
* 보호 환경에서는 direct deploy 보다 change request CTA가 우선 노출된다.

남은 보강:

* 장기적으로는 환경별 승인 규칙과 reviewer 맥락을 더 직접적으로 보여주는 편이 좋다.

#### 배포 이력

목적:

* 최근 배포 레코드 확인

표시 항목:

* 배포 ID
* 이미지 태그
* 상태
* 완료 시각

현재 구현:

* environment, 실패 메시지, 상태 배지가 함께 노출된다.

권장 보강:

* environment
* 배포 전략
* rollout phase
* trigger 주체
* rollback / abort / auto rollback 여부
* 실패 메시지

즉, 단순 테이블을 넘어서 **운영 감사 기록** 역할을 하게 만드는 편이 좋다.

#### 운영 규칙

목적:

* 앱 단위 자동 롤백 정책 설정
* 긴급 수동 조치 수행

핵심 액션:

* 자동 롤백 활성화/비활성화
* 최대 에러율 설정
* 최대 지연시간 P95 설정
* 현재 배포 중단
* 직전 버전으로 롤백

핵심 데이터:

* `GET /api/v1/applications/{applicationId}/rollback-policies`
* `POST /api/v1/applications/{applicationId}/rollback-policies`
* `POST /api/v1/applications/{applicationId}/deployments/{deploymentId}/abort`
* `POST /api/v1/applications/{applicationId}/deployments`

권장 보강:

* app 단위 규칙과 project 단위 규칙의 우선순위를 화면에서 명확히 보여준다.
* `이 값이 프로젝트 정책에 의해 강제되는지, 사용자가 조정 가능한지`가 표시돼야 한다.
* emergency action 은 권한별로 명확히 gating 해야 한다.

### 3.4 새 애플리케이션 생성 Drawer

목적:

* 프로젝트 안에서 신규 애플리케이션을 만들기 위한 단계형 입력 UX 제공

표현 방식:

* 우측 Drawer
* 내부는 Stepper 기반 위저드

스텝 구조:

1. 등록 방식과 기본 정보
2. 배포 설정
3. 환경 변수/비밀값
4. 최종 검토

주요 입력 항목:

* 등록 방식 (`빠른 생성` / `GitHub 저장소 연결`)
* 연결 저장소
* 저장소 내 서비스 ID
* 설정 파일 경로
* 이름
* 설명
* 이미지
* 서비스 포트
* 배포 전략
* 배포 환경
* Secret 목록

보조 UX:

* 저장소 카드형 선택과 연결 정보 요약
* direct 생성 가능한 환경만 선택 가능하게 제한
* 생성 후 GitOps / Vault 반영 경로 미리보기
* `.env` 텍스트 일괄 입력
* 파일 업로드 기반 환경 변수 가져오기
* Secret key/value 행 추가/삭제

현재 구현 위치:

* `frontend/src/components/ApplicationWizard.tsx`
* 호출 위치는 `frontend/src/App.tsx`

현재 구현 메모:

* `빠른 생성` 과 `GitHub 저장소 연결` 을 분리했다.
* repository 기반 등록은 현재 프로젝트에 연결된 저장소만 선택하게 한다.
* `pull_request` 환경은 직접 생성 드로어에서 비활성화하고, 변경 요청 흐름으로 유도한다.
* 검토 단계에서는 애플리케이션 ID, GitOps 경로, Vault 경로를 미리 보여준다.

### 3.5 프로젝트 설정 Drawer

목적:

* 프로젝트 단위 메타데이터와 운영 정책을 조회
* 연결된 저장소/환경/정책을 한 곳에서 확인

섹션 구조:

1. 연결된 저장소
2. 기본 정보
3. 운영 환경
4. 배포 정책

현재 성격:

* 읽기 중심 화면
* 수정 UI는 아직 없다

주요 표시 항목:

* repository 이름, 설명, 접근 방식, config file, URL
* project ID, namespace
* environment 이름, cluster, write mode, default 여부
* min replicas, required probes, prod PR required, auto rollback enabled, allowed env/strategy/cluster

주의:

* `Project Documentation` 버튼은 현재 실제 연결이 없다.
* `writeMode === pull_request` 는 UI에 표시되지만, 제품 의미상 실제 GitHub PR workflow 와 동일하지는 않다.

권장 보강:

* 이 화면은 장기적으로 `Settings` 또는 `Rules` 탭으로 승격하는 편이 더 자연스럽다.
* repository / environment / policy 는 운영상 중요도가 높으므로 shortcut drawer 에만 두기보다 프로젝트 정보 구조 안에 포함시키는 것이 맞다.
* project-level 문서 링크는 placeholder 가 아니라 실제 runbook / repo / dashboard 연결이어야 한다.

## 4. 분리 중인 화면 컴포넌트

### 4.1 Projects Workspace

목적:

* 프로젝트 중심 정보 구조를 `개요 / 애플리케이션 / 변경 요청 / 운영 규칙` 탭으로 나누는 상위 컨테이너

현재 상태:

* 컴포넌트는 존재
* 라이브 메인 경로에 전면 적용되지는 않음

현재 구현 위치:

* `frontend/src/pages/projects/ProjectsWorkspace.tsx`

권장 방향:

* 프로젝트 대시보드의 최종 상위 컨테이너로 이 컴포넌트를 채택한다.
* 프로젝트 선택은 글로벌 사이드바에서 수행하고, `개요 / 애플리케이션 / 변경 요청 / 운영 규칙` 탭은 선택 즉시 메인 상단에 노출한다.
* 현재 `App.tsx` 의 인라인 레이아웃을 이 컴포넌트에 맞춰 옮기고, 탭별 책임을 분리한다.

### 4.2 Changes Workspace

목적:

* 변경 요청 제출, 승인, 반영 작업 공간

현재 상태:

* 독립 화면, 필터, 상세, diff preview, action bar 가 있다.
* `submit / approve / merge` 액션이 UI 안에서 직접 연결돼 있다.
* 다만 프로젝트 전체 목록 API가 없어 현재 세션에서 생성했거나 수동으로 불러온 change 중심으로 동작한다.

현재 구현 위치:

* `frontend/src/pages/changes/ChangesWorkspace.tsx`

권장 목표 구조:

1. 목록
2. 상세
3. diff preview
4. 액션 바

핵심 필드:

* 상태
* 환경
* write mode
* 생성자
* 승인자
* 생성 시각
* diff preview

핵심 액션:

* draft 저장
* submit
* approve
* merge

즉, `Changes` 는 단순 보조 페이지가 아니라 **Phase 3 운영 흐름의 중심 화면**이 되어야 한다.

### 4.3 Clusters Page

목적:

* 클러스터 카탈로그 조회

현재 상태:

* 읽기 전용 테이블형 페이지
* 라이브 전역 내비게이션에 아직 완전히 연결되지 않음

현재 구현 위치:

* `frontend/src/pages/clusters/ClustersPage.tsx`

권장 보강:

* 각 클러스터가 어떤 environment 와 연결되는지 보여준다.
* default 여부 외에 용도, region, 연결 프로젝트 수 같은 운영 맥락이 있으면 더 좋다.

### 4.4 Me Page

목적:

* 현재 사용자 정보, 그룹, 접근 가능한 프로젝트 표시

현재 상태:

* 화면 컴포넌트는 존재
* 라이브 전역 내비게이션에 아직 완전히 연결되지 않음

현재 구현 위치:

* `frontend/src/pages/me/MePage.tsx`

권장 보강:

* 단순 프로필보다 `내가 할 수 있는 작업 범위`를 함께 보여준다.
* viewer / deployer / admin 기준 액션 요약이 있으면 실사용성이 높다.

## 5. 공통 UX 규칙

### 5.1 상태 표현

Flux 상태는 아래 네 개만 쓴다.

* `Unknown`
* `Syncing`
* `Synced`
* `Degraded`

추가 상태 원칙:

* 제품은 Flux 상태를 4개로 단순화하되, 상세 reason 과 message 는 보조 텍스트로 남긴다.
* change status 와 deployment status 는 별개 축으로 보여준다.
* environment protection 상태도 별도 축으로 다루는 편이 좋다.

### 5.2 데이터 갱신

현재 프론트는 polling 기반이다.

* 프로젝트 관련 데이터: 5초 주기 갱신
* 선택 앱 상세 데이터: 5초 주기 갱신

권장 보강:

* polling 은 유지하되, 사용자 액션 직후에는 즉시 refetch 한다.
* `Changes` 목록, `History`, `Events` 는 화면 진입 시 우선 불러오고 필요 시 수동 새로고침도 제공한다.

### 5.3 피드백 방식

주요 액션 결과는 Mantine notification 으로 피드백한다.

예:

* 앱 생성 성공/실패
* 배포 시작 성공/실패
* 롤백 정책 저장 성공/실패
* 배포 중단/롤백 성공/실패

권장 보강:

* notification 외에 화면 내부 상태 변화도 즉시 보여준다.
* destructive action 은 confirm step 또는 명시적 요약이 필요하다.

### 5.4 정보 구조 원칙

현재 프론트의 핵심 UX 원칙은 아래로 읽는 것이 맞다.

* 프로젝트 단위에서 앱 목록을 먼저 본다.
* 앱을 누르면 상세 페이지 전환 대신 운영 Drawer 를 연다.
* 앱 상세 내부에서 상태, 제어, 이력, 규칙을 탭으로 나눈다.

즉, 현재 AODS 프론트는 **페이지 전환형 제품**보다 **운영 콘솔형 제품**에 가깝다.

### 5.5 상태별 UX 원칙

모든 주요 화면은 아래 상태를 명시적으로 가져야 한다.

1. loading
2. empty
3. partial data
4. error
5. forbidden

예:

* 앱 없음
* metrics 없음
* events 없음
* project policy 미조회
* viewer 권한으로 배포/중단 불가

### 5.6 권한별 액션 원칙

화면은 role 기반으로 액션을 분기해야 한다.

* `viewer`
  * 조회만 가능
* `deployer`
  * 앱 생성
  * 배포
  * 긴급 조치
  * rollback policy 수정 범위는 정책에 따라 제한 가능
* `admin`
  * 변경 승인
  * 프로젝트 규칙 변경
  * 더 넓은 운영 권한

현재 코드에서는 이 분리가 기본적으로 반영돼 있다.
다만 project-level 편집 권한과 app-level 운영 권한의 경계는 더 세분화될 수 있다.

## 6. 현재 기획 공백

아래는 지금 문서나 화면이 비어 있거나 미완성인 영역이다.

1. `Changes` 섹션은 독립 화면이 생겼지만, 프로젝트 전체 목록 API가 없어 세션 추적 + 수동 불러오기 중심이다.
2. 프로젝트 설정은 조회 위주이며 편집 UX가 없다.
3. `Project Documentation` 액션은 현재 placeholder 다.
4. 로그인 화면은 실제 인증 제품 UX가 아니라 개발용 임시 폼에 가깝다.
5. 운영 센터의 `logs / diagnostics` 는 아직 실제 API가 아니라 placeholder 다.

## 7. 권장 정리 순서

프론트 페이지 구조를 정리한다면 아래 순서를 권장한다.

1. `Changes` 목록 API를 열어 세션 추적 기반 shell 을 실제 프로젝트 히스토리 화면으로 확장한다.
2. 앱 운영 센터의 `logs / diagnostics` 를 실제 계약으로 연결한다.
3. 프로젝트 설정에 편집 UX와 승인 흐름을 추가한다.
4. 로그인 화면을 실제 인증 제품 UX로 교체한다.
5. 앱 운영 센터는 유지하되, 필요 시 Drawer 와 route 기반 상세 진입을 병행한다.

## 8. 구현 진입점 요약

페이지 기획을 실제 수정으로 옮길 때 우선 보는 파일은 아래 순서가 적절하다.

1. `docs/internal-platform/openapi.yaml`
2. `docs/acceptance-criteria.md`
3. `docs/current-implementation-status.md`
4. `frontend/src/types/api.ts`
5. `frontend/src/api/client.ts`
6. `frontend/src/App.tsx`
7. `frontend/src/pages/`
8. `frontend/src/components/ApplicationWizard.tsx`
9. `frontend/src/app/layout/`

## 9. 참고 레퍼런스

아래 공식 문서의 구조 패턴을 참고했다.

* GitHub Deployment environments: <https://docs.github.com/en/actions/concepts/workflows-and-actions/deployment-environments>
* Render Dashboard: <https://render.com/docs/render-dashboard>
* Render Service Metrics: <https://render.com/docs/service-metrics>
* Qovery Introduction / Observe: <https://www.qovery.com/docs/getting-started/introduction>
* Portainer Environments: <https://docs.portainer.io/admin/environments/environments>
* Argo CD FAQ: <https://argo-cd.readthedocs.io/en/latest/faq/>
