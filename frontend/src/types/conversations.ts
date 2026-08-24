export interface Conversation {
  id: number;
  game_id: number;
  title?: string;
  conversation_type: string;
  created_by_user_id: number;
  created_at: string;
  updated_at: string;
}

interface ConversationParticipant {
  id: number;
  conversation_id: number;
  user_id: number;
  character_id?: number;
  joined_at: string;
  username: string;
  character_name?: string;
  character_avatar_url?: string | null;
}

export interface PrivateMessage {
  id: number;
  conversation_id: number;
  sender_user_id?: number;
  sender_character_id?: number;
  content: string;
  sent_at?: string;
  created_at: string;
  sender_username: string;
  sender_character_name?: string;
  sender_avatar_url?: string | null;
  deleted_at?: string;
  is_deleted?: boolean;
  is_edited?: boolean;
  edited_at?: string;
  edit_count?: number;
}

export interface ConversationListItem {
  id: number;
  game_id: number;
  title?: string;
  conversation_type: string;
  created_by_user_id: number;
  created_at: string;
  updated_at: string;
  participant_count: number;
  participant_names?: string;
  last_message?: string;
  last_message_at?: string | null;
  unread_count: number;
  last_read_message_id?: number;
  last_read_at?: string;
}

export interface ConversationWithDetails {
  conversation: Conversation;
  participants: ConversationParticipant[];
}

// Request types
export interface CreateConversationRequest {
  title?: string;
  character_ids: number[]; // At least 2 characters required
}

export interface SendMessageRequest {
  character_id: number;
  content: string;
}

export interface AddParticipantRequest {
  character_id: number;
}

export interface UpdateMessageRequest {
  content: string;
}

// Audience viewing types (read-only conversation access for audience members)
export interface AudienceConversationListItem {
  conversation_id: number;
  subject?: string | null;
  conversation_type: string;
  created_at: string;
  message_count: number;
  last_message_at?: string | null;
  participant_names: string[];
  participant_usernames: string[];
  participant_character_ids?: (number | null)[];
  last_message_content?: string | null;
  last_sender_name?: string | null;
  last_sender_username?: string | null;
  last_sender_character_id?: number | null;
}

export interface AudienceConversationMessage {
  id: number;
  conversation_id: number;
  sender_user_id?: number;
  sender_character_id?: number;
  content: string;
  created_at: string;
  updated_at: string;
  is_deleted: boolean;
  sender_username: string;
  sender_character_name?: string | null;
}

/**
 * A character appearing in at least one conversation, used for the audience
 * filter controls. The UI displays `name` but filters by `id`, since character
 * names are mutable and not unique within a game.
 */
export interface ConversationParticipantCharacter {
  id: number;
  name: string;
}
