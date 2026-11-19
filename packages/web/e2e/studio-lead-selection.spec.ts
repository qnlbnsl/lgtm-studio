// The current website is built from design/mock and served at the engine root.
import { execFileSync } from "node:child_process"
import { randomBytes } from "node:crypto"
import { join } from "node:path"
import { expect, test } from "@playwright/test"
import * as Effect from "effect/Effect"
import * as Layer from "effect/Layer"
import * as Redacted from "effect/Redacted"
import * as SqliteClient from "../../../third_party/t3code/node-sqlite/src/nodeSqliteClient.ts"
import { buildServices } from "../../engine/src/service.ts"
import { serve } from "../../engine/src/client/web/server.ts"
import { engineLists } from "../../engine/src/pieces/lists.ts"
import { Redactor } from "../../engine/src/log/Redactor.ts"
import { unusedPort } from "../../engine/test/support/browserWeb.ts"
import { makeFakeRunner } from "../../engine/test/support/fakeRunner.ts"
import { seedAccount, harnessSettings, ownerCredential as owner, success, call, makeFakeSecrets, makeRig, removeRig } from "../../engine/test/support/handlerRig.ts"
import { registerRebuildCheck } from "../../engine/test/support/rebuildCheck.ts"
import { buildStudio } from "../../studio/scripts/build.ts"
import { completedSetup } from "./support/completedSetup.ts"

test("New project launches its selected harness, subscription, model and effort without changing defaults", async ({ page }, info) => {
  test.setTimeout(90_000)
  const rig = makeRig(), password = randomBytes(32).toString("base64url")
  const previousHome = process.env.LGTM_HOME, previousUserHome = process.env.HOME
  process.env.LGTM_HOME = rig.stateFolder; process.env.HOME = rig.home
  const errors: string[] = []
  page.on("pageerror", (error) => errors.push(error.message))
  try {
    execFileSync("git", ["init", "-b", "main"], { cwd: rig.work, stdio: "ignore" })
    execFileSync("git", ["-c", "user.name=Test", "-c", "user.email=test@example.test", "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "Initial"], { cwd: rig.work, stdio: "ignore" })
    const studioRoot = await buildStudio(join(rig.root, "studio")), port = await unusedPort()
    await Effect.runPromise(Effect.scoped(Effect.gen(function* () {
      const runner = yield* makeFakeRunner()
      runner.script("discover", ({ account, model }) => Effect.succeed({
        models: { source: "negotiated", options: (account.id === "claude-main" ? ["claude-model"] : account.id === "codex-main" ? ["codex-model"] : ["other-model", "quick-model"]).map((id) => ({ id, label: id })) },
        efforts: { source: "negotiated", options: (model === "quick-model" ? [] : account.id === "claude-main" ? ["high"] : ["low", "high"]).map((id) => ({ id, label: id })) }
      }))
      const dependencies = Layer.mergeAll(runner.layer, makeFakeSecrets().layer, Layer.succeed(Redactor, {
        redact: (event) => Effect.succeed(event), redactText: (text) => Effect.succeed(text)
      }))
      yield* Effect.scoped(Effect.gen(function* () {
        yield* registerRebuildCheck()
        for (const [id, harness, name] of [["claude-main", "claude", "Claude personal"], ["codex-main", "codex", "Codex personal"], ["codex-other", "codex", "An unusually long subscription name for the second Codex account"]]) {
          yield* seedAccount(id!, harness!)
          success(yield* call(owner, "accounts", { operation: "set", account: { id, harness, displayName: name, enabled: true } }))
          success(yield* call(owner, "accounts set-secret", { accountId: id, name: harness === "claude" ? "ANTHROPIC_API_KEY" : "OPENAI_API_KEY", nonce: id, value: Redacted.make("synthetic-browser-credential-" + id) }))
        }
        for (const harness of ["claude", "codex"]) success(yield* call(owner, "harnesses", { operation: "set", harness: harnessSettings(harness, {
          models: harness === "claude" ? ["claude-model"] : ["codex-model", "other-model", "quick-model"], efforts: ["low", "high"],
          defaultModel: harness === "claude" ? "claude-model" : "codex-model", defaultEffort: "high", reportedModes: [{ id: "default", level: "ask" }]
        }) }))
        const defaults = { harness: "claude", account: { _tag: "named", accountId: "claude-main" }, model: "claude-model", effort: "high", permission: "ask" }
        success(yield* call(owner, "project.settings.set", { settings: { workspaceFolder: rig.root, shell: "sh", lead: defaults } }))
        yield* completedSetup(undefined, rig.work, "Product")
        yield* serve({ password: Redacted.make(password), staticRoot: studioRoot, lists: engineLists,
          settings: { enabled: true, port, heartbeatSeconds: 25, proxyMode: false } })
        yield* Effect.promise(async () => {
          await page.setViewportSize({ width: 1440, height: 1000 })
          await page.goto(`http://127.0.0.1:${port}/`)
          await page.locator("#login-password").fill(password)
          await page.getByRole("button", { name: "Sign in", exact: true }).click()
          await page.getByRole("button", { name: "New project", exact: true }).first().click()
          const dialog = page.getByRole("dialog", { name: "New project", exact: true })
          await dialog.getByLabel("Goal", { exact: true }).fill("Build the selected lead")
          const harness = dialog.getByLabel("Harness", { exact: true }), account = dialog.getByLabel("Subscription", { exact: true })
          const model = dialog.getByLabel("Model", { exact: true }), effort = dialog.getByLabel("Effort", { exact: true })
          await expect(harness).toHaveValue("claude")
          await expect(account).toHaveValue("claude-main")
          await expect(model).toHaveValue("claude-model")
          await expect(effort).toHaveValue("high")
          await harness.selectOption("codex")
          await expect(account).toHaveValue("codex-main")
          await expect(model.locator("option")).toContainText(["Harness default", "codex-model"])
          await model.selectOption("codex-model")
          await expect(effort).toBeEnabled()
          await effort.selectOption("high")
          await account.selectOption("codex-other")
          await expect(model.locator("option")).toContainText(["Harness default", "other-model", "quick-model"])
          await expect(model.locator('option[value="codex-model"]')).toHaveCount(0)
          await expect(effort).toHaveValue("")
          await model.selectOption("quick-model")
          await expect(effort).toBeDisabled()
          await expect(dialog).toContainText("This harness did not report effort choices.")
          await model.selectOption("other-model")
          await expect(effort).toBeEnabled()
          await effort.selectOption("low")
          for (const width of [1440, 390]) {
            await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 })
            await expect(dialog).toBeVisible()
            expect(await dialog.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true)
            for (const control of [harness, account, model, effort]) {
              const box = await control.boundingBox()
              expect(box?.width).toBeGreaterThan(100)
              expect(box!.x + box!.width).toBeLessThanOrEqual(width)
              await expect.poll(() => control.evaluate((el) => {
                const b = el.getBoundingClientRect()
                return document.elementFromPoint(b.x + b.width / 2, b.y + b.height / 2) === el
              })).toBe(true)
            }
            await page.screenshot({ path: info.outputPath(`new-project-${width}.png`), animations: "disabled" })
          }
          await dialog.getByRole("button", { name: "Create project", exact: true }).click()
          await expect(dialog).toHaveCount(0)
          await expect(page.locator(".topbar")).toContainText("Selected lead")
        })
        expect(runner.callsOf("start")).toHaveLength(1)
        expect(runner.callsOf("start")[0]?.input).toMatchObject({ harness: { id: "codex" }, account: { id: "codex-other" }, model: "other-model", effort: "low", permission: "ask" })
        expect(success(yield* call(owner, "project.settings", {})).settings.lead).toEqual(defaults)
      })).pipe(Effect.provide(buildServices(rig, engineLists).pipe(Layer.provide(dependencies))), Effect.provide(runner.layer))
    })).pipe(Effect.provide(SqliteClient.layer({ filename: rig.database, readerWorker: true })), Effect.scoped) as Effect.Effect<void, unknown, never>)
    expect(errors).toEqual([])
  } finally {
    if (previousHome === undefined) delete process.env.LGTM_HOME; else process.env.LGTM_HOME = previousHome
    if (previousUserHome === undefined) delete process.env.HOME; else process.env.HOME = previousUserHome
    removeRig(rig)
  }
})
