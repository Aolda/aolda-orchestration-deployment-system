# Frontend Agent Guide

이 문서는 `frontend/` 작업에서 우선 적용되는 프론트엔드 전용 지침이다.

## Read First

작업 전에 아래 문서를 먼저 확인한다.

1. `docs/internal-platform/openapi.yaml`
2. `docs/acceptance-criteria.md`
3. `docs/phase1-decisions.md`
4. `docs/domain-rules.md`
5. `docs/current-implementation-status.md`
6. `docs/frontend-swagger-guide.md`
7. `docs/user-feedback-log.md`
8. `docs/agent-code-index.md`
9. 루트 `AGENTS.md`

## Non-Negotiables

* 프론트엔드는 Mantine + CSS Modules 기준이다.
* Tailwind 는 사용하지 않는다.
* 새 인라인 스타일은 추가하지 않는다. 기존 레거시 인라인 스타일은 가능하면 줄이는 방향으로 수정한다.
* API 계약은 `docs/internal-platform/openapi.yaml` 을 먼저 기준으로 맞춘다.
* 프론트 API 연동 시 `docs/frontend-swagger-guide.md` 의 operation map 과 request/response 메모를 같이 본다.
* 유저저니를 바꾸는 프론트 작업은 `docs/frontend-user-journeys.md` 와 `frontend/user-stories.json` 을 같이 갱신한다.
* Flux 상태는 UI에 `Unknown`, `Syncing`, `Synced`, `Degraded` 네 개만 노출한다.
* 권한 최소 모델은 `viewer`, `deployer`, `admin` 이다.
* 실제 API/연동이 없는 상태를 가짜 카드, 합성 숫자, placeholder 성공 상태로 감추지 않는다.
* 최소 계약 문서는 여전히 Phase 1 문서군이지만, 회귀 기준선은 `docs/current-implementation-status.md` 의 current implementation baseline 이다.
* 사용자가 명시적으로 요청하지 않으면 새 미래 기능 UI 를 임의로 열지 않는다.
* 이미 노출된 later-phase UI 를 `아직 future scope` 라는 이유로 축소하거나 제거하지 않는다.

## Current Active Structure

* 진입점: `frontend/src/main.tsx`
* 현재 실제 활성 메인 조립 지점: `frontend/src/App.tsx`
* API 타입: `frontend/src/types/api.ts`
* API 클라이언트: `frontend/src/api/client.ts`
* 인증: `frontend/src/auth/oidc.ts`
* 앱 생성 위저드: `frontend/src/components/ApplicationWizard.tsx`
* 분리 중인 화면 컴포넌트: `frontend/src/pages/`

주의:

* `frontend/src/pages/` 와 `frontend/src/app/layout/` 구조가 있지만, 현재 앱 전체가 완전히 그 구조로 이관된 상태는 아니다.
* 새 기능을 붙일 때는 먼저 `App.tsx` 가 실제 연결 지점인지 확인한다.

## Editing Rules

* API 변경이 있으면 `types/api.ts`, `api/client.ts`, 연결된 화면 컴포넌트를 같이 수정한다.
* 백엔드 응답 shape 와 프론트 타입이 어긋나지 않게 유지한다.
* UX 카피는 현재 한국어 중심 흐름을 유지한다. 사용자가 요청하지 않으면 임의로 영어로 바꾸지 않는다.
* 새 사용자 흐름이나 상태 전이가 추가되면 대응하는 `[US-...]` 테스트를 같이 추가한다.
* 사용자가 직접 준 UI/UX 피드백은 `docs/user-feedback-log.md`에 새 항목으로 남기고, 후속 작업에서도 먼저 확인한다.
* 연동 부재는 `Unknown`, `확인 불가`, empty state 같은 정직한 상태로 표현하고, 없는 데이터를 추정치처럼 꾸며서 보여주지 않는다.
* 컴포넌트 분리가 자연스러운 경우에만 `pages/` 나 `components/` 로 이동하고, 억지 리팩토링은 하지 않는다.
* 디자인을 손볼 때는 Mantine props 와 CSS Modules 를 우선 사용한다.
* 새 기능 때문에 `App.tsx` 가 더 비대해지면, 실제로 재사용되는 조각만 안전하게 추출한다.

## Validation

프론트 변경 후 기본 검증은 아래 순서를 권장한다.

1. `cd frontend && npm run lint`
2. `cd frontend && npm run test`
3. `cd frontend && npm run test:stories`
4. 필요하면 `cd frontend && npm run test:stories:report`
5. 공유 가능한 산출물이 필요하면 `cd frontend && npm run test:stories:export`
6. `cd frontend && npm run build`
7. 루트에서 `make check`
   - 현재 `make check` 는 백엔드 `go vet ./...` + `go test ./...`, 프론트 `npm run check` 를 함께 돌린다.

## Common Pitfalls

* `openapi.yaml` 을 안 보고 프론트 타입부터 바꾸는 실수를 피한다.
* `App.tsx` 와 `pages/` 양쪽에 비슷한 UI 흐름을 중복 구현하지 않는다.
* 기존 레거시 인라인 스타일을 따라 새 코드까지 계속 인라인 스타일로 확장하지 않는다.
* 권한, 상태명, 배포 전략명 같은 도메인 용어를 임의로 바꾸지 않는다.
