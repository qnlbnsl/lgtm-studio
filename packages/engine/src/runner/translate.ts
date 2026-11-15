import type { Event } from "@lgtm/contracts";

type Content = Extract<Event, { _tag: "ThreadItem" }>["content"];
export interface Translated {
  readonly kind: "message" | "plan" | "tool-call" | "file-change";
  // Issue 144: the speaker of a message chunk.
  readonly messageId?: string;
  readonly role?: "user" | "agent";
  readonly text: string;
  readonly content: Content;
}
const record = (value: unknown): Record<string, any> | undefined => typeof value === "object" && value !== null ? value as Record<string, any> : undefined;

// Legacy ACP before/after values may be snippets, not complete files.
// Keep their changes without inventing file positions or end-of-file metadata.
export function replacementDiff(before: string, after: string): string {
  if (before === after) return '';
  const lines = (text: string, marker: string) => text === '' ? [] : text.replace(/\n$/, '').split('\n').map((line) => marker + line);
  return [`@@ ${before === '' ? 'New file' : 'Change'} (line numbers unavailable) @@`,
    ...lines(before, '-'), ...lines(after, '+')].join('\n');
}

// Terminal output and exits are correlated and streamed through the redactor
// by terminal.ts before this stateless translator handles the tool report.
// ACP has open unions. Unknown variants are data to ignore; known variants
// are checked again at the event boundary before they reach the writer.
export const translate = (value: unknown, replay: boolean): ReadonlyArray<Translated> => {
  const update = record(value);
  if (update === undefined) return [];
  if (update.sessionUpdate === "agent_message_chunk" || (replay && update.sessionUpdate === "user_message_chunk")) {
    const content = record(update.content);
    return content?.type === "text" && typeof content.text === "string"
      ? [{ kind: "message", ...(typeof update.messageId === "string" ? { messageId: update.messageId } : {}), role: update.sessionUpdate === "user_message_chunk" ? "user" : "agent", text: content.text, content: { _tag: "text", text: content.text } }] : [];
  }
  const entries = update.sessionUpdate === "plan" ? update.entries
    : update.sessionUpdate === "plan_update" && update.plan?.type === "items" ? update.plan.entries : undefined;
  if (Array.isArray(entries)) return [{ kind: "plan", text: JSON.stringify(entries), content: { _tag: "plan", entries } }];
  if (update.sessionUpdate !== "tool_call" && update.sessionUpdate !== "tool_call_update") return [];
  const correlation = typeof update.toolCallId === "string" && update.toolCallId !== "" ? { callId: update.toolCallId } : {};
  const content = Array.isArray(update.content) ? update.content : [];
  const diffs: Translated[] = content.flatMap((part: any) => {
    if (part?.type !== "diff") return [];
    if (typeof part.path === "string") {
      const diff = typeof part.oldText === 'string' || part.oldText === null
        ? replacementDiff(part.oldText ?? '', part.newText ?? '') : part.newText ?? '';
      return [{ kind: "file-change" as const, text: diff, content: { _tag: "file-change" as const, ...correlation, path: part.path, diff } }];
    }
    return (part.changes ?? []).filter((change: any) => typeof change.path === "string").map((change: any) => ({
      kind: "file-change" as const, text: part.patch?.text ?? "", content: { _tag: "file-change" as const, ...correlation, path: change.path, diff: part.patch?.text ?? "" },
    }));
  });
  // A structured edit can contain several fragments for the same path.
  const files = new Map<string, Translated>();
  for (const item of diffs) {
    if (item.content._tag !== 'file-change') continue;
    const previous = files.get(item.content.path);
    if (previous?.content._tag === 'file-change') {
      // Equal fragments can be a replace-all at different positions.
      // The durable event keeps every part; only viewers fold repeated updates.
      const diff = previous.content.diff + '\n' + item.content.diff;
      files.set(item.content.path, { ...item, text: diff, content: { ...item.content, diff } });
    } else files.set(item.content.path, item);
  }
  const changes = [...files.values()];
  if (typeof update.toolCallId !== "string" || update.toolCallId === "") return changes;
  const toolContent = content.filter((part: any) => part?.type !== "terminal" && part?.type !== "diff");
  const hasToolContent = Array.isArray(update.content) && (content.length === 0 || toolContent.length > 0);
  if (changes.length && update.title === undefined && update.status === undefined && update.rawInput === undefined && update.rawOutput === undefined && !hasToolContent) return changes;
  return [...changes, { kind: "tool-call", text: JSON.stringify(update), content: {
    _tag: "tool-call", callId: update.toolCallId, title: typeof update.title === "string" ? update.title : "",
    ...(["pending", "in_progress", "completed", "failed"].includes(update.status) ? { status: update.status } : {}),
    // Preserve tool data in the durable event. Viewers derive readable text
    // with toolText; evidence readers still need every argument and result.
    ...(update.rawInput === undefined ? {} : { input: JSON.stringify(update.rawInput) }),
    // Terminal snapshots are serialized separately by terminal.ts. A reference
    // alone must not overwrite that correlated output with an empty result.
    ...(update.rawOutput !== undefined ? { output: JSON.stringify(update.rawOutput) } : hasToolContent ? { output: JSON.stringify(toolContent) } : {}),
  } }];
};
