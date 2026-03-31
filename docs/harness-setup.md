# Harness Setup

이 문서는 AODS 저장소에서 하네스 엔지니어링을 시작할 때 필요한 초기 세팅만 다룹니다. 목적은 MVP 속도를 높이되, 이후 직접 개발 단계로 넘어갈 때 하네스 전용 설정이 코어 제품 코드에 엉키지 않게 유지하는 것입니다.

## 1. Source Of Truth
- 제품 동작 계약은 `AGENTS.md`, `docs/internal-platform/openapi.yaml`, `docs/internal-platform/prd.md`, `docs/domain-rules.md`, `docs/acceptance-criteria.md`를 기준으로 삼습니다.
- 하네스 문서는 계약을 보조할 뿐이며, 제품 API나 도메인 규칙보다 우선하지 않습니다.

## 2. Current Repository Status
- 현재 저장소의 핵심 디렉터리는 `backend/`, `frontend/`, `docs/`입니다.
- 하네스 유틸은 별도 영역으로 다룹니다.
- `scripts/`, `config/`, `.envrc` 같은 항목은 로컬 하네스 세팅을 위해 추가될 수 있습니다.

## 3. Required Local Tooling
- `git`
- `make`
- `go`
- `node` and `npm`
- `codex` or `claude`

다음 항목은 하네스 확장 시 선택적으로 필요합니다.

- `gstack`
- `agent-browser`
- `gws`
- ClawFlows related tooling

gstack## 4. Optional Harness Integration Inputs
- `config/mcporter.json`
  Notion sync를 실제로 활성화할 때만 필요한 로컬 토큰 파일입니다. 저장소에 커밋하면 안 됩니다.
- `GCAL_CALENDAR_ID`
  Google Calendar deadline/event 등록을 실제로 활성화할 때만 필요한 환경변수입니다.

중요한 점:

- 위 두 항목은 하네스 연동용 설정입니다.
- 제품 코어 런타임이 이 값들에 직접 의존하도록 만들지 않습니다.
- 값이 없으면 해당 외부 연동만 비활성화되고, 제품 개발 자체는 계속 가능해야 합니다.
- 초기 하네스 MVP에서 Notion sync 또는 Calendar automation을 아직 쓰지 않는다면 세팅하지 않아도 됩니다.

## 5. Recommended Local Layout

```text
AODS/
  backend/
  frontend/
  docs/
  scripts/
  config/
    mcporter.json
  .envrc
  .envrc.example
```

## 6. Environment Variable Setup
프로젝트 전용 환경변수는 글로벌 셸보다 `.envrc`로 관리하는 편이 안전합니다.

예시:

```zsh
export GCAL_CALENDAR_ID="your-calendar-id@group.calendar.google.com"
```

`direnv`를 쓰지 않는 경우에는 현재 셸에서 직접 export 해도 됩니다.

```zsh
export GCAL_CALENDAR_ID="your-calendar-id@group.calendar.google.com"
```

## 7. Bootstrap Steps
1. 저장소 루트로 이동합니다.
2. `config/`와 `scripts/` 디렉터리를 준비합니다.
3. Notion sync가 필요하면 `config/mcporter.json`을 로컬에 배치합니다.
4. Google Calendar 연동이 필요하면 `.envrc` 또는 현재 셸에 `GCAL_CALENDAR_ID`를 설정합니다.
5. `bash scripts/doctor.sh`로 기본 세팅을 점검합니다.
6. `make backend-run`, `make frontend-run`, `make check`가 최소한 동작하는지 확인합니다.

## 8. Verification Commands

```zsh
test -f config/mcporter.json
printenv GCAL_CALENDAR_ID
bash scripts/doctor.sh
make backend-run
make frontend-run
make check
```

위 두 줄은 외부 연동을 실제로 사용할 때만 확인하면 됩니다.

## 9. Separation Rules
- `backend/`와 `frontend/`는 제품 코드입니다.
- `scripts/`, `config/`, 로컬 env는 하네스 운영 영역입니다.
- 하네스 자동화는 제품 코드를 보조해야 하며, 제품 계약을 재정의하면 안 됩니다.
- 외부 연동은 가능하면 `scripts/` 또는 명확한 integration layer에만 모읍니다.

## 10. Definition Of Done For Initial Harness Setup
- 필수 계약 문서 경로가 문서화되어 있다.
- 로컬 비밀정보 파일과 환경변수의 위치가 명확하다.
- 하네스 전용 설정이 `.gitignore`로 보호된다.
- 최소 점검 스크립트가 존재한다.
- 제품 개발 명령과 하네스 점검 명령이 구분되어 있다.
- 외부 연동값이 없어도 기본 개발 루프가 막히지 않는다.

## 11. Planned Follow-Up
- `scripts/sync_internals_notion.py`
- Google Calendar helper scripts
- release and QA orchestration wrappers

위 항목들은 실제 파일이 추가되기 전까지는 planned 상태로 취급합니다.
