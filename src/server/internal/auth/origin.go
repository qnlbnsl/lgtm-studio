package auth

import (
	"errors"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/publicsuffix"
)

const (
	CookieName          = "lgtm_session"
	SecureCookieName    = "__Host-lgtm_session"
	CrossSiteCookieName = "__Host-lgtm_cross_site_session"
)

// CookieMode selects the session cookie for an admitted request.
type CookieMode struct {
	Name     string
	Secure   bool
	SameSite http.SameSite
}

// OriginPolicy admits the configured origins and selects their cookie modes.
type OriginPolicy struct {
	public     browserOrigin
	publicMode CookieMode
	allowed    map[browserOrigin]CookieMode
}

type browserOrigin struct {
	scheme string
	host   string
	port   uint16
}

// NewOriginPolicy validates the public origin and all additional origins.
func NewOriginPolicy(publicURL string, allowed []string) (*OriginPolicy, error) {
	public, ok := parseOrigin(publicURL, true)
	if !ok {
		return nil, errors.New("LGTM_PUBLIC_URL must be an HTTPS or plaintext loopback origin")
	}
	mode := CookieMode{Name: SecureCookieName, Secure: true, SameSite: http.SameSiteStrictMode}
	if public.scheme == "http" {
		mode = CookieMode{Name: CookieName, SameSite: http.SameSiteStrictMode}
	}

	policy := &OriginPolicy{
		public:     public,
		publicMode: mode,
		allowed:    make(map[browserOrigin]CookieMode, len(allowed)),
	}
	if public.scheme == "http" {
		for _, host := range []string{"127.0.0.1", "localhost", "::1"} {
			alias := browserOrigin{scheme: public.scheme, host: host, port: public.port}
			policy.allowed[alias] = mode
		}
	}
	for _, value := range allowed {
		origin, ok := parseOrigin(value, true)
		if !ok {
			return nil, errors.New("LGTM_ALLOWED_ORIGINS contains an invalid origin")
		}
		if _, present := policy.allowed[origin]; present {
			continue
		}
		if public.sameSite(origin) {
			policy.allowed[origin] = mode
			continue
		}
		if public.scheme != "https" || origin.scheme != "https" {
			return nil, errors.New("LGTM_ALLOWED_ORIGINS requires HTTPS on both sides of a cross-site request")
		}
		policy.allowed[origin] = CookieMode{Name: CrossSiteCookieName, Secure: true, SameSite: http.SameSiteNoneMode}
	}
	return policy, nil
}

var errOriginRefused = errors.New("this website origin is not allowed")

// Check selects the cookie mode from the configured policy and the request origin.
func (p *OriginPolicy) Check(r *http.Request) (CookieMode, error) {
	if p == nil || p.publicMode.Name == "" {
		return CookieMode{}, errOriginRefused
	}
	origins, hasOrigin := r.Header["Origin"]
	if !hasOrigin {
		for _, field := range r.Header.Values("Sec-Fetch-Site") {
			for value := range strings.SplitSeq(field, ",") {
				if strings.EqualFold(strings.TrimSpace(value), "cross-site") {
					return CookieMode{}, errOriginRefused
				}
			}
		}
		return p.publicMode, nil
	}
	if len(origins) != 1 {
		return CookieMode{}, errOriginRefused
	}
	origin, ok := parseOrigin(origins[0], false)
	if !ok {
		return CookieMode{}, errOriginRefused
	}
	if origin == p.public {
		return p.publicMode, nil
	}
	mode, allowed := p.allowed[origin]
	if !allowed {
		return CookieMode{}, errOriginRefused
	}
	return mode, nil
}

func parseOrigin(raw string, allowRootSlash bool) (browserOrigin, bool) {
	if strings.ContainsAny(raw, "?#%") {
		return browserOrigin{}, false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil ||
		(u.Scheme != "http" && u.Scheme != "https") ||
		(u.Path != "" && !(allowRootSlash && u.Path == "/")) ||
		strings.HasSuffix(u.Host, ":") {
		return browserOrigin{}, false
	}

	host := u.Hostname()
	ip, ipErr := netip.ParseAddr(host)
	if strings.HasPrefix(u.Host, "[") {
		if ipErr != nil || !ip.Is6() || ip.Zone() != "" {
			return browserOrigin{}, false
		}
		host = ip.String()
	} else {
		if !validOriginHost(host) {
			return browserOrigin{}, false
		}
		host = strings.ToLower(host)
	}
	if u.Scheme == "http" && host != "localhost" && !ip.Unmap().IsLoopback() {
		return browserOrigin{}, false
	}

	var port uint16
	if text := u.Port(); text != "" {
		number, err := strconv.ParseUint(text, 10, 16)
		if err != nil || number == 0 {
			return browserOrigin{}, false
		}
		if !(u.Scheme == "http" && number == 80 || u.Scheme == "https" && number == 443) {
			port = uint16(number)
		}
	}
	return browserOrigin{scheme: u.Scheme, host: host, port: port}, true
}

func validOriginHost(host string) bool {
	if host == "" || host[0] == '.' || host[len(host)-1] == '.' || strings.Contains(host, "..") {
		return false
	}
	for _, c := range host {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '.', c == '_':
		default:
			return false
		}
	}
	return true
}

func (o browserOrigin) sameSite(other browserOrigin) bool {
	return o.scheme == other.scheme && siteHost(o.host) == siteHost(other.host)
}

func siteHost(host string) string {
	if _, err := netip.ParseAddr(host); err == nil {
		return host
	}
	domain, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return host
	}
	return domain
}
