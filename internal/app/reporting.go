package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jeonghanlee/repocue/internal/evidence"
	"github.com/jeonghanlee/repocue/internal/reporting"
	"github.com/jeonghanlee/repocue/internal/repository"
	"github.com/jeonghanlee/repocue/internal/storage"
)

func (a App) runCalibration(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("calibration requires the import subcommand")
	}
	if args[0] == "import" {
		args = args[1:]
	} else if !helpFlags[args[0]] {
		return errors.New("calibration supports only import")
	}
	flags := newFlagSet("calibration")
	repositoryPath := flags.String("repository", ".", "repository path")
	cacheDir := flags.String("cache-dir", "", "cache root")
	reports := flags.String("reports", "", "external report directory")
	evidenceFile := flags.String("evidence", "", "calibration-evidence-1 JSON file")
	assessments := flags.String("assessments", "", "optional external qualitative assessment directory")
	if err := a.parseFlags(flags, args); err != nil {
		return err
	}
	if err := requireFlagsOnly(flags); err != nil {
		return err
	}
	if *reports == "" || *evidenceFile == "" {
		return errors.New("calibration import requires --reports and --evidence")
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
		return storage.ErrNotInitialized
	} else if err != nil {
		return err
	}
	store, err := storage.Open(ctx, path)
	if err != nil {
		return err
	}
	calibration, ref, importErr := reporting.Import(ctx, repo, store, reporting.ImportConfig{Reports: *reports, EvidenceFile: *evidenceFile, Assessments: *assessments})
	closeErr := store.Close()
	if importErr != nil {
		return importErr
	}
	if closeErr != nil {
		return closeErr
	}
	return a.writeJSON(map[string]any{"schema_version": reporting.CalibrationSchema, "kind": "calibration-import", "id": calibration.ID,
		"artifact_path": ref.ArtifactPath, "digest": ref.Digest, "sample_grade": calibration.SampleGrade, "binding": calibration.Binding})
}

func (a App) runReport(ctx context.Context, args []string) error {
	now := time.Now().UTC()
	flags := newFlagSet("report")
	repositoryPath := flags.String("repository", ".", "repository path")
	cacheDir := flags.String("cache-dir", "", "cache root")
	window := flags.String("window", "30d", "positive fixed UTC-day window, for example 30d")
	format := flags.String("format", "json", "json or markdown")
	profile := flags.String("profile", "", "explicit calibration ID; required when multiple profiles exist")
	consumers := flags.Int64("consumers", 0, "additional positive consumer scenario; never inferred from cue runs")
	asOf := flags.String("as-of", "", "non-future RFC3339 UTC observation cutoff")
	if err := a.parseFlags(flags, args); err != nil {
		return err
	}
	if err := requireFlagsOnly(flags); err != nil {
		return err
	}
	if *format != "json" && *format != "markdown" {
		return errors.New("report format must be json or markdown")
	}
	if !strings.HasSuffix(*window, "d") {
		return errors.New("report window must be a positive integer followed by d")
	}
	digits := strings.TrimSuffix(*window, "d")
	if digits == "" || strings.Trim(digits, "0123456789") != "" {
		return errors.New("report window must be a positive integer followed by d")
	}
	days, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || days < 1 || days > int64((1<<63-1)/int64(48*time.Hour)) {
		return errors.New("report window is outside supported day range")
	}
	explicitConsumers := false
	flags.Visit(func(value *flag.Flag) { explicitConsumers = explicitConsumers || value.Name == "consumers" })
	if explicitConsumers && *consumers < 1 {
		return errors.New("consumers must be positive")
	}
	cutoff := now
	if *asOf != "" {
		cutoff, err = time.Parse(time.RFC3339Nano, *asOf)
		if err != nil {
			return fmt.Errorf("invalid UTC observation cutoff: %w", err)
		}
		_, offset := cutoff.Zone()
		if offset != 0 || cutoff.After(now) {
			return errors.New("report cutoff must be a non-future UTC instant")
		}
		cutoff = cutoff.UTC()
	}
	repo, err := repository.Open(ctx, *repositoryPath)
	if err != nil {
		return err
	}
	path, err := statePath(repo, *cacheDir)
	if err != nil {
		return err
	}
	store, err := storage.OpenReadOnly(ctx, path)
	if err != nil {
		return err
	}
	config := reporting.ReportConfig{GeneratedAt: now, Cutoff: cutoff, WindowDays: days, Historical: *asOf != "", Profile: *profile, Consumers: *consumers}
	if executable, err := os.Executable(); err == nil {
		if producer, err := evidence.ExecutableProducer(executable); err == nil {
			config.Producer = &producer
		}
	}
	result, reportErr := reporting.Generate(ctx, repo, store, config)
	closeErr := store.Close()
	if reportErr != nil {
		return reportErr
	}
	if closeErr != nil {
		return closeErr
	}
	return reporting.Write(a.stdout, result, *format)
}
