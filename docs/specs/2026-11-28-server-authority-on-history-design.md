# The server is the authority on a machine's history

Date: 2026-11-28.
Status: the rule and the handling of commands are decided by the owner. The plans follow this file.

This design changes one rule of the link between the server and a client.
It comes after the release proofs of sprint 4 and before sprint 5.
It answers issue 1701, and it carries the wording repair of issue 1702, point 1.

## What the rule is

A machine numbers its reports, which the contract calls events.
The server keeps the last number it has applied for each machine. This file calls that number the mark.
Today the two sides stop when their numbers do not fit.
From this design on, a mismatch heals without a person.

| Situation | Before | From this design on |
|---|---|---|
| The client's saved history is older than the server's mark. | The server refuses the client on every attempt, and the website shows "Out of sync". | The server admits the client. The client takes the server's mark and reports its present state. |
| The client's history file is missing, and its key and identity are intact. | The client stops before it dials. | The client starts an empty history, and the first row applies. |
| The server's mark is older than what the client holds, and the client still holds the events in between. | The client replays them. | Unchanged. |
| The server's mark is older than what the client still holds. | The client stops. | The server takes the number the client reports. The client goes on. |
| The client's history file is damaged inside. | The client stops. | Unchanged. A silent repair would hide a failing disk. |

Nothing of the Go rewrite has shipped. This design holds no upgrade path and no handling of an earlier version.

## Decisions

| Date | Decision |
|---|---|
| 2026-11-28 | The server is the authority on a machine's saved history. An older client is brought to the server's state and does not stop. A newer client reports what the server lacks. |
| 2026-11-28 | "Out of sync" stops being a state of a machine. The status and its reason line leave the website. |
| 2026-11-28 | A command that the server handed to an older state of the client, and that has no result, is failed by the server and is never sent again. |
| 2026-11-28 | The person is told about such a command for a short time and can ask for it again. The bell in the top bar is the place, which is a visible change. |
| 2026-11-28 | Events that the client wrote under numbers the server has already used are dropped. The client reports its present state as new events instead. |
| 2026-11-28 | Revision 2 of the link is amended in place. No revision 3 is made for this, because nothing has shipped. |

## How the two sides compare their numbers

The server can decide only when it knows three numbers of the client. Hello carries one of them today.

| Number in Hello | Meaning | Status |
|---|---|---|
| `first_event_number` | The first number this run of the client program will write. It is the run's boundary. | Exists. |
| `acknowledged_event_number` | The highest number the client has forgotten because the server acknowledged it. It is the retained floor minus one. | New. |
| `last_event_number` | The highest number the client has written. It is the next number minus one. | New. |

The server refuses a Hello with `invalid_argument` when the acknowledged number is above the last number, or when the first number is above the last number plus one.

The server decides at the admission of a stream, after the Proof verified, under the lock of the machine, in the write that admits the stream.

1. **The server is behind.** The mark is below the acknowledged number. The server sets the mark to the acknowledged number.
2. **The client is behind.** The mark is above the client's last number. The server keeps the mark and does the reset that the next section describes.
3. **The numbers fit.** Nothing changes, and the stream replays as today.

Case 1 runs before case 2. After case 1 the mark is never above the client's last number.

The Welcome carries the mark as today.
The client reads it by one rule in three parts.

- A mark between its acknowledged number and its last number is the normal case. The client forgets the events up to the mark and sends the rest.
- A mark above its last number means that the server holds more. The client deletes its retained events and sets its next number to the mark plus one, in one write to its history file.
- A mark below its acknowledged number cannot arrive any more, because the server took that number in case 1. A client that still sees one ends the stream and dials again. It does not stop.

After either of the first two, an approved client writes its current Facts and HarnessesFound as new events. It does that on every Welcome today, so the fresh state needs no new step.

## What the server does when the client is behind

This is the reset of case 2. All of it happens in the admitting write.

- **Commands that were handed over and have no result fail.** The server settles each one with the new end state `history_restored`. It never sends such a command again, on any stream.
- **SetState is the exception.** It carries the machine's standing, the Welcome carries the same standing, and repeating it is harmless. It stays pending as today.
- **Commands that wait and were never handed over stay.** They go out on the admitted stream in their order.
- **A result that waits for events is settled with its own outcome.** The server received that result, so the outcome is known. The events it waited for do not come any more.
- **A folder look without its report fails.** The report of a look cannot be made again. The open look ends as a failure, and a website call that waits for it gets an error.
- **The run is ready at admission.** Its boundary lies at or below the mark. A boundary that an earlier run left pending is dropped.
- **The server writes one log line** with the machine, the mark and the client's last number.

The reset is safe to repeat. A client that lost the Welcome and dials again meets no command that could still fail.

The rule on an unseen run changes with this.
An unseen run whose boundary is at or below the mark is admitted and is ready at once.
It is not refused, and no warning is set.

## What the server does when it is behind

This is case 1. It happens in the admitting write as well.

- The mark becomes the client's acknowledged number.
- A waiting result whose awaited number is now at or below the mark is settled with its outcome.
- No command fails. The client's receipts keep a command from running twice, by the rules that hold today.
- The server writes one log line with the machine, the old mark and the new one.

The effects of the events in the gap are lost with the server's older database.
The client's fresh Facts and HarnessesFound bring the server to the present.
A machine can raise only its own mark, and only with its own key. The security list holds the note on that.

## What the client stops doing

- The client no longer stops on a Welcome mark outside its range. The stop reason `impossible_ack` goes away.
- A missing history file with an intact key and identity is created empty. The client then dials, and case 2 brings it to the mark.
- A history file that cannot be read, or that fails its own checks, still stops the client. The log line and the exit status of each such case stay as they are today.
- The run's boundary does not change within a process. A reconnect repeats it, as today.
- Receipts stay as they are on disk. They expire by the rules that hold today.

## What leaves and what arrives on the website

**Leaves:** the status "Out of sync" of a machine and its reason line.
The field `event_history_needs_recovery` of the website contract is removed, and its number 19 is reserved.
The server's warning flag and its column go with it.

**Arrives:** a failed command that a person asked for shows on the bell in the top bar.

- The bell counts it, in the tone of an error.
- The bell's menu lists it as one entry. The entry's label is the action that asks again, such as "Scan harnesses on build-box again" or "Restart build-box again". Its second line says that it did not finish, and how long ago.
- Choosing the entry makes the same call as the first time.
- The entry leaves ten minutes after the failure. It leaves earlier when the same kind of command is asked for that machine again, or when the machine is removed.

The website contract carries this as a list on the machine: one entry for each such command, with its kind and the time of the failure.
The kinds of today are the harness scan and the restart.
A folder look is not listed, because the "Add repository" dialog that asked for it shows the failure itself.
SetState is not listed, because no person asked for it.
The server derives the list from its command records and keeps no second store.

## What changes in the contract text

- `events.proto`: the paragraph that calls a Welcome mark outside the client's range incompatible is replaced by the three-part rule above. The sentences that tell a person to restore the prefix or to enroll again, and that put a server rollback outside wave 1, go away.
- `link.proto`: Hello gains the two numbers. The rule on an unseen run is replaced. The sentence on history refusal goes away. The cases 1 and 2 are written into "Run admission and readiness".
- `link.proto`, for issue 1702, point 1: the sentence on the limit of 65,536 bytes says that the limit applies to the size on the wire and to the size after decompression.
- `commands.proto`: the life of a command gains the end state `history_restored` and the exception of SetState.
- `clients.proto`: field 19 is reserved, and the list of failed commands is added to `Client`.
- The frozen record of the link keeps revision 2 with the amended text. The history base for the comparison moves to the commit that amends it.

## What this replaces in earlier documents

- The design of sprint 4, `docs/specs/2026-11-27-running-an-agent-design.md`: the decision of 2026-11-27 on the status "Out of sync", and the parts of the proofs R5, R6, R9, R10 and R11 that expect a refusal, a warning or a stopped client.
- The foundation design, `docs/specs/2026-11-25-go-rewrite-foundation-design.md`: the sentences that put a restored history outside wave 1.

The rule that a result waits for the events before it stays. So does the rule that no command is sent to a run before its past is applied.

## How it is proven

Each line is one observation. The paired tests run a real server and a real client.

| Check | What is observed |
|---|---|
| H1 | A client whose history file was replaced by an older copy becomes Online without a person. The server's mark is unchanged, and the client's next number is the mark plus one. Its fresh Facts and HarnessesFound are applied. |
| H2 | A client whose history file is missing, with its key and identity intact, starts and becomes Online. |
| H3 | A scan that was handed over before the restore, and has no result, ends as `history_restored`. No later stream carries its id. A scan that waited and was never handed over is sent and finishes. |
| H4 | A restart that was handed over before the restore is not sent again. The client process is not asked to end a second time. |
| H5 | A result that waited for events at the restore is settled with its outcome. A folder look without its report fails, and the waiting website call gets an error. |
| H6 | A server whose mark is older, within what the client retains, gets the events replayed. This holds today and stays green. |
| H7 | A server whose mark is older than the client's retained floor takes the client's acknowledged number. The client does not stop and becomes Online. |
| H8 | A client that lost the Welcome of a reset dials again. Nothing more fails, and the bell holds no second entry. |
| H9 | A Hello whose acknowledged number is above its last number is refused with `invalid_argument`. So is one whose first number is above its last number plus one. |
| H10 | A history file that fails its own checks stops the client with the log line and the exit status that it has today. |
| H11 | The website shows no "Out of sync" status and no reason line for any machine. |
| H12 | After H3, the bell counts one and lists the scan. Choosing the entry calls the scan again, and the entry leaves. An entry that nobody chooses leaves after ten minutes. |

The tests that pin the old rule are replaced, not kept beside the new ones.
They are the paired test `TestSprintFourHistoryRecovery`, the client tests on a terminal history and on impossible marks, the server tests on the recovery warning and on run admission, and the two browser files on machine recovery.

## How the work is cut

Four small plans, in this order. Each follows the build loop: tests first and frozen, then the build, then a review.

1. **Contract.** The four files of the contract, the frozen record and the history base.
2. **Server.** The comparison at admission, the reset, the raised mark, the end state, the list of failed commands, and the removal of the warning.
3. **Client.** The two numbers in Hello, the reading of the Welcome mark, the empty history for a missing file, and the removal of the stop.
4. **Website and paired tests.** The bell, the removal of "Out of sync", and the paired tests H1 to H8.

The server and the client can be built side by side after the contract.

## What is left out

- A journal of events on the server. The server keeps one mark for each machine, as today.
- A repair of a history file that is damaged inside.
- A way to bring back the report of a folder look that was lost.
- Any handling of a client or a server of an earlier revision.
