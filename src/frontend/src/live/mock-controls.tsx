// Owner screen controls. Only acknowledged reads publish product state.
import { useRef } from 'preact/hooks';
import { store } from '../lib';
import * as engine from '../engine';
import { run } from './board';
import { loadMocks, loadDesign } from './mocks';
import { busy } from './board';
import { toast, confirm } from '../actions';
import type { Loose, Project } from '../types';

/** A screen of a project's mocks, as the live website reads it. */
export interface MockScreenRef { id: string; rev?: string; status?: string }
export function comment(project: Project, screen: MockScreenRef, text: string) {
  return run('mock-comment:' + screen.id, async () => {
    const reply = await engine.request('project.mocks.comment', { projectId: project.id, revisionId: screen.rev, screenId: screen.id, text });
    await loadMocks(project.id);
    toast(reply.notified === false ? 'Comment saved. ' + reply.notificationReason : 'Comment saved', 'info');
    return true;
  });
}
export function approve(project: Project, screen: MockScreenRef) {
  return run('mock-approve:' + screen.id, async () => {
    await engine.request('project.mocks.screen.approve', { projectId: project.id, revisionId: screen.rev, screenId: screen.id, approved: screen.status !== 'approved' });
    await loadMocks(project.id);
    return true;
  }, screen.status === 'approved' ? 'Approval withdrawn' : 'Screen approved');
}
export function markSeen(project: Project, mocks: Loose) {
  return run('mock-seen:' + project.id, async () => {
    try {
      const versions = mocks.changes.filter((c: { seen?: boolean }) => !c.seen).map((c: { to: string; screen: string }) => ({ revisionId: c.to, screenId: c.screen }));
      if (versions.length) await engine.request('project.mocks.seen.batch', { projectId: project.id, versions });
      return true;
    } finally { await loadMocks(project.id); }
  }, 'Changes marked seen');
}

export function saveEditor(project: Project, mocks: Loose, file: File | undefined, confirmed = false): unknown {
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
export function SaveEditor({ project, mocks }: { project: Project; mocks: Loose }) {
  const input = useRef<HTMLInputElement>(null), pending = busy('mock-editor:' + project.id);
  const design = store.state.design?.[project.repo];
  const ready = design?.projectId === project.id && design.revisionId === mocks.designRevision && !!design.worktreeId;
  return <>
    <input ref={input} type="file" accept=".fig,.pen" hidden aria-label="Editor export" onChange={async (event) => {
    const target = event.currentTarget; const file = target.files?.[0]; target.value = ''; if (file) await saveEditor(project, mocks, file);
  }} />
    <button type="button" class="btn ghost" disabled={pending || !ready} title="Export in the editor, then choose that file." onClick={() => input.current?.click()}>
      {pending ? 'Saving…' : 'Save changes from editor'}
    </button>
  </>;
}
