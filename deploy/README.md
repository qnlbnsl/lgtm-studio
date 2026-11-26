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
