import { PHASES } from '../data';
import type { Project } from '../types';

/** When the engine recorded a project entering one phase. */
export interface PhaseTime { phase?: string; start: number; end?: number | null }

// Missing records stay empty. Only the engine's actual phase boundaries count.
export const recordedPhaseTimes = (project: Project): (PhaseTime | null)[] => PHASES.map(phase =>
  ((project.phaseTimes || []) as PhaseTime[]).find(time => time.phase === phase.id) || null);
