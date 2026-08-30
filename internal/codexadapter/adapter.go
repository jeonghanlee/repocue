package codexadapter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jeonghanlee/repocue/internal/benchmark"
	"github.com/jeonghanlee/repocue/internal/evaluation"
)

const AdapterVersion = "2"

var (
	gitCommandPattern     = regexp.MustCompile(`(^|[\s;&|()'\"])git(?:\s|$)`)
	searchCommandPattern  = regexp.MustCompile(`(^|[\s;&|()'\"])(rg|grep|find|fd|ls|tree)(?:\s|$)`)
	readCommandPattern    = regexp.MustCompile(`(^|[\s;&|()'\"])(cat|sed|head|tail|less|bat|wc)(?:\s|$)`)
	contentCommandPattern = regexp.MustCompile(`(^|[\s;&|()'\"])(cat|sed|head|tail|less|bat|wc|rg|grep)(?:\s|$)`)
)

type Config struct {
	Condition           string
	RunIndex            int
	Repository          string
	TaskFile            string
	CueFile             string
	Snapshot            string
	Head                string
	BenchmarkVersion    string
	OutputSchemaVersion string
	OutputSchemaFile    string
	CodexBinary         string
	Model               string
	ReasoningEffort     string
	Sandbox             string
}

type codexUsage struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
}

type codexItem struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Text    string `json:"text"`
}

type codexEvent struct {
	Type  string      `json:"type"`
	Usage *codexUsage `json:"usage,omitempty"`
	Item  *codexItem  `json:"item,omitempty"`
}

func ConfigFromEnvironment() (Config, error) {
	runIndex, err := strconv.Atoi(os.Getenv("REPOCUE_EVAL_RUN_INDEX"))
	if err != nil || runIndex < 1 {
		return Config{}, errors.New("evaluation run index must be positive")
	}
	config := Config{
		Condition: os.Getenv("REPOCUE_EVAL_CONDITION"), RunIndex: runIndex,
		Repository: os.Getenv("REPOCUE_EVAL_REPOSITORY"),
		TaskFile:   os.Getenv("REPOCUE_EVAL_TASK_FILE"), CueFile: os.Getenv("REPOCUE_EVAL_CUE_FILE"),
		Snapshot: os.Getenv("REPOCUE_EVAL_SNAPSHOT"), Head: os.Getenv("REPOCUE_EVAL_HEAD"),
		BenchmarkVersion:    os.Getenv("REPOCUE_EVAL_BENCHMARK_VERSION"),
		OutputSchemaVersion: os.Getenv("REPOCUE_EVAL_OUTPUT_SCHEMA_VERSION"),
		OutputSchemaFile:    os.Getenv("REPOCUE_CODEX_OUTPUT_SCHEMA_FILE"),
		CodexBinary:         os.Getenv("REPOCUE_CODEX_BIN"), Model: os.Getenv("REPOCUE_CODEX_MODEL"),
		ReasoningEffort: os.Getenv("REPOCUE_CODEX_REASONING_EFFORT"), Sandbox: "read-only",
	}
	if config.CodexBinary == "" {
		config.CodexBinary = "codex"
	}
	if os.Getenv("REPOCUE_EVAL_SCHEMA") != evaluation.M2RunnerSchemaVersion {
		return Config{}, errors.New("unsupported RepoCue runner schema")
	}
	if os.Getenv("REPOCUE_EVAL_READ_ONLY") != "1" {
		return Config{}, errors.New("Codex evaluation requires read-only mode")
	}
	if !evaluation.ValidM2Condition(config.Condition) {
		return Config{}, errors.New("unsupported evaluation condition")
	}
	if config.Repository == "" || config.TaskFile == "" || config.Snapshot == "" ||
		config.OutputSchemaFile == "" || config.Model == "" || config.ReasoningEffort == "" {
		return Config{}, errors.New("Codex evaluation configuration is incomplete")
	}
	if config.BenchmarkVersion != benchmark.Version || config.OutputSchemaVersion != benchmark.AnswerSchemaVersion {
		return Config{}, errors.New("unsupported benchmark or output schema version")
	}
	if config.Condition != evaluation.ConditionDirect && config.CueFile == "" {
		return Config{}, errors.New("assisted evaluation requires a RepoCue cue")
	}
	if config.Condition == evaluation.ConditionDirect && config.CueFile != "" {
		return Config{}, errors.New("direct evaluation must not receive a RepoCue cue")
	}
	return config, nil
}

func Run(ctx context.Context, config Config) (evaluation.M2RunnerObservation, error) {
	prompt, err := buildPrompt(config)
	if err != nil {
		return evaluation.M2RunnerObservation{}, err
	}
	codexVersion, err := commandVersion(ctx, config.CodexBinary)
	if err != nil {
		return evaluation.M2RunnerObservation{}, err
	}
	command := exec.CommandContext(ctx, config.CodexBinary,
		"exec", "--ephemeral", "--ignore-user-config", "--ignore-rules", "--json", "--color", "never",
		"--sandbox", config.Sandbox, "--model", config.Model,
		"-c", fmt.Sprintf("model_reasoning_effort=%q", config.ReasoningEffort),
		"--cd", config.Repository, "--output-schema", config.OutputSchemaFile, "-",
	)
	command.Dir = config.Repository
	command.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	started := time.Now()
	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		return evaluation.M2RunnerObservation{}, fmt.Errorf("run Codex: %w: %s", err, detail)
	}
	duration := time.Since(started)
	observation, err := parseEvents(stdout.Bytes(), config, codexVersion, duration)
	if err != nil {
		return evaluation.M2RunnerObservation{}, err
	}
	return observation, nil
}

func buildPrompt(config Config) (string, error) {
	task, err := os.ReadFile(config.TaskFile)
	if err != nil {
		return "", fmt.Errorf("read benchmark task: %w", err)
	}
	var prompt strings.Builder
	prompt.Write(task)
	if len(task) == 0 || task[len(task)-1] != '\n' {
		prompt.WriteByte('\n')
	}
	if config.Condition != evaluation.ConditionDirect {
		serializedCue, err := os.ReadFile(config.CueFile)
		if err != nil {
			return "", fmt.Errorf("read RepoCue context: %w", err)
		}
		prompt.WriteString("\nUse the supplied RepoCue context first. Inspect the repository whenever the context is incomplete, uncertain, or insufficient.\n\nRepoCue context:\n")
		prompt.Write(serializedCue)
		prompt.WriteByte('\n')
	}
	return prompt.String(), nil
}

func commandVersion(ctx context.Context, binary string) (string, error) {
	output, err := exec.CommandContext(ctx, binary, "--version").Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return "", fmt.Errorf("read Codex CLI version: %w: %s", err, strings.TrimSpace(string(exitError.Stderr)))
		}
		return "", fmt.Errorf("read Codex CLI version: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func parseEvents(serialized []byte, config Config, codexVersion string, duration time.Duration) (evaluation.M2RunnerObservation, error) {
	scanner := bufio.NewScanner(bytes.NewReader(serialized))
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	usageEvents := []evaluation.UsageEvent{}
	commands := []evaluation.CommandObservation{}
	toolCalls := int64(0)
	var finalText string
	for scanner.Scan() {
		var event codexEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return evaluation.M2RunnerObservation{}, fmt.Errorf("decode Codex JSONL event: %w", err)
		}
		if event.Type == "turn.completed" && event.Usage != nil {
			usageEvents = append(usageEvents, evaluation.UsageEvent{
				Turn: len(usageEvents) + 1, InputTokens: event.Usage.InputTokens,
				CachedInputTokens: event.Usage.CachedInputTokens, OutputTokens: event.Usage.OutputTokens,
				ReasoningOutputTokens: event.Usage.ReasoningOutputTokens,
			})
		}
		if event.Type != "item.completed" || event.Item == nil {
			continue
		}
		switch event.Item.Type {
		case "command_execution":
			toolCalls++
			commands = append(commands, classifyCommand(config.Repository, event.Item.Command))
		case "mcp_tool_call", "web_search":
			toolCalls++
		case "agent_message":
			finalText = event.Item.Text
		}
	}
	if err := scanner.Err(); err != nil {
		return evaluation.M2RunnerObservation{}, err
	}
	if len(usageEvents) == 0 {
		return evaluation.M2RunnerObservation{}, errors.New("Codex JSONL omitted turn.completed usage")
	}
	if finalText == "" || !json.Valid([]byte(finalText)) {
		return evaluation.M2RunnerObservation{}, errors.New("Codex JSONL omitted a valid structured final response")
	}
	return buildObservation(config, codexVersion, duration, usageEvents, toolCalls, commands, json.RawMessage(finalText)), nil
}

func buildObservation(config Config, codexVersion string, duration time.Duration, usageEvents []evaluation.UsageEvent, toolCalls int64, commands []evaluation.CommandObservation, final json.RawMessage) evaluation.M2RunnerObservation {
	commandCount := int64(len(commands))
	gitCalls := int64(0)
	searchCalls := int64(0)
	readFiles := map[string]struct{}{}
	for _, command := range commands {
		if strings.Contains(command.Classification, "git") {
			gitCalls++
		}
		if strings.Contains(command.Classification, "filesystem_search") {
			searchCalls++
		}
		for _, path := range command.FilesRead {
			readFiles[path] = struct{}{}
		}
	}
	files := make([]string, 0, len(readFiles))
	var bytesRead int64
	for path := range readFiles {
		files = append(files, path)
		if info, err := os.Stat(filepath.Join(config.Repository, filepath.FromSlash(path))); err == nil {
			bytesRead += info.Size()
		}
	}
	sort.Strings(files)
	fileCount := int64(len(files))
	var inputTokens, cachedInputTokens, outputTokens, reasoningOutputTokens int64
	for _, usage := range usageEvents {
		inputTokens += usage.InputTokens
		cachedInputTokens += usage.CachedInputTokens
		outputTokens += usage.OutputTokens
		reasoningOutputTokens += usage.ReasoningOutputTokens
	}
	totalTokens := inputTokens + outputTokens
	durationMS := float64(duration) / float64(time.Millisecond)
	metrics := evaluation.M2RunnerMetrics{
		InputTokens: &inputTokens, CachedInputTokens: &cachedInputTokens,
		OutputTokens: &outputTokens, ReasoningOutputTokens: &reasoningOutputTokens,
		TotalTokens: &totalTokens, ExecutionDurationMS: &durationMS, CommandExecutions: &commandCount,
		RepositoryFilesNamed: &fileCount, NamedFileSizeProxyBytes: &bytesRead, GitCalls: &gitCalls,
		FilesystemSearchCalls: &searchCalls, ToolCalls: &toolCalls,
		Statuses: map[string]string{
			"input_tokens": "observed", "cached_input_tokens": "observed", "output_tokens": "observed",
			"reasoning_output_tokens": "observed", "total_tokens": "derived",
			"execution_duration_ms": "observed", "command_executions": "observed",
			"repository_files_named": "derived", "named_file_size_proxy_bytes": "derived",
			"git_calls": "derived", "filesystem_search_calls": "derived", "tool_calls": "observed",
		},
	}
	if config.Condition != evaluation.ConditionDirect {
		fallbackCommands := int64(0)
		for _, command := range commands {
			if command.Classification != "other" {
				fallbackCommands++
			}
		}
		metrics.FallbackRepositoryCommands = &fallbackCommands
		metrics.FallbackRepositoryFilesNamed = &fileCount
		metrics.FallbackNamedFileSizeProxyBytes = &bytesRead
		metrics.Statuses["fallback_repository_commands"] = "derived"
		metrics.Statuses["fallback_repository_files_named"] = "derived"
		metrics.Statuses["fallback_named_file_size_proxy_bytes"] = "derived"
	}
	return evaluation.M2RunnerObservation{
		SchemaVersion: evaluation.M2RunnerSchemaVersion, Condition: config.Condition, RunIndex: config.RunIndex,
		Metadata: evaluation.RunnerMetadata{
			Adapter: "codex-cli", AdapterVersion: AdapterVersion, CodexVersion: codexVersion,
			Model: config.Model, ReasoningEffort: config.ReasoningEffort, Sandbox: config.Sandbox,
			BenchmarkVersion: config.BenchmarkVersion, OutputSchemaVersion: config.OutputSchemaVersion,
			RepositoryHead: config.Head, RepoCueSnapshot: config.Snapshot,
		},
		Metrics: metrics, UsageEvents: usageEvents, TokenizerCounts: []evaluation.TokenizerCount{}, Commands: commands,
		FinalResponse: final, Findings: []evaluation.ContextFinding{},
		Limitations: []string{"named_file_size_proxy_bytes sums current sizes of command-visible repository paths; it is not observed filesystem I/O"},
	}
}

func classifyCommand(root, command string) evaluation.CommandObservation {
	classifications := []string{}
	if gitCommandPattern.MatchString(command) {
		classifications = append(classifications, "git")
	}
	if searchCommandPattern.MatchString(command) {
		classifications = append(classifications, "filesystem_search")
	}
	if readCommandPattern.MatchString(command) {
		classifications = append(classifications, "filesystem_read")
	}
	if len(classifications) == 0 {
		classifications = append(classifications, "other")
	}
	files := explicitFiles(root, command)
	return evaluation.CommandObservation{
		Command: command, Classification: strings.Join(classifications, "+"), FilesRead: files,
	}
}

func explicitFiles(root, command string) []string {
	if !contentCommandPattern.MatchString(command) {
		return []string{}
	}
	seen := map[string]struct{}{}
	for _, token := range strings.Fields(command) {
		candidate := strings.Trim(token, "'\"`;|&(){}[],:=")
		if candidate == "" || strings.HasPrefix(candidate, "-") || filepath.IsAbs(candidate) {
			continue
		}
		candidate = filepath.Clean(candidate)
		if candidate == "." || candidate == ".." || strings.HasPrefix(candidate, ".."+string(filepath.Separator)) {
			continue
		}
		full := filepath.Join(root, candidate)
		info, err := os.Stat(full)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		relative, err := filepath.Rel(root, full)
		if err == nil {
			seen[filepath.ToSlash(relative)] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for path := range seen {
		result = append(result, path)
	}
	sort.Strings(result)
	return result
}
