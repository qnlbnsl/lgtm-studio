# REVISE

A signed folder report can record several spellings of one checkout as successful resolved paths, which would give the planned exact-path attachment lookup different keys for the same folder.

OpenAI supplied this reviewer, the test writers and the builders. I wrote neither the implementation nor the frozen tests. I reviewed Projects items 3 and 4 on `sprint-0` through commit `6c88bce1`, using the range `24d72c59~1..6c88bce1`. The task identifier is `postbuild-go-s4-server-projects-4` and the nonce is `ee433c`.

## Findings

### post-4 1 · MAJOR · Folder reports accept unresolved path spellings

Sprint 4 checks R2 and R3 require the resolved checkout path to identify one folder on one machine. Plan item 4 validates the report before passing it to attachment, and item 5 looks up the reported path by exact spelling. [Sprint 4 checks](../../../../docs/specs/2026-11-27-running-an-agent-design.md#L292) · [Folder report contract](../../../../src/contracts/proto/lgtm/link/v1/folders.proto#L49) · [Plan item 4](../plan.md#L144) · [Plan item 5](../plan.md#L188)

By reading, Link and Projects validate absoluteness and the base name. Projects then copies the path verbatim into the response or attachment input. Confirmed live, the signed PostgreSQL fixture acknowledged five reports and stored five successful Inspect replies with the exact paths `/workspace/checkout`, `/workspace//checkout`, `/workspace/./checkout`, `/workspace/subfolder/../checkout` and `/workspace/checkout/`. [Link validation](../../../../src/server/internal/link/inbox.go#L162) · [Projects validation](../../../../src/server/internal/projects/reports.go#L123) · [Attachment call](../../../../src/server/internal/projects/reports.go#L67)

If one machine reports the first two spellings in separate Add operations, item 5's exact-path lookup would treat them as different folders. Require the reported absolute path to equal its lexically cleaned form and consume any other spelling without changing the operation. Cleaning a path that still contains a parent component can change its meaning if a preceding directory is a symbolic link.

### post-4 2 · MINOR · The same nondefault port produces different comparison keys

Plan items 4 and 5 require normalized origin comparison. The numeric port is part of the destination. [Report validation requirement](../plan.md#L144) · [Origin comparison grammar](../plan.md#L190) · [Projects contract](../../../../src/contracts/proto/lgtm/v1/projects.proto#L37)

By reading, Projects parses the port as a number for validation and default-port removal, then appends the original digits for a nondefault port. Confirmed live, OpenSSH resolves both `02222` and `2222` to port `2222`, and the Projects parser accepts both but emits different comparison keys for the same host and path. The probe also confirmed scheme and host case folding, default-port removal, and equivalence between ordinary absolute Secure Shell (SSH) shorthand and its Uniform Resource Locator (URL) form. [Port parsing](../../../../src/server/internal/projects/origins.go#L72)

Two checkouts using the same SSH host and path on port 2222 could receive a repository mismatch solely because one spelling has a leading zero. Format the parsed port in decimal when constructing the comparison key. Keep the credential-free spelling in the Inspect reply.

### post-4 3 · MINOR · The frozen tests accept a parser that returns the original origin text

Plan item 4 supplies the comparison helper for item 5. The frozen Inspect tests check acceptance and validity, and allow the original spelling for absolute SSH shorthand. The report handler likewise checks the validity flag and keeps the supplied spelling for Inspect. Confirmed live, replacing the normalized return value with the input, while preserving all validation, passed all five frozen item 3 and 4 groups in `29.547s`. [Helper requirement](../plan.md#L144) · [Report translation](../../../../src/server/internal/projects/reports.go#L134) · [Shorthand assertion](../../../../src/server/internal/projects/reports_test.go#L484) · [Mutated return](../../../../src/server/internal/projects/origins.go#L86)

That implementation would give different comparison keys to a mixed-case host and its lowercase spelling, or to an explicit default port and an omitted port. Check comparison keys directly with equivalent and distinct fixtures, alongside item 5's attachment cases.

## Post-build scenarios and plan conformance

The plan and its round-one register have no unreviewed mechanism for items 3 or 4. The flagged mechanism concerns repository changes and refresh in items 5 through 7. The checks below cover the parts of the plan's uncertain commit, restart, removal and filtering scenarios available at the reviewed commit. [Round-one register](round-1-findings.md#L7) · [Flagged mechanism](../plan.md#L191) · [Post-build scenarios](../plan.md#L279)

- **Retirement and replay.** Confirmed live, ordinary pruning removed the unrelated receipt and preserved the open and retained Projects keys. Conflicting calls on those keys were refused. The first committed observation stamped the terminal operation, and 520 tied timestamps retired across the page boundary. When replay held the key first, it returned the original bytes after both rows disappeared. When retirement held it first, the retry followed the fresh offline rule, and a subsequent online retry created one new operation and command. Two retirement workers committed one deletion, and shutdown during a blocked delete preserved both rows. [Retention assertions](../../../../src/server/internal/projects/retire_test.go#L194) · [Page fixture](../../../../src/server/internal/projects/retire_test.go#L298) · [Replay wins](../../../../src/server/internal/projects/retire_test.go#L401) · [Retirement wins](../../../../src/server/internal/projects/retire_test.go#L488) · [Workers and shutdown](../../../../src/server/internal/projects/retire_test.go#L560)
- **Uncertain commit.** Confirmed live after reopening the same database, a lost reply to a committed retirement left neither row, and a lost reply to a rolled-back retirement left both rows and the original replay bytes. Faulting either delete preserved both rows before retry. A missing held receipt reported the fixed diagnostic. The plan gate also passed both inherited acceptance commit-loss cases using fixture replies. [Retirement faults](../../../../src/server/internal/projects/retire_test.go#L618) · [Commit-loss fixture](../../../../src/server/internal/projects/retire_test.go#L648) · [Invariant diagnostic](../../../../src/server/internal/projects/retire_test.go#L706) · [Acceptance fixture](../../../../src/server/internal/projects/operations_test.go#L807)
- **Report identity, settlement and rollback.** Confirmed live, crossed or wrong-machine reports advanced eligible marks without rewriting the operation. Valid Inspect facts returned their Protocol Buffers reply with no membership write. After reopening Store, the signed report won over the staged outcome in the same write. With outcome first, the late report only advanced the mark and received an acknowledgement. The tests observed all fixed report and outcome refusals, including removal. A pre-commit failure preserved the previous operation, mark and staged result, with no acknowledgement. Retry committed one result and the duplicate left it unchanged. A valid Add rolled back through the temporary attachment seam as planned. [Identity and facts](../../../../src/server/internal/projects/reports_test.go#L347) · [Outcome cases](../../../../src/server/internal/projects/reports_test.go#L537) · [Restart and ordering](../../../../src/server/internal/projects/reports_test.go#L648) · [Rollback](../../../../src/server/internal/projects/reports_test.go#L729) · [Attachment seam](../../../../src/server/internal/projects/reports.go#L155)
- **Filtering and exact replay.** Confirmed live in the signed server fixture, the credential-bearing origin advanced the mark, left the operation unchanged and stayed out of the captured Link log. In separate client fixtures, synthetic values in resolved paths and branches produced fixed refusals, and an unsafe origin was withheld before the event was stored or sent. The client filtering cases and the client interruption after the durable report both printed `PASS`. The inherited server restart cases preserved an exact typed path with trailing space and refused the same key when the space was removed. [Server origin fixture](../../../../src/server/internal/projects/reports_test.go#L389) · [Server no-write check](../../../../src/server/internal/projects/reports_test.go#L412) · [Client append filtering](../../../../src/client/internal/link/filter_test.go#L189) · [Client wire filtering](../../../../src/client/internal/link/folder_command_test.go#L941) · [Client interruption](../../../../src/client/internal/link/folder_command_test.go#L777) · [Exact-path replay](../../../../src/server/internal/projects/operations_test.go#L725)
- **Worker and transaction scope.** By reading, the worker waits an hour after a sweep, uses the database clock and stamps committed terminal rows in pages of 512. The retained day starts at the first stamp, as the plan specifies. Candidate reads carry no row lock. The delete rechecks maturity under Store's receipt lock, and Store commits both deletes in one write. The report handler matches all three identifiers on an open row in Link's write, where report application precedes the mark update and staged settlement. The hourly interval was checked in source. The test fixtures control the database clock and exercise startup sweeps. [Worker](../../../../src/server/internal/projects/retire.go#L25) · [Stamp and candidates](../../../../src/server/internal/projects/retire.go#L73) · [Atomic deletion](../../../../src/server/internal/projects/retire.go#L119) · [Store receipt lock](../../../../src/server/internal/store/receipts.go#L160) · [Report row match](../../../../src/server/internal/projects/reports.go#L49) · [Link ordering](../../../../src/server/internal/link/inbox.go#L68)

A mounted Projects service, actual inspection counts across a server restart, the Add attachment against removal race, and a combined client-to-browser filtering trace require the later attachment and startup work. The signed fixture exercises the local Projects callbacks using real PostgreSQL. The intentionally red router and Compose checks are outside the plan gate. Issue 1704 concerns item 2 and is already recorded. [Attachment assignment](../plan.md#L185) · [Startup assignment](../plan.md#L252) · [Paired checks](../plan.md#L267) · [Issue 1704](../../../../docs/issues.md#L8151)

## PostgreSQL results

Confirmed live, the plan command passed 32 top-level tests with zero skips in one scratch copy of `6c88bce1`. This includes all five named item 3 and 4 groups. [Plan command](../plan.md#L165)

```sh
bash deploy/test/with-postgres.sh -- go test -v ./src/server/internal/store ./src/server/internal/projects \
  -run '^Test(HeldReceipt|Migrate|Prune|WriteOnce|RecordValidationAndLimit|Project(Migration|Storage|Operation|Receipt|Replay|Report|FolderSettlement))' -count=1
```

```text
ok  github.com/qnlbnsl/lgtm-studio/src/server/internal/store     72.988s
--- PASS: TestProjectReportIdentityAndFacts (1.12s)
--- PASS: TestProjectFolderSettlement (4.90s)
--- PASS: TestProjectReportRollback (0.64s)
--- PASS: TestProjectReceiptRetention (3.71s)
--- PASS: TestProjectReplayRetirementRace (9.85s)
PASS
ok  github.com/qnlbnsl/lgtm-studio/src/server/internal/projects  61.910s
```

The full Projects package also passed with `-race -count=1 -p 1 -timeout 25m`. Go printed `PASS` and `ok` at `90.662s`.

## Mutation results

Each mutation used an isolated Go overlay in the same scratch copy with disposable PostgreSQL. Every planned mutation exited 1 on its matching assertion.

| Mutation | Observed assertion |
|---|---|
| Read after releasing the receipt lock. | Retirement deleted the operation before the admitted replay could read it. [Assertion](../../../../src/server/internal/projects/retire_test.go#L444) |
| Admit held receipts to ordinary pruning. | Ordinary pruning deleted a held key as well as the ordinary receipt. [Assertion](../../../../src/server/internal/projects/retire_test.go#L232) |
| Commit the operation delete separately from the receipt delete. | A receipt-delete fault left the operation and receipt in different committed states. [Assertion](../../../../src/server/internal/projects/retire_test.go#L635) |
| Omit the report's operation identifier match. | A crossed operation rewrote the row. [Assertion](../../../../src/server/internal/projects/reports_test.go#L418) |
| Omit the report's command identifier match. | A crossed command rewrote the row. [Assertion](../../../../src/server/internal/projects/reports_test.go#L418) |
| Omit the report's authenticated machine match. | A report from the other machine rewrote the row. [Assertion](../../../../src/server/internal/projects/reports_test.go#L358) |
| Let a late report overwrite a terminal operation. | The late report rewrote the row and replaced its fixed failure. [Assertion](../../../../src/server/internal/projects/reports_test.go#L691) |
| Let a command outcome overwrite a terminal operation. | The staged outcome replaced the successful reply. [Assertion](../../../../src/server/internal/projects/reports_test.go#L670) |
| Settle a staged outcome before applying its report. | The operation lost the successful reply in the shared write. [Assertion](../../../../src/server/internal/projects/reports_test.go#L670) |

**Grade.** 7 out of 10.
