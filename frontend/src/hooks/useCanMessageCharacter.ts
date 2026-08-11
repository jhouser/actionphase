import { useQuery } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import { useOptionalGameContext } from '../contexts/GameContext';
import type { Character } from '../types/characters';

/**
 * Phase types during which a new private conversation may be started. Mirrors
 * the gate PrivateMessages applies to its "+ New" button — the envelope must
 * not offer a route into a form the messages page would then disable.
 */
const MESSAGING_PHASE_TYPES = ['common_room', 'interlude'];

interface UseCanMessageCharacterResult {
  /** Whether to offer a "send private message" affordance for this character. */
  canMessage: boolean;
  /** The game the target character belongs to, once known. */
  gameId: number | undefined;
}

/**
 * Whether the current user can start a private conversation with a character.
 *
 * True only when all of these hold:
 *  - the game's current phase allows new conversations (common room/interlude)
 *  - the user controls at least one approved character in that game
 *  - the target character is approved
 *  - the target is not one of the user's own characters
 *
 * Works both inside a game (reads GameContext) and outside one (the standalone
 * character page and the global utility drawer's sheet modal), where it falls
 * back to fetching the phase and the user's controllable characters itself.
 */
export function useCanMessageCharacter(
  character: Character | undefined
): UseCanMessageCharacterResult {
  const gameContext = useOptionalGameContext();
  const gameId = character?.game_id;
  // Only fall back to fetching when this hook is used outside the character's
  // own game — inside it, GameContext already holds the roster.
  const hasContextForGame = !!gameContext && gameContext.gameId === gameId;

  const { data: phaseData } = useQuery({
    queryKey: ['currentPhase', gameId],
    queryFn: () => apiClient.phases.getCurrentPhase(gameId!).then((r) => r.data),
    enabled: !!gameId,
    staleTime: 30_000,
  });

  const { data: fetchedCharacters } = useQuery({
    queryKey: ['userControllableCharacters', gameId],
    queryFn: () =>
      apiClient.characters.getUserControllableCharacters(gameId!).then((r) => r.data),
    enabled: !!gameId && !hasContextForGame,
    staleTime: 30_000,
  });

  const rawUserCharacters = hasContextForGame
    ? gameContext.userCharacters
    : fetchedCharacters;
  // Guard the shape rather than trusting it: this hook renders an affordance on
  // pages that may be loaded before/outside their game, and an unexpected
  // payload should hide the envelope, not crash the page around it.
  const userCharacters = Array.isArray(rawUserCharacters) ? rawUserCharacters : [];

  const phaseAllowsMessaging = MESSAGING_PHASE_TYPES.includes(
    phaseData?.phase?.phase_type ?? ''
  );

  const hasSendingCharacter = userCharacters.some((c) => c.status === 'approved');
  const isOwnCharacter = userCharacters.some((c) => c.id === character?.id);

  const canMessage =
    !!character &&
    !!gameId &&
    phaseAllowsMessaging &&
    hasSendingCharacter &&
    character.status === 'approved' &&
    !isOwnCharacter;

  return { canMessage, gameId };
}
