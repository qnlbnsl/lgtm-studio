package contracts_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// describe reads the description file golden and returns the description that the
// generated descriptors give for the same names (got), and the file's content (want).
// The format is the one in the plan go-s0-contract, section "The description format that
// the tests compare". A name that is not found adds the line "missing <name>" and never
// stops the test.
func describe(t *testing.T, golden string) (got, want string) {
	t.Helper()
	raw, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("cannot read the description file: %v", err)
	}
	want = string(raw)
	var b strings.Builder
	for _, line := range strings.Split(want, "\n") {
		words := strings.Fields(line)
		if len(words) < 2 || strings.HasPrefix(line, " ") {
			continue
		}
		kind, name := words[0], words[1]
		if kind != "rpc" && kind != "message" && kind != "enum" {
			continue
		}
		d, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(name))
		if err != nil {
			fmt.Fprintf(&b, "missing %s\n", name)
			continue
		}
		switch kind {
		case "rpc":
			m, ok := d.(protoreflect.MethodDescriptor)
			if !ok {
				fmt.Fprintf(&b, "missing %s\n", name)
				continue
			}
			in, out := string(m.Input().FullName()), string(m.Output().FullName())
			if m.IsStreamingClient() {
				in = "stream " + in
			}
			if m.IsStreamingServer() {
				out = "stream " + out
			}
			fmt.Fprintf(&b, "rpc %s %s %s\n", name, in, out)
		case "message":
			m, ok := d.(protoreflect.MessageDescriptor)
			if !ok {
				fmt.Fprintf(&b, "missing %s\n", name)
				continue
			}
			fmt.Fprintf(&b, "message %s\n", name)
			fields := m.Fields()
			for i := 0; i < fields.Len(); i++ {
				f := fields.Get(i)
				typ := f.Kind().String()
				if f.Message() != nil {
					typ = string(f.Message().FullName())
				} else if f.Enum() != nil {
					typ = string(f.Enum().FullName())
				}
				line := fmt.Sprintf("  %d %s %s", f.Number(), f.Name(), typ)
				if f.IsList() {
					line += " repeated"
				}
				if f.HasOptionalKeyword() {
					line += " optional"
				}
				if o := f.ContainingOneof(); o != nil && !o.IsSynthetic() {
					line += " oneof=" + string(o.Name())
				}
				b.WriteString(line + "\n")
			}
			ranges := m.ReservedRanges()
			for i := 0; i < ranges.Len(); i++ {
				r := ranges.Get(i) // the end of a message range is exclusive
				fmt.Fprintf(&b, "  reserved %d-%d\n", r[0], r[1]-1)
			}
			names := m.ReservedNames()
			for i := 0; i < names.Len(); i++ {
				fmt.Fprintf(&b, "  reserved %s\n", names.Get(i))
			}
		case "enum":
			e, ok := d.(protoreflect.EnumDescriptor)
			if !ok {
				fmt.Fprintf(&b, "missing %s\n", name)
				continue
			}
			fmt.Fprintf(&b, "enum %s\n", name)
			values := e.Values()
			for i := 0; i < values.Len(); i++ {
				v := values.Get(i)
				fmt.Fprintf(&b, "  %d %s\n", v.Number(), v.Name())
			}
			ranges := e.ReservedRanges()
			for i := 0; i < ranges.Len(); i++ {
				r := ranges.Get(i) // the end of an enum range is inclusive
				fmt.Fprintf(&b, "  reserved %d-%d\n", r[0], r[1])
			}
			names := e.ReservedNames()
			for i := 0; i < names.Len(); i++ {
				fmt.Fprintf(&b, "  reserved %s\n", names.Get(i))
			}
		}
	}
	return b.String(), want
}

// matchDescription fails the test with both texts when the descriptors differ from the file.
func matchDescription(t *testing.T, golden string) {
	t.Helper()
	got, want := describe(t, golden)
	if got != want {
		t.Errorf("the generated descriptors differ from %s\n--- got ---\n%s--- want ---\n%s", golden, got, want)
	}
}
