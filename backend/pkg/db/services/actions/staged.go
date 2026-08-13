package actions

import (
	"context"
	"errors"
	"fmt"

	core "actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"actionphase/pkg/validation"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Staged result reveals.
//
// A chain is a linked list of action_results: part N points at part N-1 via
// parent_result_id, and reveal_delay_minutes is measured from the moment the
// parent became visible. The head is an ordinary result; only parts 2..N carry
// a parent and a delay.
//
// Visibility to the recipient is `is_published = TRUE AND released_at IS NOT
// NULL`. Publishing sets released_at on the head only (in SQL, see
// PublishActionResult); later parts wait for ReleaseDueStagedParts to set
// theirs. That NULL is the whole feature.
//
// See .claude/planning/staged-result-reveals.md.

// CreateStagedResultChain creates an entire chain in one transaction.
//
// All-or-nothing matters more here than in most writes: a chain that is
// partially written contains parts whose parent does not exist, and those parts
// can never become due. There is no repair path short of manual SQL, so the
// transaction is the safeguard.
func (as *ActionSubmissionService) CreateStagedResultChain(ctx context.Context, req core.CreateStagedResultChainRequest) ([]models.ActionResult, error) {
	if err := validateStagedChainRequest(req); err != nil {
		return nil, err
	}

	var created []models.ActionResult

	err := pgx.BeginFunc(ctx, as.DB, func(tx pgx.Tx) error {
		queries := models.New(tx)

		// The GM user ID is taken from the game rather than the request, matching
		// CreateActionResult: the caller's claimed identity is not authoritative.
		game, err := queries.GetGame(ctx, req.GameID)
		if err != nil {
			return fmt.Errorf("failed to get game: %w", err)
		}

		var characterID pgtype.Int4
		if req.CharacterID != nil {
			characterID = pgtype.Int4{Int32: *req.CharacterID, Valid: true}
		}

		var actionSubmissionID pgtype.Int4
		if req.ActionSubmissionID != nil {
			actionSubmissionID = pgtype.Int4{Int32: *req.ActionSubmissionID, Valid: true}
		}

		isPublished := pgtype.Bool{Bool: req.IsPublished, Valid: true}

		// Part 1 is an ordinary result: no parent, no delay, and released_at
		// set by CreateActionResult when publishing immediately. Publishing the
		// head is what starts the clock for everything after it.
		head, err := queries.CreateActionResult(ctx, models.CreateActionResultParams{
			GameID:             req.GameID,
			UserID:             req.UserID,
			PhaseID:            req.PhaseID,
			CharacterID:        characterID,
			ActionSubmissionID: actionSubmissionID,
			GmUserID:           game.GmUserID,
			Content:            fmt.Sprintf("%v", req.Parts[0].Content),
			IsPublished:        isPublished,
		})
		if err != nil {
			return fmt.Errorf("failed to create chain head: %w", err)
		}
		created = append(created, head)

		// Parts 2..N each hang off the previous one. Recipient fields are copied
		// from the request rather than varied per part, so a chain cannot change
		// recipient midway — the invariant holds by construction and needs no
		// separate check.
		parentID := head.ID
		for i, part := range req.Parts[1:] {
			next, err := queries.CreateStagedResultPart(ctx, models.CreateStagedResultPartParams{
				GameID:             req.GameID,
				UserID:             req.UserID,
				PhaseID:            req.PhaseID,
				CharacterID:        characterID,
				ActionSubmissionID: actionSubmissionID,
				GmUserID:           game.GmUserID,
				Content:            fmt.Sprintf("%v", part.Content),
				IsPublished:        isPublished,
				ParentResultID:     pgtype.Int4{Int32: parentID, Valid: true},
				RevealDelayMinutes: pgtype.Int4{Int32: part.DelayMinutes, Valid: true},
			})
			if err != nil {
				// i is the index into Parts[1:], so the human-facing part number
				// is i+2.
				return fmt.Errorf("failed to create chain part %d: %w", i+2, err)
			}
			created = append(created, next)
			parentID = next.ID
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	as.Logger.Info(ctx, "Staged result chain created",
		"head_id", created[0].ID,
		"parts", len(created),
		"game_id", req.GameID,
		"user_id", req.UserID,
		"published", req.IsPublished,
	)

	return created, nil
}

// ReleaseDueStagedParts reveals every part whose wait has elapsed.
//
// Due-ness is computed in SQL from the parent's release time, so this is
// restart-safe: a part that came due while the process was down is simply due on
// the next tick, and there is no in-memory timer to lose or double-fire.
//
// Each part is released in its own transaction. One malformed part therefore
// cannot block the rest of the queue, which matters for a worker that runs
// unattended every minute.
func (as *ActionSubmissionService) ReleaseDueStagedParts(ctx context.Context) (int, int, error) {
	queries := models.New(as.DB)

	due, err := queries.GetDueStagedParts(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get due staged parts: %w", err)
	}

	examined := len(due)
	released := 0

	for _, part := range due {
		err := pgx.BeginFunc(ctx, as.DB, func(tx pgx.Tx) error {
			txQueries := models.New(tx)

			// Guarded on released_at IS NULL. A concurrent worker that got here
			// first leaves no row for us, which is the signal to skip rather
			// than an error — releasing twice would notify twice.
			result, err := txQueries.ReleaseStagedPart(ctx, part.ID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil
				}
				return fmt.Errorf("failed to release staged part: %w", err)
			}

			released++
			as.notifyStagedRelease(ctx, result)
			return nil
		})
		if err != nil {
			// Log and continue: the next tick retries this part, and a
			// permanently broken one must not stall the others behind it.
			as.Logger.LogError(ctx, err, "Failed to release staged part",
				"result_id", part.ID,
				"game_id", part.GameID,
				"user_id", part.UserID,
			)
		}
	}

	if released > 0 {
		as.Logger.Info(ctx, "Released due staged parts",
			"examined", examined,
			"released", released,
		)
	}

	return examined, released, nil
}

// notifyStagedRelease tells the recipient that the next part is readable.
//
// The body deliberately carries no content and no part numbering. Content would
// defeat the reveal outright; "part 3 of 4" would leak how much is left, which
// is itself a spoiler about whether the story is over. Notification failure is
// logged but does not fail the release — the part is already visible in the UI,
// and un-releasing it to match a missing notification would be worse.
func (as *ActionSubmissionService) notifyStagedRelease(ctx context.Context, result models.ActionResult) {
	content := "The next part of your action result is now available"
	linkURL := fmt.Sprintf("/games/%d?tab=actions", result.GameID)
	relatedType := "action_result"

	_, err := as.NotificationService.CreateNotification(ctx, &core.CreateNotificationRequest{
		UserID:      result.UserID,
		GameID:      &result.GameID,
		Type:        core.NotificationTypeActionResult,
		Title:       "Action Result Continues",
		Content:     &content,
		RelatedType: &relatedType,
		RelatedID:   &result.ID,
		LinkURL:     &linkURL,
	})
	if err != nil {
		as.Logger.LogError(ctx, err, "Failed to create notification for released staged part",
			"result_id", result.ID,
			"user_id", result.UserID,
		)
	}
}

// CancelPendingPart deletes a not-yet-released part.
//
// ON DELETE CASCADE takes the rest of the chain behind it. That is intended:
// a part whose parent is gone has no clock to measure its delay against and
// would never release, so orphaning is strictly worse than cascading.
func (as *ActionSubmissionService) CancelPendingPart(ctx context.Context, resultID int32) error {
	err := pgx.BeginFunc(ctx, as.DB, func(tx pgx.Tx) error {
		queries := models.New(tx)

		result, err := queries.GetActionResult(ctx, resultID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("action result not found")
			}
			return fmt.Errorf("failed to get action result: %w", err)
		}

		// Already visible to the player: there is no taking it back.
		if result.ReleasedAt.Valid {
			return fmt.Errorf("%w: part has already been released", core.ErrCannotCancelPart)
		}

		// A head has no schedule to cancel — deleting it is an ordinary result
		// deletion, and routing it here would silently destroy the whole chain
		// under a name that does not suggest it.
		if !result.ParentResultID.Valid {
			return fmt.Errorf("%w: result %d is not a staged part; use DeleteActionResult", core.ErrCannotCancelPart, resultID)
		}

		if err := queries.DeletePublishedDrafts(ctx, resultID); err != nil {
			return fmt.Errorf("failed to delete draft character updates: %w", err)
		}

		if err := queries.DeleteStagedPart(ctx, resultID); err != nil {
			return fmt.Errorf("failed to delete staged part: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	as.Logger.Info(ctx, "Pending staged part cancelled", "result_id", resultID)
	return nil
}

// GetResultChain returns every part of the chain containing resultID, head
// first, with 1-based part numbers. Accepts any member, not just the head.
//
// This is an unfiltered read: it returns pending parts and their content. It
// serves the GM and audience views, both of which see everything by design.
// Do not hand its rows to a recipient player without filtering on ReleasedAt.
func (as *ActionSubmissionService) GetResultChain(ctx context.Context, resultID int32) ([]models.GetResultChainRow, error) {
	queries := models.New(as.DB)

	chain, err := queries.GetResultChain(ctx, resultID)
	if err != nil {
		return nil, fmt.Errorf("failed to get result chain: %w", err)
	}

	return chain, nil
}

// validateStagedChainRequest enforces the invariants a CHECK constraint cannot.
//
// The delay bounds duplicate action_results_delay_bounds deliberately: the
// database is the real guarantee, this is what produces a readable error before
// the database produces a blunt one.
func validateStagedChainRequest(req core.CreateStagedResultChainRequest) error {
	if len(req.Parts) < 2 {
		return fmt.Errorf("%w: needs at least 2 parts, got %d; use CreateActionResult for a single result", core.ErrInvalidStagedChain, len(req.Parts))
	}

	if len(req.Parts) > core.MaxStagedChainLength {
		return fmt.Errorf("%w: may have at most %d parts, got %d", core.ErrInvalidStagedChain, core.MaxStagedChainLength, len(req.Parts))
	}

	for i, part := range req.Parts {
		content := fmt.Sprintf("%v", part.Content)
		if err := validation.ValidateActionResult(content); err != nil {
			return fmt.Errorf("%w: part %d: %w", core.ErrInvalidStagedChain, i+1, err)
		}

		// The head has nothing to wait for, so its delay is meaningless rather
		// than merely unused — reject a nonzero one instead of silently
		// dropping it, since a GM who set it expected it to do something.
		if i == 0 {
			if part.DelayMinutes != 0 {
				return fmt.Errorf("%w: part 1 is the chain head and cannot have a delay, got %d minutes", core.ErrInvalidStagedChain, part.DelayMinutes)
			}
			continue
		}

		if part.DelayMinutes < core.MinStagedDelayMinutes || part.DelayMinutes > core.MaxStagedDelayMinutes {
			return fmt.Errorf("%w: part %d: delay must be between %d and %d minutes, got %d",
				core.ErrInvalidStagedChain, i+1, core.MinStagedDelayMinutes, core.MaxStagedDelayMinutes, part.DelayMinutes)
		}
	}

	return nil
}
