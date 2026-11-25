// The existing connection badge and its menu also explain recovery. The mock
// can supply the same counts without an engine connection.
import type { Connection } from '../types';

/** While the service starts, the live connection counts the agents it has recovered. */
export type StartingConnection = Connection & { starting?: { recovered: number; total: number } };
export const connectionWord = (connection: StartingConnection): string => connection.starting
  ? `Starting · ${connection.starting.recovered}/${connection.starting.total}`
  : ({ live: 'Live', reconnecting: 'Reconnecting', offline: 'Offline' } as Record<string, string>)[connection.status];
