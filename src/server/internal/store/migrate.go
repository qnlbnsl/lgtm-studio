package store

import (
	"context"
	"io/fs"
)

// Stub of the test writer: the builder of work item 3 replaces this file.

type Migrations struct {
	Part  string
	Files fs.FS
}

func (s *Store) Migrate(ctx context.Context, parts ...Migrations) error { return nil }
