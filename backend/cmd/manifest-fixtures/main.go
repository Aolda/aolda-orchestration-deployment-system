package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/core"
	"github.com/aolda/aods-backend/internal/project"
)

var errUsage = errors.New("usage")

type staticProjectSource struct {
	items []project.CatalogProject
}

func (s staticProjectSource) ListProjects(context.Context) ([]project.CatalogProject, error) {
	return append([]project.CatalogProject(nil), s.items...), nil
}

type staticStatusReader struct{}

func (staticStatusReader) Read(context.Context, application.Record) (application.SyncInfo, error) {
	return application.SyncInfo{Status: application.SyncStatusSynced}, nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, errUsage) {
			fmt.Fprintln(os.Stderr, "usage: manifest-fixtures <output-dir>")
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return errUsage
	}

	outputDir := args[0]
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	catalog := []project.CatalogProject{
		{
			ID:          "project-a",
			Name:        "Project A",
			Description: "Fixture project for manifest validation",
			Namespace:   "project-a",
			Access: project.Access{
				DeployerGroups: []string{"aods:project-a:deploy"},
			},
			Environments: []project.Environment{
				{
					ID:        "dev",
					Name:      "Development",
					ClusterID: "edge",
					WriteMode: project.WriteModeDirect,
				},
				{
					ID:        "prod",
					Name:      "Production",
					ClusterID: "default",
					WriteMode: project.WriteModeDirect,
					Default:   true,
				},
			},
			Repositories: []project.Repository{
				{
					ID:         "payments-api",
					Name:       "Payments API",
					URL:        "https://github.com/aolda/payments-api",
					ConfigFile: "deploy/aods.yaml",
				},
				{
					ID:         "checkout-api",
					Name:       "Checkout API",
					URL:        "https://github.com/aolda/checkout-api",
					ConfigFile: "deploy/canary.yaml",
				},
			},
			Policies: project.PolicySet{
				MinReplicas:                 1,
				AllowedEnvironments:         []string{"dev", "prod"},
				AllowedDeploymentStrategies: []string{"Rollout", "Canary"},
				AllowedClusterTargets:       []string{"edge", "default"},
				RequiredProbes:              true,
			},
		},
	}

	if err := writeFixtureCatalog(outputDir); err != nil {
		return fmt.Errorf("write fixture catalog: %w", err)
	}

	service := application.Service{
		Projects: &project.Service{
			Source: staticProjectSource{items: catalog},
		},
		Store: application.LocalManifestStore{
			RepoRoot: outputDir,
		},
		StatusReader: staticStatusReader{},
	}

	user := core.User{
		Username: "fixture-validator",
		Groups:   []string{"aods:project-a:deploy"},
	}

	requests := []application.CreateRequest{
		{
			Name:                "rollout-app",
			Description:         "Standard rollout validation fixture",
			Image:               "ghcr.io/aolda/rollout-app:v1.2.3",
			ServicePort:         8080,
			DeploymentStrategy:  application.DeploymentStrategyStandard,
			Environment:         "prod",
			Secrets:             []application.SecretEntry{{Key: "DATABASE_URL", Value: "postgres://fixture"}},
			RepositoryID:        "payments-api",
			RepositoryServiceID: "payments",
			ConfigPath:          "deploy/aods.yaml",
		},
		{
			Name:                "canary-app",
			Description:         "Canary validation fixture",
			Image:               "ghcr.io/aolda/canary-app:v4.5.6",
			ServicePort:         9090,
			DeploymentStrategy:  application.DeploymentStrategyCanary,
			Environment:         "dev",
			RepositoryID:        "checkout-api",
			RepositoryServiceID: "checkout",
			ConfigPath:          "deploy/canary.yaml",
		},
	}

	for index, request := range requests {
		requestID := fmt.Sprintf("req_fixture_%02d", index+1)
		if _, err := service.CreateApplication(context.Background(), user, "project-a", request, requestID); err != nil {
			return fmt.Errorf("create fixture application %s: %w", request.Name, err)
		}
	}

	return nil
}

func writeFixtureCatalog(outputDir string) error {
	data := []byte(`projects:
  - id: project-a
    name: Project A
    description: Fixture project for manifest validation
    namespace: project-a
    access:
      deployerGroups:
        - aods:project-a:deploy
    environments:
      - id: dev
        name: Development
        clusterId: edge
        writeMode: direct
      - id: prod
        name: Production
        clusterId: default
        writeMode: direct
        default: true
    repositories:
      - id: payments-api
        name: Payments API
        url: https://github.com/aolda/payments-api
        configFile: deploy/aods.yaml
      - id: checkout-api
        name: Checkout API
        url: https://github.com/aolda/checkout-api
        configFile: deploy/canary.yaml
`)

	path := filepath.Join(outputDir, "platform", "projects.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
