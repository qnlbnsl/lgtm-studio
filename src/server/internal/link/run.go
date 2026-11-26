package link

import (
	"context"
	"crypto/ed25519"
	"log/slog"
	"net/netip"
	"slices"
	"sync"
	"time"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

type Options struct {
	PrivateStack bool
	Logger       *slog.Logger
	Seams        Seams
	Commands     CommandHooks
}

type Seams struct {
	AfterProof, AfterAdmission func(context.Context, string)
	BeforeEventAck             func(context.Context, string, uint64)
}

type CommandHooks interface {
	SendNext(ctx context.Context, machineID, streamID string, send func(*linkv1.Command) error) (sent bool, err error)
	AcceptResult(ctx context.Context, machineID, streamID string, result *linkv1.CommandResult) error
}

func NewStreamingService(life context.Context, s *store.Store, t *machines.Tokens, r *machines.Registry, state *State, hub *Hub, key ed25519.PrivateKey, options Options) *Service {
	service := NewService(s, t, key.Public().(ed25519.PublicKey))
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	service.streaming = &streamingService{
		life: life, registry: r, state: state, hub: hub, key: slices.Clone(key), options: options,
		cleanup: newDisconnectQueue(), cleanupTimeout: 2 * time.Second, cleanupRetry: time.Second,
		feedTimeout: 5 * time.Second, feedRetry: time.Second,
	}
	return service
}

// streamContext lets the request and service lifetime independently end the stream.
func (s *Service) streamContext(parent context.Context) (context.Context, context.CancelCauseFunc) {
	ctx, cancel := context.WithCancelCause(parent)
	life := s.streaming.life
	stop := context.AfterFunc(life, func() { cancel(errLinkStopping) })
	if life.Err() != nil {
		cancel(errLinkStopping)
	}
	return ctx, func(cause error) {
		cancel(cause)
		stop()
	}
}

func peerAddress(remoteAddr string, privateStack bool) string {
	if privateStack {
		return ""
	}
	peer, err := netip.ParseAddrPort(remoteAddr)
	if err != nil {
		return ""
	}
	return peer.Addr().WithZone("").Unmap().String()
}

// Run starts maintenance once and joins its workers before it returns.
func (s *Service) Run(ctx context.Context, follower *store.Follower) {
	link := s.streaming
	if link == nil || !link.running.CompareAndSwap(false, true) {
		return
	}
	ctx, cancel := s.streamContext(ctx)
	defer cancel(nil)
	var workers sync.WaitGroup
	workers.Go(func() { link.runCleanup(ctx, &link.cleanup.first) })
	workers.Go(func() { link.runCleanup(ctx, &link.cleanup.retry) })
	if follower != nil {
		workers.Go(func() { s.follow(ctx, follower) })
	}
	workers.Wait()
}

type disconnectRequest struct {
	machineID, streamID string
}

type disconnectJob struct {
	request disconnectRequest
	readyAt time.Time
}

type disconnectLane struct {
	jobs []disconnectJob
	wake chan struct{}
}

// disconnectQueue coalesces exact stream identifiers and permits only one attempt per machine.
type disconnectQueue struct {
	mu           sync.Mutex
	first, retry disconnectLane
	pending      map[disconnectRequest]struct{}
	active       map[string]struct{}
}

func newDisconnectQueue() disconnectQueue {
	return disconnectQueue{
		first:   disconnectLane{wake: make(chan struct{}, 1)},
		retry:   disconnectLane{wake: make(chan struct{}, 1)},
		pending: make(map[disconnectRequest]struct{}),
		active:  make(map[string]struct{}),
	}
}

func (s *Service) queueDisconnect(machineID, streamID string) {
	q := &s.streaming.cleanup
	q.mu.Lock()
	defer q.mu.Unlock()
	request := disconnectRequest{machineID: machineID, streamID: streamID}
	if _, queued := q.pending[request]; queued {
		return
	}
	q.pending[request] = struct{}{}
	q.first.jobs = append(q.first.jobs, disconnectJob{request: request})
	q.first.signal()
}

func (lane *disconnectLane) signal() {
	select {
	case lane.wake <- struct{}{}:
	default:
	}
}

func (q *disconnectQueue) next(lane *disconnectLane) (disconnectRequest, time.Time, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := time.Now()
	for i, job := range lane.jobs {
		if _, busy := q.active[job.request.machineID]; busy {
			continue
		}
		if job.readyAt.After(now) {
			return disconnectRequest{}, job.readyAt, false
		}
		lane.jobs = slices.Delete(lane.jobs, i, i+1)
		q.active[job.request.machineID] = struct{}{}
		return job.request, time.Time{}, true
	}
	return disconnectRequest{}, time.Time{}, false
}

func (q *disconnectQueue) finish(request disconnectRequest, retryAfter time.Duration, retry bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.active, request.machineID)
	if retry {
		q.retry.jobs = append(q.retry.jobs, disconnectJob{request: request, readyAt: time.Now().Add(retryAfter)})
	} else {
		delete(q.pending, request)
	}
	q.first.signal()
	q.retry.signal()
}

func (s *streamingService) runCleanup(ctx context.Context, lane *disconnectLane) {
	for ctx.Err() == nil {
		request, readyAt, ok := s.cleanup.next(lane)
		if ok {
			attempt, cancel := context.WithTimeout(ctx, s.cleanupTimeout)
			_, err := s.state.Disconnect(attempt, request.machineID, request.streamID)
			cancel()
			s.cleanup.finish(request, s.cleanupRetry, err != nil && ctx.Err() == nil)
			continue
		}
		var ready <-chan time.Time
		if !readyAt.IsZero() {
			ready = time.After(time.Until(readyAt))
		}
		select {
		case <-ctx.Done():
			return
		case <-lane.wake:
		case <-ready:
		}
	}
}

// The controls observe subscription setup without changing the production feed loop.
type runnerControlsKey struct{}

type runnerControls struct {
	subscribed func(*store.Subscription)
	started    func(*store.Subscription)
}

func (s *Service) follow(ctx context.Context, follower *store.Follower) {
	controls, _ := ctx.Value(runnerControlsKey{}).(*runnerControls)
	for ctx.Err() == nil {
		s.followAttempt(ctx, follower, controls)
		if ctx.Err() != nil {
			return
		}
		s.streaming.hub.WakeAll()
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.streaming.feedRetry):
		}
	}
}

func (s *Service) followAttempt(ctx context.Context, follower *store.Follower, controls *runnerControls) {
	sub, err := follower.Subscribe(ctx)
	if err != nil {
		return
	}
	defer sub.Close()
	if controls != nil && controls.subscribed != nil {
		controls.subscribed(sub)
	}

	read, cancel := context.WithTimeout(sub.Context(), s.streaming.feedTimeout)
	head, err := s.store.Snapshot(read, func(context.Context, *store.ReadTx) error { return nil })
	cancel()
	if err != nil || sub.Start(head) != nil {
		return
	}
	hub := s.streaming.hub
	hub.WakeAll()
	if controls != nil && controls.started != nil {
		controls.started(sub)
	}
	for {
		row, err := sub.Next()
		if err != nil {
			return
		}
		switch {
		case row.Topic == "machine_outbox":
			hub.Wake(row.Key)
		case row.Topic == "machines" && row.Change == store.Removed:
			hub.EndRemoved(row.Key)
		}
		if err := sub.Ack(row.Position); err != nil {
			return
		}
	}
}
