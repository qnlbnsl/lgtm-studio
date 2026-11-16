import { PHASES } from '../data.js';

// Missing records stay empty. Only the engine's actual phase boundaries count.
export const recordedPhaseTimes = (project) => PHASES.map(phase =>
  (project.phaseTimes || []).find(time => time.phase === phase.id) || null);
