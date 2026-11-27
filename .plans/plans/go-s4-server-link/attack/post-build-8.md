# SHIP

The fixture tests pass when the command send observer's error is discarded, allowing a claimed command to proceed to dispatch.

The frozen test writer and the builder were OpenAI workers, the same vendor as this reviewer. I reviewed commit `294f870a` on `sprint-0` for task `postbuild-go-s4-server-link-8`, nonce `5c41dd`, on 2026-11-27.

## Findings

### post-8 1 · MINOR · The send test accepts a discarded callback error

**Statement.** Plan item 8 requires a send observer error to stop the writer's turn and trigger cleanup. By reading, the stream forwards the error and releases the permit. The claimed-send test releases its hold with a successful return. Confirmed live, the full server fixture package passed when a scratch mutation called the observer and discarded its error. [Plan requirement](../plan.md#L289) · [Stream callback and permit release](../../../../src/server/internal/link/stream.go#L99) · [Claimed-send test](../../../../src/server/servertest/link_v2_test.go#L375)

**Failure scenario.** A paired fixture returns an error after observing the durable claim to interrupt delivery. The mutated writer still sends the command and can mark it dispatched.

**Fix.** Add a case where the observer returns a sentinel error while the stream is live. Assert that the stream closes before any command arrives, the dispatch marker stays false and a queued stream can use the released slot when all three permits are occupied.

### post-8 2 · MINOR · The consumed-event test leaves the pre-commit callback unchecked

**Statement.** Plan item 8 puts the pre-commit fault after report handling and before advancing the mark. By reading, the implementation calls the hook for the next numbered event even when report handling consumes it. The test checks the hook's count through the repeated Facts event, then checks the inventory event only at the acknowledgement hook. Confirmed live, the full server fixture package passed with a scratch mutation bypassing the pre-commit callback for inventory reports. [Plan requirement](../plan.md#L287) · [Event write](../../../../src/server/internal/link/inbox.go#L53) · [Event assertions](../../../../src/server/servertest/link_v2_test.go#L311)

**Failure scenario.** A fixture injects a fault on the last inventory event replayed by a waiting machine. The mutated writer commits and acknowledges the new mark. The next Welcome then tells the machine to resume past the event whose write the fixture meant to interrupt.

**Fix.** Fault the consumed inventory event at its pre-commit callback. Assert that the old mark survives without an acknowledgement. Retry, approve while the acknowledgement is held, and require the acknowledgement to arrive before the standing command.

## Post-build scenarios and plan fit

By reading, the only mechanism the review register marks as unreviewed is the large-removal refresh owned by Projects. The plan assigns the duplicate route scenario to Projects startup and assigns the stream identity recheck to Link item 5. The reviewed server still runs revision 1. Projects rollback, all three occupied send permits, the replay credit limit and the revision 2 rerun remain in the integrated gate. [Review register](round-1-findings.md#L55) · [Item 8 boundary](../plan.md#L282) · [Post-build scenarios](../plan.md#L337) · [Runtime revision](../../../../src/server/internal/link/handshake.go#L21)

Confirmed live on revision 1. The pre-commit Facts fault rolled back the write and mark with no acknowledgement. Restart returned the old mark, and retry committed once. The inherited fixture cut a stream during the committed acknowledgement hold and verified the durable mark and repeat after restart. The replacement test checked both registry lock orders and the resulting Welcome marks. A consumed inventory report advanced the mark without changing inventory. Its acknowledgement preceded the standing command after approval. The application log recorded each committed advance once. The held command claim was visible and independently lockable before dispatch. Both launches used their configured PostgreSQL pool and reached Link migration 3. The longer-timeout client retained its signed-in session. [Event and send fixture](../../../../src/server/servertest/link_v2_test.go#L196) · [Interrupted acknowledgement](../../../../src/server/servertest/link_stream_test.go#L218) · [Replacement test](../../../../src/server/internal/link/stream_test.go#L403) · [Pool fixture](../../../../src/server/servertest/link_v2_test.go#L56)

By reading, the normal application start supplies empty observers. Event faults use the store rollback path, and command faults use the existing stream cleanup and fixed public error mapping. The fixture writes its database password to a test temporary directory with mode `0400`. [Foundation rules 4, 8 and 9](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L365) · [App start](../../../../src/server/internal/app/start.go#L104) · [Store rollback](../../../../src/server/internal/store/pool.go#L320) · [Error mapping](../../../../src/server/internal/link/handshake.go#L351) · [Password file](../../../../src/server/servertest/server.go#L81)

## PostgreSQL commands and output

Confirmed live. I ran the two plan commands from one scratch archive of the reviewed commit, with a writable Go build cache in that directory. [Verification command](../plan.md#L321)

```sh
bash deploy/test/with-postgres.sh -- go test -v ./src/server/internal/link ./src/server/internal/machines ./src/server/internal/api ./src/server/internal/app ./src/server/servertest -count=1
bash deploy/test/with-postgres.sh -- go test -race -v ./src/server/internal/link ./src/server/internal/machines ./src/server/internal/api ./src/server/internal/app ./src/server/servertest -count=1
```

| Package | Regular output | Race detector output |
| --- | --- | --- |
| `src/server/internal/link` | `FAIL …/link 336.263s` | `FAIL …/link 468.585s` |
| `src/server/internal/machines` | `ok …/machines 211.123s` | `ok …/machines 381.766s` |
| `src/server/internal/api` | `FAIL …/api 56.487s` | `FAIL …/api 148.654s` |
| `src/server/internal/app` | `ok …/app 116.451s` | `ok …/app 247.558s` |
| `src/server/servertest` | `ok …/servertest 38.930s` | `ok …/servertest 88.434s` |

Both full commands exited 1 on Link item 1 and Projects router tests outside the assignment. All named item 8 tests passed in both runs with zero skips. The race detector emitted zero race warnings. [Builder's record of the earlier package failures](../../../../.foundry/go-rewrite/results/s4-link-8-build.md#L45)

The separate focused run explicitly passed `TestServerLinkRevisionTwoWiring` and all three of its subtests, `TestLinkSeamsReachTheService`, and both lock orders of `TestReplacementWelcomeCarriesCommittedMark`. It printed `ok github.com/qnlbnsl/lgtm-studio/src/server/internal/link 5.860s` and `ok github.com/qnlbnsl/lgtm-studio/src/server/servertest 28.809s`, with zero skips. The same tests passed in the focused race run, which printed `ok github.com/qnlbnsl/lgtm-studio/src/server/internal/link 9.935s` and `ok github.com/qnlbnsl/lgtm-studio/src/server/servertest 46.257s`.

## Required mutations

Confirmed live. Each mutation used a separate Go source overlay in the same scratch archive. Each focused test exited 1 and printed the expected assertion. [Mutation requirements](../plan.md#L295)

| Mutation | Assertion printed |
| --- | --- |
| Detach the send observer. | `link_v2_test.go:395` reported that the stream answered before the callback held the claimed send. |
| Move the event fault after commit. | `link_v2_test.go:277` reported that the callback ran outside the interval between the Facts and mark writes. |
| Bypass the configured pool on restart. | `link_v2_test.go:89` reported that launch 2 did not use its configured pool for sign-in. |

The two additional mutations described in the findings each passed the full server fixture package. The send-error mutation printed `ok github.com/qnlbnsl/lgtm-studio/src/server/servertest 33.080s`. The inventory mutation printed `ok github.com/qnlbnsl/lgtm-studio/src/server/servertest 33.752s`.

**Grade.** 8 out of 10.
