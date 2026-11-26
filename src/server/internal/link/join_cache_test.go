package link_test

import "testing"

func TestJoinSuccessAndReplaySendNoStore(t *testing.T) {
	e := newEnv(t)
	_, token := e.newToken(t)
	key := keyOf(0x31)

	first, err := e.connect.Join(t.Context(), request(token, key, "box"))
	if err != nil {
		t.Fatal("The first Join failed.")
	}
	if first.Header().Get("Cache-Control") != "no-store" {
		t.Error("The first Join must send Cache-Control with the value no-store.")
	}

	replay, err := e.connect.Join(t.Context(), request(token, key, "other"))
	if err != nil {
		t.Fatal("The Join replay failed.")
	}
	if replay.Msg.GetMachineId() == "" || replay.Msg.GetMachineId() != first.Msg.GetMachineId() {
		t.Error("The Join replay must return the original machine identifier.")
	}
	if replay.Header().Get("Cache-Control") != "no-store" {
		t.Error("The Join replay must send Cache-Control with the value no-store.")
	}
}
