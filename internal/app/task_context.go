package app

import (
	"context"

	"github.com/jeonghanlee/repocue/internal/cue"
	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/jeonghanlee/repocue/internal/repository"
	"github.com/jeonghanlee/repocue/internal/taskcontext"
)

type stringList []string

func (values *stringList) String() string { return "" }
func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func taskCue(ctx context.Context, repo *repository.Repository, state model.CurrentState, contextFile string, request taskcontext.Request, freshness string, maxTokens int) ([]byte, int, error) {
	if _, err := repo.ValidateSnapshot(ctx, state.Snapshot, state.Files); err != nil {
		return nil, 0, err
	}
	serialized, estimate, err := cue.Task(ctx, state, contextFile, request, freshness, maxTokens)
	if err != nil {
		return nil, 0, err
	}
	if _, err := repo.ValidateSnapshot(ctx, state.Snapshot, state.Files); err != nil {
		return nil, 0, err
	}
	return serialized, estimate, nil
}
