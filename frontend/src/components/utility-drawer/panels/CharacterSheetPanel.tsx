import { useEffect, useMemo, useRef } from 'react';
import { ChevronRight } from 'lucide-react';
import { Spinner, Alert, Badge } from '../../ui';
import { useGlobalCharacters, groupCharactersByGame } from '../../../hooks/useGlobalCharacters';
import { isEditableGameState } from '../../../hooks/useCharacterSheetPermissions';
import type { Character } from '../../../types/characters';
import type { GameUtilityContext, UtilityContext, UtilityPanelProps } from '../types';

/**
 * Launches the standard character-sheet modal from the Utility Drawer, exactly
 * as clicking "View/Edit Sheet" on the Characters tab does.
 *
 * Two modes, depending on whether the drawer was opened inside a game:
 *
 * - In a game: operates on the characters the user controls *there*. Exactly
 *   one means there's nothing to choose, so it opens immediately; more than one
 *   shows a picker. The GM instead gets every character in the game, as a
 *   reference — they can already open any sheet from the Characters tab, and
 *   running a scene means looking up whoever is in it.
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
    <InGameCharacterSheetPanel
      game={ctx.game}
      openCharacterSheet={ctx.openCharacterSheet}
    />
  ) : (
    <GlobalCharacterSheetPanel openCharacterSheet={ctx.openCharacterSheet} />
  );
}

interface InGamePanelProps {
  game: GameUtilityContext;
  openCharacterSheet: UtilityContext['openCharacterSheet'];
}

/**
 * Who is playing a character, for the GM's cast list. NPCs report their
 * assignee — an NPC nobody has been given is the GM's own to play, which is
 * worth saying rather than leaving blank. Player characters report their owner.
 *
 * Returns null when there's no name to show: the backend withholds usernames in
 * anonymous games from roles that shouldn't see them, and while a GM is not one
 * of those, this must degrade to just the character name rather than render an
 * empty line if that ever changes.
 */
function controllerOf(character: Character): string | null {
  if (character.character_type === 'npc') {
    return character.assigned_username ?? 'Unassigned';
  }
  return character.username ?? null;
}

/** In-game mode: characters and permissions from the contributed game context. */
function InGameCharacterSheetPanel({
  game,
  openCharacterSheet,
}: InGamePanelProps) {
  const {
    userCharacters,
    allGameCharacters,
    userRole,
    gameState,
    isAnonymous,
    isGM,
  } = game;

  // The GM runs the game, so every sheet in it is theirs to reference — they can
  // already open any of them from the Characters tab. Everyone else is limited
  // to the characters they control.
  const characters = isGM ? allGameCharacters : userCharacters;

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

  // With a single character there is nothing to choose — open it directly. Not
  // for the GM: their list is a reference of everyone in the game, so a game
  // that happens to hold one character shouldn't skip the list and fling that
  // sheet open. Only "the one character you control" is unambiguous enough.
  const soleCharacterId =
    !isGM && characters.length === 1 ? characters[0].id : null;

  // The GM's list is the whole cast and arrives in creation order, which is no
  // help when scanning for one name; sort it. A player's handful of characters
  // is left in the order the game hands them over. Copied first — the context's
  // array is shared with other consumers and must not be sorted in place.
  const listed = useMemo(
    () =>
      isGM
        ? [...characters].sort((a, b) => a.name.localeCompare(b.name))
        : characters,
    [isGM, characters],
  );

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

  if (characters.length === 0) {
    return (
      <p className="text-sm text-content-secondary text-center py-6 px-4">
        {isGM
          ? 'This game has no characters yet.'
          : "You don't control a character in this game."}
      </p>
    );
  }

  if (soleCharacterId !== null) {
    // The effect above opens the sheet; render nothing meaningful meanwhile.
    return null;
  }

  return (
    <ul className="p-2" data-testid="character-sheet-picker">
      {listed.map((c) => {
        const controller = isGM ? controllerOf(c) : null;
        return (
          <li key={c.id}>
            <button
              type="button"
              onClick={() => openCharacterSheet(c.id, sheetOptions)}
              className="w-full flex items-center gap-3 px-3 py-3 rounded-md hover:surface-raised transition-colors text-left group"
              data-testid={`character-sheet-open-${c.id}`}
              data-faro-user-action-name="open-character-sheet-from-drawer"
            >
              <span className="flex-1 min-w-0">
                <span className="block text-sm font-medium text-content-primary truncate group-hover:text-interactive-primary">
                  {c.name}
                </span>
                {/* Who's behind the character, so a GM scanning the cast has more
                  than a list of names to go on. */}
                {controller && (
                  <span
                    className="block text-xs text-content-secondary truncate"
                    data-testid={`character-sheet-owner-${c.id}`}
                  >
                    {controller}
                  </span>
                )}
              </span>
              {/* The GM's list spans the whole game, so mark which entries are
                NPCs. Players only ever see characters they control. */}
              {isGM && c.character_type === 'npc' && (
                <Badge
                  variant="secondary"
                  size="sm"
                  data-testid={`character-sheet-npc-${c.id}`}
                >
                  NPC
                </Badge>
              )}
              <ChevronRight className="w-4 h-4 shrink-0 text-content-tertiary" />
            </button>
          </li>
        );
      })}
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
