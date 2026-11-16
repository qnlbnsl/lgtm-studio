// The live build attaches page lifetime cancellation beneath every screen reader.
export function readRequestsSource(source: string): string {
  const from = 'return current.request(tag, payload, options);'
  if (source.split(from).length !== 2) throw new Error('Read lifecycle integration no longer matches js/engine.js')
  return "import { readOptions } from './live/reads.js';\n" + source.replace(from, 'return current.request(tag, payload, readOptions(options));')
}
