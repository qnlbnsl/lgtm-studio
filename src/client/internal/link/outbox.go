package link

// This file is a stub of the test writer. The builder replaces every body.

// Event is one numbered event waiting to be sent.
type Event struct {
	Seq     uint64
	Payload []byte
}

// Append stores a payload under the next number and returns that number after the commit.
func (s *Store) Append(payload []byte) (uint64, error) { return 0, nil }

// Read returns, in order, at most limit stored events whose number is at least from.
func (s *Store) Read(from uint64, limit int) ([]Event, error) { return nil, nil }

// Ack removes every stored event up to and including n.
func (s *Store) Ack(n uint64) error { return nil }

// Next is the number the next event gets.
func (s *Store) Next() uint64 { return 0 }

// Len is the number of stored events.
func (s *Store) Len() int { return 0 }

// Bytes is the sum of the lengths of the stored payloads.
func (s *Store) Bytes() int64 { return 0 }

// Changed signals without blocking after each append, and is closed by Close.
func (s *Store) Changed() <-chan struct{} { return nil }
