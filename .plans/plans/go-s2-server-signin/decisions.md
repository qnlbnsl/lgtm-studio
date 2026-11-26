# Decisions — go-s2-server-signin

The repository's writing rule forbids a quotation of a person in a tracked file.
A row therefore gives the date and the source of a decision of the owner, without the words.

| id | question | decision | rationale | status | date |
|---|---|---|---|---|---|
| 1 | Which process tier fits this plan? | Tier M. This is the orchestrator's first reading, and the planner confirms it. | It is the boundary that every call of the website crosses. It has one workstream, and it changes no contract. | closed | 2026-11-25 |
| 2 | What does this plan cover? | The work items of the brief. They are a slice of waves 0 and 1 of the design. | The owner approved the design on 2026-11-25, in the planning session. The owner asked the same day for separate plans for the server and the client, and for smaller work items. | closed | 2026-11-25 |
| 3 | Which process runs this plan? | The build loop in `docs/specs/2026-11-05-build-loop-design.md`. | Decided by the owner on 2026-11-25, in the planning session. | closed | 2026-11-25 |
| 4 | Which harnesses build and review? | A mix of Codex and Claude, and Cursor with Grok as well. | Decided by the owner on 2026-11-25, in the planning session. | closed | 2026-11-25 |
| 5 | Is the cut of wave 1 into three sprints confirmed? | Not yet. The orchestrator made the cut. | It gives each sprint an end that a person can observe. The owner has not ruled on it. | open | 2026-11-25 |
