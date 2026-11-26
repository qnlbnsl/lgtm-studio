package machinetest

import (
	"context"
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

// These declarations are stubs of the test writer.
type Machine struct {
	ID        string
	Key       ed25519.PrivateKey
	ServerKey ed25519.PublicKey
}

func NewServer(testing.TB, http.Handler, time.Duration) *httptest.Server { return nil }
func Dial(context.Context, linkv1connect.LinkServiceClient) *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse] {
	return nil
}
func Join(context.Context, linkv1connect.LinkServiceClient, string, string) (*Machine, error) {
	return nil, nil
}
func (*Machine) Connect(context.Context, linkv1connect.LinkServiceClient) (*connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse], *linkv1.Welcome, error) {
	return nil, nil, nil
}
