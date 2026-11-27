package main

import (
	"strings"
	"testing"
)

func TestReadJoinTokenTrimsCarriageReturnAtEndOfInput(t *testing.T) {
	const token = "fixture only, not a valid join token"
	for _, tc := range []struct {
		name, ending string
	}{
		{name: "carriage return at end of input", ending: "\r"},
		{name: "carriage return and line feed", ending: "\r\n"},
		{name: "no line ending"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if readJoinToken(strings.NewReader(token+tc.ending)) != token {
				t.Error("The join token reader did not return the token without its line ending.")
			}
		})
	}
}
