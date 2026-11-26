# Round 2 threat review of the plan `go-s1-stack-vault`

- Task identifier: review-go-s1-stack-vault-r2-threat
- Nonce: f5128a
- Lens: threat. Reviewed range: `git diff 25d3132e cd46223f` on `plan.md` and `decisions.md`, with the register `attack/round-1-findings.md`.
- Throwaway checks run outside the repository and removed afterwards: PostgreSQL image behavior on fresh and older volumes, the visibility of container environment values in the host process list, and what a container on an `internal: true` network can reach. No OpenBao image was present locally, so nothing about OpenBao was run.

## Verdict

REVISE

## Top finding

The operator role (`lgtm-admin`) is described as unable to reach stored secrets, but it can rewrite the server policy and register a server secret identifier, so it can log in as the server under any policy it likes, and its credential never expires (threat-r2 1).

## Findings

### threat-r2 1. The operator role is root-equivalent over the vault, and the plan and its test say it is not

- Severity: MAJOR
- Tied to: D9
- Evidence: `plan.md:223` grants the operator read and update on `sys/policies/acl/lgtm-server`, read on the server role's `role-id`, and update on `custom-secret-id`. It then claims the role gives maintenance "without granting root-generation access or access to the stored data". `plan.md:231` has `operator_role_is_separate` assert that "the operator token cannot read a stored secret or generate a root token". `plan.md:345` says the recovery share cannot mint a root token, while `plan.md:244` retains that share in `admin/recovery.key`.
- Failure scenario: Someone holding `admin/operator.json` logs in as `lgtm-admin`. They write `lgtm-server` a policy that allows every path. They read the role identifier and register a secret identifier they chose through `custom-secret-id`. They log in as `lgtm-server`, which now holds the widened policy, and read `lgtm/data/server/link/signing-key`. With the policy widened to the system paths and the retained recovery key, they can also ask for a new root token. The test `operator_role_is_separate` passes throughout, because it checks only that the operator's own token is denied a direct read. The bootstrap revokes the transient root token, and it is replaced by a standing credential with the same reach.
- Fix: Say in D9 and in item 4 that the operator is root-equivalent for everything the server can be granted. Treat `admin/operator.json` as the new root credential. Rename the test to what it proves, which is no direct read, no `generate-root` and no other mounts. Add an assertion that records the indirect path as an accepted result, so the claim cannot drift back. In item 7, make `apply-server-policy` print the difference between the live policy and the file and require `--yes`, because the file in the checkout is what widens runtime access. Resolve findings 7 and 8 below at the same time.

### threat-r2 2. The reset deletes a directory chosen by an environment variable, and it cannot resume after a crash

- Severity: MAJOR
- Tied to: D10
- Evidence: `plan.md:326` has `stack-prepare reset-state` remove "its local state contents" after the volume is removed. `plan.md:137` makes the state directory any absolute path in `LGTM_STACK_STATE_DIR`. `plan.md:304` binds the volume to a fingerprint of the state path, and that binding lives in the volume that the reset removes first. `plan.md:324` is the only table row that sends the operator to the reset.
- Failure scenario A: The reset removes the volume, and the process stops before `reset-state` runs. On the next run no volume binding exists to check against. Either the guard accepts a missing binding, and `reset-state` empties whatever directory the variable names, or the guard refuses, and startup says `state directory is ahead of the database` forever. The recovery table sends that message to a restore that cannot succeed (`plan.md:323`).
- Failure scenario B: The variable names a directory that also holds other files. The text does not say whether `reset-state` removes only the entries the stack created or everything under the path. `stack_reset_requires_confirmation` (`plan.md:335`) uses only a fixture directory made by the stack, so it cannot show the difference.
- Fix: Record a `reset` intent in the state root before the volume is touched. Let a retry with that intent finish the state removal without a volume binding. Make `reset-state` remove only a fixed list of names (`bao`, `admin`, `server`, `installation.id`, `pg.reconciled`, the three password files and the lock file). Make it refuse to run if the directory holds any other entry, or if a listed name is a symbolic link. Add a test with a foreign file in the directory, and a test that interrupts between the two steps and reruns.

### threat-r2 3. The SCRAM setting applies only when the cluster is created, and nothing checks the rules at run time

- Severity: MINOR
- Tied to: R9 (round 1 `threat-r1 4`, `logic-r1 4`)
- Evidence: `plan.md:139` relies on `POSTGRES_INITDB_ARGS` and `POSTGRES_HOST_AUTH_METHOD`. `plan.md:157` reads the rules only in the fixture. Throwaway check: with the pinned PostgreSQL image, a fresh volume gets `scram-sha-256` on the local and loopback lines. A volume first created without the flags and then started with them keeps `trust` on those lines, and `psql -w` to loopback then returned `1`.
- Failure scenario: A volume that was initialized by a fixture override without the flags, or restored from a dump taken elsewhere, passes the volume claim. `reconcile-db` never reads the rules, so the loopback and socket lines stay `trust`. The readiness probe and the wrong-password test pass on fresh fixtures only.
- Fix: Make `reconcile-db` read `pg_hba_file_rules` through its authenticated administrator connection and fail on any `trust` or `peer` row. Run `postgres_has_no_trust_rules` against a volume that was created without the flags.

### threat-r2 4. An `internal: true` network still reaches services on the host through the bridge gateway

- Severity: MINOR
- Tied to: D10
- Evidence: `plan.md:143` and `plan.md:179` rely on internal networks for "no external route". Throwaway check: a container on an internal network could open a TCP connection to a service the host had bound on all interfaces, using the network's gateway address. It could not reach the host's LAN address, because no route exists.
- Failure scenario: A compromised PostgreSQL or OpenBao container, or the later server, reaches any host service that listens on all interfaces, such as the real LGTM service or a Docker API on TCP. The test `stack_services_have_no_egress_and_no_net_raw` asserts "no external route" and passes, because it does not probe the gateway.
- Fix: Add the gateway address of each internal network to the probe, with a listener on the host, and record the result. If the host can be reached, write it into the plan as a known limit of the boundary, and say the host must not bind sensitive services to all interfaces.

### threat-r2 5. Test fixtures can leak the disposable password and the captured credentials

- Severity: MINOR
- Tied to: R9
- Evidence: `plan.md:93` starts the disposable PostgreSQL with "a disposable password" and does not say how it reaches the container. `plan.md:248` and `plan.md:288` run a recording proxy that sees the root token and every secret identifier, and "reports ... using an isolated test file". Throwaway check: a value passed as `docker run -e NAME=value` is visible in the host process list for the life of the command.
- Failure scenario: Another local user reads the password from the process list and connects to the loopback port with a login that can create databases. A killed wrapper leaves the container and its port behind. The proxy's capture file holds valid, non-expiring secret identifiers (`secret_id_ttl=0`), and the plan does not say where the file lives, who can read it or when it is removed.
- Fix: Pass the password through a private temporary file (`--env-file` or a `POSTGRES_PASSWORD_FILE` mount) and name the cleanup on interruption. Say that the proxy writes statuses and boundary names only, in a per-test directory with mode `0700` removed on every exit. Add that to `two_fixture_projects_clean_up`.

### threat-r2 6. The operator credential, the recovery key and the server identifier have no stated end

- Severity: MINOR
- Tied to: D9
- Evidence: `plan.md:222` sets `secret_id_ttl=0` and `secret_id_num_uses=0` for the server. `plan.md:223` says nothing about the operator identifier's lifetime, use count or rotation. `plan.md:244` and `plan.md:345` keep `admin/recovery.key` although the plan says it cannot mint a root token.
- Failure scenario: The operator credential lives as long as the installation. Nothing replaces it, and the only action for a lost one is restore or reset (`plan.md:345`). The recovery key stays on disk with no use, and it becomes a root-token path if the root endpoint settings change.
- Fix: State the operator identifier's lifetime and add `rotate-operator-credential` to item 7, or say in the plan that there is none. Delete `admin/recovery.key` after the `provisioned` checkpoint, or write down the one action that needs it.

### threat-r2 7. The helper image's packages are pinned by name only, and nothing tests the pins

- Severity: MINOR
- Tied to: D10
- Evidence: `plan.md:220` pins `curl` and `jq` versions "at the exact-commit build" and builds the image locally when it is absent. `plan.md:233` (`images_are_pinned_by_digest`) checks digests only. The helper holds the root token and the operator credential while it runs.
- Failure scenario: The Alpine repository drops the pinned package version. The first build on a new machine fails, or the builder loosens the pin to get it through. The locally built image has no digest of its own, so a changed result cannot be told from the original.
- Fix: Add a check that every package line in `deploy/openbao-tools/Dockerfile` carries an exact version. Record the built image identifier in the proof receipt. Or drop the build: the pinned OpenBao image already has the `bao` command, which prints JSON with `-format=json`.

### threat-r2 8. The capability lists are left to the first clean run, and the test checks one capability only

- Severity: MINOR
- Tied to: D10
- Evidence: `plan.md:143` says "add back only file setup and user-switch capabilities needed by the pinned entries" and "verify those capability lists during the first clean run". `plan.md:179` asserts no raw-network capability.
- Failure scenario: The builder adds `CHOWN`, `DAC_OVERRIDE` and `FOWNER` to make the preparer and the PostgreSQL entry work, and later adds more for the setup job. The test passes as long as `NET_RAW` is absent, so the growth is never seen.
- Fix: Write the exact capability list of each service and job into the plan. Make `stack_services_have_no_egress_and_no_net_raw` compare the whole set read from the running container, not one entry.

### threat-r2 9. The state table does not place `operator.json`, and its files have no names

- Severity: MINOR
- Tied to: D10
- Evidence: `plan.md:242` lists the steps `intent`, `initialized`, `provisioned`, `revoked` and `published`. `plan.md:223` says provisioning writes `admin/operator.json`, but no step covers it. `plan.md:244`, `plan.md:224` and `plan.md:271` refer to "the response file", "protected pending JSON files", "the protected journal directory" and "leftover staging" without names. At least three scripts, the launcher, `reset-state` and four tests read or remove them.
- Failure scenario: A crash after the `revoked` step with the pending credential lost lands on the last row (`plan.md:273`), which prints "incomplete" and sends the operator to restore or reset. If `operator.json` is already durable, the cheaper `rotate-server-credential` would have worked. If it is not durable, no maintenance identity exists, and the table does not say which case applies.
- Fix: Add a row for `revoked` with lost pending objects. Say whether `operator.json` is durable at `provisioned`. List every file in `admin/` and `server/` once, with its writer, its readers and when it is removed.

### threat-r2 10. The contract table omits the log event that C13 reads

- Severity: MINOR
- Tied to: C13
- Evidence: `deploy/test/checks.sh:604` and `deploy/test/checks.sh:611` read the server's log for the event `link_signing_key_loaded` with `source` equal to `openbao` and a `fingerprint` field. The sprint 2 contract table (`plan.md:279`) lists the file, the settings, the path and the request, and nothing about that event.
- Failure scenario: The Server team picks another event or field name. C13 fails at the signing-key step, and the cause looks like a vault failure. Neither plan names the owner of the event.
- Fix: Add a row to the contract table that names the event, its two fields, and the Server team as writer and `deploy/test/checks.sh` as reader.

## Answers to the task questions

1. Patches against their findings. The destination-mount rename removes the `logic2-r1 1` failure, because the hidden file and the target share one mount (`plan.md:275`). The patches for `architect-r1 3`, `logic-r1 2`, `logic2-r1 5` and `logic2-r1 7` remove their failures as written. The SCRAM patch removes `threat-r1 4` for new clusters only (finding 3). The operator patch for `threat-r1 1` replaces a transient root token with a standing, root-equivalent credential (finding 1). The reset patch for `logic-r1 3` and `threat-r1 2` can delete an unbounded directory and cannot resume (finding 2). The proof replay for `architect-r1 1` and `logic-r1 5` is still gated and unverified, as the register says. The image pin patch removes the digest failure and leaves the helper packages loose (finding 7). The internal network and capability patch for `threat-r1 7` is only partly tested (findings 4 and 8).
2. Marks of `UNREVIEWED MECHANISM`, each attacked as a new design:
   - `plan.md:93` (`with-postgres.sh`): password handling and interruption cleanup are open (finding 5).
   - `plan.md:139` (SCRAM through initialization flags): works on a fresh cluster, fails on an older volume (finding 3). I ran it.
   - `plan.md:223` (operator role): root-equivalent (finding 1).
   - `plan.md:224` (secret identifier lookup classification): stopping on storage, authorization and transport errors is the right rule. The status for an absent identifier is measured by the frozen test, so the first run sets it. Nothing else found.
   - `plan.md:275` (publication by within-mount rename): sound for the crash points listed. Nothing found beyond finding 9.
   - `plan.md:304` (volume claim by atomic link): serializes two claimants. The binding is destroyed with the volume, which matters to finding 2.
   - `plan.md:326` (reset): findings 2 and 9.
   - `plan.md:343` (`apply-server-policy`): the file in the checkout is the authority for what the server may reach. The command needs a visible difference and a confirmation (finding 1).
3. Findings with the outcome "rejected": the register has none. The one "not folded" (`critic-r1 12`) is a style finding. Paragraphs at `plan.md:224`, `plan.md:275`, `plan.md:304` and `plan.md:326` still hold many claims each. That is not a threat finding.
4. Names that others build against: the sprint 2 table names the credential file, the password file and the setting names, and its readers are the Server team plan, which is still a brief (`.plans/plans/go-s2-server-signin/plan.md:63`). The log event that `deploy/test/checks.sh` reads is missing (finding 10). The pending, journal and response files have no names (finding 9). `LGTM_TEST_POSTGRES` has two readers, `deploy/test/checks.sh:596` and the Server store plan, and both are named in the plan.
5. Contradictions and truth tables. The state table (`plan.md:264` to `plan.md:273`) and the launcher's binding messages (`plan.md:304`) both print `state directory is ahead of the database`. The cases do not overlap, because the launcher checks the volume before the job runs. The table has one gap, the revoked state with lost pending objects (finding 9). The reset flow and the recovery table disagree for an interrupted reset (finding 2).
6. Round 1 blockers and majors still open although marked fixed: no blocker. `threat-r1 4` is partly open for older volumes (finding 3). `threat-r1 1` is fixed in form and open in effect (finding 1). `logic-r1 3` is open for an interrupted reset (finding 2).
7. Lens questions touched by the patches are answered below.

## Answers to the lens questions

1. Trust boundaries. The vault network and the data network are both reachable from OpenBao, so the PostgreSQL container can reach the vault listener (`plan.md:169`) and its unauthenticated init call during the window before bootstrap. Only a compromised PostgreSQL could use it, and PB5 covers the window. Internal networks still reach the host through the gateway (finding 4).
2. Inventory of secrets. Made: unseal key, three PostgreSQL passwords, root token, recovery key, operator credential, server credential. Stored: all under the state root with the modes in the plan. Sent: in HTTP bodies on internal networks, and the disposable password on a command line (finding 5). Logged: R9 is covered by PB3. Deleted: the root token after revocation. The recovery key has no deletion rule (finding 6).
3. How credentials end. The root token ends by revocation. The batch tokens end in 15 minutes. The server identifier ends by rotation. The operator identifier has no end (finding 6).
4. Encryption. The internal listener and the PostgreSQL connection are not encrypted (`plan.md:169`, `sslmode=disable`). That is a recorded decision on internal networks. At rest, PostgreSQL holds only OpenBao ciphertext, and the key sits on the same host (D10, accepted by the owner). Issue 1633 is not touched by this plan.
5. Dependencies and images. Both service images are pinned by digest. The helper's packages are not tested (finding 7). The proof digests are recorded in scouting.
6. Integrity against untrusted parties. Publication is atomic, and the volume claim serializes claimants. The reset is the weak point (finding 2).
7. Post-build checks: listed below.

## Post-build checks

| Check | Scenario | Expected result |
|---|---|---|
| PB-r2-1 | Log in as the operator, widen `lgtm-server`, register a custom secret identifier, log in as the server. | The server token reads the signing key. The plan states this as accepted. |
| PB-r2-2 | Start the stack on a volume created without the SCRAM flags. | `reconcile-db` fails on a `trust` row and names it. |
| PB-r2-3 | Probe the gateway address of each internal network from each service. | The result is recorded, and the plan states whether the host is reachable. |
| PB-r2-4 | Run the reset with a foreign file in the state directory, and again after stopping it between the two steps. | The first run refuses. The second run finishes the removal. |
| PB-r2-5 | List host processes while `with-postgres.sh` runs. | No password appears. |
| PB-r2-6 | Build the helper image when a pinned package version has been removed from the repository. | The build fails with a clear message, and the pin is not loosened. |
| PB-r2-7 | Read the capability set of each running service and job. | It equals the list in the plan. |
| PB-r2-8 | Generate a root token with the recovery key, with and without an operator token. | Without a token it is refused. The result with a widened server policy is recorded. |

## Grade

6 of 10. Blockers 0, majors 2, minors 8.
