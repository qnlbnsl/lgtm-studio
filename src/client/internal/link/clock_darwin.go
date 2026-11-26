package link

import (
	"time"

	"golang.org/x/sys/unix"
)

// elapsedClock reads the clock that keeps counting while the machine sleeps.
func elapsedClock() (time.Duration, error) {
	var reading unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC_RAW, &reading); err != nil {
		return 0, err
	}
	return time.Duration(reading.Nano()), nil
}
