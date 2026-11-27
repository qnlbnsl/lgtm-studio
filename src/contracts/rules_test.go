package contracts_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The tests of this file read the text of the comments of the link. They cannot run the rules, which
// belong to the built programs. They hold the rules that a reader can mistake, so that a change to
// the text is seen (plan go-s0-contract, amendment "The freeze of the link", item F4).

const linkDir = "proto/lgtm/link/v1"

// readLines returns the lines of one proto file of the link.
func readLines(t *testing.T, file string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(linkDir, file))
	if err != nil {
		t.Fatalf("cannot read %s: %v", file, err)
	}
	return strings.Split(string(raw), "\n")
}

// commentBefore returns the raw lines of the comment block that ends directly above the first line
// whose trimmed text starts with decl.
func commentBefore(t *testing.T, lines []string, decl string) []string {
	t.Helper()
	at := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), decl) {
			at = i
			break
		}
	}
	if at < 0 {
		t.Fatalf("the declaration %q is not in the file", decl)
	}
	start := at
	for start > 0 && strings.HasPrefix(strings.TrimSpace(lines[start-1]), "//") {
		start--
	}
	return lines[start:at]
}

var spaces = regexp.MustCompile(`\s+`)

// normalize joins comment lines into one text with single spaces and no comment markers.
func normalize(lines []string) string {
	var parts []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, "//") {
			continue
		}
		parts = append(parts, strings.TrimPrefix(l, "//"))
	}
	return strings.TrimSpace(spaces.ReplaceAllString(strings.Join(parts, " "), " "))
}

// codeRow is one object of testdata/link-codes.json.
type codeRow struct {
	Call                   string   `json:"call"`
	Code                   string   `json:"code"`
	Action                 string   `json:"action"`
	Key                    string   `json:"key"`
	Reasons                []string `json:"reasons"`
	AfterVerifiedChallenge bool     `json:"after_verified_challenge"`
}

// tableRow is one row of the table of codes in the comment of LinkService.
type tableRow struct {
	call, code, action, text string
}

var (
	tableStart = regexp.MustCompile(`^//   (Join|Open|both)\s+(\S+)\s+(stop|dial again)\s+(.*)$`)
	tableCont  = regexp.MustCompile(`^//\s{20,}(\S.*)$`)
)

// readTable returns the rows of the table of codes, in order. A row's text runs over its
// continuation lines.
func readTable(t *testing.T) []tableRow {
	t.Helper()
	block := commentBefore(t, readLines(t, "link.proto"), "service LinkService {")
	var rows []tableRow
	for _, l := range block {
		if m := tableStart.FindStringSubmatch(l); m != nil {
			rows = append(rows, tableRow{call: m[1], code: m[2], action: m[3], text: m[4]})
			continue
		}
		if m := tableCont.FindStringSubmatch(l); m != nil && len(rows) > 0 {
			rows[len(rows)-1].text += " " + m[1]
		}
	}
	for i := range rows {
		rows[i].text = spaces.ReplaceAllString(rows[i].text, " ")
	}
	return rows
}

const verifiedSentence = "After a Challenge whose signature verified."

// TestLinkCodesAreWritten holds the table of codes in the comment of LinkService to the data file
// that plans go-s3-client-link and go-s3-server-stream read.
func TestLinkCodesAreWritten(t *testing.T) {
	raw, err := os.ReadFile("testdata/link-codes.json")
	if err != nil {
		t.Fatalf("the table of codes as data is missing: %v", err)
	}
	var want []codeRow
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatalf("the table of codes does not parse: %v", err)
	}
	if len(want) != 9 {
		t.Fatalf("link-codes.json holds %d rows, want 9", len(want))
	}
	for i, w := range want {
		if w.Action != "stop" && w.Action != "dial again" {
			t.Errorf("row %d: the action is %q, want \"stop\" or \"dial again\"", i, w.Action)
		}
		// Every stop on Open waits for a verified signature: the answer before it is not proof.
		if w.Call == "Open" && w.Action == "stop" && !w.AfterVerifiedChallenge {
			t.Errorf("row %d: a stop of Open must hold after_verified_challenge", i)
		}
	}
	got := readTable(t)
	if len(got) != len(want) {
		t.Fatalf("the table in the comment of LinkService has %d rows, want %d", len(got), len(want))
	}
	for i, w := range want {
		g := got[i]
		if g.call != w.Call || g.code != w.Code || g.action != w.Action {
			t.Errorf("row %d reads %q %q %q, want %q %q %q", i, g.call, g.code, g.action, w.Call, w.Code, w.Action)
			continue
		}
		if w.Key != "" && !strings.Contains(g.text, "`"+w.Key+"`") {
			t.Errorf("row %d (%s %s) does not name its key `%s`", i, g.call, g.code, w.Key)
		}
		for _, r := range w.Reasons {
			if !strings.Contains(g.text, r) {
				t.Errorf("row %d (%s %s) does not name its reason %s", i, g.call, g.code, r)
			}
		}
		if has := strings.Contains(g.text, verifiedSentence); has != w.AfterVerifiedChallenge {
			t.Errorf("row %d (%s %s): the text says %q is %v, the data says after_verified_challenge is %v",
				i, g.call, g.code, verifiedSentence, has, w.AfterVerifiedChallenge)
		}
	}
}

// TestCodeFallbackIsWritten holds the rule for a reason that a machine does not know to Open
// failed_precondition alone, and says that both stops of Join hold whatever the reason.
func TestCodeFallbackIsWritten(t *testing.T) {
	text := normalize(commentBefore(t, readLines(t, "link.proto"), "service LinkService {"))
	phrases := []string{
		"For Open failed_precondition, a reason the machine does not know, or a missing reason, means dial again.",
		"Both stops of Join hold whatever the reason",
	}
	for _, p := range phrases {
		if n := strings.Count(text, p); n != 1 {
			t.Errorf("the comment of LinkService holds %q %d times, want once", p, n)
		}
	}
	if n := strings.Count(text, "a reason the machine does not know"); n != 1 {
		t.Errorf("the fallback for an unknown reason is written %d times, want once, for Open failed_precondition only", n)
	}
}

var joinReasonLine = regexp.MustCompile(`^  //   - (\w+):`)

// TestJoinRefusalOrderIsWritten holds the order in which Join checks a token.
func TestJoinRefusalOrderIsWritten(t *testing.T) {
	block := commentBefore(t, readLines(t, "link.proto"), "rpc Join(")
	var got []string
	for _, l := range block {
		if m := joinReasonLine.FindStringSubmatch(l); m != nil {
			got = append(got, m[1])
		}
	}
	want := []string{"token_unknown", "token_expired", "token_revoked", "token_used", "machine_removed"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the reasons of Join read %v in the comment, want %v", got, want)
	}
	if !strings.Contains(normalize(block), "checking in this order against its state at the moment it decides") {
		t.Errorf("the comment of Join no longer says that the refusals are checked in this order")
	}
}

// TestHandshakeOrderIsWritten holds the six decisions of the server after Challenge, as exact
// consecutive lines.
func TestHandshakeOrderIsWritten(t *testing.T) {
	block := commentBefore(t, readLines(t, "link.proto"), "rpc Open(")
	want := []string{
		"  //   - `unavailable` when it cannot look the machine up.",
		"  //   - `failed_precondition` with unknown_machine when it does not know the machine id.",
		"  //   - `failed_precondition` with link_revision_mismatch when it does not speak the revision.",
		"  //   - `deadline_exceeded` when Proof is not there 10 seconds after it sent Challenge.",
		"  //   - `invalid_argument` when Proof breaks a value rule.",
		"  //   - `unauthenticated` when the machine's signature does not verify.",
	}
	at := -1
	for i, l := range block {
		if l == want[0] {
			at = i
			break
		}
	}
	if at < 0 || at+len(want) > len(block) {
		t.Fatalf("the first decision after Challenge is not written as the exact line %q", want[0])
	}
	for i, w := range want {
		if block[at+i] != w {
			t.Errorf("decision %d reads %q, want %q", i+1, block[at+i], w)
		}
	}
	if !strings.Contains(normalize(block), "Then it decides in this order") {
		t.Errorf("the comment of Open no longer says that the server decides in this order")
	}
}

// TestLinkNumbersAreWritten holds the numbers of the link to exact phrases.
func TestLinkNumbersAreWritten(t *testing.T) {
	want := map[string][]string{
		"link.proto": {
			"Hello or Proof came more than 10 seconds late",
			"when Proof is not there 10 seconds after it sent Challenge",
			"when Hello is not there 10 seconds after the stream opened",
			"A machine that hears nothing from the server for 30 seconds ends the stream and dials again",
			"Event.number is at most 2^63 - 1, which is 9223372036854775807",
			"Heartbeat.cpu_percent and Heartbeat.memory_percent, when present, are 0 to 100",
			"HarnessesFound holds at most 16 harnesses",
			`"lgtm-link-v1 server"`,
			`"lgtm-link-v1 machine"`,
		},
		"events.proto": {
			"30 seconds without one, measured by the server's monotonic clock",
			"A machine that hears nothing from the server for 30 seconds ends the stream and dials again",
			"A number of 0, or above 2^63 - 1, ends the stream with `invalid_argument`",
			"At most 16 harnesses.",
			"The load of the processor, from 0 to 100",
			"The memory in use, from 0 to 100",
		},
		"commands.proto": {
			"until 23 hours after expires_at, and never later",
			"no earlier than 24 hours after expires_at",
			"until 24 hours after expires_at by its own clock",
		},
	}
	files := make([]string, 0, len(want))
	for f := range want {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, f := range files {
		text := normalize(readLines(t, f))
		for _, p := range want[f] {
			if !strings.Contains(text, p) {
				t.Errorf("%s does not hold the phrase %q", f, p)
			}
		}
	}
}

// part is one piece of a proof text, with its place.
type part struct {
	name   string
	offset int
}

// TestProofCommentMatchesTheExample holds the comment of Proof to the order of the proof texts in
// the worked example, which TestProofExamplesHoldTogether builds and signs.
func TestProofCommentMatchesTheExample(t *testing.T) {
	raw, err := os.ReadFile("testdata/link-proof.json")
	if err != nil {
		t.Fatalf("the worked example is missing: %v", err)
	}
	var ex map[string]string
	if err := json.Unmarshal(raw, &ex); err != nil {
		t.Fatalf("the worked example does not parse: %v", err)
	}
	dec := func(field string) []byte {
		b, err := hex.DecodeString(ex[field])
		if err != nil || len(b) == 0 {
			t.Fatalf("%s is missing or not hexadecimal", field)
		}
		return b
	}
	comment := normalize(commentBefore(t, readLines(t, "link.proto"), "message Proof {"))
	// The phrase that names each part in the comment.
	phrase := map[string]string{
		"label":         "a label",
		"zero byte":     "one zero byte",
		"length":        "one byte that holds the length of the machine id",
		"machine id":    "the machine id in UTF-8",
		"machine nonce": "the machine nonce",
		"server nonce":  "the server nonce",
	}
	id := []byte(ex["machine_id"])
	for _, c := range []struct{ field, label string }{
		{"machine_text", "lgtm-link-v1 machine"},
		{"server_text", "lgtm-link-v1 server"},
	} {
		text := dec(c.field)
		label := []byte(c.label)
		if !bytes.HasPrefix(text, label) || text[len(label)] != 0 || int(text[len(label)+1]) != len(id) {
			t.Fatalf("%s does not start with its label, a zero byte and the length of the machine id", c.field)
		}
		if !strings.Contains(comment, `"`+c.label+`"`) {
			t.Errorf("the comment of Proof does not name the label %q", c.label)
		}
		parts := []part{
			{"label", bytes.Index(text, label)},
			{"zero byte", len(label)},
			{"length", len(label) + 1},
			{"machine id", bytes.Index(text, id)},
			{"machine nonce", bytes.Index(text, dec("machine_nonce"))},
			{"server nonce", bytes.Index(text, dec("server_nonce"))},
		}
		sort.Slice(parts, func(i, j int) bool { return parts[i].offset < parts[j].offset })
		from := 0
		for _, p := range parts {
			i := strings.Index(comment[from:], phrase[p.name])
			if i < 0 {
				t.Errorf("%s: the comment of Proof does not name the %s (%q) after the parts before it", c.field, p.name, phrase[p.name])
				break
			}
			from += i + len(phrase[p.name])
		}
	}
}

// requireWrittenRules accepts wrapped comments and checks each rule independently.
func requireWrittenRules(t *testing.T, source, text string, rules [][2]string) {
	t.Helper()
	text = strings.ToLower(strings.ReplaceAll(text, "`", ""))
	for _, rule := range rules {
		if !regexp.MustCompile(rule[1]).MatchString(text) {
			t.Errorf("%s does not state %s", source, rule[0])
		}
	}
}

func TestRevisionTwoLinkRulesAreWritten(t *testing.T) {
	link := readLines(t, "link.proto")
	open := normalize(commentBefore(t, link, "rpc Open("))
	requireWrittenRules(t, "LinkService.Open", open, [][2]string{
		{"history validation follows verified proof and the revision check",
			`history[^.]*(after|once)[^.]*((proof[^.]*verif|verified proof)[^.]*revision|revision[^.]*(proof[^.]*verif|verified proof))|after [^.]*((proof[^.]*verif|verified proof)[^.]*revision|revision[^.]*(proof[^.]*verif|verified proof))[^.]*history`},
		{"only the latest admitted run may reconnect, even while draining",
			`only the (latest|most recent) admitted run[^.]*reconnect[^.]*(drain|pending)`},
		{"a reconnect repeats the immutable recorded boundary",
			`reconnect[^.]*(same|saved|recorded|unchanged|exact)[^.]*boundar|reconnect[^.]*boundar[^.]*(immutable|unchanged)`},
		{"superseded runs are refused before boundary validation, including after restart",
			`superseded run[^.]*refus[^.]*before[^.]*boundar[^.]*after[^.]*restart|refus[^.]*superseded run[^.]*before[^.]*boundar[^.]*after[^.]*restart`},
		{"an unseen run begins above the mark and staged watermarks",
			`unseen run[^.]*above[^.]*mark[^.]*staged[^.]*watermark`},
		{"an unseen run cannot move behind the pending boundary",
			`unseen run[^.]*(at or above|at or after|cannot lower|at least)[^.]*pending boundar`},
		{"a tie goes to the later successful admission",
			`(tie|equal boundar)[^.]*later successful admission|later successful admission[^.]*(tie|equal boundar)`},
		{"the run becomes ready once when the mark reaches first_event_number minus one",
			`ready[^.]*once[^.]*mark[^.]*first_event_number\s*-\s*1|mark[^.]*first_event_number\s*-\s*1[^.]*ready[^.]*once`},
		{"there is no command claim, send permit or cursor advance before readiness",
			`(before readiness|before [^.]*ready|until [^.]*ready)[^.]*(no |without )[^.]*claim[^.]*permit[^.]*cursor|no [^.]*claim[^.]*permit[^.]*cursor[^.]*(before readiness|before [^.]*ready|until [^.]*ready)`},
		{"waiting clients can replay retained events and send heartbeats",
			`(waiting|waits?) for approval[^.]*replay[^.]*retained[^.]*event[^.]*heartbeat`},
		{"during waiting replay, valid Facts apply and other eligible events are consumed without effect",
			`(apply|applies)[^.]*valid facts[^.]*consum[^.]*other[^.]*event[^.]*without effect`},
		{"replay acknowledgements precede the gated SetState",
			`replay[^.]*(acknowledg|eventack)[^.]*(before|precede)[^.]*setstate`},
		{"fresh reports wait for approval",
			`fresh report[^.]*(wait[^.]*approval|only after approval)`},
		{"history announcements are refused with the existing retryable invalid_argument",
			`history[^.]*refus[^.]*invalid_argument|refus[^.]*history[^.]*invalid_argument`},
		{"history refusal leaves the incumbent and command ledger intact",
			`refus[^.]*leaves[^.]*incumbent[^.]*command ledger[^.]*(intact|unchanged)`},
		{"the outer encoded message budget includes the wrapper and applies after decompression",
			`(65,?536 bytes|64 kibibytes)[^.]*wrapper[^.]*after decompression|wrapper[^.]*(65,?536 bytes|64 kibibytes)[^.]*after decompression`},
	})
	hello := normalize(commentBefore(t, link, "string run_id"))
	requireWrittenRules(t, "Hello.run_id", hello, [][2]string{
		{"the client chooses a version 7 identifier once per process", `version 7[^.]* (once |each |per )[^.]*process`},
	})
	first := normalize(commentBefore(t, link, "uint64 first_event_number"))
	requireWrittenRules(t, "Hello.first_event_number", first, [][2]string{
		{"the first number is captured before append", `before[^.]*append`},
		{"the first-number range includes 1 and 2^63", `(\[1, 2\^63\]|(from |between )1 (to|through|and) 2\^63)`},
	})

	events := readLines(t, "events.proto")
	requireWrittenRules(t, "events.proto", normalize(events), [][2]string{
		{"Welcome.last_event_number is the durable replay mark", `welcome\.last_event_number[^.]*durable[^.]*mark|durable[^.]*mark[^.]*welcome\.last_event_number`},
		{"each repeat receives one cumulative acknowledgement", `repeat[^.]*\b(one|exactly one) (cumulative )?(eventack|acknowledgement)`},
		{"each eligible next event receives one cumulative acknowledgement after commit", `next event[^.]*\b(one|exactly one) (cumulative )?(eventack|acknowledgement)[^.]*after[^.]*commit`},
		{"a gap receives no acknowledgement", `gap[^.]*\b(no|none) (eventack|acknowledgement)|gap[^.]*receive none`},
		{"bad numbers, undecodable frames, failed writes and obsolete streams receive no acknowledgement",
			`(bad|invalid) number[^.]*undecodable[^.]*failed writ[^.]*(obsolete|noncurrent)[^.]*(no (eventack|acknowledgement)|receive none)`},
		{"the client may hand over up to 32 events ahead of Welcome or the latest acknowledgement",
			`(up to|at most) 32 event[^.]*ahead[^.]*welcome[^.]*(latest|last)[^.]*(acknowledg|eventack)`},
		{"an incompatible Welcome mark is at or above local next or below the retained floor minus one",
			`welcome[^.]*mark[^.]*at or above[^.]*local next[^.]*below[^.]*floor\s*(-\s*1|minus one)`},
		{"missing or corrupt required history stops replay without fabricated advancement",
			`missing or corrupt[^.]*history[^.]*stop[^.]*replay[^.]*without[^.]*(fabricat|invent|skip)[^.]*advance`},
		{"recovery uses the real prefix or removal and new enrollment",
			`(restore|recover)[^.]*real prefix[^.]*or[^.]*remov[^.]*enrol`},
		{"the complete encoded command or event budget is 49,152 bytes",
			`(command|event)[^.]*(49,?152 bytes|48 kibibytes)|(49,?152 bytes|48 kibibytes)[^.]*(command|event)`},
	})
	requireWrittenRules(t, "OpenResponse.event_ack", normalize(commentBefore(t, link, "EventAck event_ack")), [][2]string{
		{"the acknowledgement is cumulative", `cumulative|through[^.]*last[^.]*number`},
	})

	commands := readLines(t, "commands.proto")
	requireWrittenRules(t, "commands.proto", normalize(commands), [][2]string{
		{"the hourly sweep uses server-only timed_out", `(hourly|every hour)[^.]*timed_out[^.]*server|server[^.]*(hourly|every hour)[^.]*timed_out`},
		{"SetState keeps its expiry exception", `setstate[^.]*(whatever its expires_at|regardless of [^.]*expiry|without [^.]*expiry)`},
		{"first results above the mark are staged and excluded from resend and sweep",
			`stag[^.]*first[^.]*result[^.]*above[^.]*mark[^.]*resend[^.]*sweep|first[^.]*result[^.]*above[^.]*mark[^.]*stag[^.]*resend[^.]*sweep`},
		{"a covered result settles in its result write", `result[^.]*covered[^.]*mark[^.]*settle[^.]*result write`},
		{"an event write applies or consumes the report, advances the mark, then settles",
			`(appl(y|ies)|consum(e|es))[^.]*or[^.]*(appl(y|ies)|consum(e|es))[^.]*advanc[^.]*mark[^.]*(then|before)[^.]*settl`},
		{"a valid folder report in the same write wins over settlement",
			`valid folder report[^.]*same write[^.]*wins[^.]*settlement|valid folder report[^.]*wins[^.]*settlement[^.]*same write`},
		{"removal ends pending results as removed", `removal[^.]*pending result[^.]*as removed`},
		{"expiry and unsupported are client declines and clock skew can decline a ran command",
			`expired[^.]*unsupported[^.]*declin[^.]*(clock[^.]*(ran|execut))|clock[^.]*declin[^.]*(ran|execut)`},
	})
	result := normalize(commentBefore(t, commands, "message CommandResult {"))
	watermark := normalize(commentBefore(t, commands, "uint64 last_event_number"))
	requireWrittenRules(t, "CommandResult", result+" "+watermark, [][2]string{
		{"the first result captures a durable high-water, from 0 through 2^63 - 1",
			`(durable[^.]*high[- ]water[^.]*first result|first result[^.]*durable[^.]*high[- ]water)[^.]*\b0\b[^.]*2\^63\s*-\s*1`},
		{"SetState, scans, restarts and declines all retain the first watermark",
			`setstate[^.]*scan[^.]*restart[^.]*declin[^.]*(same|first|original)[^.]*watermark|(first|original)[^.]*watermark[^.]*setstate[^.]*scan[^.]*restart[^.]*declin`},
		{"replayed results retain the watermark on the current attempt",
			`replay[^.]*(same|original|first)[^.]*watermark[^.]*(current|newest)[^.]*attempt`},
		{"an interrupted result uses the last durable number of the process that began the command",
			`interrupt[^.]*last durable[^.]*number[^.]*process[^.]*(started|began)[^.]*command[^.]*restart`},
		{"legacy zero is for positively identified revision 1 receipts",
			`(positively identified|recognized)[^.]*revision 1[^.]*receipt[^.]*(zero|\b0\b)`},
		{"revision 2 zero is valid for empty history",
			`revision 2[^.]*receipt[^.]*empty history[^.]*(zero|\b0\b)`},
		{"missing or damaged revision 2 metadata is a local fault",
			`missing or (damaged|corrupt)[^.]*revision 2[^.]*metadata[^.]*local fault`},
		{"result matching still requires the current stream and latest attempt",
			`(only from|only on)[^.]*current stream[^.]*(attempt it sent last|latest attempt)`},
		{"unspecified and unknown outcomes still mean failed",
			`command_outcome_unspecified[^.]*(unknown|does not know)[^.]*command_outcome_failed`},
	})

	folders, err := os.ReadFile(filepath.Join(linkDir, "folders.proto"))
	if os.IsNotExist(err) {
		t.Error("the rules for folder reports and filtering are absent from the revision 2 contract")
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	folderText := normalize(strings.Split(string(folders), "\n"))
	requireWrittenRules(t, "the outgoing report rules", normalize(link)+" "+normalize(events)+" "+folderText, [][2]string{
		{"the shared filter covers existing Facts and harness inventory and new FolderSeen reports",
			`(shared|one|same) filter[^.]*\bfacts\b[^.]*(harnessesfound|harness inventory)[^.]*folderseen`},
		{"the filter screens the client key and connection token in every held form",
			`client key[^.]*connection token[^.]*every form`},
		{"unsafe display text is masked and unsafe workspace or identifier is omitted",
			`mask[^.]*display text[^.]*omit[^.]*workspace[^.]*identifier|display text[^.]*mask[^.]*omit[^.]*workspace[^.]*identifier`},
		{"safe legacy bytes replay unchanged and unsafe required legacy events stop without discarding bytes or numbers",
			`safe legacy[^.]*replay[^.]*unchanged[^.]*\.\s*[^.]*unsafe[^.]*legacy[^.]*stop[^.]*bytes[^.]*number`},
		{"account-relative Secure Shell origins, including URL home markers, are withheld",
			`account-relative[^.]*(secure shell|ssh)[^.]*(home marker|tilde)[^.]*remote_omitted[^.]*true`},
	})
	requireWrittenRules(t, "folders.proto", folderText, [][2]string{
		{"inspection reads only local facts without network or repository-controlled execution",
			`(read-only[^.]*local|local[^.]*read-only)[^.]*without[^.]*network[^.]*repository-controlled[^.]*(execution|program)`},
		{"an unsafe path, name, branch or commit uses the fixed path refusal",
			`unsafe[^.]*path[^.]*name[^.]*branch[^.]*commit[^.]*folder_refusal_path_not_usable`},
		{"a configured but withheld origin is absent with remote_omitted true",
			`(withheld|over-limit|unsafe)[^.]*origin[^.]*absent[^.]*remote_omitted[^.]*true`},
		{"an unconfigured origin is absent with remote_omitted false",
			`unconfigured origin[^.]*absent[^.]*remote_omitted[^.]*false`},
		{"a remote cannot be both present and omitted",
			`present remote[^.]*(flag|remote_omitted)[^.]*invalid`},
		{"origin user information is stripped before measuring and sending",
			`strip[^.]*(user information|user info|credentials)[^.]*before[^.]*(measur|budget)[^.]*send`},
		{"the resolved path is the absolute working-tree root after following links",
			`(resolved path|resolv)[^.]*absolute[^.]*(top|root)[^.]*working tree[^.]*follow[^.]*links`},
		{"the main branch uses the local remote head, then checked-out branch, then main",
			`main branch[^.]*local[^.]*remote head[^.]*checked-out branch[^.]*\bmain\b`},
		{"a folder report is committed before its result is recorded",
			`commit[^.]*folderseen[^.]*before[^.]*result`},
		{"an absent arm or zero or unknown refusal is consumed without effect",
			`absent arm[^.]*zero[^.]*unknown refusal[^.]*consum[^.]*without effect`},
	})
}
