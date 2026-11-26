package link_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
)

// TestJoinRefusalIsATrailerOverGRPC reads the raw HTTP/2 response of a gRPC refusal. The Connect
// client merges headers and trailers into one metadata value, so only the raw response shows that
// the reason travels in the trailer and in no initial header.
func TestJoinRefusalIsATrailerOverGRPC(t *testing.T) {
	e := newEnv(t)
	_, msg := e.newToken(t)

	mux := http.NewServeMux()
	mux.Handle(linkv1connect.NewLinkServiceHandler(link.NewService(e.s, e.tokens, e.serverKey)))
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	server := httptest.NewUnstartedServer(mux)
	server.Config.Protocols = protocols
	server.Start()
	t.Cleanup(server.Close)
	client := &http.Client{Transport: &http.Transport{Protocols: protocols}}

	// A token with the right identifier and a wrong secret is unknown.
	payload, err := proto.Marshal(&linkv1.JoinRequest{TokenId: msg.GetTokenId(), TokenSecret: keyOf(0x55), MachinePublicKey: keyOf(0x31), HostName: "box"})
	if err != nil {
		t.Fatalf("marshalling failed: %v", err)
	}
	frame := make([]byte, 5, 5+len(payload))
	binary.BigEndian.PutUint32(frame[1:], uint32(len(payload)))
	frame = append(frame, payload...)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+linkv1connect.LinkServiceJoinProcedure, bytes.NewReader(frame))
	if err != nil {
		t.Fatalf("building the request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/grpc")
	req.Header.Set("Te", "trailers")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("the raw call failed: %v", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("reading the body failed: %v", err)
	}
	if resp.ProtoMajor != 2 {
		t.Fatalf("the response is HTTP/%d, want HTTP/2", resp.ProtoMajor)
	}

	if got := resp.Header.Values(refusalKey); len(got) != 0 {
		t.Errorf("the initial headers carry the reason %q, want it in the trailer only", got)
	}
	if got := resp.Trailer.Get(refusalKey); got != "token_unknown" {
		t.Errorf("the trailer carries the reason %q, want token_unknown", got)
	}
	if got := resp.Trailer.Get("Grpc-Status"); got != "7" {
		t.Errorf("the trailer's grpc-status is %q, want 7 (permission denied)", got)
	}
	if got := resp.Trailer.Get("Grpc-Message"); strings.Contains(got, "token_unknown") {
		t.Errorf("the gRPC message %q holds the reason word", got)
	}
}
