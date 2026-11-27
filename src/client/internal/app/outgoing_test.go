package app

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/harness"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/state"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

const (
	outgoingSignIn      = "outgoing.initial.fixture"
	outgoingRefresh     = "outgoing.rotated"
	outgoingPeerToken   = "907.806.705"
	outgoingMask        = "[redacted]"
	outgoingScanID      = "0195977c-0000-7000-8000-000000000811"
	outgoingVersion     = "client-" + outgoingSignIn + "-" + outgoingPeerToken + "-end"
	outgoingVersionSafe = "client-[redacted]-[redacted]-end"
)

var outgoingServerTime = time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC)

func TestOutgoingFilterExecutable(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "lgtm-client")
	buildCtx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	build := exec.CommandContext(buildCtx, "go", "build", "-buildvcs=false", "-ldflags",
		"-X=main.version="+outgoingVersion, "-o", binary, "../../cmd/lgtm-client")
	if build.Run() != nil {
		t.Fatal("could not build the outgoing-filter client fixture")
	}

	t.Run("startup and rotation on the approved stream", func(t *testing.T) {
		opts, _ := appOptions(t)
		home := opts.Getenv("HOME")
		configDir := filepath.Join(home, "claude-config")
		credentials := filepath.Join(configDir, ".credentials.json")
		bin := filepath.Join(home, "bin")
		for _, dir := range []string{configDir, bin} {
			if os.Mkdir(dir, 0o700) != nil {
				t.Fatal("could not create a private filter fixture directory")
			}
		}
		if replaceOutgoingCredentials(credentials, outgoingSignIn, "outgoing-initial-refresh") != nil ||
			writeOutgoingProbe(bin, "claude", outgoingPeerToken+" (Claude Code)") != nil {
			t.Fatal("could not plant the initial credentials and harness version")
		}
		control := harness.Scan(appContext(t), harness.ScanOptions{SearchPath: []string{bin}, Home: home})
		if outgoingHarnessVersion(control) != outgoingPeerToken {
			t.Fatal("the unfiltered probe did not reach the real Claude Code version parser")
		}

		workspace := filepath.Join(home, "workspace-before-"+outgoingSignIn+"-after")
		stop := make(chan struct{})
		type trace struct {
			events []*linkv1.Event
			frames [][]byte
			result *linkv1.CommandResult
			err    error
		}
		traces := make(chan trace, 1)
		var opens atomic.Int32
		p := newAppPeer(t, nil, func(p *appPeer, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) (err error) {
			if opens.Add(1) != 1 {
				return errors.New("the filter fixture unexpectedly reconnected")
			}
			var seen trace
			done := sync.OnceFunc(func() {
				seen.err = err
				traces <- seen
				close(stop)
			})
			defer done()
			if err := welcomeOutgoingPeer(p, stream, true); err != nil {
				return err
			}
			scanRequested := false
			for len(seen.events) < 3 || seen.result == nil {
				frame, err := stream.Receive()
				if err != nil {
					return err
				}
				wire, err := proto.Marshal(frame)
				if err != nil {
					return errors.New("could not capture the report frame")
				}
				seen.frames = append(seen.frames, wire)
				if event := frame.GetEvent(); event != nil {
					seen.events = append(seen.events, event)
				}
				if result := frame.GetCommandResult(); result != nil {
					seen.result = result
				}
				if len(seen.events) == 2 && !scanRequested {
					if replaceOutgoingCredentials(credentials, outgoingRefresh, "outgoing-new-refresh") != nil ||
						writeOutgoingProbe(bin, "codex", "codex-cli 2.3.4-before."+outgoingRefresh+"."+outgoingSignIn+".after") != nil {
						return errors.New("could not replace the sign-in file and harness version")
					}
					if err := sendOutgoingScan(stream, outgoingScanID); err != nil {
						return err
					}
					scanRequested = true
				}
			}
			done()
			for {
				if _, err := stream.Receive(); err != nil {
					return err
				}
			}
		})
		seedApp(t, opts, p, false)
		paths := state.PathsIn(opts.StateDir)
		settings, err := json.Marshal(map[string]any{"version": 1, "repositories_folder": workspace, "agent_limit": 7})
		if err != nil || os.WriteFile(paths.Settings, settings, 0o600) != nil {
			t.Fatal("could not prepare the workspace settings")
		}
		stdout, stderr, status := runOutgoingProcess(t, binary, opts, configDir, stop, "run")
		key := appFile(t, paths.Key)
		checkOutgoingStreams(t, p, stdout, stderr, key, outgoingSignIn, outgoingRefresh, outgoingPeerToken)
		if status != 0 || stdout.Len() != 0 || opens.Load() != 1 {
			t.Error("the approved client did not run through the refresh on one stream and exit cleanly")
		}
		checkOutgoingStartup(t, stderr, filepath.Join(home, "workspace-before-"+outgoingMask+"-after"))
		select {
		case seen := <-traces:
			if seen.err != nil || len(seen.events) != 3 {
				t.Fatal("the signed peer did not receive all three reports and the scan result")
			}
			if seen.events[0].GetNumber() != 1 || seen.events[1].GetNumber() != 2 || seen.events[2].GetNumber() != 3 {
				t.Error("the approved stream changed the report numbering")
			}
			facts := seen.events[0].GetFacts()
			if facts.GetWorkspace() != "" || facts.GetClientVersion() != outgoingVersionSafe || facts.GetAgentLimit() != 7 || facts.GetCores() == 0 {
				t.Error("wire Facts did not omit the workspace, mask both values and preserve the numeric facts")
			}
			if inventoryVersion(seen.events[1], "claude") != outgoingMask || inventoryVersion(seen.events[2], "claude") != outgoingMask ||
				inventoryVersion(seen.events[2], "codex") != "2.3.4-before.[redacted].[redacted].after" {
				t.Error("wire inventory did not mask the peer token and both sign-ins while preserving the version's safe text")
			}
			if seen.result.GetCommandId() != outgoingScanID || seen.result.GetAttempt() != 1 ||
				seen.result.GetOutcome() != linkv1.CommandOutcome_COMMAND_OUTCOME_DONE || seen.result.GetLastEventNumber() != 3 {
				t.Error("the fresh inventory did not settle its command at the durable watermark")
			}
			for _, wire := range seen.frames {
				requireOutgoingSafe(t, "wire frame", wire, outgoingSignIn, outgoingRefresh, outgoingPeerToken)
				p.checkRedaction(t, string(wire), key)
			}
			checkOutgoingReports(t, p, opts.StateDir, key, seen.events, outgoingSignIn, outgoingRefresh, outgoingPeerToken)
			checkOutgoingReportLogs(t, stderr, 1, 2, 3)
		default:
			t.Fatal("the process finished before the signed peer observed the reports")
		}
	})

	t.Run("replacement makes required history unsafe", func(t *testing.T) {
		opts, _ := appOptions(t)
		credentials := defaultOutgoingCredentials(t, opts, outgoingSignIn)
		var opens atomic.Int32
		var replacement, leaked atomic.Bool
		p := newAppPeer(t, nil, func(p *appPeer, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
			opens.Add(1)
			if err := p.challenge(stream); err != nil {
				return err
			}
			proof, err := stream.Receive()
			if err != nil || proof.GetProof() == nil {
				return errors.New("the retained-history peer did not receive proof")
			}
			if err := replaceOutgoingCredentials(credentials, outgoingRefresh, "outgoing-new-refresh"); err != nil {
				return errors.New("could not replace the sign-in after process startup")
			}
			replacement.Store(true)
			if err := sendOutgoingWelcome(stream, false); err != nil {
				return err
			}
			for {
				frame, err := stream.Receive()
				if err != nil {
					return err
				}
				if frame.GetEvent() != nil || frame.GetCommandResult() != nil {
					leaked.Store(true)
					refusal := connect.NewError(connect.CodeFailedPrecondition, errors.New("a disputed frame reached the peer"))
					refusal.Meta().Set("Lgtm-Link-Refusal", "unknown_machine")
					return refusal
				}
			}
		})
		seedApp(t, opts, p, false)
		paths := state.PathsIn(opts.StateDir)
		seedOutgoingHistory(t, opts.StateDir)
		before := appFile(t, paths.Store)
		stdout, stderr, status := runOutgoingProcess(t, binary, opts, "", nil, "run")
		checkOutgoingStreams(t, p, stdout, stderr, appFile(t, paths.Key), outgoingSignIn, outgoingRefresh, outgoingPeerToken)
		if !replacement.Load() || leaked.Load() || opens.Load() != 1 || status != 0 {
			t.Error("the rotated credential did not stop required replay before any event or result was sent")
		}
		requireAppRecoveryInstruction(t, stderr)
		if !bytes.Equal(before, appFile(t, paths.Store)) {
			t.Error("the client rewrote the database containing the disputed event")
		}
	})

	t.Run("fixed early diagnostics for both commands", func(t *testing.T) {
		opts, _ := appOptions(t)
		p := newAppPeer(t, nil, nil)
		opts.StateDir = filepath.Join(opts.Getenv("HOME"), "unusable-"+outgoingSignIn)
		if os.WriteFile(opts.StateDir, nil, 0o600) != nil {
			t.Fatal("could not obstruct the early state path")
		}
		for _, args := range [][]string{{"join", p.server.URL, p.token}, {"run"}} {
			stdout, stderr, status := runOutgoingProcess(t, binary, opts, "", nil, args...)
			checkOutgoingStreams(t, p, stdout, stderr, nil, outgoingSignIn)
			if status != 1 || stdout.Len() != 0 {
				t.Error("the invalid state path did not stop the command with one error")
			}
			lines := appLogs(t, stderr)
			want := "client_start_failed"
			if args[0] == "join" {
				want = "join_failed"
			}
			if len(lines) != 1 || lines[0].Level != "ERROR" || lines[0].Msg != want || !strings.Contains(lines[0].Message, "state folder") {
				t.Error("the early failure omitted its fixed state folder diagnostic")
			}
		}
	})
}

type outgoingLazy struct{ value string }

func (v outgoingLazy) LogValue() slog.Value {
	return slog.GroupValue(slog.String("lazy-key-"+v.value, "lazy-before-"+v.value+"-after"))
}

func TestOutgoingFilterStructuredLog(t *testing.T) {
	t.Run("structured values", testOutgoingStructuredValues)
	t.Run("replacement between receipt and result", testOutgoingResultRefresh)
}

func testOutgoingStructuredValues(t *testing.T) {
	opts, log := appOptions(t)
	p := newAppPeer(t, nil, nil)
	seedApp(t, opts, p, false)
	const refreshBefore = "outgoing-refresh-before-fixture"
	const refreshAfter = "outgoing-refresh-after-fixture"
	credentials := defaultOutgoingCredentialsWithRefresh(t, opts, outgoingSignIn, refreshBefore)
	keyFile := appFile(t, state.PathsIn(opts.StateDir).Key)
	block, _ := pem.Decode(keyFile)
	if block == nil {
		t.Fatal("could not decode the fixture key block")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	private, ok := parsed.(ed25519.PrivateKey)
	if err != nil || !ok {
		t.Fatal("could not decode the fixture signing key")
	}
	keyBody := base64.StdEncoding.EncodeToString(block.Bytes)
	useAppLink(t, func(_ context.Context, linkOptions link.RunOptions) (link.RunOutcome, error) {
		logger := linkOptions.Logger.With(
			slog.String("attached-"+outgoingSignIn, "attached-before-"+outgoingSignIn+"-after"),
			slog.Group("attached-group-"+refreshBefore, slog.String("replacement", "before-"+outgoingRefresh+"-after")),
			slog.Any("attached-lazy", outgoingLazy{outgoingRefresh}),
		).WithGroup("context-"+outgoingSignIn).With("context-value", refreshBefore)
		if replaceOutgoingCredentials(credentials, outgoingRefresh, refreshAfter) != nil {
			t.Fatal("could not replace the local sign-in while the logger was active")
		}
		if linkOptions.Filter == nil {
			t.Error("the application did not supply a process filter to Link")
		} else {
			linkOptions.Filter.AddSecrets(outgoingPeerToken)
		}
		logger.Info("message-before-"+outgoingPeerToken+"-after",
			slog.Int("count", 17),
			slog.Group("nested-"+refreshAfter, slog.String("value-"+outgoingRefresh, "before-"+outgoingSignIn+"-after")),
			slog.Any("record-lazy", outgoingLazy{outgoingRefresh}),
			slog.String("key-file", string(keyFile)),
			slog.String("key-body", keyBody),
			slog.String("key-block", string(block.Bytes)),
			slog.String("signing-key", string(private)),
			slog.String("credential", fmt.Sprintf("%#v", linkOptions.Credential)),
		)
		return link.RunStopped, nil
	})
	if status := Run(appContext(t), opts); status != 0 {
		t.Errorf("the structured-log probe exited with status %d", status)
	}
	p.checkRedaction(t, log.String(), keyFile)
	requireOutgoingSafe(t, "structured log", log.Bytes(), outgoingSignIn, outgoingRefresh, refreshBefore, refreshAfter, outgoingPeerToken)

	var row map[string]any
	decoder := json.NewDecoder(bytes.NewReader(log.Bytes()))
	for {
		var decoded map[string]any
		err := decoder.Decode(&decoded)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal("the captured structured log was not valid JSON")
		}
		if _, ok := decoded["msg"].(string); ok && decoded["msg"] != "client_starting" {
			row = decoded
		}
	}
	if row["msg"] != "message-before-[redacted]-after" {
		t.Error("the shared logger did not mask the message while preserving safe text")
	}
	attached := outgoingLogGroup(row, "attached-group-[redacted]")
	attachedLazy := outgoingLogGroup(row, "attached-lazy")
	if row["attached-[redacted]"] != "attached-before-[redacted]-after" ||
		attached["replacement"] != "before-[redacted]-after" || attachedLazy["lazy-key-[redacted]"] != "lazy-before-[redacted]-after" {
		t.Error("attached keys, groups, values or lazy attributes escaped masking across replacement")
	}
	group := outgoingLogGroup(row, "context-[redacted]")
	nested := outgoingLogGroup(group, "nested-[redacted]")
	lazy := outgoingLogGroup(group, "record-lazy")
	if group["context-value"] != outgoingMask || nested["value-[redacted]"] != "before-[redacted]-after" ||
		lazy["lazy-key-[redacted]"] != "lazy-before-[redacted]-after" {
		t.Error("the active group, nested or lazy attributes lost their masked content")
	}
	if group["count"] != float64(17) {
		t.Error("the logger changed the numeric attribute's value or JSON type")
	}
	if group["key-file"] != outgoingMask || group["key-body"] != outgoingMask ||
		group["key-block"] != outgoingMask || group["signing-key"] != outgoingMask {
		t.Error("the logger did not register every planted enrollment key form")
	}
	credential, err := enroll.Load(state.PathsIn(opts.StateDir).Key)
	if err != nil || group["credential"] != credential.String() {
		t.Error("the logger did not preserve the credential's public formatter")
	}
}

func outgoingHarnessVersion(rows []harness.Found) string {
	for _, row := range rows {
		if row.Harness == harness.Claude && row.Installed {
			return row.Version
		}
	}
	return ""
}

func inventoryVersion(event *linkv1.Event, name string) string {
	for _, entry := range event.GetHarnessesFound().GetHarnesses() {
		if entry.GetName() == name {
			return entry.GetVersion()
		}
	}
	return ""
}

func defaultOutgoingCredentials(t *testing.T, opts Options, access string) string {
	t.Helper()
	return defaultOutgoingCredentialsWithRefresh(t, opts, access, "outgoing-initial-refresh")
}

func defaultOutgoingCredentialsWithRefresh(t *testing.T, opts Options, access, refresh string) string {
	t.Helper()
	dir := filepath.Join(opts.Getenv("HOME"), ".claude")
	path := filepath.Join(dir, ".credentials.json")
	if os.Mkdir(dir, 0o700) != nil || replaceOutgoingCredentials(path, access, refresh) != nil {
		t.Fatal("could not plant the default local sign-in file")
	}
	return path
}

func replaceOutgoingCredentials(path, access, refresh string) error {
	data, err := json.Marshal(map[string]any{"claudeAiOauth": map[string]string{"accessToken": access, "refreshToken": refresh}})
	if err != nil {
		return err
	}
	replacement := path + ".replacement"
	if err := os.WriteFile(replacement, data, 0o600); err != nil {
		return err
	}
	return os.Rename(replacement, path)
}

func writeOutgoingProbe(bin, name, versionLine string) error {
	return os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nprintf '%s\\n' '"+versionLine+"'\n"), 0o700)
}

func welcomeOutgoingPeer(p *appPeer, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse], approved bool) error {
	if err := p.challenge(stream); err != nil {
		return err
	}
	frame, err := stream.Receive()
	if err != nil || frame.GetProof() == nil {
		return errors.New("the outgoing-filter peer did not receive proof")
	}
	return sendOutgoingWelcome(stream, approved)
}

func sendOutgoingWelcome(stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse], approved bool) error {
	standing := &linkv1.MachineState{Approval: linkv1.Approval_APPROVAL_WAITING, Revision: 1}
	welcome := &linkv1.Welcome{State: standing, ServerTime: timestamppb.New(outgoingServerTime), LinkRevision: 2}
	if approved {
		standing.Approval, standing.Name = linkv1.Approval_APPROVAL_APPROVED, "filter fixture"
		welcome.ConnectionToken = outgoingPeerToken
	}
	return stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Welcome{Welcome: welcome}})
}

func sendOutgoingScan(stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse], id string) error {
	cmd := &linkv1.Command{
		Id: id, Attempt: 1, ExpiresAt: timestamppb.New(outgoingServerTime.Add(time.Minute)),
		Action: &linkv1.Command_ScanHarnesses{ScanHarnesses: &linkv1.ScanHarnesses{}},
	}
	return stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Command{Command: cmd}})
}

func runOutgoingProcess(t *testing.T, binary string, opts Options, configDir string, stop <-chan struct{}, args ...string) (*bytes.Buffer, *bytes.Buffer, int) {
	t.Helper()
	ctx := appContext(t)
	cmd := exec.CommandContext(ctx, binary, args...)
	home := opts.Getenv("HOME")
	cmd.Env = []string{"HOME=" + home, "PATH=" + filepath.Join(home, "bin"), "LGTM_CLIENT_STATE_DIR=" + opts.StateDir}
	if configDir != "" {
		cmd.Env = append(cmd.Env, "CLAUDE_CONFIG_DIR="+configDir)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if cmd.Start() != nil {
		t.Fatal("could not start the outgoing-filter client fixture")
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var err error
	select {
	case <-stop:
		_ = cmd.Process.Signal(os.Interrupt)
		err = <-done
	case err = <-done:
	}
	if ctx.Err() != nil {
		t.Error("the outgoing-filter client exceeded the fixture deadline")
	}
	status := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal("could not collect the outgoing-filter client exit")
		}
		status = exit.ExitCode()
	}
	return &stdout, &stderr, status
}

func checkOutgoingStreams(t *testing.T, p *appPeer, stdout, stderr *bytes.Buffer, key []byte, values ...string) {
	t.Helper()
	for _, log := range []*bytes.Buffer{stdout, stderr} {
		p.checkRedaction(t, log.String(), key)
		requireOutgoingSafe(t, "process output", log.Bytes(), values...)
	}
}

func requireOutgoingSafe(t *testing.T, surface string, data []byte, values ...string) {
	t.Helper()
	for _, value := range values {
		for _, form := range []string{
			value, hex.EncodeToString([]byte(value)), base64.StdEncoding.EncodeToString([]byte(value)),
			base64.RawStdEncoding.EncodeToString([]byte(value)), base64.URLEncoding.EncodeToString([]byte(value)),
			base64.RawURLEncoding.EncodeToString([]byte(value)),
		} {
			if bytes.Contains(data, []byte(form)) {
				t.Errorf("%s contains a planted secret or its encoding", surface)
				return
			}
		}
	}
}

func checkOutgoingStartup(t *testing.T, log *bytes.Buffer, workspace string) {
	t.Helper()
	var found int
	decoder := json.NewDecoder(bytes.NewReader(log.Bytes()))
	for {
		var row struct {
			Message   string `json:"msg"`
			Workspace string `json:"repositories_folder"`
		}
		err := decoder.Decode(&row)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal("the startup log is not valid JSON")
		}
		if row.Message == "client_starting" {
			found++
			if row.Workspace != workspace {
				t.Error("the startup path did not preserve its safe surroundings after masking")
			}
		}
	}
	if found != 1 {
		t.Error("the client did not log one startup record")
	}
}

func checkOutgoingReportLogs(t *testing.T, log *bytes.Buffer, numbers ...uint64) {
	t.Helper()
	var got []uint64
	decoder := json.NewDecoder(bytes.NewReader(log.Bytes()))
	for {
		var row struct {
			Message  string `json:"msg"`
			Sequence uint64 `json:"sequence"`
		}
		err := decoder.Decode(&row)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal("the report log is not valid JSON with numeric sequence fields")
		}
		if row.Message == "link_event_buffered" {
			got = append(got, row.Sequence)
		}
	}
	if !slices.Equal(got, numbers) {
		t.Error("the live log did not record the initial and refreshed durable reports")
	}
}

func checkOutgoingReports(t *testing.T, peer *appPeer, dir string, key []byte, events []*linkv1.Event, values ...string) {
	t.Helper()
	folder := appFolder(t, dir)
	defer closeAppFolder(t, folder)
	store, err := link.OpenStore(folder, link.StoreOptions{})
	if err != nil {
		t.Fatal("could not open the closed process outbox")
	}
	defer store.Close()
	rows, err := store.Read(1, len(events)+1)
	if err != nil || len(rows) != len(events) {
		t.Fatal("the closed outbox did not retain every unacknowledged report")
	}
	for i, row := range rows {
		requireOutgoingSafe(t, "durable report", row.Payload, values...)
		peer.checkRedaction(t, string(row.Payload), key)
		var event linkv1.Event
		if len(row.Payload) < 2 || row.Payload[0] != 1 || proto.Unmarshal(row.Payload[1:], &event) != nil {
			t.Fatal("the durable report did not use the expected encoding")
		}
		if event.GetNumber() != 0 {
			t.Error("the durable report stored its number inside the payload")
		}
		event.Number = row.Seq
		if !proto.Equal(&event, events[i]) {
			t.Error("a numbered wire report differed from its durable contents")
		}
	}
}

func seedOutgoingHistory(t *testing.T, dir string) {
	t.Helper()
	folder := appFolder(t, dir)
	defer closeAppFolder(t, folder)
	store, err := link.OpenStore(folder, link.StoreOptions{})
	if err != nil {
		t.Fatal("could not open the retained-history fixture")
	}
	event := &linkv1.Event{
		HappenedAt: timestamppb.New(outgoingServerTime),
		Report: &linkv1.Event_Facts{Facts: &linkv1.Facts{
			Os: "fixture", Cores: 1, ClientVersion: "before-" + outgoingRefresh + "-after",
		}},
	}
	payload, err := proto.Marshal(event)
	if err != nil {
		_ = store.Close()
		t.Fatal("could not encode the retained-history fixture")
	}
	number, err := store.Append(append([]byte{1}, payload...))
	if closeErr := store.Close(); err != nil || closeErr != nil || number != 1 {
		t.Fatal("could not seed the disputed event at the expected sequence")
	}
}

func outgoingLogGroup(row map[string]any, key string) map[string]any {
	group, _ := row[key].(map[string]any)
	return group
}

type outgoingLogObserver struct {
	mu      sync.Mutex
	records bytes.Buffer
	onWrite func([]byte)
}

func (w *outgoingLogObserver) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onWrite(data)
	return w.records.Write(data)
}

func testOutgoingResultRefresh(t *testing.T) {
	const conflict = "conflicting machine states"
	const rotatedID = "0195977c-0000-7000-8000-000000000812"
	ctx, cancel := context.WithCancel(appContext(t))
	defer cancel()
	opts, _ := appOptions(t)
	credentials := defaultOutgoingCredentials(t, opts, outgoingSignIn)
	var replacement, resultSeen, invalidFrame atomic.Bool
	observer := &outgoingLogObserver{onWrite: func(data []byte) {
		var diagnostic appDiagnostic
		if json.Unmarshal(data, &diagnostic) != nil {
			return
		}
		// Link reports a conflicting state after it writes the standing receipt
		// and before it queues the result. Neither startup report contains the new value.
		if strings.Contains(diagnostic.Msg, conflict) {
			replacement.Store(replaceOutgoingCredentials(credentials, rotatedID, "outgoing-new-refresh") == nil)
		}
		if diagnostic.Msg == "link_disconnected" && diagnostic.Code == "invalid_frame" {
			invalidFrame.Store(true)
			cancel()
		}
	}}
	opts.Stderr = observer
	p := newAppPeer(t, nil, func(p *appPeer, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
		if err := welcomeOutgoingPeer(p, stream, true); err != nil {
			return err
		}
		for {
			frame, err := stream.Receive()
			if err != nil {
				return err
			}
			if frame.GetEvent().GetNumber() == 2 {
				break
			}
		}
		cmd := &linkv1.Command{
			Id: rotatedID, Attempt: 1, ExpiresAt: timestamppb.New(outgoingServerTime.Add(time.Minute)),
			Action: &linkv1.Command_SetState{SetState: &linkv1.SetState{State: &linkv1.MachineState{
				Approval: linkv1.Approval_APPROVAL_APPROVED, Name: "conflicting fixture state", Revision: 1,
			}}},
		}
		if err := stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Command{Command: cmd}}); err != nil {
			return err
		}
		for {
			frame, err := stream.Receive()
			if err != nil {
				return err
			}
			if frame.GetCommandResult() != nil {
				resultSeen.Store(true)
				cancel()
				return nil
			}
		}
	})
	seedApp(t, opts, p, false)
	if status := Run(ctx, opts); status != 0 {
		t.Errorf("the result-refresh probe exited with status %d", status)
	}
	if !replacement.Load() {
		t.Error("the standing receipt did not reach the sign-in replacement point")
	}
	if resultSeen.Load() || !invalidFrame.Load() {
		t.Error("the new sign-in did not block the queued result with a fixed diagnostic")
	}
	p.checkRedaction(t, observer.records.String(), appFile(t, state.PathsIn(opts.StateDir).Key))
	requireOutgoingSafe(t, "result handoff diagnostics", observer.records.Bytes(), rotatedID, outgoingSignIn, outgoingPeerToken)
}
