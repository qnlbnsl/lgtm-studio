package auth

import (
	"net/http"
	"strings"
	"time"
)

const sessionCookieMaxAge = 30 * 24 * 60 * 60

// ReadCookie returns the value of exactly one well-formed cookie in the selected mode.
func ReadCookie(r *http.Request, mode CookieMode) (string, bool) {
	if mode.Name == "" {
		return "", false
	}
	var token string
	for _, line := range r.Header.Values("Cookie") {
		for pair := range strings.SplitSeq(line, ";") {
			pair = strings.TrimLeft(pair, " \t")
			name, raw, found := strings.Cut(pair, "=")
			if strings.TrimRight(name, " \t") != mode.Name {
				continue
			}
			if token != "" || !found || name != mode.Name || strings.TrimRight(raw, " \t") != raw {
				return "", false
			}
			cookies, err := http.ParseCookie(pair)
			if err != nil || len(cookies) != 1 || cookies[0].Value == "" || strings.ContainsAny(cookies[0].Value, " ,") {
				return "", false
			}
			token = cookies[0].Value
		}
	}
	return token, token != ""
}

// SessionCookie builds the host-only cookie that carries a new session.
func SessionCookie(token string, mode CookieMode) *http.Cookie {
	return &http.Cookie{
		Name:     mode.Name,
		Value:    token,
		Path:     "/",
		MaxAge:   sessionCookieMaxAge,
		HttpOnly: true,
		Secure:   mode.Secure,
		SameSite: mode.SameSite,
	}
}

// ClearSessionCookie expires the selected host-only cookie.
func ClearSessionCookie(mode CookieMode) *http.Cookie {
	cookie := SessionCookie("", mode)
	cookie.MaxAge = -1
	cookie.Expires = time.Unix(0, 0).UTC()
	return cookie
}
