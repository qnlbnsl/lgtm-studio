package store

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	minPoolConnections = 8
	writerReserve      = 3
	openTimeout        = 5 * time.Second
	writeTimeout       = 30 * time.Second
	rollbackTimeout    = time.Second
)

// Store owns the database pools and controls transaction lifetimes.
type Store struct {
	pool     *pgxpool.Pool
	pagePool *pgxpool.Pool
	writers  chan struct{}
}

// ConnectionPolicy controls the explicit exception for the private Compose service.
type ConnectionPolicy struct {
	AllowPrivateStackPostgres bool
}

// Outcome describes what PostgreSQL confirmed about a failed transaction.
type Outcome string

const (
	OutcomeRolledBack Outcome = "rolled_back"
	OutcomeUnknown    Outcome = "unknown"
)

// Error exposes a fixed diagnostic and safe PostgreSQL metadata.
type Error struct {
	SQLState   string
	Constraint string
	outcome    Outcome
}

func (e *Error) Error() string {
	if e.outcome == OutcomeUnknown {
		return "The database did not confirm whether the write committed."
	}
	return "The database operation failed."
}

func (e *Error) Outcome() Outcome { return e.outcome }

func (e *Error) Retryable() bool {
	if e.outcome != OutcomeRolledBack {
		return false
	}
	switch e.SQLState {
	case "55P03", "40P01", "40001":
		return true
	default:
		return false
	}
}

type domainError struct{ message string }

func (e *domainError) Error() string { return e.message }

// NewDomainError declares a sentinel with a fixed public message.
func NewDomainError(message string) error { return &domainError{message: message} }

var (
	errConnection = NewDomainError("The database connection could not be opened.")
	sqlState      = regexp.MustCompile(`^[0-9A-Z]{5}$`)
	constraintID  = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)
)

func databaseError(code, constraint string, outcome Outcome) *Error {
	err := &Error{outcome: outcome}
	if sqlState.MatchString(code) {
		err.SQLState = code
		if strings.HasPrefix(code, "23") && constraintID.MatchString(constraint) {
			err.Constraint = constraint
		}
	}
	return err
}

func sanitizeError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if safe, ok := err.(*domainError); ok && safe != nil {
		return safe
	}
	switch err {
	case pgx.ErrNoRows, pgx.ErrTooManyRows:
		return err
	}
	var server *pgconn.PgError
	if errors.As(err, &server) && server != nil {
		return databaseError(server.Code, server.ConstraintName, OutcomeRolledBack)
	}
	var safe *Error
	if errors.As(err, &safe) && safe != nil {
		return databaseError(safe.SQLState, safe.Constraint, OutcomeRolledBack)
	}
	return databaseError("", "", OutcomeRolledBack)
}

func sanitizeCommitError(err error) error {
	var server *pgconn.PgError
	if errors.As(err, &server) && server != nil {
		outcome := OutcomeRolledBack
		switch {
		case server.Severity == "FATAL", server.Severity == "PANIC",
			server.SeverityUnlocalized == "FATAL", server.SeverityUnlocalized == "PANIC",
			strings.HasPrefix(server.Code, "08"), server.Code == "40003":
			outcome = OutcomeUnknown
		}
		return databaseError(server.Code, server.ConstraintName, outcome)
	}
	if errors.Is(err, pgx.ErrTxCommitRollback) || pgconn.SafeToRetry(err) {
		return databaseError("", "", OutcomeRolledBack)
	}
	return databaseError("", "", OutcomeUnknown)
}

// Open validates a parsed pool configuration, then pings the main pool.
func Open(ctx context.Context, config *pgxpool.Config, policy ConnectionPolicy) (*Store, error) {
	if config == nil || config.MaxConns < minPoolConnections || !validEndpoints(config.ConnConfig, policy) {
		return nil, errConnection
	}
	cfg := config.Copy()
	if beforeConnect := cfg.BeforeConnect; beforeConnect != nil {
		dial, lookup := cfg.ConnConfig.DialFunc, cfg.ConnConfig.LookupFunc
		cfg.BeforeConnect = func(ctx context.Context, conn *pgx.ConnConfig) error {
			conn.DialFunc, conn.LookupFunc = dial, lookup
			if err := beforeConnect(ctx, conn); err != nil || !validEndpoints(conn, policy) {
				return errConnection
			}
			configureConnection(conn, policy)
			return nil
		}
	}
	configureConnection(cfg.ConnConfig, policy)

	ctx, cancel := context.WithTimeout(ctx, openTimeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errConnection
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errConnection
	}

	pageConfig := cfg.Copy()
	pageConfig.MaxConns = 1
	pageConfig.MinConns = 0
	pageConfig.MinIdleConns = 0
	pagePool, err := pgxpool.NewWithConfig(ctx, pageConfig)
	if err != nil {
		pool.Close()
		return nil, errConnection
	}
	return &Store{pool: pool, pagePool: pagePool, writers: make(chan struct{}, cfg.MaxConns-writerReserve)}, nil
}

func validEndpoints(config *pgx.ConnConfig, policy ConnectionPolicy) bool {
	if config == nil {
		return false
	}
	valid := func(host string, port uint16, tlsConfig *tls.Config) bool {
		if host == "" || port == 0 {
			return false
		}
		if tlsConfig != nil {
			return !tlsConfig.InsecureSkipVerify && strings.EqualFold(tlsConfig.ServerName, host)
		}
		network, _ := pgconn.NetworkAddress(host, port)
		addr, _ := netip.ParseAddr(host)
		return network == "unix" || addr.Unmap().IsLoopback() ||
			policy.AllowPrivateStackPostgres && host == "postgres" &&
				host == config.Host && port == config.Port
	}
	if !valid(config.Host, config.Port, config.TLSConfig) {
		return false
	}
	for _, fallback := range config.Fallbacks {
		if fallback == nil || !valid(fallback.Host, fallback.Port, fallback.TLSConfig) {
			return false
		}
	}
	if privateCompose(config, policy) {
		for _, fallback := range config.Fallbacks {
			if fallback.Host != config.Host || fallback.Port != config.Port {
				return false
			}
		}
	}
	return true
}

func privateCompose(config *pgx.ConnConfig, policy ConnectionPolicy) bool {
	if !policy.AllowPrivateStackPostgres || config.Host != "postgres" {
		return false
	}
	if config.TLSConfig == nil {
		return true
	}
	for _, fallback := range config.Fallbacks {
		if fallback.Host == config.Host && fallback.Port == config.Port && fallback.TLSConfig == nil {
			return true
		}
	}
	return false
}

func configureConnection(config *pgx.ConnConfig, policy ConnectionPolicy) {
	if config.ConnectTimeout <= 0 || config.ConnectTimeout > openTimeout {
		config.ConnectTimeout = openTimeout
	}
	if config.RuntimeParams == nil {
		config.RuntimeParams = make(map[string]string)
	}
	for name, value := range map[string]string{
		"idle_in_transaction_session_timeout": "30s",
		"statement_timeout":                   "15s",
	} {
		if _, set := config.RuntimeParams[name]; !set {
			config.RuntimeParams[name] = value
		}
	}

	lookup := config.LookupFunc
	if lookup == nil {
		lookup = net.DefaultResolver.LookupHost
	}
	config.LookupFunc = func(ctx context.Context, host string) ([]string, error) {
		if _, err := netip.ParseAddr(host); err == nil {
			return []string{host}, nil
		}
		return lookup(ctx, host)
	}

	dial := config.DialFunc
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	checkPrivate := privateCompose(config, policy)
	composePort := strconv.Itoa(int(config.Port))
	config.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		if checkPrivate {
			host, port, err := net.SplitHostPort(address)
			ip, ipErr := netip.ParseAddr(host)
			if err != nil || ipErr != nil || network != "tcp" || !ip.Unmap().IsPrivate() ||
				port != composePort {
				return nil, errConnection
			}
		}
		conn, err := dial(ctx, network, address)
		if err != nil {
			return nil, err
		}
		if tcp, ok := conn.(*net.TCPConn); ok {
			if err := tcp.SetKeepAliveConfig(net.KeepAliveConfig{Enable: true, Idle: 30 * time.Second}); err != nil {
				_ = tcp.Close()
				return nil, err
			}
		}
		return conn, nil
	}
}

func (s *Store) Close() {
	s.pagePool.Close()
	s.pool.Close()
}

// Read calls read once inside a repeatable, read-only transaction.
func (s *Store) Read(ctx context.Context, read func(context.Context, *ReadTx) error) error {
	return withTransaction(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly},
		func(ctx context.Context, tx pgx.Tx) error { return read(ctx, &ReadTx{tx: tx}) })
}

func (s *Store) withWrite(ctx context.Context, write func(context.Context, pgx.Tx) error) error {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	select {
	case s.writers <- struct{}{}:
		defer func() { <-s.writers }()
	case <-ctx.Done():
		return ctx.Err()
	}

	return withTransaction(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadWrite},
		func(ctx context.Context, tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '15s'; SET LOCAL lock_timeout = '10s'"); err != nil {
				return err
			}
			return write(ctx, tx)
		})
}

func withTransaction(ctx context.Context, pool *pgxpool.Pool, options pgx.TxOptions, run func(context.Context, pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, options)
	if err != nil {
		return sanitizeError(ctx, err)
	}
	// The pool releases the connection after rollback. The driver discards it if rollback fails.
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), rollbackTimeout)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()

	if err := run(ctx, tx); err != nil {
		return sanitizeError(ctx, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return sanitizeCommitError(err)
	}
	return nil
}

// ReadTx exposes queries within the lifetime of a Store read callback.
type ReadTx struct{ tx pgx.Tx }

func (tx *ReadTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, err := tx.tx.Exec(ctx, sql, args...)
	return tag, sanitizeError(ctx, err)
}

func (tx *ReadTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := tx.tx.Query(ctx, sql, args...)
	if rows != nil {
		rows = &readRows{Rows: rows, ctx: ctx}
	}
	return rows, sanitizeError(ctx, err)
}

func (tx *ReadTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return readRow{Row: tx.tx.QueryRow(ctx, sql, args...), ctx: ctx}
}

type readRow struct {
	pgx.Row
	ctx context.Context
}

func (r readRow) Scan(dest ...any) error {
	return sanitizeError(r.ctx, r.Row.Scan(dest...))
}

type readRows struct {
	pgx.Rows
	ctx context.Context
}

func (r *readRows) Err() error { return sanitizeError(r.ctx, r.Rows.Err()) }

func (r *readRows) Scan(dest ...any) error {
	return sanitizeError(r.ctx, r.Rows.Scan(dest...))
}

func (r *readRows) Values() ([]any, error) {
	values, err := r.Rows.Values()
	return values, sanitizeError(r.ctx, err)
}

// Conn keeps pooled connection ownership inside the Store.
func (r *readRows) Conn() *pgx.Conn { return nil }
