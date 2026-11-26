package store

import (
	"io"
	"time"
)

// Stub of the test writer: the builder of work item 2 replaces this file.

func NewID() (string, error) { return "", nil }

func uuidV7(at time.Time, random io.Reader) (string, error) { return "", nil }
