package link

import (
	"errors"
	"time"
)

// elapsedClock reads the raw monotonic clock. Stub of the test writer; the builder uses
// CLOCK_MONOTONIC_RAW here.
func elapsedClock() (time.Duration, error) { return 0, errors.New("stub") }
