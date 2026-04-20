# Current Platform Shape And Detail Backlog

이 문서는 2026-04-17 기준 AODS의 **현재 합의된 플랫폼 형태**를 고정하고,
이후 작업은 큰 방향 재설계가 아니라 **디테일 완성도 보강**에 집중하기 위한 handoff 문서다.

이 문서는 `기획 초안`이 아니라, 실제 사용자 피드백과 현재 코드 경로를 바탕으로 정리한
`현재 운영 방향 고정본`으로 읽는다.

## 1. 지금 형태에서 잠긴 것

현재 AODS는 아래 성격의 플랫폼으로 보는 것이 맞다.

* AODS는 내부 애플리케이션 배포를 위한 **정책·검증·오케스트레이션 포털**이다.
* GitHub 기본 브랜치는 프로젝트/애플리케이션 메타데이터와 배포 descriptor의 source of truth 다.
* 배포 descriptor 파일명은 `aolda_deploy.json` 이다.
* AODS는 배포 전에 정책 검사, 이미지 검증, 이력 기록, Git desired state 반영을 담당한다.
* 실제 최종 클러스터 반영은 Flux가 담당한다.
* AODS는 Flux를 대체하는 배포 엔진이 아니라, **Flux 앞단의 통제 계층**이다.
* 사용자에게 보이는 런타임 정보는 mock 이 아니라 실제 연동 결과 또는 명시적 empty/unknown 상태여야 한다.

즉, 현재 구조는 아래 한 줄로 요약할 수 있다.

> GitHub와 Registry에서 배포 가능 상태를 확인하고, AODS가 정책과 이력을 관리한 뒤, 최종 반영은 Flux가 수행하는 구조

## 2. 백엔드 운영 모델

현재 합의된 백엔드 운영 흐름은 아래와 같다.

1. GitHub polling
   * webhook 없이 주기 polling 으로 `aolda_deploy.json` 또는 서비스 정의 변경을 감지한다.
   * 운영 전제는 `GitHub poll 기반`이며, webhook 운영은 현재 기본 가정이 아니다.
2. Registry preflight
   * 배포 직전 AODS가 이미지 존재 여부와 접근 가능 여부를 확인한다.
   * repo token 과 registry token 은 분리 발급/관리하는 방향이 현재 기준이다.
3. AODS policy / history / desired state
   * 프로젝트 정책, 배포 전략, 접근 권한, 변경 가드, 이벤트/히스토리/롤백 기준은 AODS가 관리한다.
   * 배포 가능하다고 판단되면 AODS가 Git desired state 를 갱신한다.
4. Flux apply
   * 실제 클러스터 reconcile 과 sync 는 Flux가 담당한다.
   * UI는 Flux 상태를 `Unknown`, `Syncing`, `Synced`, `Degraded` 네 개로 단순 노출한다.

### mutable tag 에 대한 현재 판단

현재 구조는 `tag 변경 기반 운영`에는 잘 맞지만, `same tag overwrite` 운영에는 약하다.

이유:

* 현재 자동 감지는 descriptor 안의 `imageTag` 나 서비스 정의 변경을 기준으로 한다.
* 같은 tag 를 registry 에 다시 올려도 Git descriptor 문자열이 안 바뀌면 AODS는 새 배포로 보지 않는다.
* 따라서 현재 권장 운영은 `immutable tag` 기반이다.

권장 예시:

* `v2026.04.17-001`
* Git commit SHA
* GitHub Actions run number 기반 tag

비권장 예시:

* `latest` 같은 mutable tag 를 계속 덮어쓰기 하는 방식

## 3. 인증/권한 방향

현재 인증/권한 모델은 아래 방향으로 고정한다.

* 인증은 Keycloak OIDC 연동을 기본으로 한다.
* 권한은 새 role 체계를 억지로 설계하기보다, 이미 운영 중인 Keycloak group hierarchy 를 최대한 재사용한다.
* 전역 admin 은 platform-level authority 로 처리하고, 프로젝트별 접근은 `platform/projects.yaml` 의 exact match 문자열 기준으로 판별한다.
* Keycloak 장애나 로컬 QA 상황에서는 dev fallback 로그인 경로를 유지할 수 있다. 다만 운영 기본 경로는 OIDC 다.

상세 기준은 아래 문서를 우선 참고한다.

* `docs/keycloak-group-auth-model.md`

## 4. 프론트엔드 UX 방향

현재 사용자 피드백 기준으로 잠긴 UX 방향은 아래와 같다.

* 프로젝트 첫 화면은 장황한 개요보다 **애플리케이션 중심**이어야 한다.
* 프로젝트 상세는 `애플리케이션 / 모니터링 / 운영 규칙` 중심 구조가 맞다.
* 운영 규칙은 라벨만 던지지 말고, tooltip/help 로 의미를 바로 설명해야 한다.
* 서비스 노출은 기본적으로 내부 Service 기준으로 두고, 필요할 때만 앱 단위로 `LoadBalancer` 노출을 켜고 끄는 모델이 맞다.
* Istio는 기본 강제가 아니라 opt-in 고급 기능으로 두는 편이 현실 운영과 맞다.
* 변경 요청 탭은 현재 우선순위가 아니므로 전면 노출하지 않는다.
* 한국어 사용자 기준으로, 영문 중심 라벨보다는 한국어 중심의 운영 언어를 쓴다.
* 입력 폼에는 임의의 기본값이나 시크릿 placeholder 를 미리 채워두지 않는다.
* 실제로 연결되지 않은 저장소/지표/상태를 연결된 것처럼 보이게 하는 UI는 두지 않는다.

즉, 현재 UX의 핵심은 아래와 같다.

> 운영자가 바로 쓰는 흐름을 우선하고, 불필요한 추상 개요보다 실제 액션과 실제 상태를 더 잘 보이게 만든다.

## 5. 현재 의도적으로 안 하는 것

아래 항목은 지금 단계에서 기본 전제로 두지 않는다.

* GitHub webhook 운영
* 플랫폼 내부 PR 승인 워크플로우 본격 도입
* mutable tag overwrite 기반 자동 감지
* 사용자에게 보이는 mock runtime 데이터
* Flux를 대체하는 별도 apply 엔진 설계
* Keycloak 안의 기존 그룹 구조를 무시한 신규 권한 체계 강제

## 6. 이제 남은 일은 디테일 완성도다

현재부터의 작업은 `플랫폼 형태를 다시 바꾸는 일`보다 아래 디테일 항목을 정리하는 일에 가깝다.

### Backend / Integration

* GitHub polling 간격, 오류 메시지, descriptor 파싱 실패 케이스를 더 명확히 다듬기
* `aolda_deploy.json` schema validation 과 에러 리포팅 보강
* registry token 분리 발급 기준과 preflight 오류 분류를 더 명확히 정리
* deployment history / rollback / event 흐름의 회귀 테스트 강화
* 실제 GitHub + Registry + Flux 연동 시나리오의 운영 가이드 보강
* app 단위 `meshEnabled` / `loadBalancerEnabled` 정책과 floating IP 운영 가이드를 문서로 더 분리 정리

### Frontend / UX / DX

* 새 애플리케이션 생성 흐름에서 GitHub 저장소 연결 가이드를 더 쉽게 제공
* `aolda_deploy.json` 예시 다운로드와 설명 페이지 연결
* 애플리케이션 카드와 모니터링 지표를 더 직관적으로 정리
* 운영 규칙 탭의 `Istio mesh` / `LoadBalancer 노출` 상태를 앱 카드와 운영 센터에서 더 일관되게 보이게 다듬기
* 실연동이 안 된 상태와 권한 부족 상태를 더 명확한 문구/empty state 로 구분
* 운영 규칙, 토큰, 저장소, 레지스트리 개념을 화면 안에서 더 쉽게 이해하게 만들기

### Quality / Documentation

* 핵심 사용자 여정 기준 프론트 회귀 테스트 확대
* poller / auth / drawer / descriptor parsing / deployment preflight 테스트 보강
* 현재 구조를 흔들지 않도록 handoff 문서와 로그 문서를 지속 갱신

## 7. 후속 작업 원칙

앞으로 후속 작업은 아래 원칙으로 진행한다.

* 큰 구조를 다시 뒤엎는 제안보다, 현재 구조의 완성도와 운영 가능성을 높이는 쪽을 우선한다.
* GitHub, Registry, Flux, Keycloak 중 일부가 아직 완전히 붙지 않았더라도 mock 으로 덮지 않는다.
* 실제 연동이 없으면 그 사실을 UI와 API에서 정직하게 드러낸다.
* 사용자가 플랫폼의 형태를 다시 바꾸자고 명시적으로 요청하지 않는 한, 이 문서의 방향을 유지한다.

## 8. 함께 봐야 하는 문서

이 문서와 함께 아래 문서를 본다.

* `docs/current-implementation-status.md`
* `docs/current-baseline-runbook.md`
* `docs/keycloak-group-auth-model.md`
* `docs/user-feedback-log.md`
* `docs/mistake-log.md`
