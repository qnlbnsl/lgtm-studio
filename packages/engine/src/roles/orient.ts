import { parseRole } from "@lgtm/contracts";
import * as Effect from "effect/Effect";
import { agentReadReply } from "../client/reads/index.ts";
import { roster } from "../client/reads/roster.ts";
import { refused, type Input } from "../commands/internal.ts";
import { composeInstructions } from "./compose.ts";
import { instructionLimits } from "./instructions.ts";
export const unavailableAdvice = "Role instructions are unavailable. Ask your lead for the current brief.";

export const orient = (input: Input, payload: Record<string, unknown>) => {
  if (input.actor._tag !== "agent") return Effect.fail(refused());
  const agentId = input.actor.agentId;
  const agents = roster(input.actor, input.snapshot);
  const self = agents.find((row) => row.agentId === agentId);
  if (self === undefined) return Effect.fail(refused());
  const role = parseRole(self.role);
  return Effect.gen(function* () {
    const advice = yield* composeInstructions(input.runtime, input.snapshot, self.role, undefined, self.folder).pipe(Effect.catchTag("CommandRefused", () => Effect.succeed(unavailableAdvice)));
    const makeReply = (advice: string) => agentReadReply(payload.requestId, {
      agentId: self.agentId, name: self.name, kind: role.kind,
      ...(role.name === undefined ? {} : { roleName: role.name }), parent: self.parent,
      ...(self.parentName === undefined ? {} : { parentName: self.parentName }),
      openPermissionRequests: self.openPermissionRequests, openQuestionsAsked: self.openQuestionsAsked,
      agents, advice,
    });
    let reply = makeReply(advice);
    if (Buffer.byteLength(JSON.stringify(reply)) > instructionLimits.replyBytes) reply = makeReply(unavailableAdvice);
    if (Buffer.byteLength(JSON.stringify(reply)) > instructionLimits.replyBytes) return yield* refused("Role orientation exceeds 256 KiB.");
    return { reply };
  });
};
