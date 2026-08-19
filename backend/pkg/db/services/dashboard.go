package db

import (
	"actionphase/pkg/core"
	db "actionphase/pkg/db/models"
	"actionphase/pkg/observability"
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DashboardService handles dashboard data aggregation and business logic
type DashboardService struct {
	DB     *pgxpool.Pool
	Logger *observability.Logger
}

// Ensure DashboardService implements the interface
var _ core.DashboardServiceInterface = (*DashboardService)(nil)

// GetUserDashboard retrieves complete dashboard data for a user
func (s *DashboardService) GetUserDashboard(ctx context.Context, userID int32) (*core.DashboardData, error) {
	q := db.New(s.DB)

	// Fetch user preferences to determine comment read mode
	prefsSvc := NewUserPreferencesService(s.DB)
	prefs, err := prefsSvc.GetUserPreferences(ctx, userID)
	if err != nil {
		s.Logger.LogError(ctx, err, "Failed to get user preferences", "user_id", userID)
		return nil, err
	}

	// Check if user has any games
	gameCount, err := q.CountUserGames(ctx, userID)
	if err != nil {
		s.Logger.LogError(ctx, err, "Failed to count user games", "user_id", userID)
		return nil, err
	}

	dashboard := &core.DashboardData{
		UserID:              userID,
		HasGames:            gameCount > 0,
		PlayerGames:         []*core.DashboardGameCard{},
		GMGames:             []*core.DashboardGameCard{},
		AudienceGames:       []*core.DashboardGameCard{},
		MixedRoleGames:      []*core.DashboardGameCard{},
		RecentMessages:      []*core.DashboardMessage{},
		UpcomingDeadlines:   []*core.DashboardDeadline{},
		NotificationsByType: map[string]int{},
	}

	// If no games, return early
	if !dashboard.HasGames {
		return dashboard, nil
	}

	// Fan out independent queries concurrently.
	var (
		dbGames        []db.GetUserDashboardGamesRow
		dbMessages     []db.GetUserRecentMessagesRow
		dbDeadlines    []db.GetUserUpcomingDeadlinesRow
		notifByType    []db.GetUserUnreadNotificationsByTypeRow
		unreadComments []unreadCommentCount

		mu       sync.Mutex
		firstErr error
	)

	setErr := func(err error) {
		mu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		mu.Unlock()
	}

	// Each query below defers wg.Done() *before* recoverFanOut, so Done still
	// runs on the panic path. A recover that skipped it would leave wg.Wait()
	// blocked forever, turning a process crash into a permanently hung request.
	var wg sync.WaitGroup
	wg.Add(5)

	go func() {
		defer wg.Done()
		defer recoverFanOut(ctx, s.Logger, "dashboard-games", setErr)

		res, err := q.GetUserDashboardGames(ctx, userID)
		if err != nil {
			s.Logger.LogError(ctx, err, "Failed to get dashboard games", "user_id", userID)
			setErr(err)
			return
		}
		dbGames = res
	}()

	go func() {
		defer wg.Done()
		defer recoverFanOut(ctx, s.Logger, "dashboard-recent-messages", setErr)

		res, err := q.GetUserRecentMessages(ctx, db.GetUserRecentMessagesParams{UserID: userID, Limit: 5})
		if err != nil {
			s.Logger.LogError(ctx, err, "Failed to get recent messages", "user_id", userID)
			setErr(err)
			return
		}
		dbMessages = res
	}()

	go func() {
		defer wg.Done()
		defer recoverFanOut(ctx, s.Logger, "dashboard-upcoming-deadlines", setErr)

		res, err := q.GetUserUpcomingDeadlines(ctx, db.GetUserUpcomingDeadlinesParams{UserID: userID, Limit: 10})
		if err != nil {
			s.Logger.LogError(ctx, err, "Failed to get upcoming deadlines", "user_id", userID)
			setErr(err)
			return
		}
		dbDeadlines = res
	}()

	go func() {
		defer wg.Done()
		defer recoverFanOut(ctx, s.Logger, "dashboard-notification-counts", setErr)

		res, err := q.GetUserUnreadNotificationsByType(ctx, userID)
		if err != nil {
			s.Logger.LogError(ctx, err, "Failed to get notification counts by type", "user_id", userID)
			setErr(err)
			return
		}
		notifByType = res
	}()

	go func() {
		defer wg.Done()
		defer recoverFanOut(ctx, s.Logger, "dashboard-unread-comments", setErr)

		res, err := getUnreadCommentCountsForDashboard(ctx, s.DB, userID, prefs.CommentReadMode)
		if err != nil {
			s.Logger.LogError(ctx, err, "Failed to get unread comment counts", "user_id", userID)
			setErr(err)
			return
		}
		unreadComments = res
	}()

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	dashboard.PlayerGames, dashboard.GMGames, dashboard.AudienceGames, dashboard.MixedRoleGames = groupGamesByRole(dbGames)
	dashboard.RecentMessages = transformMessages(dbMessages)
	dashboard.UpcomingDeadlines = transformDeadlines(dbDeadlines)
	for _, row := range notifByType {
		dashboard.NotificationsByType[row.Type] = int(row.Count)
		dashboard.UnreadNotifications += int(row.Count)
	}
	applyUnreadCommentCounts(dashboard, unreadComments)

	return dashboard, nil
}

type unreadCommentCount struct {
	GameID      int32
	UnreadCount int64
}

// applyUnreadCommentCounts sets UnreadComments on each game card from the query results.
func applyUnreadCommentCounts(dashboard *core.DashboardData, rows []unreadCommentCount) {
	counts := make(map[int32]int, len(rows))
	for _, row := range rows {
		counts[row.GameID] = int(row.UnreadCount)
	}
	for _, card := range dashboard.PlayerGames {
		card.UnreadComments = counts[card.GameID]
	}
	for _, card := range dashboard.GMGames {
		card.UnreadComments = counts[card.GameID]
	}
	for _, card := range dashboard.AudienceGames {
		card.UnreadComments = counts[card.GameID]
	}
	for _, card := range dashboard.MixedRoleGames {
		card.UnreadComments = counts[card.GameID]
	}
}

// groupGamesByRole groups games into player, GM, audience, and mixed role categories
func groupGamesByRole(dbGames []db.GetUserDashboardGamesRow) (
	playerGames []*core.DashboardGameCard,
	gmGames []*core.DashboardGameCard,
	audienceGames []*core.DashboardGameCard,
	mixedGames []*core.DashboardGameCard,
) {
	// Initialize slices to ensure they serialize as [] not null
	playerGames = make([]*core.DashboardGameCard, 0)
	gmGames = make([]*core.DashboardGameCard, 0)
	audienceGames = make([]*core.DashboardGameCard, 0)
	mixedGames = make([]*core.DashboardGameCard, 0)

	for _, game := range dbGames {
		card := transformGameCard(game)

		// Determine role grouping
		switch card.UserRole {
		case "player":
			playerGames = append(playerGames, card)
		case "gm", "co_gm":
			card.UserRole = "gm" // Normalize co_gm to gm
			gmGames = append(gmGames, card)
		case "audience":
			audienceGames = append(audienceGames, card)
		default:
			mixedGames = append(mixedGames, card)
		}
	}

	return
}

// transformGameCard converts database row to domain model with business logic
func transformGameCard(game db.GetUserDashboardGamesRow) *core.DashboardGameCard {
	isGM := game.UserRole == "gm" || game.UserRole == "co_gm"
	card := &core.DashboardGameCard{
		GameID:              game.ID,
		Title:               game.Title,
		State:               stringValue(game.State),
		Genre:               ptrStringValue(game.Genre),
		GMUserID:            game.GmUserID,
		GMUsername:          stringValue(game.GmUsername),
		UserRole:            game.UserRole,
		HasPendingAction:    game.HasPendingAction && !isGM,
		PendingApplications: int(game.PendingApplicationsCount),
		UnvotedPolls:        int(game.UnvotedPollsCount),
		UpdatedAt:           game.UpdatedAt.Time,
		CreatedAt:           game.CreatedAt.Time,
	}

	// Set description (optional field)
	if game.Description.Valid {
		desc := game.Description.String
		card.Description = &desc
	}

	// Set current phase information
	if game.CurrentPhaseID.Valid {
		phaseID := game.CurrentPhaseID.Int32
		card.CurrentPhaseID = &phaseID
	}

	if game.CurrentPhaseType.Valid {
		phaseType := game.CurrentPhaseType.String
		card.CurrentPhaseType = &phaseType
	}

	if game.CurrentPhaseTitle.Valid {
		phaseTitle := game.CurrentPhaseTitle.String
		card.CurrentPhaseTitle = &phaseTitle
	}

	if game.CurrentPhaseDeadline.Valid {
		deadline := game.CurrentPhaseDeadline.Time
		card.CurrentPhaseDeadline = &deadline

		// Calculate deadline status and urgency
		card.DeadlineStatus = core.CalculateDeadlineStatus(deadline)
		card.IsUrgent = core.IsGameUrgent(game.HasPendingAction, &deadline)
	} else {
		card.DeadlineStatus = "normal"
		card.IsUrgent = false
	}

	return card
}

// transformMessages converts database message rows to domain models.
// Redacts author names for players in anonymous games (GMs and co-GMs retain visibility).
func transformMessages(dbMessages []db.GetUserRecentMessagesRow) []*core.DashboardMessage {
	messages := make([]*core.DashboardMessage, 0, len(dbMessages))

	for _, msg := range dbMessages {
		authorName := msg.AuthorName
		if msg.IsAnonymous && msg.ViewerRole == "player" {
			authorName = ""
		}

		message := &core.DashboardMessage{
			MessageID:   msg.MessageID,
			GameID:      msg.GameID,
			GameTitle:   msg.GameTitle,
			AuthorName:  authorName,
			Content:     core.TruncateContent(msg.Content, 100),
			MessageType: string(msg.MessageType),
			CreatedAt:   msg.CreatedAt.Time,
		}

		// Set optional character name
		if msg.CharacterName.Valid {
			charName := msg.CharacterName.String
			message.CharacterName = &charName
		}

		// Set optional phase ID
		if msg.PhaseID.Valid {
			phaseID := msg.PhaseID.Int32
			message.PhaseID = &phaseID
		}

		messages = append(messages, message)
	}

	return messages
}

// transformDeadlines converts database deadline rows to domain models
func transformDeadlines(dbDeadlines []db.GetUserUpcomingDeadlinesRow) []*core.DashboardDeadline {
	deadlines := make([]*core.DashboardDeadline, 0, len(dbDeadlines))

	for _, dl := range dbDeadlines {
		if !dl.EndTime.Valid {
			continue
		}

		endTime := dl.EndTime.Time
		hoursRemaining := int(time.Until(endTime).Hours())

		deadline := &core.DashboardDeadline{
			DeadlineType:         dl.DeadlineType,
			SourceID:             dl.SourceID,
			PhaseID:              dl.PhaseID,
			GameID:               dl.GameID,
			GameTitle:            dl.GameTitle,
			Title:                dl.Title,
			PhaseType:            dl.PhaseType,
			PhaseTitle:           dl.PhaseTitle,
			PhaseNumber:          dl.PhaseNumber,
			EndTime:              endTime,
			HasPendingSubmission: dl.HasPendingSubmission,
			HoursRemaining:       hoursRemaining,
		}

		deadlines = append(deadlines, deadline)
	}

	return deadlines
}

// getUnreadCommentCountsForDashboard counts unread comments at all nesting depths per game.
// Uses a raw recursive CTE query — same pattern as GetPostCommentsWithThreads — because
// sqlc cannot resolve aliases from recursive CTEs in aggregate FILTER clauses.
func getUnreadCommentCountsForDashboard(ctx context.Context, pool *pgxpool.Pool, userID int32, commentReadMode string) ([]unreadCommentCount, error) {
	query := `
WITH RECURSIVE all_comments AS (
  SELECT
    c.id,
    c.author_id,
    c.created_at,
    c.is_deleted,
    posts.id AS root_post_id,
    posts.game_id
  FROM messages posts
  INNER JOIN game_phases gp ON gp.id = posts.phase_id
    AND gp.is_active = true
    AND gp.phase_type = 'common_room'
  INNER JOIN messages c ON c.parent_id = posts.id
  WHERE posts.message_type = 'post'
    AND posts.is_deleted = false
    AND posts.is_draft = false

  UNION ALL

  SELECT
    child.id,
    child.author_id,
    child.created_at,
    child.is_deleted,
    parent.root_post_id,
    parent.game_id
  FROM messages child
  INNER JOIN all_comments parent ON child.parent_id = parent.id
  WHERE child.message_type = 'comment'
)
SELECT
  g.id AS game_id,
  COALESCE(SUM(CASE
    WHEN $2::text = 'auto'
         AND ac.created_at > COALESCE(ucr.last_read_at, '1970-01-01'::timestamptz)
         AND ac.author_id != $1
         AND ac.is_deleted = false
    THEN 1
    WHEN $2::text != 'auto'
         AND ucmr.comment_id IS NULL
         AND ac.author_id != $1
         AND ac.is_deleted = false
    THEN 1
    ELSE 0
  END), 0)::bigint AS unread_count
FROM games g
LEFT JOIN game_participants part ON g.id = part.game_id AND part.user_id = $1 AND part.status = 'active'
LEFT JOIN all_comments ac ON ac.game_id = g.id
LEFT JOIN user_common_room_reads ucr ON ucr.post_id = ac.root_post_id AND ucr.user_id = $1
LEFT JOIN user_comment_reads ucmr ON ucmr.comment_id = ac.id AND ucmr.user_id = $1
WHERE ((part.user_id = $1 AND part.status = 'active') OR g.gm_user_id = $1)
GROUP BY g.id`

	rows, err := pool.Query(ctx, query, userID, commentReadMode)
	if err != nil {
		return nil, fmt.Errorf("failed to get unread comment counts: %w", err)
	}
	defer rows.Close()

	var results []unreadCommentCount
	for rows.Next() {
		var r unreadCommentCount
		if err := rows.Scan(&r.GameID, &r.UnreadCount); err != nil {
			return nil, fmt.Errorf("failed to scan unread comment count: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// Helper functions for nullable fields

func stringValue(v pgtype.Text) string {
	if v.Valid {
		return v.String
	}
	return ""
}

func ptrStringValue(v pgtype.Text) *string {
	if v.Valid {
		s := v.String
		return &s
	}
	return nil
}

// recoverFanOut recovers a panic in one dashboard fan-out query and reports it
// through setErr so the request fails cleanly.
//
// This deliberately does not use observability.SafeRun. SafeRun is for
// fire-and-forget background work, where swallowing the panic and carrying on is
// the right outcome. Here a panicking query means part of the dashboard is
// missing, and returning a silently incomplete dashboard as a 200 would be
// worse than an error: the caller cannot tell the difference between "no
// upcoming deadlines" and "the deadlines query blew up".
//
// Callers must defer wg.Done() *before* deferring this, so that Done still runs
// on the panic path and wg.Wait() cannot block forever.
func recoverFanOut(ctx context.Context, logger *observability.Logger, name string, setErr func(error)) {
	r := recover()
	if r == nil {
		return
	}

	err := fmt.Errorf("panic in %s: %v", name, r)
	if logger != nil {
		logger.LogError(ctx, err, "Dashboard fan-out query panicked",
			"unit", name,
			"stack_trace", string(debug.Stack()))
	}
	setErr(err)
}
