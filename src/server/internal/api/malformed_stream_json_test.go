package api_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	webrpc "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
)

type malformedWatchProbe struct {
	webrpc.UnimplementedClientServiceHandler
}

func (malformedWatchProbe) WatchClients(context.Context, *connect.Request[webv1.WatchClientsRequest], *connect.ServerStream[webv1.WatchClientsResponse]) error {
	return nil
}

func TestMalformedWatchClientsRedactsSubmittedBytes(t *testing.T) {
	const marker = "WatchSubmittedPayload"
	fixture := newSessionDatabase(t)
	token := issueTestSession(t, fixture.sessions)
	cookie := (&http.Cookie{Name: auth.CookieName, Value: token}).String()
	server := startHTTP2(t, api.NewHandler(api.Options{
		Origins: testOriginPolicy(t, loopbackOrigin), Sessions: fixture.sessions, Clients: malformedWatchProbe{},
	}))
	for _, contentType := range []string{"application/connect+json", "application/grpc+json", "application/grpc-web+json"} {
		t.Run(contentType, func(t *testing.T) {
			body := []byte(`{"ignored":` + marker + `}`)
			malformedJSONReply(t, server, webrpc.ClientServiceWatchClientsProcedure, contentType, cookie, body, marker)
		})
	}
}

func TestMalformedJoinRedactsGRPCAndCharsetJSON(t *testing.T) {
	const message = "The link message broke a rule of the contract."
	server := startHTTP2(t, api.NewHandler(api.Options{Link: link.NewService(nil, nil, nil)}))
	validField := base64.StdEncoding.EncodeToString(make([]byte, 32))
	for _, contentType := range []string{"application/grpc+json", "application/grpc-web+json", "application/json; charset=utf-8"} {
		t.Run(contentType, func(t *testing.T) {
			for _, field := range []string{"tokenSecret", "machinePublicKey"} {
				t.Run(field, func(t *testing.T) {
					const marker = "JoinSubmittedField"
					fields := map[string]string{
						"tokenId":          "00000000-0000-7000-8000-000000000001",
						"tokenSecret":      validField,
						"machinePublicKey": validField,
					}
					fields[field] = "!" + marker
					body, err := json.Marshal(fields)
					if err != nil {
						t.Fatal("Encoding the malformed Join fixture failed.")
					}
					if got := malformedJSONReply(t, server, linkv1connect.LinkServiceJoinProcedure, contentType, "", body, marker); got != message {
						t.Error("The malformed Join did not return the fixed public message.")
					}
				})
			}
		})
	}
}

func malformedJSONReply(t *testing.T, server *httptest.Server, procedure, contentType, cookie string, body []byte, marker string) string {
	t.Helper()
	grpc := strings.HasPrefix(contentType, "application/grpc")
	connectStream := contentType == "application/connect+json"
	if grpc || connectStream {
		frame := make([]byte, 5, 5+len(body))
		binary.BigEndian.PutUint32(frame[1:], uint32(len(body)))
		body = append(frame, body...)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+procedure, bytes.NewReader(body))
	if err != nil {
		t.Fatal("Building the malformed request failed.")
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Connect-Protocol-Version", "1")
	if grpc {
		req.Header.Set("Te", "trailers")
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	res, err := server.Client().Do(req)
	if err != nil {
		t.Fatal("Sending the malformed request failed.")
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal("Reading the malformed request's reply failed.")
	}
	status := http.StatusOK
	if !grpc && !connectStream {
		status = http.StatusBadRequest
	}
	if res.ProtoMajor != 2 || res.StatusCode != status {
		t.Fatalf("The malformed request returned HTTP %d over %s. Want HTTP %d over HTTP/2.", res.StatusCode, res.Proto, status)
	}
	echoed := false
	checkMarker := func(value string) {
		echoed = echoed || strings.Contains(value, marker)
	}
	defer func() {
		if echoed {
			t.Error("The malformed request's reply echoed the submitted marker.")
		}
	}()
	checkMarker(string(raw))
	metadata := []http.Header{res.Header, res.Trailer}
	if strings.HasPrefix(contentType, "application/grpc-web") && len(raw) != 0 {
		block := malformedReplyFrame(t, raw, 0x80)
		reader := textproto.NewReader(bufio.NewReader(bytes.NewReader(append(block, '\r', '\n'))))
		trailers, err := reader.ReadMIMEHeader()
		if err != nil {
			t.Fatal("The malformed request's gRPC-Web trailer frame was invalid.")
		}
		metadata = append(metadata, http.Header(trailers))
	}
	for _, headers := range metadata {
		for name, values := range headers {
			checkMarker(name)
			for _, value := range values {
				checkMarker(value)
				if decoded, err := url.PathUnescape(value); err == nil && decoded != value {
					checkMarker(decoded)
				}
			}
		}
	}
	if grpc {
		for _, headers := range metadata {
			if code := headers.Get("Grpc-Status"); code != "" {
				if code != strconv.Itoa(int(connect.CodeInvalidArgument)) {
					t.Error("The malformed request did not return gRPC status invalid_argument.")
				}
				message, err := url.PathUnescape(headers.Get("Grpc-Message"))
				if err != nil {
					t.Fatal("The malformed request's gRPC error message could not be decoded.")
				}
				return message
			}
		}
		t.Fatal("The malformed request's gRPC reply had no status.")
	}

	var reply struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if connectStream {
		var end struct {
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(malformedReplyFrame(t, raw, 0x02), &end); err != nil {
			t.Fatal("The malformed watch did not return a Connect end envelope.")
		}
		raw = end.Error
	}
	if err := json.Unmarshal(raw, &reply); err != nil || reply.Code != connect.CodeInvalidArgument.String() {
		t.Fatal("The malformed request did not return a Connect invalid_argument error.")
	}
	checkMarker(reply.Message)
	return reply.Message
}

func malformedReplyFrame(t *testing.T, raw []byte, flags byte) []byte {
	t.Helper()
	const header = 5
	if len(raw) < header || raw[0] != flags || int(binary.BigEndian.Uint32(raw[1:header])) != len(raw)-header {
		t.Fatal("The malformed request did not return one terminal frame.")
	}
	return raw[header:]
}
