package app

import (
	"context"
	"errors"
	"os"

	"github.com/jeonghanlee/repocue/internal/evaluation"
	"github.com/jeonghanlee/repocue/internal/taskbenchmark"
)

func (a App) runEvaluateTask(ctx context.Context, args []string) error {
	if len(args) > 0 && args[0] == "summary" {
		flags := newFlagSet("evaluate-task")
		directory := flags.String("input-directory", "", "external retained pair directory")
		assessment := flags.String("assessment-file", "", "bound source-backed assessment bundle")
		if err := a.parseFlags(flags, args[1:]); err != nil {
			return err
		}
		if err := requireFlagsOnly(flags); err != nil {
			return err
		}
		if *directory == "" {
			return errors.New("input-directory is required")
		}
		summary, err := evaluation.SummarizeTasks(*directory, *assessment)
		if err != nil {
			return err
		}
		return a.writeJSON(summary)
	}
	flags := newFlagSet("evaluate-task")
	repository := flags.String("repository", ".", "evaluated Git repository")
	cache := flags.String("cache-dir", "", "maintained cache root")
	task := flags.String("task-file", "", "frozen task-benchmark-1 JSON")
	output := flags.String("output-directory", "", "external attempt/evidence directory")
	runner := flags.String("runner", "", "explicit executable; omitted prepares context without an agent")
	order := flags.String("order", "overview,task", "one pair: overview,task or task,overview")
	binary := flags.String("cue-binary", "", "actual RepoCue CLI; defaults to this executable")
	codex := flags.String("codex-binary", "codex", "provider executable in explicit runner mode")
	model := flags.String("model", "", "explicit provider model")
	reasoning := flags.String("reasoning-effort", "", "explicit provider reasoning effort")
	speed := flags.String("speed-tier", "standard", "declared standard or fast provider setting")
	rates := flags.String("rates-file", "", "dated compatible credit rate record; omission leaves cost unknown")
	if err := a.parseFlags(flags, args); err != nil {
		return err
	}
	if err := requireFlagsOnly(flags); err != nil {
		return err
	}
	if *task == "" || *output == "" {
		return errors.New("task-file and external output-directory are required")
	}
	if *binary == "" {
		value, err := os.Executable()
		if err != nil {
			return err
		}
		*binary = value
	}
	report, err := evaluation.RunTask(ctx, evaluation.TaskConfig{Repository: *repository, CacheDir: *cache,
		TaskFile: *task, OutputDirectory: *output, CLI: *binary, Runner: *runner, Order: *order, RatesFile: *rates,
		Settings: taskbenchmark.Settings{CodexBinary: *codex, Model: *model, ReasoningEffort: *reasoning, SpeedTier: *speed}})
	if report.ID != "" {
		if outputErr := a.writeJSON(report); outputErr != nil {
			return outputErr
		}
	}
	return err
}
