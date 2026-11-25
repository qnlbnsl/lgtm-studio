# AGENTS.md

Instructions for any coding agent working in this repository.
`CLAUDE.md` imports this file, so there is one set of rules.

## What this project is

LGTM Studio runs teams of coding agents and shows their work on a website.

The product is being rewritten, and many decisions are still open.
This file is short on purpose.
When this file and a newer decision of the owner disagree, the newer decision wins.
An older document in `docs/` records what was true when it was written. It is not a rule.

The owner decided on 2026-11-23 that the product is three parts:

- **The website** is `src/frontend/`. It is a static build in Preact, Vite and TypeScript.
- **The server** holds the state and answers the website.
- **The client** is the program on each machine that runs agents.

The three meet only through the contract in `src/contracts/`, which is written in Protocol Buffers.

The owner decided on 2026-11-24 that the server and the client are written from scratch in Go.
The design in progress is in `docs/specs/`, in the newest file about the Go rewrite.

## Where things are

- `packages/`, `design/mock/` and `plugins/` hold the old TypeScript engine and its page. They are a reference. New code does not go there.
- `src/mock-server/` is a server with sample data for the website.
- A screen is designed in `src/frontend/`.
- A change to what the parts exchange starts in `src/contracts/proto/`.

## Habits

1. **Credit copied code.** Code copied from another project goes under `third_party/<project>/` with that project's licence file, and gets an entry in [NOTICE](NOTICE). Ideas and designs can be reused freely.
2. **Write down what you find.** A feature idea goes into [docs/features.md](docs/features.md). A problem goes into [docs/issues.md](docs/issues.md), with the next number after the highest in that file and in [docs/issues-closed.md](docs/issues-closed.md).
3. **Finished work reaches the base branch through a pull request.** Inside a team's own branches, agents commit, push and merge without asking.
4. **This repository is not a fork of [Herdr Synapse](https://github.com/vitalysim/herdr-synapse).** Nothing is pushed there.

## Writing style for documents

Documents in this repository are read by a person with ADHD on a black-and-white terminal.
Write in full sentences with articles and commas.
Make headings name their content.
Put one claim in each sentence.
Expand every acronym the first time it appears in a document.
Record a decision and its date, never a quotation of a person, a private project, a home folder path or a network address; `scripts/check-private.mjs` enforces this.
