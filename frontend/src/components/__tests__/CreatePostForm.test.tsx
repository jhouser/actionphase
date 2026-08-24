import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test-utils/render';
import { CreatePostForm } from '../CreatePostForm';
import type { Character } from '../../types/characters';
import { postCachingService } from '../../services/PostCachingService';

const mockCharacters: Character[] = [
  {
    id: 1,
    game_id: 1,
    name: 'GM Character',
    character_type: 'gm_character',
    user_id: 100,
    status: 'approved',
    created_at: '2024-01-01T00:00:00Z',
  },
  {
    id: 2,
    game_id: 1,
    name: 'Second GM Character',
    character_type: 'gm_character',
    user_id: 100,
    status: 'approved',
    created_at: '2024-01-01T00:00:00Z',
  }
];

describe('CreatePostForm', () => {
  const mockOnSubmit = vi.fn();

  beforeEach(() => {
    localStorage.clear();
    mockOnSubmit.mockClear();
  });

  describe('Empty State', () => {
    it('shows message when no characters available', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      expect(screen.getByText(/you need a character to post/i)).toBeInTheDocument();
    });

    it('shows instruction to create character first', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      expect(screen.getByText(/please create a character first/i)).toBeInTheDocument();
    });

    it('does not show form when no characters available', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    });
  });

  describe('Rendering', () => {
    it('renders form heading', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      expect(screen.getByText(/create new gm post/i)).toBeInTheDocument();
    });

    it('renders markdown tip', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      expect(screen.getByText(/you can use markdown formatting/i)).toBeInTheDocument();
    });

    it('renders content textarea', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      expect(screen.getByLabelText(/post content/i)).toBeInTheDocument();
    });

    it('renders submit button', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      expect(screen.getByRole('button', { name: /create gm post/i })).toBeInTheDocument();
    });

    it('shows character count', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      expect(screen.getByText(/0 characters \(longer posts will be collapsible for players\)/i)).toBeInTheDocument();
    });

    it('has placeholder text in textarea', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const textarea = screen.getByLabelText(/post content/i);
      expect(textarea).toHaveAttribute('placeholder');
    });
  });

  describe('Character Selection', () => {
    it('auto-selects first character when only one available', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      // With one character, selector should not be visible
      expect(screen.queryByLabelText(/post as/i)).not.toBeInTheDocument();
    });

    it('shows character selector when multiple characters available', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={mockCharacters}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      expect(screen.getByLabelText(/post as/i)).toBeInTheDocument();
    });

    it('displays all available characters in selector', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={mockCharacters}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const selector = screen.getByLabelText(/post as/i);
      expect(selector).toHaveTextContent('GM Character');
      expect(selector).toHaveTextContent('Second GM Character');
    });

    it('allows changing selected character', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={mockCharacters}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const selector = screen.getByLabelText(/post as/i) as HTMLSelectElement;

      await user.selectOptions(selector, '2');

      expect(selector.value).toBe('2');
    });

    it('auto-selects first character in multi-character scenario', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={mockCharacters}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const selector = screen.getByLabelText(/post as/i) as HTMLSelectElement;
      expect(selector.value).toBe('1');
    });
  });

  describe('Form Input', () => {
    it('updates content when user types', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const textarea = screen.getByLabelText(/post content/i);
      await user.type(textarea, 'Test post content');

      expect(textarea).toHaveValue('Test post content');
    });

    it('updates character count as user types', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const textarea = screen.getByLabelText(/post content/i);
      await user.type(textarea, 'Hello');

      // The Textarea component shows dynamic character count (may appear in multiple places)
      const characterCounts = screen.getAllByText(/5.*characters/i);
      expect(characterCounts.length).toBeGreaterThan(0);
    });

    it('allows markdown formatting in content', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const textarea = screen.getByLabelText(/post content/i);
      const markdownContent = '# Heading\n\n**Bold text**';
      await user.type(textarea, markdownContent);

      expect(textarea).toHaveValue(markdownContent);
    });
    
    it('saves post to localstorage cache with the proper tag', async () => {
      const user = userEvent.setup();
      const phaseId = 1;

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          phaseId={phaseId}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const autosaveId = postCachingService.createAutosaveId('cr-main-post', phaseId);
      
      await user.type(screen.getByLabelText(/post content/i), 'Hello there');

      expect(localStorage.getItem(autosaveId)).toBeDefined();
      expect(localStorage.getItem(autosaveId)).toContain('Hello there');
    });
  });

  describe('Validation', () => {
    it('disables submit button when content is empty', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const submitButton = screen.getByRole('button', { name: /create gm post/i });
      expect(submitButton).toBeDisabled();
    });

    it('enables submit button when content is provided', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const textarea = screen.getByLabelText(/post content/i);
      await user.type(textarea, 'Test content');

      const submitButton = screen.getByRole('button', { name: /create gm post/i });
      expect(submitButton).not.toBeDisabled();
    });

    it('disables submit button when content is only whitespace', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const textarea = screen.getByLabelText(/post content/i);
      await user.type(textarea, '   ');

      const submitButton = screen.getByRole('button', { name: /create gm post/i });
      expect(submitButton).toBeDisabled();
    });

    it('shows error when submitting without content', async () => {
      const _user = userEvent.setup();

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const form = screen.getByRole('textbox').closest('form')!;

      // Manually trigger submit (bypassing disabled button). fireEvent wraps the
      // dispatch in act(), which a raw form.dispatchEvent does not — that leaves
      // the resulting state update outside act and triggers a React warning.
      fireEvent.submit(form);

      await waitFor(() => {
        expect(screen.queryByText(/please enter a message/i)).toBeInTheDocument();
      });
    });
  });

  describe('Form Submission', () => {
    it('calls onSubmit with character ID and content', async () => {
      const user = userEvent.setup();
      mockOnSubmit.mockResolvedValue(undefined);

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const textarea = screen.getByLabelText(/post content/i);
      await user.type(textarea, 'Test post content');

      const submitButton = screen.getByRole('button', { name: /create gm post/i });
      await user.click(submitButton);

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith(1, 'Test post content');
      });
    });

    it('trims whitespace from content before submitting', async () => {
      const user = userEvent.setup();
      mockOnSubmit.mockResolvedValue(undefined);

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const textarea = screen.getByLabelText(/post content/i);
      await user.type(textarea, '  Test content  ');

      const submitButton = screen.getByRole('button', { name: /create gm post/i });
      await user.click(submitButton);

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith(1, 'Test content');
      });
    });

    it('clears content after successful submission', async () => {
      const user = userEvent.setup();
      mockOnSubmit.mockResolvedValue(undefined);

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const textarea = screen.getByLabelText(/post content/i);
      await user.type(textarea, 'Test content');

      const submitButton = screen.getByRole('button', { name: /create gm post/i });
      await user.click(submitButton);

      await waitFor(() => {
        expect(textarea).toHaveValue('');
      });
    });

    it('submits with selected character when multiple available', async () => {
      const user = userEvent.setup();
      mockOnSubmit.mockResolvedValue(undefined);

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={mockCharacters}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const selector = screen.getByLabelText(/post as/i);
      await user.selectOptions(selector, '2');

      const textarea = screen.getByLabelText(/post content/i);
      await user.type(textarea, 'Test content');

      const submitButton = screen.getByRole('button', { name: /create gm post/i });
      await user.click(submitButton);

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith(2, 'Test content');
      });
    });

    
    it('clears localstorage cache after submission', async () => {
      const user = userEvent.setup();
      const phaseId = 1;

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          phaseId={phaseId}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const autosaveId = postCachingService.createAutosaveId('cr-main-post', phaseId);
      
      await user.type(screen.getByLabelText(/post content/i), 'Hello there');

      expect(localStorage.getItem(autosaveId)).toBeDefined();

      const submitButton = screen.getByRole('button', { name: /create gm post/i });
      await user.click(submitButton);

      expect(localStorage.getItem(autosaveId)).toBeNull();
    });
  });

  describe('Error Handling', () => {
    it('displays error message when submission fails', async () => {
      const user = userEvent.setup();
      mockOnSubmit.mockRejectedValue(new Error('Failed to create post'));

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const textarea = screen.getByLabelText(/post content/i);
      await user.type(textarea, 'Test content');

      const submitButton = screen.getByRole('button', { name: /create gm post/i });
      await user.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText(/failed to create post/i)).toBeInTheDocument();
      });
    });

    it('does not clear content when submission fails', async () => {
      const user = userEvent.setup();
      mockOnSubmit.mockRejectedValue(new Error('Network error'));

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const textarea = screen.getByLabelText(/post content/i);
      await user.type(textarea, 'Test content');

      const submitButton = screen.getByRole('button', { name: /create gm post/i });
      await user.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText(/failed to create post/i)).toBeInTheDocument();
      });

      expect(textarea).toHaveValue('Test content');
    });

    it('shows custom error message from Error object', async () => {
      const user = userEvent.setup();
      mockOnSubmit.mockRejectedValue(new Error('Custom error message'));

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const textarea = screen.getByLabelText(/post content/i);
      await user.type(textarea, 'Test content');

      const submitButton = screen.getByRole('button', { name: /create gm post/i });
      await user.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText(/custom error message/i)).toBeInTheDocument();
      });
    });

    it('clears previous error on successful submission', async () => {
      const user = userEvent.setup();
      mockOnSubmit
        .mockRejectedValueOnce(new Error('First error'))
        .mockResolvedValueOnce(undefined);

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      const textarea = screen.getByLabelText(/post content/i);
      await user.type(textarea, 'Test content');

      const submitButton = screen.getByRole('button', { name: /create gm post/i });
      await user.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText(/first error/i)).toBeInTheDocument();
      });

      // Try again
      await user.type(textarea, 'New content');
      await user.click(submitButton);

      await waitFor(() => {
        expect(screen.queryByText(/first error/i)).not.toBeInTheDocument();
      });
    });
  });

  describe('Loading State', () => {
    it('shows loading text in button when submitting', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={true}
        />
      );

      expect(screen.getByRole('button', { name: /creating gm post/i })).toBeInTheDocument();
    });

    it('disables submit button when submitting', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={true}
        />
      );

      const submitButton = screen.getByRole('button', { name: /creating gm post/i });
      expect(submitButton).toBeDisabled();
    });

    it('disables textarea when submitting', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={true}
        />
      );

      const textarea = screen.getByLabelText(/post content/i);
      expect(textarea).toBeDisabled();
    });

    it('disables character selector when submitting', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={mockCharacters}
          onSubmit={mockOnSubmit}
          isSubmitting={true}
        />
      );

      const selector = screen.getByLabelText(/post as/i);
      expect(selector).toBeDisabled();
    });
  });

  describe('Collapse/Expand Behavior', () => {
    it('starts expanded when shouldStartCollapsed is false', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
          shouldStartCollapsed={false}
        />
      );

      // Form should be visible
      expect(screen.getByLabelText(/post content/i)).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /create gm post/i })).toBeInTheDocument();
    });

    it('starts collapsed when shouldStartCollapsed is true', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
          shouldStartCollapsed={true}
        />
      );

      // Form should be collapsed - no textarea
      expect(screen.queryByLabelText(/post content/i)).not.toBeInTheDocument();

      // Collapsed button should be visible
      expect(screen.getByRole('button', { name: /create new gm post/i })).toBeInTheDocument();
    });

    it('expands form when clicking collapsed button', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
          shouldStartCollapsed={true}
        />
      );

      // Click expand button
      const expandButton = screen.getByRole('button', { name: /create new gm post/i });
      await user.click(expandButton);

      // Form should now be visible
      expect(screen.getByLabelText(/post content/i)).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /create gm post/i })).toBeInTheDocument();
    });

    it('collapses form when clicking collapse button', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
          shouldStartCollapsed={false}
        />
      );

      // Click collapse button
      const collapseButton = screen.getByRole('button', { name: /collapse form/i });
      await user.click(collapseButton);

      // Form should now be collapsed
      expect(screen.queryByLabelText(/post content/i)).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: /create new gm post/i })).toBeInTheDocument();
    });

    it('preserves content when collapsing and expanding', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
          shouldStartCollapsed={false}
        />
      );

      // Type some content
      const textarea = screen.getByLabelText(/post content/i);
      await user.type(textarea, 'Test content');

      // Collapse
      const collapseButton = screen.getByRole('button', { name: /collapse form/i });
      await user.click(collapseButton);

      // Expand
      const expandButton = screen.getByRole('button', { name: /create new gm post/i });
      await user.click(expandButton);

      // Content should be preserved
      const newTextarea = screen.getByLabelText(/post content/i);
      expect(newTextarea).toHaveValue('Test content');
    });

    it('defaults to expanded when shouldStartCollapsed is not provided', () => {
      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={[mockCharacters[0]]}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      // Form should be visible by default
      expect(screen.getByLabelText(/post content/i)).toBeInTheDocument();
    });
  });

  describe('Integration', () => {
    it('handles complete post creation workflow', async () => {
      const user = userEvent.setup();
      mockOnSubmit.mockResolvedValue(undefined);

      renderWithProviders(
        <CreatePostForm
          gameId={1}
          characters={mockCharacters}
          onSubmit={mockOnSubmit}
          isSubmitting={false}
        />
      );

      // Select character
      const selector = screen.getByLabelText(/post as/i);
      await user.selectOptions(selector, '2');

      // Type content
      const textarea = screen.getByLabelText(/post content/i);
      await user.type(textarea, '# Important Update\n\nThis is a test post');

      // Verify character count updates (Textarea shows dynamic count, may appear in multiple places)
      const characterCounts = screen.getAllByText(/39.*characters/i);
      expect(characterCounts.length).toBeGreaterThan(0);

      // Submit
      const submitButton = screen.getByRole('button', { name: /create gm post/i });
      await user.click(submitButton);

      // Verify submission
      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith(2, '# Important Update\n\nThis is a test post');
      });

      // Verify content cleared
      expect(textarea).toHaveValue('');
    });
  });
});
