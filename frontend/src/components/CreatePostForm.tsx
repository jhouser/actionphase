import { useState, useEffect } from 'react';
import type { Character } from '../types/characters';
import { CommentEditor } from './CommentEditor';
import { Button, Select, Alert } from './ui';
import { ChevronDown, ChevronUp, Plus } from 'lucide-react';

interface CreatePostFormProps {
  gameId: number;
  phaseId?: number;
  characters: Character[]; // Characters the user can post as
  allCharacters?: Character[]; // All characters for autocomplete mentions
  onSubmit: (characterId: number, content: string) => Promise<void>;
  isSubmitting: boolean;
  shouldStartCollapsed?: boolean; // Start collapsed when posts already exist
}

export function CreatePostForm({ gameId: _gameId, phaseId = undefined, characters, allCharacters, onSubmit, isSubmitting, shouldStartCollapsed = false }: CreatePostFormProps) {
  const [selectedCharacterId, setSelectedCharacterId] = useState<number | null>(null);
  const [content, setContent] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [isCollapsed, setIsCollapsed] = useState(shouldStartCollapsed);
  
  //Content autosave id for comment textbox
  const autosaveRefId = phaseId ? `cr-main-post-${phaseId}` : undefined;

  // Auto-select first character if available
  useEffect(() => {
    if (characters.length > 0 && selectedCharacterId === null) {
      setSelectedCharacterId(characters[0].id);
    }
  }, [characters, selectedCharacterId]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!selectedCharacterId) {
      setError('Please select a character');
      return;
    }

    if (!content.trim()) {
      setError('Please enter a message');
      return;
    }

    try {
      await onSubmit(selectedCharacterId, content.trim());
      setContent('');
      if (autosaveRefId) {
        localStorage.removeItem(autosaveRefId);
      }
    } catch (err: unknown) {
      // Use fallback message for generic errors like "Network error"
      const errorMessage = err instanceof Error && err.message !== 'Network error'
        ? err.message
        : 'Failed to create post';
      setError(errorMessage);
    }
  };

  if (characters.length === 0) {
    return (
      <Alert variant="warning">
        You need a character to post in the Common Room. Please create a character first.
      </Alert>
    );
  }

  // Collapsed view - show compact button to expand
  if (isCollapsed) {
    return (
      <button
        type="button"
        onClick={() => setIsCollapsed(false)}
        className="w-full bg-interactive-primary hover:bg-interactive-primary-hover text-white font-semibold rounded-lg p-4 mb-6 flex items-center justify-center gap-2 transition-colors shadow-md"
      >
        <Plus className="w-5 h-5" />
        Create New GM Post
        <ChevronDown className="w-5 h-5" />
      </button>
    );
  }

  // Expanded view - show full form
  return (
    <form onSubmit={handleSubmit} className="bg-interactive-primary-subtle border-2 border-interactive-primary shadow-lg rounded-lg p-6 mb-6">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <svg className="w-6 h-6 text-interactive-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5.882V19.24a1.76 1.76 0 01-3.417.592l-2.147-6.15M18 13a3 3 0 100-6M5.436 13.683A4.001 4.001 0 017 6h1.832c4.1 0 7.625-1.234 9.168-3v14c-1.543-1.766-5.067-3-9.168-3H7a3.988 3.988 0 01-1.564-.317z" />
          </svg>
          <h3 className="text-lg font-bold text-content-primary">Create New GM Post</h3>
        </div>
        <button
          type="button"
          onClick={() => setIsCollapsed(true)}
          className="text-content-secondary hover:text-content-primary transition-colors"
          aria-label="Collapse form"
        >
          <ChevronUp className="w-5 h-5" />
        </button>
      </div>

      <Alert variant="info" className="mb-4">
        <strong>💡 Tip:</strong> You can use Markdown formatting for rich text. Type <code className="surface-sunken px-1 rounded">@</code> to mention characters and trigger autocomplete.
      </Alert>

      {error && (
        <Alert variant="danger" className="mb-4">
          {error}
        </Alert>
      )}

      {/* Only show character selector if user has multiple characters */}
      {characters.length > 1 && (
        <div className="mb-4">
          <Select
            id="character"
            label="Post as:"
            value={selectedCharacterId || ''}
            onChange={(e) => setSelectedCharacterId(Number(e.target.value))}
            disabled={isSubmitting}
          >
            {characters.map((char) => (
              <option key={char.id} value={char.id}>
                {char.name}
              </option>
            ))}
          </Select>
        </div>
      )}

      <div className="mb-4">
        <label htmlFor="content" className="block text-sm font-medium text-content-primary mb-2">
          Post Content (Markdown supported):
        </label>
        <CommentEditor
          id="content"
          value={content}
          onChange={setContent}
          placeholder="# Phase Title&#10;&#10;## Important Information&#10;&#10;Your phase description here...&#10;&#10;- Bullet point 1&#10;- Bullet point 2&#10;&#10;**Remember:** This post will be visible to all players.&#10;&#10;💡 Use @CharacterName to mention characters"
          disabled={isSubmitting}
          rows={12}
          warnOnUnsavedChanges
          stickyTabBar
          characters={allCharacters || characters}
          showPreviewByDefault={false}
          maxLength={50000}
          showCharacterCount={true}
          autosaveRefId={autosaveRefId}
        />
        <p className="text-xs text-content-secondary mt-1">
          Maximum 50,000 characters (longer posts will be collapsible for players)
        </p>
      </div>

      <Button
        type="submit"
        variant="primary"
        disabled={isSubmitting || !content.trim()}
        className="w-full text-lg py-3"
        data-faro-user-action-name="create-post"
      >
        {isSubmitting ? 'Creating GM Post...' : 'Create GM Post'}
      </Button>
    </form>
  );
}
