package db

import (
	"context"
	"fmt"
	"testing"

	"actionphase/pkg/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationService_CreateNotification(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test user
	user := testDB.CreateTestUser(t, "testuser", "test@example.com")

	tests := []struct {
		name    string
		req     *core.CreateNotificationRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid notification",
			req: &core.CreateNotificationRequest{
				UserID:  int32(user.ID),
				GameID:  nil, // No game association to avoid foreign key constraint
				Type:    core.NotificationTypePrivateMessage,
				Title:   "You have a new message",
				LinkURL: stringPtr("/messages"),
			},
			wantErr: false,
		},
		{
			name: "missing title",
			req: &core.CreateNotificationRequest{
				UserID: int32(user.ID),
				Type:   core.NotificationTypePrivateMessage,
				Title:  "",
			},
			wantErr: true,
			errMsg:  "Title",
		},
		{
			name: "invalid notification type",
			req: &core.CreateNotificationRequest{
				UserID: int32(user.ID),
				Type:   "invalid_type",
				Title:  "Test",
			},
			wantErr: true,
			errMsg:  "", // Validator returns empty error for custom validation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notification, err := service.CreateNotification(ctx, tt.req)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			assert.NotZero(t, notification.ID)
			assert.Equal(t, tt.req.UserID, notification.UserID)
			assert.Equal(t, tt.req.Type, notification.Type)
			assert.Equal(t, tt.req.Title, notification.Title)
			assert.False(t, notification.IsRead)
			assert.NotZero(t, notification.CreatedAt)
		})
	}
}

func TestNotificationService_GetUnreadCount(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test user
	user := testDB.CreateTestUser(t, "testuser", "test@example.com")

	// Create 5 notifications
	for i := 0; i < 5; i++ {
		_, err := service.CreateNotification(ctx, &core.CreateNotificationRequest{
			UserID: int32(user.ID),
			Type:   core.NotificationTypePrivateMessage,
			Title:  "Test notification",
		})
		require.NoError(t, err)
	}

	// Get unread count
	count, err := service.GetUnreadCount(ctx, int32(user.ID))
	require.NoError(t, err)
	assert.Equal(t, int64(5), count)

	// Mark 2 as read
	notifications, err := service.GetUserNotifications(ctx, int32(user.ID), 2, 0)
	require.NoError(t, err)
	require.Len(t, notifications, 2)

	err = service.MarkAsRead(ctx, notifications[0].ID, int32(user.ID))
	require.NoError(t, err)
	err = service.MarkAsRead(ctx, notifications[1].ID, int32(user.ID))
	require.NoError(t, err)

	// Check unread count again
	count, err = service.GetUnreadCount(ctx, int32(user.ID))
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestNotificationService_MarkAsRead(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test user
	user := testDB.CreateTestUser(t, "testuser", "test@example.com")

	// Create notification
	notification, err := service.CreateNotification(ctx, &core.CreateNotificationRequest{
		UserID: int32(user.ID),
		Type:   core.NotificationTypePrivateMessage,
		Title:  "Test notification",
	})
	require.NoError(t, err)
	assert.False(t, notification.IsRead)

	// Mark as read
	err = service.MarkAsRead(ctx, notification.ID, int32(user.ID))
	require.NoError(t, err)

	// Verify it's marked as read
	notifications, err := service.GetUserNotifications(ctx, int32(user.ID), 10, 0)
	require.NoError(t, err)
	require.Len(t, notifications, 1)
	assert.True(t, notifications[0].IsRead)
	assert.NotNil(t, notifications[0].ReadAt)
}

func TestNotificationService_MarkAsUnread(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	user := testDB.CreateTestUser(t, "testuser", "test@example.com")
	otherUser := testDB.CreateTestUser(t, "other", "other@example.com")

	notification, err := service.CreateNotification(ctx, &core.CreateNotificationRequest{
		UserID: int32(user.ID),
		Type:   core.NotificationTypePrivateMessage,
		Title:  "Test notification",
	})
	require.NoError(t, err)

	// Mark read, then unread
	err = service.MarkAsRead(ctx, notification.ID, int32(user.ID))
	require.NoError(t, err)

	err = service.MarkAsUnread(ctx, notification.ID, int32(user.ID))
	require.NoError(t, err)

	// Verify unread state is restored
	notifications, err := service.GetUserNotifications(ctx, int32(user.ID), 10, 0)
	require.NoError(t, err)
	require.Len(t, notifications, 1)
	assert.False(t, notifications[0].IsRead)
	assert.Nil(t, notifications[0].ReadAt)

	t.Run("ownership check: cannot mark another user's notification as unread", func(t *testing.T) {
		// pgx `:one` query returns "no rows in result set" when user_id does not match,
		// which the service wraps and returns as an error — ownership enforced.
		err = service.MarkAsUnread(ctx, notification.ID, int32(otherUser.ID))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to mark notification as unread")
	})
}

func TestNotificationService_MarkAllAsRead(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test user
	user := testDB.CreateTestUser(t, "testuser", "test@example.com")

	// Create 3 notifications
	for i := 0; i < 3; i++ {
		_, err := service.CreateNotification(ctx, &core.CreateNotificationRequest{
			UserID: int32(user.ID),
			Type:   core.NotificationTypePrivateMessage,
			Title:  "Test notification",
		})
		require.NoError(t, err)
	}

	// Verify unread count
	count, err := service.GetUnreadCount(ctx, int32(user.ID))
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// Mark all as read
	err = service.MarkAllAsRead(ctx, int32(user.ID))
	require.NoError(t, err)

	// Verify all marked as read
	count, err = service.GetUnreadCount(ctx, int32(user.ID))
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestNotificationService_GetUserNotifications_Pagination(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test user
	user := testDB.CreateTestUser(t, "testuser", "test@example.com")

	// Create 10 notifications
	for i := 0; i < 10; i++ {
		_, err := service.CreateNotification(ctx, &core.CreateNotificationRequest{
			UserID: int32(user.ID),
			Type:   core.NotificationTypePrivateMessage,
			Title:  "Test notification",
		})
		require.NoError(t, err)
	}

	// Get first 5
	page1, err := service.GetUserNotifications(ctx, int32(user.ID), 5, 0)
	require.NoError(t, err)
	assert.Len(t, page1, 5)

	// Get next 5
	page2, err := service.GetUserNotifications(ctx, int32(user.ID), 5, 5)
	require.NoError(t, err)
	assert.Len(t, page2, 5)

	// Verify no overlap (IDs should be different)
	page1IDs := make(map[int32]bool)
	for _, n := range page1 {
		page1IDs[n.ID] = true
	}
	for _, n := range page2 {
		assert.False(t, page1IDs[n.ID], "Pagination should not have overlapping results")
	}
}

func TestNotificationService_DeleteNotification(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test users
	user1 := testDB.CreateTestUser(t, "user1", "user1@example.com")
	user2 := testDB.CreateTestUser(t, "user2", "user2@example.com")

	// Create notification for user1
	notification, err := service.CreateNotification(ctx, &core.CreateNotificationRequest{
		UserID: int32(user1.ID),
		Type:   core.NotificationTypePrivateMessage,
		Title:  "Test notification",
	})
	require.NoError(t, err)

	// User1 can delete their own notification
	err = service.DeleteNotification(ctx, notification.ID, int32(user1.ID))
	require.NoError(t, err)

	// Verify it's deleted
	notifications, err := service.GetUserNotifications(ctx, int32(user1.ID), 10, 0)
	require.NoError(t, err)
	assert.Len(t, notifications, 0)

	// Create another notification
	notification2, err := service.CreateNotification(ctx, &core.CreateNotificationRequest{
		UserID: int32(user1.ID),
		Type:   core.NotificationTypePrivateMessage,
		Title:  "Test notification 2",
	})
	require.NoError(t, err)

	// User2 cannot delete user1's notification (should have no effect)
	err = service.DeleteNotification(ctx, notification2.ID, int32(user2.ID))
	// This should not error but should not delete the notification
	require.NoError(t, err)

	// Verify it still exists
	notifications, err = service.GetUserNotifications(ctx, int32(user1.ID), 10, 0)
	require.NoError(t, err)
	assert.Len(t, notifications, 1)
}

func TestNotificationService_NotifyPhaseCreated(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test users (GM + 2 players)
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	// Create test game with GM
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Add players as participants (with status 'active')
	testDB.AddTestGameParticipant(t, int32(game.ID), int32(player1.ID), "player")
	testDB.AddTestGameParticipant(t, int32(game.ID), int32(player2.ID), "player")

	// Create a test phase
	phase := testDB.CreateTestPhase(t, int32(game.ID), "action", "Test Phase")

	// Notify all participants about the phase (excluding GM who created it)
	err := service.NotifyPhaseCreated(ctx, int32(game.ID), int32(phase.ID), phase.Title, int32(gm.ID))
	require.NoError(t, err)

	// Verify player1 received notification
	player1Notifications, err := service.GetUserNotifications(ctx, int32(player1.ID), 10, 0)
	require.NoError(t, err)
	assert.Len(t, player1Notifications, 1)
	assert.Equal(t, "New phase: Test Phase", player1Notifications[0].Title)
	assert.Equal(t, core.NotificationTypePhaseCreated, player1Notifications[0].Type)
	assert.False(t, player1Notifications[0].IsRead)

	// Verify player2 received notification
	player2Notifications, err := service.GetUserNotifications(ctx, int32(player2.ID), 10, 0)
	require.NoError(t, err)
	assert.Len(t, player2Notifications, 1)
	assert.Equal(t, "New phase: Test Phase", player2Notifications[0].Title)

	// Verify GM did NOT receive notification (excluded)
	gmNotifications, err := service.GetUserNotifications(ctx, int32(gm.ID), 10, 0)
	require.NoError(t, err)
	assert.Len(t, gmNotifications, 0)
}

// TestNotificationService_NotifyHandoutPublished_LinksToSpecificHandout verifies that a
// handout-published notification links directly to the specific handout (via the `handout`
// query param) rather than just the handouts tab, so clicking the notification opens the
// relevant handout instead of the tab's default view.
func TestNotificationService_NotifyHandoutPublished_LinksToSpecificHandout(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	testDB.AddTestGameParticipant(t, int32(game.ID), int32(player.ID), "player")

	const handoutID int32 = 4242
	err := service.NotifyHandoutPublished(ctx, int32(game.ID), handoutID, "The Map", int32(gm.ID))
	require.NoError(t, err)

	playerNotifications, err := service.GetUserNotifications(ctx, int32(player.ID), 10, 0)
	require.NoError(t, err)
	require.Len(t, playerNotifications, 1)

	notif := playerNotifications[0]
	require.NotNil(t, notif.LinkURL)
	expected := fmt.Sprintf("/games/%d?tab=handouts&handout=%d", game.ID, handoutID)
	assert.Equal(t, expected, *notif.LinkURL,
		"handout notification should link to the specific handout, not just the handouts tab")
}

func TestNotificationService_DeleteOldReadNotifications(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	user := testDB.CreateTestUser(t, "cleanup_user", "cleanup@example.com")

	// Create a recent notification (should NOT be deleted)
	recentNotif, err := service.CreateNotification(ctx, &core.CreateNotificationRequest{
		UserID: int32(user.ID),
		Type:   core.NotificationTypePrivateMessage,
		Title:  "Recent notification",
	})
	require.NoError(t, err)

	// Create an old notification (should be deleted) and backdate it
	oldNotif, err := service.CreateNotification(ctx, &core.CreateNotificationRequest{
		UserID: int32(user.ID),
		Type:   core.NotificationTypePrivateMessage,
		Title:  "Old notification",
	})
	require.NoError(t, err)

	_, err = testDB.Pool.Exec(ctx,
		"UPDATE notifications SET created_at = NOW() - INTERVAL '31 days' WHERE id = $1",
		oldNotif.ID,
	)
	require.NoError(t, err)

	// Run cleanup
	err = service.DeleteOldReadNotifications(ctx)
	require.NoError(t, err)

	// Old notification should be gone
	notifications, err := service.GetUserNotifications(ctx, int32(user.ID), 10, 0)
	require.NoError(t, err)
	require.Len(t, notifications, 1)
	assert.Equal(t, recentNotif.ID, notifications[0].ID, "Only the recent notification should remain")
}

// TestNotificationService_NotifyApplicationApproved verifies that an approval notification
// is created with the correct type and title.
func TestNotificationService_NotifyApplicationApproved(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	ctx := context.Background()
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	err := service.NotifyApplicationApproved(ctx, int32(player.ID), game.ID, game.Title)
	require.NoError(t, err)

	notifs, err := service.GetUserNotifications(ctx, int32(player.ID), 10, 0)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(notifs), 1)

	var found bool
	for _, n := range notifs {
		if n.Type == core.NotificationTypeApplicationApproved {
			assert.Contains(t, n.Title, "approved")
			assert.Contains(t, n.Title, game.Title)
			found = true
			break
		}
	}
	assert.True(t, found, "approval notification should exist with correct type")
}

// TestNotificationService_NotifyCharacterApproved verifies that a character approval
// notification is created with the correct type and title.
func TestNotificationService_NotifyCharacterApproved(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	ctx := context.Background()
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	charID := int32(999)

	err := service.NotifyCharacterApproved(ctx, int32(player.ID), game.ID, charID, "HeroChar")
	require.NoError(t, err)

	notifs, err := service.GetUserNotifications(ctx, int32(player.ID), 10, 0)
	require.NoError(t, err)

	var found bool
	for _, n := range notifs {
		if n.Type == core.NotificationTypeCharacterApproved {
			assert.Contains(t, n.Title, "HeroChar")
			assert.Contains(t, n.Title, "published")
			found = true
			break
		}
	}
	assert.True(t, found, "character approval notification should exist with correct type")
}

// TestNotificationService_NotifyCommonRoomPost validates that the bulk notification
// excludes the poster and notifies other participants. If excludeUserID is ignored,
// the poster receives a spurious notification about their own post.
func TestNotificationService_NotifyCommonRoomPost(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	ctx := context.Background()
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err := gameService.AddGameParticipant(ctx, game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(ctx, game.ID, int32(player2.ID), "player")
	require.NoError(t, err)

	postID := int32(1)

	// player1 is the poster — should be excluded
	err = service.NotifyCommonRoomPost(ctx, game.ID, postID, "A New Post", int32(player1.ID))
	require.NoError(t, err)

	t.Run("poster does not receive notification about their own post", func(t *testing.T) {
		notifs, err := service.GetUserNotifications(ctx, int32(player1.ID), 10, 0)
		require.NoError(t, err)
		for _, n := range notifs {
			assert.NotEqual(t, core.NotificationTypeCommonRoomPost, n.Type,
				"poster should not receive a notification about their own post")
		}
	})

	t.Run("other participants receive the post notification", func(t *testing.T) {
		notifs, err := service.GetUserNotifications(ctx, int32(player2.ID), 10, 0)
		require.NoError(t, err)

		var found bool
		for _, n := range notifs {
			if n.Type == core.NotificationTypeCommonRoomPost {
				assert.Contains(t, n.Title, "A New Post")
				found = true
				break
			}
		}
		assert.True(t, found, "non-poster participants should receive the post notification")
	})
}

// TestNotificationService_NotifyPhaseCreated_ExcludesAudience verifies that audience members
// do not receive phase_created notifications — only players and co-GMs should.
func TestNotificationService_NotifyPhaseCreated_ExcludesAudience(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "notifications", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	svc := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	ctx := context.Background()
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	audience := testDB.CreateTestUser(t, "audience", "audience@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err := gameService.AddGameParticipant(ctx, game.ID, int32(player.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(ctx, game.ID, int32(audience.ID), "audience")
	require.NoError(t, err)

	err = svc.NotifyPhaseCreated(ctx, game.ID, 1, "Act One", int32(gm.ID))
	require.NoError(t, err)

	playerNotifs, err := svc.GetUserNotifications(ctx, int32(player.ID), 10, 0)
	require.NoError(t, err)
	var playerFound bool
	for _, n := range playerNotifs {
		if n.Type == core.NotificationTypePhaseCreated {
			playerFound = true
			break
		}
	}
	assert.True(t, playerFound, "player should receive a phase_created notification")

	audienceNotifs, err := svc.GetUserNotifications(ctx, int32(audience.ID), 10, 0)
	require.NoError(t, err)
	for _, n := range audienceNotifs {
		assert.NotEqual(t, core.NotificationTypePhaseCreated, n.Type,
			"audience member should not receive phase_created notifications")
	}
}

// TestNotificationService_GetUnreadNotifications verifies that only unread notifications
// are returned and that the limit is respected. Also exercises convertUnreadRowToCore.
func TestNotificationService_GetUnreadNotifications(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	user := testDB.CreateTestUser(t, "unread_user", "unread@example.com")

	// Create 5 notifications
	for i := 0; i < 5; i++ {
		_, err := service.CreateNotification(ctx, &core.CreateNotificationRequest{
			UserID: int32(user.ID),
			Type:   core.NotificationTypePrivateMessage,
			Title:  "Unread notification",
		})
		require.NoError(t, err)
	}

	// Mark 2 as read
	all, err := service.GetUserNotifications(ctx, int32(user.ID), 2, 0)
	require.NoError(t, err)
	require.Len(t, all, 2)
	err = service.MarkAsRead(ctx, all[0].ID, int32(user.ID))
	require.NoError(t, err)
	err = service.MarkAsRead(ctx, all[1].ID, int32(user.ID))
	require.NoError(t, err)

	t.Run("returns only unread notifications", func(t *testing.T) {
		unread, err := service.GetUnreadNotifications(ctx, int32(user.ID), 100)
		require.NoError(t, err)
		assert.Equal(t, 3, len(unread))
		for _, n := range unread {
			assert.False(t, n.IsRead, "GetUnreadNotifications should only return unread notifications")
		}
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		unread, err := service.GetUnreadNotifications(ctx, int32(user.ID), 2)
		require.NoError(t, err)
		assert.Equal(t, 2, len(unread))
	})

	t.Run("zero limit returns all unread", func(t *testing.T) {
		unread, err := service.GetUnreadNotifications(ctx, int32(user.ID), 0)
		require.NoError(t, err)
		assert.Equal(t, 3, len(unread))
	})
}

// TestNotificationService_CreateBulkNotifications verifies that bulk creation reaches
// all target users. Fire-and-forget errors should not surface to the caller.
func TestNotificationService_CreateBulkNotifications(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	user1 := testDB.CreateTestUser(t, "bulk1", "bulk1@example.com")
	user2 := testDB.CreateTestUser(t, "bulk2", "bulk2@example.com")
	user3 := testDB.CreateTestUser(t, "bulk3", "bulk3@example.com")

	t.Run("creates notifications for all users", func(t *testing.T) {
		err := service.CreateBulkNotifications(ctx,
			[]int32{int32(user1.ID), int32(user2.ID), int32(user3.ID)},
			&core.CreateNotificationRequest{
				Type:  core.NotificationTypePrivateMessage,
				Title: "Bulk notification",
			},
		)
		require.NoError(t, err)

		for _, uid := range []int{user1.ID, user2.ID, user3.ID} {
			notifs, err := service.GetUserNotifications(ctx, int32(uid), 10, 0)
			require.NoError(t, err)
			assert.Len(t, notifs, 1, "each user should have received exactly one notification")
		}
	})

	t.Run("empty user list is a no-op and returns nil", func(t *testing.T) {
		err := service.CreateBulkNotifications(ctx, []int32{},
			&core.CreateNotificationRequest{
				Type:  core.NotificationTypePrivateMessage,
				Title: "Should not be created",
			},
		)
		require.NoError(t, err)
	})
}

// TestNotificationService_NotifyCommentReply verifies title format and notification type.
// A wrong type means the frontend renders it with the wrong icon/copy.
func TestNotificationService_NotifyCommentReply(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	author := testDB.CreateTestUser(t, "author", "author@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	replyID := int32(42)
	err := service.NotifyCommentReply(ctx, int32(author.ID), replyID, game.ID, "Replier")
	require.NoError(t, err)

	notifs, err := service.GetUserNotifications(ctx, int32(author.ID), 10, 0)
	require.NoError(t, err)
	require.Len(t, notifs, 1)
	assert.Equal(t, core.NotificationTypeCommentReply, notifs[0].Type)
	assert.Contains(t, notifs[0].Title, "Replier")
	assert.Contains(t, notifs[0].Title, "replied")
}

// TestNotificationService_NotifyCharacterMention verifies title format and notification type.
// A wrong type means mention alerts are invisible to the mentioned character's owner.
func TestNotificationService_NotifyCharacterMention(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	charOwner := testDB.CreateTestUser(t, "charowner", "charowner@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	commentID := int32(99)
	err := service.NotifyCharacterMention(ctx, int32(charOwner.ID), commentID, game.ID, "Mentioner", "MentionedChar")
	require.NoError(t, err)

	notifs, err := service.GetUserNotifications(ctx, int32(charOwner.ID), 10, 0)
	require.NoError(t, err)
	require.Len(t, notifs, 1)
	assert.Equal(t, core.NotificationTypeCharacterMention, notifs[0].Type)
	assert.Contains(t, notifs[0].Title, "Mentioner")
	assert.Contains(t, notifs[0].Title, "MentionedChar")
}

// TestNotificationService_NotifyActionSubmitted verifies that the GM and co-GMs receive
// action_submitted notifications, but not the submitting player.
func TestNotificationService_NotifyActionSubmitted(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "notifications", "game_participants", "games", "users")

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	coGM := testDB.CreateTestUser(t, "cogm", "cogm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err := gameService.AddGameParticipant(ctx, game.ID, int32(coGM.ID), "co_gm")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(ctx, game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	actionID := int32(7)
	err = service.NotifyActionSubmitted(ctx, actionID, game.ID, int32(player.ID), "BraveHero")
	require.NoError(t, err)

	// GM should be notified
	gmNotifs, err := service.GetUserNotifications(ctx, int32(gm.ID), 10, 0)
	require.NoError(t, err)
	require.Len(t, gmNotifs, 1)
	assert.Equal(t, core.NotificationTypeActionSubmitted, gmNotifs[0].Type)
	assert.Contains(t, gmNotifs[0].Title, "BraveHero")
	assert.Contains(t, gmNotifs[0].Title, "submitted")

	// co-GM should also be notified
	coGMNotifs, err := service.GetUserNotifications(ctx, int32(coGM.ID), 10, 0)
	require.NoError(t, err)
	require.Len(t, coGMNotifs, 1)
	assert.Equal(t, core.NotificationTypeActionSubmitted, coGMNotifs[0].Type)

	// Submitting player should not receive their own notification
	playerNotifs, err := service.GetUserNotifications(ctx, int32(player.ID), 10, 0)
	require.NoError(t, err)
	for _, n := range playerNotifs {
		assert.NotEqual(t, core.NotificationTypeActionSubmitted, n.Type)
	}
}

// TestNotificationService_NotifyActionResult verifies type and title for player result alerts.
// A wrong type means players don't see their action outcome notification.
func TestNotificationService_NotifyActionResult(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	resultID := int32(3)
	err := service.NotifyActionResult(ctx, int32(player.ID), resultID, game.ID, "Storm the Castle")
	require.NoError(t, err)

	notifs, err := service.GetUserNotifications(ctx, int32(player.ID), 10, 0)
	require.NoError(t, err)
	require.Len(t, notifs, 1)
	assert.Equal(t, core.NotificationTypeActionResult, notifs[0].Type)
	assert.Contains(t, notifs[0].Title, "Storm the Castle")
	assert.Contains(t, notifs[0].Title, "result")
}

// contextNotificationFor builds an unread notification scoped to a conversation.
func contextNotificationFor(userID int32, conversationID int32, messageID int32, title string) *core.CreateNotificationRequest {
	return &core.CreateNotificationRequest{
		UserID:      userID,
		Type:        core.NotificationTypePrivateMessage,
		Title:       title,
		RelatedType: stringPtr("message"),
		RelatedID:   &messageID,
		ContextType: stringPtr(core.NotificationContextConversation),
		ContextID:   &conversationID,
	}
}

// unreadTitles returns the titles of a user's unread notifications, which is
// enough to identify which rows a bulk clear left behind.
func unreadTitles(t *testing.T, ctx context.Context, service *NotificationService, userID int32) []string {
	t.Helper()

	unread, err := service.GetUnreadNotifications(ctx, userID, 100)
	require.NoError(t, err)

	titles := make([]string, 0, len(unread))
	for _, n := range unread {
		titles = append(titles, n.Title)
	}
	return titles
}

func TestNotificationService_MarkContextAsRead_ClearsOnlyThatContext(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	user := testDB.CreateTestUser(t, "reader", "reader@example.com")
	userID := int32(user.ID)

	// Three messages in the conversation being read, one in another
	// conversation, and one unrelated notification with no context at all.
	for i, title := range []string{"msg one", "msg two", "msg three"} {
		_, err := service.CreateNotification(ctx, contextNotificationFor(userID, 42, int32(100+i), title))
		require.NoError(t, err)
	}
	_, err := service.CreateNotification(ctx, contextNotificationFor(userID, 99, 200, "other conversation"))
	require.NoError(t, err)
	_, err = service.CreateNotification(ctx, &core.CreateNotificationRequest{
		UserID: userID,
		Type:   core.NotificationTypeHandoutPublished,
		Title:  "no context",
	})
	require.NoError(t, err)

	rows, err := service.MarkContextAsRead(ctx, userID, core.NotificationContextConversation, 42)
	require.NoError(t, err)
	assert.Equal(t, int64(3), rows, "should clear exactly the three notifications for conversation 42")

	// The untouched rows are the ones a user would otherwise have to dismiss by
	// hand; the bug being fixed is precisely that they got over-cleared or
	// under-cleared.
	assert.ElementsMatch(t, []string{"other conversation", "no context"}, unreadTitles(t, ctx, service, userID))
}

func TestNotificationService_MarkContextAsRead_LeavesOtherUsersAlone(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	reader := testDB.CreateTestUser(t, "reader", "reader@example.com")
	other := testDB.CreateTestUser(t, "other", "other@example.com")

	// Both users are in the same group conversation and each got a notification.
	_, err := service.CreateNotification(ctx, contextNotificationFor(int32(reader.ID), 42, 100, "reader copy"))
	require.NoError(t, err)
	_, err = service.CreateNotification(ctx, contextNotificationFor(int32(other.ID), 42, 100, "other copy"))
	require.NoError(t, err)

	rows, err := service.MarkContextAsRead(ctx, int32(reader.ID), core.NotificationContextConversation, 42)
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	// One participant reading the conversation must not dismiss it for everyone.
	assert.Empty(t, unreadTitles(t, ctx, service, int32(reader.ID)))
	assert.Equal(t, []string{"other copy"}, unreadTitles(t, ctx, service, int32(other.ID)))
}

func TestNotificationService_MarkContextAsRead_IsIdempotent(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	user := testDB.CreateTestUser(t, "reader", "reader@example.com")
	userID := int32(user.ID)

	_, err := service.CreateNotification(ctx, contextNotificationFor(userID, 42, 100, "msg one"))
	require.NoError(t, err)

	rows, err := service.MarkContextAsRead(ctx, userID, core.NotificationContextConversation, 42)
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	// Re-opening an already-read conversation should be a no-op rather than
	// re-stamping read_at.
	rows, err = service.MarkContextAsRead(ctx, userID, core.NotificationContextConversation, 42)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows)
}

func TestNotificationService_MarkAsRead_ClearsWholeConversation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	user := testDB.CreateTestUser(t, "reader", "reader@example.com")
	userID := int32(user.ID)

	// The reported scenario: waking up to a pile of messages in one group
	// conversation, plus unrelated notifications that must survive.
	var first *core.Notification
	for i, title := range []string{"msg one", "msg two", "msg three"} {
		n, err := service.CreateNotification(ctx, contextNotificationFor(userID, 42, int32(100+i), title))
		require.NoError(t, err)
		if i == 0 {
			first = n
		}
	}
	_, err := service.CreateNotification(ctx, &core.CreateNotificationRequest{
		UserID: userID,
		Type:   core.NotificationTypeHandoutPublished,
		Title:  "unrelated",
	})
	require.NoError(t, err)

	// Clicking a single notification clears every sibling in its conversation.
	require.NoError(t, service.MarkAsRead(ctx, first.ID, userID))

	assert.Equal(t, []string{"unrelated"}, unreadTitles(t, ctx, service, userID))
}

func TestNotificationService_MarkAsRead_WithoutContextClearsOnlyOne(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	user := testDB.CreateTestUser(t, "reader", "reader@example.com")
	userID := int32(user.ID)

	// Notifications predating context tracking share no context, so they must
	// keep the original one-at-a-time behaviour rather than clearing each other.
	target, err := service.CreateNotification(ctx, &core.CreateNotificationRequest{
		UserID: userID,
		Type:   core.NotificationTypeHandoutPublished,
		Title:  "first",
	})
	require.NoError(t, err)
	_, err = service.CreateNotification(ctx, &core.CreateNotificationRequest{
		UserID: userID,
		Type:   core.NotificationTypeHandoutPublished,
		Title:  "second",
	})
	require.NoError(t, err)

	require.NoError(t, service.MarkAsRead(ctx, target.ID, userID))

	assert.Equal(t, []string{"second"}, unreadTitles(t, ctx, service, userID))
}

func TestNotificationService_MarkAsRead_IgnoresOtherUsersNotification(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	owner := testDB.CreateTestUser(t, "owner", "owner@example.com")
	attacker := testDB.CreateTestUser(t, "attacker", "attacker@example.com")

	notification, err := service.CreateNotification(ctx, contextNotificationFor(int32(owner.ID), 42, 100, "owner msg"))
	require.NoError(t, err)

	// Marking someone else's notification read must not touch it. It reports
	// not-found rather than forbidden so notification IDs can't be probed.
	err = service.MarkAsRead(ctx, notification.ID, int32(attacker.ID))
	require.ErrorIs(t, err, core.ErrNotificationNotFound)

	assert.Equal(t, []string{"owner msg"}, unreadTitles(t, ctx, service, int32(owner.ID)))
}

func TestNotificationService_MarkAsRead_AlreadyReadIsNotAnError(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	service := &NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	user := testDB.CreateTestUser(t, "reader", "reader@example.com")
	userID := int32(user.ID)

	notification, err := service.CreateNotification(ctx, contextNotificationFor(userID, 42, 100, "msg one"))
	require.NoError(t, err)

	require.NoError(t, service.MarkAsRead(ctx, notification.ID, userID))

	// Clicking a notification that a previous action already cleared is
	// ordinary, not a missing notification: affecting zero rows must not be
	// mistaken for the row not existing.
	require.NoError(t, service.MarkAsRead(ctx, notification.ID, userID))
}
