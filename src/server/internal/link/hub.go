package link

import (
	"context"
	"errors"
)

// These declarations are stubs of the test writer.
type Hub struct{}
type Handle struct {
	machineID, streamID string
}

var ErrSuperseded = errors.New("A newer connection of this machine replaced this one.")

func NewHub() *Hub { return &Hub{} }
func (*Hub) Track(string, string, context.Context, context.CancelCauseFunc) *Handle {
	return &Handle{}
}
func (*Hub) Admit(*Handle, func() (int64, bool, error)) error { return nil }
func (*Hub) BeginWelcome(*Handle) error                       { return nil }
func (*Hub) FinishWelcome(*Handle)                            {}
func (*Hub) Unregister(*Handle)                               {}
func (*Hub) Holds(string, string) bool                        { return false }
func (*Hub) Wake(string)                                      {}
func (*Hub) WakeAll()                                         {}
func (*Hub) EndRemoved(string)                                {}
func (*Handle) Wake() <-chan struct{}                         { return nil }
