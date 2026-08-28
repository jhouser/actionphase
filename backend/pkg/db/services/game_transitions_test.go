package db

import (
	"testing"

	"actionphase/pkg/core"
)

// The game state machine had no direct test before epilogue was added, despite
// allowedTransitions being the only thing standing between a GM and an
// irreversible state change. These are pure-function tests — no database.

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		want    bool
		because string
	}{
		// Normal lifecycle
		{"setup to recruitment", core.GameStateSetup, core.GameStateRecruitment, true, "standard progression"},
		{"recruitment to character creation", core.GameStateRecruitment, core.GameStateCharacterCreation, true, "standard progression"},
		{"character creation to in progress", core.GameStateCharacterCreation, core.GameStateInProgress, true, "standard progression"},
		{"in progress to paused", core.GameStateInProgress, core.GameStatePaused, true, "GMs pause games"},
		{"paused to in progress", core.GameStatePaused, core.GameStateInProgress, true, "pausing is reversible"},
		{"in progress to completed", core.GameStateInProgress, core.GameStateCompleted, true, "skipping epilogue stays supported"},

		// Epilogue: the two doors out of in_progress
		{"in progress to epilogue", core.GameStateInProgress, core.GameStateEpilogue, true, "the new endgame option"},
		{"epilogue to completed", core.GameStateEpilogue, core.GameStateCompleted, true, "epilogue finishes by completing"},
		{"epilogue to cancelled", core.GameStateEpilogue, core.GameStateCancelled, true, "emergency exit stays available"},

		// The one-way door. This is the highest-value assertion in the file:
		// entering epilogue discloses every private message and action
		// submission, and players cannot un-see it. A transition back would let
		// a GM believe they had restored a secrecy that no longer exists.
		{"epilogue to in progress", core.GameStateEpilogue, core.GameStateInProgress, false, "disclosure cannot be undone"},
		{"epilogue to paused", core.GameStateEpilogue, core.GameStatePaused, false, "no route back into live play"},

		// completed stays terminal — several features depend on its immutability
		{"completed to epilogue", core.GameStateCompleted, core.GameStateEpilogue, false, "completed is terminal"},
		{"completed to in progress", core.GameStateCompleted, core.GameStateInProgress, false, "completed is terminal"},
		{"cancelled to in progress", core.GameStateCancelled, core.GameStateInProgress, false, "cancelled is terminal"},

		// Skipping stages
		{"setup to in progress", core.GameStateSetup, core.GameStateInProgress, false, "cannot skip recruitment"},
		{"recruitment to epilogue", core.GameStateRecruitment, core.GameStateEpilogue, false, "nothing to write an epilogue about"},

		{"unknown state", "not_a_state", core.GameStateCompleted, false, "unknown states have no transitions"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidTransition(tt.from, tt.to); got != tt.want {
				t.Errorf("isValidTransition(%q, %q) = %v, want %v (%s)",
					tt.from, tt.to, got, tt.want, tt.because)
			}
		})
	}
}

// Every state the API accepts must appear in allowedTransitions, or a game can
// reach a state it can never leave. scripts/check-game-states.sh compares the
// two textually; this catches it at test time as well.
func TestAllowedTransitionsCoversEveryValidState(t *testing.T) {
	for _, state := range core.ValidGameStates {
		if _, ok := allowedTransitions[state]; !ok {
			t.Errorf("state %q is in core.ValidGameStates but missing from allowedTransitions", state)
		}
	}

	for state := range allowedTransitions {
		found := false
		for _, valid := range core.ValidGameStates {
			if valid == state {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("state %q is in allowedTransitions but not in core.ValidGameStates", state)
		}
	}
}

// Every state a transition points AT must itself be a valid state, otherwise
// the UI offers a button whose API call the CHECK constraint will reject.
func TestAllowedTransitionTargetsAreValidStates(t *testing.T) {
	valid := make(map[string]bool, len(core.ValidGameStates))
	for _, s := range core.ValidGameStates {
		valid[s] = true
	}

	for from, targets := range allowedTransitions {
		for _, to := range targets {
			if !valid[to] {
				t.Errorf("transition %q → %q targets a state not in core.ValidGameStates", from, to)
			}
		}
	}
}
