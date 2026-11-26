package auth

import "net/http"

// This file holds stubs of the test writer. The builder of item 5 replaces them.

const CookieName = "lgtm_session"
const SecureCookieName = "__Host-lgtm_session"
const CrossSiteCookieName = "__Host-lgtm_cross_site_session"

// OriginPolicy is a stub of the test writer.
type OriginPolicy struct{}

// CookieMode is a stub of the test writer.
type CookieMode struct {
	Name     string
	Secure   bool
	SameSite http.SameSite
}

// NewOriginPolicy is a stub of the test writer.
func NewOriginPolicy(publicURL string, allowed []string) (*OriginPolicy, error) {
	return &OriginPolicy{}, nil
}

// Check is a stub of the test writer.
func (p *OriginPolicy) Check(r *http.Request) (CookieMode, error) { return CookieMode{}, nil }
