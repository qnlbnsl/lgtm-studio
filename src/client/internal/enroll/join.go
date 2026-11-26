package enroll

// This file is a stub of the test writer. The builder of work item 2 replaces every body
// and keeps the exported names and the two hooks below.

import (
	"context"
	"crypto/ed25519"
	"errors"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/state"
)

var (
	// ErrEnrollmentExists reports that the folder already holds an enrollment file.
	ErrEnrollmentExists = errors.New("enroll: enrollment exists")
	// ErrEnrollmentMissing reports that the folder holds no enrollment file.
	ErrEnrollmentMissing = errors.New("enroll: enrollment is missing")
	// ErrEnrollmentInvalid reports that the enrollment file cannot be trusted.
	ErrEnrollmentInvalid = errors.New("enroll: enrollment is invalid")
)

// The tests replace these two hooks so that no test waits for a real delay. Join reads the
// time only through joinNow and waits only through joinSleep, which returns early with the
// error of the context. The stub of the test writer does not use them.
var (
	joinNow   = time.Now
	joinSleep = func(ctx context.Context, d time.Duration) error { return ctx.Err() }
)

// Server is what a machine keeps of its enrollment.
type Server struct {
	Address              string
	MachineID            string
	ClientPublicKey      ed25519.PublicKey
	PublicKey            ed25519.PublicKey
	CertificateKeySHA256 []byte
}

// JoinError carries a fixed diagnostic code.
type JoinError struct{ Code string }

func (e JoinError) Error() string { return "enroll: join refused: " + e.Code }

// LoadServer reads the saved enrollment. Stub of the test writer.
func LoadServer(f *state.Folder) (Server, error) { return Server{}, nil }

// SaveServer commits a verified enrollment. Stub of the test writer.
func SaveServer(f *state.Folder, server Server) error { return nil }

// Join trades the token for an enrollment without saving it. Stub of the test writer.
func Join(ctx context.Context, f *state.Folder, address, token, hostName string, onRetry func()) (Server, error) {
	return Server{}, nil
}
