# Frontend Swagger Guide

이 문서는 프론트엔드가 AODS API를 가져다 쓸 때 `docs/internal-platform/openapi.yaml` 을 어떻게 읽어야 하는지 정리한 실무용 가이드다.

핵심 원칙:

* API의 최종 계약은 항상 `docs/internal-platform/openapi.yaml` 이다.
* 프론트 타입은 `frontend/src/types/api.ts`, 호출 래퍼는 `frontend/src/api/client.ts` 에 맞춘다.
* 백엔드 구현이 아니라 Swagger/OpenAPI 계약을 먼저 본다.

## Canonical Files

프론트에서 API를 붙일 때는 아래 순서로 본다.

1. `docs/internal-platform/openapi.yaml`
2. `frontend/src/types/api.ts`
3. `frontend/src/api/client.ts`
4. 실제 연결 화면인 `frontend/src/App.tsx`

## Important Contract Notes

### Base URL

프론트는 `frontend/src/api/client.ts` 에서 `VITE_API_BASE_URL` 을 읽는다.

주의:

* `VITE_API_BASE_URL` 이 `/api/v1` 로 끝나도 동작하도록 보정 로직이 있다.
* 프론트에서 경로를 직접 문자열로 추가하지 말고 `api` 객체 메서드를 우선 사용한다.

### Authentication

인증 토큰은 OIDC가 켜져 있으면 `Authorization: Bearer <token>` 으로 붙는다.

프론트 주의점:

* `401` 이면 `api/client.ts` 가 세션을 비운다.
* 에러 처리는 `ApiError` 의 `message`, `code` 를 본다.

### Domain Enums

프론트에서 아래 값은 임의 문자열로 바꾸지 않는다.

* `SyncStatus`: `Unknown`, `Syncing`, `Synced`, `Degraded`
* `ProjectRole`: `viewer`, `deployer`, `admin`
* `writeMode`: `direct`, `pull_request`
* `deploymentStrategy`: `Rollout`, `Canary`
* `ChangeRecord.status`: `Draft`, `Submitted`, `Approved`, `Merged`

## Swagger Operation Map

아래는 프론트 메서드와 Swagger `operationId` 매핑이다.

| Frontend Method | operationId | Method | Path |
| --- | --- | --- | --- |
| `api.getCurrentUser()` | `getCurrentUser` | `GET` | `/api/v1/me` |
| `api.getProjects()` | `listProjects` | `GET` | `/api/v1/projects` |
| `api.createProject(body)` | `createProject` | `POST` | `/api/v1/projects` |
| `api.deleteProject(projectId)` | `deleteProject` | `DELETE` | `/api/v1/projects/{projectId}` |
| `api.getClusters()` | `listClusters` | `GET` | `/api/v1/clusters` |
| `api.createCluster(body)` | `createCluster` | `POST` | `/api/v1/clusters` |
| `api.getProjectEnvironments(projectId)` | `listProjectEnvironments` | `GET` | `/api/v1/projects/{projectId}/environments` |
| `api.getProjectRepositories(projectId)` | `listProjectRepositories` | `GET` | `/api/v1/projects/{projectId}/repositories` |
| `api.getProjectPolicies(projectId)` | `getProjectPolicies` | `GET` | `/api/v1/projects/{projectId}/policies` |
| `api.updateProjectPolicies(projectId, body)` | `updateProjectPolicies` | `PATCH` | `/api/v1/projects/{projectId}/policies` |
| `api.createChange(projectId, body)` | `createProjectChange` | `POST` | `/api/v1/projects/{projectId}/changes` |
| `api.getChange(changeId)` | `getChange` | `GET` | `/api/v1/changes/{changeId}` |
| `api.submitChange(changeId)` | `submitChange` | `POST` | `/api/v1/changes/{changeId}/submit` |
| `api.approveChange(changeId)` | `approveChange` | `POST` | `/api/v1/changes/{changeId}/approve` |
| `api.mergeChange(changeId)` | `mergeChange` | `POST` | `/api/v1/changes/{changeId}/merge` |
| `api.getApplications(projectId)` | `listProjectApplications` | `GET` | `/api/v1/projects/{projectId}/applications` |
| `api.createApplication(projectId, body)` | `createApplication` | `POST` | `/api/v1/projects/{projectId}/applications` |
| `api.deleteApplication(applicationId)` | `deleteApplication` | `DELETE` | `/api/v1/applications/{applicationId}` |
| `api.patchApplication(applicationId, body)` | `patchApplication` | `PATCH` | `/api/v1/applications/{applicationId}` |
| `api.archiveApplication(applicationId)` | `archiveApplication` | `POST` | `/api/v1/applications/{applicationId}/archive` |
| `api.getDeployments(applicationId)` | `listDeployments` | `GET` | `/api/v1/applications/{applicationId}/deployments` |
| `api.createDeployment(applicationId, imageTag, environment)` | `createDeployment` | `POST` | `/api/v1/applications/{applicationId}/deployments` |
| `api.getDeployment(applicationId, deploymentId)` | `getDeployment` | `GET` | `/api/v1/applications/{applicationId}/deployments/{deploymentId}` |
| `api.promoteDeployment(applicationId, deploymentId)` | `promoteDeployment` | `POST` | `/api/v1/applications/{applicationId}/deployments/{deploymentId}/promote` |
| `api.abortDeployment(applicationId, deploymentId)` | `abortDeployment` | `POST` | `/api/v1/applications/{applicationId}/deployments/{deploymentId}/abort` |
| `api.getSyncStatus(applicationId)` | `getApplicationSyncStatus` | `GET` | `/api/v1/applications/{applicationId}/sync-status` |
| `api.getMetrics(applicationId, range)` | `getApplicationMetrics` | `GET` | `/api/v1/applications/{applicationId}/metrics` |
| `api.getRollbackPolicy(applicationId)` | `getRollbackPolicy` | `GET` | `/api/v1/applications/{applicationId}/rollback-policies` |
| `api.saveRollbackPolicy(applicationId, body)` | `saveRollbackPolicy` | `POST` | `/api/v1/applications/{applicationId}/rollback-policies` |
| `api.getEvents(applicationId)` | `listApplicationEvents` | `GET` | `/api/v1/applications/{applicationId}/events` |

## Frontend-Facing Request Notes

### Create Project

Swagger schema:

* `CreateProjectRequest`

실무 포인트:

* `access`, `environments`, `repositories`, `policies` 는 모두 선택값이다
* 값이 비면 백엔드가 PoC 기본값으로 채운다
* 기본 권한 그룹은 `aods:{projectId}:view`, `aods:{projectId}:deploy`, `aods:{projectId}:admin` 이고 전역 admin 권한은 backend `AODS_PLATFORM_ADMIN_AUTHORITIES` 값이 자동 포함된다
* 실서비스에서는 새 role 을 억지로 만들기보다 `platform/projects.yaml` 에 Keycloak group full path 를 직접 적는 방식이 우선이다
* 기존 canonical `aods:*` 문자열을 유지해야 하면 백엔드의 `AODS_OIDC_ROLE_MAPPINGS` 로 Keycloak group/role 을 canonical 권한 문자열에 매핑할 수 있다
* 프로젝트 생성은 `id=name=namespace` 규칙으로 맞춘다
* `name` 은 영문 소문자 slug 여야 하고 Kubernetes namespace 규칙과 동일하다
* `namespace` 를 비우면 `name` 으로 자동 채우고, 값을 보내더라도 `name` 과 같아야 한다
* `environments` 를 비우면 `shared/default/direct` 기본 환경이 생성된다

### Create Application

Swagger schema:

* `CreateApplicationRequest`

실무 포인트:

* `deploymentStrategy` 는 `Rollout` 또는 `Canary`
* `servicePort` 는 필수
* `secrets` 는 배열로 보내며, 값은 Git에 남지 않고 Vault로 간다
* `repositoryId`, `repositoryServiceId`, `configPath` 는 저장소 연동 시 사용한다

### Create Cluster

Swagger schema:

* `CreateClusterRequest`

실무 포인트:

* 현재는 `platform admin` 만 호출할 수 있다
* 이 호출은 `platform/clusters.yaml` 추가와 Flux bootstrap scaffold 생성을 같이 수행한다
* `id` 는 GitOps 경로에 직접 들어가므로 slug 규칙을 지켜야 한다

### Patch Application

Swagger schema:

* `UpdateApplicationRequest`

실무 포인트:

* 부분 업데이트로 취급한다
* 전략 전환, 환경 변경, 저장소 연동 정보 수정이 여기에 들어간다

### Archive Application

Swagger schema:

* `ApplicationLifecycleResponse`

실무 포인트:

* archive 는 active desired state 와 Flux child wiring 을 제거한다
* 대신 `.aods` 메타데이터와 배포/이벤트 이력은 유지한다
* archive 이후 앱 목록에서는 사라지므로 프론트는 상세 유지보다 목록 새로고침 흐름으로 처리하는 편이 안전하다

### Delete Application

Swagger schema:

* `ApplicationLifecycleResponse`

실무 포인트:

* delete 는 앱 디렉터리와 final Vault secret 을 함께 제거한다
* archive 와 달리 history 도 남기지 않는 hard delete 로 보면 된다

### Create Deployment

Swagger schema:

* `CreateDeploymentRequest`

실무 포인트:

* 필수는 `imageTag`
* `environment` 는 선택값이다
* 프론트 `api.getMetrics(applicationId, range)` 와 달리 metrics endpoint 는 `range`, `step` query parameter 를 받는다

## Frontend-Facing Response Notes

### Sync Status

UI는 아래 4개만 처리하면 된다.

* `Unknown`
* `Syncing`
* `Synced`
* `Degraded`

추가 상태를 프론트에서 만들어내지 않는다.

### Deployment Detail

`DeploymentRecord` 는 standard/canary 둘 다 쓰는 공통 shape 다.

주의:

* `rolloutPhase`, `currentStep`, `canaryWeight`, `stableRevision`, `canaryRevision` 은 canary 쪽에서만 의미가 클 수 있다
* 프론트는 nullable/optional 로 처리한다

### Metrics

`ApplicationMetricsResponse.metrics` 는 시계열 배열이다.

주의:

* 일부 포인트는 `value: null` 일 수 있다
* 프론트는 빈 값이 있어도 깨지지 않게 렌더링한다

### Error Response

공통 shape:

* `error.code`
* `error.message`
* `error.details`
* `error.requestId`
* `error.retryable`

프론트는 최소 `code` 와 `message` 를 화면/notification 에 사용한다.

archive/delete 에서 특히 볼 코드:

* `FORBIDDEN`
* `APPLICATION_NOT_FOUND`
* `APPLICATION_ALREADY_ARCHIVED`

## Recommended Frontend Workflow

프론트에서 새 API를 붙일 때는 아래 순서를 권장한다.

1. `openapi.yaml` 에서 endpoint, request schema, response schema, error response 를 확인한다.
2. `frontend/src/types/api.ts` 에 타입을 맞춘다.
3. `frontend/src/api/client.ts` 에 메서드를 추가하거나 기존 메서드를 수정한다.
4. `frontend/src/App.tsx` 또는 실제 연결된 화면에서 호출한다.
5. UI에서 optional 필드와 에러 코드를 처리한다.

## Swagger UI Usage

이 파일은 Swagger/OpenAPI viewer 로 바로 열 수 있다.

대상 파일:

* `docs/internal-platform/openapi.yaml`

권장 용도:

* 프론트가 request body shape 를 빠르게 확인할 때
* 응답 예시를 보고 타입을 맞출 때
* operationId 기준으로 api client 메서드 이름을 정렬할 때
