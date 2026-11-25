// Starts the mock in this folder's own page. A page that loads the mock as a package
// imports mount() from app and calls it itself.
import './css/tokens.css';
import './css/base.css';
import './css/screens.css';
import './css/mission.css';
import './css/usage.css';
import './css/git.css';
import './css/files.css';
import './css/map.css';
import './css/terminal.css';
import './css/mocks.css';
import './css/browser.css';
import './css/design.css';
import './css/messages.css';
import './css/workflow.css';
import './css/workflows.css';
import './css/service.css';
import './css/auth.css';
import './css/center.css';

if (import.meta.env.DEV) await import('preact/debug');
const { mount } = await import('./app');

declare global { interface Window { lgtm: ReturnType<typeof mount> } }
window.lgtm = mount(document.getElementById('app')!); // for poking at the mock from the console
