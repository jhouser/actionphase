import { useState } from 'react';
import { PHASE_TYPE_DESCRIPTIONS } from '../types/phases';
import type { CreatePhaseRequest } from '../types/phases';
import { Button, Select, Input, DateTimeInput } from './ui';
import { Modal } from './Modal';
import { CommentEditor } from './CommentEditor';
import { localDateTimeToUTC } from '../utils/timezone';
import { useOptionalGameContext } from '../contexts/GameContext';

export interface DraftPostData {
  characterId: number;
  content: string;
}

interface CreatePhaseModalProps {
  onClose: () => void;
  onSubmit: (data: CreatePhaseRequest, draftPost?: DraftPostData) => void;
  isSubmitting: boolean;
}

export function CreatePhaseModal({ onClose, onSubmit, isSubmitting }: CreatePhaseModalProps) {
  const gameContext = useOptionalGameContext();
  const userCharacters = gameContext?.userCharacters ?? [];

  const [formData, setFormData] = useState<CreatePhaseRequest>({
    phase_type: 'common_room',
    deadline: undefined
  });

  const [showDraftSection, setShowDraftSection] = useState(false);
  const [draftCharacterId, setDraftCharacterId] = useState<number | ''>('');
  const [draftContent, setDraftContent] = useState('');

  const handlePhaseTypeChange = (value: CreatePhaseRequest['phase_type']) => {
    setFormData(prev => ({ ...prev, phase_type: value }));
    if (value !== 'common_room') {
      setShowDraftSection(false);
      setDraftCharacterId('');
      setDraftContent('');
    }
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const phaseData = {
      ...formData,
      start_time: formData.start_time ? localDateTimeToUTC(formData.start_time) : undefined,
      deadline: formData.deadline ? localDateTimeToUTC(formData.deadline) : undefined
    };

    const hasDraft = showDraftSection && draftCharacterId !== '' && draftContent.trim();
    const draftPost: DraftPostData | undefined = hasDraft
      ? { characterId: draftCharacterId as number, content: draftContent.trim() }
      : undefined;

    onSubmit(phaseData, draftPost);
  };

  return (
    <Modal isOpen={true} onClose={onClose} title="Create New Phase">
      <form onSubmit={handleSubmit}>
        <div className="space-y-4">
            <div>
              <Select
                id="phase-type"
                label="Phase Type"
                value={formData.phase_type}
                onChange={(e) => handlePhaseTypeChange(e.target.value as CreatePhaseRequest['phase_type'])}
                required
                helperText={PHASE_TYPE_DESCRIPTIONS[formData.phase_type]}
              >
                <option value="common_room">Common Room</option>
                <option value="action">Action Phase</option>
                <option value="interlude">Interlude</option>
              </Select>
            </div>

            <div>
              <Input
                id="phase-title"
                label="Title (Optional)"
                type="text"
                value={formData.title || ''}
                onChange={(e) => setFormData(prev => ({
                  ...prev,
                  title: e.target.value || undefined
                }))}
                placeholder="e.g., 'The Gathering Storm'"
                helperText="Give this phase a custom name"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-content-primary mb-1">Description (Optional)</label>
              <CommentEditor
                value={formData.description || ''}
                onChange={(value) => setFormData(prev => ({
                  ...prev,
                  description: value || undefined
                }))}
                placeholder="Describe what happens in this phase. Supports markdown."
                rows={3}
                maxLength={2000}
                showCharacterCount
                textareaTestId="phase-description"
              />
              <p className="mt-1 text-xs text-content-tertiary">Supports markdown. Shown to players during action submission.</p>
            </div>

            <div>
              <DateTimeInput
                id="phase-start-time"
                label="Auto-activate at (Optional)"
                value={formData.start_time || ''}
                onChange={(e) => setFormData(prev => ({
                  ...prev,
                  start_time: e.target.value || undefined
                }))}
                helperText="Phase will activate automatically at this time. Leave blank to activate manually."
              />
            </div>

            <div>
              <DateTimeInput
                id="phase-deadline"
                label="Deadline (Optional)"
                value={formData.deadline || ''}
                onChange={(e) => setFormData(prev => ({
                  ...prev,
                  deadline: e.target.value || undefined
                }))}
                helperText="Set a deadline to create urgency for this phase"
              />
            </div>

            {/* Draft opening post section — only for Common Room phases */}
            {formData.phase_type === 'common_room' && (
              <div className="border border-theme-default rounded-lg overflow-hidden">
                <Button
                  variant="ghost"
                  onClick={() => setShowDraftSection(prev => !prev)}
                  data-testid="draft-post-toggle"
                  className="w-full flex items-center justify-between px-4 py-3"
                >
                  <span>Draft Opening Post (Optional)</span>
                  <span className="text-content-tertiary">{showDraftSection ? '▲' : '▼'}</span>
                </Button>

                {showDraftSection && (
                  <div className="px-4 py-3 space-y-3 surface-base">
                    <p className="text-xs text-content-tertiary">
                      Write your opening post now. It will be published automatically when this phase activates.
                    </p>

                    {userCharacters.length > 0 && (
                      <Select
                        id="draft-character"
                        label="Post as"
                        value={draftCharacterId}
                        onChange={(e) => setDraftCharacterId(e.target.value === '' ? '' : Number(e.target.value))}
                        required={showDraftSection}
                        data-testid="draft-character-select"
                      >
                        <option value="">Select a character</option>
                        {userCharacters.map(char => (
                          <option key={char.id} value={char.id}>{char.name}</option>
                        ))}
                      </Select>
                    )}

                    <div>
                      <label className="block text-sm font-medium text-content-primary mb-1">Post Content</label>
                      <CommentEditor
                        value={draftContent}
                        onChange={setDraftContent}
                        placeholder="Write your opening post here. Supports markdown and character mentions."
                        rows={8}
                        maxLength={50000}
                        showCharacterCount
                        textareaTestId="draft-post-content"
                      />
                    </div>
                  </div>
                )}
              </div>
            )}
        </div>

        <div className="flex justify-end space-x-3 mt-6">
          <Button
            type="button"
            variant="ghost"
            onClick={onClose}
          >
            Cancel
          </Button>
          <Button
            type="submit"
            variant="primary"
            disabled={isSubmitting}
            data-faro-user-action-name="create-phase"
          >
            {isSubmitting ? 'Creating...' : 'Create Phase'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
