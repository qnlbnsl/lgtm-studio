// Issue 1235: the one rule for whether a conversation owes its role
// instructions. The runner applies it as each turn ends (AcpRunner.ts). The
// engine applies it to the log when it loads a session after a restart
// (recovery/reload.ts), because the runner's record does not survive one.
//
// A new conversation owes the instructions. A turn that succeeded while
// carrying them discharges them. A successful compaction owes them again,
// because it drops them from the conversation. Nothing else changes the
// obligation: a turn that failed, was paused, was cancelled or was cut off by
// a restart leaves it as it was, even when the adapter had accepted the turn
// (docs/roles.md, and the ending rows in test/roles/instruction-sessions.test.ts).
export type InstructionStep =
  | { readonly _tag: "conversationOpened" }
  | { readonly _tag: "turnSucceeded"; readonly compact: boolean; readonly carried: boolean };

export const owedAfter = (owed: boolean, step: InstructionStep): boolean => {
  switch (step._tag) {
    case "conversationOpened": return true;
    case "turnSucceeded": return step.compact ? true : step.carried ? false : owed;
  }
};
