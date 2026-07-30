import { createContext, useContext, useState, useCallback, useEffect, useMemo } from 'react';
import type { ReactNode } from 'react';
import type {
  GameUtilityContext,
  OpenCharacterSheetOptions,
  UtilityContext,
} from '../components/utility-drawer/types';

interface UtilityDrawerContextValue {
  /** Whether the drawer is currently open. */
  isOpen: boolean;
  openDrawer: () => void;
  closeDrawer: () => void;
  /**
   * Contribute a game context to the drawer under a stable key, or withdraw it
   * by passing null. Contributions are stacked: the most recently published one
   * wins, and withdrawing it falls back to whatever is still published rather
   * than to no game at all — the game page publishes a page-wide context, and a
   * surface within it (CommonRoom) may publish a narrower, phase-scoped one on
   * top for as long as it is mounted.
   *
   * Prefer useProvideGameUtilityContext, which handles withdrawal on unmount.
   */
  setGameContext: (key: string, game: GameUtilityContext | null) => void;
  /** The assembled context handed to the drawer and its panels. */
  utilityContext: UtilityContext;
  /** The character sheet currently open in the modal, if any. */
  openSheet: { characterId: number; options: OpenCharacterSheetOptions } | null;
  closeSheet: () => void;
}

const UtilityDrawerContext = createContext<UtilityDrawerContextValue | undefined>(undefined);

/**
 * Whether two game contexts are equivalent for the drawer's purposes. Compares
 * the scalars the drawer's gates read, plus character lists by id and name —
 * the arrays arrive as fresh objects from React Query on every render, so
 * comparing by identity would report a change that isn't one. Name is included
 * because the pickers display it, so a rename must still propagate.
 */
function isSameGameContext(
  a: GameUtilityContext | null,
  b: GameUtilityContext | null
): boolean {
  if (a === b) return true;
  if (!a || !b) return false;

  const sameCharacters = (
    xs: { id: number; name: string }[],
    ys: { id: number; name: string }[]
  ) =>
    xs.length === ys.length &&
    xs.every((x, i) => x.id === ys[i].id && x.name === ys[i].name);

  return (
    a.gameId === b.gameId &&
    a.currentPhase?.id === b.currentPhase?.id &&
    a.isGM === b.isGM &&
    a.isAudience === b.isAudience &&
    a.isGameCompleted === b.isGameCompleted &&
    a.userRole === b.userRole &&
    a.gameState === b.gameState &&
    a.isAnonymous === b.isAnonymous &&
    a.commentReadMode === b.commentReadMode &&
    sameCharacters(a.userCharacters, b.userCharacters) &&
    sameCharacters(a.allGameCharacters, b.allGameCharacters)
  );
}

interface UtilityDrawerProviderProps {
  children: ReactNode;
}

/** One publisher's contribution, kept in publish order. */
interface GameContextEntry {
  key: string;
  game: GameUtilityContext;
}

/**
 * Hosts the Utility Drawer's state above the router, so the drawer and the
 * character-sheet modal it launches are reachable from every page rather than
 * only from the common room.
 *
 * Game-scoped data still comes from the game itself: the game page contributes
 * its GameContext-derived slice via `setGameContext`, surfaces within it may
 * contribute a narrower one on top, and the drawer falls back to cross-game
 * behaviour when no game is mounted at all.
 */
export function UtilityDrawerProvider({ children }: UtilityDrawerProviderProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [gameStack, setGameStack] = useState<GameContextEntry[]>([]);
  const [openSheet, setOpenSheet] = useState<{
    characterId: number;
    options: OpenCharacterSheetOptions;
  } | null>(null);

  const openDrawer = useCallback(() => setIsOpen(true), []);
  const closeDrawer = useCallback(() => setIsOpen(false), []);
  const closeSheet = useCallback(() => setOpenSheet(null), []);

  // Ignore republishes whose contents haven't actually changed. Publishing
  // components re-render whenever this provider's state changes, so storing
  // every new object identity they hand us would loop: set state → re-render →
  // fresh object → set state. Only a real change gets through.
  const setGameContext = useCallback(
    (key: string, next: GameUtilityContext | null) => {
      setGameStack((prev) => {
        const index = prev.findIndex((entry) => entry.key === key);

        if (!next) {
          return index === -1 ? prev : prev.filter((entry) => entry.key !== key);
        }

        if (index === -1) {
          return [...prev, { key, game: next }];
        }
        if (isSameGameContext(prev[index].game, next)) {
          return prev;
        }
        // Replace in place: a re-publish is an update, not a re-entry, so it
        // must not jump ahead of a narrower context published above it.
        const updated = [...prev];
        updated[index] = { key, game: next };
        return updated;
      });
    },
    []
  );

  // The most recently published context wins — the page publishes first and a
  // surface within it (e.g. a phase-scoped common room) publishes over the top.
  const game = gameStack.length > 0 ? gameStack[gameStack.length - 1].game : null;

  // Opening a sheet closes the drawer so the modal stacks cleanly over the page.
  const openCharacterSheet = useCallback(
    (characterId: number, options: OpenCharacterSheetOptions) => {
      setIsOpen(false);
      setOpenSheet({ characterId, options });
    },
    []
  );

  const utilityContext = useMemo<UtilityContext>(
    () => ({ game, openCharacterSheet, closeDrawer }),
    [game, openCharacterSheet, closeDrawer]
  );

  const value = useMemo<UtilityDrawerContextValue>(
    () => ({
      isOpen,
      openDrawer,
      closeDrawer,
      setGameContext,
      utilityContext,
      openSheet,
      closeSheet,
    }),
    [isOpen, openDrawer, closeDrawer, setGameContext, utilityContext, openSheet, closeSheet]
  );

  return <UtilityDrawerContext.Provider value={value}>{children}</UtilityDrawerContext.Provider>;
}

// eslint-disable-next-line react-refresh/only-export-components
export function useUtilityDrawer(): UtilityDrawerContextValue {
  const context = useContext(UtilityDrawerContext);
  if (!context) {
    throw new Error('useUtilityDrawer must be used within UtilityDrawerProvider');
  }
  return context;
}

/**
 * Like useUtilityDrawer, but returns null instead of throwing when no provider
 * is mounted. For surfaces that merely offer a way into the drawer — the global
 * nav — where the drawer being absent should hide the entry point, not take the
 * whole page down with it.
 */
// eslint-disable-next-line react-refresh/only-export-components
export function useOptionalUtilityDrawer(): UtilityDrawerContextValue | null {
  return useContext(UtilityDrawerContext) ?? null;
}

/**
 * Publish a game's context to the global Utility Drawer for as long as the
 * calling component is mounted, withdrawing it on unmount so the drawer reverts
 * to whatever remains published — a broader context from an ancestor, or
 * cross-game behaviour once the user leaves the game entirely.
 *
 * `key` identifies the publisher, so republishing updates that publisher's
 * entry rather than stacking a second one. Two publishers that could be mounted
 * at once must use different keys; the innermost one to publish wins.
 */
// eslint-disable-next-line react-refresh/only-export-components
export function useProvideGameUtilityContext(key: string, game: GameUtilityContext) {
  const { setGameContext } = useUtilityDrawer();

  useEffect(() => {
    setGameContext(key, game);
    return () => setGameContext(key, null);
  }, [key, game, setGameContext]);
}
