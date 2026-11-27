package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"connectrpc.com/connect"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	joinRefusal  = "Lgtm-Join-Refusal"
	linkRefusal  = "Lgtm-Link-Refusal"
	messageLimit = 64 << 10

	invalidLinkMessage = "The link message broke a rule of the contract."
	malformedJoinHost  = "\xff"
)

var (
	errDisconnected = errors.New("The test link was disconnected.")
	errObservation  = errors.New("The relay could not record an observation.")
	canonicalID     = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	// The stream allocator uses the 26-character base32 output of rand.Text in Go 1.26.
	streamIDFormat = regexp.MustCompile(`^[A-Z2-7]{26}$`)
)

func main() {
	flags := flag.NewFlagSet("sprint4relay", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	listen := flags.String("listen", "", "address to listen on")
	upstream := flags.String("upstream", "", "upstream base URL")
	controlDir := flags.String("control-dir", "", "private control directory")
	cert := flags.String("tls-cert", "", "listener certificate file")
	key := flags.String("tls-key", "", "listener private key file")
	if flags.Parse(os.Args[1:]) != nil || flags.NArg() != 0 || *listen == "" || *controlDir == "" {
		stopRelay("The relay needs a listen address, upstream URL and control directory.")
	}
	if (*cert == "") != (*key == "") {
		stopRelay("The relay needs both certificate and key files to serve TLS.")
	}
	target, err := url.Parse(*upstream)
	if err != nil || target.Hostname() == "" || target.User != nil || target.RawQuery != "" ||
		target.Fragment != "" || (target.Scheme != "http" && target.Scheme != "https") {
		stopRelay("The relay needs a valid upstream URL.")
	}

	var upstreamProtocols http.Protocols
	if target.Scheme == "https" {
		upstreamProtocols.SetHTTP2(true)
	} else {
		upstreamProtocols.SetUnencryptedHTTP2(true)
	}
	client := &http.Client{
		Transport: &http.Transport{Protocols: &upstreamProtocols},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	output, err := os.OpenFile(filepath.Join(*controlDir, "observations.jsonl"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		stopRelay(errObservation.Error())
	}
	defer output.Close()
	r := &frameRelay{
		upstream:    linkv1connect.NewLinkServiceClient(client, target.String(), connect.WithReadMaxBytes(messageLimit)),
		controlPath: filepath.Join(*controlDir, "control.json"),
		output:      json.NewEncoder(output),
		streams:     make(map[string]*relayStream),
	}
	_, handler := linkv1connect.NewLinkServiceHandler(r,
		connect.WithReadMaxBytes(messageLimit),
		connect.WithConditionalHandlerOptions(joinDecodeOptions),
	)
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	server := &http.Server{
		Handler:     handler,
		Protocols:   &protocols,
		ErrorLog:    log.New(io.Discard, "", 0),
		ConnContext: r.connectionContext,
	}
	if *cert == "" {
		protocols.SetUnencryptedHTTP2(true)
	} else {
		protocols.SetHTTP2(true)
		if _, err := tls.LoadX509KeyPair(*cert, *key); err != nil {
			stopRelay("The relay could not load its certificate and key.")
		}
		server.TLSConfig = &tls.Config{
			MinVersion:             tls.VersionTLS12,
			SessionTicketsDisabled: true,
			GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				// Each new handshake reads the current pair so the fixture can rotate the leaf.
				pair, err := tls.LoadX509KeyPair(*cert, *key)
				if err != nil {
					return nil, errors.New("The relay could not load its certificate and key.")
				}
				return &pair, nil
			},
		}
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		stopRelay("The relay could not bind its listener.")
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.watchControl(ctx)
	fmt.Printf("relay ready %s\n", listener.Addr())
	if *cert == "" {
		err = server.Serve(listener)
	} else {
		err = server.ServeTLS(listener, "", "")
	}
	if !errors.Is(err, http.ErrServerClosed) {
		stopRelay("The relay listener stopped.")
	}
}

func stopRelay(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}

func joinDecodeOptions(spec connect.Spec) []connect.HandlerOption {
	if spec.Procedure != linkv1connect.LinkServiceJoinProcedure {
		return nil
	}
	return []connect.HandlerOption{
		connect.WithCodec(joinCodec{"proto"}),
		connect.WithCodec(joinCodec{"json"}),
		connect.WithCodec(joinCodec{"json; charset=utf-8"}),
		connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				if req.Any().(*linkv1.JoinRequest).GetHostName() == malformedJoinHost {
					return nil, connect.NewError(connect.CodeInvalidArgument, errors.New(invalidLinkMessage))
				}
				return next(ctx, req)
			}
		})),
	}
}

type joinCodec struct{ name string }

func (c joinCodec) Name() string { return c.name }

func (c joinCodec) Marshal(message any) ([]byte, error) {
	msg := message.(proto.Message)
	if c.name == "proto" {
		return proto.Marshal(msg)
	}
	return protojson.Marshal(msg)
}

func (c joinCodec) Unmarshal(data []byte, message any) error {
	msg := message.(proto.Message)
	var err error
	if c.name == "proto" {
		err = proto.Unmarshal(data, msg)
	} else {
		err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, msg)
	}
	if err != nil {
		join, ok := msg.(*linkv1.JoinRequest)
		if !ok {
			return errors.New(invalidLinkMessage)
		}
		// Both decoders reject invalid UTF-8 in the host name. The interceptor can
		// recognize this marker and return the public sentence without a decoder prefix.
		proto.Reset(join)
		join.HostName = malformedJoinHost
	}
	return nil
}

type relaySelector struct {
	MachineID   string `json:"machine_id"`
	Direction   string `json:"direction"`
	Frame       string `json:"frame"`
	EventNumber uint64 `json:"event_number,omitempty"`
	CommandID   string `json:"command_id,omitempty"`
	Attempt     uint32 `json:"attempt,omitempty"`
}

type relayFrame struct {
	relaySelector
	Digest          string                 `json:"digest,omitempty"`
	Outcome         *linkv1.CommandOutcome `json:"outcome,omitempty"`
	LastEventNumber *uint64                `json:"last_event_number,omitempty"`
	LinkRevision    uint32                 `json:"link_revision,omitempty"`
}

type relayObservation struct {
	relayFrame
	ConnectionID uint64 `json:"connection_id"`
	StreamID     string `json:"stream_id"`
	Generation   uint64 `json:"generation"`
	Action       string `json:"action"`
}

type relayRule struct {
	selector relaySelector
	held     *relayFrame
}

type relayStream struct {
	id           string
	connectionID uint64
	machine      string
	generation   uint64
	rules        []relayRule
}

type relayControl struct {
	Generation uint64          `json:"generation"`
	StreamID   string          `json:"stream_id"`
	Action     string          `json:"action"`
	Rules      []relaySelector `json:"rules"`
}

type frameRelay struct {
	upstream    linkv1connect.LinkServiceClient
	controlPath string
	output      *json.Encoder

	mu             sync.Mutex
	streams        map[string]*relayStream
	nextConnection uint64
	lastGeneration uint64
}

type relayConnectionKey struct{}

func (r *frameRelay) connectionContext(ctx context.Context, _ net.Conn) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextConnection++
	return context.WithValue(ctx, relayConnectionKey{}, r.nextConnection)
}

func (r *frameRelay) Join(ctx context.Context, req *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
	leg := r.startStream(ctx)
	defer r.closeStream(leg)
	reply, err := r.upstream.Join(ctx, connect.NewRequest(req.Msg))
	frame := relayFrame{relaySelector: relaySelector{Direction: "up", Frame: "join"}}
	if reply != nil {
		frame.MachineID = safeIdentifier(reply.Msg.GetMachineId())
	}
	if err == nil || connect.IsWireError(err) {
		if err := r.forwarded(leg, frame); err != nil {
			return nil, err
		}
	}
	if err != nil {
		var wireErr *connect.Error
		downstreamErr := publicLinkError(err)
		if connect.IsWireError(err) && errors.As(err, &wireErr) {
			copyRefusal(downstreamErr.Meta(), joinRefusal, wireErr.Meta().Values(joinRefusal))
		}
		return nil, downstreamErr
	}
	response := connect.NewResponse(reply.Msg)
	response.Header().Set("Cache-Control", "no-store")
	return response, nil
}

type relayUpstream = connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse]
type relayDownstream = connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]

func (r *frameRelay) Open(ctx context.Context, downstream *relayDownstream) error {
	leg := r.startStream(ctx)
	defer r.closeStream(leg)
	ctx, cancel := context.WithCancelCause(ctx)
	upstream := r.upstream.Open(ctx)
	defer func() {
		cancel(nil)
		_ = upstream.CloseResponse()
	}()
	go func() {
		if err := r.forwardRequests(leg, downstream, upstream); err != nil {
			cancel(err)
		}
	}()

	for {
		msg, err := upstream.Receive()
		if err != nil {
			if cause := context.Cause(ctx); cause != nil {
				return publicLinkError(cause)
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			return relayOpenError(downstream, upstream, err)
		}
		if err := downstream.Send(msg); err != nil {
			return publicLinkError(err)
		}
		if err := r.forwarded(leg, describeReply(msg)); err != nil {
			return err
		}
	}
}

func (r *frameRelay) forwardRequests(leg *relayStream, downstream *relayDownstream, upstream *relayUpstream) error {
	defer upstream.CloseRequest()
	// Open the upstream request before reading Hello so its handshake deadline still applies.
	if err := upstream.Send(nil); err != nil {
		return sendError(err)
	}
	for first := true; ; first = false {
		msg, err := downstream.Receive()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		frame := describeRequest(msg)
		if first {
			frame.Frame = "open"
			frame.MachineID = safeIdentifier(msg.GetHello().GetMachineId())
			frame.LinkRevision = msg.GetHello().GetLinkRevision()
		}
		held, err := r.hold(leg, frame)
		if err != nil {
			return err
		}
		if held {
			continue
		}
		if err := upstream.Send(msg); err != nil {
			return sendError(err)
		}
		if err := r.forwarded(leg, frame); err != nil {
			return err
		}
	}
}

func sendError(err error) error {
	// A send can see EOF before Receive obtains the upstream refusal.
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func describeRequest(msg *linkv1.OpenRequest) relayFrame {
	frame := relayFrame{relaySelector: relaySelector{Direction: "up"}}
	switch {
	case msg.GetEvent() != nil:
		ev := msg.GetEvent()
		frame.Frame, frame.EventNumber = "event", ev.GetNumber()
		normalized := proto.CloneOf(ev)
		normalized.Number = 0
		if data, err := (proto.MarshalOptions{Deterministic: true}).Marshal(normalized); err == nil {
			sum := sha256.Sum256(data)
			frame.Digest = hex.EncodeToString(sum[:])
		}
	case msg.GetCommandResult() != nil:
		result := msg.GetCommandResult()
		frame.Frame, frame.CommandID, frame.Attempt = "command_result", safeIdentifier(result.GetCommandId()), result.GetAttempt()
		outcome, mark := result.GetOutcome(), result.GetLastEventNumber()
		frame.Outcome, frame.LastEventNumber = &outcome, &mark
	case msg.GetProof() != nil:
		frame.Frame = "proof"
	case msg.GetHeartbeat() != nil:
		frame.Frame = "heartbeat"
	}
	return frame
}

func describeReply(msg *linkv1.OpenResponse) relayFrame {
	frame := relayFrame{relaySelector: relaySelector{Direction: "down"}}
	switch {
	case msg.GetCommand() != nil:
		cmd := msg.GetCommand()
		frame.Frame, frame.CommandID, frame.Attempt = "command", safeIdentifier(cmd.GetId()), cmd.GetAttempt()
	case msg.GetChallenge() != nil:
		frame.Frame = "challenge"
	case msg.GetWelcome() != nil:
		frame.Frame = "welcome"
		mark := msg.GetWelcome().GetLastEventNumber()
		frame.LastEventNumber = &mark
		frame.LinkRevision = msg.GetWelcome().GetLinkRevision()
	case msg.GetEventAck() != nil:
		frame.Frame, frame.EventNumber = "event_ack", msg.GetEventAck().GetThrough()
	case msg.GetHeartbeatAck() != nil:
		frame.Frame = "heartbeat_ack"
	}
	return frame
}

func safeIdentifier(id string) string {
	if !canonicalID.MatchString(id) {
		return ""
	}
	return id
}

func relayOpenError(downstream *relayDownstream, upstream *relayUpstream, err error) error {
	if connect.IsWireError(err) && connect.CodeOf(err) == connect.CodeFailedPrecondition {
		reasons := upstream.ResponseTrailer().Values(linkRefusal)
		if len(reasons) == 1 {
			copyRefusal(downstream.ResponseTrailer(), linkRefusal, reasons)
		}
	}
	return publicLinkError(err)
}

func copyRefusal(dst http.Header, header string, reasons []string) {
	for _, reason := range reasons {
		switch header {
		case joinRefusal:
			switch reason {
			case "token_unknown", "token_expired", "token_revoked", "token_used", "machine_removed":
				dst.Add(header, reason)
			}
		case linkRefusal:
			switch reason {
			case "unknown_machine", "link_revision_mismatch", "event_gap":
				dst.Add(header, reason)
			}
		}
	}
}

func publicLinkError(err error) *connect.Error {
	code := connect.CodeOf(err)
	if errors.Is(err, context.Canceled) {
		code = connect.CodeCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		code = connect.CodeDeadlineExceeded
	}
	if errors.Is(err, errObservation) {
		return connect.NewError(connect.CodeUnavailable, errObservation)
	}
	// Only fixed public sentences cross this boundary. Transport errors may contain private data.
	var wireErr *connect.Error
	if connect.IsWireError(err) && errors.As(err, &wireErr) {
		switch wireErr.Message() {
		case invalidLinkMessage,
			"The server cannot reach its database, try again.",
			"The server does not know this join token.",
			"This join token has expired. Make a new one.",
			"This join token was revoked. Make a new one.",
			"This join token was already used. Make a new one.",
			"The machine that used this join token was removed. Make a new token.",
			"The server does not know this machine. Join it again.",
			"The server speaks revision 1 of the link.",
			"The server speaks revision 2 of the link.",
			"The event leaves a gap after the server's mark.",
			"The machine's signature did not verify.",
			"A newer connection of this machine replaced this one.",
			"The machine did not answer in time.",
			"The server is stopping.":
			return connect.NewError(code, errors.New(wireErr.Message()))
		}
	}
	return connect.NewError(code, errDisconnected)
}

func (r *frameRelay) startStream(ctx context.Context) *relayStream {
	r.mu.Lock()
	defer r.mu.Unlock()
	connectionID, _ := ctx.Value(relayConnectionKey{}).(uint64)
	leg := &relayStream{id: rand.Text(), connectionID: connectionID}
	r.streams[leg.id] = leg
	return leg
}

func (r *frameRelay) closeStream(leg *relayStream) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.streams, leg.id)
	for _, rule := range leg.rules {
		if rule.held != nil {
			if err := r.record(leg, "discarded", *rule.held); err != nil {
				stopRelay(errObservation.Error())
			}
		}
	}
	if err := r.record(leg, "closed", relayFrame{}); err != nil {
		stopRelay(errObservation.Error())
	}
}

func (r *frameRelay) forwarded(leg *relayStream, frame relayFrame) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.streams[leg.id] == nil || frame.Frame == "" {
		return nil
	}
	return r.record(leg, "forwarded", frame)
}

// record appends a single observation while the caller holds the relay mutex.
func (r *frameRelay) record(leg *relayStream, action string, frame relayFrame) error {
	if frame.MachineID == "" {
		frame.MachineID = leg.machine
	}
	if err := r.output.Encode(relayObservation{
		relayFrame: frame, ConnectionID: leg.connectionID, StreamID: leg.id, Generation: leg.generation, Action: action,
	}); err != nil {
		return connect.NewError(connect.CodeUnavailable, errObservation)
	}
	return nil
}

func (r *frameRelay) hold(leg *relayStream, frame relayFrame) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.streams[leg.id] == nil {
		return true, nil
	}
	if err := r.reconcileControl(); err != nil {
		return false, err
	}
	if frame.Frame == "open" {
		leg.machine = frame.MachineID
	}
	frame.MachineID = leg.machine
	for i := range leg.rules {
		rule := &leg.rules[i]
		if rule.selector == frame.relaySelector {
			if rule.held == nil {
				rule.held = &frame
				return true, r.record(leg, "held", frame)
			}
			return true, nil
		}
	}
	for _, rule := range leg.rules {
		if rule.held != nil && frame.Frame == "event" && rule.selector.Frame == "event" &&
			frame.EventNumber > rule.selector.EventNumber {
			// The client owns delivery of every numbered successor on a fresh stream.
			return true, nil
		}
	}
	return false, nil
}

func (r *frameRelay) watchControl(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.mu.Lock()
			err := r.reconcileControl()
			r.mu.Unlock()
			if err != nil {
				stopRelay(errObservation.Error())
			}
		}
	}
}

// reconcileControl runs with the relay mutex held so a replacement and a frame
// classification cannot observe different generations out of order.
func (r *frameRelay) reconcileControl() error {
	data, err := os.ReadFile(r.controlPath)
	if err != nil {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var control relayControl
	if decoder.Decode(&control) != nil || decoder.Decode(new(json.RawMessage)) != io.EOF ||
		control.Action != "arm" || control.Generation <= r.lastGeneration ||
		!streamIDFormat.MatchString(control.StreamID) || len(control.Rules) == 0 {
		return nil
	}
	machine := control.Rules[0].MachineID
	if safeIdentifier(machine) == "" {
		return nil
	}
	seen := make(map[relaySelector]bool, len(control.Rules))
	for _, rule := range control.Rules {
		if rule.MachineID != machine || !validRule(rule) || seen[rule] {
			return nil
		}
		seen[rule] = true
	}
	leg := r.streams[control.StreamID]
	if leg != nil && leg.machine != "" && (leg.machine != machine || leg.generation != 0) {
		return nil
	}
	r.lastGeneration = control.Generation
	if leg == nil || leg.machine == "" {
		// A control observed before its stream is live retires without arming.
		return nil
	}
	// A stream keeps its original rules through closure. The relay-wide high
	// water mark still rejects their generation after the stream is gone.
	leg.generation = control.Generation
	for _, selector := range control.Rules {
		leg.rules = append(leg.rules, relayRule{selector: selector})
		if err := r.record(leg, "armed", relayFrame{relaySelector: selector}); err != nil {
			return err
		}
	}
	return nil
}

func validRule(rule relaySelector) bool {
	if rule.Direction != "up" {
		return false
	}
	switch rule.Frame {
	case "event":
		return rule.EventNumber > 0 && rule.EventNumber <= math.MaxInt64 && rule.CommandID == "" && rule.Attempt == 0
	case "command_result":
		return safeIdentifier(rule.CommandID) != "" && rule.Attempt > 0 && rule.EventNumber == 0
	default:
		return false
	}
}
