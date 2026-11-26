# Start and stop the PostgreSQL and OpenBao stack

Run `deploy/up.sh` from the repository root on a Linux host with Docker
Compose and the `flock` command. The launcher starts PostgreSQL and OpenBao
and finishes when both are healthy. OpenBao reads the same seal key on
subsequent launches.

The Compose project defaults to `lgtm` and the state directory defaults to
`deploy/.state`. To use a separate installation, set `COMPOSE_PROJECT_NAME`
and set `LGTM_STACK_STATE_DIR` to an absolute path. Use the same settings
for every launch and shutdown of that installation.

To shut down from the repository root while keeping the database volume,
run the following command with the same project and state settings.

```bash
docker compose -f deploy/compose.yaml down
```

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
