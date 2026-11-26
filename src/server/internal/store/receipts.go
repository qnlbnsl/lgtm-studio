package store

import "context"

// This file contains test writer stubs for request receipts.

const RequestKeyHeader = "Lgtm-Request-Key"

type Request struct {
	Key                string
	ReceiptSafe        bool
	Caller             string
	Procedure          string
	FingerprintVersion int16
	Fingerprints       map[int16][]byte
}

var (
	ErrInvalidRequestKey             = NewDomainError("The request key is invalid.")
	ErrInvalidRequest                = NewDomainError("The request is invalid.")
	ErrRequestKeyConflict            = NewDomainError("That request key was already used for a different request.")
	ErrRequestFingerprintUnavailable = NewDomainError("This server cannot verify the saved request.")
	ErrReceiptReplyTooLarge          = NewDomainError("The receipt reply is too large.")
)

func (s *Store) WriteOnce(context.Context, Request, func(context.Context, *Tx) ([]byte, error)) ([]byte, error) {
	return nil, nil
}
