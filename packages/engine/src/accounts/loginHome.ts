import { lstat, mkdir, realpath, rmdir } from 'node:fs/promises'
import { dirname, isAbsolute, join, parse, relative, resolve, sep } from 'node:path'
type Account = { id: string; harness: string; home?: string; environment?: Readonly<Record<string, string>> }
import { harnessLogin } from '../runner/harnesses.ts'
export const folderReasons = {
  unsafe: 'The login folder must be owned by you and have no symbolic links.',
  shared: 'Two accounts cannot share one login folder.',
  unsupported: 'Separate sign-in folders are not supported by this harness.',
  exists: 'The separate login folder already exists.',
  absolute: 'Use an absolute login folder.',
  id: 'The account identifier is invalid.'
} as const
export const defaultAccountHome = (home: string, harness: string) => join(home, ({ claude: '.claude', codex: '.codex', gemini: '.gemini', cursor: '.cursor' } as Record<string, string>)[harness] ?? '.' + harness)
export const accountLoginHome = (home: string, account: Pick<Account, 'harness' | 'home' | 'environment'>) => {
  const variable = harnessLogin(account.harness).homeVariable
  const configured = account.home ?? (variable ? account.environment?.[variable] : undefined)
  if (configured?.startsWith('secret:')) throw new Error(folderReasons.unsafe)
  return configured ? account.harness === 'gemini' ? join(configured, '.gemini') : configured : defaultAccountHome(home, account.harness)
}
// Walk every component without following links. Existing directories must be owned
// by the service user. System ancestors are checked for links, not user ownership.
async function walk(path: string, ownerRoot: string, create: boolean, uid: number, created?: string[]) {
  const root = parse(path).root
  let current = root
  for (const part of relative(root, path).split(sep).filter(Boolean)) {
    current = join(current, part)
    let info
    try { info = await lstat(current) } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== 'ENOENT' || !create) throw new Error(folderReasons.unsafe)
      await mkdir(current, { mode: 0o700 }); created?.push(current)
      info = await lstat(current)
    }
    if (!info.isDirectory() || info.isSymbolicLink() || ((current === ownerRoot || current.startsWith(ownerRoot + sep)) && (info.uid !== uid || (info.mode & 0o022) !== 0))) throw new Error(folderReasons.unsafe)
  }
}
const canonical = async (p: string) => { try { return await realpath(p) } catch { return resolve(p) } }
export const sameAccountHome = async (home: string, a: Account, b: Account) => a.harness === b.harness && await canonical(accountLoginHome(home, a)) === await canonical(accountLoginHome(home, b))
export async function removeCreatedHomes(created: string[]) { for (const path of [...created].reverse()) { try { await rmdir(path) } catch {} } }
export async function assertDistinctHome(home: string, account: Account, accounts: ReadonlyArray<Account>) {
  if (!harnessLogin(account.harness).homeVariable) {
    if (account.harness === 'cursor' && accounts.some(a => a.id !== account.id && a.harness === 'cursor')) throw new Error(folderReasons.unsupported)
    return
  }
  const target = await canonical(accountLoginHome(home, account))
  for (const other of accounts) if (other.id !== account.id && other.harness === account.harness && await canonical(accountLoginHome(home, other)) === target) throw new Error(folderReasons.shared)
}
export async function prepareAccountHome(input: { home: string; harness: string; id: string; choice: 'existing' | 'separate' | 'advanced'; advanced?: string | undefined; accounts: ReadonlyArray<Account>; uid?: () => number; created?: string[] }) {
  if (!/^[a-zA-Z0-9_-]{1,100}$/.test(input.id)) throw new Error(folderReasons.id)
  const variable = harnessLogin(input.harness).homeVariable
  if (input.choice === 'existing') {
    await assertDistinctHome(input.home, { id: input.id, harness: input.harness }, input.accounts)
    return undefined
  }
  if (!variable) throw new Error(folderReasons.unsupported)
  const path = input.choice === 'separate' ? join(input.home, '.lgtm', 'accounts', input.harness, input.id) : input.advanced ?? ''
  if (!isAbsolute(path)) throw new Error(folderReasons.absolute)
  const folder = resolve(path), uid = input.uid?.() ?? process.getuid!()
  const account = { id: input.id, harness: input.harness, home: folder } as Account
  await assertDistinctHome(input.home, account, input.accounts)
  if (input.choice === 'separate') {
    if (!/^(claude|codex|gemini|cursor)$/.test(input.harness)) throw new Error(folderReasons.unsupported)
    try { await lstat(folder); throw new Error(folderReasons.exists) } catch (error) { if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw error }
  }
  if (input.choice === 'advanced') {
    let parent = folder
    try { await lstat(folder) } catch (error) { if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw new Error(folderReasons.unsafe); parent = dirname(folder) }
    let info
    try { info = await lstat(parent) } catch { throw new Error(folderReasons.unsafe) }
    if (!info.isDirectory() || info.isSymbolicLink() || info.uid !== uid || (info.mode & 0o022) !== 0) throw new Error(folderReasons.unsafe)
  }
  await walk(folder, folder.startsWith(resolve(input.home) + sep) ? resolve(input.home) : folder, true, uid, input.created)
  return account.home!
}
