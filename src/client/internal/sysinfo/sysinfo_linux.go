//go:build linux

package sysinfo

import "errors"

// This file is a stub of the test writer. The builder replaces the bodies.

// procDir is the folder that holds meminfo and loadavg. Tests change it.
var procDir = "/proc"

// ReadFacts reads the facts of this machine.
func ReadFacts(clientVersion string) (Facts, error) {
	return Facts{}, errors.New("stub of the test writer")
}

// ReadLoad reads the load of this machine.
func ReadLoad() (Load, error) {
	return Load{}, errors.New("stub of the test writer")
}
