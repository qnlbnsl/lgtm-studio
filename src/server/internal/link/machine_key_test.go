package link

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"testing"
)

type unusableKey struct {
	name  string
	bytes []byte
}

func unusableKeys(t *testing.T) []unusableKey {
	t.Helper()
	decode := func(text string) []byte {
		key, err := hex.DecodeString(text)
		if err != nil || len(key) != ed25519.PublicKeySize {
			t.Fatal("A fixed public-key fixture has the wrong encoding.")
		}
		return key
	}
	return []unusableKey{
		{"not on the curve", decode("0200000000000000000000000000000000000000000000000000000000000000")},
		{"identity", decode("0100000000000000000000000000000000000000000000000000000000000000")},
		{"noncanonical field element", decode("eeffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f")},
		{"noncanonical sign of zero", decode("0100000000000000000000000000000000000000000000000000000000000080")},
		{"order two", decode("ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f")},
		{"order four", make([]byte, 32)},
		// Adding the order-two point (0, -1) to the standard base point (x, y)
		// gives (-x, -y). This is canonical and not in the prime-order subgroup.
		{"mixed order", decode("9599999999999999999999999999999999999999999999999999999999999999")},
	}
}

func TestMachinePublicKeyValidation(t *testing.T) {
	for _, seed := range []byte{1, 2, 251} {
		key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
		if !validMachinePublicKey(key) {
			t.Error("A generated prime-order public key was refused.")
		}
	}
	for _, tc := range append(unusableKeys(t), unusableKey{"short", make([]byte, 31)}, unusableKey{"long", make([]byte, 33)}) {
		t.Run(tc.name, func(t *testing.T) {
			if validMachinePublicKey(tc.bytes) {
				t.Error("The unusable public-key encoding was accepted.")
			}
		})
	}
}
