package link

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

func TestOpenRefusalReasonUsesTrailingMetadata(t *testing.T) {
	const key = "Lgtm-Link-Refusal"
	for _, p := range protocols {
		for _, tc := range []struct {
			name, trailingReason string
		}{
			{name: "leading reason only"},
			{name: "different leading and trailing reasons", trailingReason: "link_revision_mismatch"},
		} {
			t.Run(p.name+"/"+tc.name, func(t *testing.T) {
				_, client, server, cred := startScene(t, p, func(s *scene, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
					hello, err := s.readHello(stream)
					if err != nil {
						return err
					}
					stream.ResponseHeader().Set(key, "unknown_machine")
					if err := stream.Send(s.challenge(hello, fixedServerNonce)); err != nil {
						return err
					}
					proof, err := s.readProof(stream)
					if err != nil {
						return err
					}
					if !s.verifiesProof(hello, fixedServerNonce, proof) {
						return connect.NewError(connect.CodeUnauthenticated, errors.New("bad fixture proof"))
					}
					refusal := connect.NewError(connect.CodeFailedPrecondition, errors.New("fixture refusal"))
					if tc.trailingReason != "" {
						refusal.Meta().Set(key, tc.trailingReason)
					}
					return refusal
				})

				_, err := open(t, client, server, cred)
				diagnostic, terminal := isTerminal(err)
				if tc.trailingReason == "" {
					if terminal || connect.CodeOf(err) != connect.CodeFailedPrecondition {
						t.Fatal("a failed precondition with a leading reason alone must stay retryable")
					}
				} else if !terminal || diagnostic != tc.trailingReason {
					t.Fatal("the terminal refusal did not use the reason in trailing metadata")
				}
			})
		}
	}
}

func TestOpenRejectsMalformedWelcomeBeforeRevisionMismatch(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*linkv1.Welcome)
	}{
		{"removed state", func(w *linkv1.Welcome) {
			w.State.Approval = linkv1.Approval_APPROVAL_REMOVED
			w.ConnectionToken = ""
		}},
		{"waiting state with a token", func(w *linkv1.Welcome) {
			w.State.Approval = linkv1.Approval_APPROVAL_WAITING
			w.State.Name = ""
		}},
		{"approved state without a token", func(w *linkv1.Welcome) { w.ConnectionToken = "" }},
		{"missing state", func(w *linkv1.Welcome) { w.State = nil }},
		{"invalid server time", func(w *linkv1.Welcome) { w.ServerTime = &timestamppb.Timestamp{Nanos: -1} }},
	} {
		w := goodWelcome()
		w.LinkRevision = 2
		tc.edit(w)
		if !openRejectsWelcome(t, w) {
			t.Errorf("a revision mismatch must be retryable when the welcome has %s", tc.name)
		}
	}
}

func TestOpenRejectsNamedWaitingWelcome(t *testing.T) {
	w := goodWelcome()
	w.State.Approval = linkv1.Approval_APPROVAL_WAITING
	w.ConnectionToken = ""
	if !openRejectsWelcome(t, w) {
		t.Fatal("a waiting welcome with a machine name must be rejected")
	}
}

func TestOpenRejectsApprovedWelcomeWithoutToken(t *testing.T) {
	w := goodWelcome()
	w.ConnectionToken = ""
	if !openRejectsWelcome(t, w) {
		t.Fatal("an approved welcome without a token must be rejected")
	}
}

func openRejectsWelcome(t *testing.T, w *linkv1.Welcome) bool {
	t.Helper()
	_, client, server, cred := startScene(t, protocols[0], honest(w))
	sess, err := open(t, client, server, cred)
	_, terminal := isTerminal(err)
	return sess == nil && errors.Is(err, errInvalidWelcome) && !terminal
}
