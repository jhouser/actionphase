import React, { useState, useRef, useCallback } from 'react';
import Cropper from 'react-easy-crop';
import type { Area, Point } from 'react-easy-crop';
import { useUploadCharacterAvatar, useDeleteCharacterAvatar } from '../hooks/useCharacterAvatar';
import CharacterAvatar from './CharacterAvatar';
import { Button, Alert } from './ui';
import { Modal } from './Modal';
import { cropImage } from '../utils/cropImage';

interface AvatarUploadModalProps {
  isOpen: boolean;
  onClose: () => void;
  characterId: number;
  characterName: string;
  currentAvatarUrl?: string | null;
  onUploadSuccess?: () => void;
  portraitMode?: boolean;
}

const MAX_FILE_SIZE = 5 * 1024 * 1024; // 5MB
const ALLOWED_TYPES = ['image/jpeg', 'image/png', 'image/webp'];

const AvatarUploadModal: React.FC<AvatarUploadModalProps> = ({
  isOpen,
  onClose,
  characterId,
  characterName,
  currentAvatarUrl,
  onUploadSuccess,
  portraitMode = false,
}) => {
  const cropAspect = portraitMode ? 2 / 3 : 1;

  const [step, setStep] = useState<'select' | 'crop'>('select');
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [imageSrc, setImageSrc] = useState<string | null>(null);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [crop, setCrop] = useState<Point>({ x: 0, y: 0 });
  const [zoom, setZoom] = useState(1);
  const [croppedAreaPixels, setCroppedAreaPixels] = useState<Area | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const uploadMutation = useUploadCharacterAvatar();
  const deleteMutation = useDeleteCharacterAvatar();

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    setValidationError(null);

    if (!file) {
      setSelectedFile(null);
      setImageSrc(null);
      return;
    }

    if (!ALLOWED_TYPES.includes(file.type)) {
      setValidationError('Only JPG, PNG, and WebP images are allowed');
      return;
    }

    if (file.size > MAX_FILE_SIZE) {
      setValidationError('File size must be less than 5MB');
      return;
    }

    const reader = new FileReader();
    reader.onloadend = () => {
      setImageSrc(reader.result as string);
      setSelectedFile(file);
      setCrop({ x: 0, y: 0 });
      setZoom(1);
      setStep('crop');
    };
    reader.readAsDataURL(file);
  };

  const onCropComplete = useCallback((_: Area, pixels: Area) => {
    setCroppedAreaPixels(pixels);
  }, []);

  const handleUpload = async () => {
    if (!imageSrc || !croppedAreaPixels) return;

    let blob: Blob;
    try {
      blob = await cropImage(imageSrc, croppedAreaPixels);
    } catch {
      setValidationError('Failed to process image. Please try again.');
      return;
    }

    const file = new File([blob], selectedFile?.name ?? 'avatar.jpg', { type: 'image/jpeg' });

    uploadMutation.mutate(
      { characterId, file },
      {
        onSuccess: () => {
          resetState();
          onUploadSuccess?.();
          onClose();
        },
      }
    );
  };

  const handleDelete = () => {
    // eslint-disable-next-line no-alert
    if (!confirm('Remove this avatar? The character will show initials from now on. Existing posts keep the avatar they were written with.')) return;

    deleteMutation.mutate(characterId, {
      onSuccess: () => {
        onUploadSuccess?.();
        onClose();
      },
    });
  };

  const handleBack = () => {
    setStep('select');
    setImageSrc(null);
    setSelectedFile(null);
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  const resetState = () => {
    setStep('select');
    setSelectedFile(null);
    setImageSrc(null);
    setValidationError(null);
    setCrop({ x: 0, y: 0 });
    setZoom(1);
    setCroppedAreaPixels(null);
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  const handleClose = () => {
    resetState();
    onClose();
  };

  const isUploading = uploadMutation.isPending;
  const isDeleting = deleteMutation.isPending;
  const hasError = uploadMutation.isError || deleteMutation.isError;

  return (
    <Modal isOpen={isOpen} onClose={handleClose} title={`Upload Avatar for ${characterName}`}>
      <div>
        {step === 'select' && (
          <>
            {/* Current Avatar */}
            {currentAvatarUrl && (
              <div className="mb-4">
                <p className="text-sm text-content-secondary mb-2">Current Avatar:</p>
                <div className="flex items-center gap-3">
                  <CharacterAvatar
                    avatarUrl={currentAvatarUrl}
                    characterName={characterName}
                    size="lg"
                  />
                  <Button
                    variant="danger"
                    size="sm"
                    onClick={handleDelete}
                    disabled={isDeleting || isUploading}
                  >
                    {isDeleting ? 'Removing...' : 'Remove Avatar'}
                  </Button>
                </div>
              </div>
            )}

            {/* File Input */}
            <div className="mb-4">
              <label
                htmlFor="avatar-file-input"
                className="block text-sm font-medium text-content-primary mb-2"
              >
                Choose File
              </label>
              <input
                id="avatar-file-input"
                ref={fileInputRef}
                type="file"
                accept="image/jpeg,image/png,image/webp"
                onChange={handleFileChange}
                disabled={isUploading || isDeleting}
                className="block w-full text-sm text-content-tertiary
                  file:mr-4 file:py-2 file:px-4
                  file:rounded file:border-0
                  file:text-sm file:font-semibold
                  file:bg-interactive-primary-subtle file:text-interactive-primary
                  hover:file:bg-interactive-primary-subtle
                  disabled:opacity-50 disabled:cursor-not-allowed"
              />
              <p className="mt-1 text-xs text-content-tertiary">
                JPG, PNG, or WebP. Max 5MB.
              </p>
            </div>

            {validationError && (
              <Alert variant="danger" className="mb-4">
                {validationError}
              </Alert>
            )}

            {hasError && (
              <Alert variant="danger" className="mb-4">
                {uploadMutation.error?.message || deleteMutation.error?.message || 'An error occurred'}
              </Alert>
            )}
          </>
        )}

        {step === 'crop' && imageSrc && (
          <>
            <div className="relative w-full mb-4" style={{ height: 300 }}>
              <Cropper
                image={imageSrc}
                crop={crop}
                zoom={zoom}
                aspect={cropAspect}
                onCropChange={setCrop}
                onZoomChange={setZoom}
                onCropComplete={onCropComplete}
              />
            </div>

            <div className="mb-4">
              <label className="block text-sm font-medium text-content-primary mb-1">
                Zoom
              </label>
              <input
                type="range"
                min={1}
                max={3}
                step={0.01}
                value={zoom}
                onChange={(e) => setZoom(Number(e.target.value))}
                className="w-full accent-interactive-primary"
              />
            </div>

            {hasError && (
              <Alert variant="danger" className="mb-4">
                {uploadMutation.error?.message || 'An error occurred'}
              </Alert>
            )}
          </>
        )}
      </div>

      {/* Footer */}
      <div className="flex justify-end gap-3 mt-6 pt-4 border-t border-theme-default">
        {step === 'select' ? (
          <Button variant="ghost" onClick={handleClose} disabled={isUploading || isDeleting}>
            Cancel
          </Button>
        ) : (
          <>
            <Button variant="ghost" onClick={handleBack} disabled={isUploading}>
              Back
            </Button>
            <Button
              variant="primary"
              onClick={handleUpload}
              disabled={!croppedAreaPixels || isUploading}
            >
              {isUploading ? 'Uploading...' : 'Crop & Upload'}
            </Button>
          </>
        )}
      </div>
    </Modal>
  );
};

export default AvatarUploadModal;
