package linktest

import (
	"encoding/binary"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/qnlbnsl/lgtm-studio/src/client/clienttest"
)

func copyProjectHTTPClient(signedIn *http.Client) *http.Client {
	client := *signedIn
	client.Timeout = 35 * time.Second
	return &client
}

func TestProjectHTTPClientKeepsSessionAndExtendsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sign-in" {
			http.SetCookie(w, &http.Cookie{Name: "fixture", Value: "signed-in", Path: "/"})
		} else if cookie, err := r.Cookie("fixture"); err != nil || cookie.Value != "signed-in" {
			http.Error(w, "Session missing.", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	signedIn := &http.Client{Transport: server.Client().Transport, Jar: jar, Timeout: 10 * time.Second}
	call := func(client *http.Client, route string) {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+route, nil)
		if err != nil {
			t.Fatal(err)
		}
		reply, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer reply.Body.Close()
		if reply.StatusCode != http.StatusNoContent {
			t.Fatalf("The fixture website returned %d.", reply.StatusCode)
		}
	}
	call(signedIn, "/sign-in")
	copied := copyProjectHTTPClient(signedIn)
	if copied == signedIn || copied.Transport != signedIn.Transport ||
		copied.Timeout <= 25*time.Second || signedIn.Timeout != 10*time.Second {
		t.Fatal("The copied client did not retain its transport and independently extend its timeout.")
	}
	call(copied, "/session-probe")
}

// Reading the closed outbox distinguishes consumption from delivery by the relay.
func requireAcknowledgedOutbox(t *testing.T, opts clienttest.Options, through uint64) {
	t.Helper()
	withClosedClientDB(t, opts, false, func(db *bolt.DB) error {
		return db.View(func(tx *bolt.Tx) error {
			return checkAcknowledgedOutbox(tx, through)
		})
	})
}

func checkAcknowledgedOutbox(tx *bolt.Tx, through uint64) error {
	meta, events := tx.Bucket([]byte("meta")), tx.Bucket([]byte("events"))
	if meta == nil || events == nil {
		return errors.New("the closed client store is missing a bucket")
	}
	next := meta.Get([]byte("next"))
	if len(next) != 8 || binary.BigEndian.Uint64(next) <= through {
		return errors.New("the client did not allocate the acknowledged event")
	}
	key, _ := events.Cursor().First()
	if key != nil && (len(key) != 8 || binary.BigEndian.Uint64(key) <= through) {
		return errors.New("the client did not consume the acknowledgement")
	}
	return nil
}

func TestClosedOutboxAcknowledgementObserver(t *testing.T) {
	db, err := bolt.Open(filepath.Join(t.TempDir(), "outbox.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Update(func(tx *bolt.Tx) error {
		meta, err := tx.CreateBucket([]byte("meta"))
		if err != nil {
			return err
		}
		events, err := tx.CreateBucket([]byte("events"))
		if err != nil {
			return err
		}
		if err := meta.Put([]byte("next"), binary.BigEndian.AppendUint64(nil, 4)); err != nil {
			return err
		}
		key := binary.BigEndian.AppendUint64(nil, 2)
		if err := events.Put(key, []byte{1}); err != nil {
			return err
		}
		if err := events.Put(binary.BigEndian.AppendUint64(nil, 3), []byte{1}); err != nil {
			return err
		}
		if checkAcknowledgedOutbox(tx, 2) == nil {
			return errors.New("the observer accepted an acknowledgement without durable consumption")
		}
		if err := events.Delete(key); err != nil {
			return err
		}
		return checkAcknowledgedOutbox(tx, 2)
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRelayAcknowledgementConsumption(t *testing.T) {
	requirePostgres(t)
	f := newCommandFixture(t, relayControls{})
	run, leg := f.startApproved(t, 0)
	number := mark(t, f.db, f.id)
	waitForAck(t, leg, number)
	waitForClientToProcessReplies(t, leg)
	sends := leg.observe().sends
	ack := slices.Index(sends, frameSend{kind: "ack", number: number})
	if ack < 0 || sends[len(sends)-1].kind != "command" || ack >= len(sends)-1 {
		t.Fatal("The reply barrier was not sent after the acknowledgement.")
	}
	run.stop(t)
	requireAcknowledgedOutbox(t, f.opts, number)
}
