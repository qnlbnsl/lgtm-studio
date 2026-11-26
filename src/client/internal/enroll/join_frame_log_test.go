package enroll

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"io"
	"log"
	"os"
	"os/exec"
	"testing"
)

func TestJoinFrameDebugDoesNotLogCredentials(t *testing.T) {
	const childOrigin = "LGTM_TEST_JOIN_FRAME_DEBUG_ORIGIN"
	if url := os.Getenv(childOrigin); url != "" {
		useFakeClock(t)
		// The client may decline enrollment while payload diagnostics are enabled.
		_, _ = join(t, newFolder(t), url, goodSpec(url).text(t), "host", nil)
		return
	}

	// Only the client runs in the child. Silence the fixture server's own frame logs
	// if the parent test process also inherited frame debugging.
	previousLog := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(previousLog) })
	_, url := startLink(t, answerGood)
	ctx, cancel := context.WithTimeout(t.Context(), 2*callBudget)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestJoinFrameDebugDoesNotLogCredentials$")
	cmd.Env = append(os.Environ(), childOrigin+"="+url, "GODEBUG=http2debug=2")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatal("the enrollment fixture subprocess failed")
	}

	token := goodSpec(url).text(t)
	for _, value := range []string{
		token, token[len("lgtm1_"):], string(fakeSecret),
		hex.EncodeToString(fakeSecret), base64.StdEncoding.EncodeToString(fakeSecret),
		base64.RawStdEncoding.EncodeToString(fakeSecret), base64.URLEncoding.EncodeToString(fakeSecret),
		base64.RawURLEncoding.EncodeToString(fakeSecret),
	} {
		if bytes.Contains(stderr.Bytes(), []byte(value)) {
			t.Fatal("frame debugging exposed the synthetic enrollment credential on standard error")
		}
	}
}
