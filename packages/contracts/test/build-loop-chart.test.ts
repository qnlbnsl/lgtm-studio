import { readFileSync } from "node:fs";
import { expect, test } from "vitest";
import { chartFile, parseSteps, renderChart, specFile } from "../scripts/build-loop-chart.ts";

test("the build loop chart matches the steps written in the design", () => {
  const current = readFileSync(chartFile, "utf8");
  expect(current, "Run node packages/contracts/scripts/build-loop-chart.ts to redraw the chart.").toBe(renderChart(readFileSync(specFile, "utf8")));
});

test("every phase and block in the design gets a chart", () => {
  const headings = parseSteps(readFileSync(specFile, "utf8")).map((section) => section.heading);
  expect(headings).toEqual([
    "1 Define", "2 Design and teams", "3 Plan", "Block: Write each plan", "Block: Review each plan", "Block: Next round",
    "4 Build", "Block: Build subitems", "Block: Build without tests", "Block: Fix team",
    "5 Verify", "6 Review", "7 Hand over", "8 Keep running",
  ]);
});

test("owner steps, decisions and phase links are drawn with their own shapes", () => {
  const chart = renderChart([
    "## The steps in each phase", "", "### 1 Define", "",
    "- Start -> Ready?: look [OWNER IF NEEDED]",
    "- Ready? -> Done: yes: say \"go\" [OWNER]",
    "- Ready? -> 2 Next: no", "", "### Test types", "",
  ].join("\n"));
  expect(chart).toContain('s0(["Start"])');
  expect(chart).toContain('s1{"Ready?"}');
  expect(chart).toContain('s2("Done")');
  expect(chart).toContain('s3(["2 Next"])');
  expect(chart).toContain('s0 -.->|"look"| s1');
  expect(chart).toContain('s1 ==>|"yes: say #quot;go#quot;"| s2');
  expect(chart).toContain('s1 -->|"no"| s3');
});
