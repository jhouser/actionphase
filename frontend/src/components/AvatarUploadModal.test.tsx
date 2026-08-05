import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import AvatarUploadModal from './AvatarUploadModal';
import { useUploadCharacterAvatar, useDeleteCharacterAvatar } from '../hooks/useCharacterAvatar';

// Mock the hooks
vi.mock('../hooks/useCharacterAvatar', () => ({
  useUploadCharacterAvatar: vi.fn(),
  useDeleteCharacterAvatar: vi.fn(),
}));

// Mock cropImage so tests don't need canvas support
vi.mock('../utils/cropImage', () => ({
  cropImage: vi.fn().mockResolvedValue(new Blob(['cropped'], { type: 'image/jpeg' })),
}));

// Mock react-easy-crop so tests don't depend on canvas/ResizeObserver.
// Call onCropComplete via useEffect to avoid infinite re-render loops.
vi.mock('react-easy-crop', () => ({
  default: function MockCropper({ onCropComplete }: { onCropComplete: (a: unknown, b: { x: number; y: number; width: number; height: number }) => void }) {
    React.useEffect(() => {
      onCropComplete({}, { x: 0, y: 0, width: 100, height: 100 });
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);
    return <div data-testid="cropper" />;
  },
}));

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
};

/** Helper: select a file and advance to the crop step */
const selectFile = async (fileInput: HTMLElement, file = new File(['test'], 'avatar.jpg', { type: 'image/jpeg' })) => {
  // FileReader.readAsDataURL is sync-mocked in jsdom; fire the change event
  fireEvent.change(fileInput, { target: { files: [file] } });
  // Wait for the crop step to appear
  await screen.findByTestId('cropper');
  return file;
};

describe('AvatarUploadModal', () => {
  const mockUploadMutate = vi.fn();
  const mockDeleteMutate = vi.fn();
  const mockOnClose = vi.fn();
  const mockOnUploadSuccess = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();

    vi.mocked(useUploadCharacterAvatar).mockReturnValue({
      mutate: mockUploadMutate,
      isPending: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useUploadCharacterAvatar>);

    vi.mocked(useDeleteCharacterAvatar).mockReturnValue({
      mutate: mockDeleteMutate,
      isPending: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useDeleteCharacterAvatar>);
  });

  it('does not render when isOpen is false', () => {
    const { container } = render(
      <AvatarUploadModal
        isOpen={false}
        onClose={mockOnClose}
        characterId={1}
        characterName="Test Character"
      />,
      { wrapper: createWrapper() }
    );

    expect(container.firstChild).toBeNull();
  });

  it('renders modal when isOpen is true', () => {
    render(
      <AvatarUploadModal
        isOpen={true}
        onClose={mockOnClose}
        characterId={1}
        characterName="Test Character"
      />,
      { wrapper: createWrapper() }
    );

    expect(screen.getByText(/Upload Avatar for Test Character/i)).toBeInTheDocument();
  });

  it('shows file input for selecting avatar', () => {
    render(
      <AvatarUploadModal
        isOpen={true}
        onClose={mockOnClose}
        characterId={1}
        characterName="Test Character"
      />,
      { wrapper: createWrapper() }
    );

    const fileInput = screen.getByLabelText(/choose file/i);
    expect(fileInput).toBeInTheDocument();
    expect(fileInput).toHaveAttribute('type', 'file');
    expect(fileInput).toHaveAttribute('accept', 'image/jpeg,image/png,image/webp');
  });

  it('advances to crop step when valid file is selected', async () => {
    render(
      <AvatarUploadModal
        isOpen={true}
        onClose={mockOnClose}
        characterId={1}
        characterName="Test Character"
      />,
      { wrapper: createWrapper() }
    );

    const fileInput = screen.getByLabelText(/choose file/i);
    await selectFile(fileInput);

    expect(screen.getByTestId('cropper')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /crop & upload/i })).toBeInTheDocument();
  });

  it('validates file type and shows error for invalid types', async () => {
    render(
      <AvatarUploadModal
        isOpen={true}
        onClose={mockOnClose}
        characterId={1}
        characterName="Test Character"
      />,
      { wrapper: createWrapper() }
    );

    const invalidFile = new File(['test'], 'document.pdf', { type: 'application/pdf' });
    const fileInput = screen.getByLabelText(/choose file/i);

    fireEvent.change(fileInput, { target: { files: [invalidFile] } });

    await waitFor(() => {
      expect(screen.getByText(/only jpg, png, and webp/i)).toBeInTheDocument();
    });
  });

  it('validates file size and shows error for large files', async () => {
    render(
      <AvatarUploadModal
        isOpen={true}
        onClose={mockOnClose}
        characterId={1}
        characterName="Test Character"
      />,
      { wrapper: createWrapper() }
    );

    const largeFile = new File([new ArrayBuffer(6 * 1024 * 1024)], 'large.jpg', {
      type: 'image/jpeg',
    });
    const fileInput = screen.getByLabelText(/choose file/i);

    fireEvent.change(fileInput, { target: { files: [largeFile] } });

    await waitFor(() => {
      expect(screen.getByText(/file size must be less than 5mb/i)).toBeInTheDocument();
    });
  });

  it('uploads cropped file when Crop & Upload button clicked', async () => {
    render(
      <AvatarUploadModal
        isOpen={true}
        onClose={mockOnClose}
        characterId={123}
        characterName="Test Character"
        onUploadSuccess={mockOnUploadSuccess}
      />,
      { wrapper: createWrapper() }
    );

    const fileInput = screen.getByLabelText(/choose file/i);
    await selectFile(fileInput);

    const uploadButton = screen.getByRole('button', { name: /crop & upload/i });
    fireEvent.click(uploadButton);

    await waitFor(() => {
      expect(mockUploadMutate).toHaveBeenCalledWith(
        { characterId: 123, file: expect.any(File) },
        expect.any(Object)
      );
    });
  });

  it('calls onUploadSuccess and closes modal on successful upload', async () => {
    const mockMutate = vi.fn((_, { onSuccess }) => {
      onSuccess({ data: { avatar_url: 'http://example.com/avatar.jpg' } });
    });

    vi.mocked(useUploadCharacterAvatar).mockReturnValue({
      mutate: mockMutate,
      isPending: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useUploadCharacterAvatar>);

    render(
      <AvatarUploadModal
        isOpen={true}
        onClose={mockOnClose}
        characterId={123}
        characterName="Test Character"
        onUploadSuccess={mockOnUploadSuccess}
      />,
      { wrapper: createWrapper() }
    );

    const fileInput = screen.getByLabelText(/choose file/i);
    await selectFile(fileInput);

    const uploadButton = screen.getByRole('button', { name: /crop & upload/i });
    fireEvent.click(uploadButton);

    await waitFor(() => {
      expect(mockOnUploadSuccess).toHaveBeenCalled();
      expect(mockOnClose).toHaveBeenCalled();
    });
  });

  it('shows delete button when current avatar exists', () => {
    render(
      <AvatarUploadModal
        isOpen={true}
        onClose={mockOnClose}
        characterId={1}
        characterName="Test Character"
        currentAvatarUrl="http://example.com/current.jpg"
      />,
      { wrapper: createWrapper() }
    );

    expect(screen.getByRole('button', { name: /remove avatar/i })).toBeInTheDocument();
  });

  it('does not show delete button when no current avatar', () => {
    render(
      <AvatarUploadModal
        isOpen={true}
        onClose={mockOnClose}
        characterId={1}
        characterName="Test Character"
        currentAvatarUrl={null}
      />,
      { wrapper: createWrapper() }
    );

    expect(screen.queryByRole('button', { name: /remove avatar/i })).not.toBeInTheDocument();
  });

  it('deletes avatar when delete button clicked with confirmation', async () => {
    const mockConfirm = vi.spyOn(window, 'confirm').mockReturnValue(true);

    render(
      <AvatarUploadModal
        isOpen={true}
        onClose={mockOnClose}
        characterId={456}
        characterName="Test Character"
        currentAvatarUrl="http://example.com/current.jpg"
      />,
      { wrapper: createWrapper() }
    );

    const deleteButton = screen.getByRole('button', { name: /remove avatar/i });
    fireEvent.click(deleteButton);

    // The prompt must say the removal is forward-looking only: past posts pin the
    // avatar they were written with and keep rendering it.
    expect(mockConfirm).toHaveBeenCalledWith(
      expect.stringContaining('Existing posts keep the avatar they were written with')
    );
    expect(mockDeleteMutate).toHaveBeenCalledWith(456, expect.any(Object));

    mockConfirm.mockRestore();
  });

  it('does not delete avatar when confirmation cancelled', async () => {
    const mockConfirm = vi.spyOn(window, 'confirm').mockReturnValue(false);

    render(
      <AvatarUploadModal
        isOpen={true}
        onClose={mockOnClose}
        characterId={456}
        characterName="Test Character"
        currentAvatarUrl="http://example.com/current.jpg"
      />,
      { wrapper: createWrapper() }
    );

    const deleteButton = screen.getByRole('button', { name: /remove avatar/i });
    fireEvent.click(deleteButton);

    expect(mockDeleteMutate).not.toHaveBeenCalled();

    mockConfirm.mockRestore();
  });

  it('closes modal when cancel button clicked', () => {
    render(
      <AvatarUploadModal
        isOpen={true}
        onClose={mockOnClose}
        characterId={1}
        characterName="Test Character"
      />,
      { wrapper: createWrapper() }
    );

    const cancelButton = screen.getByRole('button', { name: /cancel/i });
    fireEvent.click(cancelButton);

    expect(mockOnClose).toHaveBeenCalled();
  });

  it('back button in crop step returns to file select', async () => {
    render(
      <AvatarUploadModal
        isOpen={true}
        onClose={mockOnClose}
        characterId={1}
        characterName="Test Character"
      />,
      { wrapper: createWrapper() }
    );

    const fileInput = screen.getByLabelText(/choose file/i);
    await selectFile(fileInput);

    const backButton = screen.getByRole('button', { name: /back/i });
    fireEvent.click(backButton);

    expect(screen.getByLabelText(/choose file/i)).toBeInTheDocument();
    expect(screen.queryByTestId('cropper')).not.toBeInTheDocument();
  });

  it('disables Crop & Upload button during upload', async () => {
    vi.mocked(useUploadCharacterAvatar).mockReturnValue({
      mutate: mockUploadMutate,
      isPending: true,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useUploadCharacterAvatar>);

    render(
      <AvatarUploadModal
        isOpen={true}
        onClose={mockOnClose}
        characterId={1}
        characterName="Test Character"
      />,
      { wrapper: createWrapper() }
    );

    const fileInput = screen.getByLabelText(/choose file/i);
    await selectFile(fileInput);

    const uploadButton = screen.getByRole('button', { name: /uploading/i });
    expect(uploadButton).toBeDisabled();
  });
});
