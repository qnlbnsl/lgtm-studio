package api

import (
	"errors"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

const linkEventLimit = 48 << 10

var errMalformedLinkFrame = errors.New(messageMalformedJoin)

// linkOpenProto checks the received Event payload after decompression and decoding.
type linkOpenProto struct{}

func (linkOpenProto) Name() string { return "proto" }

func (linkOpenProto) Marshal(message any) ([]byte, error) {
	msg, ok := message.(proto.Message)
	if !ok {
		return nil, errMalformedLinkFrame
	}
	return proto.Marshal(msg)
}

func (linkOpenProto) Unmarshal(data []byte, message any) error {
	msg, ok := message.(*linkv1.OpenRequest)
	if !ok || proto.Unmarshal(data, msg) != nil {
		return errMalformedLinkFrame
	}
	if event := msg.GetEvent(); event != nil {
		fields := msg.ProtoReflect().Descriptor().Fields()
		size, err := openEventWireSize(data, fields)
		if err != nil {
			return err
		}
		if size > linkEventLimit {
			event.Report = nil
		}
	}
	return nil
}

func openEventWireSize(data []byte, fields protoreflect.FieldDescriptors) (int, error) {
	eventNumber := fields.ByName("event").Number()
	var size int
	for len(data) > 0 {
		number, wireType, tagSize := protowire.ConsumeTag(data)
		if tagSize < 0 {
			return 0, errMalformedLinkFrame
		}
		data = data[tagSize:]
		if wireType == protowire.BytesType {
			value, fieldSize := protowire.ConsumeBytes(data)
			if fieldSize < 0 {
				return 0, errMalformedLinkFrame
			}
			// Repeated message fields merge until another Open arm takes their place.
			if number == eventNumber {
				size += len(value)
			} else if field := fields.ByNumber(number); field != nil && field.ContainingOneof() != nil {
				size = 0
			}
			data = data[fieldSize:]
			continue
		}
		fieldSize := protowire.ConsumeFieldValue(number, wireType, data)
		if fieldSize < 0 {
			return 0, errMalformedLinkFrame
		}
		data = data[fieldSize:]
	}
	return size, nil
}
