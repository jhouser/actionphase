package games

import (
	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// TestGameAPI_AutoAcceptAudience tests the auto-accept audience feature
// Covers: Application auto-approval, participant creation, application deletion, notifications
func TestGameAPI_AutoAcceptAudience(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "notifications", "game_applications", "game_participants", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "notifications", "game_applications", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create audience user
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	audienceUser, err := userService.CreateUser(&core.User{
		Username: "autoaccept_audience",
		Password: "testpass123",
		Email:    "autoaccept_audience@example.com",
	})
	core.AssertNoError(t, err, "Audience user creation should succeed")
	testDB.MarkUserVerified(t, audienceUser.ID)

	audienceToken, err := core.CreateTestJWTTokenForUser(app, audienceUser)
	core.AssertNoError(t, err, "Audience token creation should succeed")

	// Create a recruiting game with auto-accept enabled
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:              "Auto-Accept Test Game",
		Description:        "Testing auto-accept audience",
		GMUserID:           int32(fixtures.TestUser.ID),
		IsPublic:           true,
		AutoAcceptAudience: true, // Enable auto-accept
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	_, err = gameService.UpdateGameState(context.Background(), game.ID, "recruitment")
	core.AssertNoError(t, err, "Game state update should succeed")

	t.Run("auto_accept_creates_participant_and_deletes_application", func(t *testing.T) {
		// Apply as audience to auto-accept game
		applicationData := ApplyToGameRequest{
			Role:    "audience",
			Message: "I want to watch!",
		}
		body, _ := json.Marshal(applicationData)

		req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/apply", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+audienceToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 201, w.Code, "Application should be created with 201 Created")

		// Verify participant was created
		participants, err := gameService.GetGameParticipants(context.Background(), game.ID)
		core.AssertNoError(t, err, "Should get participants")
		core.AssertEqual(t, 1, len(participants), "Should have 1 participant (auto-accepted audience)")
		core.AssertEqual(t, int32(audienceUser.ID), participants[0].UserID, "Participant should be the audience user")
		core.AssertEqual(t, "audience", participants[0].Role, "Participant role should be audience")

		// Verify application was deleted (not just approved)
		appService := &db.GameApplicationService{DB: testDB.Pool}
		_, err = appService.GetGameApplicationByUserAndGame(context.Background(), game.ID, int32(audienceUser.ID))
		core.AssertError(t, err, "Application should be deleted after auto-accept")

		// Verify user received notification
		notificationService := &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}
		notifications, err := notificationService.GetUserNotifications(context.Background(), int32(audienceUser.ID), 10, 0)
		core.AssertNoError(t, err, "Should get notifications")
		core.AssertTrue(t, len(notifications) > 0, "User should have received notification")

		// Check notification content
		var joinedNotif *core.Notification
		for i := range notifications {
			if notifications[i].Type == core.NotificationTypeApplicationApproved {
				joinedNotif = notifications[i]
				break
			}
		}
		core.AssertTrue(t, joinedNotif != nil, "Should have application approved notification")
		core.AssertTrue(t, strings.Contains(joinedNotif.Title, "Joined"), "Notification title should mention 'Joined'")
	})

	t.Run("auto_accept_does_not_notify_gm", func(t *testing.T) {
		// Create another audience user
		audienceUser2, err := userService.CreateUser(&core.User{
			Username: "autoaccept_audience2",
			Password: "testpass123",
			Email:    "autoaccept_audience2@example.com",
		})
		core.AssertNoError(t, err, "Audience user 2 creation should succeed")
		testDB.MarkUserVerified(t, audienceUser2.ID)

		audienceToken2, err := core.CreateTestJWTTokenForUser(app, audienceUser2)
		core.AssertNoError(t, err, "Audience token 2 creation should succeed")

		// Get GM's current notification count
		notificationService := &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}
		notificationsBefore, err := notificationService.GetUserNotifications(context.Background(), int32(fixtures.TestUser.ID), 100, 0)
		core.AssertNoError(t, err, "Should get GM notifications")
		countBefore := len(notificationsBefore)

		// Apply as audience
		applicationData := ApplyToGameRequest{
			Role:    "audience",
			Message: "I want to watch too!",
		}
		body, _ := json.Marshal(applicationData)

		req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/apply", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+audienceToken2)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 201, w.Code, "Application should be created")

		// Verify GM DID receive a notification (GMs now get notified when audience joins)
		notificationsAfter, err := notificationService.GetUserNotifications(context.Background(), int32(fixtures.TestUser.ID), 100, 0)
		core.AssertNoError(t, err, "Should get GM notifications")
		countAfter := len(notificationsAfter)

		core.AssertEqual(t, countBefore+1, countAfter, "GM should receive notification for auto-accepted audience")
	})

	t.Run("auto_accept_disabled_creates_pending_application", func(t *testing.T) {
		// Create game with auto-accept disabled
		gameNoAutoAccept, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
			Title:              "Manual Approval Game",
			Description:        "Testing manual approval",
			GMUserID:           int32(fixtures.TestUser.ID),
			IsPublic:           true,
			AutoAcceptAudience: false, // Disabled
		})
		core.AssertNoError(t, err, "Game creation should succeed")

		_, err = gameService.UpdateGameState(context.Background(), gameNoAutoAccept.ID, "recruitment")
		core.AssertNoError(t, err, "Game state update should succeed")

		// Create audience user
		audienceUser3, err := userService.CreateUser(&core.User{
			Username: "manual_audience",
			Password: "testpass123",
			Email:    "manual_audience@example.com",
		})
		core.AssertNoError(t, err, "Audience user 3 creation should succeed")
		testDB.MarkUserVerified(t, audienceUser3.ID)

		audienceToken3, err := core.CreateTestJWTTokenForUser(app, audienceUser3)
		core.AssertNoError(t, err, "Audience token 3 creation should succeed")

		// Apply as audience
		applicationData := ApplyToGameRequest{
			Role:    "audience",
			Message: "I want to watch!",
		}
		body, _ := json.Marshal(applicationData)

		req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(gameNoAutoAccept.ID))+"/apply", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+audienceToken3)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 201, w.Code, "Application should be created")

		// Verify application exists and is pending (not auto-approved)
		appService := &db.GameApplicationService{DB: testDB.Pool}
		application, err := appService.GetGameApplicationByUserAndGame(context.Background(), gameNoAutoAccept.ID, int32(audienceUser3.ID))
		core.AssertNoError(t, err, "Application should exist")
		core.AssertEqual(t, "pending", application.Status.String, "Application should be pending (not auto-approved)")

		// Verify participant was NOT created
		participants, err := gameService.GetGameParticipants(context.Background(), gameNoAutoAccept.ID)
		core.AssertNoError(t, err, "Should get participants")
		core.AssertEqual(t, 0, len(participants), "Should have no participants (application still pending)")

		// Verify GM received notification for manual approval
		notificationService := &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}
		gmNotifications, err := notificationService.GetUserNotifications(context.Background(), int32(fixtures.TestUser.ID), 100, 0)
		core.AssertNoError(t, err, "Should get GM notifications")

		var appNotif *core.Notification
		for i := range gmNotifications {
			if gmNotifications[i].Type == core.NotificationTypeApplicationSubmitted && gmNotifications[i].RelatedID != nil && *gmNotifications[i].RelatedID == application.ID {
				appNotif = gmNotifications[i]
				break
			}
		}
		core.AssertTrue(t, appNotif != nil, "GM should receive notification for manual approval")
	})

	t.Run("auto_accept_only_applies_to_audience", func(t *testing.T) {
		// Create player user
		playerUser, err := userService.CreateUser(&core.User{
			Username: "autoaccept_player",
			Password: "testpass123",
			Email:    "autoaccept_player@example.com",
		})
		core.AssertNoError(t, err, "Player user creation should succeed")
		testDB.MarkUserVerified(t, playerUser.ID)

		playerToken, err := core.CreateTestJWTTokenForUser(app, playerUser)
		core.AssertNoError(t, err, "Player token creation should succeed")

		// Apply as player to auto-accept game (should NOT auto-accept)
		applicationData := ApplyToGameRequest{
			Role:    "player",
			Message: "I want to play!",
		}
		body, _ := json.Marshal(applicationData)

		req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/apply", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+playerToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 201, w.Code, "Application should be created")

		// Verify application exists and is pending (not auto-approved)
		appService := &db.GameApplicationService{DB: testDB.Pool}
		application, err := appService.GetGameApplicationByUserAndGame(context.Background(), game.ID, int32(playerUser.ID))
		core.AssertNoError(t, err, "Application should exist")
		core.AssertEqual(t, "pending", application.Status.String, "Player application should be pending (not auto-approved)")

		// Verify GM received notification for player application
		notificationService := &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}
		gmNotifications, err := notificationService.GetUserNotifications(context.Background(), int32(fixtures.TestUser.ID), 100, 0)
		core.AssertNoError(t, err, "Should get GM notifications")

		var appNotif *core.Notification
		for i := range gmNotifications {
			if gmNotifications[i].Type == core.NotificationTypeApplicationSubmitted && gmNotifications[i].RelatedID != nil && *gmNotifications[i].RelatedID == application.ID {
				appNotif = gmNotifications[i]
				break
			}
		}
		core.AssertTrue(t, appNotif != nil, "GM should receive notification for player application")
	})
}
