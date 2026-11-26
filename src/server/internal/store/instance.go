package store

import (
	"context"
	"time"
)

// These are the test writer's stubs for the server instance lock.
var ErrServerInstanceBusy = NewDomainError("Another server is already using this database.")

type InstanceLock struct{}

func (*Store) AcquireServerInstance(context.Context, time.Duration) (*InstanceLock, error) {
	return nil, NewDomainError("The instance lock stub has not been implemented.")
}

func (*InstanceLock) Check(context.Context) error { return nil }
func (*InstanceLock) Close(context.Context) error { return nil }
