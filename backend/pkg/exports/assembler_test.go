package exports

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
	"testing"

	core "actionphase/pkg/core"
	models "actionphase/pkg/db/models"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeQuerier serves canned rows so assembly can be tested without a database.
type fakeQuerier struct {
	game          models.Game
	gameErr       error
	phases        []models.ListExportPhasesRow
	characters    []models.ListExportCharactersRow
	characterData []models.ListExportCharacterDataRow
	participants  []models.ListExportParticipantsRow
	posts         []models.ListExportPostsRow
	comments      map[int32][]models.ListExportCommentTreeRow
	conversations []models.ListExportConversationsRow
	convParts     []models.ListExportConversationParticipantsRow
	privateMsgs   []models.ListExportPrivateMessagesRow
	submissions   []models.ListExportActionSubmissionsRow
	results       []models.ListExportActionResultsRow
	handouts      []models.ListExportHandoutsRow
	polls         []models.ListExportPollsRow
	pollOptions   []models.ListExportPollOptionsRow
	pollVotes     []models.ListExportPollVotesRow
	fingerprint   models.GetGameContentFingerprintRow

	postsErr error
}

func (f *fakeQuerier) GetGame(context.Context, int32) (models.Game, error) {
	return f.game, f.gameErr
}
func (f *fakeQuerier) ListExportPhases(context.Context, int32) ([]models.ListExportPhasesRow, error) {
	return f.phases, nil
}
func (f *fakeQuerier) ListExportCharacters(context.Context, int32) ([]models.ListExportCharactersRow, error) {
	return f.characters, nil
}
func (f *fakeQuerier) ListExportCharacterData(context.Context, int32) ([]models.ListExportCharacterDataRow, error) {
	return f.characterData, nil
}
func (f *fakeQuerier) ListExportParticipants(context.Context, int32) ([]models.ListExportParticipantsRow, error) {
	return f.participants, nil
}
func (f *fakeQuerier) ListExportPosts(context.Context, int32) ([]models.ListExportPostsRow, error) {
	return f.posts, f.postsErr
}
func (f *fakeQuerier) ListExportCommentTree(_ context.Context, parentID pgtype.Int4) ([]models.ListExportCommentTreeRow, error) {
	return f.comments[parentID.Int32], nil
}
func (f *fakeQuerier) ListExportConversations(context.Context, int32) ([]models.ListExportConversationsRow, error) {
	return f.conversations, nil
}
func (f *fakeQuerier) ListExportConversationParticipants(context.Context, int32) ([]models.ListExportConversationParticipantsRow, error) {
	return f.convParts, nil
}
func (f *fakeQuerier) ListExportPrivateMessages(context.Context, int32) ([]models.ListExportPrivateMessagesRow, error) {
	return f.privateMsgs, nil
}
func (f *fakeQuerier) ListExportActionSubmissions(context.Context, int32) ([]models.ListExportActionSubmissionsRow, error) {
	return f.submissions, nil
}
func (f *fakeQuerier) ListExportActionResults(context.Context, int32) ([]models.ListExportActionResultsRow, error) {
	return f.results, nil
}
func (f *fakeQuerier) ListExportHandouts(context.Context, int32) ([]models.ListExportHandoutsRow, error) {
	return f.handouts, nil
}
func (f *fakeQuerier) ListExportPolls(context.Context, int32) ([]models.ListExportPollsRow, error) {
	return f.polls, nil
}
func (f *fakeQuerier) ListExportPollOptions(context.Context, int32) ([]models.ListExportPollOptionsRow, error) {
	return f.pollOptions, nil
}
func (f *fakeQuerier) ListExportPollVotes(context.Context, int32) ([]models.ListExportPollVotesRow, error) {
	return f.pollVotes, nil
}
func (f *fakeQuerier) GetGameContentFingerprint(context.Context, int32) (models.GetGameContentFingerprintRow, error) {
	return f.fingerprint, nil
}

// completedGame returns a fake preloaded with a small but complete game.
func completedGame() *fakeQuerier {
	return &fakeQuerier{
		game: models.Game{
			ID:          164,
			Title:       "The Hollow Crown",
			Description: txt("A mystery."),
			State:       txt(core.GameStateCompleted),
			Genre:       txt("Gothic"),
			StartDate:   tstz(0),
			EndDate:     tstz(9000),
		},
		phases: []models.ListExportPhasesRow{
			{ID: 1, PhaseType: "common_room", PhaseNumber: 1, Title: "The Gathering", StartTime: tstz(0)},
			{ID: 2, PhaseType: "action", PhaseNumber: 2, Title: "Descent", StartTime: tstz(100)},
			{ID: 3, PhaseType: "interlude", PhaseNumber: 3, Title: "Quiet Hours", StartTime: tstz(200)},
		},
		characters: []models.ListExportCharactersRow{
			{ID: 10, Name: "Ada Lovelace", CharacterType: "player_character",
				Status: txt("approved"), IsActive: true, PlayerUsername: txt("ada_player"), CreatedAt: tstz(0)},
			{ID: 11, Name: "Charles Babbage", CharacterType: "player_character",
				Status: txt("approved"), IsActive: false, PlayerUsername: txt("chuck"), CreatedAt: tstz(0)},
		},
		characterData: []models.ListExportCharacterDataRow{
			{CharacterID: 10, ModuleType: "basic_info", FieldName: "background", FieldValue: txt("Mathematician.")},
		},
		participants: []models.ListExportParticipantsRow{
			{UserID: 1, Role: "gm", Username: "gm_user", JoinedAt: tstz(0)},
			{UserID: 2, Role: "player", Username: "ada_player", JoinedAt: tstz(0)},
			{UserID: 3, Role: "audience", Username: "watcher", JoinedAt: tstz(0)},
		},
		posts: []models.ListExportPostsRow{
			{ID: 100, PhaseID: i4(1), Content: "The body in the library.", CreatedAt: ts(10),
				CharacterName: "Ada Lovelace", AuthorUsername: "ada_player"},
			{ID: 200, PhaseID: i4(1), Content: "Second post.", CreatedAt: ts(20),
				CharacterName: "Charles Babbage", AuthorUsername: "chuck"},
		},
		comments: map[int32][]models.ListExportCommentTreeRow{
			100: {
				comment(101, 100, 1, "Charles Babbage", "chuck", "Who found it?", 15),
				comment(102, 101, 2, "Ada Lovelace", "ada_player", "I did.", 16),
			},
		},
		conversations: []models.ListExportConversationsRow{
			{ID: 300, ConversationType: "direct", CreatedAt: tstz(50)},
		},
		convParts: []models.ListExportConversationParticipantsRow{
			{ConversationID: 300, Username: "ada_player", CharacterName: txt("Ada Lovelace")},
			{ConversationID: 300, Username: "chuck", CharacterName: txt("Charles Babbage")},
		},
		privateMsgs: []models.ListExportPrivateMessagesRow{
			{ID: 301, ConversationID: 300, Content: "Meet me at dawn.", CreatedAt: tstz(51),
				SenderUsername: "ada_player", SenderCharacterName: txt("Ada Lovelace")},
		},
		submissions: []models.ListExportActionSubmissionsRow{
			{ID: 400, PhaseID: 2, Content: "I search the cellar.", SubmittedAt: tstz(110),
				Username: "ada_player", CharacterName: txt("Ada Lovelace")},
		},
		results: []models.ListExportActionResultsRow{
			{ID: 500, PhaseID: 2, ActionSubmissionID: i4(400), Content: "You find a door.",
				SentAt: tstz(120), RecipientUsername: "ada_player",
				CharacterName: txt("Ada Lovelace"), GmUsername: "gm_user"},
		},
		handouts: []models.ListExportHandoutsRow{
			{ID: 600, Title: "The Cellar Map", Content: "A sketch.", Status: "published",
				CreatedAt: tstz(0), UpdatedAt: tstz(0)},
		},
		polls: []models.ListExportPollsRow{
			{ID: 700, PhaseID: i4(1), Question: "Search the cellar?", Deadline: tstz(90),
				CreatedAt: tstz(60), CreatorUsername: "gm_user", CreatorCharacterName: nullText(),
				ShowIndividualVotes: pgtype.Bool{Bool: false, Valid: true}},
		},
		pollOptions: []models.ListExportPollOptionsRow{
			{ID: 710, PollID: 700, OptionText: "Yes", DisplayOrder: 1},
		},
		pollVotes: []models.ListExportPollVotesRow{
			{PollID: 700, SelectedOptionID: i4(710), VoterUsername: "ada_player", CreatedAt: tstz(70)},
		},
		fingerprint: models.GetGameContentFingerprintRow{
			MessageCount: 4, GameUpdatedAt: tstz(9000),
		},
	}
}

// readArchive unzips into a path->content map.
func readArchive(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)

	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		b, err := io.ReadAll(rc)
		require.NoError(t, err)
		rc.Close()
		out[f.Name] = string(b)
	}
	return out
}

func assembleToMap(t *testing.T, q Querier) (map[string]string, *Result) {
	t.Helper()
	var buf bytes.Buffer
	a := &Assembler{Queries: q}
	res, err := a.Assemble(context.Background(), 164, &buf, nil)
	require.NoError(t, err)
	return readArchive(t, buf.Bytes()), res
}

func TestAssemble_RefusesNonCompletedGame(t *testing.T) {
	for _, state := range []string{
		core.GameStateInProgress, core.GameStateCancelled,
		core.GameStatePaused, core.GameStateRecruitment, core.GameStateSetup,
	} {
		t.Run(state, func(t *testing.T) {
			q := completedGame()
			q.game.State = txt(state)

			var buf bytes.Buffer
			a := &Assembler{Queries: q}
			_, err := a.Assemble(context.Background(), 164, &buf, nil)

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrGameNotCompleted,
				"non-completed games must be refused with a typed error")
			assert.Zero(t, buf.Len(), "nothing may be written for a refused export")
		})
	}
}

func TestAssemble_RefusesNullState(t *testing.T) {
	q := completedGame()
	q.game.State = pgtype.Text{Valid: false}

	var buf bytes.Buffer
	a := &Assembler{Queries: q}
	_, err := a.Assemble(context.Background(), 164, &buf, nil)

	assert.ErrorIs(t, err, ErrGameNotCompleted)
}

func TestAssemble_PropagatesQueryErrors(t *testing.T) {
	q := completedGame()
	q.postsErr = errors.New("db exploded")

	var buf bytes.Buffer
	a := &Assembler{Queries: q}
	_, err := a.Assemble(context.Background(), 164, &buf, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "db exploded")
	assert.Contains(t, err.Error(), "list posts")
}

func TestAssemble_ProducesExpectedLayout(t *testing.T) {
	files, res := assembleToMap(t, completedGame())

	root := "game-164-the-hollow-crown/"
	expected := []string{
		root + "README.md",
		root + "manifest.json",
		root + "characters/README.md",
		root + "characters/ada-lovelace.md",
		root + "characters/charles-babbage.md",
		root + "phases/01-common-room-the-gathering/README.md",
		root + "phases/01-common-room-the-gathering/posts/001-the-body-in-the-library.md",
		root + "phases/01-common-room-the-gathering/posts/002-second-post.md",
		root + "phases/01-common-room-the-gathering/polls/001-search-the-cellar.md",
		root + "phases/02-action-descent/README.md",
		root + "phases/02-action-descent/actions/ada-lovelace.md",
		root + "phases/03-interlude-quiet-hours/README.md",
		root + "conversations/README.md",
		root + "handouts/the-cellar-map.md",
	}
	for _, want := range expected {
		assert.Contains(t, files, want, "archive must contain %s", want)
	}

	assert.Equal(t, len(files), res.FileCount, "reported file count must match archive")
	assert.Positive(t, res.Bytes)
}

// Every entry must live under exactly one root directory, so extracting the
// archive never scatters files into the user's working directory.
func TestAssemble_AllEntriesUnderSingleRoot(t *testing.T) {
	files, _ := assembleToMap(t, completedGame())

	for name := range files {
		assert.True(t, strings.HasPrefix(name, "game-164-the-hollow-crown/"),
			"entry %q must be under the archive root", name)
		assert.NotContains(t, name, "..", "entry %q must not contain traversal", name)
		assert.False(t, strings.HasPrefix(name, "/"), "entry %q must be relative", name)
	}
}

func TestAssemble_PostContainsCommentTree(t *testing.T) {
	files, _ := assembleToMap(t, completedGame())

	post := files["game-164-the-hollow-crown/phases/01-common-room-the-gathering/posts/001-the-body-in-the-library.md"]
	require.NotEmpty(t, post)

	assert.Contains(t, post, "The body in the library.")
	assert.Contains(t, post, "## Comments")
	assert.Contains(t, post, "Who found it?")
	assert.Contains(t, post, "I did.")
	assert.Contains(t, post, "comment_count: 2")
}

func TestAssemble_ActionIncludesResult(t *testing.T) {
	files, _ := assembleToMap(t, completedGame())

	action := files["game-164-the-hollow-crown/phases/02-action-descent/actions/ada-lovelace.md"]
	require.NotEmpty(t, action)

	assert.Contains(t, action, "I search the cellar.")
	assert.Contains(t, action, "## Result")
	assert.Contains(t, action, "You find a door.")
	assert.Contains(t, action, "has_result: true")
}

// Regression: action_results.action_submission_id is frequently NULL in real
// data even when the matching submission exists. Pairing on the FK alone split
// every exchange into a separate submission file and "-result" file.
func TestAssemble_PairsResultToSubmissionWithoutFK(t *testing.T) {
	q := completedGame()
	q.submissions = []models.ListExportActionSubmissionsRow{
		{ID: 400, PhaseID: 2, UserID: 7, Content: "I search the cellar.",
			SubmittedAt: tstz(110), Username: "ada_player", CharacterName: txt("Ada Lovelace")},
	}
	q.results = []models.ListExportActionResultsRow{
		{ID: 500, PhaseID: 2, UserID: 7, // same phase + user, but NO FK
			ActionSubmissionID: pgtype.Int4{Valid: false},
			Content:            "You find a door.", SentAt: tstz(120),
			RecipientUsername: "ada_player", CharacterName: txt("Ada Lovelace"),
			GmUsername: "gm_user"},
	}

	files, _ := assembleToMap(t, q)

	action := files["game-164-the-hollow-crown/phases/02-action-descent/actions/ada-lovelace.md"]
	require.NotEmpty(t, action, "submission must be archived")
	assert.Contains(t, action, "I search the cellar.")
	assert.Contains(t, action, "## Result", "result must be merged into the submission file")
	assert.Contains(t, action, "You find a door.")
	assert.Contains(t, action, "has_result: true")

	// No separate orphan file should be produced.
	for name := range files {
		assert.NotContains(t, name, "-result.md",
			"result must not be written as a separate orphan file")
	}
}

// A result for a different user in the same phase must not be mis-paired.
func TestAssemble_DoesNotMisPairResultsAcrossUsers(t *testing.T) {
	q := completedGame()
	q.submissions = []models.ListExportActionSubmissionsRow{
		{ID: 400, PhaseID: 2, UserID: 7, Content: "Ada's action", SubmittedAt: tstz(110),
			Username: "ada_player", CharacterName: txt("Ada")},
		{ID: 401, PhaseID: 2, UserID: 8, Content: "Charles's action", SubmittedAt: tstz(111),
			Username: "chuck", CharacterName: txt("Charles")},
	}
	q.results = []models.ListExportActionResultsRow{
		{ID: 500, PhaseID: 2, UserID: 7, ActionSubmissionID: pgtype.Int4{Valid: false},
			Content: "Ada's result", SentAt: tstz(120),
			RecipientUsername: "ada_player", CharacterName: txt("Ada"), GmUsername: "gm"},
		{ID: 501, PhaseID: 2, UserID: 8, ActionSubmissionID: pgtype.Int4{Valid: false},
			Content: "Charles's result", SentAt: tstz(121),
			RecipientUsername: "chuck", CharacterName: txt("Charles"), GmUsername: "gm"},
	}

	files, _ := assembleToMap(t, q)

	ada := files["game-164-the-hollow-crown/phases/02-action-descent/actions/ada.md"]
	charles := files["game-164-the-hollow-crown/phases/02-action-descent/actions/charles.md"]
	require.NotEmpty(t, ada)
	require.NotEmpty(t, charles)

	assert.Contains(t, ada, "Ada's result")
	assert.NotContains(t, ada, "Charles's result")
	assert.Contains(t, charles, "Charles's result")
	assert.NotContains(t, charles, "Ada's result")
}

// A GM may stage several results against one submission over time. Every one
// must appear: keying results by submission in a plain map would silently drop
// all but the last, losing archived content.
func TestAssemble_KeepsEveryResultForOneSubmission(t *testing.T) {
	q := completedGame()
	q.submissions = []models.ListExportActionSubmissionsRow{
		{ID: 400, PhaseID: 2, UserID: 7, Content: "I search the cellar.",
			SubmittedAt: tstz(110), Username: "ada_player", CharacterName: txt("Ada Lovelace")},
	}
	q.results = []models.ListExportActionResultsRow{
		{ID: 500, PhaseID: 2, UserID: 7, ActionSubmissionID: i4(400),
			Content: "First you find a door.", SentAt: tstz(120),
			RecipientUsername: "ada_player", GmUsername: "gm_user"},
		{ID: 501, PhaseID: 2, UserID: 7, ActionSubmissionID: i4(400),
			Content: "Hours later, it opens.", SentAt: tstz(200),
			RecipientUsername: "ada_player", GmUsername: "gm_user"},
		{ID: 502, PhaseID: 2, UserID: 7, ActionSubmissionID: pgtype.Int4{Valid: false},
			Content: "At dawn, a sound below.", SentAt: tstz(300),
			RecipientUsername: "ada_player", GmUsername: "gm_user"},
	}

	files, _ := assembleToMap(t, q)

	action := files["game-164-the-hollow-crown/phases/02-action-descent/actions/ada-lovelace.md"]
	require.NotEmpty(t, action)

	// Every result survives.
	assert.Contains(t, action, "First you find a door.")
	assert.Contains(t, action, "Hours later, it opens.")
	assert.Contains(t, action, "At dawn, a sound below.")
	assert.Contains(t, action, "result_count: 3")

	// Numbered and ordered by send time.
	assert.Contains(t, action, "## Result 1 of 3")
	assert.Contains(t, action, "## Result 3 of 3")
	assert.Less(t, strings.Index(action, "First you find a door."),
		strings.Index(action, "Hours later, it opens."))
	assert.Less(t, strings.Index(action, "Hours later, it opens."),
		strings.Index(action, "At dawn, a sound below."))
}

// With exactly one result the heading stays unnumbered.
func TestAssemble_SingleResultIsNotNumbered(t *testing.T) {
	files, _ := assembleToMap(t, completedGame())

	action := files["game-164-the-hollow-crown/phases/02-action-descent/actions/ada-lovelace.md"]
	require.NotEmpty(t, action)
	assert.Contains(t, action, "## Result\n")
	assert.NotContains(t, action, "## Result 1 of")
	assert.Contains(t, action, "result_count: 1")
}

// A GM-initiated result answering no submission is normal, not a data fault.
func TestAssemble_GMInitiatedResultsAllArchived(t *testing.T) {
	q := completedGame()
	q.submissions = nil
	q.results = []models.ListExportActionResultsRow{
		{ID: 500, PhaseID: 2, UserID: 7, ActionSubmissionID: pgtype.Int4{Valid: false},
			Content: "The ground trembles.", SentAt: tstz(120),
			RecipientUsername: "ada_player", CharacterName: txt("Ada"), GmUsername: "gm_user"},
		{ID: 501, PhaseID: 2, UserID: 7, ActionSubmissionID: pgtype.Int4{Valid: false},
			Content: "Then it stops.", SentAt: tstz(130),
			RecipientUsername: "ada_player", CharacterName: txt("Ada"), GmUsername: "gm_user"},
	}

	files, _ := assembleToMap(t, q)

	var found []string
	for name, content := range files {
		if strings.Contains(name, "phases/02-action-descent/actions/") {
			found = append(found, content)
		}
	}
	require.Len(t, found, 2, "both unprompted results must be archived")

	joined := strings.Join(found, "\n")
	assert.Contains(t, joined, "The ground trembles.")
	assert.Contains(t, joined, "Then it stops.")
	// The note must not claim a submission went missing when none was referenced.
	assert.Contains(t, joined, "no associated action submission")
	assert.NotContains(t, joined, "no longer present")
}

// A result whose submission row is gone must still be archived rather than
// silently dropped.
func TestAssemble_OrphanResultStillWritten(t *testing.T) {
	q := completedGame()
	q.submissions = nil
	// FK points at a submission that is not in the export (deleted, or a draft).
	q.results = []models.ListExportActionResultsRow{
		{ID: 500, PhaseID: 2, UserID: 7, ActionSubmissionID: i4(9999),
			Content: "Orphaned result.", SentAt: tstz(120),
			RecipientUsername: "ada_player", CharacterName: txt("Ada Lovelace"), GmUsername: "gm_user"},
	}

	files, _ := assembleToMap(t, q)

	var found string
	for name, content := range files {
		if strings.Contains(name, "phases/02-action-descent/actions/") {
			found = content
		}
	}
	require.NotEmpty(t, found, "orphan result must produce a file")
	assert.Contains(t, found, "Orphaned result.")
	assert.Contains(t, found, "referenced action submission no longer present")
}

// Content pointing at a phase that was never published (or no phase at all)
// must land somewhere rather than vanishing.
func TestAssemble_UnfiledContentIsNotDropped(t *testing.T) {
	q := completedGame()
	q.posts = []models.ListExportPostsRow{
		{ID: 100, PhaseID: pgtype.Int4{Valid: false}, Content: "Phaseless post.", CreatedAt: ts(10),
			CharacterName: "Ada Lovelace", AuthorUsername: "ada_player"},
		{ID: 101, PhaseID: i4(999), Content: "Unpublished phase post.", CreatedAt: ts(11),
			CharacterName: "Ada Lovelace", AuthorUsername: "ada_player"},
	}

	files, _ := assembleToMap(t, q)

	var unfiled []string
	for name := range files {
		if strings.Contains(name, "00-unfiled") {
			unfiled = append(unfiled, name)
		}
	}
	assert.Len(t, unfiled, 2, "both orphaned posts must be archived under unfiled")
}

// Two characters with the same name must not overwrite each other.
func TestAssemble_DuplicateNamesGetUniquePaths(t *testing.T) {
	q := completedGame()
	q.characters = []models.ListExportCharactersRow{
		{ID: 10, Name: "Ada", CharacterType: "player_character", Status: txt("approved"),
			IsActive: true, PlayerUsername: txt("player_one"), CreatedAt: tstz(0)},
		{ID: 11, Name: "Ada", CharacterType: "player_character", Status: txt("approved"),
			IsActive: true, PlayerUsername: txt("player_two"), CreatedAt: tstz(0)},
		{ID: 12, Name: "Ada", CharacterType: "npc", Status: txt("approved"),
			IsActive: true, PlayerUsername: nullText(), CreatedAt: tstz(0)},
	}

	files, _ := assembleToMap(t, q)

	root := "game-164-the-hollow-crown/characters/"
	assert.Contains(t, files, root+"ada.md")
	assert.Contains(t, files, root+"ada-2.md")
	assert.Contains(t, files, root+"ada-3.md")

	// Each file must hold its own character, not a duplicate.
	assert.Contains(t, files[root+"ada.md"], "player_one")
	assert.Contains(t, files[root+"ada-2.md"], "player_two")
}

func TestAssemble_ManifestListsEveryFile(t *testing.T) {
	files, _ := assembleToMap(t, completedGame())

	raw := files["game-164-the-hollow-crown/manifest.json"]
	require.NotEmpty(t, raw)

	var m manifest
	require.NoError(t, json.Unmarshal([]byte(raw), &m))

	assert.Equal(t, int32(164), m.GameID)
	assert.Equal(t, "The Hollow Crown", m.GameTitle)
	assert.Equal(t, "markdown", m.Format)
	assert.NotEmpty(t, m.Fingerprint)

	// Manifest describes every archived file except itself.
	assert.Len(t, m.Files, len(files)-1)

	inManifest := map[string]bool{}
	for _, e := range m.Files {
		inManifest[e.Path] = true
		assert.NotEmpty(t, e.Type, "entry %s needs a type", e.Path)
		assert.Positive(t, e.Bytes, "entry %s needs a byte count", e.Path)
	}
	for name := range files {
		rel := strings.TrimPrefix(name, "game-164-the-hollow-crown/")
		if rel == "manifest.json" {
			continue
		}
		assert.True(t, inManifest[rel], "file %s missing from manifest", rel)
	}
}

func TestAssemble_ReadmeSummarizesGame(t *testing.T) {
	files, _ := assembleToMap(t, completedGame())

	readme := files["game-164-the-hollow-crown/README.md"]
	require.NotEmpty(t, readme)

	assert.Contains(t, readme, "# The Hollow Crown")
	assert.Contains(t, readme, "A mystery.")
	assert.Contains(t, readme, "Gothic")
	// The GM must appear even though games.gm_user_id is not guaranteed to have
	// a game_participants row; ListExportParticipants unions them in.
	assert.Contains(t, readme, "gm_user")
	assert.Contains(t, readme, "ada_player")
	assert.Contains(t, readme, "watcher")
	// Explicit statement of what is omitted.
	assert.Contains(t, readme, "images are not included")
}

// Every relative Markdown link in an index file must resolve to a real archive
// entry. This catches path-prefix drift between writers, which is otherwise
// invisible until someone opens the archive.
func TestAssemble_IndexLinksResolve(t *testing.T) {
	files, _ := assembleToMap(t, completedGame())

	root := "game-164-the-hollow-crown/"
	linkPattern := regexp.MustCompile(`\]\(([^)]+\.md)\)`)

	checked := 0
	for name, content := range files {
		if !strings.HasSuffix(name, "README.md") {
			continue
		}
		dir := path.Dir(name)
		for _, m := range linkPattern.FindAllStringSubmatch(content, -1) {
			target := path.Join(dir, m[1])
			assert.Contains(t, files, target,
				"link %q in %q must resolve (relative to %s)", m[1], name, dir)
			checked++
		}
	}
	assert.Positive(t, checked, "expected index files to contain links")
	assert.Contains(t, files, root+"phases/01-common-room-the-gathering/README.md")
}

func TestAssemble_EmptyConversationSkipped(t *testing.T) {
	q := completedGame()
	q.conversations = append(q.conversations,
		models.ListExportConversationsRow{ID: 999, ConversationType: "direct", CreatedAt: tstz(1)})
	// No messages for 999.

	files, _ := assembleToMap(t, q)

	for name := range files {
		assert.NotContains(t, name, "conversation-999")
	}
	// The one real conversation is still present.
	var convFiles int
	for name := range files {
		if strings.HasPrefix(name, "game-164-the-hollow-crown/conversations/") &&
			!strings.HasSuffix(name, "README.md") {
			convFiles++
		}
	}
	assert.Equal(t, 1, convFiles)
}

func TestAssemble_EmptyGameStillProducesValidArchive(t *testing.T) {
	q := &fakeQuerier{
		game: models.Game{
			ID: 5, Title: "Empty Game", State: txt(core.GameStateCompleted),
		},
	}

	var buf bytes.Buffer
	a := &Assembler{Queries: q}
	res, err := a.Assemble(context.Background(), 5, &buf, nil)
	require.NoError(t, err)

	files := readArchive(t, buf.Bytes())
	assert.Contains(t, files, "game-5-empty-game/README.md")
	assert.Contains(t, files, "game-5-empty-game/manifest.json")
	assert.Equal(t, len(files), res.FileCount)
}

func TestAssemble_ReportsProgress(t *testing.T) {
	var notes []string
	var buf bytes.Buffer
	a := &Assembler{Queries: completedGame()}

	_, err := a.Assemble(context.Background(), 164, &buf, func(n string) {
		notes = append(notes, n)
	})
	require.NoError(t, err)

	assert.NotEmpty(t, notes)
	assert.Contains(t, notes, "writing private conversations")
}

func TestAssemble_NilProgressIsSafe(t *testing.T) {
	var buf bytes.Buffer
	a := &Assembler{Queries: completedGame()}
	_, err := a.Assemble(context.Background(), 164, &buf, nil)
	assert.NoError(t, err)
}

// A hostile game title must not escape the archive root.
func TestAssemble_HostileTitleIsContained(t *testing.T) {
	q := completedGame()
	q.game.Title = "../../etc/passwd"

	files, _ := assembleToMap(t, q)

	for name := range files {
		assert.True(t, strings.HasPrefix(name, "game-164-etc-passwd/"),
			"entry %q must stay under the sanitized root", name)
	}
}

func TestAssemble_ProducesReadableZip(t *testing.T) {
	var buf bytes.Buffer
	a := &Assembler{Queries: completedGame()}
	_, err := a.Assemble(context.Background(), 164, &buf, nil)
	require.NoError(t, err)

	// Standard readers must accept the archive.
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err, "archive must be a valid zip")
	assert.NotEmpty(t, zr.File)

	names := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		names = append(names, f.Name)
		assert.False(t, f.FileInfo().IsDir(), "no directory entries expected")
	}
	assert.True(t, sort.StringsAreSorted(names) || len(names) > 0)
}

func TestFingerprint_StableAndSensitive(t *testing.T) {
	base := models.GetGameContentFingerprintRow{
		MessageCount: 10, MessageMaxTs: tstz(100),
		SubmissionCount: 2, ResultCount: 2,
		GameUpdatedAt: tstz(500),
	}

	assert.Equal(t, Fingerprint(base), Fingerprint(base), "must be deterministic")

	// A count change must change the hash (catches insert/delete).
	countChanged := base
	countChanged.MessageCount = 11
	assert.NotEqual(t, Fingerprint(base), Fingerprint(countChanged))

	// A timestamp change must change the hash (catches in-place edits, which
	// leave counts untouched).
	tsChanged := base
	tsChanged.MessageMaxTs = tstz(101)
	assert.NotEqual(t, Fingerprint(base), Fingerprint(tsChanged))

	// NULL vs zero-value timestamps must be distinguishable.
	nullTs := base
	nullTs.MessageMaxTs = pgtype.Timestamptz{Valid: false}
	assert.NotEqual(t, Fingerprint(base), Fingerprint(nullTs))
}

func TestFingerprintFor(t *testing.T) {
	a := &Assembler{Queries: completedGame()}
	got, err := a.FingerprintFor(context.Background(), 164)
	require.NoError(t, err)
	assert.Len(t, got, 64, "sha256 hex digest")
}

func TestUniqueName(t *testing.T) {
	used := map[string]bool{}
	assert.Equal(t, "ada", uniqueName(used, "ada"))
	assert.Equal(t, "ada-2", uniqueName(used, "ada"))
	assert.Equal(t, "ada-3", uniqueName(used, "ada"))
	assert.Equal(t, "bob", uniqueName(used, "bob"))
}

func TestConversationLabel(t *testing.T) {
	titled := models.ListExportConversationsRow{ID: 1, Title: txt("Plotting")}
	assert.Equal(t, "Plotting", conversationLabel(titled, nil))

	untitled := models.ListExportConversationsRow{ID: 2, Title: nullText()}
	parts := []models.ListExportConversationParticipantsRow{
		{ConversationID: 2, Username: "u2", CharacterName: txt("Zed")},
		{ConversationID: 2, Username: "u1", CharacterName: txt("Ada")},
	}
	assert.Equal(t, "Ada and Zed", conversationLabel(untitled, parts))

	assert.Equal(t, "conversation", conversationLabel(untitled, nil))
}

func TestPhaseLocation(t *testing.T) {
	byID := map[int32]models.ListExportPhasesRow{
		1: {ID: 1, PhaseNumber: 1, PhaseType: "action", Title: "Descent"},
	}
	// phaseDir holds full archive-relative paths, not bare directory names.
	dirs := map[int32]string{1: "phases/01-action-descent"}

	dir, title := phaseLocation(i4(1), byID, dirs)
	assert.Equal(t, "phases/01-action-descent", dir)
	assert.Equal(t, "Descent", title)

	dir, title = phaseLocation(pgtype.Int4{Valid: false}, byID, dirs)
	assert.Equal(t, "phases/00-unfiled", dir)
	assert.Equal(t, "Unfiled", title)

	dir, _ = phaseLocation(i4(42), byID, dirs)
	assert.Equal(t, "phases/00-unfiled", dir, "unknown phase id falls back to unfiled")
}

var _ Querier = (*fakeQuerier)(nil)
