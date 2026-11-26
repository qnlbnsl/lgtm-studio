package store

import (
	"context"
	"time"
)

// This file contains test writer stubs for retention.

type PruneResult struct {
	FeedRows    int64
	ReceiptRows int64
}

func (s *Store) Prune(context.Context) (PruneResult, error) { return PruneResult{}, nil }

func (s *Store) RunPruner(context.Context, func(error)) {}

func (s *Store) pruneBefore(context.Context, time.Time) (PruneResult, error) {
	return PruneResult{}, nil
}

func (s *Store) runPruner(context.Context, func(time.Duration) <-chan time.Time, func(error)) {}
