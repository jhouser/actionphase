import { useState } from 'react';
import { Button, Select, Alert } from './ui';
import { CommentEditor } from './CommentEditor';
import type { Character } from '../types/characters';

interface UnreadReplyBoxProps {
  controllableCharacters: Character[];
  /** Characters eligible for @-mention autocomplete (broader than controllableCharacters). */
  mentionableCharacters: Character[];
  defaultCharacterId: number | null;
  onSubmit: (characterId: number, content: string) => void;
  isSubmitting: boolean;
  error?: string | null;
  autosaveRefId?: string;
}

export function UnreadReplyBox({
  controllableCharacters,
  mentionableCharacters,
  defaultCharacterId,
  onSubmit,
  isSubmitting,
  error,
  autosaveRefId = undefined,
}: UnreadReplyBoxProps) {
  const [characterId, setCharacterId] = useState<number | null>(defaultCharacterId);
  const [content, setContent] = useState('');

  if (controllableCharacters.length === 0) {
    return (
      <p className="text-sm text-content-tertiary italic">
        You don't control a character in this game, so you can't reply here.
      </p>
    );
  }

  const handleSubmit = () => {
    const selectedCharacterId = characterId ?? controllableCharacters[0].id;
    if (!content.trim()) return;
    onSubmit(selectedCharacterId, content.trim());
  };

  return (
    <div className="space-y-2">
      {controllableCharacters.length > 1 && (
        <Select
          label="Reply as"
          value={characterId ?? controllableCharacters[0].id}
          onChange={(e) => setCharacterId(parseInt(e.target.value, 10))}
        >
          {controllableCharacters.map((character) => (
            <option key={character.id} value={character.id}>
              {character.name}
            </option>
          ))}
        </Select>
      )}

      <CommentEditor
        value={content}
        onChange={setContent}
        placeholder="Write a reply..."
        disabled={isSubmitting}
        rows={3}
        maxLength={10000}
        characters={mentionableCharacters}
        textareaTestId="unread-reply-textarea"
        autosaveRefId={autosaveRefId}
      />

      {error && <Alert variant="danger">{error}</Alert>}

      <div className="flex justify-end">
        <Button
          variant="primary"
          size="sm"
          onClick={handleSubmit}
          loading={isSubmitting}
          disabled={!content.trim()}
          data-testid="unread-reply-send"
        >
          Send
        </Button>
      </div>
    </div>
  );
}
