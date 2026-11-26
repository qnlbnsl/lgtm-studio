# Handoff to the plans of the stream and the commands

The orchestrator cut the plan `go-s3-server-machines` in three on 2026-11-26 (decision 87).
This file lists what that plan knew about its parts B and C, for the planners who write them.
It is a list, not a design.
The reviewed text of every item is in `attack/plan-v4.md`, and the four registers are in `attack/`.

## The two future plans

- **Part B, the stream,** has the future slug `go-s3-server-stream`. It covers the link tables and admission, the hub, the handshake, the stream loop and incoming events, which were items 5, 7, 8, 9 and 10 of `attack/plan-v4.md`.
- **Part C, the commands,** has the future slug `go-s3-server-commands`. It covers stored commands and the command calls, which were items 6 and 13 of `attack/plan-v4.md`.
- Both are written after part A and sign-in item 9 are built, against real code and the frozen contract of the link. Each gets its own review.

## What parts B and C change in the files of part A

- `src/server/internal/link/service.go`: `Service` stops embedding the unimplemented `Open`, and `NewService` gains the hub, the state, the options and the seams.
- `src/server/internal/machines/store.go`: `Registry` gains `Lock`, `ApplyFacts`, `ApplyHarnesses`, `NoteSeen`, `Touch` and `Standing`, with the tests `TestApplyFactsAndHarnesses` and `TestLockHoldsOffRemoval`. The columns they write already exist.
- `src/server/internal/machines/project.go`: the four disjoint cases of presence return, with load only while online.
- `src/server/internal/machines/records.go`: `ApproveClient` and `RenameClient` store their `SetState` in the same write (part C, decision 89). `RevokeClientToken` and `RemoveClient` call `Link.Forget` (part B). `ScanClientHarnesses` and `ReconnectClient` enqueue their commands (part C).
- The application's start clears the connection rows under the instance lock, and starts and stops the link's background work. The test fixture gains the admission seams.
- The post-build checks P1 to P5, P8 and P9 of `attack/plan-v4.md`, and its check of an unknown field of a report, move with these parts.

## Decisions that were settled

The numbers are rows of `decisions.md`.

- **Admission (ruling 2 of round 3).** The commit of `BecomeCurrent` replaces the older stream and authorizes the new stream's first frame, and `State.Confirm` is removed. `Welcome` is sent after that commit. A `Welcome` authorized before a removal may arrive after it, and the next frame of that stream is refused (row 78).
- **Failed cleanup (ruling 3 of round 3).** An action that needs a live stream needs the connection row and this process's hold on that stream, and otherwise answers not connected. A refused delete may leave the machine shown online, which is an accepted degraded case of wave 1, retried until it succeeds and cleared at the next start (row 79).
- **One declaration per item (ruling 1 of round 3).** It was replaced by the split. Each future plan gives each item the names it declares (rows 77 and 88).
- **Locking.** Every write of one machine first locks the machine's row, and a token's row is locked before a machine's row (row 40).
- **Presence.** A connection row exists only for a current stream, and no wall clock decides presence (rows 41 and 65). A server that starts clears every row under the instance lock (row 41).
- **A waiting machine on the link.** It can connect and do nothing else (row 11).
- **Events.** Each event is decided by the contract's table, and its effect and its number commit together (rows 20 and 44). A number above 2^63 − 1 gives `ErrInvalidMessage` (row 86). The server stores no report time (row 68) and only the declared fields of a report (row 69).
- **The handshake.** The codes after `Challenge`, and the reason in the error's metadata (rows 42 and 43). No connection token is stored (row 45).
- **The stream's lifetime.** A silent or blocked stream ends within 30 seconds (row 55). Shutdown reaches a stream that has not registered (row 67). The first terminal cause wins (row 81). A lost reply of an admission's commit leads to a retried `Disconnect` of that stream (row 80).
- **The writer.** Each turn handles at most one received message and then at most one command (row 82).
- **Tests.** A test holds a real commit with a `pgx.QueryTracer` (row 70). The server's own tests use a scripted machine (row 36).
- **Commands.** A claim takes one command, the oldest by `seq` (rows 57 and 66). A result closes only the newest attempt on the current stream (rows 47 and 58). A `SetState` does not expire (row 46). Every final state also sets `finished_at`, and the first stored outcome wins (rows 71 and 83). A claim sends an action other than `set_state` only before `expires_at + CommandGrace` less one hour (row 83). At most once holds while the machine's clock is right (row 75). Only an approved machine whose stream this process holds may be scanned or restarted (rows 63 and 79).

## Open findings of round 4 for these parts

Each is open, and none was folded. The register is `attack/round-4-findings.md`.

- **Part B.**
  - architect-r4 02 and logic-r4 03, major, C8: `Hub.Holds` must check the exact stream identifier and a live context, and a terminal handle must stop counting at cancellation.
  - architect-r4 03 and logic-r4 02, major, C10: a superseded admission must not register after the newer handle left, and the first-frame rule must match the contract's check 10.
  - architect-r4 04 and logic-r4 06, major, C11: the combined outage gate needs a count of committed effects and an assigned change of `deploy/test/checks.sh`.
  - logic-r4 04, major, C8: a claim can walk past an unbounded prefix of ineligible rows under the machine's lock.
  - logic-r4 07, major, C9: a held cleanup retry can delay the shutdown cause of other streams.
- **Part C.**
  - architect-r4 05, major, R6: the contract's post-build check 11 requires a retry after the server's new 23-hour window.
  - logic-r4 05, major, R6: a claimed command can still be sent after the sweep finalized it.
- **Post-build checks that the reviewers proposed** for the stream are in the sections "Post-build checks" of `attack/round-4-architect.md` and `attack/round-4-logic.md`.
- **The skeleton's leftovers of architect-r4 01 and logic-r4 01** (the service's lifetime context, the handoff of cleanup before `Run` exists, and the typed signature of `NewState`) apply to part B's own items.

## Requests to other plans

- **`go-s0-contract`:** the sentence on a `Welcome` authorized before a removal, for checks 10, 16 and 23; the send window of 23 hours and the first stored outcome in `commands.proto` and its check 11; the limit of the machine's clock; the ceiling of event numbers; dropping the promise to show `happened_at`. The exact sentences are in the result file of this fold.
- **The Stack team and the orchestrator:** revise check C11 of `deploy/test/checks.sh`, its README and gate G4 of `go-s3-stack-client` to read `link_machines.event_mark` and keep the log for diagnosis.
- **`go-s1-server-store`:** confirm the test tracer that holds a commit, and offer a controlled authorization tick of the shared watch for tests of other packages.
- **`go-s3-client-link`:** state the same limit of at most once, and observe that its outbox is empty for the combined gate.
