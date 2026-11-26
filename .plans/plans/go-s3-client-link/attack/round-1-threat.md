REVISE

# Round 1, lens threat: review of `go-s3-client-link`

- Task identifier: review-go-s3-client-link-r1-threat.
- Nonce: e7b2dd.
- Reviewed text: `.plans/plans/go-s3-client-link/plan.md` and `decisions.md` as they stand, checked against `docs/specs/2026-11-25-go-rewrite-foundation-design.md`, `src/contracts/proto/lgtm/link/v1/`, and the client code at the current commit.

## Top finding

The plan pins the leaf key of the web endpoint's certificate instead of the certificate fingerprint that the owner approved, leaves that decision "unanswered", and offers no way back when the key changes, so one certificate renewal that issues a new key strands every machine for good (threat-r1 1).

## Findings

### threat-r1 1. MAJOR. The certificate pin changed from the approved design, is unanswered, and a key change strands every machine

- **Tied statement:** D17, and `plan` (decisions rows 8 and 18).
- **Evidence:**
  - `docs/specs/2026-11-25-go-rewrite-foundation-design.md:322-325` says the join token carries the fingerprint of the server's certificate, and that the machine presents a self-signed certificate.
  - `.plans/plans/go-s3-client-link/plan.md:22` and `plan.md:93` pin the Subject Public Key Info digest of the leaf certificate of the stack's web endpoint instead, with no machine certificate.
  - `decisions.md:15` (row 8) and `decisions.md:25` (row 18) are both "unanswered".
  - `plan.md:195` retries a certificate mismatch forever, and `plan.md:22` says replacing the pinned key needs a new enrollment.
- **Failure scenario:** The owner turns on the public address later, behind a proxy that renews its certificate automatically. Many automatic renewals issue a new key unless told to reuse the old one. After the renewal every machine fails the pin at the Transport Layer Security (TLS) handshake, logs the same line every 15 seconds, and never reconnects. The only recovery is a fresh enrollment in a fresh folder for each machine, and the website shows every machine offline with no cause.
- **Why the Ed25519 check does not save it:** The server's Ed25519 proof has no channel binding to the TLS connection. A client that accepted a new certificate key after one verified proof could be handed to a relay that forwards the proof and then reads or alters the rest of the stream. So a silent re-pin is not an acceptable fix.
- **Prescribed fix:**
  1. Put the pin model to the owner as a named decision before the freeze, because the plan departs from the approved text.
  2. Choose one: the stack keeps the same key across renewals, which `deploy/` must state and test, or the token carries a second, spare pin that the client also accepts.
  3. Say in the client's refusal diagnostic that the pin changed and that the owner must re-enroll or reissue the token, so the failure has a cause on screen.

### threat-r1 2. MINOR. No size limit on frames from a peer that has not yet proved itself

- **Tied statement:** R9.
- **Evidence:** `plan.md:105` caps the encoded token at 4 kibibytes but sets no cap on the `JoinResponse` or on any frame of `Open`. `plan.md:153-155` sets deadlines but no read limit. The generated Connect client reads without a limit unless `connect.WithReadMaxBytes` is passed.
- **Failure scenario:** Another process on the machine answers on the dialled loopback port before the server does. The default port is the one the old engine also uses. It sends a challenge of several gigabytes. The 10-second and 30-second timers do not bound memory, so the client is killed by the memory limit, exits with status 1 and is restarted by the service in a loop.
- **Prescribed fix:** Pass a read limit of a few tens of kibibytes for `Join` and for frames before a verified `Welcome`, and a larger bounded limit after it. Add a stub-listener test in `conn_test.go` and `join_test.go` that sends one oversized frame and observes a fixed diagnostic and bounded memory.

### threat-r1 3. MINOR. The join secret goes to an unverified listener on plain loopback

- **Tied statement:** D17 and R9.
- **Evidence:** `plan.md:93` allows plain HTTP/2 on loopback with no pin. `plan.md:107-108` sends `token_secret` in the first request and compares the server key only after the reply. `plan.md:119` lists only the pin as the guard.
- **Failure scenario:** The owner joins from the machine that hosts the stack. A process of another user on that machine holds the loopback port first. It reads the token secret from the first request, and it can enroll its own key at the real server within the 15 minutes. The client then sees a signing-key mismatch and prints only that the keys differ, so the owner does not learn that the secret is already disclosed.
- **Prescribed fix:** Make the fixed diagnostic for a signing-key mismatch after a plain loopback request say that the token may be exposed and should be revoked on the Machines page. Add the plain-loopback case to the risk table at `plan.md:119` and to `TestJoinRejectsAddressAndRedirect`.

### threat-r1 4. MINOR. The token stays readable in the process arguments for the whole retry window

- **Tied statement:** D7 and R9.
- **Evidence:** `plan.md:24` and `plan.md:121` accept the exposure and leave it to the orchestrator's pending decision. `plan.md:105` keeps the process alive for up to 15 minutes while the server is unavailable.
- **Failure scenario:** The server is down when the owner runs `lgtm-client join <address> <token>` on a shared machine. For up to 15 minutes another user reads the token from the process listing, runs `join` with its own key and a copied host name, and wins the single use. The owner's command fails with `token_used`, and a waiting machine that looks right is waiting for approval.
- **Prescribed fix:** Keep the printed command as the owner approved it, and also accept the token from standard input (argument `-`) and from an environment variable. Make the installer use one of those. Record in the plan that the printed form stays visible in the process listing.

### threat-r1 5. MINOR. Receipt retention follows the machine's wall clock, so the contract's retry window is not kept

- **Tied statement:** R6 and D16.
- **Evidence:** `plan.md:26` and `plan.md:246` say `Finish` prunes receipts older than 24 hours past expiry by the local wall clock. `.plans/plans/go-s1-client-local/plan.md:47` and `:449` fix that rule. `.plans/plans/go-s3-client-link/scouting.md:71` records that the contract still promises the full window.
- **Failure scenario:** A virtual machine resumes with its clock 26 hours ahead. The next `Finish` prunes a receipt whose command has not yet expired. The server retries that command, `Begin` returns a new command, and the restart or scan runs a second time. With a clock only a little ahead, a retry near the end of the window finds no receipt, is answered `EXPIRED`, and the server records that a finished command expired.
- **Prescribed fix:** Ask the local-state plan for a pruning input that cannot run ahead of the server, such as the same authenticated upper bound used for expiry, or a rule that never prunes while the wall clock is ahead of that bound by more than a small margin. Add a test with the clock 25 hours ahead and an unexpired command.

### threat-r1 6. MINOR. Security-bearing decisions are open at the freeze gate, and one row names a method that does not exist

- **Tied statement:** `plan`.
- **Evidence:** `decisions.md:15`, `:16`, `:17`, `:25`, `:34`, `:39` (rows 8, 9, 10, 18, 27 and 32) are "unanswered". `decisions.md:20` (row 13) says to use `Store.Reconcile`. The Sprint 1 store has no such method (`.plans/plans/go-s1-client-local/plan.md:408-413`), and row 25 at `decisions.md:32` supersedes it.
- **Failure scenario:** A builder reads row 13 first, writes against `Store.Reconcile`, and the build breaks at compile time. Worse, the rows that fix the trust model are still assumptions when the link freezes, so the server plan and the client plan can freeze on different readings of rows 8 and 18.
- **Prescribed fix:** Mark row 13 as superseded in its own text. Close or send to the owner rows 8, 9, 10 and 18 before the freeze, naming who decides.

## Answers to the attack questions

1. **Trust boundaries.** Nothing found beyond findings 2 and 3.
   - The pasted token is checked for length, prefix, identifier form, hash lengths and origin before any network use (`plan.md:105`).
   - The server's frames are trusted only after the Ed25519 check (`plan.md:153-155`).
   - The key file is checked for mode, owner and links by the code that exists (`src/client/internal/enroll/credential.go`).
   - `identity.json`, settings and the state folder variable are validated (`plan.md:108`, `plan.md:257`).
2. **Inventory.**
   - The join secret is made on the website, travels through the clipboard and the process arguments (finding 4), and is sent once in `JoinRequest`. It is never logged, because diagnostics are fixed (`plan.md:104`, `plan.md:257`).
   - The machine's private key is made by `enroll.Create` at mode 0600, stays in one file, is never sent and is never deleted.
   - The two public keys and the pin sit in `identity.json`, which is not secret but is validated as private.
   - The `connection_token` stays in memory, and wave 1 never uses it (`plan.md:154`). The redaction test at `plan.md:261` does not name it, so check it after the build.
   - Events and receipts in `client.db` hold the facts and the workspace path, in a private file.
   - No password verifier or cookie reaches the client.
3. **How each credential ends.**
   - The join token ends after one use, after 15 minutes, or by revocation on the server.
   - The machine key ends only when the owner removes the machine. The client then exits with status 0 and keeps the key, `identity.json` and `client.db` (`plan.md:259`). Rotation is out of wave 1 (issue 1639).
   - A pin or signing-key change ends the enrollment with no automatic path (finding 1).
   - A receipt ends 24 hours after expiry by the local clock (finding 5).
4. **Encryption.**
   - Plain HTTP/2 is allowed on loopback only, and any other address needs HTTPS with a pin (`plan.md:92-93`), as the design says at lines 316-320.
   - The gap is the departure from the approved certificate model (finding 1).
   - The plan does not state a minimum TLS version, so it gets the Go default.
   - No frame is bound to the TLS connection, so the proxy that ends TLS is trusted, which issue 1633 records.
   - At rest, the key and `identity.json` are private files. Nothing is encrypted beyond the disk, which the design does not ask for.
5. **Dependencies.**
   - `golang.org/x/sys` v0.50.0 and `go.etcd.io/bbolt` v1.4.3 are pinned by version.
   - Both resolve. The module cache holds v0.50.0 from the Go project's own repository, and `go.sum` already lists bbolt v1.4.3 but only `x/sys` v0.29.0, so item 3 adds the new line.
   - Adding them trusts the Go team for one clock call and the etcd maintainers for the store that holds all local state. Both are checked by the Go checksum database.
   - `connectrpc.com/connect` v1.21.1 is already pinned. This plan adds no image.
6. **Integrity.**
   - A listener on the dialled loopback port can read the join secret (finding 3) and send oversized frames (finding 2).
   - Another local user can read the process arguments (finding 4) and cannot read the key.
   - A wrong endpoint over HTTPS gets no token, because the pin fails first.
   - A wrong signer gets no machine proof, because the signature is checked first (`plan.md:155`).
   - A refusal from an endpoint that has not proved itself cannot stop the client (`plan.md:195`).
   - A website on another site has no path to this code.
7. **Post-build checks.** Listed below.

### Task questions

- **a. Joining and the proof.**
  - The client keeps the private key file, which is made before the first request, and `identity.json` with the machine identifier, both public keys, the address and the pin. It keeps `client.db`, which is made before `identity.json` (`plan.md:258`).
  - It refuses a server whose key does not match. It compares the digest of `server_public_key` with the token's digest in constant time before `SaveServer` (`plan.md:108`), and it checks the Ed25519 signature with the saved key on every stream before it sends a proof (`plan.md:155`). C10 is covered.
  - A lost reply is retried with the same key and token for up to 15 minutes, and the server returns its first reply (`src/contracts/proto/lgtm/link/v1/link.proto:75-76`). The server checks expiry before the replay (`link.proto:66-76`), so after the token's own 15 minutes the retry gets `token_expired`. The owner then removes the waiting record and uses a fresh folder, which `plan.md:258` says.
- **b. Reconnecting.**
  - It retries after 1, 2, 4, 8 and then at most 15 seconds (`plan.md:195`). It stops only on a refusal that follows a verified challenge (`plan.md:155`, `plan.md:195`).
  - On each verified welcome it acknowledges the server's mark and sends the rest in number order, and the server drops repeats (`plan.md:168-169`). C9 and C11 are covered.
  - Nothing found. The waits have no jitter, which I did not file because this is a one-user stack.
- **c. Running a command.**
  - A command that arrives twice runs once, because `Begin` answers `Replay` or `Running` (`plan.md:244`). It can run twice only after a receipt is pruned early (finding 5).
  - An expired command is declined. A reserved receipt is finished as `EXPIRED` before the reply, and a missing clock bound counts as expired (`plan.md:242-244`).
  - The plan builds on the current protos and on names that exist now: `enroll.Create` and `enroll.Load`, `state.Open`, `Folder.Paths`, `config.StateDir`, `harness.Definitions`, `harness.DefaultScanOptions`, `harness.Scan`, `sysinfo.ReadFacts` and `sysinfo.ReadLoad`. `link.Store` and its methods match `.plans/plans/go-s1-client-local/plan.md:408-413` and are not in code yet. The one stale name is `Store.Reconcile` in `decisions.md:20` (finding 6).

## Post-build checks

1. **Pin on every connection.** After a connection with a stored session ticket, change the server's certificate key and reconnect. Expect a refusal with zero HTTP bytes sent, which shows the pin runs on resumed connections too.
2. **No secret in any output.** Run join and run through every refusal and failure code, then search standard error, the log and the error values for the token secret, the connection token and the key bytes. Expect no match.
3. **File modes after a crash.** Kill the process at each crash step of `state.WriteAtomic` and of store creation. Expect the key, `identity.json` and `client.db` at mode 0600, the folder private, and no temporary file left.
4. **A tampered identity.** Edit `identity.json` to an HTTPS address with no pin, a pin of the wrong length, or a path in the address. Expect `link.Run` to dial zero times and exit 1 with a fixed line.
5. **Oversized frame.** Let a stub peer send a 1 gibibyte `JoinResponse`, then a 1 gibibyte `Challenge`. Expect a fixed diagnostic and bounded memory (finding 2).
6. **Clock ahead.** Set the machine clock 25 hours ahead, finish a command, then replay it before its server-side expiry. Expect no second run (finding 5).
7. **Plain loopback squat.** Join against a stub that answers a wrong signing key. Expect the refusal to tell the owner to revoke the token (finding 3).
8. **Removal and restart.** Remove the machine while it is connected, then reboot. Expect status 0, no restart loop, and one verified refusal per boot.
9. **Dependencies.** Run `go mod verify` and `govulncheck ./src/client/...` after item 3 and the Sprint 1 store item. Expect `go.mod` and `go.sum` to name the two pinned versions and no known vulnerability.
10. **C10 on the real stack.** Run the wrong-key check both ways. Expect the server's log to show no proof from the client when the server's signature was wrong.
11. **Process arguments.** With the server stopped, run `lgtm-client join` and read the process listing as another user. Expect the token visible with the positional form, and absent with the standard-input form once finding 4 is folded.

## Grade

7 out of 10. Blockers: 0. Majors: 1. Minors: 5.
