package messages

import (
	"context"
	"fmt"

	core "actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"actionphase/pkg/observability"
	"actionphase/pkg/validation"

	"github.com/jackc/pgx/v5/pgtype"
)

// GetRecursiveCommentCount counts all descendant comments recursively
func (s *MessageService) GetRecursiveCommentCount(ctx context.Context, parentID int32) (int64, error) {
	queries := models.New(s.DB)

	descendants, err := queries.GetAllDescendantComments(ctx, int32ValueToPgInt4(parentID))
	if err != nil {
		return 0, fmt.Errorf("failed to count descendants: %w", err)
	}

	return int64(len(descendants)), nil
}

// CreateComment creates a comment reply to a post or another comment
func (s *MessageService) CreateComment(ctx context.Context, req core.CreateCommentRequest) (*models.Message, error) {
	queries := models.New(s.DB)

	// Validate game is not completed/cancelled (archived games are read-only)
	game, err := queries.GetGame(ctx, req.GameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get game: %w", err)
	}

	if err := core.ValidateGameNotCompleted(ctx, &game); err != nil {
		return nil, err
	}

	// Validate character ownership before creating comment
	if err := s.ValidateCharacterOwnership(ctx, req.CharacterID, req.AuthorID, req.GameID); err != nil {
		return nil, fmt.Errorf("character validation failed: %w", err)
	}

	// Validate content length
	if err := validation.ValidateComment(req.Content); err != nil {
		return nil, err
	}

	// Extract character mentions from content
	mentionedIDs, err := s.extractCharacterMentions(ctx, req.Content, req.GameID)
	if err != nil {
		// Log error but don't fail the comment creation
		// Mention extraction is a non-critical feature
		s.Logger.LogError(ctx, err, "Failed to extract mentions during comment creation",
			"game_id", req.GameID,
			"character_id", req.CharacterID,
		)
		mentionedIDs = []int32{}
	}

	// A comment always belongs to the same phase as what it replies to, so the
	// phase is derived here rather than trusted from the client. Reply surfaces
	// that render flat, cross-phase comment lists (the Dashboard unread inbox and
	// the New Comments view) have no phase in scope and omit it. Without this
	// fallback those replies stored phase_id = NULL, which reads fine inline but
	// makes them undeep-linkable: GetMessage omits a NULL phase_id from the
	// response, so HistoryView cannot tell which phase to open.
	phaseID := int32ToPgInt4(req.PhaseID)
	if !phaseID.Valid {
		inherited, perr := queries.GetMessagePhaseID(ctx, req.ParentID)
		if perr != nil {
			// Non-fatal: a comment with no phase is still a valid comment (legacy
			// rows predate phase tracking), so don't fail the write over it.
			s.Logger.LogError(ctx, perr, "Failed to inherit phase from parent message",
				"game_id", req.GameID,
				"parent_id", req.ParentID,
			)
		} else {
			phaseID = inherited
		}
	}

	// Create the comment using sqlc-generated query
	message, err := queries.CreateComment(ctx, models.CreateCommentParams{
		GameID:                req.GameID,
		PhaseID:               phaseID,
		AuthorID:              req.AuthorID,
		CharacterID:           req.CharacterID,
		Content:               req.Content,
		ParentID:              int32ValueToPgInt4(req.ParentID),
		Visibility:            models.MessageVisibility(req.Visibility),
		MentionedCharacterIds: mentionedIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	s.Logger.Info(ctx, "Comment created",
		"message_id", message.ID,
		"game_id", message.GameID,
		"phase_id", message.PhaseID,
		"parent_id", req.ParentID,
		"author_id", req.AuthorID,
		"character_id", req.CharacterID,
		"visibility", string(message.Visibility),
		"mention_count", len(mentionedIDs),
	)
	s.Metrics.RecordCommentCreated(ctx)

	// Auto-mark the comment as read for its author — they just wrote it.
	// Requires RootPostID to be set (zero means caller didn't provide it; skip silently).
	// ON CONFLICT DO NOTHING makes this idempotent.
	if req.RootPostID != 0 {
		if err := queries.MarkCommentRead(ctx, models.MarkCommentReadParams{
			UserID:    req.AuthorID,
			CommentID: message.ID,
			PostID:    req.RootPostID,
			GameID:    req.GameID,
		}); err != nil {
			// Non-fatal: log and continue. The comment was created successfully.
			s.Logger.LogError(ctx, err, "Failed to auto-mark own comment as read",
				"comment_id", message.ID,
				"author_id", req.AuthorID,
				"post_id", req.RootPostID,
			)
		}
	}

	// Preserve context values (correlation_id, trace_id) without inheriting cancellation
	notifCtx := context.WithoutCancel(ctx)

	// Trigger notifications for character mentions (fire-and-forget)
	if len(mentionedIDs) > 0 {
		observability.SafeGo(notifCtx, s.Logger, "notify-character-mentions", func() {
			s.notifyCharacterMentions(notifCtx, mentionedIDs, req.CharacterID, req.AuthorID, req.GameID, message.ID)
		})
	}

	// Trigger notification for comment reply (fire-and-forget)
	observability.SafeGo(notifCtx, s.Logger, "notify-comment-reply", func() {
		s.notifyCommentReply(notifCtx, req.ParentID, req.CharacterID, req.AuthorID, req.GameID, message.ID)
	})

	return &message, nil
}

// GetComment retrieves a specific comment by ID with metadata
func (s *MessageService) GetComment(ctx context.Context, commentID int32) (*core.MessageWithDetails, error) {
	queries := models.New(s.DB)

	comment, err := queries.GetComment(ctx, commentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get comment: %w", err)
	}

	var avatarURL *string
	if comment.CharacterAvatarUrl.Valid {
		avatarURL = &comment.CharacterAvatarUrl.String
	}

	return &core.MessageWithDetails{
		Message: models.Message{
			ID:                    comment.ID,
			GameID:                comment.GameID,
			PhaseID:               comment.PhaseID,
			AuthorID:              comment.AuthorID,
			CharacterID:           comment.CharacterID,
			Content:               comment.Content,
			MessageType:           comment.MessageType,
			ParentID:              comment.ParentID,
			ThreadDepth:           comment.ThreadDepth,
			Visibility:            comment.Visibility,
			MentionedCharacterIds: comment.MentionedCharacterIds,
			IsEdited:              comment.IsEdited,
			IsDeleted:             comment.IsDeleted,
			CreatedAt:             comment.CreatedAt,
			DeletedAt:             comment.DeletedAt,
			DeletedByUserID:       comment.DeletedByUserID,
			EditedAt:              comment.EditedAt,
			EditCount:             comment.EditCount,
		},
		AuthorUsername:     comment.AuthorUsername,
		CharacterName:      comment.CharacterName.String,
		CharacterAvatarUrl: avatarURL,
		ReplyCount:         comment.ReplyCount,
	}, nil
}

// GetMessage retrieves a single message by ID (used for deep linking)
func (s *MessageService) GetMessage(ctx context.Context, messageID int32) (*core.MessageWithDetails, error) {
	queries := models.New(s.DB)

	message, err := queries.GetMessage(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	var avatarURL *string
	if message.CharacterAvatarUrl.Valid {
		avatarURL = &message.CharacterAvatarUrl.String
	}

	result := &core.MessageWithDetails{
		Message: models.Message{
			ID:                    message.ID,
			GameID:                message.GameID,
			PhaseID:               message.PhaseID,
			AuthorID:              message.AuthorID,
			CharacterID:           message.CharacterID,
			Content:               message.Content,
			MessageType:           message.MessageType,
			ParentID:              message.ParentID,
			ThreadDepth:           message.ThreadDepth,
			Visibility:            message.Visibility,
			MentionedCharacterIds: message.MentionedCharacterIds,
			IsEdited:              message.IsEdited,
			IsDeleted:             message.IsDeleted,
			CreatedAt:             message.CreatedAt,
			DeletedAt:             message.DeletedAt,
		},
		AuthorUsername:     message.AuthorUsername,
		CharacterName:      message.CharacterName.String,
		CharacterAvatarUrl: avatarURL,
		ReplyCount:         message.ReplyCount,
	}

	return result, nil
}

// GetMessageWithParentContext retrieves a message plus up to maxParents nearest
// ancestors (ordered parent-to-child) and the true root post ID, in a single
// recursive query. Used for deep-linking to nested comments so the client avoids
// a per-level request waterfall.
func (s *MessageService) GetMessageWithParentContext(ctx context.Context, messageID int32, maxParents int32) (*core.MessageThreadContext, error) {
	queries := models.New(s.DB)

	rows, err := queries.GetMessageWithParentContext(ctx, models.GetMessageWithParentContextParams{
		MessageID:  messageID,
		MaxParents: maxParents,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get message with parent context: %w", err)
	}

	chain := make([]core.MessageWithDetails, len(rows))
	var rootPostID int32
	for i, row := range rows {
		var avatarURL *string
		if row.CharacterAvatarUrl.Valid {
			avatarURL = &row.CharacterAvatarUrl.String
		}

		chain[i] = core.MessageWithDetails{
			Message: models.Message{
				ID:                    row.ID,
				GameID:                row.GameID,
				PhaseID:               row.PhaseID,
				AuthorID:              row.AuthorID,
				CharacterID:           row.CharacterID,
				Content:               row.Content,
				MessageType:           row.MessageType,
				ParentID:              row.ParentID,
				ThreadDepth:           row.ThreadDepth,
				Visibility:            row.Visibility,
				MentionedCharacterIds: row.MentionedCharacterIds,
				IsEdited:              row.IsEdited,
				IsDeleted:             row.IsDeleted,
				CreatedAt:             row.CreatedAt,
				DeletedAt:             row.DeletedAt,
				DeletedByUserID:       row.DeletedByUserID,
				EditedAt:              row.EditedAt,
				EditCount:             row.EditCount,
			},
			AuthorUsername:     row.AuthorUsername,
			CharacterName:      row.CharacterName.String,
			CharacterAvatarUrl: avatarURL,
			ReplyCount:         row.ReplyCount,
		}

		// root_post_id is identical on every row; capture it once.
		if row.RootPostID.Valid {
			rootPostID = row.RootPostID.Int32
		}
	}

	// The chain reaches root when its first (shallowest) element is the root post.
	hasFullThread := len(chain) > 0 && !chain[0].ParentID.Valid

	return &core.MessageThreadContext{
		Chain:         chain,
		RootPostID:    rootPostID,
		HasFullThread: hasFullThread,
	}, nil
}

// GetPostComments retrieves direct child comments for a post or comment
func (s *MessageService) GetPostComments(ctx context.Context, parentID int32) ([]core.MessageWithDetails, error) {
	queries := models.New(s.DB)

	comments, err := queries.GetPostComments(ctx, int32ValueToPgInt4(parentID))
	if err != nil {
		return nil, fmt.Errorf("failed to get post comments: %w", err)
	}

	result := make([]core.MessageWithDetails, len(comments))
	for i, comment := range comments {
		var avatarURL *string
		if comment.CharacterAvatarUrl.Valid {
			avatarURL = &comment.CharacterAvatarUrl.String
		}
		result[i] = core.MessageWithDetails{
			Message: models.Message{
				ID:                    comment.ID,
				GameID:                comment.GameID,
				PhaseID:               comment.PhaseID,
				AuthorID:              comment.AuthorID,
				CharacterID:           comment.CharacterID,
				Content:               comment.Content,
				MessageType:           comment.MessageType,
				ParentID:              comment.ParentID,
				ThreadDepth:           comment.ThreadDepth,
				Visibility:            comment.Visibility,
				MentionedCharacterIds: comment.MentionedCharacterIds,
				IsEdited:              comment.IsEdited,
				IsDeleted:             comment.IsDeleted,
				CreatedAt:             comment.CreatedAt,
				DeletedAt:             comment.DeletedAt,
			},
			AuthorUsername:     comment.AuthorUsername,
			CharacterName:      comment.CharacterName.String,
			CharacterAvatarUrl: avatarURL,
			ReplyCount:         comment.ReplyCount,
		}
	}

	return result, nil
}

// GetPostCommentsWithThreads fetches paginated top-level comments with all nested replies up to maxDepth
// Uses raw SQL with recursive CTE since sqlc doesn't support it
// Returns flat array - frontend builds tree using parent_id relationships
func (s *MessageService) GetPostCommentsWithThreads(ctx context.Context, postID int32, limit int32, offset int32, maxDepth int32) ([]core.CommentWithDepth, error) {
	query := `
WITH RECURSIVE top_level_comments AS (
  -- Get IDs of paginated top-level comments (newest first)
  -- INCLUDES deleted comments to preserve thread structure
  SELECT id
  FROM messages
  WHERE messages.parent_id = $1
    AND messages.message_type = 'comment'
  ORDER BY messages.created_at DESC
  LIMIT $2 OFFSET $3
),
comment_tree AS (
  -- Base case: Get full details for paginated top-level comments
  SELECT
    m.id,
    m.game_id,
    m.phase_id,
    m.author_id,
    m.character_id,
    m.content,
    m.message_type,
    m.parent_id,
    m.thread_depth,
    m.visibility,
    m.mentioned_character_ids,
    m.is_edited,
    m.is_deleted,
    m.created_at,
    m.edited_at,
    m.edit_count,
    m.deleted_at,
    m.deleted_by_user_id,
    u.username as author_username,
    c.name as character_name,
    COALESCE(m.character_avatar_url_at_post, c.avatar_url) as character_avatar_url,
    0::int as depth
  FROM messages m
  JOIN top_level_comments tlc ON m.id = tlc.id
  JOIN users u ON m.author_id = u.id
  LEFT JOIN characters c ON m.character_id = c.id

  UNION ALL

  -- Recursive case: Get nested replies up to max_depth - 1
  -- This ensures comments at (maxDepth - 1) can have Reply buttons
  -- and "Continue thread" appears when they have deeper replies
  SELECT
    m.id,
    m.game_id,
    m.phase_id,
    m.author_id,
    m.character_id,
    m.content,
    m.message_type,
    m.parent_id,
    m.thread_depth,
    m.visibility,
    m.mentioned_character_ids,
    m.is_edited,
    m.is_deleted,
    m.created_at,
    m.edited_at,
    m.edit_count,
    m.deleted_at,
    m.deleted_by_user_id,
    u.username as author_username,
    c.name as character_name,
    COALESCE(m.character_avatar_url_at_post, c.avatar_url) as character_avatar_url,
    comment_tree.depth + 1 as depth
  FROM messages m
  JOIN comment_tree ON m.parent_id = comment_tree.id AND comment_tree.depth + 1 < $4
  JOIN users u ON m.author_id = u.id
  LEFT JOIN characters c ON m.character_id = c.id
  WHERE m.message_type = 'comment'
),
descendant_counts AS (
  -- Recursively count all descendants for each comment in the tree
  WITH RECURSIVE all_descendants AS (
    SELECT id as root_id, id as descendant_id FROM comment_tree
    UNION ALL
    SELECT ad.root_id, m.id
    FROM all_descendants ad
    JOIN messages m ON m.parent_id = ad.descendant_id
    WHERE m.message_type = 'comment'
  )
  SELECT root_id as comment_id, COUNT(*) - 1 as reply_count
  FROM all_descendants
  GROUP BY root_id
)
SELECT ct.*, COALESCE(dc.reply_count, 0)::bigint as reply_count
FROM comment_tree ct
LEFT JOIN descendant_counts dc ON ct.id = dc.comment_id
ORDER BY ct.created_at DESC`

	rows, err := s.DB.Query(ctx, query, postID, limit, offset, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("failed to execute recursive comment query: %w", err)
	}
	defer rows.Close()

	var results []core.CommentWithDepth
	for rows.Next() {
		var (
			id                    int32
			gameID                int32
			phaseID               pgtype.Int4
			authorID              int32
			characterID           int32
			content               string
			messageType           string
			parentID              pgtype.Int4
			threadDepth           int32
			visibility            string
			mentionedCharacterIds []int32
			isEdited              bool
			isDeleted             bool
			createdAt             pgtype.Timestamp
			editedAt              pgtype.Timestamptz
			editCount             int32
			deletedAt             pgtype.Timestamp
			deletedByUserID       pgtype.Int4
			authorUsername        string
			characterName         *string
			characterAvatarURL    *string
			depth                 int32
			replyCount            int64
		)

		err := rows.Scan(
			&id, &gameID, &phaseID, &authorID, &characterID,
			&content, &messageType, &parentID, &threadDepth, &visibility,
			&mentionedCharacterIds, &isEdited, &isDeleted,
			&createdAt, &editedAt, &editCount, &deletedAt, &deletedByUserID,
			&authorUsername, &characterName, &characterAvatarURL,
			&depth, &replyCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan comment row: %w", err)
		}

		commentWithDepth := core.CommentWithDepth{
			Comment: core.MessageWithDetails{
				Message: models.Message{
					ID:                    id,
					GameID:                gameID,
					PhaseID:               phaseID,
					AuthorID:              authorID,
					CharacterID:           characterID,
					Content:               content,
					MessageType:           models.MessageType(messageType),
					ParentID:              parentID,
					ThreadDepth:           threadDepth,
					Visibility:            models.MessageVisibility(visibility),
					MentionedCharacterIds: mentionedCharacterIds,
					IsEdited:              isEdited,
					IsDeleted:             isDeleted,
					CreatedAt:             createdAt,
					DeletedAt:             deletedAt,
					DeletedByUserID:       deletedByUserID,
					EditedAt:              editedAt,
					EditCount:             editCount,
				},
				AuthorUsername: authorUsername,
				CharacterName: func() string {
					if characterName != nil {
						return *characterName
					}
					return ""
				}(),
				CharacterAvatarUrl: characterAvatarURL,
				ReplyCount:         replyCount,
			},
			Depth: depth,
		}

		results = append(results, commentWithDepth)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating comment rows: %w", err)
	}

	return results, nil
}

// CountTopLevelComments returns the total count of top-level comments for a post
func (s *MessageService) CountTopLevelComments(ctx context.Context, postID int32) (int64, error) {
	queries := models.New(s.DB)
	count, err := queries.CountTopLevelComments(ctx, int32ValueToPgInt4(postID))
	if err != nil {
		return 0, fmt.Errorf("failed to count top-level comments: %w", err)
	}
	return count, nil
}

// UpdateComment updates the content and optionally the character of an existing comment
func (s *MessageService) UpdateComment(ctx context.Context, commentID int32, content string, newCharacterID *int32) (*models.Message, error) {
	queries := models.New(s.DB)

	// Get the existing comment to compare mentions
	existingComment, err := queries.GetComment(ctx, commentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing comment: %w", err)
	}

	// Cannot edit deleted comments
	if existingComment.IsDeleted {
		return nil, fmt.Errorf("cannot edit deleted comment")
	}

	// Determine which character ID to use
	characterIDToUse := existingComment.CharacterID
	if newCharacterID != nil {
		characterIDToUse = *newCharacterID
	}

	// If character is being changed, validate ownership of new character
	if newCharacterID != nil && *newCharacterID != existingComment.CharacterID {
		// Validate user can control the new character
		if err := s.ValidateCharacterOwnership(ctx, *newCharacterID, existingComment.AuthorID, existingComment.GameID); err != nil {
			return nil, core.ErrCharacterNotControlled
		}
	}

	// Extract character mentions from new content
	mentionedIDs, err := s.extractCharacterMentions(ctx, content, existingComment.GameID)
	if err != nil {
		// Log error but don't fail the update
		// Mention extraction is a non-critical feature
		s.Logger.LogError(ctx, err, "Failed to extract mentions during comment update",
			"comment_id", commentID,
			"game_id", existingComment.GameID,
		)
		mentionedIDs = []int32{}
	}

	// Update the comment with new content, character, and mentions
	message, err := queries.UpdateComment(ctx, models.UpdateCommentParams{
		ID:                    commentID,
		Content:               content,
		CharacterID:           characterIDToUse,
		MentionedCharacterIds: mentionedIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update comment: %w", err)
	}

	// Compare old mentions vs new mentions to find newly added ones
	oldMentions := make(map[int32]bool)
	for _, id := range existingComment.MentionedCharacterIds {
		oldMentions[id] = true
	}

	newMentions := make([]int32, 0)
	for _, id := range mentionedIDs {
		if !oldMentions[id] {
			newMentions = append(newMentions, id)
		}
	}

	// Determine which character to use for notifications (new character if changed)
	notificationCharacterID := existingComment.CharacterID
	if newCharacterID != nil && *newCharacterID != existingComment.CharacterID {
		notificationCharacterID = *newCharacterID
		s.Logger.Info(ctx, "Comment character swapped",
			"comment_id", commentID,
			"old_character_id", existingComment.CharacterID,
			"new_character_id", *newCharacterID,
			"game_id", existingComment.GameID,
		)
	}

	// Trigger notifications for NEW mentions only (fire-and-forget)
	if len(newMentions) > 0 {
		s.Logger.Info(ctx, "Comment updated successfully",
			"comment_id", commentID,
			"user_id", existingComment.AuthorID,
			"edit_count", message.EditCount,
			"new_mentions", len(newMentions),
			"game_id", existingComment.GameID,
		)
		observability.SafeGo(context.Background(), s.Logger, "notify-character-mentions", func() {
			s.notifyCharacterMentions(context.Background(), newMentions, notificationCharacterID, existingComment.AuthorID, existingComment.GameID, message.ID)
		})
	} else {
		s.Logger.Info(ctx, "Comment updated successfully",
			"comment_id", commentID,
			"user_id", existingComment.AuthorID,
			"edit_count", message.EditCount,
			"game_id", existingComment.GameID,
		)
	}

	return &message, nil
}

// DeleteComment soft-deletes a comment (preserves thread structure)
// deleterID: the user performing the deletion (could be author, GM, or admin)
func (s *MessageService) DeleteComment(ctx context.Context, commentID int32, deleterID int32) error {
	queries := models.New(s.DB)

	// Check if comment is already deleted
	comment, err := queries.GetComment(ctx, commentID)
	if err != nil {
		return fmt.Errorf("failed to get comment: %w", err)
	}

	if comment.IsDeleted {
		return fmt.Errorf("cannot delete already deleted comment")
	}

	err = queries.DeleteComment(ctx, models.DeleteCommentParams{
		ID:              commentID,
		DeletedByUserID: int32ValueToPgInt4(deleterID),
	})
	if err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	return nil
}

// GetPostCommentCount returns total comment count for a post
func (s *MessageService) GetPostCommentCount(ctx context.Context, postID int32) (int64, error) {
	queries := models.New(s.DB)

	count, err := queries.GetPostCommentCount(ctx, int32ValueToPgInt4(postID))
	if err != nil {
		return 0, fmt.Errorf("failed to get post comment count: %w", err)
	}

	return count, nil
}

// CanUserEditComment checks if a user can edit a comment (must be author)
func (s *MessageService) CanUserEditComment(ctx context.Context, commentID int32, userID int32) (bool, error) {
	queries := models.New(s.DB)

	comment, err := queries.CheckCommentOwnership(ctx, commentID)
	if err != nil {
		return false, fmt.Errorf("failed to check comment ownership: %w", err)
	}

	// Cannot edit deleted comments
	if comment.DeletedAt.Valid {
		return false, nil
	}

	// Only the author can edit
	return comment.AuthorID == userID, nil
}

// CanUserDeleteComment checks if a user can delete a comment
// Users who can delete: author, GM of the game, or admin (when in admin mode)
func (s *MessageService) CanUserDeleteComment(ctx context.Context, commentID int32, userID int32, isAdmin bool) (bool, error) {
	queries := models.New(s.DB)

	comment, err := queries.CheckCommentOwnership(ctx, commentID)
	if err != nil {
		return false, fmt.Errorf("failed to check comment ownership: %w", err)
	}

	// Cannot delete already deleted comments
	if comment.DeletedAt.Valid {
		return false, nil
	}

	// Author can always delete
	if comment.AuthorID == userID {
		return true, nil
	}

	// Get the full comment to access game_id
	fullComment, err := queries.GetComment(ctx, commentID)
	if err != nil {
		return false, fmt.Errorf("failed to get comment details: %w", err)
	}

	// Check if user is the GM or Co-GM of the game
	game, err := queries.GetGame(ctx, fullComment.GameID)
	if err != nil {
		return false, fmt.Errorf("failed to get game: %w", err)
	}

	if game.GmUserID == userID || core.IsUserCoGM(ctx, s.DB, fullComment.GameID, userID) {
		return true, nil
	}

	// Admin with admin mode enabled can delete
	if isAdmin {
		return true, nil
	}

	return false, nil
}

// recentCommentRow is the field set shared by the read and unread variants of the
// recent-comments query. sqlc generates a distinct (but identical) struct per query,
// so both are normalized through this type before conversion to the domain model.
type recentCommentRow = models.ListRecentCommentsWithParentsRow

// recentCommentRowToDomain converts a recent-comments query row to the domain model.
func recentCommentRowToDomain(row recentCommentRow) core.CommentWithParent {
	return core.CommentWithParent{
		// Comment data
		ID:                 row.ID,
		GameID:             row.GameID,
		ParentID:           pgInt4ToInt32Ptr(row.ParentID),
		PostID:             pgInt4ToInt32Ptr(row.PostID),
		AuthorID:           row.AuthorID,
		CharacterID:        row.CharacterID,
		Content:            row.Content,
		CreatedAt:          pgTimestampToTime(row.CreatedAt),
		EditedAt:           pgTimestamptzToTimePtr(row.EditedAt),
		EditCount:          row.EditCount,
		DeletedAt:          pgTimestampToTimePtr(row.DeletedAt),
		IsDeleted:          row.IsDeleted,
		AuthorUsername:     row.AuthorUsername,
		CharacterName:      pgTextToStringPtr(row.CharacterName),
		CharacterAvatarUrl: pgTextToStringPtr(row.CharacterAvatarUrl),

		// Parent data
		ParentContent:            pgTextToStringPtr(row.ParentContent),
		ParentCreatedAt:          pgTimestampToTimePtr(row.ParentCreatedAt),
		ParentDeletedAt:          pgTimestampToTimePtr(row.ParentDeletedAt),
		ParentIsDeleted:          pgBoolToBoolPtr(row.ParentIsDeleted),
		ParentMessageType:        nullMessageTypeToStringPtr(row.ParentMessageType),
		ParentAuthorUsername:     pgTextToStringPtr(row.ParentAuthorUsername),
		ParentCharacterName:      pgTextToStringPtr(row.ParentCharacterName),
		ParentCharacterAvatarUrl: pgTextToStringPtr(row.ParentCharacterAvatarUrl),
	}
}

// ListRecentCommentsWithParents retrieves recent comments with their parent messages/posts
// for the "New Comments" view. Supports pagination via limit/offset.
func (s *MessageService) ListRecentCommentsWithParents(ctx context.Context, gameID int32, limit, offset int32) ([]core.CommentWithParent, error) {
	queries := models.New(s.DB)

	// Call the generated sqlc method
	rows, err := queries.ListRecentCommentsWithParents(ctx, models.ListRecentCommentsWithParentsParams{
		GameID: gameID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list recent comments with parents: %w", err)
	}

	comments := make([]core.CommentWithParent, len(rows))
	for i, row := range rows {
		comments[i] = recentCommentRowToDomain(row)
	}

	s.Logger.Info(ctx, "Listed recent comments with parents",
		"game_id", gameID,
		"limit", limit,
		"offset", offset,
		"count", len(comments),
	)

	return comments, nil
}

// ListRecentUnreadCommentsWithParents retrieves recent comments with their parent
// messages/posts, omitting comments the user has manually marked as read.
// Backs the "New Comments" view's unread-only filter in manual read mode.
func (s *MessageService) ListRecentUnreadCommentsWithParents(ctx context.Context, gameID, userID int32, limit, offset int32) ([]core.CommentWithParent, error) {
	queries := models.New(s.DB)

	rows, err := queries.ListRecentUnreadCommentsWithParents(ctx, models.ListRecentUnreadCommentsWithParentsParams{
		GameID: gameID,
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list recent unread comments with parents: %w", err)
	}

	comments := make([]core.CommentWithParent, len(rows))
	for i, row := range rows {
		comments[i] = recentCommentRowToDomain(recentCommentRow{
			ID:                       row.ID,
			GameID:                   row.GameID,
			ParentID:                 row.ParentID,
			PostID:                   row.PostID,
			AuthorID:                 row.AuthorID,
			CharacterID:              row.CharacterID,
			Content:                  row.Content,
			CreatedAt:                row.CreatedAt,
			EditedAt:                 row.EditedAt,
			EditCount:                row.EditCount,
			DeletedAt:                row.DeletedAt,
			IsDeleted:                row.IsDeleted,
			AuthorUsername:           row.AuthorUsername,
			CharacterName:            row.CharacterName,
			CharacterAvatarUrl:       row.CharacterAvatarUrl,
			ParentContent:            row.ParentContent,
			ParentCreatedAt:          row.ParentCreatedAt,
			ParentDeletedAt:          row.ParentDeletedAt,
			ParentIsDeleted:          row.ParentIsDeleted,
			ParentMessageType:        row.ParentMessageType,
			ParentAuthorUsername:     row.ParentAuthorUsername,
			ParentCharacterName:      row.ParentCharacterName,
			ParentCharacterAvatarUrl: row.ParentCharacterAvatarUrl,
		})
	}

	s.Logger.Info(ctx, "Listed recent unread comments with parents",
		"game_id", gameID,
		"user_id", userID,
		"limit", limit,
		"offset", offset,
		"count", len(comments),
	)

	return comments, nil
}

// GetTotalCommentCount returns the total count of non-deleted comments in a game
func (s *MessageService) GetTotalCommentCount(ctx context.Context, gameID int32) (int64, error) {
	queries := models.New(s.DB)

	count, err := queries.GetTotalCommentCount(ctx, gameID)
	if err != nil {
		return 0, fmt.Errorf("failed to get total comment count: %w", err)
	}

	return count, nil
}

// GetTotalUnreadCommentCount returns the count of non-deleted comments in a game
// that the user has not manually marked as read
func (s *MessageService) GetTotalUnreadCommentCount(ctx context.Context, gameID, userID int32) (int64, error) {
	queries := models.New(s.DB)

	count, err := queries.GetTotalUnreadCommentCount(ctx, models.GetTotalUnreadCommentCountParams{
		GameID: gameID,
		UserID: userID,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get total unread comment count: %w", err)
	}

	return count, nil
}
