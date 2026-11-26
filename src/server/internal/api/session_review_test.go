package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/reflect/protoreflect"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	webrpc "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
)

func TestMalformedSignInRedactsPassword(t *testing.T) {
	const password = "6307194285"
	fixture := sessionDatabaseWithPassword(t, password)
	handler := sessionHandler(t, fixture, loopbackOrigin)
	req := jsonProcedure(t, webrpc.SessionServiceSignInProcedure, `{"password":`+password+`}`)
	res := serveRequest(handler, req)
	requireNoCookie(t, res.Header())
	if bytes.Contains(res.Body.Bytes(), []byte(password)) {
		t.Error("The malformed SignIn response exposed the submitted password.")
	}
	var reply struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &reply); err != nil {
		t.Fatal("The malformed SignIn response did not contain a JSON error object.")
	}
	if res.Code != http.StatusBadRequest || reply.Code != connect.CodeInvalidArgument.String() {
		t.Errorf("Malformed SignIn returned HTTP %d or the wrong error code, want HTTP 400 and invalid_argument.", res.Code)
	}
	if reply.Message != "The request body is invalid." {
		t.Error("Malformed SignIn did not return the fixed public message.")
	}
}

func TestGeneratedWebProcedurePreflights(t *testing.T) {
	const origin = "http://localhost:18787"
	const requestHeader = "content-type"
	handler := api.NewHandler(api.Options{Origins: testOriginPolicy(t, loopbackOrigin, origin)})
	for _, file := range []protoreflect.FileDescriptor{
		webv1.File_lgtm_v1_session_proto,
		webv1.File_lgtm_v1_clients_proto,
		webv1.File_lgtm_v1_map_proto,
		webv1.File_lgtm_v1_preview_proto,
	} {
		services := file.Services()
		for i := range services.Len() {
			service := services.Get(i)
			methods := service.Methods()
			for j := range methods.Len() {
				procedure := string(service.FullName()) + "/" + string(methods.Get(j).Name())
				t.Run(procedure, func(t *testing.T) {
					req := httptest.NewRequest(http.MethodOptions, "/"+procedure, nil)
					req.Header.Set("Origin", origin)
					req.Header.Set("Access-Control-Request-Method", http.MethodPost)
					req.Header.Set("Access-Control-Request-Headers", requestHeader)
					res := serveRequest(handler, req)
					if res.Code != http.StatusNoContent || res.Body.Len() != 0 {
						t.Fatalf("Preflight returned HTTP %d with %d body bytes, want an empty 204.", res.Code, res.Body.Len())
					}
					header := res.Header()
					requireOriginResponse(t, header, origin)
					if header.Get("Access-Control-Allow-Methods") != http.MethodPost ||
						!headerListContains(header, "Access-Control-Allow-Headers", requestHeader) {
						t.Error("Preflight must allow the requested POST and content-type header.")
					}
					requireNoCookie(t, header)
				})
			}
		}
	}
}
