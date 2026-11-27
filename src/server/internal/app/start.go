// Package app starts the server: it checks every prerequisite before it listens, then serves until it is closed.
package app

import (
	"context"
	"crypto/ed25519"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/config"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/vault"
)

// The fixed sentences of startup. None of them carries a setting's value.
const (
	linePassword        = "LGTM_WEB_PASSWORD is missing or invalid."
	lineDatabaseConfig  = "The PostgreSQL settings are invalid."
	lineDatabaseOpen    = "The database connection failed, check the PostgreSQL connection settings."
	lineInstanceBusy    = "Another server is already using this database."
	lineCanceled        = "Server startup was canceled."
	lineDeadline        = "The startup deadline expired."
	lineInstanceAcquire = "The server could not acquire its PostgreSQL instance lock."
	lineInstanceLost    = "The server lost its PostgreSQL instance lock."
	lineMigrate         = "The server could not migrate its PostgreSQL database."
	lineReconcile       = "Session reconciliation did not complete."
	lineUnconfirmed     = "Session revocation was not confirmed."
	lineCommitted       = "Session revocation committed."
	lineListenAddr      = "The LGTM_LISTEN_ADDR setting is invalid."
	linePublicURL       = "The LGTM_PUBLIC_URL setting is invalid."
	lineOrigins         = "The LGTM_ALLOWED_ORIGINS setting is invalid."
	lineLinkCleanup     = "The server could not clear stale link connections."
	lineFeed            = "The server could not follow its PostgreSQL change feed."
	lineListen          = "The server could not listen on LGTM_LISTEN_ADDR."
)

// Production durations of the startup phases and of one probe of the instance lock.
const (
	admissionBudget     = 10 * time.Second
	migrationBudget     = 5 * time.Minute
	prerequisitesBudget = 2 * time.Minute
	probeBudget         = 5 * time.Second
)

const (
	probeInterval      = time.Second
	headerTimeout      = 5 * time.Second
	idleTimeout        = 60 * time.Second
	drainTimeout       = 5 * time.Second
	unlockTimeout      = 5 * time.Second
	readyMessage       = "link_signing_key_loaded"
	storeReportMessage = "store_report"
)

// errLockLost is the cause of the lifetime context when a probe of the instance lock fails.
var errLockLost = errors.New(lineInstanceLost)

// errClosed is the cause of the lifetime context when the owner of the server closes it.
var errClosed = errors.New("The server was closed.")

// StartupError holds the applicable diagnostic sentences of a refused startup, in order.
// Its text is the sentences joined by line feeds.
type StartupError struct {
	lines []string
}

func (e *StartupError) Error() string { return strings.Join(e.lines, "\n") }

// Lines returns a copy of the diagnostic sentences.
func (e *StartupError) Lines() []string { return append([]string(nil), e.lines...) }

// Server is a running application. It holds the instance lock until it stops.
type Server struct {
	addr      net.Addr
	publicKey ed25519.PublicKey
	cancel    context.CancelCauseFunc
	done      chan struct{}
	err       error
}

type phaseBudgets struct {
	admission, migration, prerequisites, probe time.Duration
}

// LinkSeams lets tests observe handshake and acknowledgement phases.
type LinkSeams struct {
	AfterProof, AfterAdmission func(context.Context, string)
	BeforeEventAck             func(context.Context, string, uint64)
}

// Start checks the prerequisites in order and returns a listening server, or a *StartupError.
// The server stops when ctx ends, when it loses its instance lock or when Close is called.
func Start(ctx context.Context, cfg config.Config, log *slog.Logger) (*Server, error) {
	return StartWithLinkSeams(ctx, cfg, log, LinkSeams{})
}

// StartWithLinkSeams starts the server with optional link callbacks for tests.
func StartWithLinkSeams(ctx context.Context, cfg config.Config, log *slog.Logger, seams LinkSeams) (*Server, error) {
	return startWithLinkSeams(ctx, cfg, log, phaseBudgets{
		admission: admissionBudget, migration: migrationBudget,
		prerequisites: prerequisitesBudget, probe: probeBudget,
	}, seams)
}

// startup carries the state of one start, so a failure can release what was taken in reverse order.
type startup struct {
	ctx      context.Context
	life     context.Context
	cancel   context.CancelCauseFunc
	budgets  phaseBudgets
	usable   bool
	store    *store.Store
	lock     *store.InstanceLock
	monitor  sync.WaitGroup
	follower *store.Follower
	workers  sync.WaitGroup
}

func start(ctx context.Context, cfg config.Config, log *slog.Logger, budgets phaseBudgets) (*Server, error) {
	return startWithLinkSeams(ctx, cfg, log, budgets, LinkSeams{})
}

func startWithLinkSeams(ctx context.Context, cfg config.Config, log *slog.Logger, budgets phaseBudgets, seams LinkSeams) (*Server, error) {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	password, passwordErr := auth.NewPassword(cfg.Password)
	life, cancel := context.WithCancelCause(ctx)
	st := &startup{ctx: ctx, life: life, cancel: cancel, budgets: budgets, usable: passwordErr == nil}

	server, lines := st.run(cfg, password, log, seams)
	if lines != nil {
		st.release()
		return nil, &StartupError{lines: lines}
	}
	return server, nil
}

// run performs the phases. It returns the diagnostic sentences of a refused start.
func (st *startup) run(cfg config.Config, password auth.Password, log *slog.Logger, seams LinkSeams) (*Server, []string) {
	// The database phase ends before session reconciliation, so each of its failures reports both.
	if cfg.Postgres == nil {
		return nil, st.unreconciled(lineDatabaseConfig)
	}
	s, err := store.Open(st.ctx, cfg.Postgres, store.ConnectionPolicy{AllowPrivateStackPostgres: cfg.AllowPrivateStack})
	if err != nil {
		return nil, st.unreconciled(st.interrupted(nil, lineDatabaseOpen))
	}
	st.store = s

	lock, err := s.AcquireServerInstance(st.life, st.budgets.admission)
	switch {
	case errors.Is(err, store.ErrServerInstanceBusy):
		return nil, st.unreconciled(lineInstanceBusy)
	case err != nil:
		return nil, st.unreconciled(st.interrupted(nil, lineInstanceAcquire))
	}
	st.lock = lock
	if err := st.probe(); err != nil {
		return nil, st.unreconciled(st.interrupted(nil, lineInstanceAcquire))
	}
	st.monitor.Add(1)
	go st.watchLock()

	migration, cancelMigration := context.WithTimeout(st.life, st.budgets.migration)
	err = s.Migrate(migration, auth.Migrations(), vault.Migrations(), machines.Migrations(), link.Migrations())
	cancelMigration()
	if err != nil {
		return nil, st.unreconciled(st.interrupted(migration, lineMigrate))
	}

	// The prerequisite deadline starts after the migration, which has a budget of its own.
	prerequisites, cancelPrerequisites := context.WithTimeout(st.life, st.budgets.prerequisites)
	defer cancelPrerequisites()
	var configured *auth.Password
	if st.usable {
		configured = &password
	}
	if err := auth.Reconcile(prerequisites, s, configured); err != nil {
		if interrupted := st.interrupted(prerequisites, ""); interrupted != "" {
			return nil, st.unreconciled(interrupted)
		}
		return nil, st.unreconciled()
	}
	if !st.usable {
		return nil, []string{lineCommitted, linePassword}
	}

	// The other settings are checked only now, so a password change revokes sessions even if they are wrong.
	var lines []string
	if !validListenAddr(cfg.ListenAddr, cfg.AllowPrivateStack) {
		lines = append(lines, lineListenAddr)
	}
	origins, err := auth.NewOriginPolicy(cfg.PublicURL, nil)
	if err != nil {
		lines = append(lines, linePublicURL)
	} else if origins, err = auth.NewOriginPolicy(cfg.PublicURL, cfg.AllowedOrigins); err != nil {
		lines = append(lines, lineOrigins)
	}
	client, err := vault.Open(cfg.OpenBaoAddr, cfg.OpenBaoCredentialFile, cfg.AllowPrivateStack)
	if err != nil {
		lines = append(lines, vaultLine(err))
	}
	if lines != nil {
		return nil, lines
	}

	identity, err := vault.LoadLinkIdentity(prerequisites, s, client, true)
	if err != nil {
		if interrupted := st.interrupted(prerequisites, ""); interrupted != "" {
			return nil, []string{interrupted}
		}
		return nil, []string{vaultLine(err)}
	}

	registry := machines.NewRegistry()
	hub := link.NewHub()
	state := link.NewState(s, registry, hub)
	if err := state.ClearConnections(prerequisites); err != nil {
		log.Error(lineLinkCleanup)
		return nil, []string{lineLinkCleanup}
	}

	report := func(err error) {
		if err != nil {
			log.Warn(storeReportMessage, "error", err.Error())
		}
	}
	// The follower must stay open until in-flight requests have drained.
	follower, err := store.StartFollowerWithLifetime(st.life, context.WithoutCancel(st.life), s, report)
	if err != nil {
		return nil, []string{st.interrupted(prerequisites, lineFeed)}
	}
	st.follower = follower
	st.workers.Add(1)
	go func() {
		defer st.workers.Done()
		s.RunPruner(st.life, report)
	}()

	sessions := auth.NewSessions(s, password)
	public := identity.SigningKey.Public().(ed25519.PublicKey)
	tokens := machines.NewTokens(s, registry, machines.Issuer{PublicURL: cfg.PublicURL, ServerKey: public})
	streamService := link.NewStreamingService(st.life, s, tokens, registry, state, hub, identity.SigningKey, link.Options{
		PrivateStack: cfg.AllowPrivateStack,
		Logger:       log,
		Seams:        link.Seams(seams),
		Commands:     state,
	})
	st.workers.Go(func() { streamService.Run(st.life, follower) })
	clients := machines.NewService(s, sessions, registry, tokens).
		WithFollower(follower).WithWatchShutdown(st.life).WithLink(state).WithCommands(state)
	handler := api.NewHandler(api.Options{
		Sessions: sessions,
		Origins:  origins,
		Session:  api.NewSessionService(password, sessions),
		Clients:  clients,
		Link:     streamService,
	})

	// The last probe comes immediately before the listener opens.
	if err := st.probe(); err != nil {
		return nil, []string{st.interrupted(nil, lineInstanceLost)}
	}
	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return nil, []string{lineListen}
	}
	if st.life.Err() != nil {
		// The monitor or the caller ended the lifetime after the last probe, so readiness is not reported.
		_ = listener.Close()
		return nil, []string{st.interrupted(nil, lineInstanceLost)}
	}

	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	httpServer := &http.Server{
		Handler:           handler,
		Protocols:         &protocols,
		ReadHeaderTimeout: headerTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
	server := &Server{
		addr:      listener.Addr(),
		publicKey: append(ed25519.PublicKey(nil), public...),
		cancel:    st.cancel,
		done:      make(chan struct{}),
	}
	log.Info(readyMessage,
		"source", "openbao",
		"fingerprint", identity.Fingerprint,
		"vault_version", identity.VaultVersion,
		"adopted", identity.Adopted())

	served := make(chan error, 1)
	go func() { served <- httpServer.Serve(listener) }()
	go st.serve(server, httpServer, served)
	return server, nil
}

// serve waits for the end of the lifetime, then drains requests and releases the workers, the lock and the Store.
func (st *startup) serve(server *Server, httpServer *http.Server, served <-chan error) {
	var failure error
	stopped := false
	select {
	case <-st.life.Done():
	case err := <-served:
		// The listener failed while the lifetime was still running.
		failure, stopped = err, true
		st.cancel(err)
	}

	if errors.Is(context.Cause(st.life), errLockLost) {
		// Another server may take the lock at once, so this one stops answering at once.
		failure = errLockLost
		_ = httpServer.Close()
	} else {
		drain, cancel := context.WithTimeout(context.Background(), drainTimeout)
		_ = httpServer.Shutdown(drain)
		cancel()
		_ = httpServer.Close()
	}
	if !stopped {
		<-served
	}
	st.release()
	server.err = failure
	close(server.done)
}

// release stops the workers and the monitor, unlocks and closes the Store. It runs once per start.
func (st *startup) release() {
	st.cancel(errClosed)
	st.workers.Wait()
	if st.follower != nil {
		st.follower.Close()
	}
	st.monitor.Wait()
	if st.lock != nil {
		ctx, cancel := context.WithTimeout(context.Background(), unlockTimeout)
		_ = st.lock.Close(ctx)
		cancel()
	}
	if st.store != nil {
		st.store.Close()
	}
}

// probe checks the instance lock once within the probe budget.
// It does not use the lifetime context, so closing the server never interrupts a probe halfway.
func (st *startup) probe() error {
	ctx, cancel := context.WithTimeout(context.Background(), st.budgets.probe)
	defer cancel()
	if err := st.lock.Check(ctx); err != nil {
		st.cancel(errLockLost)
		return err
	}
	return nil
}

// watchLock probes one second after each completed probe until the lifetime ends.
// A failed probe ends the lifetime, which stops startup or serving.
func (st *startup) watchLock() {
	defer st.monitor.Done()
	timer := time.NewTimer(probeInterval)
	defer timer.Stop()
	for {
		select {
		case <-st.life.Done():
			return
		case <-timer.C:
		}
		if st.life.Err() != nil || st.probe() != nil {
			return
		}
		timer.Reset(probeInterval)
	}
}

// interrupted names the reason that a phase stopped early, or returns fallback.
// A lost lock comes first, then the caller's context, then the phase's own deadline.
func (st *startup) interrupted(phase context.Context, fallback string) string {
	switch {
	case errors.Is(context.Cause(st.life), errLockLost):
		return lineInstanceLost
	case errors.Is(st.ctx.Err(), context.Canceled):
		return lineCanceled
	case errors.Is(st.ctx.Err(), context.DeadlineExceeded):
		return lineDeadline
	case phase != nil && errors.Is(phase.Err(), context.DeadlineExceeded):
		return lineDeadline
	}
	return fallback
}

// unreconciled completes the sentences of a start that stopped before session reconciliation finished.
// An unusable password always ends with the revocation outcome and then the password line.
func (st *startup) unreconciled(lines ...string) []string {
	lines = append(lines, lineReconcile)
	if !st.usable {
		lines = append(lines, lineUnconfirmed, linePassword)
	}
	return lines
}

// vaultLine returns the vault package's fixed sentence, or the establishment sentence for any other error.
func vaultLine(err error) string {
	for _, known := range []error{vault.ErrCredential, vault.ErrAddress, vault.ErrUnavailable, vault.ErrEstablish, vault.ErrReplaced} {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	return vault.ErrEstablish.Error()
}

// validListenAddr accepts a literal loopback address. With the private stack it also accepts the
// unspecified and private addresses of the stack network, including an empty host.
func validListenAddr(addr string, privateStack bool) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return false
	}
	if host == "" {
		return privateStack
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	ip = ip.Unmap()
	return ip.IsLoopback() || privateStack && (ip.IsUnspecified() || ip.IsPrivate())
}

// Addr returns the listener's address.
func (s *Server) Addr() net.Addr { return s.addr }

// SigningPublicKey returns a copy of the link's public signing key.
func (s *Server) SigningPublicKey() ed25519.PublicKey {
	return append(ed25519.PublicKey(nil), s.publicKey...)
}

// Wait blocks until the server has stopped and released its lock.
// It returns nil after Close or the end of the start context, and an error when the server lost its lock or listener.
func (s *Server) Wait() error {
	<-s.done
	return s.err
}

// Close drains requests for at most five seconds, then stops the server and waits for its cleanup.
// It is safe to call more than once.
func (s *Server) Close() error {
	s.cancel(errClosed)
	<-s.done
	return nil
}
