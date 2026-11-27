package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"connectrpc.com/connect"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"google.golang.org/protobuf/proto"
)

func TestSprintFourRelayRejectsControlForAnotherStream(t *testing.T) {
	t.Run("AfterRestart", func(t *testing.T) {
		upstream := newRelayPeer(t, false)
		dir := t.TempDir()
		first := startReviewRelay(t, upstream.address, dir)
		serverKey := checkJoin(t, first.client, upstream, false)
		old, cancel := openLink(t, t.Context(), first.client, upstream, serverKey, 1)
		expectAttempt(t, old, 1)
		streamID := waitObservation(t, dir, func(o observation) bool {
			return o.Action == "forwarded" && o.frameSelector == commandSelector(1)
		}).StreamID
		requireStreamID(t, streamID)

		const generation = 1
		event, result := eventSelector(7), resultSelector(1)
		replaceControl(t, dir, streamID, generation, event, result)
		waitRule(t, dir, streamID, generation, "armed", event)
		waitRule(t, dir, streamID, generation, "armed", result)
		sendFrame(t, old, eventFrame(7))
		sendFrame(t, old, resultFrame(fixtureCommand, 1))
		waitRule(t, dir, streamID, generation, "held", event)
		waitRule(t, dir, streamID, generation, "held", result)
		cancel()
		_ = old.CloseRequest()
		_ = old.CloseResponse()
		waitRule(t, dir, streamID, generation, "discarded", event)
		waitRule(t, dir, streamID, generation, "discarded", result)
		waitObservation(t, dir, func(o observation) bool {
			return o.Action == "closed" && bytes.Equal(o.StreamID, streamID) && o.Generation == generation
		})
		upstream.waitOldClosed(t)
		first.stop()
		retiredRecords := len(observations(t, dir))

		second := startReviewRelay(t, upstream.address, dir)
		serverKey = checkJoin(t, second.client, upstream, false)
		next, _ := openLink(t, t.Context(), second.client, upstream, serverKey, 2)
		heartbeatBarrier(t, next)
		forwarded := waitObservation(t, dir, func(o observation) bool {
			return o.Action == "forwarded" && o.Frame == "heartbeat_ack"
		})
		requireStreamID(t, forwarded.StreamID)
		if bytes.Equal(streamID, forwarded.StreamID) {
			t.Error("The relay reused the retired stream identifier after a restart.")
		}
		sendFrame(t, next, eventFrame(7))
		replay := waitObservation(t, dir, func(o observation) bool {
			return bytes.Equal(o.StreamID, forwarded.StreamID) && o.frameSelector == event &&
				(o.Action == "forwarded" || o.Action == "held")
		})
		if replay.Action != "forwarded" {
			t.Fatal("The retained control held the fresh replay after a relay restart.")
		}
		requireEventDigest(t, replay, eventFrame(7).GetEvent())
		expectAck(t, next, 7, "The fresh replay did not receive its acknowledgement.")
		expectAttempt(t, next, 2)
		sendFrame(t, next, resultFrame(fixtureCommand, 2))
		heartbeatBarrier(t, next)
		upstream.requireTraffic(t, 2, "heartbeat", "event:7", "result:scan:2", "heartbeat")
		for _, record := range observations(t, dir)[retiredRecords:] {
			if bytes.Equal(record.StreamID, forwarded.StreamID) && (record.Action == "armed" || record.Action == "held") {
				t.Fatal("The relay applied the retained control after a restart.")
			}
		}
	})

	for _, beforeAllocation := range []bool{true, false} {
		name := "BeforeHello"
		if beforeAllocation {
			name = "BeforeAllocation"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			r := &frameRelay{
				controlPath: filepath.Join(dir, "control.json"),
				output:      json.NewEncoder(io.Discard),
			}
			r.streams = emptyReviewMap(r.streams)
			hello := relayFrame{relaySelector: relaySelector{MachineID: fixtureMachine, Direction: "up", Frame: "open"}}
			old := r.startStream(t.Context())
			if held, err := r.hold(old, hello); held || err != nil {
				t.Fatal("The first stream could not establish its identity.")
			}
			next := r.startStream(t.Context())
			nextID, err := json.Marshal(next.id)
			if err != nil {
				t.Fatal("Could not encode the replacement stream identifier.")
			}
			// Reserve the exact identifier for the test without predicting the allocator's next value.
			if beforeAllocation {
				delete(r.streams, next.id)
			}
			replaceControl(t, dir, nextID, 1, eventSelector(7))
			heartbeat := relayFrame{relaySelector: relaySelector{Direction: "up", Frame: "heartbeat"}}
			if held, err := r.hold(old, heartbeat); held || err != nil {
				t.Fatal("The live stream could not observe the early control.")
			}
			r.closeStream(old)
			r.streams[next.id] = next
			if held, err := r.hold(next, hello); held || err != nil {
				t.Fatal("The replacement stream could not establish its identity.")
			}
			frame := describeRequest(eventFrame(7))
			if held, err := r.hold(next, frame); held || err != nil {
				t.Fatal("A control observed before its stream was live held the replacement event.")
			}
			replaceControl(t, dir, nextID, 2, eventSelector(7))
			if held, err := r.hold(next, frame); !held || err != nil {
				t.Fatal("A fresh control for the live replacement stream did not hold its event.")
			}
		})
	}
}

// The synthetic stream table follows the relay's identifier type.
func emptyReviewMap[K comparable, V any](_ map[K]V) map[K]V {
	return make(map[K]V)
}

func TestSprintFourRelayUsesOnlyOneContractualRefusalTrailer(t *testing.T) {
	const header = "Lgtm-Link-Refusal"
	const reason = "link_revision_mismatch"
	cases := []struct {
		name     string
		leading  string
		trailers []string
		want     string
	}{
		{name: "LeadingHeaderOnly", leading: reason},
		{name: "OneContractualTrailer", leading: "unknown_machine", trailers: []string{reason}, want: reason},
		{name: "RepeatedTrailer", trailers: []string{reason, reason}},
		{name: "UnknownTrailer", trailers: []string{"unlisted_reason"}},
		{name: "MixedTrailers", trailers: []string{reason, "unlisted_reason"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			public, private, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal("Could not generate the fixture signing key.")
			}
			_, handler := linkv1connect.NewLinkServiceHandler(reviewRefusalPeer{
				key: private, leading: tc.leading, trailers: tc.trailers,
			})
			upstream := httptest.NewUnstartedServer(handler)
			protocols := new(http.Protocols)
			protocols.SetUnencryptedHTTP2(true)
			upstream.Config.Protocols = protocols
			upstream.Start()
			t.Cleanup(upstream.Close)
			transport := &http.Transport{Protocols: protocols}
			t.Cleanup(transport.CloseIdleConnections)
			direct := linkv1connect.NewLinkServiceClient(&http.Client{Transport: transport}, upstream.URL)
			relayed, _ := startRelay(t, upstream.URL, nil)

			receiveRefusal := func(client linkv1connect.LinkServiceClient) *wireStream {
				t.Helper()
				ctx, cancel := context.WithTimeout(t.Context(), waitLimit)
				t.Cleanup(cancel)
				stream := client.Open(ctx)
				sendHelloAndVerifyChallenge(t, stream, public, 1)
				reply, err := stream.Receive()
				if reply != nil || connect.CodeOf(err) != connect.CodeFailedPrecondition {
					t.Fatal("The signed Challenge was not followed by the expected refusal status.")
				}
				return stream
			}
			original := receiveRefusal(direct)
			actualLeading := original.ResponseHeader().Get(header)
			actualTrailers := original.ResponseTrailer().Values(header)
			if actualLeading != tc.leading || !slices.Equal(actualTrailers, tc.trailers) {
				t.Fatal("The upstream did not send the fixture's refusal metadata.")
			}

			reply := receiveRefusal(relayed)
			if len(reply.ResponseHeader().Values(header)) != 0 {
				t.Fatal("The relay sent refusal metadata in a leading response header.")
			}
			relayedTrailers := reply.ResponseTrailer().Values(header)
			if tc.want == "" && len(relayedTrailers) != 0 {
				t.Fatal("The relay promoted invalid refusal metadata to a terminal trailer.")
			}
			if tc.want != "" && (len(relayedTrailers) != 1 || relayedTrailers[0] != tc.want) {
				t.Fatal("The relay did not preserve the single contractual refusal trailer.")
			}
		})
	}
}

type reviewRefusalPeer struct {
	linkv1connect.UnimplementedLinkServiceHandler
	key      ed25519.PrivateKey
	leading  string
	trailers []string
}

func (p reviewRefusalPeer) Open(_ context.Context, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
	first, err := stream.Receive()
	if err != nil {
		return err
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return connect.NewError(connect.CodeInternal, errors.New("The fixture could not create a nonce."))
	}
	if p.leading != "" {
		stream.ResponseHeader().Set("Lgtm-Link-Refusal", p.leading)
	}
	challenge := &linkv1.Challenge{
		ServerNonce:     nonce,
		ServerSignature: ed25519.Sign(p.key, proofText("server", first.GetHello().GetMachineNonce(), nonce)),
	}
	if err := stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_Challenge{Challenge: challenge}}); err != nil {
		return err
	}
	for _, trailer := range p.trailers {
		stream.ResponseTrailer().Add("Lgtm-Link-Refusal", trailer)
	}
	return connect.NewError(connect.CodeFailedPrecondition, errors.New("The server speaks revision 2 of the link."))
}

func TestSprintFourRelaySanitizesMalformedJoin(t *testing.T) {
	upstream := newRelayPeer(t, false)
	relay := startReviewRelay(t, upstream.address, t.TempDir())
	ctx, cancel := context.WithTimeout(t.Context(), waitLimit)
	defer cancel()
	var fixed string
	for _, submitted := range []string{"!first-private-join-fixture!", "!second-private-join-fixture!"} {
		body := strings.NewReader(`{"tokenSecret":"` + submitted + `"}`)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, relay.url+linkv1connect.LinkServiceJoinProcedure, body)
		if err != nil {
			t.Fatal("Could not construct the malformed Join request.")
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Connect-Protocol-Version", "1")
		response, err := relay.http.Do(req)
		if err != nil {
			t.Fatal("Could not send the malformed Join request.")
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, 1<<16))
		_ = response.Body.Close()
		var reply struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if err != nil || response.StatusCode != http.StatusBadRequest ||
			json.Unmarshal(data, &reply) != nil || reply.Code != "invalid_argument" || reply.Message == "" {
			t.Fatal("Malformed Join did not return a public invalid-argument error.")
		}
		if bytes.Contains(data, []byte(submitted)) || strings.Contains(reply.Message, submitted) {
			t.Error("The malformed Join response included the submitted enrollment secret.")
		}
		if fixed != "" && reply.Message != fixed {
			t.Error("The malformed Join response message changed with the submitted value.")
		}
		fixed = reply.Message
	}
	checkJoin(t, relay.client, upstream, false)
}

func TestSprintFourRelayFailsWhenTerminalObservationCannotBeWritten(t *testing.T) {
	for _, holdFrames := range []bool{true, false} {
		name := "Close"
		if holdFrames {
			name = "DiscardAndClose"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			upstream := newRelayPeer(t, false)
			dir := t.TempDir()
			path := filepath.Join(dir, "observations.jsonl")
			if syscall.Mkfifo(path, 0600) != nil {
				t.Fatal("Could not create the observation pipe.")
			}
			reader, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
			if err != nil {
				t.Fatal("Could not open the observation pipe.")
			}
			t.Cleanup(func() { _ = reader.Close() })
			relay := startReviewRelay(t, upstream.address, dir)
			records := make(chan observation, 32)
			go func() {
				defer close(records)
				scanner := bufio.NewScanner(reader)
				for scanner.Scan() {
					var record observation
					if json.Unmarshal(scanner.Bytes(), &record) != nil {
						return
					}
					select {
					case records <- record:
					case <-t.Context().Done():
						return
					}
				}
			}()
			waitRecord := func(matches func(observation) bool) observation {
				t.Helper()
				timeout := time.NewTimer(waitLimit)
				defer timeout.Stop()
				for {
					select {
					case record, ok := <-records:
						if !ok {
							t.Fatal("The observation pipe ended before the required record.")
						}
						if matches(record) {
							return record
						}
					case <-timeout.C:
						t.Fatal("The observation pipe did not receive the required record.")
					}
				}
			}

			serverKey := checkJoin(t, relay.client, upstream, false)
			stream, cancel := openLink(t, t.Context(), relay.client, upstream, serverKey, 1)
			expectAttempt(t, stream, 1)
			var streamID json.RawMessage
			pending := map[string]bool{"open": true, "challenge": true, "proof": true, "welcome": true, "command": true}
			for len(pending) > 0 {
				record := waitRecord(func(o observation) bool {
					return o.Action == "forwarded" && pending[o.Frame]
				})
				if streamID == nil {
					streamID = record.StreamID
				}
				if !bytes.Equal(streamID, record.StreamID) {
					t.Fatal("The handshake observations came from different streams.")
				}
				delete(pending, record.Frame)
			}
			requireStreamID(t, streamID)
			if holdFrames {
				const generation = 1
				event, result := eventSelector(7), resultSelector(1)
				replaceControl(t, dir, streamID, generation, event, result)
				requireRule := func(action string, rule frameSelector) {
					waitRecord(func(o observation) bool {
						return o.Action == action && bytes.Equal(o.StreamID, streamID) &&
							o.Generation == generation && o.frameSelector == rule
					})
				}
				requireRule("armed", event)
				requireRule("armed", result)
				sendFrame(t, stream, eventFrame(7))
				sendFrame(t, stream, resultFrame(fixtureCommand, 1))
				requireRule("held", event)
				requireRule("held", result)
			}

			if reader.Close() != nil {
				t.Fatal("Could not disconnect the observation reader.")
			}
			cancel()
			_ = stream.CloseRequest()
			_ = stream.CloseResponse()
			upstream.waitOldClosed(t)
			select {
			case <-relay.done:
				var failure *exec.ExitError
				if !errors.As(relay.exitErr, &failure) || failure.ExitCode() <= 0 {
					t.Fatal("The relay did not exit with a failure status.")
				}
				if strings.TrimSpace(relay.stderr.String()) != "The relay could not record an observation." {
					t.Fatal("The relay did not emit only the fixed observation failure diagnostic.")
				}
			case <-time.After(waitLimit):
				t.Fatal("The relay stayed running after a required terminal observation write failed.")
			}
		})
	}
}

func TestSprintFourRelayObservationsExcludePrivateLinkData(t *testing.T) {
	upstream := newRelayPeer(t, false)
	client, dir := startRelay(t, upstream.address, nil)
	serverKey := checkJoin(t, client, upstream, true)

	ctx, cancel := context.WithTimeout(t.Context(), 3*waitLimit)
	defer cancel()
	stream := client.Open(ctx)
	machineNonce, serverNonce := sendHelloAndVerifyChallenge(t, stream, serverKey, 1)
	machineSignature := ed25519.Sign(upstream.machineKey, proofText("machine", machineNonce, serverNonce))
	serverSignature := ed25519.Sign(upstream.serverKey, proofText("server", machineNonce, serverNonce))
	sendFrame(t, stream, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Proof{
		Proof: &linkv1.Proof{MachineSignature: machineSignature},
	}})
	welcome, err := stream.Receive()
	connectionToken := welcome.GetWelcome().GetConnectionToken()
	if err != nil || connectionToken == "" {
		t.Fatal("The fixture did not establish a proven link with a connection token.")
	}
	expectAttempt(t, stream, 1)
	streamID := waitObservation(t, dir, func(o observation) bool {
		return o.Action == "forwarded" && o.frameSelector == commandSelector(1)
	}).StreamID
	requireStreamID(t, streamID)

	const generation = 1
	event, result := eventSelector(7), resultSelector(1)
	forwardedEvent := eventFrame(6)
	sendFrame(t, stream, forwardedEvent)
	expectAck(t, stream, 6, "The fixture did not forward its first event.")
	replaceControl(t, dir, streamID, generation, event, result)
	waitRule(t, dir, streamID, generation, "armed", event)
	waitRule(t, dir, streamID, generation, "armed", result)
	heldEvent := eventFrame(7)
	const payload = "private-relay-observation-fixture-payload"
	heldEvent.GetEvent().GetHarnessesFound().Harnesses[0].Name = payload
	sendFrame(t, stream, heldEvent)
	sendFrame(t, stream, resultFrame(fixtureCommand, 1))
	requireEventDigest(t, waitRule(t, dir, streamID, generation, "held", event), heldEvent.GetEvent())
	requireDone(t, waitRule(t, dir, streamID, generation, "held", result))
	heartbeatBarrier(t, stream)
	cancel()
	_ = stream.CloseRequest()
	_ = stream.CloseResponse()
	waitRule(t, dir, streamID, generation, "discarded", event)
	waitRule(t, dir, streamID, generation, "discarded", result)
	waitObservation(t, dir, func(o observation) bool {
		return o.Action == "closed" && o.Generation == generation && bytes.Equal(o.StreamID, streamID)
	})

	private := [][]byte{
		[]byte(fixtureTokenID), upstream.tokenSecret, make([]byte, 32), []byte(connectionToken),
		upstream.machineKey, upstream.machineKey.Seed(), upstream.machineKey.Public().(ed25519.PublicKey),
		upstream.serverKey, upstream.serverKey.Seed(), serverKey,
		machineNonce, serverNonce, machineSignature, serverSignature,
		[]byte(payload), []byte(forwardedEvent.GetEvent().GetFacts().GetOs()),
	}
	for _, frame := range []*linkv1.OpenRequest{forwardedEvent, heldEvent} {
		encoded, err := proto.Marshal(frame.GetEvent())
		if err != nil {
			t.Fatal("Could not encode the fixture event.")
		}
		private = append(private, encoded)
	}
	requireSafeReviewObservations(t, dir, private)
}

func requireSafeReviewObservations(t *testing.T, dir string, private [][]byte) {
	t.Helper()
	encoded := make([][]byte, 0, len(private)*7)
	for _, value := range private {
		hexValue := hex.EncodeToString(value)
		encoded = append(encoded, value, []byte(hexValue), []byte(strings.ToUpper(hexValue)))
		for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
			encoded = append(encoded, []byte(encoding.EncodeToString(value)))
		}
	}
	checkPrivate := func(data []byte) {
		t.Helper()
		for _, value := range encoded {
			if bytes.Contains(data, value) {
				t.Fatal("A relay observation contained private fixture data.")
			}
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "observations.jsonl"))
	if err != nil || len(data) == 0 {
		t.Fatal("Could not read the complete relay observation file.")
	}
	checkPrivate(data)
	allowed := map[string]bool{
		"machine_id": true, "direction": true, "frame": true,
		"event_number": true, "command_id": true, "attempt": true,
		"stream_id": true, "connection_id": true, "generation": true, "action": true,
		"digest": true, "outcome": true, "last_event_number": true, "link_revision": true,
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		first, err := decoder.Token()
		if err != nil || first != json.Delim('{') {
			t.Fatal("A relay observation was not a JSON object.")
		}
		fields := make(map[string]bool)
		for decoder.More() {
			key, err := decoder.Token()
			field, ok := key.(string)
			if err != nil || !ok || !allowed[field] || fields[field] {
				t.Fatal("A relay observation contained an unexpected or duplicate field.")
			}
			fields[field] = true
			value, err := decoder.Token()
			if err != nil {
				t.Fatal("A relay observation contained a malformed field.")
			}
			switch value := value.(type) {
			case string:
				checkPrivate([]byte(value))
			case float64, nil:
			default:
				t.Fatal("A relay observation field had an unexpected value type.")
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') || decoder.Decode(new(json.RawMessage)) != io.EOF {
			t.Fatal("A relay observation contained malformed or trailing data.")
		}
	}
	if scanner.Err() != nil {
		t.Fatal("Could not read all relay observation records.")
	}
}

type reviewRelayProcess struct {
	client  linkv1connect.LinkServiceClient
	http    *http.Client
	url     string
	stop    func()
	done    chan struct{}
	exitErr error
	stderr  bytes.Buffer
}

func startReviewRelay(t *testing.T, upstream, dir string) *reviewRelayProcess {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal("Could not reserve a private relay address.")
	}
	address := listener.Addr().String()
	if listener.Close() != nil || strings.HasSuffix(address, ":8787") {
		t.Fatal("Could not select a nondefault relay port.")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cmd := relayCommand(t, ctx, address, upstream, dir)
	relay := &reviewRelayProcess{done: make(chan struct{}), url: "http://" + address}
	cmd.Stderr = &relay.stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		cancel()
		t.Fatal("Could not start the private relay process.")
	}
	relay.stop = func() {
		cancel()
		<-relay.done
	}
	t.Cleanup(relay.stop)

	ready := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "relay ready" || strings.HasPrefix(line, "relay ready ") {
				select {
				case <-ready:
				default:
					close(ready)
				}
			}
		}
		relay.exitErr = cmd.Wait()
		close(relay.done)
	}()
	select {
	case <-ready:
	case <-relay.done:
		t.Fatal("The private relay exited before announcing readiness.")
	case <-time.After(waitLimit):
		t.Fatal("The private relay did not announce readiness.")
	}
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	transport := &http.Transport{Protocols: protocols}
	t.Cleanup(transport.CloseIdleConnections)
	relay.http = &http.Client{Transport: transport}
	relay.client = linkv1connect.NewLinkServiceClient(relay.http, relay.url)
	return relay
}
