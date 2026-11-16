export function connectionSource(source: string): string {
  const from = '${c.notice || (c.status ==='
  if (source.split(from).length !== 2) throw new Error('Connection notice integration no longer matches')
  return "import { connectionNotice } from './live/connection.js';\n" + source.replace(from, '${connectionNotice(c) || (c.status ===')
}
