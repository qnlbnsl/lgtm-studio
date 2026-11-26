package servertest

import (
	"crypto/ed25519"
	"net/http"
	"testing"
)

// These are the test writer's stubs for the public server fixture.
type Options struct {
	Password string
}

type Server struct {
	URL              string
	HTTPClient       *http.Client
	SigningPublicKey ed25519.PublicKey
}

func Start(testing.TB, Options) *Server        { return &Server{} }
func (*Server) Stop(testing.TB)                {}
func (*Server) Restart(testing.TB)             {}
func (*Server) Environment() map[string]string { return nil }
