import { closeSync, constants, fchmodSync, fstatSync, openSync, renameSync, writeSync } from "node:fs";
import { join } from "node:path";

// Issue 1576: the service keeps its own log in the state folder, so a service
// started in the background still leaves a trace of its errors. It receives
// only lines that have already passed the redacting logger and the
// `serviceLog` filter.
//
// The log is `service.log`, readable and writable by the owner only. When a
// line would take it past `limitBytes`, it is renamed to `service.log.1`,
// replacing the older one, and a new `service.log` starts. The two files
// together never hold more than twice the limit. A line longer than the limit
// is dropped. A failure to write the log never stops the service.
export const serviceLogFileName = "service.log";
export const serviceLogLimitBytes = 1024 * 1024;

export const serviceLogFile = (folder: string, limitBytes = serviceLogLimitBytes) => {
  const path = join(folder, serviceLogFileName);
  let descriptor: number | undefined;
  let size = 0;
  const open = () => {
    // No symbolic link is followed, and an existing file is narrowed to the
    // owner. Opening does not block, so a FIFO planted at the path cannot stall
    // the service, and anything but a regular file is refused.
    const opened = openSync(path, constants.O_WRONLY | constants.O_APPEND | constants.O_CREAT | constants.O_NOFOLLOW | constants.O_NONBLOCK, 0o600);
    const stats = fstatSync(opened);
    if (!stats.isFile()) {
      closeSync(opened);
      throw new Error("service log is not a regular file");
    }
    descriptor = opened;
    fchmodSync(descriptor, 0o600);
    size = stats.size;
  };
  const close = () => {
    if (descriptor !== undefined) closeSync(descriptor);
    descriptor = undefined;
  };
  return {
    path,
    write: (line: string): void => {
      try {
        const bytes = Buffer.from(`${line}\n`, "utf8");
        if (bytes.length > limitBytes) return;
        if (descriptor === undefined) open();
        if (size + bytes.length > limitBytes) {
          close();
          renameSync(path, `${path}.1`);
          open();
        }
        size += writeSync(descriptor!, bytes);
      } catch {
        // A full or failing disk must not take the service down with its log.
        try { close(); } catch { /* already closed */ }
      }
    },
    close: (): void => { try { close(); } catch { /* already closed */ } },
  };
};
