---
applyTo: "frontend/**"
---

# Frontend Instructions

When editing the frontend:

* Use Mantine components plus CSS Modules.
* Do not introduce Tailwind.
* Do not add new inline styles unless there is a temporary, documented reason.
* Check `frontend/src/App.tsx` first; it is still the active integration shell for much of the app.
* Keep API types in `frontend/src/types/api.ts` aligned with `docs/internal-platform/openapi.yaml`.
* Update `frontend/src/api/client.ts` whenever an endpoint or payload changes.
* Preserve domain language and current Korean-first UX copy unless the task explicitly requests otherwise.
* Keep Flux status display limited to `Unknown`, `Syncing`, `Synced`, `Degraded`.
