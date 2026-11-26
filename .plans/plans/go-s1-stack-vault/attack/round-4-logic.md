# Round 4 review of the PostgreSQL and OpenBao plan, seat logic

Task identifier: review-go-s1-stack-vault-r4-logic. Nonce: 413cc4. Finding prefix: logic-r4.

The review reads `plan.md` and `decisions.md` as committed at `11ee059d`. It attacks only the patches made since `040b6ce0`, and what follows from them. The working tree of the plan folder has no uncommitted change to those two files.

## Verdict

REVISE

## Top finding

Item 4 requires the setup job to run in a container that has only the vault network and three mounts, and in the same sentence requires it to parse JavaScript Object Notation (JSON) with the host's `jq`, so a builder cannot follow both rules, and every way of resolving the conflict either breaks a secrecy rule or an ownership rule.

## Findings

### logic-r4 1

- Severity: MAJOR.
- Tied statement: D10 (OpenBao is a service of the stack beside PostgreSQL), with R9 (secrets never reach a log, the board or the website).
- Evidence: `plan.md:198`, `plan.md:158`, `plan.md:237`, `plan.md:204`, `plan.md:205`, `decisions.md:44` (row 37), and `scouting.md:245`.
- What it is: Patch architect-r3 4 removed the helper image and told item 4 to "use the host's existing `jq` prerequisite for structured output" (`plan.md:198`). The same paragraph says the job runs from the daemon's pinned image, joins only `vault`, and mounts only `admin/`, `server/` and the policy file. The process table says the job runs as user 0 with `CHOWN` and `DAC_OVERRIDE` (`plan.md:158`). Row 37 of the decisions repeats the host `jq` as an assumption that is still "unanswered". The plan never says that the pinned image carries `jq`, and `scouting.md:245` and `scouting.md:279` record only that the `bao` command line offers structured output.
- Failure scenario: There are two readings, and both fail.
  - Reading A: the host script parses. The host user is not root, so it cannot create `admin/init-response.json` owned by 10002 at mode `0400`, or `server/openbao.json` owned by 10001. The alternative is to pass the root token, the identifier and the response through host command-line arguments or environment values, which the same sentence forbids.
  - Reading B: the container script parses. The image is Alpine-based with the Busybox tools, so there is no `jq` unless the script runs `apk add jq` at start. That is an unpinned network install, which is the mechanism that architect-r3 4 removed, and it makes the first start on a clean machine depend on the package mirror. The other route is `sed` on pretty-printed JSON, which no assertion checks.
  - The listed test `vault_publishes_scoped_credential` passes under every one of these implementations, because it checks modes, fields and denials and never checks where parsing happens. The cheapest wrong implementation is the `apk add` line. Only PB1 on a machine without a package mirror would show it.
- Fix: Write down where each of the three parse steps runs (the root token from the init response, the `role_id`, and the `secret_id`). Prescribe the in-container route with no extra tool: every single value comes from `bao ... -field=<name>` or `bao ... -format=json -field=<name>`, the init response is stored as the raw output of `bao operator init -format=json`, and the only reading of the saved file is a fixed-shape extraction of the one `root_token` string by `sed` inside the job. Delete the sentence about the host `jq` from item 4 and row 37, and keep `jq` in `deploy/README.md` only if a host script of item 6 still reads JSON. Add to `vault_publishes_scoped_credential` a run of the job with no `jq` on its `PATH` and no network, and a truncated saved response that must give `vault is initialized but bootstrap state is incomplete`.

### logic-r4 2

- Severity: MAJOR.
- Tied statement: D10.
- Evidence: `plan.md:208`, `plan.md:207`, `plan.md:211`.
- What it is: Patch logic-r3 4 says to "issue one identifier only when none exists for the role" and that "a read or authorization error never counts as absence". Listing the identifiers of an AppRole role (`LIST auth/approle/role/lgtm-server/secret-id`) answers with HTTP 404 and an empty error list when the role has no identifiers. The command line prints "No value found" and exits with status 2. This is the documented behaviour of every list call in the Vault family. I could not run the pinned daemon in this review, because no image is present on the machine, so the first builder test must confirm it.
- Truth table, the answer of the list call against the plan's rules:

| Role state | Answer of the list call | Count the plan needs | Literal reading of "an error never counts as absence" |
|---|---|---|---|
| Role applied, no identifier yet (every clean first start) | 404, no keys | 0, issue one | Not counted as zero, so the job never issues the first identifier. |
| Role applied, one identifier from an interrupted run | 200, one accessor | 1, report incomplete setup | Correct. |
| Role missing because a mount or role call failed | 404 or an error body | Unknown, stop | Correct only if the job also reads the role itself first. |
| Token denied | 403 | Unknown, stop | Correct. |

- Failure scenario: A builder who follows the sentence literally gets a first start that stops on every clean machine, and C1 fails. The test shows this at once, and the cheapest repair is to treat any 404 as zero. That repair also treats a failed role creation as zero, and the job then issues an identifier against a missing role or reports a misleading refusal, so the plan has made the builder choose between two wrong readings.
- Fix: Add the order and the rule to item 4. First apply the role, then read the role itself and require success. Only after that read, a 404 with no keys from the list call counts as zero identifiers, and any other status stops with `incomplete setup` or the authorization diagnostic. Add the first-start case (role applied, zero identifiers) to the named test, next to the existing case with one identifier.

### logic-r4 3

- Severity: MAJOR.
- Tied statement: plan (the contract for sprint 2 and the writer and reader list of every removed name).
- Evidence: `plan.md:224`, `plan.md:221`, `decisions.md:45` (row 38), and in the neighbouring plan `.plans/plans/go-s2-stack-web/plan.md:101`, `.plans/plans/go-s2-stack-web/plan.md:303`, `.plans/plans/go-s2-stack-web/plan.md:315`, `.plans/plans/go-s2-stack-web/plan.md:318`, `.plans/plans/go-s2-stack-web/plan.md:542`, `.plans/plans/go-s2-stack-web/decisions.md:33` (row 24) and `.plans/plans/go-s2-stack-web/decisions.md:34` (row 25).
- What it is: The ruling of 2026-11-26 removed names that the Stack web plan still builds against, and the patch relays only two things: the five `LGTM_POSTGRES_*` settings and the `--until vault` option. The names with a live reader that the relay does not carry are:
  - `deploy/test/vault-testlib.sh` with `fixture_create` and `fixture_cleanup`. The web plan requires every test that needs a stack to use them (`go-s2-stack-web/plan.md:101`). Nothing in this plan writes them, and items 3, 4 and 6 each say only "ordinary disposable Compose projects".
  - The launcher option `--reset --yes`. The web plan extends it and tests it with `up_reset_removes_web_and_server` (`:303`, `:318`). This plan now documents a manual `down -v` and no reset.
  - "Sprint 1 check PB5: no server starts before the vault's setup succeeds" (`:315`, `:542`). PB5 of this plan is now the database rollout hook check, and no Stack check covers the sentence.
  - Row 24 of the web decisions assigns the C5 default change to the owner of the check script in sprint 3, while `plan.md:221` gives the whole `psql_in_stack` change to item 2. Row 25 still names `LGTM_POSTGRES_URL`.
- Failure scenario: The web builder starts item 4 of its plan, finds no `vault-testlib.sh`, and either writes its own, which duplicates the fixture of items 3, 4 and 6 of this plan with different project and state-folder rules, or stops. The same builder writes `up_reset_removes_web_and_server` against a flag that does not exist, and the test cannot pass.
- Fix: Add one row to the contract table that lists the removed names, namely `vault-testlib.sh`, `--reset --yes`, the old PB5 and the `LGTM_POSTGRES_URL` setting, and says for each what the web plan does instead. Decide who writes the shared disposable-project helper. The simplest choice is that item 3 writes a small `deploy/test/vault-testlib.sh` with `fixture_create` and `fixture_cleanup` that items 3, 4 and 6 and the web plan all use, which also removes the unnamed duplication across `seal.sh`, `vault-init.sh` and `stack-vault.sh`. Extend the relay request of row 38 to carry the whole list, and add a sentence that the web plan's reset test and its PB5 trace row are dropped or rewritten.

## Answers to the seven questions of the task file

1. Patches against their findings: the cuts of architect-r3 1 to 3 remove their failures, including the dead end of a stopped stack with a rotation journal (architect-r3 6). The patches for architect-r3 4 and logic-r3 4 each leave a new defect, in logic-r4 1 and logic-r4 2. The patch for architect-r3 8 and logic-r3 10 is incomplete, as logic-r4 3 shows.
2. `UNREVIEWED MECHANISM`: the plan carries no marked place after the cuts. The one mechanism that is new in substance is the two-fact bootstrap of item 4, and logic-r4 1 and logic-r4 2 attack it.
3. Rejected findings: the register has none with the outcome "rejected". All 18 findings are "fixed", and I checked the cuts by reading the text. The evidence of the cuts is real, and the removed names are gone from `plan.md` and from `decisions.md` except for the contexts that record the ruling.
4. Writers and readers: complete for the five database settings, the published credential, the signing-key path and `--until vault`. Incomplete for the removed names, as logic-r4 3 lists. The names of item 8 (`lgtm_store_test`, `LGTM_TEST_POSTGRES`, `mutate-feed-lock.sh`) match `deploy/test/checks.sh:11`, `deploy/test/checks.sh:590` and the Server store plan.
5. Contradictions between patches: yes, one, which is the host `jq` against the isolated job (logic-r4 1). The seal guard and the item 4 refusals do not contradict, and the truth table below has no empty cell.
6. Round 3 blockers and majors still open: none open as written. D10 is no longer stuck because the mechanisms were removed, and the two new defects above sit in the retained first-start path of D10.
7. Lens questions touched by the patches: answered below.

## Truth table of the retained first-start path

Columns are the facts that the seal guard and item 4 read. The initialization state comes from the daemon, and the three files come from the state folder.

| Vault initialized | Response file valid | Published file valid | Identifier count | Outcome of the plan | Check |
|---|---|---|---|---|---|
| No | Absent | Absent | Not read | Initialize, save the response, provision, publish. | Correct. |
| No | Present or published present | Any | Not read | `state directory is ahead of the database`, state preserved. | Correct. |
| Yes | Absent or invalid | Any | Not read | `vault is initialized but bootstrap state is incomplete`. | Correct, but it also refuses a stack whose published file works. The manual cleanup then loses data. A minor point, outside round 4. |
| Yes | Valid | Absent | 0 | Provision and issue. | Reaches this row only if logic-r4 2 is fixed. |
| Yes | Valid | Absent | 1 or more | `incomplete setup`, evidence preserved. | Correct as ruled. |
| Yes | Valid | Valid | Not read | One validation login, nothing changed. | Correct. |
| Yes | Valid | Invalid | Not read | `server vault credential needs repair`. | Correct, and it makes the web plan's `up_starts_no_server_before_the_vault` possible. |

## Answers to the eight questions of the lens file

1. Ordering, counters and restarts: the order is database, seal, vault, ready, and each phase runs once under the state lock. The only new ordering defect is the list-before-role order of logic-r4 2. A first start that crashes between the issuance and the rename ends in the ruled manual cleanup. The one avoidable trap is the validation login: the job still holds the identifier in memory when that login fails, so a retry inside the same run would avoid the wipe. It is a refinement and not a finding.
2. Failure permits or denies, and repair: an ambiguous issuance denies and preserves, which is the ruled behaviour. A missing or invalid response denies. A missing key with any retained fact denies. Repair is manual for the first install and a restore for an installation with data.
3. Truth tables: built above for the first-start path, and in logic-r4 2 for the list answers.
4. Zero values: the zero-identifier case is the defect of logic-r4 2. The empty `rows` case of the seal guard is covered by the key-missing rule.
5. Two actors on the same state: two launchers serialize under the exclusive `flock` of item 6 (`plan.md:228`). The word "existing" in that sentence has no earlier definition in the plan, so item 6 creates the lock and the sentence should say "take". It does not change the behaviour.
6. Risky items: item 4 names a mutation (broaden the policy, or publish an invalid identifier) that fails the denial or login assertion. Nothing named mutates the parse step, which is why logic-r4 1 asks for a run without `jq`.
7. Cheapest wrong implementation that passes the listed tests: for item 4, installing `jq` at run time (logic-r4 1), or treating every 404 of the list as zero (logic-r4 2). For item 6, a launcher that skips the lock passes unless the concurrency test runs.
8. Shapes that make wrong states impossible: the two-fact bootstrap is a good shape, because the second fact is the file name itself. The plan would be stronger if the zero-count rule were a function of the role read, as logic-r4 2 prescribes.

## Post-build checks

- PB-A (new, for logic-r4 1). Run the setup job in the pinned image with no network and no `jq`. Require one published credential. Mutation: make the script call `jq`, and require the run to fail.
- PB-B (new, for logic-r4 2). On a clean project, apply the role and list its identifiers with the pinned command line, and record the status. Require the job to issue exactly one identifier. Mutation: make the job count a 404 as an error, and require the first start to fail.
- PB-C (for logic-r4 3). With the web plan's launcher merged, run its `up_starts_everything_in_order` and `up_until_vault_stops_early` against this plan's launcher and require the fixture helper to come from the file that this plan names.
- Keep PB1 to PB5 of the plan as written.

## Grade

6 of 10. Blockers: 0. Majors: 3. Minors: 0 (none reported in round 4).
