package auth

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

const cookieToken = "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8" // 43 characters

func TestCookieAttributes(t *testing.T) {
	modes := []struct {
		name     string
		mode     CookieMode
		wantName string
		secure   bool
		sameSite http.SameSite
		site     string // a URL on which a browser would store the cookie
	}{
		{"plaintext loopback", plainMode, "lgtm_session", false, http.SameSiteStrictMode, "http://127.0.0.1:18787/"},
		{"https same site", secureMode, "__Host-lgtm_session", true, http.SameSiteStrictMode, "https://app.example.test/"},
		{"https cross site", crossSiteMode, "__Host-lgtm_cross_site_session", true, http.SameSiteNoneMode, "https://app.example.test/"},
	}
	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			if m.mode.Name != m.wantName || m.mode.Secure != m.secure || m.mode.SameSite != m.sameSite {
				t.Fatalf("the fixture mode %+v does not match the specified names", m.mode)
			}
			issued := SessionCookie(cookieToken, m.mode)
			if issued == nil {
				t.Fatalf("SessionCookie returned no cookie")
			}
			header := issued.String()
			parsed, err := http.ParseSetCookie(header)
			if err != nil {
				t.Fatalf("the cookie %q does not parse: %v", header, err)
			}
			if parsed.Name != m.wantName || parsed.Value != cookieToken {
				t.Errorf("name and value = %q=%q, want %q=%q", parsed.Name, parsed.Value, m.wantName, cookieToken)
			}
			if parsed.Path != "/" {
				t.Errorf("Path = %q, want /", parsed.Path)
			}
			if parsed.Domain != "" || strings.Contains(strings.ToLower(header), "domain") {
				t.Errorf("the cookie %q sets a Domain, so it is not host-only", header)
			}
			if !parsed.HttpOnly {
				t.Errorf("the cookie %q is readable by page scripts", header)
			}
			if parsed.MaxAge != 30*24*60*60 || !strings.Contains(header, "Max-Age=2592000") {
				t.Errorf("Max-Age = %d in %q, want 2592000", parsed.MaxAge, header)
			}
			if parsed.SameSite != m.sameSite {
				t.Errorf("SameSite = %v in %q, want %v", parsed.SameSite, header, m.sameSite)
			}
			if parsed.Secure != m.secure {
				t.Errorf("Secure = %v in %q, want %v", parsed.Secure, header, m.secure)
			}
			if parsed.Partitioned || strings.Contains(strings.ToLower(header), "partitioned") {
				t.Errorf("the cookie %q is partitioned", header)
			}

			cleared := ClearSessionCookie(m.mode)
			if cleared == nil {
				t.Fatalf("ClearSessionCookie returned no cookie")
			}
			if cleared.MaxAge >= 0 {
				t.Errorf("the deletion cookie has Go MaxAge %d, want a negative value", cleared.MaxAge)
			}
			wire := cleared.String()
			if !strings.Contains(wire, "Max-Age=0") {
				t.Errorf("the deletion cookie %q has no Max-Age=0", wire)
			}
			gone, err := http.ParseSetCookie(wire)
			if err != nil {
				t.Fatalf("the deletion cookie %q does not parse: %v", wire, err)
			}
			if gone.Name != m.wantName || gone.Value != "" {
				t.Errorf("deletion name and value = %q=%q, want %q with an empty value", gone.Name, gone.Value, m.wantName)
			}
			if !gone.Expires.Equal(time.Unix(0, 0)) {
				t.Errorf("deletion Expires = %v, want the Unix epoch", gone.Expires)
			}
			if gone.Path != "/" || gone.Domain != "" || !gone.HttpOnly || gone.Secure != m.secure || gone.SameSite != m.sameSite {
				t.Errorf("the deletion cookie %q does not carry the attributes of the issued cookie %q", wire, header)
			}
			if strings.Contains(strings.ToLower(wire), "domain") || strings.Contains(strings.ToLower(wire), "partitioned") {
				t.Errorf("the deletion cookie %q has a Domain or Partitioned attribute", wire)
			}

			// A browser-style jar stores the cookie and then removes the same host-only entry.
			jar, err := cookiejar.New(nil)
			if err != nil {
				t.Fatal(err)
			}
			site, _ := url.Parse(m.site)
			set := func(c *http.Cookie) []*http.Cookie {
				rec := httptest.NewRecorder()
				http.SetCookie(rec, c)
				return rec.Result().Cookies()
			}
			jar.SetCookies(site, set(issued))
			stored := jar.Cookies(site)
			if len(stored) != 1 || stored[0].Name != m.wantName || stored[0].Value != cookieToken {
				t.Fatalf("the jar holds %v after the sign-in cookie, want one %s cookie", stored, m.wantName)
			}
			jar.SetCookies(site, set(cleared))
			if left := jar.Cookies(site); len(left) != 0 {
				t.Errorf("the jar still holds %v after the deletion cookie", left)
			}
		})
	}

	t.Run("emits only the named cookie", func(t *testing.T) {
		if a, b := SessionCookie(cookieToken, secureMode), SessionCookie(cookieToken, crossSiteMode); a == nil || b == nil || a.Name == b.Name {
			t.Errorf("the two HTTPS modes share one cookie name")
		}
	})
}

func cookieRequest(lines ...string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/lgtm.v1.SessionService/GetServer", nil)
	for _, line := range lines {
		r.Header.Add("Cookie", line)
	}
	return r
}

func TestReadsOneCanonicalCookie(t *testing.T) {
	const other = "BBECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8"
	cases := []struct {
		name  string
		lines []string
		mode  CookieMode
		want  string
		ok    bool
	}{
		{"single", []string{"lgtm_session=" + cookieToken}, plainMode, cookieToken, true},
		{"single among others", []string{"a=1; lgtm_session=" + cookieToken + "; b=2"}, plainMode, cookieToken, true},
		{"single on the second line", []string{"a=1", "lgtm_session=" + cookieToken}, plainMode, cookieToken, true},
		{"single secure name", []string{"__Host-lgtm_session=" + cookieToken}, secureMode, cookieToken, true},
		{"single cross-site name", []string{"__Host-lgtm_cross_site_session=" + cookieToken}, crossSiteMode, cookieToken, true},

		{"absent", nil, plainMode, "", false},
		{"only other cookies", []string{"a=1; b=2"}, plainMode, "", false},
		{"empty value", []string{"lgtm_session="}, plainMode, "", false},
		{"similar names", []string{"xlgtm_session=" + cookieToken + "; lgtm_session2=" + cookieToken + "; LGTM_SESSION=" + cookieToken}, plainMode, "", false},
		{"suffix of the secure name", []string{"__Host-lgtm_session=" + cookieToken}, plainMode, "", false},

		{"duplicate on one line", []string{"lgtm_session=" + cookieToken + "; lgtm_session=" + cookieToken}, plainMode, "", false},
		{"duplicate with different values", []string{"lgtm_session=" + cookieToken + "; lgtm_session=" + other}, plainMode, "", false},
		{"duplicate on two lines", []string{"lgtm_session=" + cookieToken, "lgtm_session=" + other}, plainMode, "", false},
		{"duplicate on two lines, identical", []string{"lgtm_session=" + cookieToken, "a=1; lgtm_session=" + cookieToken}, plainMode, "", false},
		{"duplicate secure name", []string{"__Host-lgtm_session=" + cookieToken, "__Host-lgtm_session=" + other}, secureMode, "", false},
		{"duplicate cross-site name", []string{"__Host-lgtm_cross_site_session=" + cookieToken + "; __Host-lgtm_cross_site_session=" + other}, crossSiteMode, "", false},

		{"malformed value", []string{"lgtm_session=bad\x01value"}, plainMode, "", false},
		{"malformed value with a DEL byte", []string{"lgtm_session=bad\x7fvalue"}, plainMode, "", false},
		{"malformed then valid on one line", []string{"lgtm_session=bad\x01value; lgtm_session=" + cookieToken}, plainMode, "", false},
		{"malformed then valid on two lines", []string{"lgtm_session=bad\x01value", "lgtm_session=" + cookieToken}, plainMode, "", false},
		{"valid then malformed on two lines", []string{"lgtm_session=" + cookieToken, "lgtm_session=bad\x01value"}, plainMode, "", false},
		{"valid then malformed on the same mode name", []string{"__Host-lgtm_session=" + cookieToken, "__Host-lgtm_session=bad\x01value"}, secureMode, "", false},

		// Other mode names do not matter, whether they are single or duplicated.
		{"other names present", []string{"lgtm_session=" + cookieToken + "; __Host-lgtm_session=" + other + "; __Host-lgtm_cross_site_session=" + other}, plainMode, cookieToken, true},
		{"other names duplicated", []string{"__Host-lgtm_session=" + cookieToken, "__Host-lgtm_session=" + other, "lgtm_session=" + cookieToken}, plainMode, cookieToken, true},
		{"other name malformed", []string{"__Host-lgtm_session=bad\x01value", "__Host-lgtm_cross_site_session=" + cookieToken}, crossSiteMode, cookieToken, true},
		{"cross-site only, secure mode asked", []string{"__Host-lgtm_cross_site_session=" + cookieToken}, secureMode, "", false},
		{"secure only, cross-site mode asked", []string{"__Host-lgtm_session=" + cookieToken}, crossSiteMode, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ReadCookie(cookieRequest(c.lines...), c.mode)
			if ok != c.ok || got != c.want {
				t.Errorf("ReadCookie = %q, %v, want %q, %v", got, ok, c.want, c.ok)
			}
		})
	}

	t.Run("cross-site cookie on an Origin-less native call", func(t *testing.T) {
		policy := mustPolicy(t, "http://127.0.0.1:8787", nil)
		r := cookieRequest("__Host-lgtm_cross_site_session=" + cookieToken)
		mode, err := policy.Check(r)
		if err != nil {
			t.Fatalf("an Origin-less native call was refused: %v", err)
		}
		if token, ok := ReadCookie(r, mode); ok {
			t.Errorf("the native call read the cross-site cookie %q", token)
		}

		https := mustPolicy(t, "https://app.example.test", []string{"https://studio.example.org"})
		r = cookieRequest("__Host-lgtm_cross_site_session=" + cookieToken)
		mode, err = https.Check(r)
		if err != nil {
			t.Fatalf("an Origin-less HTTPS call was refused: %v", err)
		}
		if token, ok := ReadCookie(r, mode); ok {
			t.Errorf("the Origin-less call read the cross-site cookie %q", token)
		}
		r = cookieRequest("__Host-lgtm_cross_site_session=" + cookieToken)
		r.Header.Set("Origin", "https://studio.example.org")
		mode, err = https.Check(r)
		if err != nil {
			t.Fatalf("the listed origin was refused: %v", err)
		}
		if token, ok := ReadCookie(r, mode); !ok || token != cookieToken {
			t.Errorf("the explicit cross-site origin read %q, %v, want the cookie", token, ok)
		}
	})
}
