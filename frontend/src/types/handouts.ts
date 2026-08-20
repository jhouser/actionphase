/**
 * Handouts Types
 *
 * Handouts are GM-created reference materials (rules, world info) that exist across all game phases.
 * Only GMs can create/update/delete handouts. Players can view published handouts.
 */

export interface Handout {
  id: number;
  game_id: number;
  title: string;
  content: string; // Markdown content
  status: 'draft' | 'published';
  created_at?: string;
  updated_at?: string;
}

/**
 * A handout together with the game it belongs to, as returned by the cross-game
 * list. Used by the global Utility Drawer, where there is no game in scope to
 * resolve the title from.
 */
export interface HandoutWithGame extends Handout {
  game_title: string;
}

export interface HandoutComment {
  id: number;
  handout_id: number;
  user_id: number;
  parent_comment_id?: number | null;
  content: string;
  edit_count: number;
  created_at?: string;
  updated_at?: string;
  edited_at?: string | null;
  deleted_at?: string | null;
  deleted_by_user_id?: number | null;
}

// Request types
export interface CreateHandoutRequest {
  title: string;
  content: string;
  status: 'draft' | 'published';
}

export interface UpdateHandoutRequest {
  title: string;
  content: string;
  status: 'draft' | 'published';
}

export interface CreateHandoutCommentRequest {
  content: string;
  parent_comment_id?: number;
}

export interface UpdateHandoutCommentRequest {
  content: string;
}
