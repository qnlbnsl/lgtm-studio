//go:build darwin

package sysinfo

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func sysctlText(t *testing.T, name string) string {
	t.Helper()
	out, err := exec.Command("/usr/sbin/sysctl", "-n", name).Output()
	if err != nil {
		t.Fatalf("sysctl %s: %v", name, err)
	}
	return strings.TrimSpace(string(out))
}

func within(a, b uint32, d uint32) bool {
	if a > b {
		return a-b <= d
	}
	return b-a <= d
}

func TestReadLoadOnMacMatchesSystem(t *testing.T) {
	facts, err := ReadFacts("test")
	if err != nil {
		t.Fatal(err)
	}
	mem, err := strconv.ParseUint(sysctlText(t, "hw.memsize"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if facts.OS != OSMacOS || facts.MemoryBytes != mem {
		t.Errorf("facts are %+v, memory should be %d", facts, mem)
	}

	// The output of vm.loadavg is "{ 1.50 1.20 1.10 }".
	fields := strings.Fields(strings.Trim(sysctlText(t, "vm.loadavg"), "{} "))
	first, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		t.Fatal(err)
	}
	level, err := strconv.ParseUint(sysctlText(t, "kern.memorystatus_level"), 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	load, err := ReadLoad()
	if err != nil {
		t.Fatal(err)
	}
	if !within(load.CPUPercent, cpuPercent(first, runtime.NumCPU()), 2) {
		t.Errorf("CPUPercent is %d", load.CPUPercent)
	}
	if !within(load.MemoryPercent, 100-uint32(level), 2) {
		t.Errorf("MemoryPercent is %d, level is %d", load.MemoryPercent, level)
	}
}
