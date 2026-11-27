// Package machinetest provides a scripted peer for the machine link.
package machinetest

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/transport"
)

const linkRevision = 2

type Machine struct {
	ID               string
	Key              ed25519.PrivateKey
	ServerKey        ed25519.PublicKey
	RunID            string
	FirstEventNumber uint64
}

var (
	errJoinKey = errors.New("The server's signing key did not match the join token.")
	errProof   = errors.New("The server's Challenge did not verify.")
	errWelcome = errors.New("The server did not send a revision 2 Welcome.")
)

func NewServer(t testing.TB, handler http.Handler, timeout time.Duration) *httptest.Server {
	t.Helper()
	withDeadline := transport.WithTimeout(handler, timeout)
	open := transport.WithLinkStream(withDeadline, 64<<10)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == linkv1connect.LinkServiceOpenProcedure {
			open.ServeHTTP(w, r)
			return
		}
		withDeadline.ServeHTTP(w, r)
	}))
	server.Config.Protocols = new(http.Protocols)
	server.Config.Protocols.SetUnencryptedHTTP2(true)
	server.Start()
	t.Cleanup(server.Close)
	return server
}

func Dial(ctx context.Context, client linkv1connect.LinkServiceClient) *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse] {
	return client.Open(ctx)
}

func Join(ctx context.Context, client linkv1connect.LinkServiceClient, tokenText, host string) (*Machine, error) {
	token, err := machines.DecodeToken(tokenText)
	if err != nil {
		return nil, err
	}
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, errors.New("A machine signing key could not be generated.")
	}
	runID, err := store.NewID()
	if err != nil {
		return nil, err
	}
	reply, err := client.Join(ctx, connect.NewRequest(&linkv1.JoinRequest{
		TokenId: token.GetTokenId(), TokenSecret: token.GetSecret(),
		MachinePublicKey: public, HostName: host,
	}))
	if err != nil {
		return nil, err
	}
	key := reply.Msg.GetServerPublicKey()
	hash := sha256.Sum256(key)
	if len(key) != ed25519.PublicKeySize || subtle.ConstantTimeCompare(hash[:], token.GetServerKeySha256()) != 1 {
		return nil, errJoinKey
	}
	return &Machine{
		ID: reply.Msg.GetMachineId(), Key: private, ServerKey: key,
		RunID: runID, FirstEventNumber: 1,
	}, nil
}

func (m *Machine) Connect(ctx context.Context, client linkv1connect.LinkServiceClient) (*connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse], *linkv1.Welcome, error) {
	stream := Dial(ctx, client)
	welcome, err := m.handshake(stream)
	if err != nil {
		_ = stream.CloseRequest()
		_ = stream.CloseResponse()
		return nil, nil, err
	}
	return stream, welcome, nil
}

func (m *Machine) handshake(stream *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse]) (*linkv1.Welcome, error) {
	if len(m.Key) != ed25519.PrivateKeySize || len(m.ServerKey) != ed25519.PublicKeySize {
		return nil, errProof
	}
	var nonce [32]byte
	rand.Read(nonce[:])
	hello := &linkv1.Hello{
		MachineId: m.ID, MachineNonce: nonce[:], LinkRevision: linkRevision,
		RunId: m.RunID, FirstEventNumber: m.FirstEventNumber,
	}
	if err := stream.Send(&linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Hello{Hello: hello}}); err != nil {
		return nil, err
	}
	reply, err := stream.Receive()
	if err != nil {
		return nil, err
	}
	challenge := reply.GetChallenge()
	if len(challenge.GetServerNonce()) != 32 ||
		!ed25519.Verify(m.ServerKey, wireProof("server", m.ID, nonce[:], challenge.GetServerNonce()), challenge.GetServerSignature()) {
		return nil, errProof
	}
	proof := &linkv1.Proof{
		MachineSignature: ed25519.Sign(m.Key, wireProof("machine", m.ID, nonce[:], challenge.GetServerNonce())),
	}
	if err := stream.Send(&linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Proof{Proof: proof}}); err != nil {
		return nil, err
	}
	reply, err = stream.Receive()
	if err != nil {
		return nil, err
	}
	welcome := reply.GetWelcome()
	if welcome.GetLinkRevision() != linkRevision || welcome.GetState() == nil {
		return nil, errWelcome
	}
	return welcome, nil
}

// wireProof follows the frozen byte examples independently of the server.
func wireProof(role, machineID string, machineNonce, serverNonce []byte) []byte {
	prefix := []byte("lgtm-link-v1 " + role + "\x00")
	identity := append([]byte{byte(len(machineID))}, machineID...)
	return append(append(append(prefix, identity...), machineNonce...), serverNonce...)
}
