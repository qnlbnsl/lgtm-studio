// Package machines keeps the server's record of machines and their join tokens.
//
// This file is a STUB of the test writer. The builder replaces every body.
package machines

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// Approval is the standing of a machine.
type Approval string

const (
	ApprovalWaiting  Approval = "waiting"
	ApprovalApproved Approval = "approved"
)

// Errors of the registry. Each crosses the store unwrapped.
var (
	ErrNoMachine   = store.NewDomainError("No machine has that id.")
	ErrNameTaken   = store.NewDomainError("Another machine already has that name.")
	ErrNotWaiting  = store.NewDomainError("This machine does not wait for approval.")
	ErrInvalidName = store.NewDomainError("A machine needs a name of 1 to 64 characters.")
)

// Machine is the stored record of one machine.
type Machine struct {
	ID               string
	PublicKey        []byte
	HostName         string
	Name             string
	Approval         Approval
	StateRevision    int64
	HostsServer      bool
	TokenID          string
	JoinedAt         time.Time
	ApprovedAt       *time.Time
	Facts            []byte
	Harnesses        []byte
	HarnessesScanned bool
	LastSeenAt       *time.Time
	LastAddress      string
}

// NewMachine is what a join gives the registry.
type NewMachine struct {
	PublicKey   []byte
	HostName    string
	TokenID     string
	HostsServer bool
}

// Querier is what Get and List need. Both *store.Tx and *store.ReadTx satisfy it.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Registry keeps no state. Its writes take the transaction of the caller.
type Registry struct{}

// NewRegistry returns the registry.
func NewRegistry() *Registry { return &Registry{} }

// CreateWaiting inserts a waiting machine at revision 1 and records `machines` saved.
func (r *Registry) CreateWaiting(ctx context.Context, tx *store.Tx, in NewMachine) (Machine, error) {
	return Machine{}, nil
}

// Get returns the machine or ErrNoMachine.
func (r *Registry) Get(ctx context.Context, q Querier, id string) (Machine, error) {
	return Machine{}, nil
}

// List returns every machine ordered by joined_at, then id.
func (r *Registry) List(ctx context.Context, q Querier) ([]Machine, error) { return nil, nil }

// SetApproved approves a waiting machine under a normalized name, or returns ErrNotWaiting or ErrNameTaken.
func (r *Registry) SetApproved(ctx context.Context, tx *store.Tx, id, name string) error { return nil }

// SetName renames a machine and reports whether anything changed.
func (r *Registry) SetName(ctx context.Context, tx *store.Tx, id, name string) (bool, error) {
	return false, nil
}

// Delete removes a machine of any standing, or returns ErrNoMachine.
func (r *Registry) Delete(ctx context.Context, tx *store.Tx, id string) error { return nil }

// DeleteWaiting removes the machine only if it still waits, and reports whether it did.
func (r *Registry) DeleteWaiting(ctx context.Context, tx *store.Tx, id string) (bool, error) {
	return false, nil
}

// NormalizeName trims spaces and checks the rules of a machine's name.
func NormalizeName(name string) (string, error) { return "", nil }
