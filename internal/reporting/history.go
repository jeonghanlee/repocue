package reporting

import (
	"sort"
	"time"

	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/jeonghanlee/repocue/internal/storage"
)

type Window struct {
	Start         time.Time `json:"start"`
	Cutoff        time.Time `json:"cutoff"`
	PreviousStart time.Time `json:"previous_start"`
	Days          int64     `json:"days"`
}

type Trend struct {
	Current  Statistic   `json:"current"`
	Previous Statistic   `json:"previous"`
	Percent  Measurement `json:"percent"`
}

type EpochActivity struct {
	Epoch      string                      `json:"epoch"`
	ObservedAt time.Time                   `json:"observed_at"`
	Operations map[string]map[string]Trend `json:"operations"`
	Cues       map[string]Trend            `json:"cues"`
}

type Maintenance struct {
	Epoch        string                 `json:"epoch"`
	Cutoff       time.Time              `json:"cutoff"`
	OperationIDs []string               `json:"operation_ids"`
	Baseline     map[string]Statistic   `json:"baseline"`
	Refresh      map[string]Statistic   `json:"refresh"`
	Total        map[string]Measurement `json:"total"`
	Limitations  []string               `json:"limitations"`
}

func operationValues(run storage.OperationRun) map[string]Measurement {
	values := map[string]Measurement{
		"count":                 measured("derived", 1, "operations", "recorded operation rows"),
		"execution_duration_ms": measured("observed", run.DurationMS, "ms", "recorded operation duration, excluding process startup and failed operations"),
		"scan_duration_ms":      measured("observed", run.ScanDurationMS, "ms", "scan portion already included in operation duration"),
		"git_invocations":       measured("observed", float64(run.GitCommands), "Git processes", "core scan Git invocations"),
		"files_scanned":         measured("observed", float64(run.FilesScanned), "files", "core tracked-file scan"),
		"bytes_scanned":         measured("observed", float64(run.BytesScanned), "scan bytes", "core tracked content, not physical filesystem I/O"),
	}
	if run.ScanDurationMS == 0 {
		values["scan_duration_ms"] = missing("ms", "stored scan_duration_ms", "legacy zero has no independent measurement provenance")
	}
	if run.GitCommands == 0 {
		values["git_invocations"] = missing("Git processes", "stored git_commands", "legacy zero has no independent measurement provenance")
	}
	return values
}

func cueValues(run storage.CueRun) map[string]Measurement {
	return map[string]Measurement{
		"count":            measured("derived", 1, "cue runs", "recorded cue metrics, not observed consumers"),
		"output_bytes":     measured("observed", float64(run.OutputBytes), "bytes", "recorded cue JSON bytes"),
		"estimated_tokens": measured("estimated", float64(run.EstimatedTokens), "estimated tokens", "recorded runtime token estimates"),
	}
}

func activity(history storage.History, window Window) []EpochActivity {
	result := []EpochActivity{}
	snapshotEpoch := map[string]string{}
	for _, snapshot := range history.Snapshots {
		snapshotEpoch[snapshot.ID] = snapshot.EpochID
	}
	for _, epoch := range history.Epochs {
		if !epoch.CreatedAt.Before(window.Cutoff) {
			continue
		}
		item := EpochActivity{Epoch: epoch.ID, ObservedAt: epoch.CreatedAt, Operations: map[string]map[string]Trend{}, Cues: map[string]Trend{}}
		for _, kind := range []string{"baseline", "rebaseline", "refresh"} {
			current, previous := []map[string]Measurement{}, []map[string]Measurement{}
			for _, run := range history.Operations {
				if run.EpochID != epoch.ID || run.Operation != kind {
					continue
				}
				if inWindow(run.CreatedAt, window.Start, window.Cutoff) {
					current = append(current, operationValues(run))
				}
				if inWindow(run.CreatedAt, window.PreviousStart, window.Start) {
					previous = append(previous, operationValues(run))
				}
			}
			item.Operations[kind] = trends(current, previous, operationValues(storage.OperationRun{}), !epoch.CreatedAt.After(window.Start), !epoch.CreatedAt.After(window.PreviousStart))
		}
		current, previous := []map[string]Measurement{}, []map[string]Measurement{}
		for _, run := range history.Cues {
			if snapshotEpoch[run.SnapshotID] != epoch.ID {
				continue
			}
			if inWindow(run.CreatedAt, window.Start, window.Cutoff) {
				current = append(current, cueValues(run))
			}
			if inWindow(run.CreatedAt, window.PreviousStart, window.Start) {
				previous = append(previous, cueValues(run))
			}
		}
		item.Cues = trends(current, previous, cueValues(storage.CueRun{}), !epoch.CreatedAt.After(window.Start), !epoch.CreatedAt.After(window.PreviousStart))
		result = append(result, item)
	}
	return result
}

func inWindow(value, start, end time.Time) bool { return !value.Before(start) && value.Before(end) }

func aggregateRows(rows []map[string]Measurement, name string, template Measurement, complete bool) Statistic {
	values := []float64{}
	unknown := 0
	for _, row := range rows {
		item, found := row[name]
		if !found || item.Value == nil {
			unknown++
			continue
		}
		values = append(values, *item.Value)
	}
	result := summarize(values, unknown, "derived", template.Unit, template.Basis)
	if !complete {
		result.Coverage = "partial-history"
		result.Total = nil
	}
	if len(rows) == 0 && complete {
		result.Status = "derived"
		result.Total = number(0)
	}
	return result
}

func trends(current, previous []map[string]Measurement, templates map[string]Measurement, currentComplete, previousComplete bool) map[string]Trend {
	result := map[string]Trend{}
	for name, template := range templates {
		left := aggregateRows(current, name, template, currentComplete)
		right := aggregateRows(previous, name, template, previousComplete)
		percent := missing("percent", "equal-window totals", "incomplete history, incomplete metric coverage, or zero prior activity")
		if left.Total != nil && right.Total != nil && *right.Total > 0 {
			percent = measured("derived", 100*(*left.Total-*right.Total) / *right.Total, "percent", "current minus previous equal-window recorded activity")
		}
		result[name] = Trend{Current: left, Previous: right, Percent: percent}
	}
	return result
}

func maintenance(history storage.History, selected *model.CurrentState, cutoff time.Time) Maintenance {
	result := Maintenance{Cutoff: cutoff, OperationIDs: []string{}, Baseline: map[string]Statistic{}, Refresh: map[string]Statistic{}, Total: map[string]Measurement{},
		Limitations: []string{"H includes recorded epoch baseline and refresh rows before cutoff, even before the activity window", "recording time is not SQLite publication time", "unrecorded failures and maintenance process startup are excluded", "cue freshness work is in Q, not added to H", "calibration experiment setup is separate from local maintenance"}}
	baseline, refresh := []map[string]Measurement{}, []map[string]Measurement{}
	if selected != nil {
		result.Epoch = selected.Epoch.ID
		for _, run := range history.Operations {
			if run.EpochID != result.Epoch || !run.CreatedAt.Before(cutoff) {
				continue
			}
			if run.Operation == "baseline" || run.Operation == "rebaseline" {
				baseline = append(baseline, operationValues(run))
			} else if run.Operation == "refresh" {
				refresh = append(refresh, operationValues(run))
			} else {
				continue
			}
			result.OperationIDs = append(result.OperationIDs, run.ID)
		}
	}
	all := append(append([]map[string]Measurement{}, baseline...), refresh...)
	for name, template := range operationValues(storage.OperationRun{}) {
		result.Baseline[name] = aggregateRows(baseline, name, template, len(baseline) > 0)
		result.Refresh[name] = aggregateRows(refresh, name, template, true)
		stat := aggregateRows(all, name, template, len(baseline) > 0)
		value := missing(template.Unit, "full recorded epoch maintenance", "missing baseline or incomplete metric coverage")
		if stat.Total != nil {
			value = measured("derived", *stat.Total, template.Unit, "full recorded epoch maintenance before cutoff")
		}
		result.Total[name] = value
	}
	for _, name := range agentMetricNames() {
		unit, _ := metricBoundary(name)
		if stringsContainsTokens(name) {
			result.Total[name] = measured("derived", 0, unit, "deterministic RepoCue core invokes no LLM")
		}
	}
	sort.Strings(result.OperationIDs)
	return result
}

func stringsContainsTokens(name string) bool {
	switch name {
	case "input_tokens", "cached_input_tokens", "output_tokens", "reasoning_output_tokens", "total_tokens":
		return true
	}
	return false
}
