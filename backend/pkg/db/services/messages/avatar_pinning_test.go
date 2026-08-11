package messages

import (
	"context"
	"testing"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	db "actionphase/pkg/db/services"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Avatars are narrative content in some games (a character growing progressively
// more injured, a shapeshifter, a costume change). Message read paths used to
// join characters.avatar_url live, so changing an avatar silently repainted every
// post that character had ever written. These tests pin that behavior down:
// a message renders the avatar its author had at the moment it was written.

// setAvatar points a character's current avatar at url.
func setAvatar(t *testing.T, pool *pgxpool.Pool, characterID int32, url string) {
	t.Helper()
	queries := models.New(pool)
	_, err := queries.UpdateCharacterAvatar(context.Background(), models.UpdateCharacterAvatarParams{
		ID:        characterID,
		AvatarUrl: pgtype.Text{String: url, Valid: true},
	})
	core.AssertNoError(t, err, "Failed to set character avatar")
}

// TestPostAvatarPinnedAtCreation is the core guarantee: a post keeps the avatar
// it was written with even after the character uploads a new one.
func TestPostAvatarPinnedAtCreation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)
	fixtures := testDB.SetupFixtures(t)
	msgService := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	ctx := context.Background()
	gameID := fixtures.TestGame.ID

	player := testDB.CreateTestUser(t, "pin_player", "pin_player@example.com")
	userID := int32(player.ID)
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	character, err := characterService.CreateCharacter(ctx, db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &userID,
		Name:          "Uninjured Hero",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create character")

	const healthyAvatar = "http://localhost:3000/uploads/avatars/characters/1/100.png"
	const injuredAvatar = "http://localhost:3000/uploads/avatars/characters/1/200.png"

	// Write a post while the character still looks healthy.
	setAvatar(t, testDB.Pool, character.ID, healthyAvatar)
	post, err := msgService.CreatePost(ctx, core.CreatePostRequest{
		GameID:      gameID,
		AuthorID:    int32(player.ID),
		CharacterID: character.ID,
		Content:     "I charge into battle, unscathed.",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Failed to create post")

	// The character is wounded and uploads new art.
	setAvatar(t, testDB.Pool, character.ID, injuredAvatar)

	fetched, err := msgService.GetPost(ctx, post.ID)
	core.AssertNoError(t, err, "Failed to fetch post")
	core.AssertTrue(t, fetched.CharacterAvatarUrl != nil, "Post should carry an avatar URL")
	core.AssertEqual(t, healthyAvatar, *fetched.CharacterAvatarUrl,
		"Post should render the avatar it was written with, not the character's current one")
}

// TestCommentAvatarPinnedAtCreation covers the comment write path, which uses a
// separate INSERT from posts.
func TestCommentAvatarPinnedAtCreation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)
	fixtures := testDB.SetupFixtures(t)
	msgService := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	ctx := context.Background()
	gameID := fixtures.TestGame.ID

	player := testDB.CreateTestUser(t, "pin_commenter", "pin_commenter@example.com")
	userID := int32(player.ID)
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	character, err := characterService.CreateCharacter(ctx, db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &userID,
		Name:          "Shifting Creature",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create character")

	const dayForm = "http://localhost:3000/uploads/avatars/characters/2/300.png"
	const nightForm = "http://localhost:3000/uploads/avatars/characters/2/400.png"

	setAvatar(t, testDB.Pool, character.ID, dayForm)
	post, err := msgService.CreatePost(ctx, core.CreatePostRequest{
		GameID:      gameID,
		AuthorID:    int32(player.ID),
		CharacterID: character.ID,
		Content:     "A post to reply to.",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Failed to create post")

	comment, err := msgService.CreateComment(ctx, core.CreateCommentRequest{
		GameID:      gameID,
		AuthorID:    int32(player.ID),
		CharacterID: character.ID,
		Content:     "In daylight I appear as I always have.",
		ParentID:    post.ID,
		RootPostID:  post.ID,
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Failed to create comment")

	// Night falls; the creature changes shape.
	setAvatar(t, testDB.Pool, character.ID, nightForm)

	comments, err := msgService.GetPostComments(ctx, post.ID)
	core.AssertNoError(t, err, "Failed to fetch comments")

	var found bool
	for _, c := range comments {
		if c.ID == comment.ID {
			found = true
			core.AssertTrue(t, c.CharacterAvatarUrl != nil, "Comment should carry an avatar URL")
			core.AssertEqual(t, dayForm, *c.CharacterAvatarUrl,
				"Comment should render the avatar it was written with")
		}
	}
	core.AssertTrue(t, found, "Created comment should appear in the post's comments")
}

// TestEditingPostDoesNotRepaintAvatar guards the "pin once, never update" rule:
// editing a message's text months later must not adopt the current avatar.
func TestEditingPostDoesNotRepaintAvatar(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)
	fixtures := testDB.SetupFixtures(t)
	msgService := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	ctx := context.Background()
	gameID := fixtures.TestGame.ID

	player := testDB.CreateTestUser(t, "pin_editor", "pin_editor@example.com")
	userID := int32(player.ID)
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	character, err := characterService.CreateCharacter(ctx, db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &userID,
		Name:          "Costume Changer",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create character")

	const firstOutfit = "http://localhost:3000/uploads/avatars/characters/3/500.png"
	const secondOutfit = "http://localhost:3000/uploads/avatars/characters/3/600.png"

	setAvatar(t, testDB.Pool, character.ID, firstOutfit)
	post, err := msgService.CreatePost(ctx, core.CreatePostRequest{
		GameID:      gameID,
		AuthorID:    int32(player.ID),
		CharacterID: character.ID,
		Content:     "Original text.",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Failed to create post")

	setAvatar(t, testDB.Pool, character.ID, secondOutfit)

	_, err = msgService.UpdatePost(ctx, post.ID, "Fixed a typo.")
	core.AssertNoError(t, err, "Failed to update post")

	fetched, err := msgService.GetPost(ctx, post.ID)
	core.AssertNoError(t, err, "Failed to fetch post after edit")
	core.AssertTrue(t, fetched.CharacterAvatarUrl != nil, "Post should carry an avatar URL")
	core.AssertEqual(t, firstOutfit, *fetched.CharacterAvatarUrl,
		"Editing a post must not repaint its avatar")
}

// TestUnpinnedMessagesFallBackToCurrentAvatar covers the COALESCE fallback that
// lets this ship without a backfill: rows predating the column (NULL pin) keep
// rendering the character's live avatar rather than showing nothing.
func TestUnpinnedMessagesFallBackToCurrentAvatar(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)
	fixtures := testDB.SetupFixtures(t)
	msgService := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	ctx := context.Background()
	gameID := fixtures.TestGame.ID

	player := testDB.CreateTestUser(t, "pin_legacy", "pin_legacy@example.com")
	userID := int32(player.ID)
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	character, err := characterService.CreateCharacter(ctx, db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &userID,
		Name:          "Legacy Poster",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create character")

	const currentAvatar = "http://localhost:3000/uploads/avatars/characters/4/700.png"

	setAvatar(t, testDB.Pool, character.ID, currentAvatar)
	post, err := msgService.CreatePost(ctx, core.CreatePostRequest{
		GameID:      gameID,
		AuthorID:    int32(player.ID),
		CharacterID: character.ID,
		Content:     "A post from before avatars were pinned.",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Failed to create post")

	// Simulate a pre-migration row by clearing the pin.
	_, err = testDB.Pool.Exec(ctx,
		"UPDATE messages SET character_avatar_url_at_post = NULL WHERE id = $1", post.ID)
	core.AssertNoError(t, err, "Failed to clear pinned avatar")

	fetched, err := msgService.GetPost(ctx, post.ID)
	core.AssertNoError(t, err, "Failed to fetch post")
	core.AssertTrue(t, fetched.CharacterAvatarUrl != nil,
		"An unpinned post should fall back to the character's current avatar")
	core.AssertEqual(t, currentAvatar, *fetched.CharacterAvatarUrl,
		"Unpinned rows should read through to characters.avatar_url")
}

// TestPostWithoutAvatarPinsNothing confirms a character with no avatar produces a
// NULL pin (and a nil URL) rather than an empty string sentinel.
func TestPostWithoutAvatarPinsNothing(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)
	fixtures := testDB.SetupFixtures(t)
	msgService := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	ctx := context.Background()
	gameID := fixtures.TestGame.ID

	player := testDB.CreateTestUser(t, "pin_noavatar", "pin_noavatar@example.com")
	userID := int32(player.ID)
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	character, err := characterService.CreateCharacter(ctx, db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &userID,
		Name:          "Avatarless",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create character")

	post, err := msgService.CreatePost(ctx, core.CreatePostRequest{
		GameID:      gameID,
		AuthorID:    int32(player.ID),
		CharacterID: character.ID,
		Content:     "No avatar here.",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Failed to create post")

	fetched, err := msgService.GetPost(ctx, post.ID)
	core.AssertNoError(t, err, "Failed to fetch post")
	core.AssertTrue(t, fetched.CharacterAvatarUrl == nil,
		"A post by a character with no avatar should have no avatar URL")

	// Later giving the character an avatar must not retroactively fill in the post:
	// the pin is NULL, so it falls through to the (now set) live avatar. This
	// documents the known limitation of the COALESCE fallback.
	const laterAvatar = "http://localhost:3000/uploads/avatars/characters/5/800.png"
	setAvatar(t, testDB.Pool, character.ID, laterAvatar)

	refetched, err := msgService.GetPost(ctx, post.ID)
	core.AssertNoError(t, err, "Failed to re-fetch post")
	core.AssertTrue(t, refetched.CharacterAvatarUrl != nil,
		"Fallback: an unpinned post adopts the character's avatar once one exists")
}
