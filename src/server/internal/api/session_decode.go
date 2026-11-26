package api

import (
	"context"
	"errors"
	"fmt"
	"mime"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	lgtmv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
)

const messageMalformed = "The request body is invalid."
const messageMalformedJoin = "The link message broke a rule of the contract."

// malformedMark is an unknown field that the JSON codec leaves on a message it could not decode.
// JSON decoding discards unknown fields, so a decoded JSON body never carries it.
var malformedMark = protowire.AppendBytes(protowire.AppendTag(nil, protowire.MaxValidNumber, protowire.BytesType), nil)

// redactMalformedJSON hides the cause of a malformed JSON body, because the cause can repeat what was submitted.
// The codec marks the message so unary interceptors and stream handlers can return a fixed sentence.
// Connect checks the read bound before decoding, so the size error keeps its own message.
func redactMalformedJSON(message string) connect.HandlerOption {
	publicError := errors.New(message)
	return connect.WithHandlerOptions(
		connect.WithCodec(redactingJSON{"json"}),
		connect.WithCodec(redactingJSON{"json; charset=utf-8"}),
		connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				if isMalformedJSON(req) {
					return nil, connect.NewError(connect.CodeInvalidArgument, publicError)
				}
				return next(ctx, req)
			}
		})),
	)
}

type redactingClientService struct {
	lgtmv1connect.ClientServiceHandler
}

func (s redactingClientService) WatchClients(ctx context.Context, req *connect.Request[lgtmv1.WatchClientsRequest], stream *connect.ServerStream[lgtmv1.WatchClientsResponse]) error {
	if isMalformedJSON(req) {
		return connect.NewError(connect.CodeInvalidArgument, errors.New(messageMalformed))
	}
	return s.ClientServiceHandler.WatchClients(ctx, req, stream)
}

func isMalformedJSON(req connect.AnyRequest) bool {
	msg, ok := req.Any().(proto.Message)
	if !ok || string(msg.ProtoReflect().GetUnknown()) != string(malformedMark) {
		return false
	}
	// A valid binary Protocol Buffers request may carry the same unknown field.
	mediaType, _, _ := mime.ParseMediaType(req.Header().Get("Content-Type"))
	switch mediaType {
	case "application/json", "application/connect+json", "application/grpc+json", "application/grpc-web+json":
		return true
	default:
		return false
	}
}

// redactingJSON is Connect's JSON codec, except that a body it cannot decode leaves only the mark.
type redactingJSON struct{ name string }

func (c redactingJSON) Name() string { return c.name }

func (c redactingJSON) Marshal(message any) ([]byte, error) {
	msg, ok := message.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("%T is not a Protocol Buffers message", message)
	}
	return protojson.Marshal(msg)
}

func (c redactingJSON) Unmarshal(data []byte, message any) error {
	msg, ok := message.(proto.Message)
	if !ok {
		return fmt.Errorf("%T is not a Protocol Buffers message", message)
	}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, msg); err != nil {
		proto.Reset(msg)
		msg.ProtoReflect().SetUnknown(malformedMark)
	}
	return nil
}
