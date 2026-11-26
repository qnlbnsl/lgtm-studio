package contracts_test

import (
	"sort"
	"strings"
	"testing"

	_ "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// waitingForTheShape is the list of Watch calls that do not follow the shared shape yet.
// The design moves them later, and the list must shrink when either one moves.
var waitingForTheShape = map[string]bool{
	"lgtm.v1.MapService.WatchMap":          true,
	"lgtm.v1.PreviewService.WatchPreviews": true,
}

// watchMethods returns every method of the package lgtm.v1 whose name starts with Watch.
func watchMethods() []protoreflect.MethodDescriptor {
	var out []protoreflect.MethodDescriptor
	protoregistry.GlobalFiles.RangeFilesByPackage("lgtm.v1", func(f protoreflect.FileDescriptor) bool {
		services := f.Services()
		for i := 0; i < services.Len(); i++ {
			methods := services.Get(i).Methods()
			for j := 0; j < methods.Len(); j++ {
				if m := methods.Get(j); strings.HasPrefix(string(m.Name()), "Watch") {
					out = append(out, m)
				}
			}
		}
		return true
	})
	sort.Slice(out, func(i, j int) bool { return out[i].FullName() < out[j].FullName() })
	return out
}

// TestEveryWatchCallHasTheSharedShape holds every Watch call to the shape of common.proto (item 3).
func TestEveryWatchCallHasTheSharedShape(t *testing.T) {
	checked := map[string]bool{}
	for _, m := range watchMethods() {
		name := string(m.FullName())
		if waitingForTheShape[name] {
			continue
		}
		checked[name] = true
		if !m.IsStreamingServer() || m.IsStreamingClient() {
			t.Errorf("%s: the server must stream and the client must not", name)
		}
		reply := m.Output()
		change := reply.Oneofs().ByName("change")
		if change == nil {
			t.Errorf("%s: the reply %s has no oneof named change", name, reply.FullName())
			continue
		}
		type want struct {
			number protoreflect.FieldNumber
			name   protoreflect.Name
			kind   protoreflect.Kind
			inOne  bool
		}
		for _, w := range []want{
			{1, "all", protoreflect.MessageKind, true},
			{2, "saved", protoreflect.MessageKind, true},
			{3, "removed_id", protoreflect.StringKind, true},
			{4, "position", protoreflect.MessageKind, false},
		} {
			f := reply.Fields().ByNumber(w.number)
			if f == nil || f.Name() != w.name {
				t.Errorf("%s: field %d must be %s", name, w.number, w.name)
				continue
			}
			if f.Kind() != w.kind {
				t.Errorf("%s: %s must be of kind %s, it is %s", name, w.name, w.kind, f.Kind())
			}
			if f.IsList() || f.IsMap() {
				t.Errorf("%s: %s must be a single field", name, w.name)
			}
			inChange := f.ContainingOneof() != nil && f.ContainingOneof().Name() == "change"
			if w.inOne && !inChange {
				t.Errorf("%s: %s must be in the oneof change", name, w.name)
			}
			if !w.inOne && f.ContainingOneof() != nil {
				t.Errorf("%s: %s must be in no oneof", name, w.name)
			}
			if w.name == "position" && f.Message() != nil && f.Message().FullName() != "lgtm.v1.FeedPosition" {
				t.Errorf("%s: position must be of type lgtm.v1.FeedPosition, it is %s", name, f.Message().FullName())
			}
		}
		if reply.Fields().Len() != 4 {
			t.Errorf("%s: the reply must have exactly four fields, it has %d", name, reply.Fields().Len())
		}
		all, saved := reply.Fields().ByName("all"), reply.Fields().ByName("saved")
		if all != nil && saved != nil && all.Message() != nil && saved.Message() != nil {
			if all.Message().Fields().Len() != 1 {
				t.Errorf("%s: the type of all must have exactly one field", name)
			} else if l := all.Message().Fields().Get(0); !l.IsList() || l.Message() == nil || l.Message().FullName() != saved.Message().FullName() {
				t.Errorf("%s: the one field of all must be a list of %s", name, saved.Message().FullName())
			}
		}
	}
	if !checked["lgtm.v1.ClientService.WatchClients"] {
		t.Errorf("lgtm.v1.ClientService.WatchClients was not among the methods checked: %v", checked)
	}
}

// TestOnlyMapAndPreviewsWaitForTheShape holds the waiting list to exactly two calls (item 3).
func TestOnlyMapAndPreviewsWaitForTheShape(t *testing.T) {
	var without []string
	for _, m := range watchMethods() {
		if m.Output().Oneofs().ByName("change") == nil {
			without = append(without, string(m.FullName()))
		}
	}
	want := "lgtm.v1.MapService.WatchMap lgtm.v1.PreviewService.WatchPreviews"
	if got := strings.Join(without, " "); got != want {
		t.Errorf("the Watch calls without a oneof change are %q, want exactly %q", got, want)
	}
	for name := range waitingForTheShape {
		d, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(name))
		if err != nil {
			t.Errorf("%s is not in the registry", name)
			continue
		}
		if o := d.(protoreflect.MethodDescriptor).Output().Oneofs().ByName("change"); o != nil {
			t.Errorf("%s has a oneof change, so it must leave the waiting list", name)
		}
	}
}
