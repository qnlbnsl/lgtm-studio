import type * as Effect from "effect/Effect"
import type { DisplayFrame } from "@lgtm/contracts/project/display"
import type { MapSelection } from "@lgtm/contracts/project/map"
import type { CommandRefused, Input } from "../../../commands/internal.ts"
import type { Views } from "../../../views/Views.ts"

type MapRuntime = {
  readonly open: (selection: MapSelection, caller: Pick<Input, "ownerSessionId" | "connectionSignal">) => Effect.Effect<DisplayFrame, CommandRefused>
  readonly release: (id: string) => Effect.Effect<void, CommandRefused>
}
// The command surface reuses the web server's display listener and lifecycle.
const runtimes = new WeakMap<Views["Service"], MapRuntime>()
export const mapDisplayRuntime = (views: Views["Service"]) => runtimes.get(views)
export const installMapDisplayRuntime = (views: Views["Service"], runtime: MapRuntime) => {
  runtimes.set(views, runtime)
  return () => { if (runtimes.get(views) === runtime) runtimes.delete(views) }
}
