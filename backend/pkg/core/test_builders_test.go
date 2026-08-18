package core

import (
	"testing"
	"time"
)

func TestCharacterBuilder(t *testing.T) {
	testDB := NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "games", "sessions", "users")

	factory := NewTestDataFactory(testDB, t)

	// Create test user and game
	user := factory.NewUser().WithUsername("testplayer").Create()
	game := factory.NewGame().WithGM(user.ID).Create()

	t.Run("creates player character with defaults", func(t *testing.T) {
		character := factory.NewCharacter().
			InGame(game).
			OwnedBy(user).
			Create()

		AssertEqual(t, game.ID, character.GameID, "Game ID should match")
		AssertEqual(t, user.ID, character.UserID.Int32, "User ID should match")
		AssertEqual(t, "player_character", character.CharacterType, "Should be player character")
		AssertEqual(t, "pending", character.Status.String, "Should be pending status")
	})

	t.Run("creates NPC with GM control", func(t *testing.T) {
		npc := factory.NewCharacter().
			InGame(game).
			GMControlled().
			WithName("Important NPC").
			NPCGMControlled().
			Create()

		AssertEqual(t, game.ID, npc.GameID, "Game ID should match")
		AssertEqual(t, false, npc.UserID.Valid, "NPC should not have user ID")
		AssertEqual(t, "npc", npc.CharacterType, "Should be GM NPC")
		AssertEqual(t, "Important NPC", npc.Name, "Name should match")
	})

	t.Run("creates approved player character", func(t *testing.T) {
		character := factory.NewCharacter().
			InGame(game).
			OwnedBy(user).
			WithName("Aragorn").
			Approved().
			Create()

		AssertEqual(t, "Aragorn", character.Name, "Name should match")
		AssertEqual(t, "approved", character.Status.String, "Should be approved")
	})

	t.Run("creates audience NPC", func(t *testing.T) {
		npc := factory.NewCharacter().
			InGame(game).
			NPCAudience().
			WithName("Crowd Member").
			Create()

		AssertEqual(t, "npc", npc.CharacterType, "Should be audience NPC")
		AssertEqual(t, "Crowd Member", npc.Name, "Name should match")
	})
}

func TestPhaseBuilder(t *testing.T) {
	testDB := NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_phases", "games", "sessions", "users")

	factory := NewTestDataFactory(testDB, t)

	// Create test user and game
	user := factory.NewUser().Create()
	game := factory.NewGame().WithGM(user.ID).Create()

	t.Run("creates phase with defaults", func(t *testing.T) {
		phase := factory.NewPhase().
			InGame(game).
			Create()

		AssertEqual(t, game.ID, phase.GameID, "Game ID should match")
		AssertEqual(t, "common_room", phase.PhaseType, "Default should be common_room")
		AssertEqual(t, int32(1), phase.PhaseNumber, "Should be phase 1")
		AssertEqual(t, false, phase.IsActive.Bool, "Should not be active by default")
	})

	t.Run("creates action phase with deadline", func(t *testing.T) {
		phase := factory.NewPhase().
			InGame(game).
			ActionPhase().
			WithTitle("Planning Phase").
			WithDeadlineIn(48 * time.Hour).
			Create()

		AssertEqual(t, "action", phase.PhaseType, "Should be action phase")
		AssertEqual(t, "Planning Phase", phase.Title, "Title should match")
		AssertTrue(t, phase.Deadline.Valid, "Deadline should be set")
	})

	t.Run("creates active action phase", func(t *testing.T) {
		phase := factory.NewPhase().
			InGame(game).
			ActionPhase().
			WithTitle("Action Required").
			Active().
			Create()

		AssertEqual(t, "action", phase.PhaseType, "Should be action phase")
		AssertEqual(t, true, phase.IsActive.Bool, "Should be active")
	})

	t.Run("auto-increments phase numbers", func(t *testing.T) {
		// Create fresh game for this test
		freshGame := factory.NewGame().WithGM(user.ID).Create()

		// Create first phase
		phase1 := factory.NewPhase().InGame(freshGame).Create()
		AssertEqual(t, int32(1), phase1.PhaseNumber, "First phase should be 1")

		// Create second phase - should auto-increment
		phase2 := factory.NewPhase().InGame(freshGame).Create()
		AssertEqual(t, int32(2), phase2.PhaseNumber, "Second phase should be 2")

		// Create third phase - should auto-increment
		phase3 := factory.NewPhase().InGame(freshGame).Create()
		AssertEqual(t, int32(3), phase3.PhaseNumber, "Third phase should be 3")
	})

	t.Run("respects manual phase number", func(t *testing.T) {
		phase := factory.NewPhase().
			InGame(game).
			WithPhaseNumber(10).
			Create()

		AssertEqual(t, int32(10), phase.PhaseNumber, "Should use manual phase number")
	})

	t.Run("creates phase with time range", func(t *testing.T) {
		phase := factory.NewPhase().
			InGame(game).
			WithTimeRange(72 * time.Hour).
			Create()

		AssertTrue(t, phase.StartTime.Valid, "Start time should be set")
		AssertTrue(t, phase.EndTime.Valid, "End time should be set")
	})

	t.Run("creates phase with description", func(t *testing.T) {
		phase := factory.NewPhase().
			InGame(game).
			WithTitle("Gathering Clues").
			WithDescription("Search the mansion for evidence").
			Create()

		AssertEqual(t, "Gathering Clues", phase.Title, "Title should match")
		AssertTrue(t, phase.Description.Valid, "Description should be set")
		AssertEqual(t, "Search the mansion for evidence", phase.Description.String, "Description should match")
	})
}

func TestCharacterBuilderIntegration(t *testing.T) {
	testDB := NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "games", "sessions", "users")

	factory := NewTestDataFactory(testDB, t)

	// Create a full scenario with multiple characters
	gm := factory.NewUser().WithUsername("gamemaster").Create()
	game := factory.NewGame().WithGM(gm.ID).WithTitle("Epic Quest").Create()

	player1 := factory.NewUser().WithUsername("player1").Create()
	player2 := factory.NewUser().WithUsername("player2").Create()

	t.Run("creates complex character scenario", func(t *testing.T) {
		// Create player characters
		char1 := factory.NewCharacter().
			InGame(game).
			OwnedBy(player1).
			WithName("Warrior").
			Approved().
			Create()

		char2 := factory.NewCharacter().
			InGame(game).
			OwnedBy(player2).
			WithName("Mage").
			Approved().
			Create()

		// Create GM-controlled NPCs
		villain := factory.NewCharacter().
			InGame(game).
			NPCGMControlled().
			WithName("Dark Lord").
			Approved().
			Create()

		guide := factory.NewCharacter().
			InGame(game).
			NPCAudience().
			WithName("Wise Elder").
			Approved().
			Create()

		// Verify all characters created correctly
		AssertEqual(t, "Warrior", char1.Name, "Character 1 name should match")
		AssertEqual(t, "Mage", char2.Name, "Character 2 name should match")
		AssertEqual(t, "Dark Lord", villain.Name, "Villain name should match")
		AssertEqual(t, "Wise Elder", guide.Name, "Guide name should match")

		AssertEqual(t, "player_character", char1.CharacterType, "Char1 should be player character")
		AssertEqual(t, "player_character", char2.CharacterType, "Char2 should be player character")
		AssertEqual(t, "npc", villain.CharacterType, "Villain should be GM NPC")
		AssertEqual(t, "npc", guide.CharacterType, "Guide should be audience NPC")
	})
}

func TestPhaseBuilderIntegration(t *testing.T) {
	testDB := NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_phases", "games", "sessions", "users")

	factory := NewTestDataFactory(testDB, t)

	// Create test game
	gm := factory.NewUser().Create()
	game := factory.NewGame().WithGM(gm.ID).Create()

	t.Run("creates complete phase sequence", func(t *testing.T) {
		// Phase 1: Introduction (common room)
		intro := factory.NewPhase().
			InGame(game).
			CommonRoom().
			WithTitle("Introduction").
			WithDescription("Meet the other characters").
			WithTimeRange(24 * time.Hour).
			Active().
			Create()

		AssertEqual(t, int32(1), intro.PhaseNumber, "Introduction should be phase 1")
		AssertEqual(t, true, intro.IsActive.Bool, "Introduction should be active")

		// Phase 2: Investigation (action)
		investigation := factory.NewPhase().
			InGame(game).
			ActionPhase().
			WithTitle("Investigation").
			WithDescription("Search for clues").
			WithDeadlineIn(48 * time.Hour).
			Create()

		AssertEqual(t, int32(2), investigation.PhaseNumber, "Investigation should be phase 2")
		AssertEqual(t, "action", investigation.PhaseType, "Should be action phase")

		// Phase 3: Confrontation (action)
		confrontation := factory.NewPhase().
			InGame(game).
			ActionPhase().
			WithTitle("Confrontation").
			WithDescription("Face the antagonist").
			Create()

		AssertEqual(t, int32(3), confrontation.PhaseNumber, "Confrontation should be phase 3")

		// Phase 4: Resolution (common room for discussion)
		resolution := factory.NewPhase().
			InGame(game).
			CommonRoom().
			WithTitle("Resolution").
			Create()

		AssertEqual(t, int32(4), resolution.PhaseNumber, "Resolution should be phase 4")
		AssertEqual(t, "common_room", resolution.PhaseType, "Should be common room phase")
	})
}

func TestActionSubmissionBuilder(t *testing.T) {
	testDB := NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_submissions", "game_phases", "games", "sessions", "users")

	factory := NewTestDataFactory(testDB, t)

	t.Run("creates action submission with defaults", func(t *testing.T) {
		user := factory.NewUser().Create()
		game := factory.NewGame().WithGM(user.ID).Create()
		phase := factory.NewPhase().InGame(game).ActionPhase().Active().Create()

		submission := factory.NewActionSubmission().
			InPhase(phase).
			ByUser(user).
			Create()

		AssertEqual(t, game.ID, submission.GameID, "Game ID should match")
		AssertEqual(t, user.ID, submission.UserID, "User ID should match")
		AssertEqual(t, phase.ID, submission.PhaseID, "Phase ID should match")
	})

	t.Run("stamps every submission with a submission time", func(t *testing.T) {
		user := factory.NewUser().Create()
		game := factory.NewGame().WithGM(user.ID).Create()
		phase := factory.NewPhase().InGame(game).ActionPhase().Active().Create()

		submission := factory.NewActionSubmission().
			InPhase(phase).
			ByUser(user).
			WithContent("I search the ancient library for clues").
			Create()

		AssertEqual(t, "I search the ancient library for clues", submission.Content, "Content should match")
		AssertTrue(t, submission.SubmittedAt.Valid, "Submission should have submission time")
	})

	t.Run("creates submission with character", func(t *testing.T) {
		user := factory.NewUser().Create()
		game := factory.NewGame().WithGM(user.ID).Create()
		phase := factory.NewPhase().InGame(game).ActionPhase().Active().Create()
		character := factory.NewCharacter().InGame(game).OwnedBy(user).Create()

		submission := factory.NewActionSubmission().
			InPhase(phase).
			ByUser(user).
			AsCharacter(character).
			WithContent("My character performs this action").
			Create()

		AssertTrue(t, submission.CharacterID.Valid, "Should have character ID")
		AssertEqual(t, character.ID, submission.CharacterID.Int32, "Character ID should match")
	})
}

func TestMessageBuilder(t *testing.T) {
	testDB := NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_phases", "games", "sessions", "users")

	factory := NewTestDataFactory(testDB, t)

	// Create test data
	user := factory.NewUser().Create()
	game := factory.NewGame().WithGM(user.ID).Create()
	character := factory.NewCharacter().InGame(game).OwnedBy(user).Approved().Create()
	phase := factory.NewPhase().InGame(game).CommonRoom().Active().Create()

	t.Run("creates post with defaults", func(t *testing.T) {
		post := factory.NewPost().
			InGame(game).
			ByCharacter(character).
			Create()

		AssertEqual(t, game.ID, post.GameID, "Game ID should match")
		AssertEqual(t, character.ID, post.CharacterID, "Character ID should match")
		AssertEqual(t, "post", string(post.MessageType), "Message type should be post")
		AssertEqual(t, "game", string(post.Visibility), "Default visibility should be game")
		AssertEqual(t, false, post.ParentID.Valid, "Post should not have parent")
	})

	t.Run("creates post with custom content", func(t *testing.T) {
		post := factory.NewPost().
			InGame(game).
			ByCharacter(character).
			WithContent("The dragon stirs from its centuries-long slumber...").
			Create()

		AssertEqual(t, "The dragon stirs from its centuries-long slumber...", post.Content, "Content should match")
	})

	t.Run("creates post in phase", func(t *testing.T) {
		post := factory.NewPost().
			InPhase(phase).
			ByCharacter(character).
			Create()

		AssertTrue(t, post.PhaseID.Valid, "Post should have phase ID")
		AssertEqual(t, phase.ID, post.PhaseID.Int32, "Phase ID should match")
	})

	t.Run("creates private post", func(t *testing.T) {
		post := factory.NewPost().
			InGame(game).
			ByCharacter(character).
			Private().
			Create()

		AssertEqual(t, "private", string(post.Visibility), "Visibility should be private")
	})

	t.Run("creates post with character mentions", func(t *testing.T) {
		otherChar := factory.NewCharacter().InGame(game).OwnedBy(user).Create()

		post := factory.NewPost().
			InGame(game).
			ByCharacter(character).
			WithContent("Hey @OtherCharacter, what do you think?").
			MentioningCharacters(otherChar.ID).
			Create()

		AssertEqual(t, 1, len(post.MentionedCharacterIds), "Should have 1 mentioned character")
		AssertEqual(t, otherChar.ID, post.MentionedCharacterIds[0], "Mentioned character ID should match")
	})

	t.Run("creates comment on post", func(t *testing.T) {
		post := factory.NewPost().
			InGame(game).
			ByCharacter(character).
			Create()

		comment := factory.NewComment().
			OnPost(post).
			ByCharacter(character).
			WithContent("Great idea!").
			Create()

		AssertEqual(t, "comment", string(comment.MessageType), "Message type should be comment")
		AssertTrue(t, comment.ParentID.Valid, "Comment should have parent ID")
		AssertEqual(t, post.ID, comment.ParentID.Int32, "Parent ID should match post")
		AssertEqual(t, game.ID, comment.GameID, "Game ID should be inherited from post")
	})

	t.Run("creates nested comments", func(t *testing.T) {
		post := factory.NewPost().
			InGame(game).
			ByCharacter(character).
			Create()

		comment1 := factory.NewComment().
			OnPost(post).
			ByCharacter(character).
			WithContent("First level comment").
			Create()

		comment2 := factory.NewComment().
			InGame(game).
			ByCharacter(character).
			WithParentID(comment1.ID).
			WithContent("Reply to comment").
			Create()

		AssertEqual(t, comment1.ID, comment2.ParentID.Int32, "Second comment should reply to first")
		AssertTrue(t, comment2.ThreadDepth > comment1.ThreadDepth, "Thread depth should increase")
	})
}

func TestActionSubmissionBuilderIntegration(t *testing.T) {
	testDB := NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_submissions", "characters", "game_phases", "games", "sessions", "users")

	factory := NewTestDataFactory(testDB, t)

	// Create complete scenario
	gm := factory.NewUser().WithUsername("gm").Create()
	game := factory.NewGame().WithGM(gm.ID).WithTitle("Mystery Mansion").Create()

	player1 := factory.NewUser().WithUsername("player1").Create()
	player2 := factory.NewUser().WithUsername("player2").Create()

	char1 := factory.NewCharacter().InGame(game).OwnedBy(player1).WithName("Detective").Approved().Create()
	char2 := factory.NewCharacter().InGame(game).OwnedBy(player2).WithName("Investigator").Approved().Create()

	phase := factory.NewPhase().InGame(game).ActionPhase().WithTitle("Investigation").Active().Create()

	t.Run("multiple players submit actions", func(t *testing.T) {
		// Player 1 submits with character
		action1 := factory.NewActionSubmission().
			InPhase(phase).
			ByUser(player1).
			AsCharacter(char1).
			WithContent("I examine the study for fingerprints").
			Create()

		// Player 2 submits with character
		action2 := factory.NewActionSubmission().
			InPhase(phase).
			ByUser(player2).
			AsCharacter(char2).
			WithContent("I search the library for hidden passages").
			Create()

		AssertEqual(t, phase.ID, action1.PhaseID, "Action 1 should be in phase")
		AssertEqual(t, phase.ID, action2.PhaseID, "Action 2 should be in phase")
		AssertEqual(t, char1.ID, action1.CharacterID.Int32, "Action 1 character should match")
		AssertEqual(t, char2.ID, action2.CharacterID.Int32, "Action 2 character should match")
	})
}

func TestMessageBuilderIntegration(t *testing.T) {
	testDB := NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_phases", "games", "sessions", "users")

	factory := NewTestDataFactory(testDB, t)

	// Create complete scenario
	gm := factory.NewUser().WithUsername("gm").Create()
	game := factory.NewGame().WithGM(gm.ID).Create()

	player1 := factory.NewUser().WithUsername("player1").Create()
	player2 := factory.NewUser().WithUsername("player2").Create()

	char1 := factory.NewCharacter().InGame(game).OwnedBy(player1).WithName("Wizard").Approved().Create()
	char2 := factory.NewCharacter().InGame(game).OwnedBy(player2).WithName("Warrior").Approved().Create()

	phase := factory.NewPhase().InGame(game).CommonRoom().WithTitle("Tavern").Active().Create()

	t.Run("creates conversation with mentions", func(t *testing.T) {
		// Character 1 posts
		post := factory.NewPost().
			InPhase(phase).
			ByCharacter(char1).
			WithContent("Greetings, fellow adventurers! @Warrior, shall we quest together?").
			MentioningCharacters(char2.ID).
			Create()

		// Character 2 replies
		reply := factory.NewComment().
			OnPost(post).
			ByCharacter(char2).
			WithContent("@Wizard, I would be honored to join your quest!").
			MentioningCharacters(char1.ID).
			Create()

		AssertEqual(t, phase.ID, post.PhaseID.Int32, "Post should be in phase")
		AssertEqual(t, 1, len(post.MentionedCharacterIds), "Post should mention 1 character")
		AssertEqual(t, char2.ID, post.MentionedCharacterIds[0], "Should mention Warrior")

		AssertEqual(t, post.ID, reply.ParentID.Int32, "Reply should reference post")
		AssertEqual(t, 1, len(reply.MentionedCharacterIds), "Reply should mention 1 character")
		AssertEqual(t, char1.ID, reply.MentionedCharacterIds[0], "Should mention Wizard")
	})
}
