package enroll

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/state"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/transport"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

var (
	// ErrEnrollmentExists reports that the folder already holds an enrollment file.
	ErrEnrollmentExists = errors.New("enroll: enrollment exists")
	// ErrEnrollmentMissing reports that the folder holds no enrollment file.
	ErrEnrollmentMissing = errors.New("enroll: enrollment is missing")
	// ErrEnrollmentInvalid reports that the enrollment file cannot be trusted.
	ErrEnrollmentInvalid = errors.New("enroll: enrollment is invalid")

	canonicalUUID   = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	errJoinRedirect = errors.New("enroll: join endpoint redirected the request")
)

const (
	maxEnrollmentBytes = 4096
	maxJoinTokenBytes  = 4096
	joinBudget         = 15 * time.Minute
	joinAttemptTimeout = 10 * time.Second
	maxJoinBackoff     = 15 * time.Second
)

// The tests replace the clock and wait to exercise retries without real delays.
var (
	joinNow   = time.Now
	joinSleep = func(ctx context.Context, d time.Duration) error {
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	}
)

// Server is what a machine keeps of its enrollment.
type Server struct {
	Address              string
	MachineID            string
	ClientPublicKey      ed25519.PublicKey
	PublicKey            ed25519.PublicKey
	CertificateKeySHA256 []byte
}

// JoinError carries a fixed diagnostic code.
type JoinError struct{ Code string }

func (e JoinError) Error() string { return "enroll: join refused: " + e.Code }

type enrollmentFile struct {
	Version              int    `json:"version"`
	Address              string `json:"address"`
	MachineID            string `json:"machineId"`
	ClientPublicKey      string `json:"clientPublicKey"`
	PublicKey            string `json:"serverPublicKey"`
	CertificateKeySHA256 string `json:"certificateKeySHA256,omitempty"`
}

// LoadServer reads and validates the saved enrollment.
func LoadServer(f *state.Folder) (Server, error) {
	file, err := os.OpenFile(f.Paths().Identity, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(err, os.ErrNotExist) {
		return Server{}, ErrEnrollmentMissing
	}
	if err != nil {
		return Server{}, ErrEnrollmentInvalid
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 ||
		info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		return Server{}, ErrEnrollmentInvalid
	}
	data, err := io.ReadAll(io.LimitReader(file, maxEnrollmentBytes+1))
	if err != nil || len(data) > maxEnrollmentBytes {
		return Server{}, ErrEnrollmentInvalid
	}
	var saved enrollmentFile
	if err := json.Unmarshal(data, &saved); err != nil || saved.Version != 1 {
		return Server{}, ErrEnrollmentInvalid
	}
	clientKey, clientOK := decodeBase64(base64.StdEncoding, saved.ClientPublicKey)
	serverKey, serverOK := decodeBase64(base64.StdEncoding, saved.PublicKey)
	pin, pinOK := decodeBase64(base64.StdEncoding, saved.CertificateKeySHA256)
	if !clientOK || !serverOK || !pinOK {
		return Server{}, ErrEnrollmentInvalid
	}
	return validateServer(Server{
		Address: saved.Address, MachineID: saved.MachineID,
		ClientPublicKey: clientKey, PublicKey: serverKey, CertificateKeySHA256: pin,
	})
}

func validateServer(server Server) (Server, error) {
	origin, err := transport.ParseOrigin(server.Address)
	if err != nil || !validCertificatePin(origin, server.CertificateKeySHA256) ||
		!canonicalUUID.MatchString(server.MachineID) ||
		len(server.PublicKey) != ed25519.PublicKeySize ||
		len(server.ClientPublicKey) != ed25519.PublicKeySize {
		return Server{}, ErrEnrollmentInvalid
	}
	server.Address = origin
	return server, nil
}

func validCertificatePin(origin string, pin []byte) bool {
	if strings.HasPrefix(origin, "https://") {
		return len(pin) == sha256.Size
	}
	return len(pin) == 0
}

func decodeBase64(encoding *base64.Encoding, text string) ([]byte, bool) {
	data, err := encoding.DecodeString(text)
	return data, err == nil && encoding.EncodeToString(data) == text
}

// SaveServer commits a verified enrollment to the locked folder.
func SaveServer(f *state.Folder, server Server) error {
	paths, release, err := f.Claim("enrollment")
	if err != nil {
		return err
	}
	defer release()
	if err := requireUnenrolled(paths); err != nil {
		return err
	}
	server, err = validateServer(server)
	if err != nil {
		return err
	}
	data, err := json.Marshal(enrollmentFile{
		Version:              1,
		Address:              server.Address,
		MachineID:            server.MachineID,
		ClientPublicKey:      base64.StdEncoding.EncodeToString(server.ClientPublicKey),
		PublicKey:            base64.StdEncoding.EncodeToString(server.PublicKey),
		CertificateKeySHA256: base64.StdEncoding.EncodeToString(server.CertificateKeySHA256),
	})
	if err != nil || len(data) > maxEnrollmentBytes {
		return ErrEnrollmentInvalid
	}
	err = state.WriteAtomic(paths.Dir, state.IdentityFile, data)
	if !errors.Is(err, state.ErrPlacedNotSynced) {
		return err
	}
	if _, err := LoadServer(f); err != nil {
		return err
	}
	return state.SyncDir(paths.Dir)
}

// Join trades the token for a verified enrollment. SaveServer publishes it.
func Join(ctx context.Context, f *state.Folder, address, token, hostName string, onRetry func()) (Server, error) {
	paths, release, err := f.Claim("enrollment")
	if err != nil {
		return Server{}, err
	}
	defer release()
	if err := requireUnenrolled(paths); err != nil {
		return Server{}, err
	}
	if err := ctx.Err(); err != nil {
		return Server{}, err
	}
	decoded, origin, err := decodeJoinToken(address, token)
	if err != nil {
		return Server{}, err
	}
	httpClient, err := transport.NewHTTPClient(origin, decoded.GetCertificateKeySha256())
	if err != nil {
		return Server{}, JoinError{Code: "invalid_token"}
	}
	defer httpClient.CloseIdleConnections()
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return errJoinRedirect
	}

	key, err := durableJoinKey(paths)
	if err != nil {
		return Server{}, err
	}
	request := &linkv1.JoinRequest{
		TokenId:          decoded.GetTokenId(),
		TokenSecret:      decoded.GetSecret(),
		MachinePublicKey: key.PublicKey(),
		HostName:         joinHostName(hostName),
	}
	client := linkv1connect.NewLinkServiceClient(httpClient, origin, connect.WithReadMaxBytes(64<<10))
	reply, err := joinWithRetry(ctx, client, request, onRetry)
	if err != nil {
		return Server{}, err
	}
	if !canonicalUUID.MatchString(reply.GetMachineId()) || len(reply.GetServerPublicKey()) != ed25519.PublicKeySize {
		return Server{}, JoinError{Code: "invalid_reply"}
	}
	digest := sha256.Sum256(reply.GetServerPublicKey())
	if subtle.ConstantTimeCompare(digest[:], decoded.GetServerKeySha256()) != 1 {
		return Server{}, JoinError{Code: "server_key_mismatch"}
	}
	return Server{
		Address: origin, MachineID: reply.GetMachineId(),
		ClientPublicKey: request.GetMachinePublicKey(), PublicKey: reply.GetServerPublicKey(),
		CertificateKeySHA256: decoded.GetCertificateKeySha256(),
	}, nil
}

func requireUnenrolled(paths state.Paths) error {
	_, err := os.Lstat(paths.Identity)
	switch {
	case err == nil:
		return ErrEnrollmentExists
	case errors.Is(err, os.ErrNotExist):
		return nil
	default:
		return ErrEnrollmentInvalid
	}
}

func decodeJoinToken(address, token string) (*linkv1.JoinToken, string, error) {
	text, hasPrefix := strings.CutPrefix(token, "lgtm1_")
	invalid := JoinError{Code: "invalid_token"}
	if !hasPrefix || len(token) > maxJoinTokenBytes {
		return nil, "", invalid
	}
	data, ok := decodeBase64(base64.RawURLEncoding, text)
	var decoded linkv1.JoinToken
	if !ok || proto.Unmarshal(data, &decoded) != nil || !canonicalUUID.MatchString(decoded.GetTokenId()) ||
		len(decoded.GetSecret()) != 32 || len(decoded.GetServerKeySha256()) != sha256.Size {
		return nil, "", invalid
	}
	origin, err := transport.ParseOrigin(decoded.GetAddress())
	if err != nil || !validCertificatePin(origin, decoded.GetCertificateKeySha256()) {
		return nil, "", invalid
	}
	requested, err := transport.ParseOrigin(address)
	if err != nil || requested != origin {
		return nil, "", JoinError{Code: "origin_mismatch"}
	}
	return &decoded, origin, nil
}

func durableJoinKey(paths state.Paths) (*Credential, error) {
	key, err := Load(paths.Key)
	if errors.Is(err, ErrKeyMissing) {
		key, err = Create(paths.Key)
		if !errors.Is(err, state.ErrPlacedNotSynced) {
			return key, err
		}
		key, err = Load(paths.Key)
	}
	if err != nil {
		return nil, err
	}
	if err := state.SyncDir(paths.Dir); err != nil {
		return nil, err
	}
	return key, nil
}

func joinHostName(name string) string {
	name = strings.ReplaceAll(name, "\x00", "")
	name = strings.ToValidUTF8(name, "")
	if len(name) <= 255 {
		return name
	}
	end := 255
	for !utf8.RuneStart(name[end]) {
		end--
	}
	return name[:end]
}

func joinWithRetry(ctx context.Context, client linkv1connect.LinkServiceClient, request *linkv1.JoinRequest, onRetry func()) (*linkv1.JoinResponse, error) {
	deadline := joinNow().Add(joinBudget)
	backoff := time.Second
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		remaining := deadline.Sub(joinNow())
		if remaining <= 0 {
			return nil, JoinError{Code: "join_timeout"}
		}
		attempt, cancel := context.WithTimeout(ctx, min(joinAttemptTimeout, remaining))
		reply, err := client.Join(attempt, connect.NewRequest(request))
		cancel()
		if stopped := ctx.Err(); stopped != nil {
			return nil, stopped
		}
		if err == nil {
			return reply.Msg, nil
		}
		if final := joinRefusal(err); final != nil {
			return nil, final
		}

		remaining = deadline.Sub(joinNow())
		if remaining <= 0 {
			return nil, JoinError{Code: "join_timeout"}
		}
		if onRetry != nil {
			onRetry()
			onRetry = nil
			remaining = deadline.Sub(joinNow())
			if remaining <= 0 {
				return nil, JoinError{Code: "join_timeout"}
			}
		}
		if err := joinSleep(ctx, min(backoff, remaining)); err != nil {
			return nil, err
		}
		backoff = min(backoff*2, maxJoinBackoff)
	}
}

func joinRefusal(err error) error {
	// Transport and RPC errors may contain server-controlled text.
	switch {
	case errors.Is(err, transport.ErrCertificateKeyMismatch):
		return JoinError{Code: "certificate_key_mismatch"}
	case errors.Is(err, errJoinRedirect):
		return JoinError{Code: "invalid_reply"}
	}
	var remote *connect.Error
	if !errors.As(err, &remote) {
		return nil
	}
	switch remote.Code() {
	case connect.CodePermissionDenied:
		switch reason := remote.Meta().Get("Lgtm-Join-Refusal"); reason {
		case "token_unknown", "token_expired", "token_revoked", "token_used", "machine_removed":
			return JoinError{Code: reason}
		default:
			return JoinError{Code: "token_refused"}
		}
	case connect.CodeInvalidArgument:
		return JoinError{Code: "invalid_argument"}
	case connect.CodeResourceExhausted:
		if !connect.IsWireError(err) {
			return JoinError{Code: "invalid_reply"}
		}
	}
	return nil
}
