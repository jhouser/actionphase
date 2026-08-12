package core

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"

	models "actionphase/pkg/db/models"
	"github.com/jackc/pgx/v5/pgtype"
)

// TestCanSeeUsernamesInAnonymousGame tests the anonymous game username visibility rule:
// - Non-anonymous game: all users can see usernames
// - Anonymous game: GM, co-GM, and audience can see usernames; players cannot
func TestCanSeeUsernamesInAnonymousGame(t *testing.T) {
	if os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Skip("Skipping integration test - SKIP_DB_TESTS=true")
	}

	testDB := NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	queries := models.New(testDB.Pool)

	gmUser := testDB.CreateTestUser(t, "anon_gm", "anon_gm@example.com")
	coGMUser := testDB.CreateTestUser(t, "anon_cogm", "anon_cogm@example.com")
	audienceUser := testDB.CreateTestUser(t, "anon_audience", "anon_audience@example.com")
	playerUser := testDB.CreateTestUser(t, "anon_player", "anon_player@example.com")

	// Create anonymous game directly (CreateTestGame doesn't set IsAnonymous)
	anonGame, err := queries.CreateGame(ctx, models.CreateGameParams{
		Title:       "Anonymous Test Game",
		Description: pgtype.Text{String: "Test", Valid: true},
		GmUserID:    int32(gmUser.ID),
		IsAnonymous: true,
		IsPublic:    pgtype.Bool{Bool: true, Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to create anonymous test game: %v", err)
	}

	// Create non-anonymous game for contrast
	normalGame, err := queries.CreateGame(ctx, models.CreateGameParams{
		Title:       "Normal Test Game",
		Description: pgtype.Text{String: "Test", Valid: true},
		GmUserID:    int32(gmUser.ID),
		IsPublic:    pgtype.Bool{Bool: true, Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to create normal test game: %v", err)
	}

	// Anonymous game that has COMPLETED: public archive mode lifts anonymity.
	completedAnonGame, err := queries.CreateGame(ctx, models.CreateGameParams{
		Title:       "Completed Anonymous Game",
		Description: pgtype.Text{String: "Test", Valid: true},
		GmUserID:    int32(gmUser.ID),
		IsAnonymous: true,
		IsPublic:    pgtype.Bool{Bool: true, Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to create completed anonymous test game: %v", err)
	}
	completedAnonGame.State = pgtype.Text{String: GameStateCompleted, Valid: true}

	// Anonymous game that was CANCELLED: not public, so anonymity still applies.
	cancelledAnonGame, err := queries.CreateGame(ctx, models.CreateGameParams{
		Title:       "Cancelled Anonymous Game",
		Description: pgtype.Text{String: "Test", Valid: true},
		GmUserID:    int32(gmUser.ID),
		IsAnonymous: true,
		IsPublic:    pgtype.Bool{Bool: true, Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to create cancelled anonymous test game: %v", err)
	}
	cancelledAnonGame.State = pgtype.Text{String: GameStateCancelled, Valid: true}

	type participant struct {
		userID int32
		role   string
	}
	participants := []participant{
		{int32(coGMUser.ID), "co_gm"},
		{int32(audienceUser.ID), "audience"},
		{int32(playerUser.ID), "player"},
	}
	for _, p := range participants {
		for _, gameRef := range []*models.Game{&anonGame, &normalGame, &completedAnonGame, &cancelledAnonGame} {
			_, err := queries.AddGameParticipant(ctx, models.AddGameParticipantParams{
				GameID: gameRef.ID,
				UserID: p.userID,
				Role:   p.role,
			})
			if err != nil {
				t.Fatalf("Failed to add %s participant: %v", p.role, err)
			}
		}
	}

	tests := []struct {
		name   string
		game   models.Game
		userID int32
		want   bool
	}{
		// Non-anonymous game: everyone sees usernames
		{"non-anon: GM sees username", normalGame, int32(gmUser.ID), true},
		{"non-anon: player sees username", normalGame, int32(playerUser.ID), true},
		{"non-anon: audience sees username", normalGame, int32(audienceUser.ID), true},
		{"non-anon: co-GM sees username", normalGame, int32(coGMUser.ID), true},

		// Anonymous game: GM, co-GM, audience see usernames; players do not
		{"anon: GM sees username", anonGame, int32(gmUser.ID), true},
		{"anon: co-GM sees username", anonGame, int32(coGMUser.ID), true},
		{"anon: audience sees username", anonGame, int32(audienceUser.ID), true},
		{"anon: player cannot see username", anonGame, int32(playerUser.ID), false},
		{"anon: non-participant cannot see username", anonGame, 99999, false},

		// Completed anonymous game: public archive mode lifts anonymity for
		// everyone, including players and non-participants.
		{"completed anon: player sees username", completedAnonGame, int32(playerUser.ID), true},
		{"completed anon: non-participant sees username", completedAnonGame, 99999, true},
		{"completed anon: GM sees username", completedAnonGame, int32(gmUser.ID), true},
		{"completed anon: audience sees username", completedAnonGame, int32(audienceUser.ID), true},

		// Cancelled games are NOT public, so the play-time rule still applies.
		{"cancelled anon: player cannot see username", cancelledAnonGame, int32(playerUser.ID), false},
		{"cancelled anon: non-participant cannot see username", cancelledAnonGame, 99999, false},
		{"cancelled anon: GM sees username", cancelledAnonGame, int32(gmUser.ID), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanSeeUsernamesInAnonymousGame(ctx, testDB.Pool, tt.game, tt.userID)
			if got != tt.want {
				t.Errorf("CanSeeUsernamesInAnonymousGame() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsUserCoGM tests the IsUserCoGM permission check
func TestIsUserCoGM(t *testing.T) {
	if os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Skip("Skipping integration test - SKIP_DB_TESTS=true")
	}

	testDB := NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()

	// Create test users
	gmUser := testDB.CreateTestUser(t, "gm_user", "gm@example.com")
	coGMUser := testDB.CreateTestUser(t, "cogm_user", "cogm@example.com")
	playerUser := testDB.CreateTestUser(t, "player_user", "player@example.com")

	// Create test game
	game := testDB.CreateTestGame(t, int32(gmUser.ID), "Test Game")

	// Add co-GM participant
	queries := models.New(testDB.Pool)
	_, err := queries.AddGameParticipant(ctx, models.AddGameParticipantParams{
		GameID: game.ID,
		UserID: int32(coGMUser.ID),
		Role:   "co_gm",
	})
	if err != nil {
		t.Fatalf("Failed to add co-GM participant: %v", err)
	}

	// Add player participant
	_, err = queries.AddGameParticipant(ctx, models.AddGameParticipantParams{
		GameID: game.ID,
		UserID: int32(playerUser.ID),
		Role:   "player",
	})
	if err != nil {
		t.Fatalf("Failed to add player participant: %v", err)
	}

	tests := []struct {
		name   string
		gameID int32
		userID int32
		want   bool
	}{
		{
			name:   "co-GM user returns true",
			gameID: game.ID,
			userID: int32(coGMUser.ID),
			want:   true,
		},
		{
			name:   "player user returns false",
			gameID: game.ID,
			userID: int32(playerUser.ID),
			want:   false,
		},
		{
			name:   "primary GM user returns false (not co-GM)",
			gameID: game.ID,
			userID: int32(gmUser.ID),
			want:   false,
		},
		{
			name:   "non-participant returns false",
			gameID: game.ID,
			userID: 99999,
			want:   false,
		},
		{
			name:   "invalid game ID returns false",
			gameID: 99999,
			userID: int32(coGMUser.ID),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsUserCoGM(ctx, testDB.Pool, tt.gameID, tt.userID)
			if got != tt.want {
				t.Errorf("IsUserCoGM() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsUserAudience tests the IsUserAudience permission check
func TestIsUserAudience(t *testing.T) {
	if os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Skip("Skipping integration test - SKIP_DB_TESTS=true")
	}

	testDB := NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()

	// Create test users
	gmUser := testDB.CreateTestUser(t, "gm_user2", "gm2@example.com")
	audienceUser := testDB.CreateTestUser(t, "audience_user", "audience@example.com")
	playerUser := testDB.CreateTestUser(t, "player_user2", "player2@example.com")

	// Create test game
	game := testDB.CreateTestGame(t, int32(gmUser.ID), "Test Game 2")

	// Add audience participant
	queries := models.New(testDB.Pool)
	_, err := queries.AddGameParticipant(ctx, models.AddGameParticipantParams{
		GameID: game.ID,
		UserID: int32(audienceUser.ID),
		Role:   "audience",
	})
	if err != nil {
		t.Fatalf("Failed to add audience participant: %v", err)
	}

	// Add player participant
	_, err = queries.AddGameParticipant(ctx, models.AddGameParticipantParams{
		GameID: game.ID,
		UserID: int32(playerUser.ID),
		Role:   "player",
	})
	if err != nil {
		t.Fatalf("Failed to add player participant: %v", err)
	}

	tests := []struct {
		name   string
		gameID int32
		userID int32
		want   bool
	}{
		{
			name:   "audience user returns true",
			gameID: game.ID,
			userID: int32(audienceUser.ID),
			want:   true,
		},
		{
			name:   "player user returns false",
			gameID: game.ID,
			userID: int32(playerUser.ID),
			want:   false,
		},
		{
			name:   "non-participant returns false",
			gameID: game.ID,
			userID: 99999,
			want:   false,
		},
		{
			name:   "invalid game ID returns false",
			gameID: 99999,
			userID: int32(audienceUser.ID),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsUserAudience(ctx, testDB.Pool, tt.gameID, tt.userID)
			if got != tt.want {
				t.Errorf("IsUserAudience() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsUserGameMaster tests the IsUserGameMaster permission check with HTTP request headers
func TestIsUserGameMaster(t *testing.T) {
	if os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Skip("Skipping integration test - SKIP_DB_TESTS=true")
	}

	testDB := NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()

	// Create test users
	gmUser := testDB.CreateTestUser(t, "gm_user3", "gm3@example.com")
	coGMUser := testDB.CreateTestUser(t, "cogm_user2", "cogm2@example.com")
	adminUser := testDB.CreateTestUser(t, "admin_user", "admin@example.com")
	adminUser.IsAdmin = true // Mark as admin
	playerUser := testDB.CreateTestUser(t, "player_user3", "player3@example.com")

	// Create test game
	game := testDB.CreateTestGame(t, int32(gmUser.ID), "Test Game 3")

	// Add co-GM participant
	queries := models.New(testDB.Pool)
	_, err := queries.AddGameParticipant(ctx, models.AddGameParticipantParams{
		GameID: game.ID,
		UserID: int32(coGMUser.ID),
		Role:   "co_gm",
	})
	if err != nil {
		t.Fatalf("Failed to add co-GM participant: %v", err)
	}

	tests := []struct {
		name         string
		userID       int32
		isAdmin      bool
		game         models.Game
		adminModeHdr string
		want         bool
		description  string
	}{
		{
			name:         "primary GM returns true",
			userID:       int32(gmUser.ID),
			isAdmin:      false,
			game:         *game,
			adminModeHdr: "",
			want:         true,
			description:  "Primary GM should have access",
		},
		{
			name:         "co-GM returns true",
			userID:       int32(coGMUser.ID),
			isAdmin:      false,
			game:         *game,
			adminModeHdr: "",
			want:         true,
			description:  "Co-GM should have access",
		},
		{
			name:         "admin with admin mode enabled returns true",
			userID:       int32(adminUser.ID),
			isAdmin:      true,
			game:         *game,
			adminModeHdr: "true",
			want:         true,
			description:  "Admin with admin mode should have access",
		},
		{
			name:         "admin without admin mode returns false",
			userID:       int32(adminUser.ID),
			isAdmin:      true,
			game:         *game,
			adminModeHdr: "",
			want:         false,
			description:  "Admin without admin mode should not have access",
		},
		{
			name:         "admin with admin mode false returns false",
			userID:       int32(adminUser.ID),
			isAdmin:      true,
			game:         *game,
			adminModeHdr: "false",
			want:         false,
			description:  "Admin with admin mode=false should not have access",
		},
		{
			name:         "regular player returns false",
			userID:       int32(playerUser.ID),
			isAdmin:      false,
			game:         *game,
			adminModeHdr: "",
			want:         false,
			description:  "Regular player should not have GM access",
		},
		{
			name:         "non-admin with admin mode header returns false",
			userID:       int32(playerUser.ID),
			isAdmin:      false,
			game:         *game,
			adminModeHdr: "true",
			want:         false,
			description:  "Non-admin cannot use admin mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			// AdminModeMiddleware reads the header and sets context; simulate that here.
			ctx := WithAdminMode(req.Context(), tt.adminModeHdr == "true")
			req = req.WithContext(ctx)

			got := IsUserGameMaster(req, tt.userID, tt.isAdmin, tt.game, testDB.Pool)
			if got != tt.want {
				t.Errorf("IsUserGameMaster() = %v, want %v - %s", got, tt.want, tt.description)
			}
		})
	}
}

// TestIsUserGameMasterCtx tests the context-based IsUserGameMaster variant
func TestIsUserGameMasterCtx(t *testing.T) {
	if os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Skip("Skipping integration test - SKIP_DB_TESTS=true")
	}

	testDB := NewTestDatabase(t)
	defer testDB.Close()

	baseCtx := context.Background()

	// Create test users
	gmUser := testDB.CreateTestUser(t, "gm_user4", "gm4@example.com")
	coGMUser := testDB.CreateTestUser(t, "cogm_user3", "cogm3@example.com")
	adminUser := testDB.CreateTestUser(t, "admin_user2", "admin2@example.com")
	adminUser.IsAdmin = true
	playerUser := testDB.CreateTestUser(t, "player_user4", "player4@example.com")

	// Create test game
	game := testDB.CreateTestGame(t, int32(gmUser.ID), "Test Game 4")

	// Add co-GM participant
	queries := models.New(testDB.Pool)
	_, err := queries.AddGameParticipant(baseCtx, models.AddGameParticipantParams{
		GameID: game.ID,
		UserID: int32(coGMUser.ID),
		Role:   "co_gm",
	})
	if err != nil {
		t.Fatalf("Failed to add co-GM participant: %v", err)
	}

	tests := []struct {
		name        string
		userID      int32
		isAdmin     bool
		game        models.Game
		adminMode   bool
		want        bool
		description string
	}{
		{
			name:        "primary GM returns true",
			userID:      int32(gmUser.ID),
			isAdmin:     false,
			game:        *game,
			adminMode:   false,
			want:        true,
			description: "Primary GM should have access",
		},
		{
			name:        "co-GM returns true",
			userID:      int32(coGMUser.ID),
			isAdmin:     false,
			game:        *game,
			adminMode:   false,
			want:        true,
			description: "Co-GM should have access",
		},
		{
			name:        "admin with admin mode enabled returns true",
			userID:      int32(adminUser.ID),
			isAdmin:     true,
			game:        *game,
			adminMode:   true,
			want:        true,
			description: "Admin with admin mode should have access",
		},
		{
			name:        "admin without admin mode returns false",
			userID:      int32(adminUser.ID),
			isAdmin:     true,
			game:        *game,
			adminMode:   false,
			want:        false,
			description: "Admin without admin mode should not have access",
		},
		{
			name:        "regular player returns false",
			userID:      int32(playerUser.ID),
			isAdmin:     false,
			game:        *game,
			adminMode:   false,
			want:        false,
			description: "Regular player should not have GM access",
		},
		{
			name:        "non-admin with admin mode returns false",
			userID:      int32(playerUser.ID),
			isAdmin:     false,
			game:        *game,
			adminMode:   true,
			want:        false,
			description: "Non-admin cannot use admin mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create context with admin mode
			ctx := WithAdminMode(baseCtx, tt.adminMode)

			got := IsUserGameMasterCtx(ctx, tt.userID, tt.isAdmin, tt.game, testDB.Pool)
			if got != tt.want {
				t.Errorf("IsUserGameMasterCtx() = %v, want %v - %s", got, tt.want, tt.description)
			}
		})
	}
}

// TestAdminModeContext tests the WithAdminMode and GetAdminMode context helpers
func TestAdminModeContext(t *testing.T) {
	ctx := context.Background()

	t.Run("admin mode not set returns false", func(t *testing.T) {
		got := GetAdminMode(ctx)
		if got != false {
			t.Errorf("GetAdminMode() on empty context = %v, want false", got)
		}
	})

	t.Run("admin mode set to true returns true", func(t *testing.T) {
		ctx := WithAdminMode(ctx, true)
		got := GetAdminMode(ctx)
		if got != true {
			t.Errorf("GetAdminMode() after WithAdminMode(true) = %v, want true", got)
		}
	})

	t.Run("admin mode set to false returns false", func(t *testing.T) {
		ctx := WithAdminMode(ctx, false)
		got := GetAdminMode(ctx)
		if got != false {
			t.Errorf("GetAdminMode() after WithAdminMode(false) = %v, want false", got)
		}
	})

	t.Run("admin mode can be overwritten", func(t *testing.T) {
		ctx := WithAdminMode(ctx, true)
		ctx = WithAdminMode(ctx, false)
		got := GetAdminMode(ctx)
		if got != false {
			t.Errorf("GetAdminMode() after overwrite = %v, want false", got)
		}
	})
}

// TestCanUserControlNPC tests the NPC control permission check
func TestCanUserControlNPC(t *testing.T) {
	if os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Skip("Skipping integration test - SKIP_DB_TESTS=true")
	}

	testDB := NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()

	// Create test users
	gmUser := testDB.CreateTestUser(t, "gm_user5", "gm5@example.com")
	coGMUser := testDB.CreateTestUser(t, "cogm_user4", "cogm4@example.com")
	assignedUser := testDB.CreateTestUser(t, "assigned_user", "assigned@example.com")
	playerUser := testDB.CreateTestUser(t, "player_user5", "player5@example.com")
	otherPlayerUser := testDB.CreateTestUser(t, "player_user6", "player6@example.com")

	// Create test game
	game := testDB.CreateTestGame(t, int32(gmUser.ID), "Test Game 5")

	// Add co-GM participant
	queries := models.New(testDB.Pool)
	_, err := queries.AddGameParticipant(ctx, models.AddGameParticipantParams{
		GameID: game.ID,
		UserID: int32(coGMUser.ID),
		Role:   "co_gm",
	})
	if err != nil {
		t.Fatalf("Failed to add co-GM participant: %v", err)
	}

	// Create player character (has user_id)
	playerChar, err := queries.CreateCharacter(ctx, models.CreateCharacterParams{
		GameID:        game.ID,
		Name:          "Player Character",
		CharacterType: "player_character",
		UserID:        pgtype.Int4{Int32: int32(playerUser.ID), Valid: true},
		Status:        pgtype.Text{String: "approved", Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to create player character: %v", err)
	}

	// Create NPC (no user_id, unassigned)
	unassignedNPC, err := queries.CreateCharacter(ctx, models.CreateCharacterParams{
		GameID:        game.ID,
		Name:          "Unassigned NPC",
		CharacterType: "npc",
		UserID:        pgtype.Int4{Valid: false}, // NULL
		Status:        pgtype.Text{String: "approved", Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to create unassigned NPC: %v", err)
	}

	// Create NPC and assign to user
	assignedNPC, err := queries.CreateCharacter(ctx, models.CreateCharacterParams{
		GameID:        game.ID,
		Name:          "Assigned NPC",
		CharacterType: "npc",
		UserID:        pgtype.Int4{Valid: false}, // NULL
		Status:        pgtype.Text{String: "approved", Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to create assigned NPC: %v", err)
	}

	// Assign NPC to user
	_, err = queries.AssignNPCToUser(ctx, models.AssignNPCToUserParams{
		CharacterID:      assignedNPC.ID,
		AssignedUserID:   int32(assignedUser.ID),
		AssignedByUserID: int32(gmUser.ID), // GM assigns the NPC
	})
	if err != nil {
		t.Fatalf("Failed to assign NPC: %v", err)
	}

	tests := []struct {
		name        string
		characterID int32
		userID      int32
		want        bool
		description string
	}{
		{
			name:        "player can control their own character",
			characterID: playerChar.ID,
			userID:      int32(playerUser.ID),
			want:        true,
			description: "Player should control their own character",
		},
		{
			name:        "other player cannot control player character",
			characterID: playerChar.ID,
			userID:      int32(otherPlayerUser.ID),
			want:        false,
			description: "Other player should not control someone else's character",
		},
		{
			name:        "assigned user can control assigned NPC",
			characterID: assignedNPC.ID,
			userID:      int32(assignedUser.ID),
			want:        true,
			description: "Assigned user should control their assigned NPC",
		},
		{
			name:        "GM can control unassigned NPC",
			characterID: unassignedNPC.ID,
			userID:      int32(gmUser.ID),
			want:        true,
			description: "GM should control unassigned NPCs",
		},
		{
			name:        "co-GM can control unassigned NPC",
			characterID: unassignedNPC.ID,
			userID:      int32(coGMUser.ID),
			want:        true,
			description: "Co-GM should control unassigned NPCs",
		},
		{
			name:        "GM can control assigned NPC",
			characterID: assignedNPC.ID,
			userID:      int32(gmUser.ID),
			want:        true,
			description: "GM should control all NPCs including assigned ones",
		},
		{
			name:        "co-GM can control assigned NPC",
			characterID: assignedNPC.ID,
			userID:      int32(coGMUser.ID),
			want:        true,
			description: "Co-GM should control all NPCs including assigned ones",
		},
		{
			name:        "regular player cannot control unassigned NPC",
			characterID: unassignedNPC.ID,
			userID:      int32(playerUser.ID),
			want:        false,
			description: "Regular player should not control unassigned NPCs",
		},
		{
			name:        "non-assigned user cannot control assigned NPC",
			characterID: assignedNPC.ID,
			userID:      int32(playerUser.ID),
			want:        false,
			description: "Non-assigned user should not control assigned NPC",
		},
		{
			name:        "invalid character ID returns false",
			characterID: 99999,
			userID:      int32(gmUser.ID),
			want:        false,
			description: "Invalid character should return false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanUserControlNPC(ctx, testDB.Pool, tt.characterID, tt.userID)
			if got != tt.want {
				t.Errorf("CanUserControlNPC() = %v, want %v - %s", got, tt.want, tt.description)
			}
		})
	}
}
