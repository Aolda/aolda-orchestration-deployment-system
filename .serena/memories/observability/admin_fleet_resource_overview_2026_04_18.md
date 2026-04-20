Added an admin-only fleet resource overview for the Clusters section.

Backend:
- New package `backend/internal/admin` with `/api/v1/admin/resource-overview`.
- `admin.Service` lists all projects from `platform/projects.yaml`, lists all apps across projects, and merges runtime metrics into service-level efficiency rows.
- Access is restricted to `PlatformAdminAuthorities` (same authority model used elsewhere).
- `backend/internal/kubernetes/fleet.go` is the runtime reader for the currently connected Kubernetes API, not for every catalog cluster independently.
- Capacity totals come from:
  - node allocatable: `/api/v1/nodes`
  - cluster-wide pod requests: `/api/v1/pods`
  - cluster-wide pod metrics: `/apis/metrics.k8s.io/v1beta1/pods`
- Per-service rows are matched by namespace + `app.kubernetes.io/name` label, with pod-name fallback.
- Response includes total capacity/request/usage/available CPU+memory, status counts, and service rows classified as `Balanced`, `Underutilized`, `Overutilized`, `NoMetrics`, or `Unknown`.
- If metrics.k8s.io is unavailable, capacity still returns request-based values and message explains that usage is omitted.

Frontend:
- `frontend/src/pages/clusters/ClustersPage.tsx` now renders an admin-only dashboard above the cluster catalog.
- Dashboard shows:
  - project/service counts
  - available CPU/memory
  - request/usage utilization bars for CPU and memory
  - service efficiency distribution cards
  - a detailed per-service table (project, service, cluster, namespace, ready/pod counts, usage/request/limit, status, summary)
- `frontend/src/App.tsx` fetches this overview only for platform admins when the active section is `clusters`, and exposes a `리소스 새로고침` action.

Tests:
- Added `backend/internal/admin/service_test.go`.
- Extended `backend/internal/server/server_test.go` for admin route authorization and response.
- Added `frontend/src/pages/clusters/ClustersPage.test.tsx`.

Important architectural note:
- This feature reflects the single Kubernetes runtime currently connected to the backend. The cluster catalog may list multiple logical targets, but the runtime totals are only as real as the one configured Kubernetes API connection.