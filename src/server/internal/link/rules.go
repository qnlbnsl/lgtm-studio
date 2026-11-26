package link

import (
	"crypto/ed25519"
	"crypto/sha256"
	"regexp"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

var canonicalUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func validJoinRequest(msg *linkv1.JoinRequest) bool {
	return canonicalUUID.MatchString(msg.GetTokenId()) &&
		len(msg.GetTokenSecret()) == sha256.Size &&
		len(msg.GetMachinePublicKey()) == ed25519.PublicKeySize
}
