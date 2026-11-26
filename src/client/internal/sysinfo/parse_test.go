package sysinfo

import (
	"encoding/binary"
	"os"
	"testing"
)

func TestParseLinuxFiles(t *testing.T) {
	total, avail, err := parseMeminfo("MemTotal:       16384000 kB\nMemFree: 1 kB\nMemAvailable:    4096000 kB\n")
	if err != nil {
		t.Fatalf("parseMeminfo: %v", err)
	}
	if total != 16_777_216_000 || avail != 4_194_304_000 {
		t.Fatalf("got total %d and available %d", total, avail)
	}

	fixture, err := os.ReadFile("testdata/meminfo")
	if err != nil {
		t.Fatal(err)
	}
	total, avail, err = parseMeminfo(string(fixture))
	if err != nil || total != 16_777_216_000 || avail != 4_194_304_000 {
		t.Fatalf("fixture gave %d, %d, %v", total, avail, err)
	}

	for name, text := range map[string]string{
		"no available": "MemTotal:       16384000 kB\nMemFree: 1 kB\n",
		"no total":     "MemAvailable:    4096000 kB\n",
		"empty":        "",
	} {
		if _, _, err := parseMeminfo(text); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}

	load, err := parseLoadavg("0.52 0.58 0.59 1/1234 5678\n")
	if err != nil || load != 0.52 {
		t.Fatalf("parseLoadavg gave %v, %v", load, err)
	}
	if _, err := parseLoadavg(""); err == nil {
		t.Error("empty loadavg: want an error")
	}
	if _, err := parseLoadavg("abc 1 2\n"); err == nil {
		t.Error("loadavg that is not a number: want an error")
	}
}

func darwinLoadavg(a, b, c uint32, scale uint64) []byte {
	buf := make([]byte, 24)
	binary.LittleEndian.PutUint32(buf[0:], a)
	binary.LittleEndian.PutUint32(buf[4:], b)
	binary.LittleEndian.PutUint32(buf[8:], c)
	binary.LittleEndian.PutUint64(buf[16:], scale)
	return buf
}

func TestDecodeDarwinValues(t *testing.T) {
	full := darwinLoadavg(3072, 1024, 512, 2048)
	got, err := decodeDarwinLoadavg(full)
	if err != nil || got != 1.5 {
		t.Fatalf("full value gave %v, %v", got, err)
	}
	// The system call drops one trailing zero byte, and the scale ends in zero bytes.
	got, err = decodeDarwinLoadavg(full[:23])
	if err != nil || got != 1.5 {
		t.Fatalf("value without the last zero byte gave %v, %v", got, err)
	}
	if _, err := decodeDarwinLoadavg(darwinLoadavg(3072, 1024, 512, 0)); err == nil {
		t.Error("scale 0: want an error")
	}
	if _, err := decodeDarwinLoadavg(make([]byte, 10)); err == nil {
		t.Error("10 bytes: want an error")
	}

	mem := make([]byte, 8)
	binary.LittleEndian.PutUint64(mem, 17_179_869_184)
	n, err := decodeDarwinUint64(mem)
	if err != nil || n != 17_179_869_184 {
		t.Fatalf("hw.memsize gave %d, %v", n, err)
	}
	n, err = decodeDarwinUint64(mem[:7])
	if err != nil || n != 17_179_869_184 {
		t.Fatalf("hw.memsize without the last zero byte gave %d, %v", n, err)
	}
	if _, err := decodeDarwinUint64([]byte{1, 2, 3}); err == nil {
		t.Error("3 bytes: want an error")
	}
}

func TestPercentages(t *testing.T) {
	cases := []struct {
		name string
		got  uint32
		want uint32
	}{
		{"load 2.0 on 4 cores", cpuPercent(2.0, 4), 50},
		{"load 9.0 on 4 cores is capped", cpuPercent(9.0, 4), 100},
		{"0 cores", cpuPercent(2.0, 0), 0},
		{"load 0.126 on 1 core rounds to 13", cpuPercent(0.126, 1), 13},
		{"load 0.124 on 1 core rounds to 12", cpuPercent(0.124, 1), 12},
		{"negative load", cpuPercent(-1, 4), 0},
		{"memory 25 of 100 available", memoryPercent(100, 25), 75},
		{"memory total 0", memoryPercent(0, 0), 0},
		{"memory rounds up", memoryPercent(1000, 335), 67},
		{"memory available above total", memoryPercent(100, 200), 0},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, c.got, c.want)
		}
	}
}
