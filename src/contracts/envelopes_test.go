package contracts_test

import (
	"testing"

	_ "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
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
