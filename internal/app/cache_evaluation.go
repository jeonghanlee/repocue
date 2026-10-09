package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/jeonghanlee/repocue/internal/evaluation"
)

func (a App) runEvaluateCache(ctx context.Context, args []string) error {
	if len(args) == 0 || helpFlags[args[0]] {
		a.writeCacheUsage()
		return flag.ErrHelp
	}
	switch args[0] {
	case "prepare":
		flags := newFlagSet("evaluate-cache prepare")
		repo := flags.String("repository", ".", "evaluated Git repository")
		cache := flags.String("cache-dir", "", "maintained cache root")
		benchmark := flags.String("benchmark-file", "", "cache-benchmark-1 JSON")
		output := flags.String("output-directory", "", "external evidence parent")
		cli := flags.String("cue-binary", "", "actual RepoCue CLI; defaults to this executable")
		if err := a.parseCacheFlags(flags, args[1:]); err != nil {
			return err
		}
		if err := requireFlagsOnly(flags); err != nil {
			return err
		}
		if *benchmark == "" || *output == "" {
			return errors.New("benchmark-file and output-directory are required")
		}
		if *cli == "" {
			value, err := os.Executable()
			if err != nil {
				return err
			}
			*cli = value
		}
		report, err := evaluation.PrepareCache(ctx, evaluation.CacheConfig{Repository: *repo, CacheDir: *cache, BenchmarkFile: *benchmark, OutputDirectory: *output, CLI: *cli})
		if report.ID != "" {
			if outputErr := a.writeJSON(report); outputErr != nil {
				return outputErr
			}
		}
		return err
	case "run":
		flags := newFlagSet("evaluate-cache run")
		directory := flags.String("input-directory", "", "one frozen preparation directory")
		step := flags.String("step", "", "exact frozen step ID; one request only")
		endpoint := flags.String("endpoint", "", "official Messages API or explicit HTTPS loopback fixture")
		if err := a.parseCacheFlags(flags, args[1:]); err != nil {
			return err
		}
		if err := requireFlagsOnly(flags); err != nil {
			return err
		}
		if *directory == "" || *step == "" {
			return errors.New("input-directory and step are required")
		}
		observation, err := evaluation.RunCacheStep(ctx, *directory, *step, *endpoint)
		if observation.StepID != "" {
			if outputErr := a.writeJSON(observation); outputErr != nil {
				return outputErr
			}
		}
		return err
	case "summary":
		flags := newFlagSet("evaluate-cache summary")
		directory := flags.String("input-directory", "", "one retained preparation directory")
		assessment := flags.String("assessment-file", "", "optional source-bound independent assessments")
		if err := a.parseCacheFlags(flags, args[1:]); err != nil {
			return err
		}
		if err := requireFlagsOnly(flags); err != nil {
			return err
		}
		if *directory == "" {
			return errors.New("input-directory is required")
		}
		result, err := evaluation.SummarizeCache(*directory, *assessment)
		if err != nil {
			return err
		}
		return a.writeJSON(result)
	default:
		return errors.New("evaluate-cache requires prepare, run or summary")
	}
}

func (a App) parseCacheFlags(flags *flag.FlagSet, args []string) error {
	err := flags.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprintf(a.stdout, "Usage: %s %s [flags]\n\nFlags:\n", programName, flags.Name())
		flags.SetOutput(a.stdout)
		flags.PrintDefaults()
	}
	return err
}

func (a App) writeCacheUsage() {
	a.stdout.Write([]byte("Usage: repocue evaluate-cache <prepare|run|summary> [flags]\n\nPrepare freezes original context without a key or network request.\nRun sends exactly one explicit frozen step.\nSummary reads retained evidence without a process or network request.\n"))
}
