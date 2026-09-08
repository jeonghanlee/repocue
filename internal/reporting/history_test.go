package reporting

import (
	"testing"
	"time"

	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/jeonghanlee/repocue/internal/storage"
)

func TestHistoryWindowEpochMaintenanceAndLegacyCoverage(t *testing.T) {
	cutoff := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	window := Window{Start: cutoff.Add(-24 * time.Hour), PreviousStart: cutoff.Add(-48 * time.Hour), Cutoff: cutoff, Days: 1}
	epoch := model.Epoch{ID: "epoch-1", CreatedAt: cutoff.Add(-10 * 24 * time.Hour)}
	state := model.CurrentState{Epoch: epoch}
	history := storage.History{Epochs: []model.Epoch{epoch}, Operations: []storage.OperationRun{
		{ID: "baseline", EpochID: epoch.ID, Operation: "baseline", CreatedAt: epoch.CreatedAt, DurationMS: 100, ScanDurationMS: 80, GitCommands: 5, BytesScanned: 1000},
		{ID: "prior", EpochID: epoch.ID, Operation: "refresh", CreatedAt: window.PreviousStart, DurationMS: 20, ScanDurationMS: 10, GitCommands: 2},
		{ID: "current", EpochID: epoch.ID, Operation: "refresh", CreatedAt: window.Start, DurationMS: 10, ScanDurationMS: 0, GitCommands: 0, BytesScanned: 0},
		{ID: "at-cutoff", EpochID: epoch.ID, Operation: "refresh", CreatedAt: cutoff, DurationMS: 999, ScanDurationMS: 999, GitCommands: 999},
		{ID: "another-epoch", EpochID: "epoch-2", Operation: "rebaseline", CreatedAt: window.Start, DurationMS: 999},
	}}
	m := maintenance(history, &state, cutoff)
	if *m.Total["execution_duration_ms"].Value != 130 || len(m.OperationIDs) != 3 || *m.Total["bytes_scanned"].Value != 1000 {
		t.Fatalf("wrong full epoch maintenance: %+v", m)
	}
	if m.Total["git_invocations"].Value != nil || m.Total["scan_duration_ms"].Value != nil || m.Refresh["git_invocations"].Count != 1 || m.Refresh["git_invocations"].Unknown != 1 {
		t.Fatal("legacy zero filled missing coverage")
	}
	activity := activity(history, window)[0]
	if *activity.Operations["refresh"]["execution_duration_ms"].Percent.Value != -50 {
		t.Fatal("equal-window trend is incorrect")
	}
	if activity.Operations["refresh"]["git_invocations"].Percent.Value != nil || *activity.Operations["refresh"]["bytes_scanned"].Current.Total != 0 {
		t.Fatal("per-dimension zero semantics lost")
	}
	if *activity.Cues["count"].Current.Total != 0 || activity.Cues["count"].Percent.Value != nil {
		t.Fatal("zero activity invented a trend")
	}
	history.Operations = history.Operations[1:]
	m = maintenance(history, &state, cutoff)
	if m.Total["execution_duration_ms"].Value != nil || m.Total["git_invocations"].Value != nil {
		t.Fatal("missing pre-window baseline was treated as free")
	}
}

func TestHistoryPredatingEpochIsIncompleteNotZero(t *testing.T) {
	cutoff := time.Now().UTC()
	history := storage.History{Epochs: []model.Epoch{{ID: "new", CreatedAt: cutoff.Add(-time.Hour)}}}
	result := activity(history, Window{Start: cutoff.Add(-24 * time.Hour), PreviousStart: cutoff.Add(-48 * time.Hour), Cutoff: cutoff})
	for _, trend := range result[0].Cues {
		if trend.Current.Total != nil || trend.Previous.Total != nil || trend.Percent.Value != nil || trend.Current.Coverage != "partial-history" {
			t.Fatal("history before creation became zero activity")
		}
	}
}
