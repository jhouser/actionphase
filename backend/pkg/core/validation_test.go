package core

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "actionphase/pkg/db/models"
)

func TestValidateGameNotCompleted(t *testing.T) {
	tests := []struct {
		name      string
		gameState string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "active game allows writes",
			gameState: GameStateInProgress,
			wantErr:   false,
		},
		{
			name:      "paused game allows writes",
			gameState: GameStatePaused,
			wantErr:   false,
		},
		{
			name:      "recruitment game allows writes",
			gameState: GameStateRecruitment,
			wantErr:   false,
		},
		{
			name:      "setup game allows writes",
			gameState: GameStateSetup,
			wantErr:   false,
		},
		{
			name:      "character_creation game allows writes",
			gameState: GameStateCharacterCreation,
			wantErr:   false,
		},
		{
			// The load-bearing case for the epilogue feature. Epilogue grants the
			// READ access of completed while leaving the write gate open — that
			// separation is the entire point of the state. If someone "fixes"
			// the apparent omission in ValidateGameNotCompleted by adding
			// epilogue to it, posting silently breaks and epilogue collapses
			// into a second name for completed. This test fails first.
			name:      "epilogue game allows writes despite being a public archive",
			gameState: GameStateEpilogue,
			wantErr:   false,
		},
		{
			name:      "completed game blocks writes",
			gameState: GameStateCompleted,
			wantErr:   true,
			errMsg:    "archived",
		},
		{
			name:      "cancelled game blocks writes",
			gameState: GameStateCancelled,
			wantErr:   true,
			errMsg:    "archived",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			game := &db.Game{
				ID:    1,
				State: pgtype.Text{String: tt.gameState, Valid: true},
			}

			err := ValidateGameNotCompleted(ctx, game)

			if tt.wantErr {
				require.Error(t, err, "Expected error for game state: %s", tt.gameState)
				assert.Contains(t, err.Error(), tt.errMsg,
					"Error message should contain '%s' for state: %s", tt.errMsg, tt.gameState)
				assert.Contains(t, err.Error(), tt.gameState,
					"Error message should contain game state: %s", tt.gameState)
			} else {
				require.NoError(t, err, "No error expected for game state: %s", tt.gameState)
			}
		})
	}
}

func TestValidateGameNotCompleted_ErrorMessage(t *testing.T) {
	// Test that error message includes game ID and state for debugging
	ctx := context.Background()
	game := &db.Game{
		ID:    42,
		State: pgtype.Text{String: GameStateCompleted, Valid: true},
	}

	err := ValidateGameNotCompleted(ctx, game)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "42", "Error should contain game ID")
	assert.Contains(t, err.Error(), GameStateCompleted, "Error should contain game state")
	assert.Contains(t, err.Error(), "archived", "Error should indicate game is archived")
	assert.Contains(t, err.Error(), "read-only", "Error should indicate read-only status")
}
