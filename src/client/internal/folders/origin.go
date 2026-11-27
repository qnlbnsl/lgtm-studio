package folders

import (
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxOriginBytes = 2048

// sanitizeOrigin preserves the address spelling after removing user information.
func sanitizeOrigin(raw string) string {
	if !utf8.ValidString(raw) || strings.ContainsAny(raw, "\\?#") ||
		strings.ContainsFunc(raw, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return ""
	}
	var address string
	if scheme, rest, ok := strings.Cut(raw, "://"); ok {
		switch strings.ToLower(scheme) {
		case "http", "https", "git", "ssh":
		default:
			return ""
		}
		authority, path, _ := strings.Cut(rest, "/")
		host := withoutOriginUser(authority)
		address = scheme + "://" + host + "/" + path
		parsed, err := url.Parse(address)
		if err != nil || !validOriginHost(parsed.Hostname()) || !validOriginAuthority(host, parsed.Hostname()) ||
			strings.HasSuffix(host, ":") || !validOriginPort(parsed.Port()) ||
			!validOriginPath(path, strings.EqualFold(scheme, "ssh")) {
			return ""
		}
	} else {
		authority, path, ok := strings.Cut(raw, ":/")
		if user, _, hasUser := strings.Cut(authority, "@"); hasUser && strings.ContainsAny(user, ":/[]") {
			return ""
		}
		host := withoutOriginUser(authority)
		name := strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
		if !ok || len(authority) == 1 || !validOriginHost(name) || !validOriginAuthority(host, name) ||
			!validOriginPath(path, false) {
			return ""
		}
		address = host + ":/" + path
	}
	if len(address) > maxOriginBytes {
		return ""
	}
	return address
}

func withoutOriginUser(authority string) string {
	if user, host, present := strings.Cut(authority, "@"); present {
		if user == "" || strings.ContainsRune(host, '@') {
			return ""
		}
		return host
	}
	return authority
}

func validOriginAuthority(authority, host string) bool {
	if strings.ContainsRune(host, ':') {
		return strings.HasPrefix(authority, "[") && strings.Contains(authority, "]")
	}
	return !strings.ContainsAny(authority, "[]")
}

func validOriginHost(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	if host == "" || host[0] == '-' || host[0] == '.' || strings.Contains(host, "..") {
		return false
	}
	for _, r := range host {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '.') {
			return false
		}
	}
	return true
}

func validOriginPort(port string) bool {
	if port == "" {
		return true
	}
	value, err := strconv.ParseUint(port, 10, 16)
	return err == nil && value != 0
}

func validOriginPath(path string, sshURL bool) bool {
	if strings.Trim(path, "/") == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "%") ||
		(sshURL && strings.HasPrefix(path, "~")) {
		return false
	}
	for part := range strings.SplitSeq(path, "/") {
		if part == "." || part == ".." {
			return false
		}
	}
	return true
}
