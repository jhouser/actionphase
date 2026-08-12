package exports

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	core "actionphase/pkg/core"
	models "actionphase/pkg/db/models"

	"github.com/jackc/pgx/v5/pgtype"
)

// Querier is the subset of the generated query API the assembler needs.
// Declaring it here (rather than depending on *models.Queries) keeps the
// assembler unit-testable with an in-memory fake and no database.
type Querier interface {
	GetGame(ctx context.Context, id int32) (models.Game, error)
	ListExportPhases(ctx context.Context, gameID int32) ([]models.ListExportPhasesRow, error)
	ListExportCharacters(ctx context.Context, gameID int32) ([]models.ListExportCharactersRow, error)
	ListExportCharacterData(ctx context.Context, gameID int32) ([]models.ListExportCharacterDataRow, error)
	ListExportParticipants(ctx context.Context, gameID int32) ([]models.ListExportParticipantsRow, error)
	ListExportPosts(ctx context.Context, gameID int32) ([]models.ListExportPostsRow, error)
	ListExportCommentTree(ctx context.Context, parentID pgtype.Int4) ([]models.ListExportCommentTreeRow, error)
	ListExportConversations(ctx context.Context, gameID int32) ([]models.ListExportConversationsRow, error)
	ListExportConversationParticipants(ctx context.Context, gameID int32) ([]models.ListExportConversationParticipantsRow, error)
	ListExportPrivateMessages(ctx context.Context, gameID int32) ([]models.ListExportPrivateMessagesRow, error)
	ListExportActionSubmissions(ctx context.Context, gameID int32) ([]models.ListExportActionSubmissionsRow, error)
	ListExportActionResults(ctx context.Context, gameID int32) ([]models.ListExportActionResultsRow, error)
	ListExportHandouts(ctx context.Context, gameID int32) ([]models.ListExportHandoutsRow, error)
	ListExportPolls(ctx context.Context, gameID int32) ([]models.ListExportPollsRow, error)
	ListExportPollOptions(ctx context.Context, gameID int32) ([]models.ListExportPollOptionsRow, error)
	ListExportPollVotes(ctx context.Context, gameID int32) ([]models.ListExportPollVotesRow, error)
	GetGameContentFingerprint(ctx context.Context, id int32) (models.GetGameContentFingerprintRow, error)
}

// ProgressFunc reports a human-readable step so long jobs can surface status.
type ProgressFunc func(note string)

// Assembler builds a game's archive. It is stateless between calls.
type Assembler struct {
	Queries Querier
}

// Result summarizes a completed assembly.
type Result struct {
	FileCount int
	Bytes     int64
}

// manifestEntry describes one archive file in manifest.json, so the archive is
// machine-readable without parsing every Markdown file.
type manifestEntry struct {
	Path  string `json:"path"`
	Type  string `json:"type"`
	ID    int32  `json:"id,omitempty"`
	Title string `json:"title,omitempty"`
	Phase string `json:"phase,omitempty"`
	Bytes int    `json:"bytes"`
}

type manifest struct {
	GameID      int32           `json:"game_id"`
	GameTitle   string          `json:"game_title"`
	GeneratedAt string          `json:"generated_at"`
	Fingerprint string          `json:"content_fingerprint"`
	Format      string          `json:"format"`
	Files       []manifestEntry `json:"files"`
}

// Assemble writes the full archive for gameID as a ZIP into w.
//
// Only COMPLETED games may be exported: the archive discloses private
// conversations, action submissions, and results, which is only permissible
// under public archive mode (CanUserViewGame). Refusing here means no caller
// can produce an over-disclosing artifact by mistake.
func (a *Assembler) Assemble(ctx context.Context, gameID int32, w io.Writer, progress ProgressFunc) (*Result, error) {
	report := func(note string) {
		if progress != nil {
			progress(note)
		}
	}

	game, err := a.Queries.GetGame(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("load game %d: %w", gameID, err)
	}
	if !game.State.Valid || game.State.String != core.GameStateCompleted {
		return nil, fmt.Errorf("%w: game %d is %q",
			ErrGameNotCompleted, gameID, stateOrUnknown(game.State))
	}

	fpRow, err := a.Queries.GetGameContentFingerprint(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("fingerprint game %d: %w", gameID, err)
	}
	fingerprint := Fingerprint(fpRow)

	root := GameDirName(gameID, game.Title)
	zw := zip.NewWriter(w)
	counting := &countingWriter{}

	ar := &archive{
		zw:       zw,
		root:     root,
		counting: counting,
	}

	report("loading game content")
	phases, err := a.Queries.ListExportPhases(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("list phases: %w", err)
	}
	// phaseDir holds the FULL archive-relative directory ("phases/02-action-x"),
	// not the bare name, so every writer places phase content identically.
	phaseByID := map[int32]models.ListExportPhasesRow{}
	phaseDir := map[int32]string{}
	for _, p := range phases {
		phaseByID[p.ID] = p
		phaseDir[p.ID] = path.Join("phases", PhaseDirName(p.PhaseNumber, p.PhaseType, p.Title))
	}

	if err := a.writeCharacters(ctx, ar, gameID); err != nil {
		return nil, err
	}
	report("writing common room posts")
	if err := a.writePosts(ctx, ar, gameID, phaseByID, phaseDir); err != nil {
		return nil, err
	}
	report("writing polls")
	if err := a.writePolls(ctx, ar, gameID, phaseByID, phaseDir); err != nil {
		return nil, err
	}
	report("writing actions and results")
	if err := a.writeActions(ctx, ar, gameID, phaseByID, phaseDir); err != nil {
		return nil, err
	}
	report("writing private conversations")
	if err := a.writeConversations(ctx, ar, gameID); err != nil {
		return nil, err
	}
	report("writing handouts")
	if err := a.writeHandouts(ctx, ar, gameID); err != nil {
		return nil, err
	}

	// Phase READMEs after their content, so counts are known.
	if err := a.writePhaseReadmes(ctx, ar, phases, phaseDir); err != nil {
		return nil, err
	}

	report("writing index")
	if err := a.writeGameReadme(ctx, ar, game, phases, gameID); err != nil {
		return nil, err
	}

	m := manifest{
		GameID:      gameID,
		GameTitle:   game.Title,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Fingerprint: fingerprint,
		Format:      "markdown",
		Files:       ar.entries,
	}
	manifestJSON, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode manifest: %w", err)
	}
	// Written last but not itself listed: a manifest cannot describe its own size.
	if err := ar.writeRaw("manifest.json", manifestJSON); err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("finalize zip: %w", err)
	}

	return &Result{FileCount: ar.fileCount, Bytes: counting.n}, nil
}

// FingerprintFor returns the current content fingerprint without assembling.
func (a *Assembler) FingerprintFor(ctx context.Context, gameID int32) (string, error) {
	row, err := a.Queries.GetGameContentFingerprint(ctx, gameID)
	if err != nil {
		return "", fmt.Errorf("fingerprint game %d: %w", gameID, err)
	}
	return Fingerprint(row), nil
}

func stateOrUnknown(s pgtype.Text) string {
	if !s.Valid {
		return "unknown"
	}
	return s.String
}

// --- section writers -------------------------------------------------------

func (a *Assembler) writeCharacters(ctx context.Context, ar *archive, gameID int32) error {
	chars, err := a.Queries.ListExportCharacters(ctx, gameID)
	if err != nil {
		return fmt.Errorf("list characters: %w", err)
	}
	if len(chars) == 0 {
		return nil
	}
	data, err := a.Queries.ListExportCharacterData(ctx, gameID)
	if err != nil {
		return fmt.Errorf("list character data: %w", err)
	}
	byChar := map[int32][]models.ListExportCharacterDataRow{}
	for _, d := range data {
		byChar[d.CharacterID] = append(byChar[d.CharacterID], d)
	}

	var index strings.Builder
	index.WriteString("# Characters\n\n")

	used := map[string]bool{}
	for _, ch := range chars {
		name := uniqueName(used, Slug(ch.Name, fmt.Sprintf("character-%d", ch.ID)))
		p := path.Join("characters", name+".md")
		if err := ar.writeFile(p, RenderCharacter(ch, byChar[ch.ID]), manifestEntry{
			Type: "character", ID: ch.ID, Title: ch.Name,
		}); err != nil {
			return err
		}
		state := "active"
		if !ch.IsActive {
			state = "inactive"
		}
		index.WriteString(fmt.Sprintf("- [%s](%s.md) — %s, %s (player: %s)\n",
			ch.Name, name, ch.CharacterType, state, text(ch.PlayerUsername, "unassigned")))
	}

	return ar.writeFile(path.Join("characters", "README.md"), index.String(), manifestEntry{
		Type: "index", Title: "Characters",
	})
}

func (a *Assembler) writePosts(
	ctx context.Context,
	ar *archive,
	gameID int32,
	phaseByID map[int32]models.ListExportPhasesRow,
	phaseDir map[int32]string,
) error {
	posts, err := a.Queries.ListExportPosts(ctx, gameID)
	if err != nil {
		return fmt.Errorf("list posts: %w", err)
	}

	// Number posts per phase so filenames sort in posting order.
	counters := map[string]int{}
	used := map[string]bool{}

	for _, p := range posts {
		dir, phaseTitle := phaseLocation(p.PhaseID, phaseByID, phaseDir)
		comments, err := a.Queries.ListExportCommentTree(ctx, pgtype.Int4{Int32: p.ID, Valid: true})
		if err != nil {
			return fmt.Errorf("comment tree for post %d: %w", p.ID, err)
		}

		counters[dir]++
		title := firstLineTitle(p.Content)
		name := NumberedSlug(counters[dir], title, "post")
		full := uniqueName(used, path.Join(dir, "posts", name))

		if err := ar.writeFile(full+".md", RenderPost(p, comments, phaseTitle), manifestEntry{
			Type: "post", ID: p.ID, Title: title, Phase: phaseTitle,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (a *Assembler) writePolls(
	ctx context.Context,
	ar *archive,
	gameID int32,
	phaseByID map[int32]models.ListExportPhasesRow,
	phaseDir map[int32]string,
) error {
	polls, err := a.Queries.ListExportPolls(ctx, gameID)
	if err != nil {
		return fmt.Errorf("list polls: %w", err)
	}
	if len(polls) == 0 {
		return nil
	}
	options, err := a.Queries.ListExportPollOptions(ctx, gameID)
	if err != nil {
		return fmt.Errorf("list poll options: %w", err)
	}
	votes, err := a.Queries.ListExportPollVotes(ctx, gameID)
	if err != nil {
		return fmt.Errorf("list poll votes: %w", err)
	}
	optsByPoll := map[int32][]models.ListExportPollOptionsRow{}
	for _, o := range options {
		optsByPoll[o.PollID] = append(optsByPoll[o.PollID], o)
	}
	votesByPoll := map[int32][]models.ListExportPollVotesRow{}
	for _, v := range votes {
		votesByPoll[v.PollID] = append(votesByPoll[v.PollID], v)
	}

	counters := map[string]int{}
	used := map[string]bool{}
	for _, p := range polls {
		dir, phaseTitle := phaseLocation(p.PhaseID, phaseByID, phaseDir)
		counters[dir]++
		name := NumberedSlug(counters[dir], p.Question, "poll")
		full := uniqueName(used, path.Join(dir, "polls", name))

		if err := ar.writeFile(full+".md",
			RenderPoll(p, optsByPoll[p.ID], votesByPoll[p.ID]),
			manifestEntry{Type: "poll", ID: p.ID, Title: p.Question, Phase: phaseTitle},
		); err != nil {
			return err
		}
	}
	return nil
}

func (a *Assembler) writeActions(
	ctx context.Context,
	ar *archive,
	gameID int32,
	phaseByID map[int32]models.ListExportPhasesRow,
	phaseDir map[int32]string,
) error {
	subs, err := a.Queries.ListExportActionSubmissions(ctx, gameID)
	if err != nil {
		return fmt.Errorf("list action submissions: %w", err)
	}
	results, err := a.Queries.ListExportActionResults(ctx, gameID)
	if err != nil {
		return fmt.Errorf("list action results: %w", err)
	}

	// Attach results to the submission they answer.
	//
	// action_results.action_submission_id is nullable and is legitimately unset
	// for results that answer no submission (GM-initiated narration). A phase
	// may also carry SEVERAL results for one player — staged reveals sent over
	// time. So results are collected into a SLICE per submission, never a
	// single value: overwriting would silently drop archived content.
	//
	// Matching is by FK when set; otherwise by (phase_id, user_id), which
	// action_submissions makes unique via UNIQUE(game_id, user_id, phase_id).
	type subKey struct{ phaseID, userID int32 }

	subIDToKey := map[int32]subKey{}
	knownKeys := map[subKey]bool{}
	for _, s := range subs {
		k := subKey{s.PhaseID, s.UserID}
		subIDToKey[s.ID] = k
		knownKeys[k] = true
	}

	resultsByKey := map[subKey][]models.ListExportActionResultsRow{}
	var orphanResults []models.ListExportActionResultsRow
	for _, r := range results {
		key := subKey{r.PhaseID, r.UserID}
		// Prefer the explicit FK when it points at a submission we exported.
		if r.ActionSubmissionID.Valid {
			if k, ok := subIDToKey[r.ActionSubmissionID.Int32]; ok {
				key = k
			}
		}
		if knownKeys[key] {
			resultsByKey[key] = append(resultsByKey[key], r)
			continue
		}
		// No exported submission answers this result — either it was
		// GM-initiated, or the submission was deleted or left a draft. It
		// still belongs in the archive rather than being dropped.
		orphanResults = append(orphanResults, r)
	}

	used := map[string]bool{}
	for _, s := range subs {
		dir, phaseTitle := phaseLocation(pgtype.Int4{Int32: s.PhaseID, Valid: true}, phaseByID, phaseDir)
		who := text(s.CharacterName, s.Username)
		name := uniqueName(used, path.Join(dir, "actions", Slug(who, fmt.Sprintf("action-%d", s.ID))))

		if err := ar.writeFile(name+".md",
			RenderActionFile(s, resultsByKey[subKey{s.PhaseID, s.UserID}], phaseTitle),
			manifestEntry{Type: "action", ID: s.ID, Title: who, Phase: phaseTitle},
		); err != nil {
			return err
		}
	}

	for _, r := range orphanResults {
		dir, phaseTitle := phaseLocation(pgtype.Int4{Int32: r.PhaseID, Valid: true}, phaseByID, phaseDir)
		who := text(r.CharacterName, r.RecipientUsername)
		name := uniqueName(used, path.Join(dir, "actions",
			Slug(who, fmt.Sprintf("result-%d", r.ID))+"-result"))

		// A result with no matching submission is expected, not anomalous: the
		// GM may send narration unprompted. Only claim a submission is missing
		// when the row actually pointed at one.
		note := "no associated action submission"
		if r.ActionSubmissionID.Valid {
			note = "referenced action submission no longer present"
		}

		content := frontmatter([][2]string{
			{"type", "action_result"},
			{"id", fmt.Sprintf("%d", r.ID)},
			{"phase", phaseTitle},
			{"recipient", r.RecipientUsername},
			{"gm", r.GmUsername},
			{"sent", fmtTime(r.SentAt)},
			{"note", note},
		}) + fmt.Sprintf("# Result — %s\n\n*From %s — %s*\n\n%s\n",
			who, r.GmUsername, fmtTime(r.SentAt), body(r.Content))

		if err := ar.writeFile(name+".md", content, manifestEntry{
			Type: "action_result", ID: r.ID, Title: who, Phase: phaseTitle,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (a *Assembler) writeConversations(ctx context.Context, ar *archive, gameID int32) error {
	convs, err := a.Queries.ListExportConversations(ctx, gameID)
	if err != nil {
		return fmt.Errorf("list conversations: %w", err)
	}
	if len(convs) == 0 {
		return nil
	}
	parts, err := a.Queries.ListExportConversationParticipants(ctx, gameID)
	if err != nil {
		return fmt.Errorf("list conversation participants: %w", err)
	}
	msgs, err := a.Queries.ListExportPrivateMessages(ctx, gameID)
	if err != nil {
		return fmt.Errorf("list private messages: %w", err)
	}

	partsByConv := map[int32][]models.ListExportConversationParticipantsRow{}
	for _, p := range parts {
		partsByConv[p.ConversationID] = append(partsByConv[p.ConversationID], p)
	}
	msgsByConv := map[int32][]models.ListExportPrivateMessagesRow{}
	for _, m := range msgs {
		msgsByConv[m.ConversationID] = append(msgsByConv[m.ConversationID], m)
	}

	used := map[string]bool{}
	var index strings.Builder
	index.WriteString("# Private Conversations\n\n")

	for i, c := range convs {
		// An empty conversation carries no archival value.
		if len(msgsByConv[c.ID]) == 0 {
			continue
		}
		label := conversationLabel(c, partsByConv[c.ID])
		name := uniqueName(used, NumberedSlug(i+1, label, "conversation"))
		p := path.Join("conversations", name+".md")

		if err := ar.writeFile(p,
			RenderConversation(c, partsByConv[c.ID], msgsByConv[c.ID]),
			manifestEntry{Type: "conversation", ID: c.ID, Title: label},
		); err != nil {
			return err
		}
		index.WriteString(fmt.Sprintf("- [%s](%s.md) — %d message(s)\n",
			label, name, len(msgsByConv[c.ID])))
	}

	return ar.writeFile(path.Join("conversations", "README.md"), index.String(),
		manifestEntry{Type: "index", Title: "Private Conversations"})
}

func (a *Assembler) writeHandouts(ctx context.Context, ar *archive, gameID int32) error {
	handouts, err := a.Queries.ListExportHandouts(ctx, gameID)
	if err != nil {
		return fmt.Errorf("list handouts: %w", err)
	}
	used := map[string]bool{}
	for _, h := range handouts {
		name := uniqueName(used, Slug(h.Title, fmt.Sprintf("handout-%d", h.ID)))
		if err := ar.writeFile(path.Join("handouts", name+".md"), RenderHandout(h),
			manifestEntry{Type: "handout", ID: h.ID, Title: h.Title},
		); err != nil {
			return err
		}
	}
	return nil
}

func (a *Assembler) writePhaseReadmes(
	_ context.Context,
	ar *archive,
	phases []models.ListExportPhasesRow,
	phaseDir map[int32]string,
) error {
	for _, p := range phases {
		var b strings.Builder
		b.WriteString(frontmatter([][2]string{
			{"type", "phase"},
			{"id", fmt.Sprintf("%d", p.ID)},
			{"phase_number", fmt.Sprintf("%d", p.PhaseNumber)},
			{"phase_type", p.PhaseType},
			{"title", p.Title},
			{"start", fmtTime(p.StartTime)},
			{"end", fmtTime(p.EndTime)},
			{"deadline", fmtTime(p.Deadline)},
		}))
		b.WriteString(fmt.Sprintf("# Phase %d — %s\n\n", p.PhaseNumber, p.Title))
		b.WriteString(fmt.Sprintf("- **Type:** %s\n", p.PhaseType))
		b.WriteString(fmt.Sprintf("- **Started:** %s\n", fmtTime(p.StartTime)))
		if p.Deadline.Valid {
			b.WriteString(fmt.Sprintf("- **Deadline:** %s\n", fmtTime(p.Deadline)))
		}
		if p.EndTime.Valid {
			b.WriteString(fmt.Sprintf("- **Ended:** %s\n", fmtTime(p.EndTime)))
		}
		if d := text(p.Description, ""); d != "" {
			b.WriteString("\n" + body(d) + "\n")
		}

		if err := ar.writeFile(path.Join(phaseDir[p.ID], "README.md"), b.String(),
			manifestEntry{Type: "phase", ID: p.ID, Title: p.Title},
		); err != nil {
			return err
		}
	}
	return nil
}

func (a *Assembler) writeGameReadme(
	ctx context.Context,
	ar *archive,
	game models.Game,
	phases []models.ListExportPhasesRow,
	gameID int32,
) error {
	participants, err := a.Queries.ListExportParticipants(ctx, gameID)
	if err != nil {
		return fmt.Errorf("list participants: %w", err)
	}

	var b strings.Builder
	b.WriteString(frontmatter([][2]string{
		{"type", "game"},
		{"id", fmt.Sprintf("%d", game.ID)},
		{"title", game.Title},
		{"state", stateOrUnknown(game.State)},
		{"genre", text(game.Genre, "")},
		{"started", fmtTime(game.StartDate)},
		{"ended", fmtTime(game.EndDate)},
		{"exported", time.Now().UTC().Format(timeFormat)},
	}))

	b.WriteString("# " + game.Title + "\n\n")
	if d := text(game.Description, ""); d != "" {
		b.WriteString(body(d) + "\n\n")
	}

	b.WriteString("## Overview\n\n")
	b.WriteString(fmt.Sprintf("- **Status:** %s\n", stateOrUnknown(game.State)))
	if g := text(game.Genre, ""); g != "" {
		b.WriteString(fmt.Sprintf("- **Genre:** %s\n", g))
	}
	b.WriteString(fmt.Sprintf("- **Started:** %s\n", fmtTime(game.StartDate)))
	b.WriteString(fmt.Sprintf("- **Ended:** %s\n", fmtTime(game.EndDate)))
	b.WriteString(fmt.Sprintf("- **Phases:** %d\n\n", len(phases)))

	b.WriteString("## Participants\n\n")
	byRole := map[string][]string{}
	for _, p := range participants {
		byRole[p.Role] = append(byRole[p.Role], p.Username)
	}
	roles := make([]string, 0, len(byRole))
	for r := range byRole {
		roles = append(roles, r)
	}
	sort.Strings(roles)
	for _, r := range roles {
		names := byRole[r]
		sort.Strings(names)
		b.WriteString(fmt.Sprintf("- **%s:** %s\n", humanize(r), strings.Join(names, ", ")))
	}

	b.WriteString("\n## Phases\n\n")
	for _, p := range phases {
		b.WriteString(fmt.Sprintf("%d. [%s](phases/%s/README.md) — %s\n",
			p.PhaseNumber, p.Title, PhaseDirName(p.PhaseNumber, p.PhaseType, p.Title), p.PhaseType))
	}

	b.WriteString("\n## Contents\n\n")
	b.WriteString("- `characters/` — character sheets and roster\n")
	b.WriteString("- `phases/` — posts, polls, actions and results, by phase\n")
	b.WriteString("- `conversations/` — private message transcripts\n")
	b.WriteString("- `handouts/` — published GM handouts\n")
	b.WriteString("- `manifest.json` — machine-readable index\n\n")
	b.WriteString("Generated by ActionPhase. Drafts, deleted content, and images are not included.\n")

	return ar.writeFile("README.md", b.String(), manifestEntry{
		Type: "index", ID: game.ID, Title: game.Title,
	})
}

// --- helpers ---------------------------------------------------------------

// phaseLocation resolves the directory and display title for a phase id.
// Content with no phase (or a phase that was never published) lands in an
// "unfiled" directory rather than being dropped.
func phaseLocation(
	phaseID pgtype.Int4,
	phaseByID map[int32]models.ListExportPhasesRow,
	phaseDir map[int32]string,
) (dir string, title string) {
	if phaseID.Valid {
		if p, ok := phaseByID[phaseID.Int32]; ok {
			return phaseDir[p.ID], p.Title
		}
	}
	return path.Join("phases", "00-unfiled"), "Unfiled"
}

func conversationLabel(
	c models.ListExportConversationsRow,
	parts []models.ListExportConversationParticipantsRow,
) string {
	if t := text(c.Title, ""); t != "" {
		return t
	}
	seen := map[string]bool{}
	names := make([]string, 0, len(parts))
	for _, p := range parts {
		n := text(p.CharacterName, p.Username)
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "conversation"
	}
	return strings.Join(names, " and ")
}

// uniqueName appends a numeric suffix when a path is already taken. Slugs are
// derived from free text, so two characters named "Ada" would otherwise
// overwrite each other inside the archive.
func uniqueName(used map[string]bool, name string) string {
	if !used[name] {
		used[name] = true
		return name
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", name, i)
		if !used[candidate] {
			used[candidate] = true
			return candidate
		}
	}
}

// archive wraps zip.Writer with the root prefix and manifest bookkeeping.
type archive struct {
	zw        *zip.Writer
	root      string
	entries   []manifestEntry
	fileCount int
	counting  *countingWriter
}

func (ar *archive) writeFile(relPath, content string, entry manifestEntry) error {
	if err := ar.writeRaw(relPath, []byte(content)); err != nil {
		return err
	}
	entry.Path = relPath
	entry.Bytes = len(content)
	ar.entries = append(ar.entries, entry)
	return nil
}

func (ar *archive) writeRaw(relPath string, content []byte) error {
	full := path.Join(ar.root, relPath)
	f, err := ar.zw.Create(full)
	if err != nil {
		return fmt.Errorf("create zip entry %s: %w", full, err)
	}
	if _, err := f.Write(content); err != nil {
		return fmt.Errorf("write zip entry %s: %w", full, err)
	}
	ar.fileCount++
	ar.counting.n += int64(len(content))
	return nil
}

// countingWriter tracks uncompressed bytes written, for reporting.
type countingWriter struct{ n int64 }
