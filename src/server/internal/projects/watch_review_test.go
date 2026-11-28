package projects_test

import (
	"errors"
	"testing"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth/authtest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/projects"
)

func TestProjectWatchEntryRefusalSentence(t *testing.T) {
	start := time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC)
	db, _, setClock := authtest.ClockStore(t, start, auth.Migrations(), machines.Migrations(), projects.Migrations())
	sessions := auth.NewSessions(db, auth.Password{})
	const sentence = "Sign in to continue."

	t.Run("MissingSession", func(t *testing.T) {
		client, _ := serveProjectWatch(t, db, sessions, t.Context())
		requireProjectWatchEnd(t, openProjectWatch(t, client), "opening without a session", sentence)
	})
	t.Run("ExpiredSession", func(t *testing.T) {
		base, verifier := watchSessionContext(t, sessions)
		setClock(start.Add((auth.SessionMaxAgeSeconds + 1) * time.Second))
		if err := sessions.Check(t.Context(), verifier); !errors.Is(err, auth.ErrNoSession) {
			t.Fatalf("The watch fixture must have an expired session. The session check returned %v.", err)
		}
		client, _ := serveProjectWatch(t, db, sessions, base)
		requireProjectWatchEnd(t, openProjectWatch(t, client), "opening with an expired session", sentence)
	})
}
