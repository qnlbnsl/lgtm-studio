// Package linktest exercises the client and server together through their public
// test entry points, with a disposable PostgreSQL database.
//
// The command process scenario remains deferred. TestLinkCommandProcess would
// need a public fixture that launches the actual client command and exposes its
// operating system exit status. The clienttest fixture runs the application in
// process, and the command package's subprocess entry is private to its tests.
package linktest
