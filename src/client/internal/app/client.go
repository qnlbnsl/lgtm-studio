// Package app owns the enrollment and foreground client lifecycle.
package app

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/config"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/harness"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/state"
)

// Options configures the client lifecycle.
type Options struct {
	StateDir string
	Version  string
	Getenv   func(string) string
	Stderr   io.Writer

	OnFolderInspectionStart func(context.Context, string, string)
	AfterFolderReport       func(context.Context, uint64) error
}

var (
	closeCreatedStore = (*link.Store).Close
	runLink           = link.Run
)

// Join enrolls the machine and publishes its identity after its store is ready.
func Join(ctx context.Context, opts Options, address, token string) (result error) {
	opts = withDefaults(opts)
	filter := link.NewOutgoingFilter()
	// An enrollment token has room for at least its 32-byte secret.
	if body, ok := strings.CutPrefix(token, "lgtm1_"); ok && len(body) >= base64.RawURLEncoding.EncodedLen(32) {
		filter.AddSecrets(token, body)
	}
	logger := newOutgoingLogger(opts.Stderr, filter)
	folder, err := openState(opts)
	if err != nil {
		return joinFailure(logger, "state_open_failed", err.Error())
	}
	defer func() {
		if err := folder.Close(); err != nil && result == nil {
			result = joinFailure(logger, "state_close_failed", "Could not close the client state folder.")
		}
	}()

	hostName, _ := os.Hostname()
	_, keyErr := os.Lstat(folder.Paths().Key)
	pending := keyErr == nil
	server, err := enroll.Join(ctx, folder, address, token, hostName, func() {
		pending = true
		logger.Info("join_retrying", "message", "The server has not answered. Retrying enrollment.")
	})
	if err != nil {
		return reportJoinError(logger, err, address, pending)
	}

	store, err := link.CreateStore(folder, link.StoreOptions{})
	if err != nil {
		return joinFailure(logger, "store_failed", "Could not prepare client.db.")
	}
	if err := closeCreatedStore(store); err != nil {
		return joinFailure(logger, "store_failed", "Could not close client.db.")
	}
	if err := enroll.SaveServer(folder, server); err != nil {
		return joinFailure(logger, "identity_save_failed", "Could not save the verified enrollment.")
	}
	return nil
}

// Run connects a previously enrolled machine and returns its process status.
func Run(ctx context.Context, opts Options) (status int) {
	opts = withDefaults(opts)
	filter := link.NewRefreshingOutgoingFilter(claudeSignInReader(opts.Getenv))
	logger := newOutgoingLogger(opts.Stderr, filter)
	folder, err := openState(opts)
	if err != nil {
		logger.Error("client_start_failed", "message", err.Error())
		return 1
	}
	var outcome link.RunOutcome
	defer func() {
		if err := folder.Close(); err != nil && outcome != link.RunRecoveryStop {
			logger.Error("client_stop_failed", "message", "Could not close the client state folder.")
			status = 1
		}
	}()

	paths := folder.Paths()
	server, err := enroll.LoadServer(folder)
	if err == nil {
		err = state.SyncDir(paths.Dir)
	}
	if err != nil {
		logger.Error("client_start_failed", "message", "No usable saved enrollment. Run lgtm-client join to enroll this machine.")
		return 1
	}
	credential, err := enroll.LoadVerified(paths.Key, server.ClientPublicKey, func(values ...string) {
		filter.AddSecrets(secretForms(values...)...)
	})
	if err != nil {
		logger.Error("client_start_failed", "message", "The saved enrollment has no matching machine key. Run lgtm-client join with a fresh state folder.")
		return 1
	}

	settings, err := config.LoadSettings(paths.Settings, opts.Getenv("HOME"), paths.Dir)
	if err != nil {
		logger.Error("client_start_failed", "message", "Could not load the client settings.")
		return 1
	}
	logger.Info("client_starting", "state_dir", paths.Dir, "repositories_folder", settings.RepositoriesFolder)
	outcome, err = runLink(ctx, link.RunOptions{
		Server: server, Credential: credential, Folder: folder, Settings: settings,
		ScanOptions: harness.DefaultScanOptions(opts.Getenv), Version: opts.Version, Logger: logger,
		Filter:                  filter,
		OnFolderInspectionStart: opts.OnFolderInspectionStart,
		AfterFolderReport:       opts.AfterFolderReport,
	})
	if err != nil {
		return 1
	}
	switch outcome {
	case link.RunStopped, link.RunRecoveryStop:
		return 0
	case link.RunRestart:
		logger.Info("link_restart_requested", "message", "The server requested a client restart.")
		return 75
	default:
		logger.Error("link_stopped", "code", "invalid_outcome", "message", "The link returned an invalid outcome.")
		return 1
	}
}

func withDefaults(opts Options) Options {
	if opts.Getenv == nil {
		opts.Getenv = os.Getenv
	}
	if opts.Version == "" {
		opts.Version = "0.0.0-dev"
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}
	return opts
}

func openState(opts Options) (*state.Folder, error) {
	dir := opts.StateDir
	if dir == "" {
		var err error
		dir, err = config.StateDir(opts.Getenv)
		if err != nil {
			return nil, errors.New("Set LGTM_CLIENT_STATE_DIR to an absolute state folder path.")
		}
	} else if !filepath.IsAbs(dir) {
		return nil, errors.New("The client state folder path must be absolute.")
	}
	folder, err := state.Open(dir)
	switch {
	case errors.Is(err, state.ErrFolderLocked):
		return nil, errors.New("Another client is using the state folder.")
	case err != nil:
		return nil, errors.New("Could not open the private client state folder.")
	default:
		return folder, nil
	}
}

func joinFailure(logger *slog.Logger, code, message string) error {
	logger.Error("join_failed", "code", code, "message", message)
	return errors.New(message)
}

func reportJoinError(logger *slog.Logger, err error, address string, pending bool) error {
	var refusal enroll.JoinError
	switch {
	case errors.As(err, &refusal):
		code, message := joinRefusalDiagnostic(refusal.Code, address, pending)
		logger.Error("join_refused", "code", code, "message", message)
		return errors.New(message)
	case errors.Is(err, enroll.ErrEnrollmentExists):
		const message = "This folder is already joined."
		logger.Error("join_refused", "code", "already_joined", "message", message)
		return errors.New(message)
	case errors.Is(err, context.Canceled):
		return joinFailure(logger, "join_canceled", "Enrollment was canceled.")
	case errors.Is(err, context.DeadlineExceeded):
		return joinFailure(logger, "join_timeout", "Enrollment timed out.")
	case errors.Is(err, enroll.CheckFrameDebug()):
		return joinFailure(logger, "frame_debug_enabled", "Disable HTTP/2 frame debugging before enrolling.")
	default:
		return joinFailure(logger, "join_failed", "Could not enroll this machine.")
	}
}

func joinRefusalDiagnostic(code, address string, pending bool) (string, string) {
	switch code {
	case "token_unknown":
		return code, "The server did not recognize the join token."
	case "token_expired":
		if pending {
			return code, "The join token expired. The server may know the saved key. Match the waiting or approved machine by host name and join time. Remove it if present, then use a new token and a fresh folder."
		}
		return code, "The join token expired."
	case "token_revoked":
		return code, "The join token was revoked."
	case "token_used":
		return code, "The join token was already used."
	case "machine_removed":
		return code, "The server has removed the machine that used this join token."
	case "invalid_argument":
		return code, "The server rejected the enrollment request."
	case "invalid_token":
		return code, "The join token is invalid."
	case "origin_mismatch":
		return code, "The address does not match the join token."
	case "certificate_key_mismatch":
		return code, "The server certificate key does not match the join token."
	case "server_key_mismatch":
		// Enrollment returns this error only after validating the origin.
		origin, _ := url.Parse(address)
		host := origin.Hostname()
		ip, _ := netip.ParseAddr(host)
		if strings.EqualFold(host, "localhost") || ip.Unmap().IsLoopback() {
			return code, "The server signing key does not match the join token. Revoke the token."
		}
		return code, "The server signing key does not match the join token."
	case "invalid_reply":
		return code, "The server sent an invalid enrollment reply."
	case "join_timeout":
		return code, "The server did not answer before enrollment timed out."
	default:
		return "token_refused", "The server refused the join token."
	}
}
