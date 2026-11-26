package enroll

import (
	"io"
	"net/http"
	"sync/atomic"
	"testing"
)

func TestJoinPlainForbiddenIsNotTokenRefusal(t *testing.T) {
	useFakeClock(t)
	f := newFolder(t)
	var attempts atomic.Int32
	url := serve(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		http.Error(w, "Forbidden by an intermediary.", http.StatusForbidden)
	}))

	_, err := join(t, f, url, goodSpec(url).text(t), "host", nil)
	if attempts.Load() == 0 || err == nil {
		t.Fatal("enrollment did not reject the plain forbidden response")
	}
	diagnostic, _ := code(err)
	switch diagnostic {
	case "token_refused", "token_unknown", "token_expired", "token_revoked", "token_used", "machine_removed", "invalid_argument":
		t.Fatal("enrollment classified a plain forbidden response as a Link service refusal")
	}
}

func TestJoinPlainSuccessStopsAsInvalidReply(t *testing.T) {
	clock := useFakeClock(t)
	f := newFolder(t)
	var attempts atomic.Int32
	url := serve(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "This is a web page response.")
	}))

	_, err := join(t, f, url, goodSpec(url).text(t), "host", nil)
	if diagnostic, _ := code(err); diagnostic != "invalid_reply" {
		t.Error("enrollment did not return invalid_reply for a plain successful response")
	}
	if n := attempts.Load(); n != 1 || len(clock.waited()) != 0 {
		t.Errorf("enrollment made %d attempts, want one attempt without a retry wait", n)
	}
}
