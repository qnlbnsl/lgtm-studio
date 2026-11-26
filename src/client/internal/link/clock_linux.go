package link

import (
	"errors"
	"time"
)

// elapsedClock reads the clock that keeps counting while the machine sleeps. Stub of the
// test writer; the builder uses CLOCK_BOOTTIME here.
func elapsedClock() (time.Duration, error) { return 0, errors.New("stub") }
