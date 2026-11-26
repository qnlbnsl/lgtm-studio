# Register of review round 1 — go-s1-client-local

Five reviewers attacked the frozen plan `attack/plan-v1.md` alone: architect, critic, logic, logic2 and threat.
They reported 57 findings: 18 blockers, 30 majors and 9 minors.
The planner grouped duplicates into 26 groups, checked each finding against the plan, the design and the repository, and gave each one outcome.
The fold was done on 2026-11-26.

## Every finding and its outcome

A duplicate names the first finding of its group in the column "Repeats".
The group letter links the finding to the points below.

| Finding | Severity | Statement | Group | Repeats | Outcome | Reason |
|---|---|---|---|---|---|---|
| critic-r1 1 | blocker | C11 | A | | fixed | A failed folder sync after placement now returns `ErrPlacedNotSynced`, event files are linked and never replaced, and the outbox stops until it is reopened. |
| architect-r1 2 | blocker | C11 | A | critic-r1 1 | fixed | The same fix. The key and the receipts also say what an uncertain write means for them. |
| logic-r1 1 | blocker | C11 | A | critic-r1 1 | fixed | The same fix, proven by `TestAppendAfterDirectorySyncErrorPreservesEvent`. |
| threat-r1 07 | major | C11 | A | critic-r1 1 | fixed | The same fix. |
| logic2-r1 8 | minor | plan | A | critic-r1 1 | fixed | `ErrPlacedNotSynced` and `TestWriteAtomicReportsPlacedNotSynced`, as prescribed. |
| architect-r1 1 | blocker | C11 | B | | fixed | `Open` syncs the parent of every new folder, and every open syncs the tree again. `TestOpenSyncsNewFolderEntries`. A real loss of power is post-build check 1. |
| critic-r1 2 | blocker | C11 | B | architect-r1 1 | fixed | The same fix. |
| logic-r1 4 | blocker | C11 | B | architect-r1 1 | fixed | The same fix. |
| threat-r1 03 | blocker | C11 | B | architect-r1 1 | fixed | The same fix. |
| architect-r1 3 | blocker | C11 | C | | fixed another way | `Ack` refuses numbers never handed out, and a separate `Reconcile` checks the server's number. The epoch of the outbox replaces the jump of the counter. UNREVIEWED MECHANISM. |
| critic-r1 3 | blocker | C11 | C | architect-r1 3 | fixed another way | The same fix, with `TestNewOutboxKeepsEventsBeforeFirstReconcile`. |
| logic-r1 5 | blocker | C11 | C | architect-r1 3 | fixed another way | The same fix. The identity the reviewer asked for is the epoch. |
| threat-r1 01 | blocker | C11 | C | architect-r1 3 | fixed another way | The same fix, with `TestAckRefusesNumberNotHandedOut` and `TestReconcileCases`, which cover both directions of disagreement. |
| logic2-r1 1 | major | C11 | C | architect-r1 3 | fixed another way | The same fix. |
| logic2-r1 5 | major | C11 | C | logic-r1 5 | fixed | Numbers run from 1 to below 2 to the power 62. Zero is refused, and `Append` stops at the ceiling with `ErrSequenceExhausted`. |
| critic-r1 4 | blocker | R6 | D | | fixed | A damaged receipt becomes `interrupted`, pinned and never pruned. `TestDamagedReceiptIsNeverPruned` uses a clock ten years later. |
| logic-r1 2 | blocker | R6 | D | critic-r1 4 | fixed | Every receipt is checked for its version, name, state and times. An unreadable file makes `OpenReceipts` fail. |
| threat-r1 05 | blocker | R6 | D | critic-r1 4 | fixed | The same fix. |
| architect-r1 4 | major | R6 | D | critic-r1 4 | fixed | The same fix, and a later expiry on a second arrival raises the stored expiry. |
| logic2-r1 11 | minor | plan | E | | fixed | The file `receipts/clock` keeps the highest time seen, as prescribed. `TestClockHighWaterSurvivesRollback`. |
| threat-r1 08 | major | R6 | E | logic2-r1 11 | fixed | The high-water clock covers the change forward and back across a prune. A command expires at the instant of its expiry, `TestBeginExpiresAtTheInstant`. |
| logic-r1 6 | major | R6 | E | logic2-r1 11 | for the owner | Its parts about a clock set back, the instant of expiry and a later expiry are fixed in this group. A client clock that runs behind the server's needs a time authority that the design does not name. Decision 49 and the section "For the owner" give three options. |
| logic-r1 3 | blocker | C11 | F | | fixed | `OpenOutbox` and `OpenReceipts` take the held `state.Folder` and claim their store. A second open returns `ErrStoreOpen`. |
| architect-r1 5 | major | C11 | F | logic-r1 3 | fixed | The same fix. The lock file stays open until `Close`. |
| critic-r1 5 | major | C11 | F | logic-r1 3 | fixed | The same fix, with `TestLockIsFreedWhenHolderDies`. |
| logic2-r1 10 | minor | plan | F | logic-r1 3 | fixed | The same fix. `Read`, `Ack` and `Reconcile` share one lock, and `Read` copies payloads. `TestReadDuringAckNeverFails`. |
| threat-r1 04 | blocker | D19 | G | | fixed | `Open` refuses a parent folder that another user can change, and a lock file that is unsafe. `join` and `run` hold the folder for their whole life. |
| architect-r1 7 | major | D7 | H | | fixed | `server.json` is written last, after the server confirmed the join. A key without it is a join to finish, and the next `join` reuses the key. The rules are in the fold "What plan go-s3-client-link must do". |
| logic-r1 7 | major | C6 | H | architect-r1 7 | fixed | The same fix. A join that stops half-way is post-build check 4. |
| logic2-r1 2 | major | D7 | H | architect-r1 7 | fixed | The same fix. `join` refuses a folder that already holds `server.json`. |
| threat-r1 09 | major | D7 | H | architect-r1 7 | fixed | The same fix. A new enrollment needs a new folder, so a key is bound to one server. |
| architect-r1 8 | major | D7 | I | | fixed | `Load` opens without blocking, checks the open file, reads at most 4,097 bytes, and accepts exactly one PEM block. |
| critic-r1 7 | major | D7 | I | architect-r1 8 | fixed | The same fix, with `TestLoadRefusesNamedPipe`. |
| threat-r1 11 | major | D7 | I | architect-r1 8 | fixed | The same fix. |
| logic-r1 10 | minor | D7 | I | architect-r1 8 | fixed | `ErrKeyUnreadable` wraps the cause, with `TestLoadReportsUnreadableKey`. |
| architect-r1 9 | major | C7 | J | | fixed | Fakes name `sleep` by an absolute path and write a file when they start. All candidates of a harness share one time limit. The child gets the cleaned search path. |
| critic-r1 8 | major | C7 | J | architect-r1 9 | fixed | The same fix. An exit status other than 0 wins over a matching version. |
| logic-r1 9 | major | C7 | J | architect-r1 9 | fixed | The same fix. Verified: a fake with an empty `PATH` exits with 127 at once. |
| threat-r1 06 | blocker | R9 | K | | fixed another way | The scan runs only a program whose file and folders no other user can change, with `ProblemUntrusted` otherwise. The rule is the one OpenSSH applies. UNREVIEWED MECHANISM. |
| threat-r1 10 | major | R9 | L | | fixed | A program counts as installed only when its whole first line matches its harness's pattern. The real lines are post-build check 7. |
| architect-r1 6 | major | C11 | M | | fixed | The fold "What plan go-s3-client-link must do" makes producers keep a refused event and try again, and names one ordered sender. |
| critic-r1 11 | major | C11 | M | architect-r1 6 | fixed | The same fix. |
| threat-r1 12 | major | C11 | M | architect-r1 6 | fixed | The same fix. |
| critic-r1 6 | major | C11 | N | | fixed | The crash tests pin the expected state at each step, the kill loop needs at least 100 numbers, and each item has its own helper test. |
| logic2-r1 7 | major | C11 | N | critic-r1 6 | fixed | `TestAppendReturnsOnlyAfterDurable`, `TestAckDeletesOnlyAfterMarkAtEveryRemoval` and `TestOutboxCrashDuringAck` now make the named mutations fail. |
| logic2-r1 4 | major | R9 | O | | fixed | The credential is a value with only a pointer inside, and `String`, `GoString` and `Format` are on the value. The test covers structs, all verbs and JSON. |
| critic-r1 10 | major | plan | P | | fixed | After `Close`, the accessors return their last values, `Changed()` is closed, and a second `Close` returns nil. |
| critic-r1 9 | major | C7 | Q | | fixed | `TestReadLoadOnMacMatchesSystem` compares with `sysctl` on a real Mac, in phase 5 of sprint 1 or of sprint 3. Decision 54 aligns the timing. |
| logic-r1 8 | major | R6 | R | | fixed | `Finish` stores a result that is too large as `failed`, so the receipt never stays `started`. |
| logic2-r1 3 | major | R6 | S | | fixed | Receipts have the states `done`, `failed`, `interrupted` and `expired`, and an ended command never runs again under its identifier. The result file asks the Contract team for the outcome `interrupted`. |
| threat-r1 02 | blocker | C11 | T | | fixed | The mark carries a checksum, and recovery checks everything before it deletes anything. A missing mark in a used outbox is damage. |
| logic2-r1 6 | major | C11 | U | | for the owner | What to do with a damaged outbox is a choice about risk. Decision 48 and the section "For the owner" give three options. Until the owner rules, the client refuses and deletes nothing. |
| architect-r1 10 | minor | plan | V | | fixed | Verified: `go build -o <file>` refuses several packages. The test now builds all packages without output and then the program with output. |
| architect-r1 11 | minor | plan | W | | fixed | Verified: the scope file exists. The sizing fold says so. |
| critic-r1 12 | minor | plan | X | | fixed | The fixtures of item 4 are named, and every task without a pattern now names one. |
| logic2-r1 9 | minor | C11 | Y | | fixed | The repeat is accepted. `TestOutboxCrashAtEachWriteStep` is named as its proof, and the fold for plan `go-s3-client-link` makes reports safe to apply twice. |
| logic2-r1 12 | minor | D19 | Z | | fixed another way | The default folder for clones is inside the state folder, not beside it. `run` logs the folders, and the installer sets the variable. UNREVIEWED MECHANISM. |

## Points for each statement

A group counts once, at its highest severity, under the statement of its first finding.
Group H counts under D7, because three of its four findings name D7, and logic-r1 7 names C6.

| Statement | Groups and points | Points | Share of the round |
|---|---|---|---|
| C11 | A 9, B 9, C 9, F 9, T 9, M 3, N 3, U 3, Y 1 | 55 | 47% |
| R6 | D 9, E 3, R 3, S 3 | 18 | 16% |
| R9 | K 9, L 3, O 3 | 15 | 13% |
| D19 | G 9, Z 1 | 10 | 9% |
| D7 | H 3, I 3 | 6 | 5% |
| C7 | J 3, Q 3 | 6 | 5% |
| plan | P 3, V 1, W 1, X 1 | 6 | 5% |
| **Total of the round** | | **116** | 100% |

## Stuck statements

C11, "buffered events arrive once, in order", is stuck, with 55 points and 47% of the round.
No other statement reaches 12 points and 30% of the round.

Five blocker groups sit in the mechanism of the outbox: A, B, C, F and T.
The planner considered whether the mechanism is wrong.
The storage of one file for each event is not wrong: no finding came from it, and a log of segments would carry the same four faults.
The interface around it was wrong in one place.
The call `Ack` both acknowledged events and recovered a lost outbox, and that mix is gone: `Ack` now acknowledges only numbers the outbox handed out, `Reconcile` checks the server's number, and the epoch replaces any move of the counter.
The other four groups were missing rules in a sound design: the meaning of an uncertain write, the syncs of new folders, the claim of a store, and a checksum on the mark.
The next round should attack C11 first.

## The patches, by heading of the plan

The next round attacks only these places.

1. Overview: the paragraph on the one way of writing a file, and the paragraph on owner decisions.
2. Key decisions: "A command runs at most once", the new "The outbox deletes only events that it numbered and the server applied", "The harness scan never runs a shell", and "The load on macOS uses the load average".
3. Workstreams: "The machine has a key", "The client can describe its machine", and "Nothing the client must send".
4. Rollout order: steps 5 and 6.
5. For the owner: new section.
6. Implementation details, the rules of every item: the helper tests and the names of the workstreams.
7. Implementation details, the layout of the state folder: `lock`, `machine.key`, `server.json`, `outbox/epoch`, `outbox/acked`, `outbox/<number>.event`, `receipts/clock`, `repositories/`, and the last line.
8. Item 1: the public names, the default folder for clones, `TestBuildsForLinuxAndMacOS` and `TestLoadSettingsMissingFileGivesDefaults`.
9. Item 2: all of it.
10. Tasks of WS-1, done-when, risks and verify.
11. Item 3: all of it.
12. Tasks of WS-2, done-when and risks.
13. Item 4: the files, `TestReadLoadOnMacMatchesSystem`, and the acceptance.
14. Item 5: all of it.
15. Tasks of WS-3, done-when and risks.
16. Item 6: all of it, now named "Event files, the epoch and the acknowledged mark".
17. Item 7: all of it.
18. Item 8: all of it.
19. Tasks of WS-4, done-when, risks and verify.
20. The fold "What plan go-s3-client-link must do": new.
21. The fold "Post-build checks": new.
22. Traceability: source 6 and the four rows of round 1.
23. Execution sizing: the reasons of units 2, 5, 6, 7 and 8, and the line on the scope file.
24. Attack log: the row of round 1 and the known gaps.
25. `decisions.md`: rows 36 to 54.
26. `scouting.md`: the two new sections of round 1.

## The top finding of the round

The top finding is architect-r1 3, a blocker, which four other reviewers found alone: critic-r1 3, logic-r1 5, threat-r1 01 and logic2-r1 1.
An acknowledgement above the next number moved the counter and deleted events that were never sent.
A new outbox could lose its first events before its first greeting from the server, and a wrong number from the server could delete any pending event.

## The verdict and the grade of each reviewer

| Reviewer | Verdict | Grade | Blockers | Majors | Minors |
|---|---|---|---|---|---|
| architect | REVISE | 5 of 10 | 3 | 6 | 2 |
| critic | REVISE | 4 of 10 | 4 | 7 | 1 |
| logic | REVISE | 4 of 10 | 5 | 4 | 1 |
| logic2 | REVISE | 5 of 10 | 0 | 7 | 5 |
| threat | REVISE | 4 of 10 | 6 | 6 | 0 |

## Counts by outcome

| Outcome | Findings |
|---|---|
| fixed | 48 |
| fixed another way | 7 |
| rejected | 0 |
| post-build check | 0 |
| for the owner | 2 |
| not folded | 0 |
| **Total** | **57** |

No finding was its own post-build check, because each had a fix in the plan.
The reviewers' lists of checks that need the code went into the fold "Post-build checks" as ten rows.
