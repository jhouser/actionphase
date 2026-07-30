package db

import (
	"context"
	"testing"
	"time"

	"actionphase/pkg/core"
	db "actionphase/pkg/db/models"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestDashboardService_GetUserDashboard_NoGames(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "users")

	factory := core.NewTestDataFactory(testDB, t)
	service := &DashboardService{DB: testDB.Pool}

	// Create a user with no games
	user := factory.NewUser().Create()

	// Get dashboard
	dashboard, err := service.GetUserDashboard(context.Background(), user.ID)
	core.AssertNoError(t, err, "Failed to get dashboard")

	// Verify user has no games
	core.AssertEqual(t, false, dashboard.HasGames, "User should have no games")
	core.AssertEqual(t, 0, len(dashboard.PlayerGames), "Should have 0 player games")
	core.AssertEqual(t, 0, len(dashboard.GMGames), "Should have 0 GM games")
	core.AssertEqual(t, 0, len(dashboard.MixedRoleGames), "Should have 0 mixed role games")
	core.AssertEqual(t, 0, len(dashboard.RecentMessages), "Should have 0 recent messages")
	core.AssertEqual(t, 0, len(dashboard.UpcomingDeadlines), "Should have 0 deadlines")
}

func TestDashboardService_GetUserDashboard_AsPlayer(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "users")

	factory := core.NewTestDataFactory(testDB, t)
	service := &DashboardService{DB: testDB.Pool}

	// Create users
	gm := factory.NewUser().WithUsername("gamemaster").Create()
	player := factory.NewUser().WithUsername("player").Create()

	// Create game as GM
	game := factory.NewGame().
		WithTitle("Player Test Game").
		WithGM(gm.ID).
		WithState("in_progress").
		Create()

	// Add player as participant
	factory.NewGameParticipant().
		ForGame(game.ID).
		WithUser(player.ID).
		WithRole("player").
		Create()

	// Get dashboard for player
	dashboard, err := service.GetUserDashboard(context.Background(), player.ID)
	core.AssertNoError(t, err, "Failed to get dashboard")

	// Verify dashboard data
	core.AssertEqual(t, true, dashboard.HasGames, "Player should have games")
	core.AssertEqual(t, 1, len(dashboard.PlayerGames), "Should have 1 player game")
	core.AssertEqual(t, 0, len(dashboard.GMGames), "Should have 0 GM games")

	// Verify game details
	playerGame := dashboard.PlayerGames[0]
	core.AssertEqual(t, game.ID, playerGame.GameID, "Game ID should match")
	core.AssertEqual(t, "Player Test Game", playerGame.Title, "Game title should match")
	core.AssertEqual(t, "player", playerGame.UserRole, "User role should be player")
}

func TestDashboardService_GetUserDashboard_AsGM(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_applications", "game_participants", "games", "users")

	factory := core.NewTestDataFactory(testDB, t)
	service := &DashboardService{DB: testDB.Pool}

	// Create GM user
	gm := factory.NewUser().WithUsername("gamemaster").Create()

	// Create game as GM
	game := factory.NewGame().
		WithTitle("GM Test Game").
		WithGM(gm.ID).
		WithState("recruitment").
		Create()

	// Add GM as game participant with "co_gm" role (GMs need participant record for dashboard queries)
	factory.NewGameParticipant().
		ForGame(game.ID).
		WithUser(gm.ID).
		WithRole("co_gm").
		Create()

	// Create pending application
	applicant := factory.NewUser().WithUsername("applicant").Create()
	q := db.New(testDB.Pool)
	_, err := q.CreateGameApplication(context.Background(), db.CreateGameApplicationParams{
		GameID:  game.ID,
		UserID:  applicant.ID,
		Role:    "player",
		Message: pgtype.Text{String: "I'd like to join!", Valid: true},
	})
	core.AssertNoError(t, err, "Failed to create game application")

	// Get dashboard for GM
	dashboard, err := service.GetUserDashboard(context.Background(), gm.ID)
	core.AssertNoError(t, err, "Failed to get dashboard")

	// Verify dashboard data
	core.AssertEqual(t, true, dashboard.HasGames, "GM should have games")
	core.AssertEqual(t, 0, len(dashboard.PlayerGames), "Should have 0 player games")
	core.AssertEqual(t, 1, len(dashboard.GMGames), "Should have 1 GM game")

	// Verify GM game details
	gmGame := dashboard.GMGames[0]
	core.AssertEqual(t, game.ID, gmGame.GameID, "Game ID should match")
	core.AssertEqual(t, "gm", gmGame.UserRole, "User role should be GM")
	core.AssertEqual(t, 1, gmGame.PendingApplications, "Should have 1 pending application")
}

func TestDashboardService_GetUserDashboard_UrgentGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_submissions", "game_phases", "game_participants", "games", "users")

	factory := core.NewTestDataFactory(testDB, t)
	service := &DashboardService{DB: testDB.Pool}

	// Create users
	gm := factory.NewUser().WithUsername("gamemaster").Create()
	player := factory.NewUser().WithUsername("player").Create()

	// Create game
	game := factory.NewGame().
		WithGM(gm.ID).
		WithState("in_progress").
		Create()

	// Add player as participant
	factory.NewGameParticipant().
		ForGame(game.ID).
		WithUser(player.ID).
		WithRole("player").
		Create()

	// Create action phase with near deadline (30 minutes from now)
	deadlineNear := time.Now().Add(30 * time.Minute)
	phase := factory.NewPhase().
		InGame(game).
		ActionPhase().
		Active().
		WithDeadline(deadlineNear).
		Create()

	// Player has not submitted action yet (creates draft)
	factory.NewActionSubmission().
		InGame(game).
		ByUser(player).
		InPhase(phase).
		Draft().
		Create()

	// Get dashboard
	dashboard, err := service.GetUserDashboard(context.Background(), player.ID)
	core.AssertNoError(t, err, "Failed to get dashboard")

	// Verify urgent game
	core.AssertEqual(t, 1, len(dashboard.PlayerGames), "Should have 1 player game")
	urgentGame := dashboard.PlayerGames[0]

	core.AssertEqual(t, true, urgentGame.IsUrgent, "Game should be urgent (<3h with pending action)")
	core.AssertEqual(t, "critical", urgentGame.DeadlineStatus, "Deadline status should be critical (<1h)")
	core.AssertEqual(t, true, urgentGame.HasPendingAction, "Should have pending action")
	core.AssertTrue(t, urgentGame.CurrentPhaseDeadline != nil, "Should have deadline")
}

func TestDashboardService_GetUserDashboard_WarningDeadline(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_submissions", "game_phases", "game_participants", "games", "users")

	factory := core.NewTestDataFactory(testDB, t)
	service := &DashboardService{DB: testDB.Pool}

	// Create users
	gm := factory.NewUser().Create()
	player := factory.NewUser().Create()

	// Create game
	game := factory.NewGame().WithGM(gm.ID).WithState("in_progress").Create()
	factory.NewGameParticipant().
		ForGame(game.ID).
		WithUser(player.ID).
		WithRole("player").
		Create()

	// Create phase with warning deadline (2 hours from now)
	deadlineWarning := time.Now().Add(2 * time.Hour)
	phase := factory.NewPhase().
		InGame(game).
		ActionPhase().
		Active().
		WithDeadline(deadlineWarning).
		Create()

	// Player has pending action
	factory.NewActionSubmission().
		InGame(game).
		ByUser(player).
		InPhase(phase).
		Draft().
		Create()

	// Get dashboard
	dashboard, err := service.GetUserDashboard(context.Background(), player.ID)
	core.AssertNoError(t, err, "Failed to get dashboard")

	// Verify warning status
	game1 := dashboard.PlayerGames[0]
	core.AssertEqual(t, "warning", game1.DeadlineStatus, "Deadline status should be warning (1-3h)")
	core.AssertEqual(t, true, game1.IsUrgent, "Game should be urgent (pending action <3h)")
}

func TestDashboardService_GetUserDashboard_NormalDeadline(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_phases", "game_participants", "games", "users")

	factory := core.NewTestDataFactory(testDB, t)
	service := &DashboardService{DB: testDB.Pool}

	// Create users
	gm := factory.NewUser().Create()
	player := factory.NewUser().Create()

	// Create game
	game := factory.NewGame().WithGM(gm.ID).WithState("in_progress").Create()
	factory.NewGameParticipant().
		ForGame(game.ID).
		WithUser(player.ID).
		WithRole("player").
		Create()

	// Create phase with normal deadline (3 days from now)
	deadlineNormal := time.Now().Add(72 * time.Hour)
	factory.NewPhase().
		InGame(game).
		ActionPhase().
		Active().
		WithDeadline(deadlineNormal).
		Create()

	// Get dashboard (no action submission, so no pending action)
	dashboard, err := service.GetUserDashboard(context.Background(), player.ID)
	core.AssertNoError(t, err, "Failed to get dashboard")

	// Verify normal status
	game1 := dashboard.PlayerGames[0]
	core.AssertEqual(t, "normal", game1.DeadlineStatus, "Deadline status should be normal (>24h)")
	core.AssertEqual(t, false, game1.IsUrgent, "Game should not be urgent (no pending action)")
}

func TestDashboardService_GetUserDashboard_GMDoesNotGetPendingAction(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_phases", "game_participants", "games", "users")

	factory := core.NewTestDataFactory(testDB, t)
	service := &DashboardService{DB: testDB.Pool}

	// Create GM user (no participant record — pure GM)
	gm := factory.NewUser().WithUsername("gamemaster").Create()

	// Create game in action phase
	game := factory.NewGame().
		WithTitle("GM Action Game").
		WithGM(gm.ID).
		WithState("in_progress").
		Create()

	// Create active action phase
	factory.NewPhase().
		InGame(game).
		ActionPhase().
		Active().
		Create()

	// Get dashboard for GM
	dashboard, err := service.GetUserDashboard(context.Background(), gm.ID)
	core.AssertNoError(t, err, "Failed to get dashboard")

	core.AssertEqual(t, 1, len(dashboard.GMGames), "Should have 1 GM game")
	gmGame := dashboard.GMGames[0]

	// GMs cannot submit actions, so has_pending_action must be false
	core.AssertEqual(t, false, gmGame.HasPendingAction, "GM should not have pending action")
	core.AssertEqual(t, false, gmGame.IsUrgent, "GM game should not be urgent due to action phase")
}

func TestDashboardService_GetUserDashboard_RecentMessages(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_phases", "game_participants", "games", "users")

	factory := core.NewTestDataFactory(testDB, t)
	service := &DashboardService{DB: testDB.Pool}

	// Create users
	gm := factory.NewUser().Create()
	player := factory.NewUser().Create()
	otherPlayer := factory.NewUser().Create()

	// Create game
	game := factory.NewGame().WithGM(gm.ID).WithState("in_progress").Create()
	factory.NewGameParticipant().
		ForGame(game.ID).
		WithUser(player.ID).
		WithRole("player").
		Create()
	factory.NewGameParticipant().
		ForGame(game.ID).
		WithUser(otherPlayer.ID).
		WithRole("player").
		Create()

	// Create phase for messages
	phase := factory.NewPhase().InGame(game).CommonRoom().Active().Create()

	// Create character for the other player
	character := factory.NewCharacter().
		InGame(game).
		OwnedBy(otherPlayer).
		WithName("Test Character").
		Create()

	// Other player creates a message
	message := factory.NewPost().
		InGame(game).
		InPhase(phase).
		ByAuthor(otherPlayer).
		ByCharacter(character).
		WithContent("This is a test message that should appear on the dashboard").
		Create()

	// Get dashboard for player
	dashboard, err := service.GetUserDashboard(context.Background(), player.ID)
	core.AssertNoError(t, err, "Failed to get dashboard")

	// Verify recent messages
	core.AssertTrue(t, len(dashboard.RecentMessages) > 0, "Should have recent messages")

	recentMsg := dashboard.RecentMessages[0]
	core.AssertEqual(t, message.ID, recentMsg.MessageID, "Message ID should match")
	core.AssertEqual(t, game.ID, recentMsg.GameID, "Game ID should match")
	core.AssertNotEqual(t, "", recentMsg.Content, "Message content should not be empty")
	core.AssertTrue(t, len(recentMsg.Content) <= 103, "Content should be truncated (100 chars + '...')")
}

// TestDashboardService_GetUserDashboard_ExcludesDraftPosts verifies that GM draft
// posts (is_draft = true, not yet published to the phase) do not appear in the
// dashboard's recent activity feed.
func TestDashboardService_GetUserDashboard_ExcludesDraftPosts(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_phases", "game_participants", "games", "users")

	factory := core.NewTestDataFactory(testDB, t)
	service := &DashboardService{DB: testDB.Pool}

	gm := factory.NewUser().Create()
	player := factory.NewUser().Create()

	game := factory.NewGame().WithGM(gm.ID).WithState("in_progress").Create()
	factory.NewGameParticipant().
		ForGame(game.ID).
		WithUser(player.ID).
		WithRole("player").
		Create()

	phase := factory.NewPhase().InGame(game).CommonRoom().Active().Create()

	character := factory.NewCharacter().
		InGame(game).
		OwnedBy(player).
		WithName("Test Character").
		Create()

	// A published post should show up on the dashboard.
	publishedPost := factory.NewPost().
		InGame(game).
		InPhase(phase).
		ByAuthor(gm).
		ByCharacter(character).
		WithContent("Published post visible on dashboard").
		Create()

	// A GM draft post (not yet published) should NOT show up on the dashboard.
	queries := db.New(testDB.Pool)
	_, err := queries.CreateDraftPost(context.Background(), db.CreateDraftPostParams{
		GameID:                game.ID,
		PhaseID:               pgtype.Int4{Int32: phase.ID, Valid: true},
		AuthorID:              gm.ID,
		CharacterID:           character.ID,
		Content:               "Draft post that should not appear on dashboard",
		Visibility:            db.MessageVisibilityGame,
		MentionedCharacterIds: []int32{},
	})
	core.AssertNoError(t, err, "Failed to create draft post")

	dashboard, err := service.GetUserDashboard(context.Background(), player.ID)
	core.AssertNoError(t, err, "Failed to get dashboard")

	core.AssertTrue(t, len(dashboard.RecentMessages) > 0, "Should have recent messages")
	for _, msg := range dashboard.RecentMessages {
		core.AssertNotEqual(t, "Draft post that should not appear on dashboard", msg.Content,
			"Draft post should not appear in recent activity")
	}

	found := false
	for _, msg := range dashboard.RecentMessages {
		if msg.MessageID == publishedPost.ID {
			found = true
		}
	}
	core.AssertTrue(t, found, "Published post should appear in recent activity")
}

func TestDashboardService_GetUserDashboard_UpcomingDeadlines(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_submissions", "game_phases", "game_participants", "games", "users")

	factory := core.NewTestDataFactory(testDB, t)
	service := &DashboardService{DB: testDB.Pool}

	// Create users
	gm := factory.NewUser().Create()
	player := factory.NewUser().Create()

	// Create game
	game := factory.NewGame().WithGM(gm.ID).WithState("in_progress").Create()
	factory.NewGameParticipant().
		ForGame(game.ID).
		WithUser(player.ID).
		WithRole("player").
		Create()

	// Create game 2 for second deadline (only one active phase per game)
	game2 := factory.NewGame().WithGM(gm.ID).WithState("in_progress").Create()
	factory.NewGameParticipant().
		ForGame(game2.ID).
		WithUser(player.ID).
		WithRole("player").
		Create()

	// Create multiple phases with different deadlines across different games
	deadline1 := time.Now().Add(2 * time.Hour)
	deadline2 := time.Now().Add(5 * time.Hour)
	deadline3 := time.Now().Add(24 * time.Hour)

	phase1 := factory.NewPhase().InGame(game).ActionPhase().Active().WithDeadline(deadline1).Create()
	factory.NewPhase().InGame(game).ActionPhase().WithDeadline(deadline2).Create() // Not active
	factory.NewPhase().InGame(game2).ActionPhase().Active().WithDeadline(deadline3).Create()

	// Create action submission for phase1
	factory.NewActionSubmission().
		InGame(game).
		ByUser(player).
		InPhase(phase1).
		Draft().
		Create()

	// Get dashboard
	dashboard, err := service.GetUserDashboard(context.Background(), player.ID)
	core.AssertNoError(t, err, "Failed to get dashboard")

	// Verify upcoming deadlines (only active phases should appear)
	core.AssertTrue(t, len(dashboard.UpcomingDeadlines) >= 2, "Should have at least 2 upcoming deadlines from active phases")

	// First deadline should be soonest
	firstDeadline := dashboard.UpcomingDeadlines[0]
	core.AssertEqual(t, phase1.ID, firstDeadline.PhaseID, "First deadline should be from phase1")
	core.AssertEqual(t, true, firstDeadline.HasPendingSubmission, "Should have pending submission")
	core.AssertTrue(t, firstDeadline.HoursRemaining < 3, "Should be less than 3 hours remaining")
}

func TestDashboardService_GetUserDashboard_GMDoesNotGetPendingSubmissionInDeadlines(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_phases", "games", "users")

	factory := core.NewTestDataFactory(testDB, t)
	service := &DashboardService{DB: testDB.Pool}

	// Create GM user (no participant record — pure GM)
	gm := factory.NewUser().WithUsername("gmuser").Create()

	// Create game in action phase with deadline
	game := factory.NewGame().
		WithTitle("GM Deadline Game").
		WithGM(gm.ID).
		WithState("in_progress").
		Create()

	// Create active action phase with upcoming deadline
	deadline := time.Now().Add(6 * time.Hour)
	factory.NewPhase().
		InGame(game).
		ActionPhase().
		Active().
		WithDeadline(deadline).
		Create()

	// Get dashboard for GM
	dashboard, err := service.GetUserDashboard(context.Background(), gm.ID)
	core.AssertNoError(t, err, "Failed to get dashboard")

	core.AssertTrue(t, len(dashboard.UpcomingDeadlines) >= 1, "GM should see the upcoming deadline")

	// GMs cannot submit actions, so has_pending_submission must be false
	dl := dashboard.UpcomingDeadlines[0]
	core.AssertEqual(t, false, dl.HasPendingSubmission, "GM should not have pending submission in upcoming deadlines")
}

func TestDashboardService_GetUserDashboard_SortingOrder(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_submissions", "game_phases", "game_participants", "games", "users")

	factory := core.NewTestDataFactory(testDB, t)
	service := &DashboardService{DB: testDB.Pool}

	// Create users
	gm := factory.NewUser().Create()
	player := factory.NewUser().Create()

	// Create 3 games with different urgency levels
	game1 := factory.NewGame().WithTitle("Normal Game").WithGM(gm.ID).WithState("in_progress").Create()
	game2 := factory.NewGame().WithTitle("Urgent Game").WithGM(gm.ID).WithState("in_progress").Create()
	game3 := factory.NewGame().WithTitle("No Deadline Game").WithGM(gm.ID).WithState("in_progress").Create()

	factory.NewGameParticipant().ForGame(game1.ID).WithUser(player.ID).WithRole("player").Create()
	factory.NewGameParticipant().ForGame(game2.ID).WithUser(player.ID).WithRole("player").Create()
	factory.NewGameParticipant().ForGame(game3.ID).WithUser(player.ID).WithRole("player").Create()

	// Game 1: Normal deadline (48 hours)
	factory.NewPhase().InGame(game1).ActionPhase().Active().
		WithDeadline(time.Now().Add(48 * time.Hour)).Create()

	// Game 2: Urgent deadline (3 hours) with pending action
	phase2 := factory.NewPhase().InGame(game2).ActionPhase().Active().
		WithDeadline(time.Now().Add(3 * time.Hour)).Create()
	factory.NewActionSubmission().
		InGame(game2).
		ByUser(player).
		InPhase(phase2).
		Draft().
		Create()

	// Game 3: No active phase/deadline

	// Get dashboard
	dashboard, err := service.GetUserDashboard(context.Background(), player.ID)
	core.AssertNoError(t, err, "Failed to get dashboard")

	// Verify sorting: Urgent game should be first
	core.AssertEqual(t, 3, len(dashboard.PlayerGames), "Should have 3 player games")
	core.AssertEqual(t, "Urgent Game", dashboard.PlayerGames[0].Title, "Urgent game should be first")
	core.AssertEqual(t, true, dashboard.PlayerGames[0].IsUrgent, "First game should be urgent")
}

func TestDashboardService_GetUserDashboard_ExcludesCompletedGames(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "users")

	factory := core.NewTestDataFactory(testDB, t)
	service := &DashboardService{DB: testDB.Pool}

	// Create users
	gm := factory.NewUser().WithUsername("gamemaster").Create()
	player := factory.NewUser().WithUsername("player").Create()

	// Create an active/in_progress game (should appear on dashboard)
	activeGame := factory.NewGame().
		WithTitle("Active Game").
		WithGM(gm.ID).
		WithState("in_progress").
		Create()

	// Create a completed game (should NOT appear on dashboard)
	completedGame := factory.NewGame().
		WithTitle("Completed Game").
		WithGM(gm.ID).
		WithState("completed").
		Create()

	// Add player to both games
	factory.NewGameParticipant().
		ForGame(activeGame.ID).
		WithUser(player.ID).
		WithRole("player").
		Create()

	factory.NewGameParticipant().
		ForGame(completedGame.ID).
		WithUser(player.ID).
		WithRole("player").
		Create()

	// Get dashboard for player
	dashboard, err := service.GetUserDashboard(context.Background(), player.ID)
	core.AssertNoError(t, err, "Failed to get dashboard")

	// Verify only the active game appears
	core.AssertEqual(t, true, dashboard.HasGames, "Player should have games")
	core.AssertEqual(t, 1, len(dashboard.PlayerGames), "Should have exactly 1 game (completed game excluded)")

	// Verify it's the active game, not the completed one
	playerGame := dashboard.PlayerGames[0]
	core.AssertEqual(t, activeGame.ID, playerGame.GameID, "Game should be the active game")
	core.AssertEqual(t, "Active Game", playerGame.Title, "Game title should be 'Active Game'")
	core.AssertEqual(t, "in_progress", playerGame.State, "Game state should be in_progress")

	// Test GM view as well
	dashboardGM, err := service.GetUserDashboard(context.Background(), gm.ID)
	core.AssertNoError(t, err, "Failed to get GM dashboard")

	// GM should also only see active game (completed excluded)
	core.AssertEqual(t, 1, len(dashboardGM.GMGames), "GM should have exactly 1 game (completed game excluded)")
	core.AssertEqual(t, activeGame.ID, dashboardGM.GMGames[0].GameID, "GM should see the active game")
	core.AssertEqual(t, "Active Game", dashboardGM.GMGames[0].Title, "GM should see 'Active Game'")
}

// TestDashboardService_RecentMessages_AnonymousMode verifies that player usernames are
// redacted in recent activity messages when the game is in anonymous mode.
// Anonymous mode hides which player controls which character, so a character is required.
func TestDashboardService_RecentMessages_AnonymousMode(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_phases", "game_participants", "games", "users")

	factory := core.NewTestDataFactory(testDB, t)
	service := &DashboardService{DB: testDB.Pool}

	gm := factory.NewUser().Create()
	player := factory.NewUser().Create()
	otherPlayer := factory.NewUser().WithUsername("visible_player").Create()

	anonGame := factory.NewGame().WithGM(gm.ID).WithState("in_progress").WithAnonymous().Create()
	factory.NewGameParticipant().ForGame(anonGame.ID).WithUser(player.ID).WithRole("player").Create()
	factory.NewGameParticipant().ForGame(anonGame.ID).WithUser(otherPlayer.ID).WithRole("player").Create()

	phase := factory.NewPhase().InGame(anonGame).CommonRoom().Active().Create()
	character := factory.NewCharacter().InGame(anonGame).OwnedBy(otherPlayer).WithName("Mystery Knight").Create()
	factory.NewPost().InGame(anonGame).InPhase(phase).ByAuthor(otherPlayer).ByCharacter(character).
		WithContent("Anonymous game message").Create()

	t.Run("player cannot see author username in anonymous game", func(t *testing.T) {
		dashboard, err := service.GetUserDashboard(context.Background(), player.ID)
		core.AssertNoError(t, err, "Failed to get dashboard")
		core.AssertTrue(t, len(dashboard.RecentMessages) > 0, "Should have recent messages")
		core.AssertEqual(t, "", dashboard.RecentMessages[0].AuthorName,
			"Author name must be empty for player in anonymous game")
	})

	t.Run("GM can see author username in anonymous game", func(t *testing.T) {
		dashboard, err := service.GetUserDashboard(context.Background(), gm.ID)
		core.AssertNoError(t, err, "Failed to get GM dashboard")
		core.AssertTrue(t, len(dashboard.RecentMessages) > 0, "GM should see recent messages")
		core.AssertEqual(t, otherPlayer.Username, dashboard.RecentMessages[0].AuthorName,
			"GM must see author name even in anonymous game")
	})
}

// TestDashboardService_RecentMessages_NonAnonymousShowsUsername verifies that usernames
// are visible in non-anonymous games (ensures no over-redaction).
func TestDashboardService_RecentMessages_NonAnonymousShowsUsername(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_phases", "game_participants", "games", "users")

	factory := core.NewTestDataFactory(testDB, t)
	service := &DashboardService{DB: testDB.Pool}

	gm := factory.NewUser().Create()
	player := factory.NewUser().Create()
	otherPlayer := factory.NewUser().WithUsername("known_player").Create()

	game := factory.NewGame().WithGM(gm.ID).WithState("in_progress").Create()
	factory.NewGameParticipant().ForGame(game.ID).WithUser(player.ID).WithRole("player").Create()
	factory.NewGameParticipant().ForGame(game.ID).WithUser(otherPlayer.ID).WithRole("player").Create()

	phase := factory.NewPhase().InGame(game).CommonRoom().Active().Create()
	character := factory.NewCharacter().InGame(game).OwnedBy(otherPlayer).WithName("Named Hero").Create()
	factory.NewPost().InGame(game).InPhase(phase).ByAuthor(otherPlayer).ByCharacter(character).
		WithContent("Public game message").Create()

	dashboard, err := service.GetUserDashboard(context.Background(), player.ID)
	core.AssertNoError(t, err, "Failed to get dashboard")
	core.AssertTrue(t, len(dashboard.RecentMessages) > 0, "Should have recent messages")
	core.AssertEqual(t, otherPlayer.Username, dashboard.RecentMessages[0].AuthorName,
		"Author name must be visible in non-anonymous game")
}

// TestDashboardService_RecentMessages_ExcludesAudienceGames verifies that messages from
// games where the user is only an audience member do not appear in the recent activity feed.
func TestDashboardService_RecentMessages_ExcludesAudienceGames(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_phases", "game_participants", "games", "users", "characters")

	factory := core.NewTestDataFactory(testDB, t)
	service := &DashboardService{DB: testDB.Pool}

	gm := factory.NewUser().Create()
	player := factory.NewUser().Create()
	poster := factory.NewUser().Create()

	// Game where the user is an audience member
	audienceGame := factory.NewGame().WithGM(gm.ID).WithState("in_progress").Create()
	factory.NewGameParticipant().ForGame(audienceGame.ID).WithUser(player.ID).WithRole("audience").Create()
	factory.NewGameParticipant().ForGame(audienceGame.ID).WithUser(poster.ID).WithRole("player").Create()

	phase := factory.NewPhase().InGame(audienceGame).CommonRoom().Active().Create()
	character := factory.NewCharacter().InGame(audienceGame).OwnedBy(poster).WithName("Poster Character").Create()
	factory.NewPost().InGame(audienceGame).InPhase(phase).ByAuthor(poster).ByCharacter(character).
		WithContent("Audience game message - should not appear in recent activity").Create()

	dashboard, err := service.GetUserDashboard(context.Background(), player.ID)
	core.AssertNoError(t, err, "Failed to get dashboard")

	for _, msg := range dashboard.RecentMessages {
		core.AssertNotEqual(t, audienceGame.ID, msg.GameID,
			"Recent activity must not include messages from games where user is only an audience member")
	}
}
