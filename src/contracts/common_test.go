package contracts_test

import (
	"testing"

	_ "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
)

// TestCommonMatchesItsDescription holds common.proto to its description (item 3).
func TestCommonMatchesItsDescription(t *testing.T) {
	matchDescription(t, "testdata/golden/common.txt")
}

// TestTokenReplyMatchesItsDescription holds the reply that makes a join token to its description (item 3).
func TestTokenReplyMatchesItsDescription(t *testing.T) {
	matchDescription(t, "testdata/golden/token-reply.txt")
}
