import type * as Effect from "effect/Effect";
import * as Data from "effect/Data";
import type { Caller, GitMutationIdentity, GitReviewDelivery } from "@lgtm/contracts";

export type GitFailureReason =
  | "base_branch_required" | "base_branch_too_long"
  | "gh_unavailable" | "github_refused" | "github_remote_changed" | "github_create_refused" | "github_base_moved" | "forbidden_remote" | "invalid_path" | "not_repository" | "invalid_ref" | "output_limit" | "timeout" | "cancelled"
  | "git_unavailable" | "git_failed" | "busy" | "collision" | "dirty_destination" | "dirty_source"
  | "conflicts" | "confirmation_required" | "stale_state" | "nothing_staged" | "invalid_message"
  | "missing_remote" | "missing_upstream" | "authentication_required" | "network_failed" | "push_rejected"
  | "pull_diverged" | "base_requires_review" | "unsupported_provider" | "push_required"
  | "unsupported_conflict" | "incomplete_choices" | "owner_required" | "unsupported_review" | "no_merge" | "abort_blocked";

// Neither subprocess output nor rejected paths are attached to a failure.
export class GitFailure extends Data.TaggedError("GitFailure")<{ readonly reason: GitFailureReason; readonly uncertain?: boolean; readonly determinate?: boolean; readonly githubDelivery?: boolean; readonly pullRequestState?: "OPEN" | "CLOSED" | "MERGED" }> {}
export type GitOperation<A> = Effect.Effect<A, GitFailure>;
export interface Bounded<A> { readonly items: ReadonlyArray<A>; readonly truncated: boolean }
export interface Repository { readonly path: string; readonly name: string; readonly commonDirectory: string }
export interface Branch { readonly name: string; readonly head: string; readonly current: boolean; readonly upstream: string | null }
export interface Worktree { readonly path: string; readonly head: string | null; readonly branch: string | null; readonly locked: boolean; readonly available: boolean }
export interface Change {
  readonly path: string; readonly originalPath: string | null; readonly index: string; readonly worktree: string;
  readonly staged: boolean; readonly unstaged: boolean; readonly conflict: boolean; readonly untracked: boolean;
}
export interface Diff {
  readonly patch: string; readonly additions: number; readonly deletions: number;
  readonly binary: boolean; readonly truncated: boolean;
}
export interface Commit { readonly head: string; readonly parents: ReadonlyArray<string>; readonly author: string; readonly at: string; readonly title: string }
export interface CommitFile { readonly path: string; readonly originalPath: string | null; readonly status: string }
export interface GitState {
  readonly head: string | null; readonly branch: string | null; readonly upstream: string | null;
  readonly mergeHead: string | null;
  readonly behindParent?: number;
  readonly ahead: number; readonly behind: number; readonly pending: number; readonly pendingSince: string | null;
}
export type ConflictChoice = "ours" | "theirs" | "both";
export type ConflictSegment = { readonly kind: "context"; readonly line: number; readonly context: ReadonlyArray<string> }
  | { readonly kind: "conflict"; readonly ours: ReadonlyArray<string>; readonly base: ReadonlyArray<string>; readonly theirs: ReadonlyArray<string> };
export interface Conflict {
  readonly path: string; readonly fingerprint: string; readonly head: string;
  readonly ours: string; readonly base: string; readonly theirs: string;
  readonly segments: ReadonlyArray<ConflictSegment>; readonly choices: ReadonlyArray<ConflictChoice | null>;
}
export interface Review {
  readonly head: string; readonly baseHead: string; readonly branch: string; readonly base: string;
  readonly commits: Bounded<Commit>; readonly files: Bounded<CommitFile>; readonly diff: Diff;
}
export interface ResolvedConflict { readonly path: string; readonly fingerprint: string }
export type FileChange = "added" | "modified" | "deleted" | "renamed" | "untracked" | "conflict";
export interface FileEntry { readonly name: string; readonly path: string; readonly kind: "folder" | "file"; readonly size: number | null; readonly change: FileChange | null }
export interface FileListing { readonly folder: string; readonly items: ReadonlyArray<FileEntry>; readonly truncated: boolean }
export type FileImageType = "image/png" | "image/jpeg" | "image/gif" | "image/webp" | "image/svg+xml";
export type FileRead =
  | { readonly kind: "text"; readonly path: string; readonly size: number; readonly version: string; readonly text: string }
  | { readonly kind: "image"; readonly path: string; readonly size: number; readonly version: string; readonly mediaType: FileImageType; readonly data: string }
  | { readonly kind: "binary" | "large"; readonly path: string; readonly size: number };
export interface DiscardPlan { readonly fingerprint: string; readonly paths: ReadonlyArray<string> }
export interface PreparedDelivery { readonly delivery: GitReviewDelivery; readonly merged?: { readonly head: string; readonly localSync: string } }
export interface GitService {
  mutationIdentity(folder: string): GitOperation<GitMutationIdentity>;
  readonly contextFiles: (folder: string, query: string) => GitOperation<Bounded<string>>;
  readonly contextFile: (folder: string, path: string) => GitOperation<void>;
  discover(): GitOperation<Bounded<Repository>>;
  createRepository(name: string): GitOperation<Repository>;
  register(folder: string): GitOperation<Repository>;
  detectBaseBranch(folder: string, registration?: boolean): GitOperation<string>;
  isBaseBranch(folder: string, branch: string): GitOperation<boolean>;
  repositoryBase(folder: string): GitOperation<string>;
  projectStart(folder: string, base: string): GitOperation<{ head: string | null; detail: string }>;
  validateBranch(folder: string, name: string): GitOperation<void>;
  branches(folder: string): GitOperation<Bounded<Branch>>;
  worktrees(folder: string): GitOperation<Bounded<Worktree>>;
  createWorktree(folder: string, name: string, from?: string, branchPrefix?: string): GitOperation<Worktree>;
  rollbackWorktree(folder: string, worktree: Worktree): GitOperation<void>;
  changes(folder: string, agentRead?: boolean): GitOperation<Bounded<Change>>;
  diff(folder: string, options?: { readonly path?: string; readonly staged?: boolean; readonly commit?: string; readonly agentRead?: boolean }): GitOperation<Diff>;
  history(folder: string, limit?: number): GitOperation<Bounded<Commit>>;
  commitFiles(folder: string, commit: string, agentRead?: boolean): GitOperation<Bounded<CommitFile>>;
  // A registration and checked state may be reused only within the same uninterrupted engine gate.
  state(folder: string, target?: string, registration?: Repository, checkedState?: GitState): GitOperation<GitState>;
  stage(folder: string, path: string): GitOperation<void>;
  unstage(folder: string, path: string): GitOperation<void>;
  stageAll(folder: string): GitOperation<void>;
  unstageAll(folder: string): GitOperation<void>;
  planDiscard(folder: string, paths?: ReadonlyArray<string>): GitOperation<DiscardPlan>;
  discard(folder: string, plan: DiscardPlan, confirmed: boolean): GitOperation<void>;
  commit(folder: string, message: string, path?: string): GitOperation<string>;
  // The Files tab: one folder, one file, or one replaced file at a time. Only
  // tracked and nonignored untracked regular files are listed, read or written.
  listFolder(folder: string, path?: string): GitOperation<FileListing>;
  readFile(folder: string, path: string): GitOperation<FileRead>;
  writeFile(folder: string, path: string, version: string, bytes: Uint8Array): GitOperation<string>;
  fetch(folder: string): GitOperation<void>;
  pull(folder: string): GitOperation<void>;
  push(folder: string): GitOperation<void>;
  mergeTeam(source: string, destination: string, expectedHead: string): GitOperation<string>;
  updateFromParent(source: string, destination: string, expectedHead: string): GitOperation<string>;
  review(folder: string, base: string, agentRead?: boolean): GitOperation<Review>;
  prepareDelivery(source: string, reviewed: Pick<Review, "head" | "baseHead" | "branch" | "base">,
    text: { title: string; body: string }, caller: Caller, destination: string): GitOperation<PreparedDelivery>;
  approveDelivery(source: string, destination: string, reviewed: Pick<Review, "head" | "baseHead" | "branch" | "base">,
    delivery: Extract<GitReviewDelivery, { mode: "github" }>, caller: Caller, confirmed: boolean):
      GitOperation<{ head: string; delivery: GitReviewDelivery; localSync: string }>;
  // Supplying the authenticated owner enables a live hosted-base fetch.
  // Caller-less comparisons and local approval stay entirely local.
  reviewMain(source: string, destination: string, caller?: Caller): GitOperation<Review>;
  // A distinct execution boundary, never a bypass on mergeTeam. The engine
  // supplies the authenticated caller, not a field from the command payload.
  approveMain(source: string, destination: string, reviewed: Pick<Review, "head" | "baseHead" | "branch" | "base">,
    caller: Caller | undefined, confirmed: boolean): GitOperation<string>;
  abortMerge(folder: string, expectedHead: string, expectedMergeHead: string, confirmed: boolean): GitOperation<void>;
  resolvedConflicts(folder: string): GitOperation<Bounded<ResolvedConflict>>;
  reopen(folder: string, path: string, expectedHead: string, expectedMergeHead: string, fingerprint: string, confirmed: boolean): GitOperation<void>;
  conflict(folder: string, path: string, agentRead?: boolean): GitOperation<Conflict>;
  choose(folder: string, selection: Conflict, hunk: number, choice: ConflictChoice): GitOperation<Conflict>;
  resolve(folder: string, selection: Conflict): GitOperation<void>;
}
