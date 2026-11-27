package store

import (
	"bytes"
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
	if req.Key != "" {
		return s.writeOnceKeyed(ctx, req, false, write, nil)
	}
	var reply []byte
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

// WriteOnceRetained requires a request key and saves the operation identifier from start.
// On a matching retry, it calls replay under the receipt lock with the saved identifier.
// The receipt remains held until it is retired with the operation.
func (s *Store) WriteOnceRetained(
	ctx context.Context, req Request,
	start func(context.Context, *Tx) ([]byte, error),
	replay func(context.Context, *Tx, []byte) error,
) ([]byte, error) {
	return s.writeOnceKeyed(ctx, req, true, start, replay)
}

func (s *Store) writeOnceKeyed(
	ctx context.Context, req Request, held bool,
	write func(context.Context, *Tx) ([]byte, error),
	replay func(context.Context, *Tx, []byte) error,
) ([]byte, error) {
	if !validReceiptKey(req.Key) {
		return nil, ErrInvalidRequestKey
	}
	current := req.Fingerprints[req.FingerprintVersion]
	if !req.ReceiptSafe || req.Caller == "" || req.Procedure == "" || req.FingerprintVersion <= 0 || len(current) == 0 {
		return nil, ErrInvalidRequest
	}
	principal := sha256.Sum256([]byte(req.Caller))

	var reply []byte
	err := s.writeFeed(ctx, func(ctx context.Context, tx *Tx) error {
		if err := lockReceiptKey(ctx, tx, principal, req.Key); err != nil {
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
			if replay != nil {
				if err := replay(ctx, tx, bytes.Clone(savedReply)); err != nil {
					return err
				}
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
		// Ordinary inserts also work on the schema from before held receipts.
		insert := `
INSERT INTO receipts (principal_hash, request_key, fingerprint_version, request_hash, reply)
VALUES ($1, $2, $3, $4, $5)`
		if held {
			insert = `
INSERT INTO receipts (principal_hash, request_key, fingerprint_version, request_hash, reply, held)
VALUES ($1, $2, $3, $4, $5, true)`
		}
		if _, err := tx.tx.Exec(ctx, insert, principal[:], req.Key, req.FingerprintVersion, digest[:], result); err != nil {
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

// RetireReceipt calls remove under the receipt lock if the held receipt matches expectedReply.
// When remove returns true, the operation removal and receipt deletion commit together.
// A callback that returns false without an error must leave the operation unchanged.
func (s *Store) RetireReceipt(
	ctx context.Context, caller string, key string, expectedReply []byte,
	remove func(context.Context, *Tx) (bool, error),
) (bool, error) {
	if !validReceiptKey(key) {
		return false, ErrInvalidRequestKey
	}
	if caller == "" {
		return false, ErrInvalidRequest
	}
	principal := sha256.Sum256([]byte(caller))
	var retired bool
	err := s.writeFeed(ctx, func(ctx context.Context, tx *Tx) error {
		if err := lockReceiptKey(ctx, tx, principal, key); err != nil {
			return err
		}
		var held bool
		var reply []byte
		err := tx.tx.QueryRow(ctx, `
SELECT held, reply FROM receipts
WHERE principal_hash = $1 AND request_key = $2
FOR UPDATE`, principal[:], key).Scan(&held, &reply)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil || !held || !bytes.Equal(reply, expectedReply) {
			return err
		}
		removed, err := remove(ctx, tx)
		if err != nil || !removed {
			return err
		}
		if _, err := tx.tx.Exec(ctx, `DELETE FROM receipts WHERE principal_hash = $1 AND request_key = $2`,
			principal[:], key); err != nil {
			return err
		}
		retired = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return retired, nil
}

func validReceiptKey(key string) bool {
	return key != "" && len(key) <= maxRequestKeyBytes && validText(key)
}

// lockReceiptKey serializes all writes for a caller and key, including the first insert.
func lockReceiptKey(ctx context.Context, tx *Tx, principal [sha256.Size]byte, key string) error {
	_, err := tx.tx.Exec(ctx, `
SELECT pg_advisory_xact_lock($1::int4, hashtext(encode($2::bytea, 'hex') || ':' || $3))`,
		receiptLockNamespace, principal[:], key)
	return err
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
