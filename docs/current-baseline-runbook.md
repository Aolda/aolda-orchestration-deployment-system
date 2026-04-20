# Codex Current Baseline Runbook

이 문서는 새 Codex 세션이 AODS의 현재 구현 기준선을 바로 이어받기 위한 handoff 파일이다.

숨겨진 세션 메모리를 기대하지 말고, 이 파일과 계약 문서를 기준으로 이어서 작업한다.

## 1. Current State

현재 기준:

* 브랜치: `main`
* 시작 계약 문서는 여전히 Phase 1 문서군이다.
* 하지만 baseline 판단 기준은 **`docs/current-implementation-status.md` 에 정리된 current implementation baseline** 이다.
* 다만 현재 코드베이스는 Phase 2, 3, 4의 일부 기능을 이미 포함한다.
* source of truth: **GitHub 기본 브랜치**
* 프로젝트 목록: `platform/projects.yaml`
* 앱 ID 규칙: `{projectId}__{appName}`
* Git write 모델: Phase 1은 `direct push`
* Secret 처리: Vault KV v2, `staging -> git commit -> final -> cleanup`
* 개발 실연동 순서: `GitHub 먼저`, `Vault는 local adapter 유지 후 후속 연결`

현재 구현 범위를 코드 기준으로 확인하려면 아래 문서를 먼저 본다.

* `docs/current-implementation-status.md`
* `docs/platform-shape-and-detail-backlog.md`

## 2. Read Order

반드시 아래 순서로 읽는다.

1. `AGENTS.md`
2. `CLAUDE.md`
3. `.agents/workflows/harness-engineering.md`
4. `docs/internal-platform/prd.md`
5. `docs/domain-rules.md`
6. `docs/phase1-decisions.md`
7. `docs/future-phases-roadmap.md`
8. `docs/internal-platform/openapi.yaml`
9. `docs/acceptance-criteria.md`

## 3. What Has Already Been Decided

이미 잠긴 결정:

* `GET /api/v1/projects` 는 Phase 1에 포함된다.
* 권한 최소 모델은 `viewer`, `deployer`, `admin` 이다.
* Flux UI 상태는 `Unknown`, `Syncing`, `Synced`, `Degraded` 네 개만 쓴다.
* Phase 2는 canary / promote / abort / rollout visibility 다.
* Phase 3은 PR mode / environments / audit / approval 다.
* Phase 4는 policy / auto rollback / cluster scale 다.

이 결정은 문서로 고정돼 있다. 추측으로 바꾸지 않는다.

주의:

* 이 runbook 은 Phase 1 문서군을 최소 계약으로 삼지만, 실제 작업 우선순위는 current implementation baseline 회귀 방지 기준으로 잡아야 한다.
* 현재 레포를 `Phase 1만 구현된 상태`라고 가정하면 실제 코드 상태를 과소평가하게 된다.
* later-phase 진행상황 판단은 반드시 `docs/current-implementation-status.md` 와 실제 코드 기준으로 확인한다.
* 제품 방향을 다시 재해석하기 전에 `docs/platform-shape-and-detail-backlog.md` 로 현재 잠긴 플랫폼 형태와 남은 디테일 범위를 먼저 확인한다.

## 4. Codex Runtime Settings

권장 실행 설정:

* Sandbox: `workspace-write`
* Approval policy: `on-failure`
* Network access: `enabled`
* Plan mode: `off`

이유:

* 코드 수정과 로컬 테스트는 막지 않는다.
* 진짜로 막히는 외부 연동에서만 승인 또는 개입을 요청한다.

## 5. Execution Prompt

새 Codex 세션에서 아래 프롬프트를 그대로 사용해도 된다.

```text
AODS current implementation baseline을 사용자 개입 최소화로 유지·강화해라.

반드시 먼저 아래 문서를 이 순서로 읽어라.
1. AGENTS.md
2. CLAUDE.md
3. .agents/workflows/harness-engineering.md
4. docs/internal-platform/prd.md
5. docs/domain-rules.md
6. docs/phase1-decisions.md
7. docs/future-phases-roadmap.md
8. docs/internal-platform/openapi.yaml
9. docs/acceptance-criteria.md

작업 목표:
- Phase 1 최소 계약을 깨지 않는다.
- 현재 코드와 UI에 이미 노출된 later-phase 기능을 regression 없이 유지한다.
- harness engineering은 orchestration, verification, QA 보조 용도로만 사용한다.
- product runtime이 harness-only env나 local file에 직접 의존하지 않게 한다.
- 사용자가 명시적으로 요청하지 않는 한 완전히 새로운 future-scope 기능은 임의로 열지 않는다.

작업 순서:
1. docs/current-implementation-status.md 와 활성 코드 경로를 기준으로 현재 baseline 을 파악한다.
2. openapi 계약과 acceptance criteria 최소선을 확인한다.
3. 이미 노출된 API/UI/실연동 경로의 regression 을 먼저 막는다.
4. 필요한 backend/frontend 수정 후 make check 와 실환경 검증을 수행한다.
5. handoff 문서와 현재 구현 상태 문서를 함께 갱신한다.

운영 규칙:
- GitHub, Vault, Kubernetes, Prometheus credential이 없으면 interface 와 unavailable/empty-state 처리까지는 먼저 구현하되, fabricated runtime data 는 만들지 않는다.
- 실제 외부 연동이 없더라도 작업을 멈추지 말고, `Unknown`, 명시적 오류, empty response 로 현재 한계를 정직하게 드러낸다.
- 이미 코드에 있는 later-phase 기능을 `아직 구현 대상 아님` 이라고 오판해서 제거하거나 숨기지 않는다.
- 문서 계약과 충돌하거나 destructive action이 필요하거나 실제 credential이 반드시 필요한 마지막 연결 단계가 아니면 사용자에게 질문하지 말고 진행한다.
- 각 큰 단계 후 make check를 실행하고 실패 원인을 수정한다.
- openapi 계약과 acceptance criteria를 구현 기준으로 삼는다.
- Secret 평문을 Git에 저장하지 않는다.
- Go backend는 net/http만 사용한다.
- frontend는 Mantine + CSS Modules만 사용한다.

중간 보고는 짧게 하고, 구현과 검증을 우선하라.
```

## 6. First Commands

세션 시작 후 권장 첫 명령:

```bash
make install-self-hosted-kubeconfig
bash scripts/doctor.sh
make check
```

환경이 덜 깔려 있더라도, missing integration 때문에 구현을 멈추지 않는다.

### Recent QA Notes

2026-04-17 기준 로컬 QA 에서 확인된 중요한 점:

* `frontend-run` 은 `5173` 이 이미 점유돼 있으면 `5174` 같은 fallback port 로 뜰 수 있다.
* backend 는 이제 `AODS_ALLOW_DEV_FALLBACK=true` 일 때 loopback origin(`localhost`, `127.0.0.1`) 을 요청 origin 기준으로 허용한다.
* 따라서 dev QA 에서는 `5173` 고정 여부보다 실제 페이지 origin 과 bootstrap API 성공 여부를 먼저 확인한다.

이번 회귀 수정 검증에 사용한 명령:

```bash
/bin/zsh -lc 'source .envrc && export AODS_ADDR=:28081 && cd backend && go run cmd/server/main.go'
/bin/zsh -lc 'cd frontend && export VITE_API_BASE_URL=http://localhost:28081 && npm run dev -- --port 5175 --strictPort'
```

검증 포인트:

* `curl -H 'Origin: http://localhost:5174' http://127.0.0.1:28081/api/v1/me`
* `curl -H 'Origin: http://localhost:5174' http://127.0.0.1:28081/api/v1/projects`
* 브라우저에서 emergency login 후 `/api/v1/me`, `/api/v1/projects`, `/api/v1/clusters` 가 모두 `200` 인지 확인
* 허용되지 않은 origin 에는 `Access-Control-Allow-Origin` 이 붙지 않는지 확인

관련 코드:

* [backend/internal/core/http.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/core/http.go:42)
* [backend/internal/core/http_test.go](/Users/ichanju/Desktop/aolda/AODS/backend/internal/core/http_test.go:9)

### 6.1 GitHub First Dev Env

개발 환경에서 GitHub real adapter 를 먼저 붙일 때는 아래 env 를 사용한다.

```bash
export AODS_GIT_MODE=git
export AODS_GIT_REPO_DIR=/tmp/aods-managed-gitops
export AODS_GIT_SYNC_TTL=3s
export AODS_GIT_BRANCH=main
export AODS_GIT_AUTHOR_NAME="AODS Bot"
export AODS_GIT_AUTHOR_EMAIL="aods-bot@local"
export AODS_GITHUB_USERNAME="your-github-username"
export AODS_GITHUB_TOKEN="github_pat_xxx"
export AODS_GIT_REMOTE="https://${AODS_GITHUB_USERNAME}:${AODS_GITHUB_TOKEN}@github.com/Aolda/aods-manifest.git"
export AODS_IMAGE_CHECK_MODE=anonymous
export AODS_IMAGE_CHECK_TIMEOUT="5s"
```

Vault는 GitHub-first 초기 단계에서는 local adapter 를 유지해도 된다.
다만 real Vault 검증 단계에서는 아래 env 를 추가한다.

```bash
export AODS_VAULT_MODE=token
export AODS_VAULT_ADDR="http://127.0.0.1:18200"
export AODS_VAULT_TOKEN="root"
export AODS_VAULT_NAMESPACE=""
export AODS_VAULT_REQUEST_TIMEOUT="5s"
```

주의:

* local backend 가 in-cluster Vault service 를 직접 볼 수 없다면 `kubectl port-forward -n vault svc/vault 18200:8200` 같은 터널이 필요하다.
* 현재 self-hosted dev cluster 의 External Secrets Operator 는 `external-secrets.io/v1` 만 served 이므로, generated `ExternalSecret` manifest 도 `v1` 여야 한다.

주의:

* `git` 모드 startup preflight 는 `platform/projects.yaml` 이 target GitOps repo 에 없으면 즉시 실패한다.
* credential helper 를 쓰지 않는다면 `AODS_GIT_REMOTE` 에 tokenized HTTPS remote 를 넣어야 한다.
* `AODS_IMAGE_CHECK_MODE=anonymous` 이면 create/redeploy/change 생성 전에 레지스트리 manifest 접근을 먼저 확인한다.
* 사전 확인 결과는 `IMAGE_NOT_FOUND`, `IMAGE_AUTH_REQUIRED`, `IMAGE_CHECK_FAILED` 로 구분돼 사용자에게 그대로 노출된다.

실제 Kubernetes sync-status reader 를 붙일 때는 아래 env 를 추가한다.

```bash
export AODS_K8S_MODE=kubeconfig
export AODS_K8S_KUBECONFIG="$HOME/.kube/aods-self-hosted.yaml"
export AODS_K8S_CONTEXT="default"
export AODS_FLUX_KUSTOMIZATION_NAMESPACE="flux-system"
export AODS_FLUX_SOURCE_NAME="aods-manifest"
export AODS_K8S_REQUEST_TIMEOUT="5s"
```

주의:

* 앱 생성과 환경 전환은 GitOps repo 안의 `platform/flux/clusters/{clusterId}/` 아래에 Flux child `Kustomization` manifest 를 자동 생성하거나 갱신한다.
* cluster 에는 1회 bootstrap 으로 `platform/flux/bootstrap/{clusterId}/root-kustomization.yaml` 을 apply 해두면, 이후 앱 create/redeploy 가 별도 수동 `Kustomization` 생성 없이 자동 연결된다.

실제 Prometheus metrics reader 를 붙일 때는 아래 env 를 추가한다.

```bash
export AODS_PROMETHEUS_MODE=prometheus
export AODS_PROMETHEUS_URL="http://127.0.0.1:19090"
export AODS_PROMETHEUS_REQUEST_TIMEOUT="5s"
export AODS_PROMETHEUS_RANGE="1h"
export AODS_PROMETHEUS_STEP="5m"
export AODS_PROMETHEUS_PORT_FORWARD_NAMESPACE="monitoring"
export AODS_PROMETHEUS_PORT_FORWARD_SERVICE="kube-prometheus-stack-prometheus"
export AODS_PROMETHEUS_PORT_FORWARD_LOCAL_PORT="19090"
export AODS_PROMETHEUS_PORT_FORWARD_REMOTE_PORT="9090"
```

주의:

* `scripts/backend-run.sh` 는 `AODS_K8S_KUBECONFIG` 가 설정돼 있고 `AODS_PROMETHEUS_URL` 이 localhost 라면 Prometheus port-forward 를 자동으로 연다.
* 앱 생성 시 base manifest 아래에 `ServiceMonitor` 가 자동 생성되어 Flux 경로를 통해 cluster Prometheus scrape 대상에 붙는다.
* CPU/메모리 값은 Prometheus container series 가 비어 있어도 Kubernetes `metrics.k8s.io` 값을 마지막 point 에 채워서 대시보드가 완전히 비지 않게 한다.
* 요청 수/에러율/지연시간은 앱 또는 mesh 가 Prometheus-compatible metrics 를 실제로 노출해야 채워진다. 단순 nginx 처럼 `/metrics` 가 없으면 해당 series 는 `null` 로 유지될 수 있다.

실제 Vault adapter 를 붙일 때는 아래 env 를 추가한다.

```bash
export AODS_VAULT_MODE=token
export AODS_VAULT_ADDR="http://127.0.0.1:18200"
export AODS_VAULT_TOKEN="root"
export AODS_VAULT_NAMESPACE=""
export AODS_VAULT_REQUEST_TIMEOUT="5s"
export AODS_VAULT_PORT_FORWARD_NAMESPACE="vault"
export AODS_VAULT_PORT_FORWARD_SERVICE="vault"
export AODS_VAULT_PORT_FORWARD_LOCAL_PORT="18200"
export AODS_VAULT_PORT_FORWARD_REMOTE_PORT="8200"
```

주의:

* `scripts/backend-run.sh` 는 `AODS_K8S_KUBECONFIG` 가 설정돼 있고 `AODS_VAULT_ADDR` 가 localhost 라면 Vault port-forward 를 자동으로 연다.
* self-hosted dev cluster 에서 `AODS_VAULT_TOKEN=root` 로 `/v1/sys/health` 응답이 확인돼야 real Vault 검증 기준을 만족한다.

## 7. Implementation Order

최소 계약에서 시작해 현재 baseline 으로 넓혀 볼 때의 권장 순서:

1. `platform/projects.yaml`
2. backend `GET /api/v1/projects`
3. backend `GET /api/v1/projects/{projectId}/applications`
4. backend create app
5. backend deploy
6. backend sync status
7. backend metrics
8. frontend project list
9. frontend app list
10. frontend create/app detail wiring

실연동 단계에서는 아래 순서를 권장한다.

1. GitHub-backed project/app reader + writer
2. local secret adapter 유지 상태로 create/redeploy 검증
3. Vault KV v2 adapter 추가
4. Kubernetes/Prometheus real adapter 추가
5. Keycloak real auth 연결

Keycloak real auth 를 붙일 때는 아래 env 를 추가한다.

```bash
export AODS_AUTH_MODE=oidc
export AODS_OIDC_ISSUER_URL="https://sso.aoldacloud.com/realms/<realm>"
export AODS_OIDC_AUDIENCE=""
export AODS_OIDC_JWKS_URL=""
export AODS_OIDC_USER_ID_CLAIM="sub"
export AODS_OIDC_USERNAME_CLAIM="preferred_username"
export AODS_OIDC_DISPLAY_NAME_CLAIM="name"
export AODS_OIDC_GROUPS_CLAIM="groups"
export AODS_PLATFORM_ADMIN_AUTHORITIES="/Ajou_Univ/Aolda_Admin,aods:platform:admin"
export AODS_OIDC_REQUEST_TIMEOUT="5s"
export AODS_ALLOW_DEV_FALLBACK=false
```

주의:

* `oidc` 모드에서는 `Authorization: Bearer ...` 토큰을 우선 사용한다.
* `X-AODS-User-*` 개발용 헤더 인증은 `header` 모드에서만 허용된다.
* `AODS_ALLOW_DEV_FALLBACK=true` 면 로컬에서 Authorization 없이도 dev user 로 기동 가능하지만, real auth 검증 단계에서는 `false` 로 두는 편이 맞다.
* 현재 Keycloak 브라우저 로그인 기준 권장값은 `Client authentication=Off`, `Standard flow=On`, `Implicit flow=Off` 이다.
* 이미 realm 내부 role 구조가 정해져 있으면 audience mapper 를 새로 강제하지 않아도 된다. 이 경우 `AODS_OIDC_AUDIENCE` 는 비워 둔다.
* 백엔드는 `groups` 뿐 아니라 `realm_access.roles`, `resource_access.*.roles` 도 함께 읽어 권한 문자열로 해석한다.
* 현재 이 저장소의 권장 모델은 **새 client role 체계를 만드는 것보다 기존 Keycloak group hierarchy 를 그대로 재사용하는 방식**이다.
* `AODS_PLATFORM_ADMIN_AUTHORITIES` 는 프로젝트 목록과 클러스터 생성에서 전역 admin override 로 동작한다. 예시는 `/Ajou_Univ/Aolda_Admin` 이다.
* 프로젝트별 접근은 여전히 `platform/projects.yaml` 의 `viewerGroups`, `deployerGroups`, `adminGroups` 와 **정확히 문자열 매칭**한다.
* 따라서 `Group Membership` mapper 로 full path 가 들어온다면 `platform/projects.yaml` 에 `/Ajou_Univ/Aolda_Member/<project>/ops` 같은 경로를 직접 써도 된다.
* `AODS_OIDC_ROLE_MAPPINGS` 는 기존 canonical `aods:*` 문자열을 계속 쓰고 싶을 때만 bridge 로 둔다. 형식은 `외부권한=내부권한1|내부권한2` 이고, 여러 항목은 `,` 로 구분한다.

프론트에서 Keycloak MVP 로그인을 직접 붙일 때는 아래 env 를 추가한다.

```bash
export VITE_AODS_AUTH_MODE=oidc
export VITE_AODS_OIDC_ISSUER_URL="https://sso.aoldacloud.com/realms/<realm>"
export VITE_AODS_OIDC_CLIENT_ID="aods"
export VITE_AODS_OIDC_REDIRECT_URI="http://localhost:5173"
export VITE_AODS_OIDC_SCOPE="openid profile email"
export VITE_AODS_PLATFORM_ADMIN_AUTHORITIES="/Ajou_Univ/Aolda_Admin,aods:platform:admin"
```

주의:

* 프론트는 Authorization Code + PKCE 로 access token 을 `sessionStorage` 에 저장한다.
* API client 는 stored access token 을 자동으로 bearer header 로 붙인다.
* access token 이 만료되면 refresh token 으로 1차 갱신을 시도하고, 실패하면 다시 Keycloak authorize endpoint 로 보낸다.
* 브라우저 로그인만 쓸 경우 `Direct access grants` 는 굳이 켜지 않는 편이 안전하다.
* 로그아웃 시 discovery document 에 `end_session_endpoint` 가 있으면 Keycloak logout endpoint 로 리다이렉트하고, 가능하면 `id_token_hint` 를 함께 전달해 SSO 세션도 같이 종료한다.

## 8. When To Stop And Ask

아래 경우만 멈춘다.

* destructive command 필요
* 실제 GitHub/Vault/K8s/Prometheus credential 없이는 마지막 연결 검증이 불가능
* 문서끼리 실제 충돌 발견
* 외부 side effect 가 생기는 단계

그 외에는 계속 진행한다.

## 9. Stronger Persistence

이 파일은 저장소에 남는 handoff 용도다.

더 강한 지속성을 원하면:

* 이 문서들과 계약 문서를 커밋한다.
* 구현은 새 브랜치에서 진행한다.

하지만 커밋 전에도, 이 파일이 있으면 다음 Codex 세션은 문맥을 충분히 이어받을 수 있다.
