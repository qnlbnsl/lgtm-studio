package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth/authtest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

// fixtureNow is the start of the fixture clock. It is far from the Go clock on purpose.
var fixtureNow = time.Date(2031, 3, 4, 10, 0, 0, 0, time.UTC)

const sessionLifetime = time.Duration(SessionMaxAgeSeconds) * time.Second

// sessionFixture is a migrated store whose database clock the test controls.
type sessionFixture struct {
	s        *store.Store
	obs      *pgx.Conn
	setClock func(time.Time)
}

func newSessionFixture(t *testing.T, at time.Time) sessionFixture {
	t.Helper()
	s, obs, set := authtest.ClockStore(t, at, Migrations())
	return sessionFixture{s: s, obs: obs, setClock: set}
}

// sessionRow is one row of web_sessions.
type sessionRow struct {
	verifier  Verifier
	createdAt time.Time
	salt      [16]byte
	binding   [32]byte
}

// verifierOf returns a verifier that sorts by n first.
func verifierOf(n byte) Verifier {
	var v Verifier
	v[0] = n
	return v
}

// feedKeyOf is the independent derivation of the published reference key of the plan.
func feedKeyOf(v Verifier) string {
	sum := sha256.Sum256(append([]byte("lgtm-web-session-feed-v1\x00"), v[:]...))
	return hex.EncodeToString(sum[:])
}

// validRow builds a row whose binding was computed independently for the password.
func validRow(t *testing.T, p Password, v Verifier, createdAt time.Time) sessionRow {
	t.Helper()
	row := sessionRow{verifier: v, createdAt: createdAt.Truncate(time.Microsecond)}
	if _, err := rand.Read(row.salt[:]); err != nil {
		t.Fatal(err)
	}
	binding, err := bindPassword(p, row.salt, v, row.createdAt)
	if err != nil {
		t.Fatalf("binding the fixture row failed: %v", err)
	}
	row.binding = binding
	return row
}

// junkRow builds a row of valid shape whose binding no password produces. It is cheap.
func junkRow(t *testing.T, v Verifier, createdAt time.Time) sessionRow {
	t.Helper()
	row := sessionRow{verifier: v, createdAt: createdAt.Truncate(time.Microsecond)}
	if _, err := rand.Read(row.salt[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(row.binding[:]); err != nil {
		t.Fatal(err)
	}
	return row
}

func insertRow(t *testing.T, obs *pgx.Conn, row sessionRow) {
	t.Helper()
	storetest.Exec(t, obs,
		"INSERT INTO web_sessions (verifier, created_at, password_salt, password_binding) VALUES ($1, $2, $3, $4)",
		row.verifier[:], row.createdAt, row.salt[:], row.binding[:])
}

// readRow returns the stored row of a verifier, or false if there is none.
func readRow(t *testing.T, obs *pgx.Conn, v Verifier) (sessionRow, bool) {
	t.Helper()
	var verifier, salt, binding []byte
	row := sessionRow{}
	err := obs.QueryRow(t.Context(),
		"SELECT verifier, created_at, password_salt, password_binding FROM web_sessions WHERE verifier = $1", v[:]).
		Scan(&verifier, &row.createdAt, &salt, &binding)
	if errors.Is(err, pgx.ErrNoRows) {
		return sessionRow{}, false
	}
	if err != nil {
		t.Fatalf("reading the stored session failed: %v", err)
	}
	copy(row.verifier[:], verifier)
	copy(row.salt[:], salt)
	copy(row.binding[:], binding)
	return row, true
}

// storedVerifiers lists the stored verifiers, newest first, in the declared order.
func storedVerifiers(t *testing.T, obs *pgx.Conn) []Verifier {
	t.Helper()
	var out []Verifier
	for _, raw := range storetest.Rows[[]byte](t, obs, "SELECT verifier FROM web_sessions ORDER BY created_at DESC, verifier DESC") {
		var v Verifier
		copy(v[:], raw)
		out = append(out, v)
	}
	return out
}

func sortedVerifiers(vs ...Verifier) []Verifier {
	out := slices.Clone(vs)
	slices.SortFunc(out, func(a, b Verifier) int { return slices.Compare(a[:], b[:]) })
	return out
}

func requireStored(t *testing.T, obs *pgx.Conn, what string, want ...Verifier) {
	t.Helper()
	got := sortedVerifiers(storedVerifiers(t, obs)...)
	if !slices.Equal(got, sortedVerifiers(want...)) {
		t.Fatalf("%s: the stored sessions are %s, want %s", what, shortList(got), shortList(sortedVerifiers(want...)))
	}
}

func shortList(vs []Verifier) string {
	out := "["
	for i, v := range vs {
		if i > 0 {
			out += " "
		}
		out += fmt.Sprintf("%02x", v[0])
	}
	return out + "]"
}

type feedEntry struct {
	Topic, Scope, Key, Change string
}

func feedEntries(t *testing.T, obs *pgx.Conn) []feedEntry {
	t.Helper()
	rows, err := obs.Query(t.Context(), "SELECT topic, scope, key, change FROM feed ORDER BY position")
	if err != nil {
		t.Fatalf("reading the feed failed: %v", err)
	}
	entries, err := pgx.CollectRows(rows, pgx.RowToStructByPos[feedEntry])
	if err != nil {
		t.Fatalf("reading the feed failed: %v", err)
	}
	return entries
}

func feedHead(t *testing.T, obs *pgx.Conn) int64 {
	t.Helper()
	return storetest.Scalar[int64](t, obs, "SELECT COALESCE(max(position), 0) FROM feed")
}

// requireFeedFrom checks the entries after a head: the same set of removal references, and nothing else.
func requireRemovalsSince(t *testing.T, obs *pgx.Conn, head int64, what string, removed ...Verifier) {
	t.Helper()
	entries := feedEntries(t, obs)
	if int64(len(entries)) < head {
		t.Fatalf("%s: the feed lost entries", what)
	}
	var got []string
	for _, e := range entries[head:] {
		if e.Topic != "web_sessions" || e.Scope != "" || e.Change != string(store.Removed) {
			t.Fatalf("%s: unexpected feed entry %+v", what, e)
		}
		got = append(got, e.Key)
	}
	var want []string
	for _, v := range removed {
		want = append(want, feedKeyOf(v))
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("%s: the removal references are %v, want one for each of %d deleted rows: %v", what, got, len(want), want)
	}
}

// requireEqualDigestCall parses a source file of the package and requires a call of equalDigest in it.
func requireEqualDigestCall(t *testing.T, file string) {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s failed: %v", file, err)
	}
	found := false
	ast.Inspect(parsed, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "equalDigest" {
				found = true
			}
		}
		return true
	})
	if !found {
		t.Errorf("%s must compare bindings with equalDigest", file)
	}
}

func TestSessionMigration(t *testing.T) {
	m := Migrations()
	if m.Part != "auth" {
		t.Fatalf("the manifest part is %q, want auth", m.Part)
	}
	f := newSessionFixture(t, fixtureNow)
	ctx := t.Context()
	for run := 1; run <= 2; run++ {
		if err := f.s.Migrate(ctx, Migrations()); err != nil {
			t.Fatalf("running the manifest again (%d) failed: %v", run, err)
		}
	}
	if got := storetest.Scalar[int](t, f.obs, "SELECT version FROM schema_migrations WHERE part = 'auth'"); got != 1 {
		t.Fatalf("the auth ledger entry is %d, want 1", got)
	}

	type column struct{ Name, Type, Nullable string }
	rows, err := f.obs.Query(ctx, `
SELECT column_name, data_type, is_nullable FROM information_schema.columns
WHERE table_schema = current_schema() AND table_name = 'web_sessions' ORDER BY column_name`)
	if err != nil {
		t.Fatal(err)
	}
	columns, err := pgx.CollectRows(rows, pgx.RowToStructByPos[column])
	if err != nil {
		t.Fatal(err)
	}
	wantColumns := []column{
		{"created_at", "timestamp with time zone", "NO"},
		{"password_binding", "bytea", "NO"},
		{"password_salt", "bytea", "NO"},
		{"verifier", "bytea", "NO"},
	}
	if !slices.Equal(columns, wantColumns) {
		t.Fatalf("web_sessions has the columns %+v, want exactly %+v", columns, wantColumns)
	}
	primary := storetest.Rows[string](t, f.obs, `
SELECT a.attname FROM pg_index i JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY (i.indkey)
WHERE i.indrelid = 'web_sessions'::regclass AND i.indisprimary`)
	if !slices.Equal(primary, []string{"verifier"}) {
		t.Fatalf("the primary key is %v, want verifier only", primary)
	}

	good := junkRow(t, verifierOf(1), fixtureNow)
	insertRow(t, f.obs, good)
	insert := "INSERT INTO web_sessions (verifier, created_at, password_salt, password_binding) VALUES ($1, $2, $3, $4)"
	cases := map[string][4]any{
		"short verifier":   {make([]byte, 31), fixtureNow, good.salt[:], good.binding[:]},
		"long verifier":    {make([]byte, 33), fixtureNow, good.salt[:], good.binding[:]},
		"short salt":       {verifierOf(2).bytes(), fixtureNow, make([]byte, 15), good.binding[:]},
		"long salt":        {verifierOf(2).bytes(), fixtureNow, make([]byte, 17), good.binding[:]},
		"short binding":    {verifierOf(2).bytes(), fixtureNow, good.salt[:], make([]byte, 31)},
		"long binding":     {verifierOf(2).bytes(), fixtureNow, good.salt[:], make([]byte, 33)},
		"infinite created": {verifierOf(2).bytes(), "infinity", good.salt[:], good.binding[:]},
	}
	for name, args := range cases {
		_, err := f.obs.Exec(ctx, insert, args[0], args[1], args[2], args[3])
		var server *pgconn.PgError
		if !errors.As(err, &server) || server.Code != "23514" {
			t.Errorf("%s: inserting returned %v, want a check violation (23514)", name, err)
		}
	}
	_, err = f.obs.Exec(ctx, insert, good.verifier[:], fixtureNow, good.salt[:], good.binding[:])
	var server *pgconn.PgError
	if !errors.As(err, &server) || server.Code != "23505" {
		t.Errorf("inserting a duplicate verifier returned %v, want a unique violation (23505)", err)
	}
	if got := storetest.Scalar[int](t, f.obs, "SELECT count(*) FROM web_sessions"); got != 1 {
		t.Errorf("web_sessions holds %d rows after the refused inserts, want 1", got)
	}
}

func (v Verifier) bytes() []byte { return v[:] }

func TestReconcilePassword(t *testing.T) {
	requireEqualDigestCall(t, "reconcile.go")
	one := mustPassword(t, "reconcile password one")
	two := mustPassword(t, "reconcile password two")
	T := fixtureNow
	var (
		live1   = verifierOf(0x11)
		live2   = verifierOf(0x12)
		edge    = verifierOf(0x13) // exactly one minute ahead: still live
		expired = verifierOf(0x14) // exactly 720 hours old: expired
		future  = verifierOf(0x15) // one microsecond over a minute ahead: removed
	)

	t.Run("the same password keeps the live rows", func(t *testing.T) {
		f := newSessionFixture(t, T)
		insertRow(t, f.obs, validRow(t, one, live1, T.Add(-time.Hour)))
		insertRow(t, f.obs, validRow(t, one, live2, T.Add(-2*time.Hour)))
		insertRow(t, f.obs, validRow(t, one, edge, T.Add(time.Minute)))
		insertRow(t, f.obs, validRow(t, one, expired, T.Add(-sessionLifetime)))
		insertRow(t, f.obs, validRow(t, one, future, T.Add(time.Minute+time.Microsecond)))
		head := feedHead(t, f.obs)
		if err := Reconcile(t.Context(), f.s, &one); err != nil {
			t.Fatalf("Reconcile failed: %v", err)
		}
		requireStored(t, f.obs, "after the same password", live1, live2, edge)
		requireRemovalsSince(t, f.obs, head, "after the same password", expired, future)
		head = feedHead(t, f.obs)
		if err := Reconcile(t.Context(), f.s, &one); err != nil {
			t.Fatalf("the second Reconcile failed: %v", err)
		}
		requireStored(t, f.obs, "after the second run", live1, live2, edge)
		requireRemovalsSince(t, f.obs, head, "after the second run")
	})

	t.Run("a changed password deletes every row", func(t *testing.T) {
		f := newSessionFixture(t, T)
		insertRow(t, f.obs, validRow(t, one, live1, T.Add(-time.Hour)))
		insertRow(t, f.obs, validRow(t, one, live2, T.Add(-2*time.Hour)))
		insertRow(t, f.obs, validRow(t, one, expired, T.Add(-sessionLifetime)))
		head := feedHead(t, f.obs)
		if err := Reconcile(t.Context(), f.s, &two); err != nil {
			t.Fatalf("Reconcile failed: %v", err)
		}
		requireStored(t, f.obs, "after the changed password")
		requireRemovalsSince(t, f.obs, head, "after the changed password", live1, live2, expired)
	})

	t.Run("a removed password deletes every row", func(t *testing.T) {
		f := newSessionFixture(t, T)
		insertRow(t, f.obs, validRow(t, one, live1, T.Add(-time.Hour)))
		insertRow(t, f.obs, validRow(t, one, live2, T.Add(-2*time.Hour)))
		insertRow(t, f.obs, validRow(t, one, expired, T.Add(-sessionLifetime)))
		head := feedHead(t, f.obs)
		if err := Reconcile(t.Context(), f.s, nil); err != nil {
			t.Fatalf("Reconcile failed: %v", err)
		}
		requireStored(t, f.obs, "after the removed password")
		requireRemovalsSince(t, f.obs, head, "after the removed password", live1, live2, expired)
	})

	t.Run("one tampered binding deletes the whole set", func(t *testing.T) {
		f := newSessionFixture(t, T)
		tampered := validRow(t, one, live2, T.Add(-2*time.Hour))
		tampered.binding[0] ^= 0xff
		insertRow(t, f.obs, validRow(t, one, live1, T.Add(-time.Hour)))
		insertRow(t, f.obs, tampered)
		insertRow(t, f.obs, validRow(t, one, expired, T.Add(-sessionLifetime)))
		head := feedHead(t, f.obs)
		if err := Reconcile(t.Context(), f.s, &one); err != nil {
			t.Fatalf("Reconcile failed: %v", err)
		}
		requireStored(t, f.obs, "after the tampered binding")
		requireRemovalsSince(t, f.obs, head, "after the tampered binding", live1, live2, expired)
	})

	t.Run("a tag with the right shape for another created time is refused", func(t *testing.T) {
		f := newSessionFixture(t, T)
		row := validRow(t, one, live1, T.Add(-time.Hour))
		row.createdAt = row.createdAt.Add(time.Microsecond)
		insertRow(t, f.obs, row)
		if err := Reconcile(t.Context(), f.s, &one); err != nil {
			t.Fatalf("Reconcile failed: %v", err)
		}
		requireStored(t, f.obs, "after the shifted created time")
	})

	t.Run("restoring the former password does not bring rows back", func(t *testing.T) {
		f := newSessionFixture(t, T)
		insertRow(t, f.obs, validRow(t, one, live1, T.Add(-time.Hour)))
		insertRow(t, f.obs, validRow(t, one, live2, T.Add(-2*time.Hour)))
		if err := Reconcile(t.Context(), f.s, &two); err != nil {
			t.Fatalf("Reconcile with the changed password failed: %v", err)
		}
		head := feedHead(t, f.obs)
		if head != 2 {
			t.Fatalf("the feed head is %d after the purge, want 2", head)
		}
		if err := Reconcile(t.Context(), f.s, &one); err != nil {
			t.Fatalf("Reconcile with the restored password failed: %v", err)
		}
		requireStored(t, f.obs, "after the restored password")
		requireRemovalsSince(t, f.obs, head, "after the restored password")
	})

	t.Run("an empty table needs no change", func(t *testing.T) {
		f := newSessionFixture(t, T)
		for _, password := range []*Password{nil, &one} {
			if err := Reconcile(t.Context(), f.s, password); err != nil {
				t.Fatalf("Reconcile of an empty table failed: %v", err)
			}
		}
		if got := feedHead(t, f.obs); got != 0 {
			t.Fatalf("the feed head is %d, want 0", got)
		}
	})
}

func TestReconcileRollsBack(t *testing.T) {
	one := mustPassword(t, "rollback password one")
	two := mustPassword(t, "rollback password two")
	T := fixtureNow
	f := newSessionFixture(t, T)
	var (
		expired = verifierOf(0x21)
		live1   = verifierOf(0x22)
		live2   = verifierOf(0x23)
	)
	insertRow(t, f.obs, validRow(t, one, expired, T.Add(-sessionLifetime)))
	insertRow(t, f.obs, validRow(t, one, live1, T.Add(-time.Hour)))
	insertRow(t, f.obs, validRow(t, one, live2, T.Add(-2*time.Hour)))
	storetest.Exec(t, f.obs, fmt.Sprintf(`
CREATE FUNCTION refuse_one_session_delete() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.verifier = '\x%s'::bytea THEN
        RAISE EXCEPTION 'The fixture refuses this delete.' USING ERRCODE = '23514';
    END IF;
    RETURN OLD;
END;
$$`, hex.EncodeToString(live2[:])))
	storetest.Exec(t, f.obs, "CREATE TRIGGER refuse_one BEFORE DELETE ON web_sessions FOR EACH ROW EXECUTE FUNCTION refuse_one_session_delete()")
	head := feedHead(t, f.obs)

	if err := Reconcile(t.Context(), f.s, &two); err == nil {
		t.Fatal("Reconcile reported success although the purge could not commit")
	}
	requireStored(t, f.obs, "after the refused purge", expired, live1, live2)
	if got := feedHead(t, f.obs); got != head {
		t.Fatalf("the feed head moved from %d to %d although the purge rolled back", head, got)
	}

	storetest.Exec(t, f.obs, "DROP TRIGGER refuse_one ON web_sessions")
	if err := Reconcile(t.Context(), f.s, &two); err != nil {
		t.Fatalf("Reconcile failed once the trigger was gone: %v", err)
	}
	requireStored(t, f.obs, "after the committed purge")
	requireRemovalsSince(t, f.obs, head, "after the committed purge", expired, live1, live2)
}
