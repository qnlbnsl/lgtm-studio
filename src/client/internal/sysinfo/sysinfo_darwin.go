//go:build darwin

package sysinfo

import (
	"runtime"
	"syscall"
)

// ReadFacts reads the facts of this machine.
func ReadFacts(clientVersion string) (Facts, error) {
	raw, err := syscall.Sysctl("hw.memsize")
	if err != nil {
		return Facts{}, err
	}
	total, err := decodeDarwinUint64([]byte(raw))
	if err != nil {
		return Facts{}, err
	}
	return machineFacts(OSMacOS, clientVersion, total)
}

// ReadLoad reads the load of this machine.
func ReadLoad() (Load, error) {
	raw, err := syscall.Sysctl("vm.loadavg")
	if err != nil {
		return Load{}, err
	}
	load, err := decodeDarwinLoadavg([]byte(raw))
	if err != nil {
		return Load{}, err
	}
	available, err := syscall.SysctlUint32("kern.memorystatus_level")
	if err != nil {
		return Load{}, err
	}
	return Load{
		CPUPercent:    cpuPercent(load, runtime.NumCPU()),
		MemoryPercent: 100 - min(available, 100),
	}, nil
}
