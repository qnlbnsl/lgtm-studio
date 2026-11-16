import { phoneProject } from "./support/phoneProject.ts"
import { randomBytes } from "node:crypto"
import { join } from "node:path"
import { expect, test } from "@playwright/test"
import { buildStudio } from "../../studio/scripts/build.ts"
import * as SqliteClient from "../../../third_party/t3code/node-sqlite/src/nodeSqliteClient.ts"
import * as Effect from "effect/Effect"
import * as Layer from "effect/Layer"
import * as Redacted from "effect/Redacted"
import { buildServices } from "../../engine/src/service.ts"
import { serve } from "../../engine/src/client/web/server.ts"
import { Redactor } from "../../engine/src/log/Redactor.ts"
import { engineLists } from "../../engine/src/pieces/lists.ts"
import { unusedPort } from "../../engine/test/support/browserWeb.ts"
import { makeFakeRunner, incarnationOf } from "../../engine/test/support/fakeRunner.ts"
import { configure, ntfyServer, posts } from "../../engine/test/support/carrierRig.ts"
import {
  call, makeFakeSecrets, makeRig, ownerCredential, removeRig,
  seedAccount, seedHarness, startPayload, success
} from "../../engine/test/support/handlerRig.ts"
import { registerRebuildCheck } from "../../engine/test/support/rebuildCheck.ts"
import { httpsProxy } from "./support/httpsProxy.ts"

// This browser test ignores errors for the proxy's self-signed certificate.
// It installs no certificate trust; the production listener stays on HTTP.
test.use({ ignoreHTTPSErrors: true })

test("website phone permission alert: signs in, answers once, releases the agent and reads back after reload", async ({ page }) => {
  const rig = makeRig()
  const ntfy = await ntfyServer()
  const previousHome = process.env.LGTM_HOME
  process.env.LGTM_HOME = rig.stateFolder
  const password = randomBytes(32).toString("base64url")
  const staticRoot = join(rig.root, "page")
  let proxy: Awaited<ReturnType<typeof httpsProxy>> | undefined
  try {
    await buildStudio(staticRoot)
    const port = await unusedPort()
    proxy = await httpsProxy(port, rig.root)
    const origin = proxy.origin
    const program = Effect.gen(function* () {
      const runner = yield* makeFakeRunner()
      const run = Effect.gen(function* () {
        yield* registerRebuildCheck()
        yield* seedHarness()
        yield* seedAccount()
        yield* serve({ password: Redacted.make(password), staticRoot, lists: engineLists,
          settings: { enabled: true, port, heartbeatSeconds: 25, proxyMode: true, publicAddress: origin } })
        yield* configure(ntfy.origin)
        success(yield* call(ownerCredential, "client.web.settings.set", { settings: {
          enabled: true, port, heartbeatSeconds: 25, proxyMode: true, publicAddress: origin,
        } }))
        const worker = success(yield* call(ownerCredential, "start", startPayload(rig, { name: "PhoneWorker" })))
        const projectId = yield* phoneProject(worker.agentId, rig.work)
        yield* runner.subscribed
        success(yield* call(ownerCredential, "prompt", { agentId: worker.agentId, text: "Run the phone permission check." }))
        yield* Effect.promise(async () => {
          await expect.poll(() => runner.callsOf("prompt").length).toBe(1)
        })
        const sent = runner.callsOf("prompt")[0]!.input
        const releasedText = "The approved phone command finished."
        runner.script("answerPermission", (input) => {
          expect(input).toEqual({ agentId: worker.agentId, requestId: "phone-permission", optionId: "once" })
          return runner.emitRaw({ _tag: "deliveryUpdate", agentId: worker.agentId, incarnation: sent.incarnation,
            deliveryId: sent.delivery.id, kind: "message", role: "agent", replay: false, text: releasedText,
            content: { _tag: "text", text: releasedText } })
        })
        const start = runner.callsOf("start").find((entry) => entry.input.agentId === worker.agentId)!
        yield* runner.emitRaw({
          _tag: "permissionRequested", agentId: worker.agentId, incarnation: incarnationOf(Redacted.value(start.input.token)),
          permissionRequestId: "phone-permission", toolCallId: "phone-tool", description: "Run a harmless command",
          options: [{ id: "once", name: "Allow once", persistent: false, kind: "allow_once", level: "ask" },
            { id: "reject", name: "Reject", persistent: false, kind: "reject_once", level: "read-only" }],
          settingsWrite: false, toolKind: "execute", toolInput: { command: "printf phone-flow" },
        })
        yield* Effect.promise(async () => {
          await expect.poll(() => posts(ntfy).length).toBeGreaterThan(0)
          const request = posts(ntfy)[0]!
          const headerClick = request.headers.click ?? request.headers["x-click"]
          let click = Array.isArray(headerClick) ? headerClick[0] : headerClick
          if (click === undefined && request.headers["content-type"]?.includes("application/json")) {
            click = (JSON.parse(request.body) as { click?: string }).click
          }
          expect(click, "the phone alert must carry a thread link").toBeDefined()
          expect(typeof click).toBe("string")
          const link = new URL(click!)
          expect(link.origin).toBe(origin)
          expect(link.href).toBe(`${origin}/#/p/${projectId}/chat/${worker.agentId}?req=phone-permission`)
          expect(link.href).not.toMatch(/token|credential|password|approve|answer|dismiss/i)

          await page.setViewportSize({ width: 360, height: 800 })
          await page.goto(origin)
          await page.locator("#login-password").fill(password)
          await page.getByRole("button", { name: "Sign in", exact: true }).click()
          await expect(page.locator(".home")).toBeVisible()
          expect(await page.evaluate(() => ({ secure: isSecureContext, uuid: typeof crypto.randomUUID })))
            .toEqual({ secure: true, uuid: "function" })
          expect((await page.context().cookies(origin)).some((cookie) => cookie.secure && cookie.httpOnly && cookie.sameSite === "Strict")).toBe(true)
          const documentHandle = await page.evaluateHandle(() => window.document)
          await page.goto(link.href)
          await expect(page).toHaveURL(link.href)
          expect(await page.evaluate((previous) => previous === window.document, documentHandle),
            "the hash link must keep the logged-in document").toBe(true)
          await documentHandle.dispose()
          await expect(page.locator(".chat-main h1")).toHaveText("Project lead")
          const requestPanel = page.locator("#req-phone-permission")
          await requestPanel.getByRole("button", { name: "Details", exact: true }).click()
          await expect(requestPanel).toContainText("printf phone-flow")
          await requestPanel.getByRole("button", { name: "Allow", exact: true }).click()
          await expect.poll(() => runner.callsOf("answerPermission").length).toBe(1)
          expect(runner.callsOf("answerPermission")[0]!.input).toEqual({
            agentId: worker.agentId, requestId: "phone-permission", optionId: "once"
          })
          await expect(requestPanel.getByRole("button", { name: "Allowed", exact: true })).toBeDisabled()
          await expect(page.locator(".chat-main .thread")).toContainText(releasedText)
          await expect(requestPanel).toHaveClass(/done/)
          await expect(requestPanel.getByRole("button", { name: "Allow", exact: true })).toHaveCount(0)
          // A fresh document must read the committed answer and the released
          // agent's output, without resending the permission answer.
          const readback = await page.reload()
          expect(readback?.status()).toBe(200)
          await expect(page.locator(".chat-main h1")).toBeVisible()
          await expect(requestPanel.getByRole("button", { name: "Allowed", exact: true })).toBeDisabled()
          await expect(page.locator(".chat-main .thread")).toContainText(releasedText)
          await expect(requestPanel).toHaveClass(/done/)
          await expect(requestPanel.getByRole("button", { name: "Allow", exact: true })).toHaveCount(0)
          expect(runner.callsOf("answerPermission")).toHaveLength(1)
          const widths = await page.evaluate(() => ({ viewport: document.documentElement.clientWidth,
            document: document.documentElement.scrollWidth, body: document.body.scrollWidth }))
          expect(widths.document).toBeLessThanOrEqual(widths.viewport)
          expect(widths.body).toBeLessThanOrEqual(widths.viewport)
        })
      })
      const dependencies = Layer.mergeAll(runner.layer, makeFakeSecrets().layer, Layer.succeed(Redactor, {
        redact: (event) => Effect.succeed(event), redactText: (text) => Effect.succeed(text)
      }))
      yield* Effect.scoped(run).pipe(Effect.provide(buildServices(rig, engineLists).pipe(Layer.provide(dependencies))),
        Effect.provide(runner.layer))
    }).pipe(Effect.provide(SqliteClient.layer({ filename: rig.database, readerWorker: true })), Effect.scoped)
    await Effect.runPromise(program as Effect.Effect<void, unknown>)
  } finally {
    await proxy?.close()
    await ntfy.close()
    if (previousHome === undefined) delete process.env.LGTM_HOME
    else process.env.LGTM_HOME = previousHome
    removeRig(rig)
  }
})
