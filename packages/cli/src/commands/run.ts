import * as Effect from "effect/Effect"
import * as Stream from "effect/Stream"
import * as RpcSchema from "effect/rpc/RpcSchema"
import { CommandsRpcGroup, type RequestId } from "@lgtm/contracts"
import { randomUUID } from "node:crypto"
import { connect } from "@lgtm/client"
import { waitForAgents } from "@lgtm/client/waitForAgents"
import { exportThread } from "./threadExport.ts"
import { type CommandName } from "./registry.ts"
import { type Parsed } from "./parse.ts"
import { describeFailure, err, formatEvent, out } from "./output.ts"
import { sanitizeJson } from "./sanitize.ts"
import { reportWait } from "./wait.ts"
import { readMockEditorExport } from "./mockEditorFile.ts"

// SIGTERM or SIGINT interrupts a blocking wait; closing its connection
// cancels the wait on the engine (an Interrupt frame, then the socket).
const cancelled = Effect.sync(() => { process.stderr.write("wait cancelled\n") })

// Sends one RPC and prints its answer. Returns the exit code.
export const runCommand = (
  command: CommandName, parsed: Parsed, environment: Readonly<Record<string, string | undefined>>
): Effect.Effect<number> =>
  Effect.scoped(Effect.gen(function* () {
    const payload = command === "project.mocks.editor.save" && parsed.file !== undefined
      ? { ...parsed.payload, content: yield* readMockEditorExport(parsed.file) } : parsed.payload
    const client = yield* connect({ environment })
    if (command === "client.wait") return reportWait(yield* waitForAgents(client, parsed.payload as unknown as Parameters<typeof waitForAgents>[1]).pipe(Effect.onInterrupt(() => cancelled)))
    if (command === "thread.export") {
      yield* exportThread(client, parsed.payload!.agentId as string, parsed.out!)
      return 0
    }
    const call = (client as any)[command] as (payload: unknown) => any
    const rpc = CommandsRpcGroup.requests.get(command as any)
    if (rpc && RpcSchema.isStreamSchema(rpc.successSchema)) {
      yield* Stream.runForEach(call(parsed.payload) as Stream.Stream<any, unknown>, (envelope) => Effect.sync(() => out(
        envelope.event === undefined ? `${sanitizeJson(envelope)}\n` : formatEvent(envelope)
      )))
      return 0
    }
    if (command === "wait") {
      const reply = yield* (call(parsed.payload) as Effect.Effect<any, unknown>).pipe(
        parsed.block ? Effect.onInterrupt(() => cancelled) : (effect) => effect
      )
      return reportWait(reply)
    }
    const reply = yield* (call(payload) as Effect.Effect<any, unknown>)
    if (command === "client.harness.signin.start" && process.stdin.isTTY && process.stdout.isTTY) {
      const sessionId = reply.sessionId
      const send = (tag: string, fields = {}) => Effect.runPromise((client as any)[tag]({ requestId: randomUUID(), sessionId, ...fields })).catch(() => {})
      const keys = (data: Buffer) => { void send("client.harness.signin.write", { text: data.toString("utf8") }) }
      const resize = () => { void send("client.harness.signin.resize", { cols: Math.max(20, Math.min(240, process.stdout.columns || 80)), rows: Math.max(5, Math.min(100, process.stdout.rows || 24)) }) }
      const wasRaw = process.stdin.isRaw
      yield* Effect.addFinalizer(() => Effect.promise(async () => {
        process.stdin.off("data", keys); process.stdout.off("resize", resize)
        process.stdin.setRawMode(wasRaw); process.stdin.pause()
        await send("client.harness.signin.stop")
      }))
      process.stdin.setRawMode(true); process.stdin.on("data", keys); process.stdin.resume()
      process.stdout.on("resize", resize); resize()
      yield* client["client.harness.signin.subscribe"]({ requestId: randomUUID() as RequestId, sessionId }).pipe(Stream.runForEach(item => Effect.sync(() => {
        if (item._tag === "output") out(item.text)
        else out(`\n${item.reason} ${item.check.state === "local-credential" ? "Local sign-in found." : item.check.state === "signed-out" ? "No local sign-in found." : "Local sign-in could not be checked."}\n`)
      })))
      return 0
    }
    out(typeof reply?.data === "string" ? `${reply.data}\n` : `${sanitizeJson(reply, 2)}\n`)
    return 0
  })).pipe(
    // A failure that is not a typed one (an invalid payload, say) is a defect.
    Effect.catchCause((cause) => Effect.sync(() => {
      const failure = cause.reasons.find((reason) => reason._tag === "Fail")
      const defect = cause.reasons.find((reason) => reason._tag === "Die")
      err(`lgtm: ${describeFailure(failure !== undefined ? (failure as { error: unknown }).error : (defect as { defect: unknown } | undefined)?.defect)}\n`)
      return 1
    }))
  )
