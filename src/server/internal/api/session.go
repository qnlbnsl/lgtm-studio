package api

import (
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
)

const Product = "lgtm-studio"
const ContractRevision uint32 = 1
const Version = "0.1.0"

// NewSessionService is a test-writer stub.
func NewSessionService(password auth.Password, sessions *auth.Sessions) lgtmv1connect.SessionServiceHandler {
	return lgtmv1connect.UnimplementedSessionServiceHandler{}
}
