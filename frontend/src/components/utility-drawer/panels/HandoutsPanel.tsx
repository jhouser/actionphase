import { useMemo, useState } from 'react';
import { ChevronDown, ChevronRight } from 'lucide-react';
import { Spinner, Alert } from '../../ui';
import { useHandouts } from '../../../hooks/useHandouts';
import { useGlobalHandouts, groupHandoutsByGame } from '../../../hooks/useGlobalHandouts';
import type { Handout } from '../../../types/handouts';
import type { GameUtilityContext, UtilityContext, UtilityPanelProps } from '../types';

/**
 * Opens a handout in a read-only modal from the Utility Drawer.
 *
 * The drawer is reference material: you pull a handout up to check a prompt or
 * some mechanics without leaving the action submission you were writing. So the
 * modal only reads — no editing the handout, and no posting or managing updates
 * on it, for the GM either. Those live on the game's Handouts tab.
 *
 * Two modes, depending on whether the drawer was opened inside a game:
 *
 * - In a game: the handouts of that game alone. The drawer is scoped to what's
 *   on screen, so a game with no handouts says so rather than falling back to
 *   the user's other games.
 * - Outside a game: published handouts across every in_progress game the user
 *   is in, grouped by game under collapsible headers.
 *
 * Published only, in both modes and for every role including the GM. The drawer
 * is a reading surface; a GM's drafts are work in progress and stay on the
 * Handouts tab, which is where they can be edited. Unlike the character-sheet
 * panel this never auto-opens a lone entry: handouts are reference material you
 * return to, and a game holding one handout today holds two as soon as the GM
 * publishes another, so the list stays put rather than changing shape underneath
 * a user who has learned where things are.
 */
export function HandoutsPanel({ ctx }: UtilityPanelProps) {
  // Passing `game` explicitly rather than letting the child re-read ctx.game
  // narrows it to non-null for the in-game branch, no assertion needed.
  return ctx.game ? (
    <InGameHandoutsPanel game={ctx.game} openHandout={ctx.openHandout} />
  ) : (
    <GlobalHandoutsPanel openHandout={ctx.openHandout} />
  );
}

/** One row in either list. */
function HandoutRow({
  handout,
  onOpen,
  indented = false,
}: {
  handout: Handout;
  onOpen: () => void;
  indented?: boolean;
}) {
  return (
    <li>
      <button
        type="button"
        onClick={onOpen}
        className={`w-full flex items-center gap-3 py-3 rounded-md hover:surface-raised transition-colors text-left group ${
          indented ? 'pl-6 pr-3' : 'px-3'
        }`}
        data-testid={`handout-open-${handout.id}`}
        data-faro-user-action-name="open-handout-from-drawer"
      >
        <span className="flex-1 min-w-0">
          <span className="block text-sm font-medium text-content-primary truncate group-hover:text-interactive-primary">
            {handout.title}
          </span>
        </span>
        <ChevronRight className="w-4 h-4 shrink-0 text-content-tertiary" />
      </button>
    </li>
  );
}

/** In-game mode: the handouts of the game currently on screen. */
function InGameHandoutsPanel({
  game,
  openHandout,
}: {
  game: GameUtilityContext;
  openHandout: UtilityContext['openHandout'];
}) {
  const { handouts, isLoading } = useHandouts(game.gameId);

  // The per-game endpoint hands a GM their drafts too, since it backs the
  // Handouts tab where drafts are edited. The drawer only reads, so filter them
  // out here and keep both modes showing the same thing.
  const published = useMemo(
    () => handouts.filter((h) => h.status === 'published').sort((a, b) => a.title.localeCompare(b.title)),
    [handouts]
  );

  if (isLoading) {
    return (
      <div className="flex justify-center py-8" data-testid="handouts-loading">
        <Spinner size="md" />
      </div>
    );
  }

  if (published.length === 0) {
    return (
      <p className="text-sm text-content-secondary text-center py-6 px-4">
        This game has no published handouts yet.
      </p>
    );
  }

  return (
    <ul className="p-2" data-testid="handout-picker">
      {published.map((handout) => (
        <HandoutRow
          key={handout.id}
          handout={handout}
          onOpen={() => openHandout(handout)}
        />
      ))}
    </ul>
  );
}

/**
 * Global mode: published handouts across all the user's in_progress games,
 * grouped by game under collapsible headers. Groups start expanded — someone in
 * two games should see everything at once — and collapse for users in enough
 * games that the full list is a scroll.
 */
function GlobalHandoutsPanel({
  openHandout,
}: {
  openHandout: UtilityContext['openHandout'];
}) {
  const { data: handouts, isLoading, isError } = useGlobalHandouts();
  // Holds only the games the user has explicitly collapsed, so groups default to
  // expanded without having to seed this from the data once it arrives.
  const [collapsed, setCollapsed] = useState<Set<number>>(new Set());

  const groups = useMemo(() => groupHandoutsByGame(handouts ?? []), [handouts]);

  const toggle = (gameId: number) => {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(gameId)) {
        next.delete(gameId);
      } else {
        next.add(gameId);
      }
      return next;
    });
  };

  if (isLoading) {
    return (
      <div className="flex justify-center py-8" data-testid="global-handouts-loading">
        <Spinner size="md" />
      </div>
    );
  }

  if (isError) {
    return (
      <div className="p-4">
        <Alert variant="danger" data-testid="global-handouts-error">
          Couldn't load your handouts. Please try again.
        </Alert>
      </div>
    );
  }

  if (groups.length === 0) {
    return (
      <p className="text-sm text-content-secondary text-center py-6 px-4">
        None of your active games have published handouts.
      </p>
    );
  }

  return (
    <div className="p-2" data-testid="global-handout-picker">
      {groups.map((group) => {
        const isCollapsed = collapsed.has(group.gameId);
        const regionId = `handout-group-${group.gameId}`;
        return (
          <div key={group.gameId} className="mb-3 last:mb-0">
            <button
              type="button"
              onClick={() => toggle(group.gameId)}
              aria-expanded={!isCollapsed}
              aria-controls={regionId}
              className="w-full flex items-center gap-1 px-3 py-1 rounded-md hover:surface-raised transition-colors text-left"
              data-testid={`handout-group-toggle-${group.gameId}`}
              data-faro-user-action-name="toggle-handout-game-group"
            >
              {isCollapsed ? (
                <ChevronRight className="w-3 h-3 shrink-0 text-content-tertiary" />
              ) : (
                <ChevronDown className="w-3 h-3 shrink-0 text-content-tertiary" />
              )}
              <span className="flex-1 min-w-0 text-xs font-semibold uppercase tracking-wide text-content-tertiary truncate">
                {group.gameTitle}
              </span>
            </button>
            {!isCollapsed && (
              <ul id={regionId}>
                {group.handouts.map((handout) => (
                  <HandoutRow
                    key={handout.id}
                    handout={handout}
                    indented
                    onOpen={() => openHandout(handout)}
                  />
                ))}
              </ul>
            )}
          </div>
        );
      })}
    </div>
  );
}
