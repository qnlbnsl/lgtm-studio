package store

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"

	"github.com/jackc/pgx/v5"
)

// RequestKeyHeader names the header that carries a caller's request key.
const RequestKeyHeader = "Lgtm-Request-Key"

const (
	maxRequestKeyBytes = 256
	maxReceiptReply    = 65536
)

// Request describes a keyed call that WriteOnce may answer from a saved receipt.
// Caller is the authenticated principal. Fingerprints holds the semantic request encoding of each retained version.
// Set ReceiptSafe only for a procedure whose fingerprint and reply contain no credential.
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

// WriteOnce runs write at most once for each caller and request key.
// A retry with the same request returns the saved reply without calling write.
// A request without a key runs write through Write every time.
func (s *Store) WriteOnce(
	ctx context.Context,
	req Request,
	write func(context.Context, *Tx) ([]byte, error),
) ([]byte, error) {
	var reply []byte
	if req.Key == "" {
		err := s.writeFeed(ctx, func(ctx context.Context, tx *Tx) error {
			var err error
			reply, err = write(ctx, tx)
			return err
		})
		if err != nil {
			return nil, err
		}
		return reply, nil
	}

	if len(req.Key) > maxRequestKeyBytes || !validText(req.Key) {
		return nil, ErrInvalidRequestKey
	}
	current := req.Fingerprints[req.FingerprintVersion]
	if !req.ReceiptSafe || req.Caller == "" || req.Procedure == "" || req.FingerprintVersion <= 0 || len(current) == 0 {
		return nil, ErrInvalidRequest
	}
	principal := sha256.Sum256([]byte(req.Caller))

	err := s.writeFeed(ctx, func(ctx context.Context, tx *Tx) error {
		// The transaction-level key lock serializes callers that present the same key, including a first insert.
		if _, err := tx.tx.Exec(ctx, `
SELECT pg_advisory_xact_lock($1::int4, hashtext(encode($2::bytea, 'hex') || ':' || $3))`,
			receiptLockNamespace, principal[:], req.Key); err != nil {
			return err
		}
		var version int16
		var saved, savedReply []byte
		err := tx.tx.QueryRow(ctx, `
SELECT fingerprint_version, request_hash, reply FROM receipts
WHERE principal_hash = $1 AND request_key = $2
FOR UPDATE`, principal[:], req.Key).Scan(&version, &saved, &savedReply)
		switch {
		case err == nil:
			fingerprint := req.Fingerprints[version]
			if len(fingerprint) == 0 {
				return ErrRequestFingerprintUnavailable
			}
			digest := requestDigest(version, req.Caller, req.Procedure, fingerprint)
			if subtle.ConstantTimeCompare(digest[:], saved) != 1 {
				return ErrRequestKeyConflict
			}
			reply = savedReply
			return nil
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}

		result, err := write(ctx, tx)
		if tx.err != nil {
			return tx.err
		}
		if err != nil {
			return err
		}
		if len(result) > maxReceiptReply {
			return ErrReceiptReplyTooLarge
		}
		if result == nil {
			result = []byte{}
		}
		digest := requestDigest(req.FingerprintVersion, req.Caller, req.Procedure, current)
		if _, err := tx.tx.Exec(ctx, `
INSERT INTO receipts (principal_hash, request_key, fingerprint_version, request_hash, reply)
VALUES ($1, $2, $3, $4, $5)`, principal[:], req.Key, req.FingerprintVersion, digest[:], result); err != nil {
			return err
		}
		reply = result
		return nil
	})
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// WriteOnceRetained is a stub of the test writer.
func (s *Store) WriteOnceRetained(
	ctx context.Context, req Request,
	start func(context.Context, *Tx) ([]byte, error),
	replay func(context.Context, *Tx, []byte) error,
) ([]byte, error) {
	return nil, nil
}

// RetireReceipt is a stub of the test writer.
func (s *Store) RetireReceipt(
	ctx context.Context, caller string, key string, expectedReply []byte,
	remove func(context.Context, *Tx) (bool, error),
) (bool, error) {
	return false, nil
}

// requestDigest hashes the version as two big-endian bytes,
// then the caller, procedure and fingerprint, each after its eight-byte big-endian length.
func requestDigest(version int16, caller, procedure string, fingerprint []byte) [sha256.Size]byte {
	framed := make([]byte, 0, 2+3*8+len(caller)+len(procedure)+len(fingerprint))
	framed = binary.BigEndian.AppendUint16(framed, uint16(version))
	for _, field := range [][]byte{[]byte(caller), []byte(procedure), fingerprint} {
		framed = binary.BigEndian.AppendUint64(framed, uint64(len(field)))
		framed = append(framed, field...)
	}
	return sha256.Sum256(framed)
}
