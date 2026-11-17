import type { GitReviewDelivery } from "@lgtm/contracts";
import { ghProcess, type ProcessReply } from "./process.ts";
import { GitFailure } from "./types.ts";

type Run = (args: ReadonlyArray<string>) => Promise<ProcessReply>;
export type GithubDelivery = Extract<GitReviewDelivery, { mode: "github" }>;
const fail = (reason: "github_refused" | "forbidden_remote" | "stale_state") : never => { throw new GitFailure({ reason }); };
export const forbiddenRemote = (value: string) => {
  let path = value;
  try { path = new URL(value).pathname; } catch { /* SCP or local path. */ }
  try { path = decodeURIComponent(path); } catch { return true; }
  return /(?:^|[/:])vitalysim\/herdr-synapse(?:\.git)?$/i.test(path.replace(/\/+$/, ""));
};
const githubRepository = (url: string): string | undefined => {
  const match = /^(?:https:\/\/github\.com\/|ssh:\/\/git@github\.com\/|git@github\.com:)([\w.-]+\/[\w.-]+?)(?:\.git)?\/?$/i.exec(url);
  return match?.[1];
};

/** No diagnostics leave this module. In particular, auth status is only an exit code. */
export const githubCommands = (folder: string, signal: AbortSignal, timeoutMs: number) => {
  const run: Run = (args) => ghProcess(folder, args, { bytes: 64 * 1024, timeoutMs }, signal);
  const json = async (args: ReadonlyArray<string>): Promise<any> => {
    const reply = await run(args);
    if (reply.code !== 0) return fail("github_refused");
    try { return JSON.parse(reply.stdout); } catch { return fail("github_refused"); }
  };
  const fields = "number,url,state,title,body,headRefName,baseRefName,headRefOid,baseRefOid,mergeable,mergeStateStatus,mergeCommit";
  return { run, json, fields };
};

export const deliveryMode = async (git: Run, gh: Run): Promise<{ mode: "github"; remote: string; repository: string } | Extract<GitReviewDelivery, { mode: "local" }>> => {
  const names = (await git(["remote"])).stdout.trim().split("\n").filter(Boolean);
  const candidates: { remote: string; url: string; repository: string | undefined }[] = [];
  for (const name of names) {
    if (!/^[\w.-]+$/.test(name) || name.startsWith("-")) continue;
    const raw = await git(["config", "--get", `remote.${name}.url`]);
    if (raw.code !== 0) continue;
    const url = raw.stdout.trim();
    if (forbiddenRemote(url)) return fail("forbidden_remote");
    candidates.push({ remote: name, url, repository: githubRepository(url) });
  }
  const hosted = candidates.filter((candidate) => candidate.repository !== undefined);
  const selected = hosted.find((candidate) => candidate.remote === "origin") ?? (hosted.length === 1 ? hosted[0] : undefined);
  if (!selected) return { mode: "local", reason: candidates.length === 0 ? "Local merge: no repository remote is configured."
    : hosted.length > 1 ? "Local merge: several GitHub remotes are configured; none is origin."
    : "Local merge: the repository remote is not on GitHub.com." };
  const { remote } = selected;
  const urls = [selected.url];
  for (const push of [false, true]) {
    const reply = await git(["remote", "get-url", ...(push ? ["--push"] : []), "--all", remote]);
    if (reply.code !== 0) return fail("github_refused");
    const values = reply.stdout.trim().split("\n");
    if (values.length !== 1) return fail("github_refused");
    urls.push(...values);
  }
  if (urls.some(forbiddenRemote)) return fail("forbidden_remote");
  const repository = githubRepository(urls[0]!);
  if (!repository) return { mode: "local", reason: "Local merge: the repository remote is not on GitHub.com." };
  // A configured push URL may be local (including an offline mirror), but a
  // different hosted repository must never receive the project's branch.
  if (urls.some((url) => githubRepository(url) !== undefined && githubRepository(url)?.toLowerCase() !== repository.toLowerCase())) return fail("github_refused");
  let auth: ProcessReply;
  try { auth = await gh(["auth", "status", "--active", "--hostname", "github.com"]); }
  catch (error) {
    if (error instanceof GitFailure && error.reason === "gh_unavailable") return { mode: "local", reason: "Local merge: the GitHub CLI is not installed for the service user." };
    throw error;
  }
  if (auth.code !== 0) return { mode: "local", reason: "Local merge: the service user is not signed in to the GitHub CLI." };
  return { mode: "github", remote, repository };
};

export const checkedPullRequest = (value: any, repo: string, remote: string, branch: string, base: string, head: string): GithubDelivery => {
  if (!value || !Number.isSafeInteger(value.number) || value.number < 1 || value.number > 2147483647
    || typeof value.url !== "string" || value.url.toLowerCase() !== `https://github.com/${repo}/pull/${value.number}`.toLowerCase()
    || !["OPEN", "CLOSED", "MERGED"].includes(value.state)
    || value.headRefName !== branch || value.baseRefName !== base || value.headRefOid !== head
    || !/^[a-f0-9]{40,64}$/.test(value.baseRefOid)
    || typeof value.title !== "string" || typeof value.body !== "string"
    || Buffer.byteLength(value.title) > 1024 || Buffer.byteLength(value.body) > 32 * 1024) return fail("stale_state");
  return { mode: "github", repository: repo, remote, number: value.number, url: value.url, state: value.state,
    title: value.title, body: value.body, remoteBaseHead: value.baseRefOid };
};
