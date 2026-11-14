import { act } from "react"
import { createRoot, type Root } from "react-dom/client"
import * as Effect from "effect/Effect"
import { afterEach, beforeEach, expect, it, vi } from "vitest"
import type { Project, RosterEntry, Workspace } from "@lgtm/contracts"

// The page talks to a fake client. The editor is a plain text box here; the
// real CodeMirror bundle is exercised in the browser checks.
const fake = vi.hoisted(() => ({ calls: [] as Array<{ name: string; payload: any }>, files: new Map<string, string>(), stale: false }))
vi.mock("@lgtm/client/browser", () => ({ connect: () => Effect.succeed(new Proxy({}, { get: (_target, name: string) => (payload: any) => Effect.suspend((): Effect.Effect<any, any> => {
  fake.calls.push({ name, payload })
  const sha = (text: string) => `v-${text.length}-${[...text].reduce((sum, c) => sum + c.charCodeAt(0), 0)}`
  if (name === "project.files.list") {
    const folder = payload.folder ?? ""
    const items = folder === "" ? [
      { name: "src", path: "src", kind: "folder", size: null, change: "modified" },
      { name: "README.md", path: "README.md", kind: "file", size: 10, change: null }
    ] : [{ name: "app.ts", path: "src/app.ts", kind: "file", size: 20, change: "modified" }]
    return Effect.succeed({ requestId: payload.requestId, result: { folder, items, truncated: false } })
  }
  if (name === "project.files.read") {
    const text = fake.files.get(payload.path)!
    return Effect.succeed({ requestId: payload.requestId, result: { kind: "text", path: payload.path, size: text.length, version: sha(text), text, editable: true } })
  }
  if (name === "project.files.write") {
    if (fake.stale) { fake.stale = false; fake.files.set(payload.path, "changed by an agent\n"); return Effect.fail({ _tag: "CommandError", requestId: payload.requestId, reason: "the file changed since you opened it; reload it and save again" }) }
    fake.files.set(payload.path, payload.text)
    return Effect.succeed({ requestId: payload.requestId, result: { version: sha(payload.text), told: 1 } })
  }
  if (name === "project.commit") return Effect.succeed({ requestId: payload.requestId, result: { head: "abcdef1234567" } })
  return Effect.fail({ _tag: "CommandError", requestId: payload.requestId, reason: "command refused" })
}) })) }))
vi.mock("../src/files/editor.tsx", () => ({
  CodeEditor: (props: { doc: string; path: string; readOnly: boolean; onChange: (text: string) => void; onSave: () => void }) =>
    <textarea aria-label={`Contents of ${props.path}`} defaultValue={props.doc} readOnly={props.readOnly}
      onChange={(event) => props.onChange(event.target.value)} onKeyDown={(event) => { if (event.key === "s" && event.ctrlKey) props.onSave() }} />
}))
const { FilesPage } = await import("../src/files/FilesPage.tsx")
;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const project = { projectId: "project-1", repoId: "repo-1", name: "Product", goal: "", leadAgentId: "lead", worktreeId: "work-1", machineId: "local" } as unknown as Project
const feature = { worktreeId: "work-1", repoId: "repo-1", machineId: "local", branch: "feature", path: "/work/feature", checkout: "worktree" } as unknown as Workspace
const main = { ...feature, worktreeId: "work-0", branch: "main", path: "/work", checkout: "main" } as unknown as Workspace
const lead = { agentId: "lead", name: "Project lead", folder: "/work/feature", state: { _tag: "working" } } as unknown as RosterEntry
let root: Root, host: HTMLDivElement
beforeEach(() => {
  fake.calls.length = 0; fake.stale = false
  fake.files.set("README.md", "# Product\n"); fake.files.set("src/app.ts", "export const app = 1\n")
  HTMLDialogElement.prototype.showModal = function () { this.open = true }
  host = document.createElement("div"); document.body.append(host); root = createRoot(host)
})
afterEach(() => { act(() => root.unmount()); host.remove() })
const flush = () => act(async () => { for (let i = 0; i < 5; i++) await new Promise((resolve) => setTimeout(resolve, 0)) })
const render = async (props: Partial<Parameters<typeof FilesPage>[0]> = {}) => {
  let file = props.file
  const draw = () => root.render(<FilesPage project={project} workspace={feature} file={file} agents={[lead]} gitHref="/#/git"
    mutationDisabled={false} connection="Live" onSessionExpired={() => {}} {...props} onOpen={(path) => { file = path; draw() }} />)
  await act(async () => draw()); await flush()
}
const button = (name: string) => {
  const found = [...host.querySelectorAll<HTMLButtonElement>("button")].find((item) => item.textContent?.trim() === name || item.querySelector(".files-name")?.textContent === name)
  expect(found, name).toBeTruthy(); return found!
}
const type = async (path: string, text: string) => {
  const box = host.querySelector<HTMLTextAreaElement>(`textarea[aria-label="Contents of ${path}"]`)!
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(box, text)
    box.dispatchEvent(new Event("input", { bubbles: true }))
  })
}

it("lists the root, opens a folder on demand and opens a file from the tree", async () => {
  await render()
  expect(fake.calls.filter((call) => call.name === "project.files.list").map((call) => call.payload.folder)).toEqual([undefined])
  expect(host.textContent).toContain("README.md")
  expect(host.textContent).toContain("No file open")
  await act(async () => button("src").click()); await flush()
  expect(fake.calls.at(-1)).toMatchObject({ name: "project.files.list", payload: { worktreeId: "work-1", folder: "src" } })
  await act(async () => button("app.ts").click()); await flush()
  expect(fake.calls.some((call) => call.name === "project.files.read" && call.payload.path === "src/app.ts")).toBe(true)
  expect(host.querySelector<HTMLTextAreaElement>('textarea[aria-label="Contents of src/app.ts"]')?.value).toBe("export const app = 1\n")
  expect(host.textContent).toContain("Project lead is working in this worktree")
})

it("saves an edited file against the version it opened, then commits that one file", async () => {
  await render({ file: "src/app.ts" })
  expect(button("Save").disabled).toBe(true)
  await type("src/app.ts", "export const app = 2\n")
  expect(button("Save").disabled).toBe(false)
  await act(async () => button("Save").click()); await flush()
  const write = fake.calls.find((call) => call.name === "project.files.write")!
  expect(write.payload).toMatchObject({ worktreeId: "work-1", projectId: "project-1", path: "src/app.ts", text: "export const app = 2\n" })
  expect(write.payload.version).toBe(`v-21-${[..."export const app = 1\n"].reduce((sum, c) => sum + c.charCodeAt(0), 0)}`)
  expect(host.textContent).toContain("Saved app.ts. The agent working here was told which file you changed.")
  await act(async () => button("Commit").click()); await flush()
  const message = host.querySelector<HTMLTextAreaElement>("#files-commit-message")!
  await act(async () => { Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(message, "Bump the app"); message.dispatchEvent(new Event("input", { bubbles: true })) })
  await act(async () => [...host.querySelectorAll<HTMLButtonElement>(".files-dialog button")].find((item) => item.textContent === "Commit")!.click()); await flush()
  expect(fake.calls.find((call) => call.name === "project.commit")?.payload).toMatchObject({ worktreeId: "work-1", message: "Bump the app", path: "src/app.ts" })
  expect(host.textContent).toContain("Committed app.ts as abcdef1.")
})

it("keeps the owner's text and shows the current file when the file changed since it was opened", async () => {
  await render({ file: "README.md" })
  await type("README.md", "# Mine\n")
  fake.stale = true
  await act(async () => button("Save").click()); await flush()
  expect(host.textContent).toContain("The file changed since you opened it. Your text is kept.")
  expect(host.querySelector(".files-conflict pre")?.textContent).toBe("changed by an agent\n")
  await act(async () => button("Save my text over it").click()); await flush()
  expect(fake.files.get("README.md")).toBe("# Mine\n")
})

it("shows the base branch read-only and offers no save", async () => {
  await render({ workspace: main, file: "README.md" })
  expect(host.textContent).toContain("Read-only. Agents never edit the base branch directly.")
  expect(host.querySelector<HTMLTextAreaElement>('textarea[aria-label="Contents of README.md"]')?.readOnly).toBe(true)
  expect([...host.querySelectorAll("button")].some((item) => item.textContent === "Save")).toBe(false)
})

it("reads nothing on another machine or while offline", async () => {
  await render({ workspace: { ...feature, machineId: "studio" } as unknown as Workspace })
  expect(host.textContent).toContain("awaiting machine support")
  await render({ connection: "Offline" })
  expect(fake.calls).toHaveLength(0)
})
