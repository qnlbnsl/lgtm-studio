//go:build linux

package sysinfo

import (
	"os"
	"path/filepath"
	"runtime"
)

// procDir holds meminfo and loadavg. Tests change it.
var procDir = "/proc"

// ReadFacts reads the facts of this machine.
func ReadFacts(clientVersion string) (Facts, error) {
	total, _, err := readMeminfo()
	if err != nil {
		return Facts{}, err
	}
	return machineFacts(OSLinux, clientVersion, total)
}

// ReadLoad reads the load of this machine.
func ReadLoad() (Load, error) {
	total, available, err := readMeminfo()
	if err != nil {
		return Load{}, err
	}
	data, err := os.ReadFile(filepath.Join(procDir, "loadavg"))
	if err != nil {
		return Load{}, err
	}
	load, err := parseLoadavg(string(data))
	if err != nil {
		return Load{}, err
	}
	return Load{
		CPUPercent:    cpuPercent(load, runtime.NumCPU()),
		MemoryPercent: memoryPercent(total, available),
	}, nil
}

func readMeminfo() (total, available uint64, err error) {
	data, err := os.ReadFile(filepath.Join(procDir, "meminfo"))
	if err != nil {
		return 0, 0, err
	}
	return parseMeminfo(string(data))
}
