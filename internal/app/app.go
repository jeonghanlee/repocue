package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/jeonghanlee/repocue/internal/cue"
	"github.com/jeonghanlee/repocue/internal/evaluation"
	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/jeonghanlee/repocue/internal/repository"
	"github.com/jeonghanlee/repocue/internal/snapshot"
	"github.com/jeonghanlee/repocue/internal/storage"
)

type App struct {
	stdout io.Writer
	stderr io.Writer
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	application := App{stdout: stdout, stderr: stderr}
	if len(args) == 0 {
		application.writeError(errors.New("command is required"))
		return 2
	}
	var err error
	switch args[0] {
	case "init":
		err = application.runInit(ctx, args[1:])
	case "status":
		err = application.runStatus(ctx, args[1:])
	case "refresh":
		err = application.runRefresh(ctx, args[1:])
	case "rebaseline":
		err = application.runRebaseline(ctx, args[1:])
	case "cue":
		err = application.runCue(ctx, args[1:])
	case "metrics":
		err = application.runMetrics(ctx, args[1:])
	case "evaluate":
		err = application.runEvaluate(ctx, args[1:])
	case "evaluate-m2":
		err = application.runEvaluateM2(ctx, args[1:])
	default:
		err = fmt.Errorf("unknown command %q", args[0])
	}
	if err != nil {
		application.writeError(err)
		return 1
	}
	return 0
}

func (a App) runInit(ctx context.Context, args []string) error {
	flags := newFlagSet("init")
	cacheDir := flags.String("cache-dir", "", "cache root")
	if err := flags.Parse(args); err != nil {
		return err
	}
	repositoryPath := "."
	if flags.NArg() > 1 {
		return errors.New("init accepts at most one repository path")
	}
	if flags.NArg() == 1 {
		repositoryPath = flags.Arg(0)
	}
	repo, err := repository.Open(ctx, repositoryPath)
	if err != nil {
		return err
	}
	scan, err := repo.FullScan(ctx)
	if err != nil {
		return err
	}
	store, err := openStore(ctx, repo, *cacheDir)
	if err != nil {
		return err
	}
	defer store.Close()
	transition, err := store.Initialize(ctx, repositoryModel(repo, scan.Basis.ObservedAt), scan)
	if err != nil {
		return err
	}
	return a.writeJSON(operationOutput("init", repo, transition))
}

func (a App) runStatus(ctx context.Context, args []string) error {
	flags := newFlagSet("status")
	repositoryPath := flags.String("repository", ".", "repository path")
	cacheDir := flags.String("cache-dir", "", "cache root")
	if err := flags.Parse(args); err != nil {
		return err
	}
	repo, err := repository.Open(ctx, *repositoryPath)
	if err != nil {
		return err
	}
	path, err := statePath(repo, *cacheDir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return a.writeJSON(map[string]any{
			"schema_version": model.SchemaVersion,
			"kind":           "status",
			"repository":     map[string]any{"id": repo.ID, "name": repo.Name, "root": repo.Root},
			"initialized":    false,
			"freshness":      "unknown",
		})
	}
	store, err := storage.Open(ctx, path)
	if err != nil {
		return err
	}
	defer store.Close()
	state, live, freshness, items, err := inspectCurrent(ctx, repo, store)
	if err != nil {
		return err
	}
	changedPaths := make([]string, 0)
	for _, item := range items {
		if item.Path != "" {
			changedPaths = append(changedPaths, item.Path)
		}
	}
	return a.writeJSON(map[string]any{
		"schema_version": model.SchemaVersion,
		"kind":           "status",
		"repository":     map[string]any{"id": repo.ID, "name": repo.Name, "root": repo.Root},
		"initialized":    true,
		"epoch":          state.Epoch,
		"snapshot":       state.Snapshot.ID,
		"epoch_count":    state.EpochCount,
		"indexed_basis":  state.Snapshot.Basis,
		"live_basis":     live.Basis,
		"freshness":      freshness,
		"changed_paths":  changedPaths,
		"database":       path,
	})
}

func (a App) runRefresh(ctx context.Context, args []string) error {
	flags := newFlagSet("refresh")
	repositoryPath := flags.String("repository", ".", "repository path")
	cacheDir := flags.String("cache-dir", "", "cache root")
	if err := flags.Parse(args); err != nil {
		return err
	}
	repo, store, err := repositoryAndStore(ctx, *repositoryPath, *cacheDir)
	if err != nil {
		return err
	}
	defer store.Close()
	state, err := store.Current(ctx, repo.ID)
	if err != nil {
		return err
	}
	scan, err := repo.IncrementalScan(ctx, state.Files)
	if err != nil {
		return err
	}
	after := snapshotFromScan(state, scan)
	items := snapshot.Diff(repo.ID, state.Snapshot, after, state.Files, scan.Files)
	transition, err := store.Refresh(ctx, state, scan, items)
	if err != nil {
		return err
	}
	output := operationOutput("refresh", repo, transition)
	output["delta"] = transition.DeltaID
	output["changed"] = transition.Changed
	return a.writeJSON(output)
}

func (a App) runRebaseline(ctx context.Context, args []string) error {
	flags := newFlagSet("rebaseline")
	repositoryPath := flags.String("repository", ".", "repository path")
	cacheDir := flags.String("cache-dir", "", "cache root")
	label := flags.String("label", "manual", "epoch label")
	reason := flags.String("reason", "manual", "rebaseline reason")
	if err := flags.Parse(args); err != nil {
		return err
	}
	repo, store, err := repositoryAndStore(ctx, *repositoryPath, *cacheDir)
	if err != nil {
		return err
	}
	defer store.Close()
	state, err := store.Current(ctx, repo.ID)
	if err != nil {
		return err
	}
	scan, err := repo.FullScan(ctx)
	if err != nil {
		return err
	}
	transition, err := store.Rebaseline(ctx, state, scan, *label, *reason)
	if err != nil {
		return err
	}
	output := operationOutput("rebaseline", repo, transition)
	output["superseded_epoch"] = state.Epoch.ID
	return a.writeJSON(output)
}

func (a App) runCue(ctx context.Context, args []string) error {
	flags := newFlagSet("cue")
	repositoryPath := flags.String("repository", ".", "repository path")
	cacheDir := flags.String("cache-dir", "", "cache root")
	view := flags.String("view", "overview", "cue view")
	since := flags.String("since", "", "starting snapshot")
	pathPrefix := flags.String("path", "", "path filter for provenance")
	maxTokens := flags.Int("max-tokens", 500, "maximum estimated tokens")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *maxTokens < 1 {
		return errors.New("max-tokens must be positive")
	}
	repo, store, err := repositoryAndStore(ctx, *repositoryPath, *cacheDir)
	if err != nil {
		return err
	}
	defer store.Close()
	state, _, freshness, _, err := inspectCurrent(ctx, repo, store)
	if err != nil {
		return err
	}
	var serialized []byte
	var estimated int
	actualView := *view
	var sincePointer *string
	if *since != "" {
		if actualView == "overview" {
			actualView = "delta"
		}
		if actualView != "delta" && actualView != "delta-v2" {
			return fmt.Errorf("unsupported cue view %q with --since", actualView)
		}
		from, err := store.StateAt(ctx, repo.ID, *since)
		if err != nil {
			return fmt.Errorf("load snapshot %s: %w", *since, err)
		}
		items := snapshot.Diff(repo.ID, from.Snapshot, state.Snapshot, from.Files, state.Files)
		if actualView == "delta-v2" {
			serialized, estimated, err = cue.DeltaV2(state, from.Snapshot, items, freshness, *maxTokens)
		} else {
			serialized, estimated, err = cue.Delta(state, from.Snapshot, items, freshness, *maxTokens)
		}
		sincePointer = since
	} else {
		switch actualView {
		case "overview":
			serialized, estimated, err = cue.Overview(state, freshness, *maxTokens)
		case "placebo":
			serialized, estimated, err = cue.Placebo(state, freshness, *maxTokens)
		case "ranked":
			facts, factsErr := repo.ContextFacts(ctx)
			if factsErr != nil {
				return factsErr
			}
			serialized, estimated, err = cue.RankedOverview(state, cue.RankedFacts{
				RecentCommits: facts.RecentCommits, EntryPoints: facts.EntryPoints, MakeTargets: facts.MakeTargets,
			}, freshness, *maxTokens)
		case "provenance":
			serialized, estimated, err = cue.Provenance(state, *pathPrefix, freshness, *maxTokens)
		default:
			return fmt.Errorf("unsupported cue view %q", actualView)
		}
	}
	if err != nil {
		return err
	}
	if err := store.RecordCue(ctx, storage.CueRun{
		View: actualView, SnapshotID: state.Snapshot.ID, SinceSnapshotID: sincePointer,
		CreatedAt: time.Now().UTC(), MaxTokens: *maxTokens, OutputBytes: len(serialized), EstimatedTokens: estimated,
	}); err != nil {
		return err
	}
	_, err = fmt.Fprintf(a.stdout, "%s\n", serialized)
	return err
}

func (a App) runMetrics(ctx context.Context, args []string) error {
	flags := newFlagSet("metrics")
	repositoryPath := flags.String("repository", ".", "repository path")
	cacheDir := flags.String("cache-dir", "", "cache root")
	if err := flags.Parse(args); err != nil {
		return err
	}
	_, store, err := repositoryAndStore(ctx, *repositoryPath, *cacheDir)
	if err != nil {
		return err
	}
	defer store.Close()
	report, err := store.Metrics(ctx)
	if err != nil {
		return err
	}
	return a.writeJSON(map[string]any{"schema_version": model.SchemaVersion, "kind": "metrics", "content": report})
}

func (a App) runEvaluate(ctx context.Context, args []string) error {
	flags := newFlagSet("evaluate")
	repositoryPath := flags.String("repository", ".", "repository path")
	maxTokens := flags.Int("max-tokens", 500, "overview cue token budget")
	taskFile := flags.String("task-file", "", "runner task file")
	directRunner := flags.String("direct-runner", "", "direct discovery runner executable")
	assistedRunner := flags.String("assisted-runner", "", "RepoCue-assisted runner executable")
	temporaryRoot := flags.String("temporary-root", "", "evaluation temporary workspace parent")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("evaluate accepts flags only")
	}
	report, err := evaluation.Run(ctx, evaluation.Config{
		RepositoryPath: *repositoryPath,
		MaxTokens:      *maxTokens,
		TaskFile:       *taskFile,
		DirectRunner:   *directRunner,
		AssistedRunner: *assistedRunner,
		TemporaryRoot:  *temporaryRoot,
	})
	if err != nil {
		return err
	}
	return a.writeJSON(report)
}

func (a App) runEvaluateM2(ctx context.Context, args []string) error {
	flags := newFlagSet("evaluate-m2")
	repositoryPath := flags.String("repository", ".", "repository path")
	maxTokens := flags.Int("max-tokens", 500, "condition context token budget")
	taskFile := flags.String("task-file", "", "runner task file")
	runner := flags.String("runner", "", "condition runner executable")
	oracleTool := flags.String("oracle-tool", "", "structural oracle executable")
	outputDirectory := flags.String("output-directory", "", "final condition report directory")
	runIndex := flags.Int("run-index", 1, "positive run index")
	temporaryRoot := flags.String("temporary-root", "", "evaluation temporary workspace parent")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("evaluate-m2 accepts flags only")
	}
	manifest, err := evaluation.RunM2(ctx, evaluation.M2Config{
		RepositoryPath: *repositoryPath, MaxTokens: *maxTokens, TaskFile: *taskFile,
		Runner: *runner, OracleTool: *oracleTool, OutputDirectory: *outputDirectory,
		RunIndex: *runIndex, TemporaryRoot: *temporaryRoot,
	})
	if err != nil {
		return err
	}
	return a.writeJSON(manifest)
}

func inspectCurrent(ctx context.Context, repo *repository.Repository, store *storage.Store) (model.CurrentState, model.Scan, string, []model.DeltaItem, error) {
	state, err := store.Current(ctx, repo.ID)
	if err != nil {
		return model.CurrentState{}, model.Scan{}, "", nil, err
	}
	live, err := repo.IncrementalScan(ctx, state.Files)
	if err != nil {
		return model.CurrentState{}, model.Scan{}, "", nil, err
	}
	after := snapshotFromScan(state, live)
	items := snapshot.Diff(repo.ID, state.Snapshot, after, state.Files, live.Files)
	freshness := "refresh-needed"
	if state.Snapshot.RepositoryDigest == live.RepositoryDigest && state.Snapshot.Basis.StatusDigest == live.Basis.StatusDigest {
		freshness = "current"
		if live.Basis.Dirty {
			freshness = "dirty-but-indexed"
		}
	}
	return state, live, freshness, items, nil
}

func snapshotFromScan(state model.CurrentState, scan model.Scan) model.Snapshot {
	return model.Snapshot{
		ID: state.Snapshot.ID, EpochID: state.Epoch.ID, Basis: scan.Basis,
		RepositoryDigest: scan.RepositoryDigest, FileCount: len(scan.Files),
	}
}

func repositoryAndStore(ctx context.Context, repositoryPath, cacheDir string) (*repository.Repository, *storage.Store, error) {
	repo, err := repository.Open(ctx, repositoryPath)
	if err != nil {
		return nil, nil, err
	}
	path, err := statePath(repo, cacheDir)
	if err != nil {
		return nil, nil, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, nil, storage.ErrNotInitialized
	}
	store, err := storage.Open(ctx, path)
	if err != nil {
		return nil, nil, err
	}
	return repo, store, nil
}

func openStore(ctx context.Context, repo *repository.Repository, cacheDir string) (*storage.Store, error) {
	path, err := statePath(repo, cacheDir)
	if err != nil {
		return nil, err
	}
	return storage.Open(ctx, path)
}

func statePath(repo *repository.Repository, override string) (string, error) {
	root := override
	if root == "" {
		root = os.Getenv("REPOCUE_CACHE_DIR")
	}
	if root == "" {
		if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
			root = filepath.Join(xdg, "repocue")
		} else {
			cache, err := os.UserCacheDir()
			if err != nil {
				return "", err
			}
			root = filepath.Join(cache, "repocue")
		}
	}
	return filepath.Join(root, repo.ID, "state.db"), nil
}

func repositoryModel(repo *repository.Repository, created time.Time) model.Repository {
	return model.Repository{ID: repo.ID, Name: repo.Name, Root: repo.Root, GitDir: repo.GitDir, CreatedAt: created}
}

func operationOutput(operation string, repo *repository.Repository, transition storage.Transition) map[string]any {
	freshness := "current"
	if transition.Snapshot.Basis.Dirty {
		freshness = "dirty-but-indexed"
	}
	return map[string]any{
		"schema_version": model.SchemaVersion,
		"kind":           "operation",
		"operation":      operation,
		"repository":     map[string]any{"id": repo.ID, "name": repo.Name, "root": repo.Root},
		"epoch":          transition.Epoch.ID,
		"snapshot":       transition.Snapshot.ID,
		"basis":          transition.Snapshot.Basis,
		"freshness":      freshness,
		"metrics": map[string]any{
			"duration_ms":         transition.Run.DurationMS,
			"scan_duration_ms":    transition.Run.ScanDurationMS,
			"git_commands":        transition.Run.GitCommands,
			"files_scanned":       transition.Run.FilesScanned,
			"bytes_scanned":       transition.Run.BytesScanned,
			"database_size_bytes": transition.DatabaseSizeBytes,
		},
	}
}

func newFlagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	return flags
}

func (a App) writeJSON(value any) error {
	serialized, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(a.stdout, "%s\n", serialized)
	return err
}

func (a App) writeError(err error) {
	serialized, marshalErr := json.Marshal(map[string]any{
		"schema_version": model.SchemaVersion,
		"kind":           "error",
		"error":          err.Error(),
	})
	if marshalErr != nil {
		fmt.Fprintln(a.stderr, err)
		return
	}
	fmt.Fprintf(a.stderr, "%s\n", serialized)
}
