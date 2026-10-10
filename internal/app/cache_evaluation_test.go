package app_test

import (
	"bytes"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jeonghanlee/repocue/internal/cue"
	"github.com/jeonghanlee/repocue/internal/evaluation"
	"github.com/jeonghanlee/repocue/internal/promptcache"
	"github.com/jeonghanlee/repocue/internal/taskbenchmark"
	"github.com/jeonghanlee/repocue/internal/taskcontext"
)

const cacheFixtureKey = "fixture-cache-credential"

func TestCacheEvaluationShippedHelp(t *testing.T) {
	f := taskFixture{binary: buildTaskExecutable(t, "./cmd/repocue")}
	for _, args := range [][]string{{"help", "evaluate-cache"}, {"evaluate-cache", "--help"}, {"evaluate-cache"}} {
		out, stderr, err := cacheRaw(f, nil, "", args...)
		if err != nil || len(stderr) != 0 || bytes.Count(out, []byte("Usage:")) != 1 || !bytes.Contains(out, []byte("<prepare|run|summary>")) {
			t.Fatalf("top-level cache help: %v %s %s", err, stderr, out)
		}
	}
	for _, command := range []string{"prepare", "run", "summary"} {
		out, stderr, err := cacheRaw(f, nil, "", "evaluate-cache", command, "--help")
		if err != nil || len(stderr) != 0 || bytes.Count(out, []byte("Usage:")) != 1 || !bytes.HasPrefix(out, []byte("Usage: repocue evaluate-cache "+command+" [flags]")) || !bytes.Contains(out, []byte("-")) {
			t.Fatalf("cache subcommand help: %v %s %s", err, stderr, out)
		}
	}
}

type cacheHTTPFixture struct {
	server   *httptest.Server
	caFile   string
	mutex    sync.Mutex
	requests [][]byte
	cached   int
	mode     string
	arrived  chan struct{}
	mutate   func()
}

func newCacheHTTPFixture(t *testing.T, mode string) *cacheHTTPFixture {
	t.Helper()
	f := &cacheHTTPFixture{mode: mode, arrived: make(chan struct{}, 1)}
	f.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		if r.Method != "POST" || r.URL.Path != "/v1/messages" || r.Header.Get("anthropic-version") != "2023-06-01" || r.Header.Get("x-api-key") != cacheFixtureKey {
			t.Errorf("invalid actual HTTP request: %s %s", r.Method, r.URL.Path)
		}
		if bytes.Contains(raw, []byte(cacheFixtureKey)) {
			t.Error("credential appears in submitted body")
		}
		var wire promptcache.WireRequest
		if err := json.Unmarshal(raw, &wire); err != nil {
			t.Error(err)
			return
		}
		f.mutex.Lock()
		f.requests = append(f.requests, append([]byte{}, raw...))
		cached := len(wire.Messages) == 1 && len(wire.Messages[0].Content) == 2 && wire.Messages[0].Content[0].CacheControl != nil
		if cached {
			control := wire.Messages[0].Content[0].CacheControl
			if control.Type != "ephemeral" || control.TTL != "5m" || wire.Messages[0].Content[1].CacheControl != nil {
				t.Error("cache boundary or frozen TTL differs from submitted settings")
			}
			f.cached++
		}
		cacheOrdinal := f.cached
		mutate := f.mutate
		f.mutex.Unlock()
		if mutate != nil {
			mutate()
		}
		select {
		case f.arrived <- struct{}{}:
		default:
		}
		if f.mode == "timeout" || f.mode == "interrupt" {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(time.Second):
				return
			}
		}
		if f.mode == "redirect" {
			w.Header().Set("Location", f.server.URL+"/v1/messages")
			w.WriteHeader(307)
			return
		}
		if f.mode == "credential" {
			w.WriteHeader(400)
			w.Write([]byte(cacheFixtureKey))
			return
		}
		answer := cacheBoundaryAnswer(t, wire)
		if f.mode == "wrong" {
			answer.RequiredIDs = []string{}
			answer.SourceIDs = []string{}
		}
		answerBytes, _ := json.Marshal(answer)
		usage := map[string]any{"input_tokens": 1010, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0, "output_tokens": 5}
		if cached {
			usage["input_tokens"] = 10
			if cacheOrdinal == 1 {
				usage["cache_creation_input_tokens"] = 1000
			} else {
				usage["cache_read_input_tokens"] = 1000
			}
		}
		if f.mode == "missing" {
			delete(usage, "cache_read_input_tokens")
		}
		if f.mode == "bad-usage" {
			usage["input_tokens"] = -1
		}
		if f.mode == "mixed" {
			usage = map[string]any{"input_tokens": 10, "cache_creation_input_tokens": 50, "cache_read_input_tokens": 20, "output_tokens": 5,
				"cache_creation": map[string]any{"ephemeral_5m_input_tokens": 30, "ephemeral_1h_input_tokens": 20}}
		}
		if f.mode == "no-reuse" {
			usage = map[string]any{"input_tokens": 1, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0, "output_tokens": 5}
		}
		stop := "end_turn"
		if f.mode == "truncated" {
			stop = "max_tokens"
		}
		response := map[string]any{"id": "msg_fixture", "type": "message", "role": "assistant", "model": wire.Model,
			"stop_reason": stop, "content": []any{map[string]any{"type": "text", "text": string(answerBytes)}}, "usage": usage}
		if f.mode == "thinking" {
			response["content"] = []any{map[string]any{"type": "thinking", "thinking": "", "signature": "fixture-signature"},
				map[string]any{"type": "text", "text": string(answerBytes)}}
		}
		if f.mode == "model" {
			response["model"] = "incompatible-model"
		}
		body, _ := json.Marshal(response)
		if f.mode == "malformed" {
			body = []byte(`{"usage":`)
		}
		if f.mode == "trailing" {
			body = append(body, []byte(` {}`)...)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("request-id", "req_fixture")
		if f.mode == "partial" {
			w.Header().Set("Content-Length", stringInt(len(body)+10))
		}
		if f.mode == "http-error" {
			w.WriteHeader(500)
		}
		w.Write(body)
	}))
	t.Cleanup(f.server.Close)
	f.caFile = filepath.Join(t.TempDir(), "fixture-ca.pem")
	writeFile(t, f.caFile, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: f.server.Certificate().Raw})))
	return f
}

func cacheBoundAssessments(t *testing.T, f taskFixture, report evaluation.CacheReport, summary evaluation.CacheSummary, correct bool) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(report.Directory, "task.json"))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := cue.DecodeTask(raw)
	if err != nil {
		t.Fatal(err)
	}
	sources, err := taskbenchmark.Sources(expected)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, source := range sources {
		ids = append(ids, source.ID)
	}
	assessments := []promptcache.Assessment{}
	for _, item := range summary.Attempts {
		raw, err := os.ReadFile(filepath.Join(report.Directory, "attempt-"+item.StepID, "observation.json"))
		if err != nil {
			t.Fatal(err)
		}
		var o evaluation.CacheObservation
		if err := taskcontext.DecodeStrict(raw, "cache-observation-v1.schema.json", &o); err != nil {
			t.Fatal(err)
		}
		yes := true
		assessments = append(assessments, promptcache.Assessment{StepID: item.StepID, BenchmarkDigest: report.BenchmarkDigest,
			RequestDigest: o.RequestDigest, ResponseDigest: *o.ResponseDigest, AnswerDigest: *item.AnswerDigest,
			BasisFingerprint: report.Basis.Fingerprint, SourceIDs: ids, Assessor: "independent-fixture-input", Rubric: report.Spec.Rubric, Correct: correct, Compliant: &yes})
	}
	name := filepath.Join(f.external, "bound-assessments.json")
	taskWriteJSON(t, name, map[string]any{"schema_version": promptcache.EvaluationVersion, "kind": "assessments", "assessments": assessments})
	return name
}

func TestCacheEvaluationShippedAccountingExplanationAndBasis(t *testing.T) {
	binary := buildTaskExecutable(t, "./cmd/repocue")
	for _, mode := range []string{"mixed", "explanation", "no-reuse", "basis"} {
		t.Run(mode, func(t *testing.T) {
			f := newTaskFixture(t, binary)
			server := newCacheHTTPFixture(t, mode)
			spec := cacheSpec()
			if mode != "no-reuse" {
				spec.Steps = spec.Steps[:1]
			}
			if mode == "explanation" {
				spec.TaskKind = "explanation"
			}
			report := prepareCacheFixture(t, f, spec)
			if mode == "basis" {
				server.mutex.Lock()
				server.mutate = func() { writeFile(t, filepath.Join(f.root, "README.md"), "Concurrent repository basis change.\n") }
				server.mutex.Unlock()
			}
			for _, step := range spec.Steps {
				_, stderr, err := cacheRaw(f, server, cacheFixtureKey, cacheRunArgs(report, step.ID, server)...)
				if (err != nil) != (mode == "basis") {
					t.Fatalf("actual run result: %v %s", err, stderr)
				}
			}
			summary := cacheSummaryFixture(t, f, report, "")
			item := summary.Attempts[0]
			switch mode {
			case "mixed":
				if item.Usage == nil || item.Usage.TotalInput == nil || *item.Usage.TotalInput != 80 || item.EstimatedUSD == nil || *item.EstimatedUSD != "0.0000995" {
					t.Fatalf("adapter-to-summary lifetime accounting: %+v", item)
				}
			case "basis":
				if item.Status != "failed" || item.EstimatedUSD == nil || item.Correctness.Value != nil {
					t.Fatal("post-send basis change concealed or its known cost omitted")
				}
			case "explanation":
				if item.Correctness.Value != nil || item.Compliance.Value != nil {
					t.Fatal("unassessed explanation treated as verified")
				}
				for _, correct := range []bool{true, false} {
					assessment := cacheBoundAssessments(t, f, report, summary, correct)
					observed := cacheSummaryFixture(t, f, report, assessment)
					value := observed.Attempts[0].Correctness.Value
					if value == nil || *value != correct || observed.Attempts[0].Compliance.Value == nil || !*observed.Attempts[0].Compliance.Value {
						t.Fatal("bound explanation assessment ignored")
					}
				}
			case "no-reuse":
				assessment := cacheBoundAssessments(t, f, report, summary, true)
				observed := cacheSummaryFixture(t, f, report, assessment)
				if observed.Pairs[0].DifferenceUSD == nil || observed.Pairs[0].Verdict.Value != nil {
					t.Fatal("requested cache policy became an observed reuse verdict")
				}
			}
			if server.count() != len(spec.Steps) {
				t.Fatal("unexpected provider invocation")
			}
		})
	}
}

func TestCacheEvaluationShippedStrictBenchmarkInputs(t *testing.T) {
	f := newTaskFixture(t, buildTaskExecutable(t, "./cmd/repocue"))
	good, _ := json.Marshal(cacheSpec())
	for _, mode := range []string{"version", "unknown", "duplicate", "trailing", "model", "api", "count", "rate-unit", "rate-date", "rate-model"} {
		t.Run(mode, func(t *testing.T) {
			raw := append([]byte{}, good...)
			var value map[string]any
			json.Unmarshal(raw, &value)
			switch mode {
			case "version":
				value["schema_version"] = "repocue/cache-benchmark-999"
			case "unknown":
				value["unknown"] = true
			case "duplicate":
				raw = append([]byte(`{"id":"duplicate",`), raw[1:]...)
			case "trailing":
				raw = append(raw, []byte(` {}`)...)
			case "model":
				value["provider"].(map[string]any)["model"] = "unsupported-model"
			case "api":
				value["provider"].(map[string]any)["api_version"] = "unsupported-version"
			case "count":
				value["limits"].(map[string]any)["max_requests"] = 1
			case "rate-unit":
				value["rates"].(map[string]any)["unit"] = "credits"
			case "rate-date":
				value["rates"].(map[string]any)["as_of"] = "9999-01-01"
			case "rate-model":
				value["rates"].(map[string]any)["model"] = "another-model"
			}
			if mode != "duplicate" && mode != "trailing" {
				raw, _ = json.Marshal(value)
			}
			input := filepath.Join(f.external, "invalid-benchmark.json")
			writeFile(t, input, string(raw))
			if _, _, err := cacheRaw(f, nil, "", "evaluate-cache", "prepare", "--repository", f.root, "--cache-dir", f.cache, "--benchmark-file", input, "--output-directory", f.external); err == nil {
				t.Fatal("invalid benchmark passed real CLI preflight")
			}
		})
	}
}

func stringInt(value int) string { raw, _ := json.Marshal(value); return string(raw) }
func (f *cacheHTTPFixture) count() int {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return len(f.requests)
}
func cacheBoundaryAnswer(t *testing.T, wire promptcache.WireRequest) promptcache.Answer {
	t.Helper()
	if wire.Stream || len(wire.Messages) != 1 || len(wire.Messages[0].Content) != 2 || wire.Messages[0].Role != "user" ||
		wire.OutputConfig.Format.Type != "json_schema" || wire.System != promptcache.SystemInstruction {
		t.Error("actual request did not preserve the non-streaming source/schema boundary")
		return promptcache.Answer{}
	}
	var prefix struct {
		Items   []cue.TaskItem       `json:"items"`
		Sources []taskcontext.Source `json:"sources"`
	}
	var suffix struct {
		TaskID      string   `json:"task_id"`
		RequiredIDs []string `json:"required_ids"`
	}
	if err := json.Unmarshal([]byte(wire.Messages[0].Content[0].Text), &prefix); err != nil {
		t.Error(err)
	}
	if err := json.Unmarshal([]byte(wire.Messages[0].Content[1].Text), &suffix); err != nil {
		t.Error(err)
	}
	sources := map[string]bool{}
	for _, item := range prefix.Items {
		if item.Required {
			sources[item.SourceID] = true
			for _, id := range item.CheckSourceIDs {
				sources[id] = true
			}
		}
	}
	ids := []string{}
	for id := range sources {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return promptcache.Answer{SchemaVersion: promptcache.AnswerVersion, TaskID: suffix.TaskID, RequiredIDs: suffix.RequiredIDs, SourceIDs: ids,
		Claims: []promptcache.Claim{{Text: "The delivered original sources define the scoped context.", SourceIDs: ids}}, Uncertainties: []string{}}
}

func cacheSpec() promptcache.Spec {
	return promptcache.Spec{SchemaVersion: promptcache.BenchmarkVersion, ID: "cache-fixture", TaskKind: "discovery", ContextFile: "docs/context.json", Targets: []string{"cmd"}, Rubric: "Exact original item and source identities.", MaxContextTokens: 5000,
		Provider: promptcache.Provider{Model: "claude-sonnet-4-6", APIVersion: "2023-06-01", TTL: "5m"},
		Limits:   promptcache.Limits{MaxRequests: 4, MaxOutputTokens: 256, TimeoutMS: 2000, MaxResponseBytes: 1048576},
		Steps: []promptcache.Step{{ID: "base-a", PairID: "pair", Arm: "baseline", Condition: "baseline", Prompt: "Identify the required original context."},
			{ID: "base-b", PairID: "pair", Arm: "baseline", Condition: "baseline", Prompt: "Identify the required original context."},
			{ID: "cache-a", PairID: "pair", Arm: "cache", Condition: "create", Prompt: "Identify the required original context."},
			{ID: "cache-b", PairID: "pair", Arm: "cache", Condition: "reuse", Prompt: "Identify the required original context."}},
		Rates: &promptcache.Rates{ID: "synthetic-only", AsOf: "2026-10-08", Unit: "usd_per_million_tokens", Model: "claude-sonnet-4-6", APIVersion: "2023-06-01", Input: "1", Write5m: "1.25", Write1h: "2", Read: "0.1", Output: "2"}}
}
func cacheRaw(f taskFixture, server *cacheHTTPFixture, key string, args ...string) ([]byte, []byte, error) {
	cmd := exec.Command(f.binary, args...)
	cmd.Env = append(os.Environ(), "ANTHROPIC_API_KEY="+key)
	if server != nil {
		cmd.Env = append(cmd.Env, "SSL_CERT_FILE="+server.caFile)
	}
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	return out.Bytes(), stderr.Bytes(), err
}
func prepareCacheFixture(t *testing.T, f taskFixture, spec promptcache.Spec) evaluation.CacheReport {
	t.Helper()
	input := filepath.Join(f.external, "benchmark-input.json")
	taskWriteJSON(t, input, spec)
	out, stderr, err := cacheRaw(f, nil, "", "evaluate-cache", "prepare", "--repository", f.root, "--cache-dir", f.cache, "--benchmark-file", input, "--output-directory", f.external)
	if err != nil {
		t.Fatalf("real preparation: %v: %s: %s", err, stderr, out)
	}
	var report evaluation.CacheReport
	if err := taskcontext.DecodeStrict(out, "cache-evaluation-v1.schema.json#/$defs/report", &report); err != nil {
		t.Fatal(err)
	}
	return report
}
func cacheRunArgs(report evaluation.CacheReport, id string, server *cacheHTTPFixture) []string {
	return []string{"evaluate-cache", "run", "--input-directory", report.Directory, "--step", id, "--endpoint", server.server.URL + "/v1/messages"}
}
func cacheSummaryFixture(t *testing.T, f taskFixture, report evaluation.CacheReport, assessment string) evaluation.CacheSummary {
	t.Helper()
	args := []string{"evaluate-cache", "summary", "--input-directory", report.Directory}
	if assessment != "" {
		args = append(args, "--assessment-file", assessment)
	}
	out, stderr, err := cacheRaw(f, nil, "", args...)
	if err != nil {
		t.Fatalf("real summary: %v: %s %s", err, stderr, out)
	}
	var summary evaluation.CacheSummary
	if err := taskcontext.DecodeStrict(out, "cache-evaluation-v1.schema.json#/$defs/summary", &summary); err != nil {
		t.Fatal(err)
	}
	return summary
}
func cacheEvidenceHashes(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.Walk(root, func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			raw, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			result[name] = info.ModTime().String() + info.Mode().String() + taskBytesDigest(raw)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCacheEvaluationShippedSonnet55Profile(t *testing.T) {
	binary := buildTaskExecutable(t, "./cmd/repocue")
	sonnetSpec := func() promptcache.Spec {
		s := cacheSpec()
		s.Provider.Model = "claude-sonnet-5-5"
		s.Rates.Model = s.Provider.Model
		s.Rates.Input, s.Rates.Write5m, s.Rates.Write1h = "2.00", "2.50", "4.00"
		s.Rates.Read, s.Rates.Output = "0.10", "10.00"
		return s
	}
	t.Run("profile-and-cycle", func(t *testing.T) {
		f := newTaskFixture(t, binary)
		server := newCacheHTTPFixture(t, "")
		spec := sonnetSpec()
		report := prepareCacheFixture(t, f, spec)
		if server.count() != 0 {
			t.Fatal("preparation launched provider")
		}
		var prefixDigest string
		for _, step := range spec.Steps {
			out, stderr, err := cacheRaw(f, server, cacheFixtureKey, cacheRunArgs(report, step.ID, server)...)
			if err != nil {
				t.Fatalf("Sonnet profile run: %v %s %s", err, stderr, out)
			}
			raw, err := os.ReadFile(filepath.Join(report.Directory, "request-"+step.ID+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var request promptcache.Request
			if err := taskcontext.DecodeStrict(raw, "cache-request-v1.schema.json", &request); err != nil {
				t.Fatal(err)
			}
			server.mutex.Lock()
			captured := append([]byte{}, server.requests[len(server.requests)-1]...)
			server.mutex.Unlock()
			if !bytes.Equal(captured, []byte(request.WireBody)) || taskBytesDigest(captured) != request.WireDigest {
				t.Fatal("submitted Sonnet wire differs from frozen request")
			}
			var wire map[string]json.RawMessage
			if err := json.Unmarshal(captured, &wire); err != nil {
				t.Fatal(err)
			}
			if string(wire["model"]) != `"claude-sonnet-5-5"` || string(wire["thinking"]) != `{"type":"between_tools"}` ||
				!bytes.Equal(wire["stream"], []byte("false")) || wire["tools"] != nil {
				t.Fatal("fixed text-only Sonnet profile differs from submitted HTTP body")
			}
			var config struct {
				Effort string `json:"effort"`
				Format struct {
					Type   string          `json:"type"`
					Schema json.RawMessage `json:"schema"`
				} `json:"format"`
			}
			if err := json.Unmarshal(wire["output_config"], &config); err != nil {
				t.Fatal(err)
			}
			schema, err := os.ReadFile(filepath.Join(report.Directory, "answer.schema.json"))
			if err != nil || config.Effort != "high" || config.Format.Type != "json_schema" ||
				!bytes.Equal(config.Format.Schema, schema) {
				t.Fatal("fixed effort or retained structured-output schema lost")
			}
			if step.Arm == "cache" {
				if prefixDigest != "" && prefixDigest != request.PrefixDigest {
					t.Fatal("unchanged Sonnet prefix acquired different identity")
				}
				prefixDigest = request.PrefixDigest
			}
		}
		if _, _, err := cacheRaw(f, server, cacheFixtureKey, cacheRunArgs(report, "cache-b", server)...); err == nil || server.count() != spec.Limits.MaxRequests {
			t.Fatal("request bound allowed an extra attempt")
		}
		summary := cacheSummaryFixture(t, f, report, "")
		if len(summary.Errors) != 0 || len(summary.Attempts) != 4 || summary.TotalEstimatedUSD == nil || *summary.TotalEstimatedUSD != "0.00688" {
			t.Fatalf("Sonnet full-cycle exact decimal accounting: %+v", summary)
		}
		if summary.Attempts[2].CacheActivity != "creation" || summary.Attempts[3].CacheActivity != "reuse" {
			t.Fatal("Sonnet cache activity differs from actual counters")
		}
	})
	t.Run("changing-task-and-basis", func(t *testing.T) {
		f := newTaskFixture(t, binary)
		spec := sonnetSpec()
		readRequest := func(report evaluation.CacheReport) promptcache.Request {
			raw, err := os.ReadFile(filepath.Join(report.Directory, "request-cache-a.json"))
			if err != nil {
				t.Fatal(err)
			}
			var request promptcache.Request
			if err := taskcontext.DecodeStrict(raw, "cache-request-v1.schema.json", &request); err != nil {
				t.Fatal(err)
			}
			return request
		}
		first := prepareCacheFixture(t, f, spec)
		a := readRequest(first)
		writeFile(t, filepath.Join(f.root, "README.md"), "Unrelated repository metadata changes.\n")
		spec.Steps[2].Prompt = "Explain the same original context differently."
		second := prepareCacheFixture(t, f, spec)
		b := readRequest(second)
		if a.PrefixDigest != b.PrefixDigest || a.WireDigest == b.WireDigest || first.Basis.Fingerprint == second.Basis.Fingerprint {
			t.Fatal("changing task/Git metadata changed the reusable Sonnet prefix")
		}
		cacheSummaryFixture(t, f, first, "")
		cacheSummaryFixture(t, f, second, "")
	})
	for _, mode := range []string{"thinking", "model"} {
		t.Run(mode, func(t *testing.T) {
			f := newTaskFixture(t, binary)
			server := newCacheHTTPFixture(t, mode)
			spec := sonnetSpec()
			spec.Steps = spec.Steps[2:3]
			spec.Limits.MaxRequests = 1
			report := prepareCacheFixture(t, f, spec)
			if _, _, err := cacheRaw(f, server, cacheFixtureKey, cacheRunArgs(report, "cache-a", server)...); err == nil {
				t.Fatal("unexpected response passed fixed profile")
			}
			summary := cacheSummaryFixture(t, f, report, "")
			if len(summary.Attempts) != 1 || server.count() != 1 || summary.Attempts[0].Correctness.Value != nil {
				t.Fatal("unsuccessful Sonnet attempt lost or gained correctness")
			}
			cost := summary.Attempts[0].EstimatedUSD
			if mode == "thinking" {
				if cost == nil || *cost != "0.00257" || summary.TotalEstimatedUSD == nil || *summary.TotalEstimatedUSD != *cost {
					t.Fatal("unexpected thinking block lost known usage and cost")
				}
			} else if cost != nil || summary.TotalEstimatedUSD != nil {
				t.Fatal("response model mismatch acquired a cost estimate")
			}
			if _, _, err := cacheRaw(f, server, cacheFixtureKey, cacheRunArgs(report, "cache-a", server)...); err == nil || server.count() != 1 {
				t.Fatal("unsuccessful Sonnet step was retried")
			}
		})
	}
	t.Run("unknown-model", func(t *testing.T) {
		f := newTaskFixture(t, binary)
		server := newCacheHTTPFixture(t, "")
		spec := sonnetSpec()
		spec.Provider.Model = "claude-sonnet-5-5-unknown"
		spec.Rates.Model = spec.Provider.Model
		input := filepath.Join(f.external, "unknown-model.json")
		taskWriteJSON(t, input, spec)
		out, stderr, err := cacheRaw(f, server, cacheFixtureKey, "evaluate-cache", "prepare", "--repository", f.root,
			"--cache-dir", f.cache, "--benchmark-file", input, "--output-directory", f.external)
		if err == nil || !bytes.Contains(stderr, []byte("no supported frozen")) || server.count() != 0 {
			t.Fatalf("unknown profile escaped zero-launch preflight: %v %s %s", err, stderr, out)
		}
	})
}

func TestCacheEvaluationShippedFullCycleAndReadOnlySummary(t *testing.T) {
	f := newTaskFixture(t, buildTaskExecutable(t, "./cmd/repocue"))
	server := newCacheHTTPFixture(t, "")
	spec := cacheSpec()
	report := prepareCacheFixture(t, f, spec)
	if server.count() != 0 {
		t.Fatal("preparation launched provider")
	}
	for _, step := range spec.Steps {
		out, stderr, err := cacheRaw(f, server, cacheFixtureKey, cacheRunArgs(report, step.ID, server)...)
		if err != nil {
			t.Fatalf("real run %s: %v %s %s", step.ID, err, stderr, out)
		}
		var observation evaluation.CacheObservation
		if err := taskcontext.DecodeStrict(out, "cache-observation-v1.schema.json", &observation); err != nil {
			t.Fatal(err)
		}
		if !observation.RequestWritten || !observation.HTTPAttempted || observation.Status != "completed" {
			t.Fatalf("missing actual wire completion: %+v", observation)
		}
		if observation.HTTPStartedAt == nil || observation.HTTPFinishedAt == nil || observation.FinishedAt == nil || observation.DurationMS == nil ||
			!observation.HTTPStartedAt.After(observation.StartedAt) || !observation.HTTPFinishedAt.Before(*observation.FinishedAt) ||
			observation.HTTPFinishedAt.Before(*observation.HTTPStartedAt) ||
			float64(observation.HTTPFinishedAt.Sub(*observation.HTTPStartedAt))/float64(time.Millisecond) != *observation.DurationMS {
			t.Fatalf("actual HTTP interval not retained separately from real Git pre/postflight: %+v", observation)
		}
	}
	summary := cacheSummaryFixture(t, f, report, "")
	if len(summary.Errors) != 0 || len(summary.Attempts) != 4 || len(summary.Pairs) != 1 || summary.TotalEstimatedUSD == nil || *summary.TotalEstimatedUSD != "0.00343" {
		t.Fatalf("complete inventory and disjoint cost: %+v", summary)
	}
	pair := summary.Pairs[0]
	if pair.DifferenceUSD == nil || *pair.DifferenceUSD != "0.00065" || pair.Verdict.Value != nil {
		t.Fatalf("full cycle comparison/quality gate: %+v", pair)
	}
	if pair.InitialRequestUSD == nil || *pair.InitialRequestUSD != "0.00127" || pair.SubsequentRequestsUSD == nil || *pair.SubsequentRequestsUSD != "0.00012" || pair.ObservedReuseCount == nil || *pair.ObservedReuseCount != 1 {
		t.Fatal("initial/repeated cost or observed reuse count missing")
	}
	if summary.Attempts[2].CacheActivity != "creation" || summary.Attempts[3].CacheActivity != "reuse" {
		t.Fatal("observed counters did not control activity")
	}
	cueBytes, err := os.ReadFile(filepath.Join(report.Directory, "task.json"))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := cue.DecodeTask(cueBytes)
	if err != nil {
		t.Fatal(err)
	}
	sourceSet, err := taskbenchmark.Sources(expected)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, source := range sourceSet {
		ids = append(ids, source.ID)
	}
	var bundle struct {
		SchemaVersion string                   `json:"schema_version"`
		Kind          string                   `json:"kind"`
		Assessments   []promptcache.Assessment `json:"assessments"`
	}
	bundle.SchemaVersion = promptcache.EvaluationVersion
	bundle.Kind = "assessments"
	for _, attempt := range summary.Attempts {
		raw, err := os.ReadFile(filepath.Join(report.Directory, "attempt-"+attempt.StepID, "observation.json"))
		if err != nil {
			t.Fatal(err)
		}
		var observation evaluation.CacheObservation
		if err := json.Unmarshal(raw, &observation); err != nil {
			t.Fatal(err)
		}
		yes := true
		bundle.Assessments = append(bundle.Assessments, promptcache.Assessment{StepID: attempt.StepID, BenchmarkDigest: report.BenchmarkDigest,
			RequestDigest: observation.RequestDigest, ResponseDigest: *observation.ResponseDigest, AnswerDigest: *attempt.AnswerDigest,
			BasisFingerprint: report.Basis.Fingerprint, SourceIDs: ids, Assessor: "independent-fixture-input", Rubric: spec.Rubric, Correct: true, Compliant: &yes})
	}
	assessment := filepath.Join(f.external, "assessments.json")
	taskWriteJSON(t, assessment, bundle)
	accepted := cacheSummaryFixture(t, f, report, assessment)
	if accepted.Pairs[0].Verdict.Value == nil || !*accepted.Pairs[0].Verdict.Value {
		t.Fatal("bound complete local fixture cycle not scored")
	}
	bundle.Assessments[0].Correct = false
	taskWriteJSON(t, assessment, bundle)
	rejected := cacheSummaryFixture(t, f, report, assessment)
	if rejected.Attempts[0].Correctness.Value == nil || *rejected.Attempts[0].Correctness.Value || rejected.Pairs[0].Verdict.Value != nil {
		t.Fatal("negative independent discovery assessment ignored")
	}
	bundle.Assessments[0].Correct = true
	taskWriteJSON(t, assessment, bundle)
	before := cacheEvidenceHashes(t, report.Directory)
	// Remove process lookup and live-repository availability from the summary environment.
	moved := f.root + "-unavailable"
	if err := os.Rename(f.root, moved); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(moved, f.root)
	cmd := exec.Command(f.binary, "evaluate-cache", "summary", "--input-directory", report.Directory)
	cmd.Env = append(os.Environ(), "PATH=", "ANTHROPIC_API_KEY=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("summary used unavailable repository/process: %v %s", err, out)
	}
	if !reflect.DeepEqual(before, cacheEvidenceHashes(t, report.Directory)) || server.count() != 4 {
		t.Fatal("summary mutated evidence or invoked provider")
	}
	// Every selected source is present once before the boundary and absent from the suffix.
	server.mutex.Lock()
	wireBytes := append([]byte{}, server.requests[2]...)
	server.mutex.Unlock()
	var wire promptcache.WireRequest
	if err := json.Unmarshal(wireBytes, &wire); err != nil {
		t.Fatal(err)
	}
	var prefix struct {
		Sources []taskcontext.Source `json:"sources"`
	}
	json.Unmarshal([]byte(wire.Messages[0].Content[0].Text), &prefix)
	if !reflect.DeepEqual(prefix.Sources, expected.Content.Sources) || strings.Contains(wire.Messages[0].Content[1].Text, "\"text_digest\"") {
		t.Fatal("original text duplicated or changed across boundary")
	}
	for _, source := range prefix.Sources {
		if taskcontext.Digest([]byte(source.Text)) != source.TextDigest {
			t.Fatal("original source text changed")
		}
	}
	for _, hash := range cacheEvidenceHashes(t, report.Directory) {
		if strings.Contains(hash, cacheFixtureKey) {
			t.Fatal("credential in evidence")
		}
	}
	filepath.Walk(report.Directory, func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			raw, _ := os.ReadFile(name)
			if bytes.Contains(raw, []byte(cacheFixtureKey)) {
				t.Errorf("credential persisted: %s", name)
			}
		}
		return nil
	})
}

func TestCacheEvaluationShippedFailuresAndUnknownCosts(t *testing.T) {
	binary := buildTaskExecutable(t, "./cmd/repocue")
	for _, mode := range []string{"wrong", "missing", "truncated", "bad-usage", "http-error", "partial", "timeout", "redirect", "credential", "limit", "no-key", "model", "malformed", "trailing"} {
		t.Run(mode, func(t *testing.T) {
			f := newTaskFixture(t, binary)
			server := newCacheHTTPFixture(t, mode)
			spec := cacheSpec()
			spec.Steps = spec.Steps[:1]
			if mode == "timeout" {
				spec.Limits.TimeoutMS = 200
			}
			if mode == "limit" {
				spec.Limits.MaxResponseBytes = 64
			}
			report := prepareCacheFixture(t, f, spec)
			key := cacheFixtureKey
			if mode == "no-key" {
				key = ""
			}
			out, _, err := cacheRaw(f, server, key, cacheRunArgs(report, spec.Steps[0].ID, server)...)
			if (err != nil) != (mode != "wrong" && mode != "missing") {
				t.Fatalf("completion state: %v", err)
			}
			var observation evaluation.CacheObservation
			if err := taskcontext.DecodeStrict(out, "cache-observation-v1.schema.json", &observation); err != nil {
				t.Fatal(err)
			}
			if mode == "no-key" {
				if observation.HTTPAttempted || observation.HTTPStartedAt != nil || observation.HTTPFinishedAt != nil || observation.DurationMS != nil {
					t.Fatal("pre-HTTP failure acquired invented request timestamps")
				}
			} else if observation.HTTPStartedAt == nil || observation.HTTPFinishedAt == nil || observation.DurationMS == nil {
				t.Fatal("failed HTTP attempt lost actual absolute timestamps")
			}
			wantCalls := 1
			if mode == "no-key" {
				wantCalls = 0
			}
			if server.count() != wantCalls {
				t.Fatalf("retry or implicit request: %d", server.count())
			}
			if _, _, err := cacheRaw(f, server, cacheFixtureKey, cacheRunArgs(report, spec.Steps[0].ID, server)...); err == nil || server.count() != wantCalls {
				t.Fatal("failed/reserved step was retried")
			}
			summary := cacheSummaryFixture(t, f, report, "")
			if len(summary.Attempts) != 1 {
				t.Fatal("failed attempt dropped")
			}
			item := summary.Attempts[0]
			if mode == "wrong" {
				if item.Correctness.Value == nil || *item.Correctness.Value || item.EstimatedUSD == nil {
					t.Fatal("wrong answer or its cost was dropped")
				}
			} else if mode == "truncated" || mode == "http-error" {
				if item.EstimatedUSD == nil || item.Correctness.Value != nil {
					t.Fatal("failed task usage cost not retained independently")
				}
			} else if item.EstimatedUSD != nil || summary.TotalEstimatedUSD != nil {
				t.Fatal("unknown/incomplete usage became favorable cost")
			}
			if mode == "credential" {
				if _, err := os.Stat(filepath.Join(report.Directory, "attempt-base-a", "response.json")); !os.IsNotExist(err) {
					t.Fatal("credential-echo response persisted")
				}
			}
		})
	}
}

func TestCacheEvaluationShippedStablePrefixFreshnessAndBudget(t *testing.T) {
	binary := buildTaskExecutable(t, "./cmd/repocue")
	f := newTaskFixture(t, binary)
	spec := cacheSpec()
	first := prepareCacheFixture(t, f, spec)
	request := func(report evaluation.CacheReport) promptcache.Request {
		raw, err := os.ReadFile(filepath.Join(report.Directory, "request-cache-a.json"))
		if err != nil {
			t.Fatal(err)
		}
		var r promptcache.Request
		if err := json.Unmarshal(raw, &r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	a := request(first)
	writeFile(t, filepath.Join(f.root, "README.md"), "Unrelated repository metadata changes.\n")
	spec.Steps[2].Prompt = "Explain the same original context differently."
	second := prepareCacheFixture(t, f, spec)
	b := request(second)
	if a.PrefixDigest != b.PrefixDigest || a.WireDigest == b.WireDigest || first.Basis.Fingerprint == second.Basis.Fingerprint {
		t.Fatal("changing task/Git metadata did not stay behind stable prefix")
	}
	source := filepath.Join(f.root, "docs", "rules.md")
	raw, _ := os.ReadFile(source)
	writeFile(t, source, string(raw)+"New original source bytes.\n")
	f.resetDeclaration(t)
	third := prepareCacheFixture(t, f, spec)
	c := request(third)
	if c.PrefixDigest == b.PrefixDigest {
		t.Fatal("full-file/declaration pin change did not change prefix identity")
	}
	server := newCacheHTTPFixture(t, "")
	if _, _, err := cacheRaw(f, server, cacheFixtureKey, cacheRunArgs(second, "base-a", server)...); err == nil || server.count() != 0 {
		t.Fatal("stale frozen request launched")
	}
	for _, mode := range []string{"missing", "pin", "range", "symlink", "budget", "envelope"} {
		t.Run(mode, func(t *testing.T) {
			g := newTaskFixture(t, binary)
			s := cacheSpec()
			name := filepath.Join(g.root, "docs", "rules.md")
			switch mode {
			case "missing":
				if err := os.Remove(name); err != nil {
					t.Fatal(err)
				}
			case "pin":
				writeFile(t, name, "Changed without renewing the full-file pin.\n")
			case "range":
				raw, _ := os.ReadFile(filepath.Join(g.root, "docs", "context.json"))
				var d taskcontext.Declaration
				json.Unmarshal(raw, &d)
				d.Items[0].Source.Lines.End = 999
				taskWriteJSON(t, filepath.Join(g.root, "docs", "context.json"), d)
			case "symlink":
				target := filepath.Join(g.external, "linked.md")
				raw, _ := os.ReadFile(name)
				writeFile(t, target, string(raw))
				if err := os.Remove(name); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, name); err != nil {
					t.Fatal(err)
				}
			case "budget":
				s.MaxContextTokens = 1
			case "envelope":
				expected := g.task(t, 5000, "cmd")
				s.MaxContextTokens = expected.Budget.EstimatedTokens + 5
			}
			input := filepath.Join(g.external, "bad-input.json")
			taskWriteJSON(t, input, s)
			out, _, err := cacheRaw(g, nil, "", "evaluate-cache", "prepare", "--repository", g.root, "--cache-dir", g.cache, "--benchmark-file", input, "--output-directory", g.external)
			if err == nil {
				t.Fatalf("invalid original source/budget accepted: %s", out)
			}
		})
	}
}

func TestCacheEvaluationShippedTamperAliasesAndInterruptedJournal(t *testing.T) {
	binary := buildTaskExecutable(t, "./cmd/repocue")
	for _, artifact := range []string{"benchmark.json", "task.json", "answer.schema.json", "request-base-a.json", "attempt-base-a/submitted-request.json", "attempt-base-a/response.json", "assessment", "http-timing", "http-duration", "http-missing"} {
		t.Run(artifact, func(t *testing.T) {
			f := newTaskFixture(t, binary)
			server := newCacheHTTPFixture(t, "")
			spec := cacheSpec()
			spec.Steps = spec.Steps[:1]
			report := prepareCacheFixture(t, f, spec)
			if _, stderr, err := cacheRaw(f, server, cacheFixtureKey, cacheRunArgs(report, "base-a", server)...); err != nil {
				t.Fatalf("run: %v %s", err, stderr)
			}
			if strings.HasPrefix(artifact, "http-") {
				name := filepath.Join(report.Directory, "attempt-base-a", "observation.json")
				raw, err := os.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				var o evaluation.CacheObservation
				if err := taskcontext.DecodeStrict(raw, "cache-observation-v1.schema.json", &o); err != nil {
					t.Fatal(err)
				}
				switch artifact {
				case "http-timing":
					before := o.StartedAt.Add(-time.Millisecond)
					o.HTTPStartedAt = &before
				case "http-duration":
					wrong := *o.DurationMS + 1
					o.DurationMS = &wrong
				case "http-missing":
					o.HTTPStartedAt = nil
				}
				taskWriteJSON(t, name, o)
				result := cacheSummaryFixture(t, f, report, "")
				if len(result.Errors) == 0 || result.TotalEstimatedUSD != nil || result.Pairs[0].Verdict.Value != nil {
					t.Fatal("invalid HTTP interval accepted by real summary")
				}
			} else if artifact == "assessment" {
				summary := cacheSummaryFixture(t, f, report, "")
				raw, _ := os.ReadFile(filepath.Join(report.Directory, "attempt-base-a", "observation.json"))
				var o evaluation.CacheObservation
				json.Unmarshal(raw, &o)
				a := promptcache.Assessment{StepID: "base-a", BenchmarkDigest: report.BenchmarkDigest, RequestDigest: o.RequestDigest, ResponseDigest: *o.ResponseDigest, AnswerDigest: *summary.Attempts[0].AnswerDigest, BasisFingerprint: report.Basis.Fingerprint, SourceIDs: []string{}, Assessor: "fixture", Rubric: spec.Rubric, Correct: true}
				name := filepath.Join(f.external, "bad-assessment.json")
				taskWriteJSON(t, name, map[string]any{"schema_version": promptcache.EvaluationVersion, "kind": "assessments", "assessments": []promptcache.Assessment{a}})
				result := cacheSummaryFixture(t, f, report, name)
				if len(result.Errors) == 0 || result.Pairs[0].Verdict.Value != nil {
					t.Fatal("unbound assessment accepted")
				}
			} else {
				name := filepath.Join(report.Directory, artifact)
				raw, err := os.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, name, string(raw)+" ")
				out, _, err := cacheRaw(f, nil, "", "evaluate-cache", "summary", "--input-directory", report.Directory)
				if strings.HasPrefix(artifact, "attempt-") {
					if err != nil {
						t.Fatalf("attempt inventory missing: %s", out)
					}
					var summary evaluation.CacheSummary
					json.Unmarshal(out, &summary)
					if len(summary.Errors) == 0 || summary.TotalEstimatedUSD != nil {
						t.Fatal("captured artifact tamper accepted")
					}
				} else if err == nil {
					t.Fatal("frozen root artifact tamper accepted")
				}
			}
			if server.count() != 1 {
				t.Fatal("summary invoked provider")
			}
		})
	}
	t.Run("aliases", func(t *testing.T) {
		f := newTaskFixture(t, binary)
		report := prepareCacheFixture(t, f, cacheSpec())
		alias := filepath.Join(f.external, "alias")
		if err := os.Symlink(report.Directory, alias); err != nil {
			t.Fatal(err)
		}
		if _, _, err := cacheRaw(f, nil, "", "evaluate-cache", "summary", "--input-directory", alias); err == nil {
			t.Fatal("directory alias accepted")
		}
		link := filepath.Join(report.Directory, "attempt-base-a")
		if err := os.Symlink(f.external, link); err != nil {
			t.Fatal(err)
		}
		summary := cacheSummaryFixture(t, f, report, "")
		if len(summary.Errors) == 0 {
			t.Fatal("attempt alias accepted")
		}
	})
	t.Run("interrupted", func(t *testing.T) {
		f := newTaskFixture(t, binary)
		server := newCacheHTTPFixture(t, "interrupt")
		spec := cacheSpec()
		spec.Steps = spec.Steps[:1]
		report := prepareCacheFixture(t, f, spec)
		cmd := exec.Command(f.binary, cacheRunArgs(report, "base-a", server)...)
		cmd.Env = append(os.Environ(), "ANTHROPIC_API_KEY="+cacheFixtureKey, "SSL_CERT_FILE="+server.caFile)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		select {
		case <-server.arrived:
		case <-time.After(5 * time.Second):
			cmd.Process.Kill()
			cmd.Wait()
			t.Fatal("real request did not reach HTTP boundary")
		}
		cmd.Process.Kill()
		cmd.Wait()
		summary := cacheSummaryFixture(t, f, report, "")
		if len(summary.Attempts) != 1 || summary.Attempts[0].Status != "started" || summary.TotalEstimatedUSD != nil || len(summary.Errors) == 0 {
			t.Fatal("interrupted attempt was hidden")
		}
		if server.count() != 1 {
			t.Fatal("interrupted step retried")
		}
	})
}
