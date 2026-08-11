/**
 * Post-game statistics for completed games.
 *
 * Terminology: a "comment" here is what players call a post — an entry in a
 * common room discussion. The backend's `post` message type is the GM-authored
 * topic each phase's discussion hangs off, and is excluded from every statistic
 * below. The UI says "comments" throughout to keep the two unambiguous.
 */

interface CommentStats {
  total_comments: number;
  avg_comment_length: number;
  longest_comment_length: number;
  /** Reply nesting depth, 0-based on the topic post. */
  max_thread_depth: number;
  avg_thread_depth: number;
}

interface PrivateMessageStats {
  total_private_messages: number;
  avg_private_message_length: number;
  longest_private_message_length: number;
  /** Conversations carrying at least one message, not merely created. */
  active_conversations: number;
}

/** The private conversation with the most messages. */
interface LongestConversation {
  conversation_id: number;
  title?: string;
  conversation_type: string;
  message_count: number;
  participant_names: string[];
  first_message_at?: string;
  last_message_at?: string;
}

/** One character's writing stats. Characters with no activity appear with zeros. */
export interface CharacterWriting {
  character_id: number;
  character_name: string;
  is_active: boolean;
  comment_count: number;
  avg_comment_length: number;
  private_message_count: number;
  avg_private_message_length: number;
}

/** The single longest comment, carrying enough identity to deep-link to it. */
interface LongestComment {
  message_id: number;
  content_length: number;
  character_name?: string;
  phase_id?: number;
}

export interface GameStats {
  comments: CommentStats;
  private_messages: PrivateMessageStats;
  characters: CharacterWriting[];
  /** Absent when the game had no private conversations carrying messages. */
  longest_conversation?: LongestConversation;
  /** Absent when the game had no comments. */
  longest_comment?: LongestComment;
}
