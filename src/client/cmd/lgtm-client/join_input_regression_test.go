package main

import (
	"context"
	"encoding/base64"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/encoding/protowire"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

const joinTokenLineLimit = 4096

func TestRunJoinStopsOnSignalWhileReadingToken(t *testing.T) {
	for _, tc := range []struct {
		name   string
		signal os.Signal
	}{
		{name: "interrupt", signal: os.Interrupt},
		{name: "termination", signal: syscall.SIGTERM},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, w := commandStdinPipe(t)
			// Consuming one byte proves the command reached its read. The pipe
			// is then empty and stays open without a complete token line.
			if _, err := io.WriteString(w, "x"); err != nil {
				t.Fatal("could not prime standard input")
			}
			process := startLinkCommand(t, commandStateDir(t), "normal", r, "join", "http://127.0.0.1:1", "-")
			waitForJoinTokenRead(t, process, r)
			if err := process.cmd.Process.Signal(tc.signal); err != nil {
				t.Fatal("could not signal the command while it was reading standard input")
			}
			select {
			case <-process.done:
				if process.status <= 0 {
					t.Errorf("the interrupted join returned %d, want a normal nonzero exit", process.status)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("the join did not exit after the signal while standard input stayed open")
			}
		})
	}
}

func waitForJoinTokenRead(t *testing.T, process *commandProcess, r *os.File) {
	t.Helper()
	fds := []unix.PollFd{{Fd: int32(r.Fd()), Events: unix.POLLIN}}
	poll := time.NewTicker(5 * time.Millisecond)
	defer poll.Stop()
	for {
		ready, err := unix.Poll(fds, 0)
		if err == nil && ready == 0 {
			return
		}
		if err != nil && err != unix.EINTR {
			t.Fatal("could not observe the standard input pipe")
		}
		select {
		case <-poll.C:
		case <-process.done:
			t.Fatal("the command exited before reading standard input")
		case <-process.ctx.Done():
			t.Fatal("the command did not start reading standard input")
		}
	}
}

func TestRunJoinRejectsOversizedTokenBeforeLineEnds(t *testing.T) {
	peer := newCommandPeer(t)
	peer.start(t)
	r, w := commandStdinPipe(t)
	dir := commandStateDir(t)
	process := startLinkCommand(t, dir, "normal", r, "join", peer.server.URL, "-")
	_ = r.Close()
	input := peer.token + strings.Repeat("x", joinTokenLineLimit+1-len(peer.token))
	if _, err := io.WriteString(w, input); err != nil {
		t.Fatal("could not supply the oversized token line")
	}
	select {
	case <-process.done:
		process.exit(t, 2)
	case <-time.After(2 * time.Second):
		t.Fatal("the join kept reading after 4097 ordinary bytes while standard input stayed open")
	}
	process.checkOutput(t, peer)
	if process.stderr.text() != clientUsage || process.stdout.Len() != 0 {
		t.Error("the oversized token did not produce only the fixed usage line on standard error")
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Error("the oversized token reached enrollment state setup")
	}
}

func TestRunJoinAcceptsFullLengthToken(t *testing.T) {
	peer := newCommandPeer(t)
	peer.join = func(context.Context, *linkv1.JoinRequest) (*linkv1.JoinResponse, error) {
		return &linkv1.JoinResponse{MachineId: commandMachineID, ServerPublicKey: peer.public}, nil
	}
	peer.start(t)
	token := fullLengthJoinToken(t, peer.token)
	for _, tc := range []struct {
		name   string
		ending string
	}{
		{name: "line feed", ending: "\n"},
		{name: "carriage return and line feed", ending: "\r\n"},
		{name: "end of input"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, w := commandStdinPipe(t)
			process := startLinkCommand(t, commandStateDir(t), "normal", r, "join", peer.server.URL, "-")
			_ = r.Close()
			if _, err := io.WriteString(w, token+tc.ending); err != nil {
				t.Fatal("could not supply the full-length token")
			}
			if tc.ending == "" {
				_ = w.Close()
			}
			process.exit(t, 0)
			process.checkOutput(t, peer)
			if strings.Contains(process.stdout.String()+process.stderr.text(), token) {
				t.Error("the command exposed the full-length join token")
			}
		})
	}
}

func fullLengthJoinToken(t *testing.T, token string) string {
	t.Helper()
	const prefix = "lgtm1_"
	encoding := base64.RawURLEncoding
	wire, err := encoding.DecodeString(strings.TrimPrefix(token, prefix))
	if err != nil {
		t.Fatal("could not decode the join token fixture")
	}
	// A length-delimited unknown field pads the token without changing its credentials.
	wire = protowire.AppendTag(wire, 15, protowire.BytesType)
	target := encoding.DecodedLen(joinTokenLineLimit - len(prefix))
	// The padding length needs a two-byte varint.
	payloadSize := target - len(wire) - 2
	wire = protowire.AppendBytes(wire, make([]byte, payloadSize))
	token = prefix + encoding.EncodeToString(wire)
	if len(token) != joinTokenLineLimit {
		t.Fatal("the join token fixture did not reach the required byte length")
	}
	return token
}
