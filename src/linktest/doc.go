// Package linktest exercises the client and server together through their public
// test entry points, with a disposable PostgreSQL database.
//
// The combined checks still to come are TestLinkCommandAttemptsAcrossReconnect,
// TestLinkRestartReplaysAfterDroppedResult, TestLinkSameStreamApproval,
// TestLinkScanDoesNotDelayRestart, TestLinkCommandProcess, and busy scan expiry.
// They require the server's stored commands.
package linktest
