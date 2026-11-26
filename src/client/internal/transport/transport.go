// Package transport validates server origins and authenticates the client connection.
package transport

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ErrCertificateKeyMismatch reports that the server's leaf key differs from the pin.
var ErrCertificateKeyMismatch = errors.New("transport: certificate key mismatch")

var errInvalidOrigin = errors.New("transport: invalid server origin")

// ParseOrigin returns the canonical origin allowed by the client transport.
func ParseOrigin(raw string) (string, error) {
	origin, err := parseOrigin(raw)
	if err != nil {
		return "", err
	}
	return origin.String(), nil
}

func parseOrigin(raw string) (*url.URL, error) {
	if strings.ContainsAny(raw, "?#%") || strings.IndexFunc(raw, unicode.IsSpace) >= 0 {
		return nil, errInvalidOrigin
	}
	origin, err := url.Parse(raw)
	if err != nil || origin.Host == "" || origin.User != nil || origin.Path != "" ||
		strings.HasSuffix(origin.Host, ":") {
		return nil, errInvalidOrigin
	}

	var defaultPort uint64
	switch origin.Scheme {
	case "http":
		defaultPort = 80
	case "https":
		defaultPort = 443
	default:
		return nil, errInvalidOrigin
	}

	host := origin.Hostname()
	ip, ipErr := netip.ParseAddr(host)
	if strings.HasPrefix(origin.Host, "[") {
		if ipErr != nil || !ip.Is6() {
			return nil, errInvalidOrigin
		}
		host = ip.String()
	} else if !validHostName(host) {
		return nil, errInvalidOrigin
	} else {
		host = strings.ToLower(host)
	}
	if origin.Scheme == "http" && host != "localhost" && !ip.Unmap().IsLoopback() {
		return nil, errors.New("transport: plain HTTP requires a loopback origin")
	}

	if port := origin.Port(); port != "" {
		number, err := strconv.ParseUint(port, 10, 16)
		if err != nil || number == 0 {
			return nil, errInvalidOrigin
		}
		if number != defaultPort {
			origin.Host = net.JoinHostPort(host, strconv.FormatUint(number, 10))
			return origin, nil
		}
	}
	origin.Host = host
	if ip.Is6() {
		origin.Host = "[" + host + "]"
	}
	return origin, nil
}

func validHostName(host string) bool {
	name := strings.TrimSuffix(host, ".")
	if name == "" || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") || strings.Contains(name, "..") {
		return false
	}
	for _, c := range name {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '.', c == '_':
		default:
			return false
		}
	}
	return true
}

// NewHTTPClient returns a client for the given origin. The caller closes its idle
// connections when it finishes with the client.
func NewHTTPClient(origin string, certificateKeySHA256 []byte) (*http.Client, error) {
	parsed, err := parseOrigin(origin)
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{Timeout: 30 * time.Second}
	transport := &http.Transport{
		DialContext:         dialer.DialContext,
		Protocols:           new(http.Protocols),
		TLSHandshakeTimeout: 10 * time.Second,
		IdleConnTimeout:     90 * time.Second,
	}
	if parsed.Scheme == "http" {
		if len(certificateKeySHA256) != 0 {
			return nil, errors.New("transport: plain HTTP requires an empty certificate pin")
		}
		transport.Protocols.SetUnencryptedHTTP2(true)
		transport.DialContext = loopbackDialer(dialer)
	} else {
		if len(certificateKeySHA256) != sha256.Size {
			return nil, errors.New("transport: HTTPS requires a 32-byte certificate key pin")
		}
		transport.Protocols.SetHTTP2(true)
		host, pin := parsed.Hostname(), [sha256.Size]byte(certificateKeySHA256)
		transport.TLSClientConfig = &tls.Config{
			ServerName: host,
			// The pinned leaf key is the trust anchor. The callback checks it and the host.
			InsecureSkipVerify: true,
			VerifyConnection: func(connection tls.ConnectionState) error {
				return verifyLeaf(connection, host, pin)
			},
		}
	}
	return &http.Client{
		Transport: &originTransport{transport: transport, origin: parsed.String()},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

type originTransport struct {
	transport *http.Transport
	origin    string
}

func (t *originTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL != nil && req.URL.Opaque == "" && req.URL.User == nil &&
		t.matches(req.URL.Scheme, req.URL.Host) &&
		(req.Host == "" || req.Host == req.URL.Host || t.matches(req.URL.Scheme, req.Host)) {
		return t.transport.RoundTrip(req)
	}
	if req.Body != nil {
		_ = req.Body.Close()
	}
	return nil, errors.New("transport: request must use the client's server origin")
}

func (t *originTransport) matches(scheme, host string) bool {
	origin, err := ParseOrigin(scheme + "://" + host)
	return err == nil && origin == t.origin
}

func (t *originTransport) CloseIdleConnections() {
	t.transport.CloseIdleConnections()
}

func loopbackDialer(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(host, "localhost") {
			// Try both numeric addresses so the resolver cannot change the destination.
			conn, firstErr := dialer.DialContext(ctx, network, net.JoinHostPort("127.0.0.1", port))
			if firstErr == nil {
				return conn, nil
			}
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort("::1", port))
			if err == nil {
				return conn, nil
			}
			return nil, errors.Join(firstErr, err)
		}
		ip, err := netip.ParseAddr(host)
		if err != nil || ip.Zone() != "" || !ip.Unmap().IsLoopback() {
			return nil, errors.New("transport: plain HTTP dial requires a loopback address")
		}
		return dialer.DialContext(ctx, network, address)
	}
}

func verifyLeaf(connection tls.ConnectionState, host string, pin [sha256.Size]byte) error {
	if len(connection.PeerCertificates) == 0 {
		return errors.New("transport: server did not present a certificate")
	}
	leaf := connection.PeerCertificates[0]
	key := sha256.Sum256(leaf.RawSubjectPublicKeyInfo)
	if subtle.ConstantTimeCompare(key[:], pin[:]) != 1 {
		return ErrCertificateKeyMismatch
	}
	now := time.Now()
	if now.Before(leaf.NotBefore) || now.After(leaf.NotAfter) {
		return errors.New("transport: server certificate is outside its validity period")
	}
	if err := leaf.VerifyHostname(host); err != nil {
		return err
	}
	if connection.NegotiatedProtocol != "h2" {
		return errors.New("transport: server did not negotiate HTTP/2")
	}
	return nil
}
