package sysinfo

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// parseMeminfo reads MemTotal and MemAvailable in bytes from /proc/meminfo.
func parseMeminfo(text string) (total, available uint64, err error) {
	var hasTotal, hasAvailable bool
	for line := range strings.Lines(text) {
		name, value, _ := strings.Cut(line, ":")
		if name != "MemTotal" && name != "MemAvailable" {
			continue
		}
		fields := strings.Fields(value)
		if len(fields) != 2 || fields[1] != "kB" {
			return 0, 0, fmt.Errorf("invalid %s in meminfo", name)
		}
		size, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil || size > math.MaxUint64/1024 {
			return 0, 0, fmt.Errorf("invalid %s in meminfo", name)
		}
		if name == "MemTotal" {
			total, hasTotal = size*1024, true
		} else {
			available, hasAvailable = size*1024, true
		}
	}
	if !hasTotal || !hasAvailable {
		return 0, 0, errors.New("meminfo must contain MemTotal and MemAvailable")
	}
	return total, available, nil
}

// parseLoadavg reads the first field of /proc/loadavg.
func parseLoadavg(text string) (float64, error) {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return 0, errors.New("loadavg is empty")
	}
	load, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || math.IsNaN(load) || math.IsInf(load, 0) || load < 0 {
		return 0, errors.New("loadavg has an invalid one-minute load")
	}
	return load, nil
}

// decodeDarwinLoadavg reads the one-minute load and its scale.
func decodeDarwinLoadavg(b []byte) (float64, error) {
	var data [24]byte
	if len(b) != len(data) && len(b) != len(data)-1 {
		return 0, errors.New("vm.loadavg has an invalid size")
	}
	// syscall.Sysctl strips a trailing zero byte.
	copy(data[:], b)
	scale := binary.LittleEndian.Uint64(data[16:])
	if scale == 0 {
		return 0, errors.New("vm.loadavg has a zero scale")
	}
	return float64(binary.LittleEndian.Uint32(data[:])) / float64(scale), nil
}

// decodeDarwinUint64 reads the memory size, restoring a stripped zero byte.
func decodeDarwinUint64(b []byte) (uint64, error) {
	var data [8]byte
	if len(b) != len(data) && len(b) != len(data)-1 {
		return 0, errors.New("hw.memsize has an invalid size")
	}
	copy(data[:], b)
	return binary.LittleEndian.Uint64(data[:]), nil
}

// cpuPercent turns the one-minute load into a percentage of the cores.
func cpuPercent(load1 float64, cores int) uint32 {
	if cores <= 0 || load1 <= 0 || math.IsNaN(load1) {
		return 0
	}
	return uint32(math.Min(100, math.Round(load1/float64(cores)*100)))
}

// memoryPercent reports the share of total memory in use.
func memoryPercent(total, available uint64) uint32 {
	if available >= total {
		return 0
	}
	return uint32(math.Round(float64(total-available) * 100 / float64(total)))
}
