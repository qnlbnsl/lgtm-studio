package api_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
)

func TestMalformedJoinRedactsSubmittedBytes(t *testing.T) {
	const marker = "!this-is-not-a-real-join-credential!"
	const message = "The link message broke a rule of the contract."
	validBytes := base64.StdEncoding.EncodeToString(make([]byte, 32))
	// Malformed input must be rejected before the link service needs a store.
	server := httptest.NewServer(api.NewHandler(api.Options{Link: link.NewService(nil, nil, nil)}))
	t.Cleanup(server.Close)

	for _, field := range []string{"tokenSecret", "machinePublicKey"} {
		t.Run(field, func(t *testing.T) {
			fields := map[string]string{
				"tokenId":          "00000000-0000-7000-8000-000000000001",
				"tokenSecret":      validBytes,
				"machinePublicKey": validBytes,
			}
			fields[field] = marker
			body, err := json.Marshal(fields)
			if err != nil {
				t.Fatal("Encoding the Join fixture failed.")
			}
			req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+linkv1connect.LinkServiceJoinProcedure, bytes.NewReader(body))
			if err != nil {
				t.Fatal("Building the Join request failed.")
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Connect-Protocol-Version", "1")
			res, err := server.Client().Do(req)
			if err != nil {
				t.Fatal("Sending the Join request failed.")
			}
			defer res.Body.Close()
			raw, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatal("Reading the Join response failed.")
			}
			if bytes.Contains(raw, []byte(marker)) {
				t.Error("The malformed Join response body echoed the submitted marker.")
			}
			for _, headers := range []http.Header{res.Header, res.Trailer} {
				for name, values := range headers {
					if strings.Contains(name, marker) || strings.Contains(strings.Join(values, "\n"), marker) {
						t.Error("The malformed Join response metadata echoed the submitted marker.")
					}
				}
			}
			var reply struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(raw, &reply); err != nil {
				t.Fatal("The malformed Join response did not contain a JSON error object.")
			}
			if res.StatusCode != http.StatusBadRequest || reply.Code != connect.CodeInvalidArgument.String() {
				t.Errorf("Malformed Join returned HTTP %d or the wrong error code. Want HTTP 400 and invalid_argument.", res.StatusCode)
			}
			if reply.Message != message {
				t.Error("Malformed Join did not return the fixed public message.")
			}
		})
	}
}
