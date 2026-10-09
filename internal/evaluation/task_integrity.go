package evaluation

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/jeonghanlee/repocue/internal/cue"
	"github.com/jeonghanlee/repocue/internal/repository"
	"github.com/jeonghanlee/repocue/internal/taskbenchmark"
	"github.com/jeonghanlee/repocue/internal/taskcontext"
)

func sameTaskJSON(left, right any) bool {
	a, errA := json.Marshal(left)
	b, errB := json.Marshal(right)
	return errA == nil && errB == nil && bytes.Equal(a, b)
}

func validTaskBasis(basis *repository.ContentEvidence) bool {
	if basis == nil || !basis.Complete || basis.Algorithm != repository.EvidenceAlgorithm {
		return false
	}
	identity, _ := json.Marshal([]string{basis.Algorithm, basis.RepositoryID,
		basis.StatusDigest, basis.TrackedDigest, basis.UntrackedDigest})
	return taskDigest(identity) == basis.Fingerprint
}

// Retained protocol observations are checked against their captured raw files.
// Integrity is local evidence consistency, not a signature from the provider.
func verifyTaskPair(root string, report TaskReport) error {
	outputSchema, err := readTaskFile(filepath.Join(root, "answer.schema.json"))
	if err != nil || report.OutputSchemaDigest == nil || taskDigest(outputSchema) != *report.OutputSchemaDigest {
		return errors.New("frozen answer schema digest mismatch")
	}
	input, err := readTaskFile(filepath.Join(root, "benchmark.json"))
	if err != nil || taskDigest(input) != report.TaskDigest {
		return errors.New("frozen benchmark digest mismatch")
	}
	spec, err := taskbenchmark.Decode(input)
	if err != nil || !sameTaskJSON(spec, report.Task) || taskbenchmark.PolicyDigest(spec) != report.PolicyDigest ||
		taskRateDigest(report.Rates) != report.RateDigest || !validTaskBasis(report.Basis) {
		return errors.New("frozen task policy, rates or basis mismatch")
	}
	if report.Rates != nil {
		input, err := readTaskFile(filepath.Join(root, "rates.json"))
		var rates taskbenchmark.Rates
		if err != nil || taskcontext.DecodeStrict(input, "task-benchmark-v1.schema.json#/$defs/rates", &rates) != nil ||
			!sameTaskJSON(rates, report.Rates) || taskbenchmark.ValidateRates(rates, report.CreatedAt) != nil {
			return errors.New("frozen rate file mismatch")
		}
	}
	input, err = readTaskFile(filepath.Join(root, "task.json"))
	expected, decodeErr := cue.DecodeTask(input)
	if err != nil || decodeErr != nil || !sameTaskJSON(expected, report.Expected) ||
		expected.Basis.RepositoryDigest != report.Basis.TrackedDigest || expected.Basis.StatusDigest != report.Basis.StatusDigest {
		return errors.New("frozen task cue mismatch")
	}
	if _, err := taskbenchmark.Sources(expected); err != nil {
		return err
	}
	if len(report.CueCosts) != 2 {
		return errors.New("incomplete cue process costs")
	}
	seen := map[string]bool{}
	for _, cost := range report.CueCosts {
		if (cost.Operation != "overview" && cost.Operation != "task") || seen[cost.Operation] ||
			cost.ExitCode == nil || *cost.ExitCode != 0 || cost.Error != nil {
			return errors.New("invalid cue process observation")
		}
		seen[cost.Operation] = true
		input, err := readTaskFile(filepath.Join(root, cost.Operation+".json"))
		if err != nil || cost.OutputDigest != taskDigest(input) || cost.OutputBytes != int64(len(input)) ||
			cost.EstimatedTokens != int64((len(input)+3)/4) {
			return errors.New("cue process bytes or digest mismatch")
		}
	}
	return nil
}

func verifyTaskAttempt(root string, report TaskReport, attempt *TaskAttempt) error {
	for name, digest := range attempt.Artifacts {
		if filepath.Base(name) != name {
			return errors.New("invalid artifact path")
		}
		file, err := os.Open(filepath.Join(root, name))
		if err != nil {
			return err
		}
		actual, err := hashTaskArtifact(file)
		file.Close()
		if err != nil || actual != digest {
			return errors.New("artifact digest mismatch")
		}
	}
	if attempt.Status == "completed" {
		for _, name := range []string{"cue.json", "prompt.txt", "request.json", "runner.stdout", "runner.stderr",
			"submitted-prompt.txt", "answer.schema.json", "invocation.json", "provider.jsonl", "provider.stderr"} {
			if _, present := attempt.Artifacts[name]; !present {
				return errors.New("completed attempt missing captured artifact")
			}
		}
	}
	if attempt.Transport == nil {
		attempt.Eligible = false
		return nil
	}
	raw, err := readTaskFile(filepath.Join(root, "runner.stdout"))
	var transport taskbenchmark.Transport
	if err != nil || taskcontext.DecodeStrict(raw, "task-runner-v1.schema.json#/$defs/observation", &transport) != nil ||
		!sameTaskJSON(transport, attempt.Transport) || transport.AttemptID != attempt.ID {
		return errors.New("captured runner observation mismatch")
	}
	attempt.Metrics = taskMetrics(&transport, report.Rates, *report.Settings, attempt.StartedAt)
	attempt.Eligible = attempt.Status == "completed" && attempt.Error == nil && attempt.BasisError == nil &&
		attempt.FinishedAt != nil && attempt.ExitCode != nil && *attempt.ExitCode == 0 && transport.Status == "completed" &&
		validTaskBasis(attempt.Before) && validTaskBasis(attempt.After) &&
		attempt.Before.Fingerprint == report.Basis.Fingerprint && attempt.After.Fingerprint == report.Basis.Fingerprint
	cueBytes, err := readTaskFile(filepath.Join(root, "cue.json"))
	if err != nil {
		return err
	}
	frozen, err := readTaskFile(filepath.Join(report.Directory, attempt.Arm+".json"))
	if err != nil || !bytes.Equal(cueBytes, frozen) {
		return errors.New("attempt cue differs from frozen arm")
	}
	prompt, err := readTaskFile(filepath.Join(root, "prompt.txt"))
	if err != nil || !bytes.Equal(prompt, taskPrompt(report.Task, cueBytes)) {
		return errors.New("attempt prompt differs from frozen task")
	}
	submitted, err := readTaskFile(filepath.Join(root, "submitted-prompt.txt"))
	if err != nil || transport.PromptDigest == nil || transport.CueDigest == nil ||
		*transport.PromptDigest != taskDigest(submitted) || *transport.CueDigest != taskDigest(cueBytes) ||
		!bytes.Equal(submitted, prompt) {
		return errors.New("submitted prompt or cue mismatch")
	}
	raw, err = readTaskFile(filepath.Join(root, "request.json"))
	var request taskbenchmark.RunnerRequest
	if err != nil || taskcontext.DecodeStrict(raw, "task-runner-v1.schema.json#/$defs/request", &request) != nil ||
		request.AttemptID != attempt.ID || !sameTaskJSON(request.Settings, report.Settings) ||
		request.AttemptDirectory != root || request.TaskFile != filepath.Join(report.Directory, "benchmark.json") ||
		report.OutputSchemaDigest == nil || request.OutputSchemaDigest != *report.OutputSchemaDigest ||
		request.OutputSchemaFile != filepath.Join(report.Directory, "answer.schema.json") {
		return errors.New("captured task settings mismatch")
	}
	if err := verifyTaskSchema(root, report, transport); err != nil {
		return err
	}
	attempt.Delivery, err = taskbenchmark.DeliveryEvidence(*report.Expected, cueBytes, submitted, transport)
	return err
}

func verifyTaskSchema(root string, report TaskReport, transport taskbenchmark.Transport) error {
	captured, err := readTaskFile(filepath.Join(root, "answer.schema.json"))
	if err != nil || report.OutputSchemaDigest == nil || transport.OutputSchemaDigest == nil ||
		taskDigest(captured) != *report.OutputSchemaDigest || *transport.OutputSchemaDigest != *report.OutputSchemaDigest {
		return errors.New("captured answer schema digest mismatch")
	}
	invocation, err := readTaskFile(filepath.Join(root, "invocation.json"))
	var argv []string
	if err != nil || json.Unmarshal(invocation, &argv) != nil || !sameTaskJSON(argv, transport.Argv) {
		return errors.New("captured answer schema invocation mismatch")
	}
	count := 0
	for index, arg := range argv {
		if arg == "--output-schema" {
			if index+1 >= len(argv) || argv[index+1] != filepath.Join(root, "answer.schema.json") {
				return errors.New("captured answer schema path mismatch")
			}
			count++
		}
	}
	if count != 1 {
		return errors.New("captured answer schema argument unavailable")
	}
	return nil
}

func addTaskCueMetrics(metrics map[string]TaskMetric, report TaskReport, arm string) {
	for _, cost := range report.CueCosts {
		if cost.Operation == arm && cost.ExitCode != nil && *cost.ExitCode == 0 && cost.Error == nil {
			metrics["cue_duration_ms"] = taskMeasured(cost.DurationMS, "observed", "ms", "full cue CLI process wall time")
			metrics["cue_bytes"] = taskMeasured(float64(cost.OutputBytes), "observed", "bytes", "actual cue stdout")
			metrics["cue_estimated_tokens"] = taskMeasured(float64(cost.EstimatedTokens), "estimated", "tokens", "ceil(actual cue stdout bytes / 4)")
		}
	}
}
