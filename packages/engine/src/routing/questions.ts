import { canReceiveAnswer } from "./questionAvailability.ts";
import { requireBuilding } from "../project/approvals.ts";
import { projectView } from "../project/view.ts";
import { approvalQuestion, approvalAnswer } from "../project/approvals.ts";
import { questionLinks, linkedTask, canChangeLinkedTask } from "../project/decisions.ts";
import { senderOf } from "../routing/envelope.ts";
import { randomUUID } from "node:crypto";
import { QuestionId } from "@lgtm/contracts";
import * as Effect from "effect/Effect";
import * as Schema from "effect/Schema";

import { refused, type Input, type Payload, type Plan } from "../commands/internal.ts";
import { envelope, escapeLeadingSlash } from "./envelope.ts";
import { freshDelivery, holderAbove, mayActForHolder, routedDelivery, withdrawQueued } from "./holders.ts";

export const ask = ({ actor, snapshot }: Input, payload: Payload<"ask">): Effect.Effect<Plan, ReturnType<typeof refused>> => Effect.gen(function* () {
  if (actor._tag !== "agent") return yield* refused();
  const links = yield* questionLinks(snapshot, actor.agentId, payload);
  const approval = yield* approvalQuestion(snapshot, actor.agentId, payload);
  const questionId = Schema.decodeUnknownSync(QuestionId)(randomUUID());
  const holder = payload.projectApproval === undefined ? holderAbove(snapshot, actor.agentId) : { _tag: "owner" as const };
  const deliveryId = freshDelivery();
  return { events: [{
    _tag: "QuestionAsked", agentId: actor.agentId, questionId, holder, deliveryId, text: payload.text, ...links,
    ...Object.fromEntries(["title", "recommendation", "proposedAnswer"]
      .flatMap((key) => payload[key as keyof typeof payload] === undefined ? [] : [[key, payload[key as keyof typeof payload]]])),
    ...approval,
  }, ...routedDelivery(actor.agentId, holder, deliveryId, { _tag: "question", id: questionId }, payload.text)] };
});

export const answerQuestion = (
  input: Input, payload: Extract<Payload<"answer">, { kind: "question" }>,
): Effect.Effect<Plan, ReturnType<typeof refused>> => Effect.gen(function* () {
  const { actor, snapshot } = input;
  if (actor._tag !== "owner" && actor._tag !== "agent") return yield* refused();
  const row = snapshot.questions[payload.id];
  if (row === undefined || row.status !== "open" || !mayActForHolder(snapshot, actor, row.holder)) return yield* refused();
  if (!canReceiveAnswer(snapshot, row.agentId)) return yield* refused("The asker cannot receive an answer.");
  const approval = yield* approvalAnswer(input, payload.id, payload.text);
  const task = linkedTask(snapshot, row);
  const mayFinishTask = task !== undefined && canChangeLinkedTask(snapshot, row.agentId, task);
  if (mayFinishTask && task.state === "queue") yield* requireBuilding(input, projectView(snapshot).projects[task.projectId]!);
  const outcome = approval.outcome === undefined ? "" : "\n" + approval.outcome;
  const deliveryId = freshDelivery();
  return { events: [{
    _tag: "QuestionAnswered", agentId: row.agentId, questionId: payload.id, answeredBy: actor, text: payload.text, deliveryId,
    ...(approval.outcome === undefined ? {} : { projectApprovalOutcome: approval.outcome }),
  }, ...approval.events, ...(!mayFinishTask || task.state === "done" ? [] : [{ _tag: "project.TaskChanged" as const,
    projectId: task.projectId, taskId: task.taskId, state: "done" as const, note: "Linked question answered" }]), ...withdrawQueued(snapshot, row.deliveryId), {
    _tag: "DeliveryQueued", delivery: {
      id: deliveryId, target: row.agentId, sender: senderOf(actor), source: { _tag: "question", id: payload.id },
      content: { _tag: "prompt", text: envelope(deliveryId,
        `Answer to question ${payload.id}:\n${actor._tag === "agent" ? escapeLeadingSlash(payload.text) : payload.text}${outcome}`) },
    },
  }] };
});
