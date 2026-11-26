package link

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

type currentReadObserverKey struct{}
type currentReadSQLKey struct{}

func TestCheckCurrentUsesRegistrySnapshot(t *testing.T) {
	f := newPresenceFixture(t, func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = streamReviewQueryTrace{
			handshakeQueryTrace: func(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
				sql := strings.TrimSpace(data.SQL)
				if ctx.Value(currentReadObserverKey{}) != nil && strings.HasPrefix(strings.ToLower(sql), "select ") {
					return context.WithValue(ctx, currentReadSQLKey{}, sql)
				}
				return ctx
			},
			end: func(ctx context.Context, data pgx.TraceQueryEndData) {
				if sql, ok := ctx.Value(currentReadSQLKey{}).(string); ok && data.Err == nil {
					ctx.Value(currentReadObserverKey{}).(func(string))(sql)
				}
			},
		}
	})
	m := f.machine(t, bytes.Repeat([]byte{4}, 32), "station changing streams")
	a := f.admit(t, m.ID, 1)

	// Observe the actual registry query so the test follows registry schema changes.
	var registryQuery string
	refCtx := context.WithValue(t.Context(), currentReadObserverKey{}, func(sql string) {
		registryQuery = sql
	})
	if err := f.s.Read(refCtx, func(ctx context.Context, tx *store.ReadTx) error {
		got, err := f.r.Get(ctx, tx, m.ID)
		if err == nil && got.Approval != machines.ApprovalWaiting {
			return errors.New("The reference registry lookup did not see the waiting machine.")
		}
		return err
	}); err != nil || registryQuery == "" {
		t.Fatal("The fixture could not observe a successful reference registry lookup.")
	}

	var queries []string
	firstRead := make(chan struct{})
	hold, release := storetest.Signal(t)
	defer release()
	checkCtx := context.WithValue(t.Context(), currentReadObserverKey{}, func(sql string) {
		queries = append(queries, sql)
		if len(queries) == 1 {
			close(firstRead)
			<-hold
		}
	})
	type result struct {
		approval machines.Approval
		err      error
	}
	check := storetest.Start(t, func() result {
		approval, err := f.state.CheckCurrent(checkCtx, m.ID, a.ID)
		return result{approval, err}
	})
	storetest.Receive(t, "the first identity query before the snapshot changes", firstRead)
	f.approve(t, m.ID)
	b := f.admit(t, m.ID, 2)
	release()

	got := check.Wait(t)
	if !slices.Contains(queries, registryQuery) {
		t.Error("CheckCurrent bypassed the registry lookup when it read the machine's approval.")
	}
	if got.err != nil || got.approval != machines.ApprovalWaiting {
		t.Error("CheckCurrent mixed the registry standing and stream identity from different snapshots.")
	}
	if approval, err := f.state.CheckCurrent(t.Context(), m.ID, b.ID); err != nil || approval != machines.ApprovalApproved {
		t.Error("A fresh identity check did not see the approved replacement.")
	}
	if _, err := f.state.CheckCurrent(t.Context(), m.ID, a.ID); !errors.Is(err, ErrNotCurrent) {
		t.Error("The old identifier still identified the approved replacement.")
	}

	// Retain the exact link row to distinguish registry absence from displacement.
	if err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		return f.r.Delete(ctx, tx, m.ID)
	}); err != nil || f.connectionCount(t, m.ID) != 1 {
		t.Fatal("The fixture could not leave the link row behind after deleting the machine.")
	}
	for _, streamID := range []string{a.ID, b.ID} {
		if _, err := f.state.CheckCurrent(t.Context(), m.ID, streamID); !errors.Is(err, ErrUnknownMachine) {
			t.Error("The missing machine did not take precedence over the link row.")
		}
	}
}

func TestCurrentHeartbeatRejectsCPUAboveLimit(t *testing.T) {
	f := newPresenceFixture(t, nil)
	m := f.machine(t, bytes.Repeat([]byte{7}, 32), "station reporting load")
	stream := f.admit(t, m.ID, 1)
	if err := f.state.Heartbeat(t.Context(), m.ID, stream.ID, &linkv1.Heartbeat{
		CpuPercent: proto.Uint32(100), MemoryPercent: proto.Uint32(27),
	}); err != nil {
		t.Fatal("The current heartbeat could not store the baseline loads, including the inclusive CPU limit.")
	}
	saved := f.current(t, m.ID)
	if !sameLoad(saved.CPUPercent, proto.Uint32(100)) || !sameLoad(saved.MemoryPercent, proto.Uint32(27)) {
		t.Fatal("The baseline heartbeat did not store both loads.")
	}
	const readConnection = "SELECT to_jsonb(c)::text FROM link_connections c WHERE machine_id = $1"
	before := storetest.Scalar[string](t, f.obs, readConnection, m.ID)
	head := f.head(t)
	err := f.state.Heartbeat(t.Context(), m.ID, stream.ID, &linkv1.Heartbeat{
		CpuPercent: proto.Uint32(101), MemoryPercent: proto.Uint32(28),
	})
	if code := connect.CodeOf(openStatus(err)); code != connect.CodeInvalidArgument {
		t.Errorf("A current heartbeat with CPU at 101 mapped to %v, want invalid_argument.", code)
	}
	if after := storetest.Scalar[string](t, f.obs, readConnection, m.ID); after != before {
		t.Error("The rejected heartbeat changed the saved connection row.")
	}
	f.requireSaved(t, head)
}
