import { Mail } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { Button } from './ui';
import { useCanMessageCharacter } from '../hooks/useCanMessageCharacter';
import type { Character } from '../types/characters';

interface MessageCharacterButtonProps {
  character: Character | undefined;
  /**
   * Called before navigating away. Surfaces that render inside a modal (the
   * character sheet) pass their dismiss handler so the modal doesn't linger
   * over the messages page.
   *
   * Return `false` to cancel the navigation — the character sheet uses this to
   * hold position while it asks about unsaved edits that leaving would destroy.
   */
  onNavigate?: () => boolean | void;
  className?: string;
}

/**
 * Envelope shortcut that jumps to the game's Messages tab with the New
 * Conversation form open and this character pre-selected as a participant.
 *
 * Renders nothing unless the user actually could start that conversation —
 * see useCanMessageCharacter for the conditions.
 */
export function MessageCharacterButton({
  character,
  onNavigate,
  className,
}: MessageCharacterButtonProps) {
  const navigate = useNavigate();
  const { canMessage, gameId } = useCanMessageCharacter(character);

  if (!canMessage || !character || !gameId) {
    return null;
  }

  const handleClick = () => {
    if (onNavigate?.() === false) return;
    navigate(`/games/${gameId}?tab=messages&newConversationWith=${character.id}`);
  };

  return (
    <Button
      variant="secondary"
      size="sm"
      onClick={handleClick}
      // flex-shrink-0: both placements sit beside a truncating character name,
      // which would otherwise squeeze the button before giving up its own width.
      // The tighter padding below `sm` keeps the icon-only form from sitting in
      // an over-wide pill.
      className={`flex-shrink-0 px-2 sm:px-3 ${className ?? ''}`}
      title={`Send a private message to ${character.name}`}
      aria-label={`Send a private message to ${character.name}`}
      data-faro-user-action-name="message-character"
    >
      <Mail className="w-4 h-4" aria-hidden="true" />
      {/* Icon-only on narrow screens, where header space is tight. The
          aria-label carries the full description at every width. */}
      <span className="hidden sm:inline">Message</span>
    </Button>
  );
}
