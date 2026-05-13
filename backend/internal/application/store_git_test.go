package application

import (
	"context"
	"testing"
	"time"
)

func TestGitManifestStoreRequiresRepository(t *testing.T) {
	t.Parallel()

	store := GitManifestStore{}
	ctx := context.Background()
	project := ProjectContext{ID: "project-a", Namespace: "project-a"}
	createRequest := CreateRequest{Name: "demo", Image: "repo/demo:v1", ServicePort: 8080}
	deployment := DeploymentRecord{
		DeploymentID:  "dep_1",
		ApplicationID: "project-a__demo",
		CreatedAt:     time.Now().UTC(),
	}

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "list applications", run: func() error {
			_, err := store.ListApplications(ctx, "project-a")
			return err
		}},
		{name: "get application", run: func() error {
			_, err := store.GetApplication(ctx, "project-a__demo")
			return err
		}},
		{name: "create application", run: func() error {
			_, err := store.CreateApplication(ctx, project, createRequest, "")
			return err
		}},
		{name: "archive application", run: func() error {
			_, err := store.ArchiveApplication(ctx, "project-a__demo", "admin")
			return err
		}},
		{name: "delete application", run: func() error {
			_, err := store.DeleteApplication(ctx, "project-a__demo")
			return err
		}},
		{name: "update image", run: func() error {
			_, err := store.UpdateApplicationImage(ctx, project, "project-a__demo", "v2", "dep_1")
			return err
		}},
		{name: "patch application", run: func() error {
			_, err := store.PatchApplication(ctx, project, "project-a__demo", UpdateApplicationRequest{})
			return err
		}},
		{name: "save secret path", run: func() error {
			_, err := store.SaveApplicationSecretPath(ctx, project, "project-a__demo", "secret/aods/apps/project-a/demo/prod")
			return err
		}},
		{name: "list deployments", run: func() error {
			_, err := store.ListDeployments(ctx, "project-a__demo")
			return err
		}},
		{name: "get deployment", run: func() error {
			_, err := store.GetDeployment(ctx, "project-a__demo", "dep_1")
			return err
		}},
		{name: "update deployment", run: func() error {
			_, err := store.UpdateDeployment(ctx, "project-a__demo", deployment)
			return err
		}},
		{name: "get rollback policy", run: func() error {
			_, err := store.GetRollbackPolicy(ctx, "project-a__demo")
			return err
		}},
		{name: "save rollback policy", run: func() error {
			_, err := store.SaveRollbackPolicy(ctx, "project-a__demo", RollbackPolicy{Enabled: true})
			return err
		}},
		{name: "list events", run: func() error {
			_, err := store.ListEvents(ctx, "project-a__demo")
			return err
		}},
		{name: "append event", run: func() error {
			return store.AppendEvent(ctx, "project-a__demo", Event{ID: "evt_1", Type: "Test"})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := tt.run(); err == nil {
				t.Fatal("expected missing repository error")
			}
		})
	}
}

func TestGitManifestStoreLocalStoreCarriesFluxSettings(t *testing.T) {
	t.Parallel()

	store := GitManifestStore{
		FluxKustomizationNamespace: "flux-system",
		FluxSourceName:             "aods-manifest",
	}
	local := store.localStore("/tmp/repo")
	if local.RepoRoot != "/tmp/repo" || local.FluxKustomizationNamespace != "flux-system" || local.FluxSourceName != "aods-manifest" {
		t.Fatalf("unexpected local store config: %#v", local)
	}
}
