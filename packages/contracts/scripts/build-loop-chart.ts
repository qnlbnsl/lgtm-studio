// Draws the build loop's steps as Mermaid charts that GitHub renders.
//
// The source is the "The steps in each phase" section of
// docs/specs/2026-11-05-build-loop-design.md. Each step line there is one
// move: "- From -> To: action". This script turns every phase and block into
// one flowchart and writes them to docs/specs/2026-11-05-build-loop-chart.md.
//
// Run it after changing a step line:
//   node packages/contracts/scripts/build-loop-chart.ts
// Check that the chart file is current, without writing it:
//   node packages/contracts/scripts/build-loop-chart.ts --check

import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

export const specFile = fileURLToPath(new URL("../../../docs/specs/2026-11-05-build-loop-design.md", import.meta.url));
export const chartFile = fileURLToPath(new URL("../../../docs/specs/2026-11-05-build-loop-chart.md", import.meta.url));

type Who = "agent" | "owner" | "owner if needed";
interface Move { readonly from: string; readonly to: string; readonly label: string; readonly who: Who }
interface Section { readonly heading: string; readonly phase: string; readonly moves: Move[] }

const phaseLink = /^[1-8] /;

export function parseSteps(spec: string): Section[] {
  const start = spec.indexOf("## The steps in each phase");
  const end = spec.indexOf("### Test types");
  if (start < 0 || end < start) throw new Error("The spec has no steps section.");
  // A step line that wraps continues on the next line with two spaces.
  const lines = spec.slice(start, end).replace(/\n  /g, " ").split("\n");
  const sections: Section[] = [];
  let phase = "";
  for (const line of lines) {
    if (line.startsWith("### ")) {
      phase = line.slice(4).trim();
      sections.push({ heading: phase, phase, moves: [] });
    } else if (line.startsWith("#### ")) {
      sections.push({ heading: line.slice(5).trim(), phase, moves: [] });
    } else if (line.startsWith("- ") && line.includes(" -> ")) {
      const section = sections.at(-1);
      if (!section) throw new Error(`The step "${line}" comes before any phase heading.`);
      const [from, rest] = splitOnce(line.slice(2), " -> ");
      const [to, action] = splitOnce(rest, ": ");
      let label = action.trim();
      let who: Who = "agent";
      if (label.endsWith("[OWNER IF NEEDED]")) { who = "owner if needed"; label = label.slice(0, -17).trim(); }
      else if (label.endsWith("[OWNER]")) { who = "owner"; label = label.slice(0, -7).trim(); }
      section.moves.push({ from: from.trim(), to: to.trim(), label, who });
    }
  }
  return sections.filter((section) => section.moves.length > 0);
}

function splitOnce(text: string, separator: string): [string, string] {
  const at = text.indexOf(separator);
  return at < 0 ? [text, ""] : [text.slice(0, at), text.slice(at + separator.length)];
}

const quote = (text: string) => `"${text.replace(/"/g, "#quot;")}"`;

function arrow(move: { label: string; who: Who }): string {
  const label = move.label ? `|${quote(move.label)}|` : "";
  if (move.who === "owner") return `==>${label}`;
  if (move.who === "owner if needed") return `-.->${label}`;
  return `-->${label}`;
}

function flowchart(moves: readonly Move[]): string {
  const ids = new Map<string, string>();
  const id = (name: string) => {
    let found = ids.get(name);
    if (!found) { found = `s${ids.size}`; ids.set(name, found); }
    return found;
  };
  for (const move of moves) { id(move.from); id(move.to); }
  const targets = new Set(moves.map((move) => move.to));
  const out = ["```mermaid", "flowchart TD"];
  for (const [name, key] of ids) {
    // A state nothing moves into is where the chart starts, so it is drawn
    // as a pill, like a link to another phase.
    if (phaseLink.test(name) || !targets.has(name)) out.push(`  ${key}([${quote(name)}])`);
    else if (name.endsWith("?")) out.push(`  ${key}{${quote(name)}}`);
    else out.push(`  ${key}(${quote(name)})`);
  }
  for (const move of moves) out.push(`  ${id(move.from)} ${arrow(move)} ${id(move.to)}`);
  out.push("```");
  return out.join("\n");
}

// The overview joins every phase link in the step lines with the list under
// "Moves between phases", such as "- Build to Plan: plan wrong.". A step
// line's own label, such as "yes", means nothing outside its phase, so the
// overview labels only the moves that list names.
function overview(spec: string, sections: readonly Section[]): string {
  const numbered = new Map(sections.filter((section) => section.heading === section.phase).map((section) => [section.phase.replace(phaseLink, ""), section.phase]));
  const named = new Map<string, Move>();
  const listed = spec.slice(spec.indexOf("### Moves between phases")).split("\n");
  for (const line of listed) {
    const match = /^- (.+?) to (.+?): (.+?)\.?$/.exec(line);
    if (!match) continue;
    const from = numbered.get(match[1]!), to = numbered.get(match[2]!);
    if (!from || !to) throw new Error(`The move "${line}" names a phase that does not exist.`);
    const owner = match[3]!.endsWith("[OWNER]");
    named.set(`${from} -> ${to}`, { from, to, label: match[3]!.replace(/\s*\[OWNER\]$/, ""), who: owner ? "owner" : "agent" });
  }
  const moves = new Map<string, Move>();
  for (const section of sections) {
    for (const move of section.moves) {
      if (!phaseLink.test(move.to)) continue;
      for (const target of move.to.split(" or ")) {
        const key = `${section.phase} -> ${target}`;
        if (target !== section.phase && !moves.has(key)) moves.set(key, named.get(key) ?? { from: section.phase, to: target, label: "", who: "agent" });
      }
    }
  }
  for (const [key, move] of named) if (!moves.has(key)) moves.set(key, move);
  return flowchart([...moves.values()]);
}

export function renderChart(spec: string): string {
  const sections = parseSteps(spec);
  const parts = [
    "# Build loop chart",
    "",
    "<!-- Generated by packages/contracts/scripts/build-loop-chart.ts. Do not edit by hand. -->",
    "",
    "This file draws the steps of the build loop as charts. The steps themselves",
    "are written in [the build loop design](2026-11-05-build-loop-design.md#the-steps-in-each-phase),",
    "and that text is the source. A script makes this file from it, and a test",
    "fails when this file is out of date. GitHub draws each chart from its Mermaid",
    "source. Run `node packages/contracts/scripts/build-loop-chart.ts` after",
    "changing a step.",
    "",
    "## How to read the charts",
    "",
    "- A rounded box is a state the work is in.",
    "- A diamond is a decision.",
    "- A pill is where a chart starts, or another phase the work moves to.",
    "- A plain arrow is an action an agent or a script takes.",
    "- A thick arrow is an action the owner takes.",
    "- A dotted arrow is an action that reaches the owner only when needed.",
    "",
    "## The phases and the moves between them",
    "",
    overview(spec, sections),
  ];
  for (const section of sections) {
    parts.push("", `${section.heading === section.phase ? "##" : "###"} ${section.heading}`, "", flowchart(section.moves));
  }
  return `${parts.join("\n")}\n`;
}

if (import.meta.main) {
  const chart = renderChart(readFileSync(specFile, "utf8"));
  if (process.argv.includes("--check")) {
    if (readFileSync(chartFile, "utf8") !== chart) {
      console.error("The build loop chart is out of date. Run node packages/contracts/scripts/build-loop-chart.ts.");
      process.exit(1);
    }
  } else {
    writeFileSync(chartFile, chart);
  }
}
