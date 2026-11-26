//go:build !linux && !darwin

package sysinfo

import "errors"

// ReadFacts is not supported on this operating system.
func ReadFacts(clientVersion string) (Facts, error) {
	return Facts{}, errors.ErrUnsupported
}

// ReadLoad is not supported on this operating system.
func ReadLoad() (Load, error) {
	return Load{}, errors.ErrUnsupported
}
