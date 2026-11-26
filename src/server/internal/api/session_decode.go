package api

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

const messageMalformed = "The request body is invalid."

// malformedMark is an unknown field that the Session JSON codec leaves on a message it could not decode.
// JSON decoding discards unknown fields, so a decoded JSON body never carries it.
var malformedMark = protowire.AppendBytes(protowire.AppendTag(nil, protowire.MaxValidNumber, protowire.BytesType), nil)

// sessionDecoding hides the cause of a malformed Session JSON body, because the cause can repeat what was submitted.
// Connect prefixes every codec error, so the codec marks the message and an interceptor writes the fixed sentence.
// Connect checks the read bound before decoding, so the size error keeps its own message.
func sessionDecoding() connect.HandlerOption {
	return connect.WithHandlerOptions(
		connect.WithCodec(sessionJSON{"json"}),
		connect.WithCodec(sessionJSON{"json; charset=utf-8"}),
		connect.WithInterceptors(connect.UnaryInterceptorFunc(refuseMalformed)),
	)
}

func refuseMalformed(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if msg, ok := req.Any().(proto.Message); ok && isMalformed(msg) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New(messageMalformed))
		}
		return next(ctx, req)
	}
}

func isMalformed(msg proto.Message) bool {
	return string(msg.ProtoReflect().GetUnknown()) == string(malformedMark)
}

// sessionJSON is Connect's JSON codec, except that a body it cannot decode leaves only the mark.
type sessionJSON struct{ name string }

func (c sessionJSON) Name() string { return c.name }

func (c sessionJSON) Marshal(message any) ([]byte, error) {
	msg, ok := message.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("%T is not a Protocol Buffers message", message)
	}
	return protojson.Marshal(msg)
}

func (c sessionJSON) Unmarshal(data []byte, message any) error {
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
