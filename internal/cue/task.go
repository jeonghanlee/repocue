package cue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/jeonghanlee/repocue/internal/metrics"
	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/jeonghanlee/repocue/internal/taskcontext"
)

const TaskSchemaVersion = "repocue/task-cue-1"

type TaskItem struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	Required       bool     `json:"required"`
	SourceID       string   `json:"source_id"`
	CheckSourceIDs []string `json:"check_source_ids"`
}

type Omission struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type DeclarationIdentity struct {
	Path          string `json:"path"`
	ContentDigest string `json:"content_digest"`
}

type TaskContent struct {
	Declaration  DeclarationIdentity  `json:"declaration"`
	Request      taskcontext.Request  `json:"request"`
	RequiredIDs  []string             `json:"required_ids"`
	Items        []TaskItem           `json:"items"`
	Sources      []taskcontext.Source `json:"sources"`
	Omitted      []Omission           `json:"omitted"`
	OmittedCount int                  `json:"omitted_count"`
}

type TaskEnvelope struct {
	SchemaVersion string      `json:"schema_version"`
	Kind          string      `json:"kind"`
	Repository    Repository  `json:"repository"`
	Epoch         string      `json:"epoch"`
	Snapshot      string      `json:"snapshot"`
	Basis         Basis       `json:"basis"`
	Freshness     string      `json:"freshness"`
	Budget        Budget      `json:"budget"`
	Content       TaskContent `json:"content"`
	Warnings      []Warning   `json:"warnings"`
	More          []More      `json:"more"`
}

// Task assembles mandatory sources and omission accounting first. Optional
// units are admitted whole in item-ID order; all selected files are validated.
func Task(ctx context.Context, state model.CurrentState, contextFile string, request taskcontext.Request, freshness string, maxTokens int) ([]byte, int, error) {
	if freshness != "current" && freshness != "dirty-but-indexed" {
		return nil, 0, errors.New("task cue requires refresh before source delivery")
	}
	if maxTokens < 1 || maxTokens > int(^uint(0)>>1)/4 {
		return nil, 0, errors.New("invalid task token budget")
	}
	reader, err := taskcontext.NewReader(state)
	if err != nil {
		return nil, 0, err
	}
	defer reader.Close()
	declaration, digest, err := reader.Declaration(ctx, contextFile)
	if err != nil {
		return nil, 0, err
	}
	items, request, err := taskcontext.Select(declaration, state.Repository.Root, request)
	if err != nil {
		return nil, 0, err
	}
	base := baseEnvelope(state, freshness, maxTokens)
	envelope := TaskEnvelope{SchemaVersion: TaskSchemaVersion, Kind: "task",
		Repository: base.Repository, Epoch: base.Epoch, Snapshot: base.Snapshot,
		Basis: base.Basis, Freshness: freshness, Budget: base.Budget,
		Warnings: base.Warnings, More: []More{},
		Content: TaskContent{Declaration: DeclarationIdentity{Path: contextFile, ContentDigest: digest},
			Request: request, RequiredIDs: []string{}, Items: []TaskItem{},
			Sources: []taskcontext.Source{}, Omitted: []Omission{}}}
	sources := map[string]taskcontext.Source{}
	usedBytes := 0
	for _, item := range items {
		if !item.Required {
			envelope.Content.Omitted = append(envelope.Content.Omitted, Omission{ID: item.ID, Reason: "budget"})
			continue
		}
		unit, overflow, err := readTaskUnit(ctx, reader, item, sources, maxTokens*4-usedBytes)
		if err != nil {
			return nil, 0, err
		}
		if overflow {
			return nil, 0, fmt.Errorf("%w: required item %s exceeds budget", ErrBudgetTooSmall, item.ID)
		}
		usedBytes += mergeTaskSources(sources, unit)
		envelope.Content.RequiredIDs = append(envelope.Content.RequiredIDs, item.ID)
		envelope.Content.Items = append(envelope.Content.Items, taskItem(item))
	}
	envelope.Content.Sources = sortedTaskSources(sources)
	envelope.Content.OmittedCount = len(envelope.Content.Omitted)
	_, estimate, err := marshalTask(envelope)
	if err != nil {
		return nil, 0, err
	}
	if estimate > maxTokens {
		return nil, estimate, fmt.Errorf("%w: mandatory task envelope needs %d tokens", ErrBudgetTooSmall, estimate)
	}
	for _, item := range items {
		if item.Required {
			continue
		}
		unit, overflow, err := readTaskUnit(ctx, reader, item, sources, maxTokens*4-usedBytes)
		if err != nil {
			return nil, 0, err
		}
		if overflow {
			continue
		}
		trialSources := make(map[string]taskcontext.Source, len(sources)+len(unit))
		for id, source := range sources {
			trialSources[id] = source
		}
		added := mergeTaskSources(trialSources, unit)
		trial := envelope
		trial.Content.Items = append(append([]TaskItem{}, envelope.Content.Items...), taskItem(item))
		trial.Content.Sources = sortedTaskSources(trialSources)
		trial.Content.Omitted = []Omission{}
		for _, omission := range envelope.Content.Omitted {
			if omission.ID != item.ID {
				trial.Content.Omitted = append(trial.Content.Omitted, omission)
			}
		}
		trial.Content.OmittedCount = len(trial.Content.Omitted)
		_, estimate, err = marshalTask(trial)
		if err != nil {
			return nil, 0, err
		}
		if estimate <= maxTokens {
			envelope, sources = trial, trialSources
			usedBytes += added
		}
	}
	sort.Slice(envelope.Content.Items, func(i, j int) bool { return envelope.Content.Items[i].ID < envelope.Content.Items[j].ID })
	return marshalTask(envelope)
}

func taskItem(item taskcontext.Item) TaskItem {
	checks := make([]string, 0, len(item.Checks))
	for _, reference := range item.Checks {
		checks = append(checks, taskcontext.ReferenceID(reference))
	}
	return TaskItem{ID: item.ID, Kind: item.Kind, Required: item.Required,
		SourceID: taskcontext.ReferenceID(item.Source), CheckSourceIDs: checks}
}

func readTaskUnit(ctx context.Context, reader *taskcontext.Reader, item taskcontext.Item, included map[string]taskcontext.Source, remaining int) (map[string]taskcontext.Source, bool, error) {
	unit := map[string]taskcontext.Source{}
	overflow := false
	for _, reference := range append([]taskcontext.Reference{item.Source}, item.Checks...) {
		id := taskcontext.ReferenceID(reference)
		if _, found := included[id]; found {
			continue
		}
		if _, found := unit[id]; found {
			continue
		}
		source, err := reader.Read(ctx, reference, remaining)
		if err != nil {
			return nil, false, err
		}
		if source.Overflow {
			overflow, remaining = true, 0
			unit = map[string]taskcontext.Source{}
		} else if !overflow {
			unit[id] = source
			remaining -= len(source.Text)
		}
	}
	return unit, overflow, nil
}

func mergeTaskSources(target, additions map[string]taskcontext.Source) int {
	bytes := 0
	for id, source := range additions {
		if _, found := target[id]; !found {
			target[id] = source
			bytes += len(source.Text)
		}
	}
	return bytes
}

func sortedTaskSources(sources map[string]taskcontext.Source) []taskcontext.Source {
	result := make([]taskcontext.Source, 0, len(sources))
	for _, source := range sources {
		result = append(result, source)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func marshalTask(envelope TaskEnvelope) ([]byte, int, error) {
	for range 8 {
		serialized, err := json.Marshal(envelope)
		if err != nil {
			return nil, 0, err
		}
		estimate := metrics.EstimateTokens(serialized)
		if estimate == envelope.Budget.EstimatedTokens {
			return serialized, estimate, nil
		}
		envelope.Budget.EstimatedTokens = estimate
	}
	return nil, 0, errors.New("task budget estimate did not converge")
}

func DecodeTask(serialized []byte) (TaskEnvelope, error) {
	var result TaskEnvelope
	err := taskcontext.DecodeStrict(serialized, "task-cue-v1.schema.json", &result)
	return result, err
}
