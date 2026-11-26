package auth

import (
	"net/http/httptest"
	"testing"
)

// This test records a present choice of the code. Issue 1651 keeps open whether
// a quoted cookie value should be refused instead.
func TestPresentChoiceQuotedCookieValueIsReadWithoutItsQuotes(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Cookie", CookieName+`="`+cookieToken+`"`)
	got, ok := ReadCookie(r, plainMode)
	if !ok || got != cookieToken {
		t.Fatalf("ReadCookie = %q, %v, want the token without its quotes", got, ok)
	}
}
