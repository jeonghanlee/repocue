// This executable replaces only the outer provider boundary in local tests.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func emit(value any) {
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		panic(err)
	}
}

func readJSON(name string, target any) {
	value, err := os.ReadFile(name)
	if err != nil || json.Unmarshal(value, target) != nil {
		panic("fixture input unavailable")
	}
}

func completed(command string, output []byte, code int, complete bool, id string) {
	emit(map[string]any{"type": "item.completed", "item": map[string]any{
		"type": "command_execution", "id": id, "command": command, "status": "completed",
		"exit_code": code, "aggregated_output": string(output), "output_complete": complete}})
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println("codex task-fixture 1")
		return
	}
	if len(os.Args) < 2 || os.Args[1] != "exec" {
		os.Exit(70)
	}
	var request struct {
		AttemptID, Repository, TaskFile, CueFile, AttemptDirectory string
	}
	var fields map[string]json.RawMessage
	readJSON(os.Getenv("REPOCUE_TASK_RUNNER_CONFIG"), &fields)
	for key, target := range map[string]*string{"attempt_id": &request.AttemptID, "repository": &request.Repository,
		"task_file": &request.TaskFile, "cue_file": &request.CueFile, "attempt_directory": &request.AttemptDirectory} {
		if json.Unmarshal(fields[key], target) != nil {
			panic("request field missing")
		}
	}
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(request.AttemptDirectory, "outer-stdin.txt"), stdin, 0600); err != nil {
		panic(err)
	}
	args, _ := json.Marshal(os.Args)
	if err := os.WriteFile(filepath.Join(request.AttemptDirectory, "outer-argv.json"), args, 0600); err != nil {
		panic(err)
	}
	schemaPath := ""
	for index, arg := range os.Args {
		if arg == "--output-schema" && index+1 < len(os.Args) {
			schemaPath = os.Args[index+1]
		}
	}
	outputSchema, err := os.ReadFile(schemaPath)
	if err != nil || !json.Valid(outputSchema) {
		panic("actual output schema unavailable")
	}
	if err := os.WriteFile(filepath.Join(request.AttemptDirectory, "outer-schema.json"), outputSchema, 0600); err != nil {
		panic(err)
	}
	arm := "overview"
	if strings.HasPrefix(request.AttemptID, "attempt-task-") {
		arm = "task"
	}
	log, err := os.OpenFile(os.Getenv("REPOCUE_TEST_EXEC_LOG"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	fmt.Fprintln(log, arm)
	log.Close()
	cueBytes, err := os.ReadFile(request.CueFile)
	if err != nil || !bytes.Contains(stdin, cueBytes) {
		panic("actual stdin lost cue")
	}
	mode := os.Getenv("REPOCUE_TEST_MODE")
	failArm := os.Getenv("REPOCUE_TEST_FAIL_ARM")
	failing := failArm == "" || failArm == arm
	var answer map[string]any
	readJSON(os.Getenv("REPOCUE_TEST_ANSWER_FILE"), &answer)
	emit(map[string]any{"type": "turn.started"})
	usage := map[string]any{"input_tokens": 1000, "cached_input_tokens": 400, "output_tokens": 100}
	if arm == "task" {
		usage["input_tokens"] = 500
		usage["cached_input_tokens"] = 200
	}
	if value := os.Getenv("REPOCUE_TEST_USAGE_JSON"); value != "" {
		var counters map[string][]int64
		if json.Unmarshal([]byte(value), &counters) != nil || len(counters[arm]) != 3 {
			panic("invalid fixture usage")
		}
		usage["input_tokens"], usage["cached_input_tokens"], usage["output_tokens"] = counters[arm][0], counters[arm][1], counters[arm][2]
	}
	if failing && mode == "missing-counter" {
		delete(usage, "cached_input_tokens")
	}
	if failing && mode == "malformed" {
		emit(map[string]any{"type": "turn.completed", "usage": usage})
		fmt.Println("{malformed")
		fmt.Fprintln(os.Stderr, "fixture malformed stream")
		os.Exit(9)
	}
	if failing && mode == "cancel" {
		emit(map[string]any{"type": "turn.completed", "usage": usage})
		if err := os.WriteFile(filepath.Join(request.AttemptDirectory, "outer-ready"), []byte("ready"), 0600); err != nil {
			panic(err)
		}
		time.Sleep(10 * time.Minute)
	}
	if failing && (mode == "mutation" || mode == "basis-unavailable") {
		if mode == "mutation" {
			name := filepath.Join(request.Repository, "docs", "rules.md")
			content, err := os.ReadFile(name)
			if err != nil {
				panic(err)
			}
			if err := os.WriteFile(name, append(content, []byte("Mutation evidence.\n")...), 0644); err != nil {
				panic(err)
			}
		} else if err := os.Rename(filepath.Join(request.Repository, ".git"), filepath.Join(request.Repository, ".git-unavailable")); err != nil {
			panic(err)
		}
		emit(map[string]any{"type": "turn.completed", "usage": usage})
		fmt.Fprintln(os.Stderr, "fixture repository mutation")
		os.Exit(8)
	}
	if mode != "final-only" {
		references, _ := answer["source_refs"].([]any)
		for index, value := range references {
			reference := value.(map[string]any)
			name := reference["path"].(string)
			command := exec.Command("cat", name)
			command.Dir = request.Repository
			output, err := command.Output()
			if err != nil {
				panic(err)
			}
			completed("cat "+name, output, 0, mode != "partial-output", fmt.Sprint("read-", index))
		}
		if answer["task_kind"] == "verification" {
			var benchmark struct {
				Policy *struct {
					Checks []struct{ ID, Command string }
				}
			}
			readJSON(request.TaskFile, &benchmark)
			for _, check := range benchmark.Policy.Checks {
				if check.Command != "go test ./..." {
					panic("fixture check command unsupported")
				}
				command := exec.Command("go", "test", "./...")
				command.Dir = request.Repository
				output, runErr := command.CombinedOutput()
				code := 0
				if runErr != nil {
					code = command.ProcessState.ExitCode()
				}
				name := check.Command
				if mode == "wrong-command" {
					name = "git status"
				}
				completed(name, output, code, mode != "partial-output", check.ID)
			}
		}
	}
	if failing && mode == "wrong-answer" {
		answer["required_ids"] = []string{}
		answer["source_refs"] = []any{}
		usage["input_tokens"] = 1
		usage["cached_input_tokens"] = 0
		usage["output_tokens"] = 1
	}
	encoded, _ := json.Marshal(answer)
	emit(map[string]any{"type": "item.completed", "item": map[string]any{"type": "agent_message", "text": string(encoded)}})
	emit(map[string]any{"type": "turn.completed", "usage": usage})
	if failing && mode == "schema-mutation" {
		if err := os.WriteFile(filepath.Join(filepath.Dir(request.TaskFile), "answer.schema.json"), []byte("{}\n"), 0600); err != nil {
			panic(err)
		}
	}
	if failing && mode == "failure" {
		fmt.Fprintln(os.Stderr, "fixture terminal failure")
		os.Exit(7)
	}
}
