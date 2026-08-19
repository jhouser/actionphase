import { useEffect, useMemo, useRef } from 'react';
import { ChevronRight } from 'lucide-react';
import { Spinner, Alert, Badge } from '../../ui';
import { useGlobalCharacters, groupCharactersByGame } from '../../../hooks/useGlobalCharacters';
import { isEditableGameState } from '../../../hooks/useCharacterSheetPermissions';
import { useCastFilter, type CastFilter } from '../../../hooks/useCastFilter';
import { useAuth } from '../../../contexts/AuthContext';
import type {
  Character,
  ControllableCharacterWithGame,
} from '../../../types/characters';
import type {
  GameUtilityContext,
  OpenCharacterSheetOptions,
  UtilityContext,
  UtilityPanelProps,
} from '../types';

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
 *   games and shows them grouped by game — including, for a GM, the full cast of
 *   games they run, since being off a game page shouldn't narrow what they can
 *   look up. Exactly one character opens immediately, same as in-game — the
 *   overwhelmingly common case is a player or audience member with a single
 *   character, for whom the picker is a list of one standing between them and
 *   the only thing it can lead to. The game grouping only earns its place once
 *   there's actually a choice to make.
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
function controllerOf(
  character: Pick<Character, 'character_type' | 'assigned_username' | 'username'>
): string | null {
  if (character.character_type === 'npc') {
    return character.assigned_username ?? 'Unassigned';
  }
  return character.username ?? null;
}

/**
 * Whether a character belongs in a GM's "Mine" slice: the ones they play rather
 * than merely oversee. That means their own player characters plus every NPC in
 * the game — a GM can step in for any NPC, assigned or not — and excludes other
 * players' characters, which are the bulk of what makes a full cast list long
 * enough to want filtering in the first place.
 *
 * Used by the cross-game list, where characters carry a `user_id` but there is
 * no game context saying which ones this user controls. The in-game list gets
 * that answer directly and does not need this.
 */
function isPlayedByGM(
  character: ControllableCharacterWithGame,
  userId: number | undefined
): boolean {
  if (character.character_type === 'npc') return true;
  return userId !== undefined && character.user_id === userId;
}

/**
 * The All / Mine switch over a GM's cast list. Offered only to GMs and co-GMs:
 * everyone else's list is already just what they control, so there would be
 * nothing for the filter to remove.
 */
function CastFilterToggle({
  filter,
  onChange,
}: {
  filter: CastFilter;
  onChange: (filter: CastFilter) => void;
}) {
  const options: { value: CastFilter; label: string }[] = [
    { value: 'all', label: 'All' },
    { value: 'mine', label: 'Mine' },
  ];

  return (
    <div
      className="flex gap-1 p-1 mx-2 mt-2 rounded-md bg-surface-sunken"
      role="group"
      aria-label="Filter characters"
      data-testid="cast-filter"
    >
      {options.map((option) => {
        const selected = filter === option.value;
        return (
          <button
            key={option.value}
            type="button"
            aria-pressed={selected}
            onClick={() => onChange(option.value)}
            // The selected pill uses `surface-overlay`, not `surface-raised`:
            // in the dark themes `surface-raised` resolves to the same value as
            // the `surface-sunken` track behind it, which made the selected
            // state invisible.
            className={
              selected
                ? 'flex-1 px-3 py-1.5 text-xs font-medium rounded bg-surface-overlay text-content-primary shadow-sm transition-colors'
                : 'flex-1 px-3 py-1.5 text-xs font-medium rounded text-content-secondary hover:text-content-primary transition-colors'
            }
            data-testid={`cast-filter-${option.value}`}
            data-faro-user-action-name="filter-character-sheet-cast"
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
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
    portraitAvatars,
    sheetConfig,
    isGM,
  } = game;

  const [castFilter, setCastFilter] = useCastFilter();

  // The GM runs the game, so every sheet in it is theirs to reference — they can
  // already open any of them from the Characters tab. Everyone else is limited
  // to the characters they control, and gets no filter: their list is already
  // "mine".
  //
  // In-game, "mine" needs no user id: the game context already reports which
  // characters this user controls, so it's those plus every NPC (a GM can step
  // in for any of them). Deduped by id — a GM-owned NPC appears in both halves.
  const characters = useMemo(() => {
    if (!isGM) return userCharacters;
    if (castFilter === 'all') return allGameCharacters;

    const ownIds = new Set(userCharacters.map((c) => c.id));
    return allGameCharacters.filter(
      (c) => c.character_type === 'npc' || ownIds.has(c.id)
    );
  }, [isGM, castFilter, userCharacters, allGameCharacters]);

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
    portraitAvatars,
    sheetConfig,
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

  // The filter stays on screen alongside every outcome below, including the
  // empty one — a GM who filtered their way to nothing needs the control that
  // got them there in order to get back.
  const filterToggle = isGM ? (
    <CastFilterToggle filter={castFilter} onChange={setCastFilter} />
  ) : null;

  if (characters.length === 0) {
    return (
      <>
        {filterToggle}
        <p className="text-sm text-content-secondary text-center py-6 px-4">
          {!isGM
            ? "You don't control a character in this game."
            : castFilter === 'mine'
              ? 'You don\'t play a character in this game. Switch to "All" to see the full cast.'
              : 'This game has no characters yet.'}
        </p>
      </>
    );
  }

  if (soleCharacterId !== null) {
    // The effect above opens the sheet; render nothing meaningful meanwhile.
    return null;
  }

  return (
    <>
    {filterToggle}
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
    </>
  );
}

/**
 * Sheet permissions for a character reached from outside a game. The rules
 * reduce to the game's state plus the role the backend reports for it: editing
 * turns on the game still being open, stat editing additionally on being its
 * GM — which also covers the cast entries a GM receives for characters they
 * oversee but don't play.
 *
 * Presentation settings travel the same way and for the same reason: the sheet
 * normally reads the avatar shape from GameContext, which does not exist out
 * here, so it comes from the character's own game rather than defaulting.
 */
function sheetOptionsFor(
  character: ControllableCharacterWithGame,
): OpenCharacterSheetOptions {
  const editable = isEditableGameState(character.game_state);
  return {
    canEdit: editable,
    canEditStats: editable && character.user_role === 'gm',
    isAnonymous: character.game_is_anonymous,
    userRole: character.user_role,
    gameState: character.game_state,
    portraitAvatars: character.game_portrait_avatars,
    sheetConfig: character.game_character_sheet,
  };
}

/**
 * Global mode: characters across all the user's in_progress games, grouped by
 * game. Permissions come from the role the backend reports per character, since
 * there is no GameContext out here to derive them from.
 *
 * A GM's entry for a game they run is its whole cast, matching what the in-game
 * drawer gives them — being off a game page is no reason to narrow what they can
 * look up. Those rows get the same owner line and NPC badge as in-game, and the
 * All / Mine filter to cut the cast down to what they personally play.
 */
function GlobalCharacterSheetPanel({
  openCharacterSheet,
}: {
  openCharacterSheet: UtilityContext['openCharacterSheet'];
}) {
  const { data: characters, isLoading, isError } = useGlobalCharacters();
  const { currentUser } = useAuth();
  const [castFilter, setCastFilter] = useCastFilter();

  // Whether any returned character is one this user only oversees. That's what
  // makes the list a cast reference rather than "characters you control", and it
  // decides both whether to offer the filter and whether auto-open is safe.
  const hasCastEntries = useMemo(
    () =>
      (characters ?? []).some(
        (c) =>
          (c.user_role === 'gm' || c.user_role === 'co_gm') &&
          !isPlayedByGM(c, currentUser?.id)
      ),
    [characters, currentUser?.id]
  );

  const visible = useMemo(() => {
    if (!characters) return [];
    if (castFilter === 'all') return characters;
    // Only the GM's oversight rows are filtered away; a character the user plays
    // in someone else's game is theirs and stays under either setting.
    return characters.filter(
      (c) =>
        !(c.user_role === 'gm' || c.user_role === 'co_gm') ||
        isPlayedByGM(c, currentUser?.id)
    );
  }, [characters, castFilter, currentUser?.id]);

  // Nothing to choose between, so skip the picker — but only when the list is
  // purely "characters you control". A GM's list is a cast reference, so a game
  // holding one character must still show the list rather than fling that sheet
  // open, exactly as in-game.
  const soleCharacter =
    !hasCastEntries && visible.length === 1 ? visible[0] : null;
  const soleCharacterId = soleCharacter?.id ?? null;

  // Opening the sheet updates state in the provider above us, which re-renders
  // this panel. Without a guard that re-entry re-fires the effect and loops, so
  // track which character we've already opened and only act on a change.
  const openedForRef = useRef<number | null>(null);

  useEffect(() => {
    if (!soleCharacter || soleCharacterId === null) return;
    if (openedForRef.current === soleCharacterId) return;
    openedForRef.current = soleCharacterId;
    openCharacterSheet(soleCharacterId, sheetOptionsFor(soleCharacter));
    // Only fire on mount / when the sole character changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [soleCharacterId]);

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

  if (soleCharacterId !== null) {
    // The effect above opens the sheet; render nothing meaningful meanwhile.
    return null;
  }

  // Offered whenever the list holds oversight rows — under "mine" those are
  // filtered out, so the presence of the filter has to be judged on the full
  // set or the control would remove itself on first use.
  const filterToggle = hasCastEntries ? (
    <CastFilterToggle filter={castFilter} onChange={setCastFilter} />
  ) : null;

  const groups = groupCharactersByGame(visible);

  if (groups.length === 0) {
    return (
      <>
        {filterToggle}
        <p className="text-sm text-content-secondary text-center py-6 px-4">
          {castFilter === 'mine' && hasCastEntries
            ? 'You don\'t play a character in any active game. Switch to "All" to see the casts you run.'
            : "You don't control a character in any active game."}
        </p>
      </>
    );
  }

  return (
    <>
      {filterToggle}
      <div className="p-2" data-testid="global-character-sheet-picker">
        {groups.map((group) => (
          <div key={group.gameId} className="mb-3 last:mb-0">
            <h3 className="px-3 py-1 text-xs font-semibold uppercase tracking-wide text-content-tertiary truncate">
              {group.gameTitle}
            </h3>
            <ul>
              {group.characters.map((c) => {
                // The owner line is GM context, and only within the game they
                // run: a character in someone else's game is the user's own, so
                // crediting them to themselves is noise — and in an anonymous
                // game it must not become a way to see who else is playing.
                const isOverseen = c.user_role === 'gm' || c.user_role === 'co_gm';
                const controller = isOverseen ? controllerOf(c) : null;
                return (
                  <li key={c.id}>
                    <button
                      type="button"
                      onClick={() => openCharacterSheet(c.id, sheetOptionsFor(c))}
                      className="w-full flex items-center gap-3 px-3 py-3 rounded-md hover:surface-raised transition-colors text-left group"
                      data-testid={`character-sheet-open-${c.id}`}
                      data-faro-user-action-name="open-character-sheet-from-drawer"
                    >
                      <span className="flex-1 min-w-0">
                        <span className="block text-sm font-medium text-content-primary truncate group-hover:text-interactive-primary">
                          {c.name}
                        </span>
                        {controller && (
                          <span
                            className="block text-xs text-content-secondary truncate"
                            data-testid={`character-sheet-owner-${c.id}`}
                          >
                            {controller}
                          </span>
                        )}
                      </span>
                      {isOverseen && c.character_type === 'npc' && (
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
          </div>
        ))}
      </div>
    </>
  );
}
