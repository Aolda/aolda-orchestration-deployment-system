package application

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aolda/aods-backend/internal/project"
	"gopkg.in/yaml.v3"
)

type OrphanFluxManifestCleaner interface {
	CleanupOrphanFluxManifests(ctx context.Context) (int, error)
}

type OrphanFluxManifestCleanupWorker struct {
	Cleaner  OrphanFluxManifestCleaner
	Interval time.Duration
}

type fluxChildManifestMetadata struct {
	Annotations map[string]string `yaml:"annotations"`
}

type fluxChildManifest struct {
	Metadata fluxChildManifestMetadata `yaml:"metadata"`
}

func (w *OrphanFluxManifestCleanupWorker) Start(ctx context.Context) {
	if w == nil || w.Cleaner == nil || w.Interval <= 0 {
		return
	}

	slog.Info("starting orphan Flux manifest cleanup worker", "interval", w.Interval)

	w.runOnce(ctx)

	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *OrphanFluxManifestCleanupWorker) runOnce(ctx context.Context) {
	count, err := w.Cleaner.CleanupOrphanFluxManifests(ctx)
	if err != nil {
		slog.Error("orphan Flux manifest cleanup failed", "error", err)
		return
	}
	if count > 0 {
		slog.Info("orphan Flux manifests cleaned up", "count", count)
	}
}

func (s LocalManifestStore) CleanupOrphanFluxManifests(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	records, err := s.loadKnownApplicationRecords(ctx)
	if err != nil {
		return 0, err
	}

	expectedPaths, err := s.expectedFluxChildPaths(ctx, records)
	if err != nil {
		return 0, err
	}

	manifestPaths, err := s.listFluxChildManifestPaths(ctx)
	if err != nil {
		return 0, err
	}

	removedCount := 0
	touchedClusters := map[string]struct{}{}

	for _, manifestPath := range manifestPaths {
		if err := ctx.Err(); err != nil {
			return removedCount, err
		}

		applicationID, managed, err := s.readManagedFluxChildApplicationID(manifestPath)
		if err != nil {
			return removedCount, err
		}
		if !managed {
			continue
		}

		expectedPath, appExists := expectedPaths[applicationID]
		if appExists && expectedPath == filepath.Clean(manifestPath) {
			continue
		}
		if appExists {
			clusterID, err := s.clusterIDFromFluxChildPath(manifestPath)
			if err != nil {
				return removedCount, err
			}
			if err := os.Remove(manifestPath); err != nil && !os.IsNotExist(err) {
				return removedCount, fmt.Errorf("remove stale flux child manifest: %w", err)
			}
			touchedClusters[clusterID] = struct{}{}
			removedCount++
			continue
		}

		if _, ok := records[applicationID]; ok {
			// The application still exists on disk, but its project is no longer
			// resolvable from the catalog. Keep the manifest until the catalog is fixed.
			continue
		}

		clusterID, err := s.clusterIDFromFluxChildPath(manifestPath)
		if err != nil {
			return removedCount, err
		}
		if err := os.Remove(manifestPath); err != nil && !os.IsNotExist(err) {
			return removedCount, fmt.Errorf("remove orphan flux child manifest: %w", err)
		}
		touchedClusters[clusterID] = struct{}{}
		removedCount++
	}

	for _, clusterID := range sortedKeys(touchedClusters) {
		if err := s.rewriteFluxClusterRoot(clusterID); err != nil {
			return removedCount, err
		}
	}

	return removedCount, nil
}

func (s GitManifestStore) CleanupOrphanFluxManifests(ctx context.Context) (int, error) {
	if s.Repository == nil {
		return 0, fmt.Errorf("git manifest repository is not configured")
	}

	removedCount := 0
	err := s.Repository.WithWrite(ctx, "chore: cleanup orphan flux manifests", func(repoDir string) error {
		count, err := s.localStore(repoDir).CleanupOrphanFluxManifests(ctx)
		if err != nil {
			return err
		}
		removedCount = count
		return nil
	})
	if err != nil {
		return 0, err
	}

	return removedCount, nil
}

func (s LocalManifestStore) loadKnownApplicationRecords(ctx context.Context) (map[string]Record, error) {
	appsRoot := filepath.Join(s.RepoRoot, "apps")
	projectEntries, err := os.ReadDir(appsRoot)
	if os.IsNotExist(err) {
		return map[string]Record{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read applications root: %w", err)
	}

	records := make(map[string]Record)
	for _, projectEntry := range projectEntries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !projectEntry.IsDir() {
			continue
		}

		projectID := projectEntry.Name()
		appEntries, err := os.ReadDir(filepath.Join(appsRoot, projectID))
		if err != nil {
			return nil, fmt.Errorf("read project applications directory: %w", err)
		}

		for _, appEntry := range appEntries {
			if !appEntry.IsDir() {
				continue
			}

			record, err := s.loadRecord(projectID, appEntry.Name())
			if err == ErrNotFound {
				continue
			}
			if err != nil {
				return nil, err
			}
			records[record.ID] = record
		}
	}

	return records, nil
}

func (s LocalManifestStore) expectedFluxChildPaths(ctx context.Context, records map[string]Record) (map[string]string, error) {
	catalog := project.LocalCatalogSource{
		Path: filepath.Join(s.RepoRoot, "platform", "projects.yaml"),
	}
	projects, err := catalog.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	expected := make(map[string]string, len(records))
	for _, item := range projects {
		projectContext := buildProjectContext(item)
		for _, record := range records {
			if record.ProjectID != item.ID {
				continue
			}

			clusterID := projectContext.clusterIDForEnvironment(record.DefaultEnvironment)
			expected[record.ID] = filepath.Clean(filepath.Join(
				s.fluxClusterDir(clusterID),
				"applications",
				fluxChildFileName(record)+".yaml",
			))
		}
	}

	return expected, nil
}

func (s LocalManifestStore) listFluxChildManifestPaths(ctx context.Context) ([]string, error) {
	clustersRoot := filepath.Join(s.RepoRoot, "platform", "flux", "clusters")
	clusterEntries, err := os.ReadDir(clustersRoot)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read flux clusters directory: %w", err)
	}

	paths := make([]string, 0)
	for _, clusterEntry := range clusterEntries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !clusterEntry.IsDir() {
			continue
		}

		applicationsDir := filepath.Join(clustersRoot, clusterEntry.Name(), "applications")
		entries, err := os.ReadDir(applicationsDir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read flux applications directory: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
				continue
			}
			paths = append(paths, filepath.Clean(filepath.Join(applicationsDir, entry.Name())))
		}
	}

	sort.Strings(paths)
	return paths, nil
}

func (s LocalManifestStore) readManagedFluxChildApplicationID(manifestPath string) (string, bool, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", false, fmt.Errorf("read flux child manifest: %w", err)
	}

	var manifest fluxChildManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return "", false, nil
	}

	applicationID := strings.TrimSpace(manifest.Metadata.Annotations["aods.io/application-id"])
	if applicationID == "" {
		return "", false, nil
	}

	return applicationID, true, nil
}

func (s LocalManifestStore) clusterIDFromFluxChildPath(manifestPath string) (string, error) {
	applicationsDir := filepath.Dir(manifestPath)
	clusterDir := filepath.Dir(applicationsDir)
	clusterID := filepath.Base(clusterDir)
	if clusterID == "." || clusterID == string(filepath.Separator) || clusterID == "" {
		return "", fmt.Errorf("resolve cluster id from flux child path %s", manifestPath)
	}
	return clusterID, nil
}

func sortedKeys(values map[string]struct{}) []string {
	items := make([]string, 0, len(values))
	for value := range values {
		items = append(items, value)
	}
	sort.Strings(items)
	return items
}
