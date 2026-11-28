// A repository's own view. A repository has a main checkout on its base branch, and a
// repository agent that works there. The owner talks to that agent on the left, as to a project
// lead, and the views on the right show the main checkout: its command center, its design, its
// files and its Git. The command center lists the projects, so there is no second list of them.
import { useEffect, useRef } from 'preact/hooks';
import { Icon, cx, href } from '../lib';
import { byId, mainOf } from '../data';
import * as act from '../actions';
import { Empty, NotConnected, Tabs, type TabItem } from '../ui';
import { ProjectShell, TalkToggle } from './mission';
import { RepoDesign } from './design';
import { BrowserPage } from './browser';
import { MapTab } from './map';
import { FilesTab } from './files';
import { GitTab } from './git';
import { RepoCenter, RepoWidgets } from './center';
import { TerminalPanel } from './terminal';
import { RepoSetup } from './service';
import { RepoBoard } from './project';
import { RepoScreens, mocksOf } from './mocks';
import type { Id, Route, State } from '../types';

const PAGES: [string, string][] = [['screens', 'Screens'], ['file', 'Design file'], ['browser', 'Browser'], ['architecture', 'Architecture']];
const TITLES: Record<string, string> = { center: 'Command center', board: 'Board', design: 'Design', files: 'Files', git: 'Git' };
const REPO_TABS: TabItem[] = [
  { id: 'center', label: 'Command center', icon: 'layout-dashboard', short: true },
  { id: 'board', label: 'Board' },
  { id: 'design', label: 'Design' },
  { id: 'files', label: 'Files' },
  { id: 'git', label: 'Git' },
];

function isWatchedRepositoryPage(parts: Route['parts'], child: string): boolean {
  switch (child) {
    case 'center':
    case 'board':
    case 'terminal':
      return parts.length <= 3;
    case 'files':
    case 'git':
      return true;
    case 'design':
      switch (parts[3]) {
        case undefined:
        case 'file':
          return true;
        case 'screens':
        case 'browser':
        case 'architecture':
          return parts.length === 4;
        case 'screen':
          return parts.length === 5;
        default:
          // Older Design file links put the category and optional item directly under Design.
          return parts.length <= 5;
      }
    default:
      return false;
  }
}

export function RepoView({ s, repoId, route }: { s: State; repoId: Id; route: Route }) {
  const r = byId(s.repos, repoId);
  const child = route.parts[2] || 'center';
  const terminal = s.server.repositoryMode && child === 'terminal';
  const tab = terminal || REPO_TABS.some(({ id }) => id === child) ? child : 'center';
  const title = terminal ? 'Terminal' : TITLES[tab];
  useEffect(() => { if (!r) return undefined; document.title = `${title} · ${r.name} · LGTM Studio`; return () => { document.title = 'LGTM Studio'; }; }, [title, r?.name]);
  const row = useRef<HTMLElement>(null);
  if (!r) return <div class="view-body"><div class="page"><div class="empty" data-empty><strong>This repository is not known</strong><span><a class="link" href={href('/home')}>Go to Home</a></span></div></div></div>;
  // The Design view of a repository: its design file, its running product, and the map of its code.
  // An older address named a part of the design file straight after "design".
  // Screens is the first page. It lists every drawn screen of the main checkout, where a project
  // lists only the screens it draws or changes.
  const p3 = route.parts[3]; const page = p3 === 'screen' || !p3 ? 'screens' : PAGES.some(([id]) => id === p3) ? p3 : 'file';
  const designNav = (drawn = 0) => <nav class="wf-nav mk-nav" aria-label="Design views" ref={row}>
    {PAGES.map(([id, label]) => <a key={id} class="wf-tab" aria-current={page === id ? 'page' : undefined} href={href(`/r/${r.id}/design${id === 'screens' ? '' : '/' + id}`)}>{label}{id === 'screens' && drawn > 0 && <span class="count">{drawn}</span>}</a>)}
  </nav>;
  const tabLinks = <Tabs tabs={REPO_TABS} current={tab} base={`/r/${r.id}/`} />;

  if (s.server.repositoryMode) {
    if (!r.folderRecords?.length) return <div class="view-body"><NotConnected what="This page" /></div>;
    if (!isWatchedRepositoryPage(route.parts, child)) return <div class="view-body"><div class="page">
      <Empty title="This page does not exist"><a class="link" href={href('/home')}>Go to Home</a></Empty>
    </div></div>;

    const unavailable = <NotConnected what={terminal ? 'The terminal' : tab === 'files' ? 'Files' : 'This page'} />;
    return <div class="workspace"><div class="view">
      <div class="tabrow" role="tablist" aria-label="Repository views">
        {tabLinks}
        {tab === 'center' && <div class="tab-end not-phone"><RepoWidgets s={s} /></div>}
      </div>
      <div class="view-body">
        {tab === 'center' ? <RepoCenter s={s} repo={r} /> : tab === 'design' ? <>{designNav()}{unavailable}</> : unavailable}
      </div>
    </div></div>;
  }

  // A repository the sample only tracks is on no machine yet. Its page sets it up.
  if (!Object.keys(r.folders).length) return <RepoSetup s={s} repo={r} />;
  const place = mainOf(s, r); const agent = r.agent ? byId(s.agents, r.agent) : undefined;
  const old = tab === 'design' && !!p3 && page === 'file' && p3 !== 'file';
  const nav = designNav(mocksOf(s, place).screens.length);
  let body;
  if (tab === 'design' && page === 'screens') body = <div class={cx('wf', p3 === 'screen' && 'mk-full')} data-main={p3 === 'screen' ? undefined : true}>{nav}<RepoScreens s={s} project={place} open={p3 === 'screen' ? route.parts[4] || 'none' : null} /></div>;
  else if (tab === 'design') body = <div class="wf mk-full">
    {!(page === 'architecture' && s.ui.focus) && nav}
    {page === 'browser' ? <BrowserPage s={s} project={place} route={route} />
      : page === 'architecture' ? <MapTab s={s} project={place} />
      : <RepoDesign s={s} repoId={r.id} within={{ base: `/r/${r.id}/design/file`, cat: old ? p3 : route.parts[4], item: old ? route.parts[4] : route.parts[5] }} />}
  </div>;
  else if (tab === 'files') body = <FilesTab s={s} project={place} route={route} />;
  else if (tab === 'git') body = <GitTab s={s} project={place} />;
  else if (tab === 'board') body = <RepoBoard s={s} repo={r} route={route} />;
  else body = <RepoCenter s={s} repo={r} />;
  // The terminal of a repository opens in its main checkout, under the view.
  const term = !!s.prefs.termOpen;
  if (term) body = <div class="with-term"><div class={cx('wt-view', (tab === 'design' || tab === 'files' || tab === 'git' || tab === 'board') && 'fill')}>{body}</div><TerminalPanel s={s} place={place} /></div>;
  const fill = term || tab === 'design' || tab === 'files' || tab === 'git' || tab === 'board';
  const hasList = tab === 'git' || tab === 'files';
  const tabrow = <div class="tabrow" role="tablist" aria-label="Repository views">
    {agent && <TalkToggle s={s} home={false} locked={false} who="Repository agent" short="Agent" />}
    {tabLinks}
    <div class="tab-end not-phone">
      {tab === 'center' && <RepoWidgets s={s} />}
      {hasList && <button type="button" class="icon-btn" aria-pressed={!!s.prefs.listOpen} aria-label={s.prefs.listOpen ? 'Hide the list' : 'Show the list'} title={s.prefs.listOpen ? 'Hide the list' : 'Show the list'} onClick={() => act.setPref('listOpen', !s.prefs.listOpen)}><Icon name="panel-right" size={15} /></button>}
    </div>
  </div>;
  // A repository that has no agent yet shows its views alone.
  if (!agent) return <div class="workspace"><div class="view">{tabrow}<div class={cx('view-body', fill && 'rp-fill')}>{body}</div></div></div>;
  return <div class="workspace">
    <div class="view">
      <ProjectShell s={s} project={place} path={route.path} flush={fill} focus={(tab === 'board' || tab === 'design' && page === 'architecture') && !!s.ui.focus} list={hasList && !!s.prefs.listOpen} tabrow={tabrow}>
        {body}
      </ProjectShell>
    </div>
  </div>;
}
