// The mock's data model in one place: the records the screens read, and the store's state.
// The sample data is a design aid, not a contract for the engine. The engine's own types live
// in packages/contracts. Fields that only some records carry are optional.
import type { ComponentChildren } from 'preact';
import type { UsageState } from './screens/usage';
import type { ProjectWorkflow } from './screens/workflow';
import type { TerminalState } from './screens/terminal';
import type { Mocks } from './screens/mocks';
import type { RepoDesignRecord } from './screens/design';
import type { Worktree, GitUi } from './screens/git';

export type Id = string;
/** A time in milliseconds since 1970, as Date.now() returns it. */
export type Ms = number;

// ---- the service and its machines -------------------------------------------------
export interface Machine {
  id: Id; name: string; role: 'master' | 'worker' | string; status: 'online' | 'offline' | 'pairing' | string;
  here?: boolean; slots: { used: number; limit: number }; lastSeen: Ms; address: string; workspace: string; os?: string;
  connectedFor?: number; cpu?: number; mem?: number; cores?: number; ram?: number; version?: string;
  /** The harness versions found on the machine: null for one that is missing, and null while it is scanned. */
  harnesses?: Record<string, string | null> | null; [more: string]: unknown;
}
export interface UsageWindow { pct: number; resetsAt: Ms }
export interface Subscription {
  id: Id; harness: HarnessId; name: string; machine: Id; enabled: boolean; signedIn?: boolean | null; signInKnown?: boolean;
  useForNew?: boolean; usage: UsageWindow | null; weekly?: number; account?: string | null; window?: UsageWindow | null; model?: string;
  /** The harness's home folder for this account, when the live service reports it. */
  home?: string;
  [more: string]: unknown;
}
export type HarnessId = 'claude' | 'codex' | string;
export interface Harness {
  name: string; models: string[]; efforts: string[]; noEffort: string[]; runMode: string;
  /** The models and efforts the owner allowed, when the harness settings limit them. */
  configuredModels?: string[]; configuredEfforts?: string[];
  // What the Harnesses page shows: the sample's seed or the live service fills these.
  installed?: boolean; version?: string | null; path?: string; defaultModel?: string; defaultEffort?: string; recipe?: HarnessRecipe;
  installationReason?: string; autoPermissionReason?: string; reason?: string; effortReason?: string; signInCommand?: string;
  modelLabels?: Record<string, string>; effortLabels?: Record<string, string>;
  /** Live only: the engine's settings record for the harness. */
  settings?: { id: string; [field: string]: unknown };
}
/** How a harness is installed and signed in to. */
export interface HarnessRecipe {
  automatic: boolean; packages: string[]; commands: string[]; source?: string; signInCommand?: string; signInSource?: string; reason?: string;
}

// ---- repositories, projects and teams ---------------------------------------------
export interface Repo {
  id: Id; name: string; main: string; remote?: string | null; folders: Record<string, string>; design?: boolean;
  /** The letters that start every identifier of the repository's sprints and tasks. */
  prefix?: string;
  description?: string; describedAt?: Ms; describedBy?: Id;
  /** The repository agent, which works in the main checkout. */
  agent?: Id;
  /** What the owner wants from the repository, as the repository agent keeps it. */
  plan?: RepoGoal[];
  [more: string]: unknown;
}
/** One line of a repository's plan. `project` is the project that carries it out. */
export interface RepoGoal { id: Id; title: string; state: 'done' | 'project' | 'open'; project?: Id; note?: string }
export type Stage = 'Define' | 'Design' | 'Plan' | 'Build' | 'Review' | 'Hand over' | string;
export interface Project {
  id: Id; repo: Id; name: string; prefix: string; goal: string; stage: Stage; phase: string; branch: string; folder: string;
  machine: Id; lead: Id; shell?: string; createdAt: Ms; hasMocks?: boolean; tests?: string; agentReview?: string;
  archived?: boolean; archiving?: boolean; naming?: boolean; base?: string; fresh?: boolean; merged?: boolean;
  /** Project settings: the lead's permissions, the approved model assignment, and the credit setting. */
  permissions?: Record<string, { setting?: string; granted?: boolean }>; assignment?: ProjectAssignment; removeCredits?: boolean;
  [more: string]: unknown;
}
/** Which harness, model and effort each kind of work uses in a project. */
export interface ProjectAssignment { state: string; rows: Record<string, { harness: string; model: string; effort: string | null }>; changed?: string[] }
export interface Team {
  id: Id; project: Id; stage: Stage; area: string | null; branch: string; needToKnow?: boolean; name?: string; forSprint?: Id; [more: string]: unknown;
}
export type Role = 'project-lead' | 'team-lead' | 'builder' | 'reviewer' | 'tester' | 'test-writer' | 'designer' | 'planner' | string;
export interface AgentFailure { reason: string; fix?: string; machine?: Id; subscription?: Id; [more: string]: unknown }
export interface Agent {
  id: Id; project: Id; team: Id | null; role: Role; name: string;
  harness: HarnessId; subscription: Id | null; model: string; effort: string | null; permission: string;
  state: 'working' | 'waiting' | 'idle' | 'stopped' | 'failed' | 'starting' | 'paused' | string;
  activity: string; task: string | null; context: number; turns: number; tokens: number;
  workState: string; stalled?: boolean; startedAt: Ms; lastOutputAt?: Ms; crashed?: boolean; failure?: AgentFailure | null;
  prevState?: string; prevActivity?: string;
  /** Live mode: the model and effort change the engine is applying, and the choices it offers. */
  configuration?: { status: string; reason?: string }; models?: (string | { id: string; label?: string })[]; efforts?: (string | { id: string; label?: string })[];
  [more: string]: unknown;
}

// ---- the plan: epics, sprints and tasks --------------------------------------------
export interface Epic { id: Id; project: Id; name: string }
export interface SprintPlan { file: string; src: string; state: string; by: string }
export interface SprintEvidence { tests?: string; review?: string; merged?: string }
export interface Sprint {
  id: Id; epic: Id; priority: string; start: number; end: number; project: Id; label: string; name: string; goal: string;
  phase: string; column: string; team: Id | null; dependsOn: Id[]; evidence?: SprintEvidence; plan?: SprintPlan;
  fix?: boolean; auto?: boolean;
  /** The numbers of the end result's statements that this sprint delivers. */
  statements?: number[];
  /** The card's place in its lane, after the owner drags it. */
  rank?: number;
  [more: string]: unknown;
}
export interface TaskEvent { ts: Ms; who: Id; text: string }
export interface Task {
  id: Id; project: Id; label: string; title: string; sprint: Id; team: Id | null; owner: Id | null; state: string;
  points: number; files: string[]; history: TaskEvent[]; blockedBy?: string | null; section?: string; sectionName?: string;
  /** Live only: the worktree the task's changes are read from. */
  worktreeId?: Id;
  [more: string]: unknown;
}

// ---- requests: questions, approvals, merges and failed starts ------------------------
export type RequestKind = 'question' | 'approval' | 'merge' | 'failed';
export interface RequestOption { id: string; label: string; primary?: boolean; note?: string; kind?: string }
export interface MergeCheck { ok: boolean; text: string; value?: string; sub?: string; agent?: Id }
/** The line counts and the conflict count are left out until they are known. */
export interface MergeEvidence { tests: MergeCheck; review: MergeCheck; commits: number; files: number; add?: number; del?: number; conflicts?: number | null }
/** How a merge reaches the base branch: a GitHub pull request, or the reason it is merged locally. */
export interface MergeDelivery { mode: 'github' | string; url?: string; number?: number; state?: string; reason?: string; title?: string; body?: string }
export interface Request {
  id: Id; kind: RequestKind | string; agent: Id; project: Id; title: string; createdAt: Ms; status: 'open' | 'answered' | 'cancelled' | string;
  task?: Id; sprint?: Id; body?: string; blocks?: string; why?: string; statement?: number;
  brief?: string; summary?: string; recommendation?: string; recShort?: string; recWhy?: string; notice?: string | null;
  what?: string[]; whyText?: string[]; refs?: string[]; files?: string[]; options?: RequestOption[];
  command?: string | null; folder?: string; tool?: string; blind?: boolean;
  source?: string; target?: string; evidence?: MergeEvidence; delivery?: MergeDelivery | null; localSync?: string | null;
  answer?: string; answeredAt?: Ms; heldBy?: Id | null; canAnswer?: boolean; answerKind?: string; reason?: string;
  projectApproval?: boolean; projectApprovalEffect?: string; projectApprovalOutcome?: string;
  [more: string]: unknown;
}

// ---- threads -------------------------------------------------------------------------
export interface ToolLine { verb: string; arg: string; res: string }
export interface ToolDetail { label: string; text: string; kind?: string; note?: string; copyLabel?: string }
/** A file or an image sent with a message, or staged in the composer. */
export interface Attachment {
  id?: Id; name: string; size: number; image?: boolean; url?: string | null; note?: string; from?: number; working?: boolean;
  useOriginal?: boolean; small?: { size: number; url: string; name: string } | null; file?: File;
}
export interface ThreadItem {
  id: Id; type: 'user' | 'agent' | 'tools' | 'request' | 'large' | string; ts: Ms;
  text?: string; from?: Id; requests?: Id[]; requestId?: Id;
  summary?: string; raw?: string; status?: string; details?: ToolDetail[];
  title?: string; size?: string; sequence?: number; files?: Attachment[]; context?: unknown[]; icon?: string;
  /** The line count of a large item; in the sample, the lines of a tool call before it is split. */
  lines?: number | ToolLine[];
  [more: string]: unknown;
}

// ---- the message board -----------------------------------------------------------------
export interface Channel { id: Id; project: Id; kind: 'leads' | 'team' | 'dm' | string; team?: Id; name: string | null; members: Id[]; archived?: boolean; [more: string]: unknown }
export interface Post {
  id: Id; channel: Id; from: Id; kind: string; action: string; text: string; ts: Ms; to: Id[]; replyTo: Id | null; closed: boolean;
  /** What a blocked post holds up, and the request that would clear it. */
  blocks?: string; request?: Id;
  /** Live only: how a closed post ended, and its place in the board's order. */
  outcome?: string; sequence?: number;
  [more: string]: unknown;
}

// ---- the page's own state ---------------------------------------------------------------
export interface Prefs {
  theme: string; density: string; sidebarFolded: boolean; sidebarWidth: number; panelWidth: number; panelOpen: boolean;
  onlyNeedingYou: boolean; talkWidth: number | null; listWidth: number | null; listOpen: boolean; termOpen: boolean; peek: boolean; pinned: Id[];
  centerHidden: string[]; repoCenterHidden: string[];
  // The owner's own order of the widgets, and the widgets whose width the owner changed (true is two columns).
  centerOrder: string[]; repoCenterOrder: string[]; centerWide: Record<string, boolean>; repoCenterWide: Record<string, boolean>;
  [more: string]: unknown;
}
export interface Toast { id: Id; text: ComponentChildren; tone: string; [more: string]: unknown }
/** The open dialog. Each dialog type reads its own fields. */
export interface DialogState { type: string; [field: string]: any }
export interface Settings {
  ntfyServer: string; ntfyTopic: string; ntfyToken: string; tokenSet: boolean; ntfyEnabled: boolean;
  email: { enabled: boolean; tokenSet: boolean; host?: string; port?: number; security?: string; username?: string; from?: string; to?: string; [more: string]: unknown };
  webEnabled: boolean; port: number; checkIn: number; proxyMode: boolean; publicAddress: string;
  stallWatch: boolean; standingRules: boolean; capacityBackoff: number; crashRetries: number; teamIdle: number;
  removeCredits: boolean; gitModel: string;
  notifyOn: Record<string, boolean>;
  quiet: { on: boolean | null; from: string; to: string; timeZone?: string; failed: boolean | null };
  defaultHarness: string; defaultModel: string; defaultEffort: string; defaultPermission: string;
  needToKnow: boolean; stallAssigned: number; stallWorker: number; stallLead: number; autoNudge: boolean; replaceAfter: number;
  capacityRetries: number; workflowPath: string; maxRounds: number; testsDefault: boolean; reviewDefault: boolean;
  version: string; commit: string; dataFolder: string; terminalEnabled?: boolean; workspaceFolder?: string;
  [more: string]: unknown;
}
/** The server the website talks to, and how far the first read of it has come. */
export interface ServerState {
  /** ask: no address yet. connecting: the first call is out. unreachable: nothing usable answered. ready: the server answered. */
  status: 'ask' | 'connecting' | 'unreachable' | 'ready';
  /** The address as the owner typed it, and as the page shows it. */
  address: string; label: string;
  /** True for a server that holds sample data only. */
  sample: boolean;
  /** True when discovery selected the real repository workspace. */
  repositoryMode: boolean;
  /** True once the records the server owns have arrived. */
  synced: boolean;
  /** True when the build fixed the address, so the page does not ask for one. */
  fixed: boolean;
  error?: string | undefined;
}
/** A client that used a token and waits for the owner's approval. */
export interface JoiningClient { id: Id; host: string; address: string; os: string; cores: number; ram: number; tokenId: string }
export interface Session { status: 'checking' | 'in' | 'out' | 'rejected' | 'expired' | 'unavailable' | string; firstRun: boolean; [more: string]: unknown }
export interface Connection {
  status: 'live' | 'reconnecting' | 'offline' | string; lastHeard: Ms;
  /** Live only: what the service said about the connection, and its start-up progress. */
  notice?: string; starting?: { recovered: number; total: number };
  [more: string]: unknown;
}
export interface Notification {
  id: Id; ts: Ms; kind: string; title: string; request?: Id; project?: Id; agent?: Id; read: boolean; delivery: string;
  to?: string; answer?: string; error?: string | null; deliveries?: { carrier: string; status: string; reason?: string }[];
  [more: string]: unknown;
}
export interface UiState {
  panelAgent?: Id | null; panelReq?: Id | null; panelDrawer?: boolean; declined?: Record<Id, boolean>; saved?: Ms | null; serviceReady?: boolean;
  boardBusy?: Record<string, boolean>; chatBusy?: Record<string, boolean>; setupCreated?: Id;
  /** Live only: the Git and service actions in flight, by key. */
  gitPending?: Record<string, boolean>;
  /** Focus hides the shell around the board. panelAuto opens the lead panel on a narrow screen. */
  focus?: boolean; panelAuto?: boolean; ccEdit?: boolean;
  /** Where the terminal outside a project is open: a repository's main checkout, or a machine. */
  termPlace?: string | null;
  /** Whether the sample server is connected to GitHub, and the repository that is being set up. */
  /** The GitHub accounts the server is connected to. */
  github?: string[]; settingUp?: Record<Id, Id>;
  [more: string]: unknown;
}

/**
 * Records that only some screens use. Their screens own their exact shape, and the sample
 * keeps them loose, so they are typed as open records here.
 */
export type Loose = Record<string, any>;

// ---- the store's state ------------------------------------------------------------------
export interface State {
  // The page's own state.
  prefs: Prefs; drafts: Record<string, string>; pending: Record<string, any>; expanded: Record<string, boolean>;
  inspecting: Record<Id, string>; toasts: Toast[]; dialog: DialogState | null; phoneOffer: string | null;
  settings: Settings; ui: UiState;
  // The records.
  session: Session; conn: Connection;
  server: ServerState; joining: JoiningClient[];
  machines: Machine[]; subscriptions: Subscription[]; repos: Repo[]; projects: Project[]; teams: Team[]; agents: Agent[];
  epics: Epic[]; sprints: Sprint[]; tasks: Task[]; requests: Request[]; threads: Record<Id, ThreadItem[]>;
  channels: Channel[]; posts: Post[];
  /** The time the sample was made. Sample times are counted back from it. */
  T: Ms;
  // Slices that each screen seeds.
  define: Record<Id, Loose>;
  worktrees: Worktree[];
  gitUi: GitUi;
  terminal: TerminalState;
  mocks: Record<Id, Mocks>;
  design: Record<Id, RepoDesignRecord>;
  boardRead: Record<Id, number>;
  workflow: Record<Id, ProjectWorkflow>;
  notifications: Notification[];
  usage: UsageState;
  /** The sample Files tab's committed text and saved text, by worktree and path. */
  fileBase?: Record<Id, Record<string, string>>; fileSaves?: Record<Id, Record<string, string>>;
  // Slices the live website fills.
  /** Live only: true once the engine's first snapshot is in the store. */
  snapshotReady?: boolean;
  boardUnread?: Record<Id, number>; boardRevision?: number; boardDetails?: Loose; taskDetails?: Loose; setupData?: Loose | null;
  boardAgents?: Loose; projectLeadSettings?: Loose; machineDetails?: Loose | null; accountLimits?: Loose;
}

/** What agentStatus() and projectStatus() return: a dot, a short word, and a tone. */
export interface Status { dot?: string; word: string; tone: string | null; n?: number }

/** The route parsed from the address after the #. */
export interface Route { parts: string[]; query: URLSearchParams; path: string }
