import { useState } from 'react';
import { useDraftPost, useUpdateDraftPost, useDeleteDraftPost, useCreateDraftPost } from '../hooks';
import { useOptionalGameContext } from '../contexts/GameContext';
import { MarkdownPreview } from './MarkdownPreview';
import { CommentEditor } from './CommentEditor';
import { Button, Select } from './ui';
import { Modal } from './Modal';

interface DraftPostSectionProps {
  phaseId: number;
  onCreateDraft: () => void;
}

function CreateDraftModal({
  phaseId,
  onClose,
  onSuccess,
}: {
  phaseId: number;
  onClose: () => void;
  onSuccess: () => void;
}) {
  const gameContext = useOptionalGameContext();
  const userCharacters = gameContext?.userCharacters ?? [];

  const [characterId, setCharacterId] = useState<number | ''>(
    userCharacters.length > 0 ? userCharacters[0].id : ''
  );
  const [content, setContent] = useState('');
  const createMutation = useCreateDraftPost(phaseId);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!characterId || !content.trim()) return;
    await createMutation.mutateAsync({ characterId: characterId as number, content: content.trim() });
    onSuccess();
  };

  return (
    <Modal isOpen title="Write Draft Opening Post" onClose={onClose}>
      <form onSubmit={handleSubmit} className="space-y-4">
        <p className="text-sm text-content-secondary">
          This post will be published automatically when the phase activates.
        </p>

        {userCharacters.length > 0 && (
          <Select
            id="create-draft-character"
            label="Post as"
            value={characterId}
            onChange={(e) => setCharacterId(e.target.value === '' ? '' : Number(e.target.value))}
            required
          >
            <option value="">Select a character</option>
            {userCharacters.map(char => (
              <option key={char.id} value={char.id}>{char.name}</option>
            ))}
          </Select>
        )}

        <div>
          <label className="block text-sm font-medium text-content-primary mb-1">Content</label>
          <CommentEditor
            value={content}
            onChange={setContent}
            placeholder="Write your opening post here. Supports markdown and character mentions."
            rows={12}
            maxLength={50000}
            showCharacterCount
            textareaTestId="create-draft-content"
          />
        </div>

        <div className="flex justify-end gap-3">
          <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
          <Button type="submit" variant="primary" disabled={createMutation.isPending || !characterId || !content.trim()}>
            {createMutation.isPending ? 'Saving...' : 'Save Draft'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

function EditDraftModal({
  phaseId,
  initialContent,
  onClose,
}: {
  phaseId: number;
  initialContent: string;
  onClose: () => void;
}) {
  const gameContext = useOptionalGameContext();
  const allGameCharacters = gameContext?.allGameCharacters ?? [];

  const [content, setContent] = useState(initialContent);
  const updateMutation = useUpdateDraftPost(phaseId);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!content.trim()) return;
    await updateMutation.mutateAsync(content.trim());
    onClose();
  };

  return (
    <Modal isOpen title="Edit Draft Opening Post" onClose={onClose}>
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-content-primary mb-1">Content</label>
          <CommentEditor
            value={content}
            onChange={setContent}
            placeholder="Write your opening post here. Supports markdown and character mentions."
            rows={12}
            maxLength={50000}
            showCharacterCount
            characters={allGameCharacters}
            textareaTestId="edit-draft-content"
          />
        </div>

        <div className="flex justify-end gap-3">
          <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
          <Button type="submit" variant="primary" disabled={updateMutation.isPending || !content.trim()}>
            {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

function PreviewDraftModal({
  content,
  onClose,
}: {
  content: string;
  onClose: () => void;
}) {
  return (
    <Modal isOpen title="Draft Post Preview" onClose={onClose}>
      <div className="space-y-3">
        <p className="text-xs text-content-tertiary italic">
          This is how the post will appear when the phase activates.
        </p>
        <div className="border border-dashed border-theme-default rounded-lg p-4 surface-raised">
          <MarkdownPreview content={content} />
        </div>
        <div className="flex justify-end">
          <Button variant="ghost" onClick={onClose}>Close</Button>
        </div>
      </div>
    </Modal>
  );
}

export function DraftPostSection({ phaseId, onCreateDraft }: DraftPostSectionProps) {
  const { data: draft, isLoading } = useDraftPost(phaseId);
  const deleteMutation = useDeleteDraftPost(phaseId);

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [showPreviewModal, setShowPreviewModal] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const handleCreateSuccess = () => {
    setShowCreateModal(false);
    onCreateDraft();
  };

  if (isLoading) {
    return (
      <div className="mt-3 pt-3 border-t border-theme-default animate-pulse">
        <div className="h-4 surface-sunken rounded w-1/3"></div>
      </div>
    );
  }

  return (
    <div
      className="mt-3 pt-3 border-t border-theme-default"
      onClick={(e) => e.stopPropagation()}
    >
      {draft === null || draft === undefined ? (
        <div className="flex items-center gap-2 text-sm">
          <span className="text-content-tertiary">No draft post</span>
          <Button
            variant="ghost"
            onClick={() => setShowCreateModal(true)}
            data-testid="add-draft-post-btn"
          >
            + Add Draft Post
          </Button>
        </div>
      ) : (
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-content-secondary uppercase tracking-wide">Draft Post</span>
            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                onClick={() => setShowPreviewModal(true)}
                data-testid="preview-draft-btn"
              >
                Preview
              </Button>
              <Button
                variant="ghost"
                onClick={() => setShowEditModal(true)}
                data-testid="edit-draft-btn"
              >
                Edit
              </Button>
              {confirmDelete ? (
                <span className="flex items-center gap-1 text-xs">
                  <span className="text-content-secondary">Delete?</span>
                  <Button
                    variant="danger"
                    onClick={() => { deleteMutation.mutate(); setConfirmDelete(false); }}
                  >
                    Yes
                  </Button>
                  <Button
                    variant="ghost"
                    onClick={() => setConfirmDelete(false)}
                  >
                    No
                  </Button>
                </span>
              ) : (
                <Button
                  variant="danger"
                  onClick={() => setConfirmDelete(true)}
                  data-testid="delete-draft-btn"
                >
                  Delete
                </Button>
              )}
            </div>
          </div>

          <div className="text-sm text-content-secondary line-clamp-2 italic">
            {draft.character_name && (
              <span className="font-medium not-italic">{draft.character_name}: </span>
            )}
            {draft.content.slice(0, 120)}{draft.content.length > 120 ? '…' : ''}
          </div>
        </div>
      )}

      {showCreateModal && (
        <CreateDraftModal phaseId={phaseId} onClose={() => setShowCreateModal(false)} onSuccess={handleCreateSuccess} />
      )}
      {showEditModal && draft && (
        <EditDraftModal phaseId={phaseId} initialContent={draft.content} onClose={() => setShowEditModal(false)} />
      )}
      {showPreviewModal && draft && (
        <PreviewDraftModal content={draft.content} onClose={() => setShowPreviewModal(false)} />
      )}
    </div>
  );
}
