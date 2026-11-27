package app

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/state"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

func TestRevisionTwoExecutableStop(t *testing.T) {
	binaryPath := filepath.Join(t.TempDir(), "lgtm-client")
	buildCtx, cancelBuild := context.WithTimeout(t.Context(), time.Minute)
	defer cancelBuild()
	build := exec.CommandContext(buildCtx, "go", "build", "-buildvcs=false", "-o", binaryPath, "../../cmd/lgtm-client")
	if build.Run() != nil {
		t.Fatal("could not build the client command")
	}

	for _, name := range []string{"missing history", "invalid counter", "invalid required event", "verified revision refusal"} {
		t.Run(name, func(t *testing.T) {
			const peerDetail = "private fixture transport detail"
			var opens atomic.Int32
			p := newAppPeer(t, nil, func(p *appPeer, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
				opens.Add(1)
				if name == "verified revision refusal" {
					if err := p.challenge(stream); err != nil {
						return err
					}
					frame, err := stream.Receive()
					if err != nil || frame.GetProof() == nil {
						return errors.New("the revision refusal fixture did not receive proof")
					}
					refusal := connect.NewError(connect.CodeFailedPrecondition, errors.New(peerDetail+" "+p.token))
					refusal.Meta().Set("Lgtm-Link-Refusal", "link_revision_mismatch")
					return refusal
				}
				if name == "invalid required event" {
					if err := p.welcome(stream, 0, false); err != nil {
						return err
					}
					for {
						frame, err := stream.Receive()
						if err != nil {
							return err
						}
						if frame.GetEvent() != nil || frame.GetCommandResult() != nil {
							t.Error("the client sent an event or command result for a damaged required history")
							break
						}
					}
				}
				return connect.NewError(connect.CodeUnavailable, errors.New(peerDetail+" "+p.token))
			})
			opts, _ := appOptions(t)
			seedApp(t, opts, p, false)
			paths := state.PathsIn(opts.StateDir)
			identity, key := appFile(t, paths.Identity), appFile(t, paths.Key)
			wantOpens := int32(0)
			var before []byte
			switch name {
			case "missing history":
				if os.Remove(paths.Store) != nil {
					t.Fatal("could not remove the fixture store")
				}
			case "invalid counter":
				editRecoveryProcessStore(t, paths.Store, func(tx *bolt.Tx) error {
					return tx.Bucket([]byte("meta")).Put([]byte("next"), binary.BigEndian.AppendUint64(nil, uint64(math.MaxInt64)+2))
				})
				before = appFile(t, paths.Store)
			case "invalid required event":
				folder := appFolder(t, opts.StateDir)
				store, err := link.OpenStore(folder, link.StoreOptions{})
				if err != nil {
					t.Fatal("could not open the process fixture store")
				}
				seq, err := store.Append([]byte{1, 0xff})
				if closeErr := store.Close(); seq != 1 || err != nil || closeErr != nil {
					t.Fatal("could not store the damaged required event")
				}
				closeAppFolder(t, folder)
				before = appFile(t, paths.Store)
				wantOpens = 1
			case "verified revision refusal":
				before = appFile(t, paths.Store)
				wantOpens = 1
			}

			stdout, stderr, status := runRecoveryProcess(t, binaryPath, opts)
			combined := stdout.String() + stderr.String()
			p.checkRedaction(t, combined, key)
			if status != 0 {
				t.Errorf("terminal history exited with status %d, want 0", status)
			}
			if stdout.Len() != 0 {
				t.Error("terminal history wrote to standard output")
			}
			if name == "verified revision refusal" {
				count := 0
				for _, line := range appLogs(t, stderr) {
					message := strings.ToLower(line.Message)
					if strings.Contains(message, "compatible") && strings.Contains(message, "client") &&
						(strings.Contains(message, "install") || strings.Contains(message, "upgrade")) &&
						(strings.Contains(message, "saved") || strings.Contains(message, "existing")) && strings.Contains(message, "state") {
						count++
					}
				}
				if count != 1 {
					t.Errorf("the process emitted %d instructions to install a compatible client with saved state, want one", count)
				}
			} else {
				requireAppRecoveryInstruction(t, stderr)
			}
			for _, detail := range []string{peerDetail, "no such file or directory", link.ErrStoreDamaged.Error(), link.ErrStoreMissing.Error()} {
				if strings.Contains(combined, detail) {
					t.Error("terminal history exposed raw error text")
				}
			}
			if got := opens.Load(); got != wantOpens {
				t.Errorf("terminal history opened %d streams, want %d", got, wantOpens)
			}
			if before == nil {
				noAppFile(t, paths.Store)
			} else if !bytes.Equal(before, appFile(t, paths.Store)) {
				t.Error("terminal history changed the disputed database bytes")
			}
			if !bytes.Equal(identity, appFile(t, paths.Identity)) || !bytes.Equal(key, appFile(t, paths.Key)) {
				t.Error("terminal history changed the saved enrollment")
			}
		})
	}

	t.Run("unreadable store", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root can read the fixture despite its file mode")
		}
		opts, _ := appOptions(t)
		p := newAppPeer(t, nil, nil)
		seedApp(t, opts, p, false)
		paths := state.PathsIn(opts.StateDir)
		before := appFile(t, paths.Store)
		if err := os.Chmod(paths.Store, 0); err != nil {
			t.Fatal("could not make the fixture store unreadable")
		}
		if _, err := os.ReadFile(paths.Store); !errors.Is(err, os.ErrPermission) {
			t.Fatalf("reading the fixture store returned %v, want a permission error", err)
		}

		stdout, stderr, status := runRecoveryProcess(t, binaryPath, opts)
		if status != 1 {
			t.Errorf("the unreadable store exited with status %d, want 1", status)
		}
		if stdout.Len() != 0 {
			t.Error("the failed store open wrote to standard output")
		}
		lines := appLogs(t, stderr)
		for _, line := range lines {
			message := strings.ToLower(line.Message)
			if strings.Contains(message, "restor") || strings.Contains(message, "recover") || strings.Contains(message, "enroll") {
				t.Error("the process gave a history recovery instruction for the unreadable store")
			}
		}
		failure := oneAppLog(t, lines, "link_stopped", "store_failed")
		if failure.Level != "ERROR" || failure.Message != "Could not use client.db." {
			t.Error("the process did not report a plain store failure")
		}
		if err := os.Chmod(paths.Store, 0o600); err != nil {
			t.Fatal("could not restore fixture access to check its bytes")
		}
		if !bytes.Equal(before, appFile(t, paths.Store)) {
			t.Error("the failed store open changed the saved database bytes")
		}
	})

	t.Run("requested restart", func(t *testing.T) {
		const commandID = "0195977c-0000-7000-8000-000000000010"
		const outcome = linkv1.CommandOutcome_COMMAND_OUTCOME_DONE
		var opens atomic.Int32
		results := make(chan *linkv1.CommandResult, 1)
		peerDone := make(chan error, 1)
		p := newAppPeer(t, nil, func(p *appPeer, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) (resultErr error) {
			if opens.Add(1) != 1 {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("unexpected restart reconnect"))
			}
			defer func() { peerDone <- resultErr }()
			if err := p.welcome(stream, 0, true); err != nil {
				return err
			}
			for number := uint64(1); number <= 2; {
				frame, err := stream.Receive()
				if err != nil {
					return err
				}
				if frame.GetHeartbeat() != nil {
					continue
				}
				if frame.GetEvent().GetNumber() != number {
					return errors.New("unexpected startup report number")
				}
				err = stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_EventAck{
					EventAck: &linkv1.EventAck{Through: number},
				}})
				if err != nil {
					return err
				}
				number++
			}
			cmd := &linkv1.Command{Id: commandID, Attempt: 1, ExpiresAt: timestamppb.New(time.Now().Add(time.Minute)),
				Action: &linkv1.Command_Restart{Restart: &linkv1.Restart{}},
			}
			if err := stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Command{Command: cmd}}); err != nil {
				return err
			}
			for {
				frame, err := stream.Receive()
				if errors.Is(err, io.EOF) {
					return nil
				}
				if err != nil {
					return err
				}
				if result := frame.GetCommandResult(); result != nil {
					select {
					case results <- result:
					default:
						return errors.New("the client sent duplicate restart results")
					}
				}
			}
		})
		opts, _ := appOptions(t)
		seedApp(t, opts, p, false)
		paths := state.PathsIn(opts.StateDir)

		stdout, stderr, status := runRecoveryProcess(t, binaryPath, opts)
		p.checkRedaction(t, stdout.String()+stderr.String(), appFile(t, paths.Key))
		if status != 75 {
			t.Errorf("the accepted restart exited with status %d, want 75", status)
		}
		if opens.Load() != 1 {
			t.Error("the requested restart did not use exactly one stream")
		}
		select {
		case err := <-peerDone:
			if err != nil {
				t.Error("the signed peer did not observe an orderly restart")
			}
		case <-appContext(t).Done():
			t.Error("the signed peer did not finish observing the restart")
		}
		select {
		case result := <-results:
			if result.GetCommandId() != commandID || result.GetAttempt() != 1 ||
				result.GetOutcome() != outcome || result.GetLastEventNumber() != 2 {
				t.Error("the restart result did not have the expected outcome and watermark")
			}
		default:
			t.Error("the client exited without handing off its restart result")
		}

		db, err := bolt.Open(paths.Store, 0o600, &bolt.Options{ReadOnly: true, Timeout: time.Second})
		if err != nil {
			t.Fatal("could not inspect the restarted process's closed database")
		}
		defer db.Close()
		err = db.View(func(tx *bolt.Tx) error {
			var receipt struct {
				Version         int     `json:"version"`
				State           string  `json:"state"`
				Result          []byte  `json:"result"`
				LastEventNumber *uint64 `json:"last_event_number"`
			}
			saved := tx.Bucket([]byte("receipts")).Get([]byte(commandID))
			if json.Unmarshal(saved, &receipt) != nil || receipt.Version != 2 || receipt.State != "finished" ||
				!bytes.Equal(receipt.Result, []byte{byte(outcome)}) || receipt.LastEventNumber == nil || *receipt.LastEventNumber != 2 {
				return errors.New("the restart result did not survive in its finished receipt")
			}
			return nil
		})
		if err != nil {
			t.Error("the restart result did not survive in its finished receipt")
		}
	})

	t.Run("cleanup error preserves recovery status", func(t *testing.T) {
		opts, log := appOptions(t)
		p := newAppPeer(t, nil, nil)
		seedApp(t, opts, p, false)
		var folder *state.Folder
		useAppLink(t, func(_ context.Context, opts link.RunOptions) (link.RunOutcome, error) {
			folder = opts.Folder
			_, release, err := folder.Claim(state.StoreFile)
			if err != nil {
				t.Fatal("could not hold a fixture claim through application cleanup")
			}
			t.Cleanup(func() {
				release()
				closeAppFolder(t, folder)
			})
			opts.Logger.Error("link_stopped", "message", "Restore the original client.db history or remove this machine and enroll again with fresh state.")
			return link.RunRecoveryStop, nil
		})
		if status := Run(appContext(t), opts); status != 0 {
			t.Errorf("cleanup after recovery returned status %d, want 0", status)
		}
		requireAppRecoveryInstruction(t, log)
	})
}

func runRecoveryProcess(t *testing.T, binaryPath string, opts Options) (*bytes.Buffer, *bytes.Buffer, int) {
	t.Helper()
	ctx := appContext(t)
	cmd := exec.CommandContext(ctx, binaryPath, "run")
	home := opts.Getenv("HOME")
	cmd.Env = []string{"HOME=" + home, "PATH=" + filepath.Join(home, "bin"), "LGTM_CLIENT_STATE_DIR=" + opts.StateDir}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Error("the client did not exit before the fixture deadline")
	}
	status := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal("could not launch or wait for the client command")
		}
		status = exit.ExitCode()
	}
	return &stdout, &stderr, status
}

func editRecoveryProcessStore(t *testing.T, path string, update func(*bolt.Tx) error) {
	t.Helper()
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		t.Fatal("could not open the process fixture database")
	}
	err = db.Update(update)
	if closeErr := db.Close(); err != nil || closeErr != nil {
		t.Fatal("could not edit the process fixture database")
	}
}

func requireAppRecoveryInstruction(t *testing.T, log *bytes.Buffer) {
	t.Helper()
	count := 0
	for _, line := range appLogs(t, log) {
		if appRecoveryInstruction(line.Message) {
			count++
		}
	}
	if count != 1 {
		t.Errorf("the process emitted %d actionable recovery instructions, want one", count)
	}
}

func appRecoveryInstruction(text string) bool {
	text = strings.ToLower(text)
	return strings.Contains(text, "client.db") &&
		(strings.Contains(text, "restor") || strings.Contains(text, "recover")) &&
		(strings.Contains(text, "histor") || strings.Contains(text, "original") || strings.Contains(text, "backup")) &&
		strings.Contains(text, "remov") && strings.Contains(text, "enroll") &&
		(strings.Contains(text, "fresh") || strings.Contains(text, "new state") || strings.Contains(text, "new folder"))
}
