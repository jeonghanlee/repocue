package evaluation

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/jeonghanlee/repocue/docs/schema"
	"github.com/jeonghanlee/repocue/internal/evidence"
	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/jeonghanlee/repocue/internal/repository"
	"github.com/jeonghanlee/repocue/internal/storage"
)

func buildCalibrationExecutable(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "repocue")
	command := exec.Command("go", "build", "-o", path, "./cmd/repocue")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build shipped cue binary: %v: %s", err, output)
	}
	return path
}

func TestCueObservationColdWarmAndDirtyUsesRealExecutable(t *testing.T) {
	ctx := context.Background()
	root, cache := createM2Repository(t), t.TempDir()
	binary := buildCalibrationExecutable(t)
	command := exec.Command(binary, "init", "--cache-dir", cache, root)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("init: %v %s", err, output)
	}
	repo, err := repository.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	producer, err := evidence.ExecutableProducer(binary)
	if err != nil {
		t.Fatal(err)
	}
	// Init may create a disposable hint. Remove only this test cache's hint
	// so the first observed invocation actually exercises cold discovery.
	if err := os.Remove(filepath.Join(cache, repo.ID, "location.json")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	store, err := storage.OpenReadOnly(ctx, filepath.Join(cache, repo.ID, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.Current(ctx, repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	var observations []evidence.CueCLI
	for index := 0; index < 2; index++ {
		data, observation, trace, err := observeCueCLI(ctx, binary, producer, repo, state, cache, 500)
		if err != nil {
			t.Fatal(err)
		}
		count, err := CountGitInvocations(trace)
		if err != nil || observation.GitInvocations == nil || *observation.GitInvocations != count || observation.OutputDigest != evidence.Digest(data) || observation.GitTraceDigest != evidence.Digest(trace) {
			t.Fatal("actual boundary evidence mismatch")
		}
		observations = append(observations, observation)
	}
	if observations[0].LocationHintState != "absent" || observations[1].LocationHintState != "present" || *observations[1].GitInvocations >= *observations[0].GitInvocations {
		t.Fatalf("warm path not exercised: %+v", observations)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("dirty documentation\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	command = exec.Command(binary, "refresh", "--repository", root, "--cache-dir", cache)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("refresh: %v %s", err, output)
	}
	store, err = storage.OpenReadOnly(ctx, filepath.Join(cache, repo.ID, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	state, err = store.Current(ctx, repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, dirty, _, err := observeCueCLI(ctx, binary, producer, repo, state, cache, 500)
	if err != nil || dirty.Freshness != "dirty-but-indexed" || dirty.Snapshot != state.Snapshot.ID {
		t.Fatalf("dirty cue failed: %+v %v", dirty, err)
	}
	for _, malformed := range [][]byte{nil, []byte("not JSON"), []byte(`{"event":"start","sid":"incomplete"}`)} {
		if _, err := CountGitInvocations(malformed); err == nil {
			t.Fatal("incomplete trace became zero observation")
		}
	}
}

func TestCueLaunchFailureStartsNoAgentAndPublishesNothing(t *testing.T) {
	binary := buildCalibrationExecutable(t)
	if err := os.Chmod(binary, 0o600); err != nil {
		t.Fatal(err)
	}
	output, sentinel := t.TempDir(), filepath.Join(t.TempDir(), "agent-was-started")
	runner := filepath.Join(t.TempDir(), "runner")
	if err := os.WriteFile(runner, []byte("#!/bin/sh\nprintf started > \"$REPOCUE_TEST_SENTINEL\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REPOCUE_TEST_SENTINEL", sentinel)
	task := filepath.Join(t.TempDir(), "task.txt")
	if err := os.WriteFile(task, []byte("fixture task"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := RunM2(context.Background(), M2Config{RepositoryPath: createM2Repository(t), Conditions: []string{ConditionDirect, ConditionBasic}, MaxTokens: 500,
		CueExecutable: binary, RunIndex: 1, OutputDirectory: output, Runner: runner, TaskFile: task})
	if err == nil {
		t.Fatal("non-executable shipped cue was accepted")
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("agent launched after cue failure")
	}
	entries, err := os.ReadDir(output)
	if err != nil || len(entries) != 0 {
		t.Fatal("failed cue published output")
	}
}

func TestContentCaptureProvenanceRoundTrip(t *testing.T) {
	repo, err := repository.Open(context.Background(), createM2Repository(t))
	if err != nil {
		t.Fatal(err)
	}
	capture, err := repo.CaptureEvidence(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(capture.Evidence)
	if err != nil {
		t.Fatal(err)
	}
	var value repository.ContentEvidence
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(value, capture.Evidence) || value.ObservedAt.After(time.Now().UTC()) {
		t.Fatal("capture provenance changed in JSON")
	}
}

func TestDirectBasicEvidenceRunsShippedCueWithoutOracleOrAgent(t *testing.T) {
	root := createM2Repository(t)
	output := t.TempDir()
	manifest, err := RunM2(context.Background(), M2Config{
		RepositoryPath: root, MaxTokens: 500, Conditions: []string{ConditionDirect, ConditionBasic},
		CueExecutable: buildCalibrationExecutable(t), OutputDirectory: output, RunIndex: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Reports) != 2 || manifest.CalibrationEvidence == nil {
		t.Fatalf("missing direct/basic evidence: %+v", manifest)
	}
	proof := manifest.CalibrationEvidence
	if len(proof.CueCLI) != 1 || proof.CueCLI[0].GitStatus != "observed" || proof.CueCLI[0].DurationMS <= 0 {
		t.Fatalf("full cue CLI process not observed: %+v", proof.CueCLI)
	}
	if proof.CueCLI[0].GitInvocations == nil || *proof.CueCLI[0].GitInvocations == 0 {
		t.Fatal("real cue Git process events are missing")
	}
	sets, err := os.ReadDir(output)
	if err != nil || len(sets) != 1 {
		t.Fatalf("published set: %v %v", sets, err)
	}
	directory := filepath.Join(output, sets[0].Name())
	serialized, err := os.ReadFile(filepath.Join(directory, "calibration-evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate("calibration-evidence-v1.schema.json", serialized); err != nil {
		t.Fatal(err)
	}
	for _, ref := range proof.Reports {
		data, err := evidence.ReadReference(directory, ref.FileReference)
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate("evaluation-v4.schema.json", data); err != nil {
			t.Fatal(err)
		}
		var report M2ConditionReport
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		if err := ValidateCalibrationReport(report, model.Basis{}); err == nil {
			t.Fatal("runnerless report was accepted as agent calibration")
		}
	}
	for _, ref := range proof.Records {
		data, err := evidence.ReadReference(directory, ref)
		if err != nil {
			t.Fatal(err)
		}
		if ref.Path == "basic-cue.json" && evidence.Digest(data) != proof.CueCLI[0].OutputDigest {
			t.Fatal("retained cue differs from measured stdout")
		}
		if ref.Path == "cue-git-events.jsonl" {
			count, err := CountGitInvocations(data)
			if err != nil || count != *proof.CueCLI[0].GitInvocations {
				t.Fatalf("Git count cannot be reproduced: %d %v", count, err)
			}
		}
	}
}
