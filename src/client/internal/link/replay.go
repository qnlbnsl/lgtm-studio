package link

import linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"

// replay is a stub of the test writer.
type replay struct{}

// newReplay is a stub of the test writer.
func newReplay(_ *Store, _ uint64) (*replay, error) { return &replay{}, nil }

// next is a stub of the test writer.
func (*replay) next() (*linkv1.Event, error) { return nil, nil }

// ack is a stub of the test writer.
func (*replay) ack(_ uint64) error { return nil }

// encodeReport is a stub of the test writer.
func encodeReport(_ *linkv1.Event) ([]byte, error) { return nil, nil }
