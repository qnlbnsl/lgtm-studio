// This sink receives already-redacted structured lines. Outside the gated
// protocol test mode it also removes arbitrary copied-library causes,
// annotations and method names, preserving only fixed diagnostic categories.
const messages = new Set([
  "piece table failed",
  "web server disabled: web password is unset or shorter than eight characters",
  "web server status could not be recorded",
  "debug RPC group enabled", "request failed: defect", "adapter frame dropped: malformed",
  "adapter request refused: unsupported", "permission answer delivery failed",
  "adapter content dropped: no active turn", "adapter update dropped: malformed",
  "runner event dropped: malformed", "runner event dropped: processing failed",
  "mode set-back failed", "account return failed", "account timer release failed",
  "account timer check failed", "leftover process group did not exit",
  "leftover process sweep failed", "unreferenced secret collection failed",
  "delivery dispatch failed", "blocking wait cancellation failed",
  "permission request id collision", "ACP notification handler failed",
  "Dropped an ACP notification that failed to decode",
  "event log write failed", "event log writes recovered", "piece reactor paused: database write failed",
  "piece reactor failed", "runner event held: database write failed", "runner events dropped: writes failed too long",
  "runner event dropped: write refused",
]);

// Fix round 2, F-2: the first-start warning names the harness adapters that
// were not found. Only the four default ids, each once and in the defaults'
// order, are allowed, so the line holds no text the sink has not fixed.
const missingAdapters = /^harness adapters not found: (?:claude|codex|gemini|cursor)(?:, (?:claude|codex|gemini|cursor))*$/;
const fixedMessage = (message: unknown): boolean => {
  if (typeof message !== "string") return false;
  if (messages.has(message)) return true;
  if (!missingAdapters.test(message)) return false;
  const ids = message.slice("harness adapters not found: ".length).split(", ");
  const order = ["claude", "codex", "gemini", "cursor"];
  return new Set(ids).size === ids.length && ids.every((id, index) => index === 0 || order.indexOf(id) > order.indexOf(ids[index - 1]!));
};

const pieceNamespaces = new Set(["client", "board", "roles", "project"]);
const pieceTables = new Set(["views", "snapshotSections", "pieceVisibility", "reach", "holdUnsettled", "requestHeld", "sourceRules", "secretKeep", "alertSources", "openMatters"]);
const errorClasses = new Set(["TypeError", "RangeError", "SyntaxError", "ReferenceError", "Error", "Unknown"]);
// Issue 1610: the supervisor's reactor lines keep the reactor's namespace,
// which is one of the fixed piece names.
const reactorAnnotations = (message: unknown, value: unknown) => {
  if ((message !== "piece reactor failed" && message !== "piece reactor paused: database write failed")
    || value === null || typeof value !== "object") return {};
  const { namespace } = value as Record<string, unknown>;
  return typeof namespace === "string" && pieceNamespaces.has(namespace) ? { annotations: { namespace } } : {};
};
const pieceAnnotations = (message: unknown, value: unknown) => {
  if (message !== "piece table failed" || value === null || typeof value !== "object") return {};
  const { namespace, table, errorClass } = value as Record<string, unknown>;
  if (typeof namespace !== "string" || !pieceNamespaces.has(namespace)
    || typeof table !== "string" || !pieceTables.has(table)
    || typeof errorClass !== "string" || !errorClasses.has(errorClass)) return {};
  return { annotations: { namespace, table, errorClass } };
};

// Issue 1575: a failed write is logged with its fixed reason, the count of
// failures in a row and SQLite's numeric result code. A reason outside this
// list is logged as "other", so the line never carries free text.
const writeReasons = new Set([
  "database write failed", "database write failed: busy", "database write failed: disk full",
  "database write failed: constraint", "database read failed", "event insert returned no sequence",
  "receipt insert returned no result", "invalid piece side rows", "invalid piece side row",
  "writer transaction is closed", "stored event is unreadable", "stored harness settings are unreadable",
  "secret store write failed", "live views are ahead of the log", "live views missed a committed batch", "database write failed: rollback failed", "database write failed: transaction left open",
  "agent events were dropped while writes failed",
]);
const count = (value: unknown) => typeof value === "number" && Number.isSafeInteger(value) && value >= 0 ? value : undefined;
const writeAnnotations = (message: unknown, value: unknown) => {
  if ((message !== "event log write failed" && message !== "event log writes recovered" && message !== "runner event dropped: write refused")
    || value === null || typeof value !== "object") return {};
  const { reason, failures, sqliteCode } = value as Record<string, unknown>;
  const annotations: Record<string, unknown> = {};
  if (message !== "event log writes recovered") annotations.reason = typeof reason === "string" && writeReasons.has(reason) ? reason : "other";
  if (count(failures) !== undefined) annotations.failures = failures;
  if (count(sqliteCode) !== undefined) annotations.sqliteCode = sqliteCode;
  return { annotations };
};

export const serviceLog = (protocol: boolean, write: (line: string) => void) => (line: string): void => {
  if (protocol) { write(line); return; }
  try {
    const entry = JSON.parse(line) as { timestamp: string; level: string; message: unknown; annotations?: unknown };
    write(JSON.stringify({
      timestamp: entry.timestamp, level: entry.level,
      message: fixedMessage(entry.message) ? entry.message : "service diagnostic",
      ...pieceAnnotations(entry.message, entry.annotations),
      ...writeAnnotations(entry.message, entry.annotations),
      ...reactorAnnotations(entry.message, entry.annotations),
    }));
  } catch { write('{"message":"service diagnostic"}'); }
};
