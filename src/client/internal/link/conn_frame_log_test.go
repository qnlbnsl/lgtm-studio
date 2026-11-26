package link

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"testing"

	"connectrpc.com/connect"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/transport"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

func TestOpenFrameDebugDoesNotLogConnectionToken(t *testing.T) {
	const childOrigin = "LGTM_TEST_OPEN_FRAME_DEBUG_ORIGIN"
	if url := os.Getenv(childOrigin); url != "" {
		keys := fixtureKeysOf(t)
		credential := credentialOf(t, keys.machineKey)
		httpClient, err := transport.NewHTTPClient(url, nil)
		if err != nil {
			return // The transport may decline to connect with payload diagnostics enabled.
		}
		t.Cleanup(httpClient.CloseIdleConnections)
		server := enroll.Server{
			Address: url, MachineID: keys.machineID,
			PublicKey: keys.serverKey.Public().(ed25519.PublicKey), ClientPublicKey: credential.PublicKey(),
		}
		client := linkv1connect.NewLinkServiceClient(httpClient, url, connect.WithReadMaxBytes(64<<10))
		_, _ = open(t, client, server, credential)
		return
	}

	previousLog := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(previousLog) })
	welcome := goodWelcome()
	fixture := &scene{keys: fixtureKeysOf(t), script: honest(welcome), done: make(chan struct{}, 1)}
	_, handler := linkv1connect.NewLinkServiceHandler(fixture)
	srv := httptest.NewUnstartedServer(handler)
	srv.Config.Protocols = new(http.Protocols)
	srv.Config.Protocols.SetUnencryptedHTTP2(true)
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.Start()
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 2*openCeiling)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestOpenFrameDebugDoesNotLogConnectionToken$")
	cmd.Env = append(os.Environ(), childOrigin+"="+srv.URL, "GODEBUG=http2debug=2")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatal("the stream fixture subprocess failed")
	}

	token := []byte(welcome.GetConnectionToken())
	for _, value := range []string{
		string(token), hex.EncodeToString(token), base64.StdEncoding.EncodeToString(token),
		base64.RawStdEncoding.EncodeToString(token), base64.URLEncoding.EncodeToString(token),
		base64.RawURLEncoding.EncodeToString(token),
	} {
		if bytes.Contains(stderr.Bytes(), []byte(value)) {
			t.Fatal("frame debugging exposed the synthetic connection token on standard error")
		}
	}
	if loggedConnectFramesContain(stderr.Bytes(), token) {
		t.Fatal("frame debugging exposed an encoded synthetic connection token on standard error")
	}
}

func loggedConnectFramesContain(stderr, token []byte) bool {
	// The Go HTTP/2 framer quotes DATA payloads. Use the framer's read lines once,
	// since the transport may log the same frame again after processing it.
	dataLine := regexp.MustCompile(`(?m)\bread DATA\b[^\n]*\bdata=("(?:[^"\\\n]|\\.)*")`)
	var stream []byte
	for _, match := range dataLine.FindAllSubmatch(stderr, -1) {
		data, err := strconv.Unquote(string(match[1]))
		if err == nil {
			stream = append(stream, data...)
		}
	}

	// A Connect envelope starts with a flag byte and a four-byte payload length.
	// The approved welcome in this fixture normally arrives compressed with gzip.
	for len(stream) >= 5 {
		length := binary.BigEndian.Uint32(stream[1:5])
		if uint64(length) > uint64(len(stream)-5) {
			break
		}
		flags, payload := stream[0], stream[5:5+int(length)]
		stream = stream[5+int(length):]
		if flags&1 != 0 {
			reader, err := gzip.NewReader(bytes.NewReader(payload))
			if err != nil {
				continue
			}
			payload, _ = io.ReadAll(io.LimitReader(reader, 64<<10))
			_ = reader.Close()
		}
		if bytes.Contains(payload, token) {
			return true
		}
	}
	return false
}
