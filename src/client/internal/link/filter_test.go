package link

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/harness"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/sysinfo"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

func TestOutgoingFilterText(t *testing.T) {
	t.Run("whole text and overlapping prefixes", func(t *testing.T) {
		filter := NewOutgoingFilter()
		filter.AddSecrets("", "雪豹", "雪豹-开", "lk-lk-X")
		input := "before 雪豹-开 / lk-lk-lk-X / 雪豹 after"
		want := "before [redacted] / lk-[redacted] / [redacted] after"
		if !filter.ContainsSecret(input) || filter.ContainsSecret("safe 雪山") || filter.MaskText(input) != want {
			t.Error("whole-text filtering did not mask the complete overlapping matches and preserve safe text")
		}
	})

	t.Run("split secret and UTF-8 character", func(t *testing.T) {
		filter := NewOutgoingFilter()
		filter.AddSecrets("雪豹", "雪豹-开", "lk-lk-X")
		stream := filter.NewTextStream()
		input := []byte("before 雪豹-开 / lk-lk-lk-X after 雪山")
		var output strings.Builder
		for i := range input {
			output.WriteString(stream.Push(input[i : i+1]))
		}
		output.WriteString(stream.Flush())
		if output.String() != "before [redacted] / lk-[redacted] after 雪山" {
			t.Error("chunked filtering did not hold a split match or preserve the surrounding UTF-8 text")
		}
	})

	t.Run("new registration while a suffix is held", func(t *testing.T) {
		filter := NewOutgoingFilter()
		filter.AddSecrets("雨天-旧")
		stream := filter.NewTextStream()
		first := stream.Push([]byte("before 雨天-"))
		if first != "before " {
			t.Error("the stream did not emit the safe prefix while holding the possible credential suffix")
		}
		filter.AddSecrets("雨天-新")
		got := first + stream.Push([]byte("新 / 雨天-旧 after 雨")) + stream.Flush()
		if got != "before [redacted] / [redacted] after 雨" {
			t.Error("the stream missed a new or retained registration, or lost the safe final suffix")
		}
	})
}

func TestOutgoingFilterReports(t *testing.T) {
	for _, workspace := range []string{"safe workspace", "normalized workspace", "abbreviated workspace"} {
		t.Run(workspace, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) { testFilteredMachineReports(t, workspace) })
		})
	}
	t.Run("typed and expanded paths", func(t *testing.T) {
		filter := NewOutgoingFilter()
		filter.AddSecrets("~/typed-fixture", "/service/home-fixture")
		if !filter.folderPathUnsafe("~/typed-fixture", "/service/safe") ||
			!filter.folderPathUnsafe("~/checkout", "/service/home-fixture") ||
			filter.folderPathUnsafe("~/checkout", "/service/safe") {
			t.Error("the pre-inspection policy did not check both typed and expanded paths")
		}
	})
	t.Run("folder append", testFilteredFolderAppend)
	t.Run("pending inventory learns verified token", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) { testFilteredPendingInventory(t) })
	})
	for _, action := range []string{"standing", "ordinary", "unsupported", "replayed", "unreadable receipt"} {
		t.Run("unsafe "+action+" command", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) { testFilteredCommand(t, action) })
		})
	}
	t.Run("queued result learns secret", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) { testFilteredQueuedResult(t) })
	})
}

func testFilteredMachineReports(t *testing.T, workspace string) {
	t.Helper()
	f := newRunPeerFixture(t, protocols[0])
	f.seed(t)
	filter := NewOutgoingFilter()
	f.opts.Filter = filter
	keyForm := base64.RawStdEncoding.EncodeToString(f.peer.keys.machineKey.Seed())
	token := "fixture-welcome-truncation-boundary"
	filter.AddSecrets(keyForm, string(harness.Gemini))
	const home = "/service/owner"
	f.opts.ScanOptions.Home = home
	f.opts.Settings.AgentLimit = 3
	f.opts.Settings.RepositoriesFolder = filepath.Join(home, "work-tree")
	wantWorkspace := "~/work-tree"
	switch workspace {
	case "normalized workspace":
		f.opts.Settings.RepositoriesFolder = filepath.Join(home, "work-\x00tree")
		filter.AddSecrets("work-tree")
		wantWorkspace = ""
	case "abbreviated workspace":
		filter.AddSecrets("~/work-tree")
		wantWorkspace = ""
	}
	prefix := strings.Repeat("v", 52)
	runReportProbes.readFacts = func(string) (sysinfo.Facts, error) {
		return sysinfo.Facts{
			OS: "os\x00 " + keyForm + " ready", ClientVersion: prefix + token + "ok",
			Cores: 7, MemoryBytes: 9<<30 + 17,
		}, nil
	}
	runReportProbes.scan = func(context.Context, harness.ScanOptions) []harness.Found {
		return []harness.Found{
			{Harness: harness.Claude, Installed: true, Version: "key " + keyForm + " ok"},
			{Harness: harness.Codex, Installed: true, Version: "token " + token + " ok"},
			{Harness: harness.Gemini, Installed: true, Version: "7.8.9"},
			{Harness: harness.Cursor, Installed: true, Version: "1.2.3"},
		}
	}
	welcome := runWelcome(approvedApproval, 1, 0)
	welcome.ConnectionToken = token
	cancel, done := f.start(t)
	defer cancel()
	leg := f.next(t, 0)
	f.welcome(t, leg, welcome)
	beat := leg.next(t).GetHeartbeat()
	if beat == nil || beat.CpuPercent == nil || beat.MemoryPercent == nil || beat.GetCpuPercent() != 17 || beat.GetMemoryPercent() != 31 {
		t.Error("the filter changed approved numeric heartbeat load")
	}

	factsFrame, inventoryFrame := eventOnLeg(t, leg), eventOnLeg(t, leg)
	if factsFrame.GetNumber() != 1 || inventoryFrame.GetNumber() != 2 {
		t.Error("filtered startup reports did not retain their event order")
	}
	cancel()
	f.stop(t, done, nil)
	store := mustOpen(t, f.opts.Folder, StoreOptions{})
	wantFacts := &linkv1.Facts{
		Os: "os [redacted] ready", ClientVersion: prefix + "[redacted]ok", Workspace: wantWorkspace,
		Cores: 7, MemoryBytes: 9<<30 + 17, AgentLimit: 3,
	}
	if !proto.Equal(factsFrame.GetFacts(), wantFacts) {
		t.Error("wire Facts missed display masking before truncation or changed an exact or numeric field")
	}
	if !filteredInventoryMatches(inventoryFrame.GetHarnessesFound(), map[string]string{
		"claude": "key [redacted] ok", "codex": "token [redacted] ok", "cursor": "1.2.3",
	}) {
		t.Error("wire inventory missed display masking, exact identifier omission or an unchanged safe row")
	}
	requireFilteredStoredEvent(t, store, factsFrame, keyForm, token, string(harness.Gemini))
	requireFilteredStoredEvent(t, store, inventoryFrame, keyForm, token, string(harness.Gemini))
	requireFilteredDiagnostics(t, f, keyForm, token)
}

func filteredInventoryMatches(report *linkv1.HarnessesFound, want map[string]string) bool {
	if report == nil || len(report.GetHarnesses()) != len(want) {
		return false
	}
	seen := make(map[string]bool)
	for _, row := range report.GetHarnesses() {
		name := row.GetName()
		version, ok := want[name]
		if !ok || seen[name] || row.GetVersion() != version {
			return false
		}
		seen[name] = true
	}
	return true
}

func testFilteredFolderAppend(t *testing.T) {
	token := "fixture-origin-credential"
	keyForm := hex.EncodeToString([]byte("fixture-key"))
	safe := &linkv1.FolderFacts{
		ResolvedPath: "/work/repo", Name: "repo", MainBranch: "main",
		CheckedOutBranch: proto.String("feature/topic"),
		Remote:           proto.String("https://git.example.test/owner/repo.git"),
		FirstCommit:      proto.String(strings.Repeat("a", 40)), Clean: true,
	}
	for _, tc := range []struct {
		name   string
		change func(*linkv1.FolderFacts)
		refuse bool
	}{
		{name: "safe exact values"},
		{"resolved path", func(f *linkv1.FolderFacts) { f.ResolvedPath = "/work/" + token + "/repo" }, true},
		{"name", func(f *linkv1.FolderFacts) { f.Name = token }, true},
		{"main branch", func(f *linkv1.FolderFacts) { f.MainBranch = "main-" + token }, true},
		{"checked-out branch", func(f *linkv1.FolderFacts) { f.CheckedOutBranch = proto.String("feature/" + keyForm) }, true},
		{"first commit", func(f *linkv1.FolderFacts) {
			f.FirstCommit = proto.String(strings.Repeat("b", 40-len(keyForm)) + keyForm)
		}, true},
		{"withheld sanitized origin", func(f *linkv1.FolderFacts) {
			f.Remote = proto.String("https://git.example.test/owner/" + token + ".git")
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			folder, _ := newState(t)
			store := mustCreate(t, folder, StoreOptions{})
			filter := NewOutgoingFilter()
			filter.AddSecrets(token, keyForm)
			run := &clientRun{opts: RunOptions{Filter: filter}, store: store}
			facts := proto.CloneOf(safe)
			if tc.change != nil {
				tc.change(facts)
			}
			seen := &linkv1.FolderSeen{
				OperationId: "000000a1-0000-7000-8000-0000000000a1",
				CommandId:   "000000a2-0000-7000-8000-0000000000a2",
				Result:      &linkv1.FolderSeen_Facts{Facts: facts},
			}
			event := &linkv1.Event{HappenedAt: timestamppb.New(tNow), Report: &linkv1.Event_FolderSeen{FolderSeen: seen}}
			raw := filterFixturePayload(t, event)
			number, err := run.appendReport(t.Context(), raw)
			if err != nil || number != 1 || store.Next() != 2 || store.Len() != 1 {
				t.Fatal("the common append did not durably allocate exactly one folder report")
			}
			if tc.refuse {
				seen.Result = &linkv1.FolderSeen_Refusal{Refusal: linkv1.FolderRefusal_FOLDER_REFUSAL_PATH_NOT_USABLE}
			} else if tc.change != nil {
				facts.Remote, facts.RemoteOmitted = nil, true
			}
			event.Number = 1
			requireFilteredStoredEvent(t, store, event, token, keyForm)
		})
	}
}

func testFilteredPendingInventory(t *testing.T) {
	t.Helper()
	f := newRunPeerFixture(t, protocols[0])
	f.seed(t)
	f.opts.Filter = NewOutgoingFilter()
	maxEvents := fillFilterFixtureOutbox(t, f)
	token := "fixture-rotated-inventory-token"
	runReportProbes.readFacts = func(string) (sysinfo.Facts, error) {
		return sysinfo.Facts{}, errors.New("fixture has no fresh facts")
	}
	var scans int
	runReportProbes.scan = func(ctx context.Context, _ harness.ScanOptions) []harness.Found {
		scans++
		if scans != 1 {
			<-ctx.Done()
			return nil
		}
		return []harness.Found{{Harness: harness.Claude, Installed: true, Version: "v " + token + " ready"}}
	}
	cancel, done := f.start(t)
	defer cancel()
	first := f.next(t, 0)
	f.welcome(t, first, runWelcome(approvedApproval, 1, 0))
	if eventOnLeg(t, first).GetNumber() != 1 || scans != 1 {
		t.Fatal("the first approved stream did not capture inventory behind the full outbox")
	}
	first.refuse <- peerError(connect.CodeUnavailable, "", "")
	second := f.next(t, 15*time.Second)
	rotated := runWelcome(approvedApproval, 1, 0)
	rotated.ConnectionToken = token
	f.welcome(t, second, rotated)
	if eventOnLeg(t, second).GetNumber() != 1 {
		t.Fatal("the rotated stream did not resume at the first retained event")
	}
	second.send(t, ackEvent(1))
	synctest.Wait()
	wantNumber := maxEvents + 1
	var buffered []uint64
	for _, line := range f.diagnostics(t) {
		if line.Msg == "link_event_buffered" {
			buffered = append(buffered, line.Sequence)
		}
	}
	if len(buffered) != 1 || buffered[0] != wantNumber {
		t.Error("releasing one slot did not buffer exactly one pending replacement at the next number")
	}
	if eventOnLeg(t, second).GetNumber() != 2 {
		t.Error("the rotated stream did not continue replaying retained history")
	}
	second.refuse <- peerError(connect.CodeUnavailable, "", "")
	third := f.next(t, 15*time.Second)
	// The new peer has consumed the old prefix. It must still get the replacement.
	f.welcome(t, third, runWelcome(approvedApproval, 1, maxEvents))
	result := receiveNow(t, third.read(), "the third stream did not respond")
	if result.frame.GetHeartbeat() != nil {
		third.send(t, ackHeartbeat())
		result = receiveNow(t, third.read(), "the third stream did not replay the pending replacement")
	}
	replacement := result.frame.GetEvent()
	version := filteredHarnessVersion(replacement.GetHarnessesFound(), "claude")
	if result.err != nil || replacement.GetNumber() != wantNumber || replacement.GetHarnessesFound() == nil ||
		(version != "v [redacted] ready" && version != "") {
		t.Error("the replacement failed to replay with the newly known token masked or omitted")
	}
	cancel()
	receiveNow(t, done, "the pending-inventory client did not stop")
	store := mustOpen(t, f.opts.Folder, StoreOptions{})
	rows, err := store.Read(wantNumber, 1)
	if err != nil || len(rows) != 1 || rows[0].Seq != wantNumber {
		t.Fatal("the pending replacement did not retain its durable number")
	}
	if bytes.Contains(rows[0].Payload, []byte(token)) {
		t.Error("the pending replacement durably stored the newly known token")
	}
	if replacement != nil {
		requireFilteredStoredEvent(t, store, replacement, token)
	}
	requireFilteredDiagnostics(t, f, token)
}

func fillFilterFixtureOutbox(t *testing.T, f *runPeerFixture) uint64 {
	t.Helper()
	limits, err := withStoreDefaults(StoreOptions{})
	if err != nil {
		t.Fatal("could not read the default outbox limits")
	}
	count := uint64(limits.MaxEvents)
	payload := fixtureFactsPayload(t, "retained")
	editStore(t, f.file(), func(tx *bolt.Tx) error {
		for n := uint64(1); n <= count; n++ {
			if err := tx.Bucket(eventsBucket).Put(be(n), payload); err != nil {
				return err
			}
		}
		return tx.Bucket(metaBucket).Put(nextKey, be(count+1))
	})
	return count
}

func filteredHarnessVersion(report *linkv1.HarnessesFound, name string) string {
	for _, row := range report.GetHarnesses() {
		if row.GetName() == name {
			return row.GetVersion()
		}
	}
	return ""
}

func testFilteredCommand(t *testing.T, action string) {
	t.Helper()
	f := newRunPeerFixture(t, protocols[0])
	f.seed(t)
	f.opts.Filter = NewOutgoingFilter()
	expires := time.Now().Add(time.Minute)
	cmd := scanCommand(153, 1, expires)
	switch action {
	case "standing":
		cmd = stateCommand(153, expires, 2, removedApproval, "")
	case "unsupported":
		cmd.Action = nil
	case "replayed":
		store := mustOpen(t, f.opts.Folder, StoreOptions{})
		if _, saved, err := store.recordResult(receiptCommand(cmd), []byte{byte(outcomeDone)}, false, time.Time{}); err != nil || saved.outcome != outcomeDone {
			t.Fatal("could not seed the completed receipt before learning its identifier")
		}
		if store.Close() != nil {
			t.Fatal("could not close the receipt fixture")
		}
		cmd = attemptCommand(cmd, 2)
	}
	before := readReceipts(t, f.file())
	if action == "unreadable receipt" {
		// The last startup acknowledgement arrives after the initial receipt scan.
		// An admission gate that looks up the receipt first would fault on this row.
		const damaged = "fixture receipt damaged after startup"
		before[cmd.GetId()] = damaged
		t.Cleanup(swapCommit(func(db *bolt.DB, write func(*bolt.Tx) error) error {
			return db.Update(func(tx *bolt.Tx) error {
				events := tx.Bucket(eventsBucket)
				hadStartupInventory := events.Get(be(2)) != nil
				if err := write(tx); err != nil {
					return err
				}
				if hadStartupInventory && events.Get(be(2)) == nil {
					return tx.Bucket(receiptsBucket).Put([]byte(cmd.GetId()), []byte(damaged))
				}
				return nil
			})
		}))
	}
	sent := captureFilterResults(t)
	scans := 0
	runReportProbes.scan = func(context.Context, harness.ScanOptions) []harness.Found {
		scans++
		return nil
	}
	f.opts.Filter.AddSecrets(cmd.GetId())
	cancel, done := f.start(t)
	defer cancel()
	leg := f.next(t, 0)
	f.welcome(t, leg, runWelcome(approvedApproval, 1, 0))
	for n := uint64(1); n <= 2; n++ {
		if eventOnLeg(t, leg).GetNumber() != n {
			t.Fatal("the approved startup did not send its two baseline reports")
		}
		leg.send(t, ackEvent(n))
	}
	synctest.Wait()
	leg.send(t, commandFrame(cmd))
	synctest.Wait()
	if scans != 1 {
		t.Error("the unsafe command reached the scan worker")
	}
	select {
	case <-sent:
		t.Error("the client handed an unsafe command result to the sender")
	default:
	}
	requireFilterInvalidFrame(t, f)
	cancel()
	receiveNow(t, done, "the unsafe-command client did not stop")
	after := readReceipts(t, f.file())
	if len(after) != len(before) || after[cmd.GetId()] != before[cmd.GetId()] {
		t.Error("the unsafe command wrote or changed a receipt")
	}
	requireFilteredDiagnostics(t, f, cmd.GetId())
}

func testFilteredQueuedResult(t *testing.T) {
	t.Helper()
	f := newRunPeerFixture(t, protocols[0])
	f.seed(t)
	f.opts.Filter = NewOutgoingFilter()
	cmd := stateCommand(211, time.Now().Add(time.Minute), 2, waitingApproval, "")
	committed := make(chan struct{}, 1)
	t.Cleanup(swapCommit(func(db *bolt.DB, write func(*bolt.Tx) error) error {
		var wroteReceipt bool
		err := db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket(receiptsBucket)
			key := []byte(cmd.GetId())
			hadReceipt := bucket.Get(key) != nil
			if err := write(tx); err != nil {
				return err
			}
			wroteReceipt = !hadReceipt && bucket.Get(key) != nil
			return nil
		})
		if err == nil && wroteReceipt {
			committed <- struct{}{}
		}
		return err
	}))
	blocked, release, sent := holdFilterHeartbeat(t)
	cancel, done := f.start(t)
	defer cancel()
	leg := f.next(t, 0)
	f.welcome(t, leg, runWelcome(waitingApproval, 1, 0))
	receiveNow(t, blocked, "the initial heartbeat did not hold the sender")
	leg.send(t, commandFrame(cmd))
	receiveNow(t, committed, "the safe command did not commit its receipt before registration")
	f.opts.Filter.AddSecrets(cmd.GetId())
	close(release)
	synctest.Wait()
	select {
	case <-sent:
		t.Error("a result that became unsafe after queuing reached the sender")
	default:
	}
	requireFilterInvalidFrame(t, f)
	cancel()
	receiveNow(t, done, "the queued-result client did not stop")
	if len(readReceipts(t, f.file())) != 1 {
		t.Error("the handoff changed the count of receipts committed before registration")
	}
	requireFilteredDiagnostics(t, f, cmd.GetId())
}

func captureFilterResults(t *testing.T) <-chan struct{} {
	t.Helper()
	sent := make(chan struct{}, 1)
	useSessionIO(t, func(_ context.Context, _ *session, wire runSessionIO) runSessionIO {
		send := wire.send
		wire.send = func(frame *linkv1.OpenRequest) error {
			if frame.GetCommandResult() != nil {
				select {
				case sent <- struct{}{}:
				default:
				}
			}
			return send(frame)
		}
		return wire
	})
	return sent
}

func holdFilterHeartbeat(t *testing.T) (<-chan struct{}, chan struct{}, <-chan struct{}) {
	t.Helper()
	blocked, release, sent := make(chan struct{}, 1), make(chan struct{}), make(chan struct{}, 1)
	useSessionIO(t, func(ctx context.Context, _ *session, wire runSessionIO) runSessionIO {
		send := wire.send
		wire.send = func(frame *linkv1.OpenRequest) error {
			if frame.GetHeartbeat() != nil {
				select {
				case blocked <- struct{}{}:
				default:
				}
				select {
				case <-release:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			if frame.GetEvent() != nil || frame.GetCommandResult() != nil {
				select {
				case sent <- struct{}{}:
				default:
				}
			}
			return send(frame)
		}
		return wire
	})
	return blocked, release, sent
}

func requireFilterInvalidFrame(t *testing.T, f *runPeerFixture) {
	t.Helper()
	count := 0
	for _, line := range f.diagnostics(t) {
		if line.Msg == "link_disconnected" && line.Code == "invalid_frame" {
			count++
		}
	}
	if count != 1 {
		t.Error("the client did not emit exactly one fixed invalid-frame diagnostic")
	}
}

func TestOutgoingFilterLegacyReplay(t *testing.T) {
	for _, standing := range []struct {
		name     string
		approval linkv1.Approval
	}{{"approved", approvedApproval}, {"waiting", waitingApproval}} {
		t.Run(standing.name, func(t *testing.T) {
			for _, scenario := range []string{"safe", "display", "exact", "unclassifiable"} {
				t.Run(scenario, func(t *testing.T) {
					synctest.Test(t, func(t *testing.T) { testFilteredLegacy(t, standing.approval, scenario) })
				})
			}
		})
	}
	t.Run("registration after preflight", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			f := newRunPeerFixture(t, protocols[0])
			token := "fixture-retained-late-credential"
			f.opts.Filter = NewOutgoingFilter()
			f.seed(t, fixtureFactsPayload(t, token))
			before := readBytes(t, f.file())
			blocked, release, sent := holdFilterHeartbeat(t)
			cancel, done := f.start(t)
			defer cancel()
			leg := f.next(t, 0)
			f.welcome(t, leg, runWelcome(waitingApproval, 1, 0))
			receiveNow(t, blocked, "the heartbeat did not hold replay after preflight")
			f.opts.Filter.AddSecrets(token)
			close(release)
			requireRecoveryStop(t, f, done, false)
			requireEmpty(t, sent, "the client handed off a report that became unsafe after preflight")
			if !bytes.Equal(before, readBytes(t, f.file())) {
				t.Error("the handoff fault changed the retained report's number or bytes")
			}
			requireFilteredDiagnostics(t, f, token)
		})
	})
}

func testFilteredLegacy(t *testing.T, approval linkv1.Approval, scenario string) {
	t.Helper()
	f := newRunPeerFixture(t, protocols[0])
	f.opts.Filter = NewOutgoingFilter()
	// The quotes, backslash and non-ASCII character are escaped in the fixture source.
	const secret = "old-\"\\雪"
	f.opts.Filter.AddSecrets(secret)
	confirmed := fixtureFactsPayload(t, secret)
	safe := &linkv1.Event{HappenedAt: timestamppb.New(tNow), Report: &linkv1.Event_Facts{
		Facts: &linkv1.Facts{Cores: 2, Os: "original OS", Workspace: "~/old-work", ClientVersion: "before upgrade"},
	}}
	// Appending a repeated known field makes valid legacy bytes with a different encoding.
	factsTag := safe.ProtoReflect().Descriptor().Fields().ByName("facts").Number()
	safeRaw := protowire.AppendTag(filterFixturePayload(t, safe), factsTag, protowire.BytesType)
	safeRaw = protowire.AppendBytes(safeRaw, protowire.AppendString(protowire.AppendTag(nil, 1, protowire.BytesType), "original OS"))
	reports := [][]byte{confirmed, safeRaw}
	if scenario != "safe" {
		required := proto.CloneOf(safe)
		switch scenario {
		case "display":
			var facts linkv1.Facts
			if protojson.Unmarshal([]byte(`{"cores":2,"clientVersion":"before old-\"\\\u96ea after"}`), &facts) != nil ||
				!strings.Contains(facts.GetClientVersion(), secret) {
				t.Fatal("the escaped legacy fixture did not decode to its planted display value")
			}
			required.Report = &linkv1.Event_Facts{Facts: &facts}
		case "exact":
			required.GetFacts().Workspace = "/work/" + secret
		case "unclassifiable":
			unknown := protowire.AppendTag(nil, 99, protowire.BytesType)
			required.GetFacts().ProtoReflect().SetUnknown(protowire.AppendString(unknown, "unclassified"))
		}
		reports = append(reports, filterFixturePayload(t, required))
	}
	f.seed(t, reports...)
	before := readBytes(t, f.file())
	var supplied <-chan struct{}
	if scenario != "safe" {
		supplied = observeRecoveryFrames(t)
	}
	cancel, done := f.start(t)
	defer cancel()
	leg := f.next(t, 0)
	f.welcome(t, leg, runWelcome(approval, 1, 1))
	if scenario == "safe" {
		safe.Number = 2
		if event := eventOnLeg(t, leg); !proto.Equal(event, safe) {
			t.Error("safe legacy replay changed the report or its wire number")
		}
		cancel()
		f.stop(t, done, nil)
		store := mustOpen(t, f.opts.Folder, StoreOptions{})
		rows, err := store.Read(1, 1)
		if err != nil || len(rows) != 1 || rows[0].Seq != 2 || !bytes.Equal(rows[0].Payload, safeRaw) {
			t.Error("safe legacy replay changed the original retained encoding")
		}
	} else {
		requireRecoveryStop(t, f, done, false)
		requireEmpty(t, supplied, "the client sent a report before validating the complete required suffix")
		if !bytes.Equal(before, readBytes(t, f.file())) {
			t.Error("the recovery stop changed saved bytes or numbers, or acknowledged the confirmed prefix")
		}
		time.Sleep(16 * time.Second)
		requireEmpty(t, f.peer.opens, "unsafe required history prompted another dial")
	}
	requireFilteredDiagnostics(t, f, secret)
}

func filterFixturePayload(t *testing.T, event *linkv1.Event) []byte {
	t.Helper()
	raw, err := (proto.MarshalOptions{}).MarshalAppend([]byte{1}, event)
	if err != nil {
		t.Fatal("could not encode the version 1 fixture report")
	}
	return raw
}

func requireFilteredStoredEvent(t *testing.T, store *Store, wire *linkv1.Event, secrets ...string) {
	t.Helper()
	rows, err := store.Read(wire.GetNumber(), 1)
	if err != nil || len(rows) != 1 || rows[0].Seq != wire.GetNumber() {
		t.Fatal("the event number is absent from the durable queue")
	}
	raw := rows[0].Payload
	checkNoFixtureSecrets(t, string(raw), secrets...)
	stored := new(linkv1.Event)
	if len(raw) < 2 || raw[0] != 1 || proto.Unmarshal(raw[1:], stored) != nil || stored.GetNumber() != 0 {
		t.Fatal("the durable queue does not hold an unnumbered version 1 report")
	}
	stored.Number = rows[0].Seq
	if !proto.Equal(stored, wire) {
		t.Error("the durable filtered report differs from the expected numbered event")
	}
}

func requireFilteredDiagnostics(t *testing.T, f *runPeerFixture, secrets ...string) {
	t.Helper()
	f.log.mu.Lock()
	raw := f.log.buffer.String()
	f.log.mu.Unlock()
	f.checkOutput(t, raw)
	checkNoFixtureSecrets(t, raw, secrets...)
}
