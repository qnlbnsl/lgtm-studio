package api

import (
	"context"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
)

// SessionVerifier is a test-writer stub.
func SessionVerifier(ctx context.Context) (auth.Verifier, bool) {
	return auth.Verifier{}, false
}
