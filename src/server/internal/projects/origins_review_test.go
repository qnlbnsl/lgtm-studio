package projects

import "testing"

func TestProjectOriginComparisonPortDecimal(t *testing.T) {
	const want = "ssh://example.test:2222/Checkout.git"
	for _, origin := range []string{"ssh://example.test:02222/Checkout.git", want} {
		got, valid := normalizeOrigin(origin)
		if !valid || got != want {
			t.Errorf("normalizeOrigin(%q) = (%q, %t), want (%q, true)", origin, got, valid, want)
		}
	}
}

func TestProjectOriginComparisonKeys(t *testing.T) {
	for _, tt := range []struct {
		name, left, right string
		equal             bool
	}{
		{"SchemeAndHostCase", "HTTPS://Example.TEST/git/Checkout.git", "https://example.test/git/Checkout.git", true},
		{"DefaultPort", "ssh://example.test:22/git/Checkout.git", "ssh://example.test/git/Checkout.git", true},
		{"AbsoluteShorthand", "example.test:/git/Checkout.git", "ssh://example.test/git/Checkout.git", true},
		{"DifferentHost", "ssh://example.test/git/Checkout.git", "ssh://other.test/git/Checkout.git", false},
		{"DifferentPath", "ssh://example.test/git/Checkout.git", "ssh://example.test/git/checkout.git", false},
		{"DifferentNondefaultPort", "ssh://example.test:2222/git/Checkout.git", "ssh://example.test:2223/git/Checkout.git", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			left, leftValid := normalizeOrigin(tt.left)
			right, rightValid := normalizeOrigin(tt.right)
			if !leftValid || !rightValid {
				t.Fatalf("Both comparison origins must be valid. Got %t for %q and %t for %q.", leftValid, tt.left, rightValid, tt.right)
			}
			if (left == right) != tt.equal {
				t.Errorf("The comparison keys %q and %q have equality %t, want %t.", left, right, left == right, tt.equal)
			}
		})
	}
}
