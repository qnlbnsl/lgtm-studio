package api

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

type verifierKey struct{}

type cookieModeKey struct{}

// SessionVerifier returns the digest of the session that the guard verified for this call.
// The raw credential never enters the context.
func SessionVerifier(ctx context.Context) (auth.Verifier, bool) {
	v, ok := ctx.Value(verifierKey{}).(auth.Verifier)
	return v, ok
}

func withVerifier(ctx context.Context, v auth.Verifier) context.Context {
	return context.WithValue(ctx, verifierKey{}, v)
}

// cookieMode returns the cookie mode that the origin gate selected for this call.
func cookieMode(ctx context.Context) (auth.CookieMode, bool) {
	mode, ok := ctx.Value(cookieModeKey{}).(auth.CookieMode)
	return mode, ok && mode.Name != ""
}

func withCookieMode(ctx context.Context, mode auth.CookieMode) context.Context {
	return context.WithValue(ctx, cookieModeKey{}, mode)
}

// sessionGate verifies the session of a protected procedure inside the protocol deadline.
// It stores only the verified digest in the context of the call.
func (r *router) sessionGate(ctx context.Context, spec connect.Spec, _ connect.Peer, header http.Header) (context.Context, error) {
	if publicProcedures[spec.Procedure] {
		return nil, nil
	}
	mode, _ := cookieMode(ctx)
	verifier, err := r.checkSession(ctx, header, mode)
	if err != nil {
		return nil, err
	}
	return withVerifier(ctx, verifier), nil
}

// checkSession reads the cookie of the selected mode and verifies its session.
func (r *router) checkSession(ctx context.Context, header http.Header, mode auth.CookieMode) (auth.Verifier, *connect.Error) {
	verifier, ok := cookieVerifier(&http.Request{Header: header}, mode)
	if !ok {
		return auth.Verifier{}, publicError(auth.ErrNoSession)
	}
	if r.sessions == nil {
		return auth.Verifier{}, connect.NewError(connect.CodeUnavailable, errors.New(messageUnavailable))
	}
	if err := r.sessions.Check(ctx, verifier); err != nil {
		return auth.Verifier{}, publicError(err)
	}
	return verifier, nil
}

// cookieVerifier returns the digest of the one canonical session cookie of the selected mode.
func cookieVerifier(req *http.Request, mode auth.CookieMode) (auth.Verifier, bool) {
	token, ok := auth.ReadCookie(req, mode)
	if !ok {
		return auth.Verifier{}, false
	}
	return auth.TokenVerifier(token)
}

const (
	messageNoSession   = "Sign in to continue."
	messageUnknown     = "The database did not confirm whether the write committed."
	messageBusy        = "The database was busy, retry the request."
	messageClock       = "The server could not issue a session, retry sign-in."
	messageUnavailable = "Web session storage is unavailable."
	messageCanceled    = "The request was canceled."
	messageTimedOut    = "The request timed out."
	messageInternal    = "The server could not complete the request."
	messageRequestKey  = "This call does not accept a request key."
)

// publicError maps a session or storage failure to a fixed public error.
// It checks auth's sentinels first, then a Store error's outcome before its retry class.
func publicError(err error) *connect.Error {
	code, message := connect.CodeInternal, messageInternal
	var storeErr *store.Error
	switch {
	case errors.Is(err, auth.ErrNoSession):
		code, message = connect.CodeUnauthenticated, messageNoSession
	case errors.Is(err, auth.ErrSessionClock):
		code, message = connect.CodeAborted, messageClock
	case errors.Is(err, context.Canceled):
		code, message = connect.CodeCanceled, messageCanceled
	case errors.Is(err, context.DeadlineExceeded):
		code, message = connect.CodeDeadlineExceeded, messageTimedOut
	case errors.As(err, &storeErr):
		switch {
		case storeErr.Outcome() == store.OutcomeUnknown:
			code, message = connect.CodeUnavailable, messageUnknown
		case storeErr.Retryable():
			code, message = connect.CodeAborted, messageBusy
		default:
			code, message = connect.CodeUnavailable, messageUnavailable
		}
	}
	return connect.NewError(code, errors.New(message))
}
