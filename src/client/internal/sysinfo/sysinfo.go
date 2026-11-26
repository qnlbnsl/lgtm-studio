// Package sysinfo reads the facts and the load of the machine the client runs on.
package sysinfo

// The names of the operating systems, as the contract spells them.
const (
	OSLinux = "Linux"
	OSMacOS = "macOS"
)

// Facts are the values that rarely change on a machine.
type Facts struct {
	Hostname      string
	OS            string
	Arch          string
	Cores         uint32
	MemoryBytes   uint64
	ClientVersion string
}

// Load is the pressure on the machine, as two whole percentages from 0 to 100.
type Load struct {
	CPUPercent    uint32
	MemoryPercent uint32
}
