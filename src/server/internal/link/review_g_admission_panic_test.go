package link

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestReviewGAdmissionCommitPanicUnregistersFailedHandle(t *testing.T) {
	var armed, panicked atomic.Bool
	var admissionConn atomic.Pointer[pgx.Conn]
	f := newServerStreamFixture(t, handshakeFixtureOptions{configure: func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
			sql := strings.ToLower(strings.TrimSpace(data.SQL))
			if armed.Load() && strings.HasPrefix(sql, "insert into link_connections") {
				admissionConn.Store(conn)
			}
			if sql == "commit" && admissionConn.CompareAndSwap(conn, nil) && armed.CompareAndSwap(true, false) {
				panicked.Store(true)
				panic("The fixture interrupted the admission before commit.")
			}
			return ctx
		})
	}})
	machine, key := f.register(t, "Admission panic survivor")
	client := steadyWire(t, f, streamTestOptions{})
	previous := startHandshake(t, client, nil)
	submitProof(t, f, previous, machine.ID, key, 1)
	requireWelcome(t, previous)
	live := f.current(t, machine.ID)
	awaitHeld(t, f.hub, machine.ID, live.StreamID)

	broken := startHandshake(t, client, nil)
	hello := helloFrame(machine.ID, LinkRevision, 2)
	sendHandshake(t, broken, hello)
	challenge := requireChallenge(t, broken, f.serverKey, hello.GetHello())
	armed.Store(true)
	sendHandshake(t, broken, proofFrame(key, hello.GetHello(), challenge))
	frame, err := broken.Receive()
	if !panicked.Load() || err == nil || frame.GetWelcome() != nil {
		t.Fatal("The fixture did not interrupt the admission before Welcome.")
	}

	f.hub.mu.Lock()
	defer f.hub.mu.Unlock()
	var tracked int
	var previousTracked bool
	if entry := f.hub.machines[machine.ID]; entry != nil {
		tracked = len(entry.handles)
		for handle := range entry.handles {
			previousTracked = previousTracked || handle.streamID == live.StreamID
		}
	}
	if tracked != 1 {
		t.Errorf("The hub tracked %d handles after the panic, want 1.", tracked)
	}
	if !previousTracked {
		t.Error("The hub lost the previous stream during the failed admission.")
	}
}

type reviewGAdmissionCommitKey struct{}

func TestReviewGCommittedAdmissionPanicCleansExactStream(t *testing.T) {
	for _, tc := range []struct {
		name        string
		replacement bool
	}{
		{name: "delete the committed candidate"},
		{name: "preserve the replacement during delayed cleanup", replacement: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const (
				mark          = 42
				cleanupBudget = 3 * time.Second
			)
			var armCommit, holdDisconnect atomic.Bool
			var admissionConn atomic.Pointer[pgx.Conn]
			committed, disconnectStarted := make(chan struct{}), make(chan struct{})
			commitHold, releaseCommit := storetest.Signal(t)
			disconnectHold, releaseDisconnect := storetest.Signal(t)
			f := newServerStreamFixture(t, handshakeFixtureOptions{configure: func(cfg *pgxpool.Config) {
				cfg.ConnConfig.Tracer = streamReviewQueryTrace{
					handshakeQueryTrace: func(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
						sql := strings.ToLower(strings.TrimSpace(data.SQL))
						if armCommit.Load() && strings.HasPrefix(sql, "insert into link_connections") {
							admissionConn.Store(conn)
						}
						if sql == "commit" && admissionConn.CompareAndSwap(conn, nil) && armCommit.CompareAndSwap(true, false) {
							return context.WithValue(ctx, reviewGAdmissionCommitKey{}, true)
						}
						if strings.Contains(sql, "nowait") && holdDisconnect.CompareAndSwap(true, false) {
							close(disconnectStarted)
							<-disconnectHold
						}
						return ctx
					},
					end: func(ctx context.Context, data pgx.TraceQueryEndData) {
						if ctx.Value(reviewGAdmissionCommitKey{}) == true && data.Err == nil && data.CommandTag.String() == "COMMIT" {
							close(committed)
							<-commitHold
							panic("The fixture interrupted the admission after PostgreSQL confirmed commit.")
						}
					},
				}
			}})
			defer releaseCommit()
			defer releaseDisconnect()
			f.startRunner(t)
			machine, key := f.register(t, "Committed admission panic")
			f.admit(t, machine.ID, 1)
			storetest.Exec(t, f.obs, "UPDATE link_machines SET event_mark = $2 WHERE machine_id = $1", machine.ID, int64(mark))

			client := steadyWire(t, f, streamTestOptions{})
			broken := startHandshake(t, client, nil)
			hello := helloFrame(machine.ID, LinkRevision, 2)
			sendHandshake(t, broken, hello)
			challenge := requireChallenge(t, broken, f.serverKey, hello.GetHello())
			armCommit.Store(true)
			holdDisconnect.Store(tc.replacement)
			sendHandshake(t, broken, proofFrame(key, hello.GetHello(), challenge))
			storetest.Receive(t, "PostgreSQL confirming the admission commit", committed)
			var failedID string
			var generation int64
			if err := f.obs.QueryRow(t.Context(), "SELECT stream_id, generation FROM link_connections WHERE machine_id = $1", machine.ID).
				Scan(&failedID, &generation); err != nil || generation != 2 || failedID == "" {
				t.Fatal("The independent connection did not see the committed admission before the panic.")
			}
			releaseCommit()
			frame, err := broken.Receive()
			if err == nil || frame.GetWelcome() != nil {
				t.Fatal("The interrupted admission did not end before Welcome.")
			}

			if !tc.replacement {
				ctx, cancel := context.WithTimeout(t.Context(), cleanupBudget)
				defer cancel()
				ticker := time.NewTicker(10 * time.Millisecond)
				defer ticker.Stop()
				for {
					var exists bool
					err := f.obs.QueryRow(ctx, "SELECT EXISTS (SELECT FROM link_connections WHERE machine_id = $1 AND stream_id = $2)", machine.ID, failedID).Scan(&exists)
					if err == nil && !exists {
						return
					}
					if err != nil && ctx.Err() == nil {
						t.Fatal("The independent connection could not check the failed stream's row.")
					}
					select {
					case <-ctx.Done():
						t.Fatalf("The exact failed stream's committed connection row remained after %v.", cleanupBudget)
					case <-ticker.C:
					}
				}
			}

			queue := &f.svc.streaming.cleanup
			select {
			case <-disconnectStarted:
				queue.mu.Lock()
				_, exact := queue.pending[disconnectRequest{machineID: machine.ID, streamID: failedID}]
				queue.mu.Unlock()
				if !exact {
					t.Error("The disconnect worker did not receive the exact failed stream's cleanup obligation.")
				}
			case <-time.After(cleanupBudget):
				t.Error("Cleanup of the committed admission did not start before the replacement.")
			}
			later := startHandshake(t, client, nil)
			submitProof(t, f, later, machine.ID, key, 3)
			if welcome := requireWelcome(t, later); welcome.GetLastEventNumber() != mark {
				t.Error("The replacement did not start at the saved event mark.")
			}
			replacement := f.current(t, machine.ID)
			if replacement.StreamID == failedID || replacement.Generation != generation+1 {
				t.Fatal("The replacement did not establish a different connection row.")
			}
			sendHandshake(t, later, eventFrame(factsEvent(mark+1, "Replacement report")))
			requireEventAck(t, later, mark+1)
			releaseDisconnect()
			storetest.Poll(t, "the delayed disconnect finishing", func() bool {
				queue.mu.Lock()
				defer queue.mu.Unlock()
				return len(queue.pending) == 0
			})
			if current := f.current(t, machine.ID); current.StreamID != replacement.StreamID || current.Generation != replacement.Generation {
				t.Error("Cleanup of the failed admission changed the replacement's connection row.")
			}
			if saved := storetest.Scalar[int64](t, f.obs, "SELECT event_mark FROM link_machines WHERE machine_id = $1", machine.ID); saved != mark+1 {
				t.Error("Cleanup of the failed admission changed the replacement's saved event mark.")
			}
		})
	}
}
