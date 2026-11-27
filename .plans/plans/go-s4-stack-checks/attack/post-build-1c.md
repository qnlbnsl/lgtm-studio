# REVISE

This review is cross-vendor. I am Claude (claude-sonnet-5-5, high), a reviewer of another vendor than the builder and the test writer, who were OpenAI (Codex) workers. This is task `postbuild-go-s4-stack-checks-1c`, nonce `55e6e8`, on branch `sprint-0`, diff range `3cec0319..ccf502a7`, for item 1 of plan `go-s4-stack-checks`. I changed no code and no test. My scratch copy was a temporary folder that I removed afterwards.

The top finding is that `s4_guard_docker` contains a stray `+` argument in its `awk` call, so every health inspection that `deploy/up.sh` makes under `sprint4_fixture_run` fails with exit status 2.

## What I ran

I ran `deploy/test/sprint4-fixture-test.sh` on the built code, and all three cases passed: `s4_private_stack_selection`, `s4_owned_cleanup` and `s4_two_git_workspaces`.

I ran the real flow once with a live Docker daemon on a private project name and a random port, never on the default project name and never on port 8787. I sourced the fixture, opened it on `HEAD`, selected 3, ran `stack-prepare.sh prepare-db` through `sprint4_fixture_run`, selected 4 and brought up PostgreSQL. A direct `docker inspect --format '{{.State.Health.Status}}' <id>` through the guard printed `awk: cannot open "+"` and exited 2, with the full and the 12-character container id. The full `deploy/up.sh` printed `phase database start`, then the same error, and exited 2.

After that run, no container, volume, network or image carrying the run label remained, no fixture folder remained under the temporary folder, and the real service on port 8787 was still up.

I ran these mutations in a scratch clone, each restored afterwards and checked byte for byte against the original:

| Mutation | Result |
|---|---|
| M1, remove the ownership check | A named assertion in `s4_private_stack_selection` fails. |
| M2, remove the port 8787 check | A named assertion in `s4_private_stack_selection` fails. |
| M3b, give checkout B the path of checkout A | `not ok 1 - s4_two_git_workspaces`, with the comment "The two repository workspaces must have distinct private Git roots." |
| M4, teardown removes any container on the selected port | All three cases still pass. |
| M6, compose and curl guards forward everything | All three cases still pass. |

The plan's two named risky mutations (ownership or port check removed, both checkouts on one path) are caught. M4 and M6 are not.

The plan marks no `UNREVIEWED FIX` place that belongs to item 1. The only one is in item 4. The plan's post-build scenario for preserving caller files and certificates across two fresh runs is covered by `s4_owned_cleanup`, and I did not find a way around it.

## post-1c 1 (BLOCKER) The docker inspect guard always fails

The statement is that `s4_guard_docker` cannot let any container health check through, so the fixture cannot drive `deploy/up.sh`. This breaks plan item 1, "Guard driver replacements against the same project, configuration and recorded stores", because the guarded driver cannot run, and it blocks items 3, 5 and 6, which consume the fixture.

The evidence is `deploy/test/sprint4-fixture.sh:611`. The line reads `awk -v id="$id" '...' +                "$s4_dir/objects" && ...`. The line continuation was lost, so `+` is a file name for `awk`. The caller is `wait_healthy` in `deploy/up.sh:134`, which runs `docker inspect --format '{{.State.Health.Status}}' "$container"`.

The failure scenario is any run of `deploy/up.sh` under `sprint4_fixture_run`. The launcher stops at the first `wait_healthy` after PostgreSQL starts, as I reproduced. An owner who runs G1 to G7 sees the launcher fail for a reason that is not about the candidate.

The fix is to remove the stray `+` so that `awk` reads `$s4_dir/objects`, then run the reproduction above.

## post-1c 2 (MAJOR) No test runs a guard

The statement is that the three frozen cases never call `sprint4_fixture_run`, `sprint4_fixture_ready`, `s4_guard_docker`, `s4_guard_compose` or `s4_guard_curl`, so a wrong guard is accepted.

The evidence is mutation M6, which makes the compose and curl guards forward every argument, and all three cases still pass. Finding 1 survived the build for the same reason. The test file is `deploy/test/sprint4-fixture-test.sh`, and the guards are at `deploy/test/sprint4-fixture.sh:538-635`. The plan says to "Guard driver replacements against the same project, configuration and recorded stores", and the plan's tests list does not name a guard case.

The failure scenario is a later edit that lets `docker compose -p default down` through. The fixture tests pass, and the next run acts on a stack it did not create. The fix is a fourth case that runs the guards against the test's Docker shim. It should assert a refusal of `-p`, `-f`, a profile other than `setup` or `'*'`, a compose verb outside the allowed list and a foreign container id, and an acceptance of the inspect call for an owned container.

## post-1c 3 (MAJOR) Teardown of foreign containers, networks and images is untested

The statement is that the cleanup tests plant only a foreign volume, so removal of foreign containers, networks or images is never exercised.

The evidence is mutation M4, which makes teardown remove any container on the selected port, and all three cases still pass. The cleanup loop is at `deploy/test/sprint4-fixture.sh:454-500`, with the per-kind removals at about lines 466-477. The plan's test row names "foreign stack on the selected port" only at selection time, and it names cleanup preservation only for the main volume, the caller file and the certificate.

The failure scenario is a regression that removes a container that merely shares the port or the project name. This is the property the task exists to protect, and it would pass. The fix is to extend `s4_owned_cleanup` with one planted foreign container, one planted foreign network and one planted foreign image, and to assert that they survive `sprint4_fixture_close`.

## post-1c 4 (MINOR) The vault-only route is refused with a misleading message

The statement is that `deploy/up.sh --until vault` cannot run under the fixture, and its error text points at the wrong cause.

The evidence is `deploy/up.sh:28-30`, which exports `LGTM_WEB_PORT=8787`, and the guard, which refuses a Compose call with that port. The printed text is "the settings cannot be read; check the quoting in deploy/.env". The refusal itself keeps the fixture away from port 8787, which is correct by plan item 1.

The failure scenario is an owner or a later item that uses `--until vault` in a proof and is told to check quoting. The fix is to document the refusal in the fixture's header, or to have the guard print a message that names the port, and to give item 3 or 5 a route that does not need that flag. This depends on `deploy/up.sh`, which is not in this item's file list.

## post-1c 5 (MINOR) The legacy tree has no drift check

The statement is that only the candidate checkout is compared before and after generation, not the legacy tree.

The evidence is that the status and head comparison in the fixture, near `deploy/test/sprint4-fixture.sh:25-26`, names `$s4_candidate` and nothing else. The plan says "Compare tracked and untracked source before and after generation inside scratch and fail on drift", and says to use both trees for builds and generation. A `git archive` tree has no Git state, so it needs a hash of its files.

The failure scenario is a build step that writes into the legacy tree, which a later build then uses as if it were `f6cbea30`. The fix is a recorded hash of the legacy tree at open time and a comparison at close and at every select.

## post-1c 6 (MINOR) The curl guard accepts caller-chosen file paths

The statement is that `s4_guard_curl` allows `--cookie-jar` and `--data-binary` with any value, so a driver can write a file at, or read a file from, any path.

The evidence is `deploy/test/sprint4-fixture.sh:626`, where those options are in the permitted list, and the permitted list in the same function checks only the URL and the loopback port. The owner's decision is that security issues do not block the build, so this is minor.

The failure scenario is a driver typo that points `--cookie-jar` at a file in the caller's checkout. The fix is to accept those two options only with a path under `$s4_dir`.

## post-1c 7 (MINOR) The "divergent" branch of repository B is only ahead

The statement is that repository B has a topic branch one commit ahead of the main branch, and the plan asks for a divergent branch.

The evidence is that `s4_two_git_workspaces` asserts the branch name, the fixture-only origin and the dirty README, and nothing asserts divergence. The plan item reads "Give one a fixture-only origin, divergent branch and dirty change". Ahead and divergent are different states, and R14's two-save procedure may depend on which one it has.

The failure scenario is an R14 check that was meant to see a branch with commits on both sides and sees only one side. The fix is a commit on the main branch after the topic branch is cut, and an assertion for it.

## post-1c 8 (MINOR) Build cache and intermediate layers are not accounted for

The statement is that cleanup removes the two named image tags and the verified containers, networks and volumes, but not the build cache or untagged layers that the two builds leave. The effective model check also looks at mounts and binding only, and does not look at keys such as `secrets` or `privileged`.

The evidence is the `image` branch of the cleanup loop in `deploy/test/sprint4-fixture.sh`, and the Compose model check inside `s4_scan`, lines 209-280. Nothing here can touch another stack's objects, and it follows the plan's wording, so this is minor.

The failure scenario is a long-lived disposable host that fills with layers from each run, or a future compose file that adds a privileged service and passes the model check. The fix is to record the cache problem in `docs/issues.md` and to extend the model check by one list of refused keys.

## Grade

I grade this work 6 out of 10. The safety design is strong: random run identity, labels on every object, daemon and path identity rechecks, guards in front of Docker, Compose and curl, and a root-container fallback for root-owned state. The plan's two named risky mutations are caught by the frozen tests, and cleanup in my live run left nothing behind. The work is not yet a usable fixture, because a one-character defect blocks the launcher and the tests cannot see it. Fixing finding 1 and adding the guard and foreign-object cases from findings 2 and 3 would bring it to a SHIP.
