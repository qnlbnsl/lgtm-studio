// Package machines keeps the server's record of machines and their join tokens.
package machines

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"

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

// Querier provides reads within either a read or a write transaction.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Registry keeps no state. Its writes take the transaction of the caller.
type Registry struct{}

// NewRegistry returns the registry.
func NewRegistry() *Registry { return &Registry{} }

const machineColumns = `id, public_key, host_name, name, approval, state_revision, hosts_server,
token_id, joined_at, approved_at, facts, harnesses, harnesses_scanned, last_seen_at, last_address`

func scanMachine(row pgx.Row) (Machine, error) {
	var m Machine
	if err := row.Scan(&m.ID, &m.PublicKey, &m.HostName, &m.Name, &m.Approval, &m.StateRevision,
		&m.HostsServer, &m.TokenID, &m.JoinedAt, &m.ApprovedAt, &m.Facts, &m.Harnesses,
		&m.HarnessesScanned, &m.LastSeenAt, &m.LastAddress); err != nil {
		return Machine{}, err
	}
	return m, nil
}

// CreateWaiting inserts a waiting machine at revision 1 and records the change.
func (r *Registry) CreateWaiting(ctx context.Context, tx *store.Tx, in NewMachine) (Machine, error) {
	id, err := store.NewID()
	if err != nil {
		return Machine{}, err
	}
	m, err := scanMachine(tx.QueryRow(ctx, `
INSERT INTO machines (id, public_key, host_name, approval, state_revision, hosts_server, token_id)
VALUES ($1, $2, $3, 'waiting', 1, $4, $5)
RETURNING `+machineColumns, id, in.PublicKey, in.HostName, in.HostsServer, in.TokenID))
	if err != nil {
		return Machine{}, err
	}
	if err := recordMachineChange(tx, id, store.Saved); err != nil {
		return Machine{}, err
	}
	return m, nil
}

// Get returns the machine or ErrNoMachine.
func (r *Registry) Get(ctx context.Context, q Querier, id string) (Machine, error) {
	m, err := scanMachine(q.QueryRow(ctx, `SELECT `+machineColumns+` FROM machines WHERE id = $1`, id))
	if err == pgx.ErrNoRows {
		return Machine{}, ErrNoMachine
	}
	return m, err
}

// List returns every machine ordered by joined_at, then id.
func (r *Registry) List(ctx context.Context, q Querier) ([]Machine, error) {
	rows, err := q.Query(ctx, `SELECT `+machineColumns+` FROM machines ORDER BY joined_at, id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Machine, error) {
		return scanMachine(row)
	})
}

// SetApproved approves a waiting machine under a normalized name.
func (r *Registry) SetApproved(ctx context.Context, tx *store.Tx, id, name string) error {
	name, err := NormalizeName(name)
	if err != nil {
		return err
	}
	var approval Approval
	err = tx.QueryRow(ctx, `SELECT approval FROM machines WHERE id = $1 FOR NO KEY UPDATE`, id).Scan(&approval)
	if err == pgx.ErrNoRows {
		return ErrNoMachine
	}
	if err != nil {
		return err
	}
	if approval != ApprovalWaiting {
		return ErrNotWaiting
	}
	_, err = tx.Exec(ctx, `
UPDATE machines
SET name = $2, approval = 'approved', approved_at = clock_timestamp(), state_revision = state_revision + 1
WHERE id = $1`, id, name)
	if err != nil {
		return nameWriteError(err)
	}
	return recordMachineChange(tx, id, store.Saved)
}

// SetName renames a machine and reports whether anything changed.
func (r *Registry) SetName(ctx context.Context, tx *store.Tx, id, name string) (bool, error) {
	name, err := NormalizeName(name)
	if err != nil {
		return false, err
	}
	var current string
	err = tx.QueryRow(ctx, `SELECT name FROM machines WHERE id = $1 FOR NO KEY UPDATE`, id).Scan(&current)
	if err == pgx.ErrNoRows {
		return false, ErrNoMachine
	}
	if err != nil || current == name {
		return false, err
	}
	_, err = tx.Exec(ctx, `UPDATE machines SET name = $2, state_revision = state_revision + 1 WHERE id = $1`, id, name)
	if err != nil {
		return false, nameWriteError(err)
	}
	if err := recordMachineChange(tx, id, store.Saved); err != nil {
		return false, err
	}
	return true, nil
}

// Delete removes a machine of any standing, or returns ErrNoMachine.
func (r *Registry) Delete(ctx context.Context, tx *store.Tx, id string) error {
	tag, err := tx.Exec(ctx, `DELETE FROM machines WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNoMachine
	}
	return recordMachineChange(tx, id, store.Removed)
}

// DeleteWaiting removes the machine only if it still waits, and reports whether it did.
func (r *Registry) DeleteWaiting(ctx context.Context, tx *store.Tx, id string) (bool, error) {
	tag, err := tx.Exec(ctx, `DELETE FROM machines WHERE id = $1 AND approval = 'waiting'`, id)
	if err != nil || tag.RowsAffected() == 0 {
		return false, err
	}
	if err := recordMachineChange(tx, id, store.Removed); err != nil {
		return false, err
	}
	return true, nil
}

// NormalizeName trims spaces and checks the rules of a machine's name.
func NormalizeName(name string) (string, error) {
	if !utf8.ValidString(name) || strings.ContainsFunc(name, unicode.IsControl) {
		return "", ErrInvalidName
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 {
		return "", ErrInvalidName
	}
	return name, nil
}

func nameWriteError(err error) error {
	var dbErr *store.Error
	if errors.As(err, &dbErr) && dbErr.SQLState == "23505" && dbErr.Constraint == "machines_approved_name" {
		return ErrNameTaken
	}
	return err
}

func recordMachineChange(tx *store.Tx, id string, change store.Change) error {
	return tx.Record(store.ChangeRef{Topic: "machines", Key: id, Change: change})
}

// Lock serializes machine changes for the remainder of the write.
func (r *Registry) Lock(ctx context.Context, tx *store.Tx, id string) (Machine, error) {
	return lockMachine(ctx, tx, id, ` FOR NO KEY UPDATE`)
}

// TryLock returns a retryable store error if another write holds the machine.
func (r *Registry) TryLock(ctx context.Context, tx *store.Tx, id string) (Machine, error) {
	return lockMachine(ctx, tx, id, ` FOR NO KEY UPDATE NOWAIT`)
}

func lockMachine(ctx context.Context, tx *store.Tx, id, lock string) (Machine, error) {
	m, err := scanMachine(tx.QueryRow(ctx, `SELECT `+machineColumns+` FROM machines WHERE id = $1`+lock, id))
	if err == pgx.ErrNoRows {
		return Machine{}, ErrNoMachine
	}
	return m, err
}

// ApplyFacts stores the declared facts of a validated report.
func (r *Registry) ApplyFacts(ctx context.Context, tx *store.Tx, id string, facts *linkv1.Facts) error {
	data, err := proto.Marshal(&linkv1.Facts{
		Os: facts.GetOs(), Cores: facts.GetCores(), MemoryBytes: facts.GetMemoryBytes(),
		ClientVersion: facts.GetClientVersion(), Workspace: facts.GetWorkspace(), AgentLimit: facts.GetAgentLimit(),
	})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE machines SET facts = $2 WHERE id = $1`, id, data)
	return err
}

// ApplyHarnesses stores a validated inventory and marks even an empty scan as complete.
func (r *Registry) ApplyHarnesses(ctx context.Context, tx *store.Tx, id string, report *linkv1.HarnessesFound) error {
	inventory := &linkv1.HarnessesFound{}
	for _, harness := range report.GetHarnesses() {
		entry := &linkv1.Harness{Name: harness.GetName()}
		if harness != nil && harness.Version != nil {
			entry.Version = proto.String(harness.GetVersion())
		}
		inventory.Harnesses = append(inventory.Harnesses, entry)
	}
	data, err := proto.Marshal(inventory)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE machines SET harnesses = $2, harnesses_scanned = true WHERE id = $1`, id, data)
	return err
}

// NoteSeen saves the last contact of a disconnected stream on the machine.
func (r *Registry) NoteSeen(ctx context.Context, tx *store.Tx, id string, at time.Time, address string) error {
	_, err := tx.Exec(ctx, `UPDATE machines SET last_seen_at = $2, last_address = $3 WHERE id = $1`, id, at, address)
	return err
}

// Touch records a saved reference after a set of changes to one machine.
func (r *Registry) Touch(_ context.Context, tx *store.Tx, id string) error {
	return recordMachineChange(tx, id, store.Saved)
}

// Standing returns the link's view of the machine's stored approval and revision.
func Standing(machine Machine) *linkv1.MachineState {
	standing := &linkv1.MachineState{Approval: linkv1.Approval_APPROVAL_WAITING, Revision: uint64(machine.StateRevision)}
	if machine.Approval == ApprovalApproved {
		standing.Approval = linkv1.Approval_APPROVAL_APPROVED
		standing.Name = machine.Name
	}
	return standing
}
