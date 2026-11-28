package api

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// Product names the program that answers GetServer.
const Product = "lgtm-studio"

// ContractRevision is the revision of the web contract that this server speaks.
const ContractRevision uint32 = 1

// Version is the server's own version.
const Version = "0.1.0"

// projectDiscoveryKey marks GetServer calls on a router that serves Projects.
type projectDiscoveryKey struct{}

type sessionService struct {
	password auth.Password
	sessions *auth.Sessions
}

// NewSessionService implements the generated Session service.
// The router selects the cookie mode, and it verifies the session before SignOut runs.
func NewSessionService(password auth.Password, sessions *auth.Sessions) lgtmv1connect.SessionServiceHandler {
	return &sessionService{password: password, sessions: sessions}
}

// GetServer describes the server and reports whether the caller's cookie holds a live session.
// It ignores a request key.
func (s *sessionService) GetServer(ctx context.Context, req *connect.Request[webv1.GetServerRequest]) (*connect.Response[webv1.GetServerResponse], error) {
	signedIn, err := s.signedIn(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	var services []string
	if mounted, _ := ctx.Value(projectDiscoveryKey{}).(bool); mounted {
		services = []string{lgtmv1connect.ProjectServiceName}
	}
	return connect.NewResponse(&webv1.GetServerResponse{
		Product:  Product,
		Version:  Version,
		Contract: ContractRevision,
		Sample:   false,
		SignedIn: signedIn,
		Services: services,
	}), nil
}

// signedIn verifies an optional cookie. Without a canonical cookie it sends no query.
func (s *sessionService) signedIn(ctx context.Context, header http.Header) (bool, error) {
	mode, ok := cookieMode(ctx)
	if !ok {
		return false, nil
	}
	verifier, ok := cookieVerifier(&http.Request{Header: header}, mode)
	if !ok {
		return false, nil
	}
	err := s.sessions.Check(ctx, verifier)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, auth.ErrNoSession):
		return false, nil
	default:
		return false, publicError(err)
	}
}

// SignIn compares the password from the decoded body and issues a session cookie only after commit.
func (s *sessionService) SignIn(ctx context.Context, req *connect.Request[webv1.SignInRequest]) (*connect.Response[webv1.SignInResponse], error) {
	if hasRequestKey(req.Header()) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New(messageRequestKey))
	}
	mode, ok := cookieMode(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New(messageInternal))
	}
	if !s.password.Matches(req.Msg.GetPassword()) {
		return connect.NewResponse(&webv1.SignInResponse{Result: webv1.SignInResult_SIGN_IN_RESULT_REJECTED}), nil
	}
	token, err := s.sessions.Issue(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	res := connect.NewResponse(&webv1.SignInResponse{Result: webv1.SignInResult_SIGN_IN_RESULT_SIGNED_IN})
	res.Header().Add("Set-Cookie", auth.SessionCookie(token, mode).String())
	return res, nil
}

// SignOut revokes the session that the router verified and clears its cookie only after commit.
func (s *sessionService) SignOut(ctx context.Context, req *connect.Request[webv1.SignOutRequest]) (*connect.Response[webv1.SignOutResponse], error) {
	verifier, ok := SessionVerifier(ctx)
	mode, hasMode := cookieMode(ctx)
	if !ok || !hasMode {
		return nil, publicError(auth.ErrNoSession)
	}
	if hasRequestKey(req.Header()) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New(messageRequestKey))
	}
	if err := s.sessions.Revoke(ctx, verifier); err != nil {
		return nil, publicError(err)
	}
	res := connect.NewResponse(&webv1.SignOutResponse{})
	res.Header().Add("Set-Cookie", auth.ClearSessionCookie(mode).String())
	return res, nil
}

// hasRequestKey reports any request key field, including an empty or repeated one.
func hasRequestKey(header http.Header) bool {
	_, present := header[http.CanonicalHeaderKey(store.RequestKeyHeader)]
	return present
}
