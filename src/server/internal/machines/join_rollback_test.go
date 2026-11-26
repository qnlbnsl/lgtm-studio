package machines_test

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestRedeemRollsBackAfterMachineInsert(t *testing.T) {
	e := newTokEnv(t, loopbackURL)
	tok, msg := e.create(t, machines.TokenOptions{})
	key := keyOf(0x31)

	// Only a token claim that can see the inserted machine raises the expected SQL state.
	storetest.Exec(t, e.obs, `
CREATE FUNCTION fail_join_token_claim() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM machines WHERE id = NEW.used_by AND token_id = NEW.id) THEN
        RAISE EXCEPTION 'The token claim did not see the inserted machine.';
    END IF;
    RAISE EXCEPTION USING ERRCODE = 'P4701', MESSAGE = 'The test refuses the token claim.';
END $$`)
	storetest.Exec(t, e.obs, `
CREATE TRIGGER fail_join_token_claim AFTER UPDATE ON client_join_tokens
FOR EACH ROW WHEN (OLD.used_at IS NULL AND NEW.used_at IS NOT NULL)
EXECUTE FUNCTION fail_join_token_claim()`)

	before := e.snapshot(t, tok.ID)
	feedBefore := e.feed(t)
	_, err := e.redeem(msg, key, "box")
	var dbErr *store.Error
	if !errors.As(err, &dbErr) || dbErr.SQLState != "P4701" || dbErr.Outcome() != store.OutcomeRolledBack {
		t.Error("Redeem must report a rolled-back database error from the trigger that observed the inserted machine.")
	}
	e.requireUnchanged(t, "failed token claim", tok.ID, before)
	if !slices.Equal(e.feed(t), feedBefore) {
		t.Error("The failed token claim changed the feed entries.")
	}

	storetest.Exec(t, e.obs, `DROP TRIGGER fail_join_token_claim ON client_join_tokens`)
	joined, err := e.redeem(msg, key, "box")
	if err != nil || joined.Replay || !uuidV7.MatchString(joined.MachineID) {
		t.Fatal("Retrying after the failed claim must create a machine as a first use.")
	}
	if got := e.machineCount(t); got != before.machines+1 {
		t.Errorf("The retry left %d machines. Want %d.", got, before.machines+1)
	}
	row := e.row(t, tok.ID)
	if row.usedAt == nil || row.usedBy == nil || *row.usedBy != joined.MachineID || !bytes.Equal(row.usedKey, key) {
		t.Error("The retry must record the token claim for the new machine.")
	}
}
