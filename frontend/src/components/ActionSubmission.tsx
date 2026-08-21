import { useState, useEffect, useRef } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { UserRound } from 'lucide-react';
import { apiClient } from '../lib/api';
import { useUserCharacters } from '../hooks/useUserCharacters';
import { useCharacterSheetItems } from '../hooks/useCharacterSheetItems';
import type { SheetItem } from '../hooks/useCharacterSheetItems';
import { CountdownTimer } from './CountdownTimer';
import { Button, Select, Alert, Drawer } from './ui';
import { SheetPanel } from './SheetPanel';
import { MarkdownPreview } from './MarkdownPreview';
import { CollapsibleMarkdown } from './CollapsibleMarkdown';
import { useExpandedSet } from '../hooks/useExpandedSet';
import { CommentEditor } from './CommentEditor';
import type { GamePhase, ActionSubmissionRequest, ActionWithDetails } from '../types/phases';

interface ActionSubmissionProps {
  gameId: number;
  currentPhase?: GamePhase | null;
  className?: string;
}

export function ActionSubmission({ gameId, currentPhase, className = '' }: ActionSubmissionProps) {
  const [content, setContent] = useState('');
  const [selectedCharacterId, setSelectedCharacterId] = useState<number | null>(null);
  const [isExpanded, setIsExpanded] = useState(false);
  const [showPreviousActions, setShowPreviousActions] = useState(false);
  const [isCurrentActionExpanded, setIsCurrentActionExpanded] = useState(false);
  const [sheetDrawerOpen, setSheetDrawerOpen] = useState(false);
  const insertSheetItemRef = useRef<((item: SheetItem) => void) | null>(null);

  const queryClient = useQueryClient();

  // Get user's controllable characters (works in anonymous mode)
  const { characters: availableCharacters } = useUserCharacters(gameId);

  // Determine which character's sheet to show in the sidebar
  const activeCharacterId = selectedCharacterId ?? (availableCharacters.length === 1 ? availableCharacters[0].id : null);
  const sheetItems = useCharacterSheetItems(activeCharacterId);
  const activeCharacter = availableCharacters.find((c) => c.id === activeCharacterId);

  //Content autosave id for comment textbox
  const autosaveRefId = currentPhase ? `action-${currentPhase.id}` : undefined;

  const handleInsertSheetItem = (item: SheetItem) => {
    insertSheetItemRef.current?.(item);
    setSheetDrawerOpen(false);
  };

  // Get user's previous actions
  const { data: userActionsData } = useQuery({
    queryKey: ['userActions', gameId],
    queryFn: () => apiClient.phases.getUserActions(gameId).then(res => res.data),
    enabled: !!gameId
  });

  // Ensure userActions is always an array
  const userActions = userActionsData || [];

  // Get current action for this phase if it exists
  const currentAction = currentPhase
    ? userActions.find(action => action.phase_id === currentPhase.id)
    : null;

  const submitActionMutation = useMutation({
    mutationFn: (data: ActionSubmissionRequest) => apiClient.phases.submitAction(gameId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['userActions', gameId] });
      setContent('');
      setIsExpanded(false);
      if (autosaveRefId) {
        localStorage.removeItem(autosaveRefId);
      }
    }
  });

  // Pre-populate form if editing existing action
  useEffect(() => {
    if (currentAction) {
      setContent(currentAction.content);
      if (currentAction.character_id) {
        setSelectedCharacterId(currentAction.character_id);
      }
    } else {
      setContent('');
      // Auto-select character if player has exactly one
      if (availableCharacters.length === 1) {
        setSelectedCharacterId(availableCharacters[0].id);
      } else {
        setSelectedCharacterId(null);
      }
    }
  }, [currentAction, availableCharacters]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!content.trim() || !currentPhase) return;

    const data: ActionSubmissionRequest = {
      content: content.trim(),
      character_id: selectedCharacterId || undefined
    };

    submitActionMutation.mutate(data);
  };

  const isActionPhase = currentPhase?.phase_type === 'action';
  const isPhaseActive = currentPhase?.is_active;
  const isDeadlinePassed = currentPhase?.deadline && new Date() > new Date(currentPhase.deadline);

  const canSubmitAction = isActionPhase && isPhaseActive && !isDeadlinePassed;

  if (!isActionPhase) {
    return (
      <div className={`surface-raised border border-theme-default rounded-lg p-6 text-center ${className}`}>
        <div className="text-content-tertiary mb-2">
          <svg className="w-12 h-12 mx-auto mb-3 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
        </div>
        <h3 className="text-lg font-medium text-content-primary mb-1">No Action Phase Active</h3>
        <p className="text-sm text-content-tertiary">
          Action submissions are only available during Action phases.
        </p>
      </div>
    );
  }

  return (
    <div className={`surface-base rounded-lg border border-theme-default ${className}`} data-testid="action-submission-container">
      <div className="p-6">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h2 className="text-xl font-semibold text-content-primary">Action Submission</h2>
            <div className="text-sm text-content-secondary mt-1">
              {currentAction
                ? 'Update your action for this phase'
                : currentPhase?.description
                  ? <MarkdownPreview content={currentPhase.description} />
                  : 'Submit your private action to the GM'
              }
            </div>
          </div>
          {currentPhase?.deadline && (
            <CountdownTimer
              deadline={currentPhase.deadline}
              className="flex-shrink-0"
              data-testid="phase-deadline"
            />
          )}
        </div>

        {!canSubmitAction && (
          <div className="mb-4">
            <Alert variant="warning">
              {isDeadlinePassed
                ? 'The action submission deadline has passed'
                : 'Action submission is not currently available'
              }
            </Alert>
          </div>
        )}

        {/* Current Action Display */}
        {currentAction && !isExpanded && (() => {
          return (
            <div className="mb-6 p-4 bg-semantic-info-subtle border border-semantic-info rounded-lg" data-testid="current-action-display">
              <div className="flex items-center justify-between mb-2">
                <h3 className="font-medium text-content-primary">Your Current Action</h3>
                {canSubmitAction && (
                  <Button
                    variant="primary"
                    size="sm"
                    onClick={() => setIsExpanded(true)}
                    data-testid="edit-action-button"
                  >
                    Edit
                  </Button>
                )}
              </div>
              <div className="text-sm text-content-primary surface-base p-3 rounded border border-theme-default" data-testid="action-content">
                <CollapsibleMarkdown
                  content={currentAction.content}
                  fullWidth
                  sheetItemRefs={sheetItems}
                  expanded={isCurrentActionExpanded}
                  onExpandedChange={setIsCurrentActionExpanded}
                />
              </div>
              {currentAction.character_name && (
                <p className="text-sm text-content-primary mt-2">
                  Acting as: <span className="font-medium">{currentAction.character_name}</span>
                </p>
              )}
              <p className="text-xs text-content-primary mt-1" data-testid="action-status">
                Last updated: {new Date(currentAction.updated_at).toLocaleString()}
              </p>
            </div>
          );
        })()}

        {/* Submission Form */}
        {(!currentAction || isExpanded) && canSubmitAction && (
          <form onSubmit={handleSubmit} className="space-y-4" data-testid="action-submission-form">
            {/* Character Selection - only show dropdown if multiple characters */}
            {availableCharacters.length > 1 && (
              <Select
                label="Acting as Character (Optional)"
                value={selectedCharacterId?.toString() || ''}
                onChange={(e) => setSelectedCharacterId(e.target.value ? parseInt(e.target.value) : null)}
                data-testid="character-select"
              >
                <option value="">Select a character (or leave blank)</option>
                {availableCharacters.map((character) => (
                  <option key={character.id} value={character.id}>
                    {character.name} ({character.character_type?.replace('_', ' ')})
                  </option>
                ))}
              </Select>
            )}

            {/* Display single character */}
            {availableCharacters.length === 1 && (
              <div className="p-3 bg-semantic-info-subtle border border-semantic-info rounded-md">
                <p className="text-sm text-content-primary">
                  <span className="font-medium">Acting as:</span> {availableCharacters[0].name}
                </p>
              </div>
            )}

            {/* Action Content */}
            <div>
              <label className="block text-sm font-medium text-content-primary mb-1">Your Action</label>
              <CommentEditor
                value={content}
                onChange={setContent}
                placeholder="Describe what your character does during this phase. Be as detailed as you like - this will only be seen by the GM until the game ends."
                rows={8}
                disabled={submitActionMutation.isPending}
                maxLength={100000}
                warnOnUnsavedChanges
                stickyTabBar
                showCharacterCount={true}
                textareaTestId="action-textarea"
                characters={availableCharacters}
                sheetItems={sheetItems}
                insertSheetItemRef={insertSheetItemRef}
                sheetButton={sheetItems.length > 0 ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => setSheetDrawerOpen(true)}
                    data-testid="sheet-toggle-button"
                    data-faro-user-action-name="open-character-sheet"
                    title="Character Sheet"
                    className="!px-2"
                  >
                    <UserRound className="w-4 h-4" />
                    <span className="hidden sm:inline">Character Sheet</span>
                  </Button>
                ) : undefined}
                autosaveRefId={autosaveRefId}
              />
              <p className="mt-1 text-xs text-content-tertiary">This action is private and will only be visible to the GM during the game. Maximum 100,000 characters.</p>
            </div>

            {/* Submit Buttons */}
            <div className="flex justify-end space-x-3">
              {currentAction && isExpanded && (
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => {
                    setIsExpanded(false);
                    setContent(currentAction.content);
                    setSelectedCharacterId(currentAction.character_id || null);
                  }}
                >
                  Cancel
                </Button>
              )}
              <Button
                type="submit"
                variant="primary"
                disabled={!content.trim() || submitActionMutation.isPending}
                data-testid="submit-action-button"
                data-faro-user-action-name="submit-action"
              >
                {submitActionMutation.isPending
                  ? 'Submitting...'
                  : currentAction
                  ? 'Update Action'
                  : 'Submit Action'
                }
              </Button>
            </div>
          </form>
        )}

        {/* Error Display */}
        {submitActionMutation.error && (
          <div className="mt-4">
            <Alert variant="danger">
              Failed to submit action: {
                submitActionMutation.error instanceof Error
                  ? submitActionMutation.error.message
                  : 'Unknown error'
              }
            </Alert>
          </div>
        )}
      </div>

      {/* Character Sheet Drawer */}
      <Drawer
        open={sheetDrawerOpen}
        onClose={() => setSheetDrawerOpen(false)}
        title="Character Sheet"
      >
        <SheetPanel
          items={sheetItems}
          onInsert={handleInsertSheetItem}
          characterName={activeCharacter?.name}
        />
      </Drawer>

      {/* Action History */}
      {(() => {
        const previousActions = userActions.filter(action => action.phase_id !== currentPhase?.id);
        return previousActions.length > 0 && (
          <div className="border-t border-theme-default">
            <button
              onClick={() => setShowPreviousActions(!showPreviousActions)}
              className="w-full px-6 py-4 flex items-center justify-between text-left hover:surface-raised transition-colors"
            >
              <div className="flex items-center">
                <svg className="w-5 h-5 text-content-tertiary mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <h3 className="text-lg font-medium text-content-primary">
                  Your Previous Actions ({previousActions.length})
                </h3>
              </div>
              <svg
                className={`w-5 h-5 text-content-tertiary transition-transform ${showPreviousActions ? 'rotate-180' : ''}`}
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
              </svg>
            </button>

            {showPreviousActions && (
              <div className="px-6 pb-6">
                <ActionHistory actions={userActions} currentPhaseId={currentPhase?.id} sheetItems={sheetItems} />
              </div>
            )}
          </div>
        );
      })()}
    </div>
  );
}

interface ActionHistoryProps {
  actions: ActionWithDetails[];
  currentPhaseId?: number;
  sheetItems?: SheetItem[];
}

function ActionHistory({ actions, currentPhaseId, sheetItems = [] }: ActionHistoryProps) {
  const expandedActions = useExpandedSet();

  // Filter out the current phase action
  const previousActions = actions.filter(action => action.phase_id !== currentPhaseId);
  const sortedActions = [...previousActions].sort((a, b) => (b.phase_number || 0) - (a.phase_number || 0));


  if (sortedActions.length === 0) {
    return (
      <div className="text-center py-4 text-content-tertiary">
        <p>No previous actions</p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {sortedActions.map((action) => {
        const isExpanded = expandedActions.isExpanded(action.id);

        return (
          <div key={action.id} className="border border-theme-default rounded-lg p-4">
            <div className="flex items-start justify-between mb-2">
              <div className="flex items-center space-x-2">
                <span className="px-2 py-1 surface-raised text-content-secondary text-xs rounded font-medium">
                  Phase {action.phase_number} - {action.phase_type?.replace('_', ' ')}
                </span>
                {action.character_name && (
                  <span className="text-sm text-content-secondary">
                    as {action.character_name}
                  </span>
                )}
              </div>
              <span className="text-xs text-content-tertiary">
                {new Date(action.submitted_at).toLocaleString()}
              </span>
            </div>
            <div className="text-sm text-content-primary surface-raised p-3 rounded">
              <CollapsibleMarkdown
                content={action.content}
                fullWidth
                sheetItemRefs={sheetItems}
                expanded={isExpanded}
                onExpandedChange={() => expandedActions.toggle(action.id)}
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}
