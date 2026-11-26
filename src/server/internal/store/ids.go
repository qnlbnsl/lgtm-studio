package store

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"time"
)

// NewID returns a version 7 universally unique identifier.
func NewID() (string, error) {
	return uuidV7(time.Now(), rand.Reader)
}

func uuidV7(at time.Time, random io.Reader) (string, error) {
	millis := at.UnixMilli()
	if millis < 0 || millis >= 1<<48 {
		return "", errors.New("The identifier timestamp is out of range.")
	}
	var id [16]byte
	for i := range 6 {
		id[i] = byte(millis >> (8 * (5 - i)))
	}
	if _, err := io.ReadFull(random, id[6:]); err != nil {
		return "", err
	}
	id[6] = 0x70 | id[6]&0x0f
	id[8] = 0x80 | id[8]&0x3f
	return fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:]), nil
}
