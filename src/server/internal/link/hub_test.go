package link

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func trackHubStream(t *testing.T, h *Hub, id string) (*Handle, context.Context, context.CancelCauseFunc) {
	t.Helper()
	ctx, cancel := context.WithCancelCause(t.Context())
	handle := h.Track("machine", id, ctx, cancel)
	if handle == nil {
		t.Fatal("Track did not return a handle.")
	}
	t.Cleanup(func() {
		cancel(nil)
		h.Unregister(handle)
	})
	return handle, ctx, cancel
}

func publishHubStream(t *testing.T, h *Hub, handle *Handle, generation int64) {
	t.Helper()
	called := false
	err := h.Admit(handle, func() (int64, bool, error) {
		called = true
		return generation, false, nil
	})
	if err != nil || !called {
		t.Fatal("Admit did not run and publish the successful commit.")
	}
}

func finishHubWelcome(t *testing.T, h *Hub, handle *Handle) {
	t.Helper()
	if err := h.BeginWelcome(handle); err != nil {
		t.Fatal("The current handle could not begin Welcome.")
	}
	h.FinishWelcome(handle)
}

func TestHubHoldsOnlyExactLiveStream(t *testing.T) {
	h := NewHub()
	if h.Holds("machine", "a") {
		t.Fatal("A missing handle was held.")
	}
	a, _, cancelA := trackHubStream(t, h, "a")
	if h.Holds("machine", "a") {
		t.Fatal("A tracked candidate was held before admission.")
	}
	publishHubStream(t, h, a, 1)
	if err := h.BeginWelcome(a); err != nil {
		t.Fatal("The admitted candidate could not start Welcome.")
	}
	if h.Holds("machine", "a") {
		t.Fatal("A started Welcome granted liveness before it finished.")
	}
	h.FinishWelcome(a)
	if !h.Holds("machine", "a") || h.Holds("machine", "b") || h.Holds("other", "a") {
		t.Fatal("The hub did not hold exactly the live machine and stream pair.")
	}
	cancelA(errors.New("The machine went away."))
	if h.Holds("machine", "a") {
		t.Fatal("A canceled handle remained live before unregister.")
	}
	b, _, _ := trackHubStream(t, h, "b")
	publishHubStream(t, h, b, 2)
	finishHubWelcome(t, h, b)
	h.Unregister(b)
	if h.Holds("machine", "b") {
		t.Fatal("An unregistered handle remained live.")
	}
}

func TestOlderGenerationCannotReplaceNewer(t *testing.T) {
	h := NewHub()
	a, ctxA, _ := trackHubStream(t, h, "a")
	b, _, _ := trackHubStream(t, h, "b")
	late, _, _ := trackHubStream(t, h, "late")
	publishHubStream(t, h, a, 1)
	publishHubStream(t, h, b, 2)
	finishHubWelcome(t, h, b)
	if !errors.Is(context.Cause(ctxA), ErrSuperseded) || !errors.Is(h.BeginWelcome(a), ErrSuperseded) {
		t.Fatal("The older generation could start Welcome after replacement.")
	}
	h.Unregister(b)
	if !errors.Is(h.BeginWelcome(a), ErrSuperseded) {
		t.Fatal("Closing the newer handle restored the older Welcome permit.")
	}
	if err := h.Admit(late, func() (int64, bool, error) { return 1, false, nil }); !errors.Is(err, ErrSuperseded) {
		t.Fatal("The high-water generation was lost while an older candidate remained tracked.")
	}
	c, _, _ := trackHubStream(t, h, "c")
	publishHubStream(t, h, c, 3)
	finishHubWelcome(t, h, c)
	h.Unregister(a)
	if !h.Holds("machine", "c") {
		t.Fatal("Unregistering the displaced handle changed the current handle.")
	}
}

func TestHubFirstWriteAndAdmissionOrder(t *testing.T) {
	h := NewHub()
	a, _, _ := trackHubStream(t, h, "a")
	b, _, _ := trackHubStream(t, h, "b")
	candidate, _, cancelCandidate := trackHubStream(t, h, "cancel")
	if err := h.BeginWelcome(a); err == nil {
		t.Fatal("Welcome began before any admission had published.")
	}
	publishHubStream(t, h, a, 1)

	inCommit := make(chan struct{})
	releaseCommit, commit := storetest.Signal(t)
	defer commit()
	admitted := storetest.Start(t, func() error {
		return h.Admit(b, func() (int64, bool, error) {
			close(inCommit)
			<-releaseCommit
			return 2, false, nil
		})
	})
	storetest.ReceiveFrom(t, admitted, "the replacement entering commit", inCommit)

	starting := make(chan struct{})
	check := storetest.Start(t, func() error {
		close(starting)
		return h.BeginWelcome(a)
	})
	storetest.Receive(t, "the older Welcome checking entry", starting)
	select {
	case <-check.Done():
		t.Fatal("The older Welcome left the gate while the replacement commit was unresolved.")
	case <-time.After(250 * time.Millisecond):
	}

	// A canceled candidate and a different machine both make progress while this gate is held.
	waiting := make(chan struct{})
	checkCanceled := storetest.Start(t, func() error {
		close(waiting)
		return h.BeginWelcome(candidate)
	})
	storetest.Receive(t, "the pending Welcome attempting entry", waiting)
	cancelCandidate(context.Canceled)
	select {
	case <-checkCanceled.Done():
		if checkCanceled.Result() == nil {
			t.Error("A canceled, unpublished candidate started Welcome.")
		}
	case <-time.After(time.Second):
		t.Fatal("A canceled candidate did not leave the occupied machine gate.")
	}

	otherCtx, cancelOther := context.WithCancelCause(t.Context())
	defer cancelOther(nil)
	other := storetest.Start(t, func() error {
		handle := h.Track("other", "independent", otherCtx, cancelOther)
		defer h.Unregister(handle)
		return h.Admit(handle, func() (int64, bool, error) { return 1, false, nil })
	})
	select {
	case <-other.Done():
		if other.Result() != nil {
			t.Error("An unrelated admission failed while the first machine was committing.")
		}
	case <-time.After(time.Second):
		t.Fatal("An unresolved admission blocked a different machine.")
	}

	commit()
	if admitted.Wait(t) != nil || !errors.Is(check.Wait(t), ErrSuperseded) {
		t.Fatal("The older Welcome passed the gate after replacement published.")
	}
	c, _, _ := trackHubStream(t, h, "c")
	if err := h.BeginWelcome(b); err != nil {
		t.Fatal("The new Welcome did not enter the transport.")
	}
	defer h.FinishWelcome(b)
	admittedC := storetest.Start(t, func() error {
		return h.Admit(c, func() (int64, bool, error) { return 3, false, nil })
	})
	select {
	case <-admittedC.Done():
		if admittedC.Result() != nil {
			t.Fatal("Admission failed during a held transport write.")
		}
	case <-time.After(time.Second):
		t.Fatal("The first-write permit held the admission gate across the transport.")
	}
}

func TestHubUnknownCommitStopsOlderHandle(t *testing.T) {
	h := NewHub()
	a, ctxA, _ := trackHubStream(t, h, "a")
	publishHubStream(t, h, a, 1)
	finishHubWelcome(t, h, a)
	b, _, _ := trackHubStream(t, h, "b")
	failure := errors.New("The admission reply was lost.")
	if err := h.Admit(b, func() (int64, bool, error) { return 0, true, failure }); err == nil {
		t.Fatal("An uncertain admission appeared to succeed.")
	}
	if h.Holds("machine", "a") || h.Holds("machine", "b") || context.Cause(ctxA) == nil ||
		errors.Is(context.Cause(ctxA), ErrSuperseded) || h.BeginWelcome(a) == nil || h.BeginWelcome(b) == nil {
		t.Fatal("An uncertain commit left an older or uncertain handle live or able to start Welcome.")
	}
	c, _, _ := trackHubStream(t, h, "c")
	publishHubStream(t, h, c, 3)
	finishHubWelcome(t, h, c)
	if !h.Holds("machine", "c") {
		t.Fatal("A later confirmed admission could not restore liveness.")
	}
}

func TestHubWakeCoalesces(t *testing.T) {
	h := NewHub()
	a, ctxA, _ := trackHubStream(t, h, "a")
	publishHubStream(t, h, a, 1)
	finishHubWelcome(t, h, a)
	select {
	case <-a.Wake():
	default:
	}
	h.Wake("other")
	select {
	case <-a.Wake():
		t.Fatal("Waking a different machine signaled this handle.")
	default:
	}
	woke := make(chan struct{})
	go func() {
		h.Wake("machine")
		h.WakeAll()
		close(woke)
	}()
	select {
	case <-woke:
	case <-time.After(time.Second):
		select {
		case <-a.Wake():
		default:
		}
		t.Fatal("Waking twice waited for a reader instead of coalescing.")
	}
	select {
	case <-a.Wake():
	default:
		t.Fatal("Neither wake reached the current handle.")
	}
	select {
	case <-a.Wake():
		t.Fatal("Two wakes without a receive produced two pending signals.")
	default:
	}
	h.EndRemoved("machine")
	if ctxA.Err() == nil || h.Holds("machine", "a") {
		t.Fatal("Removal did not end a finished Welcome immediately.")
	}

	// A distinct machine can be removed while its admission owns the gate.
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	b := h.Track("pending", "b", ctx, cancel)
	defer h.Unregister(b)
	inCommit := make(chan struct{})
	wait, release := storetest.Signal(t)
	defer release()
	admit := storetest.Start(t, func() error {
		return h.Admit(b, func() (int64, bool, error) {
			close(inCommit)
			<-wait
			return 1, false, nil
		})
	})
	storetest.ReceiveFrom(t, admit, "admission holding the pending machine gate", inCommit)
	removed := storetest.Start(t, func() struct{} {
		h.Wake("pending")
		h.EndRemoved("pending")
		return struct{}{}
	})
	select {
	case <-removed.Done():
	case <-time.After(time.Second):
		t.Fatal("Wake or removal waited for the machine's admission gate.")
	}
	release()
	if admit.Wait(t) != nil || ctx.Err() != nil || h.BeginWelcome(b) != nil {
		t.Fatal("Removal held during admission prevented an already committed Welcome.")
	}
	if h.Holds("pending", "b") {
		t.Fatal("A pending removal granted liveness.")
	}
	h.FinishWelcome(b)
	if ctx.Err() == nil || h.Holds("pending", "b") {
		t.Fatal("Finishing Welcome did not deliver the pending removal.")
	}
}
