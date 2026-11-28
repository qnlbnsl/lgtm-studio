package link

import (
	"bytes"
	"errors"
	"strings"
	"unicode/utf8"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"
)

const (
	reportVersion    byte = 1
	eventWireLimit        = 48 << 10
	linkMessageLimit      = 64 << 10
)

var errInvalidReport = errors.New("link: invalid report")

// replay belongs to the owner of one stream. Its ceiling is the last number
// handed to the sender, which a server can acknowledge before send returns.
type replay struct {
	store   *Store
	filter  *OutgoingFilter
	ceiling uint64
}

func newReplay(store *Store, through uint64) (*replay, error) {
	return newReplayWithFilter(store, through, nil)
}

func newReplayWithFilter(store *Store, through uint64, filter *OutgoingFilter) (*replay, error) {
	unlock := filter.lockSecrets()
	defer unlock()
	if err := store.validateReplay(through, filter); err != nil {
		return nil, err
	}
	if err := store.Ack(through); err != nil {
		return nil, err
	}
	return &replay{store: store, filter: filter, ceiling: through}, nil
}

func (s *Store) validateReplay(through uint64, filter *OutgoingFilter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return err
	}
	floor := s.events.next - uint64(s.events.count)
	if through >= s.events.next || through < floor-1 {
		return ErrAckAhead
	}
	for first := through + 1; first < s.events.next; first += maxUnackedEvents {
		if err := s.validateReplayBatch(first, maxUnackedEvents, filter); err != nil {
			return err
		}
	}
	return nil
}

// validateReplayBatch checks consecutive rows with the store and filter locked.
func (s *Store) validateReplayBatch(first, limit uint64, filter *OutgoingFilter) error {
	return s.db.View(func(tx *bolt.Tx) error {
		cursor := tx.Bucket(eventsBucket).Cursor()
		start := numberKey(first)
		key, payload := cursor.Seek(start[:])
		end := min(first+limit, s.events.next)
		for number := first; number < end; number++ {
			expected := numberKey(number)
			if !bytes.Equal(key, expected[:]) {
				return ErrStoreDamaged
			}
			event, err := decodeNumberedReport(number, payload)
			if err != nil || !filter.reportTextSafe(event.ProtoReflect()) {
				return ErrStoreDamaged
			}
			key, payload = cursor.Next()
		}
		return nil
	})
}

// next checks the next event under the caller's filter lock through handoff.
func (r *replay) next() (*linkv1.Event, error) {
	number := r.ceiling + 1
	rows, err := r.store.Read(number, 1)
	if err == nil && len(rows) == 0 && number < r.store.Next() {
		// The next number may have advanced since the read.
		rows, err = r.store.Read(number, 1)
		if err == nil && len(rows) == 0 {
			return nil, ErrStoreDamaged
		}
	}
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	if rows[0].Seq != number {
		return nil, ErrStoreDamaged
	}
	event, err := decodeNumberedReport(number, rows[0].Payload)
	if err != nil || !r.filter.reportTextSafe(event.ProtoReflect()) {
		return nil, ErrStoreDamaged
	}
	r.ceiling = number
	return event, nil
}

func (r *replay) ack(through uint64) error {
	if through > r.ceiling {
		return ErrAckAhead
	}
	return r.store.Ack(through)
}

func encodeReport(event *linkv1.Event) ([]byte, error) {
	if !isUnnumberedReport(event) {
		return nil, errInvalidReport
	}
	data, err := (proto.MarshalOptions{}).MarshalAppend([]byte{reportVersion}, event)
	if err != nil {
		return nil, errInvalidReport
	}
	return data, nil
}

func decodeNumberedReport(number uint64, data []byte) (*linkv1.Event, error) {
	report, err := decodeReport(data)
	if err != nil {
		return nil, err
	}
	report.Number = number
	if proto.Size(report) > eventWireLimit {
		return nil, ErrStoreDamaged
	}
	return report, nil
}

func decodeReport(data []byte) (*linkv1.Event, error) {
	event, err := decodeReportCandidate(data)
	if err != nil || !isUnnumberedReport(event) {
		return nil, ErrStoreDamaged
	}
	return event, nil
}

func decodeReportCandidate(data []byte) (*linkv1.Event, error) {
	if len(data) == 0 || data[0] != reportVersion {
		return nil, ErrStoreDamaged
	}
	event := new(linkv1.Event)
	if err := proto.Unmarshal(data[1:], event); err != nil {
		return nil, ErrStoreDamaged
	}
	return event, nil
}

func isUnnumberedReport(event *linkv1.Event) bool {
	if event.GetNumber() != 0 || !event.GetHappenedAt().IsValid() {
		return false
	}
	switch report := event.GetReport().(type) {
	case *linkv1.Event_Facts:
		if report == nil {
			return false
		}
		facts := report.Facts
		return facts.GetCores() > 0 && validReportString(facts.GetOs(), 64) &&
			validReportString(facts.GetClientVersion(), 64) && validReportString(facts.GetWorkspace(), 256)
	case *linkv1.Event_HarnessesFound:
		if report == nil || report.HarnessesFound == nil {
			return false
		}
		rows := report.HarnessesFound.GetHarnesses()
		if len(rows) > 16 {
			return false
		}
		for _, row := range rows {
			switch row.GetName() {
			case "claude", "codex", "gemini", "cursor":
			default:
				return false
			}
			if !validReportString(row.GetVersion(), 64) {
				return false
			}
		}
		return true
	case *linkv1.Event_FolderSeen:
		return report != nil && validFolderReport(report.FolderSeen)
	default:
		return false
	}
}

func validFolderReport(seen *linkv1.FolderSeen) bool {
	if !validLinkCommandID(seen.GetOperationId()) || !validLinkCommandID(seen.GetCommandId()) {
		return false
	}
	switch result := seen.GetResult().(type) {
	case *linkv1.FolderSeen_Facts:
		if result == nil || result.Facts == nil {
			return false
		}
		facts := result.Facts
		return facts.GetResolvedPath() != "" && validReportString(facts.GetResolvedPath(), 8192) &&
			facts.GetName() != "" && validReportText(facts.GetName()) &&
			facts.GetMainBranch() != "" && validReportText(facts.GetMainBranch()) &&
			optionalReportText(facts.CheckedOutBranch) && optionalReportText(facts.FirstCommit) &&
			validOptionalReportString(facts.Remote, 2048) && (facts.Remote == nil || !facts.GetRemoteOmitted())
	case *linkv1.FolderSeen_Refusal:
		return result != nil && result.Refusal >= linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_FOLDER &&
			result.Refusal <= linkv1.FolderRefusal_FOLDER_REFUSAL_PATH_NOT_USABLE
	default:
		return false
	}
}

func validOptionalReportString(value *string, limit int) bool {
	return value == nil || *value != "" && validReportString(*value, limit)
}

func optionalReportText(value *string) bool {
	return value == nil || *value != "" && validReportText(*value)
}

func validReportString(value string, limit int) bool {
	return len(value) <= limit && validReportText(value)
}

func validReportText(value string) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}
