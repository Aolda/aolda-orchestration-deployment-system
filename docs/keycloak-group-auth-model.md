# Keycloak Group Auth Model

이 문서는 AODS 저장소에서 현재 권장하는 Keycloak 권한 연동 방식을 정리한다.

핵심 원칙은 하나다.

* **새 client role 체계를 먼저 설계하지 말고, 이미 운영 중인 Keycloak group hierarchy 를 그대로 재사용한다.**

현재 사용자 피드백 기준으로 우선 고려해야 하는 그룹 구조는 아래와 같다.

* 전역 관리자: `/Ajou_Univ/Aolda_Admin`
* 프로젝트/서비스 운영 그룹: `/Ajou_Univ/Aolda_Member/<project>/ops`

`<project>` 부분은 실제 조직에서 관리하는 서비스 식별자에 맞춘다. 이 값이 꼭 AODS `projectId` 와 1:1 로 같아야 하는 것은 아니지만, 최종적으로는 `platform/projects.yaml` 의 권한 문자열과 **정확히 일치**해야 한다.

## 현재 코드 기준 동작

백엔드는 OIDC access token 에서 아래 claim 을 읽는다.

* top-level `groups`
* `realm_access.roles`
* `resource_access.*.roles`
* `resource_access.*.roles` 를 `clientId:role` 형태로 qualify 한 추가 문자열

이 중 어떤 claim 이든 문자열만 맞으면 권한 판별에 사용된다. 하지만 현재 저장소에서는 **group full path 기반**으로 맞추는 것이 가장 단순하고 운영 의도와도 맞다.

## 전역 관리자

전역 관리자 권한은 project catalog 안에 중복 기입하지 않아도 되도록 별도 환경변수로 받는다.

```bash
export AODS_PLATFORM_ADMIN_AUTHORITIES="/Ajou_Univ/Aolda_Admin,aods:platform:admin"
export VITE_AODS_PLATFORM_ADMIN_AUTHORITIES="/Ajou_Univ/Aolda_Admin,aods:platform:admin"
```

의미:

* backend:
  - 프로젝트 목록에서 모든 프로젝트를 `admin` 으로 볼 수 있다.
  - 프로젝트 생성/삭제, 클러스터 생성 같은 platform-level 작업이 가능하다.
* frontend:
  - 프로젝트 생성, 프로젝트 설정, 클러스터 생성 등 platform admin 전용 UI 를 노출한다.

`aods:platform:admin` 은 기존 테스트/샘플과의 호환을 위한 canonical 문자열이다. 실제 운영 그룹이 `/Ajou_Univ/Aolda_Admin` 이면 두 값을 함께 유지해도 된다.

role 기반 admin 이 꼭 필요하다면, bare role 이름보다 **client-qualified role** 을 우선 권장한다.

예시:

```bash
export AODS_PLATFORM_ADMIN_AUTHORITIES="/Ajou_Univ/Aolda_Admin,account:manage-account,aods:platform:admin"
export VITE_AODS_PLATFORM_ADMIN_AUTHORITIES="/Ajou_Univ/Aolda_Admin,account:manage-account,aods:platform:admin"
```

주의:

* `manage-account` 는 보통 `realm role` 이 아니라 `account` client role 이다.
* `manage-account` 자체를 bare string 으로 잡으면 다른 client role 과 이름이 충돌할 수 있다.
* 더 중요한 문제는, 이 role 이 realm default roles 에 포함돼 있으면 일반 사용자도 모두 admin 이 될 수 있다는 점이다.
* 따라서 screenshot 처럼 default role 로 묶여 있다면 `manage-account` 를 AODS admin 기준으로 쓰는 것은 권장하지 않는다.

## 프로젝트별 접근

프로젝트별 접근은 여전히 `platform/projects.yaml` 의 문자열과 exact match 다.

예시:

```yaml
projects:
  - id: aods
    name: AODS
    namespace: aods
    access:
      viewerGroups:
        - /Ajou_Univ/Aolda_Member/aods/ops
      deployerGroups:
        - /Ajou_Univ/Aolda_Member/aods/ops
      adminGroups:
        - /Ajou_Univ/Aolda_Admin
```

설명:

* `viewerGroups`, `deployerGroups`, `adminGroups` 는 토큰 claim 과 정확히 같아야 한다.
* 실제 운영에서 `ops` 그룹이 조회와 배포를 모두 해도 된다면, 같은 문자열을 `viewerGroups` 와 `deployerGroups` 에 같이 넣어도 된다.
* 글로벌 관리자 그룹은 backend 의 `AODS_PLATFORM_ADMIN_AUTHORITIES` 로도 판별되므로, `adminGroups` 에 중복으로 넣는 것은 선택 사항이다. 다만 catalog 자체를 읽는 사람이 이해하기 쉽게 남겨두는 것은 가능하다.

## Keycloak 쪽에서 필요한 설정

권장 최소 설정:

1. `aods` client 는 OIDC standard flow 사용
2. `Group Membership` mapper 추가
3. claim name 은 `groups`
4. `Full group path` 는 `ON`
5. `Add to access token` 은 `ON`

이렇게 해야 토큰 안에 `/Ajou_Univ/Aolda_Admin`, `/Ajou_Univ/Aolda_Member/aods/ops` 같은 문자열이 그대로 들어온다.

## Role Mappings 는 언제 쓰나

`AODS_OIDC_ROLE_MAPPINGS` 는 **bridge 용도**다.

즉, 아래 상황에서만 쓴다.

* `platform/projects.yaml` 은 아직 기존 canonical 문자열 `aods:*` 를 유지하고 싶다.
* 하지만 Keycloak 토큰에는 group full path 나 다른 role 이름만 들어온다.

예시:

```bash
export AODS_OIDC_ROLE_MAPPINGS="/Ajou_Univ/Aolda_Member/aods/ops=aods:shared:deploy|aods:shared:view"
```

이 방식은 과도기에는 유용하지만, 장기적으로는 `platform/projects.yaml` 을 실제 group path 에 맞춰 정리하는 편이 더 명확하다.

## 앞으로의 작업 원칙

auth 관련 작업에서는 아래 순서를 지킨다.

1. Keycloak 에 이미 있는 group hierarchy 를 먼저 확인한다.
2. 전역 admin 문자열은 `AODS_PLATFORM_ADMIN_AUTHORITIES` 로 관리한다.
3. 프로젝트별 권한은 `platform/projects.yaml` 과 exact match 로 정리한다.
4. `AODS_OIDC_ROLE_MAPPINGS` 는 bridge 가 필요할 때만 추가한다.
5. 새 agent 는 이 문서를 보고 client role 신규 설계를 기본안으로 제안하지 않는다.
