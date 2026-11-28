package linktest

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	bolt "go.etcd.io/bbolt"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/qnlbnsl/lgtm-studio/src/client/clienttest"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

const (
	scanAction     = "scan_harnesses"
	restartAction  = "restart"
	stateAction    = "set_state"
	commandDone    = linkv1.CommandOutcome_COMMAND_OUTCOME_DONE
	commandFailed  = linkv1.CommandOutcome_COMMAND_OUTCOME_FAILED
	commandExpired = linkv1.CommandOutcome_COMMAND_OUTCOME_EXPIRED
)

type commandFixture struct {
	server   *servertest.Server
	page     lgtmv1connect.ClientServiceClient
	relay    *relay
	opts     clienttest.Options
	probe    *versionProbe
	db       *pgx.Conn
	id       string
	dropNext atomic.Bool
}

func newCommandFixture(t *testing.T, controls relayControls) *commandFixture {
	t.Helper()
	f := new(commandFixture)
	f.server = servertest.Start(t, servertest.Options{Password: fixturePassword})
	f.page = f.server.Clients(t)
	controls.dropResult = func(noticeResult) bool { return f.dropNext.Swap(false) }
	f.relay = startRelayWithControls(t, f.server.Link(), controls)
	f.opts = newClient(t, new(diagnostics))
	f.probe = installVersionProbe(t, &f.opts)
	f.id = join(t, f.page, f.opts, f.relay.address)
	f.db = openObserver(t, f.server)
	return f
}

func (f *commandFixture) startApproved(t *testing.T, exitCode int) (*runningClient, *relayLeg) {
	t.Helper()
	approve(t, f.page, f.id, "Command fixture")
	run := startClientExpecting(t, f.opts, exitCode)
	leg := receive(t, f.relay.legs, "the approved command stream")
	waitForInventory(t, f, "1.0.0")
	notice := waitForCommand(t, leg, stateAction, 0)
	waitForStoredResult(t, f.db, notice, "done")
	waitForAck(t, leg, mark(t, f.db, f.id))
	f.probe.requireRuns(t, 1)
	return run, leg
}

func (f *commandFixture) scan(ctx context.Context) error {
	_, err := f.page.ScanClientHarnesses(ctx, connect.NewRequest(&webv1.ScanClientHarnessesRequest{Id: f.id}))
	return err
}

func (f *commandFixture) restart(ctx context.Context) error {
	_, err := f.page.ReconnectClient(ctx, connect.NewRequest(&webv1.ReconnectClientRequest{Id: f.id}))
	return err
}

func requestScan(t *testing.T, f *commandFixture) {
	t.Helper()
	if f.scan(t.Context()) != nil {
		t.Fatal("The signed-in scan request failed.")
	}
}

func requestRestart(t *testing.T, f *commandFixture) {
	t.Helper()
	if f.restart(t.Context()) != nil {
		t.Fatal("The signed-in restart request failed.")
	}
}

func waitForInventory(t *testing.T, f *commandFixture, version string) {
	t.Helper()
	waitFor(t, 25*time.Second, "online presence with the fixture inventory", func() bool {
		rows, _ := snapshot(t.Context(), f.page)
		for _, row := range rows {
			if row.GetId() == f.id && row.GetState() == webv1.ClientState_CLIENT_STATE_ONLINE && row.GetHarnessesScanned() {
				return slices.ContainsFunc(row.GetHarnesses(), func(item *webv1.InstalledHarness) bool {
					return item.GetHarness() == "claude" && item.GetVersion() == version
				})
			}
		}
		return false
	})
}

// A held executable has a three-second budget. Observe the committed report
// directly while it is held so a website watch handshake cannot use that budget.
func waitForCommittedInventory(t *testing.T, f *commandFixture, version string) {
	t.Helper()
	waitFor(t, 2*time.Second, "the newer connection to commit its own inventory", func() bool {
		saved := scalar[[]byte](t, f.db, `
SELECT coalesce((
    SELECT m.harnesses FROM machines m JOIN link_connections l ON m.id=l.machine_id
    WHERE m.id=$1 AND l.harnesses_received
), ''::bytea)`, f.id)
		var inventory linkv1.HarnessesFound
		if proto.Unmarshal(saved, &inventory) != nil {
			t.Fatal("Could not decode the committed inventory.")
		}
		return slices.ContainsFunc(inventory.GetHarnesses(), func(item *linkv1.Harness) bool {
			return item.GetName() == "claude" && item.GetVersion() == version
		})
	})
}

func waitForCommand(t *testing.T, leg *relayLeg, action string, ordinal int) commandDelivery {
	t.Helper()
	var found commandDelivery
	waitFor(t, 12*time.Second, "a "+action+" command on the expected stream", func() bool {
		n := 0
		for _, command := range leg.observe().commands {
			if command.kind == action {
				if n == ordinal {
					found = command
					return true
				}
				n++
			}
		}
		return false
	})
	return found
}

func waitForResult(t *testing.T, leg *relayLeg, delivery commandDelivery, outcome linkv1.CommandOutcome) {
	t.Helper()
	var result noticeResult
	waitFor(t, 12*time.Second, "the command result on the expected stream and attempt", func() bool {
		for _, observed := range leg.observe().results {
			if observed.id == delivery.id && observed.attempt == delivery.attempt {
				result = observed
				return true
			}
		}
		return false
	})
	if result.outcome != outcome {
		t.Fatalf("The command result was %s, want %s.", result.outcome, outcome)
	}
}

func waitForStoredResult(t *testing.T, db *pgx.Conn, delivery commandDelivery, outcome string) {
	t.Helper()
	waitFor(t, 10*time.Second, "the durable "+outcome+" result for the delivered attempt", func() bool {
		return scalar[bool](t, db, `
SELECT count(*) = 1 FROM link_commands
WHERE id=$1 AND attempt=$2 AND outcome=$3 AND dispatched
    AND finished_at IS NOT NULL AND payload IS NULL`, delivery.id, delivery.attempt, outcome)
	})
}

func requirePending(t *testing.T, db *pgx.Conn, delivery commandDelivery) {
	t.Helper()
	if !scalar[bool](t, db, `
SELECT count(*) = 1 FROM link_commands
WHERE id=$1 AND attempt=$2 AND dispatched AND outcome IS NULL
    AND finished_at IS NULL AND payload IS NOT NULL`, delivery.id, delivery.attempt) {
		t.Fatal("The delivered attempt did not remain pending in the database.")
	}
}

func requireRetry(t *testing.T, first, retry commandDelivery) {
	t.Helper()
	if first.id != retry.id || first.attempt != 1 || retry.attempt != first.attempt+1 ||
		!first.expiry.Equal(retry.expiry) {
		t.Fatal("The retry did not keep the command identifier and expiry while incrementing its attempt.")
	}
}

func generation(t *testing.T, f *commandFixture) uint64 {
	t.Helper()
	return scalar[uint64](t, f.db, "SELECT stream_generation FROM link_machines WHERE machine_id=$1", f.id)
}

func waitForWelcome(t *testing.T, leg *relayLeg) legObservation {
	t.Helper()
	var trace legObservation
	waitFor(t, 12*time.Second, "Welcome on the expected stream", func() bool {
		trace = leg.observe()
		return trace.welcomed
	})
	return trace
}

func injectCommand(t *testing.T, leg *relayLeg, command *linkv1.Command) commandDelivery {
	t.Helper()
	select {
	case leg.inject <- command:
	case <-leg.done:
		t.Fatal("The stream closed before the relay could inject the command.")
	case <-time.After(time.Second):
		t.Fatal("The relay did not accept the injected command.")
	}
	return commandDelivery{id: command.GetId(), attempt: command.GetAttempt()}
}

// A reply to the same standing proves processing has reached this point on
// the stream. The relay keeps this fixture command and its result off the server.
func waitForClientToProcessReplies(t *testing.T, leg *relayLeg) {
	t.Helper()
	welcome := waitForWelcome(t, leg)
	barrier := injectCommand(t, leg, &linkv1.Command{
		Id: "0195977c-0000-7000-8000-000000000110", Attempt: 1,
		ExpiresAt: timestamppb.New(time.Now().Add(time.Minute)),
		Action: &linkv1.Command_SetState{SetState: &linkv1.SetState{State: &linkv1.MachineState{
			Approval: welcome.approval, Revision: welcome.revision, Name: welcome.name,
		}}},
	})
	waitForResult(t, leg, barrier, commandDone)
}

type versionProbe struct {
	home string
}

func installVersionProbe(t *testing.T, opts *clienttest.Options) *versionProbe {
	t.Helper()
	home := opts.Getenv("HOME")
	bin := filepath.Join(home, "bin")
	if os.Mkdir(bin, 0o700) != nil {
		t.Fatal("Could not create the fixture executable directory.")
	}
	const program = `#!/bin/sh
set -eu
if /bin/mv "$HOME/linktest-arm" "$HOME/linktest-held" 2>/dev/null; then
    printf 'held %s\n' "$$" >> "$HOME/linktest-probes"
    IFS= read -r version < "$HOME/linktest-release"
else
    IFS= read -r version < "$HOME/linktest-version"
    printf 'version %s\n' "$version" >> "$HOME/linktest-probes"
fi
printf '%s (Claude Code)\n' "$version"
`
	if os.WriteFile(filepath.Join(bin, "claude"), []byte(program), 0o700) != nil ||
		unix.Mkfifo(filepath.Join(home, "linktest-release"), 0o600) != nil {
		t.Fatal("Could not install the disposable harness version probe.")
	}
	old := opts.Getenv
	opts.Getenv = func(name string) string {
		if name == "PATH" {
			return bin
		}
		return old(name)
	}
	probe := &versionProbe{home: home}
	probe.setVersion(t, "1.0.0")
	return probe
}

func (p *versionProbe) setVersion(t *testing.T, version string) {
	t.Helper()
	path := filepath.Join(p.home, "linktest-version")
	if os.WriteFile(path+".next", []byte(version+"\n"), 0o600) != nil || os.Rename(path+".next", path) != nil {
		t.Fatal("Could not set the fixture harness version.")
	}
}

func (p *versionProbe) arm(t *testing.T) {
	t.Helper()
	if os.WriteFile(filepath.Join(p.home, "linktest-arm"), nil, 0o600) != nil {
		t.Fatal("Could not arm the next harness version probe.")
	}
}

func (p *versionProbe) entries(t *testing.T) []string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(p.home, "linktest-probes"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Could not read the fixture probe ledger.")
	}
	if len(body) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(body)), "\n")
}

func (p *versionProbe) heldPID(t *testing.T) int {
	t.Helper()
	var pid int
	waitFor(t, 3*time.Second, "the requested harness version probe to block", func() bool {
		for _, entry := range p.entries(t) {
			if raw, ok := strings.CutPrefix(entry, "held "); ok {
				pid, _ = strconv.Atoi(raw)
				return pid > 0
			}
		}
		return false
	})
	return pid
}

func (p *versionProbe) release(t *testing.T, version string) {
	t.Helper()
	var fd int
	path := filepath.Join(p.home, "linktest-release")
	waitFor(t, time.Second, "the held version probe to open its release pipe", func() bool {
		var err error
		fd, err = unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if err != nil && !errors.Is(err, unix.ENXIO) {
			t.Fatal("Could not open the version probe release pipe.")
		}
		return err == nil
	})
	defer unix.Close(fd)
	if _, err := unix.Write(fd, []byte(version+"\n")); err != nil {
		t.Fatal("Could not release the version probe.")
	}
}

func (p *versionProbe) requireRuns(t *testing.T, want int) {
	t.Helper()
	if got := len(p.entries(t)); got != want {
		t.Fatalf("The version probe ran %d times, want %d.", got, want)
	}
}

type savedCommandReceipt struct {
	Version         int       `json:"version"`
	ExpiresAt       time.Time `json:"expires_at"`
	State           string    `json:"state"`
	Standing        *bool     `json:"standing"`
	LastEventNumber *uint64   `json:"last_event_number"`
	Result          []byte    `json:"result"`
}

func withClosedClientDB(t *testing.T, opts clienttest.Options, writable bool, inspect func(*bolt.DB) error) {
	t.Helper()
	db, err := bolt.Open(filepath.Join(opts.StateDir, "client.db"), 0o600, &bolt.Options{
		ReadOnly: !writable, Timeout: time.Second,
	})
	if err != nil {
		t.Fatal("Could not open the stopped client's command receipts.")
	}
	defer db.Close()
	if inspect(db) != nil {
		t.Fatal("Could not inspect or prepare the command receipt in the stopped client database.")
	}
}

func commandReceipt(tx *bolt.Tx, id string) (savedCommandReceipt, error) {
	var receipt savedCommandReceipt
	bucket := tx.Bucket([]byte("receipts"))
	if bucket == nil {
		return receipt, errors.New("the receipts bucket is missing")
	}
	err := json.Unmarshal(bucket.Get([]byte(id)), &receipt)
	return receipt, err
}

func requireReceipt(t *testing.T, opts clienttest.Options, delivery commandDelivery, outcome linkv1.CommandOutcome) {
	t.Helper()
	withClosedClientDB(t, opts, false, func(db *bolt.DB) error {
		return db.View(func(tx *bolt.Tx) error {
			receipt, err := commandReceipt(tx, delivery.id)
			if err != nil || receipt.Version != 2 || receipt.State != "finished" ||
				receipt.Standing == nil || *receipt.Standing || receipt.LastEventNumber == nil ||
				!receipt.ExpiresAt.Equal(delivery.expiry) || !slices.Equal(receipt.Result, []byte{byte(outcome)}) {
				return errors.New("the finished command receipt differs")
			}
			return nil
		})
	})
}

// observeClientTransactions reads a disposable copy so it does not take the
// live client's database lock. Callers drain the event queue before comparing
// transactions while the requested scan is held.
func observeClientTransactions(t *testing.T, opts clienttest.Options) func() (int, int) {
	t.Helper()
	source := filepath.Join(opts.StateDir, "client.db")
	copyPath := filepath.Join(t.TempDir(), "client.db")
	return func() (int, int) {
		t.Helper()
		data, err := os.ReadFile(source)
		if err != nil || os.WriteFile(copyPath, data, 0o600) != nil {
			t.Fatal("Could not copy the fixture's client database for inspection.")
		}
		db, err := bolt.Open(copyPath, 0o600, &bolt.Options{ReadOnly: true, Timeout: time.Second})
		if err != nil {
			t.Fatal("Could not open the copied client database.")
		}
		defer db.Close()
		var revision, queued int
		if db.View(func(tx *bolt.Tx) error {
			events := tx.Bucket([]byte("events"))
			if events == nil {
				return errors.New("the events bucket is missing")
			}
			revision, queued = tx.ID(), events.Stats().KeyN
			return nil
		}) != nil {
			t.Fatal("Could not inspect the transaction and outbox of the copied client database.")
		}
		return revision, queued
	}
}
