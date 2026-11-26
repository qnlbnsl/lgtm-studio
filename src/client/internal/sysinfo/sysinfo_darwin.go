//go:build darwin

package sysinfo

import "errors"

// This file is a stub of the test writer. The builder replaces the bodies.

// ReadFacts reads the facts of this machine.
func ReadFacts(clientVersion string) (Facts, error) {
	return Facts{}, errors.New("stub of the test writer")
}

// ReadLoad reads the load of this machine.
func ReadLoad() (Load, error) {
	return Load{}, errors.New("stub of the test writer")
}
