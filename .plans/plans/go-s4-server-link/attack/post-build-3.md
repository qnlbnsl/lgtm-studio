# SHIP

The frozen admission tests accept a server that uses one machine's staged command result to refuse a different machine's valid run.

The test writers, the builder and this reviewer are OpenAI workers. I reviewed item 3 on `sprint-0` at `71896c43`. The main test freeze is `e89db2dd`, immediately before the assigned diff's lower endpoint. Task `postbuild-go-s4-server-link-3`. Nonce `dee076`. [Worker and commit record](../../../../.foundry/go-rewrite/run-log.md#L591) · [Fixture handoff](../../../../.foundry/go-rewrite/run-log.md#L604) · [Review pin](../../../../.foundry/go-rewrite/run-log.md#L610)

## post-3 1 · MINOR · The staged watermark test uses one machine

**Statement.** The plan applies the staged watermark to the authenticated machine's admission. The built query filters by that machine. `StagedWatermark` seeds both staged results on the same machine that attempts admission, so it exercises the largest watermark and the equality fence. Both frozen item 3 tests passed when the scratch query took the largest watermark across all machines. [Plan](../plan.md#L179) · [Built query](../../../../src/server/internal/link/admission.go#L61) · [Frozen staged test](../../../../src/server/internal/link/admission_v2_test.go#L284)

**Failure scenario and fix.** Machine A has a staged result at event 8. Machine B has an empty history and opens with first event 1. The candidate build welcomed B and saved its run. The mutated build returned `invalid_argument`. Add this second signed peer to the staged test and require B's Welcome and saved pair while A's stage is still present. Keep A's existing refusal at 8 and success at 9. [Contract admission and readiness rules](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L133)

**Grade** 9/10.

## Post-build scenarios

The round 1 register identifies the Projects refresh after bulk removal as the unreviewed mechanism. It belongs to item 6. The post-build privacy scenario applies here to malformed or rejected run data. On Connect and gRPC, the frozen test sent a synthetic credential marker with a bad signature and with valid proof. It checked the fixed terminal sentence and metadata, unchanged durable rows and feed head, and absence of the marker from the Link logs. Both transport cases passed. Private file modes, certificate pins and replay with the installed client belong to the stack and paired checks. [Register](round-1-findings.md#L55) · [Post-build scenarios](../plan.md#L337) · [Privacy cases](../../../../src/server/internal/link/admission_v2_test.go#L50) · [Privacy assertions](../../../../src/server/internal/link/admission_v2_test.go#L125)

## Item 3 acceptance

- By reading, the server sends the signed Challenge before the registry and revision decisions, then verifies the machine signature before passing the announced pair into the admission transaction. The Welcome uses revision 2. The production and scripted proof text retain the original bytes. Confirmed live, both transports returned the exact revision 1 refusal after a verified Challenge. A scratch control withheld Proof and still received that refusal on both transports. The frozen bad-proof and malformed-history cases passed with their expected codes and unchanged database state. [Handshake](../../../../src/server/internal/link/handshake.go#L149) · [Status and trailer](../../../../src/server/internal/link/handshake.go#L353) · [Independent proof](../../../../src/server/internal/link/machinetest/machine.go#L144) · [Transport cases](../../../../src/server/internal/link/admission_v2_test.go#L31)
- By reading, admission uses the machine row lock and validates the canonical version 7 universally unique identifier and first number before converting the replay boundary to a signed integer. It checks known run identity and the saved pair before the numerical fences. A refused candidate leaves the incumbent and command ledger in place. Confirmed live, PostgreSQL preserved the admitted pair across restart, refused superseded runs on equal boundaries, accepted the upper bound and declined invalid numbers. The incumbent answered a heartbeat after a pending-boundary refusal. A scratch control started with a stale connection for a pending run. Startup cleared the connection and the same run reconnected with its saved boundary. [Classifier](../../../../src/server/internal/link/admission.go#L29) · [Machine lock](../../../../src/server/internal/link/presence.go#L53) · [Frozen restart cases](../../../../src/server/internal/link/admission_v2_test.go#L198) · [Startup cleanup](../../../../src/server/internal/link/presence.go#L229)
- By reading, the admission write installs the run, generation and connection, then calls the admission hook before the first readiness hook. The hub checks publication before Welcome and retains the genuine unknown-commit fallback. Confirmed live, the two hooks shared the committed write, a failing readiness hook rolled back admission and its feed entry, and reconnecting the ready run repeated the admission hook. The scripted peer used its initial pair after its mark advanced and accepted an explicitly supplied new pair on the next process simulation. [Admission write](../../../../src/server/internal/link/presence.go#L77) · [Hub publication](../../../../src/server/internal/link/hub.go#L99) · [Callback assertions](../../../../src/server/internal/link/admission_v2_test.go#L313) · [Scripted peer test](../../../../src/server/internal/link/admission_v2_test.go#L150)

These results cover the item 3 “Done when” row. The builder's handoff names the later ownership of recovery diagnostics, replay-time promotion and staged settlement. The final commit contains the Machines and application fixture adaptations the handoff listed. [Acceptance row](../plan.md#L159) · [Builder handoff](../../../../.foundry/go-rewrite/results/s4-link-3-build.md#L17)

## Inherited sprint 3 assertions

I compared the original Link fixture changes in `e89db2dd` and the Machines and application changes in `8acd8fd6..71896c43` against the plan's fixture list. The Link tests continue to check proof vectors, timeout and refusal precedence, stale-stream fencing, PostgreSQL commit failures and exact-stream panic cleanup. The Machines tests continue to check projection and queue behavior. The application tests still verify the Challenge and check shutdown while holding the same admission or command work. The revision-aware inherited tests derive some revision expectations from the server constant, giving those assertions less independence. The frozen revision 2 suite separately pins the accepted revision, refused revision, exact error and trailer on both transports. I found no lost behavioral assertion across the combined suite. [Fixture list](../plan.md#L299) · [Decision order](../../../../src/server/internal/link/handshake_test.go#L647) · [Direct Link fixture](../../../../src/server/internal/link/presence_test.go#L92) · [Machines fixture](../../../../src/server/internal/machines/stream_projection_test.go#L153) · [Application handshake](../../../../src/server/internal/app/link_stream_test.go#L90) · [Application command peer](../../../../src/server/internal/app/review_commands_test.go#L449)

## PostgreSQL mutation results

Each mutation used a separate Go overlay in one scratch copy of `71896c43`. The three planned mutations failed on assertions. [Plan](../plan.md#L184)

| Scratch change | Result |
|---|---|
| Check the committed mark before verifying Proof. | Exit 1. Connect and gRPC both printed `Open ended with code invalid_argument, want unauthenticated.` |
| Drop the stored pair and run history on disconnect. | Exit 1. The pending restart printed `The durable admitted pair or ready run did not match the expected history.` |
| Compare boundaries in place of latest admitted run identity. | Exit 1. Both equal-boundary cases printed `The stream sent another frame where its terminal status was expected.` |
| Take the maximum staged watermark across machines. | Exit 0 for the two frozen tests. The scratch two-machine control passed against the build and failed against the mutation with `The stream did not receive Welcome. The receive code was invalid_argument.` |
| Drop the pending run when startup clears a stale connection. | Exit 1. The frozen restart and the explicit stale-connection control both caught the lost pair. |

## PostgreSQL verification at the reviewed commit

The named tests ran against a disposable PostgreSQL database in the archived commit. Both top-level tests and all 34 nested cases passed, with zero skips. The refusal probe that withheld Proof also checked that the exact value appeared once in the response trailer and was absent from the response header. Both transport cases passed.

```sh
bash deploy/test/with-postgres.sh -- go test -v ./src/server/internal/link \
  -run '^(TestRevisionTwoProofAndRunAdmission|TestRunAdmissionPersistsAcrossRestart)$' -count=1
```

```text
--- PASS: TestRevisionTwoProofAndRunAdmission (11.48s)
--- PASS: TestRunAdmissionPersistsAcrossRestart (2.23s)
PASS
ok   github.com/qnlbnsl/lgtm-studio/src/server/internal/link   13.732s
```

The five-package checks used the plan's command and the common-build exclusion for the expected-red API and settlement cases. The four Projects router names are filtered. The two settlement tests were added after the reviewed commit. The plan assigns the paired and Compose checks to later integration. [Plan package gate](../plan.md#L321) · [Expected failures](../../../../.foundry/go-rewrite/common-build.md#L41)

```sh
bash deploy/test/with-postgres.sh -- go test -v \
  ./src/server/internal/link ./src/server/internal/machines ./src/server/internal/api \
  ./src/server/internal/app ./src/server/servertest -count=1 \
  -skip '^(TestProjectRouter(Preflight|MalformedJSON|Discovery|ReadLimit)|TestStagedResultSurvivesRestartAndSweep|TestResultFenceKeepsAttemptAndCommandRules)$'
```

The standard, race-detector and full-watermark-mutation runs each printed `ok` for all five packages. Each run passed the same 195 top-level tests and 593 nested cases, with zero skips. The race run emitted no race diagnostics. The shell rejected the exit-code variable used after the race and mutation commands finished. The completed Go output provides the package and test results below.

| Package | Standard `ok` time | With `-race` `ok` time | With the global watermark mutation `ok` time |
|---|---:|---:|---:|
| `src/server/internal/link` | 405.809s | 522.957s | 387.559s |
| `src/server/internal/machines` | 207.265s | 381.452s | 237.080s |
| `src/server/internal/api` | 54.122s | 121.980s | 84.256s |
| `src/server/internal/app` | 119.637s | 243.405s | 134.754s |
| `src/server/servertest` | 42.036s | 82.712s | 52.520s |
