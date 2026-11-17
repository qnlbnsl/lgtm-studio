import type { Account } from '@lgtm/contracts'

// Remember every check on this row, including inconclusive outcomes. Local
// stores cover only some login methods, so absence is never sign-out evidence.
// Replacing the row invalidates both the observation and the automatic check.
const checks = new WeakMap<Account, boolean>()
export const hasSignInCheck = (account: Account): boolean => checks.has(account)
export const recordSignInCheck = (account: Account, state: string): void => {
  checks.set(account, checks.get(account) === true || state === 'local-credential')
}
export const signInEvidence = (account: Account & { readonly signedIn?: boolean; readonly status?: string }): boolean | undefined =>
  account.status === 'needs-sign-in' ? false : account.signedIn ?? (checks.get(account) === true ? true : undefined)
