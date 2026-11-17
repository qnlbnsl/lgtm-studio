import type { Account } from '@lgtm/contracts'

// Existing local checks produce evidence without adding an event or storing a
// credential. A changed account row invalidates this in-memory observation.
const checks = new WeakMap<Account, boolean>()
export const recordSignInCheck = (account: Account, state: string): void => {
  if (state === 'local-credential' || state === 'signed-out') checks.set(account, state === 'local-credential')
}
export const signInEvidence = (account: Account & { readonly signedIn?: boolean; readonly status?: string }): boolean | undefined =>
  account.status === 'needs-sign-in' ? false : checks.get(account) ?? account.signedIn
