Added repository polling control end-to-end.

Backend:
- Added per-application `RepositoryPollIntervalSeconds` support with allowed values only `60`, `300`, `600`.
- Persisted interval in `.aods/metadata.yaml` and workload annotations as `aods.io/repository-poll-interval-seconds`.
- `RepositoryPollTracker` now resolves effective interval per app, supports `Due(app, now)` and `Reschedule(app, now)`, and snapshots show the app-specific interval.
- `AutoUpdatePoller` now ticks every 1 minute but only syncs a repo-backed app when the tracker says it is due. This enables mixed 1/5/10 minute schedules without multiple workers.
- Added `POST /api/v1/applications/{applicationId}/sync` for manual repository sync. It reuses the same repo-resolution/reconcile/deploy logic as the poller and returns `RepositorySyncResponse`.
- `PatchApplication` validates `repositoryPollIntervalSeconds`, rejects non-repo apps, and reschedules tracker state after a successful change.

Frontend:
- Status tab now shows explicit polling interval badge.
- Added repository control card with Select for `1분 / 5분 / 10분`, `주기 저장`, and `지금 Sync`.
- Saving interval calls `PATCH /api/v1/applications/{id}` with `repositoryPollIntervalSeconds`.
- Manual sync calls `POST /api/v1/applications/{id}/sync` and refreshes app/project details.

Contract/docs:
- Updated `docs/internal-platform/openapi.yaml` with `repositoryPollIntervalSeconds` fields and new `RepositorySyncResponse` + `/sync` path.

Verification:
- `go test ./internal/application ./internal/server ./internal/core`
- `frontend npm run check`
- Live smoke check via curl against `GET /api/v1/applications/project-a__moltbot-front-poc-web/sync-status` and `POST /api/v1/applications/project-a__moltbot-front-poc-web/sync` on local backend port 18080.
