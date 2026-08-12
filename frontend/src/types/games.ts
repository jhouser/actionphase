export interface Game {
  id: number;
  title: string;
  description: string;
  gm_user_id: number;
  state: GameState;
  genre?: string;
  start_date?: string;
  end_date?: string;
  recruitment_deadline?: string;
  max_players?: number;
  is_anonymous?: boolean;
  auto_accept_audience?: boolean;
  allow_group_conversations?: boolean;
  portrait_avatars?: boolean;
  banner_url?: string | null;
  common_room_open_day?: number | null;
  common_room_open_time?: string | null;
  common_room_close_day?: number | null;
  common_room_close_time?: string | null;
  schedule_timezone?: string | null;
  created_at: string;
  updated_at: string;
}

export interface GameWithDetails extends Game {
  gm_username?: string;
  current_players: number;
}

export interface GameListItem extends Game {
  gm_username: string;
  current_players?: number;
}

export interface GameParticipant {
  id: number;
  game_id: number;
  user_id: number;
  username: string;
  avatar_url?: string | null;
  role: ParticipantRole;
  status: ParticipantStatus;
  joined_at: string;
  is_former_player?: boolean;
}

export type GameState =
  | 'setup'
  | 'recruitment'
  | 'character_creation'
  | 'in_progress'
  | 'paused'
  | 'completed'
  | 'cancelled';

type ParticipantRole = 'player' | 'co_gm' | 'audience';
type ParticipantStatus = 'active' | 'inactive' | 'removed';

export interface CreateGameRequest {
  title: string;
  description: string;
  genre?: string;
  start_date?: string;
  end_date?: string;
  recruitment_deadline?: string;
  max_players?: number;
  is_anonymous?: boolean;
  auto_accept_audience?: boolean;
  allow_group_conversations?: boolean;
  portrait_avatars?: boolean;
  banner_url?: string | null;
  common_room_open_day?: number | null;
  common_room_open_time?: string | null;
  common_room_close_day?: number | null;
  common_room_close_time?: string | null;
  schedule_timezone?: string | null;
}

export interface UpdateGameRequest extends CreateGameRequest {
  is_public: boolean;
  is_anonymous?: boolean;
  auto_accept_audience?: boolean;
  allow_group_conversations?: boolean;
  portrait_avatars?: boolean;
}

export interface ApplyToGameRequest {
  role: 'player' | 'audience';
  message?: string;
}

export interface GameApplication {
  id: number;
  game_id: number;
  user_id: number;
  username?: string;
  avatar_url?: string | null;
  role: 'player' | 'audience';
  message?: string;
  status: ApplicationStatus;
  applied_at: string;
  reviewed_at?: string;
  reviewed_by_user_id?: number;
}


export interface GameLog {
  id: number;
  game_id: number;
  type: string;
  message: string;
  created_at: string;
}

export interface CreateLootTableRequest {
  name: string;
  items: LootTableContent[] | undefined;
}

export interface UpdateLootTableRequest {
  id: number;
  name: string;
}

export interface UpdateLootTableContentsRequest {
  id: number;
  items: LootTableContent[];
}

export interface LootTable {
  id: number;
  game_id: number;
  name: string;
  created_at: string;
  /** Bumped when the table is renamed. Equals created_at until then. */
  updated_at: string;
}

export interface LootTableContent {
  id: number;
  name: string;
  data: string;
}

// Public applicant view (no status, message, email, or review info)
// Available to anyone when game is in recruitment state
export interface PublicGameApplicant {
  id: number;
  username: string;
  avatar_url?: string | null;
  role: 'player' | 'audience';
  applied_at: string;
}

export type ApplicationStatus = 'pending' | 'approved' | 'rejected' | 'withdrawn';

export interface ReviewApplicationRequest {
  action: 'approve' | 'reject';
}

export interface UpdateGameStateRequest {
  state: GameState;
}

export const GAME_STATE_LABELS: Record<GameState, string> = {
  setup: 'Setup',
  recruitment: 'Recruiting Players',
  character_creation: 'Character Creation',
  in_progress: 'In Progress',
  paused: 'Paused',
  completed: 'Completed',
  cancelled: 'Cancelled'
};

export const GAME_STATE_COLORS: Record<GameState, string> = {
  setup: 'surface-raised text-content-secondary',
  recruitment: 'bg-semantic-success-subtle text-content-primary',
  character_creation: 'bg-interactive-primary-subtle text-content-primary',
  in_progress: 'bg-semantic-warning-subtle text-content-primary',
  paused: 'bg-semantic-warning-subtle text-content-primary',
  completed: 'bg-semantic-info-subtle text-content-primary',
  cancelled: 'bg-semantic-danger-subtle text-content-primary'
};

export const APPLICATION_STATUS_LABELS: Record<ApplicationStatus, string> = {
  pending: 'Pending Review',
  approved: 'Approved',
  rejected: 'Rejected',
  withdrawn: 'Withdrawn'
};

export const APPLICATION_STATUS_COLORS: Record<ApplicationStatus, string> = {
  pending: 'bg-semantic-warning-subtle text-content-primary',
  approved: 'bg-semantic-success-subtle text-content-primary',
  rejected: 'bg-semantic-danger-subtle text-content-primary',
  withdrawn: 'surface-raised text-content-secondary'
};

// Enhanced game listing types

export type UserRelationship = 'gm' | 'participant' | 'applied' | 'none';
type DeadlineUrgency = 'critical' | 'warning' | 'normal';
type PhaseType = 'action' | 'common_room';

export interface EnrichedGameListItem extends Game {
  gm_username: string;
  current_players: number;
  is_public: boolean;
  user_relationship?: UserRelationship;
  current_phase_type?: PhaseType;
  current_phase_deadline?: string;
  deadline_urgency: DeadlineUrgency;
  has_recent_activity: boolean;
}

interface GameListingMetadata {
  total_count: number;
  filtered_count: number;
  available_states: GameState[];
  page: number;
  page_size: number;
  total_pages: number;
  has_next_page: boolean;
  has_previous_page: boolean;
}

export interface GameListingResponse {
  games: EnrichedGameListItem[];
  metadata: GameListingMetadata;
}

export type ParticipationFilter = 'my_games' | 'applied' | 'not_joined';
export type SortBy = 'recent_activity' | 'created' | 'start_date' | 'alphabetical';

export interface GameListingFilters {
  search?: string;
  states?: GameState[];
  participation?: ParticipationFilter;
  has_open_spots?: boolean;
  sort_by?: SortBy;
  admin_mode?: boolean;
  page?: number;
  page_size?: number;
}


export const USER_RELATIONSHIP_LABELS: Record<UserRelationship, string> = {
  gm: 'You are GM',
  participant: 'You are playing',
  applied: 'Application pending',
  none: ''
};

export const SORT_BY_LABELS: Record<SortBy, string> = {
  recent_activity: 'Recent Activity',
  created: 'Recently Created',
  start_date: 'Starting Soon',
  alphabetical: 'Alphabetical'
};
