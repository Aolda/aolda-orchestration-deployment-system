# Future Phases Roadmap

이 문서는 Phase 2, 3, 4를 미리 잠가 두는 로드맵 문서다.
목적은 간단하다.

* 다음 에이전트가 와도 방향이 흔들리지 않게 한다.
* Phase 1 계약을 깨지 않고 어떻게 확장할지 정한다.
* "나중에 생각하자" 때문에 구조가 다시 뒤집히는 걸 막는다.

이 문서는 구현 시작 전 읽는 문서가 아니라, **다음 단계로 넘어갈 때 설계 축을 유지하는 문서**다.

---

## 1. Cross-Phase Invariants

아래 6개는 Phase 2, 3, 4에서도 유지한다.

1. **GitHub 기본 브랜치가 desired state의 최종 기준이다.**
2. **Secret 평문은 Git에 저장하지 않는다.**
3. **프로젝트 목록은 Git 기반 카탈로그에서 읽는다.**
4. **앱 ID는 deterministic 하다.**
5. **GitOps 디렉터리 구조의 상위 경로는 유지한다.**
6. **Flux 상태는 UI에서 `Unknown / Syncing / Synced / Degraded` 4개로만 노출한다.**

즉, 앞으로 기능이 늘어나도 지금 정한 Phase 1 계약을 뒤엎지 않는다.

---

## 1.5 Phase 1 기준 문서

Phase 1 판단은 아래 문서를 기준으로 잡았다.

내부 문서:

* `docs/internal-platform/prd.md`
* `docs/domain-rules.md`
* `docs/phase1-decisions.md`
* `docs/internal-platform/openapi.yaml`
* `docs/acceptance-criteria.md`

외부 공식 문서:

* Flux Kustomization docs: <https://fluxcd.io/flux/components/kustomize/kustomizations/>
* Vault KV v2 tutorial: <https://developer.hashicorp.com/vault/tutorials/secrets-management/versioned-kv>
* Vault KV metadata docs: <https://developer.hashicorp.com/vault/docs/commands/kv/metadata>
* External Secrets API docs: <https://external-secrets.io/v0.19.0/api/externalsecret/>

해석 원칙:

* 내부 문서가 제품 계약이다.
* 외부 문서는 구현 판단의 기술 기준이다.
* 둘이 충돌하면 제품 계약을 먼저 고치고 구현한다.

---

## 2. Phase 2: Progressive Delivery

### 2.1 목적

Phase 2의 목적은 "배포를 더 안전하게 만드는 것"이다.

Phase 1이 앱을 **배포되게** 했다면,
Phase 2는 앱을 **조심스럽게 배포되게** 한다.

### 2.2 사용자 결과

사용자는 새 버전을 한 번에 100% 배포하지 않고,
적은 트래픽부터 점진적으로 흘려보내며 상태를 보고,
필요하면 승격하거나 중단할 수 있어야 한다.

### 2.3 포함 범위

* `Standard` 외에 `Canary` 전략 허용
* Argo Rollouts 기반 rollout 상태 조회
* 수동 `Promote`
* 수동 `Abort`
* 현재 canary 비중 표시
* rollout 단계 표시
* deployment history 기본 조회

### 2.4 제외 범위

* 완전 자동 rollback
* 다중 승인 워크플로우
* 멀티클러스터
* 복잡한 release orchestration

### 2.5 권장 기본 정책

처음부터 사용자 정의 rollout step 편집 UI를 열지 않는다.
처음엔 플랫폼 기본 정책 하나로 간다.

권장 기본 step:

* `5%`
* `25%`
* `50%`
* `100%`

권장 기본 동작:

* 각 단계는 수동 승격
* 어느 단계에서든 수동 중단 가능
* `Abort` 시 stable 로 즉시 복귀

이렇게 가면 Phase 2가 "플랫폼 기능"이 아니라 "새로운 DSL 설계"로 변질되지 않는다.

### 2.6 GitOps 구조 변화

Phase 2에서는 canary 앱에 한해 `deployment.yaml` 대신 `rollout.yaml` 을 둘 수 있다.
다만 아래 규칙은 유지한다.

* 앱 경로는 그대로 `apps/{projectId}/{appName}/`
* `service.yaml`, `virtualservice.yaml`, `destinationrule.yaml` 경로 유지
* `kustomization.yaml` 만 참조 대상을 바꿀 수 있다

즉, Phase 1 경로 구조는 유지하고, 내부 파일 참조만 확장한다.

### 2.7 API 후보

아직 확정 계약은 아니지만, 다음 API를 기준으로 생각한다.

* `PATCH /api/v1/applications/{applicationId}`
  - strategy 변경
  - canary 활성화
* `GET /api/v1/applications/{applicationId}/deployments`
* `GET /api/v1/applications/{applicationId}/deployments/{deploymentId}`
* `POST /api/v1/applications/{applicationId}/deployments/{deploymentId}/promote`
* `POST /api/v1/applications/{applicationId}/deployments/{deploymentId}/abort`

### 2.8 상태 모델

Phase 2에서 새로 보여줘야 하는 필드:

* `rolloutPhase`
* `currentStep`
* `canaryWeight`
* `stableRevision`
* `canaryRevision`

이건 `syncStatus` 를 대체하지 않는다.
`syncStatus` 는 그대로 두고, rollout 상태를 별도 카드로 보여준다.

### 2.9 성공 기준

1. 사용자가 `Canary` 전략으로 배포를 시작할 수 있다.
2. 현재 트래픽 비중과 단계가 보인다.
3. 사용자가 `Promote` 로 다음 단계로 갈 수 있다.
4. 사용자가 `Abort` 로 배포를 중단할 수 있다.
5. stable 복귀 이후 앱 상세에서 실패 이유를 확인할 수 있다.

### 2.10 Handoff Guardrail

Phase 2 구현 에이전트는 아래를 건드리면 안 된다.

* GitHub source of truth 제거
* Secret 을 Git에 저장하는 설계
* 앱 ID 규칙 변경
* Phase 1 direct push 경로 제거

Phase 2는 확장이지 재설계가 아니다.

### 2.11 기준 문서

Phase 2 판단은 아래 문서를 기준으로 잡았다.

내부 문서:

* `docs/internal-platform/prd.md`
* `docs/domain-rules.md`
* `docs/phase1-decisions.md`
* `docs/future-phases-roadmap.md`

외부 공식 문서:

* Argo Rollouts canary strategy docs: <https://argo-rollouts.readthedocs.io/en/stable/features/canary/>
* Argo Rollouts rollback window docs: <https://argo-rollouts.readthedocs.io/en/stable/features/rollback/>
* Istio traffic shifting docs: <https://istio.io/latest/docs/tasks/traffic-management/traffic-shifting/>
* Flux Kustomization docs: <https://fluxcd.io/flux/components/kustomize/kustomizations/>

왜 이 문서들을 기준으로 잡았나:

* canary, promote, abort 의 주체는 Argo Rollouts 가 가장 명확하다.
* 트래픽 비중 조절은 Istio `VirtualService` 기준이 가장 직접적이다.
* GitOps 반영 상태는 여전히 Flux 기준으로 읽어야 한다.

---

## 3. Phase 3: Change Management And Environments

### 3.1 목적

Phase 3의 목적은 "변경을 더 통제 가능하게 만드는 것"이다.

Phase 2까지가 배포 안전성이라면,
Phase 3은 운영 조직이 신뢰할 수 있는 **검토 가능한 변경 흐름**을 만든다.

### 3.2 사용자 결과

사용자는 운영 환경에 대한 변경을 그냥 바로 밀어넣는 대신,
차이를 보고, 리뷰를 받고, 승인 후 반영할 수 있어야 한다.

### 3.3 포함 범위

* direct push 외에 PR 기반 write mode 추가
* 환경 개념 도입: `dev`, `staging`, `prod`
* diff preview
* 변경 요청 단위의 audit trail
* 누가 제안했고 누가 승인했는지 기록
* deployment history 고도화
* `admin` 권한 실체화

### 3.4 제외 범위

* Jira/Slack/ServiceNow 같은 외부 승인 시스템 깊은 연동
* 조직별 복잡한 다단계 승인 체계
* 정책 엔진 전면 도입

### 3.5 핵심 설계 포인트

Phase 3에서는 **Change** 라는 새 리소스를 도입하는 게 맞다.

이유:

* `Deployment` 는 "배포 행위"다
* `Change` 는 "Git에 반영되기 전 검토 가능한 변경 단위"다

둘을 하나로 섞으면 direct push와 PR mode를 같이 지원하기 어려워진다.

권장 관계:

* `Change` -> Git diff / PR / approval 상태
* `Deployment` -> merge 후 실제 rollout / sync / metrics 상태

### 3.6 환경 모델

Phase 3부터는 `prod` 고정에서 벗어난다.

권장 환경 모델:

* `dev`
* `staging`
* `prod`

권장 원칙:

* 비프로덕션은 direct push 허용 가능
* `prod` 는 project 설정에 따라 PR mode 강제 가능

### 3.7 API 후보

* `GET /api/v1/projects/{projectId}/environments`
* `POST /api/v1/projects/{projectId}/changes`
* `GET /api/v1/changes/{changeId}`
* `POST /api/v1/changes/{changeId}/submit`
* `POST /api/v1/changes/{changeId}/approve`
* `POST /api/v1/changes/{changeId}/merge`
* `GET /api/v1/applications/{applicationId}/history`

### 3.8 GitOps 구조 변화

환경 확장이 들어오더라도 아래는 유지한다.

* `apps/{projectId}/{appName}/base`
* `apps/{projectId}/{appName}/overlays/{environment}`

즉, `prod` 만 있던 구조에 `dev`, `staging` 을 추가하는 식으로 확장한다.

### 3.9 성공 기준

1. 운영 환경 변경 전에 diff preview를 볼 수 있다.
2. PR mode 프로젝트는 승인 전 merge 되지 않는다.
3. 앱 단위 변경 이력과 배포 이력을 구분해 볼 수 있다.
4. `admin` 과 `deployer` 의 권한 차이가 실제 동작으로 반영된다.

### 3.10 Handoff Guardrail

Phase 3 구현 에이전트는 PR mode를 추가하되,
Phase 1/2 direct push 흐름을 없애면 안 된다.

즉, "replace" 가 아니라 "support both" 로 가야 한다.

### 3.11 기준 문서

Phase 3 판단은 아래 문서를 기준으로 잡았다.

내부 문서:

* `docs/internal-platform/prd.md`
* `docs/domain-rules.md`
* `docs/phase1-decisions.md`
* `docs/future-phases-roadmap.md`

외부 공식 문서:

* GitHub Pull Requests docs: <https://docs.github.com/en/pull-requests>
* GitHub protected branches docs: <https://docs.github.com/articles/about-required-reviews-for-pull-requests>
* Kubernetes Kustomize docs: <https://kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization>

왜 이 문서들을 기준으로 잡았나:

* PR mode 와 승인 흐름은 GitHub 기본 모델을 따르는 게 가장 단순하다.
* branch protection 이 있어야 `prod` 에만 PR 강제 같은 정책이 현실적으로 가능하다.
* 환경 확장은 기존 `base/overlays` 구조를 유지해야 하므로 Kustomize overlay 모델을 기준으로 잡았다.

---

## 4. Phase 4: Platform Scale And Guardrails

### 4.1 목적

Phase 4의 목적은 "한 팀용 배포 포털"을 "여러 팀이 오래 써도 안 무너지는 플랫폼"으로 키우는 것이다.

### 4.2 사용자 결과

플랫폼 팀은 여러 프로젝트, 여러 환경, 나중에는 여러 클러스터를 관리할 수 있어야 하고,
개발자는 더 강한 자동 안전장치 안에서 배포할 수 있어야 한다.

### 4.3 포함 범위

* 멀티클러스터 또는 cluster target 개념
* 프로젝트별 정책 템플릿
* 메트릭 기반 자동 rollback
* 앱/프로젝트별 기본 보안 가드레일
* stale secret / orphan manifest 정리
* 프로젝트 self-service bootstrap

### 4.4 제외 범위

* 범용 CI/CD 플랫폼으로의 확장
* 모든 쿠버네티스 리소스를 UI에서 다 편집하는 범용 콘솔

이 제품은 내부 앱 배포 플랫폼이지, 또 하나의 쿠버네티스 대시보드가 아니다.

### 4.5 핵심 설계 포인트

Phase 4부터는 "정책"이 1급 개체가 된다.

예:

* 최소 replica
* allowed namespace
* required probes
* allowed environments
* prod PR required
* auto rollback enabled

이 시점에는 프로젝트별 자유 입력보다 정책 템플릿이 더 중요하다.

### 4.6 API 후보

* `GET /api/v1/clusters`
* `GET /api/v1/projects/{projectId}/policies`
* `PATCH /api/v1/projects/{projectId}/policies`
* `POST /api/v1/applications/{applicationId}/rollback-policies`
* `GET /api/v1/applications/{applicationId}/events`

### 4.7 성공 기준

1. 하나의 플랫폼이 여러 환경과 클러스터를 다룰 수 있다.
2. 정책 위반 변경은 배포 전에 막힌다.
3. 메트릭 기준 자동 rollback 이 예측 가능하게 동작한다.
4. 플랫폼 팀이 수동 cleanup 없이도 stale 리소스를 정리할 수 있다.

### 4.8 Handoff Guardrail

Phase 4 구현 에이전트는 아래 유혹을 피해야 한다.

* 범용 쿠버네티스 UI가 되려는 시도
* GitOps 대신 직접 apply 중심으로 회귀하는 시도
* Secret/정책/배포를 각기 다른 정답 저장소로 찢는 시도

이 플랫폼의 힘은 "한 곳에서 시작하지만, 최종 기준은 Git" 이라는 점이다.

### 4.9 기준 문서

Phase 4 판단은 아래 문서를 기준으로 잡았다.

내부 문서:

* `docs/internal-platform/prd.md`
* `docs/domain-rules.md`
* `docs/phase1-decisions.md`
* `docs/future-phases-roadmap.md`

외부 공식 문서:

* Flux docs overview, multi-tenancy and multi-cluster references: <https://fluxcd.io/docs/>
* Argo Rollouts rollback docs: <https://argo-rollouts.readthedocs.io/en/stable/features/rollback/>
* Prometheus alerting rules docs: <https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/>
* Prometheus Alertmanager docs: <https://prometheus.io/docs/alerting/latest/alertmanager/>
* External Secrets API docs: <https://external-secrets.io/v0.19.0/api/externalsecret/>

왜 이 문서들을 기준으로 잡았나:

* multi-cluster 와 multi-tenancy 축은 Flux 문서가 가장 직접적이다.
* 자동 rollback 은 결국 rollout controller 와 alert rule 이 함께 움직여야 하므로 Argo Rollouts 와 Prometheus 를 같이 봐야 한다.
* stale secret 정리와 refresh 정책은 External Secrets 동작 기준을 알아야 안전하다.

---

## 5. Recommended Order

추천 순서는 아래다.

1. **Phase 1**
   - 프로젝트 목록
   - Standard 배포
   - sync 상태
   - metrics

2. **Phase 2**
   - canary
   - promote / abort
   - rollout visibility

3. **Phase 3**
   - PR mode
   - environments
   - audit / approval

4. **Phase 4**
   - policy
   - auto rollback
   - cluster scale

이 순서를 뒤집으면 대체로 망한다.
PR부터 만들면 아무도 배포를 못 하고,
멀티클러스터부터 만들면 첫 팀도 못 태운다.

---

## 6. What A Future Agent Must Read First

다음 에이전트가 들어오면 최소 아래 순서로 읽어야 한다.

1. `docs/internal-platform/prd.md`
2. `docs/domain-rules.md`
3. `docs/phase1-decisions.md`
4. `docs/future-phases-roadmap.md`
5. `docs/internal-platform/openapi.yaml`

이 순서를 건너뛰고 구현부터 시작하면, 거의 반드시 source of truth 나 write model 을 흔들게 된다.
