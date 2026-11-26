# Round 3 logic review of PostgreSQL and OpenBao

- Task identifier: review-go-s1-stack-vault-r3-logic
- Nonce: 92c225
- Reviewer: lens `logic`, claude-sonnet-5-5, high effort.
- Scope: the patches between commits `cd46223f` and `040b6ce0`, and what follows from them.

## Verdict

REVISE.

## Top finding

logic-r3 1 is a major. The operator role, its credential file, the rotation journal and the revocation of the initial root token behind them are needed by no check from C1 to C13 and by no statement of the design. The plan itself says the operator file carries the authority of a standing administrator, so revoking root removes no authority from the machine. The plan pays for the revocation with item 7, a second journal, a second AppRole and four interruption tests.

## Findings

### logic-r3 1: the operator role and rotation journal are needed by nothing (MAJOR)

- Tied statement: D10 and D9. This is the question 8 mandate.
- Evidence:
  - The design text on the vault, in `docs/specs/2026-11-25-go-rewrite-foundation-design.md` lines 215 to 228, requires four things. They are a database of its own for OpenBao, self-unseal from a key file, a set-up step that gives the server a credential reaching only its own paths, and reachability only inside the stack. It says nothing about revoking the root token, an operator role, rotation or a journal.
  - C1 to C13 in the same design, lines 378 to 390, contain no check that rotates a credential, applies a policy later or logs in as an operator. Sprint 1 owns C12 and the first half of C13. C13 needs only that the vault survives a restart and the signing-key path stays readable.
  - `.plans/plans/go-s1-stack-vault/plan.md` line 264 states that the operator file is standing administrator material. A holder of that file can widen the server policy, so the revocation of root hides nothing from a local attacker who can read `admin/`.
  - Item 7, `plan.md` lines 420 to 437, exists only to use the operator role. Item 5b lines 303 to 319 and the file table at lines 323 to 332 carry the cost of two files, `admin/rotation.state` and `server/.openbao.rotation.pending.json`. Item 6 line 368 adds a startup refusal for a pending journal.
- Failure scenario: a crash in the middle of rotation leaves the journal behind. Startup then refuses with `credential rotation is pending` until the operator runs the command that the journal needs. The plan has created a new state that blocks startup and the only reason for the state is a feature no check asks for. Every later round has to attack this machinery again, which is why D10 stays stuck at 16 of 38 points.
- What the plan loses if it is cut: post-revocation policy widening and credential rotation without a reset. The fallback is the documented confirmed reset, or a manual `bao write` by the holder of the retained initialization material. Neither is exercised by a check.
- Fix: delete `lgtm-admin`, `operator.json`, `apply-server-policy`, `rotate-server-credential`, `admin/rotation.state`, the rotation pending file, the startup rotation refusal and all of item 7. Keep one AppRole, `lgtm-server`. Keep the initialization output as the single retained administrator file, mode 0600 under the root-only `admin/` directory, and say in the operator guide that it is the administrator. Do not revoke the root token in sprint 1. Add a feature idea for rotation to `docs/features.md` for a later sprint.

### logic-r3 2: the installation binding duplicates five existing guards (MAJOR)

- Tied statement: D10. This is the question 8 mandate.
- Evidence:
  - `plan.md` lines 160 to 166 add `installation.id`, a volume file `lgtm-installation`, a claim by atomic hard link, provisional and committed phases, a loser cleanup rule and a foreign-identity diagnostic.
  - The same plan already refuses a password file that does not authenticate (item 2 reconcile), a seal key that does not match the database (the truth table of item 3, lines 195 to 220), a local state ahead of the database (the 5b row at line 303 onward) and a second concurrent run (the lock of item 6, line 366). Compose already scopes the named volume by project name.
  - No check from C1 to C13 distinguishes a foreign state root from a stale one. D10 asks only that the vault persists across restarts.
- Failure scenario: the cheapest wrong implementation passes. Write `installation.id` and `lgtm-installation` with the same fixed string and the pair always matches, and every listed binding test still passes except the foreign-identity test. The identity protects against an operator who points two state roots at one Compose project, which the one-user, one-machine decision makes a rare error that the password refusal already catches.
- What the plan loses if it is cut: a distinct diagnostic for a state root that belongs to another installation, relocation of a state directory without a message, and the orphan reset of finding 3. Authentication failures still stop the launcher.
- Fix: remove the identity pair and the claim protocol. Keep the item 2 private directory layout and the user and capability table. Report a mismatch through the existing authentication and `state directory is ahead of the database` diagnostics.

### logic-r3 3: the confirmed reset is needed by no check (MAJOR)

- Tied statement: D10. This is the question 8 mandate.
- Evidence:
  - `plan.md` lines 364 to 400 add `deploy/up.sh --reset --yes`, `--orphan-volume-id`, the marker `reset.pending`, a manifest allowlist, the `reset-state` job and the lock-on-inode retention. The operator recovery table at lines 380 to 391 contains ten rows and four of them end in reset.
  - No check from C1 to C13 resets anything. The design has no reset statement. `deploy/test/checks.sh` runs `down` and `up` only for C13, and the plan keeps the named volume across that.
  - About six of the eleven assertions of `deploy/test/stack-vault.sh` test reset or its interruption.
- Failure scenario: reset deletes files from a state root that the operator owns, behind a manifest that has to list every writer's file and every `.tmp` sibling. Each new file added by a later sprint that is missing from the manifest makes reset refuse with a foreign entry, so reset is a hazard for every later change to the state root.
- What the plan loses if it is cut: a single command to discard an installation. The fallback is a documented manual removal of the compose project volume and the state directory, which the guide can state in four lines.
- Fix: delete the flags, the marker, the manifest, `reset-state` and the orphan flow. Replace them with a short section in `deploy/README.md` that names the two removals. Findings 6 and 7 disappear with this cut.

### logic-r3 4: the five-step bootstrap checkpoint is larger than the one thing it protects (MAJOR)

- Tied statement: D10.
- Evidence:
  - `plan.md` lines 280 to 297 define `admin/bootstrap.state` with the steps `intent`, `initialized`, `provisioned`, `revoked` and `published`. Lines 303 to 319 give a row for each state, and item 5a adds a 60-second deadline test.
  - The only fact that needs a durable save is the initialization response, which the vault shows once. The `revoked` step exists only for logic-r3 1. The `published` step exists because the server credential must reach the shared directory.
- Failure scenario: the checkpoint has five states and every pair of state and file presence is a row to test. A crash between the credential rename and the checkpoint write leaves `provisioned` with the file present, and the plan needs a validate-against-accessor row to cover it. A smaller design has two states.
- What the plan loses if it is cut: fine-grained resume after a crash. A retry that starts from `initialized` re-applies the policy and role, which is idempotent in OpenBao.
- Fix: keep two states, `initialized` (the response is durable) and `published` (the credential file is renamed into place). Remove `intent` and `revoked`, and let `provisioned` be a retry from `initialized`.

### logic-r3 5: four fixture shims exist only for findings 1 to 3 (MAJOR)

- Tied statement: plan, and C12 for the part that stays.
- Evidence:
  - `plan.md` lines 93 to 107 list the recording proxy, the fixture `mv` shim, the command shim and `fixture_key_action` beside `with-postgres.sh` (line 95) and `mutate-feed-lock.sh` (line 97).
  - `deploy/test/checks.sh` line 11 hard-codes `mutation_hook=deploy/test/mutate-feed-lock.sh`, and `check_c12` reads `LGTM_TEST_POSTGRES`. So the wrapper and `with-postgres.sh` are needed by C12 and by R10. The other four are needed only by the binding, the rename-only publication, rotation and reset.
- Failure scenario: a shim for `mv` that must observe `--no-copy` tests a property of the helper image that item 4 already pins and tests. The shim passes with the cheapest wrong implementation, a script that calls the shim name and not the real command.
- What the plan loses if it is cut: the cross-mount mutation test for rename-only publication. GNU `mv` 9.7 supports `--no-copy`, so the single positive assertion on the real layout in item 4 remains.
- Fix: keep `with-postgres.sh` and `mutate-feed-lock.sh`, and drop the other four with the mechanisms they serve.

### logic-r3 6: the orphan reset cannot replay (MAJOR)

- Tied statement: D10.
- Evidence: `plan.md` line 394 requires an empty state root for the orphan mode. Line 398 requires the launcher to write and flush `reset.pending` before it stops a service or deletes data. After that write the state root is no longer empty.
- Truth table for the orphan mode:

| Marker | State root content | Volume | Rule that applies | Result |
|---|---|---|---|---|
| absent | empty | claimed, identifier matches | first invocation | writes the marker, proceeds |
| present | marker only | claimed, identifier matches | replay, but the empty-root rule fails | undefined, the plan does not say |
| present | marker only | absent | replay | `reset-state` removes the marker |
| present | marker plus a restored file | any | manifest preflight | proceeds and deletes the restored file |

- Failure scenario: the launcher crashes after it wrote the marker and before it removed the volume. The operator repeats the documented command from line 391, which says "rerun the same command". The empty-root check fails because of the marker, and the operator cannot finish.
- Fix, if reset is kept against finding 3: define the empty-root rule as "no manifest entry other than `reset.pending`", and have the replay compare the marker's installation identifier with the flag.

### logic-r3 7: a foreign replacement volume after the marker leaves no exit (MAJOR)

- Tied statement: D10.
- Evidence: `plan.md` line 398 says a foreign replacement stops reset without stopping its services. Line 368 makes startup refuse while `reset.pending` exists, printing `run deploy/up.sh --reset --yes to finish`.
- Failure scenario: the marker is written, the matching volume is removed, and the operator creates a new volume under the same project name, or a restore puts a different claimed volume there. Startup demands a reset. Reset refuses the foreign volume. The two rules are each correct and together they have no way out except hand-deleting `reset.pending`, which no document names.
- Fix: if reset is kept, name the exit: reset proceeds to `reset-state` when the selected volume is foreign, because it never touches that volume, and it removes the marker. Removing finding 3 removes the case.

### logic-r3 8: the loser's temporary file is not removed (MINOR)

- Tied statement: D10.
- Evidence: `plan.md` line 162 names the temporary file `lgtm-installation.<installation-id>.tmp` on the volume. Line 164 says a losing claim removes only the provisional local identity.
- Failure scenario: a loser that created its temporary file and lost the hard link race leaves the temporary file in the volume root. The reset manifest names no such entry on the volume side, and the volume is unclaimed-with-data until that file is removed by hand.
- Fix: the loser removes its own temporary file, and a test asserts the volume root holds only the winner's file. This finding disappears with finding 2.

### logic-r3 9: the orphan reset needs Compose labels that the preparer may not create (MINOR)

- Tied statement: D10.
- Evidence: `plan.md` line 394 verifies the selected project's Docker Compose project and `pgdata` volume labels. Line 164 inspects the named volume before any Compose action that could create it.
- Failure scenario: a preparer that creates the volume with a plain `docker volume create` produces a volume with no Compose labels, and the orphan flow then rejects the installation's own volume as foreign.
- Fix: say that the volume is created by `docker compose run` only. Disappears with finding 3.

### logic-r3 10: the sprint 2 table names readers that the sprint 2 plan does not yet carry (MINOR)

- Tied statement: plan.
- Evidence: the sprint 2 contract table in `plan.md` lines 338 to 350 freezes the `LGTM_POSTGRES_*` settings, and round 2 patch architect-r2 5 asks the orchestrator to relay them. I found no relay in the plans for the server and sign-in sprints that I was allowed to read, and the reader list inside the patch names no sprint 2 file.
- Failure scenario: the Server sprint chooses its own names and the compose file and the program disagree on first start.
- Fix: name the reading file in the table, and have the orchestrator record the relay in the epic.

## Answers to the task questions

1. Patches against their findings: each patch removes the failure its finding described, as written. The two that break something else are the reset marker, which breaks the orphan replay (finding 6) and leaves a dead end (finding 7), and the binding claim, which leaves its temporary file (finding 8).
2. Unreviewed mechanisms:
   - The binding (`plan.md` lines 160 to 166) is attacked in findings 2 and 8.
   - The reset (lines 394 to 400) is attacked in findings 3, 6, 7 and 9.
   - Rename-only publication (line 336) holds. GNU `mv` 9.7 supports `--no-copy -T`, and the single mount rule is stated. The Alpine pin of the helper image still depends on the item 4 test.
   - `with-postgres.sh` (line 95) holds. It exports `LGTM_TEST_POSTGRES` and C12 reads it.
3. Rejected findings: the register lists none. All 30 are fixed or fixed another way.
4. Writers and readers: the lists are complete for the private files and the markers. The `LGTM_POSTGRES_*` readers in sprint 2 are not complete (finding 10).
5. Contradictions and truth tables: reset marker against the orphan empty-root rule (finding 6), reset marker against a foreign replacement and the startup refusal (finding 7). The pair `reset.pending` and `admin/rotation.state` is consistent because startup checks the marker first and the manifest names both files.
6. Rounds 1 and 2 blockers and majors still open: none that survives the register. The majors that remain are the ones that the round 2 patches introduced, findings 1 to 7.
7. Lens questions touched by the patches: answered below.
8. The mandate, mechanism by mechanism:

| Mechanism | Needed by | Verdict |
|---|---|---|
| Operator role and credential file | none of C1 to C13, no design statement | major, finding 1 |
| Rotation journal | none | major, finding 1 |
| Installation binding to the volume | none | major, finding 2 |
| Confirmed reset with marker, manifest and orphan flow | none | major, finding 3 |
| Bootstrap checkpoint of five steps | D10 needs a durable save of the initialization response; the other four steps serve finding 1 | major for the size, finding 4 |
| `with-postgres.sh` | C12, R10 | needed |
| `mutate-feed-lock.sh` wrapper | C12, `deploy/test/checks.sh` line 11 | needed |
| Recording proxy, `mv` shim, command shim, `fixture_key_action` | only the mechanisms above | major, finding 5 |

## Answers to the lens questions

1. Orderings: the bootstrap checkpoint order is total and the rotation journal phases are total. The reset marker ties with the empty-root check (finding 6). Phases of `deploy/up.sh` are named and ordered.
2. Failure policy: the reset interruption, the rotation interruption and the bootstrap interruption each state permit or deny and a repair. The dead end in finding 7 is the one failure with no repair.
3. Truth tables: built for orphan reset in finding 6. The state table of item 5b has no empty cell that I found.
4. Zero values: an absent `installation.id` with a claimed volume and an absent marker both have named rows. The zero phase of `installation.id` is not defined, and a file with phase missing should be read as provisional. Add that sentence if the binding stays.
5. Two actors: the lock on the state-root inode serializes the launcher, maintenance and reset on one host. Two state roots on one project are serialized by the claim, which is the thing finding 2 proposes to remove in favour of the password refusal.
6. Mutations: the mutation of the reset test, skipping the manifest check, makes the foreign-entry test fail. The mutation of rename-only publication is caught. A wrong change not caught is a marker that is flushed after the volume removal and not before. The listed tests interrupt only at named boundaries.
7. Cheapest wrong implementation: an identical fixed identifier on both sides of the binding (finding 2), and a `mv` shim that is called by name (finding 5).
8. A shape that prevents a wrong state: one file, one owner. The two-journal design (bootstrap and rotation) is the shape that made the round 2 collision possible, and removing rotation removes the second journal.

## Post-build checks

- PB-A: after the cut, run `deploy/up.sh` on a clean checkout, kill the provisioner after `initialized`, rerun and assert that exactly one `lgtm-server` credential results and the policy reads as the reviewed text.
- PB-B: run `docker compose down` and `up` and read the signing-key path as the server credential. This is the first half of C13.
- PB-C: run `deploy/test/checks.sh --only C12` through `deploy/test/with-postgres.sh` and confirm the wrapper exit code reaches the check.
- PB-D: list the files in the state root after a full start and compare with the file table at `plan.md` lines 323 to 332. A file that no writer owns is a finding.
- PB-E: if reset is kept, interrupt after the marker is written, and repeat the documented command in orphan mode. It must finish (findings 6 and 7).

## Grade

5 out of 10. Blockers 0, majors 7, minors 3.
