# Sprint 4 repository checks on a running stack

## Run the automated checks

Start a disposable Docker Compose stack before running the checks. Give it its
own Compose project name, state directory, image tag, and loopback port. Export
`LGTM_WEB_PASSWORD` with the stack password. Export `LGTM_WEB_PORT` with its
published port, or use the stack's Compose settings and let the script look up
the port.

Use Bash 4.4 or newer, curl, and jq to read JavaScript Object Notation (JSON).
The machine fixtures use Git, Go, and the Linux `timeout` command. R11 also uses
`tar`. R12 and R13 use the repository's installed Playwright and Chromium. Use
the repository's Node.js version for the R13 website build and mock server.
Set `LGTM_REV1_COMMIT` to a local commit that contains a buildable revision 1
client before running R11.

Run the checks from the repository root.

```bash
deploy/test/checks-s4.sh
deploy/test/checks-s4.sh --only R1,R12
```

Each selected check writes one Test Anything Protocol (TAP) result line, using
its number from the design. Checks run in design order. The exit status is zero
when every selected check passes, one when a check fails, and two when the
command line is invalid.

The checks make temporary enrollments and their own client state. They stop
their fixture clients and attempt to remove fixture enrollments, tokens, and
the shell's website session. R1, R2, and R4 compare repository snapshots, so
keep other repository writes off the disposable stack during the run. The
repository snapshots come through the same application programming interface
(API) as the website.

R11 extracts the selected commit with `git archive` and builds the old client
in scratch storage. R12 opens the deployed website before removal and reads
its repository store on the same page afterwards. A conflicting request with
the original operation's request key proves the original call reached the
server and is still waiting before removal. R13 serves the same unpinned
website build in three fresh Chromium contexts. It uses an interceptor in the
test build for in-page calls and observes network requests directly for the
mock and real servers. The mock runs on a temporary loopback port. Build
output, cookies, and program logs stay in private scratch storage.

## Run the deployed check for R14

1. Open the deployed website and sign in.
2. Connect and approve a real machine. Wait for the Machines page to show that it is online.
3. Open **Add repository** from the sidebar. Select the real machine and enter the path of an existing Git checkout on that machine.
4. Read the **Base branch**, **Remote**, and **Status** rows. Check them against the checkout, including the working tree status.
5. Close the dialog and open **Add repository** from the Machines page for that machine. Check that the machine is selected. Enter the same path and read the three rows again.
6. Add the repository. Reload the website and check that the repository appears in the tree with the folder on the chosen machine.

## Names and observations outside the sprint 4 contract

The table marks fixtures and test interfaces chosen for these checks. The
application and link names that the sprint 4 design and contract specify are
used as written.

| Item | Name or observation | Source |
| --- | --- | --- |
| Stack settings | `LGTM_WEB_PASSWORD` signs the checks into the stack. With `LGTM_WEB_PORT` unset, `docker compose port web 8080` finds the port. The file is `deploy/compose.yaml` unless `COMPOSE_FILE` selects files. The checks use Hypertext Transfer Protocol (HTTP) on `127.0.0.1`. | Existing stack and wave 1 checks. |
| Stack isolation | `COMPOSE_PROJECT_NAME`, `LGTM_STACK_STATE_DIR`, `LGTM_IMAGE_TAG`, and `LGTM_WEB_PORT` isolate disposable stacks. | Existing stack settings. |
| Client programs | The script builds `./src/client/cmd/lgtm-client` and invokes `lgtm-client join` and `lgtm-client run`. It gives both invocations a private `LGTM_CLIENT_STATE_DIR` ending in `lgtm-client`. | Existing client and wave 1 checks. |
| Old client source | `LGTM_REV1_COMMIT` selects a commit that declares `const linkRevision = 1` in `src/client/internal/link/conn.go`. The old binary is called `lgtm-client-revision-1` in scratch storage. | The assignment specifies the variable. The source check and binary name are chosen. |
| Revision refusal | The archived client writes a JSON log entry with `msg` set to `link_refused` and `code` set to `link_revision_mismatch`. The machine remains disconnected in `WatchClients`. | Existing client log. The sprint 4 contract specifies the refusal code. |
| Session and machine fixtures | `SignIn` returns `SIGN_IN_RESULT_SIGNED_IN` or `1`. The script uses `CreateClientToken`, `RevokeClientToken`, `ApproveClient`, `RemoveClient`, `SignOut`, and `WatchClients`. The machine watch uses `all.clients`, `saved`, `id`, `tokenId`, `connectedAt`, and `state`. The waiting and online states are `CLIENT_STATE_WAITING_FOR_APPROVAL` and `CLIENT_STATE_ONLINE`, or `1` and `3`. | Existing website contract and wave 1 checks. |
| Machine labels | Fixture machines have labels `primary`, `secondary`, `revision-1`, `removed`, and `retained`. Their names start with `Sprint 4` and include their server-assigned identifier. | Chosen. |
| Git directories | The fixtures use `inspect`, `add`, `original`, `other-name`, `different-origin`, `orphan`, `shared`, and `retained` for checkouts. They use `subfolder` inside a checkout, `typed` for a symbolic link, and `plain` for a directory outside a checkout. | Chosen. |
| Git content and origin | The fixtures start on `main` with `fixture.txt` and the author `Sprint Four` at `fixture@localhost`. The inspection switches to `feature` and makes a local change. The `origin` metadata uses `http://127.0.0.1/sprint4/original.git` or `http://127.0.0.1/sprint4/other.git`. | The design specifies the `origin` remote. The fixture names, commits, and addresses are chosen. |
| Folder request keys | The checks send `s4-inspect`, `s4-add`, `s4-plain`, `s4-original`, `s4-second`, `s4-other-origin`, `s4-orphan`, `s4-shared`, `s4-retained`, and `s4-remove-open`. Each check gets a fresh shell session. | Chosen. |
| Scratch names and signals | `TMPDIR` defaults to `/tmp`. Each check makes `lgtm-s4-check.XXXXXXXX` and uses `session.cookies`, `state-<label>/lgtm-client`, `revision-1`, `build.log`, `join.log`, `client-<label>.log`, `browser.log`, and `browser-stage.log`. R12 uses `page-ready` and `operation-failed` to coordinate the page and the pending call. | The `TMPDIR` convention and stored client folder are existing. The other scratch names are chosen. |
| Stream framing and refusals | The script sends the five-byte Connect stream header with `application/connect+json` and reads uncompressed response frames. Connect maps `failed_precondition`, `not_found`, and `already_exists` to HTTP statuses 400, 404, and 409. | Connect wire protocol. |
| Browser helper | `deploy/test/checks-s4-browser.mjs` uses the modes `removal` and `modes`. For R13 it places the copied Vite configuration under `build`, the Vite cache under `vite-cache`, and the single static build under `website`. | Chosen. |
| Website selectors and store | The browser uses `Server address`, `Connect`, `Use sample data`, `Password`, and `Sign in`. It reads `window.lgtm.store.state`, including `server.synced`, `server.sample`, `session.status`, `machines`, `repos`, and `projects`. It looks for `console` as a sample project and reads repository `folders` by machine identifier. | Existing website. |
| In-page procedure trace | The test build's `sprint-four-sample-router` plugin wraps `createRouterTransport` in `src/frontend/src/server/api.ts`. Its interceptor records procedure names and the discovery flags in `window.sprintFourSample`. The test places `out` in the existing `lgtm-mock:session` storage entry to exercise sign-in. | The plugin and trace are chosen. The transport and storage entry are existing. |
| Website and mock settings | `VITE_LGTM_SERVER` is empty in the R13 build so the website asks for the server address. The test uses `/` for the same origin. The mock uses `src/mock-server/src/serve.ts`, `LGTM_MOCK_HOST`, and `LGTM_MOCK_PORT`, with an isolated fixture password. | Existing website and mock settings. The mock port and fixture password are chosen. |
