package main

import (
	"strings"
	"testing"
)

// TestCheckMigrationState pins the behaviour that replaced the old "auto-fix".
//
// That code called m.Force(version) on a dirty state and carried on. Force does
// not repair anything — it writes dirty=false, asserting the migration completed.
// When it had not, the next Up() started at version+1 and the failed migration was
// skipped permanently while schema_migrations claimed success. The whole point of
// this function is that a dirty state stops the process instead.
func TestCheckMigrationState(t *testing.T) {
	t.Run("clean state proceeds", func(t *testing.T) {
		if err := checkMigrationState(20260818224524, false); err != nil {
			t.Fatalf("a clean state must not block startup: %v", err)
		}
	})

	t.Run("dirty state is refused", func(t *testing.T) {
		err := checkMigrationState(20260818224524, true)
		if err == nil {
			t.Fatal("a dirty state must refuse to start; forcing it clean skips the failed migration permanently")
		}

		// The operator has to know which migration to inspect, and what to run
		// once they have. An error that only says "dirty" sends them to the source.
		msg := err.Error()
		if !strings.Contains(msg, "20260818224524") {
			t.Errorf("error must name the offending version, got: %s", msg)
		}
		if !strings.Contains(msg, "migrate force") {
			t.Errorf("error must name the recovery command, got: %s", msg)
		}
	})

	t.Run("reports version zero when nothing has been applied", func(t *testing.T) {
		// A dirty flag at version 0 is still dirty. Nothing about the message
		// should assume a prior successful migration exists.
		if err := checkMigrationState(0, true); err == nil {
			t.Error("dirty at version 0 must still be refused")
		}
	})
}
