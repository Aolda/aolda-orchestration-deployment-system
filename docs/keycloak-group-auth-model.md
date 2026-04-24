# Keycloak Role Auth Model

이 문서는 AODS 저장소에서 현재 권장하는 Keycloak 권한 연동 방식을 정리한다.
운영자가 Keycloak Admin Console에서 실제로 따라 할 절차는 `docs/keycloak-operator-guide.md` 를 기준으로 한다.

핵심 원칙은 하나다.

* **전역 admin 판정은 AODS 전용 role 문자열을 사용하고, 프로젝트 운영 권한은 `platform/projects.yaml` 의 문자열과 정확히 맞춘다.**

현재 기준 권장 권한 문자열은 아래와 같다.

* 전역 관리자 role: `aods:platform:admin`
* 프로젝트 관리자 role: `aods:<projectId>:admin`
* 프로젝트 배포 role: `aods:<projectId>:deploy`
* 프로젝트 조회 role: `aods:<projectId>:view`

`<projectId>` 는 AODS `platform/projects.yaml` 의 프로젝트 ID와 맞춘다. 외부 IdP에서 다른 role 이름을 쓰는 경우에는 `AODS_OIDC_ROLE_MAPPINGS` 로 bridge 한다.

## 현재 코드 기준 동작

백엔드는 OIDC access token 에서 아래 claim 을 읽는다.

* top-level `groups`
* `realm_access.roles`
* `resource_access.*.roles`
* `resource_access.*.roles` 를 `clientId:role` 형태로 qualify 한 추가 문자열

이 중 어떤 claim 이든 문자열만 맞으면 권한 판별에 사용된다. `aods` client role `platform:admin` 은 backend에서 `aods:platform:admin` 형태로도 수집되므로, client-qualified role 기준으로 맞추는 것을 권장한다.

## 전역 관리자

전역 관리자 권한은 project catalog 안에 중복 기입하지 않아도 되도록 별도 환경변수로 받는다.

```bash
export AODS_PLATFORM_ADMIN_AUTHORITIES="aods:platform:admin"
export VITE_AODS_PLATFORM_ADMIN_AUTHORITIES="aods:platform:admin"
```

의미:

* backend:
  - 프로젝트 목록에서 모든 프로젝트를 `admin` 으로 볼 수 있다.
  - 프로젝트 생성/삭제, 클러스터 생성 같은 platform-level 작업이 가능하다.
* frontend:
  - 프로젝트 생성, 프로젝트 설정, 클러스터 생성 등 platform admin 전용 UI 를 노출한다.

`aods:platform:admin` 은 AODS 전역 관리자 canonical role 이다.

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
        - aods:aods:view
      deployerGroups:
        - aods:aods:deploy
      adminGroups:
        - aods:aods:admin
        - aods:platform:admin
```

설명:

* `viewerGroups`, `deployerGroups`, `adminGroups` 는 토큰 claim 과 정확히 같아야 한다.
* 실제 운영에서 하나의 role이 조회와 배포를 모두 해도 된다면, 같은 문자열을 `viewerGroups` 와 `deployerGroups` 에 같이 넣어도 된다.
* 전역 admin role은 backend 의 `AODS_PLATFORM_ADMIN_AUTHORITIES` 로도 판별되므로, `adminGroups` 에 중복으로 넣는 것은 선택 사항이다. 다만 catalog 자체를 읽는 사람이 이해하기 쉽게 남겨두는 것은 가능하다.

## Keycloak 쪽에서 필요한 설정

권장 최소 설정:

1. `aods` client 는 OIDC standard flow 사용
2. `aods` client role `platform:admin`, `<projectId>:view`, `<projectId>:deploy`, `<projectId>:admin` 생성
3. 사용자에게 필요한 `aods` client role 직접 할당
4. client role 이 access token 의 `resource_access.aods.roles` 에 들어오도록 mapper 설정

이렇게 하면 backend가 `resource_access.aods.roles` 의 `platform:admin` 을 `aods:platform:admin` 으로 qualify 해서 판별한다.

## Role Mappings 는 언제 쓰나

`AODS_OIDC_ROLE_MAPPINGS` 는 **bridge 용도**다.

즉, 아래 상황에서만 쓴다.

* `platform/projects.yaml` 은 아직 기존 canonical 문자열 `aods:*` 를 유지하고 싶다.
* 하지만 Keycloak 토큰에는 다른 role 이름만 들어온다.

예시:

```bash
export AODS_OIDC_ROLE_MAPPINGS="legacy-aods-admin=aods:platform:admin,legacy-aods-ops=aods:shared:deploy|aods:shared:view"
```

이 방식은 과도기에는 유용하지만, 장기적으로는 Keycloak role 과 `platform/projects.yaml` 을 같은 AODS role 문자열로 정리하는 편이 더 명확하다.

## 앞으로의 작업 원칙

auth 관련 작업에서는 아래 순서를 지킨다.

1. Keycloak 에 AODS 전용 role 이 있는지 먼저 확인한다.
2. 사용자에게 필요한 `aods` client role을 직접 할당한다.
3. 전역 admin 문자열은 `AODS_PLATFORM_ADMIN_AUTHORITIES` 로 관리한다.
4. 프로젝트별 권한은 `platform/projects.yaml` 과 exact match 로 정리한다.
5. `AODS_OIDC_ROLE_MAPPINGS` 는 bridge 가 필요할 때만 추가한다.
6. `/Ajou_Univ/Aolda_Admin` 같은 group path 는 platform admin 판정 기준으로 쓰지 않는다.
