package sysinfo

import "errors"

// This file is a stub of the test writer. The builder replaces every body.

// parseMeminfo reads MemTotal and MemAvailable, in bytes, from the text of /proc/meminfo.
func parseMeminfo(text string) (total, available uint64, err error) {
	return 0, 0, errors.New("stub of the test writer")
}

// parseLoadavg reads the first field of the text of /proc/loadavg.
func parseLoadavg(text string) (float64, error) {
	return 0, errors.New("stub of the test writer")
}

// decodeDarwinLoadavg decodes the value of the macOS sysctl vm.loadavg into the one-minute load.
func decodeDarwinLoadavg(b []byte) (float64, error) {
	return 0, errors.New("stub of the test writer")
}

// decodeDarwinUint64 decodes the value of the macOS sysctl hw.memsize.
func decodeDarwinUint64(b []byte) (uint64, error) {
	return 0, errors.New("stub of the test writer")
}

// cpuPercent turns a one-minute load into a percentage of the cores, from 0 to 100.
func cpuPercent(load1 float64, cores int) uint32 {
	return 0
}

// memoryPercent turns a total and an available number of bytes into a percentage in use.
func memoryPercent(total, available uint64) uint32 {
	return 0
}
