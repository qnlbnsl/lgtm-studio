package main

import (
	"strings"
	"testing"
)

func TestReadJoinTokenAcceptsFullLengthWithFinalCarriageReturn(t *testing.T) {
	const prefix = "fixture only, not a valid join token: "
	token := prefix + strings.Repeat("x", 4096-len(prefix))
	if readJoinToken(strings.NewReader(token+"\r")) != token {
		t.Fatal("The join token reader did not accept a full-length token with a final carriage return.")
	}
}
