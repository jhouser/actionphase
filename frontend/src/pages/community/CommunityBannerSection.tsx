import { useEffect, useRef, useState } from 'react';
import { Button, HelpTooltip } from '../../components/ui';
import { useToast } from '../../contexts/ToastContext';
import { useCommunityBanner } from '../../hooks/useCommunities';
import type { Community } from '../../types/communities';

interface CommunityBannerSectionProps {
  community: Community;
}

/**
 * A community's banner image.
 *
 * Follows the game banner in EditGameModal: choose a file, preview it locally,
 * then confirm. The preview matters because the server crops to 6:1 -- an
 * upload-on-select flow would show the crop only after the old banner was
 * already gone.
 *
 * Two deliberate departures from that component:
 *
 *  - Errors go to a toast, matching the rest of this tab, rather than inline
 *    red text. A second error idiom in one panel is worse than a slightly
 *    different one from the game form.
 *  - The upload fires on confirm rather than being staged for a later save.
 *    The game version stages because the CREATE form has no game id yet; a
 *    community always exists by the time this renders.
 *
 * Rendered only for viewers who may moderate -- SettingsTab returns a
 * read-only notice before reaching this.
 */
export function CommunityBannerSection({ community }: CommunityBannerSectionProps) {
  const { showSuccess, showError } = useToast();
  const { upload, remove } = useCommunityBanner(community.slug);

  const fileInputRef = useRef<HTMLInputElement>(null);
  const [pendingFile, setPendingFile] = useState<File | null>(null);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);

  // Releases the object URL when it is replaced and when the component
  // unmounts. Without this each preview leaks a blob for the lifetime of the
  // page -- invisible in the UI, so only a test can catch it.
  //
  // This effect is the ONLY revoke. Revoking inline in the handlers as well
  // would be dead code: the cleanup runs on every previewUrl change, so the
  // inline call could be deleted with no test failing and no behaviour change.
  useEffect(() => {
    if (!previewUrl) return;
    return () => URL.revokeObjectURL(previewUrl);
  }, [previewUrl]);

  const selectFile = (file: File) => {
    setPreviewUrl(URL.createObjectURL(file));
    setPendingFile(file);
  };

  const discardPending = () => {
    setPreviewUrl(null);
    setPendingFile(null);
  };

  const errorDetail = (err: unknown, fallback: string) =>
    (err as { response?: { data?: { error?: string } } })?.response?.data?.error ?? fallback;

  const handleConfirm = () => {
    if (!pendingFile) return;
    upload.mutate(pendingFile, {
      onSuccess: () => {
        discardPending();
        showSuccess('Banner updated');
      },
      // The pending file is kept on failure so the moderator can retry the same
      // image without picking it again.
      onError: (err: unknown) =>
        showError(errorDetail(err, 'Could not upload that image')),
    });
  };

  const handleRemove = () => {
    remove.mutate(undefined, {
      onSuccess: () => showSuccess('Banner removed'),
      onError: (err: unknown) => showError(errorDetail(err, 'Could not remove the banner')),
    });
  };

  const busy = upload.isPending || remove.isPending;
  const shownImage = previewUrl ?? community.banner_url;

  return (
    <div className="space-y-2" data-testid="community-banner-section">
      <div className="flex items-center gap-1">
        <label className="block text-sm font-medium text-content-secondary">
          Banner <span className="text-content-secondary font-normal">(optional)</span>
        </label>
        <HelpTooltip text="A wide image shown at the top of this community's page. Best at 6:1 aspect ratio (e.g. 1200×200px); images are cropped to fit. JPG, PNG or WebP, up to 5MB." />
      </div>

      {shownImage && (
        <div className="w-full rounded overflow-hidden" style={{ aspectRatio: '6/1' }}>
          <img
            src={shownImage}
            alt={previewUrl ? 'Banner preview' : `${community.name} banner`}
            className="w-full h-full object-cover"
            data-testid="community-banner-preview"
          />
        </div>
      )}

      {previewUrl ? (
        <div className="flex gap-2">
          <Button
            type="button"
            variant="primary"
            size="sm"
            onClick={handleConfirm}
            loading={upload.isPending}
            data-testid="community-banner-confirm"
          >
            Use this image
          </Button>
          <Button
            type="button"
            variant="secondary"
            size="sm"
            disabled={upload.isPending}
            onClick={discardPending}
            data-testid="community-banner-discard"
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
            onClick={() => fileInputRef.current?.click()}
            disabled={busy}
            data-testid="community-banner-choose"
          >
            {community.banner_url ? 'Replace banner' : 'Upload banner'}
          </Button>
          {community.banner_url && (
            <Button
              type="button"
              variant="danger"
              size="sm"
              onClick={handleRemove}
              loading={remove.isPending}
              data-testid="community-banner-remove"
            >
              Remove banner
            </Button>
          )}
        </div>
      )}

      {/* accept mirrors the server's allowlist, so the file picker cannot offer
          a type the upload would reject. It is a convenience, not the check --
          the server validates the declared MIME regardless. */}
      <input
        ref={fileInputRef}
        type="file"
        accept="image/jpeg,image/png,image/webp"
        className="hidden"
        data-testid="community-banner-input"
        onChange={(e) => {
          const file = e.target.files?.[0];
          if (file) selectFile(file);
          // Cleared so re-picking the SAME file fires change again.
          e.target.value = '';
        }}
      />
    </div>
  );
}
