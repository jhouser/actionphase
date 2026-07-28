import { useEffect, useRef } from 'react';
import { ChevronRight } from 'lucide-react';
import { Spinner, Alert } from '../../ui';
import { useGlobalCharacters, groupCharactersByGame } from '../../../hooks/useGlobalCharacters';
import { isEditableGameState } from '../../../hooks/useCharacterSheetPermissions';
import type { GameUtilityContext, UtilityContext, UtilityPanelProps } from '../types';

/**
 * Launches the standard character-sheet modal from the Utility Drawer, exactly
 * as clicking "View/Edit Sheet" on the Characters tab does.
 *
 * Two modes, depending on whether the drawer was opened inside a game:
 *
 * - In a game: operates on the characters the user controls *there*. Exactly
 *   one means there's nothing to choose, so it opens immediately; more than one
 *   shows a picker.
 * - Outside a game: loads the user's characters across all their in_progress
 *   games and shows them grouped by game. Always a picker, even for a single
 *   character — auto-opening a modal from a global button the user pressed to
 *   "see what's here" would be a surprise, and the game grouping is the
 *   information they came for.
 *
 * Opening a sheet closes the drawer (handled by ctx.openCharacterSheet) so the
 * modal stacks cleanly over the page.
 */
export function CharacterSheetPanel({ ctx }: UtilityPanelProps) {
  // Passing `game` explicitly rather than letting the child re-read ctx.game
  // narrows it to non-null for the in-game branch, no assertion needed.
  return ctx.game ? (
    <InGameCharacterSheetPanel game={ctx.game} openCharacterSheet={ctx.openCharacterSheet} />
  ) : (
    <GlobalCharacterSheetPanel openCharacterSheet={ctx.openCharacterSheet} />
  );
}

interface InGamePanelProps {
  game: GameUtilityContext;
  openCharacterSheet: UtilityContext['openCharacterSheet'];
}

/** In-game mode: characters and permissions from the contributed game context. */
function InGameCharacterSheetPanel({ game, openCharacterSheet }: InGamePanelProps) {
  const { userCharacters, userRole, gameState, isAnonymous } = game;

  // Permissions are derived from the contributed context rather than
  // useCharacterSheetPermissions: that hook reads GameContext, and the drawer is
  // mounted at the app root, outside any GameProvider. Every character here is
  // one the user controls, so the rules reduce to state + role.
  const editable = isEditableGameState(gameState);
  const sheetOptions = {
    canEdit: editable,
    canEditStats: editable && userRole === 'gm',
    isAnonymous,
    userRole,
    gameState,
  };

  // With a single character there is nothing to choose — open it directly.
  const soleCharacterId = userCharacters.length === 1 ? userCharacters[0].id : null;

  // Opening the sheet updates state in the provider above us, which re-renders
  // this panel. Without a guard that re-entry re-fires the effect and loops, so
  // track which character we've already opened and only act on a change.
  const openedForRef = useRef<number | null>(null);

  useEffect(() => {
    if (soleCharacterId === null) return;
    if (openedForRef.current === soleCharacterId) return;
    openedForRef.current = soleCharacterId;
    openCharacterSheet(soleCharacterId, sheetOptions);
    // Only fire on mount / when the sole character changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [soleCharacterId]);

  if (userCharacters.length === 0) {
    return (
      <p className="text-sm text-content-secondary text-center py-6 px-4">
        You don't control a character in this game.
      </p>
    );
  }

  if (soleCharacterId !== null) {
    // The effect above opens the sheet; render nothing meaningful meanwhile.
    return null;
  }

  return (
    <ul className="p-2" data-testid="character-sheet-picker">
      {userCharacters.map((c) => (
        <li key={c.id}>
          <button
            type="button"
            onClick={() => openCharacterSheet(c.id, sheetOptions)}
            className="w-full flex items-center gap-3 px-3 py-3 rounded-md hover:surface-raised transition-colors text-left group"
            data-testid={`character-sheet-open-${c.id}`}
            data-faro-user-action-name="open-character-sheet-from-drawer"
          >
            <span className="flex-1 min-w-0 text-sm font-medium text-content-primary truncate group-hover:text-interactive-primary">
              {c.name}
            </span>
            <ChevronRight className="w-4 h-4 shrink-0 text-content-tertiary" />
          </button>
        </li>
      ))}
    </ul>
  );
}

/**
 * Global mode: characters across all the user's in_progress games, grouped by
 * game. Permissions come from the role the backend reports per character, since
 * there is no GameContext out here to derive them from.
 */
function GlobalCharacterSheetPanel({
  openCharacterSheet,
}: {
  openCharacterSheet: UtilityContext['openCharacterSheet'];
}) {
  const { data: characters, isLoading, isError } = useGlobalCharacters();

  if (isLoading) {
    return (
      <div className="flex justify-center py-8" data-testid="global-characters-loading">
        <Spinner size="md" />
      </div>
    );
  }

  if (isError) {
    return (
      <div className="p-4">
        <Alert variant="danger" data-testid="global-characters-error">
          Couldn't load your characters. Please try again.
        </Alert>
      </div>
    );
  }

  const groups = groupCharactersByGame(characters ?? []);

  if (groups.length === 0) {
    return (
      <p className="text-sm text-content-secondary text-center py-6 px-4">
        You don't control a character in any active game.
      </p>
    );
  }

  return (
    <div className="p-2" data-testid="global-character-sheet-picker">
      {groups.map((group) => (
        <div key={group.gameId} className="mb-3 last:mb-0">
          <h3 className="px-3 py-1 text-xs font-semibold uppercase tracking-wide text-content-tertiary truncate">
            {group.gameTitle}
          </h3>
          <ul>
            {group.characters.map((c) => (
              <li key={c.id}>
                <button
                  type="button"
                  onClick={() =>
                    openCharacterSheet(c.id, {
                      // Same rules as the in-game branch: every character here
                      // is one the user controls, so editing turns on the game
                      // state, and stat editing additionally on being GM.
                      canEdit: isEditableGameState(c.game_state),
                      canEditStats: isEditableGameState(c.game_state) && c.user_role === 'gm',
                      isAnonymous: c.game_is_anonymous,
                      userRole: c.user_role,
                      gameState: c.game_state,
                    })
                  }
                  className="w-full flex items-center gap-3 px-3 py-3 rounded-md hover:surface-raised transition-colors text-left group"
                  data-testid={`character-sheet-open-${c.id}`}
                  data-faro-user-action-name="open-character-sheet-from-drawer"
                >
                  <span className="flex-1 min-w-0 text-sm font-medium text-content-primary truncate group-hover:text-interactive-primary">
                    {c.name}
                  </span>
                  <ChevronRight className="w-4 h-4 shrink-0 text-content-tertiary" />
                </button>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </div>
  );
}
