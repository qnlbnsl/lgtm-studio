package link

import (
	"context"
	"crypto/ed25519"
	"log/slog"
	"time"

	"connectrpc.com/connect"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// These declarations are stubs of the test writer.
const (
	HandshakeTimeout = 10 * time.Second
	LinkRefusalKey   = "Lgtm-Link-Refusal"
	LinkRevision     = 1
)

type receivedFrame struct {
	request *linkv1.OpenRequest
	err     error
	at      time.Time
}

func ServerProofText(string, []byte, []byte) []byte  { return nil }
func MachineProofText(string, []byte, []byte) []byte { return nil }

func (*Service) open(context.Context, *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse], func(context.Context, *Handle, <-chan receivedFrame, *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error) error {
	return nil
}

// The following declarations are prerequisite stubs of the test writer. Replace them
// in item 5 when the streaming constructor and runner move to their planned files.
type Seams struct {
	AfterProof, AfterAdmission func(context.Context, string)
	BeforeEventAck             func(context.Context, string, uint64)
}

type Options struct {
	Logger       *slog.Logger
	PrivateStack bool
	Seams        Seams
}

func NewStreamingService(_ context.Context, s *store.Store, t *machines.Tokens, _ *machines.Registry, _ *State, _ *Hub, key ed25519.PrivateKey, _ Options) *Service {
	return NewService(s, t, key.Public().(ed25519.PublicKey))
}

func (*Service) Run(context.Context, *store.Follower) {}

// These private controls are stubs of the test writer. The handshake reads them from
// the request context in tests and uses the ordinary clock and verifier otherwise.
type handshakeControlsKey struct{}

type handshakeControls struct {
	now               func() time.Time
	timer             func(time.Duration) (<-chan time.Time, func())
	afterReceive      func(*linkv1.OpenRequest, error, time.Time)
	verify            func(ed25519.PublicKey, []byte, []byte) bool
	onQueueDisconnect func(machineID, streamID string)
}
