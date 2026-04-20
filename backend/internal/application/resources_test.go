package application

import "testing"

func TestNormalizeResourceRequirementsAppliesDefaults(t *testing.T) {
	t.Parallel()

	normalized, err := normalizeResourceRequirements(ResourceRequirements{}, true)
	if err != nil {
		t.Fatalf("normalize resource requirements: %v", err)
	}
	if normalized.Requests.CPU != defaultResourceRequestCPU {
		t.Fatalf("expected default request cpu %q, got %q", defaultResourceRequestCPU, normalized.Requests.CPU)
	}
	if normalized.Requests.Memory != defaultResourceRequestMemory {
		t.Fatalf("expected default request memory %q, got %q", defaultResourceRequestMemory, normalized.Requests.Memory)
	}
	if normalized.Limits.CPU != defaultResourceLimitCPU {
		t.Fatalf("expected default limit cpu %q, got %q", defaultResourceLimitCPU, normalized.Limits.CPU)
	}
	if normalized.Limits.Memory != defaultResourceLimitMemory {
		t.Fatalf("expected default limit memory %q, got %q", defaultResourceLimitMemory, normalized.Limits.Memory)
	}
}

func TestValidateResourceRequirementsRejectsLimitBelowRequest(t *testing.T) {
	t.Parallel()

	err := validateResourceRequirements(ResourceRequirements{
		Requests: ResourceQuantity{CPU: "500m", Memory: "512Mi"},
		Limits:   ResourceQuantity{CPU: "250m", Memory: "256Mi"},
	})
	if err == nil {
		t.Fatal("expected invalid resource requirements")
	}
}
