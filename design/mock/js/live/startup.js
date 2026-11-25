// The existing connection badge and its menu also explain recovery. The mock
// can supply the same counts without an engine connection.
export const connectionWord = (connection) => connection.starting
  ? `Starting · ${connection.starting.recovered}/${connection.starting.total}`
  : { live: 'Live', reconnecting: 'Reconnecting', offline: 'Offline' }[connection.status];
