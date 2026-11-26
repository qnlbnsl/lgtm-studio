package config

import (
	"errors"
	"io"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config is the server's settings.
type Config struct {
	ListenAddr            string
	PublicURL             string
	AllowedOrigins        []string
	Password              string
	Postgres              *pgxpool.Config
	AllowPrivateStack     bool
	OpenBaoAddr           string
	OpenBaoCredentialFile string
}

// Load reads the settings and builds the database configuration. It leaves the
// website and vault settings for the application to validate after reconciliation.
func Load(lookup func(string) (string, bool)) (Config, error) {
	value := func(name string) string {
		v, _ := lookup(name)
		return v
	}
	withDefault := func(name, fallback string) string {
		if v, ok := lookup(name); ok {
			return v
		}
		return fallback
	}

	cfg := Config{
		ListenAddr:            withDefault("LGTM_LISTEN_ADDR", "127.0.0.1:8080"),
		PublicURL:             value("LGTM_PUBLIC_URL"),
		Password:              value("LGTM_WEB_PASSWORD"),
		OpenBaoAddr:           value("LGTM_OPENBAO_ADDR"),
		OpenBaoCredentialFile: value("LGTM_OPENBAO_CREDENTIAL_FILE"),
	}
	if cfg.PublicURL == "" {
		host, port, err := net.SplitHostPort(cfg.ListenAddr)
		if err == nil && (strings.EqualFold(host, "localhost") || isLoopback(host)) {
			cfg.PublicURL = "http://" + net.JoinHostPort(host, port)
		}
	}
	if raw := value("LGTM_ALLOWED_ORIGINS"); raw != "" {
		cfg.AllowedOrigins = strings.Split(raw, ",")
		for i, origin := range cfg.AllowedOrigins {
			cfg.AllowedOrigins[i] = strings.TrimSpace(origin)
		}
	}

	switch withDefault("LGTM_ALLOW_PRIVATE_STACK", "false") {
	case "true":
		cfg.AllowPrivateStack = true
	case "false":
	default:
		return cfg, errors.New("LGTM_ALLOW_PRIVATE_STACK must be true or false")
	}

	host := withDefault("LGTM_POSTGRES_HOST", "127.0.0.1")
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		if addr, err := netip.ParseAddr(host[1 : len(host)-1]); err == nil && addr.Is6() {
			host = addr.String()
		}
	}
	port, err := strconv.ParseUint(withDefault("LGTM_POSTGRES_PORT", "5432"), 10, 16)
	if err != nil || port == 0 {
		return cfg, errors.New("LGTM_POSTGRES_PORT must be a port from 1 to 65535")
	}
	network, _ := pgconn.NetworkAddress(host, uint16(port))
	if host == "" || strings.ContainsAny(host, ",\x00") ||
		(network == "tcp" && !validTCPHost(host)) {
		return cfg, errors.New("LGTM_POSTGRES_HOST must name one host or socket directory")
	}
	database := withDefault("LGTM_POSTGRES_DATABASE", "lgtm_app")
	if database == "" || strings.ContainsRune(database, 0) {
		return cfg, errors.New("LGTM_POSTGRES_DATABASE must name a database")
	}
	user := withDefault("LGTM_POSTGRES_USER", "lgtm_app")
	if user == "" || strings.ContainsRune(user, 0) {
		return cfg, errors.New("LGTM_POSTGRES_USER must name a user")
	}

	password, err := readPostgresPassword(value("LGTM_POSTGRES_PASSWORD_FILE"), cfg.AllowPrivateStack)
	if err != nil {
		return cfg, err
	}

	sslMode := "verify-full"
	if isLoopback(host) || network == "unix" || (cfg.AllowPrivateStack && host == "postgres") {
		sslMode = "disable"
	}
	cfg.Postgres, err = postgresPool(host, uint16(port), database, user, password, sslMode)
	return cfg, err
}

func isLoopback(host string) bool {
	addr, err := netip.ParseAddr(host)
	return err == nil && addr.Unmap().IsLoopback()
}

func validTCPHost(host string) bool {
	if _, err := netip.ParseAddr(host); err == nil {
		return true
	}
	return !strings.ContainsAny(host, " \t\r\n\v\f/\\:@[]")
}

func readPostgresPassword(path string, privateStack bool) (string, error) {
	invalidFile := errors.New("LGTM_POSTGRES_PASSWORD_FILE must point to a protected regular password file")
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", invalidFile
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o027 != 0 {
		return "", invalidFile
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || (stat.Uid != uint32(os.Geteuid()) && stat.Gid != uint32(os.Getegid())) {
		return "", invalidFile
	}
	var reader io.Reader = f
	if privateStack {
		reader = io.LimitReader(f, 66)
	}
	data, err := io.ReadAll(reader)
	defer clear(data)
	if err != nil {
		return "", invalidFile
	}
	line := string(data)
	password := strings.TrimSuffix(line, "\n")
	if privateStack {
		if len(line) != 65 || len(password) != 64 || strings.IndexFunc(password, func(r rune) bool {
			return !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f')
		}) >= 0 {
			return "", errors.New("LGTM_POSTGRES_PASSWORD_FILE must contain 64 lowercase hexadecimal characters and a line feed")
		}
	} else {
		if len(password) != len(line) {
			password = strings.TrimSuffix(password, "\r")
		}
		if password == "" || strings.ContainsAny(password, "\r\n\x00") {
			return "", errors.New("LGTM_POSTGRES_PASSWORD_FILE must contain one nonempty password line")
		}
	}
	return password, nil
}

func postgresPool(host string, port uint16, database, user, password, sslMode string) (*pgxpool.Config, error) {
	// The driver reads its process environment as well as the explicit settings.
	// A service file can supply additional driver settings.
	if os.Getenv("PGSERVICE") != "" {
		return nil, errors.New("PGSERVICE cannot be used with the LGTM_POSTGRES settings")
	}
	escape := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	dsn := "host='" + escape.Replace(host) + "' port=" + strconv.Itoa(int(port)) +
		" dbname='" + escape.Replace(database) + "' user='" + escape.Replace(user) +
		"' password='" + escape.Replace(password) + "' sslmode=" + sslMode + " pool_max_conns=8"
	cfg, err := pgxpool.ParseConfig(dsn)
	invalidConfig := errors.New("the LGTM_POSTGRES settings do not select one database endpoint with the required transport")
	if err != nil {
		return nil, invalidConfig
	}
	conn := cfg.ConnConfig
	if conn.Host != host || conn.Port != port || len(conn.Fallbacks) != 0 {
		return nil, invalidConfig
	}
	tls := conn.TLSConfig
	if sslMode == "verify-full" {
		if tls == nil || tls.InsecureSkipVerify || tls.ServerName != host {
			return nil, invalidConfig
		}
	} else if tls != nil {
		return nil, invalidConfig
	}
	return cfg, nil
}
