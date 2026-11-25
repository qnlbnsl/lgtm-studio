// scripts/check-private.mjs [root]: every rule fires on a tracked file, and a
// clean tree passes. The private values below are built from parts, so this
// file does not trip the check on the real repository.
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { afterAll, describe, expect, it } from "vitest";

const repoRoot = fileURLToPath(new URL("../../../../", import.meta.url));
const checker = join(repoRoot, "scripts/check-private.mjs");
const scratch: string[] = [];
afterAll(() => {
  for (const root of scratch) rmSync(root, { recursive: true, force: true });
});

const secret = ["Zor", "blax"].join("");
const quote = '"';
const alice = ["", "home", "alice", ""].join("/");
const bob = ["", "Users", "bob", ""].join("/");

// Writes the files, tracks them with git, and runs the check on that tree.
const run = (files: Record<string, string | Buffer>, untracked: Record<string, string> = {}) => {
  const root = realpathSync(mkdtempSync(join(tmpdir(), "lgtm-private-")));
  scratch.push(root);
  for (const [path, text] of Object.entries({ ...files, ...untracked })) {
    mkdirSync(dirname(join(root, path)), { recursive: true });
    writeFileSync(join(root, path), text);
  }
  spawnSync("git", ["init", "-q"], { cwd: root });
  spawnSync("git", ["add", "--", ...Object.keys(files)], { cwd: root });
  const result = spawnSync(process.execPath, [checker, root], { encoding: "utf8" });
  return { status: result.status, output: result.stdout + result.stderr };
};

const clean = {
  "README.md": [
    "Settings live in ~/.lgtm and in /home/user/projects.",
    "A test may use 10.0.0.5 or 192.168.1.10, and a document uses 192.0.2.4.",
    "The owner decided on 2026-11-05 to add a lane for hotfixes.",
    `The owner pressed the button ${quote}Save${quote}.`,
    `A task blocked by its owner's approval says ${quote}Blocked${quote}.`,
    "| 6 | The owner said both forms are fine |",
    `| 7 | ${quote}Send${quote} is 24 pixels high |`,
    "",
  ].join("\n"),
  "flow.json": `{ "name": "Owner told", "role": "owner", "text": "Owner says Y." }\n`,
};

describe("check-private", () => {
  it("passes a clean tree", () => {
    const result = run(clean, { ".private-words": `# a comment\n${secret}\n` });
    expect(result.output).toBe("");
    expect(result.status).toBe(0);
  });

  it("names a real user's home folder", () => {
    const result = run({ "a.md": `See ${alice}notes and ${bob}x.\n` });
    expect(result.status).toBe(1);
    expect(result.output).toContain("a.md:1: home-path");
  });

  it("names a session scratch path", () => {
    const result = run({ "a.md": ["one", `Saved in /tmp/${"claude"}-1000/x.`].join("\n") });
    expect(result.output).toContain("a.md:2: scratch-path");
  });

  it("names a private address that is not a generic example", () => {
    const address = ["192", "168", "143", "7"].join(".");
    const result = run({ "a.ts": `const host = "${address}";\n` });
    expect(result.status).toBe(1);
    expect(result.output).toContain("a.ts:1: private-address");
  });

  it("names a private word without printing it, and exempts copyright lines", () => {
    const result = run(
      {
        "a.md": `Built for ${secret}.\n`,
        "NOTICE": `Copyright (C) 2026 ${secret}\n`,
        "third_party/x/README.md": `Copyright 2026 ${secret}\n`,
      },
      { ".private-words": `# names\n${secret}\n` },
    );
    expect(result.status).toBe(1);
    expect(result.output).toContain("a.md:1: private-word (line 2 of .private-words)");
    expect(result.output).not.toContain(secret);
    expect(result.output).not.toContain("NOTICE");
    expect(result.output).not.toContain("third_party");
  });

  it("names a quotation of the owner, also when the sentence wraps", () => {
    const result = run({
      "a.md": [
        `The owner said: ${quote}ship it${quote}.`,
        "",
        "On 2026-11-23 the owner's",
        `words were ${quote}call it actions${quote}.`,
        "",
      ].join("\n"),
    });
    expect(result.status).toBe(1);
    expect(result.output).toContain("a.md:1: owner-quotation");
    expect(result.output).toContain("a.md:3: owner-quotation");
  });

  it("skips binary files and pnpm-lock.yaml", () => {
    const result = run({
      "image.png": Buffer.concat([Buffer.from([0x89, 0x00]), Buffer.from(alice)]),
      "pnpm-lock.yaml": `resolution: ${alice}x\n`,
    });
    expect(result.status).toBe(0);
  });
});
