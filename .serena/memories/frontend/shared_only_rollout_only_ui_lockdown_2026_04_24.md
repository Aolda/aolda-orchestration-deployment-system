2026-04-24 frontend state for AODS after UI simplification pass. Scope: make current UX operate in a locked-down shared/dev mode so next agents understand what is intentionally hidden vs unfinished.

What was changed:
- Only the shared project is shown in frontend bootstrap/project selection flow. App.tsx now filters fetched projects through isSharedProject(project) and only keeps namespace=shared projects visible. selectedProjectId falls back to the first visible shared project.
- New project UI is hidden in frontend. showProjectComposer=false, SidebarNav create action is disabled from AppShell props, and the project creation drawer is additionally gated by showProjectComposer so it cannot open even if stale state exists.
- Application operations / rules drawer was simplified:
  - Hidden from UI: automatic rollback policy section, Istio mesh toggle section, emergency action section, application lifecycle section.
  - Visible/active in rules tab: external exposure (LoadBalancer) and resource allocation only.
- Application catalog cards no longer show Istio mesh badge/sentence. Summary focuses on namespace, deployment strategy, loadbalancer exposure, latest deployment/state.
- Resource allocation UX changed from freeform request/limit text inputs to:
  - fixed default requests: CPU 250m, Memory 256Mi
  - selectable limit presets only: CPU 500m or 1000m, Memory 512Mi or 1Gi
  - save path always persists default requests plus selected limits. Requests are not user-editable in UI anymore.
- Deployment strategy is frontend-locked to Rollout only:
  - supportedDeploymentStrategies constant is ['Rollout'].
  - project policy save forces allowedDeploymentStrategies=['Rollout'] regardless of UI state.
  - ApplicationWizard receives only Rollout as allowedStrategies and its deployment strategy select is disabled.
  - ChangesWorkspace create-draft form also only shows Rollout and request payload forces deploymentStrategy='Rollout'.
- Project policy panel further simplified:
  - allowedEnvironments and allowedClusterTargets are shown as read-only fixed values from current policy, not editable.
  - allowedDeploymentStrategies is shown as read-only Rollout only.
  - guidance text explains these are platform-fixed values.
- Existing prodPRRequired ('운영 환경 변경 요청 필수') toggle still exists in project policy panel as of this pass. It maps conceptually to requiring reviewed change flows for protected environments, but UX may still feel redundant because environment writeMode already communicates direct vs pull_request behavior.

Files touched in this pass:
- frontend/src/App.tsx
- frontend/src/components/ApplicationWizard.tsx
- frontend/src/pages/changes/ChangesWorkspace.tsx
- frontend/src/components/navigation/SidebarNav.test.tsx
- docs/frontend-user-journeys.md

Behavioral intent / product assumptions now encoded in frontend:
- current operational mode is effectively a shared-project-only portal
- project creation is intentionally hidden, not broken
- rollout is the only supported deployment strategy in frontend for now
- environment/cluster targets are platform-fixed, not operator-editable from frontend
- resource sizing is opinionated and constrained for shared/dev usage

Suggested next-agent awareness:
- If future work re-enables project creation, shared-only project filtering and related tests/docs must be revisited together.
- If future work re-enables Canary or Istio UI, multiple conditions in App.tsx were intentionally bypassed and will need restoration plus policy/validation reconsideration.
- If backend still allows broader values, current behavior is frontend constraint only; no backend contract change was made in this pass.

Verification completed after this pass:
- frontend npm run check passed fully (lint, vitest, story coverage, build).
- Only remaining build note was existing Vite large-chunk warning; not treated as failure.