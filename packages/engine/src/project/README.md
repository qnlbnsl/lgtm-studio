# Project data, Git and lifecycle

## Typed task and file context

`project.context.search` searches Tasks and Files for one readable project.
Its payload contains `projectId`, the assigned `worktreeId`, and a literal `query`.
An optional `teamId` restricts tasks and must match that team's workspace.
An optional `scope` selects `tasks` or `files`.
The default result limit is 100, and a smaller `limit` is allowed.
The response contains `result.items` and `result.truncated`.
Each item has a display `label` and a typed `reference`.
An agent receives the same result inside the existing untrusted-data envelope.
An unavailable search refuses safely rather than returning an empty successful result.

A task reference contains `_tag: "task"`, `projectId`, and `taskId`.
A file reference contains `_tag: "file"`, `projectId`, `repoId`, `worktreeId`, and a relative `path`.
The resolver in `context.ts` checks these identities at prompt or board-post admission.
Every agent who can currently receive the conversation must be able to read the project.
The sender must also have current project read access.
Services and caller-less reactors cannot admit new references.
File admission reuses the registered local Git workspace and common-directory checks.
It rejects missing files, metadata paths, traversal, and symbolic links.
It never reads file contents.
Remote file search is unavailable before machine pairing.

Search uses actual tracked and nonignored untracked Git paths.
It inherits the Git subprocess byte and time budgets and the 200-file candidate limit.
The complete `result` object has a 65,536-byte budget, including `items` and `truncated`.
The count uses JavaScript Object Notation (JSON) encoded in the eight-bit Unicode Transformation Format (UTF-8).
This budget excludes the reply's `requestId` and the agent envelope.
The truncation flag distinguishes a bounded search from a complete empty result.
Secret-bearing identities cannot be selected.
Display labels pass through the production redactor.

`prompt` and `board.post` accept an optional `context` array.
Each send accepts at most 16 references and 16 kilobytes of serialized identities.
The ordinary command receipt preserves the admitted array and delivery identity on retry.
The durable queued prompt, sent prompt, board post, and board wake retain typed references.
A wake has a bounded manifest assembled from its admitted posts and completion replies.
Dispatch verifies that manifest rather than resolving additional context without a caller.
Canonical descriptions reach the runner within an untrusted-data envelope.
Owner text and the existing sender envelope retain their original handling.
Old events and payloads without the optional field remain readable.
The future React composer consumes this surface separately.

## The public local Git commands

The project piece keeps the existing repository, location, workspace, project,
team, sprint, and task records. A successful Git state refresh can finish pending
attempt records without performing a Git mutation.
Git mutations have ordinary command receipts. Repeating a committed request
identifier returns its original receipt, without repeating the Git mutation.

The owner first saves `project.settings.set`, including the trusted absolute
workspace folder. `project.repository.discover` searches that folder.
`project.repository.add` and `project.repository.locate` validate real local
Git checkouts. `project.workspace.register` checks the actual repository,
branch, and checkout type before recording its identity. These registration
commands accept paths. Subsequent commands use registered identities.

`project.repositories` and `project.workspaces` page the durable records.
`project.branches` and `project.checkouts` enumerate actual Git branches and
checkouts by repository identity. A record outside a changed workspace folder
is preserved, but Git operations on it refuse. The owner can restore the
workspace setting. A branch changed outside LGTM Studio requires restoring the
recorded branch before that workspace's Git operations can proceed.

The workspace commands take `worktreeId`. They are `project.changes`,
`project.diff`, `project.history`, `project.commit.files`, `project.git.state`,
`project.stage`, `project.unstage`, `project.stage.all`, `project.unstage.all`,
`project.discard.plan`, `project.discard`, `project.commit`, `project.fetch`,
`project.pull`, `project.push`, `project.integrate`, `project.review`,
`project.conflict`, `project.conflict.choose`, and
`project.conflict.resolve`. The contracts define every request and reply.
The command-line interface accepts their spaced and dotted names, with the
same payload supplied as `--input` JavaScript Object Notation (JSON).

Only the owner can mutate Git. Agents can read changes, diffs, history,
commit files, Git state, review evidence, and conflicts in their own project.
An agent supplies its `projectId` as well as the workspace identity.
The workspace must belong to that project or one of its registered teams.
Agent replies use the existing plain-text data envelope.
No remote machine is accepted before machine pairing is implemented.

The queue belongs to one command runtime. Its service instance is reused for
one trusted workspace setting. A finite outer gate spans service replacement,
including changes between overlapping roots. Waiting requests can be cancelled.
Each active service operation retains its own time budget. A request can call
several service operations. The gate is released when the active request finishes.
This can defer cancellation of an active read until those operations finish.
A queued request whose workspace setting changed refuses and asks for refresh.
The service has no second queue. Its process deadlines remain intact.

Every display string passes the production redactor. Fixed conflict segment
kinds, hunk choices, and the review result kind retain their protocol meaning.
The serialized reply, including an agent's data envelope, fits 256 kibibytes.
A larger reply refuses with a fixed recoverable reason. The Git service also
bounds each subprocess, patch, and collection. Raw subprocess errors and remote
addresses are never attached to public failures.

Integration requires a registered team source and its project destination.
The service refuses a dirty source or destination and direct integration into
the repository's recorded base branch. Conflict choices carry a fingerprint and choices, without
client-supplied write content. The service reconstructs current canonical hunks.
The visible owner review dialog uses bounded local comparison data and shows the prepared delivery mode.
The hosted title and description come from the project name, goal, and reported evidence.
Manual editing and replacement generation remain unavailable.
The unused external compare-link route has been removed.

## Explicit owner review and merge to the recorded base

`project.review.prepare` takes `projectId`, its assigned `worktreeId`, and the
registered repository base's `destinationId`. It reads complete bounded Git
comparison data and records an open review with both heads.
It does not approve or merge.
When a GitHub.com remote and a signed-in `gh` are available, it pushes the reviewed project branch and creates or finds its open pull request.
The optional typed `delivery` field records the mode.
A GitHub delivery includes the remote, repository, pull request number, URL, state, title, description, and remote base head.
A local delivery contains a sentence explaining the fallback.
Old events without that optional field retain local merge behaviour.
Prepare is a durable external operation because a push or pull request creation can outlive a missing receipt.
There is no force-local setting.
Refresh always reads Git state before considering a new Prepare.
It prepares again only when the project head differs from the recorded reviewed head.
A failed Prepare is reported without discarding the refreshed Git state.
GitHub connectivity or ambiguous authentication failures refuse the request instead of enabling a local merge.
A new owner approval can reconcile a confirmed GitHub merge after a lost receipt.
The live base head comes from GitHub's reference endpoint rather than pull request metadata.

Hosted comparisons use the fetched live remote base rather than the local checkout's head.
A dirty or outdated local base does not block hosted approval.
The commit list, file totals, and patch are calculated against that fetched commit.
Reported test and review results remain reports for the project head.
Approval checks remote base movement and updates the local checkout only afterwards when safe.
The fetch updates only the explicit remote-tracking base reference.
An unverified provider failure leaves the review open for a fresh owner request.
Prepare records an exact-head pull request already merged elsewhere, including after a prior refusal.
That reconciliation can conditionally fast-forward the local base without another hosted merge.
Reusing an open request refreshes its redacted description through standard input.
A canonical repository-name change gives an explanation to update the remote URL.
Renamed remotes must be updated because delivery does not follow redirects.
The transport can refuse the redirect before provider metadata is available.
An explicit create refusal explains that the branch was pushed and the CLI reported a creation failure.
The origin GitHub remote is preferred.
A sole GitHub remote with another name is also supported.
Several GitHub remotes without origin require the owner to resolve the ambiguity. Binary or truncated comparison data refuses. A newer preparation
supersedes the previous open review for that workspace.

`project.review.report` stores a reported test or reviewer outcome for the
project workspace's actual current head. It takes `kind` of `tests` or `review`,
`status` of `passed` or `failed`, a summary, and a genuine `threadAgentId` and
`threadSequence`. The thread item must exist and belong to that project.
An agent can report only against its own project thread. The owner can report
against an existing member thread. The authenticated reporter and time are
recorded, and provenance is always `reported`, not engine-executed proof.
The projection keeps the latest report per kind and workspace. The log keeps
history. `project.review.status` returns that evidence and the latest review.
Reports for another head or an unavailable thread are labeled stale.

Only the owner can call `project.review.prepare` and `project.review.approve`.
Approval takes the open `reviewId`, `expectedHead`, `expectedBaseHead`,
`confirmed: true`, and an explicit `decision` of `approve` or
`approve_without_evidence`. Neither decision imposes a passing-test or reviewer
policy. Missing and failed evidence remain visible in the recorded review.
Reports, compare URLs, and agent verdicts never supply approval authority.
The service checks the authenticated owner again before execution. It validates
both reviewed heads and branch identities, and exact common repository metadata.
The local fallback also requires clean source and destination checkouts with no active merge.
A GitHub approval checks the pinned pull request and remote base before requesting a merge commit with the reviewed head condition.
Closed pull requests, conflicts, and merge states that are not ready refuse with a sentence reason.
The remote base must still match the fetched base recorded at Prepare.
The local base may remain behind that fetched base during hosted delivery.
It fetches and fast-forwards a clean local base checkout on its registered branch after the remote merge succeeds.
It preserves a dirty checkout or a checkout on another branch.
The optional `localSync` reply explains whether that update completed.
A failed local update still records the confirmed remote merge.
The `project.integrate` and `mergeTeam` refusal protects the repository's recorded base branch.

The approval reply has an explicit `outcome` of `merged` or `refused`, with a
fixed safe `reason` on refusal. A refusal is not a successful merge. The log and
receipt retain the reviewed heads, decision, and outcome. An unknown outcome
asks the owner to refresh Git state. A changed body refuses.
A known refusal consumes that review; obtain a new review and approve explicitly
after correcting the cause. Conflicted destination bytes and merge state remain
inspectable through the existing Git operations.

Git and the database are not crash-atomic. A crash after a Git effect but before
the receipt commits can leave an uncertain outcome. Inspect actual refs, working
bytes, and merge state before preparing another review. A changed base head
refuses the old execution rather than silently approving the new comparison.
The engine gate does not exclude terminal operations by the same Unix user.
The existing local filesystem race limitation still applies.

## Real project creation and lifecycle

The owner saves `project.settings.set` with a workspace folder and shell.
The optional `lead` defaults select a harness, an account, a model, an effort,
and a permission ceiling.
Old settings without `lead` still replay.
Creation chooses a locally ready enabled account using the normal account ordering.
Readiness requires an executable Agent Client Protocol adapter and available credentials.
It does not assert a provider usage balance or successful remote authentication.
The production runner still performs its ordinary session handshake.
A saved named account is never silently replaced by another account.

The owner calls `project.create` with a stable request identity, a name, a goal,
`machineId: "local"`, and either an existing repository identity or a new repository name.
Only the confirmed local machine is implemented.
Every project has its own dedicated worktree.
`useWorktree: false` is refused, including by the preview read.
New repositories live beneath the configured workspace folder.
Worktrees live beneath the repository's `.worktrees` directory.
Names collide case-insensitively within one repository and machine.
Branch and directory collisions receive a numeric suffix.
There is no main-checkout project option.
New repositories receive an empty initial main commit containing no user files.
The main checkout stays recorded separately for owner review and approval.
Unfinished historical main-checkout intents stay readable and reserved, but cannot resume effects.
The owner can explicitly abandon them without changing their files.

`project.placement.preview` accepts the same placement input as `project.create`.
It calls the same authoritative selector against the current repository and filesystem.
It returns the observed repository folder, checkout folder, branch, and checkout type.
The page always previews Branch and Folder.
It returns `reserved: false` because creation must check the placement again.
It creates no intent, directory, branch, receipt, account login, or lead.

The durable creation intent is specific to this repository, checkout, and lead journey.
It is not a general workflow queue.
The engine serializes project creation and archive independently of caller connections.
The normal start planner commits the lead and project in the same transaction as the receipt.
The normal runner then opens its session and delivers the initial goal prompt.
A failed adapter setup leaves one visible stopped lead, rather than creating a second one.
Later child start events inherit project membership.

## Recovering unfinished creation

The owner reads `project.creations` through the authenticated client application
programming interface or the command-line interface.
It returns only unfinished intents, using the ordinary bounded cursor pagination.
Each intent includes the original request identity, request hash, selected defaults,
repository and workspace identities, exact paths, and committed directory claims.
There are at most 64 unfinished intents.
Completed creations use ordinary durable receipts instead.

Retry the original `project.create` request with the same request identity and body.
The engine reuses the recorded branch, paths, and defaults.
A filesystem witness permits recovery if mkdir succeeded before its claim was logged.
The engine refuses unknown, replaced, or redirected directories.
It verifies both the linked checkout and the actual Git branch before launch.
It appends an exact local exclusion rule without rewriting existing exclusion entries.
It never deletes checkouts, branches, unrelated files, or exclusion entries.
Only its exact directory witness is removed after the ownership claim commits.

A database write failure latches the writer until service restart.
Read the pending intent before restarting, and retry the original request after recovery.
If the final project and receipt were committed, restart recovery uses the same lead identity.
Caller cancellation within the running service does not cancel engine-owned creation.
A retry waits behind the existing operation and returns its receipt.
A changed request body under the original identity is refused.

A crash between mkdir and the witness write cannot establish ownership safely.
The planned path remains visible, but automatic recovery refuses it.
The owner must inspect that path and resolve it before retrying.
Unverifiable partial Git metadata receives the same conservative treatment.
The engine does not claim that an error rolled filesystem effects back.
These rules retain the repository's existing local filesystem race limitation.

## Editing and archiving projects

`project.rename`, `project.goal.set`, and `project.preferences.set` edit durable project metadata.
Renaming does not change the branch or folder.
`project.archive` first marks the project as archiving, blocking new children and resumes.
It confirms a normal runner stop for every member under that agent's normal send lane.
It applies the ordinary stop events, including permission cancellation and wait checks.
It preserves questions.
The archived timestamp commits only after every process stop succeeds.
A failed stop keeps `archiving: true`; retry archive after resolving the stop failure.
`project.restore` clears the archived state without launching a lead or resetting setup.
An incomplete archive must finish before restore.

## First-run progress and discovery

The owner reads `project.setup` and advances with `project.setup.advance`.
Progress moves through Welcome, Found, and Project without moving backward.
`project.setup.complete` records positive completion against a successfully created project.
Its timestamp survives restart and the archive of every project.
Authentication remains the prerequisite for these owner-only commands.

`project.setup.discover` reads the configured workspace and supported local harnesses.
It returns actual repositories, install guidance, and the normal ready launch defaults.
Its fixed failure asks the owner to retry or change the workspace through `project.settings.set`.
The scan never materializes a Codex shadow home, installs software, changes login state, or starts an agent.
Local account results distinguish a missing executable, signed-out state, unavailable evidence, and a recognized local credential.
Every account result says remote authentication was not checked.
The scan and project creation share the provider-specific effective-home and credential rules.

Owner settings include a durable, editable `displayName`.
The settings read and discovery can suggest the actual configured Git `user.name`.
An absent suggestion remains absent.
The name is redacted display prose and grants no authority.

## Repository mock revisions and approval

An owner or an agent in the project records a complete bounded bundle with `project.mocks.record`.
The caller supplies the expected previous revision, preventing lost concurrent updates.
The request identity becomes the immutable revision identity.
An existing revision identity cannot be reused by another caller, even after the head advances.
Identical retries by the original caller still return their durable receipt.
The event records the actual author, timestamp, artifact fingerprints, and field comparisons.
`project.mocks.read` pages revisions within 256 kibibytes, including the agent data envelope.
`project.mocks.artifact` returns bounded, redacted repository text or an explicit unavailable result.

Sources distinguish screens, data, tokens, and component indexes.
Each source names a registered project checkout and a confined relative file.
Screen files are data artifacts of at most 8 kibibytes.
Data and component indexes require JavaScript Object Notation files with the same bound.
Tokens can also name a Cascading Style Sheets file, returned only as text data.
Draft fields can name their source, entity, and affected screens.
The engine does not interpret data artifacts as an application schema.
The optional `bundle.reviewedComparisons` array uses the existing `MockFieldComparison` type.
Each row contains the exact complete drafted `field`, a `status`, and an optional `known` type.
Each field can have at most one review, and every review must match its draft before and after redaction.
New, Matches, and Differs require an entity and a data source.
Matches requires `known` to equal the drafted type, and Differs requires a different `known` type.
New and Unavailable omit `known`.
Fields without review metadata report Unavailable, regardless of the artifact's contents.
Reported comparisons are claims reviewed by the recorded author, not engine verification of an application schema.
The stored `reviewedComparisons`, author, and timestamp identify this provenance.
Historical revisions without this optional input retain their original stored comparisons during reads and replay.
Approval derives its implementation changes from the recorded comparisons and still rechecks actual artifact fingerprints.
Draft field identity consists of its entity and field name together.
The removed experimental `bundle.schemaChanges` input is explicitly forbidden at admission.
It cannot be silently stripped as an excess property.
Declared removals are unsupported; this contract supplies reviewed additions and type changes.

Screen routes and preview origins are validated metadata.
Each screen records routes only for the typed scenarios it supports.
Scenario display names never determine these bindings.
The engine refuses unknown or duplicate bindings, external routes, and brace placeholders.
The engine never fetches a preview origin, evaluates a component, or starts a preview command.
The web client must keep preview code outside its application authority.
Component references require matching repository byte fingerprints.
Their screen references must identify screens in the recorded bundle.
Omitted screen references mean that the relationship was not reported.
A change from Draft to Shared also requires a different evidenced repository reference.
An in-memory provenance flag does not establish that a file was moved.

Projects without screens begin in Plan and omit Design.
Recording screens moves the project into Design.
Stage-derived team labels use their areas to distinguish teams at the same stage.
Only the owner can approve the current revision through `project.mocks.approve`.
Approval rechecks the actual source and component fingerprints and refuses changed or unavailable evidence.
It records the reviewed revision, screens, remaining draft components, fields, and derived schema changes.
It advances to Plan and queues one durable handoff to the recorded project lead.
Retries use the normal durable receipt; fresh duplicate and stale approvals refuse.
The strict mock-specific reach predicate rechecks the project, approval, revision, and lead during later dispatch.
An obsolete queued handoff is withdrawn.
The handoff creates no fixed teams, tasks, or conversations.

## Registered repository Design evidence

`project.design.record` records an explicit repository, registered workspace, and relative design file.
Its revision identity is unique within the repository across all authenticated callers.
The caller must own the project or be one of its agents.
Registration checks the workspace's real checkout, branch, and common Git directory.
It requires an actual bounded `.fig`, `.pen`, or `.json` file of at most one mebibyte.
It never invents a path or launches an editor.
An optional token document contains at most 64 named values in 8 kibibytes of JSON.
Its path must pass public admission as a confined relative JSON-file path.
Raw secret probes, traversal, absolute paths, and credential-file references refuse before a recorded event.
Each optional shared component names an actual repository reference with a matching byte fingerprint.
Each component file is bounded to 64 kibibytes.

`project.design.read` takes the repository identity and an optional exact revision identity.
An agent must also identify its own project in that repository.
The read preserves the registration, authenticated author, event time, and recorded fingerprints.
It compares the current file, tokens, and component bytes with their recorded fingerprints independently.
Changed, missing, redirected, or inaccessible evidence returns explicit artifact unavailability.
An absent registration returns explicit repository Design unavailability.
Available tokens contain actual bounded, redacted values.
No availability result asserts that an editor or isolated preview can run.

A mock bundle may explicitly name `designRevision`.
The revision must belong to the project's repository.
Recording and approving the mock both recheck all required Design evidence against that exact revision.
An omitted Design identity stays absent.
The page must never derive a design file path from a repository or component display name.

## Git attempts and uncertain outcomes

Git mutations use the existing event log for a bounded attempt before possible effects.
The attempt binds the caller, request, canonical body hash, operation, workspace, and pre-effect Git identities.
It stores no raw payload or commit message.
The request lane covers a short writer preflight, Git under the runtime gate outside the writer, and a short final outcome transaction.
A Git-proven conflict, rejected push, diverged pull or remote refusal retains its fixed reason and durable refused outcome.
Only explicit network commands can classify fixed remote-refusal diagnostics; a local permission or mixed-reference failure after possible effects stays uncertain.
A conflict requires both MERGE_HEAD and actual unmerged entries.
A genuinely unknown possible effect returns “Git outcome is uncertain; refresh Git state” without a success or refusal receipt.
An identical retry never clears that attempt merely because the current state appears unchanged.
Prior attempts return uncertainty without re-executing commit or approval, including before a refresh.
A new owner approval request can use actual containment of both reviewed ancestors in the registered destination, without another merge.
Only a project.git.state read that observes a pending attempt takes the workspace operation lane before reading Git and finishing attempts in short current-binding transactions.
An ordinary state read does not take that lane or settle attempts that started after its initial check.
The existing GitFinished event records terminal uncertainty in the ordinary envelope receipt, preserving the original caller, request and body hash after clearing the pending attempt.
For fresh determinate conflicts, rejected push, diverged pull and refused-remote outcomes, its optional fixed refusal enum preserves the known reason across receipt replay and registered-secret collisions.
Arbitrary CommandRefused reason text remains redacted.
Identical retries still return uncertainty after rebuild, and changed bodies or commands refuse; cleanup never makes an old mutation executable again.
Failed or stale reads leave attempts unchanged.
The same command-layer finalizer serves direct and authenticated transport reads; an agent read grants no Git mutation authority.
The normal writer failure latch still requires recovery before writes or receipt replay.

Registration and project creation observe checkouts outside the writer too.
Creation retains its durable intent and filesystem ownership witnesses.
Its final checkout verification and main-branch read precede the final transaction.
That transaction checks the current intent, settings, placements, reservations and name collisions before running the current lead and launch admission.

Design, mocks and file-context commands prepare bounded physical observations outside the writer.
They do not retain an admitted event plan.
Their final planners rerun current authority, revision, placement and collision checks before using those observations.
Prompt and board admissions still resolve current holders, recipients, channel access, quotas, replies and task references in the writer.
An observation is usable only with its unchanged settings root and registered repository, location, workspace and parent bindings.
Artifact fingerprints describe the bytes observed during preparation.
They do not promise that a file remains unchanged after that observation.
Later artifact reads continue to compare actual bytes with the recorded fingerprint.

The owner can use `project.creation.abandon` to release a pending creation reservation.
Abandonment takes the lifecycle lane and records only `project.CreationFinished`.
It leaves the repository, checkout, branches, exclusions, witnesses, files, and modes untouched.


A main checkout outside the recorded base receives a correctable refusal that names that base.
Creation checks this before effects, including saved intent retries, and never records a feature branch as its main review destination.
Only a current-admitted initial branch refusal with no creation intent receives a terminal refusal receipt; unavailable accounts and unfinished intents retain their existing retry behavior.
The later New project consumer joins this fixed reason directly; unknown creation errors retain their frozen request handling.

## The lead's first decisions and permissions.

A newly created project starts in Define.
The lifecycle uses `projectLeadInstruction` for the opening message on every creation path.
The lead is named "Project lead" and receives the role `lead:project-lead`.
Historical agents with the role `project-lead` also receive lead advice.
The lead first understands the goal and confirms the end result through an ordinary owner question.
It waits for that confirmation before creating sprints, teams or tasks.
The lead proposes a path based on the size of the work.
Small work may use one build team whose lead works without sub-workers.
The project lead may propose doing a nitpick itself.
Larger work uses sprints and teams with builders.
New command batches append explicit `project.StageChanged` events as planning and work begin.
Creating a sprint, team or queued task moves Define to Plan.
Starting or resuming a project worker moves Define or Plan to Build.
Creating a nonqueue task or moving a task out of the queue makes the same move to Build.
Automatic moves leave Design and stages from Build onward untouched.
The owner's explicit stage changes are unchanged.
The lead can use `project.stage.set` when an automatic move does not fit.
The project reducers do not derive these moves from old stored creation or task events.
Existing stored events keep their previous stage views during replay.

"Project planning" allows the lead to create sprints, teams and tasks.
"Starting work" allows the lead to start workers and move tasks out of the queue.
The planning permission also covers existing planning edits, task annotations by other agents, sprint completion and advancement beyond Define.
The starting permission also covers worker resume, prompts that restart idle workers, queued-task completion through answers and advancement to Build, Review or Handover.
A worker may still finish or annotate its own started task.
The owner's commands remain ungated.
The two permissions are independent.
Each setting is `allowed`, `denied` or `ask`.
Both settings default to `allowed`, including on historical projects with no recorded setting.
Allowed actions need no approval card.
A denied action explains that the owner turned the permission off and that it cannot be requested.
An approval request for a denied permission returns the same refusal.

Only the owner can use `project.permission.set`.
Its payload contains `projectId`, `permission` and `setting`.
The permission is `planning` or `starting-work`.
The website and command-line interface use the same client application programming interface (API) command.
Changing a setting appends `project.PermissionChanged` and clears only that permission's grant.
Saving the same effective setting preserves its grant.
Choosing "Always ask for permission" starts from "Not granted", including for a project approved under pull request 58.
The owner must grant that permission again through a new card or the existing owner command.
The existing owner commands `project.approval.grant` and `project.approval.withdraw` remain available for ask permissions.
Withdrawing a grant appends `project.PermissionWithdrawn` and leaves the other grant intact.
Setting changes and withdrawals never move the stage backwards.
The three legacy events `project.ApprovalGranted`, `project.ApprovalWithdrawn` and `project.ApprovalDecided` still replay.
Legacy withdrawal events retain their historical cascade during replay.
New withdrawals use the independent event.
Old grants retain their recorded identifiers.
An absent setting remains allowed even when old grant events exist.

Under "Always ask for permission", the lead uses `ask` with `projectApproval`.
The supported values remain `end-result`, `skip-planning`, `build`, and `skip-planning-and-build`.
The first two request Project planning.
The third requests Starting work.
The last requests both permissions together.
If only one still needs a grant, the engine records a card for that permission alone.
Its fixed effect sentence names only the permission the card can newly grant.
A combined request refuses if either permission is denied.
The title and recommendation are the short request.
The question text holds the full proposal.
The engine supplies the effect sentence and the acceptance choice "Accepted".
Only the owner's answer with that exact text grants the requested permission through a card.
An ordinary question never grants a lead permission.
Each decision appends `project.PermissionDecided` beside the question answer.
Its `granted` array records each permission independently.
New setting changes close affected cards before they can be answered.
Historical cards stored before this closure behavior can still grant their unchanged permission.
A grant lasts until its setting changes or the owner withdraws it.
A setting change or withdrawal closes its affected open cards in the same command batch.
The closure uses ordinary `QuestionAnswered` events and the ordinary answer delivery.
The fixed setting note is "Permission request closed because the owner changed its setting."
The fixed withdrawal note is "Permission request closed because the owner withdrew its grant."
An affected combined card is closed as a whole.
An unrelated permission card and ordinary questions remain open.
A closed card refuses later answers.
Saving the same effective setting does not close a card.
The engine composes these closures only after validating the owner-only project batch.
Other piece commands do not gain permission to emit question events.
The outcome is "Approval recorded." when this card grants at least one permission.
Otherwise it is "Approval not recorded. Ask again if you still need it."
A decline never reports a grant made by another action.
Changing the displayed stage cannot grant either permission.

## Project names and team deletion.

`project.rename` is available to the owner and the project's own lead.
It applies the creation name rules and appends `project.Renamed`.
It keeps the existing branch and worktree folder names.

`project.task.delete` removes a task only while its full recorded state history remains in the queue.
It appends `project.TaskDeleted`.
A task that was started and later returned to the queue cannot be deleted.

`project.team.delete` requires that its agents have stopped and its tasks have been deleted or moved.
It appends `project.TeamDeleted` and archives the team's board channel with `board.ChannelArchived`.
Any open posts are withdrawn with the existing `board.PostClosed` event in the same transaction.
The channel and its posts remain readable as history.
New posts and membership changes are refused on the archived channel.
The team's branch and worktree folder remain registered.
The owner and the project's own lead can use these requests.
The command line and agent tools expose the same requests as the client application programming interface.


## Deleting an archived project

Only the owner can call `project.delete` with `projectId` and `confirmed: true`.
The command refuses an active project, an incomplete archive, and an agent that is not stopped or crashed.
It also refuses an unfinished Git operation or an open terminal in a workspace that deletion would unregister.
An unfinished Git attempt by one of the project's agents also prevents deletion, even in a kept workspace.
That attempt must finish before its caller is deleted, so its finish event can still clear it.
An open merge review is removed with the rest of the project history.
The command appends `project.Deleted` to the existing event log.
It does not erase events from that log.
The same fold removes the project and its agents, threads, questions, requests, teams, sprints, tasks, mock records, Git evidence and review records from the live and rebuilt views.
Identifier tombstones prevent the event and thread reads from revealing that stored history again.
Every removed post identifier stays in the tombstone for as long as its stored events remain in the log.
Later acknowledgements, closures and read events can identify only the post, without its channel or author.
Forgetting a post identifier would make that stored history readable again.
A safe bound requires coordinated log compaction and removal of every reference to the forgotten identifiers.
This command does not compact the log.
The deletion event itself remains readable as the identifier-only notification that clients use to refresh.

The project's anchored channels, channels managed by its agents, and direct chats with its agents disappear with their posts when no other project's agent belongs to them.
Posts authored by its agents disappear from shared channels too.
Other authors' posts and replies remain in the channel's stored view.
Channel reads omit a kept post whose context refers to the deleted project.
Those references no longer open.
A kept reply without that context remains readable even when its parent post was deleted.
The reply's parent identifier then names nothing, and a new reply to that deleted parent is refused.
Shared channels remain available to their other members.
The deleted agents leave those channels.
The repository remains registered for other projects.
Repository design registrations and their current revision remain recorded.
A registration can open its artifacts only while its worktree remains registered.
When deletion unregisters that worktree, design reads can still return the registration with unavailable artifacts.
The design head stays on that recorded revision.
Its files remain on disk.
The repository checkout stays registered when identified by its checkout type or its repository path.
A parentless workspace whose repository row is missing also stays registered as a legacy checkout.
A worktree named by a surviving project or team also stays registered.
Deletion starts at the other worktrees named by the deleted project or its teams, and at workspaces carrying its project identifier.
It follows their registered descendants and stops at a kept workspace.
It never walks through a kept workspace.
A separate team or explicitly marked project workspace below a kept worktree still starts its own removal walk.
A workspace with an exclusive team identifier from the deleted project also starts its own removal walk.
Workspaces outside those walks remain registered, including unowned children of the repository checkout.
In the default layout, a team can share a kept worktree while its builders have only that worktree as their parent.
The workspace registration command does not record the builders' owning project or team.
Those builder registrations remain because their parent alone cannot establish exclusive ownership.
Issue 1028 records this accepted residue.
Issue 1029 records that a surviving project identifier alone does not protect a workspace under a removed root.
Issue 1030 records that a kept workspace can retain a parent identifier naming a removed workspace.
Every worktree folder and every branch stays on disk, including dirty and externally registered folders.
No safe worktree removal operation exists in the engine, so this command has no removal option.

The command-line interface exposes the same operation:

```sh
./bin/lgtm project.delete --projectId PROJECT_ID --confirmed true
```

An archived project remains reachable through All projects and its repository's project list.
Open its Settings tab to restore or delete it.

### Changing a repository base branch

`project.repository.base.set` takes `repoId` and `baseBranch` from the owner.
The branch must exist locally.
An open review, an unfinished Git operation, an active merge, or an unfinished project creation refuses the change.
The main checkout must be switched to the recorded branch before project creation or local approval.
The setting does not move a branch or switch a checkout.
The main workspace binding changes only if a bounded `HEAD` read in the setting transaction confirms that branch is checked out.
That read uses checked metadata paths and never acquires the Git gate inside the writer.
An owner who switches afterwards can repeat the setting command to refresh that binding.
Registration records the branch named by `refs/remotes/origin/HEAD` only when it exists locally.
A malformed remote default is treated as absent.
Otherwise, an existing current `main` or `master`, or the current unborn branch, supplies the registration default.
The registration reply includes that recorded base.
Old rows acquire only an existing current `main` or `master` through a new owner-authored `project.RepositoryBaseChanged` event.
Their inference never reads `origin/HEAD`.
Agent reads never infer or record a base.
Both conventional branches stay guarded while a row has no stored base.
Failed inference leaves old reads available.
Collection reads reuse a miss only while the checked main-checkout `HEAD` is unchanged.
Every owner command scoped to that repository retries the inference.
New repositories use `main`.

Project creation fetches the configured remote base through the hardened explicit tracking-reference fetch.
It uses the fetched commit only when the local base is an ancestor and confirmed GitHub delivery uses that same remote branch.
Local delivery starts from the local base.
A second `project.CreationPlanned` event records the selected `start` before the worktree write.
That write checks the current intent inside the transaction and retains an existing `start`.
Recovery reuses that decision instead of fetching a different start.
`project.Created.startingHead` retains the commit for subsequent comparisons.
Review patches use the later merge base with the current target or starting commit.
One `git merge-base <head> <target> <start>` call selects the comparison base without merging trees or writing objects.
When the inherited histories diverge, the review may show additional inherited changes while retaining the project's own edits.
Commit lists and pending counts exclude the starting commit's history, including after hosted delivery becomes unavailable.
The actual target head still supplies the approval identity and destination checks.
Optional remote discovery and fetching share a 20-second deadline.
It otherwise uses the local commit and records why in `project.CreationFinished.detail`.
A resumed creation that finds its existing worktree records that it reused that worktree.
The fetch never updates a local branch or the main checkout's files.
