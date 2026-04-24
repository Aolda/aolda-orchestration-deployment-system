# Keycloak Operator Guide for AODS

이 문서는 운영자가 Keycloak에서 AODS 권한을 설정할 때 따라야 하는 절차를 정리한다.

현재 AODS 기준은 **group path 기반 admin 판정이 아니라 role 기반 admin 판정**이다.

## 목표 상태

운영자는 Keycloak에서 AODS 전용 role을 만들고, 사용자에게 필요한 role을 직접 할당한다.
AODS는 group 이름이 아니라 access token 안의 role 문자열을 보고 권한을 판별한다.

권장 role 문자열은 아래와 같다.

| 용도 | Keycloak `aods` client role | AODS가 보는 권한 문자열 |
| --- | --- | --- |
| 전역 관리자 | `platform:admin` | `aods:platform:admin` |
| 프로젝트 조회 | `<projectId>:view` | `aods:<projectId>:view` |
| 프로젝트 배포 | `<projectId>:deploy` | `aods:<projectId>:deploy` |
| 프로젝트 관리자 | `<projectId>:admin` | `aods:<projectId>:admin` |

예를 들어 `shared` 프로젝트는 아래 role을 쓴다.

```text
aods client role: platform:admin
aods client role: shared:view
aods client role: shared:deploy
aods client role: shared:admin
```

Keycloak access token의 `resource_access.aods.roles` 에 `platform:admin` 이 들어오면, AODS backend는 이를 `aods:platform:admin` 으로 qualify 해서 권한 판정에 사용한다.

## 1. AODS Client 확인

Keycloak realm에서 `aods` client를 확인한다.

권장 설정:

| 항목 | 값 |
| --- | --- |
| Client ID | `aods` |
| Client authentication | Off |
| Standard flow | On |
| Implicit flow | Off |
| Direct access grants | 운영 정책에 맞게 선택 |
| Valid redirect URIs | 실제 프론트 origin 기준으로 등록 |
| Web origins | 실제 프론트 origin 기준으로 등록 |

로컬 개발 예시:

```text
Valid redirect URIs:
  http://localhost:5173/*
  http://127.0.0.1:5173/*

Web origins:
  http://localhost:5173
  http://127.0.0.1:5173
```

프론트가 `http://127.0.0.1:5175` 처럼 다른 origin에서 뜨면 Keycloak redirect URI도 같은 origin으로 맞춰야 한다.

## 2. Client Role 생성

Keycloak Admin Console에서 `Clients -> aods -> Roles` 로 이동해 아래 role을 만든다.

필수 role:

```text
platform:admin
```

프로젝트별 role:

```text
shared:view
shared:deploy
shared:admin
```

새 프로젝트를 추가하면 같은 규칙으로 role을 추가한다.

```text
<projectId>:view
<projectId>:deploy
<projectId>:admin
```

주의:

* `account:manage-account` 를 AODS admin 기준으로 쓰지 않는다.
* `/Ajou_Univ/Aolda_Admin` 같은 group full path 를 AODS platform admin 기준으로 쓰지 않는다.
* AODS 권한 판정 기준은 token에 들어온 role 문자열이다.

## 3. 사용자에게 Role 할당

사용자에게 `aods` client role을 직접 할당한다.

Keycloak Admin Console 기준 절차:

1. `Users` 로 이동한다.
2. 권한을 줄 사용자를 선택한다.
3. `Role mapping` 탭으로 이동한다.
4. `Assign role` 을 누른다.
5. `Filter by clients` 또는 client role 필터를 선택한다.
6. `aods` client의 role을 선택한다.
7. `Assign` 을 눌러 저장한다.

Platform admin 사용자:

```text
aods client role: platform:admin
```

Shared 프로젝트 배포 운영자:

```text
aods client role: shared:view
aods client role: shared:deploy
```

Shared 프로젝트 관리자:

```text
aods client role: shared:view
aods client role: shared:deploy
aods client role: shared:admin
```

역할별 요약:

| 사용자 유형 | 할당할 `aods` client role |
| --- | --- |
| 플랫폼 관리자 | `platform:admin` |
| shared 조회자 | `shared:view` |
| shared 배포 운영자 | `shared:view`, `shared:deploy` |
| shared 프로젝트 관리자 | `shared:view`, `shared:deploy`, `shared:admin` |

Platform admin은 전역 override로 모든 프로젝트를 `admin`처럼 볼 수 있으므로, 일반 프로젝트 운영자에게 `platform:admin`을 주면 안 된다.

## 4. Token Claim 확인

로그인 후 access token을 디코딩해서 아래 claim이 들어오는지 확인한다.

```json
{
  "resource_access": {
    "aods": {
      "roles": [
        "platform:admin",
        "shared:deploy"
      ]
    }
  }
}
```

AODS backend는 위 값을 아래처럼 확장해서 내부 권한 목록에 넣는다.

```text
platform:admin
aods:platform:admin
shared:deploy
aods:shared:deploy
```

따라서 AODS 설정과 GitOps catalog에는 `aods:` prefix가 붙은 canonical 문자열을 쓰는 것이 권장된다.

토큰에 role이 없다면 Keycloak에서 client role이 access token에 포함되도록 client scope 또는 protocol mapper 설정을 확인한다. Keycloak 버전에 따라 화면명은 다를 수 있지만, 핵심은 `resource_access.aods.roles` 가 access token에 들어와야 한다는 점이다.

## 5. AODS 환경 변수

Backend:

```bash
export AODS_AUTH_MODE=oidc
export AODS_OIDC_ISSUER_URL="https://sso.aoldacloud.com/realms/<realm>"
export AODS_OIDC_AUDIENCE=""
export AODS_OIDC_GROUPS_CLAIM="groups"
export AODS_PLATFORM_ADMIN_AUTHORITIES="aods:platform:admin"
export AODS_ALLOW_DEV_FALLBACK=false
```

Frontend:

```bash
export VITE_AODS_AUTH_MODE=oidc
export VITE_AODS_OIDC_ISSUER_URL="https://sso.aoldacloud.com/realms/<realm>"
export VITE_AODS_OIDC_CLIENT_ID="aods"
export VITE_AODS_OIDC_REDIRECT_URI="https://<aods-frontend-host>"
export VITE_AODS_OIDC_SCOPE="openid profile email"
export VITE_AODS_PLATFORM_ADMIN_AUTHORITIES="aods:platform:admin"
```

`VITE_AODS_PLATFORM_ADMIN_AUTHORITIES` 는 Vite build/start 시점에 읽힌다. 값을 바꿨으면 프론트 dev server 또는 배포 빌드를 다시 실행해야 한다.

## 6. GitOps Project Catalog 확인

프로젝트별 권한은 GitOps repo의 `platform/projects.yaml` 과 exact match 되어야 한다.

예시:

```yaml
projects:
  - id: shared
    name: shared
    namespace: shared
    access:
      viewerGroups:
        - aods:shared:view
      deployerGroups:
        - aods:shared:deploy
      adminGroups:
        - aods:shared:admin
        - aods:platform:admin
```

운영자가 해야 할 일:

1. Keycloak에 project role을 만든다.
2. 사용자에게 필요한 role을 직접 할당한다.
3. `platform/projects.yaml` 에 같은 canonical 문자열을 넣는다.
4. GitOps repo에 commit/push 한다.
5. AODS에서 `/api/v1/projects` 또는 화면의 `내 정보` 탭으로 role 해석 결과를 확인한다.

## 7. 검증 방법

Backend에서 현재 사용자가 어떻게 보이는지 확인한다.

```bash
curl -fsS "$AODS_API_BASE_URL/api/v1/me"
```

기대 예시:

```json
{
  "id": "user-id",
  "username": "operator",
  "groups": [
    "aods:platform:admin",
    "aods:shared:deploy"
  ]
}
```

프로젝트 권한 해석을 확인한다.

```bash
curl -fsS "$AODS_API_BASE_URL/api/v1/projects"
```

platform admin이면 모든 프로젝트가 `role: "admin"` 으로 보여야 한다. 일반 shared deployer이면 `shared` 프로젝트가 `role: "deployer"` 로 보여야 한다.

Frontend에서 확인할 항목:

* `aods:platform:admin` 사용자는 `클러스터` 탭이 보인다.
* platform admin이 아닌 사용자는 `클러스터` 탭이 보이지 않는다.
* `viewer` 는 생성/저장/배포 액션이 막힌다.
* `deployer` 는 배포 운영과 환경 변수 저장이 가능하다.
* `admin` 은 프로젝트 운영 규칙과 admin-only 앱 설정을 조정할 수 있다.

## 8. 장애 대응 체크리스트

### 클러스터 탭이 안 보임

확인 순서:

1. access token의 `resource_access.aods.roles` 에 `platform:admin` 이 있는지 확인한다.
2. `/api/v1/me` 응답 `groups` 에 `aods:platform:admin` 이 있는지 확인한다.
3. backend `AODS_PLATFORM_ADMIN_AUTHORITIES` 가 `aods:platform:admin` 인지 확인한다.
4. frontend `VITE_AODS_PLATFORM_ADMIN_AUTHORITIES` 가 `aods:platform:admin` 인지 확인한다.
5. 프론트를 재시작하거나 새 빌드로 배포한다.

### 프로젝트가 안 보임

확인 순서:

1. `/api/v1/me` 에 프로젝트 role이 들어오는지 확인한다.
2. `platform/projects.yaml` 의 `viewerGroups`, `deployerGroups`, `adminGroups` 와 문자열이 정확히 같은지 확인한다.
3. GitOps repo가 최신 commit으로 push 되었는지 확인한다.
4. AODS backend가 Git source를 최신으로 읽고 있는지 확인한다.

### Keycloak에는 role이 있는데 token에 없음

확인 순서:

1. role이 realm role인지 client role인지 확인한다.
2. client role이면 `aods` client 아래 role인지 확인한다.
3. role이 사용자에게 직접 할당됐는지 확인한다.
4. access token에 client role이 포함되도록 client scope 또는 mapper가 연결되어 있는지 확인한다.
5. 사용자가 다시 로그인해 새 access token을 받았는지 확인한다.

### 잘못된 admin 승격 방지

아래 값은 AODS platform admin 기준으로 쓰지 않는다.

```text
/Ajou_Univ/Aolda_Admin
manage-account
account:manage-account
realm-management:*
```
