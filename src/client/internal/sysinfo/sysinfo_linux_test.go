//go:build linux

package sysinfo

import (
	"math"
	"os"
	"runtime"
	"testing"
)

func TestReadOnLinux(t *testing.T) {
	old := procDir
	procDir = "testdata"
	t.Cleanup(func() { procDir = old })

	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	facts, err := ReadFacts("9.9.9-test")
	if err != nil {
		t.Fatalf("ReadFacts: %v", err)
	}
	if facts.OS != OSLinux {
		t.Errorf("OS is %q", facts.OS)
	}
	if facts.MemoryBytes != 16_777_216_000 {
		t.Errorf("MemoryBytes is %d", facts.MemoryBytes)
	}
	if facts.Cores != uint32(runtime.NumCPU()) {
		t.Errorf("Cores is %d", facts.Cores)
	}
	if facts.ClientVersion != "9.9.9-test" {
		t.Errorf("ClientVersion is %q", facts.ClientVersion)
	}
	if facts.Hostname != host || facts.Hostname == "" {
		t.Errorf("Hostname is %q, want %q", facts.Hostname, host)
	}
	if facts.Arch != runtime.GOARCH {
		t.Errorf("Arch is %q", facts.Arch)
	}

	load, err := ReadLoad()
	if err != nil {
		t.Fatalf("ReadLoad: %v", err)
	}
	// The fixture holds a first load of 2.0 and a memory use of 75 percent.
	want := uint32(math.Min(100, math.Round(2.0/float64(runtime.NumCPU())*100)))
	if load.CPUPercent != want {
		t.Errorf("CPUPercent is %d, want %d", load.CPUPercent, want)
	}
	if load.MemoryPercent != 75 {
		t.Errorf("MemoryPercent is %d", load.MemoryPercent)
	}
}

func TestReadLoadFailsWithoutFiles(t *testing.T) {
	old := procDir
	procDir = t.TempDir()
	t.Cleanup(func() { procDir = old })
	if _, err := ReadLoad(); err == nil {
		t.Error("ReadLoad on an empty folder: want an error")
	}
	if _, err := ReadFacts("x"); err == nil {
		t.Error("ReadFacts on an empty folder: want an error")
	}
}
