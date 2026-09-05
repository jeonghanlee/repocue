package evaluation

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jeonghanlee/repocue/internal/benchmark"
	"github.com/jeonghanlee/repocue/internal/cue"
	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/jeonghanlee/repocue/internal/repository"
	"github.com/jeonghanlee/repocue/internal/snapshot"
	"github.com/jeonghanlee/repocue/internal/storage"
)

const (
	M2SchemaVersion       = "repocue/evaluation-3"
	M2RunnerSchemaVersion = "repocue/evaluation-runner-3"

	ConditionDirect           = "direct"
	ConditionPlacebo          = "placebo"
	ConditionBasic            = "basic"
	ConditionRanked           = "ranked"
	ConditionStructuralOracle = "structural-oracle"
)

var M2Conditions = []string{
	ConditionDirect,
	ConditionPlacebo,
	ConditionBasic,
	ConditionRanked,
	ConditionStructuralOracle,
}

type UsageEvent struct {
	Turn                  int   `json:"turn"`
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
}

type M2RunnerMetrics struct {
	InputTokens                     *int64            `json:"input_tokens,omitempty"`
	CachedInputTokens               *int64            `json:"cached_input_tokens,omitempty"`
	OutputTokens                    *int64            `json:"output_tokens,omitempty"`
	ReasoningOutputTokens           *int64            `json:"reasoning_output_tokens,omitempty"`
	TotalTokens                     *int64            `json:"total_tokens,omitempty"`
	ExecutionDurationMS             *float64          `json:"execution_duration_ms,omitempty"`
	CommandExecutions               *int64            `json:"command_executions,omitempty"`
	RepositoryFilesNamed            *int64            `json:"repository_files_named,omitempty"`
	NamedFileSizeProxyBytes         *int64            `json:"named_file_size_proxy_bytes,omitempty"`
	GitCalls                        *int64            `json:"git_calls,omitempty"`
	FilesystemSearchCalls           *int64            `json:"filesystem_search_calls,omitempty"`
	ToolCalls                       *int64            `json:"tool_calls,omitempty"`
	FallbackRepositoryCommands      *int64            `json:"fallback_repository_commands,omitempty"`
	FallbackRepositoryFilesNamed    *int64            `json:"fallback_repository_files_named,omitempty"`
	FallbackNamedFileSizeProxyBytes *int64            `json:"fallback_named_file_size_proxy_bytes,omitempty"`
	Statuses                        map[string]string `json:"statuses,omitempty"`
}

type M2RunnerObservation struct {
	SchemaVersion   string               `json:"schema_version"`
	Condition       string               `json:"condition"`
	RunIndex        int                  `json:"run_index"`
	Metadata        RunnerMetadata       `json:"metadata"`
	Metrics         M2RunnerMetrics      `json:"metrics"`
	UsageEvents     []UsageEvent         `json:"usage_events"`
	TokenizerCounts []TokenizerCount     `json:"tokenizer_counts"`
	Commands        []CommandObservation `json:"commands"`
	FinalResponse   json.RawMessage      `json:"final_response,omitempty"`
	Findings        []ContextFinding     `json:"findings"`
	Limitations     []string             `json:"limitations"`
}

type M2Config struct {
	RepositoryPath  string
	MaxTokens       int
	TaskFile        string
	Runner          string
	OracleTool      string
	OutputDirectory string
	RunIndex        int
	TemporaryRoot   string
}

type M2Repository struct {
	SuppliedPath          string  `json:"supplied_path"`
	ResolvedRoot          string  `json:"resolved_root"`
	ID                    string  `json:"id"`
	Name                  string  `json:"name"`
	Branch                *string `json:"branch"`
	Head                  *string `json:"head"`
	Dirty                 bool    `json:"dirty"`
	RepoCueStatusDigest   string  `json:"repocue_status_digest"`
	RepositoryDigest      string  `json:"repository_digest"`
	WorkingTreeDigest     string  `json:"working_tree_digest"`
	PorcelainStatusDigest string  `json:"porcelain_status_digest"`
	BinaryDiffDigest      string  `json:"binary_diff_digest"`
	StateFingerprint      string  `json:"state_fingerprint"`
	TrackedFiles          int     `json:"tracked_files"`
	TrackedBytes          int64   `json:"tracked_bytes"`
}

type M2CueCost struct {
	SchemaVersion                string  `json:"schema_version,omitempty"`
	Bytes                        int     `json:"bytes"`
	EstimatedTokens              int     `json:"estimated_tokens"`
	MaxTokens                    int     `json:"max_tokens"`
	WallDurationMS               float64 `json:"wall_duration_ms"`
	StructuralAnalysisDurationMS float64 `json:"structural_analysis_duration_ms,omitempty"`
	StructuralCandidateCount     int     `json:"structural_candidate_count,omitempty"`
}

type M2ConditionReport struct {
	SchemaVersion       string               `json:"schema_version"`
	Kind                string               `json:"kind"`
	GeneratedAt         time.Time            `json:"generated_at"`
	MaintenanceID       string               `json:"maintenance_id"`
	Condition           string               `json:"condition"`
	ConditionOrder      int                  `json:"condition_order"`
	RunIndex            int                  `json:"run_index"`
	BenchmarkVersion    string               `json:"benchmark_version"`
	OutputSchemaVersion string               `json:"output_schema_version"`
	RepoCueSnapshot     string               `json:"repocue_snapshot"`
	Repository          M2Repository         `json:"repository"`
	Baseline            OperationCost        `json:"baseline"`
	Update              OperationCost        `json:"update"`
	Cue                 M2CueCost            `json:"cue"`
	Runner              *M2RunnerObservation `json:"runner,omitempty"`
	Limitations         []string             `json:"limitations"`
}

type M2Manifest struct {
	SchemaVersion  string              `json:"schema_version"`
	Kind           string              `json:"kind"`
	GeneratedAt    time.Time           `json:"generated_at"`
	MaintenanceID  string              `json:"maintenance_id"`
	Repository     M2Repository        `json:"repository"`
	ConditionOrder []string            `json:"condition_order"`
	Reports        []M2ConditionReport `json:"reports"`
	Limitations    []string            `json:"limitations"`
}

func ValidM2Condition(condition string) bool {
	for _, candidate := range M2Conditions {
		if condition == candidate {
			return true
		}
	}
	return false
}

func RunM2(ctx context.Context, config M2Config) (M2Manifest, error) {
	if config.MaxTokens < 1 {
		return M2Manifest{}, errors.New("evaluation max tokens must be positive")
	}
	if config.RunIndex < 1 {
		return M2Manifest{}, errors.New("evaluation run index must be positive")
	}
	var err error
	if config.Runner != "" {
		if config.TaskFile == "" {
			return M2Manifest{}, errors.New("a task file is required when a runner is configured")
		}
		config.Runner, err = resolveExecutable(config.Runner)
		if err != nil {
			return M2Manifest{}, fmt.Errorf("resolve evaluation runner: %w", err)
		}
		config.TaskFile, err = resolveRegularFile(config.TaskFile)
		if err != nil {
			return M2Manifest{}, fmt.Errorf("resolve evaluation task: %w", err)
		}
	}
	if config.OracleTool == "" {
		return M2Manifest{}, errors.New("a structural oracle tool is required")
	}
	config.OracleTool, err = resolveExecutable(config.OracleTool)
	if err != nil {
		return M2Manifest{}, fmt.Errorf("resolve structural oracle: %w", err)
	}
	repo, err := repository.Open(ctx, config.RepositoryPath)
	if err != nil {
		return M2Manifest{}, err
	}
	paths := []struct {
		name string
		path string
	}{
		{"output directory", config.OutputDirectory},
		{"temporary root", config.TemporaryRoot},
	}
	for _, candidate := range paths {
		if candidate.path == "" {
			continue
		}
		for _, protectedRoot := range []string{repo.Root, repo.GitDir} {
			inside, err := pathWithinRepository(protectedRoot, candidate.path)
			if err != nil {
				return M2Manifest{}, fmt.Errorf("resolve M2 %s: %w", candidate.name, err)
			}
			if inside {
				return M2Manifest{}, fmt.Errorf("M2 %s must be outside the evaluated repository", candidate.name)
			}
		}
	}
	if config.TemporaryRoot != "" {
		if err := os.MkdirAll(config.TemporaryRoot, 0o755); err != nil {
			return M2Manifest{}, fmt.Errorf("create M2 temporary root: %w", err)
		}
	}
	workspace, err := os.MkdirTemp(config.TemporaryRoot, "repocue-m2-")
	if err != nil {
		return M2Manifest{}, fmt.Errorf("create M2 workspace: %w", err)
	}
	defer os.RemoveAll(workspace)

	baselineStarted := time.Now()
	baselineScan, err := repo.FullScan(ctx)
	if err != nil {
		return M2Manifest{}, fmt.Errorf("M2 baseline scan: %w", err)
	}
	store, err := storage.Open(ctx, filepath.Join(workspace, "state.db"))
	if err != nil {
		return M2Manifest{}, err
	}
	defer store.Close()
	baselineTransition, err := store.Initialize(ctx, model.Repository{
		ID: repo.ID, Name: repo.Name, Root: repo.Root, GitDir: repo.GitDir, CreatedAt: baselineScan.Basis.ObservedAt,
	}, baselineScan)
	if err != nil {
		return M2Manifest{}, fmt.Errorf("persist M2 baseline: %w", err)
	}
	baselineWall := time.Since(baselineStarted)
	state, err := store.Current(ctx, repo.ID)
	if err != nil {
		return M2Manifest{}, err
	}
	updateStarted := time.Now()
	updateScan, err := repo.IncrementalScan(ctx, state.Files)
	if err != nil {
		return M2Manifest{}, fmt.Errorf("M2 no-op refresh: %w", err)
	}
	updateSnapshot := model.Snapshot{ID: state.Snapshot.ID, EpochID: state.Epoch.ID, Basis: updateScan.Basis, RepositoryDigest: updateScan.RepositoryDigest, FileCount: len(updateScan.Files)}
	items := snapshot.Diff(repo.ID, state.Snapshot, updateSnapshot, state.Files, updateScan.Files)
	updateTransition, err := store.Refresh(ctx, state, updateScan, items)
	if err != nil {
		return M2Manifest{}, fmt.Errorf("persist M2 no-op refresh: %w", err)
	}
	updateWall := time.Since(updateStarted)
	if updateTransition.Changed {
		return M2Manifest{}, errors.New("repository changed during M2 maintenance preparation")
	}
	current, err := store.Current(ctx, repo.ID)
	if err != nil {
		return M2Manifest{}, err
	}
	facts, err := repo.ContextFactsAt(ctx, current.Snapshot, current.Files)
	if err != nil {
		return M2Manifest{}, fmt.Errorf("capture M2 context facts: %w", err)
	}
	evidence, err := repo.StateEvidence(ctx)
	if err != nil {
		return M2Manifest{}, fmt.Errorf("capture M2 repository evidence: %w", err)
	}
	oracleStarted := time.Now()
	oracleCandidates, oracleLimitations, err := runStructuralOracle(ctx, config.OracleTool, repo.Root)
	if err != nil {
		return M2Manifest{}, err
	}
	oracleDuration := time.Since(oracleStarted)
	if _, err := repo.ValidateSnapshot(ctx, current.Snapshot, current.Files); err != nil {
		return M2Manifest{}, fmt.Errorf("verify M2 structural context basis: %w", err)
	}
	afterEvidence, err := repo.StateEvidence(ctx)
	if err != nil {
		return M2Manifest{}, fmt.Errorf("verify M2 repository evidence: %w", err)
	}
	if evidence != afterEvidence {
		return M2Manifest{}, errors.New("repository evidence changed during M2 context preparation")
	}
	freshness := "current"
	if current.Snapshot.Basis.Dirty {
		freshness = "dirty-but-indexed"
	}
	contexts, costs, err := prepareM2Contexts(current, facts, oracleCandidates, freshness, config.MaxTokens)
	if err != nil {
		return M2Manifest{}, err
	}
	structuralCost := costs[ConditionStructuralOracle]
	structuralCost.StructuralAnalysisDurationMS = durationMS(oracleDuration)
	structuralCost.StructuralCandidateCount = len(oracleCandidates)
	costs[ConditionStructuralOracle] = structuralCost
	for condition, content := range contexts {
		if condition == ConditionDirect {
			continue
		}
		path := filepath.Join(workspace, condition+".json")
		if err := os.WriteFile(path, content, 0o600); err != nil {
			return M2Manifest{}, fmt.Errorf("write %s context: %w", condition, err)
		}
	}

	repositorySummary := summarizeM2Repository(config.RepositoryPath, repo, baselineScan, evidence)
	maintenanceID := maintenanceIdentifier(repositorySummary, current.Snapshot.ID)
	baselineCost := operationCost("full", baselineWall, baselineScan, baselineTransition, len(baselineScan.Files))
	updateCost := operationCost("no-op", updateWall, updateScan, updateTransition, 0)
	manifest := M2Manifest{
		SchemaVersion: M2SchemaVersion, Kind: "m2-condition-set", GeneratedAt: time.Now().UTC(),
		MaintenanceID: maintenanceID, Repository: repositorySummary, ConditionOrder: append([]string{}, M2Conditions...),
		Reports: []M2ConditionReport{}, Limitations: append([]string{}, oracleLimitations...),
	}
	var runnerIdentity *RunnerMetadata
	for order, condition := range M2Conditions {
		report := M2ConditionReport{
			SchemaVersion: M2SchemaVersion, Kind: "m2-condition", GeneratedAt: time.Now().UTC(),
			MaintenanceID: maintenanceID, Condition: condition, ConditionOrder: order + 1, RunIndex: config.RunIndex,
			BenchmarkVersion: benchmark.Version, OutputSchemaVersion: benchmark.AnswerSchemaVersion,
			RepoCueSnapshot: current.Snapshot.ID, Repository: repositorySummary,
			Baseline: baselineCost, Update: updateCost, Cue: costs[condition],
			Limitations: append([]string{}, oracleLimitations...),
		}
		if config.Runner != "" {
			cuePath := ""
			if condition != ConditionDirect {
				cuePath = filepath.Join(workspace, condition+".json")
			}
			observation, err := runM2External(ctx, repo, current, condition, config.RunIndex, config.Runner, config.TaskFile, cuePath)
			if err != nil {
				return M2Manifest{}, err
			}
			if runnerIdentity == nil {
				identity := observation.Metadata
				runnerIdentity = &identity
			} else if err := validateM2RunnerIdentity(*runnerIdentity, observation.Metadata); err != nil {
				return M2Manifest{}, fmt.Errorf("condition %s: %w", condition, err)
			}
			report.Runner = &observation
		}
		if err := validateM2Report(report); err != nil {
			return M2Manifest{}, err
		}
		manifest.Reports = append(manifest.Reports, report)
	}
	if config.OutputDirectory != "" {
		if err := writeM2ReportSetAtomic(config.OutputDirectory, manifest.Reports); err != nil {
			return M2Manifest{}, err
		}
	}
	return manifest, nil
}

func prepareM2Contexts(state model.CurrentState, facts repository.ContextFacts, candidates []cue.StructuralCandidate, freshness string, maxTokens int) (map[string][]byte, map[string]M2CueCost, error) {
	contexts := map[string][]byte{ConditionDirect: nil}
	costs := map[string]M2CueCost{ConditionDirect: {MaxTokens: maxTokens}}
	rankedFacts := cue.RankedFacts{RecentCommits: facts.RecentCommits, EntryPoints: facts.EntryPoints, MakeTargets: facts.MakeTargets}
	builders := []struct {
		condition string
		schema    string
		build     func() ([]byte, int, error)
	}{
		{ConditionPlacebo, cue.ExperimentalSchemaVersion, func() ([]byte, int, error) { return cue.Placebo(state, freshness, maxTokens) }},
		{ConditionBasic, model.SchemaVersion, func() ([]byte, int, error) { return cue.Overview(state, freshness, maxTokens) }},
		{ConditionRanked, cue.ExperimentalSchemaVersion, func() ([]byte, int, error) { return cue.RankedOverview(state, rankedFacts, freshness, maxTokens) }},
		{ConditionStructuralOracle, cue.ExperimentalSchemaVersion, func() ([]byte, int, error) {
			return cue.StructuralOverview(state, rankedFacts, candidates, freshness, maxTokens)
		}},
	}
	for _, builder := range builders {
		started := time.Now()
		serialized, estimated, err := builder.build()
		if err != nil {
			return nil, nil, fmt.Errorf("build %s context: %w", builder.condition, err)
		}
		contexts[builder.condition] = serialized
		costs[builder.condition] = M2CueCost{
			SchemaVersion: builder.schema, Bytes: len(serialized), EstimatedTokens: estimated,
			MaxTokens: maxTokens, WallDurationMS: durationMS(time.Since(started)),
		}
	}
	return contexts, costs, nil
}

func runStructuralOracle(ctx context.Context, tool, root string) ([]cue.StructuralCandidate, []string, error) {
	command := exec.CommandContext(ctx, tool, root)
	command.Dir = root
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, nil, fmt.Errorf("run structural oracle: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	candidates := []cue.StructuralCandidate{}
	limitations := []string{}
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 5 || fields[0] == "" || fields[1] == "" || fields[2] == "" || fields[3] == "" {
			limitations = append(limitations, "structural oracle omitted one unsupported record")
			continue
		}
		candidates = append(candidates, cue.StructuralCandidate{
			Language: fields[0], Module: fields[1], Kind: fields[2], Name: fields[3], Signature: fields[4],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	return candidates, limitations, nil
}

func runM2External(ctx context.Context, repo *repository.Repository, state model.CurrentState, condition string, runIndex int, runner, taskFile, cueFile string) (M2RunnerObservation, error) {
	before, err := repo.ValidateSnapshot(ctx, state.Snapshot, state.Files)
	if err != nil {
		return M2RunnerObservation{}, fmt.Errorf("capture %s runner basis: %w", condition, err)
	}
	beforeEvidence, err := repo.StateEvidence(ctx)
	if err != nil {
		return M2RunnerObservation{}, fmt.Errorf("capture %s runner evidence: %w", condition, err)
	}
	command := exec.CommandContext(ctx, runner)
	command.Dir = repo.Root
	head := ""
	if state.Snapshot.Basis.Head != nil {
		head = *state.Snapshot.Basis.Head
	}
	command.Env = append(os.Environ(),
		"REPOCUE_EVAL_SCHEMA="+M2RunnerSchemaVersion,
		"REPOCUE_EVAL_CONDITION="+condition,
		fmt.Sprintf("REPOCUE_EVAL_RUN_INDEX=%d", runIndex),
		"REPOCUE_EVAL_REPOSITORY="+repo.Root,
		"REPOCUE_EVAL_TASK_FILE="+taskFile,
		"REPOCUE_EVAL_CUE_FILE="+cueFile,
		"REPOCUE_EVAL_BENCHMARK_VERSION="+benchmark.Version,
		"REPOCUE_EVAL_OUTPUT_SCHEMA_VERSION="+benchmark.AnswerSchemaVersion,
		"REPOCUE_EVAL_SNAPSHOT="+state.Snapshot.ID,
		"REPOCUE_EVAL_HEAD="+head,
		"REPOCUE_EVAL_READ_ONLY=1",
	)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return M2RunnerObservation{}, fmt.Errorf("run %s evaluation runner: %w: %s", condition, err, strings.TrimSpace(stderr.String()))
	}
	observation, err := decodeM2Observation(stdout.Bytes())
	if err != nil {
		return M2RunnerObservation{}, fmt.Errorf("decode %s runner observation: %w", condition, err)
	}
	if err := validateM2Observation(observation, condition, runIndex, state.Snapshot); err != nil {
		return M2RunnerObservation{}, err
	}
	after, err := repo.ValidateSnapshot(ctx, state.Snapshot, state.Files)
	if err != nil {
		return M2RunnerObservation{}, fmt.Errorf("verify %s runner basis: %w", condition, err)
	}
	afterEvidence, err := repo.StateEvidence(ctx)
	if err != nil {
		return M2RunnerObservation{}, fmt.Errorf("verify %s runner evidence: %w", condition, err)
	}
	if before.Basis.StatusDigest != after.Basis.StatusDigest || before.RepositoryDigest != after.RepositoryDigest {
		return M2RunnerObservation{}, fmt.Errorf("%s evaluation runner modified the repository", condition)
	}
	if beforeEvidence != afterEvidence {
		return M2RunnerObservation{}, fmt.Errorf("%s evaluation runner changed repository evidence", condition)
	}
	return observation, nil
}

func decodeM2Observation(serialized []byte) (M2RunnerObservation, error) {
	decoder := json.NewDecoder(bytes.NewReader(serialized))
	decoder.DisallowUnknownFields()
	var observation M2RunnerObservation
	if err := decoder.Decode(&observation); err != nil {
		return M2RunnerObservation{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return M2RunnerObservation{}, errors.New("runner emitted more than one JSON value")
	}
	return observation, nil
}

func validateM2Observation(observation M2RunnerObservation, condition string, runIndex int, expected model.Snapshot) error {
	if observation.SchemaVersion != M2RunnerSchemaVersion || observation.Condition != condition || observation.RunIndex != runIndex {
		return errors.New("runner observation does not match the requested condition contract")
	}
	metadata := observation.Metadata
	if metadata.Adapter == "" || metadata.AdapterVersion == "" || metadata.Model == "" ||
		metadata.ReasoningEffort == "" || metadata.Sandbox == "" {
		return errors.New("runner metadata requires adapter, adapter_version, model, reasoning_effort, and sandbox")
	}
	head := ""
	if expected.Basis.Head != nil {
		head = *expected.Basis.Head
	}
	if metadata.BenchmarkVersion != benchmark.Version || metadata.OutputSchemaVersion != benchmark.AnswerSchemaVersion ||
		metadata.RepositoryHead != head || metadata.RepoCueSnapshot != expected.ID {
		return errors.New("runner metadata does not match the requested benchmark and repository basis")
	}
	metrics := map[string]*int64{
		"input_tokens": observation.Metrics.InputTokens, "cached_input_tokens": observation.Metrics.CachedInputTokens,
		"output_tokens": observation.Metrics.OutputTokens, "reasoning_output_tokens": observation.Metrics.ReasoningOutputTokens,
		"total_tokens": observation.Metrics.TotalTokens, "command_executions": observation.Metrics.CommandExecutions,
		"repository_files_named":      observation.Metrics.RepositoryFilesNamed,
		"named_file_size_proxy_bytes": observation.Metrics.NamedFileSizeProxyBytes,
		"git_calls":                   observation.Metrics.GitCalls, "filesystem_search_calls": observation.Metrics.FilesystemSearchCalls,
		"tool_calls":                           observation.Metrics.ToolCalls,
		"fallback_repository_commands":         observation.Metrics.FallbackRepositoryCommands,
		"fallback_repository_files_named":      observation.Metrics.FallbackRepositoryFilesNamed,
		"fallback_named_file_size_proxy_bytes": observation.Metrics.FallbackNamedFileSizeProxyBytes,
	}
	for name, value := range metrics {
		if value != nil && *value < 0 {
			return fmt.Errorf("runner metric %s must be non-negative", name)
		}
	}
	if observation.Metrics.ExecutionDurationMS != nil && *observation.Metrics.ExecutionDurationMS < 0 {
		return errors.New("runner metric execution_duration_ms must be non-negative")
	}
	for name, status := range observation.Metrics.Statuses {
		value, known := metrics[name]
		if name == "execution_duration_ms" {
			known = true
			if observation.Metrics.ExecutionDurationMS != nil {
				placeholder := int64(0)
				value = &placeholder
			}
		}
		if !known {
			return fmt.Errorf("runner reported a status for unknown metric %q", name)
		}
		if status != measurementObserved && status != measurementDerived && status != measurementEstimated && status != measurementUnobserved {
			return fmt.Errorf("runner metric %q has invalid status %q", name, status)
		}
		if value == nil && status != measurementUnobserved {
			return fmt.Errorf("runner metric %q has status %q without a value", name, status)
		}
		if value != nil && status == measurementUnobserved {
			return fmt.Errorf("runner metric %q has a value marked not_observed", name)
		}
	}
	for name, value := range metrics {
		if value != nil {
			if _, found := observation.Metrics.Statuses[name]; !found {
				return fmt.Errorf("runner metric %q requires a measurement status", name)
			}
		}
	}
	if observation.Metrics.ExecutionDurationMS != nil {
		if _, found := observation.Metrics.Statuses["execution_duration_ms"]; !found {
			return errors.New("runner metric \"execution_duration_ms\" requires a measurement status")
		}
	}
	for _, name := range []string{"input_tokens", "cached_input_tokens", "output_tokens", "reasoning_output_tokens", "total_tokens"} {
		if metrics[name] == nil {
			return fmt.Errorf("runner observation requires metric %s", name)
		}
	}
	if *observation.Metrics.CachedInputTokens > *observation.Metrics.InputTokens {
		return errors.New("cached input tokens cannot exceed input tokens")
	}
	if *observation.Metrics.TotalTokens != *observation.Metrics.InputTokens+*observation.Metrics.OutputTokens {
		return errors.New("total tokens must equal input tokens plus output tokens")
	}
	if len(observation.UsageEvents) == 0 {
		return errors.New("runner observation requires at least one usage event")
	}
	var inputTokens, cachedInputTokens, outputTokens, reasoningOutputTokens int64
	for index, event := range observation.UsageEvents {
		if event.Turn != index+1 || event.InputTokens < 0 || event.CachedInputTokens < 0 ||
			event.OutputTokens < 0 || event.ReasoningOutputTokens < 0 || event.CachedInputTokens > event.InputTokens {
			return errors.New("runner usage events must be ordered and non-negative")
		}
		inputTokens += event.InputTokens
		cachedInputTokens += event.CachedInputTokens
		outputTokens += event.OutputTokens
		reasoningOutputTokens += event.ReasoningOutputTokens
	}
	if inputTokens != *observation.Metrics.InputTokens || cachedInputTokens != *observation.Metrics.CachedInputTokens ||
		outputTokens != *observation.Metrics.OutputTokens || reasoningOutputTokens != *observation.Metrics.ReasoningOutputTokens {
		return errors.New("runner token metrics do not equal the usage event totals")
	}
	if observation.Metrics.CommandExecutions != nil && *observation.Metrics.CommandExecutions != int64(len(observation.Commands)) {
		return errors.New("command execution count does not equal the command observations")
	}
	var gitCalls, searchCalls int64
	namedFiles := map[string]struct{}{}
	for _, count := range observation.TokenizerCounts {
		if count.Tokenizer == "" || count.Tokens < 0 {
			return errors.New("tokenizer counts require a name and non-negative token count")
		}
	}
	for _, command := range observation.Commands {
		if command.Command == "" || !validCommandClassification(command.Classification) || command.FilesRead == nil {
			return errors.New("command observations require command, classification, and files_read")
		}
		if strings.Contains(command.Classification, "git") {
			gitCalls++
		}
		if strings.Contains(command.Classification, "filesystem_search") {
			searchCalls++
		}
		for _, path := range command.FilesRead {
			cleaned := filepath.Clean(path)
			if path == "" || filepath.IsAbs(path) || cleaned == "." || cleaned == ".." ||
				strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
				return errors.New("command file observations must be relative repository paths")
			}
			namedFiles[filepath.ToSlash(cleaned)] = struct{}{}
		}
	}
	if observation.Metrics.GitCalls != nil && *observation.Metrics.GitCalls != gitCalls {
		return errors.New("git call count does not equal the command classifications")
	}
	if observation.Metrics.FilesystemSearchCalls != nil && *observation.Metrics.FilesystemSearchCalls != searchCalls {
		return errors.New("filesystem search count does not equal the command classifications")
	}
	if observation.Metrics.RepositoryFilesNamed != nil && *observation.Metrics.RepositoryFilesNamed != int64(len(namedFiles)) {
		return errors.New("repository file count does not equal the named command files")
	}
	if observation.Metrics.ToolCalls != nil && observation.Metrics.CommandExecutions != nil &&
		*observation.Metrics.ToolCalls < *observation.Metrics.CommandExecutions {
		return errors.New("tool call count cannot be smaller than command executions")
	}
	if condition == ConditionDirect {
		if observation.Metrics.FallbackRepositoryCommands != nil || observation.Metrics.FallbackRepositoryFilesNamed != nil ||
			observation.Metrics.FallbackNamedFileSizeProxyBytes != nil {
			return errors.New("direct condition must not report fallback repository metrics")
		}
	} else if exceeds(observation.Metrics.FallbackRepositoryCommands, observation.Metrics.CommandExecutions) ||
		exceeds(observation.Metrics.FallbackRepositoryFilesNamed, observation.Metrics.RepositoryFilesNamed) ||
		exceeds(observation.Metrics.FallbackNamedFileSizeProxyBytes, observation.Metrics.NamedFileSizeProxyBytes) {
		return errors.New("fallback repository metrics cannot exceed their total metrics")
	}
	for _, finding := range observation.Findings {
		if finding.Category == "" || finding.Severity == "" || finding.Message == "" {
			return errors.New("context findings require category, severity, and message")
		}
	}
	for _, limitation := range observation.Limitations {
		if limitation == "" {
			return errors.New("runner limitations must not contain empty values")
		}
	}
	if len(observation.FinalResponse) == 0 {
		return errors.New("runner final_response is required")
	}
	if _, _, err := benchmark.ParseAndScore(observation.FinalResponse, expected.Basis); err != nil {
		return fmt.Errorf("runner final_response does not satisfy %s: %w", benchmark.AnswerSchemaVersion, err)
	}
	return nil
}

func validateM2RunnerIdentity(expected, actual RunnerMetadata) error {
	if expected != actual {
		return errors.New("M2 conditions did not use identical runner metadata")
	}
	return nil
}

func validCommandClassification(value string) bool {
	switch value {
	case "other", "git", "filesystem_search", "filesystem_read",
		"git+filesystem_search", "git+filesystem_read", "filesystem_search+filesystem_read",
		"git+filesystem_search+filesystem_read":
		return true
	default:
		return false
	}
}

func exceeds(part, total *int64) bool {
	return part != nil && (total == nil || *part > *total)
}

func validateM2Report(report M2ConditionReport) error {
	if report.SchemaVersion != M2SchemaVersion || !ValidM2Condition(report.Condition) || report.RunIndex < 1 {
		return errors.New("invalid M2 condition report")
	}
	if report.Cue.EstimatedTokens > report.Cue.MaxTokens {
		return errors.New("M2 condition context exceeds its token budget")
	}
	if report.Runner != nil && report.Runner.Condition != report.Condition {
		return errors.New("M2 report and runner conditions differ")
	}
	return nil
}

func writeM2ReportSetAtomic(directory string, reports []M2ConditionReport) error {
	if len(reports) != len(M2Conditions) {
		return errors.New("M2 report set is incomplete")
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	first := reports[0]
	modelName := "no-runner"
	if first.Runner != nil && first.Runner.Metadata.Model != "" {
		modelName = first.Runner.Metadata.Model
	}
	setName := fmt.Sprintf("%s_%s_run-%02d_%s", safeName(first.Repository.Name), safeName(modelName), first.RunIndex, safeName(first.MaintenanceID))
	finalSet := filepath.Join(directory, setName)
	if _, err := os.Stat(finalSet); err == nil {
		return fmt.Errorf("final M2 report set already exists: %s", finalSet)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	staging, err := os.MkdirTemp(directory, "."+setName+".draft-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	for index, report := range reports {
		if report.Condition != M2Conditions[index] || report.MaintenanceID != first.MaintenanceID ||
			report.RunIndex != first.RunIndex || report.Repository.StateFingerprint != first.Repository.StateFingerprint {
			return errors.New("M2 report set does not share one ordered repository basis")
		}
		if err := writeM2ReportFile(filepath.Join(staging, m2ReportName(report, modelName)), report); err != nil {
			return err
		}
	}
	if err := os.Rename(staging, finalSet); err != nil {
		return fmt.Errorf("publish M2 report set: %w", err)
	}
	return nil
}

func m2ReportName(report M2ConditionReport, modelName string) string {
	return fmt.Sprintf("%s_%s_%s_run-%02d.json", safeName(report.Repository.Name), safeName(report.Condition), safeName(modelName), report.RunIndex)
}

func writeM2ReportFile(path string, report M2ConditionReport) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return nil
}

func summarizeM2Repository(supplied string, repo *repository.Repository, scan model.Scan, evidence repository.StateEvidence) M2Repository {
	var trackedBytes int64
	for _, file := range scan.Files {
		trackedBytes += file.SizeBytes
	}
	digest := sha256.Sum256([]byte(strings.Join([]string{
		scan.Basis.StatusDigest, scan.RepositoryDigest, scan.Basis.WorkingTreeDigest,
		evidence.PorcelainStatusDigest, evidence.BinaryDiffDigest,
	}, "\x00")))
	return M2Repository{
		SuppliedPath: supplied, ResolvedRoot: repo.Root, ID: repo.ID, Name: repo.Name,
		Branch: scan.Basis.Branch, Head: scan.Basis.Head, Dirty: scan.Basis.Dirty,
		RepoCueStatusDigest: scan.Basis.StatusDigest, RepositoryDigest: scan.RepositoryDigest,
		WorkingTreeDigest:     scan.Basis.WorkingTreeDigest,
		PorcelainStatusDigest: evidence.PorcelainStatusDigest, BinaryDiffDigest: evidence.BinaryDiffDigest,
		StateFingerprint: "sha256:" + hex.EncodeToString(digest[:]),
		TrackedFiles:     len(scan.Files), TrackedBytes: trackedBytes,
	}
}

func maintenanceIdentifier(repo M2Repository, snapshotID string) string {
	digest := sha256.Sum256([]byte(repo.ResolvedRoot + "\x00" + repo.StateFingerprint + "\x00" + snapshotID))
	return "maintenance-" + hex.EncodeToString(digest[:12])
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func safeName(value string) string {
	value = unsafeName.ReplaceAllString(value, "-")
	return strings.Trim(value, "-")
}

func pathWithinRepository(root, candidate string) (bool, error) {
	resolvedRoot, err := canonicalPath(root)
	if err != nil {
		return false, err
	}
	resolved, err := canonicalPath(candidate)
	if err != nil {
		return false, err
	}
	if !strings.EqualFold(filepath.VolumeName(resolvedRoot), filepath.VolumeName(resolved)) {
		return false, nil
	}
	relative, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil {
		return false, err
	}
	return relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)), nil
}

func canonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	cursor := filepath.Clean(absolute)
	suffix := []string{}
	for {
		resolved, err := filepath.EvalSymlinks(cursor)
		if err == nil {
			for index := len(suffix) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, suffix[index])
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(cursor)
		if parent == cursor {
			return "", err
		}
		suffix = append(suffix, filepath.Base(cursor))
		cursor = parent
	}
}
