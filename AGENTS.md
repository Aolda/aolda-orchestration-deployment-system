# Internal App Deployment Platform - Agent Guide

이 문서는 코덱스(Codex) 및 타 개발 에이전트들이 본 저장소에 기여할 때 참조해야 할 워크로딩(Workloading) 규칙입니다.

## Repo Map
- `docs/`: PRD, `openapi.yaml`, 도메인 규칙 등 에이전트가 코딩을 위해 **반드시 먼저 참고해야 할 계약(Contracts)**이 있습니다.
- `frontend/`: React + Vite 기반의 포털 웹플리케이션이 배치될 폴더입니다.
- `backend/`: Go (Echo/Fiber/Stdlib) 기반의 플랫폼 백엔드 API가 배치될 폴더입니다.
- `scripts/`: 하네스 파이프라인 연동(노션/Discord) 스크립트 모음입니다.

## Allowed / Forbidden Changes
✅ **허용(Allowed)**:
- `openapi.yaml`에 정의된 규격에 맞춰 `backend/`와 `frontend/` 하위 코드를 새롭게 구현하거나 리팩토링하는 행위.
- `docs/domain-rules.md`에 정의된 아키텍처 규칙을 위반하지 않는 선의 데이터 모델 변경.

🚫 **금지(Forbidden)**:
- Kustomize 매니페스트 구조 등 GitOps와 매핑되는 **근간을 벗어나는 구조적 변경** (예: Flux 대신 ArgoCD 배포 로직 임의 작성)
- `openapi.yaml` 내용과 일치하지 않는 형태의 JSON Response 임의 변경.

## Makefile Commands
로컬 개발 및 검증 시 아래 명령을 사용하세요. (수동 검증 또는 `#back-qa` / `#front-qa` 테스트 시)

- `make backend-run`: Go 백엔드 로컬 서버를 구동합니다.
- `make frontend-run`: React Vite 프론트엔드를 개발 서버 모드로 구동합니다.
- `make check`: 전역 컨벤션/린트/테스트를 수행합니다.
