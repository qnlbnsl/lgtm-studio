package api

import (
	"net/http"

	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
)

type Options struct {
	Sessions *auth.Sessions
	Origins  *auth.OriginPolicy
	Session  lgtmv1connect.SessionServiceHandler
	Clients  lgtmv1connect.ClientServiceHandler
	Link     linkv1connect.LinkServiceHandler
}

// NewHandler is a test-writer stub.
func NewHandler(options Options) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	})
}
