import { execFileSync } from "node:child_process"
import { createHash, randomUUID } from "node:crypto"
import { mkdirSync, writeFileSync, symlinkSync, unlinkSync, truncateSync, rmSync } from "node:fs"
import { join } from "node:path"
import * as Effect from "effect/Effect"
import * as Schema from "effect/Schema"
import { ProjectMapRpcGroup } from "@lgtm/contracts/project/map"
import { afterEach, expect, it, vi } from "vitest"
import { createRequire } from "node:module"
// jsdom is already a root test dependency. This test uses only its standard
// browser interfaces; the repository does not install @types/jsdom.
const { JSDOM } = createRequire(import.meta.url)("jsdom") as {
  JSDOM: new (html: string, options?: { runScripts: "dangerously" }) => { window: Window & typeof globalThis }
}
import { CommandHandler } from "../../src/commands/CommandHandler.ts"
import * as MapFiles from "../../src/project/mapFile.ts"
import { pathToFileURL } from "node:url"
import { mapByteLimit } from "../../src/project/map.ts"
import { makeDisplaySessions } from "../../src/client/web/display/sessions.ts"
import { makeMockRenderer } from "../../src/client/web/display/renderer.ts"
import { displayConfiguration } from "../../src/client/web/display/policy.ts"
import { reskinMap } from "../../src/client/web/display/mapMarkup.ts"
import { registerRebuildCheck } from "../support/rebuildCheck.ts"
import { agentCredential, appendSetup, call, eventOf, ownerCredential as owner, success, readLog } from "../support/handlerRig.ts"
import { seedScreens } from "./mockScreenFixtures.ts"
import { withWeb, freePort, http, setSettings } from "../support/webRig.ts"

registerRebuildCheck({ noLog: true })
afterEach(() => vi.restoreAllMocks())
const page = '<!doctype html><html><head><style>body{color:red}</style></head><body><h1>Map fixture</h1><script>window.mapReady=true</script></body></html>'
const expectedPolicy = (ownerOrigin: string) => `default-src 'none'; sandbox allow-scripts allow-popups; frame-ancestors ${ownerOrigin}; base-uri 'none'; form-action 'none'; object-src 'none'; frame-src 'none'; connect-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src 'none'; worker-src 'none'; webrtc 'block'`
const writeMap = (root: string) => { mkdirSync(join(root, "floorplan"), { recursive: true }); writeFileSync(join(root, "floorplan/index.html"), page) }

it("reads only map metadata and real optional graph facts, selects project or workspace, and refuses agent identities", async () => withWeb(ctx => Effect.gen(function* () {
  const seeded = yield* seedScreens(ctx.rig)
  const status = (selection: Record<string, unknown>) => call(owner, "project.map", selection).pipe(Effect.map(success))
  expect((yield* status({ repoId: seeded.repoId })).result).toEqual({ exists: false })
  writeMap(ctx.rig.work)
  writeFileSync(join(ctx.rig.work, "floorplan/graph.json"), JSON.stringify({ meta: { generatedAt: "2026-11-23T12:00:00Z", counts: { files: 7, loc: 42 } }, nodes: [{}, {}], links: [{}], private: "not returned" }))
  const before = yield* readLog
  for (const selection of [{ repoId: seeded.repoId }, { projectId: "project" }, { worktreeId: "workspace" }]) {
    const reply = yield* status(selection)
    expect(reply.result).toEqual({ exists: true, size: Buffer.byteLength(page), modifiedAt: expect.any(Number),
      facts: { generatedAt: "2026-11-23T12:00:00Z", files: 7, loc: 42, nodes: 2, links: 1 } })
    expect(JSON.stringify(reply)).not.toContain("Map fixture")
  }
  writeFileSync(join(ctx.rig.work, "floorplan/graph.json"), "not JSON")
  expect((yield* status({ repoId: seeded.repoId })).result.facts).toBeUndefined()
  writeFileSync(join(ctx.rig.work, "floorplan/graph.json"), JSON.stringify({ meta: { counts: { files: -1, loc: "wrong" } } }))
  expect((yield* status({ repoId: seeded.repoId })).result.facts).toEqual({})
  const agent = agentCredential(seeded.lead.token)
  for (const [name, payload] of [["project.map", { projectId: "project" }], ["project.map.open", { projectId: "project" }], ["project.map.release", { id: "frame" }]] as const) {
    expect(Schema.is(Schema.toType(ProjectMapRpcGroup.requests.get(name)!.payloadSchema))({ ...payload, requestId: randomUUID() })).toBe(true)
    const reply = yield* call(agent, name, payload)
    expect(reply._tag).toBe("Failure")
    expect(JSON.stringify(reply)).toContain("command refused")
  }
  expect((yield* call(owner, "project.map", {}))._tag).toBe("Failure")
  expect((yield* call(owner, "project.map", { projectId: "project", repoId: "other" }))._tag).toBe("Failure")
  expect(yield* readLog).toEqual(before)
})))

it("selects a registered repository even before any workspace is registered", async () => withWeb(ctx => Effect.gen(function* () {
  execFileSync("git", ["-C", ctx.rig.work, "init", "-b", "main"], { stdio: "ignore" })
  success(yield* call(owner, "project.settings.set", { settings: { workspaceFolder: ctx.rig.root, shell: "sh" } }))
  const { repoId } = success(yield* call(owner, "project.repository.add", { name: "Map", path: ctx.rig.work, machineId: "local" }))
  writeMap(ctx.rig.work)
  expect(success(yield* call(owner, "project.map", { repoId })).result).toMatchObject({ exists: true })
})))

it("uses a project's distinct registered worktree rather than the repository's main checkout", async () => withWeb(ctx => Effect.gen(function* () {
  const seeded = yield* seedScreens(ctx.rig), worktree = join(ctx.rig.root, "project-checkout")
  execFileSync("git", ["-C", ctx.rig.work, "-c", "user.name=Map fixture", "-c", "user.email=map@example.test", "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "Map fixture"], { stdio: "ignore" })
  execFileSync("git", ["-C", ctx.rig.work, "worktree", "add", "-b", "map-project", worktree], { stdio: "ignore" })
  const registered = success(yield* call(owner, "project.workspace.register", { repoId: seeded.repoId, machineId: "local", path: worktree, branch: "map-project", checkout: "worktree", parentId: "workspace" }))
  yield* appendSetup([eventOf({ _tag: "project.Created", projectId: "map-project", name: "Map worktree", goal: "Map this checkout", repoId: seeded.repoId, worktreeId: registered.worktreeId, leadAgentId: seeded.lead.agentId, machineId: "local" })])
  writeMap(worktree)
  expect(success(yield* call(owner, "project.map", { repoId: seeded.repoId })).result.exists).toBe(false)
  expect(success(yield* call(owner, "project.map", { projectId: "map-project" })).result.exists).toBe(true)
  expect(success(yield* call(owner, "project.map", { projectId: "map-project", worktreeId: registered.worktreeId })).result.exists).toBe(true)
})))

it("refuses outside symlinks, nonregular pages and oversized pages on both status and display", async () => withWeb(ctx => Effect.gen(function* () {
  const seeded = yield* seedScreens(ctx.rig), handler = yield* CommandHandler
  const configuration = displayConfiguration(ctx.origin, "https://map.example", yield* Effect.promise(freePort))
  const sessions = makeDisplaySessions(configuration, () => {})
  const token = sessions.issue({ kind: "map", selection: { repoId: seeded.repoId } })
  const render = makeMockRenderer({ configuration, sessions, input: () => Effect.runPromise(handler.input({ _tag: "owner" })) })
  const check = () => Effect.gen(function* () {
    expect((yield* call(owner, "project.map", { repoId: seeded.repoId }))._tag).toBe("Failure")
    expect((yield* Effect.promise(() => render({ url: new URL(token.base).pathname + "index.html", method: "GET", headers: { host: "map.example" } }))).status).toBe(404)
  })
  writeFileSync(join(ctx.rig.outside, "index.html"), page)
  symlinkSync(ctx.rig.outside, join(ctx.rig.work, "floorplan")); yield* check(); unlinkSync(join(ctx.rig.work, "floorplan"))
  mkdirSync(join(ctx.rig.work, "floorplan"))
  symlinkSync(join(ctx.rig.outside, "index.html"), join(ctx.rig.work, "floorplan/index.html")); yield* check(); unlinkSync(join(ctx.rig.work, "floorplan/index.html"))
  mkdirSync(join(ctx.rig.work, "floorplan/index.html")); yield* check(); rmSync(join(ctx.rig.work, "floorplan/index.html"), { recursive: true })
  writeMap(ctx.rig.work); truncateSync(join(ctx.rig.work, "floorplan/index.html"), mapByteLimit + 1); yield* check()
})))

it("opens and releases a real isolated address through owner commands without a browser sign-in, and serves its policy and credit", async () => withWeb(ctx => Effect.gen(function* () {
  const seeded = yield* seedScreens(ctx.rig), port = yield* Effect.promise(freePort)
  writeMap(ctx.rig.work)
  expect((yield* call(owner, "project.map.open", { repoId: seeded.repoId }))._tag).toBe("Failure")
  yield* setSettings({ ...ctx.settings, display: { publicAddress: "https://map.example", port } } as any)
  const before = yield* readLog
  const opened = success(yield* call(owner, "project.map.open", { projectId: "project" }))
  expect(opened.frame).toMatchObject({ displayOrigin: "https://map.example", expiresAt: expect.any(Number), id: expect.any(String) })
  expect(opened.frame.expiresAt - Date.now()).toBeGreaterThan(9 * 60_000)
  const address = new URL(opened.frame.url), path = address.pathname
  const endpoint = { settings: { ...ctx.settings, port }, origin: "null", host: "map.example" }
  const response = yield* Effect.promise(() => http(endpoint, path))
  expect(response.status).toBe(200)
  const policy = response.headers["content-security-policy"] as string
  expect(policy).toBe(expectedPolicy(ctx.origin))
  expect(response.headers).not.toHaveProperty("access-control-allow-origin")
  expect(response.headers).not.toHaveProperty("cross-origin-resource-policy")
  const document = new JSDOM(response.body).window.document
  expect(document.querySelectorAll("#lgtm-map-style")).toHaveLength(1)
  const style = document.querySelector("#lgtm-map-style")!
  expect(style.hasAttribute("nonce")).toBe(false)
  expect(style.textContent).toContain("--sans:")
  expect(style.textContent).not.toContain("--bg:")
  expect(document.querySelector('a[href="https://github.com/haydenschmitty/floorplan"]')?.outerHTML).toContain('target="_blank" rel="noopener noreferrer"')
  for (const other of [path.replace("index.html", "graph.json"), path.replace("index.html", "serve.mjs"), "/api", path + "?other=1"]) expect((yield* Effect.promise(() => http(endpoint, other))).status).not.toBe(200)
  expect((yield* Effect.promise(() => http(endpoint, path, { headers: { Cookie: "owner=synthetic" } }))).status).toBe(403)
  expect((yield* Effect.promise(() => http(endpoint, path, { method: "HEAD" }))).body).toBe("")
  success(yield* call(owner, "project.map.release", { id: opened.frame.id }))
  expect((yield* Effect.promise(() => http(endpoint, path))).status).not.toBe(200)
  expect(yield* readLog).toEqual(before)
}), { registerSecret: () => {} }))

it("expires a map address at ten minutes and refuses serving it after expiry", async () => withWeb(ctx => Effect.gen(function* () {
  const seeded = yield* seedScreens(ctx.rig), handler = yield* CommandHandler
  writeMap(ctx.rig.work)
  let now = 0
  const configuration = displayConfiguration(ctx.origin, "https://map.example", 4321)
  const sessions = makeDisplaySessions(configuration, () => {}, () => now)
  const frame = sessions.issue({ kind: "map", selection: { repoId: seeded.repoId } })
  const render = makeMockRenderer({ configuration, sessions, input: () => Effect.runPromise(handler.input({ _tag: "owner" })) })
  const request = { url: new URL(frame.base).pathname + "index.html", method: "GET", headers: { host: "map.example" } }
  expect((yield* Effect.promise(() => render(request))).status).toBe(200)
  now = frame.expiresAt
  expect((yield* Effect.promise(() => render(request))).status).toBe(401)
})))

it("keeps one original credit for static and script-created Floorplan links and injects its style once", async () => {
  const href = "https://github.com/haydenschmitty/floorplan"
  for (const credit of [`<a id="madewith" href="${href}">Made with Floorplan</a>`, `<script>const a=document.createElement('a');a.id='madewith';a.href='${href}';a.textContent='Made with Floorplan';document.body.append(a)</script>`, ""]) {
    const served = reskinMap(page.replace("</body>", `<a id="source-link" href="${href}">Source</a>` + credit + "</body>"))
    const dom = new JSDOM(served.text, { runScripts: "dangerously" })
    await new Promise<void>(resolve => dom.window.addEventListener("load", () => resolve()))
    const links = dom.window.document.querySelectorAll(`a.lgtm-floorplan-credit[href="${href}"]`)
    expect(links).toHaveLength(1)
    expect(links[0]?.textContent).toBe("Made with Floorplan")
    expect(links[0]?.getAttribute("rel")).toBe("noopener noreferrer")
    expect(links[0]?.classList.contains("lgtm-floorplan-credit")).toBe(true)
    expect(dom.window.document.querySelector("#source-link")?.textContent).toBe("Source")
    expect(new JSDOM(reskinMap(served.text).text).window.document.querySelectorAll("#lgtm-map-style")).toHaveLength(1)
    dom.window.close()
  }
})

it("admits the helper hash through both script directives without broadening Floorplan's meta policy", () => {
  for (const directives of ["script-src", "script-src-elem", "script-src;script-src-elem"]) {
    const original = `default-src 'none'; ${directives.split(";").map(d => `${d} 'sha256-original'`).join("; ")}; connect-src 'self'; style-src 'unsafe-inline'`
    const fixture = page.replace("<head>", `<head><meta http-equiv="Content-Security-Policy" data-floorplan-policy="local" content="${original}">`)
    const document = new JSDOM(reskinMap(fixture).text).window.document
    const script = document.querySelector("#lgtm-map-credit-script")!.textContent!
    const hash = createHash("sha256").update(script).digest("base64")
    expect(document.querySelector("meta[data-floorplan-policy]")!.getAttribute("content"))
      .toBe(original.replaceAll("'sha256-original'", `'sha256-original' 'sha256-${hash}'`))
  }
})

it("supports runtime style creation and preserves the original palettes when the theme changes", async () => {
  const fixture = page.replace("</head>", `<style>:root{--bg:darkgray}:root[data-theme="light"]{--bg:ivory}</style></head>`)
    .replace("</body>", `<script>const style=document.createElement('style');style.textContent='.runtime-style{color:green}';style.id='runtime-style';document.head.appendChild(style)</script></body>`)
  // jsdom executes the fixture, but does not enforce Content Security Policy.
  // The HTTP test above asserts the entire response policy independently.
  const dom = new JSDOM(reskinMap(fixture).text, { runScripts: "dangerously" })
  await new Promise<void>(resolve => dom.window.addEventListener("load", () => resolve()))
  expect(dom.window.document.querySelector("#runtime-style")?.textContent).toBe(".runtime-style{color:green}")
  expect(dom.window.document.querySelector("#runtime-style")?.hasAttribute("nonce")).toBe(false)
  const root = dom.window.document.documentElement
  expect(dom.window.getComputedStyle(root).getPropertyValue("--bg")).toBe("darkgray")
  root.dataset.theme = "light"
  expect(dom.window.getComputedStyle(root).getPropertyValue("--bg")).toBe("ivory")
  dom.window.close()
})

it("removes automatic refresh and speculative links, preserving unrelated metadata and stylesheets", () => {
  const hints = ["dns-prefetch", "preconnect", "prefetch", "preload", "modulepreload", "prerender"]
  const fixture = page.replace("<head>", `<head><meta http-equiv="ReFrEsH" content="0;url=https://outside.example"><meta name="description" content="kept"><link rel="stylesheet" href="local.css">${hints.map(rel => `<link rel="alternate ${rel.toUpperCase()}" href="https://outside.example">`).join("")}`)
  const document = new JSDOM(reskinMap(fixture).text).window.document
  expect(document.querySelector("meta[http-equiv]")).toBeNull()
  expect(document.querySelector('meta[name="description"]')?.getAttribute("content")).toBe("kept")
  expect([...document.querySelectorAll("link")].map(link => link.rel)).toEqual(["stylesheet"])
})

it("reuses a prepared response on reload, invalidates edits and rechecks newly registered secrets", async () => withWeb(ctx => Effect.gen(function* () {
  const seeded = yield* seedScreens(ctx.rig), handler = yield* CommandHandler
  writeMap(ctx.rig.work)
  const input = yield* handler.input({ _tag: "owner" })
  const read = vi.spyOn(MapFiles, "readMapFile")
  let key = {}, privatePage = false
  const redactPage = vi.fn((text: string) => Effect.succeed(privatePage && text.includes("Map fixture") ? "redacted" : text))
  const currentInput = { ...input, runtime: { ...input.runtime, log: { ...input.runtime.log, redactText: redactPage,
    redactor: { ...input.runtime.log.redactor!, cacheKey: Effect.sync(() => key) } } } }
  const configuration = displayConfiguration(ctx.origin, "https://map.example", 4321)
  const sessions = makeDisplaySessions(configuration, () => {})
  const frame = sessions.issue({ kind: "map", selection: { repoId: seeded.repoId } })
  const render = makeMockRenderer({ configuration, sessions, input: async () => currentInput })
  const request = { url: new URL(frame.base).pathname + "index.html", method: "GET", headers: { host: "map.example" } }
  const first = yield* Effect.promise(() => render(request))
  expect(first.status).toBe(200)
  const second = yield* Effect.promise(() => render(request))
  expect(second.status).toBe(200)
  expect(second.body).toBe(first.body)
  expect(read).toHaveBeenCalledTimes(1)
  expect(redactPage.mock.calls.filter(([text]) => text === page)).toHaveLength(1)
  writeFileSync(join(ctx.rig.work, "floorplan/index.html"), page.replace("Map fixture", "Map fixture changed"))
  const edited = yield* Effect.promise(() => render(request))
  expect(edited.status).toBe(200)
  expect(edited.body).not.toBe(first.body)
  expect(Buffer.from(edited.body).toString()).toContain("Map fixture changed")
  key = {}; privatePage = true
  expect((yield* Effect.promise(() => render(request))).status).toBe(404)
  privatePage = false; key = {}
  expect((yield* Effect.promise(() => render(request))).status).toBe(200)
  // A cached token still performs path checks rather than trusting its body.
  unlinkSync(join(ctx.rig.work, "floorplan/index.html"))
  symlinkSync(join(ctx.rig.outside, "index.html"), join(ctx.rig.work, "floorplan/index.html"))
  expect((yield* Effect.promise(() => render(request))).status).toBe(404)
})))


// This supplementary fixture is built from an explicitly supplied upstream
// checkout. It runs only mechanical offline scripts against upstream's own
// temporary shop example. Neither upstream source nor generated HTML is copied
// into this repository. The synthetic runtime-style test always runs above.
const floorplanReference = process.env.FLOORPLAN_REFERENCE_CHECKOUT
it.runIf(floorplanReference)("serves a real Floorplan 0.11.0 map built offline from its own shop fixture", async () => withWeb(ctx => Effect.gen(function* () {
  const helpers = pathToFileURL(join(floorplanReference!, "tests/helpers.mjs")).href
  const generated = execFileSync(process.execPath, ["--input-type=module", "-e", `
    import { readFileSync } from 'node:fs';
    const {shopApp,run,cleanup}=await import(${JSON.stringify(helpers)});
    const root=shopApp();
    try {
      run('scan.mjs',root,'--unattended','--json');
      run('build.mjs',root,'--unattended','--json','--no-claude-md');
      process.stdout.write(readFileSync(root+'/floorplan/index.html'));
    } finally {cleanup(root)}
  `], { encoding: "utf8", stdio: ["ignore", "pipe", "pipe"], maxBuffer: mapByteLimit + 1, timeout: 30_000 })
  const seeded = yield* seedScreens(ctx.rig), port = yield* Effect.promise(freePort)
  writeMap(ctx.rig.work); writeFileSync(join(ctx.rig.work, "floorplan/index.html"), generated)
  yield* setSettings({ ...ctx.settings, display: { publicAddress: "https://map.example", port } } as any)
  const opened = success(yield* call(owner, "project.map.open", { repoId: seeded.repoId }))
  const response = yield* Effect.promise(() => http({ settings: { ...ctx.settings, port }, origin: "null", host: "map.example" }, new URL(opened.frame.url).pathname))
  expect(response.status).toBe(200)
  expect(response.headers["content-security-policy"]).toBe(expectedPolicy(ctx.origin))
  const before = new JSDOM(generated).window.document, after = new JSDOM(response.body).window.document
  const scripts = (document: Document) => [...document.querySelectorAll("script")].map(script => script.textContent!)
  expect(scripts(after).slice(0, -1)).toEqual(scripts(before))
  expect(scripts(after).filter(script => /createElement\(['"]style['"]\)/.test(script)).length).toBeGreaterThanOrEqual(9)
  expect(after.querySelector("style[nonce]")).toBeNull()
  expect(after.querySelectorAll("#lgtm-map-style")).toHaveLength(1)
  const meta = after.querySelector("meta[data-floorplan-policy]")!.getAttribute("content")!
  for (const script of scripts(after)) expect(meta).toContain(`'sha256-${createHash("sha256").update(script).digest("base64")}'`)
  expect(after.querySelector('a.lgtm-floorplan-credit')?.textContent).toBe("Made with Floorplan")
}), { registerSecret: () => {} }), 40_000)
