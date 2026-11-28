package projects

import (
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxOriginBytes = 2048

// normalizeOrigin returns the comparison form of a credential-free network origin.
func normalizeOrigin(raw string) (string, bool) {
	if len(raw) > maxOriginBytes || !utf8.ValidString(raw) || strings.ContainsAny(raw, "\\%?#") ||
		strings.ContainsFunc(raw, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return "", false
	}

	scheme, rest, network := strings.Cut(raw, "://")
	var authority, originPath string
	if network {
		scheme = strings.ToLower(scheme)
		authority, originPath, _ = strings.Cut(rest, "/")
	} else {
		scheme = "ssh"
		authority, originPath, _ = strings.Cut(raw, ":/")
		// A single-letter shorthand host is ambiguous with a local drive path.
		if len(authority) == 1 {
			return "", false
		}
	}
	var defaultPort uint64
	switch scheme {
	case "http":
		defaultPort = 80
	case "https":
		defaultPort = 443
	case "ssh":
		defaultPort = 22
	case "git":
		defaultPort = 9418
	default:
		return "", false
	}
	if originPath == "" || strings.HasPrefix(originPath, "/") ||
		(network && scheme == "ssh" && strings.HasPrefix(originPath, "~")) {
		return "", false
	}
	for segment := range strings.SplitSeq(originPath, "/") {
		if segment == "." || segment == ".." {
			return "", false
		}
	}

	parsed, err := url.Parse(scheme + "://" + authority)
	if err != nil || parsed.User != nil || parsed.Path != "" {
		return "", false
	}
	host := parsed.Hostname()
	if strings.HasPrefix(authority, "[") {
		address, err := netip.ParseAddr(host)
		if err != nil || !address.Is6() {
			return "", false
		}
		host = "[" + host + "]"
	} else if !validOriginHost(host) {
		return "", false
	}

	port := parsed.Port()
	if port == "" {
		if authority != host {
			return "", false
		}
	} else {
		value, err := strconv.ParseUint(port, 10, 16)
		if !network || err != nil || value == 0 || authority != host+":"+port {
			return "", false
		}
		if value != defaultPort {
			host += ":" + strconv.FormatUint(value, 10)
		}
	}
	return scheme + "://" + strings.ToLower(host) + "/" + originPath, true
}

func validOriginHost(host string) bool {
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
