import { useEffect, useRef, useState } from 'react';
import { apiClient } from '../lib/api';
import { Button, Alert } from './ui';
import { GameFormFields } from './GameFormFields';
import { useReportDirty } from '../hooks/useReportDirty';
import type { GameFormTabId } from './gameFormTabs';
import { useRevealInvalidTab } from '../hooks/useRevealInvalidTab';
import { HelpTooltip } from './ui/HelpTooltip';
import { useGameForm } from '../hooks/useGameForm';
import { useActiveCommunities } from '../hooks/useCommunities';
import { useGameFormDirty } from '../hooks/useGameFormDirty';
import { ConfirmDiscardEdits } from './ConfirmDiscardEdits';

interface CreateGameFormProps {
  onSuccess?: (gameId: number) => void;
  onCancel?: () => void;
  /**
   * Reports whether the form holds unsaved edits, so the containing modal can
   * withdraw its backdrop dismissal. The form owns the comparison because only
   * it holds the field state; the modal owns the backdrop.
   */
  onDirtyChange?: (isDirty: boolean) => void;
  /**
   * Set when the containing modal's close affordance (its header X) was used.
   * The form, not the modal, raises the confirmation — it is the thing holding
   * the unsaved edits and the thing that can show the prompt inline.
   */
  closeRequested?: boolean;
  /** Acknowledges a handled close request so it does not re-fire. */
  onCloseRequestHandled?: () => void;
}

export const CreateGameForm = ({
  onSuccess,
  onCancel,
  onDirtyChange,
  closeRequested = false,
  onCloseRequestHandled,
}: CreateGameFormProps) => {
  const bannerInputRef = useRef<HTMLInputElement>(null);
  const [activeTab, setActiveTab] = useState<GameFormTabId>('game-form-info');
  const handleInvalid = useRevealInvalidTab(setActiveTab);
  const {
    formData,
    initialFormData,
    handleChange,
    error,
    setError,
    loading,
    setLoading,
    pendingBannerFile,
    bannerPreviewUrl,
    handleBannerFileSelect,
    discardPendingBanner,
    uploadPendingBanner,
    uploadBanner,
    buildApiPayload,
    resetFormData,
  } = useGameForm();

  const { communities } = useActiveCommunities();

  // Preselect when there is exactly one choice: the picker would otherwise be
  // a required field with a single option, which is a step, not a decision.
  // Guarded on the field still being empty so it never overrides the GM.
  //
  // resetFormData, NOT handleChange: this moves the unsaved-edit baseline along
  // with the value. A default the form filled in for itself is not an edit, and
  // with handleChange an untouched form would prompt "discard your changes?" on
  // close.
  useEffect(() => {
    if (communities.length === 1 && formData.community_id === '') {
      resetFormData({ ...formData, community_id: communities[0].id });
    }
  }, [communities, formData, resetFormData]);

  const isDirty = useGameFormDirty(formData, initialFormData, pendingBannerFile);
  const [confirmingClose, setConfirmingClose] = useState(false);
  useReportDirty(isDirty, onDirtyChange);

  // Drop the prompt if the edits it asks about disappear underneath it.
  useEffect(() => {
    if (!isDirty) setConfirmingClose(false);
  }, [isDirty]);

  // A close requested from the modal header raises the same inline prompt the
  // Cancel button does, so both routes out ask the same question.
  useEffect(() => {
    if (closeRequested) {
      setConfirmingClose(true);
      onCloseRequestHandled?.();
    }
  }, [closeRequested, onCloseRequestHandled]);

  const requestCancel = () => {
    if (isDirty) {
      setConfirmingClose(true);
      return;
    }
    onCancel?.();
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    const { payload, error: validationError } = buildApiPayload(true);
    if (!payload) {
      setError(validationError);
      return;
    }
    // buildApiPayload(true) already refused a missing community, but the shared
    // return type cannot express that. Narrow rather than cast, so a future
    // change that drops the check fails here instead of posting without one.
    if (payload.community_id === undefined) {
      setError('Please choose a community for this game');
      return;
    }
    const createPayload = { ...payload, community_id: payload.community_id };

    setLoading(true);
    try {
      const response = await apiClient.games.createGame(createPayload);
      const gameId = response.data.id;

      if (pendingBannerFile) {
        uploadPendingBanner(gameId, {
          onSuccess: () => onSuccess?.(gameId),
          onError: () => onSuccess?.(gameId),
        });
      } else {
        onSuccess?.(gameId);
      }
    } catch (err: unknown) {
      const axiosErr = err as { response?: { data?: { error?: string } }; message?: string };
      const errorMessage =
        axiosErr?.response?.data?.error ||
        (axiosErr?.message && axiosErr.message !== 'Network Error'
          ? axiosErr.message
          : 'Failed to create game');
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  const bannerUpload = (
    <div className="space-y-2">
      <div className="flex items-center gap-1">
        <label className="block text-sm font-medium text-content-secondary">
          Game Banner <span className="text-content-secondary font-normal">(optional)</span>
        </label>
        <HelpTooltip text="A wide horizontal image shown at the top of your game page. Best at 6:1 aspect ratio (e.g. 1200×200px) — images will be cropped to fit." />
      </div>

      {bannerPreviewUrl && (
        <div className="w-full rounded overflow-hidden" style={{ aspectRatio: '6/1' }}>
          <img src={bannerPreviewUrl} alt="Game banner preview" className="w-full h-full object-cover" />
        </div>
      )}

      {bannerPreviewUrl ? (
        <div className="flex gap-2">
          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={discardPendingBanner}
          >
            Choose different
          </Button>
        </div>
      ) : (
        <div className="flex gap-2">
          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={() => bannerInputRef.current?.click()}
          >
            Upload Banner
          </Button>
        </div>
      )}

      <input
        ref={bannerInputRef}
        type="file"
        accept="image/jpeg,image/png,image/webp"
        className="hidden"
        onChange={(e) => {
          const file = e.target.files?.[0];
          if (file) handleBannerFileSelect(file);
          e.target.value = '';
        }}
      />
      {uploadBanner.isError && (
        <p className="text-sm text-semantic-danger">Failed to upload banner. Please try again.</p>
      )}
      {bannerPreviewUrl && (
        <p className="text-xs text-content-secondary">
          The banner will upload after the game is created.
        </p>
      )}
    </div>
  );

  return (
    // No max-width here: the containing Modal owns the width, and this wrapper's
    // former `max-w-2xl mx-auto` made Create render 159px narrower than Edit for
    // the same form.
    <div>
      {error && (
        <Alert variant="danger" className="mb-6" dismissible onDismiss={() => setError(null)} data-testid="error-message">
          {error}
        </Alert>
      )}

      <form onSubmit={handleSubmit} onInvalid={handleInvalid} className="space-y-6">
        <GameFormFields
          formData={formData}
          communities={communities}
          communityRequired
          onChange={handleChange}
          bannerUpload={bannerUpload}
          activeTab={activeTab}
          onTabChange={setActiveTab}
        />

        <Alert variant="info" title="Game Creation Process">
          <ul className="text-sm space-y-1 list-disc list-inside">
            <li>Your game will start in "Setup" mode after creation</li>
            <li>Switch to "Recruitment" when ready to accept players</li>
            <li>Players can join until the recruitment deadline</li>
            <li>Move to "Character Creation" once recruitment is complete</li>
          </ul>
        </Alert>

        {error && (
          <Alert variant="danger">
            {error}
          </Alert>
        )}

        {confirmingClose ? (
          <div className="flex justify-end pt-4">
            <ConfirmDiscardEdits
              onDiscard={() => onCancel?.()}
              onKeepEditing={() => setConfirmingClose(false)}
            />
          </div>
        ) : (
          <div className="flex gap-4 pt-4">
            <Button
              type="submit"
              variant="primary"
              loading={loading}
              className="flex-1"
              data-testid="create-game-submit"
            >
              {loading ? 'Creating Game...' : 'Create Game'}
            </Button>
            {onCancel && (
              <Button
                type="button"
                variant="secondary"
                onClick={requestCancel}
                className="px-6"
              >
                Cancel
              </Button>
            )}
          </div>
        )}
      </form>
    </div>
  );
};
