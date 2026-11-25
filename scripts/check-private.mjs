#!/usr/bin/env node
// Checks that no tracked file holds a private detail. AGENTS.md, "Writing
// style for documents", states the rule this script enforces.
//
// Usage: node scripts/check-private.mjs [root]
//
// The root defaults to the repository that holds this script. The check reads
// every file that `git ls-files` lists, except binary files and pnpm-lock.yaml,
// and reports each finding as `file:line: rule`:
//   home-path         a home folder of a real user, /home/<name>/ or /Users/<name>/;
//   scratch-path      a session scratch folder, a folder in /tmp named claude-<id>;
//   private-address   a private network address that is not a generic example;
//   private-word      a match of an expression in the local, untracked file
//                     .private-words (the match itself is never printed);
//   owner-quotation   a sentence in which "owner" is followed by said, says,
//                     words, asked, wrote or told and then an opening quotation mark.
// A copyright line in LICENSE, NOTICE or under third_party/ may hold a private
// word. The exit status is 1 when there is any finding and 0 when there is none.
import { execFileSync } from "node:child_process";
import { existsSync, lstatSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const root = process.argv[2] ?? fileURLToPath(new URL("..", import.meta.url));

// Invented names that documents, samples and tests use for a home folder.
const placeholderUsers = new Set(["user", "owner", "maya", "someone", "agent", "u", "planted-secret-path", "other-planted"]);
// Generic private addresses that tests and sample data may use.
const genericAddresses = new Set([
  "10.0.0.5", "10.2.3.4", "172.16.0.1",
  "192.168.1.1", "192.168.1.10", "192.168.1.24", "192.168.1.31", "192.168.1.52", "192.168.3.4",
]);

const homePath = /\/(?:home|Users)\/([A-Za-z0-9._-]+)\//g;
const scratchPath = /\/tmp\/claude-/;
const privateAddress = /(?<![\d.])(?:10\.\d{1,3}|172\.(?:1[6-9]|2\d|3[01])|192\.168)\.\d{1,3}\.\d{1,3}(?![\d.]*\d)/g;
// "The owner's approval says" is not a quotation, and a table cell ends a sentence.
const ownerQuotation = /\bowner\b(?!['’]s\s+(?!words\b))[^.!?|]{0,300}?\b(?:said|says|words|asked|wrote|told)\b[^.!?|"“”]{0,40}?[\s:(]["“]/gi;

const privateWordsFile = join(root, ".private-words");
const privateWords = existsSync(privateWordsFile)
  ? readFileSync(privateWordsFile, "utf8").split("\n").flatMap((text, index) =>
    text.trim() && !text.startsWith("#") ? [{ line: index + 1, pattern: new RegExp(text.trim()) }] : [])
  : [];
const copyrightExempt = (file, text) =>
  /copyright/i.test(text) && (file === "LICENSE" || file === "NOTICE" || file.startsWith("third_party/"));

const lineAt = (text, offset) => text.slice(0, offset).split("\n").length;

const check = (file, text) => {
  const findings = [];
  const lines = text.split("\n");
  lines.forEach((line, index) => {
    const at = (rule) => findings.push(`${file}:${index + 1}: ${rule}`);
    for (const match of line.matchAll(homePath)) if (!placeholderUsers.has(match[1])) at("home-path");
    if (scratchPath.test(line)) at("scratch-path");
    for (const match of line.matchAll(privateAddress)) if (!genericAddresses.has(match[0])) at("private-address");
    if (!copyrightExempt(file, line)) {
      for (const word of privateWords) if (word.pattern.test(line)) at(`private-word (line ${word.line} of .private-words)`);
    }
  });
  // A quoted sentence often wraps, so this rule reads whole paragraphs.
  for (const paragraph of text.matchAll(/(?:^|\n)((?:[^\n]*\S[^\n]*(?:\n|$))+)/g)) {
    const start = paragraph.index + paragraph[0].indexOf(paragraph[1]);
    for (const match of paragraph[1].matchAll(ownerQuotation)) {
      findings.push(`${file}:${lineAt(text, start + match.index)}: owner-quotation`);
    }
  }
  return findings;
};

const files = execFileSync("git", ["-C", root, "ls-files", "-z"], { encoding: "utf8" }).split("\0").filter(Boolean);
const findings = [];
for (const file of files) {
  if (file === "pnpm-lock.yaml") continue;
  const path = join(root, file);
  if (!existsSync(path) || !lstatSync(path).isFile()) continue;
  const bytes = readFileSync(path);
  if (bytes.subarray(0, 8000).includes(0)) continue;
  findings.push(...check(file, bytes.toString("utf8")));
}
for (const finding of findings) console.error(finding);
if (findings.length > 0) {
  console.error(`check-private: ${findings.length} finding(s); see AGENTS.md, "Writing style for documents".`);
  process.exit(1);
}
