// Package servertest starts the real server application in process, for tests outside the server's internal packages.
// Each fixture has its own disposable PostgreSQL database and its own loopback OpenBao fixture.
package servertest

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	lgtmv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/app"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/config"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/vault/vaulttest"
)

// Options configures a fixture.
type Options struct {
	// Password is the website password the server starts with.
	Password string
	// Log receives application diagnostics in JavaScript Object Notation when set.
	Log io.Writer
	// LinkSeams lets tests observe link phases on start and after restart.
	LinkSeams LinkSeams
	// ConfigurePostgres configures a fresh copy of the pool settings before each launch.
	ConfigurePostgres func(*pgxpool.Config)
}

// LinkSeams lets tests observe link phases.
type LinkSeams struct {
	AfterProof, AfterAdmission func(context.Context, string)
	// BeforeEventAck observes each committed mark advance before the acknowledgement.
	BeforeEventAck func(context.Context, string, uint64)
	// BeforeEventCommit can fail an event write after report handling and before the mark advances.
	BeforeEventCommit func(context.Context, string, uint64) error
	// BeforeCommandSend runs after the command claim commits and before the send begins.
	BeforeCommandSend func(context.Context, string, string) error
}

// Server is a running fixture. Its client speaks cleartext HTTP/2 and keeps its cookies across restarts.
type Server struct {
	URL              string
	HTTPClient       *http.Client
	SigningPublicKey ed25519.PublicKey

	env               map[string]string
	cfg               config.Config
	transport         *http.Transport
	log               *slog.Logger
	linkSeams         LinkSeams
	configurePostgres func(*pgxpool.Config)
	app               *app.Server
}

// clientTimeout bounds one request from the fixture's website client.
const clientTimeout = 10 * time.Second

// Start provisions a database and an OpenBao fixture, starts the application on a loopback port
// and fails the test if startup is refused. Cleanup stops the application before the vault and the database go.
func Start(t testing.TB, options Options) *Server {
	t.Helper()
	database := storetest.NewDatabase(t, "public")
	conn := database.ConnConfig

	// The fixture's credential file is readable only by the process owner.
	passwordFile := filepath.Join(t.TempDir(), "postgres.password")
	if err := os.WriteFile(passwordFile, []byte(conn.Password+"\n"), 0o400); err != nil {
		t.Fatal("The fixture could not write its PostgreSQL password file.")
	}
	vaultAddr, credentialFile := vaulttest.Start(t, fixtureSeed(t))
	listen := loopbackAddress(t)

	env := map[string]string{
		"LGTM_WEB_PASSWORD":            options.Password,
		"LGTM_LISTEN_ADDR":             listen,
		"LGTM_PUBLIC_URL":              "http://" + listen,
		"LGTM_POSTGRES_HOST":           conn.Host,
		"LGTM_POSTGRES_PORT":           strconv.Itoa(int(conn.Port)),
		"LGTM_POSTGRES_DATABASE":       conn.Database,
		"LGTM_POSTGRES_USER":           conn.User,
		"LGTM_POSTGRES_PASSWORD_FILE":  passwordFile,
		"LGTM_ALLOW_PRIVATE_STACK":     "false",
		"LGTM_OPENBAO_ADDR":            vaultAddr,
		"LGTM_OPENBAO_CREDENTIAL_FILE": credentialFile,
	}
	cfg, err := config.Load(func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	})
	if err != nil {
		t.Fatal("The fixture settings could not be loaded.")
	}

	var protocols http.Protocols
	protocols.SetUnencryptedHTTP2(true)
	transport := &http.Transport{Protocols: &protocols, Proxy: nil}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal("The fixture could not create its cookie jar.")
	}
	output := options.Log
	if output == nil {
		output = io.Discard
	}
	s := &Server{
		URL:               "http://" + listen,
		HTTPClient:        &http.Client{Transport: transport, Jar: jar, Timeout: clientTimeout},
		env:               env,
		cfg:               cfg,
		transport:         transport,
		log:               slog.New(slog.NewJSONHandler(output, nil)),
		linkSeams:         options.LinkSeams,
		configurePostgres: options.ConfigurePostgres,
	}
	// Registered after the database and the vault, so it runs before their cleanup.
	t.Cleanup(func() { s.stop() })
	s.launch(t)
	return s
}

// Clients signs in through the fixture's cookie jar and returns the website's client service.
func (s *Server) Clients(t testing.TB) lgtmv1connect.ClientServiceClient {
	t.Helper()
	session := lgtmv1connect.NewSessionServiceClient(s.HTTPClient, s.URL)
	res, err := session.SignIn(t.Context(), connect.NewRequest(&lgtmv1.SignInRequest{Password: s.cfg.Password}))
	if err != nil || res.Msg.GetResult() != lgtmv1.SignInResult_SIGN_IN_RESULT_SIGNED_IN {
		t.Fatal("The fixture could not sign in to the website.")
	}
	return lgtmv1connect.NewClientServiceClient(s.HTTPClient, s.URL)
}

// Link returns a machine-link client that sends no website cookies and uses caller deadlines.
func (s *Server) Link() linkv1connect.LinkServiceClient {
	client := &http.Client{Transport: s.transport}
	return linkv1connect.NewLinkServiceClient(client, s.URL)
}

// Stop closes the application and its listener. It is safe to call more than once.
func (s *Server) Stop(t testing.TB) {
	t.Helper()
	s.stop()
}

// Restart stops the application if it runs and starts it again at the same address,
// with the same database, OpenBao fixture and cookie jar.
func (s *Server) Restart(t testing.TB) {
	t.Helper()
	s.stop()
	s.launch(t)
}

// Environment returns a fresh copy of the settings the fixture started with, for command tests.
func (s *Server) Environment() map[string]string {
	return maps.Clone(s.env)
}

func (s *Server) launch(t testing.TB) {
	t.Helper()
	cfg := s.cfg
	if s.configurePostgres != nil {
		cfg.Postgres = cfg.Postgres.Copy()
		s.configurePostgres(cfg.Postgres)
	}
	server, err := app.StartWithLinkSeams(t.Context(), cfg, s.log, app.LinkSeams(s.linkSeams))
	if err != nil {
		t.Fatal("The fixture application did not start.")
	}
	s.app = server
	s.SigningPublicKey = server.SigningPublicKey()
}

func (s *Server) stop() {
	s.transport.CloseIdleConnections()
	if s.app != nil {
		_ = s.app.Close()
		s.app = nil
	}
}

// fixtureSeed draws one random signing seed per fixture. Restarts reuse it through the OpenBao fixture.
func fixtureSeed(t testing.TB) [32]byte {
	t.Helper()
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal("The fixture could not generate a signing seed.")
	}
	return seed
}

// loopbackAddress reserves a free loopback port. The application binds it right after the reservation ends.
func loopbackAddress(t testing.TB) string {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal("The fixture could not reserve a loopback port.")
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	return addr
}
