/**
 * Participant Actions Menu Component
 *
 * Provides a dropdown menu for GM actions on game participants:
 * - Promote audience members to co-GM
 * - Demote co-GM back to audience
 * - Remove players from game
 *
 * Only shows actions appropriate for the participant's current role.
 * Only primary GM can promote/demote. Both primary and co-GM can remove.
 */

import { useState, useRef, useEffect } from 'react';
import { Button, Alert, Input } from './ui';
import { Modal } from './Modal';
import { usePromoteToCoGM, useDemoteFromCoGM, useRemovePlayer, useTransitionPlayerToAudience } from '../hooks/usePlayerManagement';
import { apiClient } from '../lib/api';
import type { GameParticipant, GameApplication } from '../types/games';
import { logger } from '@/services/LoggingService';

interface ParticipantActionsMenuProps {
  gameId: number;
  participant?: GameParticipant;
  application?: GameApplication;
  isPrimaryGM: boolean;
  onSuccess?: () => void;
}

export function ParticipantActionsMenu({
  gameId,
  participant,
  application,
  isPrimaryGM,
  onSuccess
}: ParticipantActionsMenuProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [showPromoteConfirm, setShowPromoteConfirm] = useState(false);
  const [showDemoteConfirm, setShowDemoteConfirm] = useState(false);
  const [showTransitionToAudienceConfirm, setShowTransitionToAudienceConfirm] = useState(false);
  const [transitionConfirmText, setTransitionConfirmText] = useState('');
  const [showRemoveConfirm, setShowRemoveConfirm] = useState(false);
  const [showApproveConfirm, setShowApproveConfirm] = useState(false);
  const [showRejectConfirm, setShowRejectConfirm] = useState(false);
  const [isProcessing, setIsProcessing] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  const promoteToCoGM = usePromoteToCoGM(gameId);
  const demoteFromCoGM = useDemoteFromCoGM(gameId);
  const transitionToAudience = useTransitionPlayerToAudience(gameId);
  const removePlayer = useRemovePlayer(gameId);

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside);
    }

    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [isOpen]);

  const handlePromote = async () => {
    if (!participant) return;
    try {
      await promoteToCoGM.mutateAsync(participant.user_id);
      setShowPromoteConfirm(false);
      setIsOpen(false);
      onSuccess?.();
    } catch (error) {
      logger.error('Failed to promote to co-GM', { error, gameId, userId: participant.user_id, username: participant.username });
    }
  };

  const handleDemote = async () => {
    if (!participant) return;
    try {
      await demoteFromCoGM.mutateAsync(participant.user_id);
      setShowDemoteConfirm(false);
      setIsOpen(false);
      onSuccess?.();
    } catch (error) {
      logger.error('Failed to demote from co-GM', { error, gameId, userId: participant.user_id, username: participant.username });
    }
  };

  const handleTransitionToAudience = async () => {
    if (!participant) return;
    try {
      await transitionToAudience.mutateAsync(participant.user_id);
      setShowTransitionToAudienceConfirm(false);
      setTransitionConfirmText('');
      setIsOpen(false);
      onSuccess?.();
    } catch (error) {
      logger.error('Failed to transition player to audience', { error, gameId, userId: participant.user_id, username: participant.username });
    }
  };

  const handleRemove = async () => {
    if (!participant) return;
    try {
      await removePlayer.mutateAsync(participant.user_id);
      setShowRemoveConfirm(false);
      setIsOpen(false);
      onSuccess?.();
    } catch (error) {
      logger.error('Failed to remove player', { error, gameId, userId: participant.user_id, username: participant.username });
    }
  };

  const handleApprove = async () => {
    if (!application) return;
    try {
      setIsProcessing(true);
      setErrorMessage(null);
      await apiClient.games.reviewGameApplication(gameId, application.id, { action: 'approve' });
      setShowApproveConfirm(false);
      setIsOpen(false);
      onSuccess?.();
    } catch (error) {
      logger.error('Failed to approve application', { error, gameId, applicationId: application.id });
      setErrorMessage(error instanceof Error ? error.message : 'Failed to approve application');
    } finally {
      setIsProcessing(false);
    }
  };

  const handleReject = async () => {
    if (!application) return;
    try {
      setIsProcessing(true);
      setErrorMessage(null);
      await apiClient.games.reviewGameApplication(gameId, application.id, { action: 'reject' });
      setShowRejectConfirm(false);
      setIsOpen(false);
      onSuccess?.();
    } catch (error) {
      logger.error('Failed to reject application', { error, gameId, applicationId: application.id });
      setErrorMessage(error instanceof Error ? error.message : 'Failed to reject application');
    } finally {
      setIsProcessing(false);
    }
  };

  // Determine available actions
  const canApprove = !!application;
  const canReject = !!application;
  const canPromote = !!participant && isPrimaryGM && participant.role === 'audience';
  const canDemote = !!participant && isPrimaryGM && participant.role === 'co_gm';
  const canTransitionToAudience = !!participant && isPrimaryGM && participant.role === 'player';
  const canRemove = !!participant; // Both primary GM and co-GM can remove

  // If no actions available, don't render anything
  if (!canPromote && !canDemote && !canTransitionToAudience && !canRemove && !canApprove && !canReject) {
    return null;
  }

  return (
    <>
      <div className="relative" ref={menuRef}>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => setIsOpen(!isOpen)}
          aria-label="Participant actions"
        >
          ⋮
        </Button>

        {isOpen && (
          <div className="absolute right-0 mt-2 w-48 rounded-md shadow-lg surface-base border border-theme-default z-10 surface-raised">
            <div className="py-1" role="menu">
              {canApprove && (
                <button
                  onClick={() => {
                    setShowApproveConfirm(true);
                    setIsOpen(false);
                  }}
                  className="block w-full text-left px-4 py-2 text-sm text-content-primary hover:surface-raised"
                  role="menuitem"
                >
                  Approve Application
                </button>
              )}

              {canReject && (
                <button
                  onClick={() => {
                    setShowRejectConfirm(true);
                    setIsOpen(false);
                  }}
                  className="block w-full text-left px-4 py-2 text-sm text-semantic-danger hover:surface-raised"
                  role="menuitem"
                >
                  Reject Application
                </button>
              )}

              {canPromote && (
                <button
                  onClick={() => {
                    setShowPromoteConfirm(true);
                    setIsOpen(false);
                  }}
                  className="block w-full text-left px-4 py-2 text-sm text-content-primary hover:surface-raised"
                  role="menuitem"
                >
                  Promote to Co-GM
                </button>
              )}

              {canDemote && (
                <button
                  onClick={() => {
                    setShowDemoteConfirm(true);
                    setIsOpen(false);
                  }}
                  className="block w-full text-left px-4 py-2 text-sm text-content-primary hover:surface-raised"
                  role="menuitem"
                >
                  Demote from Co-GM
                </button>
              )}

              {canTransitionToAudience && (
                <button
                  onClick={() => {
                    setShowTransitionToAudienceConfirm(true);
                    setIsOpen(false);
                  }}
                  className="block w-full text-left px-4 py-2 text-sm text-content-primary hover:surface-raised"
                  role="menuitem"
                >
                  <div>Move to Former Players</div>
                  <div className="text-xs text-content-tertiary">For players exiting the game</div>
                </button>
              )}

              {canRemove && (
                <button
                  onClick={() => {
                    setShowRemoveConfirm(true);
                    setIsOpen(false);
                  }}
                  className="block w-full text-left px-4 py-2 text-sm text-semantic-danger hover:surface-raised"
                  role="menuitem"
                >
                  Remove Player
                </button>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Promote to Co-GM Confirmation Modal */}
      <Modal
        isOpen={showPromoteConfirm}
        onClose={() => setShowPromoteConfirm(false)}
        title="Promote to Co-GM?"
      >
        <div className="space-y-4">
          <Alert variant="info" title="Co-GM Permissions">
            <p className="text-sm">
              Co-GMs can do everything you can except:
            </p>
            <ul className="text-sm space-y-1 list-disc list-inside mt-2">
              <li>Edit game settings (title, description, etc.)</li>
              <li>Promote others to co-GM</li>
            </ul>
            <p className="text-sm mt-2">
              They will have full access to manage phases, characters, actions, and messages.
            </p>
          </Alert>

          <p className="text-sm text-content-secondary">
            Promote <strong>{participant?.username}</strong> to co-GM?
          </p>

          <div className="flex justify-end gap-3">
            <Button
              variant="secondary"
              onClick={() => setShowPromoteConfirm(false)}
              disabled={promoteToCoGM.isPending}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              onClick={handlePromote}
              loading={promoteToCoGM.isPending}
            >
              Promote to Co-GM
            </Button>
          </div>

          {promoteToCoGM.isError && (
            <Alert variant="danger" dismissible onDismiss={() => promoteToCoGM.reset()}>
              <p className="text-sm">
                Failed to promote to co-GM. {(promoteToCoGM.error as Error)?.message || 'Please try again.'}
              </p>
            </Alert>
          )}
        </div>
      </Modal>

      {/* Demote from Co-GM Confirmation Modal */}
      <Modal
        isOpen={showDemoteConfirm}
        onClose={() => setShowDemoteConfirm(false)}
        title="Demote from Co-GM?"
      >
        <div className="space-y-4">
          <p className="text-sm text-content-secondary">
            <strong>{participant?.username}</strong> will be demoted from co-GM to audience member.
            They will lose GM permissions but remain in the game.
          </p>

          <div className="flex justify-end gap-3">
            <Button
              variant="secondary"
              onClick={() => setShowDemoteConfirm(false)}
              disabled={demoteFromCoGM.isPending}
            >
              Cancel
            </Button>
            <Button
              variant="danger"
              onClick={handleDemote}
              loading={demoteFromCoGM.isPending}
            >
              Demote to Audience
            </Button>
          </div>

          {demoteFromCoGM.isError && (
            <Alert variant="danger" dismissible onDismiss={() => demoteFromCoGM.reset()}>
              <p className="text-sm">
                Failed to demote from co-GM. {(demoteFromCoGM.error as Error)?.message || 'Please try again.'}
              </p>
            </Alert>
          )}
        </div>
      </Modal>

      {/* Move to Audience Confirmation Modal */}
      <Modal
        isOpen={showTransitionToAudienceConfirm}
        onClose={() => { setShowTransitionToAudienceConfirm(false); setTransitionConfirmText(''); }}
        title="Transition Player Out of Game?"
      >
        <div className="space-y-4">
          <Alert variant="warning" title="This action cannot be reversed">
            <p className="text-sm">
              <strong>{participant?.username}</strong> will be moved to the Former Players section.
            </p>
            <ul className="text-sm space-y-1 list-disc list-inside mt-2">
              <li>Their character(s) will remain active (not deactivated)</li>
              <li>They can still view their private messages and character history</li>
            </ul>
            <p className="text-sm mt-2">
              Note: this does not disable posting in common rooms (to allow for meta threads / epilogue
              threads) or sending private messages, so make sure the player is aware of expectations
              around being an audience member.
            </p>
            <p className="text-sm mt-2 font-medium">
              This will place them in the "Former Players" section, not Audience. To add a spectator
              who was never a player, use "Add Audience Member" instead.
            </p>
          </Alert>

          <Input
            label='Type "confirm" to proceed'
            value={transitionConfirmText}
            onChange={(e) => setTransitionConfirmText(e.target.value)}
            placeholder="confirm"
          />

          <div className="flex justify-end gap-3">
            <Button
              variant="secondary"
              onClick={() => { setShowTransitionToAudienceConfirm(false); setTransitionConfirmText(''); }}
              disabled={transitionToAudience.isPending}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              onClick={handleTransitionToAudience}
              loading={transitionToAudience.isPending}
              disabled={transitionConfirmText.toLowerCase() !== 'confirm'}
            >
              Move to Former Players
            </Button>
          </div>

          {transitionToAudience.isError && (
            <Alert variant="danger" dismissible onDismiss={() => transitionToAudience.reset()}>
              <p className="text-sm">
                Failed to move player to Former Players. {(transitionToAudience.error as Error)?.message || 'Please try again.'}
              </p>
            </Alert>
          )}
        </div>
      </Modal>

      {/* Remove Player Confirmation Modal */}
      <Modal
        isOpen={showRemoveConfirm}
        onClose={() => setShowRemoveConfirm(false)}
        title="Remove Player from Game?"
      >
        <div className="space-y-4">
          <Alert variant="danger" title="Warning: This action has serious consequences">
            <ul className="text-sm space-y-1 list-disc list-inside">
              <li>Player <strong>{participant?.username}</strong> will be removed from the game</li>
              <li>They will lose all access to the game immediately</li>
              <li>Their character(s) will be marked as inactive</li>
              <li>You can reassign their characters to yourself or other players</li>
              <li>This action can be reversed by adding them back</li>
            </ul>
          </Alert>

          <p className="text-sm text-content-secondary">
            Are you sure you want to remove this player?
          </p>

          <div className="flex justify-end gap-3">
            <Button
              variant="secondary"
              onClick={() => setShowRemoveConfirm(false)}
              disabled={removePlayer.isPending}
            >
              Cancel
            </Button>
            <Button
              variant="danger"
              onClick={handleRemove}
              loading={removePlayer.isPending}
            >
              Remove Player
            </Button>
          </div>

          {removePlayer.isError && (
            <Alert variant="danger" dismissible onDismiss={() => removePlayer.reset()}>
              <p className="text-sm">
                Failed to remove player. {(removePlayer.error as Error)?.message || 'Please try again.'}
              </p>
            </Alert>
          )}
        </div>
      </Modal>

      {/* Approve Application Confirmation Modal */}
      <Modal
        isOpen={showApproveConfirm}
        onClose={() => setShowApproveConfirm(false)}
        title="Approve Audience Application?"
      >
        <div className="space-y-4">
          <p className="text-sm text-content-secondary">
            Approve <strong>{application?.username}</strong> to join as an audience member?
          </p>

          {application?.message && (
            <div className="surface-raised border border-theme-default rounded-lg p-3">
              <p className="text-sm text-content-tertiary mb-1">Application message:</p>
              <p className="text-sm text-content-primary italic">"{application.message}"</p>
            </div>
          )}

          <div className="flex justify-end gap-3">
            <Button
              variant="secondary"
              onClick={() => setShowApproveConfirm(false)}
              disabled={isProcessing}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              onClick={handleApprove}
              loading={isProcessing}
            >
              Approve Application
            </Button>
          </div>

          {errorMessage && (
            <Alert variant="danger" dismissible onDismiss={() => setErrorMessage(null)}>
              <p className="text-sm">{errorMessage}</p>
            </Alert>
          )}
        </div>
      </Modal>

      {/* Reject Application Confirmation Modal */}
      <Modal
        isOpen={showRejectConfirm}
        onClose={() => setShowRejectConfirm(false)}
        title="Reject Audience Application?"
      >
        <div className="space-y-4">
          <p className="text-sm text-content-secondary">
            Reject <strong>{application?.username}</strong>'s application to join as an audience member?
            The application will be removed.
          </p>

          <div className="flex justify-end gap-3">
            <Button
              variant="secondary"
              onClick={() => setShowRejectConfirm(false)}
              disabled={isProcessing}
            >
              Cancel
            </Button>
            <Button
              variant="danger"
              onClick={handleReject}
              loading={isProcessing}
            >
              Reject Application
            </Button>
          </div>

          {errorMessage && (
            <Alert variant="danger" dismissible onDismiss={() => setErrorMessage(null)}>
              <p className="text-sm">{errorMessage}</p>
            </Alert>
          )}
        </div>
      </Modal>
    </>
  );
}
