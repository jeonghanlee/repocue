package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeonghanlee/repocue/internal/cue"
	"github.com/jeonghanlee/repocue/internal/evaluation"
	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/jeonghanlee/repocue/internal/repository"
	"github.com/jeonghanlee/repocue/internal/snapshot"
	"github.com/jeonghanlee/repocue/internal/storage"
	"github.com/jeonghanlee/repocue/internal/taskcontext"
)

type App struct {
	stdout   io.Writer
	stderr   io.Writer
	warnings *bytes.Buffer
}

// command binds one CLI verb to its argument synopsis, one-line summary,
// and handler. The help output is derived from this table so every
// dispatchable verb is documented.
type command struct {
	name     string
	synopsis string
	summary  string
	detail   string
	run      func(App, context.Context, []string) error
}

const (
	programName      = "repocue"
	helpCommand      = "help"
	documentationURL = "https://jeonghanlee.github.io/repocue/"
)

var evaluationContractNote = "External runner and M2 experiment contracts are documented in docs/EVALUATION.md (" + documentationURL + "EVALUATION.html)."

var helpFlags = map[string]bool{"-h": true, "--help": true, "-help": true}

// commands is assigned in init so handlers may reference the table
// without forming a package initialization cycle.
var commands []command

func init() {
	commands = []command{
		{"init", "[flags] [repository]", "Initialize a Git repository with a deterministic full baseline",
			"The repository defaults to the current directory when omitted.", App.runInit},
		{"status", "[flags]", "Inspect the live Git basis and cached RepoCue state", "", App.runStatus},
		{"refresh", "[flags]", "Refresh changed tracked files and publish a snapshot when indexed state changed", "", App.runRefresh},
		{"rebaseline", "[flags]", "Start a new epoch with a full baseline while retaining the superseded epoch", "", App.runRebaseline},
		{"cue", "[flags]", "Generate a compact repository cue within an estimated token budget", "", App.runCue},
		{"metrics", "[flags]", "Read recorded baseline, refresh, and cue measurements", "", App.runMetrics},
		{"calibration", "import [flags]", "Import validated external direct/basic calibration evidence", "Raw evidence remains external; no agent is invoked.", App.runCalibration},
		{"report", "[flags]", "Report maintained costs and explicitly projected consumer savings", "Read-only; does not refresh, migrate, generate a cue, or invoke an agent.", App.runReport},
		{"evaluate", "[flags]", "Run the model-neutral direct-versus-assisted repository evaluation",
			evaluationContractNote, App.runEvaluate},
		{"evaluate-m2", "[flags]", "Run the five-condition M2 evaluation and publish one report set",
			evaluationContractNote, App.runEvaluateM2},
		{"evaluate-task", "[flags] | summary [flags]", "Prepare task context, explicitly run one pair, or summarize retained attempts",
			"Task contracts and evidence are separate from direct/basic calibration.", App.runEvaluateTask},
	}
}

// Run dispatches the CLI. Help requests (help, -h, --help, help <command>,
// <command> -h) print plain text to stdout and exit 0; a missing command
// prints usage to stderr and exits 2; every other failure is a JSON error
// on stderr with exit 1. Nonfatal warnings follow the command result so a
// failure's JSON error remains the first stderr line.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var warnings bytes.Buffer
	defer warnings.WriteTo(stderr)
	application := App{stdout: stdout, stderr: stderr, warnings: &warnings}
	if len(args) == 0 {
		application.writeUsage(stderr)
		return 2
	}
	if args[0] == helpCommand || helpFlags[args[0]] {
		return application.runHelp(ctx, args[1:])
	}
	cmd, found := lookupCommand(args[0])
	if !found {
		application.writeError(fmt.Errorf("unknown command %q", args[0]))
		fmt.Fprintf(stderr, "Run '%s help' for usage.\n", programName)
		return 1
	}
	err := cmd.run(application, ctx, args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		application.writeError(err)
		return 1
	}
	return 0
}

func lookupCommand(name string) (command, bool) {
	for _, cmd := range commands {
		if cmd.name == name {
			return cmd, true
		}
	}
	return command{}, false
}

// runHelp prints the global usage, or delegates to the named command with a
// help flag so its own flag set renders the command-level usage.
func (a App) runHelp(ctx context.Context, args []string) int {
	if len(args) > 1 {
		a.writeError(errors.New("help accepts at most one command name"))
		return 1
	}
	if len(args) == 0 || args[0] == helpCommand || helpFlags[args[0]] {
		a.writeUsage(a.stdout)
		return 0
	}
	cmd, found := lookupCommand(args[0])
	if !found {
		a.writeError(fmt.Errorf("unknown command %q", args[0]))
		a.writeUsage(a.stderr)
		return 1
	}
	if err := cmd.run(a, ctx, []string{"-h"}); !errors.Is(err, flag.ErrHelp) {
		a.writeError(err)
		return 1
	}
	return 0
}

func (a App) writeUsage(out io.Writer) {
	fmt.Fprintf(out, "Usage: %s <command> [flags]\n\nCommands:\n", programName)
	width := len(helpCommand)
	for _, cmd := range commands {
		if len(cmd.name) > width {
			width = len(cmd.name)
		}
	}
	for _, cmd := range commands {
		fmt.Fprintf(out, "  %-*s  %s\n", width, cmd.name, cmd.summary)
	}
	fmt.Fprintf(out, "  %-*s  %s\n", width, helpCommand, "Show this usage or the usage of one command")
	fmt.Fprintf(out, "\nRun '%s help <command>' or '%s <command> --help' for command flags.\n", programName, programName)
}

func (a App) runInit(ctx context.Context, args []string) error {
	flags := newFlagSet("init")
	cacheDir := flags.String("cache-dir", "", "cache root")
	// A "--" terminator splits the arguments before parsing so it can never
	// be consumed as a flag value; without one, flags may also follow the
	// repository path and are parsed as a second round.
	flagArgs, tail, terminated := splitAtTerminator(args)
	if err := a.parseFlags(flags, flagArgs); err != nil {
		return err
	}
	positionals := flags.Args()
	if terminated {
		positionals = append(positionals, tail...)
	} else if len(positionals) > 1 {
		if err := a.parseFlags(flags, positionals[1:]); err != nil {
			return err
		}
		positionals = append([]string{positionals[0]}, flags.Args()...)
	}
	if len(positionals) > 1 {
		return errors.New("init accepts at most one repository path")
	}
	repositoryPath := "."
	if len(positionals) == 1 {
		repositoryPath = positionals[0]
	}
	repo, err := a.openRepository(ctx, repositoryPath, *cacheDir)
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
	if err := a.parseFlags(flags, args); err != nil {
		return err
	}
	if err := requireFlagsOnly(flags); err != nil {
		return err
	}
	repo, err := a.openRepository(ctx, *repositoryPath, *cacheDir)
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
	if err := a.parseFlags(flags, args); err != nil {
		return err
	}
	if err := requireFlagsOnly(flags); err != nil {
		return err
	}
	repo, store, err := a.repositoryAndStore(ctx, *repositoryPath, *cacheDir)
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
	if err := a.parseFlags(flags, args); err != nil {
		return err
	}
	if err := requireFlagsOnly(flags); err != nil {
		return err
	}
	repo, store, err := a.repositoryAndStore(ctx, *repositoryPath, *cacheDir)
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
	view := flags.String("view", "overview", "cue view: overview, task, ranked, provenance, or placebo; delta or delta-v2 with --since")
	since := flags.String("since", "", "starting snapshot id for a delta view (overview becomes delta)")
	pathPrefix := flags.String("path", "", "path filter for provenance")
	maxTokens := flags.Int("max-tokens", 500, "maximum estimated tokens")
	contextFile := flags.String("context-file", "", "tracked task-context declaration (task view)")
	taskKind := flags.String("task-kind", "", "explicit declared task kind (task view)")
	var targets stringList
	flags.Var(&targets, "target", "repository-relative target; repeat for multiple targets (task view)")
	if err := a.parseFlags(flags, args); err != nil {
		return err
	}
	if err := requireFlagsOnly(flags); err != nil {
		return err
	}
	if *maxTokens < 1 {
		return errors.New("max-tokens must be positive")
	}
	taskFlags, legacyTaskFlags := false, false
	flags.Visit(func(value *flag.Flag) {
		switch value.Name {
		case "context-file", "task-kind", "target":
			taskFlags = true
		case "since", "path":
			legacyTaskFlags = true
		}
	})
	if *view != "task" && taskFlags {
		return errors.New("task inputs require --view task")
	}
	if *view == "task" && (legacyTaskFlags || *contextFile == "" || *taskKind == "" || len(targets) == 0) {
		return errors.New("task view requires context-file, task-kind and target, and rejects since/path")
	}
	repo, store, err := a.repositoryAndStore(ctx, *repositoryPath, *cacheDir)
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
		case "task":
			serialized, estimated, err = taskCue(ctx, repo, state, *contextFile,
				taskcontext.Request{TaskKind: *taskKind, Targets: targets}, freshness, *maxTokens)
		case "overview":
			serialized, estimated, err = cue.Overview(state, freshness, *maxTokens)
		case "placebo":
			serialized, estimated, err = cue.Placebo(state, freshness, *maxTokens)
		case "ranked":
			facts, factsErr := repo.ContextFactsAt(ctx, state.Snapshot, state.Files)
			if factsErr != nil {
				return fmt.Errorf("build ranked cue from snapshot %s: %w", state.Snapshot.ID, factsErr)
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
	if err := a.parseFlags(flags, args); err != nil {
		return err
	}
	if err := requireFlagsOnly(flags); err != nil {
		return err
	}
	_, store, err := a.repositoryAndStore(ctx, *repositoryPath, *cacheDir)
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
	if err := a.parseFlags(flags, args); err != nil {
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
	outputDirectory := flags.String("output-directory", "", "final report-set parent directory")
	runIndex := flags.Int("run-index", 1, "positive run index")
	temporaryRoot := flags.String("temporary-root", "", "evaluation temporary workspace parent")
	conditions := flags.String("conditions", "", "direct,basic for calibration; default is all five M2 conditions")
	if err := a.parseFlags(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("evaluate-m2 accepts flags only")
	}
	var selected []string
	if *conditions != "" {
		selected = strings.Split(*conditions, ",")
	}
	manifest, err := evaluation.RunM2(ctx, evaluation.M2Config{
		RepositoryPath: *repositoryPath, MaxTokens: *maxTokens, TaskFile: *taskFile,
		Runner: *runner, OracleTool: *oracleTool, OutputDirectory: *outputDirectory,
		RunIndex: *runIndex, TemporaryRoot: *temporaryRoot, Conditions: selected,
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

func (a App) openRepository(ctx context.Context, repositoryPath, cacheDir string) (*repository.Repository, error) {
	root, err := cacheRoot(cacheDir)
	if err != nil {
		return nil, err
	}
	repo, warning, err := repository.OpenCached(ctx, repositoryPath, root)
	if warning != nil {
		fmt.Fprintf(a.warnings, "warning: %v\n", warning)
	}
	return repo, err
}

func (a App) repositoryAndStore(ctx context.Context, repositoryPath, cacheDir string) (*repository.Repository, *storage.Store, error) {
	repo, err := a.openRepository(ctx, repositoryPath, cacheDir)
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
	root, err := cacheRoot(override)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, repo.ID, "state.db"), nil
}

func cacheRoot(override string) (string, error) {
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
	return root, nil
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

// parseFlags parses command arguments; a help request renders the command
// usage with its flag defaults to stdout and returns flag.ErrHelp.
func (a App) parseFlags(flags *flag.FlagSet, args []string) error {
	err := flags.Parse(args)
	if !errors.Is(err, flag.ErrHelp) {
		return err
	}
	cmd, _ := lookupCommand(flags.Name())
	fmt.Fprintf(a.stdout, "Usage: %s %s %s\n\n%s\n", programName, cmd.name, cmd.synopsis, cmd.summary)
	if cmd.detail != "" {
		fmt.Fprintf(a.stdout, "%s\n", cmd.detail)
	}
	fmt.Fprintf(a.stdout, "\nFlags:\n")
	flags.SetOutput(a.stdout)
	flags.PrintDefaults()
	return err
}

// splitAtTerminator divides args at the first "--" into the flag-bearing
// head and the positional tail, reporting whether a terminator was present.
func splitAtTerminator(args []string) ([]string, []string, bool) {
	for index, arg := range args {
		if arg == "--" {
			return args[:index], args[index+1:], true
		}
	}
	return args, nil, false
}

func requireFlagsOnly(flags *flag.FlagSet) error {
	if flags.NArg() != 0 {
		return fmt.Errorf("%s accepts flags only", flags.Name())
	}
	return nil
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
