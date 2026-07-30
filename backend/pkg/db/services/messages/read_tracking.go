package messages

import (
	"context"
	"fmt"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"github.com/jackc/pgx/v5/pgtype"
)

// MarkPostAsRead marks a post (and optionally a specific comment) as read by a user.
// This is used to track read status for the common room.
//
// Parameters:
//   - ctx: Request context
//   - userID: The user who is marking content as read
//   - gameID: The game the post belongs to
//   - postID: The post being marked as read
//   - lastReadCommentID: Optional - the most recent comment read (nil if just marking post as read)
//
// Returns:
//   - *core.ReadMarker: The updated read marker
//   - error: Any error that occurred
func (s *MessageService) MarkPostAsRead(ctx context.Context, userID, gameID, postID int32, lastReadCommentID *int32) (*core.ReadMarker, error) {
	queries := models.New(s.DB)

	// Build the upsert params
	params := models.MarkPostReadParams{
		UserID:            userID,
		GameID:            gameID,
		PostID:            postID,
		LastReadCommentID: int32ToPgInt4(lastReadCommentID),
	}

	// Execute the upsert
	readMarker, err := queries.MarkPostRead(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to mark post as read: %w", err)
	}

	// Convert to core model
	return readMarkerToCore(&readMarker), nil
}

// GetUserReadMarker retrieves the read tracking info for a specific user and post.
//
// Parameters:
//   - ctx: Request context
//   - userID: The user ID
//   - postID: The post ID
//
// Returns:
//   - *core.ReadMarker: The read marker if found, nil if not found
//   - error: Any error that occurred (except ErrNotFound)
func (s *MessageService) GetUserReadMarker(ctx context.Context, userID, postID int32) (*core.ReadMarker, error) {
	queries := models.New(s.DB)

	params := models.GetUserReadMarkerParams{
		UserID: userID,
		PostID: postID,
	}

	readMarker, err := queries.GetUserReadMarker(ctx, params)
	if err != nil {
		// If not found, return nil marker (user hasn't read this post yet)
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get read marker: %w", err)
	}

	return readMarkerToCore(&readMarker), nil
}

// GetUserReadMarkersForGame retrieves all read markers for a user in a specific game.
// This is used to batch-check which posts have unread content.
//
// Parameters:
//   - ctx: Request context
//   - userID: The user ID
//   - gameID: The game ID
//
// Returns:
//   - []*core.ReadMarker: List of read markers for the user/game
//   - error: Any error that occurred
func (s *MessageService) GetUserReadMarkersForGame(ctx context.Context, userID, gameID int32) ([]*core.ReadMarker, error) {
	queries := models.New(s.DB)

	params := models.GetUserReadMarkersForGameParams{
		UserID: userID,
		GameID: gameID,
	}

	readMarkers, err := queries.GetUserReadMarkersForGame(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get read markers for game: %w", err)
	}

	// Convert to core models
	result := make([]*core.ReadMarker, len(readMarkers))
	for i, marker := range readMarkers {
		result[i] = readMarkerToCore(&marker)
	}

	return result, nil
}

// GetPostsWithUnreadInfo retrieves posts with their total comment count and latest comment timestamp.
// Frontend will compare these with read markers to determine unread status.
//
// Parameters:
//   - ctx: Request context
//   - gameID: The game ID
//
// Returns:
//   - []*core.PostUnreadInfo: List of post unread info
//   - error: Any error that occurred
func (s *MessageService) GetPostsWithUnreadInfo(ctx context.Context, gameID int32) ([]*core.PostUnreadInfo, error) {
	queries := models.New(s.DB)

	rows, err := queries.GetPostsWithUnreadCount(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get posts with unread info: %w", err)
	}

	// Convert to core models
	result := make([]*core.PostUnreadInfo, len(rows))
	for i, row := range rows {
		info := &core.PostUnreadInfo{
			PostID:        row.PostID,
			PostCreatedAt: row.PostCreatedAt.Time,
			TotalComments: row.TotalComments,
		}

		// Handle nullable latest comment timestamp
		// LatestCommentAt is interface{} and can be nil or pgtype.Timestamptz
		if row.LatestCommentAt != nil {
			if ts, ok := row.LatestCommentAt.(pgtype.Timestamptz); ok && ts.Valid {
				info.LatestCommentAt = &ts.Time
			}
		}

		result[i] = info
	}

	return result, nil
}

// GetUnreadCommentIDsForPosts retrieves the IDs of comments that are "new since last visit"
// for each post in a game. A comment is considered new if it was created after the user's
// last_read_at timestamp for that post.
//
// Parameters:
//   - ctx: Request context
//   - userID: The user ID
//   - gameID: The game ID
//
// Returns:
//   - []*core.PostUnreadComments: List of posts with their unread comment IDs
//   - error: Any error that occurred
func (s *MessageService) GetUnreadCommentIDsForPosts(ctx context.Context, userID, gameID int32) ([]*core.PostUnreadComments, error) {
	queries := models.New(s.DB)

	params := models.GetUnreadCommentIDsForPostsParams{
		UserID: userID,
		GameID: gameID,
	}

	rows, err := queries.GetUnreadCommentIDsForPosts(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get unread comment IDs: %w", err)
	}

	// Convert to core models
	result := make([]*core.PostUnreadComments, len(rows))
	for i, row := range rows {
		unreadIDs := []int32{}

		// The UnreadCommentIds field is interface{} representing a PostgreSQL array
		// It can be []interface{} or other types depending on the driver
		if row.UnreadCommentIds != nil {
			switch ids := row.UnreadCommentIds.(type) {
			case []interface{}:
				for _, id := range ids {
					if intID, ok := id.(int64); ok {
						unreadIDs = append(unreadIDs, int32(intID))
					} else if intID, ok := id.(int32); ok {
						unreadIDs = append(unreadIDs, intID)
					}
				}
			case []int32:
				unreadIDs = ids
			case []int64:
				for _, id := range ids {
					unreadIDs = append(unreadIDs, int32(id))
				}
			}
		}

		result[i] = &core.PostUnreadComments{
			PostID:           row.PostID,
			UnreadCommentIDs: unreadIDs,
		}
	}

	return result, nil
}

// ToggleCommentRead marks or unmarks a single comment as manually read by a user.
// Validates that the comment actually belongs to the specified post and game
// before writing to prevent users from toggling arbitrary comment IDs.
//
// Parameters:
//   - ctx: Request context
//   - userID: The user toggling the read state
//   - gameID: The game the comment belongs to
//   - postID: The post the comment belongs to
//   - commentID: The comment being toggled
//   - markAsRead: true to mark as read, false to mark as unread
func (s *MessageService) ToggleCommentRead(ctx context.Context, userID, gameID, postID, commentID int32, markAsRead bool) error {
	queries := models.New(s.DB)

	// Validate the comment belongs to the specified game to prevent cross-game manipulation
	msg, err := queries.GetMessage(ctx, commentID)
	if err != nil {
		return fmt.Errorf("comment not found: %w", err)
	}
	if msg.GameID != gameID {
		return fmt.Errorf("comment does not belong to this game")
	}

	if markAsRead {
		return queries.MarkCommentRead(ctx, models.MarkCommentReadParams{
			UserID:    userID,
			CommentID: commentID,
			PostID:    postID,
			GameID:    gameID,
		})
	}
	return queries.UnmarkCommentRead(ctx, models.UnmarkCommentReadParams{
		UserID:    userID,
		CommentID: commentID,
	})
}

// GetManualReadCommentIDsForGame retrieves all comment IDs manually marked as read
// by a user across all posts in a game. Results are grouped by post ID.
//
// Parameters:
//   - ctx: Request context
//   - userID: The user ID
//   - gameID: The game ID
//
// Returns:
//   - []*core.ManualCommentReads: Per-post lists of manually read comment IDs
//   - error: Any error that occurred
func (s *MessageService) GetManualReadCommentIDsForGame(ctx context.Context, userID, gameID int32) ([]*core.ManualCommentReads, error) {
	queries := models.New(s.DB)

	rows, err := queries.GetManualReadCommentIDsForGame(ctx, models.GetManualReadCommentIDsForGameParams{
		UserID: userID,
		GameID: gameID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get manual read comment IDs: %w", err)
	}

	// Group by post_id
	postMap := make(map[int32]*core.ManualCommentReads)
	for _, row := range rows {
		entry, ok := postMap[row.PostID]
		if !ok {
			entry = &core.ManualCommentReads{PostID: row.PostID, ReadCommentIDs: []int32{}}
			postMap[row.PostID] = entry
		}
		entry.ReadCommentIDs = append(entry.ReadCommentIDs, row.CommentID)
	}

	result := make([]*core.ManualCommentReads, 0, len(postMap))
	for _, v := range postMap {
		result = append(result, v)
	}
	return result, nil
}

// DeleteManualCommentReadsForGame deletes all manual read records for a game.
// Should be called when a game transitions to completed/archived status.
func (s *MessageService) DeleteManualCommentReadsForGame(ctx context.Context, gameID int32) error {
	queries := models.New(s.DB)
	if err := queries.DeleteManualCommentReadsForGame(ctx, gameID); err != nil {
		return fmt.Errorf("failed to delete manual comment reads for game: %w", err)
	}
	return nil
}

// MarkAllCommentsReadForPhase marks every comment in a phase as manually read by a user.
// Idempotent - comments already marked read are left untouched.
//
// Parameters:
//   - ctx: Request context
//   - userID: The user marking content as read
//   - gameID: The game the phase belongs to
//   - phaseID: The phase whose comments should be marked read
func (s *MessageService) MarkAllCommentsReadForPhase(ctx context.Context, userID, gameID, phaseID int32) error {
	queries := models.New(s.DB)
	err := queries.MarkAllCommentsReadForPhase(ctx, models.MarkAllCommentsReadForPhaseParams{
		UserID:  userID,
		GameID:  gameID,
		PhaseID: pgtype.Int4{Int32: phaseID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to mark all comments read for phase: %w", err)
	}
	return nil
}

// Helper function to convert DB read marker to core model
func readMarkerToCore(dbMarker *models.UserCommonRoomRead) *core.ReadMarker {
	marker := &core.ReadMarker{
		ID:         dbMarker.ID,
		UserID:     dbMarker.UserID,
		GameID:     dbMarker.GameID,
		PostID:     dbMarker.PostID,
		LastReadAt: dbMarker.LastReadAt.Time,
		CreatedAt:  dbMarker.CreatedAt.Time,
		UpdatedAt:  dbMarker.UpdatedAt.Time,
	}

	// Handle nullable last read comment ID
	if dbMarker.LastReadCommentID.Valid {
		marker.LastReadCommentID = &dbMarker.LastReadCommentID.Int32
	}

	return marker
}
