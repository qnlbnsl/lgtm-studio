package auth

import "testing"

// These cases close the gap of post-a 1: an HTTPS public origin never admits a
// loopback alias, because the aliases exist for a plaintext loopback origin alone.
func TestHTTPSPublicOriginRefusesLoopbackAliases(t *testing.T) {
	for _, public := range []string{"https://app.example.test", "https://app.example.test:8443"} {
		t.Run(public, func(t *testing.T) {
			p := mustPolicy(t, public, nil)
			runOriginCases(t, p, []originCase{
				{name: "localhost", origin: "https://localhost", refuse: true},
				{name: "localhost default port", origin: "https://localhost:443", refuse: true},
				{name: "localhost public port", origin: "https://localhost:8443", refuse: true},
				{name: "ipv4 loopback", origin: "https://127.0.0.1", refuse: true},
				{name: "ipv4 loopback default port", origin: "https://127.0.0.1:443", refuse: true},
				{name: "ipv4 loopback public port", origin: "https://127.0.0.1:8443", refuse: true},
				{name: "ipv6 loopback", origin: "https://[::1]", refuse: true},
				{name: "ipv6 loopback default port", origin: "https://[::1]:443", refuse: true},
				{name: "ipv6 loopback public port", origin: "https://[::1]:8443", refuse: true},
			})
		})
	}
}

// The tests below record a present choice of the code. Issue 1651 keeps open
// whether these spellings should be refused instead.

func TestPresentChoiceOriginPortWithLeadingZerosIsAccepted(t *testing.T) {
	p := mustPolicy(t, "http://127.0.0.1:8787", nil)
	runOriginCases(t, p, []originCase{
		{name: "leading zero port equals the public port", origin: "http://localhost:08787", mode: plainMode},
	})
	q := mustPolicy(t, "https://app.example.test:0443", nil)
	runOriginCases(t, q, []originCase{
		{name: "leading zero default port is the default port", origin: "https://app.example.test", mode: secureMode},
	})
}
