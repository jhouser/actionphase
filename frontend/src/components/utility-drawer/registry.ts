import { UserRound, Dices, CheckCheck } from 'lucide-react';
import type { UtilityDrawerUtility } from './types';
import { CharacterSheetPanel } from './panels/CharacterSheetPanel';
import { DiceRollerPanel } from './panels/DiceRollerPanel';
import { MarkAllReadPanel } from './panels/MarkAllReadPanel';

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
    description: 'View your abilities, skills, and inventory.',
    icon: UserRound,
    // Inside a game, only useful when the user controls a character there.
    // Outside one the panel loads the user's characters across all their games,
    // so offer it unconditionally and let the panel report an empty result —
    // gating here would require the list before the drawer is even opened.
    isAvailable: (ctx) => !ctx.game || ctx.game.userCharacters.length > 0,
    Panel: CharacterSheetPanel,
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
    // up on once the game (and thus commenting) is over, and this only means
    // anything in manual read-tracking mode — 'auto' mode has no per-comment
    // read state for this to bulk-set. Outside a game there is no phase at all.
    isAvailable: (ctx) =>
      !!ctx.game &&
      !!ctx.game.currentPhase &&
      !ctx.game.isGameCompleted &&
      ctx.game.commentReadMode === 'manual',
    Panel: MarkAllReadPanel,
  },
];
