// The Map view: the codebase map of the project, built by an agent with Floorplan.
// The page is an agent's output. It is read from a short-lived address and shown in a sandboxed
// frame, so it can never read the owner's session.
import { useEffect, useState } from 'preact/hooks';
import { Icon, ago, store, update, useKey, useNow } from '../lib';
import * as act from '../actions';
import * as server from '../server/sync';
import type { Project, State } from '../types';

export function MapTab({ s, project }: { s: State; project: Project }) {
  // A map is a wide drawing, so the view can take the whole window, as the board can.
  const focus = !!s.ui.focus;
  useKey((e) => { if (e.key === 'Escape' && store.state.ui.focus && !store.state.dialog) update((st) => { st.ui.focus = false; }); }, []);
  // The map is kept with its project's id, so a change of project never shows or opens the last project's map.
  const [held, setHeld] = useState<{ project: string; map: server.MapView } | null>(null);
  const map = held?.project === project.id ? held.map : null;
  const [error, setError] = useState('');
  const [frame, setFrame] = useState<server.MapFrame | null>(null);
  const now = useNow();
  useEffect(() => { setError(''); return server.watchMap(project.id, (m) => { setHeld({ project: project.id, map: m }); setError(''); }, setError); }, [project.id]);

  // A page is opened once for each version of the map. The address is given back when the view closes.
  const version = map?.state === 'ready' ? map.modifiedAt : 0;
  useEffect(() => {
    if (!version) { setFrame(null); return undefined; }
    let closed = false; let opened: server.MapFrame | null = null;
    server.openMap(project.id).then((f) => { if (closed) void server.releaseMap(f.id); else { opened = f; setFrame(f); } }, (e) => { if (!closed) setError(server.reason(e)); });
    return () => { closed = true; if (opened) void server.releaseMap(opened.id); };
  }, [project.id, version]);

  const build = () => { void server.buildMap(project.id).catch((e) => act.toast(server.reason(e), 'error')); };
  if (error && !map) return <div class="map"><div class="map-none"><Icon name="triangle-alert" size={20} class="error-text" /><strong>{error}</strong></div></div>;
  if (!map) return <div class="map" aria-busy="true" />;

  if (map.state === 'none') {
    return <div class="map">
      <div class="map-none" data-empty>
        <Icon name="network" size={22} class="faint" />
        <strong>{map.building ? 'Drawing the architecture' : 'No architecture diagram'}</strong>
        {map.building
          ? <span class="map-busy" role="status"><Icon name="loader-circle" size={14} class="auth-spin" />An agent is reading the code</span>
          : <button type="button" class="btn primary" onClick={build}>Draw the architecture</button>}
      </div>
    </div>;
  }

  // The map's own page counts its files and lines, so this line says only how old the map is.
  const built = map.facts?.generatedAt || map.modifiedAt;
  return <div class="map">
    <div class="map-bar">
      <span class="map-facts">{'Built ' + (ago(built, now) === 'now' ? 'just now' : ago(built, now) + ' ago')}</span>
      {error && <span class="error-text truncate" role="alert">{error}</span>}
      <span class="grow" />
      <button type="button" class="btn sm" disabled={map.building} onClick={build}>
        {map.building ? <><Icon name="loader-circle" size={14} class="auth-spin" />Updating</> : <><Icon name="refresh-cw" size={14} />Update</>}
      </button>
      <button type="button" class="btn icon" aria-pressed={focus} aria-label={focus ? 'Leave focus' : 'Focus on the diagram'} data-tip={focus ? 'Leave focus · Esc' : 'Focus'} data-tip-side="left" onClick={() => update((st) => { st.ui.focus = !st.ui.focus; })}>
        <Icon name={focus ? 'minimize-2' : 'maximize-2'} size={14} />
      </button>
      {frame && !frame.url.startsWith('data:') && <a class="icon-btn" href={frame.url} target="_blank" rel="noopener noreferrer" aria-label="Open the diagram in a new tab" data-tip="Open in a new tab" data-tip-side="left">
        <Icon name="external-link" size={15} />
      </a>}
    </div>
    {frame && <iframe key={frame.id} class="map-frame" src={frame.url} title={'Architecture of ' + project.name} sandbox="allow-scripts allow-popups" referrerpolicy="no-referrer" />}
  </div>;
}
