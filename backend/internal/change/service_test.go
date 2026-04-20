package change

import (
	"testing"

	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/project"
)

func TestResolveEnvironmentPrefersExplicitRequestThenDefaultThenShared(t *testing.T) {
	t.Parallel()

	projectInfo := project.CatalogProject{
		Environments: []project.Environment{
			{ID: "dev", Name: "Development"},
			{ID: "prod", Name: "Production", Default: true},
		},
	}

	if got := resolveEnvironment(projectInfo, "dev"); got != "dev" {
		t.Fatalf("expected explicit environment dev, got %s", got)
	}
	if got := resolveEnvironment(projectInfo, ""); got != "prod" {
		t.Fatalf("expected default environment prod, got %s", got)
	}
	if got := resolveEnvironment(project.CatalogProject{}, ""); got != "shared" {
		t.Fatalf("expected shared fallback environment, got %s", got)
	}
}

func TestResolveWriteModeUsesEnvironmentSettingAndProdFallback(t *testing.T) {
	t.Parallel()

	projectInfo := project.CatalogProject{
		Environments: []project.Environment{
			{ID: "dev", WriteMode: project.WriteModeDirect},
			{ID: "prod", WriteMode: project.WriteModePullRequest},
		},
		Policies: project.PolicySet{ProdPRRequired: true},
	}

	if got := resolveWriteMode(projectInfo, "prod"); got != project.WriteModePullRequest {
		t.Fatalf("expected prod write mode pull_request, got %s", got)
	}
	if got := resolveWriteMode(projectInfo, "dev"); got != project.WriteModeDirect {
		t.Fatalf("expected dev write mode direct, got %s", got)
	}
	if got := resolveWriteMode(project.CatalogProject{Policies: project.PolicySet{ProdPRRequired: true}}, "prod"); got != project.WriteModePullRequest {
		t.Fatalf("expected prod fallback write mode pull_request, got %s", got)
	}
	if got := resolveWriteMode(project.CatalogProject{}, "stage"); got != project.WriteModeDirect {
		t.Fatalf("expected non-prod fallback write mode direct, got %s", got)
	}
}

func TestBuildDiffPreviewUsesStrategySpecificWorkloadFile(t *testing.T) {
	t.Parallel()

	diff := buildDiffPreview("payments", "prod", Request{
		Operation:          OperationCreateApplication,
		Name:               "checkout",
		DeploymentStrategy: application.DeploymentStrategyCanary,
	})

	if len(diff) != 2 {
		t.Fatalf("expected two diff preview lines, got %d", len(diff))
	}
	if diff[0] != "apps/payments/checkout/base/rollout.yaml 생성" {
		t.Fatalf("expected rollout manifest preview, got %q", diff[0])
	}
	if diff[1] != "apps/payments/checkout/overlays/prod 생성" {
		t.Fatalf("expected overlay preview, got %q", diff[1])
	}
}

func TestResolveSummaryAndOptionalHelpers(t *testing.T) {
	t.Parallel()

	if got := resolveSummary(Request{
		Operation: OperationCreateApplication,
		Name:      "checkout",
	}); got != "애플리케이션 checkout 생성" {
		t.Fatalf("unexpected create summary: %q", got)
	}

	if got := resolveSummary(Request{
		Operation: OperationUpdatePolicies,
		Summary:   "  직접 입력한 요약  ",
	}); got != "직접 입력한 요약" {
		t.Fatalf("expected trimmed explicit summary, got %q", got)
	}

	if value := optionalString("  hello "); value == nil || *value != "hello" {
		t.Fatalf("expected trimmed optional string, got %#v", value)
	}
	if value := optionalString("   "); value != nil {
		t.Fatalf("expected nil optional string, got %#v", value)
	}
	if value := optionalInt(3); value == nil || *value != 3 {
		t.Fatalf("expected optional int 3, got %#v", value)
	}
	if value := optionalInt(0); value != nil {
		t.Fatalf("expected nil optional int, got %#v", value)
	}
	if value := optionalStrategy(application.DeploymentStrategyCanary); value == nil || *value != application.DeploymentStrategyCanary {
		t.Fatalf("expected optional strategy Canary, got %#v", value)
	}
	if value := optionalStrategy(""); value != nil {
		t.Fatalf("expected nil optional strategy, got %#v", value)
	}
}
