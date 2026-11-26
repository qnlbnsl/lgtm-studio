package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/net/publicsuffix"
)

var (
	plainMode      = CookieMode{Name: CookieName, Secure: false, SameSite: http.SameSiteStrictMode}
	secureMode     = CookieMode{Name: SecureCookieName, Secure: true, SameSite: http.SameSiteStrictMode}
	crossSiteMode  = CookieMode{Name: CrossSiteCookieName, Secure: true, SameSite: http.SameSiteNoneMode}
	noOriginHeader = "\x00absent"
)

// originRequest builds a request. An origin equal to noOriginHeader leaves the header out.
func originRequest(origin string, extra ...string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/lgtm.v1.SessionService/SignIn", nil)
	if origin != noOriginHeader {
		r.Header["Origin"] = []string{origin}
	}
	for i := 0; i+1 < len(extra); i += 2 {
		r.Header.Add(extra[i], extra[i+1])
	}
	return r
}

func mustPolicy(t *testing.T, public string, allowed []string) *OriginPolicy {
	t.Helper()
	p, err := NewOriginPolicy(public, allowed)
	if err != nil {
		t.Fatalf("NewOriginPolicy(%q, %q) failed: %v", public, allowed, err)
	}
	if p == nil {
		t.Fatalf("NewOriginPolicy(%q, %q) returned no policy", public, allowed)
	}
	return p
}

type originCase struct {
	name   string
	origin string
	extra  []string
	mode   CookieMode // used when refused is false
	refuse bool
}

func runOriginCases(t *testing.T, p *OriginPolicy, cases []originCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mode, err := p.Check(originRequest(c.origin, c.extra...))
			if c.refuse {
				if err == nil {
					t.Fatalf("Origin %q was admitted with mode %+v", c.origin, mode)
				}
				if mode != (CookieMode{}) {
					t.Errorf("a refused request still carries mode %+v", mode)
				}
				return
			}
			if err != nil {
				t.Fatalf("Origin %q was refused: %v", c.origin, err)
			}
			if mode != c.mode {
				t.Errorf("Origin %q selected mode %+v, want %+v", c.origin, mode, c.mode)
			}
		})
	}
}

func TestOriginPolicies(t *testing.T) {
	// The tenants test depends on the private section of the public suffix list.
	if got, err := publicsuffix.EffectiveTLDPlusOne("a.github.io"); err != nil || got != "a.github.io" {
		t.Fatalf("the public suffix list does not treat github.io as a suffix: %q, %v", got, err)
	}

	t.Run("https public origin", func(t *testing.T) {
		p := mustPolicy(t, "https://app.example.test", []string{
			"https://api.example.test:8443", // same site, different subdomain and port
			"https://studio.example.org/",   // unrelated site, optional root slash
			"https://TENANT-B.github.io",    // upper case host is canonicalized
			"https://other.example.net:443", // default port is dropped
		})
		runOriginCases(t, p, []originCase{
			{name: "public origin", origin: "https://app.example.test", mode: secureMode},
			{name: "public origin upper case", origin: "HTTPS://APP.Example.Test", mode: secureMode},
			{name: "public origin default port", origin: "https://app.example.test:443", mode: secureMode},
			{name: "listed same site other port", origin: "https://api.example.test:8443", mode: secureMode},
			{name: "listed unrelated site", origin: "https://studio.example.org", mode: crossSiteMode},
			{name: "listed unrelated site with port", origin: "https://other.example.net", mode: crossSiteMode},
			{name: "listed tenant", origin: "https://tenant-b.github.io", mode: crossSiteMode},
			{name: "unlisted same site", origin: "https://www.example.test", refuse: true},
			{name: "listed host at unlisted port", origin: "https://api.example.test", refuse: true},
			{name: "listed host at other port", origin: "https://api.example.test:9443", refuse: true},
			{name: "unlisted cross site", origin: "https://evil.example.org", refuse: true},
			{name: "plaintext twin of public origin", origin: "http://app.example.test", refuse: true},
			{name: "plaintext twin of listed origin", origin: "http://studio.example.org", refuse: true},
		})
	})

	t.Run("two tenants under a private suffix", func(t *testing.T) {
		p := mustPolicy(t, "https://a.github.io", []string{"https://b.github.io", "https://x.a.github.io"})
		runOriginCases(t, p, []originCase{
			{name: "own tenant", origin: "https://a.github.io", mode: secureMode},
			{name: "listed other tenant is cross site", origin: "https://b.github.io", mode: crossSiteMode},
			{name: "listed subdomain of own tenant is same site", origin: "https://x.a.github.io", mode: secureMode},
			{name: "unlisted subdomain of own tenant", origin: "https://y.a.github.io", refuse: true},
			{name: "unlisted third tenant", origin: "https://c.github.io", refuse: true},
		})
		// Same host list, other tenants unlisted: same-site admission would let both through.
		q := mustPolicy(t, "https://a.github.io", nil)
		runOriginCases(t, q, []originCase{
			{name: "other tenant unlisted", origin: "https://b.github.io", refuse: true},
		})
	})

	t.Run("plaintext loopback aliases at the public port", func(t *testing.T) {
		p := mustPolicy(t, "http://127.0.0.1:18787", nil)
		runOriginCases(t, p, []originCase{
			{name: "public", origin: "http://127.0.0.1:18787", mode: plainMode},
			{name: "localhost alias", origin: "http://localhost:18787", mode: plainMode},
			{name: "localhost alias upper case", origin: "http://LOCALHOST:18787", mode: plainMode},
			{name: "ipv6 alias", origin: "http://[::1]:18787", mode: plainMode},
			{name: "public host at default port", origin: "http://127.0.0.1", refuse: true},
			{name: "alias at the default listener port", origin: "http://localhost:8787", refuse: true},
			{name: "public host at the default listener port", origin: "http://127.0.0.1:8787", refuse: true},
			{name: "ipv6 at the default listener port", origin: "http://[::1]:8787", refuse: true},
			{name: "https twin", origin: "https://127.0.0.1:18787", refuse: true},
			{name: "non-loopback host at the port", origin: "http://app.example.test:18787", refuse: true},
			{name: "name that only looks like loopback", origin: "http://localhost.example.test:18787", refuse: true},
		})
	})

	t.Run("plaintext public origin with a listed same-site port", func(t *testing.T) {
		p := mustPolicy(t, "http://localhost:18787", []string{"http://localhost:8787"})
		runOriginCases(t, p, []originCase{
			{name: "listed port", origin: "http://localhost:8787", mode: plainMode},
			{name: "alias at the public port", origin: "http://127.0.0.1:18787", mode: plainMode},
			{name: "alias at the listed port is not implied", origin: "http://127.0.0.1:8787", refuse: true},
		})
	})

	t.Run("plaintext public origin at the default port", func(t *testing.T) {
		p := mustPolicy(t, "http://localhost:80/", nil)
		runOriginCases(t, p, []originCase{
			{name: "no port", origin: "http://localhost", mode: plainMode},
			{name: "explicit default port", origin: "http://localhost:80", mode: plainMode},
			{name: "alias without port", origin: "http://127.0.0.1", mode: plainMode},
			{name: "ipv6 alias without port", origin: "http://[::1]", mode: plainMode},
			{name: "other port", origin: "http://localhost:8080", refuse: true},
		})
	})

	for _, public := range []string{"https://localhost:8443", "https://127.0.0.1:8443", "https://[::1]:8443"} {
		t.Run("https public origin refuses plaintext aliases "+public, func(t *testing.T) {
			p := mustPolicy(t, public, nil)
			cases := []originCase{{name: "public origin", origin: public, mode: secureMode}}
			for _, port := range []string{"8443", "8787", "18787", "80", ""} {
				suffix := ""
				if port != "" {
					suffix = ":" + port
				}
				for _, host := range []string{"127.0.0.1", "localhost", "[::1]"} {
					cases = append(cases, originCase{name: "plaintext " + host + suffix, origin: "http://" + host + suffix, refuse: true})
				}
			}
			runOriginCases(t, p, cases)
		})
	}

	t.Run("header shapes", func(t *testing.T) {
		p := mustPolicy(t, "https://app.example.test", []string{"https://studio.example.org"})
		runOriginCases(t, p, []originCase{
			{name: "trailing slash", origin: "https://app.example.test/", refuse: true},
			{name: "path", origin: "https://app.example.test/x", refuse: true},
			{name: "query", origin: "https://app.example.test?x=1", refuse: true},
			{name: "fragment", origin: "https://app.example.test#x", refuse: true},
			{name: "user information", origin: "https://user@app.example.test", refuse: true},
			{name: "opaque", origin: "null", refuse: true},
			{name: "empty", origin: "", refuse: true},
			{name: "trailing dot", origin: "https://app.example.test.", refuse: true},
			{name: "wildcard", origin: "https://*.example.test", refuse: true},
			{name: "malformed port", origin: "https://app.example.test:port", refuse: true},
			{name: "port out of range", origin: "https://app.example.test:99999", refuse: true},
			{name: "empty port", origin: "https://app.example.test:", refuse: true},
			{name: "non-ASCII host", origin: "https://äpp.example.test", refuse: true},
			{name: "no scheme", origin: "app.example.test", refuse: true},
			{name: "other scheme", origin: "ftp://app.example.test", refuse: true},
			{name: "leading space", origin: " https://app.example.test", refuse: true},
		})

		for _, values := range [][]string{
			{"https://app.example.test", "https://app.example.test"},
			{"https://app.example.test", "https://studio.example.org"},
			{"https://studio.example.org", "https://app.example.test"},
			{"https://app.example.test", ""},
			{"null", "https://app.example.test"},
		} {
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.Header["Origin"] = values
			if mode, err := p.Check(r); err == nil {
				t.Errorf("multiple Origin values %q were admitted with mode %+v", values, mode)
			}
		}
	})

	t.Run("absent origin", func(t *testing.T) {
		https := mustPolicy(t, "https://app.example.test", []string{"https://studio.example.org"})
		plain := mustPolicy(t, "http://127.0.0.1:8787", nil)
		for _, c := range []struct {
			name   string
			policy *OriginPolicy
			extra  []string
			mode   CookieMode
			refuse bool
		}{
			{name: "https without metadata", policy: https, mode: secureMode},
			{name: "plaintext without metadata", policy: plain, mode: plainMode},
			{name: "same-origin metadata", policy: https, extra: []string{"Sec-Fetch-Site", "same-origin"}, mode: secureMode},
			{name: "same-site metadata", policy: https, extra: []string{"Sec-Fetch-Site", "same-site"}, mode: secureMode},
			{name: "none metadata", policy: https, extra: []string{"Sec-Fetch-Site", "none"}, mode: secureMode},
			{name: "plaintext same-origin metadata", policy: plain, extra: []string{"Sec-Fetch-Site", "same-origin"}, mode: plainMode},
			{name: "cross-site metadata", policy: https, extra: []string{"Sec-Fetch-Site", "cross-site"}, refuse: true},
			{name: "cross-site metadata on plaintext", policy: plain, extra: []string{"Sec-Fetch-Site", "cross-site"}, refuse: true},
		} {
			t.Run(c.name, func(t *testing.T) {
				mode, err := c.policy.Check(originRequest(noOriginHeader, c.extra...))
				if c.refuse {
					if err == nil {
						t.Fatalf("the request was admitted with mode %+v", mode)
					}
					return
				}
				if err != nil {
					t.Fatalf("the request was refused: %v", err)
				}
				if mode != c.mode {
					t.Errorf("mode = %+v, want %+v", mode, c.mode)
				}
				if mode.Name == CrossSiteCookieName || mode.SameSite == http.SameSiteNoneMode {
					t.Errorf("a cross-site mode was selected without an explicit origin")
				}
			})
		}

		// Fetch metadata does not override an allowed Origin.
		mode, err := https.Check(originRequest("https://studio.example.org", "Sec-Fetch-Site", "cross-site"))
		if err != nil || mode != crossSiteMode {
			t.Errorf("a listed cross-site origin with cross-site metadata gave %+v, %v", mode, err)
		}
	})

	t.Run("forwarded headers are ignored", func(t *testing.T) {
		forged := []string{
			"X-Forwarded-Proto", "https",
			"X-Forwarded-Host", "app.example.test",
			"Forwarded", "proto=https;host=app.example.test",
		}
		plain := mustPolicy(t, "http://127.0.0.1:18787", nil)
		mode, err := plain.Check(originRequest(noOriginHeader, forged...))
		if err != nil || mode != plainMode {
			t.Errorf("a forged forwarded scheme changed the plaintext mode: %+v, %v", mode, err)
		}
		if _, err := plain.Check(originRequest("https://app.example.test", forged...)); err == nil {
			t.Errorf("a forwarded host admitted an unlisted origin")
		}

		https := mustPolicy(t, "https://app.example.test", []string{"https://studio.example.org"})
		r := originRequest("https://evil.example.org", forged...)
		r.Host = "evil.example.org"
		if mode, err := https.Check(r); err == nil {
			t.Errorf("an origin equal to the request host was admitted with mode %+v", mode)
		}
		r = originRequest("https://studio.example.org", "X-Forwarded-Proto", "http")
		r.Host = "internal.example.test"
		if mode, err := https.Check(r); err != nil || mode != crossSiteMode {
			t.Errorf("the host or a forwarded scheme changed a listed origin: %+v, %v", mode, err)
		}
		r = originRequest(noOriginHeader, "X-Forwarded-Proto", "http")
		r.Host = "other.example.test"
		if mode, err := https.Check(r); err != nil || mode != secureMode {
			t.Errorf("the host or a forwarded scheme changed the public mode: %+v, %v", mode, err)
		}
	})
}

func TestOriginPolicyConfiguration(t *testing.T) {
	valid := []struct {
		name    string
		public  string
		allowed []string
	}{
		{"https", "https://app.example.test", nil},
		{"https with root slash", "https://app.example.test/", nil},
		{"https default port", "https://app.example.test:443", nil},
		{"https upper case", "HTTPS://App.Example.Test", nil},
		{"https loopback", "https://localhost:8443", nil},
		{"plaintext localhost", "http://localhost:8787", nil},
		{"plaintext ipv4", "http://127.0.0.1:8787", nil},
		{"plaintext ipv6", "http://[::1]:8787", nil},
		{"both https", "https://app.example.test", []string{"https://studio.example.org", "https://api.example.test"}},
		{"plaintext with same-site plaintext entry", "http://127.0.0.1:8787", []string{"http://127.0.0.1:9000"}},
	}
	for _, c := range valid {
		t.Run("valid "+c.name, func(t *testing.T) {
			if p, err := NewOriginPolicy(c.public, c.allowed); err != nil || p == nil {
				t.Fatalf("NewOriginPolicy(%q, %q) = %v, %v", c.public, c.allowed, p, err)
			}
		})
	}

	badOrigins := []string{
		"", "null", "app.example.test", "//app.example.test", "ftp://app.example.test",
		"https://app.example.test/path", "https://app.example.test//", "https://app.example.test?x=1",
		"https://app.example.test#x", "https://user@app.example.test", "https://user:pass@app.example.test",
		"https://app.example.test.", "https://*.example.test", "https://app.example.test:",
		"https://app.example.test:port", "https://app.example.test:0", "https://app.example.test:65536",
		"https://äpp.example.test", "https://app .example.test", "https:///", "https://",
		"http://app.example.test",            // plaintext on a name that is not loopback
		"http://localhost.example.test:8787", // a name that only starts with localhost
		"http://192.0.2.10:8787",             // a documentation address, not loopback
	}
	for _, bad := range badOrigins {
		t.Run("bad public "+bad, func(t *testing.T) {
			if p, err := NewOriginPolicy(bad, nil); err == nil {
				t.Errorf("NewOriginPolicy(%q) was accepted: %+v", bad, p)
			}
		})
		if bad == "" {
			continue
		}
		t.Run("bad allowlist "+bad, func(t *testing.T) {
			if p, err := NewOriginPolicy("https://app.example.test", []string{bad}); err == nil {
				t.Errorf("an allowlist entry %q was accepted: %+v", bad, p)
			}
		})
	}
	t.Run("empty allowlist entry", func(t *testing.T) {
		if _, err := NewOriginPolicy("https://app.example.test", []string{"https://studio.example.org", ""}); err == nil {
			t.Errorf("an empty allowlist entry was accepted")
		}
	})

	// A cross-site entry needs both origins on HTTPS.
	crossSite := []struct {
		name    string
		public  string
		allowed string
	}{
		{"plaintext public with https entry", "http://127.0.0.1:8787", "https://studio.example.org"},
		{"plaintext public with other loopback host", "http://127.0.0.1:8787", "http://localhost:9000"},
		{"https public with plaintext loopback entry", "https://app.example.test", "http://127.0.0.1:9000"},
		{"https public with plaintext entry", "https://app.example.test", "http://studio.example.org"},
		{"https loopback public with plaintext loopback entry", "https://localhost:8443", "http://localhost:9000"},
	}
	for _, c := range crossSite {
		t.Run("cross-site "+c.name, func(t *testing.T) {
			if p, err := NewOriginPolicy(c.public, []string{c.allowed}); err == nil {
				t.Errorf("NewOriginPolicy(%q, [%q]) was accepted: %+v", c.public, c.allowed, p)
			}
		})
	}
}
