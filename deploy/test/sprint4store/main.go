package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"regexp"
	"runtime/debug"
	"syscall"
	"time"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"
)

var (
	commandIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	errInvalidStore  = errors.New("Invalid revision 1 client store.")
)

type observation struct {
	EventNumber uint64                `json:"event_number"`
	Digest      string                `json:"digest"`
	CommandID   string                `json:"command_id"`
	Outcome     linkv1.CommandOutcome `json:"outcome"`
}

func main() {
	flags := flag.NewFlagSet("sprint4store", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	path := flags.String("db", "", "")
	number := flags.Uint64("event-number", 0, "")
	commandID := flags.String("command-id", "", "")
	if flags.Parse(os.Args[1:]) != nil || flags.NArg() != 0 || *path == "" ||
		*number == 0 || !commandIDPattern.MatchString(*commandID) {
		os.Exit(2)
	}
	record, err := observeStore(*path, *number, *commandID)
	if err != nil || json.NewEncoder(os.Stdout).Encode(record) != nil {
		os.Exit(1)
	}
}

func observeStore(path string, number uint64, commandID string) (record observation, err error) {
	// A damaged memory-mapped database can fault while bbolt reads its pages.
	var file *os.File
	previous := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(previous)
	defer func() {
		if recover() != nil {
			if file != nil {
				_ = file.Close()
			}
			err = errInvalidStore
		}
	}()

	db, err := bolt.Open(path, 0, &bolt.Options{
		ReadOnly: true,
		Timeout:  time.Second,
		OpenFile: func(path string, _ int, _ os.FileMode) (*os.File, error) {
			var openErr error
			file, openErr = os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
			if openErr != nil {
				return nil, openErr
			}
			info, statErr := file.Stat()
			if statErr != nil || !info.Mode().IsRegular() || info.Size() == 0 {
				_ = file.Close()
				return nil, errInvalidStore
			}
			return file, nil
		},
	})
	if err != nil {
		return observation{}, err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return observation{}, err
	}

	err = db.View(func(tx *bolt.Tx) error {
		if tx.Size() <= 0 || tx.Size() > info.Size() {
			return errInvalidStore
		}
		meta := tx.Bucket([]byte("meta"))
		events := tx.Bucket([]byte("events"))
		receipts := tx.Bucket([]byte("receipts"))
		if meta == nil || events == nil || receipts == nil || string(meta.Get([]byte("version"))) != "1" {
			return errInvalidStore
		}
		next := meta.Get([]byte("next"))
		if len(next) != 8 || number >= binary.BigEndian.Uint64(next) {
			return errInvalidStore
		}
		eventKey := binary.BigEndian.AppendUint64(nil, number)
		digest, err := reportDigest(events.Get(eventKey))
		if err != nil {
			return err
		}
		outcome, err := receiptOutcome(receipts.Get([]byte(commandID)))
		if err != nil {
			return err
		}
		record = observation{EventNumber: number, Digest: digest, CommandID: commandID, Outcome: outcome}
		return nil
	})
	return record, err
}

func reportDigest(data []byte) (string, error) {
	if len(data) == 0 || data[0] != 1 {
		return "", errInvalidStore
	}
	var event linkv1.Event
	if proto.Unmarshal(data[1:], &event) != nil || event.GetNumber() != 0 || !event.GetHappenedAt().IsValid() {
		return "", errInvalidStore
	}
	// Facts and harness inventory were the only reports in revision 1.
	if event.GetFacts() == nil && event.GetHarnessesFound() == nil {
		return "", errInvalidStore
	}
	normalized, err := (proto.MarshalOptions{Deterministic: true}).Marshal(&event)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(normalized)
	return hex.EncodeToString(digest[:]), nil
}

func receiptOutcome(data []byte) (linkv1.CommandOutcome, error) {
	var receipt struct {
		Version   int       `json:"version"`
		ExpiresAt time.Time `json:"expires_at"`
		State     string    `json:"state"`
		Result    string    `json:"result"`
	}
	if json.Unmarshal(data, &receipt) != nil || receipt.Version != 1 || receipt.ExpiresAt.IsZero() ||
		receipt.State != "finished" {
		return 0, errInvalidStore
	}
	result, err := base64.StdEncoding.DecodeString(receipt.Result)
	if err != nil || len(result) != 1 {
		return 0, errInvalidStore
	}
	outcome := linkv1.CommandOutcome(result[0])
	if outcome < linkv1.CommandOutcome_COMMAND_OUTCOME_DONE || outcome > linkv1.CommandOutcome_COMMAND_OUTCOME_INTERRUPTED {
		return 0, errInvalidStore
	}
	return outcome, nil
}
