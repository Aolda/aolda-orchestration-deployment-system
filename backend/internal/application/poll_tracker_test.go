package application

import (
	"errors"
	"testing"
	"time"

	"github.com/aolda/aods-backend/internal/project"
)

var errTestPollFailure = errors.New("repository poll failed")

func TestRepositoryPollTrackerSnapshotDefaultsForRepositoryBackedApp(t *testing.T) {
	t.Parallel()

	tracker := NewRepositoryPollTracker(5 * time.Minute)
	record := Record{
		ID:            "project-a__demo",
		RepositoryURL: "https://github.com/Aolda/demo.git",
	}

	snapshot := tracker.Snapshot(record)
	if snapshot == nil {
		t.Fatal("expected repository-backed app to expose poll snapshot")
	}
	if !snapshot.Enabled {
		t.Fatal("expected repository poll to be enabled")
	}
	if snapshot.IntervalSeconds != 300 {
		t.Fatalf("expected 300 second interval, got %d", snapshot.IntervalSeconds)
	}
	if snapshot.LastResult != RepositoryPollResultPending {
		t.Fatalf("expected pending result before first poll, got %s", snapshot.LastResult)
	}
	if snapshot.NextScheduledAt == nil || snapshot.NextScheduledAt.IsZero() {
		t.Fatal("expected next scheduled poll time to be set")
	}
	if snapshot.Source != DefaultRepositoryConfigPath {
		t.Fatalf("expected default descriptor path, got %s", snapshot.Source)
	}
}

func TestRepositoryPollTrackerTracksSuccessAndFailure(t *testing.T) {
	t.Parallel()

	tracker := NewRepositoryPollTracker(5 * time.Minute)
	record := Record{
		ID:            "project-a__demo",
		RepositoryURL: "https://github.com/Aolda/demo.git",
		ConfigPath:    "deploy/aolda_deploy.json",
	}
	repo := project.Repository{
		ID:         "repo-1",
		Name:       "demo",
		ConfigFile: "deploy/aolda_deploy.json",
	}
	checkedAt := time.Date(2026, 4, 18, 4, 30, 0, 0, time.UTC)

	tracker.MarkAttempt(record, repo, checkedAt)
	tracker.MarkSuccess(record, repo, checkedAt)

	success := tracker.Snapshot(record)
	if success == nil {
		t.Fatal("expected snapshot after success")
	}
	if success.LastResult != RepositoryPollResultSuccess {
		t.Fatalf("expected success result, got %s", success.LastResult)
	}
	if success.LastCheckedAt == nil || !success.LastCheckedAt.Equal(checkedAt) {
		t.Fatalf("expected last checked at %s, got %v", checkedAt, success.LastCheckedAt)
	}
	if success.LastSucceededAt == nil || !success.LastSucceededAt.Equal(checkedAt) {
		t.Fatalf("expected last succeeded at %s, got %v", checkedAt, success.LastSucceededAt)
	}
	if success.Source != "deploy/aolda_deploy.json" {
		t.Fatalf("expected config source to be preserved, got %s", success.Source)
	}

	failedAt := checkedAt.Add(5 * time.Minute)
	tracker.MarkFailure(record, repo, failedAt, errTestPollFailure)

	failure := tracker.Snapshot(record)
	if failure == nil {
		t.Fatal("expected snapshot after failure")
	}
	if failure.LastResult != RepositoryPollResultError {
		t.Fatalf("expected error result, got %s", failure.LastResult)
	}
	if failure.LastError != errTestPollFailure.Error() {
		t.Fatalf("expected last error to be %q, got %q", errTestPollFailure.Error(), failure.LastError)
	}
	if failure.LastSucceededAt == nil || !failure.LastSucceededAt.Equal(checkedAt) {
		t.Fatalf("expected last successful check to stay at %s, got %v", checkedAt, failure.LastSucceededAt)
	}
	if failure.NextScheduledAt == nil || !failure.NextScheduledAt.Equal(failedAt.Add(5*time.Minute)) {
		t.Fatalf("expected next poll at %s, got %v", failedAt.Add(5*time.Minute), failure.NextScheduledAt)
	}
}

func TestRepositoryPollTrackerUsesApplicationSpecificInterval(t *testing.T) {
	t.Parallel()

	tracker := NewRepositoryPollTracker(5 * time.Minute)
	record := Record{
		ID:                            "project-a__demo",
		RepositoryURL:                 "https://github.com/Aolda/demo.git",
		RepositoryPollIntervalSeconds: 600,
	}

	snapshot := tracker.Snapshot(record)
	if snapshot == nil {
		t.Fatal("expected snapshot for repository-backed app")
	}
	if snapshot.IntervalSeconds != 600 {
		t.Fatalf("expected intervalSeconds=600, got %d", snapshot.IntervalSeconds)
	}
	if tracker.Due(record, time.Now().UTC().Add(9*time.Minute)) {
		t.Fatal("expected app-specific 10 minute poll not to be due at 9 minutes")
	}

	now := time.Now().UTC()
	tracker.Reschedule(record, now)
	rescheduled := tracker.Snapshot(record)
	if rescheduled == nil || rescheduled.NextScheduledAt == nil {
		t.Fatal("expected rescheduled poll metadata")
	}
	if !rescheduled.NextScheduledAt.Equal(now.Add(10 * time.Minute)) {
		t.Fatalf("expected next schedule at %s, got %v", now.Add(10*time.Minute), rescheduled.NextScheduledAt)
	}
}
