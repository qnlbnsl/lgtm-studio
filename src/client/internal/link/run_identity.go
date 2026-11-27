package link

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"time"
)

// runIdentity is the immutable process announcement sent on each stream.
type runIdentity struct {
	id               string
	firstEventNumber uint64
}

func newRunIdentity(firstEventNumber uint64) runIdentity {
	var id [16]byte
	millis := uint64(time.Now().UnixMilli())
	binary.BigEndian.PutUint32(id[:4], uint32(millis>>16))
	binary.BigEndian.PutUint16(id[4:6], uint16(millis))
	rand.Read(id[6:])
	id[6] = 0x70 | id[6]&0x0f
	id[8] = 0x80 | id[8]&0x3f
	return runIdentity{
		id:               fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:]),
		firstEventNumber: firstEventNumber,
	}
}
