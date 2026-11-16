import { readFileSync } from "node:fs";
import { expect } from "vitest";

// Keep exact prompt equality. Engine instructions carry no delivery proof.
export const withRoleInstructions = (actual: string, deliveryId: string, caller: string, files: ReadonlyArray<string>): string => {
  const marker = /^Engine role instructions \[([A-Za-z0-9_-]{24})\]:\n/.exec(actual)?.[1];
  expect(marker).toBeTypeOf("string");
  const instructions = files.map((file) => readFileSync(new URL(`../../src/roles/instructions/${file}.md`, import.meta.url), "utf8")).join("\n\n");
  expect(actual.slice(0, actual.indexOf(`End engine role instructions [${marker}]`))).not.toContain(`(delivery ${deliveryId})`);
  return `Engine role instructions [${marker}]:\n${instructions}\nEnd engine role instructions [${marker}]\n\n${caller}`;
};

export const withWorkerInstructions = (actual: string, deliveryId: string, caller: string): string =>
  withRoleInstructions(actual, deliveryId, caller, ["worker-base"]);
