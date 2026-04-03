package application

import "testing"

func TestParseRepositoryDescriptorAndResolveService(t *testing.T) {
	t.Parallel()

	descriptor, err := parseRepositoryDescriptor([]byte(`{
		"services": [
			{"serviceId":"api","image":"ghcr.io/aolda/demo-api:v3","port":8080,"replicas":3},
			{"serviceId":"worker","image":"ghcr.io/aolda/demo-worker:v5","port":9090,"replicas":2}
		]
	}`))
	if err != nil {
		t.Fatalf("parse repository descriptor: %v", err)
	}

	service, ok := descriptor.resolveService(Record{
		Name:                "ignored",
		RepositoryServiceID: "worker",
	})
	if !ok {
		t.Fatal("expected to resolve repository service by repositoryServiceId")
	}
	if service.ServiceID != "worker" {
		t.Fatalf("expected worker service, got %s", service.ServiceID)
	}
	if service.Replicas != 2 {
		t.Fatalf("expected 2 replicas, got %d", service.Replicas)
	}
}

func TestParseRepositoryDescriptorRejectsInvalidReplicas(t *testing.T) {
	t.Parallel()

	_, err := parseRepositoryDescriptor([]byte(`{
		"services": [
			{"serviceId":"api","image":"ghcr.io/aolda/demo-api:v3","port":8080,"replicas":0}
		]
	}`))
	if err == nil {
		t.Fatal("expected invalid descriptor to fail")
	}
}

func TestRepositoryDescriptorResolvesSingleServiceWithoutExplicitID(t *testing.T) {
	t.Parallel()

	descriptor, err := parseRepositoryDescriptor([]byte(`{
		"services": [
			{"serviceId":"only","image":"ghcr.io/aolda/demo-only:v1","port":8080,"replicas":1}
		]
	}`))
	if err != nil {
		t.Fatalf("parse repository descriptor: %v", err)
	}

	service, ok := descriptor.resolveService(Record{Name: "different-name"})
	if !ok {
		t.Fatal("expected single-service descriptor to resolve without explicit id")
	}
	if service.ServiceID != "only" {
		t.Fatalf("expected only service, got %s", service.ServiceID)
	}
}

func TestImageRepositoryRefIgnoresTagAndDigest(t *testing.T) {
	t.Parallel()

	left := imageRepositoryRef("ghcr.io/aolda/service-api:v1")
	right := imageRepositoryRef("ghcr.io/aolda/service-api@sha256:abc123")
	if left != right {
		t.Fatalf("expected repository ref to ignore tag/digest, got %q and %q", left, right)
	}
}
