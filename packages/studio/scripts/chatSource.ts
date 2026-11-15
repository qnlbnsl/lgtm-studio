// The standalone sample does not connect to the engine. Guard the live chat binding.
export function chatSource(path: string, source: string): string {
  if (path !== 'js/ui.js') return source
  const anchor = 'useEffect(() => chatLive.watchThread(agentId), [agentId]);'
  if (source.split(anchor).length !== 2) throw new Error('The live chat thread binding moved')
  return source.replace(anchor, 'useEffect(() => liveMode.on ? chatLive.watchThread(agentId) : undefined, [agentId]);')
}
