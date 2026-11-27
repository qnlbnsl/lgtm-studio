package link

import (
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const redactedText = "[redacted]"

// OutgoingFilter retains known secret spellings for the life of the process.
type OutgoingFilter struct {
	mu      sync.RWMutex
	secrets map[string]struct{}
	refresh func() []string
}

// NewOutgoingFilter starts with no known secrets.
func NewOutgoingFilter() *OutgoingFilter { return &OutgoingFilter{} }

// NewRefreshingOutgoingFilter reads more secret spellings at each output gate.
// The reader must return the spellings without calling back into the filter.
func NewRefreshingOutgoingFilter(read func() []string) *OutgoingFilter {
	return &OutgoingFilter{refresh: read}
}

// AddSecrets registers additional spellings without forgetting earlier values.
func (f *OutgoingFilter) AddSecrets(values ...string) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.addSecrets(values)
}

func (f *OutgoingFilter) addSecrets(values []string) {
	for _, value := range values {
		if value == "" {
			continue
		}
		if f.secrets == nil {
			f.secrets = make(map[string]struct{})
		}
		f.secrets[value] = struct{}{}
	}
}

// MaskText replaces known secrets in a complete display string.
func (f *OutgoingFilter) MaskText(text string) string {
	unlock := f.lockSecrets()
	defer unlock()
	masked, _ := f.maskPrefix(text, true)
	return masked
}

// ContainsSecret reports whether an exact value contains any known spellings.
func (f *OutgoingFilter) ContainsSecret(text string) bool {
	unlock := f.lockSecrets()
	defer unlock()
	return f.containsSecret(text)
}

// WithTextMask refreshes and classifies all strings in one record together.
// The callback uses the mask only during the call.
func (f *OutgoingFilter) WithTextMask(screen func(mask func(string) string)) {
	unlock := f.lockSecrets()
	defer unlock()
	screen(func(text string) string {
		text, _ = f.maskPrefix(text, true)
		text = strings.ToValidUTF8(text, "\uFFFD")
		if f.containsSecret(text) {
			text, _ = f.maskPrefix(text, true)
			if f.containsSecret(text) {
				return ""
			}
		}
		return text
	})
}

func (f *OutgoingFilter) lockSecrets() func() {
	if f == nil {
		return func() {}
	}
	if f.refresh != nil {
		f.mu.Lock()
		f.addSecrets(f.refresh())
		return f.mu.Unlock
	}
	f.mu.RLock()
	return f.mu.RUnlock
}

func (f *OutgoingFilter) containsSecret(text string) bool {
	if f != nil {
		for secret := range f.secrets {
			if strings.Contains(text, secret) {
				return true
			}
		}
	}
	return false
}

// TextStream masks pieces of one text in order. The caller owns the stream.
type TextStream struct {
	filter  *OutgoingFilter
	pending string
}

// NewTextStream starts a text stream with access to current and future registrations.
func (f *OutgoingFilter) NewTextStream() *TextStream { return &TextStream{filter: f} }

// Push returns the safe prefix and keeps any incomplete character or possible match.
func (s *TextStream) Push(piece []byte) string {
	unlock := s.filter.lockSecrets()
	defer unlock()
	text, suffix := s.filter.maskPrefix(s.pending+string(piece), false)
	s.pending = strings.Clone(suffix)
	return text
}

// Flush returns the remainder of the text and clears the pending suffix.
func (s *TextStream) Flush() string {
	text := s.filter.MaskText(s.pending)
	s.pending = ""
	return text
}

// maskPrefix uses the longest complete match at each position. A chunk may end
// with a prefix of a longer secret even when a shorter match is already known.
func (f *OutgoingFilter) maskPrefix(text string, final bool) (string, string) {
	var secrets map[string]struct{}
	if f != nil {
		secrets = f.secrets
	}
	if final && len(secrets) == 0 {
		return text, ""
	}
	var out strings.Builder
	start, pos := 0, 0
	for pos < len(text) {
		tail := text[pos:]
		if !final && !utf8.FullRuneInString(tail) {
			break
		}
		size := 0
		for secret := range secrets {
			if len(secret) > len(tail) {
				if !final && strings.HasPrefix(secret, tail) {
					out.WriteString(text[start:pos])
					return out.String(), tail
				}
			} else if len(secret) > size && strings.HasPrefix(tail, secret) {
				size = len(secret)
			}
		}
		if size > 0 {
			out.WriteString(text[start:pos])
			out.WriteString(redactedText)
			pos += size
			start = pos
		} else {
			pos++
		}
	}
	if start == 0 {
		return text[:pos], text[pos:]
	}
	out.WriteString(text[start:pos])
	return out.String(), text[pos:]
}

func (f *OutgoingFilter) folderPathUnsafe(path, home string) bool {
	unlock := f.lockSecrets()
	defer unlock()
	if f.containsSecret(path) {
		return true
	}
	// Use the same expansion as folder discovery and keep parent components intact.
	return (path == "~" || strings.HasPrefix(path, "~/")) && f.containsSecret(home+path[1:])
}

// encodeFilteredReport prepares a fresh candidate without changing the pending
// snapshot. The caller holds the registration lock through durable append.
func (f *OutgoingFilter) encodeFilteredReport(event *linkv1.Event) ([]byte, error) {
	event = proto.CloneOf(event)
	switch report := event.GetReport().(type) {
	case *linkv1.Event_Facts:
		facts := report.Facts
		if facts == nil {
			return nil, errInvalidReport
		}
		facts.Os = f.displayText(facts.GetOs(), 64)
		facts.ClientVersion = f.displayText(facts.GetClientVersion(), 64)
		if f.containsSecret(facts.GetWorkspace()) {
			facts.Workspace = ""
		}
	case *linkv1.Event_HarnessesFound:
		inventory := report.HarnessesFound
		if inventory == nil {
			return nil, errInvalidReport
		}
		inventory.Harnesses = slices.DeleteFunc(inventory.GetHarnesses(), func(row *linkv1.Harness) bool {
			return f.containsSecret(row.GetName())
		})
		for _, row := range inventory.GetHarnesses() {
			if row != nil && row.Version != nil {
				version := f.displayText(row.GetVersion(), 64)
				row.Version = nil
				if version != "" {
					row.Version = proto.String(version)
				}
			}
		}
	case *linkv1.Event_FolderSeen:
		seen := report.FolderSeen
		if seen == nil {
			return nil, errInvalidReport
		}
		if f.containsSecret(seen.GetCommandId()) || f.containsSecret(seen.GetOperationId()) {
			return nil, ErrInvalidCommand
		}
		if facts := seen.GetFacts(); facts != nil {
			switch {
			case f.containsSecret(facts.GetResolvedPath()), f.containsSecret(facts.GetName()),
				f.containsSecret(facts.GetMainBranch()), f.containsSecret(facts.GetCheckedOutBranch()),
				f.containsSecret(facts.GetFirstCommit()):
				seen.Result = &linkv1.FolderSeen_Refusal{Refusal: linkv1.FolderRefusal_FOLDER_REFUSAL_PATH_NOT_USABLE}
			case f.containsSecret(facts.GetRemote()):
				facts.Remote, facts.RemoteOmitted = nil, true
			}
		}
	default:
		return nil, errInvalidReport
	}
	if !f.reportTextSafe(event.ProtoReflect()) {
		return nil, errInvalidReport
	}
	return encodeReport(event)
}

func (f *OutgoingFilter) displayText(text string, limit int) string {
	masked, _ := f.maskPrefix(normalizeReportString(text), true)
	text = truncateReportString(masked, limit)
	if f.containsSecret(text) {
		// The marker and adjacent text can themselves form a registered value.
		return ""
	}
	return text
}

// reportTextSafe screens decoded values and rejects fields whose contents the
// current contract cannot classify. Replay uses it without modifying a report.
func (f *OutgoingFilter) reportTextSafe(message protoreflect.Message) bool {
	if len(message.GetUnknown()) != 0 {
		return false
	}
	safe := true
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		switch {
		case field.IsMap():
			safe = false
		case field.IsList():
			if field.Kind() != protoreflect.MessageKind {
				safe = false
				break
			}
			list := value.List()
			for i := 0; safe && i < list.Len(); i++ {
				safe = f.reportTextSafe(list.Get(i).Message())
			}
		case field.Kind() == protoreflect.MessageKind:
			safe = f.reportTextSafe(value.Message())
		case field.Kind() == protoreflect.StringKind:
			safe = !f.containsSecret(value.String())
		case field.Kind() == protoreflect.BytesKind || field.Kind() == protoreflect.GroupKind:
			safe = false
		}
		return safe
	})
	return safe
}
