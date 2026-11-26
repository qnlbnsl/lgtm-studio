package link

import (
	"context"
	"log/slog"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/config"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/harness"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/state"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

type RunOptions struct {
	Server      enroll.Server
	Credential  *enroll.Credential
	Folder      *state.Folder
	Settings    config.Settings
	ScanOptions harness.ScanOptions
	Version     string
	Logger      *slog.Logger
}

type RunOutcome uint8

const (
	RunStopped RunOutcome = 1
	RunRestart RunOutcome = 2
)

// Run is a stub of the test writer.
func Run(ctx context.Context, opts RunOptions) (RunOutcome, error) {
	return 0, nil
}

// newRunLinkClient is a stub of the test writer.
var newRunLinkClient func(enroll.Server) (linkv1connect.LinkServiceClient, func(), error)

// runReportProbes is a stub of the test writer.
var runReportProbes reportProbes

// runSessionIO and wrapRunSessionIO are stubs of the test writer.
type runSessionIO struct {
	send    func(*linkv1.OpenRequest) error
	receive func() (*linkv1.OpenResponse, error)
}

var wrapRunSessionIO func(context.Context, *session, runSessionIO) runSessionIO
