package link

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"net"
	"net/http/httptrace"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll"
)

func TestRetryWaitsForCanceledHTTPSDial(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal("could not open the loopback listener")
	}
	peerDone := make(chan struct{})
	go func() {
		defer close(peerDone)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	defer func() {
		_ = listener.Close()
		<-peerDone
	}()

	synctest.Test(t, func(t *testing.T) {
		folder, _ := newState(t)
		store := mustCreate(t, folder, StoreOptions{})
		if err := store.Close(); err != nil {
			t.Fatal("could not close the seeded store")
		}
		keys := fixtureKeysOf(t)
		opts := RunOptions{
			Server: enroll.Server{
				Address:              "https://" + listener.Addr().String(),
				CertificateKeySHA256: make([]byte, sha256.Size),
				MachineID:            keys.machineID,
				PublicKey:            keys.serverKey.Public().(ed25519.PublicKey),
			},
			Credential: credentialOf(t, keys.machineKey),
			Folder:     folder,
		}
		firstConnected := make(chan bool, 1)
		nextStarted := make(chan struct{}, 1)
		releaseFirst, releaseNext := make(chan struct{}), make(chan struct{})
		unblockFirst := sync.OnceFunc(func() { close(releaseFirst) })
		unblockNext := sync.OnceFunc(func() { close(releaseNext) })
		var started atomic.Int32
		ctx, cancel := context.WithCancel(t.Context())
		defer func() {
			cancel()
			unblockFirst()
			unblockNext()
		}()
		ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
			ConnectStart: func(_, _ string) {
				if started.Add(1) == 2 {
					nextStarted <- struct{}{}
					<-releaseNext
				}
			},
			ConnectDone: func(_, _ string, err error) {
				if started.Load() == 1 {
					firstConnected <- err == nil
					// Keep the first dial in progress even after TCP connect
					// has finished and the handshake context has expired.
					<-releaseFirst
				}
			},
		})
		done := make(chan runResult, 1)
		go func() {
			outcome, err := Run(ctx, opts)
			done <- runResult{outcome, err}
		}()

		if !receiveNow(t, firstConnected, "the first HTTPS dial did not reach the loopback listener") {
			t.Fatal("the first HTTPS dial could not connect to the loopback listener")
		}
		time.Sleep(15 * time.Second)
		requireEmpty(t, nextStarted, "the client started a replacement HTTPS dial while the first dial was still in teardown")

		unblockFirst()
		receiveNow(t, nextStarted, "the client did not resume HTTPS dialing after the first dial retired")
		cancel()
		unblockNext()
		result := receiveNow(t, done, "the client did not stop after the second dial retired")
		if result.outcome != RunStopped || result.err != nil {
			t.Error("the client did not return RunStopped after cancellation")
		}
	})
}
