package link

import (
	"context"
	"errors"
	"sync"
)

var ErrSuperseded = errors.New("A newer connection of this machine replaced this one.")

// Hub tracks stream candidates and the latest committed admission for each machine.
type Hub struct {
	mu       sync.Mutex
	machines map[string]*hubMachine
}

type hubMachine struct {
	gate       chan struct{}
	users      int
	handles    map[*Handle]struct{}
	current    *Handle
	generation int64
	removed    bool
}

type Handle struct {
	machineID, streamID string
	machine             *hubMachine
	ctx                 context.Context
	cancel              context.CancelCauseFunc
	wake                chan struct{}
	started, finished   bool
}

func NewHub() *Hub {
	return &Hub{machines: make(map[string]*hubMachine)}
}

func (h *Hub) Track(machineID, streamID string, ctx context.Context, cancel context.CancelCauseFunc) *Handle {
	h.mu.Lock()
	defer h.mu.Unlock()
	m := h.machines[machineID]
	if m == nil {
		m = &hubMachine{gate: make(chan struct{}, 1), handles: make(map[*Handle]struct{})}
		h.machines[machineID] = m
	}
	handle := &Handle{machineID: machineID, streamID: streamID, machine: m, ctx: ctx, cancel: cancel, wake: make(chan struct{}, 1)}
	m.handles[handle] = struct{}{}
	return handle
}

// enter keeps the machine record alive for the entire gate operation.
func (h *Hub) enter(handle *Handle) (func(), error) {
	m := handle.machine
	h.mu.Lock()
	if _, ok := m.handles[handle]; !ok {
		h.mu.Unlock()
		return nil, ErrNotCurrent
	}
	m.users++
	h.mu.Unlock()

	release := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		m.users--
		h.forgetIdle(handle.machineID, m)
	}
	select {
	case m.gate <- struct{}{}:
		leave := func() { <-m.gate; release() }
		if err := context.Cause(handle.ctx); err != nil {
			leave()
			return nil, err
		}
		h.mu.Lock()
		_, tracked := m.handles[handle]
		h.mu.Unlock()
		if !tracked {
			leave()
			return nil, ErrNotCurrent
		}
		return leave, nil
	case <-handle.ctx.Done():
		release()
		return nil, context.Cause(handle.ctx)
	}
}

func (h *Hub) forgetIdle(id string, m *hubMachine) {
	if m.users == 0 && len(m.handles) == 0 {
		delete(h.machines, id)
	}
}

func (h *Hub) Admit(handle *Handle, commit func() (int64, bool, error)) error {
	leave, err := h.enter(handle)
	if err != nil {
		return err
	}
	defer leave()
	generation, unknown, err := commit()

	h.mu.Lock()
	defer h.mu.Unlock()
	m := handle.machine
	if unknown {
		if m.current != nil {
			m.current.cancel(errDatabaseUnavailable)
		}
		m.current = nil
		return errDatabaseUnavailable
	}
	if err != nil {
		return err
	}
	if generation <= m.generation {
		return ErrSuperseded
	}
	if m.current != nil && m.current != handle {
		m.current.cancel(ErrSuperseded)
	}
	m.generation = generation
	_, tracked := m.handles[handle]
	m.current = nil
	if tracked {
		m.current = handle
	}
	return nil
}

func (h *Hub) BeginWelcome(handle *Handle) error {
	leave, err := h.enter(handle)
	if err != nil {
		return err
	}
	defer leave()
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := context.Cause(handle.ctx); err != nil {
		return err
	}
	if handle.machine.current != handle || handle.started {
		return ErrNotCurrent
	}
	handle.started = true
	return nil
}

func (h *Hub) FinishWelcome(handle *Handle) {
	h.mu.Lock()
	defer h.mu.Unlock()
	m := handle.machine
	if m.current == handle && handle.started {
		handle.finished = true
		if m.removed {
			handle.cancel(ErrUnknownMachine)
		}
	}
}

func (h *Hub) Unregister(handle *Handle) {
	h.mu.Lock()
	defer h.mu.Unlock()
	m := handle.machine
	if _, ok := m.handles[handle]; !ok {
		return
	}
	delete(m.handles, handle)
	if m.current == handle {
		m.current = nil
	}
	h.forgetIdle(handle.machineID, m)
}

func (h *Hub) Holds(machineID, streamID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	m := h.machines[machineID]
	return m != nil && m.current != nil && m.current.streamID == streamID &&
		m.current.ctx.Err() == nil && m.current.finished && !m.removed
}

func (h *Hub) Wake(machineID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m := h.machines[machineID]; m != nil {
		m.signal()
	}
}

func (h *Hub) WakeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, m := range h.machines {
		m.signal()
	}
}

func (m *hubMachine) signal() {
	if m.current != nil {
		select {
		case m.current.wake <- struct{}{}:
		default:
		}
	}
}

func (h *Hub) EndRemoved(machineID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m := h.machines[machineID]; m != nil {
		m.removed = true
		if m.current != nil && m.current.finished {
			m.current.cancel(ErrUnknownMachine)
		}
	}
}

func (h *Handle) Wake() <-chan struct{} { return h.wake }
