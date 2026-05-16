package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/aolda/aods-backend/internal/project"
)

type catalogCacheStub struct {
	listRecords []Record
	listOK      bool
	listErr     error
	replaced    []Record
	invalidated []string
}

func (s *catalogCacheStub) Ensure(context.Context) error {
	return nil
}

func (s *catalogCacheStub) ListApplications(context.Context, string, time.Duration) ([]Record, bool, error) {
	return s.listRecords, s.listOK, s.listErr
}

func (s *catalogCacheStub) ReplaceProjectApplications(_ context.Context, _ string, records []Record) error {
	s.replaced = append([]Record(nil), records...)
	return nil
}

func (s *catalogCacheStub) InvalidateProject(_ context.Context, projectID string) error {
	s.invalidated = append(s.invalidated, projectID)
	return nil
}

type catalogSourceSpy struct {
	stubStore
	listCalls int
}

func (s *catalogSourceSpy) ListApplications(ctx context.Context, projectID string) ([]Record, error) {
	s.listCalls++
	return s.stubStore.ListApplications(ctx, projectID)
}

type catalogDelegateStore struct {
	records         []Record
	listErr         error
	cleanupCalls    int
	appendEventSeen bool
}

func (s *catalogDelegateStore) ListApplications(context.Context, string) ([]Record, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.records, nil
}

func (s *catalogDelegateStore) GetApplication(context.Context, string) (Record, error) {
	return Record{ID: "shared__api", ProjectID: "shared", Name: "api"}, nil
}

func (s *catalogDelegateStore) CreateApplication(context.Context, ProjectContext, CreateRequest, string) (Record, error) {
	return Record{ID: "shared__api", ProjectID: "shared", Name: "api"}, nil
}

func (s *catalogDelegateStore) ArchiveApplication(context.Context, string, string) (ApplicationLifecycleResponse, error) {
	return ApplicationLifecycleResponse{ApplicationID: "shared__api", ProjectID: "shared", Name: "api", Status: "Archived"}, nil
}

func (s *catalogDelegateStore) DeleteApplication(context.Context, string) (ApplicationLifecycleResponse, error) {
	return ApplicationLifecycleResponse{ApplicationID: "shared__api", ProjectID: "shared", Name: "api", Status: "Deleted"}, nil
}

func (s *catalogDelegateStore) UpdateApplicationImage(context.Context, ProjectContext, string, string, string) (Record, error) {
	return Record{ID: "shared__api", ProjectID: "shared", Name: "api", Image: "repo/api:v2"}, nil
}

func (s *catalogDelegateStore) PatchApplication(context.Context, ProjectContext, string, UpdateApplicationRequest) (Record, error) {
	return Record{ID: "shared__api", ProjectID: "shared", Name: "api", Description: "patched"}, nil
}

func (s *catalogDelegateStore) SaveApplicationSecretPath(context.Context, ProjectContext, string, string) (Record, error) {
	return Record{ID: "shared__api", ProjectID: "shared", Name: "api", SecretPath: "secret/aods/apps/shared/api/prod"}, nil
}

func (s *catalogDelegateStore) ListDeployments(context.Context, string) ([]DeploymentRecord, error) {
	return []DeploymentRecord{{DeploymentID: "dep_1", ApplicationID: "shared__api"}}, nil
}

func (s *catalogDelegateStore) GetDeployment(context.Context, string, string) (DeploymentRecord, error) {
	return DeploymentRecord{DeploymentID: "dep_1", ApplicationID: "shared__api"}, nil
}

func (s *catalogDelegateStore) UpdateDeployment(_ context.Context, _ string, deployment DeploymentRecord) (DeploymentRecord, error) {
	return deployment, nil
}

func (s *catalogDelegateStore) GetRollbackPolicy(context.Context, string) (RollbackPolicy, error) {
	return RollbackPolicy{Enabled: true}, nil
}

func (s *catalogDelegateStore) SaveRollbackPolicy(_ context.Context, _ string, policy RollbackPolicy) (RollbackPolicy, error) {
	return policy, nil
}

func (s *catalogDelegateStore) ListEvents(context.Context, string) ([]Event, error) {
	return []Event{{ID: "evt_1", Type: "Test"}}, nil
}

func (s *catalogDelegateStore) AppendEvent(context.Context, string, Event) error {
	s.appendEventSeen = true
	return nil
}

func (s *catalogDelegateStore) CleanupOrphanFluxManifests(context.Context) (int, error) {
	s.cleanupCalls++
	return 2, nil
}

func TestCachedManifestStoreListApplicationsUsesFreshCache(t *testing.T) {
	cache := &catalogCacheStub{
		listOK: true,
		listRecords: []Record{
			{ID: "shared__api", ProjectID: "shared", Name: "api"},
		},
	}
	source := &catalogSourceSpy{
		stubStore: stubStore{records: []Record{{ID: "shared__web", ProjectID: "shared", Name: "web"}}},
	}
	store := CachedManifestStore{Source: source, Cache: cache, Freshness: time.Minute}

	records, err := store.ListApplications(context.Background(), "shared")
	if err != nil {
		t.Fatalf("list applications: %v", err)
	}
	if len(records) != 1 || records[0].ID != "shared__api" {
		t.Fatalf("expected cached application, got %#v", records)
	}
	if source.listCalls != 0 {
		t.Fatalf("expected source not to be called, got %d calls", source.listCalls)
	}
}

func TestCachedManifestStoreRefreshesCacheOnMiss(t *testing.T) {
	cache := &catalogCacheStub{}
	source := &catalogSourceSpy{
		stubStore: stubStore{records: []Record{{ID: "shared__web", ProjectID: "shared", Name: "web"}}},
	}
	store := CachedManifestStore{Source: source, Cache: cache, Freshness: time.Minute}

	records, err := store.ListApplications(context.Background(), "shared")
	if err != nil {
		t.Fatalf("list applications: %v", err)
	}
	if len(records) != 1 || records[0].ID != "shared__web" {
		t.Fatalf("expected source application, got %#v", records)
	}
	if source.listCalls != 1 {
		t.Fatalf("expected source to be called once, got %d", source.listCalls)
	}
	if len(cache.replaced) != 1 || cache.replaced[0].ID != "shared__web" {
		t.Fatalf("expected cache replacement, got %#v", cache.replaced)
	}
}

func TestCachedManifestStoreGetApplicationUsesFreshCache(t *testing.T) {
	cache := &catalogCacheStub{
		listOK: true,
		listRecords: []Record{
			{ID: "shared__api", ProjectID: "shared", Name: "api"},
		},
	}
	source := &catalogSourceSpy{
		stubStore: stubStore{records: []Record{{ID: "shared__web", ProjectID: "shared", Name: "web"}}},
	}
	store := CachedManifestStore{Source: source, Cache: cache, Freshness: time.Minute}

	record, err := store.GetApplication(context.Background(), "shared__api")
	if err != nil {
		t.Fatalf("get application: %v", err)
	}
	if record.ID != "shared__api" {
		t.Fatalf("expected cached application, got %#v", record)
	}
	if source.listCalls != 0 {
		t.Fatalf("expected source not to be listed, got %d calls", source.listCalls)
	}
}

func TestCachedManifestStoreFallsBackWhenCacheMissesApplication(t *testing.T) {
	cache := &catalogCacheStub{
		listOK: true,
		listRecords: []Record{
			{ID: "shared__other", ProjectID: "shared", Name: "other"},
		},
	}
	source := stubStore{records: []Record{{ID: "shared__api", ProjectID: "shared", Name: "api"}}}
	store := CachedManifestStore{Source: source, Cache: cache, Freshness: time.Minute}

	record, err := store.GetApplication(context.Background(), "shared__api")
	if err != nil {
		t.Fatalf("get application: %v", err)
	}
	if record.ID != "shared__api" {
		t.Fatalf("expected source application, got %#v", record)
	}
}

func TestCachedManifestStoreRequiresSource(t *testing.T) {
	store := CachedManifestStore{}

	if _, err := store.ListApplications(context.Background(), "shared"); err == nil {
		t.Fatal("expected list to fail without source")
	}
	if _, err := store.GetApplication(context.Background(), "shared__api"); err == nil {
		t.Fatal("expected get to fail without source")
	}
	if _, err := store.CreateApplication(context.Background(), ProjectContext{ID: "shared"}, CreateRequest{Name: "api"}, ""); err == nil {
		t.Fatal("expected create to fail without source")
	}
	if _, err := store.CleanupOrphanFluxManifests(context.Background()); err == nil {
		t.Fatal("expected cleanup to fail without source support")
	}
}

func TestCachedManifestStoreDelegatesOperationsAndRefreshesAfterWrites(t *testing.T) {
	cache := &catalogCacheStub{}
	source := &catalogDelegateStore{
		records: []Record{{ID: "shared__api", ProjectID: "shared", Name: "api"}},
	}
	store := CachedManifestStore{Source: source, Cache: cache, Freshness: time.Minute}
	ctx := context.Background()
	projectCtx := ProjectContext{ID: "shared", Namespace: "shared"}

	if record, err := store.GetApplication(ctx, "shared__api"); err != nil || record.ID != "shared__api" {
		t.Fatalf("get application = %#v err=%v", record, err)
	}
	if record, err := store.CreateApplication(ctx, projectCtx, CreateRequest{Name: "api"}, ""); err != nil || record.ProjectID != "shared" {
		t.Fatalf("create application = %#v err=%v", record, err)
	}
	if response, err := store.ArchiveApplication(ctx, "shared__api", "tester"); err != nil || response.ProjectID != "shared" {
		t.Fatalf("archive application = %#v err=%v", response, err)
	}
	if response, err := store.DeleteApplication(ctx, "shared__api"); err != nil || response.ProjectID != "shared" {
		t.Fatalf("delete application = %#v err=%v", response, err)
	}
	if record, err := store.UpdateApplicationImage(ctx, projectCtx, "shared__api", "v2", "dep_1"); err != nil || record.Image == "" {
		t.Fatalf("update image = %#v err=%v", record, err)
	}
	if record, err := store.PatchApplication(ctx, projectCtx, "shared__api", UpdateApplicationRequest{}); err != nil || record.Description != "patched" {
		t.Fatalf("patch application = %#v err=%v", record, err)
	}
	if record, err := store.SaveApplicationSecretPath(ctx, projectCtx, "shared__api", "secret"); err != nil || record.SecretPath == "" {
		t.Fatalf("save secret path = %#v err=%v", record, err)
	}
	if deployments, err := store.ListDeployments(ctx, "shared__api"); err != nil || len(deployments) != 1 {
		t.Fatalf("list deployments = %#v err=%v", deployments, err)
	}
	if deployment, err := store.GetDeployment(ctx, "shared__api", "dep_1"); err != nil || deployment.DeploymentID != "dep_1" {
		t.Fatalf("get deployment = %#v err=%v", deployment, err)
	}
	if deployment, err := store.UpdateDeployment(ctx, "shared__api", DeploymentRecord{DeploymentID: "dep_2"}); err != nil || deployment.DeploymentID != "dep_2" {
		t.Fatalf("update deployment = %#v err=%v", deployment, err)
	}
	if policy, err := store.GetRollbackPolicy(ctx, "shared__api"); err != nil || !policy.Enabled {
		t.Fatalf("get rollback policy = %#v err=%v", policy, err)
	}
	if policy, err := store.SaveRollbackPolicy(ctx, "shared__api", RollbackPolicy{Enabled: true}); err != nil || !policy.Enabled {
		t.Fatalf("save rollback policy = %#v err=%v", policy, err)
	}
	if events, err := store.ListEvents(ctx, "shared__api"); err != nil || len(events) != 1 {
		t.Fatalf("list events = %#v err=%v", events, err)
	}
	if err := store.AppendEvent(ctx, "shared__api", Event{ID: "evt_2"}); err != nil || !source.appendEventSeen {
		t.Fatalf("append event err=%v seen=%v", err, source.appendEventSeen)
	}
	if count, err := store.CleanupOrphanFluxManifests(ctx); err != nil || count != 2 || source.cleanupCalls != 1 {
		t.Fatalf("cleanup count=%d calls=%d err=%v", count, source.cleanupCalls, err)
	}
	if len(cache.replaced) != 1 || cache.replaced[0].ID != "shared__api" {
		t.Fatalf("expected cache refresh after writes, got %#v", cache.replaced)
	}
}

func TestCachedManifestStoreInvalidatesWhenRefreshAfterWriteFails(t *testing.T) {
	cache := &catalogCacheStub{}
	source := &catalogDelegateStore{listErr: fmt.Errorf("git unavailable")}
	store := CachedManifestStore{Source: source, Cache: cache, Freshness: time.Minute}

	if _, err := store.CreateApplication(context.Background(), ProjectContext{ID: "shared"}, CreateRequest{Name: "api"}, ""); err != nil {
		t.Fatalf("create application: %v", err)
	}
	if len(cache.invalidated) != 1 || cache.invalidated[0] != "shared" {
		t.Fatalf("expected cache invalidation, got %#v", cache.invalidated)
	}
}

func TestApplicationCatalogProjectorRefreshesAllProjects(t *testing.T) {
	source := &catalogSourceSpy{
		stubStore: stubStore{records: []Record{{ID: "shared__api", ProjectID: "shared", Name: "api"}}},
	}
	projectSource := &catalogProjectSourceStub{
		items: []project.CatalogProject{{ID: "shared"}, {ID: ""}},
	}
	projector := ApplicationCatalogProjector{Store: CachedManifestStore{Source: source}, Projects: projectSource}

	projector.refreshAll(context.Background())
	if source.listCalls != 1 {
		t.Fatalf("expected one project refresh, got %d", source.listCalls)
	}
}

func TestApplicationCatalogProjectorStartNoopsWhenUnconfigured(t *testing.T) {
	var nilProjector *ApplicationCatalogProjector
	nilProjector.Start(context.Background())

	emptyProjector := ApplicationCatalogProjector{}
	emptyProjector.Start(context.Background())

	projector := ApplicationCatalogProjector{
		Store:    CachedManifestStore{Source: stubStore{}},
		Projects: &catalogProjectSourceStub{items: []project.CatalogProject{{ID: "shared"}}},
	}
	projector.Start(context.Background())
}

func TestApplicationCatalogProjectorStartRefreshesAndStops(t *testing.T) {
	source := &catalogSourceSpy{
		stubStore: stubStore{records: []Record{{ID: "shared__api", ProjectID: "shared", Name: "api"}}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	projector := ApplicationCatalogProjector{
		Store:    CachedManifestStore{Source: source},
		Projects: &catalogProjectSourceStub{items: []project.CatalogProject{{ID: "shared"}}},
		Interval: time.Hour,
	}
	projector.Start(ctx)

	if source.listCalls != 1 {
		t.Fatalf("expected startup refresh before stop, got %d", source.listCalls)
	}
}

func TestMariaDBApplicationCatalogCacheNilDatabaseErrors(t *testing.T) {
	store := MariaDBApplicationCatalogCache{}
	ctx := context.Background()
	if err := store.Ensure(ctx); err == nil {
		t.Fatal("expected ensure to fail without database")
	}
	if _, ok, err := store.ListApplications(ctx, "shared", time.Minute); err == nil || ok {
		t.Fatalf("expected list to fail without database, ok=%v err=%v", ok, err)
	}
	if err := store.ReplaceProjectApplications(ctx, "shared", nil); err == nil {
		t.Fatal("expected replace to fail without database")
	}
	if err := store.InvalidateProject(ctx, "shared"); err == nil {
		t.Fatal("expected invalidate to fail without database")
	}
}

func TestMariaDBApplicationCatalogCacheEnsureCreatesTables(t *testing.T) {
	db, mock, cleanup := newApplicationCatalogSQLMock(t)
	defer cleanup()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS aods_application_catalog_snapshots").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS aods_application_catalog_records").
		WillReturnResult(sqlmock.NewResult(0, 0))

	store := MariaDBApplicationCatalogCache{DB: db}
	if err := store.Ensure(context.Background()); err != nil {
		t.Fatalf("ensure catalog cache: %v", err)
	}
	assertApplicationCatalogSQLExpectations(t, mock)
}

func TestMariaDBApplicationCatalogCacheListsFreshSnapshot(t *testing.T) {
	db, mock, cleanup := newApplicationCatalogSQLMock(t)
	defer cleanup()

	mock.ExpectQuery("SELECT updated_at, record_count").
		WithArgs("shared").
		WillReturnRows(sqlmock.NewRows([]string{"updated_at", "record_count"}).AddRow(timeNowUTC(), 1))
	mock.ExpectQuery("SELECT record_json").
		WithArgs("shared").
		WillReturnRows(sqlmock.NewRows([]string{"record_json"}).AddRow([]byte(`{"ID":"shared__api","ProjectID":"shared","Name":"api"}`)))

	store := MariaDBApplicationCatalogCache{DB: db}
	records, ok, err := store.ListApplications(context.Background(), "shared", time.Minute)
	if err != nil {
		t.Fatalf("list catalog cache: %v", err)
	}
	if !ok || len(records) != 1 || records[0].ID != "shared__api" {
		t.Fatalf("expected cached record, ok=%v records=%#v", ok, records)
	}
	assertApplicationCatalogSQLExpectations(t, mock)
}

func TestMariaDBApplicationCatalogCacheReplaceAndInvalidateProject(t *testing.T) {
	db, mock, cleanup := newApplicationCatalogSQLMock(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM aods_application_catalog_records WHERE project_id = ?`)).
		WithArgs("shared").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO aods_application_catalog_records").
		WithArgs("shared", "shared__api", "api", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO aods_application_catalog_snapshots").
		WithArgs("shared", 1, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM aods_application_catalog_snapshots WHERE project_id = ?`)).
		WithArgs("shared").
		WillReturnResult(sqlmock.NewResult(0, 1))

	store := MariaDBApplicationCatalogCache{DB: db}
	if err := store.ReplaceProjectApplications(context.Background(), "shared", []Record{{ID: "shared__api", ProjectID: "shared", Name: "api"}}); err != nil {
		t.Fatalf("replace mariadb catalog cache: %v", err)
	}
	if err := store.InvalidateProject(context.Background(), "shared"); err != nil {
		t.Fatalf("invalidate mariadb catalog cache: %v", err)
	}
	assertApplicationCatalogSQLExpectations(t, mock)
}

func TestPostgresApplicationCatalogCacheEnsureCreatesTables(t *testing.T) {
	db, mock, cleanup := newApplicationCatalogSQLMock(t)
	defer cleanup()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS aods_application_catalog_snapshots").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_aods_application_catalog_snapshots_updated").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS aods_application_catalog_records").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_aods_application_catalog_project_name").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_aods_application_catalog_updated").
		WillReturnResult(sqlmock.NewResult(0, 0))

	store := PostgresApplicationCatalogCache{DB: db}
	if err := store.Ensure(context.Background()); err != nil {
		t.Fatalf("ensure postgres catalog cache: %v", err)
	}
	assertApplicationCatalogSQLExpectations(t, mock)
}

func TestPostgresApplicationCatalogCacheListsFreshSnapshot(t *testing.T) {
	db, mock, cleanup := newApplicationCatalogSQLMock(t)
	defer cleanup()

	mock.ExpectQuery("SELECT updated_at, record_count").
		WithArgs("shared").
		WillReturnRows(sqlmock.NewRows([]string{"updated_at", "record_count"}).AddRow(timeNowUTC(), 1))
	mock.ExpectQuery("SELECT record_json").
		WithArgs("shared").
		WillReturnRows(sqlmock.NewRows([]string{"record_json"}).AddRow([]byte(`{"ID":"shared__api","ProjectID":"shared","Name":"api"}`)))

	store := PostgresApplicationCatalogCache{DB: db}
	records, ok, err := store.ListApplications(context.Background(), "shared", time.Minute)
	if err != nil {
		t.Fatalf("list postgres catalog cache: %v", err)
	}
	if !ok || len(records) != 1 || records[0].ID != "shared__api" {
		t.Fatalf("expected cached record, ok=%v records=%#v", ok, records)
	}
	assertApplicationCatalogSQLExpectations(t, mock)
}

func TestPostgresApplicationCatalogCacheReplaceAndInvalidateProject(t *testing.T) {
	db, mock, cleanup := newApplicationCatalogSQLMock(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM aods_application_catalog_records WHERE project_id = $1`)).
		WithArgs("shared").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO aods_application_catalog_records").
		WithArgs("shared", "shared__api", "api", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO aods_application_catalog_snapshots").
		WithArgs("shared", 1, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM aods_application_catalog_snapshots WHERE project_id = $1`)).
		WithArgs("shared").
		WillReturnResult(sqlmock.NewResult(0, 1))

	store := PostgresApplicationCatalogCache{DB: db}
	if err := store.ReplaceProjectApplications(context.Background(), "shared", []Record{{ID: "shared__api", ProjectID: "shared", Name: "api"}}); err != nil {
		t.Fatalf("replace postgres catalog cache: %v", err)
	}
	if err := store.InvalidateProject(context.Background(), "shared"); err != nil {
		t.Fatalf("invalidate postgres catalog cache: %v", err)
	}
	assertApplicationCatalogSQLExpectations(t, mock)
}

type catalogProjectSourceStub struct {
	items []project.CatalogProject
	err   error
}

func (s *catalogProjectSourceStub) ListProjects(context.Context) ([]project.CatalogProject, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.items, nil
}

func newApplicationCatalogSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	return db, mock, func() {
		_ = db.Close()
	}
}

func assertApplicationCatalogSQLExpectations(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil && !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
