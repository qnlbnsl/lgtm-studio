# REVISE

The website image suite passes a build that leaves the browser asking for a server address and never calls its own server.

## Findings

### post-a 1 · MAJOR · Plan criterion 1c and design check C1

The automated server-address check reads Docker build history. The other website checks fetch the static files without running the application. In a scratch image, I kept the build with `VITE_LGTM_SERVER=/` and appended a second frontend build without that setting in the same Dockerfile step. The second build replaced the files served by the image. The frozen image suite still printed `8 passed, 0 failed` and exited 0. The rules and rules-only stream suites exercise the unchanged Caddy rules independently of that image. [History check](../../../../deploy/test/web-image.sh#L167) · [Static asset check](../../../../deploy/test/web-image.sh#L150) · [Dockerfile](../../../../deploy/website.Dockerfile#L15)

Headless Chromium confirmed the failure scenario. The mutated image showed one address input and made zero `SessionService/GetServer` requests. The baseline image showed no input, made a same-origin `GetServer` request and displayed the expected message that no server answered. A build regression can therefore pass the scripted gate and reach the browser acceptance step. Automate the browser step against the built image and assert both the same-origin request and the absence of the address input. [Plan criterion and browser step](../plan.md#L146) · [Design check C1](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L378)

### post-a 2 · MINOR · Plan item 7 and post-build check PB3

The quiet-up handler's body scanner sends each line on an unbuffered channel. The handler returns on a failed tick or acknowledgement write, which can leave the scanner permanently blocked on its next send. With the pinned Go toolchain, a scratch test supplied three buffered lines and made the first acknowledgement write fail. The handler returned the write error, and the goroutine profile showed `serveQuietUp.func1` blocked in `chan send` at line 182. The ordinary link suite still passes because its quiet-up probe reads the response through the trailer. A stand-in that stays running through repeated disconnects can accumulate these goroutines. Give the scanner a cancellation signal that closes on every handler return, and select between that signal and sending each line. [Scanner and write error paths](../../../../deploy/test/streamprobe/main.go#L177) · [Quiet-up assertion](../../../../deploy/test/link-stream.sh#L234) · [Peer-loss scenario](../plan.md#L372)

The review grade is **8 out of 10**.

## Scope and acceptance evidence

I reviewed commit `37abe20b` on `sprint-0` against `58ce57ff`. The round-four register has no `UNREVIEWED FIX` in items 2, 3 or 7. Its marked scenarios cover later launcher, guide, combined gate and contract work. I read the builder's explanations for the counted `PASS` and `FAIL` output and the probe's scheduled gap. [Register](round-4-findings.md#L10) · [Builder result](../../../../.foundry/go-rewrite/results/s2-web-build-a.md#L143)

The frozen checksums matched in the checkout. I ran the suites from a scratch snapshot on Docker Engine 29.7.2. The scratch wrappers changed the Docker resource prefix to `lgtm-s2w-rv-` and kept the temporary folders for evidence. Assertions and time limits were unchanged. Ports were allocated by Docker on loopback. A final check found no containers, networks or image tags with the review prefix.

```text
$ bash deploy/test/web-rules.sh
PASS [rules_serve_the_website]
PASS [rules_pass_contract_paths_on]
PASS [rules_keep_status_and_body]
PASS [rules_keep_host_and_set_forwarded_proto]
PASS [rules_refuse_other_host_names]
PASS [rules_answer_health_inside_only]
6 passed, 0 failed
Exit 0.

$ bash deploy/test/web-image.sh
PASS [web_image_bases_are_pinned]
PASS [web_image_context_is_allow_listed]
PASS [web_image_leaves_out_state]
PASS [web_image_serves_the_built_site]
PASS [web_image_fixes_the_server_address]
PASS [web_image_carries_the_rules]
PASS [web_image_runs_unprivileged]
PASS [web_image_has_no_build_tools]
8 passed, 0 failed
Exit 0.

$ bash deploy/test/link-stream.sh --rules-only
PASS [stream_passes_both_ways_loopback]
PASS [stream_hops_speak_http2]
PASS [stream_survives_upward_silence]
PASS [stream_survives_downward_silence]
PASS [other_routes_keep_the_default_limit]
PASS [server_stream_arrives_message_by_message]
PASS [stream_survives_soak] # SKIP soak not requested
6 passed, 0 failed, 1 skipped
Exit 0.
```

The basic probe ran for 55 seconds and reported 11 messages each way, trailer `11` and HyperText Transfer Protocol version 2 (HTTP/2) on both hops. Quiet-up ran for 80 seconds, received 39 ticks and acknowledged the resumed message. Quiet-down ran for 80 seconds and returned `count 40` after the marker. Both ended with the expected trailers. The ordinary watch and join control probes each exited 1 after 61 seconds.

I also exercised PB3 against a freshly built website image with the stand-in on a disposable network. Killing the client ended the upstream request in under one second with `CANCEL`. Stopping the upstream caused the client to report a closed pipe and exit 1 after a measured second, including the stop command. An additional HTTP/1.1 watch probe canceled its upstream request within one second of the downstream disconnect. [Peer-loss scenario](../plan.md#L372)

Bash syntax checks passed for all three frozen scripts. The Go format, vet and package test commands exited 0 for the probe using the pinned toolchain. The package has no permanent unit tests.

The Compose-based stream run and merged-image soak depend on the later fixture. The real sign-in and machine heartbeat parts of PB7 remain integration checks. This review exercised the ordinary-route timeout with the stand-in on the watch and join paths. [Stream rollout](../plan.md#L358) · [Stalled-request scenario](../plan.md#L379)

## Mutations

| Scratch change | Observed result |
|---|---|
| Replace the loopback and refusal sites with a catch-all. | `rules_refuse_other_host_names` failed. The suite printed `5 passed, 1 failed` and exited 1. |
| Change the upstream to HTTP/1.1. | `rules_pass_contract_paths_on` failed. The suite printed `5 passed, 1 failed` and exited 1. |
| Delete the initial `*` in the website ignore file. | The allow-list check failed, and `web_image_leaves_out_state` found a decoy in the site stage. The suite printed `6 passed, 2 failed` and exited 1. |
| Delete the runtime `USER` line. | `web_image_carries_the_rules` failed. The suite printed `7 passed, 1 failed` and exited 1. |
| Remove the route-specific timeout. | `stream_survives_upward_silence` failed after 60 seconds and 29 ticks. The suite printed `5 passed, 1 failed, 1 skipped` and exited 1. |
| Widen the route-specific timeout to `/lgtm.link.v1.*`. | `other_routes_keep_the_default_limit` failed because the join probe completed successfully after 81 seconds. The suite printed `5 passed, 1 failed, 1 skipped` and exited 1. |
| Append a second build without `VITE_LGTM_SERVER=/` in the original Dockerfile step. | The image suite printed `8 passed, 0 failed` and exited 0. Chromium showed the address input. See post-a 1. |
