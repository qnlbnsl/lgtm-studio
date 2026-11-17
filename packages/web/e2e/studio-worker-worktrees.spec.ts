import { randomBytes } from "node:crypto";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { expect as baseExpect, test } from "@playwright/test";
import * as Effect from "effect/Effect";
import * as Layer from "effect/Layer";
import * as Redacted from "effect/Redacted";
import * as SqliteClient from "../../../third_party/t3code/node-sqlite/src/nodeSqliteClient.ts";
import { buildServices } from "../../engine/src/service.ts";
import { serve } from "../../engine/src/client/web/server.ts";
import { engineLists } from "../../engine/src/pieces/lists.ts";
import { Views } from "../../engine/src/views/Views.ts";
import { Redactor } from "../../engine/src/log/Redactor.ts";
import { unusedPort } from "../../engine/test/support/browserWeb.ts";
import { makeFakeRunner } from "../../engine/test/support/fakeRunner.ts";
import { call, makeFakeSecrets, makeRig, ownerCredential, removeRig, success } from "../../engine/test/support/handlerRig.ts";
import { registerRebuildCheck } from "../../engine/test/support/rebuildCheck.ts";
import { workerCommit, workerPayload, workerProject } from "../../engine/test/support/workerWorktrees.ts";
import { completedSetup } from "./support/completedSetup.ts";
import { buildStudio } from "../../studio/scripts/build.ts";

const expect = baseExpect.configure({ timeout: 30_000 });

test("allocated worker appears in the root website Git tab and integrates into its lead", async ({ page }) => {
  test.setTimeout(90_000);
  const rig = makeRig(), previous = process.env.LGTM_HOME;
  process.env.LGTM_HOME = rig.stateFolder;
  const password = randomBytes(32).toString("base64url"), port = await unusedPort();
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  try {
    const studioRoot = await buildStudio(join(rig.root, "studio"));
    await Effect.runPromise(Effect.scoped(Effect.gen(function* () {
      const runner = yield* makeFakeRunner();
      const dependencies = Layer.mergeAll(runner.layer, makeFakeSecrets().layer, Layer.succeed(Redactor, {
        redact: (event) => Effect.succeed(event), redactText: (text) => Effect.succeed(text),
      }));
      yield* Effect.scoped(Effect.gen(function* () {
        yield* registerRebuildCheck();
        const project = yield* workerProject(rig);
        yield* completedSetup(project.projectId);
        // Exercise the owner's start API. The same planner serves lead tools.
        const reply = success(yield* call(ownerCredential, "start", workerPayload(project.path, { projectId: project.projectId })));
        const worker = (yield* (yield* Views).snapshot).agents[reply.agentId]!;
        workerCommit(worker.folder, "browser-worker.txt", "A worker's contribution.\n");
        yield* serve({ password: Redacted.make(password), staticRoot: studioRoot, lists: engineLists,
          settings: { enabled: true, port, heartbeatSeconds: 25, proxyMode: false } });
        yield* Effect.promise(async () => {
          const url = `http://127.0.0.1:${port}/#/p/${project.projectId}/git`;
          await page.goto(url);
          await page.locator("#login-password").fill(password);
          await page.getByRole("button", { name: "Sign in", exact: true }).click();
          await expect(page.locator(".tree-row.project")).toBeVisible();
          await page.goto(url);
          await page.getByRole("button", { name: "Choose a worktree", exact: true }).click();
          await page.getByRole("menuitemradio", { name: /worker\/work-trees\/builder/ }).click();
          await expect(page.locator(".git")).toHaveAttribute("aria-busy", "false");
          await expect(page.getByRole("button", { name: /Integrate into/ })).toBeEnabled();
          await page.getByRole("button", { name: /Integrate into/ }).click();
          await expect(page.locator(".git")).toHaveAttribute("aria-busy", "false");
          expect(readFileSync(join(project.path, "browser-worker.txt"), "utf8")).toBe("A worker's contribution.\n");
          await page.goto(`http://127.0.0.1:${port}/#/p/${project.projectId}/files`);
          await page.getByRole("button", { name: "Inspecting: the worktree whose code you look at", exact: true }).click();
          await expect(page.getByRole("menuitemradio", { name: /worker\/work-trees\/builder/ })).toBeVisible();
          await page.getByRole("menuitemradio", { name: /worker\/work-trees\/builder/ }).click();
          await page.goto(`http://127.0.0.1:${port}/#/p/${project.projectId}/terminal`);
          // Enable the existing switch without opening a shell.
          await page.getByRole("link", { name: "Open Service settings", exact: true }).click();
          await page.getByRole("switch", { name: "Project terminal", exact: true }).click();
          await page.goto(`http://127.0.0.1:${port}/#/p/${project.projectId}/terminal`);
          await expect(page.getByRole("combobox", { name: "Open in", exact: true }).locator("option").filter({ hasText: "worker/work-trees/builder" })).toHaveCount(1);
          expect(errors).toEqual([]);
        });
      })).pipe(Effect.provide(buildServices(rig, engineLists).pipe(Layer.provide(dependencies))), Effect.provide(runner.layer));
    })).pipe(Effect.provide(SqliteClient.layer({ filename: rig.database, readerWorker: true })), Effect.scoped) as Effect.Effect<void, unknown>);
  } finally {
    if (previous === undefined) delete process.env.LGTM_HOME; else process.env.LGTM_HOME = previous;
    removeRig(rig);
  }
});
