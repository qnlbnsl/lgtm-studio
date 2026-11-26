package link

// This file is a stub of the test writer. The builder of work item 3 replaces every body
// and keeps the names, the signatures and the hook below.

import (
	"context"
	"errors"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

// handshakeTimeout caps each exchange of the handshake. The tests shorten it. Stub of the
// test writer.
var handshakeTimeout = 10 * time.Second

// errServerKeyMismatch reports a challenge that the saved server key did not sign.
var errServerKeyMismatch = errors.New("link: the server did not prove its key")

// terminalRefusal is a final refusal of the server with a fixed code. The code is
// unknown_machine or link_revision_mismatch.
type terminalRefusal struct{ Code string }

func (e terminalRefusal) Error() string { return "link: refused: " + e.Code }

// session is one proven stream. Stub of the test writer.
type session struct {
	// welcome is the validated welcome. Its connection token is empty unless approval is
	// approved.
	welcome *linkv1.Welcome
	// sentAt is the elapsed-clock reading taken immediately before the proof was sent.
	// sentAtOK is false when the reading failed.
	sentAt   time.Duration
	sentAtOK bool
}

func (s *session) send(*linkv1.OpenRequest) error { return errors.New("stub") }

func (s *session) receive() (*linkv1.OpenResponse, error) { return nil, errors.New("stub") }

func (s *session) close() error { return nil }

// openSession dials the stream and proves both ends. Stub of the test writer.
func openSession(ctx context.Context, client linkv1connect.LinkServiceClient, server enroll.Server, credential *enroll.Credential) (*session, error) {
	return nil, errors.New("stub")
}

// proofText builds the text that one side signs. The label is "lgtm-link-v1 server" or
// "lgtm-link-v1 machine". Stub of the test writer.
func proofText(label, machineID string, machineNonce, serverNonce []byte) []byte { return nil }
