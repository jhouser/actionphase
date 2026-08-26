import { UserRound, Dices, CheckCheck, FileText } from 'lucide-react';
import type { UtilityDrawerUtility } from './types';
import { CharacterSheetPanel } from './panels/CharacterSheetPanel';
import { DiceRollerPanel } from './panels/DiceRollerPanel';
import { MarkAllReadPanel } from './panels/MarkAllReadPanel';
import { HandoutsPanel } from './panels/HandoutsPanel';

/**
 * The set of utilities offered by the Utility Drawer.
 *
 * The drawer is mounted globally, so `isAvailable` runs both inside and outside
 * a game — anything needing game data must gate on `ctx.game` rather than
 * assume it.
 *
 * To add a utility: append a descriptor here and supply its Panel component.
 * No changes to UtilityDrawer or its hosts are needed.
 */
export const UTILITY_DRAWER_UTILITIES: UtilityDrawerUtility[] = [
  {
    id: 'character-sheet',
    label: 'Character Sheet',
    // Worded to fit both readings of the panel: a player sees their own
    // characters, a GM sees the game's whole cast. Descriptors are static
    // strings, so it can't say "your" without being wrong for one of them.
    description: 'View abilities, skills, and inventory.',
    icon: UserRound,
    // Inside a game, only useful when there's a sheet to open: a character the
    // user controls, or — for the GM, who can reference the whole cast — any
    // character in the game. A game with nothing to open hides the utility
    // rather than falling back to the user's other games: the drawer is scoped
    // to the game on screen, and offering an unrelated character from an active
    // game while you're reading a completed one's archive is worse than
    // offering nothing.
    // Outside a game the panel loads the user's characters across all their
    // games, so offer it unconditionally and let the panel report an empty
    // result — gating here would require the list before the drawer is opened.
    isAvailable: (ctx) =>
      !ctx.game ||
      (ctx.game.isGM ? ctx.game.allGameCharacters.length > 0 : ctx.game.userCharacters.length > 0),
    Panel: CharacterSheetPanel,
  },
  {
    id: 'handouts',
    label: 'Handouts',
    description: 'Read the reference material for your games.',
    icon: FileText,
    // Offered unconditionally, in a game or out of it. Whether a game has any
    // published handouts is only knowable from the list itself, which the panel
    // fetches when opened — gating here would mean loading every game's handouts
    // before the drawer is even opened, on every page. The panel reports an empty
    // result instead, exactly as the character sheet's cross-game mode does.
    isAvailable: () => true,
    Panel: HandoutsPanel,
  },
  {
    id: 'dice-roller',
    label: 'Dice Roller',
    description: 'Roll dice and copy the result into a reply.',
    icon: Dices,
    // Available everywhere — rolling doesn't depend on a game.
    isAvailable: () => true,
    Panel: DiceRollerPanel,
  },
  {
    id: 'mark-all-read',
    label: 'Mark All Read',
    description: 'Mark every comment in this phase as read.',
    icon: CheckCheck,
    // Needs a phase to scope the bulk mark-read to, there's nothing to catch
    // up on once the game (and thus commenting) is over — note epilogue is
    // still writable, so it keeps this — and this only means
    // anything in manual read-tracking mode — 'auto' mode has no per-comment
    // read state for this to bulk-set. Outside a game there is no phase at all.
    isAvailable: (ctx) =>
      !!ctx.game &&
      !!ctx.game.currentPhase &&
      ctx.game.isGameWritable &&
      ctx.game.commentReadMode === 'manual',
    Panel: MarkAllReadPanel,
  },
];
