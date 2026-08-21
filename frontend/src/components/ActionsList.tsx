import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import { Button, Select, Badge, Alert } from './ui';
import { usePhaseSheetDraftConflicts } from '../hooks/useConflictingSheetDrafts';
import type { ActionWithDetails, GamePhase } from '../types/phases';
import { CreateActionResultForm } from './CreateActionResultForm';
import { StandaloneResultComposer } from './StandaloneResultComposer';
import { Modal } from './Modal';
import { MarkdownPreview } from './MarkdownPreview';
import CharacterAvatar from './CharacterAvatar';
import { useGameContext } from '../contexts/GameContext';
import { useCharacterSheetItems } from '../hooks/useCharacterSheetItems';

interface ActionsListProps {
  gameId: number;
  currentPhase?: GamePhase | null;
  className?: string;
}

export function ActionsList({ gameId, currentPhase, className = '' }: ActionsListProps) {
  const [selectedPhase, setSelectedPhase] = useState<number | null>(null);
  const [expandedActionId, setExpandedActionId] = useState<number | null>(null);
  const [showPublishConfirm, setShowPublishConfirm] = useState(false);
  const [showStandaloneComposer, setShowStandaloneComposer] = useState(false);
  const queryClient = useQueryClient();

  // Get all actions for the game
  const { data: actionsData, isLoading } = useQuery({
    queryKey: ['gameActions', gameId],
    queryFn: () => apiClient.phases.getGameActions(gameId).then(res => res.data),
    enabled: !!gameId,
    refetchInterval: 30000 // Refetch every 30 seconds
  });

  // Get all phases for filtering
  const { data: phasesData } = useQuery({
    queryKey: ['gamePhases', gameId],
    queryFn: () => apiClient.phases.getGamePhases(gameId).then(res => res.data),
    enabled: !!gameId
  });

  // Get unpublished results count for current phase
  const displayPhaseId = selectedPhase || currentPhase?.id;
  const { data: unpublishedCountData } = useQuery({
    queryKey: ['unpublishedResultsCount', gameId, displayPhaseId],
    queryFn: () => apiClient.phases.getUnpublishedResultsCount(gameId, displayPhaseId!).then(res => res.data),
    enabled: !!gameId && !!displayPhaseId,
    refetchInterval: 10000 // Refetch every 10 seconds
  });

  // Mutation for publishing all results
  const publishAllMutation = useMutation({
    mutationFn: () => apiClient.phases.publishAllPhaseResults(gameId, displayPhaseId!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['unpublishedResultsCount', gameId, displayPhaseId] });
      queryClient.invalidateQueries({ queryKey: ['actionResults', 'game', gameId] });
      queryClient.invalidateQueries({ queryKey: ['actionResults', 'user', gameId] });
      queryClient.invalidateQueries({ queryKey: ['draftCharacterUpdates'] });
      queryClient.invalidateQueries({ queryKey: ['draftUpdateCount'] });
      // Invalidate character data to show published character updates
      queryClient.invalidateQueries({ queryKey: ['characterData'] });
      setShowPublishConfirm(false);
    }
  });

  // Gated on the dialog being open: an ungated call fans out one draft-count request
  // per unpublished result on every render of the actions tab, for a warning that only
  // ever appears inside the confirmation dialog.
  const { hasConflict: hasPhaseSheetConflict, affectedCharacterCount } =
    usePhaseSheetDraftConflicts(gameId, displayPhaseId, showPublishConfirm);

  const actions = actionsData || [];
  const phases = phasesData || [];

  // Only show action phases
  const actionPhases = phases.filter(phase => phase.phase_type === 'action');

  // Filter actions by selected phase (only action phases)
  const filteredActions = displayPhaseId
    ? actions.filter(action => action.phase_id === displayPhaseId)
    : actions;

  // Group actions by phase for stats (only action phases)
  const actionsByPhase = actions.reduce((acc, action) => {
    const phaseId = action.phase_id;
    if (!acc[phaseId]) {
      acc[phaseId] = [];
    }
    acc[phaseId].push(action);
    return acc;
  }, {} as Record<number, ActionWithDetails[]>);

  if (isLoading) {
    return (
      <div className={`surface-base rounded-lg border border-theme-default p-6 ${className}`}>
        <div className="animate-pulse">
          <div className="h-6 surface-sunken rounded mb-4 w-1/3"></div>
          <div className="space-y-3">
            {[...Array(3)].map((_, i) => (
              <div key={i} className="h-24 surface-sunken rounded"></div>
            ))}
          </div>
        </div>
      </div>
    );
  }

  // Don't show the component if there are no action phases
  if (actionPhases.length === 0) {
    return null;
  }

  const unpublishedCount = unpublishedCountData?.count || 0;

  return (
    <div className={`surface-base rounded-lg border border-theme-default ${className}`} data-testid="actions-list">
      <div className="p-6">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h2 className="text-xl font-semibold text-content-primary">Submitted Actions</h2>
            <p className="text-sm text-content-secondary mt-1">
              View and manage player action submissions
            </p>
          </div>
          <div className="flex items-center space-x-2">
            <Badge variant="primary">
              {filteredActions.length} {filteredActions.length === 1 ? 'Action' : 'Actions'}
            </Badge>
            <Button
              variant="secondary"
              onClick={() => setShowStandaloneComposer(true)}
              data-testid="standalone-result-button"
              data-faro-user-action-name="open-standalone-result-composer"
            >
              Send Standalone Result
            </Button>
          </div>
        </div>

        {/* Compose a result with no parent submission (e.g. actions collected offsite) */}
        <Modal
          isOpen={showStandaloneComposer}
          onClose={() => setShowStandaloneComposer(false)}
          title="Send Standalone Result"
        >
          <div className="px-3 py-2 sm:px-6 sm:py-4">
            <p className="text-sm text-content-secondary mb-4">
              Create a result for a player without an action submission on the site.
              It will be saved as a draft so you can add character updates before publishing.
            </p>
            <StandaloneResultComposer
              gameId={gameId}
              onSuccess={() => setShowStandaloneComposer(false)}
            />
          </div>
        </Modal>

        {/* Publish All Results Button */}
        {displayPhaseId && unpublishedCount > 0 && (
          <div className="mb-6 p-4 bg-semantic-warning-subtle border border-semantic-warning rounded-lg">
            <div className="flex items-center justify-between">
              <div className="flex items-center">
                <svg className="w-5 h-5 text-semantic-warning mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
                <div>
                  <p className="font-medium text-content-primary">
                    {unpublishedCount} unpublished {unpublishedCount === 1 ? 'result' : 'results'}
                  </p>
                </div>
              </div>
              <Button
                variant="primary"
                onClick={() => setShowPublishConfirm(true)}
                disabled={publishAllMutation.isPending}
                className="bg-semantic-success hover:bg-semantic-success-hover"
              >
                {publishAllMutation.isPending ? 'Publishing...' : 'Publish All Results'}
              </Button>
            </div>
          </div>
        )}

        {/* Confirmation Dialog */}
        <Modal
          isOpen={showPublishConfirm}
          onClose={() => setShowPublishConfirm(false)}
          title="Publish All Results?"
        >
          <p className="text-sm text-content-secondary mb-4">
            This will publish {unpublishedCount} {unpublishedCount === 1 ? 'result' : 'results'} and make {unpublishedCount === 1 ? 'it' : 'them'} visible to players. This action cannot be undone.
          </p>
          {/* Publishing in bulk applies every conflicting result in one transaction, so
              a clobber here is guaranteed rather than merely possible. */}
          {hasPhaseSheetConflict && (
            <Alert variant="danger" className="mb-4" data-testid="publish-all-sheet-conflict-warning">
              {affectedCharacterCount === 1
                ? 'One character has staged sheet updates on more than one of these results.'
                : `${affectedCharacterCount} characters have staged sheet updates on more than one of these results.`}
              {' '}Each result stores a complete snapshot of the character sheet, so
              publishing them together will overwrite one set of changes with another.
              Consolidate each character&apos;s sheet updates into a single result first.
            </Alert>
          )}
          <div className="flex justify-end space-x-3">
            <Button
              variant="ghost"
              onClick={() => setShowPublishConfirm(false)}
              disabled={publishAllMutation.isPending}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              onClick={() => publishAllMutation.mutate()}
              disabled={publishAllMutation.isPending}
              className="bg-semantic-success hover:bg-semantic-success-hover"
            >
              {publishAllMutation.isPending ? 'Publishing...' : 'Confirm & Publish'}
            </Button>
          </div>
        </Modal>

        {/* Phase Filter - Only show action phases */}
        {actionPhases.length > 0 && (
          <div className="mb-6">
            <Select
              label="Filter by Action Phase"
              value={selectedPhase?.toString() || (currentPhase?.phase_type === 'action' ? currentPhase?.id.toString() : '') || ''}
              onChange={(e) => setSelectedPhase(e.target.value ? parseInt(e.target.value) : null)}
            >
              <option value="">All Action Phases</option>
              {actionPhases.map((phase) => (
                <option key={phase.id} value={phase.id}>
                  Phase {phase.phase_number} - {phase.title || 'Action Phase'}
                  {actionsByPhase[phase.id] ? ` (${actionsByPhase[phase.id].length})` : ' (0)'}
                </option>
              ))}
            </Select>
          </div>
        )}

        {/* Actions List */}
        {filteredActions.length === 0 ? (
          <div className="text-center py-8 text-content-tertiary">
            <svg className="w-12 h-12 mx-auto mb-3 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
            </svg>
            <p>No actions submitted yet</p>
            <p className="text-sm mt-1">
              {displayPhaseId ? 'No actions for this phase' : 'Actions will appear here once players submit them'}
            </p>
          </div>
        ) : (
          <div className="space-y-3">
            {filteredActions.map((action) => (
              <ActionCard
                key={action.id}
                action={action}
                gameId={gameId}
                isExpanded={expandedActionId === action.id}
                onToggleExpand={() => setExpandedActionId(expandedActionId === action.id ? null : action.id)}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

interface ActionCardProps {
  action: ActionWithDetails;
  gameId: number;
  isExpanded: boolean;
  onToggleExpand: () => void;
}

function ActionCard({ action, gameId, isExpanded, onToggleExpand }: ActionCardProps) {
  const [showResultForm, setShowResultForm] = useState(false);
  const { allGameCharacters, game } = useGameContext();

  // Lazy-fetch character sheet items for [[item]] tooltip resolution when expanded
  const sheetItems = useCharacterSheetItems(isExpanded && action.character_id ? action.character_id : null);
  const portraitAvatars = game?.portrait_avatars ?? false;
  const avatarUrl = action.character_id
    ? (allGameCharacters.find(c => c.id === action.character_id)?.avatar_url ?? null)
    : null;
  return (
    <div className="border border-theme-default rounded-lg overflow-hidden hover:border-interactive-primary transition-colors" data-testid="action-card">
      <button
        onClick={onToggleExpand}
        className="w-full px-4 py-3 text-left hover:surface-raised transition-colors"
      >
        {/* Mobile: Vertical Stack Layout */}
        <div className="md:hidden space-y-2">
          <div className="flex items-start justify-between gap-2">
            <div className="flex items-center gap-2 flex-1 min-w-0">
              <div className="flex-shrink-0">
                <CharacterAvatar
                  characterName={action.character_name || action.username || ''}
                  avatarUrl={avatarUrl}
                  size="sm"
                  shape={portraitAvatars ? 'portrait' : 'circle'}
                />
              </div>
              <div className="flex-1 min-w-0">
                <h4 className="font-medium text-base text-content-primary truncate">
                  {action.character_name || action.username}
                </h4>
                {action.character_name && action.username && (
                  <span className="text-xs text-content-tertiary">{action.username}</span>
                )}
              </div>
            </div>
            <svg
              className={`w-5 h-5 text-content-tertiary transition-transform flex-shrink-0 ${isExpanded ? 'rotate-180' : ''}`}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </div>
          <div className="flex flex-col gap-1 text-xs text-content-secondary ml-10">
            {action.phase_type && action.phase_number && (
              <span>
                Phase {action.phase_number} - {action.phase_title || action.phase_type.replace('_', ' ')}
              </span>
            )}
            <span className="text-content-tertiary">
              {new Date(action.submitted_at).toLocaleString()}
            </span>
          </div>
        </div>

        {/* Desktop: Horizontal Layout (Original) */}
        <div className="hidden md:flex items-center justify-between">
          <div className="flex items-center space-x-3 flex-1">
            <div className="flex-shrink-0">
              <CharacterAvatar
                characterName={action.character_name || action.username || ''}
                avatarUrl={avatarUrl}
                size="md"
                shape={portraitAvatars ? 'portrait' : 'circle'}
              />
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center space-x-2">
                <h4 className="font-medium text-content-primary">
                  {action.character_name || action.username}
                </h4>
                {action.character_name && action.username && (
                  <span className="text-sm text-content-tertiary">{action.username}</span>
                )}
              </div>
              <div className="flex items-center space-x-2 mt-1">
                {action.phase_type && action.phase_number && (
                  <span className="text-xs text-content-secondary">
                    Phase {action.phase_number} - {action.phase_title || action.phase_type.replace('_', ' ')}
                  </span>
                )}
                <span className="text-xs text-content-tertiary">•</span>
                <span className="text-xs text-content-secondary">
                  {new Date(action.submitted_at).toLocaleString()}
                </span>
              </div>
            </div>
          </div>
          <div className="flex items-center space-x-2">
            <svg
              className={`w-5 h-5 text-content-tertiary transition-transform ${isExpanded ? 'rotate-180' : ''}`}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </div>
        </div>
      </button>

      {isExpanded && (
        <div className="px-4 py-4 surface-raised border-t border-theme-default">
          <div className="surface-base p-4 rounded border border-theme-default">
            <MarkdownPreview content={action.content} fullWidth sheetItemRefs={sheetItems} />
          </div>
          <div className="mt-3 flex items-center justify-between text-xs text-content-tertiary">
            <span>
              Last updated: {new Date(action.updated_at).toLocaleString()}
            </span>
          </div>

          {/* GM Action: Send Result */}
          <div className="mt-4 pt-4 border-t border-theme-default">
            {!showResultForm ? (
              <Button
                variant="primary"
                onClick={() => setShowResultForm(true)}
                className="w-full bg-semantic-success hover:bg-semantic-success-hover"
              >
                Send Result to {action.character_name || action.username}
              </Button>
            ) : (
              <div>
                <div className="flex items-center justify-between mb-2">
                  <h5 className="font-semibold text-content-primary">Send Result</h5>
                </div>
                <CreateActionResultForm
                  gameId={gameId}
                  userId={action.user_id}
                  userName={action.username || 'Unknown User'}
                  characterId={action.character_id}
                  characterName={action.character_name}
                  actionSubmissionId={action.id}
                  onSuccess={() => {
                    setShowResultForm(false);
                    // Could add a success toast here
                  }}
                  onCancel={() => setShowResultForm(false)}
                />
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
