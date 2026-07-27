// Message/Post types for Common Room

export interface Message {
  id: number;
  game_id: number;
  phase_id?: number;
  author_id: number;
  character_id: number;
  content: string;
  message_type: 'post' | 'comment' | 'private_message';
  parent_id?: number;
  thread_depth: number;
  author_username: string;
  character_name: string;
  character_avatar_url?: string | null;
  comment_count?: number;
  reply_count?: number;
  is_edited: boolean;
  is_deleted: boolean;
  is_draft: boolean;
  created_at: string;
  updated_at: string;
  mentioned_character_ids?: number[];
  // Edit/Delete tracking fields
  deleted_at?: string | null;
  deleted_by_user_id?: number | null;
  edited_at?: string | null;
  edit_count?: number;
}

export interface CreatePostRequest {
  phase_id?: number;
  character_id: number;
  content: string;
}

export interface CreateCommentRequest {
  phase_id?: number;
  character_id: number;
  content: string;
  root_post_id?: number;
}

export interface UpdateCommentRequest {
  content: string;
  character_id?: number;
}

export interface GetPostsParams {
  phase_id?: number;
  limit?: number;
  offset?: number;
}

// Read tracking types
export interface ReadMarker {
  id: number;
  user_id: number;
  game_id: number;
  post_id: number;
  last_read_comment_id?: number | null;
  last_read_at: string;
  created_at: string;
  updated_at: string;
}

export interface PostUnreadInfo {
  post_id: number;
  post_created_at: string;
  total_comments: number;
  latest_comment_at?: string | null;
}

export interface MarkPostReadRequest {
  last_read_comment_id?: number | null;
}

// Unread comment IDs for posts (new since last visit)
export interface PostUnreadComments {
  post_id: number;
  unread_comment_ids: number[];
}

// Manually read comment IDs for a post (user-controlled, persisted)
export interface ManualCommentReads {
  post_id: number;
  read_comment_ids: number[];
}

// Paginated comments with threads (includes depth for tree building)
export interface CommentWithDepth extends Message {
  depth: number; // Nesting depth (0 = top-level, 1+ = nested replies)
}

export interface PaginatedCommentsResponse {
  comments: CommentWithDepth[];
  total_top_level: number;      // Total top-level comments
  returned_top_level: number;   // Top-level comments in this response
  returned_total: number;       // Total comments including nested
  has_more: boolean;            // More pages available?
  limit: number;
  offset: number;
}

// Deep-link thread context (for jumping to a nested comment).
// Returned by GET /games/{id}/messages/{messageId}/thread-context.
export interface MessageThreadContext {
  // Target comment plus up to max_parents nearest ancestors,
  // ordered parent-to-child (nearest included ancestor → target).
  chain: Message[];
  // True top-level post ID, even when the chain is trimmed above the target.
  root_post_id: number;
  // Whether the chain reaches the root post (nothing trimmed above).
  has_full_thread: boolean;
}

// Comment with parent context (for "New Comments" view)
export interface CommentWithParent {
  // Comment data
  id: number;
  game_id: number;
  parent_id?: number | null;
  post_id?: number | null;
  author_id: number;
  character_id: number;
  content: string;
  created_at: string;
  updated_at: string;
  edited_at?: string | null;
  edit_count: number;
  deleted_at?: string | null;
  is_deleted: boolean;
  author_username: string;
  character_name?: string | null;
  character_avatar_url?: string | null;

  // Parent context (nested object from backend)
  parent?: {
    content?: string | null;
    created_at?: string | null;
    deleted_at?: string | null;
    is_deleted?: boolean | null;
    message_type?: string | null;
    author_username?: string | null;
    character_name?: string | null;
    character_avatar_url?: string | null;
  } | null;

  // Parent context (flattened fields)
  parent_content?: string | null;
  parent_created_at?: string | null;
  parent_deleted_at?: string | null;
  parent_is_deleted?: boolean | null;
  parent_message_type?: string | null;
  parent_author_username?: string | null;
  parent_character_name?: string | null;
  parent_character_avatar_url?: string | null;
}

// Pagination response for recent comments
export interface RecentCommentsResponse {
  comments: CommentWithParent[];
  total: number;
  limit: number;
  offset: number;
}

// A post or comment by a specific character (for Character Page)
export interface CharacterMessage {
  id: number;
  game_id: number;
  parent_id?: number | null;
  author_id: number;
  character_id: number;
  content: string;
  message_type: 'post' | 'comment';
  created_at: string;
  edited_at?: string | null;
  edit_count: number;
  deleted_at?: string | null;
  is_deleted: boolean;
  author_username: string;
  character_name?: string | null;
  character_avatar_url?: string | null;

  // Parent context (only set for comments)
  parent?: {
    content?: string | null;
    created_at?: string | null;
    deleted_at?: string | null;
    is_deleted?: boolean | null;
    message_type?: string | null;
    author_username?: string | null;
    character_name?: string | null;
    character_avatar_url?: string | null;
  } | null;
}

export interface CharacterMessagesResponse {
  messages: CharacterMessage[];
  pagination: {
    limit: number;
    offset: number;
    total: number;
  };
}
