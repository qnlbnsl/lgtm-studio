package contracts_test

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	_ "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// TestEventsMatchItsDescription holds events.proto to its description (item 4).
func TestEventsMatchItsDescription(t *testing.T) {
	matchDescription(t, "testdata/golden/events.txt")
}

// TestCommandsMatchItsDescription holds commands.proto to its description (item 4).
func TestCommandsMatchItsDescription(t *testing.T) {
	matchDescription(t, "testdata/golden/commands.txt")
}

// newMessage makes an empty message of the named type without naming a generated Go type.
func newMessage(t *testing.T, name string) protoreflect.Message {
	t.Helper()
	mt, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(name))
	if err != nil {
		t.Fatalf("the message %s is not registered: %v", name, err)
	}
	return mt.New()
}

// TestAnUnknownActionReadsAsNoAction is how a machine recognizes a command from a newer server (item 4).
func TestAnUnknownActionReadsAsNoAction(t *testing.T) {
	msg := newMessage(t, "lgtm.link.v1.Command")
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendString(b, "c1")
	b = protowire.AppendTag(b, 2, protowire.VarintType)
	b = protowire.AppendVarint(b, 3)
	b = protowire.AppendTag(b, 99, protowire.BytesType)
	b = protowire.AppendBytes(b, nil)
	if err := proto.Unmarshal(b, msg.Interface()); err != nil {
		t.Fatalf("the command did not unmarshal: %v", err)
	}
	fields := msg.Descriptor().Fields()
	if got := msg.Get(fields.ByName("id")).String(); got != "c1" {
		t.Errorf("id = %q, want c1", got)
	}
	if got := msg.Get(fields.ByName("attempt")).Uint(); got != 3 {
		t.Errorf("attempt = %d, want 3", got)
	}
	action := msg.Descriptor().Oneofs().ByName("action")
	if action == nil {
		t.Fatalf("Command has no oneof named action")
	}
	if got := msg.WhichOneof(action); got != nil {
		t.Errorf("an unknown action set the field %s, want none", got.Name())
	}
}

// TestAnUnknownReportReadsAsNoReport is how a server recognizes an event from a newer client (item 4).
func TestAnUnknownReportReadsAsNoReport(t *testing.T) {
	msg := newMessage(t, "lgtm.link.v1.Event")
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, 7)
	b = protowire.AppendTag(b, 99, protowire.BytesType)
	b = protowire.AppendBytes(b, nil)
	if err := proto.Unmarshal(b, msg.Interface()); err != nil {
		t.Fatalf("the event did not unmarshal: %v", err)
	}
	if got := msg.Get(msg.Descriptor().Fields().ByName("number")).Uint(); got != 7 {
		t.Errorf("number = %d, want 7", got)
	}
	report := msg.Descriptor().Oneofs().ByName("report")
	if report == nil {
		t.Fatalf("Event has no oneof named report")
	}
	if got := msg.WhichOneof(report); got != nil {
		t.Errorf("an unknown report set the field %s, want none", got.Name())
	}
}

// protoName turns a field name given as text into the type that the descriptor lookup takes.
func protoName(n string) protoreflect.Name { return protoreflect.Name(n) }

func TestFoldersMatchItsDescription(t *testing.T) {
	matchDescription(t, "testdata/golden/folders.txt")
}

func messageField(t *testing.T, msg protoreflect.Message, name string) protoreflect.FieldDescriptor {
	t.Helper()
	field := msg.Descriptor().Fields().ByName(protoName(name))
	if field == nil {
		t.Fatalf("%s has no field named %s", msg.Descriptor().FullName(), name)
	}
	return field
}

func fixtureMessage(t *testing.T, name, text string) protoreflect.Message {
	t.Helper()
	msg := newMessage(t, name)
	if err := prototext.Unmarshal([]byte(text), msg.Interface()); err != nil {
		t.Fatalf("cannot populate %s: %v", name, err)
	}
	return msg
}

func wireRoundTrip(t *testing.T, msg protoreflect.Message) protoreflect.Message {
	t.Helper()
	b, err := proto.Marshal(msg.Interface())
	if err != nil {
		t.Fatal(err)
	}
	decoded := msg.New()
	if err := proto.Unmarshal(b, decoded.Interface()); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(msg.Interface(), decoded.Interface()) {
		t.Errorf("%s changed on a binary round-trip", msg.Descriptor().FullName())
	}
	return decoded
}

func wireBytes(b []byte, tag protowire.Number, value []byte) []byte {
	return protowire.AppendBytes(protowire.AppendTag(b, tag, protowire.BytesType), value)
}

func wireUint(b []byte, tag protowire.Number, value uint64) []byte {
	return protowire.AppendVarint(protowire.AppendTag(b, tag, protowire.VarintType), value)
}

func TestRevisionOnePayloadsDecodeUnderRevisionTwo(t *testing.T) {
	const pkg = "lgtm.link.v1."
	decode := func(name string, wire []byte) protoreflect.Message {
		t.Helper()
		msg := newMessage(t, pkg+name)
		if err := proto.Unmarshal(wire, msg.Interface()); err != nil {
			t.Fatalf("cannot decode the revision 1 %s: %v", name, err)
		}
		if len(msg.GetUnknown()) != 0 {
			t.Errorf("the revision 1 %s left unknown bytes: %x", name, msg.GetUnknown())
		}
		return msg
	}
	expect := func(msg protoreflect.Message, name string, want any) {
		t.Helper()
		got := msg.Get(messageField(t, msg, name)).Interface()
		if b, ok := want.([]byte); ok {
			if g, ok := got.([]byte); !ok || !bytes.Equal(g, b) {
				t.Errorf("%s.%s = %v, want %v", msg.Descriptor().Name(), name, got, want)
			}
		} else if got != want {
			t.Errorf("%s.%s = %v, want %v", msg.Descriptor().Name(), name, got, want)
		}
	}
	unset := func(msg protoreflect.Message, name string, tag protoreflect.FieldNumber, kind protoreflect.Kind) protoreflect.FieldDescriptor {
		t.Helper()
		fd := messageField(t, msg, name)
		if fd.Number() != tag || fd.Kind() != kind || fd.IsList() || fd.HasOptionalKeyword() || msg.Has(fd) {
			t.Errorf("%s.%s must be an unset singular %s at tag %d on old input", msg.Descriptor().Name(), name, kind, tag)
		}
		return fd
	}
	expectArm := func(fd protoreflect.FieldDescriptor, union, message protoreflect.FullName) {
		t.Helper()
		if fd.ContainingOneof() == nil || fd.ContainingOneof().FullName() != union ||
			fd.Message() == nil || fd.Message().FullName() != message {
			t.Errorf("%s must be an arm of %s with message type %s", fd.FullName(), union, message)
		}
	}

	nonce := bytes.Repeat([]byte{0x41}, 32)
	hello := decode("Hello", wireUint(wireBytes(wireBytes(nil, 1, []byte(proofMachineID)), 2, nonce), 3, 1))
	expect(hello, "machine_id", proofMachineID)
	expect(hello, "machine_nonce", nonce)
	expect(hello, "link_revision", uint32(1))
	unset(hello, "run_id", 4, protoreflect.StringKind)
	unset(hello, "first_event_number", 5, protoreflect.Uint64Kind)
	expect(hello, "run_id", "")
	expect(hello, "first_event_number", uint64(0))

	result := decode("CommandResult", wireUint(wireUint(wireBytes(nil, 1, []byte(proofMachineID)), 2, 3), 3, 1))
	expect(result, "command_id", proofMachineID)
	expect(result, "attempt", uint32(3))
	expect(result, "outcome", protoreflect.EnumNumber(1))
	unset(result, "last_event_number", 5, protoreflect.Uint64Kind)
	expect(result, "last_event_number", uint64(0))
	if desc := result.Descriptor(); !desc.ReservedRanges().Has(4) || !desc.ReservedNames().Has("message") {
		t.Error("CommandResult must keep tag 4 and the name message reserved")
	}

	commandWire := wireUint(wireBytes(nil, 1, []byte(proofMachineID)), 2, 3)
	commandWire = wireBytes(wireBytes(commandWire, 3, wireUint(nil, 1, 1800000000)), 10, nil)
	command := decode("Command", commandWire)
	expect(command, "id", proofMachineID)
	expect(command, "attempt", uint32(3))
	expect(command.Get(messageField(t, command, "expires_at")).Message(), "seconds", int64(1800000000))
	if scan := messageField(t, command, "scan_harnesses"); scan.ContainingOneof() == nil || command.WhichOneof(scan.ContainingOneof()) != scan {
		t.Error("the revision 1 command did not select ScanHarnesses")
	}
	expectArm(unset(command, "look_at_folder", 13, protoreflect.MessageKind), pkg+"Command.action", pkg+"LookAtFolder")

	for _, arm := range []struct {
		tag    protowire.Number
		name   string
		report []byte
	}{
		{10, "facts", wireUint(wireBytes(nil, 1, []byte("Linux")), 2, 8)},
		{11, "harnesses_found", wireBytes(nil, 1, wireBytes(wireBytes(nil, 1, []byte("codex")), 2, []byte("1.0")))},
	} {
		eventWire := wireBytes(wireUint(nil, 1, 7), 2, wireUint(nil, 1, 1700000000))
		event := decode("Event", wireBytes(eventWire, arm.tag, arm.report))
		expect(event, "number", uint64(7))
		expect(event.Get(messageField(t, event, "happened_at")).Message(), "seconds", int64(1700000000))
		field := messageField(t, event, arm.name)
		if field.ContainingOneof() == nil || event.WhichOneof(field.ContainingOneof()) != field {
			t.Fatalf("the revision 1 event did not select %s", arm.name)
		}
		report := event.Get(field).Message()
		if arm.name == "facts" {
			expect(report, "os", "Linux")
			expect(report, "cores", uint32(8))
		} else {
			list := report.Get(messageField(t, report, "harnesses")).List()
			if list.Len() != 1 {
				t.Fatalf("the old inventory has %d harnesses, want 1", list.Len())
			}
			expect(list.Get(0).Message(), "name", "codex")
			expect(list.Get(0).Message(), "version", "1.0")
		}
		expectArm(unset(event, "folder_seen", 12, protoreflect.MessageKind), pkg+"Event.report", pkg+"FolderSeen")
	}
}

func TestFolderWireBudgets(t *testing.T) {
	raw, err := os.ReadFile("testdata/link-v2-limits.json")
	if err != nil {
		t.Fatal(err)
	}
	var limits map[string]int
	if err := json.Unmarshal(raw, &limits); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		"link_message_bytes": 65536, "command_event_bytes": 49152,
		"typed_path_bytes": 4096, "resolved_path_bytes": 8192, "remote_bytes": 2048,
		"unacked_events": 32,
	}
	if !maps.Equal(limits, want) {
		t.Fatalf("link-v2-limits.json = %v, want %v", limits, want)
	}

	const pkg = "lgtm.link.v1."
	command := fixtureMessage(t, pkg+"Command", `
		id: "0193a2b4-0000-7000-8000-000000000001" attempt: 4294967295
		expires_at: {seconds: 1800000000 nanos: 123456789}
		look_at_folder: {operation_id: "0193a2b4-0000-7000-8000-000000000002" path: "/checkout"}`)
	event := fixtureMessage(t, pkg+"Event", `
		number: 9223372036854775807 happened_at: {seconds: 1799999990 nanos: 123456789}
		folder_seen: {
			operation_id: "0193a2b4-0000-7000-8000-000000000002"
			command_id: "0193a2b4-0000-7000-8000-000000000001"
			facts: {resolved_path: "/checkout" name: "checkout" main_branch: "main" first_commit: "abc123"}
		}`)
	commandBody := command.Get(messageField(t, command, "look_at_folder")).Message()
	seen := event.Get(messageField(t, event, "folder_seen")).Message()
	facts := seen.Get(messageField(t, seen, "facts")).Message()
	for _, tc := range []struct {
		key, name, prefix string
		body              protoreflect.Message
	}{
		{"typed_path_bytes", "path", "/", commandBody},
		{"resolved_path_bytes", "resolved_path", "/", facts},
		{"remote_bytes", "remote", "https://127.0.0.1/", facts},
	} {
		limit := limits[tc.key]
		field := messageField(t, tc.body, tc.name)
		remaining := limit - len(tc.prefix)
		at := tc.prefix + strings.Repeat("é", remaining/2) + strings.Repeat("a", remaining%2)
		var previousSize int
		for _, value := range []string{at, at + "a"} {
			tc.body.Set(field, protoreflect.ValueOfString(value))
			got := wireRoundTrip(t, tc.body).Get(field).String()
			if n := len(got); n != len(value) || utf8.RuneCountInString(got) >= limit || (n > limit) != (value != at) {
				t.Errorf("%s fixture has %d bytes and %d characters for the %d-byte limit", tc.name, n, utf8.RuneCountInString(got), limit)
			}
			size := proto.Size(tc.body.Interface())
			if previousSize != 0 && size != previousSize+1 {
				t.Errorf("one more byte of %s changed its encoded message size by %d bytes", tc.name, size-previousSize)
			}
			previousSize = size
		}
		tc.body.Set(field, protoreflect.ValueOfString(at))
	}

	for _, origin := range []struct {
		name              string
		present, withheld bool
	}{
		{"present", true, false}, {"absent", false, false}, {"withheld", false, true},
	} {
		t.Run(origin.name, func(t *testing.T) {
			remote := messageField(t, facts, "remote")
			if !origin.present {
				facts.Clear(remote)
			}
			facts.Set(messageField(t, facts, "remote_omitted"), protoreflect.ValueOfBool(origin.withheld))
			gotSeen := wireRoundTrip(t, event).Get(messageField(t, event, "folder_seen")).Message()
			factsField := messageField(t, gotSeen, "facts")
			if factsField.ContainingOneof() == nil || gotSeen.WhichOneof(factsField.ContainingOneof()) != factsField {
				t.Fatal("the decoded FolderSeen did not select the facts arm")
			}
			got := gotSeen.Get(factsField).Message()
			if got.Has(messageField(t, got, "remote")) != origin.present ||
				got.Get(messageField(t, got, "remote_omitted")).Bool() != origin.withheld {
				t.Error("the decoded origin lost its presence or omission flag")
			}
			if got.Has(messageField(t, got, "checked_out_branch")) || got.Get(messageField(t, got, "clean")).Bool() {
				t.Error("the decoded checkout must be detached and dirty")
			}
		})
	}

	refusal := fixtureMessage(t, pkg+"FolderSeen", `
		operation_id: "0193a2b4-0000-7000-8000-000000000002"
		command_id: "0193a2b4-0000-7000-8000-000000000001"
		refusal: FOLDER_REFUSAL_PATH_NOT_USABLE`)
	refusedEvent := event.New()
	proto.Merge(refusedEvent.Interface(), event.Interface())
	refusedEvent.Set(messageField(t, refusedEvent, "folder_seen"), protoreflect.ValueOfMessage(refusal))
	gotRefusal := wireRoundTrip(t, refusedEvent).Get(messageField(t, refusedEvent, "folder_seen")).Message()
	refusalField := messageField(t, gotRefusal, "refusal")
	if refusalField.ContainingOneof() == nil || gotRefusal.WhichOneof(refusalField.ContainingOneof()) != refusalField ||
		gotRefusal.Get(refusalField).Enum() != 4 {
		t.Error("the decoded FolderSeen did not select the fixed refusal arm at value 4")
	}

	for _, tc := range []struct {
		inner             protoreflect.Message
		outerName, member string
	}{
		{command, "OpenResponse", "command"},
		{event, "OpenRequest", "event"},
	} {
		t.Run(tc.member, func(t *testing.T) {
			outer := newMessage(t, pkg+tc.outerName)
			payload := messageField(t, outer, tc.member)
			outer.Set(payload, protoreflect.ValueOfMessage(tc.inner))
			for _, extra := range []int{0, 1} {
				padWireMessage(t, tc.inner, limits["command_event_bytes"]+extra)
				if n := proto.Size(outer.Interface()); n <= limits["command_event_bytes"]+extra || n >= limits["link_message_bytes"] {
					t.Errorf("encoded %s with its payload must include wrapper overhead and fit the outer limit, got %d", tc.outerName, n)
				}
				wireRoundTrip(t, outer)
			}
			padWireMessage(t, tc.inner, limits["command_event_bytes"])
			for _, extra := range []int{0, 1} {
				padWireMessage(t, outer, limits["link_message_bytes"]+extra)
			}
		})
	}
}

// padWireMessage uses an unknown field so fixtures can reach an aggregate limit without breaking a text limit.
func padWireMessage(t *testing.T, msg protoreflect.Message, target int) {
	t.Helper()
	msg.SetUnknown(nil)
	const tag = 100
	budget := target - proto.Size(msg.Interface()) - protowire.SizeTag(tag)
	padding := budget - protowire.SizeVarint(uint64(budget))
	if padding <= 0 {
		t.Fatalf("%s cannot be padded to %d bytes", msg.Descriptor().Name(), target)
	}
	msg.SetUnknown(wireBytes(nil, tag, make([]byte, padding)))
	wire, err := proto.Marshal(msg.Interface())
	if err != nil {
		t.Fatal(err)
	}
	if len(wire) != target || proto.Size(msg.Interface()) != target {
		t.Errorf("%s encodes to %d bytes, want exactly %d", msg.Descriptor().Name(), len(wire), target)
	}
	wireRoundTrip(t, msg)
}
