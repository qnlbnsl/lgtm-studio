import type { Account } from '@lgtm/contracts';
import * as NodeServices from '@effect/platform-node/NodeServices';
import * as Effect from 'effect/Effect';
import * as Redacted from 'effect/Redacted';
import type { Runtime } from '../commands/internal.ts';
import { buildBaseEnvironment } from '../runner/baseEnvironment.ts';
import { harnessLogin } from '../runner/harnesses.ts';
import { credentialSourceFor } from './credentialSource.ts';
import { resolveCodexHome } from './shadowHome.ts';

// Awaited at startup and inside the account mutation's writer transaction,
// before its events or subsequent agent output can be published. No filesystem
// work here is synchronous. Home resolution shares the total 500 ms budget.
export const preloadAccountCredentials = (
  runtime: Pick<Runtime, 'credentials' | 'home' | 'stateFolder' | 'secrets'>,
  accounts: ReadonlyArray<Account>,
) => Effect.gen(function* () {
  if (runtime.credentials === undefined) return;
  const sources = ['codex', 'claude', 'gemini', 'cursor'].map(harnessId => credentialSourceFor({
    harnessId, home: runtime.home,
    variables: Object.fromEntries(Object.entries(process.env).filter((entry): entry is [string,string] => entry[1] !== undefined)),
  }));
  for (const account of accounts) {
    if (account.enabled === false) continue;
    const variables: Record<string,string> = {};
    for (const [name, value] of Object.entries(account.environment ?? {})) {
      const resolved = value.startsWith('secret:') ? yield* runtime.secrets.resolve(value) : Redacted.make(value);
      if (resolved !== undefined) variables[name] = Redacted.value(resolved);
    }
    const login = harnessLogin(account.harness);
    if (account.home !== undefined && login.homeVariable !== undefined && account.harness !== 'codex') variables[login.homeVariable] = account.home;
    const homed = account.harness === 'codex' ? yield* resolveCodexHome({ home: runtime.home, stateFolder: runtime.stateFolder,
      accountId: account.id, variables, ...(account.home === undefined ? {} : { homePath: account.home }) }).pipe(Effect.option) : undefined;
    if (homed?._tag === 'None') continue;
    const environment = buildBaseEnvironment({parent:{ HOME:runtime.home },accountVariables:homed?._tag === 'Some'?homed.value:variables});
    sources.push(credentialSourceFor({harnessId:account.harness,home:environment.HOME ?? runtime.home,variables:environment}));
  }
  yield* runtime.credentials.preload(sources);
}).pipe(Effect.timeoutOption(500), Effect.asVoid, Effect.provide(NodeServices.layer));
