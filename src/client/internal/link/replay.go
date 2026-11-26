package link

import (
	"errors"
	"strings"
	"unicode/utf8"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"google.golang.org/protobuf/proto"
)

const reportVersion byte = 1

var errInvalidReport = errors.New("link: invalid report")

// replay belongs to the owner of one stream. Its ceiling is the last number
// handed to the sender, which a server can acknowledge before send returns.
type replay struct {
	store   *Store
	ceiling uint64
}

func newReplay(store *Store, through uint64) (*replay, error) {
	if err := store.Ack(through); err != nil {
		return nil, err
	}
	return &replay{store: store, ceiling: through}, nil
}

func (r *replay) next() (*linkv1.Event, error) {
	number := r.ceiling + 1
	events, err := r.store.Read(number, 1)
	if err != nil || len(events) == 0 {
		return nil, err
	}
	event := events[0]
	if event.Seq != number {
		return nil, ErrAckAhead
	}
	report, err := decodeReport(event.Payload)
	if err != nil {
		return nil, err
	}
	report.Number = event.Seq
	r.ceiling = event.Seq
	return report, nil
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

func decodeReport(data []byte) (*linkv1.Event, error) {
	if len(data) == 0 || data[0] != reportVersion {
		return nil, ErrStoreDamaged
	}
	event := new(linkv1.Event)
	if err := proto.Unmarshal(data[1:], event); err != nil || !isUnnumberedReport(event) {
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
	default:
		return false
	}
}

func validReportString(value string, limit int) bool {
	return len(value) <= limit && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}
