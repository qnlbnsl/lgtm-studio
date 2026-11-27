package contracts_test

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// requireWebsiteFields checks consecutive tags and the full shape of each field, including presence.
func requireWebsiteFields(t *testing.T, msg protoreflect.Message, want []string) {
	t.Helper()
	fields := msg.Descriptor().Fields()
	if fields.Len() != len(want) {
		t.Errorf("%s has %d fields, want %d", msg.Descriptor().Name(), fields.Len(), len(want))
	}
	for i, expected := range want {
		tag := protoreflect.FieldNumber(i + 1)
		field := fields.ByNumber(tag)
		if field == nil {
			t.Errorf("%s has no field at tag %d, want %s", msg.Descriptor().Name(), tag, expected)
			continue
		}
		typ := field.Kind().String()
		if field.Message() != nil {
			typ = string(field.Message().FullName())
		}
		if field.Enum() != nil {
			typ = string(field.Enum().FullName())
		}
		got := string(field.Name()) + " " + typ
		if field.IsList() {
			got += " repeated"
		}
		if field.HasOptionalKeyword() {
			got += " optional"
		}
		if union := field.ContainingOneof(); union != nil && !union.IsSynthetic() {
			got += " oneof=" + string(union.Name())
		}
		if got != expected {
			t.Errorf("%s tag %d is %s, want %s", msg.Descriptor().Name(), tag, got, expected)
		}
	}
}

// oldWebsiteMessage builds a separate revision 1 descriptor from the pinned old fields.
func oldWebsiteMessage(t *testing.T, current protoreflect.Message, added protoreflect.FieldNumber) protoreflect.Message {
	t.Helper()
	msg := current.Descriptor()
	file := protodesc.ToFileDescriptorProto(msg.ParentFile())
	for _, m := range file.MessageType {
		if protoreflect.Name(m.GetName()) == msg.Name() {
			m.Field = slices.DeleteFunc(m.Field, func(fd *descriptorpb.FieldDescriptorProto) bool {
				return protoreflect.FieldNumber(fd.GetNumber()) == added
			})
		}
	}
	old, err := protodesc.NewFile(file, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatalf("cannot construct the revision 1 descriptor: %v", err)
	}
	return dynamicpb.NewMessage(old.Messages().ByName(msg.Name()))
}

func decodeWebsiteBinary(t *testing.T, msg protoreflect.Message, wire []byte, discardUnknown bool) {
	t.Helper()
	if err := (proto.UnmarshalOptions{DiscardUnknown: discardUnknown}).Unmarshal(wire, msg.Interface()); err != nil {
		t.Fatalf("cannot decode %s: %v", msg.Descriptor().Name(), err)
	}
}

func encodeWebsiteBinary(t *testing.T, msg protoreflect.Message) []byte {
	t.Helper()
	wire, err := proto.Marshal(msg.Interface())
	if err != nil {
		t.Fatal(err)
	}
	return wire
}

func TestGetServerServiceDiscoveryIsAdditive(t *testing.T) {
	reply := newMessage(t, "lgtm.v1.GetServerResponse")
	requireWebsiteFields(t, reply, []string{
		"product string", "version string", "contract uint32", "sample bool", "signed_in bool", "services string repeated",
	})
	services := messageField(t, reply, "services")
	if services.Kind() != protoreflect.StringKind || !services.IsList() {
		t.Fatal("GetServerResponse.services must be a repeated string")
	}
	old := oldWebsiteMessage(t, reply, 6)
	oldValues := func(t *testing.T, msg protoreflect.Message, sample bool) {
		t.Helper()
		for _, item := range []struct {
			field string
			want  any
		}{{"product", "lgtm-studio"}, {"version", "0.9.0"}, {"contract", uint32(1)}, {"sample", sample}, {"signed_in", true}} {
			if got := msg.Get(messageField(t, msg, item.field)).Interface(); got != item.want {
				t.Errorf("%s = %v, want %v", item.field, got, item.want)
			}
		}
	}

	oldWire := wireBytes(nil, 1, []byte("lgtm-studio"))
	oldWire = wireBytes(oldWire, 2, []byte("0.9.0"))
	oldWire = wireUint(wireUint(oldWire, 3, 1), 5, 1)
	for _, sample := range []bool{false, true} {
		t.Run(fmt.Sprintf("sample=%t", sample), func(t *testing.T) {
			binary := slices.Clone(oldWire)
			if sample {
				binary = wireUint(binary, 4, 1)
			}
			got := reply.New()
			decodeWebsiteBinary(t, got, binary, false)
			oldValues(t, got, sample)
			if got.Get(services).List().Len() != 0 || len(got.GetUnknown()) != 0 {
				t.Error("an old binary reply must advertise no services")
			}

			for _, list := range []string{"", `,"services":[]`} {
				payload := fmt.Sprintf(`{"product":"lgtm-studio","version":"0.9.0","contract":1,"sample":%t,"signedIn":true%s}`, sample, list)
				got := reply.New()
				if err := protojson.Unmarshal([]byte(payload), got.Interface()); err != nil {
					t.Fatal(err)
				}
				oldValues(t, got, sample)
				if got.Get(services).List().Len() != 0 {
					t.Error("an old or empty JSON reply must advertise no services")
				}
			}
		})
	}

	service, err := protoregistry.GlobalFiles.FindDescriptorByName("lgtm.v1.ProjectService")
	if err != nil {
		t.Fatal("ProjectService is missing from the discovery contract")
	}
	const advertised = "lgtm.v1.ProjectService"
	if _, ok := service.(protoreflect.ServiceDescriptor); !ok || string(service.FullName()) != advertised {
		t.Fatalf("the advertisement must name the service %s, got %s", advertised, service.FullName())
	}
	decodeWebsiteBinary(t, reply, oldWire, false)
	reply.Mutable(services).List().Append(protoreflect.ValueOfString(string(service.FullName())))
	decoded := wireRoundTrip(t, reply)
	list := decoded.Get(services).List()
	if list.Len() != 1 || list.Get(0).String() != advertised {
		t.Errorf("a real server must advertise %s as its sole service in this fixture", advertised)
	}
	oldValues(t, decoded, false)
	decodeWebsiteBinary(t, old, encodeWebsiteBinary(t, decoded), true)
	oldValues(t, old, false)

	raw, err := os.ReadFile("proto/lgtm/v1/session.proto")
	if err != nil {
		t.Fatal(err)
	}
	comment := normalize(commentBefore(t, strings.Split(string(raw), "\n"), "repeated string services"))
	namePattern := `(^|[^A-Za-z0-9_./])` + regexp.QuoteMeta(advertised) + `([.?!]?(\s|$)|[^A-Za-z0-9_./])`
	if !regexp.MustCompile(namePattern).MatchString(comment) {
		t.Errorf("GetServerResponse.services must document the case-sensitive advertisement %s", advertised)
	}
	requireWrittenRules(t, "GetServerResponse.services", comment, [][2]string{
		{"service names are case-sensitive and fully qualified", `case-sensitive[^.]*fully qualified|fully qualified[^.]*case-sensitive`},
		{"service names omit slashes and method suffixes", `(no|without)[^.]*slash[^.]*method suffix`},
		{"empty or omitted lists advertise no opt-in service", `empty or omitted[^.]*no opt-in service|omitted or empty[^.]*no opt-in service`},
		{"unknown entries are ignored", `unknown entr[^.]*ignor|ignor[^.]*unknown entr`},
		{"sample mode stays selected for sample=true", `sample\s*=\s*true[^.]*sample (mode|behavior)`},
	})
}

func TestClientHistoryRecoveryIsAdditive(t *testing.T) {
	client := newMessage(t, "lgtm.v1.Client")
	requireWebsiteFields(t, client, []string{
		"id string", "name string", "state lgtm.v1.ClientState", "hosts_server bool", "address string",
		"workspace string", "os string", "version string", "agent_limit uint32", "cores uint32", "memory_gigabytes uint32",
		"cpu_percent uint32 optional", "memory_percent uint32 optional", "last_seen google.protobuf.Timestamp",
		"connected_at google.protobuf.Timestamp", "harnesses_scanned bool", "harnesses lgtm.v1.InstalledHarness repeated",
		"token_id string", "event_history_needs_recovery bool",
	})
	flag := messageField(t, client, "event_history_needs_recovery")
	if flag.Kind() != protoreflect.BoolKind || flag.IsList() {
		t.Fatal("Client.event_history_needs_recovery must be a singular bool")
	}
	old := oldWebsiteMessage(t, client, 19)
	const oldJSON = `{
		"id":"0193a2b4-0000-7000-8000-000000000001","name":"machine",
		"state":"CLIENT_STATE_WAITING_FOR_APPROVAL","hostsServer":true,"address":"127.0.0.1",
		"workspace":"~/work","os":"Linux","version":"0.9.0","agentLimit":4,"cores":8,"memoryGigabytes":32,
		"cpuPercent":0,"memoryPercent":87,"lastSeen":"2026-11-27T02:03:04Z","connectedAt":"2026-11-27T02:00:00Z",
		"harnessesScanned":true,"harnesses":[{"harness":"codex","version":"1.0"}],
		"tokenId":"0193a2b4-0000-7000-8000-000000000002"
	}`
	if err := protojson.Unmarshal([]byte(oldJSON), old.Interface()); err != nil {
		t.Fatal(err)
	}
	decodeWebsiteBinary(t, client, encodeWebsiteBinary(t, old), false)
	if client.Get(flag).Bool() || client.Has(flag) || len(client.GetUnknown()) != 0 {
		t.Error("an old Client must decode with a false recovery flag")
	}
	client.Set(flag, protoreflect.ValueOfBool(true))

	for _, arm := range []string{"all", "saved"} {
		t.Run(arm, func(t *testing.T) {
			watch := fixtureMessage(t, "lgtm.v1.WatchClientsResponse", "position: {value: 11}")
			change := messageField(t, watch, arm)
			if arm == "all" {
				snapshot := watch.Mutable(change).Message()
				snapshot.Mutable(messageField(t, snapshot, "clients")).List().Append(protoreflect.ValueOfMessage(client))
			} else {
				watch.Set(change, protoreflect.ValueOfMessage(client))
			}
			record := requiredMessage(t, wireRoundTrip(t, watch), arm)
			if arm == "all" {
				list := record.Get(messageField(t, record, "clients")).List()
				if list.Len() != 1 {
					t.Fatalf("the Machines snapshot has %d clients, want 1", list.Len())
				}
				record = list.Get(0).Message()
			}
			if !record.Get(flag).Bool() {
				t.Error("the Machines watch lost the true recovery flag")
			}
			oldRecord := old.New()
			decodeWebsiteBinary(t, oldRecord, encodeWebsiteBinary(t, record), true)
			if !proto.Equal(old.Interface(), oldRecord.Interface()) {
				t.Errorf("the revision 1 descriptor lost existing values from the %s watch record", arm)
			}
		})
	}

	raw, err := os.ReadFile("proto/lgtm/v1/clients.proto")
	if err != nil {
		t.Fatal(err)
	}
	comment := normalize(commentBefore(t, strings.Split(string(raw), "\n"), "bool event_history_needs_recovery"))
	requireWrittenRules(t, "Client.event_history_needs_recovery", comment, [][2]string{
		{"refusal behind the committed mark sets the flag",
			`refus[^.]*admission[^.]*history[^.]*behind[^.]*committed mark|admission[^.]*refus[^.]*behind[^.]*committed mark`},
		{"only a later successful ready admission or removal clears the flag",
			`(clear|clears)[^.]*only[^.]*later successful[^.]*admission[^.]*ready[^.]*remov|only[^.]*later successful[^.]*admission[^.]*ready[^.]*remov[^.]*clear`},
		{"traffic from a previously admitted stream never clears the flag",
			`(previously admitted|incumbent|admitted before)[^.]*stream[^.]*(never|cannot|does not) clear|stream[^.]*(previously admitted|admitted before)[^.]*(never|cannot|does not) clear`},
	})
}
