// Package taskbenchmark binds task-specific outcomes to frozen source,
// transport and assessment evidence independently of token efficiency.
package taskbenchmark

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jeonghanlee/repocue/docs/schema"
	"github.com/jeonghanlee/repocue/internal/cue"
	"github.com/jeonghanlee/repocue/internal/taskcontext"
)

const (
	Version           = "repocue/task-benchmark-1"
	AnswerVersion     = "repocue/task-answer-1"
	RunnerVersion     = "repocue/task-runner-1"
	EvaluationVersion = "repocue/task-evaluation-1"
)

// OutputSchema keeps the external provider contract self-contained while the
// canonical answer schema retains its local public reference.
func OutputSchema() ([]byte, error) {
	answer, err := schema.Contract("task-answer-v1.schema.json")
	if err != nil {
		return nil, err
	}
	context, err := schema.Contract("task-context-v1.schema.json")
	if err != nil {
		return nil, err
	}
	var document, reference map[string]any
	if err := json.Unmarshal(answer, &document); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(context, &reference); err != nil {
		return nil, err
	}
	source := reference["$defs"].(map[string]any)["reference"]
	var replace func(any) any
	replace = func(value any) any {
		switch value := value.(type) {
		case map[string]any:
			if value["$ref"] == "task-context-v1.schema.json#/$defs/reference" {
				return source
			}
			for key, child := range value {
				value[key] = replace(child)
			}
		case []any:
			for index, child := range value {
				value[index] = replace(child)
			}
		}
		return value
	}
	document = replace(document).(map[string]any)
	return json.MarshalIndent(document, "", "  ")
}

type Check struct {
	ID             string   `json:"id"`
	Command        string   `json:"command"`
	ExpectedExit   int      `json:"expected_exit"`
	OutputContains []string `json:"output_contains"`
}

type Policy struct {
	ID            string  `json:"id"`
	RequiredPairs int     `json:"required_pairs"`
	Rubric        string  `json:"rubric"`
	Checks        []Check `json:"checks"`
}

type Spec struct {
	SchemaVersion string   `json:"schema_version"`
	ID            string   `json:"id"`
	TaskKind      string   `json:"task_kind"`
	ContextFile   string   `json:"context_file"`
	Targets       []string `json:"targets"`
	MaxTokens     int      `json:"max_tokens"`
	Prompt        string   `json:"prompt"`
	Policy        *Policy  `json:"policy"`
}

type Settings struct {
	CodexBinary     string `json:"codex_binary"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
	SpeedTier       string `json:"speed_tier"`
}

type Rates struct {
	ID          string  `json:"id"`
	AsOf        string  `json:"as_of"`
	Unit        string  `json:"unit"`
	Model       string  `json:"model"`
	SpeedTier   string  `json:"speed_tier"`
	Input       float64 `json:"input"`
	CachedInput float64 `json:"cached_input"`
	Output      float64 `json:"output"`
}

type Claim struct {
	Text       string                  `json:"text"`
	SourceRefs []taskcontext.Reference `json:"source_refs"`
}

type Answer struct {
	SchemaVersion string                  `json:"schema_version"`
	TaskID        string                  `json:"task_id"`
	TaskKind      string                  `json:"task_kind"`
	RequiredIDs   []string                `json:"required_ids"`
	SourceRefs    []taskcontext.Reference `json:"source_refs"`
	Claims        []Claim                 `json:"claims"`
	Checks        []string                `json:"checks"`
	Uncertainties []string                `json:"uncertainties"`
}

type Usage struct {
	Turn                  int    `json:"turn"`
	InputTokens           *int64 `json:"input_tokens"`
	CachedInputTokens     *int64 `json:"cached_input_tokens"`
	OutputTokens          *int64 `json:"output_tokens"`
	ReasoningOutputTokens *int64 `json:"reasoning_output_tokens"`
}

type Tool struct {
	ID             *string `json:"id"`
	Command        string  `json:"command"`
	Status         *string `json:"status"`
	ExitCode       *int    `json:"exit_code"`
	Output         *string `json:"output"`
	OutputComplete bool    `json:"output_complete"`
}

type RunnerRequest struct {
	SchemaVersion      string   `json:"schema_version"`
	Kind               string   `json:"kind"`
	AttemptID          string   `json:"attempt_id"`
	Repository         string   `json:"repository"`
	TaskFile           string   `json:"task_file"`
	CueFile            string   `json:"cue_file"`
	PromptFile         string   `json:"prompt_file"`
	OutputSchemaFile   string   `json:"output_schema_file"`
	OutputSchemaDigest string   `json:"output_schema_digest"`
	AttemptDirectory   string   `json:"attempt_directory"`
	Settings           Settings `json:"settings"`
}

type Transport struct {
	SchemaVersion      string   `json:"schema_version"`
	Kind               string   `json:"kind"`
	AttemptID          string   `json:"attempt_id"`
	CodexVersion       string   `json:"codex_version"`
	DurationMS         float64  `json:"duration_ms"`
	PromptDigest       *string  `json:"prompt_digest"`
	CueDigest          *string  `json:"cue_digest"`
	OutputSchemaDigest *string  `json:"output_schema_digest"`
	Argv               []string `json:"argv"`
	Status             string   `json:"status"`
	UsageEvents        []Usage  `json:"usage_events"`
	UsageComplete      bool     `json:"usage_complete"`
	Commands           []Tool   `json:"commands"`
	FinalResponse      *Answer  `json:"final_response"`
	Error              *string  `json:"error"`
}

type Outcome struct {
	Status string `json:"status"`
	Value  *bool  `json:"value"`
	Reason string `json:"reason"`
}

type Provision struct {
	SourceID string  `json:"source_id"`
	Channel  string  `json:"channel"`
	ToolID   *string `json:"tool_id"`
}

type Delivery struct {
	Status          string      `json:"status"`
	Complete        bool        `json:"complete"`
	RequiredSources int         `json:"required_sources"`
	Provided        []Provision `json:"provided"`
}

type Assessment struct {
	TaskID           string                  `json:"task_id"`
	PolicyDigest     string                  `json:"policy_digest"`
	AttemptID        string                  `json:"attempt_id"`
	AnswerDigest     string                  `json:"answer_digest"`
	BasisFingerprint string                  `json:"basis_fingerprint"`
	Assessor         string                  `json:"assessor"`
	Rubric           string                  `json:"rubric"`
	SourceRefs       []taskcontext.Reference `json:"source_refs"`
	Correct          bool                    `json:"correct"`
	Compliant        *bool                   `json:"compliant"`
}

func Decode(serialized []byte) (Spec, error) {
	var spec Spec
	if err := taskcontext.DecodeStrict(serialized, "task-benchmark-v1.schema.json", &spec); err != nil {
		return spec, err
	}
	if err := taskcontext.ValidatePath(spec.ContextFile, false); err != nil {
		return spec, err
	}
	for _, target := range spec.Targets {
		if err := taskcontext.ValidatePath(target, true); err != nil {
			return spec, err
		}
	}
	if spec.Policy != nil {
		seenIDs, seenCommands := map[string]bool{}, map[string]bool{}
		for _, check := range spec.Policy.Checks {
			if seenIDs[check.ID] || seenCommands[check.Command] {
				return spec, errors.New("policy check IDs and commands must be unique")
			}
			seenIDs[check.ID], seenCommands[check.Command] = true, true
		}
		if spec.TaskKind == "verification" && len(spec.Policy.Checks) == 0 {
			return spec, errors.New("verification policy requires actual checks")
		}
	}
	return spec, nil
}

func PolicyDigest(spec Spec) string {
	if spec.Policy == nil {
		return ""
	}
	serialized, _ := json.Marshal(spec.Policy)
	return taskcontext.Digest(serialized)
}

func ValidateRates(rates Rates, now time.Time) error {
	serialized, err := json.Marshal(rates)
	if err != nil {
		return err
	}
	if err := taskcontext.DecodeStrict(serialized, "task-benchmark-v1.schema.json#/$defs/rates", &rates); err != nil {
		return err
	}
	date, err := time.Parse("2006-01-02", rates.AsOf)
	if err != nil || date.After(now.UTC()) {
		return errors.New("credit rates require a nonfuture date")
	}
	return nil
}

func Sources(expected cue.TaskEnvelope) ([]taskcontext.Source, error) {
	byID := map[string]taskcontext.Source{}
	for _, source := range expected.Content.Sources {
		reference := taskcontext.Reference{Path: source.Path, ContentDigest: source.ContentDigest, Lines: source.Lines}
		if _, exists := byID[source.ID]; exists || source.ID != taskcontext.ReferenceID(reference) ||
			taskcontext.Digest([]byte(source.Text)) != source.TextDigest {
			return nil, errors.New("task source identity is inconsistent")
		}
		byID[source.ID] = source
	}
	items := map[string]cue.TaskItem{}
	for _, item := range expected.Content.Items {
		if _, exists := items[item.ID]; exists {
			return nil, errors.New("duplicate task item")
		}
		items[item.ID] = item
	}
	result := map[string]taskcontext.Source{}
	ids := map[string]bool{}
	for _, id := range expected.Content.RequiredIDs {
		if ids[id] {
			return nil, errors.New("duplicate required ID")
		}
		ids[id] = true
		item, found := items[id]
		if !found || !item.Required {
			return nil, errors.New("required item not delivered")
		}
		for _, sourceID := range append([]string{item.SourceID}, item.CheckSourceIDs...) {
			source, found := byID[sourceID]
			if !found {
				return nil, errors.New("required source not delivered")
			}
			result[sourceID] = source
		}
	}
	for _, item := range items {
		if item.Required && !ids[item.ID] {
			return nil, errors.New("required item missing from accounting")
		}
	}
	sources := make([]taskcontext.Source, 0, len(result))
	for _, source := range result {
		sources = append(sources, source)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].ID < sources[j].ID })
	return sources, nil
}

// DeliveryEvidence accepts exact task cue bytes actually sent to stdin, or
// complete tool output matching a pinned full file. Neither proves compliance.
func DeliveryEvidence(expected cue.TaskEnvelope, actualCue, prompt []byte, transport Transport) (Delivery, error) {
	sources, err := Sources(expected)
	if err != nil {
		return Delivery{}, err
	}
	delivery := Delivery{Status: "not_observed", RequiredSources: len(sources), Provided: []Provision{}}
	provided := map[string]Provision{}
	if bytes.Contains(prompt, actualCue) {
		actual, err := cue.DecodeTask(actualCue)
		if err == nil && actual.Content.Declaration == expected.Content.Declaration &&
			actual.Basis == expected.Basis && sameIDs(actual.Content.RequiredIDs, expected.Content.RequiredIDs) &&
			actual.Content.Request.TaskKind == expected.Content.Request.TaskKind &&
			sameIDs(actual.Content.Request.Targets, expected.Content.Request.Targets) {
			actualSources, err := Sources(actual)
			if err != nil {
				return delivery, err
			}
			for _, source := range actualSources {
				provided[source.ID] = Provision{SourceID: source.ID, Channel: "prompt"}
			}
		}
	}
	for _, source := range sources {
		if _, found := provided[source.ID]; found {
			continue
		}
		for _, tool := range transport.Commands {
			if tool.ID != nil && tool.Status != nil && *tool.Status == "completed" &&
				tool.ExitCode != nil && *tool.ExitCode == 0 && tool.Output != nil && tool.OutputComplete &&
				taskcontext.Digest([]byte(*tool.Output)) == source.ContentDigest {
				provided[source.ID] = Provision{SourceID: source.ID, Channel: "tool", ToolID: tool.ID}
				break
			}
		}
	}
	// Prompt provisions are enumerated through the frozen required source set.
	delivery.Provided = []Provision{}
	for _, source := range sources {
		if p, ok := provided[source.ID]; ok {
			delivery.Provided = append(delivery.Provided, p)
		}
	}
	delivery.Complete = len(delivery.Provided) == len(sources)
	if delivery.Complete {
		delivery.Status = "observed"
	}
	return delivery, nil
}

func Unknown(reason string) Outcome { return Outcome{Status: "not_observed", Reason: reason} }
func Result(value bool, reason string) Outcome {
	return Outcome{Status: "derived", Value: &value, Reason: reason}
}

func Score(spec Spec, expected cue.TaskEnvelope, transport Transport, assessment *Assessment, basis string) (Outcome, Outcome, error) {
	compliance := Unknown("semantic compliance requires a source-backed assessment")
	if spec.Policy == nil {
		return Unknown("no pre-execution policy"), compliance, nil
	}
	if transport.Status != "completed" || transport.FinalResponse == nil {
		return Unknown("no complete successful transport"), compliance, nil
	}
	answer := transport.FinalResponse
	if answer.TaskID != spec.ID || answer.TaskKind != spec.TaskKind {
		return Result(false, "answer task identity mismatch"), compliance, nil
	}
	sources, err := Sources(expected)
	if err != nil {
		return Outcome{}, compliance, err
	}
	if assessment != nil {
		encoded, _ := json.Marshal(answer)
		if assessment.TaskID != spec.ID || assessment.PolicyDigest != PolicyDigest(spec) ||
			assessment.AttemptID != transport.AttemptID || assessment.AnswerDigest != taskcontext.Digest(encoded) ||
			assessment.BasisFingerprint != basis || assessment.Assessor == "" || assessment.Rubric != spec.Policy.Rubric ||
			!sameReferences(assessment.SourceRefs, sources) {
			return Outcome{}, compliance, errors.New("assessment does not bind to the frozen task, answer, policy and sources")
		}
		if assessment.Compliant != nil {
			compliance = Result(*assessment.Compliant, "bound source-backed assessment")
		}
	}
	switch spec.TaskKind {
	case "explanation":
		if assessment == nil {
			return Unknown("explanation needs independent source-backed assessment"), compliance, nil
		}
		if len(answer.Claims) == 0 {
			return Result(false, "explanation contains no claims"), compliance, nil
		}
		known := map[string]bool{}
		for _, source := range expected.Content.Sources {
			known[source.ID] = true
		}
		for _, claim := range answer.Claims {
			if len(claim.SourceRefs) == 0 {
				return Result(false, "claim has no original-source citation"), compliance, nil
			}
			for _, reference := range claim.SourceRefs {
				if !known[taskcontext.ReferenceID(reference)] {
					return Result(false, "claim cites a source outside the frozen cue"), compliance, nil
				}
			}
		}
		return Result(assessment.Correct, "bound source-backed explanation assessment"), compliance, nil
	case "discovery":
		return Result(sameIDs(answer.RequiredIDs, expected.Content.RequiredIDs) && sameReferences(answer.SourceRefs, sources), "exact required item and source set"), compliance, nil
	case "verification":
		for _, check := range spec.Policy.Checks {
			found := false
			for _, tool := range transport.Commands {
				if tool.Command != check.Command {
					continue
				}
				if tool.ID == nil || tool.Status == nil || *tool.Status != "completed" ||
					tool.ExitCode == nil || tool.Output == nil || !tool.OutputComplete {
					continue
				}
				found = true
				if *tool.ExitCode != check.ExpectedExit {
					return Result(false, "required check exit mismatch: "+check.ID), compliance, nil
				}
				for _, text := range check.OutputContains {
					if !strings.Contains(*tool.Output, text) {
						return Result(false, "required check output mismatch: "+check.ID), compliance, nil
					}
				}
				break
			}
			if !found {
				return Unknown("required check lacks complete evidence: " + check.ID), compliance, nil
			}
		}
		return Result(true, "all frozen required checks completed"), compliance, nil
	default:
		return Outcome{}, compliance, fmt.Errorf("unsupported task kind %q", spec.TaskKind)
	}
}

func sameIDs(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	a, b := append([]string{}, left...), append([]string{}, right...)
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] != b[i] || (i > 0 && a[i] == a[i-1]) {
			return false
		}
	}
	return true
}

func sameReferences(references []taskcontext.Reference, sources []taskcontext.Source) bool {
	ids := make([]string, 0, len(references))
	for _, reference := range references {
		if err := taskcontext.ValidateReference(reference); err != nil {
			return false
		}
		ids = append(ids, taskcontext.ReferenceID(reference))
	}
	expected := make([]string, 0, len(sources))
	for _, source := range sources {
		expected = append(expected, source.ID)
	}
	return sameIDs(ids, expected)
}
