package link

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
	"unicode/utf8"

	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/config"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/harness"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/sysinfo"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

type reportLog struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (l *reportLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buffer.Write(p)
}

func (l *reportLog) logger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(l, nil))
}

func (l *reportLog) sequences(t *testing.T, want ...uint64) {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	decoder := json.NewDecoder(bytes.NewReader(l.buffer.Bytes()))
	var got []uint64
	for decoder.More() {
		var entry struct {
			Msg      string `json:"msg"`
			Sequence uint64 `json:"sequence"`
		}
		if err := decoder.Decode(&entry); err != nil {
			t.Fatalf("invalid report log: %v", err)
		}
		if entry.Msg == "link_event_buffered" {
			got = append(got, entry.Sequence)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("buffered log sequences = %v, want %v", got, want)
	}
}

func fixtureReportProbes() reportProbes {
	return reportProbes{
		now: func() time.Time { return tNow },
		readFacts: func(version string) (sysinfo.Facts, error) {
			return sysinfo.Facts{OS: "Linux", Cores: 7, MemoryBytes: 9<<30 + 17, ClientVersion: version}, nil
		},
		readLoad: func() (sysinfo.Load, error) { return sysinfo.Load{}, nil },
		scan: func(context.Context, harness.ScanOptions) []harness.Found {
			defs := harness.Definitions()
			found := make([]harness.Found, len(defs))
			for i, def := range defs {
				found[i].Harness = def.Name
			}
			return found
		},
	}
}

func readReport(t *testing.T, store *Store, n uint64) (*linkv1.Event, []byte) {
	t.Helper()
	events, err := store.Read(n, 1)
	if err != nil || len(events) != 1 || events[0].Seq != n {
		t.Fatalf("reading report %d: count %d, error %v", n, len(events), err)
	}
	payload := events[0].Payload
	if len(payload) < 2 || payload[0] != 1 {
		t.Fatalf("report %d has no version 1 payload", n)
	}
	event := new(linkv1.Event)
	if err := proto.Unmarshal(payload[1:], event); err != nil {
		t.Fatalf("decoding report %d: %v", n, err)
	}
	if event.GetNumber() != 0 || !event.GetHappenedAt().IsValid() {
		t.Fatalf("stored report %d has a wire number or invalid capture time", n)
	}
	return event, payload
}

func checkReportString(t *testing.T, field, value string, limit int) {
	t.Helper()
	if !utf8.ValidString(value) || strings.ContainsRune(value, 0) || len(value) > limit {
		t.Errorf("encoded %s does not meet its string rules", field)
	}
}

func TestReportInventoryAndLoad(t *testing.T) {
	t.Run("facts, inventory and load", func(t *testing.T) {
		folder, _ := newState(t)
		store := mustCreate(t, folder, StoreOptions{})
		probes := fixtureReportProbes()
		home := t.TempDir()
		now := tNow
		probes.now = func() time.Time { return now }
		probes.readFacts = func(version string) (sysinfo.Facts, error) {
			return sysinfo.Facts{OS: "Linux", Cores: 7, MemoryBytes: 9<<30 + 17, ClientVersion: version}, nil
		}
		versions := map[harness.Name]string{
			harness.Claude: "1.2.3",
			harness.Codex:  "0.98.0",
			harness.Gemini: "0.21.4",
			harness.Cursor: "2026.11.26-a1b2c3",
		}
		probes.scan = func(_ context.Context, opts harness.ScanOptions) []harness.Found {
			if opts.Home != home {
				t.Error("the scan did not receive its configured home")
			}
			found := fixtureReportProbes().scan(context.Background(), opts)
			for i := range found {
				found[i].Installed, found[i].Version = true, versions[found[i].Harness]
				found[i].Path = "/fixture/private-executable"
				if i != 0 {
					found[i].Problem = "fixture private diagnostic"
				}
			}
			return found
		}
		type sample struct {
			name       string
			load       sysinfo.Load
			err        error
			approved   bool
			wantCPU    *uint32
			wantMemory *uint32
		}
		samples := []sample{
			{name: "zero", approved: true, wantCPU: proto.Uint32(0), wantMemory: proto.Uint32(0)},
			{name: "measured", approved: true, load: sysinfo.Load{CPUPercent: 19, MemoryPercent: 83}, wantCPU: proto.Uint32(19), wantMemory: proto.Uint32(83)},
			{name: "failed", approved: true, load: sysinfo.Load{CPUPercent: 19, MemoryPercent: 83}, err: errors.New("fixture probe failed")},
			{name: "waiting", load: sysinfo.Load{CPUPercent: 19, MemoryPercent: 83}},
		}
		var load sample
		probes.readLoad = func() (sysinfo.Load, error) { return load.load, load.err }
		version := "v" + strings.Repeat("界", 22)
		log := new(reportLog)
		reporter := newReporter(config.Settings{RepositoriesFolder: filepath.Join(home, "repos"), AgentLimit: 8},
			version, harness.ScanOptions{Home: home}, store.Append, log.logger(), probes)
		if err := reporter.captureFacts(); err != nil {
			t.Fatal(err)
		}
		now = now.Add(time.Second)
		if committed, err := reporter.scanHarnesses(t.Context(), false); err != nil || !committed {
			t.Errorf("inventory commit = %v, error = %v", committed, err)
		}
		for _, tc := range samples {
			t.Run(tc.name, func(t *testing.T) {
				load = tc
				got := reporter.heartbeat(tc.approved)
				if !proto.Equal(got, &linkv1.Heartbeat{CpuPercent: tc.wantCPU, MemoryPercent: tc.wantMemory}) {
					t.Errorf("heartbeat fields = %v, want CPU and memory presence and values from %s", got, tc.name)
				}
			})
		}
		if store.Next() != 3 || store.Len() != 2 {
			t.Fatalf("two reports and four heartbeats left length %d, next %d; want 2, 3", store.Len(), store.Next())
		}
		event, encoded := readReport(t, store, 1)
		facts := event.GetFacts()
		if !proto.Equal(event.GetHappenedAt(), timestamppb.New(tNow)) ||
			!proto.Equal(facts, &linkv1.Facts{Os: "Linux", Cores: 7, MemoryBytes: 9<<30 + 17,
				ClientVersion: "v" + strings.Repeat("界", 21), Workspace: "~/repos", AgentLimit: 8}) {
			t.Errorf("facts fields or capture time differ from the prepared snapshot")
		}
		checkReportString(t, "operating system", facts.GetOs(), 64)
		checkReportString(t, "client version", facts.GetClientVersion(), 64)
		checkReportString(t, "workspace", facts.GetWorkspace(), 256)
		if bytes.Contains(encoded, []byte(home)) {
			t.Error("facts contain the full home path")
		}

		event, encoded = readReport(t, store, 2)
		got := event.GetHarnessesFound().GetHarnesses()
		if !proto.Equal(event.GetHappenedAt(), timestamppb.New(now)) {
			t.Error("inventory did not retain its capture time")
		}
		defs := harness.Definitions()
		if len(got) != len(defs) || len(got) != 4 {
			t.Fatalf("inventory length = %d, want all four scan entries", len(got))
		}
		for i, row := range got {
			want := &linkv1.Harness{
				Name:    string(defs[i].Name),
				Version: proto.String(versions[defs[i].Name]),
			}
			if !proto.Equal(row, want) {
				t.Errorf("inventory entry %d has the wrong name or optional version for %s", i, want.Name)
			}
			checkReportString(t, "harness name", row.GetName(), 32)
			checkReportString(t, "harness version", row.GetVersion(), 64)
		}
		if bytes.Contains(encoded, []byte("fixture/private-executable")) ||
			bytes.Contains(encoded, []byte("fixture private diagnostic")) {
			t.Error("inventory contains local path or diagnostic data")
		}
		log.sequences(t, 1, 2)

		oversized := newReporter(config.Settings{RepositoriesFolder: filepath.Join(home, strings.Repeat("界", 90))},
			"v", harness.ScanOptions{Home: home}, store.Append, log.logger(), probes)
		if err := oversized.captureFacts(); err != nil {
			t.Fatal(err)
		}
		event, _ = readReport(t, store, 3)
		if event.GetFacts() == nil || event.GetFacts().GetWorkspace() != "" {
			t.Error("oversized abbreviated workspace was not omitted")
		}
	})
	t.Run("wire strings have no zero bytes", func(t *testing.T) {
		folder, _ := newState(t)
		store := mustCreate(t, folder, StoreOptions{})
		probes := fixtureReportProbes()
		readFacts := probes.readFacts
		probes.readFacts = func(version string) (sysinfo.Facts, error) {
			facts, err := readFacts(version)
			facts.OS = "Li\x00nux"
			return facts, err
		}
		scan := probes.scan
		probes.scan = func(ctx context.Context, opts harness.ScanOptions) []harness.Found {
			found := scan(ctx, opts)
			found[0].Harness += "\x00"
			found[0].Installed, found[0].Version = true, "1.\x002.3"
			found[2].Installed = true
			return found
		}
		home := t.TempDir()
		log := new(reportLog)
		r := newReporter(config.Settings{RepositoriesFolder: filepath.Join(home, "re\x00pos")},
			"fixture\x00version", harness.ScanOptions{Home: home}, store.Append, log.logger(), probes)
		if err := r.captureFacts(); err != nil {
			t.Fatal(err)
		}
		if committed, err := r.scanHarnesses(t.Context(), false); !committed || err != nil {
			t.Errorf("inventory with unprepared strings: committed %v, error %v", committed, err)
		}
		event, _ := readReport(t, store, 1)
		facts := event.GetFacts()
		if facts == nil {
			t.Fatal("report did not contain facts")
		}
		checkReportString(t, "operating system", facts.GetOs(), 64)
		checkReportString(t, "client version", facts.GetClientVersion(), 64)
		checkReportString(t, "workspace", facts.GetWorkspace(), 256)
		event, _ = readReport(t, store, 2)
		inventory := event.GetHarnessesFound().GetHarnesses()
		if len(inventory) != len(harness.Definitions()) {
			t.Fatal("preparing strings dropped a harness entry")
		}
		if row := inventory[2]; row.GetName() != string(harness.Gemini) || row.Version != nil {
			t.Error("installed Gemini has no measured version, so its version field must be absent")
		}
		for _, row := range inventory {
			checkReportString(t, "harness name", row.GetName(), 32)
			checkReportString(t, "harness version", row.GetVersion(), 64)
		}
	})
}

func TestReportLogFollowsDurableAppend(t *testing.T) {
	t.Run("log waits for append return", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			folder, _ := newState(t)
			store := mustCreate(t, folder, StoreOptions{})
			log := new(reportLog)
			committed := make(chan struct{})
			release := make(chan struct{})
			appendHeld := func(payload []byte) (uint64, error) {
				seq, err := store.Append(payload)
				close(committed)
				<-release
				return seq, err
			}
			r := newReporter(config.Settings{}, "v", harness.ScanOptions{}, appendHeld, log.logger(), fixtureReportProbes())
			finished := make(chan error, 1)
			go func() { finished <- r.captureFacts() }()
			synctest.Wait()
			select {
			case <-committed:
			default:
				close(release)
				t.Fatal("capture did not call the outbox")
			}
			func() {
				defer close(release)
				log.sequences(t)
				if store.Len() != 1 {
					t.Error("the held append did not commit before the log inspection")
				}
				select {
				case <-finished:
					t.Error("capture returned while append was still held")
				default:
				}
			}()
			if err := <-finished; err != nil {
				t.Fatal(err)
			}
			log.sequences(t, 1)
		})
	})
	t.Run("cancellation after commit preserves success", func(t *testing.T) {
		folder, _ := newState(t)
		store := mustCreate(t, folder, StoreOptions{})
		log := new(reportLog)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		r := newReporter(config.Settings{}, "v", harness.ScanOptions{}, func(payload []byte) (uint64, error) {
			seq, err := store.Append(payload)
			cancel()
			return seq, err
		}, log.logger(), fixtureReportProbes())
		if committed, err := r.scanHarnesses(ctx, true); !committed || err != nil {
			t.Fatalf("scan cancelled after commit returned %v, %v; want a successful commit", committed, err)
		}
		if event, _ := readReport(t, store, 1); event.GetHarnessesFound() == nil {
			t.Error("scan cancelled after commit did not leave its inventory on disk")
		}
		log.sequences(t, 1)
	})
}

func TestReportBackpressure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		folder, _ := newState(t)
		store := mustCreate(t, folder, StoreOptions{MaxEvents: 1})
		log := new(reportLog)
		probes := fixtureReportProbes()
		inventory := probes.scan
		started := make(chan struct{})
		release := make(chan struct{})
		var scanCalls int
		probes.scan = func(ctx context.Context, opts harness.ScanOptions) []harness.Found {
			scanCalls++
			if scanCalls == 1 {
				close(started)
				<-release
			}
			return inventory(ctx, opts)
		}
		now := tNow
		probes.now = func() time.Time { return now }
		r := newReporter(config.Settings{}, "v", harness.ScanOptions{}, store.Append, log.logger(), probes)
		if err := r.captureFacts(); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			now = now.Add(time.Second)
			if err := r.captureFacts(); err != nil {
				t.Fatal(err)
			}
		}
		newest := now
		if store.Next() != 2 || store.Len() != 1 {
			t.Fatalf("full outbox has length %d and next %d; want 1 and 2", store.Len(), store.Next())
		}
		log.sequences(t, 1)

		type result struct {
			committed bool
			err       error
		}
		done := make(chan result, 1)
		go func() {
			committed, err := r.scanHarnesses(t.Context(), true)
			done <- result{committed, err}
		}()
		unblock := sync.OnceFunc(func() { close(release) })
		defer unblock()
		synctest.Wait()
		select {
		case <-started:
		default:
			t.Fatal("requested scan did not start")
		}

		now = now.Add(time.Second)
		if err := store.Ack(1); err != nil {
			t.Fatal(err)
		}
		if err := r.flush(); err != nil {
			t.Fatal(err)
		}
		if event, _ := readReport(t, store, 2); event.GetFacts() == nil || !proto.Equal(event.GetHappenedAt(), timestamppb.New(newest)) {
			t.Error("flush did not append the newest refused facts snapshot")
		}
		if err := store.Ack(2); err != nil {
			t.Fatal(err)
		}
		if committed, err := r.scanHarnesses(t.Context(), false); err != nil || !committed {
			t.Fatalf("newer automatic inventory commit = %v, error = %v", committed, err)
		}
		if event, _ := readReport(t, store, 3); event.GetHarnessesFound() == nil {
			t.Fatal("newer automatic inventory is absent")
		}
		if err := store.Ack(3); err != nil {
			t.Fatal(err)
		}
		unblock()
		got := <-done
		if got.err != nil || got.committed {
			t.Errorf("late requested scan commit = %v, error = %v", got.committed, got.err)
		}
		if err := r.flush(); err != nil {
			t.Fatal(err)
		}
		if store.Next() != 4 || store.Len() != 0 {
			t.Error("discarded scan acquired a number or remained pending")
		}
		log.sequences(t, 1, 2, 3)
	})
}

func TestReportUncertainAppendIsNotRepeated(t *testing.T) {
	folder, _ := newState(t)
	store := mustCreate(t, folder, StoreOptions{})
	log := new(reportLog)
	var attempts int
	restore := swapCommit(func(db *bolt.DB, write func(*bolt.Tx) error) error {
		if err := db.Update(write); err != nil {
			return err
		}
		return errors.New("fixture error after durable write")
	})
	defer restore()
	r := newReporter(config.Settings{}, "v", harness.ScanOptions{}, func(payload []byte) (uint64, error) {
		attempts++
		return store.Append(payload)
	}, log.logger(), fixtureReportProbes())
	if err := r.captureFacts(); !errors.Is(err, ErrStoreFailed) {
		t.Errorf("uncertain append error = %v, want ErrStoreFailed", err)
	}
	_ = r.flush()
	_ = r.captureFacts()
	if attempts != 1 {
		t.Errorf("uncertain append led to %d append attempts, want 1", attempts)
	}
	log.sequences(t)
	restore()
	store = reopen(t, folder, store, StoreOptions{})
	if event, _ := readReport(t, store, 1); event.GetFacts() == nil || store.Next() != 2 || store.Len() != 1 {
		t.Fatal("uncertain but committed report did not survive exactly once at number 1")
	}
}
