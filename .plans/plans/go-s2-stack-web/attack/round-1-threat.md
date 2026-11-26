REVISE

The stack supplies database, private-network and browser-origin settings that the current Server plan cannot use to start and serve the clean-machine sign-in flow.

## Findings

### threat-r1 1 — BLOCKER — C1 — The server cannot start with the proposed settings

The category is design. The score is 9. The Stack and Server leads own the handoff. Evidence by reading `.plans/plans/go-s2-stack-web/plan.md:262–264`, `.plans/plans/go-s1-stack-vault/plan.md:221–224` and `.plans/plans/go-s2-server-signin/plan.md:383–404`.

On a fresh launch, Server ignores the retired `LGTM_POSTGRES_URL` and defaults the database host to its own loopback interface. The stack also omits `LGTM_ALLOW_PRIVATE_STACK=true`, which Server requires for the private listener and plaintext connections to the two named internal services. The default empty `LGTM_PUBLIC_URL` cannot establish the browser origin at the published port. A sign-in from a nondefault port must be tested even after database startup works.

Render `LGTM_POSTGRES_HOST=postgres`, `LGTM_POSTGRES_PORT=5432`, `LGTM_POSTGRES_DATABASE=lgtm_app`, `LGTM_POSTGRES_USER=lgtm_app` and `LGTM_POSTGRES_PASSWORD_FILE=/run/lgtm/postgres.password`. Pass `LGTM_ALLOW_PRIVATE_STACK=true` and an explicit `LGTM_PUBLIC_URL` for the published loopback origin or the configured public origin. Keep the operator's optional public setting distinct from the resolved application origin so loopback still selects plaintext mode. Keep `LGTM_LISTEN_ADDR=:8080` and the two OpenBao settings. Leave `LGTM_ALLOWED_ORIGINS` empty by default. Move Server's `LGTM_TLS_CERT_FILE` setting and certificate consumption to the sprint 3 handoff. Compare the complete redacted mapping with Server before freezing tests.

### threat-r1 2 — BLOCKER — R9 — The sign-in password can be read by another host user

The category is security isolation. The score is 9. The Stack lead and the check driver owner own the fix. Confirmed live with a disposable file copied under umask `022`. Evidence by reading `.plans/plans/go-s2-stack-web/plan.md:268`, `:298` and `:334–339`, and `deploy/test/checks.sh:82–94`.

The documented copy produces mode `0644` under a common umask. In a checkout that other users can traverse, the owner's only sign-in credential becomes readable as soon as it is entered. The check driver also passes that password to the external `jq` process in its arguments. Running the documented checks with the same value as the owner's password exposes it to local process inspection while `jq` runs.

Create the settings file as owner-only before entering the secret. Have the launcher reject an insecure or symbolic-link settings file before reading it, and document how to repair existing permissions. Keep the resolved environment private and disable shell tracing around secret handling. Have the driver construct the request using standard input and use a separate generated fixture password in acceptance. Land the driver change with its owner before freezing this check.

### threat-r1 3 — BLOCKER — D17 — Public browser sign-in starts with an unverified certificate

The category is security isolation. The score is 9. The Stack lead owns browser trust setup. Evidence by reading `.plans/plans/go-s2-stack-web/plan.md:334`, `:434–447` and `:502–512`, and `docs/specs/2026-11-25-go-rewrite-foundation-design.md:202–209` and `:316–325`.

The first remote browser visit instructs the owner to accept a certificate warning and type the only password. An untrusted network endpoint substituted on that first visit can present a different self-signed certificate with the same warning. The automated check uses a local authentic certificate and therefore misses the operator flow it is meant to support.

Use the existing public certificate as the browser trust material. Require the operator to obtain it through an authenticated channel and configure browser trust before entering the password. Verify the configured name and the authentic certificate before sign-in. Change the manual acceptance to prove that the genuine certificate works and that a substituted certificate stops the flow before credentials are entered. This retains the self-signed certificate and persistent key.

### threat-r1 4 — MAJOR — C4 — Removing a password leaves the old server and sessions active

The category is security access. The score is 3. The Stack and Server leads own the startup behavior. Evidence by reading `.plans/plans/go-s2-stack-web/plan.md:297–313`, `.plans/plans/go-s2-server-signin/plan.md:471–478`, `deploy/test/checks.sh:369–397` and `docs/specs/2026-11-25-go-rewrite-foundation-design.md:202–209`.

If a running stack already has sessions and the operator clears the password and invokes `deploy/up.sh`, the preflight exits before touching the old container. The previous password and cookies remain usable. Check 2 bypasses the launcher and cannot detect that live-stack case.

Keep the early status-2 refusal on a fresh project. For an existing project, under the state lock, take the old server out of service and run Server's missing-password reconciliation against the retained database. Report any failure to invalidate sessions and leave service closed. Test removal, refusal of the old cookie while closed, and refusal of that same cookie after restoring the original password.

### threat-r1 5 — MAJOR — C13 — The tests depend on launcher facilities Sprint 1 does not supply

The category is design. The score is 3. The Stack lead owns the fixture and launcher changes. Evidence by reading `.plans/plans/go-s2-stack-web/plan.md:100–104`, `:291–321` and `:558–561`, and `.plans/plans/go-s1-stack-vault/plan.md:223–227` and `:231–260`.

On the Sprint 1 handoff, Web's tests cannot source `deploy/test/vault-testlib.sh` or call `fixture_create` and `fixture_cleanup`. Even after adding those helpers, `--until vault` without a web password would fail before reaching the vault. The reset assertion depends on a reset operation Sprint 1 did not retain. A fixture that falls back to the default project risks performing a recreation or password change against the operator's stack.

Assign fixture creation and cleanup to a Web work item. Pass the same disposable project, state, environment file and port to every fixture command and check-driver invocation. Parse `--until vault` first, omit web-only validation and image work on that route, and preserve the Sprint 1 state lock. Update the retained `stack-vault.sh` call. Drop the reset test and use fixture teardown. Trace early stop and vault failure to Sprint 1's PB8 handoff check, whole-stack restart to PB4 and C13, and C5's database query to Sprint 1 item 2.

### threat-r1 6 — MAJOR — C1 — The readiness budget expires before permitted startup work finishes

The category is a timing defect. The score is 3. The Stack lead owns launcher readiness. Evidence by reading `.plans/plans/go-s2-stack-web/plan.md:262` and `:304`, and `.plans/plans/go-s2-server-signin/plan.md:474–484`.

A database migration that takes three minutes remains within Server's five-minute migration budget, but the launcher's 120-second wait returns failure before `web` starts. At the specified check interval and start period, Compose can mark the server unhealthy even sooner.

Allow at least eight minutes before the launcher or the health-check startup policy can reject a slow initial start. Keep each `lgtm-server health` call bounded. Prove that the server may be initially unavailable for longer than 120 seconds while migrations run and that the launcher waits for readiness before starting the web service.

### threat-r1 7 — MAJOR — C8 — The pinned public proxy contains known vulnerable transport code

The category is security availability. The score is 3. The Stack lead owns the image choice. Evidence by reading `.plans/plans/go-s2-stack-web/plan.md:166` and `:419–426`, `.plans/plans/go-s2-stack-web/scouting.md:73–79`, the Caddy 2.10.2 `go.mod` and `modules/caddyhttp/app.go`, and Go vulnerability report GO-2026-6617. The pinned source includes Go's affected Hypertext Transfer Protocol version 2 (HTTP/2) stack and `golang.org/x/net` version 0.42.0. The advisory covers that dependency before 0.60.0. [Pinned Caddy dependencies](https://github.com/caddyserver/caddy/blob/v2.10.2/go.mod#L38-L42), [Go advisory GO-2026-6617](https://pkg.go.dev/vuln/GO-2026-6617).

Public mode exposes HTTP/2 before the request reaches application authentication. The advisory documents a process crash in that transport, which would interrupt both browser and machine connections. The planner's comparison used default idle settings on 2.11.9 and tried different timeout controls. That release implements a one-minute request-body idle default and a separate `read_body_idle` control. [Caddy 2.11.9 defaults](https://github.com/caddyserver/caddy/blob/v2.11.9/modules/caddyhttp/app.go#L858-L876), [Caddy timeout controls](https://caddyserver.com/docs/caddyfile/options#timeouts).

Make an HTTP/2 dependency advisory review and the stream gate both prerequisites to enabling public mode. Re-run the tests with a patched digest and a route-specific request-body idle allowance that covers the required link silence while ordinary public routes keep bounded defaults. Preserve loopback-only availability until a candidate meets both gates. Record the exact tested digest and effective settings.

### threat-r1 8 — MAJOR — C9 — The Sprint 3 stream gate cannot run independently as specified

The category is design. The score is 3. The Stack lead owns the probe. Confirmed live with a reduced Compose override. Evidence by reading `.plans/plans/go-s2-stack-web/plan.md:86–93`, `:386–416` and `:500–510`.

Replacing `server.image` in a Compose override retains the production `server.build` stanza. A clean run that builds the fixture can invoke the production Dockerfile, even though step 1 promises no Server dependency. The quiet-up assertion also counts ticks without requiring the advertised closing trailer. It can pass if the stream lasts long enough to count the ticks and fails during final closure. The long gate only dials loopback despite both modes being supported at Sprint 3 handoff.

Give step 1 a self-contained fixture with an explicit probe entrypoint and no production Server build or database and vault dependencies. Run the final gate against the shipped web image. Require the quiet-up response to complete with its trailer and fail on reset. Give each probe a bounded test deadline beyond its expected duration. Run the named six-minute soak and both one-direction checks with cleartext loopback and pinned public encryption before Sprint 3 starts. Record both results.

### threat-r1 9 — MAJOR — D17 — The certificate can expire or stop matching the retained key

The category is security integrity. The score is 3. The Stack lead owns credential maintenance. Evidence by reading `.plans/plans/go-s2-stack-web/plan.md:458–488` and `:519–526`.

Starting after certificate expiry with the same host and key reuses the expired certificate. Restoring a valid but nonmatching key next to an existing certificate also leaves the name check satisfied, so Caddy receives a mismatched pair. The public hash is written separately and has no specified atomic write. A partial update can leave future provisioning with an unusable pin.

Under the existing state lock, verify that the retained private key and certificate match and fail clearly on a mismatch. Renew a near-expiry certificate with the existing key and preserve the pin. Atomically publish the certificate and its pin with a recoverable interrupted-write rule. Document explicit key replacement and re-enrollment for compromise or permanent loss, and test expiry, mismatched restore and interruption.

### threat-r1 10 — MAJOR — C1 — Preflight accepts settings that cannot serve the printed address

The category is design. The score is 3. The Stack lead owns listener settings. Confirmed live with disposable Compose configuration. Evidence by reading `.plans/plans/go-s2-stack-web/plan.md:166`, `:263–268`, `:298–304` and `:502–507`.

`config --environment` prints the interpolation environment, so an unset port and bind are absent there even though the rendered service defaults to port 8787 on loopback. A valid bind to another address in the IPv4 loopback block passes preflight but the printed host gets 421 from Caddy. A public address without an explicit port points browsers to the standard HTTPS port even when the stack publishes 8787. The existing busy-port case covers only an already occupied exact bind.

Resolve defaults and normalize the published port once for Compose, Caddy, Server and the printed address. Restrict accepted loopback binds to supported host spellings or derive Caddy's accepted host and Server's origin from the selected bind. Require the public address's effective port to match the published port unless the operator explicitly configures and verifies a forward. Preserve the busy-port diagnostic for conflicting wildcard listeners and a late Docker bind failure. Freeze tests for omitted defaults, supported binds, mismatched port and late bind failure.

### threat-r1 11 — MINOR — plan — The certificate tool has no fixed package artifact

The category is dependency integrity. The score is 1. The Stack lead owns the tool image. Evidence by reading `.plans/plans/go-s2-stack-web/plan.md:465` and `:487`, and `.plans/plans/go-s2-stack-web/decisions.md:17`.

The pinned Caddy base still fetches OpenSSL from Alpine's live package repository. A later clean build fails if the recorded package version is removed from that repository, even with all base digests unchanged. The test only checks that a version string exists.

Record the certificate tool's fixed package source and version before freezing the image tests. Use a digest-pinned tool image or a retained, verified package snapshot. Check the selected artifact in a cache-free build.

## Answers to the attack questions

1. Network input enters Caddy at the published listener, local settings enter through Compose, and mounted credential files enter Server. Caddy filters loopback hosts and routes the protocol paths, while Server owns browser origin, session and machine checks. The resolved ports, private-network opt-in and settings-file permissions need findings 1, 2 and 10. `.plans/plans/go-s2-stack-web/plan.md:166–187`, `:262–268`, `.plans/plans/go-s2-server-signin/plan.md:244–248`, `:275–277`.
2. The operator supplies the password through a shell or settings file and Compose sends it only to Server. Server owns the salted verifier and the cookie, which travels through Caddy. Vault creates the seal key, administrator file, database password and scoped application-role credential. Server mounts only the latter two credentials and keeps the signing seed in OpenBao. The certificate job makes the private web key, public certificate and public pin, and Caddy alone mounts the private key at runtime. Join tokens and machine keys belong to Sprint 3. Proxy access logging is disabled, and findings 2 and 3 cover exposed password paths. `.plans/plans/go-s2-stack-web/plan.md:166`, `:263–268`, `:467–487`, `.plans/plans/go-s1-stack-vault/plan.md:125–135`, `:198–225`, `.plans/plans/go-s2-server-signin/plan.md:244–254`.
3. Server owns sign-out, the 30-day session life and invalidation on password change or removal. The existing vault login token lasts 15 minutes and the published credential has no automatic expiry. Web introduces the persistent certificate key and a certificate with a 3,650-day life. Findings 4 and 9 cover launcher revocation and certificate or key replacement. `docs/specs/2026-11-25-go-rewrite-foundation-design.md:202–208`, `.plans/plans/go-s1-stack-vault/plan.md:209–211`, `.plans/plans/go-s2-stack-web/plan.md:469–473`.
4. Public Transport Layer Security (TLS) ends at Caddy. Loopback and the isolated `app` hop use cleartext HTTP/2, while the approved design still describes a machine certificate and the current contract instead carries a certificate public-key pin and separate signed identity. Record the superseding transport decision before Sprint 3. The owner's first browser visit and certificate-key lifecycle remain findings 3 and 9. OpenBao encrypts its stored secrets, and filesystem permissions protect the local certificate key and credential files. `docs/specs/2026-11-25-go-rewrite-foundation-design.md:215–227`, `:316–325`, `docs/issues.md:7520–7529`, `.plans/plans/go-s0-contract/plan.md:468–472`, `.plans/plans/go-s2-stack-web/plan.md:438–439`.
5. The plan pins the Go and Node build images, the distroless runtime, Caddy and the Vault plan's service images by digest. It trusts the image publishers, the package manager registry for the version-pinned pnpm install, the workspace lockfile and Alpine for OpenSSL. Findings 7 and 11 cover the transport dependency and remaining package source. `.plans/plans/go-s2-stack-web/decisions.md:17` and `.plans/plans/go-s2-stack-web/plan.md:196`, `:465`.
6. A pending machine reaches the link routes but receives no web-session authority. Sprint 3 must enforce the approval boundary at Server. An unlisted browser origin cannot obtain a session through the planned Server guard, and forged forwarded scheme cannot select cookie mode. A different local user can read a normally copied settings file, and failed password removal can leave old sessions serving. `.plans/plans/go-s2-server-signin/plan.md:244–248`, `:275–277`, `docs/specs/2026-11-25-go-rewrite-foundation-design.md:247–256`, and findings 2 and 4.
7. The post-build checks below cover actual listener and origin behavior, operator trust, secret handling, restart and both stream modes. The current checkout has no tracked stack implementation to run. Docker Compose configuration checks ran live with temporary fixtures, and Docker daemon access was denied.
a. The scouting record shows the stand-in passed on 2.10.2 for 6 minutes and 10 seconds of silence in both directions and 6 minutes and 30 seconds upward. Criterion 3b and `LGTM_STREAM_SOAK=1 bash deploy/test/link-stream.sh` are the required merged gate, with the gaps in finding 8. The proposed Caddyfile leaves timeouts at defaults and sets a five-second shutdown grace. Caddy 2.10.2 defaults to one minute for headers, five minutes for idle connections and three seconds for upstream dial. The rules set no active stream lifetime or whole body or response deadline. Server separately specifies five seconds for headers and 60 seconds for idle connections, with no global stream read or write deadline. `.plans/plans/go-s2-stack-web/scouting.md:175–192`, `.plans/plans/go-s2-stack-web/plan.md:166`, `:373–376`, `.plans/plans/go-s2-server-signin/plan.md:478`. [Caddy 2.10.2 listener defaults](https://github.com/caddyserver/caddy/blob/v2.10.2/modules/caddyhttp/app.go#L757-L769), [Caddy 2.10.2 dial default](https://github.com/caddyserver/caddy/blob/v2.10.2/modules/caddyhttp/reverseproxy/httptransport.go#L185-L198).
b. The planned order is images, database, seal, optional certificate setup, vault, server, web and ready. The clean start is blocked by finding 1. The fresh launcher promises status 2 with `LGTM_WEB_PASSWORD is not set` and points to the settings and example files. A busy port promises status 2 with the port and bind and points to `LGTM_WEB_PORT`. Check 2 independently recreates Server without the variable, disables automatic restart and requires a nonzero exit within its 20-second observation and a diagnostic naming the variable. `.plans/plans/go-s2-stack-web/plan.md:298–317`, `deploy/test/checks.sh:369–397`.
c. The current plans agree on numeric Server user and group 10001, the private `data` and `vault` networks, PostgreSQL port 5432, OpenBao port 8200, upstream Server port 8080, both protected credential mounts and the `lgtm-server health` spelling. Server's probe expects private `/healthz` to return 204 within two seconds. Web's independent health listener is internal on port 8081. The database and origin mapping, certificate setting, readiness budget and launcher fixture handoff disagree as detailed in findings 1, 5 and 6. `.plans/plans/go-s2-stack-web/plan.md:262–267`, `.plans/plans/go-s1-stack-vault/plan.md:125–135`, `:219–227`, `.plans/plans/go-s2-server-signin/plan.md:383–400`, `:475–482`.

## Post-build checks

- Start from a disposable clean checkout with the default port, then repeat on a different port. Record redacted Compose configuration, actual service users and health results. The browser and contract share the printed origin, Server and Vault have no published listener, and all four services become healthy in order.
- Seed a disposable signed-in session. Check fresh missing-password refusal, direct container check C2, removal from a live installation and restoration of the original password. The old cookie remains unusable after recovery. On a separate run, corrupt the fixture vault credential before startup. The launcher names the repair and starts neither Server nor Web.
- Inspect only fixture values in the operator settings file, subprocess arguments, image layers, process output and service logs during startup, sign-in, stream cancellation and an upstream failure. Another unprivileged host user cannot read the password or web key. The password, cookie, vault credential and key stay out of diagnostic output. The server signing-key fingerprint is public and stays stable across C13.
- Check the compiled proxy rules in both modes. In loopback mode, a nonloopback Host receives 421 before upstream handling. At a custom published port, permitted browser origins sign in and an unlisted origin receives a refusal. In public mode, a clean browser trusts only the authenticated certificate and signs in with a Secure, HttpOnly, Strict cookie. A wrong certificate fails before sign-in. Plaintext on the published public port exposes no page or procedure.
- Run the short stream checks and both six-minute soak shapes on the exact shipped image in both modes. The checks observe both HTTP/2 hops, message delivery after silence, and complete trailers. When Sprint 3 supplies the real link, a machine awaiting approval cannot exercise approved capabilities, a wrong pin sends no join secret, and a previously approved machine reconnects after a restart.

## Grade

4/10. There are 3 blockers, 7 majors and 1 minor.
