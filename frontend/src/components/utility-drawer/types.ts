import type { ComponentType } from 'react';
import type { LucideIcon } from 'lucide-react';
import type { Character } from '../../types/characters';
import type { GamePhase } from '../../types/phases';
import type { UserGameRole } from '../../contexts/GameContext';
import type { CommentReadMode } from '../../lib/api/auth';

/**
 * The parts of the Utility Drawer's context that only exist inside a game.
 * Assembled by CommonRoom from GameContext + its own props and contributed to
 * the global drawer, so panels stay decoupled from how that data is sourced.
 */
export interface GameUtilityContext {
  gameId: number;
  currentPhase?: GamePhase | null;
  isGM: boolean;
  isAudience: boolean;
  isGameCompleted: boolean;
  /** User's role in the game — used to compute sheet view/edit permissions. */
  userRole: UserGameRole;
  /** Current game state, e.g. 'in_progress' | 'completed'. */
  gameState: string;
  /** Whether the game is in anonymous mode. */
  isAnonymous: boolean;
  /** Whether the game displays character avatars as portraits rather than circles. */
  portraitAvatars: boolean;
  /** Characters the current user controls in this game (may be empty for GM/audience). */
  userCharacters: Character[];
  /** All characters in the game (for reference/lookup within panels). */
  allGameCharacters: Character[];
  /** The current user's comment read tracking mode ('auto' or 'manual'). */
  commentReadMode: CommentReadMode;
}

/**
 * Everything a panel needs to open a character sheet. Inside a game these come
 * from GameContext; outside one they ride along with each character from the
 * cross-game endpoint, since there is no GameContext to read them from.
 */
export interface OpenCharacterSheetOptions {
  canEdit: boolean;
  canEditStats: boolean;
  isAnonymous: boolean;
  userRole: string;
  gameState: string;
  /**
   * Whether that game shows avatars as portraits. Required rather than optional
   * so a new call site can't silently fall through to circles — the sheet has no
   * GameContext to recover this from when opened outside a game.
   */
  portraitAvatars: boolean;
}

/**
 * Shared context passed to every Utility Drawer panel and to each utility's
 * `isAvailable` gate.
 *
 * The drawer is mounted globally, so `game` is absent on pages outside a game
 * route. Utilities that need a game must gate on it in `isAvailable` rather
 * than assume it — see the registry.
 */
export interface UtilityContext {
  /** Game-scoped context, or null when the drawer is open outside a game. */
  game: GameUtilityContext | null;
  /**
   * Open the standard character-sheet modal for a character, as if the user
   * clicked "View/Edit Sheet" on the Characters tab. Closes the drawer first,
   * so the modal stacks cleanly over the page.
   */
  openCharacterSheet: (characterId: number, options: OpenCharacterSheetOptions) => void;
  /** Close the Utility Drawer, e.g. after a panel action completes. */
  closeDrawer: () => void;
}

/** Props every utility panel receives when rendered inside the drawer. */
export interface UtilityPanelProps {
  ctx: UtilityContext;
}

/**
 * Describes a single utility available in the Utility Drawer.
 *
 * Adding a new utility is a matter of appending one descriptor to the registry
 * (registry.ts) and supplying a Panel component — no changes to the drawer
 * container or its hosts are required.
 */
export interface UtilityDrawerUtility {
  /** Stable identifier, e.g. 'character-sheet'. */
  id: string;
  /** Short label shown in the utility list and as the panel title. */
  label: string;
  /** One-line description shown under the label in the list view. */
  description: string;
  icon: LucideIcon;
  /**
   * Whether this utility should be offered in the current context.
   * e.g. mark-all-read needs a game; the character sheet needs a character.
   */
  isAvailable: (ctx: UtilityContext) => boolean;
  /** The panel body rendered inside the drawer when this utility is selected. */
  Panel: ComponentType<UtilityPanelProps>;
}
