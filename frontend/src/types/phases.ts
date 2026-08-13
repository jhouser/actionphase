export interface GamePhase {
  id: number;
  game_id: number;
  phase_type: 'common_room' | 'action' | 'interlude';
  phase_number: number;
  title?: string;
  description?: string;
  start_time: string;
  end_time?: string;
  deadline?: string;
  is_active: boolean;
  is_published: boolean; // For action phases: whether GM has published results
  activated_at?: string; // Set when phase is first activated; null means never activated
  created_at: string;

  // Calculated fields from API
  time_remaining?: number; // seconds until deadline
  is_expired?: boolean;
}

export interface CreatePhaseRequest {
  phase_type: 'common_room' | 'action' | 'interlude';
  title?: string;
  description?: string;
  start_time?: string;
  end_time?: string;
  deadline?: string;
}

export interface UpdatePhaseRequest {
  title?: string;
  description?: string;
  start_time?: string;
  end_time?: string;
  deadline?: string;
}

export interface UpdateDeadlineRequest {
  deadline: string;
}

export interface ActionSubmission {
  id: number;
  game_id: number;
  user_id: number;
  phase_id: number;
  character_id?: number;
  content: string;
  submitted_at: string;
  updated_at: string;
}

export interface ActionSubmissionRequest {
  character_id?: number;
  content: string;
}

export interface ActionWithDetails extends ActionSubmission {
  username?: string;
  character_name?: string;
  phase_type?: string;
  phase_number?: number;
  phase_title?: string;
}

export interface ActionResult {
  id: number;
  game_id: number;
  user_id: number;
  phase_id: number;
  character_id?: number;
  action_submission_id?: number;
  gm_user_id: number;
  content: string;
  is_published: boolean;
  sent_at: string;
  phase_type?: string;
  phase_number?: number;
  gm_username?: string;
  username?: string;
  character_name?: string;

  // Staged reveal fields. All optional: an ordinary single-part result omits
  // every one of them, so existing code that never mentions staging is
  // unaffected.
  //
  // part_number / part_count are present only for a result that belongs to a
  // multi-part chain, and describe its position ("Part 2 of 3").
  part_number?: number;
  part_count?: number;

  // When this part became visible to its recipient. Absent while the part is
  // still locked — this, NOT an empty content string, is how you tell a locked
  // part from a released one. A player's response carries locked parts with
  // content blanked server-side.
  released_at?: string;

  // When a locked part is due to be revealed, for the countdown. Present only
  // for the next part due out; later parts have no knowable unlock time until
  // their predecessor releases, and show a plain "pending" state instead.
  unlocks_at?: string;
}

// One part of a staged result chain as the GM composes it. The head must carry
// delay_minutes: 0 — its delay is meaningless because it releases on publish,
// and the API rejects a head with a non-zero delay rather than ignoring it.
export interface StagedResultPart {
  content: string;
  delay_minutes: number;
}

export interface DraftCharacterUpdate {
  id: number;
  action_result_id: number;
  character_id: number;
  module_type: 'abilities' | 'skills' | 'inventory' | 'currency';
  field_name: string;
  field_value: string;
  field_type: 'text' | 'number' | 'boolean' | 'json';
  operation: 'upsert' | 'delete';
  created_at: string;
  updated_at: string;
}

export interface CreateDraftCharacterUpdateRequest {
  character_id: number;
  module_type: 'abilities' | 'skills' | 'inventory' | 'currency';
  field_name: string;
  field_value: string;
  field_type: 'text' | 'number' | 'boolean' | 'json';
  operation: 'upsert' | 'delete';
}

export interface UpdateDraftCharacterUpdateRequest {
  field_value: string;
}

// Phase display helpers
export const PHASE_TYPE_LABELS: Record<GamePhase['phase_type'], string> = {
  common_room: 'Common Room',
  action: 'Action Phase',
  interlude: 'Interlude'
};

export const PHASE_TYPE_DESCRIPTIONS: Record<GamePhase['phase_type'], string> = {
  common_room: 'Open discussion and roleplay between characters. The GM creates a public post and players can comment and send private messages.',
  action: 'Players submit private actions to the GM for resolution. No public roleplay or private messaging.',
  interlude: 'Private messaging only. No public post or action submissions.'
};

const PHASE_TYPE_COLORS: Record<GamePhase['phase_type'], string> = {
  common_room: 'bg-semantic-success-subtle text-content-primary border-semantic-success',
  action: 'bg-interactive-primary-subtle text-content-primary border-interactive-primary',
  interlude: 'bg-semantic-warning-subtle text-content-primary border-semantic-warning'
};

// Action phase states
export const getActionPhaseLabel = (phase: GamePhase): string => {
  if (phase.phase_type !== 'action') return PHASE_TYPE_LABELS[phase.phase_type];
  return phase.is_published ? 'Results Published' : 'Action Phase';
};

export const getActionPhaseDescription = (phase: GamePhase): string => {
  if (phase.phase_type !== 'action') return PHASE_TYPE_DESCRIPTIONS[phase.phase_type];
  return phase.is_published
    ? 'GM has published the results and consequences of player actions'
    : 'Submit private actions to the GM';
};

export const getActionPhaseColor = (phase: GamePhase): string => {
  if (phase.phase_type !== 'action') return PHASE_TYPE_COLORS[phase.phase_type];
  return phase.is_published
    ? 'bg-semantic-info-subtle text-content-primary border-semantic-info'
    : 'bg-interactive-primary-subtle text-content-primary border-interactive-primary';
};
