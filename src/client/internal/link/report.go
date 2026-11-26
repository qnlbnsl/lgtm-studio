package link

import (
	"context"
	"log/slog"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/config"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/harness"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/sysinfo"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

// reporter is a stub of the test writer.
type reporter struct{}

// reportProbes is a stub of the test writer.
type reportProbes struct {
	now       func() time.Time
	readFacts func(clientVersion string) (sysinfo.Facts, error)
	readLoad  func() (sysinfo.Load, error)
	scan      func(context.Context, harness.ScanOptions) []harness.Found
}

// newReporter is a stub of the test writer.
func newReporter(_ config.Settings, _ string, _ harness.ScanOptions, _ func([]byte) (uint64, error), _ *slog.Logger, _ reportProbes) *reporter {
	return &reporter{}
}

// captureFacts is a stub of the test writer.
func (*reporter) captureFacts() error { return nil }

// heartbeat is a stub of the test writer.
func (*reporter) heartbeat(_ bool) *linkv1.Heartbeat { return &linkv1.Heartbeat{} }

// scanHarnesses is a stub of the test writer.
func (*reporter) scanHarnesses(_ context.Context, _ bool) (bool, error) { return false, nil }

// flush is a stub of the test writer.
func (*reporter) flush() error { return nil }
