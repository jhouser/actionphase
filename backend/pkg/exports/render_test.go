package exports

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	models "actionphase/pkg/db/models"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ts(offsetSeconds int) pgtype.Timestamp {
	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	return pgtype.Timestamp{Time: base.Add(time.Duration(offsetSeconds) * time.Second), Valid: true}
}

func tstz(offsetSeconds int) pgtype.Timestamptz {
	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	return pgtype.Timestamptz{Time: base.Add(time.Duration(offsetSeconds) * time.Second), Valid: true}
}

func txt(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }
func nullText() pgtype.Text    { return pgtype.Text{Valid: false} }
func i4(v int32) pgtype.Int4   { return pgtype.Int4{Int32: v, Valid: true} }

func samplePost() models.ListExportPostsRow {
	return models.ListExportPostsRow{
		ID:             100,
		PhaseID:        i4(5),
		Content:        "The body in the library.\n\nWho was there?",
		CreatedAt:      ts(0),
		CharacterName:  "Ada Lovelace",
		AuthorUsername: "ada_player",
	}
}

// comment builds a tree row; path length is what the query would produce.
func comment(id int32, parent int32, depth int32, name, user, content string, at int) models.ListExportCommentTreeRow {
	return models.ListExportCommentTreeRow{
		ID:             id,
		ParentID:       i4(parent),
		Content:        content,
		CreatedAt:      ts(at),
		ThreadDepth:    depth,
		CharacterName:  name,
		AuthorUsername: user,
	}
}

func TestRenderPost_NoComments(t *testing.T) {
	out := RenderPost(samplePost(), nil, "Phase 1")

	assert.True(t, strings.HasPrefix(out, "---\n"), "must start with frontmatter")
	assert.Contains(t, out, "type: post")
	assert.Contains(t, out, "id: 100")
	assert.Contains(t, out, "comment_count: 0")
	assert.Contains(t, out, "# The body in the library.")
	assert.Contains(t, out, "**Ada Lovelace (ada_player)**")
	assert.Contains(t, out, "Who was there?")
	assert.NotContains(t, out, "## Comments")
}

func TestRenderPost_StripsCustomMarkdown(t *testing.T) {
	p := samplePost()
	p.Content = "The door is [color:red]locked[/color] and I used [[Lockpicking|skill:9f2]]."

	out := RenderPost(p, nil, "Phase 1")

	assert.Contains(t, out, "The door is locked and I used [[Lockpicking]].")
	assert.NotContains(t, out, "[color:red]")
	assert.NotContains(t, out, "skill:9f2")
}

func TestRenderPost_NestsCommentsByDepth(t *testing.T) {
	comments := []models.ListExportCommentTreeRow{
		comment(101, 100, 1, "Charles", "chuck", "First reply", 10),
		comment(102, 101, 2, "Ada", "ada_player", "Nested once", 20),
		comment(103, 102, 3, "Charles", "chuck", "Nested twice", 30),
	}

	out := RenderPost(samplePost(), comments, "Phase 1")

	assert.Contains(t, out, "## Comments")
	assert.Contains(t, out, "> **Charles (chuck)**")
	assert.Contains(t, out, "> > **Ada (ada_player)**")
	assert.Contains(t, out, "> > > **Charles (chuck)**")
	// Order must follow the input (materialized path order).
	assert.Less(t, strings.Index(out, "First reply"), strings.Index(out, "Nested once"))
	assert.Less(t, strings.Index(out, "Nested once"), strings.Index(out, "Nested twice"))
}

// The case that motivated the design: a thread deeper than the indent cap.
func TestRenderPost_FlattensBeyondMaxDepth(t *testing.T) {
	var comments []models.ListExportCommentTreeRow
	parent := int32(100)
	for i := 1; i <= maxIndentDepth+8; i++ {
		id := int32(100 + i)
		comments = append(comments, comment(id, parent, int32(i), "Ada", "ada_player",
			fmt.Sprintf("reply at depth %d", i), i*10))
		parent = id
	}

	out := RenderPost(samplePost(), comments, "Phase 1")

	// Indentation must stop growing at the cap.
	assert.NotContains(t, out, strings.Repeat("> ", maxIndentDepth+1),
		"indent must not exceed maxIndentDepth")

	// Every reply must still be present.
	for i := 1; i <= maxIndentDepth+8; i++ {
		assert.Contains(t, out, fmt.Sprintf("reply at depth %d", i))
	}

	// Flattened replies must carry an explicit backlink to their parent.
	assert.Contains(t, out, "↳ replying to")
	assert.Contains(t, out, fmt.Sprintf("depth %d", maxIndentDepth+8))

	// Shallow replies should NOT carry the backlink; nesting already shows it.
	firstReplyIdx := strings.Index(out, "reply at depth 1")
	firstBacklinkIdx := strings.Index(out, "↳ replying to")
	assert.Less(t, firstReplyIdx, firstBacklinkIdx,
		"backlinks should only appear after the indent cap is exceeded")
}

// Regression: flattened replies referenced a parent by "(#123)", but no comment
// ever printed its own id, so the reference resolved to nothing in the file and
// deep threads could not be reassembled by a reader.
func TestRenderPost_BacklinkTargetsAreFindable(t *testing.T) {
	var comments []models.ListExportCommentTreeRow
	parent := int32(100)
	for i := 1; i <= maxIndentDepth+6; i++ {
		id := int32(100 + i)
		comments = append(comments, comment(id, parent, int32(i), "Ada", "ada_player",
			fmt.Sprintf("reply at depth %d", i), i*10))
		parent = id
	}

	out := RenderPost(samplePost(), comments, "Phase 1")

	// Every id referenced by a backlink must appear as an anchor elsewhere in
	// the document, otherwise the reference is dangling.
	refs := regexp.MustCompile(`#(\d+)\)`).FindAllStringSubmatch(out, -1)
	require.NotEmpty(t, refs, "expected at least one parent backlink")
	for _, m := range refs {
		assert.Contains(t, out, "· #"+m[1]+" —",
			"backlink to #%s has no matching anchor in the document", m[1])
	}
}

// Every comment carries its own id, so any reference to it can be resolved by
// searching the file.
func TestRenderPost_EveryCommentPrintsItsID(t *testing.T) {
	comments := []models.ListExportCommentTreeRow{
		comment(101, 100, 1, "Charles", "chuck", "First reply", 10),
		comment(102, 101, 2, "Ada", "ada_player", "Nested once", 20),
	}

	out := RenderPost(samplePost(), comments, "Phase 1")

	assert.Contains(t, out, "· #101 —")
	assert.Contains(t, out, "· #102 —")
}

// Real games run wide rather than deep: a single comment may carry 100+ direct
// replies. The parent must advertise how many, so a reader can tell the archive
// is complete rather than truncated.
func TestRenderPost_AnnotatesWideReplyCounts(t *testing.T) {
	comments := []models.ListExportCommentTreeRow{
		comment(101, 100, 1, "Ada", "ada_player", "the big event", 10),
	}
	for i := 0; i < 47; i++ {
		id := int32(200 + i)
		comments = append(comments, comment(id, 101, 2, "Charles", "chuck",
			fmt.Sprintf("sibling %d", i), 20+i))
	}

	out := RenderPost(samplePost(), comments, "Phase 1")

	assert.Contains(t, out, "47 replies", "a widely-replied comment must state its reply count")
	for i := 0; i < 47; i++ {
		assert.Contains(t, out, fmt.Sprintf("sibling %d", i))
	}
}

// A comment with a single reply should not be annotated; the count only earns
// its place when the fan-out is large enough to be worth signposting.
func TestRenderPost_NoReplyCountForNarrowThreads(t *testing.T) {
	comments := []models.ListExportCommentTreeRow{
		comment(101, 100, 1, "Ada", "ada_player", "a remark", 10),
		comment(102, 101, 2, "Charles", "chuck", "a response", 20),
	}

	out := RenderPost(samplePost(), comments, "Phase 1")

	assert.NotContains(t, out, "1 replies")
	assert.NotContains(t, out, "1 reply")
}

// Threads in real games reach roughly 20 deep. A plain-text archive has no
// fixed-width constraint, so that entire range must nest faithfully rather than
// collapsing into an ambiguous flat run.
func TestRenderPost_TwentyDeepStaysNested(t *testing.T) {
	var comments []models.ListExportCommentTreeRow
	parent := int32(100)
	for i := 1; i <= 20; i++ {
		id := int32(100 + i)
		comments = append(comments, comment(id, parent, int32(i), "Ada", "ada_player",
			fmt.Sprintf("reply at depth %d", i), i*10))
		parent = id
	}

	out := RenderPost(samplePost(), comments, "Phase 1")

	assert.NotContains(t, out, "↳ replying to",
		"a 20-deep chain must nest without flattening")
	assert.Contains(t, out, strings.Repeat("> ", 20)+"**Ada (ada_player)**",
		"the deepest reply must carry full indentation")
}

func TestRenderPost_MultiLineCommentStaysQuoted(t *testing.T) {
	comments := []models.ListExportCommentTreeRow{
		comment(101, 100, 1, "Ada", "ada", "line one\nline two\n\nline four", 10),
	}

	out := RenderPost(samplePost(), comments, "Phase 1")

	// Every content line of a nested comment must carry the quote marker,
	// otherwise the block escapes its nesting level in rendered Markdown.
	for _, line := range []string{"line one", "line two", "line four"} {
		assert.Regexp(t, `(?m)^>.*`+line, out, "line %q must stay quoted", line)
	}
}

func TestRenderPost_EditedMarker(t *testing.T) {
	c := comment(101, 100, 1, "Ada", "ada", "edited body", 10)
	c.IsEdited = true

	out := RenderPost(samplePost(), []models.ListExportCommentTreeRow{c}, "Phase 1")
	assert.Contains(t, out, "*(edited)*")
}

func TestFirstLineTitle(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Simple title\n\nbody", "Simple title"},
		{"# Heading style\n\nbody", "Heading style"},
		{"\n\n   \nActual first line", "Actual first line"},
		{"> quoted opening", "quoted opening"},
		{"", "Untitled post"},
		{"   \n\n  ", "Untitled post"},
		{"[color:red]Colored title[/color]", "Colored title"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, firstLineTitle(tt.in), "input=%q", tt.in)
	}
}

func TestFirstLineTitle_TruncatesLongLine(t *testing.T) {
	got := firstLineTitle(strings.Repeat("word ", 100))
	assert.LessOrEqual(t, len([]rune(got)), 81)
	assert.True(t, strings.HasSuffix(got, "…"))
}

func TestRenderConversation(t *testing.T) {
	conv := models.ListExportConversationsRow{
		ID: 3, Title: nullText(), ConversationType: "direct", CreatedAt: tstz(0),
	}
	parts := []models.ListExportConversationParticipantsRow{
		{ConversationID: 3, Username: "ada_player", CharacterName: txt("Ada")},
		{ConversationID: 3, Username: "chuck", CharacterName: txt("Charles")},
	}
	msgs := []models.ListExportPrivateMessagesRow{
		{ID: 1, ConversationID: 3, Content: "Meet me at dawn.", CreatedAt: tstz(10),
			SenderUsername: "ada_player", SenderCharacterName: txt("Ada")},
		{ID: 2, ConversationID: 3, Content: "[color:blue]Agreed.[/color]", CreatedAt: tstz(20),
			SenderUsername: "chuck", SenderCharacterName: txt("Charles")},
	}

	out := RenderConversation(conv, parts, msgs)

	assert.Contains(t, out, "type: conversation")
	assert.Contains(t, out, "message_count: 2")
	assert.Contains(t, out, "Ada (ada_player)")
	assert.Contains(t, out, "Charles (chuck)")
	assert.Contains(t, out, "Meet me at dawn.")
	assert.Contains(t, out, "Agreed.")
	assert.NotContains(t, out, "[color:blue]")
}

func TestRenderConversation_FallsBackWhenCharacterDetached(t *testing.T) {
	// character_id is ON DELETE SET NULL, so an archived conversation can have
	// messages whose character no longer exists.
	conv := models.ListExportConversationsRow{ID: 4, ConversationType: "direct", CreatedAt: tstz(0)}
	msgs := []models.ListExportPrivateMessagesRow{
		{ID: 1, ConversationID: 4, Content: "orphaned", CreatedAt: tstz(1),
			SenderUsername: "ghost", SenderCharacterName: nullText()},
	}

	out := RenderConversation(conv, nil, msgs)

	assert.Contains(t, out, "**ghost**")
	assert.Contains(t, out, "orphaned")
	assert.NotContains(t, out, "()", "must not render an empty parenthetical")
}

func TestRenderActionFile_WithAndWithoutResult(t *testing.T) {
	sub := models.ListExportActionSubmissionsRow{
		ID: 7, PhaseID: 5, Content: "I search the cellar.",
		SubmittedAt: tstz(0), Username: "ada_player", CharacterName: txt("Ada"),
	}

	withoutResult := RenderActionFile(sub, nil, "Phase 2")
	assert.Contains(t, withoutResult, "has_result: false")
	assert.Contains(t, withoutResult, "## Submission")
	assert.NotContains(t, withoutResult, "## Result")

	results := []models.ListExportActionResultsRow{{
		ID: 9, PhaseID: 5, Content: "You find a hidden door.",
		SentAt: tstz(100), RecipientUsername: "ada_player", GmUsername: "gm_user",
	}}
	withResult := RenderActionFile(sub, results, "Phase 2")
	assert.Contains(t, withResult, "has_result: true")
	assert.Contains(t, withResult, "result_count: 1")
	assert.Contains(t, withResult, "## Result")
	assert.Contains(t, withResult, "You find a hidden door.")
	assert.Contains(t, withResult, "gm_user")
	assert.Less(t, strings.Index(withResult, "## Submission"), strings.Index(withResult, "## Result"))
}

// Staged reveals: several results against one submission, ordered by send time
// and each labelled, so a delayed follow-up is never lost or misordered.
func TestRenderActionFile_MultipleResultsOrderedAndNumbered(t *testing.T) {
	sub := models.ListExportActionSubmissionsRow{
		ID: 7, PhaseID: 5, Content: "I search the cellar.",
		SubmittedAt: tstz(0), Username: "ada_player", CharacterName: txt("Ada"),
	}
	// Deliberately supplied out of order.
	results := []models.ListExportActionResultsRow{
		{ID: 11, PhaseID: 5, Content: "Later, it opens.", SentAt: tstz(200),
			RecipientUsername: "ada_player", GmUsername: "gm_user"},
		{ID: 10, PhaseID: 5, Content: "First, a door.", SentAt: tstz(100),
			RecipientUsername: "ada_player", GmUsername: "gm_user"},
	}

	out := RenderActionFile(sub, results, "Phase 2")

	assert.Contains(t, out, "result_count: 2")
	assert.Contains(t, out, "## Result 1 of 2")
	assert.Contains(t, out, "## Result 2 of 2")
	assert.Less(t, strings.Index(out, "First, a door."), strings.Index(out, "Later, it opens."),
		"results must be ordered by send time regardless of input order")
}

// Every part of a staged chain shares one sent_at, so send time cannot order
// them — the chain link must.
//
// PublishActionResult publishes a whole chain in a single UPDATE setting
// `sent_at = COALESCE(sent_at, NOW())` (phases.sql:406), and NOW() is the
// transaction timestamp. So all N parts land on a byte-identical sent_at. This
// is not a corner case: it is what every published chain in the database looks
// like. Verified against production rows 436/437/438, which share
// `2026-08-13 19:40:36.693598+00` and come back from the export query in the
// order 438, 437, 436 — exactly reversed.
//
// With the sort key tied, sort.SliceStable preserves input order, so whatever
// order Postgres returned is what the archive renders. "Result 1 of 3" then
// labels the payoff and "Result 3 of 3" the setup, and the scene reads
// backwards with no indication anything is wrong.
func TestRenderActionFile_ChainOrderedByLinkWhenSendTimesTie(t *testing.T) {
	sub := models.ListExportActionSubmissionsRow{
		ID: 7, PhaseID: 5, Content: "I duel the swordsman.",
		SubmittedAt: tstz(0), Username: "ada_player", CharacterName: txt("Ada"),
	}
	// One shared timestamp, supplied reversed — what the export query returns.
	shared := tstz(100)
	results := []models.ListExportActionResultsRow{
		{ID: 438, PhaseID: 5, Content: "and misses! You counterattack.", SentAt: shared,
			ParentResultID: i4(437), RecipientUsername: "ada_player", GmUsername: "gm_user"},
		{ID: 437, PhaseID: 5, Content: "The blade whooshes toward your head...", SentAt: shared,
			ParentResultID: i4(436), RecipientUsername: "ada_player", GmUsername: "gm_user"},
		{ID: 436, PhaseID: 5, Content: "You get into a fight with X.", SentAt: shared,
			RecipientUsername: "ada_player", GmUsername: "gm_user"},
	}

	out := RenderActionFile(sub, results, "Phase 2")

	setup := strings.Index(out, "You get into a fight with X.")
	swing := strings.Index(out, "The blade whooshes toward your head...")
	payoff := strings.Index(out, "and misses! You counterattack.")

	assert.Less(t, setup, swing, "the chain head must render first")
	assert.Less(t, swing, payoff, "the payoff must render last, not first")

	// The numbering must agree with the order, or the labels lie.
	assert.Less(t, strings.Index(out, "## Result 1 of 3"), strings.Index(out, "## Result 3 of 3"))
}

// A result with no send time must not displace dated results.
func TestRenderActionFile_UndatedResultSortsLast(t *testing.T) {
	sub := models.ListExportActionSubmissionsRow{
		ID: 7, PhaseID: 5, Content: "Action.", SubmittedAt: tstz(0), Username: "ada",
	}
	results := []models.ListExportActionResultsRow{
		{ID: 10, Content: "Undated.", SentAt: pgtype.Timestamptz{Valid: false}, GmUsername: "gm"},
		{ID: 11, Content: "Dated.", SentAt: tstz(100), GmUsername: "gm"},
	}

	out := RenderActionFile(sub, results, "Phase 2")

	assert.Less(t, strings.Index(out, "Dated."), strings.Index(out, "Undated."))
}

// A completed game grants every authenticated user canSeeIndividualVotes
// regardless of the poll's show_individual_votes setting
// (checkPollViewAccess, backend/pkg/polls/api_polls.go:241). The archive must
// therefore attribute votes even for polls that hid them DURING play, so it
// matches what the completed-game web view shows.
func TestRenderPoll_ShowsVotersEvenWhenHiddenDuringPlay(t *testing.T) {
	poll := models.ListExportPollsRow{
		ID: 2, Question: "Search the cellar?", Deadline: tstz(500), CreatedAt: tstz(0),
		CreatorUsername: "gm_user", CreatorCharacterName: nullText(),
		ShowIndividualVotes: pgtype.Bool{Bool: false, Valid: true},
	}
	opts := []models.ListExportPollOptionsRow{
		{ID: 10, PollID: 2, OptionText: "Yes", DisplayOrder: 1},
		{ID: 11, PollID: 2, OptionText: "No", DisplayOrder: 2},
	}
	votes := []models.ListExportPollVotesRow{
		{PollID: 2, SelectedOptionID: i4(10), VoterUsername: "ada_player", CreatedAt: tstz(1)},
		{PollID: 2, SelectedOptionID: i4(10), VoterUsername: "chuck", CreatedAt: tstz(2)},
		{PollID: 2, SelectedOptionID: i4(11), VoterUsername: "bob", CreatedAt: tstz(3)},
	}

	out := RenderPoll(poll, opts, votes)

	assert.Contains(t, out, "**Yes** — 2 vote(s): ada_player, chuck")
	assert.Contains(t, out, "**No** — 1 vote(s): bob")
	// The in-game setting is still recorded as metadata for context.
	assert.Contains(t, out, "shown_to_players_during_game: false")
}

func TestRenderPoll_ShowsVotersWhenShownDuringPlay(t *testing.T) {
	poll := models.ListExportPollsRow{
		ID: 2, Question: "Search the cellar?", Deadline: tstz(500), CreatedAt: tstz(0),
		CreatorUsername: "gm_user", CreatorCharacterName: nullText(),
		ShowIndividualVotes: pgtype.Bool{Bool: true, Valid: true},
	}
	opts := []models.ListExportPollOptionsRow{{ID: 10, PollID: 2, OptionText: "Yes", DisplayOrder: 1}}
	votes := []models.ListExportPollVotesRow{
		{PollID: 2, SelectedOptionID: i4(10), VoterUsername: "chuck", CreatedAt: tstz(2)},
		{PollID: 2, SelectedOptionID: i4(10), VoterUsername: "ada_player", CreatedAt: tstz(1)},
	}

	out := RenderPoll(poll, opts, votes)

	assert.Contains(t, out, "ada_player, chuck", "voters listed and sorted")
	assert.Contains(t, out, "shown_to_players_during_game: true")
}

func TestRenderPoll_WriteInResponsesAttributed(t *testing.T) {
	poll := models.ListExportPollsRow{
		ID: 2, Question: "Where next?", Deadline: tstz(500), CreatedAt: tstz(0),
		CreatorUsername: "gm", CreatorCharacterName: nullText(),
		ShowIndividualVotes: pgtype.Bool{Bool: false, Valid: true},
	}
	votes := []models.ListExportPollVotesRow{
		{PollID: 2, OtherResponse: txt("The attic"), CreatedAt: tstz(1), VoterUsername: "ada"},
	}

	out := RenderPoll(poll, nil, votes)

	assert.Contains(t, out, "Write-in responses")
	assert.Contains(t, out, "- ada — The attic")
}

func TestRenderCharacter(t *testing.T) {
	ch := models.ListExportCharactersRow{
		ID: 42, Name: "Ada Lovelace", CharacterType: "player_character",
		Status: txt("approved"), IsActive: true, CreatedAt: tstz(0),
		PlayerUsername: txt("ada_player"),
	}
	data := []models.ListExportCharacterDataRow{
		{CharacterID: 42, ModuleType: "basic_info", FieldName: "background", FieldValue: txt("A mathematician.")},
		{CharacterID: 42, ModuleType: "basic_info", FieldName: "goal", FieldValue: txt("[color:gold]Find the truth[/color]")},
		{CharacterID: 42, ModuleType: "skills", FieldName: "lockpicking", FieldValue: txt("3")},
		{CharacterID: 42, ModuleType: "skills", FieldName: "empty_field", FieldValue: nullText()},
	}

	out := RenderCharacter(ch, data)

	assert.Contains(t, out, "# Ada Lovelace")
	assert.Contains(t, out, "**Player:** ada_player")
	assert.Contains(t, out, "## Basic Info")
	assert.Contains(t, out, "## Skills")
	assert.Contains(t, out, "**Background:**")
	assert.Contains(t, out, "A mathematician.")
	assert.Contains(t, out, "Find the truth")
	assert.NotContains(t, out, "[color:gold]")
	assert.NotContains(t, out, "Empty Field", "NULL fields are omitted")
	// Avatars are deliberately excluded from archives.
	assert.NotContains(t, strings.ToLower(out), "avatar")
}

func TestRenderCharacter_InactiveAndUnassigned(t *testing.T) {
	ch := models.ListExportCharactersRow{
		ID: 43, Name: "Nameless NPC", CharacterType: "npc",
		Status: txt("approved"), IsActive: false, CreatedAt: tstz(0),
		PlayerUsername: nullText(),
	}

	out := RenderCharacter(ch, nil)

	assert.Contains(t, out, "unassigned")
	assert.Contains(t, out, "inactive")
	assert.Contains(t, out, "active: false")
}

func TestRenderHandout(t *testing.T) {
	h := models.ListExportHandoutsRow{
		ID: 5, Title: "The Cellar Map", Content: "A [color:red]crude[/color] sketch.",
		Status: "published", CreatedAt: tstz(0), UpdatedAt: tstz(10),
	}

	out := RenderHandout(h)

	assert.Contains(t, out, "# The Cellar Map")
	assert.Contains(t, out, "A crude sketch.")
	assert.NotContains(t, out, "[color:red]")
}

// Frontmatter must survive hostile titles without corrupting the YAML block.
func TestYamlScalar_QuotesDangerousValues(t *testing.T) {
	tests := []struct{ in, want string }{
		{"simple", "simple"},
		{"", `""`},
		{"has: colon", `"has: colon"`},
		{"has \"quotes\"", `"has \"quotes\""`},
		{"line\nbreak", `"line\nbreak"`},
		{"#comment", `"#comment"`},
		{" leading space", `" leading space"`},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, yamlScalar(tt.in), "input=%q", tt.in)
	}
}

func TestRenderPost_HostileTitleDoesNotBreakFrontmatter(t *testing.T) {
	p := samplePost()
	p.CharacterName = "Evil: \"Name\"\n---\ninjected: true"

	out := RenderPost(p, nil, "Phase 1")

	// The frontmatter block must remain exactly one leading "---" ... "---".
	require.True(t, strings.HasPrefix(out, "---\n"))
	rest := out[4:]
	end := strings.Index(rest, "\n---\n")
	require.Greater(t, end, 0, "frontmatter must terminate")
	block := rest[:end]
	assert.NotContains(t, block, "\ninjected: true",
		"injected YAML must be escaped into a scalar, not become a key")
}

func TestIndentBlock(t *testing.T) {
	assert.Equal(t, "text", indentBlock("text", 0))
	assert.Equal(t, "> text", indentBlock("text", 1))
	assert.Equal(t, "> > text", indentBlock("text", 2))
	// Blank lines get a bare marker so the quote block is not broken.
	assert.Equal(t, "> a\n>\n> b", indentBlock("a\n\nb", 1))
}

func TestHumanize(t *testing.T) {
	assert.Equal(t, "Basic Info", humanize("basic_info"))
	assert.Equal(t, "Lockpicking", humanize("lockpicking"))
	assert.Equal(t, "", humanize(""))
}
