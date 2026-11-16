import { commandOutcome } from "@lgtm/contracts";
import type { Snapshot } from "../commands/internal.ts";
import { deletedAgent } from "../project/deleted.ts";

// Match prompt admission, including states that retain a queued reply until resume.
// The durable question remains open when its asker cannot currently receive it.
export const canReceiveAnswer = (snapshot: Snapshot, agentId: string): boolean => {
  const agent = snapshot.agents[agentId];
  return agent !== undefined && !deletedAgent(snapshot, agentId) && commandOutcome(agent.state._tag, "prompt") !== "refused";
};

export const offerQuestionAlert = (snapshot: Snapshot, row: { readonly kind: string; readonly agentId?: string; readonly subject?: { readonly _tag: string; readonly id?: string } }): boolean => {
  if (row.kind !== "question") return true;
  const question = row.subject?._tag === "question" ? snapshot.questions[row.subject.id!] : undefined;
  const agentId = question?.agentId ?? row.agentId;
  return agentId !== undefined && canReceiveAnswer(snapshot, agentId);
};
