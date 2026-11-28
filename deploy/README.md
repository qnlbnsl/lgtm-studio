# Run the LGTM Studio stack

## Start only PostgreSQL and OpenBao

Run `deploy/up.sh --until vault` from the repository root to start just
PostgreSQL and OpenBao. The launcher prints `postgres and openbao are healthy`
when both services are ready. OpenBao reads the same seal key on subsequent
launches.

The Compose project defaults to `lgtm` and the state directory defaults to
`deploy/.state`. To use a separate installation, set `COMPOSE_PROJECT_NAME`
and set `LGTM_STACK_STATE_DIR` to an absolute path. Use the same settings
for every launch and shutdown of that installation.

## Stop the stack and keep its data

Run the following command from the repository root with the same project and
state settings you used to launch the stack.

```bash
docker compose -f deploy/compose.yaml down
```

This command keeps the named PostgreSQL volume and the state directory. If
you launched with `COMPOSE_FILE` set to a nonempty value, use
`docker compose down` with that same value instead.

## Administer OpenBao

Manual policy administration uses the protected local administrator response
at `admin/init-response.json` within the state directory. Keep its contents
out of logs, shell history, and server mounts.

## Rebuild an unfinished disposable installation

Confirm the Compose project and its state directory before deleting any
data. For a known unfinished disposable installation, run
`docker compose -f deploy/compose.yaml -p <project> down -v` with the same
state setting. Remove that project's state directory as its owner or an
administrator, then run `deploy/up.sh` again.

Keep both the PostgreSQL volume and the state directory after the stack has
held data. Deleting either can make the data unusable. This version has no
rollback procedure.

## What the host needs

Use Linux with Docker Engine 28 or newer, Docker Compose 5.5 or newer, and the
`flock` command. Older Docker Engine releases can expose a published loopback
port to other hosts on the same local network.

## Settings

The launcher reads `deploy/.env` by default. An exported shell setting takes
precedence over the value in that file.

| Setting | Default | Effect |
| --- | --- | --- |
| `LGTM_WEB_PASSWORD` | The value is empty. | This is the password you enter on the website. The server refuses to start with an empty password. |
| `LGTM_WEB_PORT` | The port is 8787. | This sets the published loopback port. Enter one number from 1 to 65535. Choose another port if the old engine already uses 8787. |

## Start everything and sign in

On the first launch, create the settings file from the repository root.
Reuse that file on subsequent launches.

```bash
install -m 600 deploy/example.env deploy/.env
```

Set `LGTM_WEB_PASSWORD` in `deploy/.env` using a text editor. Keep the
password out of shell commands and logs. Run the launcher from the repository
root.

```bash
deploy/up.sh
```

The launcher prints a start line and a done line for each phase. The phases
are `database`, `seal`, `vault`, `images`, `server`, `web`, and `ready`.
After startup it prints `postgres and openbao are healthy`. The next line
starts with `the website is at` and gives the loopback address. Open that
address in your browser and enter the configured password to sign in.

## Installing the client

Run `deploy/install-client.sh` from the repository root with the stack
running on this computer. Use one account per computer.
Run one installer at a time. Run it as the user who runs the coding agents.
On Linux, that user needs a running systemd user manager.
On macOS, run it from a logged-in desktop session.

The installer needs `go`, `docker`, `curl`, and `jq` on your shell's search
path. Use the same Docker Compose settings that you used to start the stack.
The installer builds the client from this checkout. After a fresh install,
approve the waiting machine on the Machines page. The page shows when the
client comes online.

The program is installed at `$HOME/.local/bin/lgtm-client`. The state folder
defaults to `$HOME/.local/state/lgtm-client`. To use another folder, set
`LGTM_CLIENT_STATE_DIR` to an absolute path on the first install.
A later install uses the folder recorded in the installed service.
Run the installer again to update the program. The installer restarts
the service with the same machine identity. If a coding agent is installed
in a new directory, add that directory to your shell's search path and run
the installer again so the service can find it.

### Upgrade an existing Sprint 3 installation to Sprint 4

Update the server and website together before updating the clients. The
website can be unavailable during the cutover. After it returns, a client
that still uses link revision 1 is refused with `link_revision_mismatch`.
Its machine may show as offline until the compatible client reconnects.

1. Run the [Sprint 4 release checks](test/README.md#run-the-sprint-4-release-checks)
   on a disposable stack before changing the installation.
2. Record the installed Compose project, PostgreSQL volume, stack state
   directory, published loopback address, and source of the website password.
   Record both installed image tags and the tags and image identifiers of the
   candidate pair. Keep the installed images available through the cutover.
3. Verify privately that the candidate resolves the same valid website password
   and the same stores. Build the candidate server and website from the same
   checkout while the old stack serves. Set `LGTM_IMAGE_TAG` to a separate
   candidate tag and use the installed Compose settings. With the default
   Compose file, run `docker compose -f deploy/compose.yaml build server web`
   from the candidate checkout. If the installation sets `COMPOSE_FILE`,
   select the corresponding candidate files and run
   `docker compose build server web`.
4. Record the Sprint 3 migration ledger. Take a consistent backup of
   PostgreSQL and the stack state directory. Stop each installed client
   before backing up its state directory. Keep those clients stopped through
   the cutover. Proceed only when the credentials, backups, and image build
   have all been verified.
5. Run the candidate checkout's `deploy/up.sh` with the same project, volume,
   state directory, address, and password source. Select the candidate image
   tag. Wait for `phase server done` and `phase web done`. Verify the reported
   address, the original signed-in session, and that `GetServer` advertises
   `lgtm.v1.ProjectService` at website contract revision 1.
6. Run the candidate checkout's `deploy/install-client.sh` as each installed
   service user. The installer reuses the state directory recorded in the
   service and restarts the client. On the Machines page, verify that the
   original approved machine comes back online.
7. Follow the [deployed repository checklist](test/README.md#run-the-deployed-sprint-4-repository-check)
   on the updated website.

#### Recover from an interrupted cutover

If startup leaves the migration outcome unclear, stop the candidate server
in the selected Compose project. Read the migration ledger in that
installation's PostgreSQL database.

```sql
SELECT part, version FROM schema_migrations ORDER BY part;
```

Compare the ledger with the record from Sprint 3 and the reviewed candidate
migration list. If the ledger is verified unchanged, the prior server and
website can start with the original settings. Once any candidate migration
commits, preserve both stores and rerun the candidate's `deploy/up.sh` with
the same settings. Use that recovery path even if only the website failed to
start. Keep clients stopped while the migration outcome is unknown.

If the installer stops after replacing the binary, check the recorded service
definition, the installed binary, and the running process before rerunning
the same candidate installer as the installed service user. Keep using a
compatible client with the saved state once it has written revision 2
receipts.

For a revision mismatch, install a compatible client and keep its state.
For damaged client history, restore the original saved client history or
remove the machine and enroll it again with fresh state. Keep backups and
credentials private throughout recovery.

### Linux service and logs

The systemd user unit is installed at
`$HOME/.config/systemd/user/lgtm-client.service` by default.
If `XDG_CONFIG_HOME` is set, the unit is installed under that directory
at `systemd/user/lgtm-client.service`.
Read the client's output with `journalctl --user -u lgtm-client.service`.

The client can reconnect after a reboot before you log in when lingering
is enabled for its user and Docker starts at boot. Lingering lets the
user's services start at boot. The installer reports an error if it cannot
enable lingering, even though the client is installed.

### macOS launch agent and logs

The launch agent is installed at
`$HOME/Library/LaunchAgents/dev.lgtmstudio.client.plist`.
The agent starts the client when its user logs in.
Read the client's output in `$HOME/Library/Logs/lgtm-client.log`.

### Remove the client or join again

Run `deploy/install-client.sh --remove` to stop the client and remove its
program and service. This command keeps the state folder.

Join again if the machine was removed on the Machines page, the published
port changed, or a previous join did not finish.

1. Run `deploy/install-client.sh --remove`.
2. Remove the old machine on the Machines page if it is still listed.
3. Move the state folder aside by hand. Keep the moved folder's private
   permissions until you delete it, because it contains the machine's
   private key.
4. Run `deploy/install-client.sh` again. If you used a custom state folder,
   set `LGTM_CLIENT_STATE_DIR` again when you install.

### Join token access

Anyone who can run a command inside the server's container can make a
join token. The installer sends the token to the client on standard input.
It passes a dash as the token argument. A manual join that supplies the
token as an argument exposes it in the process list while the command runs.

## Change the password

Edit `LGTM_WEB_PASSWORD` in `deploy/.env`. Run `unset LGTM_WEB_PASSWORD`
in the shell so the file supplies the replacement. Run `deploy/up.sh` again
to apply the change. The change ends all existing website sessions. Sign in
again with the replacement password.

## Remove the password

Delete `LGTM_WEB_PASSWORD` from `deploy/.env` and run
`unset LGTM_WEB_PASSWORD` in the shell. Run `deploy/up.sh` again. The
launcher reports the server's `Session revocation committed.` diagnostic
when all existing sessions have ended. It reports
`LGTM_WEB_PASSWORD is missing or invalid.` and stops the server.
If it reports `Session revocation was not confirmed.`, run `deploy/up.sh`
again before setting a password.

## Reach the stack from another computer

Forward the loopback port with Secure Shell (SSH). Use the same port number
on both ends because the server checks the browser's origin against its
configured port. Open the forwarded loopback address in your browser.
A public address requires a proxy, which is outside this release.

## Find the published port from another tool

Run the following command from the repository root after the launcher
finishes. Use the same project settings and shell as the launcher.

```bash
docker compose -f deploy/compose.yaml port web 8080
```

Compose reports the published address and port even when `deploy/.env`
supplies the port or project name. If you launched with `COMPOSE_FILE` set
to a nonempty value, run `docker compose port web 8080` with that same value.
