package codexadapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeonghanlee/repocue/internal/taskbenchmark"
)

func TestTaskEventPresenceAndTerminalFailure(t *testing.T) {
	answer := `{"schema_version":"repocue/task-answer-1","task_id":"fixture","task_kind":"discovery","required_ids":[],"source_refs":[],"claims":[],"checks":[],"uncertainties":[]}`
	for _, mode := range []string{"complete", "late-json", "open-turn", "provider-failure", "missing-answer"} {
		t.Run(mode, func(t *testing.T) {
			var events strings.Builder
			events.WriteString("{\"type\":\"turn.started\"}\n")
			events.WriteString("{\"type\":\"item.completed\",\"item\":{\"id\":\"read-1\",\"type\":\"command_execution\",\"command\":\"cat original\",\"status\":\"completed\",\"exit_code\":0,\"aggregated_output\":\"Original\\n\"}}\n")
			if mode != "missing-answer" {
				encoded := strings.ReplaceAll(strings.ReplaceAll(answer, "\\", "\\\\"), "\"", "\\\"")
				events.WriteString("{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"" + encoded + "\"}}\n")
			}
			events.WriteString("{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":0,\"output_tokens\":0}}\n")
			switch mode {
			case "late-json":
				events.WriteString("{invalid\n")
			case "open-turn":
				events.WriteString("{\"type\":\"turn.started\"}\n")
			case "provider-failure":
				events.WriteString("{\"type\":\"turn.failed\"}\n")
			}
			name := filepath.Join(t.TempDir(), "provider.jsonl")
			if err := os.WriteFile(name, []byte(events.String()), 0600); err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(name)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			result := taskbenchmark.Transport{}
			err = parseTaskEvents(file, &result)
			if (err == nil) != (mode == "complete") || len(result.UsageEvents) != 1 || len(result.Commands) != 1 {
				t.Fatalf("captured partial stream: %v %+v", err, result)
			}
			if result.UsageEvents[0].CachedInputTokens != nil || result.Commands[0].OutputComplete ||
				result.UsageEvents[0].InputTokens == nil || *result.UsageEvents[0].InputTokens != 0 {
				t.Fatal("absent fields confused with complete output or measured zero")
			}
		})
	}
}
