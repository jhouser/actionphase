package messages

import (
	"context"
	"testing"

	core "actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	db "actionphase/pkg/db/services"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression: comments created without an explicit PhaseID must inherit their
// parent's phase.
//
// CreateCommentRequest.PhaseID is documented as "Optional - inherits from
// parent", but the inheritance was never implemented — the request value was
// passed straight through, so a nil PhaseID stored NULL. Two frontend reply
// surfaces omit phase_id because they render flat, cross-phase comment lists
// and have no phase in scope:
//   - the Dashboard unread inbox (utils/unreadInboxApi.ts replyToComment)
//   - the New Comments view (components/CommentWithParentCard.tsx)
//
// Replies posted from either landed with phase_id = NULL. That renders fine
// inline, but breaks deep linking: GetMessage serialises phase_id with
// omitempty, so the field is absent from the response, and HistoryView's
// resolver can't tell which phase to open. The notification opens the History
// tab with no phase and no comment loaded.
//
// A reply always belongs to the same phase as what it replies to, so the server
// derives it rather than trusting every caller to send it.
func TestMessageService_CreateComment_InheritsPhaseFromParent(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	service := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	ctx := context.Background()

	gm := testDB.CreateTestUser(t, "phase_inherit_gm", "phase_inherit_gm@example.com")
	player := testDB.CreateTestUser(t, "phase_inherit_player", "phase_inherit_player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Phase Inheritance Game")

	_, err := gameService.AddGameParticipant(ctx, game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	char, err := characterService.CreateCharacter(ctx, db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player.ID)),
		Name:          "Phase Inheritance Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	phase := testDB.CreateTestPhase(t, game.ID, "common_room", "Common Room Phase")

	// A post authored in the phase — the same state CommonRoom produces.
	post, err := service.CreatePost(ctx, core.CreatePostRequest{
		GameID:      game.ID,
		PhaseID:     &phase.ID,
		AuthorID:    int32(player.ID),
		CharacterID: char.ID,
		Content:     "Root post in a phase",
		Visibility:  string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)
	require.True(t, post.PhaseID.Valid, "precondition: root post must have a phase")

	t.Run("top-level comment with no PhaseID inherits the post's phase", func(t *testing.T) {
		// PhaseID omitted, exactly as the unread-inbox reply path sends it.
		comment, err := service.CreateComment(ctx, core.CreateCommentRequest{
			GameID:      game.ID,
			AuthorID:    int32(player.ID),
			CharacterID: char.ID,
			Content:     "Reply posted from the unread inbox",
			ParentID:    post.ID,
			RootPostID:  post.ID,
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		assert.True(t, comment.PhaseID.Valid,
			"comment must inherit a phase from its parent, otherwise deep links cannot resolve it")
		assert.Equal(t, phase.ID, comment.PhaseID.Int32)
	})

	t.Run("nested reply with no PhaseID inherits through the chain", func(t *testing.T) {
		parent, err := service.CreateComment(ctx, core.CreateCommentRequest{
			GameID:      game.ID,
			PhaseID:     &phase.ID,
			AuthorID:    int32(player.ID),
			CharacterID: char.ID,
			Content:     "Parent comment",
			ParentID:    post.ID,
			RootPostID:  post.ID,
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// A deep reply from the New Comments view — the prod case (thread_depth 5).
		reply, err := service.CreateComment(ctx, core.CreateCommentRequest{
			GameID:      game.ID,
			AuthorID:    int32(player.ID),
			CharacterID: char.ID,
			Content:     "Nested reply posted from the New Comments view",
			ParentID:    parent.ID,
			RootPostID:  post.ID,
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		assert.True(t, reply.PhaseID.Valid)
		assert.Equal(t, phase.ID, reply.PhaseID.Int32)
	})

	t.Run("an explicit PhaseID is preserved, not overwritten by the parent's", func(t *testing.T) {
		otherPhase := testDB.CreateTestPhase(t, game.ID, "action", "Other Phase")

		comment, err := service.CreateComment(ctx, core.CreateCommentRequest{
			GameID:      game.ID,
			PhaseID:     &otherPhase.ID,
			AuthorID:    int32(player.ID),
			CharacterID: char.ID,
			Content:     "Comment with an explicit phase",
			ParentID:    post.ID,
			RootPostID:  post.ID,
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		assert.True(t, comment.PhaseID.Valid)
		assert.Equal(t, otherPhase.ID, comment.PhaseID.Int32,
			"an explicitly supplied phase must win over inheritance")
	})

	t.Run("a parent with no phase leaves the comment's phase NULL", func(t *testing.T) {
		// Legacy rows predate phase tracking; inheritance must not invent a phase.
		phaselessPost, err := service.CreatePost(ctx, core.CreatePostRequest{
			GameID:      game.ID,
			AuthorID:    int32(player.ID),
			CharacterID: char.ID,
			Content:     "Post with no phase",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)
		require.False(t, phaselessPost.PhaseID.Valid, "precondition: parent has no phase")

		comment, err := service.CreateComment(ctx, core.CreateCommentRequest{
			GameID:      game.ID,
			AuthorID:    int32(player.ID),
			CharacterID: char.ID,
			Content:     "Reply to a phaseless post",
			ParentID:    phaselessPost.ID,
			RootPostID:  phaselessPost.ID,
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		assert.False(t, comment.PhaseID.Valid,
			"nothing to inherit; the comment must stay NULL rather than guess")
		assert.Equal(t, pgtype.Int4{}, comment.PhaseID)
	})
}
