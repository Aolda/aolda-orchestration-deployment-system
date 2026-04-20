---
applyTo: "backend/**,platform/**,apps/**"
---

# Backend And GitOps Instructions

When editing backend, platform catalog, or generated application manifests:

* Preserve the GitHub-default-branch source-of-truth model.
* Keep project catalog reads aligned with `platform/projects.yaml`.
* Keep application identity deterministic as `{projectId}__{appName}`.
* Do not introduce Kubernetes native `Secret` resources for application secrets.
* Keep Vault on the `staging -> git commit -> final -> cleanup` flow.
* Register new routes in `backend/internal/server/server.go`.
* Keep API shapes aligned with `docs/internal-platform/openapi.yaml`.
* Update backend tests when changing routes, validation, or JSON response shapes.
* Treat `apps/{projectId}/{appName}/` as store output; prefer changing store logic instead of hand-editing generated artifacts.
