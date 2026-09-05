package evaluation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/jeonghanlee/repocue/internal/benchmark"
	"github.com/jeonghanlee/repocue/internal/cue"
	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/jeonghanlee/repocue/internal/repository"
	"github.com/jeonghanlee/repocue/internal/snapshot"
	"github.com/jeonghanlee/repocue/internal/storage"
)

const (
	SchemaVersion             = "repocue/evaluation-2"
	RunnerSchemaVersion       = "repocue/evaluation-runner-2"
	legacyBenchmarkVersion    = "repository-state-v1"
	legacyAnswerSchemaVersion = "repocue/benchmark-answer-1"
	measurementObserved       = "observed"
	measurementDerived        = "derived"
	measurementEstimated      = "estimated"
	measurementUnobserved     = "not_observed"
)

var consumerCounts = []int{1, 2, 3, 5, 10}

type Config struct {
	RepositoryPath string
	MaxTokens      int
	TaskFile       string
	DirectRunner   string
	AssistedRunner string
	TemporaryRoot  string
}

type Measurement struct {
	Status string   `json:"status"`
	Value  *float64 `json:"value,omitempty"`
	Unit   string   `json:"unit"`
}

type Repository struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Root         string  `json:"root"`
	Branch       *string `json:"branch"`
	Head         *string `json:"head"`
	Dirty        bool    `json:"dirty"`
	TrackedFiles int     `json:"tracked_files"`
	PresentFiles int     `json:"present_files"`
	TrackedBytes int64   `json:"tracked_bytes"`
}

type OperationCost struct {
	Kind              string  `json:"kind"`
	WallDurationMS    float64 `json:"wall_duration_ms"`
	ScanDurationMS    float64 `json:"scan_duration_ms"`
	FilesScanned      int     `json:"files_scanned"`
	BytesScanned      int64   `json:"bytes_scanned"`
	GitCommands       int     `json:"git_commands"`
	Changed           bool    `json:"changed"`
	DatabaseSizeBytes int64   `json:"database_size_bytes"`
	MaterializedRows  int     `json:"materialized_file_rows"`
}

type CueCost struct {
	View            string  `json:"view"`
	WallDurationMS  float64 `json:"wall_duration_ms"`
	OutputBytes     int     `json:"output_bytes"`
	EstimatedTokens int     `json:"estimated_tokens"`
	MaxTokens       int     `json:"max_tokens"`
}

type TokenizerCount struct {
	Tokenizer string `json:"tokenizer"`
	Tokens    int64  `json:"tokens"`
}

type ContextFinding struct {
	Category string `json:"category"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Evidence string `json:"evidence,omitempty"`
}

type RunnerMetadata struct {
	Adapter             string `json:"adapter"`
	AdapterVersion      string `json:"adapter_version"`
	CodexVersion        string `json:"codex_version,omitempty"`
	Model               string `json:"model,omitempty"`
	ReasoningEffort     string `json:"reasoning_effort,omitempty"`
	Sandbox             string `json:"sandbox,omitempty"`
	BenchmarkVersion    string `json:"benchmark_version,omitempty"`
	OutputSchemaVersion string `json:"output_schema_version,omitempty"`
	RepositoryHead      string `json:"repository_head,omitempty"`
	RepoCueSnapshot     string `json:"repocue_snapshot,omitempty"`
}

type CommandObservation struct {
	Command        string   `json:"command"`
	Classification string   `json:"classification"`
	FilesRead      []string `json:"files_read"`
}

type RunnerMetrics struct {
	InputTokens                 *int64            `json:"input_tokens,omitempty"`
	CachedInputTokens           *int64            `json:"cached_input_tokens,omitempty"`
	OutputTokens                *int64            `json:"output_tokens,omitempty"`
	ReasoningOutputTokens       *int64            `json:"reasoning_output_tokens,omitempty"`
	TotalTokens                 *int64            `json:"total_tokens,omitempty"`
	ExecutionDurationMS         *float64          `json:"execution_duration_ms,omitempty"`
	CommandExecutions           *int64            `json:"command_executions,omitempty"`
	RepositoryFilesRead         *int64            `json:"repository_files_read,omitempty"`
	RepositoryBytesRead         *int64            `json:"repository_bytes_read,omitempty"`
	GitCalls                    *int64            `json:"git_calls,omitempty"`
	FilesystemSearchCalls       *int64            `json:"filesystem_search_calls,omitempty"`
	ToolCalls                   *int64            `json:"tool_calls,omitempty"`
	TaskReadyLatencyMS          *float64          `json:"task_ready_latency_ms,omitempty"`
	FallbackRepositoryCommands  *int64            `json:"fallback_repository_commands,omitempty"`
	FallbackRepositoryFilesRead *int64            `json:"fallback_repository_files_read,omitempty"`
	FallbackRepositoryBytesRead *int64            `json:"fallback_repository_bytes_read,omitempty"`
	ContextCorrectness          *float64          `json:"context_correctness,omitempty"`
	ContextCompleteness         *float64          `json:"context_completeness,omitempty"`
	Statuses                    map[string]string `json:"statuses,omitempty"`
}

type RunnerObservation struct {
	SchemaVersion   string               `json:"schema_version"`
	Arm             string               `json:"arm"`
	Metadata        RunnerMetadata       `json:"metadata"`
	Metrics         RunnerMetrics        `json:"metrics"`
	TokenizerCounts []TokenizerCount     `json:"tokenizer_counts,omitempty"`
	Commands        []CommandObservation `json:"commands,omitempty"`
	FinalResponse   json.RawMessage      `json:"final_response,omitempty"`
	Findings        []ContextFinding     `json:"findings,omitempty"`
}

type ArmResult struct {
	Status                      string                        `json:"status"`
	Metadata                    RunnerMetadata                `json:"metadata"`
	RunnerWallDuration          Measurement                   `json:"runner_wall_duration"`
	InputTokens                 Measurement                   `json:"input_tokens"`
	CachedInputTokens           Measurement                   `json:"cached_input_tokens"`
	OutputTokens                Measurement                   `json:"output_tokens"`
	ReasoningOutputTokens       Measurement                   `json:"reasoning_output_tokens"`
	TotalTokens                 Measurement                   `json:"total_tokens"`
	ExecutionDuration           Measurement                   `json:"execution_duration"`
	CommandExecutions           Measurement                   `json:"command_executions"`
	RepositoryFilesRead         Measurement                   `json:"repository_files_read"`
	RepositoryBytesRead         Measurement                   `json:"repository_bytes_read"`
	GitCalls                    Measurement                   `json:"git_calls"`
	FilesystemSearchCalls       Measurement                   `json:"filesystem_search_calls"`
	ToolCalls                   Measurement                   `json:"tool_calls"`
	TaskReadyLatency            Measurement                   `json:"task_ready_latency"`
	FallbackRepositoryCommands  Measurement                   `json:"fallback_repository_commands"`
	FallbackRepositoryFilesRead Measurement                   `json:"fallback_repository_files_read"`
	FallbackRepositoryBytesRead Measurement                   `json:"fallback_repository_bytes_read"`
	ContextCorrectness          Measurement                   `json:"context_correctness"`
	ContextCompleteness         Measurement                   `json:"context_completeness"`
	TokenizerCounts             []TokenizerCount              `json:"tokenizer_counts"`
	Commands                    []CommandObservation          `json:"commands"`
	FinalResponse               json.RawMessage               `json:"final_response,omitempty"`
	Answer                      *benchmark.Answer             `json:"answer,omitempty"`
	DeterministicScore          *benchmark.DeterministicScore `json:"deterministic_score,omitempty"`
	Findings                    []ContextFinding              `json:"findings"`
}

type RunnerComparison struct {
	InputTokenReduction         Measurement `json:"input_token_reduction"`
	CachedInputTokenChange      Measurement `json:"cached_input_token_change"`
	TotalTokenReduction         Measurement `json:"total_token_reduction"`
	RepositoryFileReadReduction Measurement `json:"repository_file_read_reduction"`
	RepositoryByteReadReduction Measurement `json:"repository_byte_read_reduction"`
	ToolCallReduction           Measurement `json:"tool_call_reduction"`
	ExecutionDurationReduction  Measurement `json:"execution_duration_reduction"`
}

type ReuseProjection struct {
	Consumers                      int         `json:"consumers"`
	RepoCueMaintenanceDurationMS   float64     `json:"repocue_maintenance_duration_ms"`
	RepoCueMaintenanceBytesScanned int64       `json:"repocue_maintenance_bytes_scanned"`
	CueOutputBytes                 int64       `json:"cue_output_bytes"`
	EstimatedCueTokens             int         `json:"estimated_cue_tokens"`
	DirectTotalTokens              Measurement `json:"direct_total_tokens"`
	AssistedTotalTokens            Measurement `json:"assisted_total_tokens"`
	DirectRepositoryBytesRead      Measurement `json:"direct_repository_bytes_read"`
	AssistedRepositoryBytesRead    Measurement `json:"assisted_repository_bytes_read"`
	PerAgentCueBytes               int         `json:"per_agent_cue_bytes"`
	PerAgentEstimatedCueTokens     int         `json:"per_agent_estimated_cue_tokens"`
	PerAgentCueDurationMS          float64     `json:"per_agent_cue_duration_ms"`
	PerAgentFallbackBytesRead      Measurement `json:"per_agent_fallback_bytes_read"`
	DirectExecutionDuration        Measurement `json:"direct_execution_duration"`
	AssistedTotalExecutionDuration Measurement `json:"assisted_total_execution_duration"`
}

type Report struct {
	SchemaVersion       string            `json:"schema_version"`
	Kind                string            `json:"kind"`
	GeneratedAt         time.Time         `json:"generated_at"`
	BenchmarkVersion    string            `json:"benchmark_version"`
	OutputSchemaVersion string            `json:"output_schema_version"`
	RepoCueSnapshot     string            `json:"repocue_snapshot"`
	Repository          Repository        `json:"repository"`
	Baseline            OperationCost     `json:"baseline"`
	Update              OperationCost     `json:"update"`
	Cue                 CueCost           `json:"cue"`
	Direct              ArmResult         `json:"direct"`
	Assisted            ArmResult         `json:"assisted"`
	Comparison          RunnerComparison  `json:"comparison"`
	Reuse               []ReuseProjection `json:"reuse"`
	Limitations         []string          `json:"limitations"`
}

func Run(ctx context.Context, config Config) (Report, error) {
	if config.MaxTokens < 1 {
		return Report{}, errors.New("evaluation max tokens must be positive")
	}
	if (config.DirectRunner == "") != (config.AssistedRunner == "") {
		return Report{}, errors.New("direct and assisted runners must be provided together")
	}
	if config.DirectRunner != "" && config.TaskFile == "" {
		return Report{}, errors.New("a task file is required when runners are configured")
	}
	if config.DirectRunner != "" {
		var err error
		config.TaskFile, err = resolveRegularFile(config.TaskFile)
		if err != nil {
			return Report{}, fmt.Errorf("resolve evaluation task: %w", err)
		}
		config.DirectRunner, err = resolveExecutable(config.DirectRunner)
		if err != nil {
			return Report{}, fmt.Errorf("resolve direct runner: %w", err)
		}
		config.AssistedRunner, err = resolveExecutable(config.AssistedRunner)
		if err != nil {
			return Report{}, fmt.Errorf("resolve assisted runner: %w", err)
		}
	}
	repo, err := repository.Open(ctx, config.RepositoryPath)
	if err != nil {
		return Report{}, err
	}
	temporaryRoot, err := os.MkdirTemp(config.TemporaryRoot, "repocue-evaluation-")
	if err != nil {
		return Report{}, fmt.Errorf("create evaluation workspace: %w", err)
	}
	defer os.RemoveAll(temporaryRoot)

	baselineStarted := time.Now()
	baselineScan, err := repo.FullScan(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("evaluation baseline scan: %w", err)
	}
	store, err := storage.Open(ctx, filepath.Join(temporaryRoot, "state.db"))
	if err != nil {
		return Report{}, err
	}
	defer store.Close()
	baselineTransition, err := store.Initialize(ctx, model.Repository{
		ID: repo.ID, Name: repo.Name, Root: repo.Root, GitDir: repo.GitDir,
		CreatedAt: baselineScan.Basis.ObservedAt,
	}, baselineScan)
	if err != nil {
		return Report{}, fmt.Errorf("persist evaluation baseline: %w", err)
	}
	baselineWall := time.Since(baselineStarted)

	state, err := store.Current(ctx, repo.ID)
	if err != nil {
		return Report{}, err
	}
	updateStarted := time.Now()
	updateScan, err := repo.IncrementalScan(ctx, state.Files)
	if err != nil {
		return Report{}, fmt.Errorf("evaluation update scan: %w", err)
	}
	updateSnapshot := model.Snapshot{
		ID: state.Snapshot.ID, EpochID: state.Epoch.ID, Basis: updateScan.Basis,
		RepositoryDigest: updateScan.RepositoryDigest, FileCount: len(updateScan.Files),
	}
	items := snapshot.Diff(repo.ID, state.Snapshot, updateSnapshot, state.Files, updateScan.Files)
	updateTransition, err := store.Refresh(ctx, state, updateScan, items)
	if err != nil {
		return Report{}, fmt.Errorf("persist evaluation update: %w", err)
	}
	updateWall := time.Since(updateStarted)
	if updateTransition.Changed {
		return Report{}, errors.New("repository changed during the read-only evaluation probe")
	}

	current, err := store.Current(ctx, repo.ID)
	if err != nil {
		return Report{}, err
	}
	freshness := "current"
	if current.Snapshot.Basis.Dirty {
		freshness = "dirty-but-indexed"
	}
	cueStarted := time.Now()
	serializedCue, estimatedTokens, err := cue.Overview(current, freshness, config.MaxTokens)
	if err != nil {
		return Report{}, err
	}
	cueWall := time.Since(cueStarted)
	cuePath := filepath.Join(temporaryRoot, "overview.json")
	if err := os.WriteFile(cuePath, serializedCue, 0o600); err != nil {
		return Report{}, fmt.Errorf("write evaluation cue: %w", err)
	}

	direct := unobservedArm()
	assisted := unobservedArm()
	if config.DirectRunner != "" {
		direct, err = runExternal(ctx, repo, current, "direct", config.DirectRunner, config.TaskFile, "")
		if err != nil {
			return Report{}, err
		}
		assisted, err = runExternal(ctx, repo, current, "assisted", config.AssistedRunner, config.TaskFile, cuePath)
		if err != nil {
			return Report{}, err
		}
		if err := validatePair(direct, assisted); err != nil {
			return Report{}, err
		}
	}

	repositorySummary := summarizeRepository(repo, baselineScan)
	baselineCost := operationCost("full", baselineWall, baselineScan, baselineTransition, len(baselineScan.Files))
	updateCost := operationCost("no-op", updateWall, updateScan, updateTransition, 0)
	report := Report{
		SchemaVersion:       SchemaVersion,
		Kind:                "core-evaluation",
		GeneratedAt:         time.Now().UTC(),
		BenchmarkVersion:    legacyBenchmarkVersion,
		OutputSchemaVersion: legacyAnswerSchemaVersion,
		RepoCueSnapshot:     current.Snapshot.ID,
		Repository:          repositorySummary,
		Baseline:            baselineCost,
		Update:              updateCost,
		Cue: CueCost{
			View: "overview", WallDurationMS: durationMS(cueWall), OutputBytes: len(serializedCue),
			EstimatedTokens: estimatedTokens, MaxTokens: config.MaxTokens,
		},
		Direct:     direct,
		Assisted:   assisted,
		Comparison: compareArms(direct, assisted),
		Limitations: []string{
			"Estimated cue tokens use ceil(serialized UTF-8 bytes / 4).",
			"The update probe is a no-op refresh because evaluation repositories are not modified.",
			"Agent metrics remain not_observed unless external runners provide them.",
			"Reuse totals are projections from one completed A/B pair, not repeated consumer runs.",
		},
	}
	report.Reuse = projectReuse(report)
	return report, nil
}

func runExternal(ctx context.Context, repo *repository.Repository, state model.CurrentState, arm, runner, taskFile, cueFile string) (ArmResult, error) {
	before, err := repo.ValidateSnapshot(ctx, state.Snapshot, state.Files)
	if err != nil {
		return ArmResult{}, fmt.Errorf("capture %s runner basis: %w", arm, err)
	}
	command := exec.CommandContext(ctx, runner)
	command.Dir = repo.Root
	head := ""
	if state.Snapshot.Basis.Head != nil {
		head = *state.Snapshot.Basis.Head
	}
	command.Env = append(os.Environ(),
		"REPOCUE_EVAL_SCHEMA="+RunnerSchemaVersion,
		"REPOCUE_EVAL_ARM="+arm,
		"REPOCUE_EVAL_REPOSITORY="+repo.Root,
		"REPOCUE_EVAL_TASK_FILE="+taskFile,
		"REPOCUE_EVAL_CUE_FILE="+cueFile,
		"REPOCUE_EVAL_BENCHMARK_VERSION="+legacyBenchmarkVersion,
		"REPOCUE_EVAL_OUTPUT_SCHEMA_VERSION="+legacyAnswerSchemaVersion,
		"REPOCUE_EVAL_SNAPSHOT="+state.Snapshot.ID,
		"REPOCUE_EVAL_HEAD="+head,
		"REPOCUE_EVAL_READ_ONLY=1",
	)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	started := time.Now()
	if err := command.Run(); err != nil {
		return ArmResult{}, fmt.Errorf("run %s evaluation runner: %w: %s", arm, err, stderr.String())
	}
	wall := time.Since(started)
	observation, err := decodeObservation(stdout.Bytes())
	if err != nil {
		return ArmResult{}, fmt.Errorf("decode %s runner observation: %w", arm, err)
	}
	if err := validateObservation(observation, arm); err != nil {
		return ArmResult{}, err
	}
	after, err := repo.ValidateSnapshot(ctx, state.Snapshot, state.Files)
	if err != nil {
		return ArmResult{}, fmt.Errorf("verify %s runner basis: %w", arm, err)
	}
	if before.Basis.StatusDigest != after.Basis.StatusDigest || before.RepositoryDigest != after.RepositoryDigest {
		return ArmResult{}, fmt.Errorf("%s evaluation runner modified the repository", arm)
	}
	return observedArm(observation, wall, state.Snapshot.Basis)
}

func decodeObservation(serialized []byte) (RunnerObservation, error) {
	decoder := json.NewDecoder(bytes.NewReader(serialized))
	decoder.DisallowUnknownFields()
	var observation RunnerObservation
	if err := decoder.Decode(&observation); err != nil {
		return RunnerObservation{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return RunnerObservation{}, errors.New("runner emitted more than one JSON value")
	}
	return observation, nil
}

func validateObservation(observation RunnerObservation, arm string) error {
	if observation.SchemaVersion != RunnerSchemaVersion {
		return fmt.Errorf("runner schema version %q is not supported", observation.SchemaVersion)
	}
	if observation.Arm != arm {
		return fmt.Errorf("runner reported arm %q, expected %q", observation.Arm, arm)
	}
	if observation.Metadata.Adapter == "" || observation.Metadata.AdapterVersion == "" {
		return errors.New("runner metadata requires adapter and adapter_version")
	}
	integerValues := []*int64{
		observation.Metrics.InputTokens, observation.Metrics.CachedInputTokens,
		observation.Metrics.OutputTokens, observation.Metrics.ReasoningOutputTokens,
		observation.Metrics.TotalTokens, observation.Metrics.CommandExecutions,
		observation.Metrics.RepositoryFilesRead,
		observation.Metrics.RepositoryBytesRead, observation.Metrics.GitCalls,
		observation.Metrics.FilesystemSearchCalls, observation.Metrics.ToolCalls,
		observation.Metrics.FallbackRepositoryCommands,
		observation.Metrics.FallbackRepositoryFilesRead, observation.Metrics.FallbackRepositoryBytesRead,
	}
	for _, value := range integerValues {
		if value != nil && *value < 0 {
			return errors.New("runner metrics must be non-negative")
		}
	}
	for _, value := range []*float64{observation.Metrics.ExecutionDurationMS, observation.Metrics.TaskReadyLatencyMS} {
		if value != nil && *value < 0 {
			return errors.New("runner metrics must be non-negative")
		}
	}
	for _, value := range []*float64{observation.Metrics.ContextCorrectness, observation.Metrics.ContextCompleteness} {
		if value != nil && (*value < 0 || *value > 1) {
			return errors.New("context quality metrics must be between 0 and 1")
		}
	}
	for _, count := range observation.TokenizerCounts {
		if count.Tokenizer == "" || count.Tokens < 0 {
			return errors.New("tokenizer counts require a name and non-negative count")
		}
	}
	for name, status := range observation.Metrics.Statuses {
		if !knownRunnerMetric(name) {
			return fmt.Errorf("runner reported a status for unknown metric %q", name)
		}
		if status != measurementObserved && status != measurementDerived && status != measurementEstimated {
			return fmt.Errorf("runner metric %q has invalid status %q", name, status)
		}
	}
	for _, command := range observation.Commands {
		if command.Command == "" || command.Classification == "" || command.FilesRead == nil {
			return errors.New("command observations require command, classification, and files_read")
		}
	}
	if len(observation.FinalResponse) > 0 && !json.Valid(observation.FinalResponse) {
		return errors.New("runner final_response must be valid JSON")
	}
	return nil
}

func validatePair(direct, assisted ArmResult) error {
	left := direct.Metadata
	right := assisted.Metadata
	if left.Model != right.Model || left.ReasoningEffort != right.ReasoningEffort ||
		left.Sandbox != right.Sandbox || left.BenchmarkVersion != right.BenchmarkVersion ||
		left.OutputSchemaVersion != right.OutputSchemaVersion || left.CodexVersion != right.CodexVersion ||
		left.RepositoryHead != right.RepositoryHead || left.RepoCueSnapshot != right.RepoCueSnapshot {
		return fmt.Errorf("direct and assisted runners did not use identical agent conditions: direct=%+v assisted=%+v", left, right)
	}
	return nil
}

func summarizeRepository(repo *repository.Repository, scan model.Scan) Repository {
	result := Repository{
		ID: repo.ID, Name: repo.Name, Root: repo.Root, Branch: scan.Basis.Branch,
		Head: scan.Basis.Head, Dirty: scan.Basis.Dirty, TrackedFiles: len(scan.Files),
	}
	for _, file := range scan.Files {
		if file.Exists {
			result.PresentFiles++
			result.TrackedBytes += file.SizeBytes
		}
	}
	return result
}

func operationCost(kind string, wall time.Duration, scan model.Scan, transition storage.Transition, rows int) OperationCost {
	return OperationCost{
		Kind: kind, WallDurationMS: durationMS(wall),
		ScanDurationMS: float64(scan.Metrics.Duration) / float64(time.Millisecond),
		FilesScanned:   scan.Metrics.FilesScanned, BytesScanned: scan.Metrics.BytesScanned,
		GitCommands: scan.Metrics.GitCommands, Changed: transition.Changed,
		DatabaseSizeBytes: transition.DatabaseSizeBytes, MaterializedRows: rows,
	}
}

func unobservedArm() ArmResult {
	return ArmResult{
		Status: measurementUnobserved, Metadata: RunnerMetadata{}, RunnerWallDuration: unobserved("milliseconds"),
		InputTokens: unobserved("tokens"), CachedInputTokens: unobserved("tokens"),
		OutputTokens: unobserved("tokens"), ReasoningOutputTokens: unobserved("tokens"),
		TotalTokens: unobserved("tokens"), ExecutionDuration: unobserved("milliseconds"),
		CommandExecutions: unobserved("commands"), RepositoryFilesRead: unobserved("files"),
		RepositoryBytesRead: unobserved("bytes"), GitCalls: unobserved("calls"),
		FilesystemSearchCalls: unobserved("calls"), ToolCalls: unobserved("calls"),
		TaskReadyLatency: unobserved("milliseconds"), FallbackRepositoryCommands: unobserved("commands"),
		FallbackRepositoryFilesRead: unobserved("files"),
		FallbackRepositoryBytesRead: unobserved("bytes"), ContextCorrectness: unobserved("ratio"),
		ContextCompleteness: unobserved("ratio"), TokenizerCounts: []TokenizerCount{},
		Commands: []CommandObservation{}, Findings: []ContextFinding{},
	}
}

func observedArm(observation RunnerObservation, wall time.Duration, basis model.Basis) (ArmResult, error) {
	metrics := observation.Metrics
	result := ArmResult{
		Status: measurementObserved, Metadata: observation.Metadata,
		RunnerWallDuration:          observed(durationMS(wall), "milliseconds"),
		InputTokens:                 metricFromInt(metrics.InputTokens, metricStatus(metrics, "input_tokens"), "tokens"),
		CachedInputTokens:           metricFromInt(metrics.CachedInputTokens, metricStatus(metrics, "cached_input_tokens"), "tokens"),
		OutputTokens:                metricFromInt(metrics.OutputTokens, metricStatus(metrics, "output_tokens"), "tokens"),
		ReasoningOutputTokens:       metricFromInt(metrics.ReasoningOutputTokens, metricStatus(metrics, "reasoning_output_tokens"), "tokens"),
		TotalTokens:                 metricFromInt(metrics.TotalTokens, metricStatus(metrics, "total_tokens"), "tokens"),
		ExecutionDuration:           metricFromPointer(metrics.ExecutionDurationMS, metricStatus(metrics, "execution_duration_ms"), "milliseconds"),
		CommandExecutions:           metricFromInt(metrics.CommandExecutions, metricStatus(metrics, "command_executions"), "commands"),
		RepositoryFilesRead:         metricFromInt(metrics.RepositoryFilesRead, metricStatus(metrics, "repository_files_read"), "files"),
		RepositoryBytesRead:         metricFromInt(metrics.RepositoryBytesRead, metricStatus(metrics, "repository_bytes_read"), "bytes"),
		GitCalls:                    metricFromInt(metrics.GitCalls, metricStatus(metrics, "git_calls"), "calls"),
		FilesystemSearchCalls:       metricFromInt(metrics.FilesystemSearchCalls, metricStatus(metrics, "filesystem_search_calls"), "calls"),
		ToolCalls:                   metricFromInt(metrics.ToolCalls, metricStatus(metrics, "tool_calls"), "calls"),
		TaskReadyLatency:            metricFromPointer(metrics.TaskReadyLatencyMS, metricStatus(metrics, "task_ready_latency_ms"), "milliseconds"),
		FallbackRepositoryCommands:  metricFromInt(metrics.FallbackRepositoryCommands, metricStatus(metrics, "fallback_repository_commands"), "commands"),
		FallbackRepositoryFilesRead: metricFromInt(metrics.FallbackRepositoryFilesRead, metricStatus(metrics, "fallback_repository_files_read"), "files"),
		FallbackRepositoryBytesRead: metricFromInt(metrics.FallbackRepositoryBytesRead, metricStatus(metrics, "fallback_repository_bytes_read"), "bytes"),
		ContextCorrectness:          metricFromPointer(metrics.ContextCorrectness, metricStatus(metrics, "context_correctness"), "ratio"),
		ContextCompleteness:         metricFromPointer(metrics.ContextCompleteness, metricStatus(metrics, "context_completeness"), "ratio"),
		TokenizerCounts:             nonNilTokenizerCounts(observation.TokenizerCounts),
		Commands:                    nonNilCommands(observation.Commands), FinalResponse: observation.FinalResponse,
		Findings: nonNilFindings(observation.Findings),
	}
	if len(observation.FinalResponse) > 0 && observation.Metadata.OutputSchemaVersion == benchmark.AnswerSchemaVersion {
		answer, score, err := benchmark.ParseAndScore(observation.FinalResponse, basis)
		if err != nil {
			return ArmResult{}, fmt.Errorf("score %s benchmark answer: %w", observation.Arm, err)
		}
		result.Answer = &answer
		result.DeterministicScore = &score
		value := score.Ratio
		result.ContextCorrectness = Measurement{Status: measurementDerived, Value: &value, Unit: "ratio"}
	}
	return result, nil
}

func compareArms(direct, assisted ArmResult) RunnerComparison {
	return RunnerComparison{
		InputTokenReduction:         subtract(direct.InputTokens, assisted.InputTokens),
		CachedInputTokenChange:      subtract(assisted.CachedInputTokens, direct.CachedInputTokens),
		TotalTokenReduction:         subtract(direct.TotalTokens, assisted.TotalTokens),
		RepositoryFileReadReduction: subtract(direct.RepositoryFilesRead, assisted.RepositoryFilesRead),
		RepositoryByteReadReduction: subtract(direct.RepositoryBytesRead, assisted.RepositoryBytesRead),
		ToolCallReduction:           subtract(direct.ToolCalls, assisted.ToolCalls),
		ExecutionDurationReduction:  subtract(direct.ExecutionDuration, assisted.ExecutionDuration),
	}
}

func projectReuse(report Report) []ReuseProjection {
	result := make([]ReuseProjection, 0, len(consumerCounts))
	maintenanceDuration := report.Baseline.WallDurationMS + report.Update.WallDurationMS
	maintenanceBytes := report.Baseline.BytesScanned + report.Update.BytesScanned
	for _, consumers := range consumerCounts {
		count := float64(consumers)
		assistedBytes := scaled(report.Assisted.RepositoryBytesRead, count)
		if assistedBytes.Status != measurementUnobserved {
			value := float64(maintenanceBytes) + *assistedBytes.Value
			assistedBytes = Measurement{Status: measurementDerived, Value: &value, Unit: "bytes"}
		}
		assistedLatency := scaled(report.Assisted.ExecutionDuration, count)
		if assistedLatency.Status != measurementUnobserved {
			value := maintenanceDuration + count*report.Cue.WallDurationMS + *assistedLatency.Value
			assistedLatency = Measurement{Status: measurementDerived, Value: &value, Unit: "milliseconds"}
		}
		result = append(result, ReuseProjection{
			Consumers: consumers, RepoCueMaintenanceDurationMS: maintenanceDuration,
			RepoCueMaintenanceBytesScanned: maintenanceBytes,
			CueOutputBytes:                 int64(consumers * report.Cue.OutputBytes),
			EstimatedCueTokens:             consumers * report.Cue.EstimatedTokens,
			DirectTotalTokens:              scaled(report.Direct.TotalTokens, count),
			AssistedTotalTokens:            scaled(report.Assisted.TotalTokens, count),
			DirectRepositoryBytesRead:      scaled(report.Direct.RepositoryBytesRead, count),
			AssistedRepositoryBytesRead:    assistedBytes,
			PerAgentCueBytes:               report.Cue.OutputBytes,
			PerAgentEstimatedCueTokens:     report.Cue.EstimatedTokens,
			PerAgentCueDurationMS:          report.Cue.WallDurationMS,
			PerAgentFallbackBytesRead:      report.Assisted.FallbackRepositoryBytesRead,
			DirectExecutionDuration:        scaled(report.Direct.ExecutionDuration, count),
			AssistedTotalExecutionDuration: assistedLatency,
		})
	}
	return result
}

func observed(value float64, unit string) Measurement {
	return Measurement{Status: measurementObserved, Value: &value, Unit: unit}
}

func unobserved(unit string) Measurement {
	return Measurement{Status: measurementUnobserved, Unit: unit}
}

func metricFromPointer(value *float64, status, unit string) Measurement {
	if value == nil {
		return unobserved(unit)
	}
	return Measurement{Status: status, Value: value, Unit: unit}
}

func metricFromInt(value *int64, status, unit string) Measurement {
	if value == nil {
		return unobserved(unit)
	}
	converted := float64(*value)
	return Measurement{Status: status, Value: &converted, Unit: unit}
}

func subtract(left, right Measurement) Measurement {
	if left.Status == measurementUnobserved || right.Status == measurementUnobserved {
		return unobserved(left.Unit)
	}
	result := observed(*left.Value-*right.Value, left.Unit)
	result.Status = measurementDerived
	return result
}

func scaled(value Measurement, multiplier float64) Measurement {
	if value.Status == measurementUnobserved {
		return unobserved(value.Unit)
	}
	result := observed(*value.Value*multiplier, value.Unit)
	result.Status = measurementDerived
	return result
}

func durationMS(duration time.Duration) float64 {
	return float64(duration) / float64(time.Millisecond)
}

func nonNilTokenizerCounts(values []TokenizerCount) []TokenizerCount {
	if values == nil {
		return []TokenizerCount{}
	}
	return values
}

func nonNilFindings(values []ContextFinding) []ContextFinding {
	if values == nil {
		return []ContextFinding{}
	}
	return values
}

func nonNilCommands(values []CommandObservation) []CommandObservation {
	if values == nil {
		return []CommandObservation{}
	}
	return values
}

func metricStatus(metrics RunnerMetrics, name string) string {
	if status := metrics.Statuses[name]; status != "" {
		return status
	}
	return measurementObserved
}

func knownRunnerMetric(name string) bool {
	known := map[string]bool{
		"input_tokens": true, "cached_input_tokens": true, "output_tokens": true,
		"reasoning_output_tokens": true, "total_tokens": true, "execution_duration_ms": true,
		"command_executions": true, "repository_files_read": true, "repository_bytes_read": true,
		"git_calls": true, "filesystem_search_calls": true, "tool_calls": true,
		"task_ready_latency_ms": true, "fallback_repository_commands": true,
		"fallback_repository_files_read": true, "fallback_repository_bytes_read": true,
		"context_correctness": true, "context_completeness": true,
	}
	return known[name]
}

func resolveRegularFile(path string) (string, error) {
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("path is not a regular file")
	}
	return resolved, nil
}

func resolveExecutable(command string) (string, error) {
	if filepath.IsAbs(command) || command != filepath.Base(command) {
		path, err := resolveRegularFile(command)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		if info.Mode().Perm()&0o111 == 0 {
			return "", errors.New("runner is not executable")
		}
		return path, nil
	}
	return exec.LookPath(command)
}
