package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestSprintFourStoreReadsRevisionOne(t *testing.T) {
	const (
		number    = uint64(258)
		commandID = "00000000-0000-4000-8000-000000000258"
		otherID   = "00000000-0000-4000-8000-000000000257"
		finished  = `{"version":1,"expires_at":"2026-11-27T12:00:00Z","state":"finished","result":"Ag=="}`
		started   = `{"version":1,"expires_at":"2026-11-27T12:00:00Z","state":"started"}`
	)
	marshal := func(message proto.Message) []byte {
		t.Helper()
		data, err := (proto.MarshalOptions{Deterministic: true}).Marshal(message)
		if err != nil {
			t.Fatal("Could not encode the synthetic report.")
		}
		return data
	}
	at := timestamppb.New(time.Date(2026, 11, 27, 11, 59, 45, 0, time.UTC))
	harnesses := &linkv1.HarnessesFound{Harnesses: []*linkv1.Harness{
		{Name: "codex", Version: proto.String("0.1.0")},
	}}
	wire := &linkv1.Event{
		Number:     number,
		HappenedAt: at,
		Report:     &linkv1.Event_HarnessesFound{HarnessesFound: harnesses},
	}
	// Field order on disk may differ from the normalized wire encoding.
	stored := protowire.AppendTag([]byte{1}, 11, protowire.BytesType)
	stored = protowire.AppendBytes(stored, marshal(harnesses))
	stored = protowire.AppendTag(stored, 2, protowire.BytesType)
	stored = protowire.AppendBytes(stored, marshal(at))
	otherReport := append([]byte{1}, marshal(&linkv1.Event{
		HappenedAt: at,
		Report:     &linkv1.Event_HarnessesFound{HarnessesFound: &linkv1.HarnessesFound{}},
	})...)
	normalized := proto.CloneOf(wire)
	normalized.Number = 0
	sum := sha256.Sum256(marshal(normalized))
	wantDigest := hex.EncodeToString(sum[:])
	if bytes.Equal(stored[1:], marshal(normalized)) {
		t.Fatal("The report fixture must use a different field order from the wire digest.")
	}

	type fixture struct {
		report  []byte
		receipt string
		version string
	}
	valid := fixture{report: stored, receipt: finished, version: "1"}
	open := func(t *testing.T, rows fixture) (string, *bolt.DB) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "client.db")
		db, err := bolt.Open(path, 0600, nil)
		if err != nil {
			t.Fatal("Could not create the synthetic store.")
		}
		t.Cleanup(func() { _ = db.Close() })
		if err := db.Update(func(tx *bolt.Tx) error {
			for _, entry := range []struct {
				bucket string
				key    []byte
				value  []byte
			}{
				{"meta", []byte("version"), []byte(rows.version)},
				{"meta", []byte("next"), binary.BigEndian.AppendUint64(nil, number+1)},
				{"events", binary.BigEndian.AppendUint64(nil, number-1), otherReport},
				{"events", binary.BigEndian.AppendUint64(nil, number), rows.report},
				{"receipts", []byte(otherID), []byte(`{"version":1,"expires_at":"2026-11-27T12:00:00Z","state":"finished","result":"AQ=="}`)},
				{"receipts", []byte(commandID), []byte(rows.receipt)},
			} {
				bucket, err := tx.CreateBucketIfNotExists([]byte(entry.bucket))
				if err != nil {
					return err
				}
				if len(entry.value) != 0 {
					if err := bucket.Put(entry.key, entry.value); err != nil {
						return err
					}
				}
			}
			return nil
		}); err != nil {
			t.Fatal("Could not seed the synthetic store.")
		}
		return path, db
	}
	closeStore := func(t *testing.T, db *bolt.DB) {
		t.Helper()
		if db.Close() != nil {
			t.Fatal("Could not release the fixture writer lock.")
		}
	}
	run := func(t *testing.T, path string) ([]byte, int) {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "--s4-store-observer-test",
			"--db", path, "--event-number", strconv.FormatUint(number, 10), "--command-id", commandID)
		var output bytes.Buffer
		cmd.Stdout = &output
		err := cmd.Run()
		if ctx.Err() != nil {
			t.Fatal("The reader did not finish within the lock-wait deadline.")
		}
		if err == nil {
			return output.Bytes(), 0
		}
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() < 0 {
			t.Fatal("The reader process did not exit normally.")
		}
		return output.Bytes(), exit.ExitCode()
	}
	unchanged := func(t *testing.T, path string) {
		t.Helper()
		before, err := os.ReadFile(path)
		info, statErr := os.Stat(path)
		if err != nil || statErr != nil {
			t.Fatal("Could not snapshot the synthetic store.")
		}
		t.Cleanup(func() {
			after, err := os.ReadFile(path)
			current, statErr := os.Stat(path)
			if err != nil || statErr != nil || !os.SameFile(info, current) ||
				info.Mode() != current.Mode() || !info.ModTime().Equal(current.ModTime()) ||
				!bytes.Equal(before, after) {
				t.Error("The observer changed the store.")
			}
		})
	}
	refuse := func(t *testing.T, path string) {
		t.Helper()
		data, status := run(t, path)
		if status == 0 || len(data) != 0 {
			t.Error("The reader must refuse without emitting a record.")
		}
	}

	t.Run("ClosedStore", func(t *testing.T) {
		path, db := open(t, valid)
		closeStore(t, db)
		if os.Chmod(path, 0400) != nil {
			t.Fatal("Could not make the closed fixture read-only.")
		}
		unchanged(t, path)
		data, status := run(t, path)
		decoder := json.NewDecoder(bytes.NewReader(data))
		var record struct {
			EventNumber uint64                `json:"event_number"`
			Digest      string                `json:"digest"`
			CommandID   string                `json:"command_id"`
			Outcome     linkv1.CommandOutcome `json:"outcome"`
		}
		decoder.DisallowUnknownFields()
		if status != 0 || decoder.Decode(&record) != nil || decoder.Decode(new(json.RawMessage)) != io.EOF {
			t.Fatal("The closed store must yield one revision 1 observation.")
		}
		if record.EventNumber != number || record.Digest != wantDigest || record.CommandID != commandID ||
			record.Outcome != linkv1.CommandOutcome_COMMAND_OUTCOME_FAILED {
			t.Error("The observation must match the requested event, normalized digest and saved outcome.")
		}
	})
	t.Run("MissingFile", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "client.db")
		refuse(t, path)
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Error("The reader created the missing database.")
		}
	})
	t.Run("DamagedFile", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "client.db")
		if os.WriteFile(path, []byte("truncated"), 0600) != nil {
			t.Fatal("Could not write the damaged fixture.")
		}
		unchanged(t, path)
		refuse(t, path)
	})
	t.Run("WriterLocked", func(t *testing.T) {
		path, db := open(t, valid)
		unchanged(t, path)
		refuse(t, path)
		if db.View(func(tx *bolt.Tx) error {
			if tx.Bucket([]byte("events")) == nil {
				return errors.New("events bucket missing")
			}
			return nil
		}) != nil {
			t.Error("The fixture writer lost access to its store.")
		}
	})
	for _, test := range []struct {
		name string
		rows fixture
	}{
		{"MissingEvent", fixture{nil, finished, "1"}},
		{"MissingReceipt", fixture{stored, "", "1"}},
		{"StartedReceipt", fixture{stored, started, "1"}},
		{"DamagedReceipt", fixture{stored, `{"version":1,"expires_at":"2026-11-27T12:00:00Z","state":"finished","result":"%%%"}`, "1"}},
		{"WrongReceiptVersion", fixture{stored, `{"version":2,"expires_at":"2026-11-27T12:00:00Z","state":"finished","result":"Ag=="}`, "1"}},
		{"WrongStoreVersion", fixture{stored, finished, "2"}},
		{"WrongReportVersion", fixture{append([]byte{2}, stored[1:]...), finished, "1"}},
		{"DamagedReport", fixture{[]byte{1, 0xff}, finished, "1"}},
		{"NumberInStoredReport", fixture{append([]byte{1}, marshal(wire)...), finished, "1"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			path, db := open(t, test.rows)
			closeStore(t, db)
			unchanged(t, path)
			refuse(t, path)
		})
	}
}

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--s4-store-observer-test" {
		os.Args = append(os.Args[:1], os.Args[2:]...)
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}
