package link

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/transport"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

const linkRevision = 2

// handshakeTimeout caps each exchange of the handshake. The tests shorten it.
var handshakeTimeout = 10 * time.Second

var (
	errServerKeyMismatch  = errors.New("link: the server did not prove its key")
	errInvalidChallenge   = errors.New("link: invalid challenge")
	errInvalidWelcome     = errors.New("link: invalid welcome")
	errInvalidCredentials = errors.New("link: invalid machine identity or keys")
	errEventGap           = errors.New("link: event gap")
	errStream             = errors.New("link: stream failed")
)

// terminalRefusal identifies an incompatible revision or a final refusal.
// Only a refusal received from a verified peer can stop reconnection.
type terminalRefusal struct {
	Code     string
	fromWire bool
}

func (e terminalRefusal) Error() string { return "link: refused: " + e.Code }

// session owns one proven stream.
type session struct {
	stream       *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse]
	cancel       context.CancelFunc
	welcome      *linkv1.Welcome
	welcomedAt   time.Time
	closeRequest func() error

	// sentAt is the elapsed time sampled immediately before sending the proof.
	sentAt   time.Duration
	sentAtOK bool

	closeOnce sync.Once
	closeErr  error
}

func (s *session) send(msg *linkv1.OpenRequest) error {
	return streamError(s.stream.Send(msg))
}

func (s *session) receive() (*linkv1.OpenResponse, error) {
	msg, err := s.stream.Receive()
	return msg, s.receiveError(err, true)
}

func (s *session) close() error {
	s.closeOnce.Do(func() {
		s.cancel()
		sendErr := streamError(s.closeRequest())
		receiveErr := streamError(s.stream.CloseResponse())
		s.closeErr = errors.Join(sendErr, receiveErr)
	})
	return s.closeErr
}

// openSession proves both ends on a new stream. Its client must be configured
// with connect.WithReadMaxBytes(64 << 10) when it is constructed.
func openSession(ctx context.Context, client linkv1connect.LinkServiceClient, server enroll.Server, credential *enroll.Credential, announcement runIdentity) (_ *session, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(server.MachineID) != 36 || len(server.PublicKey) != ed25519.PublicKeySize || credential == nil {
		return nil, errInvalidCredentials
	}
	if err := enroll.CheckFrameDebug(); err != nil {
		return nil, err
	}

	streamCtx, cancel := context.WithCancel(ctx)
	s := &session{stream: client.Open(streamCtx), cancel: cancel}
	s.closeRequest = sync.OnceValue(s.stream.CloseRequest)
	// Closing the request releases the HTTP/2 writer if it is parked reading
	// from Connect's request pipe while the response is also blocked.
	context.AfterFunc(streamCtx, func() { _ = s.closeRequest() })
	defer func() {
		if err != nil {
			_ = s.close()
		}
	}()

	var machineNonce [32]byte
	rand.Read(machineNonce[:])
	hello := &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Hello{Hello: &linkv1.Hello{
		MachineId: server.MachineID, MachineNonce: machineNonce[:], LinkRevision: linkRevision,
		RunId: announcement.id, FirstEventNumber: announcement.firstEventNumber,
	}}}
	first, err := s.exchange(streamCtx, hello, false)
	if err != nil {
		return nil, err
	}
	challenge := first.GetChallenge()
	serverNonce := challenge.GetServerNonce()
	signature := challenge.GetServerSignature()
	if len(serverNonce) != 32 || len(signature) != ed25519.SignatureSize {
		return nil, errInvalidChallenge
	}
	serverText := proofText("lgtm-link-v1 server", server.MachineID, machineNonce[:], serverNonce)
	if !ed25519.Verify(server.PublicKey, serverText, signature) {
		return nil, errServerKeyMismatch
	}

	machineText := proofText("lgtm-link-v1 machine", server.MachineID, machineNonce[:], serverNonce)
	machineSignature := credential.Sign(machineText)
	if len(machineSignature) != ed25519.SignatureSize {
		return nil, errInvalidCredentials
	}
	proof := &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Proof{Proof: &linkv1.Proof{
		MachineSignature: machineSignature,
	}}}
	second, err := s.exchange(streamCtx, proof, true)
	if err != nil {
		return nil, err
	}
	s.welcomedAt = time.Now()
	welcome := second.GetWelcome()
	if err := validateWelcome(welcome); err != nil {
		return nil, err
	}
	s.welcome = welcome
	return s, nil
}

func (s *session) exchange(ctx context.Context, msg *linkv1.OpenRequest, verified bool) (*linkv1.OpenResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// The exchange timer ends the stream only when the deadline is missed.
	// A successful stream can outlive both handshake timers.
	timer := time.AfterFunc(handshakeTimeout, s.cancel)
	if verified {
		sentAt, err := readElapsedClock()
		s.sentAt, s.sentAtOK = sentAt, err == nil
	}
	err := s.stream.Send(msg)
	var reply *linkv1.OpenResponse
	var receiveErr error
	if err == nil || errors.Is(err, io.EOF) {
		// Connect reports an early server refusal on receive, even if send
		// is the first operation to see that the stream ended.
		reply, receiveErr = s.stream.Receive()
	}
	if !timer.Stop() {
		return nil, context.DeadlineExceeded
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if receiveErr != nil {
		return reply, s.receiveError(receiveErr, verified)
	}
	return reply, streamError(err)
}

func validateWelcome(welcome *linkv1.Welcome) error {
	state := welcome.GetState()
	name := state.GetName()
	if state.GetRevision() == 0 || !utf8.ValidString(name) || strings.ContainsRune(name, 0) ||
		!welcome.GetServerTime().IsValid() {
		return errInvalidWelcome
	}
	switch state.GetApproval() {
	case linkv1.Approval_APPROVAL_APPROVED:
		token := welcome.GetConnectionToken()
		if token == "" || !utf8.ValidString(token) || strings.ContainsRune(token, 0) {
			return errInvalidWelcome
		}
	case linkv1.Approval_APPROVAL_WAITING:
		if name != "" || welcome.GetConnectionToken() != "" {
			return errInvalidWelcome
		}
	case linkv1.Approval_APPROVAL_REMOVED:
		return errInvalidWelcome
	default:
		state.Approval = linkv1.Approval_APPROVAL_WAITING
		welcome.ConnectionToken = ""
	}
	if welcome.GetLinkRevision() != linkRevision {
		return terminalRefusal{Code: "link_revision_mismatch"}
	}
	return nil
}

func (s *session) receiveError(err error, verified bool) error {
	if connect.CodeOf(err) == connect.CodeFailedPrecondition && connect.IsWireError(err) {
		// Connect's error metadata merges leading headers with trailing metadata.
		reasons := s.stream.ResponseTrailer().Values("Lgtm-Link-Refusal")
		if len(reasons) == 1 {
			switch reasons[0] {
			case "unknown_machine", "link_revision_mismatch":
				if verified {
					return terminalRefusal{Code: reasons[0], fromWire: true}
				}
			case "event_gap":
				return connect.NewError(connect.CodeFailedPrecondition, errEventGap)
			}
		}
	}
	return streamError(err)
}

func streamError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, transport.ErrCertificateKeyMismatch):
		return transport.ErrCertificateKeyMismatch
	case errors.Is(err, context.Canceled):
		return context.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		return context.DeadlineExceeded
	case errors.Is(err, io.EOF):
		return io.EOF
	}
	var remote *connect.Error
	if !errors.As(err, &remote) {
		return errStream
	}
	// Preserve the status without retaining any peer text, metadata or details.
	return connect.NewError(remote.Code(), errStream)
}

// proofText builds the signed bytes from the label, identifier and two nonces.
func proofText(label, machineID string, machineNonce, serverNonce []byte) []byte {
	text := make([]byte, 0, len(label)+2+len(machineID)+len(machineNonce)+len(serverNonce))
	text = append(text, label...)
	text = append(text, 0, byte(len(machineID)))
	text = append(text, machineID...)
	text = append(text, machineNonce...)
	return append(text, serverNonce...)
}
