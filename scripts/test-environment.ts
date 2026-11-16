import { mkdtempSync, realpathSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterAll, afterEach } from "vitest";

// @ts-expect-error This preload must run directly in child Node processes.
import { installServiceTestOwner } from './service-test-owner.mjs';

// Test files start without the invoking agent's credentials or socket settings.
const privateEnvironment = /^(?:ANTHROPIC|CLAUDE|CODEX|OPENAI|GEMINI|GOOGLE|CURSOR|T3CODE|LGTM)_/;
for (const name of Object.keys(process.env)) {
  if (privateEnvironment.test(name)) delete process.env[name];
}

const stateDirectory = realpathSync(mkdtempSync(join(tmpdir(), "lgtm-test-")));
process.env["LGTM_HOME"] = stateDirectory;
afterAll(() => rmSync(stateDirectory, { recursive: true, force: true }));

// Existing fixture teardown runs first. The lease also catches a failed hook,
// an orphaned grandchild, and a test worker that is terminated with SIGKILL.
let serviceOwner = installServiceTestOwner();
afterEach(async () => { await serviceOwner.close(); serviceOwner = installServiceTestOwner(); });
afterAll(async () => { await serviceOwner.close(); });
