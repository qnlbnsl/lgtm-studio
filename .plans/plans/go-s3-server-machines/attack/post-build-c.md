# Post-build review C of the plan go-s3-server-machines

Reviewer: Claude Sonnet 5.5, high effort, cross-vendor review of built code.
Task identifier: postbuild-go-s3-server-machines-c. Nonce: 77393d.
Scope: item 2 (write deadlines on the long streams and the read limit of the link), item 5 (the projection and `WatchClients`), and the repair of finding post-b 1 (a malformed Join echoes nothing).
Code read: `git diff 4ccca5a6..HEAD` on branch `sprint-0` for the six named files. The uncommitted work of the builder in `src/server` is not reviewed.

## Verdict

REVISE.

## Top finding

The last write of a `WatchClients` stream, which carries the closing error or the end-of-stream envelope, has no write deadline, so a client that stops reading holds the handler, the request and the server shutdown indefinitely (post-c 1).

## Findings

### post-c 1, MAJOR, the end of a watch is not bounded by a deadline

Statement: `transport.Send` clears the write deadline when each send returns, and `WatchClients` never sets a deadline again before it returns, so the protocol's final write has no bound.

Evidence:
- `src/server/internal/transport/deadline.go` line 60 clears the deadline with `deadline.set(time.Time{})` after every `Send`.
- `src/server/internal/machines/watch.go` returns `watchError(err)` after `watch.Serve` without calling `transport.BoundLinkEnd`. The helper exists at `src/server/internal/transport/link_stream.go` line 92 and is documented as the bound for terminal trailers or the terminal envelope.
- Connect writes the end-of-stream envelope after the handler returns, and a data frame waits for HTTP/2 flow-control credit.

Failure scenario: a signed-in browser opens the watch, receives a message that nearly fills its receive window, and stops reading. The session is revoked, or the feed lags. `watch.Serve` returns an error, Connect writes the end envelope, the write blocks on the exhausted window, and nothing ends it.

I reproduced this in a scratch copy with a window of 64 bytes and a first message of 60 bytes. With a 200 ms deadline, the handler returned in about 12 ms, but the outer `ServeHTTP` was still running after five seconds. When the handler called `BoundLinkEnd` with 200 ms, the outer `ServeHTTP` returned after about 216 ms.

This breaks the UNREVIEWED FIX statement of the plan that a blocked send ends at its deadline at the latest, because the blocked write here is not a send. It also breaks the owner's check that a blocked reader cannot hold a handler past its deadline. A stuck handler also keeps `http.Server.Shutdown` waiting for its whole drain time.

Fix: call `transport.BoundLinkEnd(ctx, transport.SendTimeout)` in a `defer` at the start of `WatchClients`, so every return path arms a deadline before Connect writes the end. Alternatively, let `Send` leave an armed deadline instead of clearing it, and re-arm it while the stream is idle.

### post-c 2, MAJOR, no test fills the window before the end of the stream

Statement: a wrong implementation that leaves the tail write unbounded passes every test of these items.

Evidence: `TestWatchClientsBlockedReaderEnds` in `src/server/internal/machines/watch_test.go` blocks the reader during many changes, so the blocked write is always a send inside `Send`. `TestDeadlineEndsBlockedStream` in `src/server/internal/transport/deadline_test.go` and `TestRouterWrapsLongStreams` in `src/server/internal/api/long_stream_test.go` also block only inside `Send`. I ran the plan's tests against the current code and they pass, and the code of post-c 1 is that wrong implementation.

Failure scenario: a later edit removes `BoundLinkEnd` from a handler, or a new long stream is added without it, and no test turns red.

Fix: add a test that exhausts the client window with the last send, then makes the handler return an error, and requires the outer `ServeHTTP` to return within the deadline plus a margin. The test I used in the scratch copy has this shape.

### post-c 3, MAJOR, the shared-follower path of `WatchClients` has no test

Statement: no test in `src/server/internal/machines` or `src/server/internal/api` builds the service with `WithFollower`, although this is the only path production uses.

Evidence: `src/server/internal/app/start.go` creates the follower on the server lifetime and passes it with `WithFollower`. `src/server/internal/machines/watch.go` lines 28 to 37 close the follower only on the standalone path. A search of the test files for `WithFollower` finds nothing.

Wrong implementation that every test accepts: move `defer follower.Close()` outside the `if follower == nil` block. The standalone path behaves the same, so the tests pass. With the shared follower, the first watch that ends closes the feed for every other watch and for the server.

Fix: add a test that builds the service with a shared follower, ends one watch, and requires a second watch to keep receiving changes and the follower to stay open until the owner closes it.

### post-c 4, MINOR, a standalone service starts one follower per watch with no outage report

Statement: without `WithFollower`, every call starts its own follower with a nil report function, so outages of the feed are not logged and each watch holds its own reader, listener connection and ring.

Evidence: `src/server/internal/machines/watch.go` line 32 calls `store.StartFollower(ctx, s.store, nil)`. The store has one page connection, `pagePool.MaxConns = 1` in `src/server/internal/store/pool.go`, so many standalone watches contend for it.

Failure scenario: a deployment or test harness builds the service without `WithFollower`, and many signed-in browsers open watches, so the cost grows with the number of watches.

Fix: either require a follower in `NewService`, or document that the standalone path is for tests only and give it a report function that reaches the log.

### post-c 5, MINOR, at the committed state a watch does not end when the server stops

Statement: at the reviewed commit, `WatchClients` is tied to the request context only, so a graceful stop waits for the whole drain time for every open watch.

Evidence: `serve` in `src/server/internal/app/start.go` calls `httpServer.Shutdown` with `drainTimeout`, which is five seconds, and only afterwards does `release` close the follower. No watch observes the server lifetime. The uncommitted work of the builder adds `WithWatchShutdown`, which addresses this, and I did not review it.

Failure scenario: any open browser makes each stop take five seconds, and a stuck tail write of post-c 1 makes it end only by `Close`.

Fix: land the shutdown wiring with a test that stops a server with an open watch and requires the stop to finish well inside the drain time.

### post-c 6, MINOR, the redaction applies only to unary calls of the client service

Statement: a malformed JSON request of the server-streaming `WatchClients` keeps Connect's own decoding error text.

Evidence: `src/server/internal/api/server.go` lines 138 to 143 add `redactMalformedJSON` only when `spec.StreamType` is unary. The interceptor in `src/server/internal/api/session_decode.go` is a unary interceptor, so the limit is deliberate. I did not measure what the error text contains for a stream.

Failure scenario: an authenticated caller sends a malformed JSON body to `WatchClients` and receives the parser's wording of the submitted bytes. The caller already holds the session, and the message is empty, so the exposure is small.

Fix: either state in the plan that streams are out of scope for the redaction, or decode the empty `WatchClientsRequest` through the same codec in the stream path.

### post-c 7, MINOR, the grpc JSON content types are not tested for the Join redaction

Statement: `TestMalformedJoinRedactsSubmittedBytes` in `src/server/internal/api/join_decode_test.go` sends only `Content-Type: application/json` and only the fields `tokenSecret` and `machinePublicKey`.

Evidence: `isMalformedJSON` in `src/server/internal/api/session_decode.go` lines 41 to 54 also accepts `application/grpc+json` and `application/grpc-web+json`, and a charset parameter.

I checked these by a scratch probe against the real router: `application/json` with seven spellings of the parameter, `application/grpc+json`, `application/grpc-web+json`, `application/connect+json`, `application/grpc` and `application/proto`. No spelling leaked the submitted bytes. The binary types `application/grpc` and `application/proto` return Connect's parser text, which holds no submitted JSON text. Removing the two grpc cases from the switch in a scratch copy also passed every test and showed no leak, because the Join handler gives the same fixed sentence for the emptied message. The media type check is therefore not provably load-bearing for Join, and a future change of the Join validation could expose the gap.

Fix: add the grpc JSON types and a charset spelling to the test table, so that the redaction is tested for every spelling the check accepts.

### post-c 8, MINOR, the acceptance regex of the plan runs a test of another plan

Statement: the command `go test -run 'TestProjectClient|TestWatchClients' ./src/server/internal/machines/` also matches `TestWatchClientsStreamPresence`, which belongs to the stream plan.

Evidence: that test failed with the sentence that the Machines service did not expose its planned link loader. This is work of another team, and I did not count it. The six tests that the plan names for these items pass, also under the race detector, and `go vet` is clean.

Fix: anchor the regex to the six test names in the plan's acceptance row.

## What I checked and found right

- A revoked session ends a watch. Replacing `Authorize` with a function that returns nil makes `TestWatchClientsNeedsLiveSession` fail with a change delivered after revocation.
- Omitting `position` from the response makes `TestWatchClientsFirstMessageAndChanges` and `TestWatchClientsPositionAlwaysPresent` fail.
- Removing the write-deadline wrapper from the client service makes `TestRouterWrapsLongStreams` fail with the sentence that the response has no write deadline setter.
- Removing the link read limit makes the Join over-limit case of `TestRouterWrapsLongStreams` fail.
- Removing the cancellation callback or the clearing step in `Send` makes `TestSendEndsOnCancel` and `TestDeadlineClearedAfterSend` fail.
- A content type cannot bypass the Join redaction in the matrix I ran (post-c 7).

## Grade

6 out of 10. The projection, the session check and the decoding repair are sound and well tested, but a stuck reader still holds the end of a watch, and no test guards that path or the shared-follower path.
