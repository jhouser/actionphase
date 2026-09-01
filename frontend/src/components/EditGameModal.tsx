import { useEffect, useRef, useState } from 'react';
import { isAxiosError } from 'axios';
import type { GameWithDetails, UpdateGameRequest } from '../types/games';
import { apiClient } from '../lib/api';
import { Button, Alert } from './ui';
import { Modal } from './Modal';
import { GameFormFields } from './GameFormFields';
import { useSelectableCommunities } from '../hooks/useCommunities';
import type { GameFormTabId } from './gameFormTabs';
import { useRevealInvalidTab } from '../hooks/useRevealInvalidTab';
import { HelpTooltip } from './ui/HelpTooltip';
import { useGameForm, gameToFormData } from '../hooks/useGameForm';
import { useGameFormDirty } from '../hooks/useGameFormDirty';
import { ConfirmDiscardEdits } from './ConfirmDiscardEdits';

interface EditGameModalProps {
  game: GameWithDetails;
  isOpen: boolean;
  onClose: () => void;
  onGameUpdated: () => void;
}

export function EditGameModal({ game, isOpen, onClose, onGameUpdated }: EditGameModalProps) {
  const bannerInputRef = useRef<HTMLInputElement>(null);
  const [activeTab, setActiveTab] = useState<GameFormTabId>('game-form-info');
  const handleInvalid = useRevealInvalidTab(setActiveTab);
  const {
    formData,
    initialFormData,
    resetFormData,
    handleChange,
    error,
    setError,
    loading,
    setLoading,
    pendingBannerFile,
    bannerPreviewUrl,
    handleBannerFileSelect,
    discardPendingBanner,
    uploadBanner,
    deleteBanner,
    buildApiPayload,
  } = useGameForm(game);

  // Reassignment is setup-only (decision 4). Past setup the game has recruited
  // under one community's rules and banlist, so the picker is not offered --
  // the server refuses the change regardless; this keeps the form honest about
  // what it can do.
  const canChangeCommunity = game?.state === 'setup';
  const { communities } = useSelectableCommunities();

  useEffect(() => {
    if (isOpen && game) {
      // resetFormData, not setFormData: this moves the unsaved-edit baseline
      // along with the contents, so reopening on fresh server data does not
      // look like the GM edited every changed field.
      resetFormData(gameToFormData(game));
      setError(null);
    }
  }, [isOpen, game, resetFormData, setError]);

  const isDirty = useGameFormDirty(formData, initialFormData, pendingBannerFile);
  const [confirmingClose, setConfirmingClose] = useState(false);

  // Drop the prompt if the edits it asks about disappear underneath it.
  useEffect(() => {
    if (!isDirty) setConfirmingClose(false);
  }, [isDirty]);

  const requestClose = () => {
    if (isDirty) {
      setConfirmingClose(true);
      return;
    }
    onClose();
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    const { payload, error: validationError } = buildApiPayload();
    if (!payload) {
      setError(validationError);
      return;
    }

    try {
      setLoading(true);
      const updateData: UpdateGameRequest = {
        ...payload,
        is_public: true,
      };
      await apiClient.games.updateGame(game.id, updateData);
      onGameUpdated();
      onClose();
    } catch (err) {
      if (isAxiosError(err) && err.response?.data?.error) {
        setError(err.response.data.error);
      } else {
        setError(err instanceof Error ? err.message : 'Failed to update game');
      }
    } finally {
      setLoading(false);
    }
  };

  const bannerUpload = (
    <div className="space-y-2">
      <div className="flex items-center gap-1">
        <label className="block text-sm font-medium text-content-secondary">Game Banner <span className="text-content-secondary font-normal">(optional)</span></label>
        <HelpTooltip text="A wide horizontal image shown at the top of your game page. Best at 6:1 aspect ratio (e.g. 1200×200px) — images will be cropped to fit." />
      </div>

      {(bannerPreviewUrl || game.banner_url) && (
        <div className="w-full rounded overflow-hidden" style={{ aspectRatio: '6/1' }}>
          <img
            src={bannerPreviewUrl ?? game.banner_url!}
            alt="Game banner"
            className="w-full h-full object-cover"
          />
        </div>
      )}

      {bannerPreviewUrl ? (
        <div className="flex gap-2">
          <Button
            type="button"
            variant="primary"
            size="sm"
            onClick={() => {
              if (pendingBannerFile) {
                uploadBanner.mutate({ gameId: game.id, file: pendingBannerFile }, {
                  onSuccess: () => {
                    discardPendingBanner();
                    onGameUpdated();
                  },
                  onError: () => {
                    discardPendingBanner();
                  },
                });
              }
            }}
            loading={uploadBanner.isPending}
          >
            Use this image
          </Button>
          <Button
            type="button"
            variant="secondary"
            size="sm"
            disabled={uploadBanner.isPending}
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
            disabled={deleteBanner.isPending}
          >
            {game.banner_url ? 'Replace Banner' : 'Upload Banner'}
          </Button>
          {game.banner_url && (
            <Button
              type="button"
              variant="danger"
              size="sm"
              onClick={() => deleteBanner.mutate(game.id, { onSuccess: onGameUpdated })}
              loading={deleteBanner.isPending}
            >
              Remove Banner
            </Button>
          )}
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
        <p className="text-sm text-red-600">Failed to upload banner. Please try again.</p>
      )}
      {deleteBanner.isError && (
        <p className="text-sm text-red-600">Failed to remove banner. Please try again.</p>
      )}
    </div>
  );

  return (
    // onClose drives the header X, so it goes through the guard too. The backdrop
    // is withdrawn entirely while dirty rather than routed through requestClose:
    // a stray click out is not a decision worth raising a prompt over — matching
    // UpdateCharacterSheetModal.
    <Modal
      isOpen={isOpen}
      onClose={requestClose}
      title="Edit Game"
      size="5xl"
      dismissOnBackdrop={!isDirty}
    >
      {error && (
        <Alert variant="danger" className="mb-4" dismissible onDismiss={() => setError(null)}>
          {error}
        </Alert>
      )}

      <form onSubmit={handleSubmit} onInvalid={handleInvalid} className="space-y-4">
        <GameFormFields
          formData={formData}
          onChange={handleChange}
          bannerUpload={bannerUpload}
          activeTab={activeTab}
          onTabChange={setActiveTab}
          communities={canChangeCommunity ? communities : undefined}
        />

        {error && (
          <Alert variant="danger">
            {error}
          </Alert>
        )}

        {confirmingClose ? (
          <div className="flex justify-end pt-4">
            <ConfirmDiscardEdits
              onDiscard={onClose}
              onKeepEditing={() => setConfirmingClose(false)}
            />
          </div>
        ) : (
          <div className="flex gap-3 pt-4">
            <Button
              type="submit"
              variant="primary"
              loading={loading}
              className="flex-1"
            >
              Save Changes
            </Button>
            <Button
              type="button"
              variant="secondary"
              onClick={requestClose}
              disabled={loading}
              className="flex-1"
            >
              Cancel
            </Button>
          </div>
        )}
      </form>
    </Modal>
  );
}
