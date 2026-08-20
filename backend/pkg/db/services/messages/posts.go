package messages

import (
	"context"
	"fmt"

	core "actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	db "actionphase/pkg/db/services"
	"actionphase/pkg/observability"
	"actionphase/pkg/validation"
)

// CreatePost creates a new top-level message post
func (s *MessageService) CreatePost(ctx context.Context, req core.CreatePostRequest) (*models.Message, error) {
	queries := models.New(s.DB)

	// Validate game is not completed/cancelled (archived games are read-only)
	game, err := queries.GetGame(ctx, req.GameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get game: %w", err)
	}

	if err := core.ValidateGameNotCompleted(ctx, &game); err != nil {
		return nil, err
	}

	// Validate character ownership before creating post
	if err := s.ValidateCharacterOwnership(ctx, req.CharacterID, req.AuthorID, req.GameID); err != nil {
		return nil, fmt.Errorf("character validation failed: %w", err)
	}

	// Validate content length
	if err := validation.ValidatePost(req.Content); err != nil {
		return nil, err
	}

	// Extract character mentions from content
	mentionedIDs, err := s.extractCharacterMentions(ctx, req.Content, req.GameID)
	if err != nil {
		// Log error but don't fail the post creation
		// Mention extraction is a non-critical feature
		mentionedIDs = []int32{}
	}

	// Create the post using sqlc-generated query
	message, err := queries.CreatePost(ctx, models.CreatePostParams{
		GameID:                req.GameID,
		PhaseID:               int32ToPgInt4(req.PhaseID),
		AuthorID:              req.AuthorID,
		CharacterID:           req.CharacterID,
		Content:               req.Content,
		Visibility:            models.MessageVisibility(req.Visibility),
		MentionedCharacterIds: mentionedIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create post: %w", err)
	}

	s.Logger.Info(ctx, "Post created",
		"post_id", message.ID,
		"game_id", message.GameID,
		"phase_id", message.PhaseID,
	)
	s.Metrics.RecordPostCreated(ctx)

	// Preserve context values (correlation_id, trace_id) without inheriting cancellation
	notifCtx := context.WithoutCancel(ctx)

	// Trigger notifications for character mentions (fire-and-forget)
	if len(mentionedIDs) > 0 {
		observability.SafeGo(notifCtx, s.Logger, "notify-character-mentions", func() {
			s.notifyCharacterMentions(notifCtx, mentionedIDs, req.CharacterID, req.AuthorID, req.GameID, message.ID)
		})
	}

	// Notify all game participants about the new GM post (fire-and-forget)
	observability.SafeGo(notifCtx, s.Logger, "notify-common-room-post", func() {
		notifSvc := db.NewNotificationService(s.DB, s.Logger)
		if err := notifSvc.NotifyCommonRoomPost(notifCtx, req.GameID, message.ID, truncatePostTitle(req.Content), req.AuthorID); err != nil {
			s.Logger.LogError(notifCtx, err, "Failed to notify common room post", "game_id", req.GameID, "post_id", message.ID)
			s.Metrics.RecordBackgroundJobFailure(notifCtx, "post_notification")
		} else {
			s.Logger.Debug(notifCtx, "Common room post notification sent", "game_id", req.GameID, "post_id", message.ID)
		}
	})

	return &message, nil
}

// truncatePostTitle returns the first 60 characters of a post for use as a notification title.
func truncatePostTitle(content string) string {
	const maxLen = 60
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "…"
}

// GetPost retrieves a specific post by ID with metadata
func (s *MessageService) GetPost(ctx context.Context, postID int32) (*core.MessageWithDetails, error) {
	queries := models.New(s.DB)

	post, err := queries.GetPost(ctx, postID)
	if err != nil {
		return nil, fmt.Errorf("failed to get post: %w", err)
	}

	// Get recursive comment count
	totalComments, err := s.GetRecursiveCommentCount(ctx, postID)
	if err != nil {
		return nil, fmt.Errorf("failed to get comment count: %w", err)
	}

	var avatarURL *string
	if post.CharacterAvatarUrl.Valid {
		avatarURL = &post.CharacterAvatarUrl.String
	}

	return &core.MessageWithDetails{
		Message: models.Message{
			ID:          post.ID,
			GameID:      post.GameID,
			PhaseID:     post.PhaseID,
			AuthorID:    post.AuthorID,
			CharacterID: post.CharacterID,
			Content:     post.Content,
			MessageType: post.MessageType,
			ParentID:    post.ParentID,
			ThreadDepth: post.ThreadDepth,
			Visibility:  post.Visibility,
			IsEdited:    post.IsEdited,
			IsDeleted:   post.IsDeleted,
			CreatedAt:   post.CreatedAt,
			DeletedAt:   post.DeletedAt,
		},
		AuthorUsername:     post.AuthorUsername,
		CharacterName:      post.CharacterName.String,
		CharacterAvatarUrl: avatarURL,
		CommentCount:       totalComments,
	}, nil
}

// GetGamePosts retrieves posts for a game, optionally filtered by phase
func (s *MessageService) GetGamePosts(ctx context.Context, gameID int32, phaseID *int32, limit, offset int32) ([]core.MessageWithDetails, error) {
	queries := models.New(s.DB)

	// Convert phaseID to int32 for use with Column2 parameter
	// If nil, pass 0 to get all posts (CASE WHEN 0 THEN TRUE)
	phaseIDValue := int32(0)
	if phaseID != nil {
		phaseIDValue = *phaseID
	}

	posts, err := queries.GetGamePosts(ctx, models.GetGamePostsParams{
		GameID:  gameID,
		Column2: phaseIDValue,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get game posts: %w", err)
	}

	result := make([]core.MessageWithDetails, len(posts))
	for i, post := range posts {
		// Get recursive comment count for this post
		totalComments, err := s.GetRecursiveCommentCount(ctx, post.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get comment count for post %d: %w", post.ID, err)
		}

		var avatarURL *string
		if post.CharacterAvatarUrl.Valid {
			avatarURL = &post.CharacterAvatarUrl.String
		}

		result[i] = core.MessageWithDetails{
			Message: models.Message{
				ID:                    post.ID,
				GameID:                post.GameID,
				PhaseID:               post.PhaseID,
				AuthorID:              post.AuthorID,
				CharacterID:           post.CharacterID,
				Content:               post.Content,
				MessageType:           post.MessageType,
				ParentID:              post.ParentID,
				ThreadDepth:           post.ThreadDepth,
				Visibility:            post.Visibility,
				MentionedCharacterIds: post.MentionedCharacterIds,
				IsEdited:              post.IsEdited,
				IsDeleted:             post.IsDeleted,
				CreatedAt:             post.CreatedAt,
				DeletedAt:             post.DeletedAt,
			},
			AuthorUsername:     post.AuthorUsername,
			CharacterName:      post.CharacterName.String,
			CharacterAvatarUrl: avatarURL,
			CommentCount:       totalComments,
		}
	}

	return result, nil
}

// GetPhasePosts retrieves all posts for a specific phase
func (s *MessageService) GetPhasePosts(ctx context.Context, phaseID int32) ([]core.MessageWithDetails, error) {
	queries := models.New(s.DB)

	posts, err := queries.GetPhasePosts(ctx, int32ValueToPgInt4(phaseID))
	if err != nil {
		return nil, fmt.Errorf("failed to get phase posts: %w", err)
	}

	result := make([]core.MessageWithDetails, len(posts))
	for i, post := range posts {
		var avatarURL *string
		if post.CharacterAvatarUrl.Valid {
			avatarURL = &post.CharacterAvatarUrl.String
		}

		result[i] = core.MessageWithDetails{
			Message: models.Message{
				ID:          post.ID,
				GameID:      post.GameID,
				PhaseID:     post.PhaseID,
				AuthorID:    post.AuthorID,
				CharacterID: post.CharacterID,
				Content:     post.Content,
				MessageType: post.MessageType,
				ParentID:    post.ParentID,
				ThreadDepth: post.ThreadDepth,
				Visibility:  post.Visibility,
				IsEdited:    post.IsEdited,
				IsDeleted:   post.IsDeleted,
				CreatedAt:   post.CreatedAt,
				DeletedAt:   post.DeletedAt,
			},
			AuthorUsername:     post.AuthorUsername,
			CharacterName:      post.CharacterName.String,
			CharacterAvatarUrl: avatarURL,
			CommentCount:       post.CommentCount,
		}
	}

	return result, nil
}

// UpdatePost updates the content of an existing post
func (s *MessageService) UpdatePost(ctx context.Context, postID int32, content string) (*models.Message, error) {
	queries := models.New(s.DB)

	row, err := queries.UpdatePost(ctx, models.UpdatePostParams{
		ID:      postID,
		Content: content,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update post: %w", err)
	}

	// Convert UpdatePostRow to Message (base fields only)
	// The handler will fetch full details with joined fields via GetPost
	// TODO: Consider refactoring to return *core.MessageWithDetails directly to avoid double DB hit
	// Currently: UpdatePost query includes JOINs but we discard joined fields, then GetPost re-fetches them
	message := &models.Message{
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
	}

	return message, nil
}

// DeletePost soft-deletes a post (preserves thread structure)
func (s *MessageService) DeletePost(ctx context.Context, postID int32) error {
	queries := models.New(s.DB)

	deleted, err := queries.DeletePost(ctx, postID)
	if err != nil {
		return fmt.Errorf("failed to delete post: %w", err)
	}

	s.Logger.Info(ctx, "Post deleted",
		"post_id", deleted.ID,
		"game_id", deleted.GameID,
	)

	return nil
}

// GetGamePostCount returns the count of posts for a game, optionally filtered by phase
func (s *MessageService) GetGamePostCount(ctx context.Context, gameID int32, phaseID *int32) (int64, error) {
	queries := models.New(s.DB)

	// Convert phaseID to int32 for use with Column2 parameter
	// If nil, pass 0 to get all posts (CASE WHEN 0 THEN TRUE)
	phaseIDValue := int32(0)
	if phaseID != nil {
		phaseIDValue = *phaseID
	}

	count, err := queries.GetGamePostCount(ctx, models.GetGamePostCountParams{
		GameID:  gameID,
		Column2: phaseIDValue,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get game post count: %w", err)
	}

	return count, nil
}

// GetUserPostsInGame retrieves all posts by a user in a game
func (s *MessageService) GetUserPostsInGame(ctx context.Context, gameID, userID int32) ([]core.MessageWithDetails, error) {
	queries := models.New(s.DB)

	posts, err := queries.GetUserPostsInGame(ctx, models.GetUserPostsInGameParams{
		GameID:   gameID,
		AuthorID: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user posts in game: %w", err)
	}

	result := make([]core.MessageWithDetails, len(posts))
	for i, post := range posts {
		var avatarURL *string
		if post.CharacterAvatarUrl.Valid {
			avatarURL = &post.CharacterAvatarUrl.String
		}

		result[i] = core.MessageWithDetails{
			Message: models.Message{
				ID:          post.ID,
				GameID:      post.GameID,
				PhaseID:     post.PhaseID,
				AuthorID:    post.AuthorID,
				CharacterID: post.CharacterID,
				Content:     post.Content,
				MessageType: post.MessageType,
				ParentID:    post.ParentID,
				ThreadDepth: post.ThreadDepth,
				Visibility:  post.Visibility,
				IsEdited:    post.IsEdited,
				IsDeleted:   post.IsDeleted,
				CreatedAt:   post.CreatedAt,
				DeletedAt:   post.DeletedAt,
			},
			AuthorUsername:     post.AuthorUsername,
			CharacterName:      post.CharacterName.String,
			CharacterAvatarUrl: avatarURL,
			CommentCount:       post.CommentCount,
		}
	}

	return result, nil
}

// CanUserEditPost checks if a user can edit a post (must be author)
func (s *MessageService) CanUserEditPost(ctx context.Context, postID int32, userID int32) (bool, error) {
	queries := models.New(s.DB)

	post, err := queries.CheckPostOwnership(ctx, postID)
	if err != nil {
		return false, fmt.Errorf("failed to check post ownership: %w", err)
	}

	// Cannot edit deleted posts
	if post.DeletedAt.Valid {
		return false, nil
	}

	// Only the author can edit
	return post.AuthorID == userID, nil
}
