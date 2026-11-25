// The actions of a repository, as sample data.
//
// An action is a workflow file of GitHub Actions, which LGTM Studio runs as it is. The Actions
// view lists these files, and the Files view shows their text. The word "workflow" stays with
// the build loop.
export interface ActionStep { name: string; uses?: string; run?: string }
export interface ActionJob { id: string; name: string; steps: ActionStep[] }
export interface ActionFile {
  kind: string; file: string;
  /** When the action runs, in words. */
  on: string;
  /** The same, as the lines under `on:` in the file. */
  trigger: string[];
  /** True when the action counts tests. A build action does not. */
  tests: boolean;
  jobs: ActionJob[];
}

const checkOut: ActionStep = { name: 'Check out', uses: 'actions/checkout@v4' };
const install: ActionStep = { name: 'Install', run: 'pnpm install --frozen-lockfile' };
const report: ActionStep = { name: 'Write the report', run: 'pnpm test:report' };
const fullJobs: ActionJob[] = [
  { id: 'set-up', name: 'Set up', steps: [checkOut, install] },
  { id: 'tests', name: 'Tests', steps: [{ name: 'Run the full suite', run: 'pnpm check' }, report] },
  { id: 'user-flows', name: 'User flows', steps: [{ name: 'Start the product', run: './bin/lgtm serve --detach' }, { name: 'Run the user flows', run: 'pnpm test:browser' }] },
];
export const ACTIONS: ActionFile[] = [
  { kind: 'Full run', file: 'full-run.yml', on: 'Before a merge', trigger: ['pull_request:', '  branches: [master]'], tests: true, jobs: fullJobs },
  { kind: 'Check', file: 'check.yml', on: 'On a commit', trigger: ['push:', "  branches-ignore: [master]"], tests: true,
    jobs: [{ id: 'check', name: 'Check', steps: [checkOut, install, { name: 'Run the related tests', run: 'pnpm test --related' }, report] }] },
  { kind: 'Nightly', file: 'nightly.yml', on: 'Every night', trigger: ['schedule:', "  - cron: '0 2 * * *'"], tests: true, jobs: fullJobs },
  { kind: 'Build', file: 'build.yml', on: 'After a merge', trigger: ['push:', '  branches: [master]'], tests: false,
    jobs: [{ id: 'build', name: 'Build', steps: [checkOut, install, { name: 'Build the website', run: 'pnpm build' }, { name: 'Keep the build', uses: 'actions/upload-artifact@v4' }] }] },
];
export const actionPath = (a: ActionFile) => `.github/workflows/${a.file}`;

/** The text of an action's file. */
export function actionText(a: ActionFile): string {
  const lines = [`name: ${a.kind}`, '', 'on:', ...a.trigger.map((l) => '  ' + l), '', 'jobs:'];
  a.jobs.forEach((job, i) => {
    lines.push(`  ${job.id}:`, `    name: ${job.name}`, '    runs-on: self-hosted');
    if (i > 0) lines.push(`    needs: ${a.jobs[i - 1]!.id}`);
    lines.push('    steps:');
    for (const step of job.steps) lines.push(`      - name: ${step.name}`, step.uses ? `        uses: ${step.uses}` : `        run: ${step.run}`);
    lines.push('');
  });
  return lines.join('\n');
}
