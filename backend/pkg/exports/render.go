package exports

import (
	"fmt"
	"sort"
	"strings"

	models "actionphase/pkg/db/models"

	"github.com/jackc/pgx/v5/pgtype"
)

// maxIndentDepth caps blockquote nesting in rendered comment trees.
//
// This is deliberately far deeper than the web UI's COMMENT_MAX_DEPTH (5). The
// UI caps low because a fixed-width column turns deep nesting into an unusable
// sliver, and it can offer a "Continue thread" affordance to escape. A text
// archive has neither the width constraint nor an interactive escape hatch, so
// flattening there destroys information instead of saving space: sibling
// branches collapse to the same indent and become indistinguishable.
//
// Observed threads run wide rather than deep, reaching roughly 20 levels, so
// nesting is preserved across that whole range. The cap remains only as a
// safety valve against pathological depth; past it, replies are emitted flat
// with an explicit backlink to the parent's id.
const maxIndentDepth = 20

// wideReplyThreshold is the number of direct replies at which a comment is
// annotated with its reply count. Wide fan-out is the common shape in real
// games (a notable event drawing 100+ responses), and a run of same-indent
// siblings gives a reader no way to tell where it ends or whether the archive
// truncated it. The count is the static equivalent of the UI's
// "Continue this thread (N replies)" label.
const wideReplyThreshold = 5

// timeFormat is used for every rendered timestamp. Fixed UTC layout keeps
// archives byte-stable regardless of the server's local timezone.
const timeFormat = "2006-01-02 15:04:05 UTC"

// fmtTime renders a nullable timestamp, or "unknown" when not set.
func fmtTime(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return "unknown"
	}
	return ts.Time.UTC().Format(timeFormat)
}

// fmtTimeNoTZ renders the timestamp columns declared without a timezone.
func fmtTimeNoTZ(ts pgtype.Timestamp) string {
	if !ts.Valid {
		return "unknown"
	}
	return ts.Time.UTC().Format(timeFormat)
}

// text unwraps a nullable text column with a fallback for NULL/empty.
func text(v pgtype.Text, fallback string) string {
	if !v.Valid || v.String == "" {
		return fallback
	}
	return v.String
}

// speaker renders "Character (username)", degrading gracefully when a
// character was detached (ON DELETE SET NULL) or never set.
func speaker(characterName pgtype.Text, username string) string {
	if name := text(characterName, ""); name != "" {
		return fmt.Sprintf("%s (%s)", name, username)
	}
	return username
}

// frontmatter renders a YAML block. Keys are emitted in the given order so
// regenerating an unchanged game produces byte-identical output.
func frontmatter(pairs [][2]string) string {
	var b strings.Builder
	b.WriteString("---\n")
	for _, kv := range pairs {
		b.WriteString(kv[0])
		b.WriteString(": ")
		b.WriteString(yamlScalar(kv[1]))
		b.WriteString("\n")
	}
	b.WriteString("---\n\n")
	return b.String()
}

// yamlScalar quotes a value when needed so that titles containing colons,
// quotes, or leading indicators cannot corrupt the frontmatter block.
func yamlScalar(v string) string {
	if v == "" {
		return `""`
	}
	if strings.ContainsAny(v, ":#\"'\n\r\t{}[]|>&*!%@`") ||
		strings.HasPrefix(v, " ") || strings.HasSuffix(v, " ") {
		escaped := strings.ReplaceAll(v, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		escaped = strings.ReplaceAll(escaped, "\n", `\n`)
		escaped = strings.ReplaceAll(escaped, "\r", `\r`)
		escaped = strings.ReplaceAll(escaped, "\t", `\t`)
		return `"` + escaped + `"`
	}
	return v
}

// body prepares stored Markdown for the archive: custom syntaxes stripped,
// line endings normalized, trailing whitespace trimmed.
func body(content string) string {
	stripped := StripCustomMarkdown(content)
	stripped = strings.ReplaceAll(stripped, "\r\n", "\n")
	return strings.TrimRight(stripped, " \t\n")
}

// quotePrefix builds the blockquote marker for a given indent level.
func quotePrefix(level int) string {
	if level <= 0 {
		return ""
	}
	return strings.Repeat("> ", level)
}

// indentBlock prefixes every line of text with a blockquote marker, so
// multi-line replies stay inside their quote level.
func indentBlock(text string, level int) string {
	if level <= 0 {
		return text
	}
	prefix := quotePrefix(level)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line == "" {
			lines[i] = strings.TrimRight(prefix, " ")
		} else {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

// RenderPost renders one common room post plus its full comment tree.
//
// comments must be in materialized-path order (parents before children), which
// ListExportCommentTree guarantees; this function does a single pass and never
// re-sorts.
func RenderPost(
	post models.ListExportPostsRow,
	comments []models.ListExportCommentTreeRow,
	phaseTitle string,
) string {
	var b strings.Builder

	title := firstLineTitle(post.Content)

	b.WriteString(frontmatter([][2]string{
		{"type", "post"},
		{"id", fmt.Sprintf("%d", post.ID)},
		{"phase", phaseTitle},
		{"character", post.CharacterName},
		{"author", post.AuthorUsername},
		{"created", fmtTimeNoTZ(post.CreatedAt)},
		{"edited", fmt.Sprintf("%t", post.IsEdited)},
		{"comment_count", fmt.Sprintf("%d", len(comments))},
	}))

	b.WriteString("# " + title + "\n\n")
	b.WriteString(fmt.Sprintf("**%s** — %s\n\n",
		speakerFromPost(post), fmtTimeNoTZ(post.CreatedAt)))
	b.WriteString(body(post.Content))
	b.WriteString("\n")

	if len(comments) == 0 {
		return b.String()
	}

	// depthOf maps a comment id to its rendered indent level so children can be
	// placed relative to their parent without a second traversal.
	depthOf := map[int32]int{post.ID: 0}
	// idToLabel lets a flattened deep reply name its parent.
	idToLabel := map[int32]string{}

	// Direct-reply counts, computed up front so a parent can advertise its
	// fan-out before its children are rendered.
	directReplies := map[int32]int{}
	for _, c := range comments {
		if c.ParentID.Valid {
			directReplies[c.ParentID.Int32]++
		}
	}

	b.WriteString("\n## Comments\n\n")

	for _, c := range comments {
		parentDepth := 0
		if c.ParentID.Valid {
			if d, ok := depthOf[c.ParentID.Int32]; ok {
				parentDepth = d
			}
		}
		depth := parentDepth + 1
		depthOf[c.ID] = depth

		label := speakerLabel(c.CharacterName, c.AuthorUsername)
		idToLabel[c.ID] = label

		indent := depth
		flattened := depth > maxIndentDepth
		if flattened {
			indent = maxIndentDepth
		}

		// The id is always emitted: it is the anchor that every "replying to"
		// backlink below points at, and without it those references resolve to
		// nothing in the file.
		header := fmt.Sprintf("**%s** · #%d — %s", label, c.ID, fmtTimeNoTZ(c.CreatedAt))
		if c.IsEdited {
			header += " *(edited)*"
		}
		// Signpost wide fan-out so a long run of same-indent siblings is
		// legible as one group of known size rather than an open-ended wall.
		if n := directReplies[c.ID]; n >= wideReplyThreshold {
			header += fmt.Sprintf(" · %d replies", n)
		}

		// Past the indent cap, state the parent explicitly: the visual nesting
		// can no longer convey it, but the relationship must not be lost.
		if flattened && c.ParentID.Valid {
			parentLabel := idToLabel[c.ParentID.Int32]
			if parentLabel == "" {
				parentLabel = "the post"
			}
			header += fmt.Sprintf("\n↳ replying to %s (#%d) — depth %d",
				parentLabel, c.ParentID.Int32, depth)
		}

		entry := header + "\n\n" + body(c.Content)
		b.WriteString(indentBlock(entry, indent))
		b.WriteString("\n\n")
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

func speakerFromPost(p models.ListExportPostsRow) string {
	return fmt.Sprintf("%s (%s)", p.CharacterName, p.AuthorUsername)
}

func speakerLabel(characterName string, username string) string {
	if characterName == "" {
		return username
	}
	return fmt.Sprintf("%s (%s)", characterName, username)
}

// firstLineTitle derives a heading from a post body, since posts have no title
// column. Falls back to a generic label for empty or non-text openings.
func firstLineTitle(content string) string {
	stripped := StripCustomMarkdown(content)
	for _, line := range strings.Split(stripped, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "#>*_-` \t")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len([]rune(line)) > 80 {
			line = string([]rune(line)[:80]) + "…"
		}
		return line
	}
	return "Untitled post"
}

// RenderConversation renders one private conversation as a transcript.
func RenderConversation(
	conv models.ListExportConversationsRow,
	participants []models.ListExportConversationParticipantsRow,
	messages []models.ListExportPrivateMessagesRow,
) string {
	var b strings.Builder

	names := make([]string, 0, len(participants))
	seen := map[string]bool{}
	for _, p := range participants {
		label := speaker(p.CharacterName, p.Username)
		if !seen[label] {
			seen[label] = true
			names = append(names, label)
		}
	}
	sort.Strings(names)

	title := text(conv.Title, "")
	if title == "" {
		title = strings.Join(names, ", ")
	}
	if title == "" {
		title = "Conversation"
	}

	b.WriteString(frontmatter([][2]string{
		{"type", "conversation"},
		{"id", fmt.Sprintf("%d", conv.ID)},
		{"conversation_type", conv.ConversationType},
		{"participants", strings.Join(names, ", ")},
		{"created", fmtTime(conv.CreatedAt)},
		{"message_count", fmt.Sprintf("%d", len(messages))},
	}))

	b.WriteString("# " + title + "\n\n")
	if len(names) > 0 {
		b.WriteString("**Participants:** " + strings.Join(names, ", ") + "\n\n")
	}
	b.WriteString("---\n\n")

	for _, m := range messages {
		label := speaker(m.SenderCharacterName, m.SenderUsername)
		header := fmt.Sprintf("**%s** — %s", label, fmtTime(m.CreatedAt))
		if m.IsEdited {
			header += " *(edited)*"
		}
		b.WriteString(header + "\n\n")
		b.WriteString(body(m.Content))
		b.WriteString("\n\n")
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

// RenderActionFile renders one player's action submission together with every
// result answering it, so the exchange reads as a unit.
//
// A submission may have zero results (never answered), one, or several — a GM
// can stage reveals over time. Results are rendered in send order and numbered
// when there is more than one, so the sequence is legible in the archive.
func RenderActionFile(
	sub models.ListExportActionSubmissionsRow,
	results []models.ListExportActionResultsRow,
	phaseTitle string,
) string {
	var b strings.Builder

	ordered := orderResults(results)

	fm := [][2]string{
		{"type", "action"},
		{"id", fmt.Sprintf("%d", sub.ID)},
		{"phase", phaseTitle},
		{"character", text(sub.CharacterName, "")},
		{"player", sub.Username},
		{"submitted", fmtTime(sub.SubmittedAt)},
		{"has_result", fmt.Sprintf("%t", len(ordered) > 0)},
		{"result_count", fmt.Sprintf("%d", len(ordered))},
	}
	b.WriteString(frontmatter(fm))

	b.WriteString(fmt.Sprintf("# Action — %s\n\n", speaker(sub.CharacterName, sub.Username)))
	b.WriteString(fmt.Sprintf("*Submitted %s*\n\n", fmtTime(sub.SubmittedAt)))
	b.WriteString("## Submission\n\n")
	b.WriteString(body(sub.Content))
	b.WriteString("\n")

	for i, r := range ordered {
		heading := "## Result"
		if len(ordered) > 1 {
			heading = fmt.Sprintf("## Result %d of %d", i+1, len(ordered))
		}
		b.WriteString("\n" + heading + "\n\n")
		b.WriteString(fmt.Sprintf("*From %s — %s*\n\n", r.GmUsername, fmtTime(r.SentAt)))
		b.WriteString(body(r.Content))
		b.WriteString("\n")
	}

	return b.String()
}

// orderResults puts one player's results into reading order.
//
// Send time alone is not enough. A staged chain is published in a single
// UPDATE that sets `sent_at = COALESCE(sent_at, NOW())` for every part at once
// (phases.sql:406), so all parts of a chain share one identical timestamp —
// this is the normal shape of published chain, not an edge case. Sorting on a
// tied key leaves the order to whatever the scan produced, which on real data
// came back reversed: the payoff rendered as "Result 1 of 3" and the setup
// last.
//
// So: sort by send time to place separate chains and standalone results
// relative to each other, then walk each chain from its head through
// parent_result_id, which is the only authoritative statement of narrative
// order.
func orderResults(results []models.ListExportActionResultsRow) []models.ListExportActionResultsRow {
	byID := make(map[int32]models.ListExportActionResultsRow, len(results))
	children := make(map[int32][]int32, len(results))
	for _, r := range results {
		byID[r.ID] = r
		if r.ParentResultID.Valid {
			children[r.ParentResultID.Int32] = append(children[r.ParentResultID.Int32], r.ID)
		}
	}

	// Chain heads, in send order. A row whose parent is absent from this set is
	// treated as a head so an orphaned follower is still rendered rather than
	// silently dropped.
	heads := make([]models.ListExportActionResultsRow, 0, len(results))
	for _, r := range results {
		if _, hasParent := byID[r.ParentResultID.Int32]; !r.ParentResultID.Valid || !hasParent {
			heads = append(heads, r)
		}
	}
	sort.SliceStable(heads, func(i, j int) bool {
		if heads[i].SentAt.Valid != heads[j].SentAt.Valid {
			return heads[i].SentAt.Valid
		}
		if sentBefore(heads[i].SentAt, heads[j].SentAt) {
			return true
		}
		if sentBefore(heads[j].SentAt, heads[i].SentAt) {
			return false
		}
		return heads[i].ID < heads[j].ID
	})

	ordered := make([]models.ListExportActionResultsRow, 0, len(results))
	seen := make(map[int32]bool, len(results))
	// Indexed, not ranged: a chain that forks appends new heads to this slice
	// and they must be visited too.
	for h := 0; h < len(heads); h++ {
		// Follow the chain down. `seen` also bounds the walk if the data ever
		// contains a cycle, so a bad row cannot hang the export.
		for id := heads[h].ID; ; {
			if seen[id] {
				break
			}
			seen[id] = true
			ordered = append(ordered, byID[id])

			next := children[id]
			if len(next) == 0 {
				break
			}
			// A part has one follower by design; sort defensively so a
			// duplicated link is still deterministic.
			sort.Slice(next, func(i, j int) bool { return next[i] < next[j] })
			id = next[0]

			// Any extra followers are appended as their own chains rather than
			// discarded.
			for _, extra := range next[1:] {
				if !seen[extra] {
					heads = append(heads, byID[extra])
				}
			}
		}
	}

	// Anything unreachable from a head (a cycle with no entry point) still
	// belongs in the archive.
	for _, r := range results {
		if !seen[r.ID] {
			seen[r.ID] = true
			ordered = append(ordered, r)
		}
	}

	return ordered
}

// sentBefore orders results by send time, placing rows with no timestamp last
// so they never displace dated results.
func sentBefore(a, b pgtype.Timestamptz) bool {
	if !a.Valid {
		return false
	}
	if !b.Valid {
		return true
	}
	return a.Time.Before(b.Time)
}

// RenderPoll renders a poll with its options and tallies. Voter identity is
// included only when the poll was configured to show individual votes.
func RenderPoll(
	poll models.ListExportPollsRow,
	options []models.ListExportPollOptionsRow,
	votes []models.ListExportPollVotesRow,
) string {
	var b strings.Builder

	// Voter identity is always rendered. In a COMPLETED game every authenticated
	// user is granted canSeeIndividualVotes regardless of the poll's
	// show_individual_votes setting (checkPollViewAccess,
	// backend/pkg/polls/api_polls.go:241), so the archive must match what the
	// completed-game web view shows rather than re-hiding it.
	//
	// Polls flagged hide_results_from_players never reach this renderer: they
	// are filtered out in ListExportPolls, because completion does NOT lift
	// that flag for unprivileged viewers.
	b.WriteString(frontmatter([][2]string{
		{"type", "poll"},
		{"id", fmt.Sprintf("%d", poll.ID)},
		{"question", poll.Question},
		{"creator", speaker(poll.CreatorCharacterName, poll.CreatorUsername)},
		{"deadline", fmtTime(poll.Deadline)},
		{"created", fmtTime(poll.CreatedAt)},
		{"vote_count", fmt.Sprintf("%d", len(votes))},
		{"shown_to_players_during_game", fmt.Sprintf("%t",
			poll.ShowIndividualVotes.Valid && poll.ShowIndividualVotes.Bool)},
	}))

	b.WriteString("# " + poll.Question + "\n\n")
	b.WriteString(fmt.Sprintf("*Asked by %s — closed %s*\n\n",
		speaker(poll.CreatorCharacterName, poll.CreatorUsername), fmtTime(poll.Deadline)))

	if d := text(poll.Description, ""); d != "" {
		b.WriteString(body(d) + "\n\n")
	}

	counts := map[int32]int{}
	voters := map[int32][]string{}
	var others []models.ListExportPollVotesRow
	for _, v := range votes {
		if v.SelectedOptionID.Valid {
			counts[v.SelectedOptionID.Int32]++
			voters[v.SelectedOptionID.Int32] = append(voters[v.SelectedOptionID.Int32], v.VoterUsername)
			continue
		}
		others = append(others, v)
	}

	b.WriteString("## Results\n\n")
	for _, o := range options {
		b.WriteString(fmt.Sprintf("- **%s** — %d vote(s)", o.OptionText, counts[o.ID]))
		if len(voters[o.ID]) > 0 {
			names := append([]string(nil), voters[o.ID]...)
			sort.Strings(names)
			b.WriteString(": " + strings.Join(names, ", "))
		}
		b.WriteString("\n")
	}

	if len(others) > 0 {
		b.WriteString("\n### Write-in responses\n\n")
		for _, v := range others {
			b.WriteString(fmt.Sprintf("- %s — %s\n",
				v.VoterUsername, text(v.OtherResponse, "(blank)")))
		}
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

// RenderCharacter renders a character sheet from its modular character_data
// rows. Avatars are intentionally omitted: archives are text-only.
func RenderCharacter(
	ch models.ListExportCharactersRow,
	data []models.ListExportCharacterDataRow,
) string {
	var b strings.Builder

	status := text(ch.Status, "unknown")
	state := "active"
	if !ch.IsActive {
		state = "inactive"
	}

	b.WriteString(frontmatter([][2]string{
		{"type", "character"},
		{"id", fmt.Sprintf("%d", ch.ID)},
		{"name", ch.Name},
		{"character_type", ch.CharacterType},
		{"status", status},
		{"active", fmt.Sprintf("%t", ch.IsActive)},
		{"player", text(ch.PlayerUsername, "unassigned")},
		{"created", fmtTime(ch.CreatedAt)},
	}))

	b.WriteString("# " + ch.Name + "\n\n")
	b.WriteString(fmt.Sprintf("- **Player:** %s\n", text(ch.PlayerUsername, "unassigned")))
	b.WriteString(fmt.Sprintf("- **Type:** %s\n", ch.CharacterType))
	b.WriteString(fmt.Sprintf("- **Status:** %s (%s)\n\n", status, state))

	// Group sheet fields by module, preserving the query's ordering.
	var moduleOrder []string
	byModule := map[string][]models.ListExportCharacterDataRow{}
	for _, d := range data {
		if _, seen := byModule[d.ModuleType]; !seen {
			moduleOrder = append(moduleOrder, d.ModuleType)
		}
		byModule[d.ModuleType] = append(byModule[d.ModuleType], d)
	}

	for _, module := range moduleOrder {
		b.WriteString("## " + humanize(module) + "\n\n")
		for _, f := range byModule[module] {
			value := text(f.FieldValue, "")
			if value == "" {
				continue
			}
			b.WriteString(fmt.Sprintf("**%s:**\n\n%s\n\n", humanize(f.FieldName), body(value)))
		}
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

// RenderHandout renders a published GM handout.
func RenderHandout(h models.ListExportHandoutsRow) string {
	var b strings.Builder
	b.WriteString(frontmatter([][2]string{
		{"type", "handout"},
		{"id", fmt.Sprintf("%d", h.ID)},
		{"title", h.Title},
		{"created", fmtTime(h.CreatedAt)},
		{"updated", fmtTime(h.UpdatedAt)},
	}))
	b.WriteString("# " + h.Title + "\n\n")
	b.WriteString(body(h.Content))
	b.WriteString("\n")
	return b.String()
}

// humanize turns a snake_case identifier into a display label.
func humanize(s string) string {
	parts := strings.Split(strings.ReplaceAll(s, "_", " "), " ")
	for i, p := range parts {
		if p == "" {
			continue
		}
		r := []rune(p)
		parts[i] = strings.ToUpper(string(r[0])) + string(r[1:])
	}
	return strings.Join(parts, " ")
}
