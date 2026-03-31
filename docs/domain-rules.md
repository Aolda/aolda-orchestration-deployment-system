# Domain Rules & Architecture Constraints

이 문서는 코덱스(Codex) 및 에이전트(Agent)가 애플리케이션 코드를 생성/수정할 때 지켜야 하는 **절대적인 도메인 제약 사항**입니다.

---

## 1. GitOps Manifest 구조 규칙 (Istio + Vault + Kustomize 우선권)
플랫폼 백엔드는 사용자의 앱 생성 요청을 받으면 클러스터 API가 아닌 **GitHub에 Kustomize 폴더 구조를 커밋**해야 합니다.

- 앱별 디렉토리 구조 템플릿:
  ```yaml
  apps/
    {projectId}/
      {appName}/
        base/
          kustomization.yaml
          deployment.yaml        # Argo Rollout 또는 기본 Deployment
          service.yaml           
          virtualservice.yaml    # Istio 라우팅 기본 명세
          destinationrule.yaml   # Istio Canary 가중치 및 라우팅 명세
          externalsecret.yaml    # Vault 패스 포인터 역할 (안에 실제 값 기록 절대 금지)
        overlays/
          prod/
            kustomization.yaml   # 이미지 태그 패치
  ```

## 2. 보안 관련 제약 (Secret 금지, Vault 직접 통신)
- **금지 조항**: 백엔드는 Kubernetes 기본 `Secret` 리소스를 사용하는 매니페스트를 Kustomize 파일셋에 생성하면 절대 안 됩니다!
- **Vault 통신 모델**: 플랫폼 포털 UI에서 사용자가 입력한 프라이빗 Secret 값은 `openapi.yaml` 에 표기된 Payload로 백엔드에 넘어오게 됩니다. 백엔드는 이 값을 즉시 **Vault API (client)**를 통해 저장소에 밀어넣습니다. Git에는 위의 `externalsecret.yaml` 이라는 껍데기만 남깁니다.

## 3. 플랫폼의 클러스터 양방향 통신 규칙 (Read / Write)
에이전트 구현 시, GitOps 시스템이라고 해서 모든 흐름을 단방향 처리하지 마십시오.
*   **Write 흐름 (Desired State)**: 매니페스트 렌더링 후 `go-git` 또는 GitHub API로 저장소에 커밋/푸시합니다. DB(PostgreSQL/SQLite)에 배포 버전 정보를 남깁니다.
*   **Read 흐름 (Actual State 조회)**: K8s `client-go`를 활용해 API Server에 접근, `Flux`의 `Kustomization` CRD 상태 정보(`status.conditions`)를 파싱하여 UI로 넘깁니다.
*   **제어 흐름 (Promote/Abort)**: 롤아웃 조작 명령 시, GitOps를 통하지 않고 즉각 `client-go` 기반으로 해당 네임스페이스 내 해당 앱의 `Rollout` CRD 상태를 패치(Patch)합니다.

## 4. 프론트엔드/백엔드 기본 컨벤션 (ADR-001, ADR-002 참조)
* 프론트엔드 (Vite + React + Zustand)
  - **절대적 금지**: Tailwind CSS 및 인라인 스타일 사용을 원천 금지합니다.
  - UI 렌더링은 무조건 **Mantine v7** 컴포넌트를 호출하여 조립하며, 커스텀 디자인은 오직 **CSS Modules**를 사용해야 합니다.
  - API 통신은 `openapi.yaml` 100% Mocking 상태에서 먼저 컴포넌트를 뽑아내야 합니다.

* 백엔드 (Go 1.22+ Standard Library)
  - **절대적 금지**: Fiber, Gin, Echo와 같은 서드파티 웹 프레임워크 사용을 금지합니다.
  - 라우팅은 오직 `net/http` 표준 라이브러리의 1.22버전 이상 Path Variable Mapping 방식을 사용합니다.
  - AI 컨텍스트 유지를 위해 너무 깊은 레이어드 아키텍처 분리를 지양하고 `internal/application/`, `internal/project/` 처럼 **도메인 단위(Flat Domain Pattern)**로 비즈니스 로직과 Data Access(DB/Git) 기능을 폴더링하십시오.
