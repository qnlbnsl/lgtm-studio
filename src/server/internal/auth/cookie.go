package auth

import "net/http"

// This file holds stubs of the test writer. The builder of item 5 replaces them.

// ReadCookie is a stub of the test writer.
func ReadCookie(r *http.Request, mode CookieMode) (string, bool) { return "", false }

// SessionCookie is a stub of the test writer.
func SessionCookie(token string, mode CookieMode) *http.Cookie { return &http.Cookie{} }

// ClearSessionCookie is a stub of the test writer.
func ClearSessionCookie(mode CookieMode) *http.Cookie { return &http.Cookie{} }
