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
	"time"

	"github.com/jeonghanlee/repocue/internal/taskbenchmark"
	"github.com/jeonghanlee/repocue/internal/taskcontext"
)

const maxTaskEventBytes = 16 * 1024 * 1024

func TaskConfigFromEnvironment() (taskbenchmark.RunnerRequest, error) {
	var config taskbenchmark.RunnerRequest
	if os.Getenv("REPOCUE_EVAL_SCHEMA") != taskbenchmark.RunnerVersion {
		return config, errors.New("unsupported task runner schema")
	}
	serialized, err := os.ReadFile(os.Getenv("REPOCUE_TASK_RUNNER_CONFIG"))
	if err != nil {
		return config, err
	}
	if err := taskcontext.DecodeStrict(serialized, "task-runner-v1.schema.json#/$defs/request", &config); err != nil {
		return config, err
	}
	if !filepath.IsAbs(config.Repository) || !filepath.IsAbs(config.AttemptDirectory) {
		return config, errors.New("task runner requires absolute repository and attempt directory")
	}
	return config, nil
}

// RunTask retains submitted bytes and raw provider streams even when execution,
// cancellation or JSONL decoding fails. No partial usage becomes a full total.
func RunTask(ctx context.Context, config taskbenchmark.RunnerRequest) (result taskbenchmark.Transport, runErr error) {
	started := time.Now()
	result = taskbenchmark.Transport{SchemaVersion: taskbenchmark.RunnerVersion, Kind: "observation",
		AttemptID: config.AttemptID, Status: "failed", Argv: []string{},
		UsageEvents: []taskbenchmark.Usage{}, Commands: []taskbenchmark.Tool{}}
	defer func() {
		result.DurationMS = float64(time.Since(started)) / float64(time.Millisecond)
		if runErr != nil {
			message := runErr.Error()
			result.Error = &message
			result.UsageComplete = false
			if ctx.Err() != nil {
				result.Status = "cancelled"
			}
		}
	}()
	prompt, err := os.ReadFile(config.PromptFile)
	if err != nil {
		return result, err
	}
	cueBytes, err := os.ReadFile(config.CueFile)
	if err != nil {
		return result, err
	}
	promptDigest, cueDigest := taskcontext.Digest(prompt), taskcontext.Digest(cueBytes)
	result.PromptDigest = &promptDigest
	result.CueDigest = &cueDigest
	if err := taskArtifact(config.AttemptDirectory, "submitted-prompt.txt", prompt); err != nil {
		return result, err
	}
	outputSchema, err := os.ReadFile(config.OutputSchemaFile)
	if err != nil {
		return result, err
	}
	outputSchemaDigest := taskcontext.Digest(outputSchema)
	result.OutputSchemaDigest = &outputSchemaDigest
	if outputSchemaDigest != config.OutputSchemaDigest {
		return result, errors.New("frozen answer schema digest mismatch")
	}
	if err := taskArtifact(config.AttemptDirectory, "answer.schema.json", outputSchema); err != nil {
		return result, err
	}
	version, err := commandVersion(ctx, config.Settings.CodexBinary)
	if err != nil {
		return result, err
	}
	result.CodexVersion = version
	args := []string{"exec", "--ephemeral", "--ignore-user-config", "--ignore-rules", "--json", "--color", "never",
		"--sandbox", "read-only", "--model", config.Settings.Model,
		"-c", fmt.Sprintf("model_reasoning_effort=%q", config.Settings.ReasoningEffort)}
	if config.Settings.SpeedTier == "fast" {
		args = append(args, "-c", `service_tier="fast"`)
	}
	args = append(args, "--cd", config.Repository, "--output-schema", filepath.Join(config.AttemptDirectory, "answer.schema.json"), "-")
	result.Argv = append([]string{config.Settings.CodexBinary}, args...)
	invocation, _ := json.Marshal(result.Argv)
	if err := taskArtifact(config.AttemptDirectory, "invocation.json", invocation); err != nil {
		return result, err
	}
	stdout, err := os.OpenFile(filepath.Join(config.AttemptDirectory, "provider.jsonl"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return result, err
	}
	defer stdout.Close()
	stderr, err := os.OpenFile(filepath.Join(config.AttemptDirectory, "provider.stderr"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return result, err
	}
	defer stderr.Close()
	command := exec.CommandContext(ctx, config.Settings.CodexBinary, args...)
	command.Dir = config.Repository
	command.Stdin = bytes.NewReader(prompt)
	command.Stdout = stdout
	command.Stderr = stderr
	execErr := command.Run()
	if err := stdout.Close(); err != nil {
		return result, err
	}
	if err := stderr.Close(); err != nil {
		return result, err
	}
	events, err := os.Open(filepath.Join(config.AttemptDirectory, "provider.jsonl"))
	if err != nil {
		return result, err
	}
	defer events.Close()
	parseErr := parseTaskEvents(events, &result)
	if parseErr != nil {
		result.Status = "invalid"
		return result, parseErr
	}
	if execErr != nil {
		return result, fmt.Errorf("Codex task process: %w", execErr)
	}
	result.Status = "completed"
	return result, nil
}

func taskArtifact(directory, name string, value []byte) error {
	file, err := os.OpenFile(filepath.Join(directory, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(value)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func parseTaskEvents(events *os.File, result *taskbenchmark.Transport) error {
	type item struct {
		ID             *string `json:"id"`
		Type           string  `json:"type"`
		Command        string  `json:"command"`
		Text           string  `json:"text"`
		Status         *string `json:"status"`
		ExitCode       *int    `json:"exit_code"`
		Output         *string `json:"aggregated_output"`
		OutputComplete *bool   `json:"output_complete"`
	}
	type event struct {
		Type  string      `json:"type"`
		Usage *codexUsage `json:"usage"`
		Item  *item       `json:"item"`
	}
	scanner := bufio.NewScanner(events)
	scanner.Buffer(make([]byte, 64*1024), maxTaskEventBytes)
	openTurn := false
	providerFailed := false
	for scanner.Scan() {
		var value event
		if err := json.Unmarshal(scanner.Bytes(), &value); err != nil {
			return fmt.Errorf("task JSONL: %w", err)
		}
		switch value.Type {
		case "turn.started":
			if openTurn {
				return errors.New("overlapping provider turns")
			}
			openTurn = true
		case "turn.failed", "error":
			providerFailed = true
		case "turn.completed":
			openTurn = false
			usage := taskbenchmark.Usage{Turn: len(result.UsageEvents) + 1}
			if value.Usage != nil {
				usage.InputTokens = value.Usage.InputTokens
				usage.CachedInputTokens = value.Usage.CachedInputTokens
				usage.OutputTokens = value.Usage.OutputTokens
				usage.ReasoningOutputTokens = value.Usage.ReasoningOutputTokens
			}
			result.UsageEvents = append(result.UsageEvents, usage)
		case "item.completed":
			if value.Item == nil {
				continue
			}
			switch value.Item.Type {
			case "command_execution":
				complete := value.Item.OutputComplete != nil && *value.Item.OutputComplete
				result.Commands = append(result.Commands, taskbenchmark.Tool{ID: value.Item.ID, Command: value.Item.Command,
					Status: value.Item.Status, ExitCode: value.Item.ExitCode, Output: value.Item.Output, OutputComplete: complete})
			case "agent_message":
				var answer taskbenchmark.Answer
				if err := taskcontext.DecodeStrict([]byte(value.Item.Text), "task-answer-v1.schema.json", &answer); err != nil {
					result.FinalResponse = nil
					return fmt.Errorf("structured task answer: %w", err)
				}
				result.FinalResponse = &answer
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if providerFailed || openTurn || result.FinalResponse == nil {
		return errors.New("provider omitted a complete successful task response")
	}
	result.UsageComplete = len(result.UsageEvents) > 0
	return nil
}
