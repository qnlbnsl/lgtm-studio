package contracts_test

import "testing"

func TestReadinessCallbackRunsInTheReadinessTransaction(t *testing.T) {
	open := normalize(commentBefore(t, readLines(t, "link.proto"), "rpc Open("))
	requireWrittenRules(t, "LinkService.Open", open, [][2]string{
		{"that the readiness callback runs within the transaction that makes the run ready",
			`\. the server calls the readiness callback within the transaction only on a run's first transition to ready\.`},
	})
}

func TestAdmissionReadinessIncludesReconnectAfterRestart(t *testing.T) {
	open := normalize(commentBefore(t, readLines(t, "link.proto"), "rpc Open("))
	// The callback statement follows the readiness rule directly, leaving no intervening exception.
	requireWrittenRules(t, "LinkService.Open", open, [][2]string{
		{"that the inclusive readiness threshold at admission also applies on reconnect after restart without an exception",
			`\. the latest admitted run is ready at admission if the committed mark is at or past ` +
				`first_event_number\s*-\s*1, including on a reconnect after a server restart\. the server calls the readiness callback\b`},
	})
}
