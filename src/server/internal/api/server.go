// Package api routes the generated services of the contract and guards the website's calls.
package api

import (
	"errors"
	"net/http"
	"path"
	"strings"

	"connectrpc.com/connect"

	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/transport"
)

// Options names the parts that the request handler serves.
// A nil service handler leaves its procedures unimplemented behind the guard.
type Options struct {
	Sessions *auth.Sessions
	Origins  *auth.OriginPolicy
	Session  lgtmv1connect.SessionServiceHandler
	Clients  lgtmv1connect.ClientServiceHandler
	Link     linkv1connect.LinkServiceHandler
}

// sessionReadMaxBytes bounds every message that the Session service reads, after decompression.
const sessionReadMaxBytes = 32 << 10

// linkReadLimit bounds each incoming Link message after decompression.
const linkReadLimit = 64 << 10

const healthPath = "/healthz"

// protocolPrefix starts every path of the contract's packages, web and link alike.
const protocolPrefix = "/lgtm."

// The two web procedures that need no session.
var publicProcedures = map[string]bool{
	lgtmv1connect.SessionServiceGetServerProcedure: true,
	lgtmv1connect.SessionServiceSignInProcedure:    true,
}

// The three Session procedures, whose responses must never be cached.
var sessionProcedures = map[string]bool{
	lgtmv1connect.SessionServiceGetServerProcedure: true,
	lgtmv1connect.SessionServiceSignInProcedure:    true,
	lgtmv1connect.SessionServiceSignOutProcedure:   true,
}

// The machine link's procedures, which have their own policy and never pass the web guard.
var linkProcedures = map[string]bool{
	linkv1connect.LinkServiceJoinProcedure: true,
	linkv1connect.LinkServiceOpenProcedure: true,
}

// webProcedures holds every generated procedure of the four web services.
var webProcedures = map[string]bool{
	lgtmv1connect.SessionServiceGetServerProcedure: true,
	lgtmv1connect.SessionServiceSignInProcedure:    true,
	lgtmv1connect.SessionServiceSignOutProcedure:   true,

	lgtmv1connect.ClientServiceWatchClientsProcedure:        true,
	lgtmv1connect.ClientServiceCreateClientTokenProcedure:   true,
	lgtmv1connect.ClientServiceRevokeClientTokenProcedure:   true,
	lgtmv1connect.ClientServiceApproveClientProcedure:       true,
	lgtmv1connect.ClientServiceRenameClientProcedure:        true,
	lgtmv1connect.ClientServiceScanClientHarnessesProcedure: true,
	lgtmv1connect.ClientServiceReconnectClientProcedure:     true,
	lgtmv1connect.ClientServiceRemoveClientProcedure:        true,

	lgtmv1connect.MapServiceWatchMapProcedure:   true,
	lgtmv1connect.MapServiceBuildMapProcedure:   true,
	lgtmv1connect.MapServiceOpenMapProcedure:    true,
	lgtmv1connect.MapServiceReleaseMapProcedure: true,

	lgtmv1connect.PreviewServiceWatchPreviewsProcedure:  true,
	lgtmv1connect.PreviewServiceStartPreviewProcedure:   true,
	lgtmv1connect.PreviewServiceStopPreviewProcedure:    true,
	lgtmv1connect.PreviewServiceOpenPreviewProcedure:    true,
	lgtmv1connect.PreviewServiceReleasePreviewProcedure: true,
	lgtmv1connect.PreviewServiceSavePreviewProcedure:    true,
	lgtmv1connect.PreviewServiceRemovePreviewProcedure:  true,
}

// The request headers that a browser may send on a web procedure, in canonical form.
var allowedRequestHeaders = map[string]bool{
	"Content-Type":             true,
	"Connect-Protocol-Version": true,
	"Connect-Timeout-Ms":       true,
	"Connect-Accept-Encoding":  true,
	"Connect-Content-Encoding": true,
	"Grpc-Timeout":             true,
	"X-Grpc-Web":               true,
	"X-User-Agent":             true,
	"Lgtm-Request-Key":         true,
}

const exposedResponseHeaders = "Grpc-Status, Grpc-Message, Grpc-Status-Details-Bin"

const notImplemented = "This call is not implemented."

const originRefused = "This website origin is not allowed."

type router struct {
	sessions    *auth.Sessions
	origins     *auth.OriginPolicy
	errors      *connect.ErrorWriter
	web         map[string]http.Handler
	link        http.Handler
	unsupported http.Handler
}

// NewHandler serves readiness, the web services behind the origin gate and the session guard,
// and the placeholders of the machine link.
func NewHandler(options Options) http.Handler {
	protocol := connect.WithRequireConnectProtocolHeader()
	r := &router{
		sessions: options.Sessions,
		origins:  options.Origins,
		errors:   connect.NewErrorWriter(protocol),
		web:      make(map[string]http.Handler),
	}
	r.unsupported = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.writeError(w, req, connect.NewError(connect.CodeUnimplemented, errors.New(notImplemented)))
	})

	// The session gate runs after Connect starts the protocol deadline and before it reads a message.
	guard := connect.WithRequestGate(r.sessionGate)
	if options.Session != nil {
		_, handler := lgtmv1connect.NewSessionServiceHandler(options.Session,
			protocol, guard, connect.WithReadMaxBytes(sessionReadMaxBytes), sessionDecoding())
		r.register(handler, lgtmv1connect.SessionServiceGetServerProcedure,
			lgtmv1connect.SessionServiceSignInProcedure, lgtmv1connect.SessionServiceSignOutProcedure)
	}
	if options.Clients != nil {
		_, handler := lgtmv1connect.NewClientServiceHandler(options.Clients, protocol, guard)
		r.register(transport.WithWriteDeadline(handler),
			lgtmv1connect.ClientServiceWatchClientsProcedure,
			lgtmv1connect.ClientServiceCreateClientTokenProcedure,
			lgtmv1connect.ClientServiceRevokeClientTokenProcedure,
			lgtmv1connect.ClientServiceApproveClientProcedure,
			lgtmv1connect.ClientServiceRenameClientProcedure,
			lgtmv1connect.ClientServiceScanClientHarnessesProcedure,
			lgtmv1connect.ClientServiceReconnectClientProcedure,
			lgtmv1connect.ClientServiceRemoveClientProcedure)
	}
	r.link = r.unsupported
	if options.Link != nil {
		_, handler := linkv1connect.NewLinkServiceHandler(options.Link, protocol, connect.WithReadMaxBytes(linkReadLimit))
		r.link = transport.WithWriteDeadline(handler)
	}
	return r
}

func (r *router) register(handler http.Handler, procedures ...string) {
	for _, procedure := range procedures {
		r.web[procedure] = handler
	}
}

func (r *router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// The path that is checked must be the path that is dispatched, so every alias is refused.
	p := req.URL.Path
	if req.URL.RawPath != "" || !strings.HasPrefix(p, "/") || path.Clean(p) != p {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if p == healthPath {
		serveHealth(w, req)
		return
	}
	if !strings.HasPrefix(p, protocolPrefix) {
		http.NotFound(w, req)
		return
	}
	if sessionProcedures[p] {
		w.Header().Set("Cache-Control", "no-store")
	}

	if linkProcedures[p] {
		if req.Method == http.MethodOptions && isPreflight(req) {
			// The machine link is not a browser procedure.
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if req.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		r.link.ServeHTTP(w, req)
		return
	}

	w.Header().Add("Vary", "Origin")
	if req.Method == http.MethodOptions && isPreflight(req) {
		r.servePreflight(w, req, p)
		return
	}
	if req.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	r.serveWeb(w, req, p)
}

func serveHealth(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", http.MethodPost)
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func isPreflight(req *http.Request) bool {
	_, origin := req.Header["Origin"]
	_, method := req.Header["Access-Control-Request-Method"]
	return origin && method
}

// servePreflight answers a browser's preflight without dispatching to a service.
func (r *router) servePreflight(w http.ResponseWriter, req *http.Request, procedure string) {
	header := w.Header()
	header.Add("Vary", "Access-Control-Request-Method")
	header.Add("Vary", "Access-Control-Request-Headers")
	headers, ok := requestedHeaders(req)
	if !ok || !webProcedures[procedure] ||
		len(req.Header.Values("Access-Control-Request-Method")) != 1 ||
		req.Header.Get("Access-Control-Request-Method") != http.MethodPost {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if _, err := r.origins.Check(req); err != nil {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	allowOrigin(header, req)
	header.Set("Access-Control-Allow-Methods", http.MethodPost)
	if len(headers) > 0 {
		header.Set("Access-Control-Allow-Headers", strings.Join(headers, ", "))
	}
	w.WriteHeader(http.StatusNoContent)
}

// requestedHeaders returns the requested header names when every one of them is allowed.
func requestedHeaders(req *http.Request) ([]string, bool) {
	var names []string
	for _, field := range req.Header.Values("Access-Control-Request-Headers") {
		for name := range strings.SplitSeq(field, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if !allowedRequestHeaders[http.CanonicalHeaderKey(name)] {
				return nil, false
			}
			names = append(names, name)
		}
	}
	return names, true
}

// allowOrigin grants the admitted browser origin credentialed access to the response.
func allowOrigin(header http.Header, req *http.Request) {
	header.Set("Access-Control-Allow-Origin", req.Header.Get("Origin"))
	header.Set("Access-Control-Allow-Credentials", "true")
}

// serveWeb applies the origin gate before any service reads the request.
// A registered service checks the session in its request gate, and the fallback checks it here.
func (r *router) serveWeb(w http.ResponseWriter, req *http.Request, procedure string) {
	mode, err := r.origins.Check(req)
	if err != nil {
		if !r.errors.IsSupported(req) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		r.writeError(w, req, connect.NewError(connect.CodePermissionDenied, errors.New(originRefused)))
		return
	}
	if _, browser := req.Header["Origin"]; browser {
		allowOrigin(w.Header(), req)
		w.Header().Set("Access-Control-Expose-Headers", exposedResponseHeaders)
	}

	ctx := withCookieMode(req.Context(), mode)
	if handler, ok := r.web[procedure]; ok {
		handler.ServeHTTP(w, req.WithContext(ctx))
		return
	}
	if !publicProcedures[procedure] {
		verifier, err := r.checkSession(ctx, req.Header, mode)
		if err != nil {
			r.writeError(w, req, err)
			return
		}
		ctx = withVerifier(ctx, verifier)
	}
	r.unsupported.ServeHTTP(w, req.WithContext(ctx))
}

// writeError writes a protocol error, or only its status when the request uses no known protocol.
func (r *router) writeError(w http.ResponseWriter, req *http.Request, err *connect.Error) {
	if !r.errors.IsSupported(req) {
		w.WriteHeader(httpStatus(err.Code()))
		return
	}
	_ = r.errors.Write(w, req, err)
}

// httpStatus gives the status that the Connect protocol pairs with the codes the guard writes.
func httpStatus(code connect.Code) int {
	switch code {
	case connect.CodeCanceled:
		return 499
	case connect.CodeInvalidArgument:
		return http.StatusBadRequest
	case connect.CodeDeadlineExceeded:
		return http.StatusGatewayTimeout
	case connect.CodePermissionDenied:
		return http.StatusForbidden
	case connect.CodeResourceExhausted:
		return http.StatusTooManyRequests
	case connect.CodeAborted:
		return http.StatusConflict
	case connect.CodeUnimplemented:
		return http.StatusNotImplemented
	case connect.CodeUnavailable:
		return http.StatusServiceUnavailable
	case connect.CodeUnauthenticated:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}
