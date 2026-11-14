import { afterEach, expect, it } from '@effect/vitest'
import * as Effect from 'effect/Effect'
import { execFileSync } from 'node:child_process'
import { writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { pathToFileURL } from 'node:url'
import { makeServiceRoot, startService, stopAllServices, waitServing } from '../support/serviceProcess.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
registerRebuildCheck({ noLog: true })
afterEach(stopAllServices)
const hook = (root: string) => {
  const path = join(root, 'no-pty.mjs')
  writeFileSync(path, `import {registerHooks} from 'node:module';registerHooks({resolve(specifier,context,next){if(specifier==='@lydell/node-pty')throw Error('NATIVE_LOAD_PRIVATE_MARKER');return next(specifier,context)}});`)
  return pathToFileURL(path).href
}
it('review 4: engine imports without a native package and start returns a fixed reason', () => {
  const root = makeServiceRoot()
  const module = pathToFileURL(join(process.cwd(), 'packages/engine/src/harnesses/signinSessions.ts')).href
  const server = pathToFileURL(join(process.cwd(), 'packages/engine/src/api/server.ts')).href
  const output = execFileSync(process.execPath, ['--import', hook(root.root), '--input-type=module', '-e', `await import(${JSON.stringify(server)});const {SignInSessions}=await import(${JSON.stringify(module)});try{await new SignInSessions().start({accountId:'a',connectionId:'c',executable:process.execPath,args:[],cwd:${JSON.stringify(root.home)},environment:{},check:async()=>({state:'signed-out'})})}catch(e){console.log(e.message)}`], { encoding: 'utf8', timeout: 15000 })
  expect(output.trim()).toBe('The sign-in program could not run.')
})
it.live('review 4: the real service starts with the native package unavailable', () => Effect.gen(function* () {
  const root = makeServiceRoot(), service = startService(root, { env: { NODE_OPTIONS: '--import=' + hook(root.root) } })
  yield* waitServing(root, service, 15000)
  expect(service.exited()).toBe(false)
}), 20000)
