package link

import (
	"bytes"

	"filippo.io/edwards25519"
)

// inverseCofactor is the inverse of eight modulo the Ed25519 prime subgroup order.
var inverseCofactor = func() *edwards25519.Scalar {
	s, err := edwards25519.NewScalar().SetCanonicalBytes([]byte{
		0x79, 0x2f, 0xdc, 0xe2, 0x29, 0xe5, 0x06, 0x61,
		0xd0, 0xda, 0x1c, 0x7d, 0xb3, 0x9d, 0xd3, 0x07,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x06,
	})
	if err != nil {
		panic(err)
	}
	return s
}()

func validMachinePublicKey(key []byte) bool {
	point, err := new(edwards25519.Point).SetBytes(key)
	if err != nil || !bytes.Equal(point.Bytes(), key) || point.Equal(edwards25519.NewIdentityPoint()) == 1 {
		return false
	}
	cleared := new(edwards25519.Point).MultByCofactor(point)
	return cleared.ScalarMult(inverseCofactor, cleared).Equal(point) == 1
}
