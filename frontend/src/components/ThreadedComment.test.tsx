import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ThreadedComment } from './ThreadedComment';
import type { Message } from '../types/messages';
import { MemoryRouter } from 'react-router-dom';
import { useScreenshotMode } from '../hooks/useScreenshotMode';
import { useGamePermissions } from '../hooks/useGamePermissions';

// Mock heavy dependencies
vi.mock('../hooks/useAdminMode', () => ({
  useAdminMode: () => ({ adminModeEnabled: false, isAdmin: false }),
}));

vi.mock('../hooks/useScreenshotMode', () => ({
  useScreenshotMode: vi.fn(() => ({ screenshotModeEnabled: false, toggleScreenshotMode: vi.fn() })),
}));

vi.mock('../hooks/useGamePermissions', () => ({
  useGamePermissions: vi.fn(() => ({ isGM: false, isPlayer: true })),
}));

vi.mock('../hooks/useCommentMutations', () => ({
  useUpdateComment: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteComment: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock('./MarkdownPreview', () => ({
  MarkdownPreview: ({ content, fullWidth }: { content: string; fullWidth?: boolean }) => (
    <div data-testid="markdown-preview" data-full-width={fullWidth ? 'true' : 'false'}>
      {content}
    </div>
  ),
}));

vi.mock('./CommentEditor', () => ({
  CommentEditor: () => <div data-testid="comment-editor" />,
}));

vi.mock('./CharacterAvatar', () => ({
  default: ({ characterName }: { characterName: string }) => <div data-testid="character-avatar">{characterName}</div>,
}));

vi.mock('./ConfirmModal', () => ({
  ConfirmModal: () => null,
}));

vi.mock('../services/LoggingService', () => ({
  logger: { debug: vi.fn(), warn: vi.fn(), error: vi.fn() },
}));

vi.mock('../hooks/usePostCollapseState', () => ({
  usePostCollapseState: () => [false, vi.fn()],
}));

vi.mock('../contexts/ToastContext', () => ({
  useToast: () => ({ showSuccess: vi.fn(), showError: vi.fn() }),
  ToastProvider: ({ children }: { children: React.ReactNode }) => children,
}));

const baseComment: Message = {
  id: 42,
  game_id: 1,
  author_id: 99,
  character_id: 10,
  character_name: 'Test Hero',
  author_username: 'testuser',
  content: 'This is a test comment',
  message_type: 'comment',
  parent_id: 1,
  thread_depth: 1,
  visibility: 'game',
  is_edited: false,
  is_deleted: false,
  created_at: '2026-01-01T00:00:00Z',
  reply_count: 0,
};

function renderComment(props: Partial<Parameters<typeof ThreadedComment>[0]> = {}) {
  return render(
    <MemoryRouter>
      <ThreadedComment
        comment={baseComment}
        gameId={1}
        postId={1}
        characters={[]}
        controllableCharacters={[]}
        onCreateReply={vi.fn()}
        currentUserId={99}
        {...props}
      />
    </MemoryRouter>
  );
}

describe('ThreadedComment — manual read mode', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useScreenshotMode).mockReturnValue({ screenshotModeEnabled: false, toggleScreenshotMode: vi.fn() });
    vi.mocked(useGamePermissions).mockReturnValue({ isGM: false, isPlayer: true } as never);
  });

  describe('Screenshot Mode', () => {
    it('shows the author username when disabled', () => {
      renderComment();
      expect(screen.getAllByText('@testuser').length).toBeGreaterThan(0);
    });

    it('hides the author username when enabled', () => {
      vi.mocked(useScreenshotMode).mockReturnValue({ screenshotModeEnabled: true, toggleScreenshotMode: vi.fn() });
      renderComment();
      expect(screen.queryByText('@testuser')).not.toBeInTheDocument();
    });

    it('shows the "You" badge for the comment author when disabled', () => {
      renderComment();
      expect(screen.getAllByText('You').length).toBeGreaterThan(0);
    });

    it('hides the "You" badge for the comment author when enabled', () => {
      vi.mocked(useScreenshotMode).mockReturnValue({ screenshotModeEnabled: true, toggleScreenshotMode: vi.fn() });
      renderComment();
      expect(screen.queryByText('You')).not.toBeInTheDocument();
    });

    it('shows the Edit and Delete buttons for the comment author when disabled', () => {
      renderComment();
      expect(screen.getByRole('button', { name: /edit this comment/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /delete this comment/i })).toBeInTheDocument();
    });

    // Edit/Delete only render for the author, so leaving them visible identifies
    // which character the screenshotter plays just as plainly as the username.
    it('hides the Edit and Delete buttons when enabled', () => {
      vi.mocked(useScreenshotMode).mockReturnValue({ screenshotModeEnabled: true, toggleScreenshotMode: vi.fn() });
      renderComment();
      expect(screen.queryByRole('button', { name: /edit this comment/i })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /delete this comment/i })).not.toBeInTheDocument();
      // The comment itself still renders — only the authorship tells are hidden.
      expect(screen.getByTestId('markdown-preview')).toBeInTheDocument();
    });

    // Delete also renders for GMs/admins on other people's comments; a visible
    // Delete button would reveal that the screenshotter is the GM.
    it('shows the Delete button for a GM viewing another author\'s comment when disabled', () => {
      vi.mocked(useGamePermissions).mockReturnValue({ isGM: true, isPlayer: false } as never);
      renderComment({ currentUserId: 12345 });
      expect(screen.getByRole('button', { name: /delete this comment/i })).toBeInTheDocument();
    });

    it('hides the Delete button for a GM viewing another author\'s comment when enabled', () => {
      vi.mocked(useGamePermissions).mockReturnValue({ isGM: true, isPlayer: false } as never);
      vi.mocked(useScreenshotMode).mockReturnValue({ screenshotModeEnabled: true, toggleScreenshotMode: vi.fn() });
      renderComment({ currentUserId: 12345 });
      expect(screen.queryByRole('button', { name: /delete this comment/i })).not.toBeInTheDocument();
    });
  });

  describe('comment body width', () => {
    it('renders MarkdownPreview with fullWidth so text uses the available screen width instead of wrapping at a fixed max-width', () => {
      renderComment();
      const preview = screen.getByTestId('markdown-preview');
      expect(preview).toHaveAttribute('data-full-width', 'true');
    });
  });

  describe('auto mode (default)', () => {
    it('does not show the Mark as Read button', () => {
      renderComment({ commentReadMode: 'auto' });
      expect(screen.queryByTestId('toggle-read-button')).toBeNull();
    });

    it('does not apply opacity-50 to unread comments', () => {
      const { container } = renderComment({
        commentReadMode: 'auto',
        unreadCommentIDs: [42],
      });
      const wrapper = container.firstChild as HTMLElement;
      expect(wrapper.className).not.toContain('opacity-50');
    });
  });

  describe('manual mode — comment NOT marked as read', () => {
    it('shows a "Read" button', () => {
      renderComment({
        commentReadMode: 'manual',
        manualReadCommentIDs: [],
        onToggleRead: vi.fn(),
      });
      const btn = screen.getByTestId('toggle-read-button');
      expect(btn).toBeInTheDocument();
      expect(btn).toHaveAttribute('aria-label', 'Mark as read');
    });

    it('does not apply opacity-50 fading', () => {
      const { container } = renderComment({
        commentReadMode: 'manual',
        manualReadCommentIDs: [],
      });
      const wrapper = container.firstChild as HTMLElement;
      expect(wrapper.className).not.toContain('opacity-50');
    });

    it('calls onToggleRead with (commentId, false) when Read button is clicked', async () => {
      const user = userEvent.setup();
      const onToggleRead = vi.fn();
      renderComment({
        commentReadMode: 'manual',
        manualReadCommentIDs: [],
        onToggleRead,
      });
      await user.click(screen.getByTestId('toggle-read-button'));
      expect(onToggleRead).toHaveBeenCalledWith(42, false);
    });
  });

  describe('manual mode — comment IS marked as read', () => {
    it('shows an "Unread" button', () => {
      renderComment({
        commentReadMode: 'manual',
        manualReadCommentIDs: [42],
        onToggleRead: vi.fn(),
      });
      const btn = screen.getByTestId('toggle-read-button');
      expect(btn).toHaveAttribute('aria-label', 'Mark as unread');
    });

    it('applies opacity-50 fading to comment content but not child replies', () => {
      const { container } = renderComment({
        commentReadMode: 'manual',
        manualReadCommentIDs: [42],
      });
      const outerWrapper = container.firstChild as HTMLElement;
      // Outer wrapper should NOT have opacity-50 (so child replies are not affected)
      expect(outerWrapper.className).not.toContain('opacity-50');
      // Inner content div should have opacity-50 (fades only this comment's content)
      const innerContent = outerWrapper.firstChild as HTMLElement;
      expect(innerContent.className).toContain('opacity-50');
    });

    it('calls onToggleRead with (commentId, true) when Unread button is clicked', async () => {
      const user = userEvent.setup();
      const onToggleRead = vi.fn();
      renderComment({
        commentReadMode: 'manual',
        manualReadCommentIDs: [42],
        onToggleRead,
      });
      await user.click(screen.getByTestId('toggle-read-button'));
      expect(onToggleRead).toHaveBeenCalledWith(42, true);
    });
  });

  describe('manual mode — allowReadTracking', () => {
    it('hides the Mark as Read button when allowReadTracking=false', () => {
      renderComment({
        commentReadMode: 'manual',
        manualReadCommentIDs: [],
        allowReadTracking: false,
      });
      expect(screen.queryByTestId('toggle-read-button')).toBeNull();
    });

    it('shows the Mark as Read button when readOnly=true but allowReadTracking=true', () => {
      renderComment({
        commentReadMode: 'manual',
        manualReadCommentIDs: [],
        readOnly: true,
        allowReadTracking: true,
      });
      expect(screen.getByTestId('toggle-read-button')).toBeInTheDocument();
    });
  });

  describe('manual mode — deleted comment', () => {
    it('hides the Mark as Read button for deleted comments', () => {
      renderComment({
        comment: { ...baseComment, is_deleted: true },
        commentReadMode: 'manual',
        manualReadCommentIDs: [],
      });
      expect(screen.queryByTestId('toggle-read-button')).toBeNull();
    });
  });
});
