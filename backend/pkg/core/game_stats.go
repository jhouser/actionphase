package core

import "time"

// GameStats is the full post-game statistics payload for a completed game.
//
// Terminology note: what players call a "post" is a `comment` row in the
// messages table. The `post` message_type is the GM-authored topic container a
// phase's discussion hangs off (typically one post per phase with hundreds of
// comments beneath it). Every writing statistic here is therefore computed over
// comments; GM posts are deliberately excluded.
type GameStats struct {
	Comments        CommentStats        `json:"comments"`
	PrivateMessages PrivateMessageStats `json:"private_messages"`
	Characters      []CharacterWriting  `json:"characters"`
	// LongestConversation is nil when the game has no private conversations
	// carrying any messages, which is a normal state and not an error.
	LongestConversation *LongestConversation `json:"longest_conversation,omitempty"`
	// LongestComment is nil when the game has no comments.
	LongestComment *LongestComment `json:"longest_comment,omitempty"`
}

// LongestComment identifies the single longest comment so the UI can deep-link
// to it. Separate from CommentStats.LongestCommentLength, which is the plain
// aggregate; this carries the identity needed to build the link.
type LongestComment struct {
	MessageID     int32  `json:"message_id"`
	ContentLength int64  `json:"content_length"`
	CharacterName string `json:"character_name,omitempty"`
	// PhaseID is the phase the comment belongs to, used to resolve the deep
	// link in the History view. Zero when the comment has no phase.
	PhaseID int32 `json:"phase_id,omitempty"`
}

// CommentStats holds game-wide common room aggregates.
type CommentStats struct {
	TotalComments        int64 `json:"total_comments"`
	AvgCommentLength     int64 `json:"avg_comment_length"`
	LongestCommentLength int64 `json:"longest_comment_length"`
	// MaxThreadDepth and AvgThreadDepth describe reply-tree shape. Depth is
	// stored 0-based on messages.thread_depth.
	MaxThreadDepth int64   `json:"max_thread_depth"`
	AvgThreadDepth float64 `json:"avg_thread_depth"`
}

// PrivateMessageStats holds game-wide private messaging aggregates.
type PrivateMessageStats struct {
	TotalPrivateMessages        int64 `json:"total_private_messages"`
	AvgPrivateMessageLength     int64 `json:"avg_private_message_length"`
	LongestPrivateMessageLength int64 `json:"longest_private_message_length"`
	// ActiveConversations counts conversations that carry at least one
	// non-deleted message, not conversations that were merely created.
	ActiveConversations int64 `json:"active_conversations"`
}

// LongestConversation describes the private conversation with the most
// messages. "Longest" is defined by message count rather than total characters
// written or wall-clock duration.
type LongestConversation struct {
	ConversationID   int32      `json:"conversation_id"`
	Title            string     `json:"title,omitempty"`
	ConversationType string     `json:"conversation_type"`
	MessageCount     int64      `json:"message_count"`
	ParticipantNames []string   `json:"participant_names"`
	FirstMessageAt   *time.Time `json:"first_message_at,omitempty"`
	LastMessageAt    *time.Time `json:"last_message_at,omitempty"`
}

// CharacterWriting holds one character's writing stats within a game.
// Characters with no activity are still included, with zero counts.
type CharacterWriting struct {
	CharacterID             int32  `json:"character_id"`
	CharacterName           string `json:"character_name"`
	IsActive                bool   `json:"is_active"`
	CommentCount            int64  `json:"comment_count"`
	AvgCommentLength        int64  `json:"avg_comment_length"`
	PrivateMessageCount     int64  `json:"private_message_count"`
	AvgPrivateMessageLength int64  `json:"avg_private_message_length"`
}
