package kubernetes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aolda/aods-backend/internal/application"
)

func TestRolloutResourcePathEscapesNamespaceAndName(t *testing.T) {
	t.Parallel()

	path := rolloutResourcePath(application.Record{Namespace: "shared env", Name: "demo/app"})
	if !strings.Contains(path, "/namespaces/shared%20env/rollouts/demo%2Fapp") {
		t.Fatalf("unexpected rollout path: %s", path)
	}
}

func TestBuildPromotePatches(t *testing.T) {
	t.Parallel()

	paused := rolloutResponse{}
	paused.Spec.Paused = true
	paused.Status.CurrentPodHash = "canary"
	paused.Status.StableRS = "stable"
	specPatch, statusPatch, unifiedPatch := buildPromotePatches(paused, true)
	if string(specPatch) != `{"spec":{"paused":false}}` {
		t.Fatalf("unexpected full promote spec patch: %s", specPatch)
	}
	if string(statusPatch) != `{"status":{"promoteFull":true}}` {
		t.Fatalf("unexpected full promote status patch: %s", statusPatch)
	}
	if string(unifiedPatch) != `{"spec":{"paused":false},"status":{"promoteFull":true}}` {
		t.Fatalf("unexpected full promote fallback patch: %s", unifiedPatch)
	}

	withPause := rolloutResponse{}
	withPause.Status.PauseConditions = append(withPause.Status.PauseConditions, struct {
		Reason string `json:"reason"`
	}{Reason: "CanaryPauseStep"})
	_, statusPatch, unifiedPatch = buildPromotePatches(withPause, false)
	if string(statusPatch) != `{"status":{"pauseConditions":null}}` {
		t.Fatalf("unexpected pause clear patch: %s", statusPatch)
	}
	if string(unifiedPatch) != `{"spec":{"paused":false},"status":{"pauseConditions":null}}` {
		t.Fatalf("unexpected unified pause clear patch: %s", unifiedPatch)
	}

	withSteps := rolloutResponse{}
	withSteps.Spec.Strategy.Canary = &struct {
		Steps []map[string]any `json:"steps"`
	}{
		Steps: []map[string]any{{"setWeight": 5}, {"pause": map[string]any{}}},
	}
	currentStep := 0
	withSteps.Status.CurrentStepIndex = &currentStep
	_, statusPatch, unifiedPatch = buildPromotePatches(withSteps, false)
	if string(statusPatch) != `{"status":{"pauseConditions":null, "currentStepIndex":1}}` {
		t.Fatalf("unexpected step promote patch: %s", statusPatch)
	}
	if string(unifiedPatch) != `{"spec":{"paused":false},"status":{"pauseConditions":null, "currentStepIndex":1}}` {
		t.Fatalf("unexpected step promote fallback patch: %s", unifiedPatch)
	}
}

func TestMapRolloutInfoDefaultsAndFields(t *testing.T) {
	t.Parallel()

	rollout := rolloutResponse{}
	currentIndex := 1
	rollout.Status.Phase = "Paused"
	rollout.Status.Message = "Waiting for approval"
	rollout.Status.CurrentStepIndex = &currentIndex
	rollout.Status.CurrentPodHash = "canary-hash"
	rollout.Status.StableRS = "stable-hash"
	rollout.Status.Canary.Weights = &struct {
		Canary struct {
			Weight int `json:"weight"`
		} `json:"canary"`
	}{}
	rollout.Status.Canary.Weights.Canary.Weight = 25

	info := mapRolloutInfo(rollout)
	if info.Phase != "Paused" || info.Message != "Waiting for approval" || info.StableRevision != "stable-hash" || info.CanaryRevision != "canary-hash" {
		t.Fatalf("unexpected rollout info: %#v", info)
	}
	if info.CurrentStep == nil || *info.CurrentStep != 2 {
		t.Fatalf("expected 1-based current step 2, got %v", info.CurrentStep)
	}
	if info.CanaryWeight == nil || *info.CanaryWeight != 25 {
		t.Fatalf("expected canary weight 25, got %v", info.CanaryWeight)
	}

	empty := rolloutResponse{}
	empty.Status.PauseConditions = append(empty.Status.PauseConditions, struct {
		Reason string `json:"reason"`
	}{Reason: "ManualPause"})
	info = mapRolloutInfo(empty)
	if info.Phase != "Unknown" || info.Message != "ManualPause" {
		t.Fatalf("expected fallback phase/message, got %#v", info)
	}
}

func TestArgoRolloutControllerGetPromoteAndAbort(t *testing.T) {
	t.Parallel()

	record := application.Record{Namespace: "project-a", Name: "canary-app"}
	resourcePath := rolloutResourcePath(record)
	var patchPaths []string

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == resourcePath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"spec": {
					"paused": true,
					"strategy": {
						"canary": {
							"steps": [
								{"setWeight": 20},
								{"pause": {}}
							]
						}
					}
				},
				"status": {
					"phase": "Paused",
					"message": "waiting for approval",
					"currentStepIndex": 0,
					"currentPodHash": "canary-rs",
					"stableRS": "stable-rs",
					"canary": {
						"weights": {
							"canary": {
								"weight": 20
							}
						}
					}
				}
			}`))
		case r.Method == http.MethodPatch && (r.URL.Path == resourcePath || r.URL.Path == resourcePath+"/status"):
			patchPaths = append(patchPaths, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected rollout request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer api.Close()

	controller := ArgoRolloutController{
		Client: &apiClient{
			BaseURL:    api.URL,
			HTTPClient: api.Client(),
		},
	}

	info, err := controller.GetRollout(context.Background(), record)
	if err != nil {
		t.Fatalf("get rollout: %v", err)
	}
	if info.Phase != "Paused" || info.CurrentStep == nil || *info.CurrentStep != 1 {
		t.Fatalf("unexpected rollout info: %#v", info)
	}

	promoted, err := controller.Promote(context.Background(), record, false)
	if err != nil {
		t.Fatalf("promote rollout: %v", err)
	}
	if promoted.CanaryWeight == nil || *promoted.CanaryWeight != 20 {
		t.Fatalf("unexpected promoted rollout info: %#v", promoted)
	}

	aborted, err := controller.Abort(context.Background(), record)
	if err != nil {
		t.Fatalf("abort rollout: %v", err)
	}
	if aborted.Message != "waiting for approval" {
		t.Fatalf("unexpected aborted rollout info: %#v", aborted)
	}

	if len(patchPaths) != 3 {
		t.Fatalf("expected unpause, promote status, and abort status patches, got %#v", patchPaths)
	}
	if patchPaths[0] != resourcePath+"/status" || patchPaths[1] != resourcePath || patchPaths[2] != resourcePath+"/status" {
		t.Fatalf("unexpected patch paths: %#v", patchPaths)
	}
}

func TestArgoRolloutControllerFallsBackWhenStatusSubresourceMissing(t *testing.T) {
	t.Parallel()

	record := application.Record{Namespace: "project-a", Name: "canary-app"}
	resourcePath := rolloutResourcePath(record)
	var unifiedPatchCount int

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == resourcePath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"spec": {"paused": true},
				"status": {
					"phase": "Progressing",
					"currentPodHash": "canary-rs",
					"stableRS": "stable-rs"
				}
			}`))
		case r.Method == http.MethodPatch && r.URL.Path == resourcePath+"/status":
			http.Error(w, "no status subresource", http.StatusNotFound)
		case r.Method == http.MethodPatch && r.URL.Path == resourcePath:
			unifiedPatchCount++
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected rollout request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer api.Close()

	controller := ArgoRolloutController{
		Client: &apiClient{
			BaseURL:    api.URL,
			HTTPClient: api.Client(),
		},
	}

	if _, err := controller.Promote(context.Background(), record, true); err != nil {
		t.Fatalf("promote should fall back to unified patch: %v", err)
	}
	if _, err := controller.Abort(context.Background(), record); err != nil {
		t.Fatalf("abort should fall back to unified patch: %v", err)
	}
	if unifiedPatchCount != 2 {
		t.Fatalf("expected two resource-level fallback patches, got %d", unifiedPatchCount)
	}
}
