package evaluation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jeonghanlee/repocue/internal/claudeadapter"
	"github.com/jeonghanlee/repocue/internal/cue"
	"github.com/jeonghanlee/repocue/internal/promptcache"
	"github.com/jeonghanlee/repocue/internal/taskbenchmark"
	"github.com/jeonghanlee/repocue/internal/taskcontext"
)

const cacheHTTPTimeToleranceMS = 0.001

type CacheInventory struct {
	StepID        string                `json:"step_id"`
	Arm           string                `json:"arm"`
	Condition     string                `json:"condition"`
	Status        string                `json:"status"`
	Integrity     bool                  `json:"integrity"`
	Delivery      bool                  `json:"delivery"`
	Correctness   taskbenchmark.Outcome `json:"correctness"`
	Compliance    taskbenchmark.Outcome `json:"compliance"`
	Usage         *promptcache.Usage    `json:"usage"`
	EstimatedUSD  *string               `json:"estimated_usd"`
	DurationMS    *float64              `json:"duration_ms"`
	CacheActivity string                `json:"cache_activity"`
	AnswerDigest  *string               `json:"answer_digest"`
	Error         *string               `json:"error"`
}
type CachePair struct {
	PairID                string                `json:"pair_id"`
	BaselineUSD           *string               `json:"baseline_usd"`
	CacheUSD              *string               `json:"cache_usd"`
	InitialRequestUSD     *string               `json:"initial_request_usd"`
	SubsequentRequestsUSD *string               `json:"subsequent_requests_usd"`
	ObservedReuseCount    *int                  `json:"observed_reuse_count"`
	DifferenceUSD         *string               `json:"difference_usd"`
	Verdict               taskbenchmark.Outcome `json:"verdict"`
}
type CacheSummary struct {
	SchemaVersion     string           `json:"schema_version"`
	Kind              string           `json:"kind"`
	Attempts          []CacheInventory `json:"attempts"`
	Pairs             []CachePair      `json:"pairs"`
	TotalEstimatedUSD *string          `json:"total_estimated_usd"`
	NotObserved       []string         `json:"not_observed"`
	Errors            []string         `json:"errors"`
}
type cacheEntry struct {
	step        promptcache.Step
	inventory   CacheInventory
	observation CacheObservation
}

func newCacheInventory(step promptcache.Step) CacheInventory {
	return CacheInventory{StepID: step.ID, Arm: step.Arm, Condition: step.Condition, Status: "not_run",
		Correctness: taskbenchmark.Unknown("successful answer not observed"),
		Compliance:  taskbenchmark.Unknown("independent source-backed assessment required"), CacheActivity: "not_observed"}
}

// SummarizeCache reconstructs frozen requests and recomputes every metric from
// retained raw observations. It performs no Git, CLI, network or evidence write.
func SummarizeCache(directory, assessmentFile string) (result CacheSummary, summaryErr error) {
	result = CacheSummary{SchemaVersion: promptcache.EvaluationVersion, Kind: "summary", Attempts: []CacheInventory{}, Pairs: []CachePair{}, Errors: []string{},
		NotObserved: []string{"account_billing_usd", "cpu_ms", "memory_bytes", "filesystem_io_bytes", "task_ready_ms", "time_to_first_token_ms"}}
	report, fullCue, err := loadCacheReport(directory)
	if err != nil {
		return result, err
	}
	expected, err := cue.DecodeTask(fullCue)
	if err != nil {
		return result, err
	}
	assessments := map[string]promptcache.Assessment{}
	if assessmentFile != "" {
		raw, err := readTaskFile(assessmentFile)
		if err != nil {
			return result, err
		}
		var bundle struct {
			SchemaVersion string                   `json:"schema_version"`
			Kind          string                   `json:"kind"`
			Assessments   []promptcache.Assessment `json:"assessments"`
		}
		if err := taskcontext.DecodeStrict(raw, "cache-evaluation-v1.schema.json#/$defs/assessments", &bundle); err != nil {
			return result, err
		}
		for _, a := range bundle.Assessments {
			if _, ok := assessments[a.StepID]; ok {
				return result, errors.New("duplicate cache assessment")
			}
			assessments[a.StepID] = a
		}
	}
	groups := map[string][]cacheEntry{}
	known := map[string]bool{}
	costs := []*string{}
	for _, step := range report.Spec.Steps {
		known["attempt-"+step.ID] = true
		inventory, observation, err := summarizeCacheAttempt(report, expected, step, assessments[step.ID])
		if err != nil {
			message := err.Error()
			inventory.Error = &message
			result.Errors = append(result.Errors, step.ID+": "+message)
		}
		delete(assessments, step.ID)
		result.Attempts = append(result.Attempts, inventory)
		costs = append(costs, inventory.EstimatedUSD)
		if step.Arm != "probe" {
			groups[step.PairID] = append(groups[step.PairID], cacheEntry{step, inventory, observation})
		}
	}
	if len(assessments) > 0 {
		return result, errors.New("assessment names an unknown frozen step")
	}
	children, err := os.ReadDir(report.Directory)
	if err != nil {
		return result, err
	}
	for _, child := range children {
		if strings.HasPrefix(child.Name(), "attempt-") && !known[child.Name()] {
			result.Errors = append(result.Errors, "unrecognized retained attempt: "+child.Name())
			item := newCacheInventory(promptcache.Step{ID: "unrecognized", Arm: "probe", Condition: "unknown"})
			message := "unrecognized retained attempt: " + child.Name()
			item.Status = "invalid"
			item.Error = &message
			result.Attempts = append(result.Attempts, item)
			costs = append(costs, nil)
		}
	}
	result.TotalEstimatedUSD = promptcache.AddUSD(costs...)
	if len(result.Errors) > 0 {
		result.TotalEstimatedUSD = nil
	}
	keys := []string{}
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		pair := summarizeCachePair(key, groups[key], report.Basis.Fingerprint)
		if len(result.Errors) > 0 {
			pair.Verdict = taskbenchmark.Unknown("retained evidence has invalid or incomplete observations")
		}
		result.Pairs = append(result.Pairs, pair)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return result, err
	}
	if err := taskcontext.DecodeStrict(encoded, "cache-evaluation-v1.schema.json#/$defs/summary", &result); err != nil {
		return result, err
	}
	return result, nil
}

func summarizeCacheAttempt(report CacheReport, expected cue.TaskEnvelope, step promptcache.Step, assessment promptcache.Assessment) (item CacheInventory, observation CacheObservation, resultErr error) {
	item = newCacheInventory(step)
	root := filepath.Join(report.Directory, "attempt-"+step.ID)
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		return item, observation, nil
	}
	item.Status = "invalid"
	if _, err := canonicalCacheDirectory(root); err != nil {
		return item, observation, err
	}
	start, err := readCacheFile(root, "attempt-start.json")
	if err != nil {
		return item, observation, err
	}
	var journal CacheObservation
	if err := taskcontext.DecodeStrict(start, "cache-observation-v1.schema.json", &journal); err != nil {
		return item, observation, err
	}
	if journal.StepID != step.ID || journal.RequestDigest != report.Requests[step.ID] || journal.Status != "started" || journal.FinishedAt != nil ||
		journal.HTTPAttempted || journal.RequestWritten || journal.HTTPStartedAt != nil || journal.HTTPFinishedAt != nil || journal.DurationMS != nil {
		return item, observation, errors.New("attempt-start identity mismatch")
	}
	raw, err := readCacheFile(root, "observation.json")
	if os.IsNotExist(err) {
		item.Status = "started"
		return item, journal, errors.New("interrupted attempt; full usage unavailable")
	}
	if err != nil {
		return item, observation, err
	}
	if err := taskcontext.DecodeStrict(raw, "cache-observation-v1.schema.json", &observation); err != nil {
		return item, observation, err
	}
	if observation.StepID != step.ID || observation.RequestDigest != journal.RequestDigest || observation.Endpoint != journal.Endpoint ||
		!observation.StartedAt.Equal(journal.StartedAt) || observation.FinishedAt == nil || observation.FinishedAt.Before(observation.StartedAt) ||
		observation.Status == "started" || claudeadapter.ValidateEndpoint(observation.Endpoint) != nil {
		return item, observation, errors.New("final observation identity/time mismatch")
	}
	if observation.HTTPAttempted != (observation.DurationMS != nil) || observation.RequestWritten && !observation.HTTPAttempted ||
		observation.DurationMS != nil && *observation.DurationMS > float64(observation.FinishedAt.Sub(observation.StartedAt).Milliseconds()+1) {
		return item, observation, errors.New("HTTP timing or write evidence mismatch")
	}
	if observation.HTTPAttempted != (observation.HTTPStartedAt != nil) || observation.HTTPAttempted != (observation.HTTPFinishedAt != nil) {
		return item, observation, errors.New("HTTP timestamps do not match attempted state")
	}
	if observation.HTTPAttempted && (observation.HTTPStartedAt.Before(observation.StartedAt) ||
		observation.HTTPFinishedAt.Before(*observation.HTTPStartedAt) || observation.HTTPFinishedAt.After(*observation.FinishedAt) ||
		math.Abs(float64(observation.HTTPFinishedAt.Sub(*observation.HTTPStartedAt))/float64(time.Millisecond)-*observation.DurationMS) > cacheHTTPTimeToleranceMS) {
		return item, observation, errors.New("HTTP timestamps or duration exceed containing attempt interval")
	}
	requestBytes, err := readCacheFile(root, "request.json")
	if err != nil {
		return item, observation, err
	}
	original, err := readCacheFile(report.Directory, "request-"+step.ID+".json")
	if err != nil || !bytes.Equal(requestBytes, original) {
		return item, observation, errors.New("attempt request differs from frozen request")
	}
	var request promptcache.Request
	if err := taskcontext.DecodeStrict(requestBytes, "cache-request-v1.schema.json", &request); err != nil {
		return item, observation, err
	}
	wire, err := readCacheFile(root, "submitted-request.json")
	if err != nil || !bytes.Equal(wire, []byte(request.WireBody)) || taskDigest(wire) != request.WireDigest {
		return item, observation, errors.New("captured wire request mismatch")
	}
	item.Status = observation.Status
	item.DurationMS = observation.DurationMS
	item.Error = observation.Error
	item.Integrity = true
	item.Delivery = observation.RequestWritten
	if observation.ResponseDigest == nil {
		if _, err := os.Lstat(filepath.Join(root, "response.json")); !os.IsNotExist(err) {
			return item, observation, errors.New("unbound raw response")
		}
		return item, observation, nil
	}
	responseBytes, err := readCacheFile(root, "response.json")
	if err != nil || taskDigest(responseBytes) != *observation.ResponseDigest || len(responseBytes) > report.Spec.Limits.MaxResponseBytes {
		item.Integrity = false
		item.Delivery = false
		item.DurationMS = nil
		return item, observation, errors.New("raw response digest or limit mismatch")
	}
	if !observation.BodyComplete {
		return item, observation, nil
	}
	if observation.HTTPStatus == nil || !observation.HTTPAttempted {
		return item, observation, errors.New("complete response lacks HTTP evidence")
	}
	ttl := report.Spec.Provider.TTL
	if step.Arm == "baseline" {
		ttl = ""
	}
	response, err := promptcache.DecodeResponse(responseBytes, ttl)
	if err != nil {
		return item, observation, err
	}
	item.Usage = &response.Usage
	item.CacheActivity = response.Usage.Activity()
	item.EstimatedUSD = promptcache.CostUSD(response.Usage, report.Spec.Rates, report.Spec.Provider, response.Model, observation.StartedAt)
	good := observation.Status == "completed" && observation.Error == nil && *observation.HTTPStatus >= 200 && *observation.HTTPStatus < 300 &&
		validTaskBasis(observation.Before) && validTaskBasis(observation.After) && observation.Before.Fingerprint == report.Basis.Fingerprint &&
		observation.After.Fingerprint == report.Basis.Fingerprint && item.Delivery && response.Model == report.Spec.Provider.Model
	if good && response.Answer != nil {
		answerBytes, err := json.Marshal(response.Answer)
		if err != nil {
			return item, observation, err
		}
		answerDigest := taskDigest(answerBytes)
		item.AnswerDigest = &answerDigest
		item.Correctness, item.Compliance, err = scoreCacheAnswer(report, expected, observation, *response.Answer, assessment, answerDigest)
		if err != nil {
			return item, observation, err
		}
	} else if observation.Status == "completed" {
		return item, observation, errors.New("completed attempt lacks successful answer/basis evidence")
	}
	return item, observation, nil
}

func scoreCacheAnswer(report CacheReport, expected cue.TaskEnvelope, observation CacheObservation, answer promptcache.Answer, a promptcache.Assessment, answerDigest string) (correct, compliance taskbenchmark.Outcome, err error) {
	correct = taskbenchmark.Unknown("explanation needs independent source-backed assessment")
	compliance = taskbenchmark.Unknown("independent source-backed assessment required")
	sources, err := taskbenchmark.Sources(expected)
	if err != nil {
		return correct, compliance, err
	}
	requiredSources := []string{}
	known := map[string]bool{}
	for _, s := range sources {
		requiredSources = append(requiredSources, s.ID)
	}
	for _, s := range expected.Content.Sources {
		known[s.ID] = true
	}
	if a.StepID != "" {
		if a.StepID != observation.StepID || a.BenchmarkDigest != report.BenchmarkDigest || a.RequestDigest != observation.RequestDigest ||
			observation.ResponseDigest == nil || a.ResponseDigest != *observation.ResponseDigest || a.AnswerDigest != answerDigest ||
			a.BasisFingerprint != report.Basis.Fingerprint || !promptcache.SameIDs(a.SourceIDs, requiredSources) ||
			a.Rubric != report.Spec.Rubric || a.Assessor == "" {
			return correct, compliance, errors.New("assessment does not bind to task/request/answer/basis/source policy")
		}
		if a.Compliant != nil {
			compliance = taskbenchmark.Result(*a.Compliant, "bound independent assessment")
		}
	}
	if answer.TaskID != report.Spec.ID || !promptcache.SameIDs(answer.RequiredIDs, expected.Content.RequiredIDs) ||
		!promptcache.SameIDs(answer.SourceIDs, requiredSources) {
		return taskbenchmark.Result(false, "answer task/item/source set mismatch"), compliance, nil
	}
	for _, claim := range answer.Claims {
		if strings.TrimSpace(claim.Text) == "" || len(claim.SourceIDs) == 0 || promptcache.RequiredSources(claim.SourceIDs, known) != nil {
			return taskbenchmark.Result(false, "claim lacks valid original-source citations"), compliance, nil
		}
	}
	if report.Spec.TaskKind == "explanation" && len(answer.Claims) == 0 {
		return taskbenchmark.Result(false, "explanation has no source-backed claims"), compliance, nil
	}
	if report.Spec.TaskKind == "discovery" {
		correct = taskbenchmark.Result(true, "exact required item/source sets")
		if a.StepID != "" && !a.Correct {
			correct = taskbenchmark.Result(false, "bound independent assessment rejects answer")
		}
	} else if a.StepID != "" {
		correct = taskbenchmark.Result(a.Correct, "bound independent source-backed explanation assessment")
	}
	return correct, compliance, nil
}

func summarizeCachePair(id string, entries []cacheEntry, basisFingerprint string) CachePair {
	pair := CachePair{PairID: id, Verdict: taskbenchmark.Unknown("complete matched cycles, successful delivery and independent quality required")}
	var baseline, cached []cacheEntry
	for _, entry := range entries {
		if entry.step.Arm == "baseline" {
			baseline = append(baseline, entry)
		} else {
			cached = append(cached, entry)
		}
	}
	if len(baseline) == 0 || len(baseline) != len(cached) {
		return pair
	}
	baseCosts, cacheCosts := []*string{}, []*string{}
	good := true
	reused := false
	reuseCount := 0
	reuseKnown := true
	endpoint := ""
	for i, left := range baseline {
		right := cached[i]
		if left.step.Prompt != right.step.Prompt {
			return pair
		}
		for _, entry := range []cacheEntry{left, right} {
			if endpoint == "" {
				endpoint = entry.observation.Endpoint
			}
			if !entry.inventory.Integrity || entry.observation.Endpoint != endpoint || !validTaskBasis(entry.observation.Before) ||
				!validTaskBasis(entry.observation.After) || entry.observation.Before.Fingerprint != basisFingerprint || entry.observation.After.Fingerprint != basisFingerprint {
				return pair
			}
			if entry.inventory.Status != "completed" || !entry.inventory.Delivery || !outcomeTrue(entry.inventory.Correctness) || !outcomeTrue(entry.inventory.Compliance) {
				good = false
			}
		}
		u := left.inventory.Usage
		if u == nil || u.CacheRead == nil || *u.CacheRead != 0 || u.CacheWrite == nil || *u.CacheWrite != 0 {
			return pair
		}
		baseCosts = append(baseCosts, left.inventory.EstimatedUSD)
		cacheCosts = append(cacheCosts, right.inventory.EstimatedUSD)
		if u := right.inventory.Usage; u == nil || u.CacheRead == nil {
			reuseKnown = false
		} else if *u.CacheRead > 0 {
			reused = true
			reuseCount++
		}
	}
	pair.BaselineUSD = promptcache.AddUSD(baseCosts...)
	pair.CacheUSD = promptcache.AddUSD(cacheCosts...)
	pair.InitialRequestUSD = cacheCosts[0]
	if len(cacheCosts) > 1 {
		pair.SubsequentRequestsUSD = promptcache.AddUSD(cacheCosts[1:]...)
	}
	if reuseKnown {
		pair.ObservedReuseCount = &reuseCount
	}
	pair.DifferenceUSD = promptcache.DifferenceUSD(pair.BaselineUSD, pair.CacheUSD)
	if good && reused && pair.DifferenceUSD != nil {
		pair.Verdict = taskbenchmark.Result(promptcache.PositiveUSD(pair.DifferenceUSD), fmt.Sprintf("matched full-cycle USD estimate for %d requests; initial writes and outputs included", len(cached)))
	}
	return pair
}
