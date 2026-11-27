// The harnesses LGTM Studio can add to a machine, and the dialog that installs one.
// The list was read on 2026-11-24 from the public registry of the Agent Client Protocol (ACP),
// https://cdn.agentclientprotocol.com/registry/v1/latest/registry.json. Each entry is in the
// registry and is named by at least two of the registry's agent list, T3 Code and Herdr.
// Claude Code and Codex are not here, because the engine ships them.
import { Icon, update, live, store } from '../lib';
import { machineOf } from '../data';
import * as act from '../actions';
import { Dialog, registerDialog, Menu, CodeBox } from '../ui';
import type { State, DialogState } from '../types';

export interface Supported {
  name: string; version: string; licence: string; site: string;
  /** An npm package, or an archive the publisher serves. */
  from: 'npm' | 'archive'; source: string; sha256?: string;
  /** The command the engine starts for the Agent Client Protocol. */
  run: string;
}
export const SUPPORTED: Record<string, Supported> = {
  gemini: { name: 'Gemini CLI', version: '0.71.1', licence: 'Apache-2.0', site: 'https://geminicli.com', from: 'npm', source: '@google/gemini-cli@0.71.1', run: 'gemini --acp' },
  cursor: { name: 'Cursor', version: '2026.11.22', licence: 'Proprietary', site: 'https://cursor.com/docs/cli/acp', from: 'archive', source: 'https://downloads.cursor.com/lab/2026.11.22-e9042b2/linux/x64/agent-cli-package.tar.gz', run: 'cursor-agent acp' },
  opencode: { name: 'OpenCode', version: '2.4.1', licence: 'MIT', site: 'https://opencode.ai', from: 'archive', source: 'https://github.com/anomalyco/opencode/releases/download/v2.4.1/opencode-linux-x64.tar.gz', sha256: '9ddd18e3faf64eaeccf6a0fcbc96a5c7a14520ae5fd5b61e5be51d894f478b47', run: 'opencode acp' },
  copilot: { name: 'GitHub Copilot', version: '1.0.115', licence: 'Proprietary', site: 'https://github.com/features/copilot/cli/', from: 'npm', source: '@github/copilot@1.0.115', run: 'copilot --acp' },
  pi: { name: 'Pi', version: '0.0.35', licence: 'MIT', site: 'https://github.com/svkozak/pi-acp', from: 'npm', source: 'pi-acp@0.0.35', run: 'pi-acp' },
  kiro: { name: 'Kiro CLI', version: '3.0.0', licence: 'Proprietary', site: 'https://kiro.dev/docs/cli/acp/', from: 'archive', source: 'https://prod.download.cli.kiro.dev/stable/3.0.0/kirocli-x86_64-linux.tar.gz', sha256: '2ebb8688b84765e4baf46d6e28b3e79780e7724e198591c22d0528d03a88a282', run: 'kiro-cli acp --agent-engine=v3' },
  grok: { name: 'Grok Build', version: '1.0.93', licence: 'Proprietary', site: 'https://x.ai/cli', from: 'npm', source: '@xai-official/grok@1.0.93', run: 'grok agent stdio' },
  antigravity: { name: 'Google Antigravity', version: '1.5.2', licence: 'Proprietary', site: 'https://antigravity.google/docs/ide/extensions', from: 'archive', source: 'https://dl.google.com/agy-extensions/releases/linux/agy-acp-server-1.5.2-linux-x86_64.zip', sha256: '99dd8612450805011619d7b8447ce374c289e00549d8557f36790e1dd6256852', run: 'agy_acp_server.par' },
  amp: { name: 'Amp', version: '0.13.0', licence: 'Apache-2.0', site: 'https://github.com/tao12345666333/amp-acp', from: 'archive', source: 'https://github.com/tao12345666333/amp-acp/releases/download/v0.13.0/amp-acp-linux-x86_64.tar.gz', sha256: '205c819383b6fd389b408120ebea9cdde5c2957530b5ce4ebc912998d4118f8b', run: 'amp-acp' },
};

/** The supported harnesses that are not on the machine. */
export const addable = (s: State): [string, Supported][] =>
  Object.entries(SUPPORTED).filter(([id]) => !s.harnessCatalog[id]?.installed && s.ui.installing !== id);

/** The commands the server runs to install a harness, as the dialog shows them. */
export function installCommands(id: string, x: Supported): string[] {
  const dir = `~/.local/share/lgtm/harnesses/${id}`;
  if (x.from === 'npm') return [`npm install --prefix ${dir} --ignore-scripts ${x.source}`];
  const file = x.source.split('/').pop()!;
  return [
    `curl --fail --location --output ${file} ${x.source}`,
    ...(x.sha256 ? [`echo "${x.sha256}  ${file}" | sha256sum --check`] : []),
    file.endsWith('.zip') ? `unzip ${file} -d ${dir}` : `tar --extract --file ${file} --directory ${dir}`,
  ];
}

function install(id: string) {
  if (store.state.server.repositoryMode) return;
  const x = SUPPORTED[id]!;
  update((st) => { st.ui.installing = id; });
  act.sampleLater(() => update((st) => {
    const base = st.harnessCatalog[id] || { name: x.name, models: [], efforts: [], noEffort: [], runMode: 'ACP' };
    st.harnessCatalog[id] = { ...base, installed: true, version: x.version };
    const m = st.machines.find((y) => y.role === 'master') || st.machines[0]; if (m) { m.harnesses ||= {}; m.harnesses[id] = x.version; }
    st.ui.installing = null;
  }), 2200);
}

/** "Add harness": the supported harnesses that are not on the machine. Choosing one asks first. */
export function AddHarness({ s }: { s: State }) {
  const list = addable(s);
  if (live.on || s.server.repositoryMode || !list.length) return null;
  return <Menu class="btn" label="Add harness" width={240} disabled={!!s.ui.installing} trigger={<><Icon name="plus" size={14} /><span>Add harness</span></>}
    items={[{ heading: 'Supported harnesses' }, ...list.map(([id, x]) => ({ label: x.name, onSelect: () => act.openDialog({ type: 'install-harness', id }) }))]} />;
}

function InstallHarness({ s, d }: { s: State; d: DialogState }) {
  const id: string = d.id; const x = SUPPORTED[id]!;
  const m = s.machines.find((y) => y.role === 'master') || machineOf(s, 'studio');
  return <Dialog title={'Install ' + x.name} onClose={act.closeDialog} onSubmit={() => { act.closeDialog(); install(id); }} foot={<>
      <button type="button" class="btn" onClick={act.closeDialog}>Cancel</button>
      <button type="submit" class="btn primary">Install on {m?.name}</button>
    </>}>
    <dl class="dl hc-facts">
      <dt>Version</dt><dd class="num">{x.version}</dd>
      <dt>Publisher's page</dt><dd><a class="link" href={x.site} target="_blank" rel="noreferrer">{x.site.replace(/^https:\/\//, '')}</a></dd>
      <dt>Licence</dt><dd>{x.licence}</dd>
      <dt>Checksum</dt><dd>{x.from === 'npm' ? 'Checked by npm' : x.sha256 ? 'SHA-256 published' : <span class="warn-text">Not published</span>}</dd>
      <dt>Starts as</dt><dd class="mono">{x.run}</dd>
    </dl>
    <CodeBox text={installCommands(id, x).join('\n')} label="install commands" />
  </Dialog>;
}
registerDialog('install-harness', InstallHarness);
