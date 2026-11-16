import { readFileSync } from "node:fs";
import { expect } from "vitest";

// Keep full prompt equality checks, including the fresh envelope's delivery.
export const withRoleInstructions = (actual: string, deliveryId: string, caller: string, files: ReadonlyArray<string>): string => {
  const marker = /^Follow these engine-supplied role instructions\.\nData from another agent \[([A-Za-z0-9_-]{24})\]/.exec(actual)?.[1];
  expect(marker).toBeTypeOf("string");
  const instructions = files.map((file) => readFileSync(new URL(`../../src/roles/instructions/${file}.md`, import.meta.url), "utf8")).join("\n\n");
  return `Follow these engine-supplied role instructions.\nData from another agent [${marker}] (delivery ${deliveryId}):\n${instructions}\nEnd data [${marker}]\n\n${caller}`;
};

export const withWorkerInstructions = (actual: string, deliveryId: string, caller: string): string =>
  withRoleInstructions(actual, deliveryId, caller, ["worker-base"]);
