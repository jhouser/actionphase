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
   * Contribute the current game's context to the drawer. Called by the surface
   * that owns a game (CommonRoom); pass null to withdraw it. Withdrawing is
   * automatic on unmount via useProvideGameUtilityContext.
   */
  setGameContext: (game: GameUtilityContext | null) => void;
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

/**
 * Hosts the Utility Drawer's state above the router, so the drawer and the
 * character-sheet modal it launches are reachable from every page rather than
 * only from the common room.
 *
 * Game-scoped data still comes from the game itself: CommonRoom contributes its
 * GameContext-derived slice via `setGameContext`, and the drawer falls back to
 * cross-game behaviour when no game is mounted.
 */
export function UtilityDrawerProvider({ children }: UtilityDrawerProviderProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [game, setGame] = useState<GameUtilityContext | null>(null);
  const [openSheet, setOpenSheet] = useState<{
    characterId: number;
    options: OpenCharacterSheetOptions;
  } | null>(null);

  const openDrawer = useCallback(() => setIsOpen(true), []);
  const closeDrawer = useCallback(() => setIsOpen(false), []);
  const closeSheet = useCallback(() => setOpenSheet(null), []);

  // Ignore republishes whose contents haven't actually changed. The publishing
  // component (CommonRoom) re-renders whenever this provider's state changes, so
  // storing every new object identity it hands us would loop: set state →
  // re-render → fresh object → set state. Only a real change gets through.
  const setGameContext = useCallback((next: GameUtilityContext | null) => {
    setGame((prev) => (isSameGameContext(prev, next) ? prev : next));
  }, []);

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
 * to cross-game behaviour when the user navigates away from the game.
 */
// eslint-disable-next-line react-refresh/only-export-components
export function useProvideGameUtilityContext(game: GameUtilityContext) {
  const { setGameContext } = useUtilityDrawer();

  useEffect(() => {
    setGameContext(game);
    return () => setGameContext(null);
  }, [game, setGameContext]);
}
