// Owner screen controls. Only acknowledged reads publish product state.
import { html, useRef, store } from '../lib.js';
import * as engine from '../engine.js';
import { run } from './board.js';
import { loadMocks, loadDesign } from './mocks.js';
import { busy } from './board.js';
import { toast, confirm } from '../actions.js';
export function comment(project, screen, text) {
  return run('mock-comment:' + screen.id, async () => {
    const reply = await engine.request('project.mocks.comment', { projectId: project.id, revisionId: screen.rev, screenId: screen.id, text });
    await loadMocks(project.id);
    toast(reply.notified === false ? 'Comment saved. ' + reply.notificationReason : 'Comment saved', 'info');
    return true;
  });
}
export function approve(project, screen) {
  return run('mock-approve:' + screen.id, async () => {
    await engine.request('project.mocks.screen.approve', { projectId: project.id, revisionId: screen.rev, screenId: screen.id, approved: screen.status !== 'approved' });
    await loadMocks(project.id);
    return true;
  }, screen.status === 'approved' ? 'Approval withdrawn' : 'Screen approved');
}
export function markSeen(project, mocks) {
  return run('mock-seen:' + project.id, async () => {
    try {
      const versions = mocks.changes.filter((c) => !c.seen).map((c) => ({ revisionId: c.to, screenId: c.screen }));
      if (versions.length) await engine.request('project.mocks.seen.batch', { projectId: project.id, versions });
      return true;
    } finally { await loadMocks(project.id); }
  }, 'Changes marked seen');
}

export function saveEditor(project, mocks, file, confirmed = false) {
  if (busy('mock-editor:' + project.id)) return;
  if (!confirmed) return confirm({ title: 'Save changes from editor?', action: 'Save changes', danger: true,
    body: 'The project returns to Design and its overall approval is withdrawn. Save this editor export?',
    onConfirm: () => saveEditor(project, mocks, file, true) });
  return run('mock-editor:' + project.id, async () => {
    if (!file || file.size < 1 || file.size > 512 * 1024) throw new Error('Choose an editor export of at most 512 KiB.');
    const design = store.state.design?.[project.repo];
    if (!design || design.revisionId !== mocks.designRevision || design.projectId !== project.id || !design.worktreeId) throw new Error('The recorded mock design is unavailable.');
    const bytes = new Uint8Array(await file.arrayBuffer());
    let binary = '';
    for (let i = 0; i < bytes.length; i += 8192) binary += String.fromCharCode(...bytes.subarray(i, i + 8192));
    const saved = await engine.request('project.mocks.editor.save', { projectId: project.id, revisionId: mocks.revisionId,
      designRevision: mocks.designRevision, worktreeId: design.worktreeId, confirmed: true, content: btoa(binary) });
    // The saved notice does not wait for the two reads that follow (issue 936).
    void loadMocks(project.id).then(() => loadDesign(project.repo)).catch(() => {});
    return saved;
  }, 'Editor changes saved');
}

// OpenPencil's existing Save exports a file. The owner selects that export;
// its name is ignored and the engine alone resolves the registered destination.
export function SaveEditor({ project, mocks }) {
  const input = useRef(null), pending = busy('mock-editor:' + project.id);
  const design = store.state.design?.[project.repo];
  const ready = design?.projectId === project.id && design.revisionId === mocks.designRevision && !!design.worktreeId;
  return html`<input ref=${input} type="file" accept=".fig,.pen" hidden aria-label="Editor export" onChange=${async (event) => {
    const file = event.target.files?.[0]; event.target.value = ''; if (file) await saveEditor(project, mocks, file);
  }} /><button type="button" class="btn ghost" disabled=${pending || !ready} title="Export in the editor, then choose that file." onClick=${() => input.current?.click()}>${pending ? 'Saving…' : 'Save changes from editor'}</button>`;
}
