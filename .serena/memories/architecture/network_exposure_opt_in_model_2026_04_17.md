# Network Exposure Model (2026-04-17)

User decision:
- Istio is no longer the default path.
- Per-application network exposure must be operator-controlled.
- Users/project admins can toggle LoadBalancer exposure per application.

Implemented model:
- `meshEnabled=false` and `loadBalancerEnabled=false` is the default posture for normal apps.
- `meshEnabled=true` makes backend render Istio-related pieces only then:
  - sidecar injection annotations/labels
  - `virtualservice.yaml`
  - `destinationrule.yaml`
  - envoy metrics service port / ServiceMonitor endpoint
- `loadBalancerEnabled=true` makes backend render `Service.spec.type=LoadBalancer`.
- These two flags are persisted in application metadata and returned by API responses (`Application`, `ApplicationSummary`).

Validation rules:
- `Canary` requires `meshEnabled=true`.
- `Canary` rejects `loadBalancerEnabled=true`.
- Updating `meshEnabled` or `loadBalancerEnabled` is admin-only, same as resource allocation.

Frontend:
- App card now shows `LB 공개` vs `내부 전용`, and `Istio mesh` vs `일반 서비스` badges.
- Operations drawer > rules tab has admin-only switches:
  - `Istio mesh 사용`
  - `LoadBalancer 노출`
- Canary apps show explanatory alert and switches are locked accordingly.

Docs updated:
- `docs/internal-platform/openapi.yaml`
- `docs/user-feedback-log.md`
- `docs/platform-shape-and-detail-backlog.md`

Regression coverage:
- `backend/internal/application/service_test.go`
- `backend/internal/application/store_local_network_test.go`
- server tests updated so non-mesh rollout apps no longer expect Istio files by default.

Important follow-up assumption:
- In this product, one AODS application corresponds to one service entry from `aolda_deploy.json`, so “service-level LB toggle” is effectively implemented at application level.