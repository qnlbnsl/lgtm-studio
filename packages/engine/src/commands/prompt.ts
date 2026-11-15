import { requireAgentBuilding } from "../project/approvals.ts";
import { own, projectView } from "../project/view.ts";
import { randomUUID } from "node:crypto";
import { DeliveryId, describeContext } from "@lgtm/contracts";
import * as Effect from "effect/Effect";
import * as Schema from "effect/Schema";
import { agentText, envelope, senderOf } from "../routing/envelope.ts";
import { resolveContext, type ContextFiles } from "../project/context.ts";
import type { Input, Payload } from "./internal.ts";

export const prompt = (input: Input, payload: Payload<"prompt">, files?: ContextFiles) => Effect.gen(function* () {
  const { actor } = input;
  const target = own(input.snapshot.agents, payload.agentId);
  const view = projectView(input.snapshot), projectId = own(view.agentProjects, payload.agentId);
  if (target?.state._tag === "idle" && projectId !== undefined && own(view.projects, projectId)?.leadAgentId !== payload.agentId) yield* requireAgentBuilding(input, payload.agentId);
  const context = yield* resolveContext(input, payload.context, [{ _tag: "agent", agentId: payload.agentId }], files);
  const id = Schema.decodeUnknownSync(DeliveryId)(randomUUID());
  return { events: [{ _tag: "DeliveryQueued" as const, delivery: {
    id, target: payload.agentId, sender: senderOf(actor), content: { _tag: "prompt" as const,
      text: agentText(senderOf(actor), id, payload.text) + (context === undefined ? "" : `\n\n${envelope(id, describeContext(context))}`),
      ...(context === undefined ? {} : { context }) },
  } }] };
});
