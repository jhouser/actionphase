import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DeadlineCard } from './DeadlineCard';
import type { UnifiedDeadline } from '../types/deadlines';

describe('DeadlineCard', () => {
  const mockDeadline: UnifiedDeadline = {
    deadline_type: 'deadline',
    source_id: 1,
    title: 'Submit character sheets',
    description: 'Please submit your finalized character sheets including backstory.',
    deadline: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(), // 24 hours from now
    game_id: 1,
    is_system_deadline: false,
  };

  describe('Title Display', () => {
    it('displays full title when under 32 characters', () => {
      render(<DeadlineCard deadline={mockDeadline} isGM={false} />);
      expect(screen.getByText('Submit character sheets')).toBeInTheDocument();
    });

    it('truncates title at 32 characters', () => {
      const longTitle = 'This is a very long title that should be truncated after thirty-two characters';
      const deadline = { ...mockDeadline, title: longTitle };
      render(<DeadlineCard deadline={deadline} isGM={false} />);

      // Should show first 32 chars + "..."
      expect(screen.getByText('This is a very long title that s...')).toBeInTheDocument();
    });

    it('shows full title in tooltip via title attribute', () => {
      const longTitle = 'This is a very long title that should be truncated';
      const deadline = { ...mockDeadline, title: longTitle };
      const { container } = render(<DeadlineCard deadline={deadline} isGM={false} />);

      const titleElement = container.querySelector('[title]');
      expect(titleElement).toHaveAttribute('title', longTitle);
    });

    it('does not display emoji icon', () => {
      const { container } = render(<DeadlineCard deadline={mockDeadline} isGM={false} />);

      // Check that there's no large emoji (text-xl class)
      const emojiElements = container.querySelectorAll('.text-xl');
      expect(emojiElements.length).toBe(0);
    });
  });

  describe('Description Tooltip', () => {
    it('shows info icon when description exists', () => {
      render(<DeadlineCard deadline={mockDeadline} isGM={false} />);

      const infoIcon = screen.getByLabelText('View description');
      expect(infoIcon).toBeInTheDocument();
    });

    it('does not show info icon when description is empty', () => {
      const deadline = { ...mockDeadline, description: '' };
      render(<DeadlineCard deadline={deadline} isGM={false} />);

      const infoIcon = screen.queryByLabelText('View description');
      expect(infoIcon).not.toBeInTheDocument();
    });

    it('does not show info icon when description is only whitespace', () => {
      const deadline = { ...mockDeadline, description: '   ' };
      render(<DeadlineCard deadline={deadline} isGM={false} />);

      const infoIcon = screen.queryByLabelText('View description');
      expect(infoIcon).not.toBeInTheDocument();
    });

    it('does not show info icon when description is undefined', () => {
      const deadline = { ...mockDeadline, description: undefined as unknown as string };
      render(<DeadlineCard deadline={deadline} isGM={false} />);

      const infoIcon = screen.queryByLabelText('View description');
      expect(infoIcon).not.toBeInTheDocument();
    });

    it('shows description in tooltip on hover', async () => {
      const user = userEvent.setup();
      render(<DeadlineCard deadline={mockDeadline} isGM={false} />);

      const infoIcon = screen.getByLabelText('View description');

      // Hover over the info icon
      await user.hover(infoIcon);

      // Tooltip should appear with description
      expect(screen.getByText(/Please submit your finalized character sheets/)).toBeInTheDocument();
    });

    it('preserves line breaks in description', async () => {
      const user = userEvent.setup();
      const multilineDescription = 'Line 1\nLine 2\nLine 3';
      const deadline = { ...mockDeadline, description: multilineDescription };
      render(<DeadlineCard deadline={deadline} isGM={false} />);

      const infoIcon = screen.getByLabelText('View description');
      await user.hover(infoIcon);

      // Check that the whitespace-pre-wrap class is applied for line breaks
      const descriptionElement = screen.getByText(/Line 1/);
      expect(descriptionElement).toHaveClass('whitespace-pre-wrap');
    });
  });

  describe('Card Dimensions', () => {
    it('has correct width classes (220px max)', () => {
      const { container } = render(<DeadlineCard deadline={mockDeadline} isGM={false} />);

      const card = container.firstChild as HTMLElement;
      expect(card).toHaveClass('max-w-[220px]');
      expect(card).toHaveClass('min-w-[200px]');
    });
  });

  describe('Urgency Display', () => {
    it('shows critical urgency for deadlines < 1 hour away', () => {
      const soon = new Date(Date.now() + 30 * 60 * 1000).toISOString(); // 30 minutes
      const deadline = { ...mockDeadline, deadline: soon };
      const { container } = render(<DeadlineCard deadline={deadline} isGM={false} />);

      const card = container.firstChild as HTMLElement;
      expect(card).toHaveClass('border-semantic-danger');
    });

    it('shows warning urgency for deadlines 1-3 hours away', () => {
      const warning = new Date(Date.now() + 2 * 60 * 60 * 1000).toISOString(); // 2 hours
      const deadline = { ...mockDeadline, deadline: warning };
      const { container } = render(<DeadlineCard deadline={deadline} isGM={false} />);

      const card = container.firstChild as HTMLElement;
      expect(card).toHaveClass('border-semantic-warning');
    });

    it('shows normal urgency for deadlines > 3 hours away', () => {
      const normal = new Date(Date.now() + 6 * 60 * 60 * 1000).toISOString(); // 6 hours
      const deadline = { ...mockDeadline, deadline: normal };
      const { container } = render(<DeadlineCard deadline={deadline} isGM={false} />);

      const card = container.firstChild as HTMLElement;
      expect(card).toHaveClass('border-interactive-primary');
    });

    it('shows expired urgency for past deadlines', () => {
      const expired = new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString(); // 24 hours ago
      const deadline = { ...mockDeadline, deadline: expired };
      const { container } = render(<DeadlineCard deadline={deadline} isGM={false} />);

      const card = container.firstChild as HTMLElement;
      expect(card).toHaveClass('border-border-secondary');
    });
  });

  describe('GM Actions', () => {
    it('shows edit button on hover for GM', async () => {
      const user = userEvent.setup();
      const onEdit = vi.fn();
      const { container } = render(
        <DeadlineCard deadline={mockDeadline} isGM={true} onEdit={onEdit} />
      );

      const card = container.firstChild as HTMLElement;
      await user.hover(card);

      const editButton = screen.getByLabelText('Edit deadline');
      expect(editButton).toBeInTheDocument();
    });

    it('does not show GM actions for non-GM users', async () => {
      const user = userEvent.setup();
      const onEdit = vi.fn();
      const { container } = render(
        <DeadlineCard deadline={mockDeadline} isGM={false} onEdit={onEdit} />
      );

      const card = container.firstChild as HTMLElement;
      await user.hover(card);

      const editButton = screen.queryByLabelText('Edit deadline');
      expect(editButton).not.toBeInTheDocument();
    });

    it('calls onEdit when edit button clicked', async () => {
      const user = userEvent.setup();
      const onEdit = vi.fn();
      const { container } = render(
        <DeadlineCard deadline={mockDeadline} isGM={true} onEdit={onEdit} />
      );

      const card = container.firstChild as HTMLElement;
      await user.hover(card);

      // Use fireEvent instead of userEvent because stopPropagation affects userEvent
      const editButtons = screen.getAllByRole('button');
      const editButton = editButtons.find(btn => btn.getAttribute('aria-label') === 'Edit deadline');
      expect(editButton).toBeDefined();

      fireEvent.click(editButton!);

      expect(onEdit).toHaveBeenCalledTimes(1);
    });

    it('calls onDelete when delete button clicked', async () => {
      const user = userEvent.setup();
      const onDelete = vi.fn();
      const { container } = render(
        <DeadlineCard deadline={mockDeadline} isGM={true} onDelete={onDelete} />
      );

      const card = container.firstChild as HTMLElement;
      await user.hover(card);

      // Use fireEvent instead of userEvent because stopPropagation affects userEvent
      const deleteButtons = screen.getAllByRole('button');
      const deleteButton = deleteButtons.find(btn => btn.getAttribute('aria-label') === 'Delete deadline');
      expect(deleteButton).toBeDefined();

      fireEvent.click(deleteButton!);

      expect(onDelete).toHaveBeenCalledTimes(1);
    });
  });

  describe('Countdown Display', () => {
    it('displays countdown in hours and minutes format', () => {
      const soon = new Date(Date.now() + 5 * 60 * 60 * 1000 + 30 * 60 * 1000).toISOString(); // 5h 30m
      const deadline = { ...mockDeadline, deadline: soon };
      render(<DeadlineCard deadline={deadline} isGM={false} />);

      // Should show format like "5h 29m" or "5h 30m" (timing may vary by milliseconds)
      expect(screen.getByText(/5h (29|30)m/)).toBeInTheDocument();
    });

    it('displays "Expired" for past deadlines', () => {
      const expired = new Date(Date.now() - 1000).toISOString();
      const deadline = { ...mockDeadline, deadline: expired };
      render(<DeadlineCard deadline={deadline} isGM={false} />);

      expect(screen.getByText('Expired')).toBeInTheDocument();
    });
  });

  describe('Clickable Behavior', () => {
    it('calls onClick when card is clicked', async () => {
      const user = userEvent.setup();
      const onClick = vi.fn();
      render(
        <DeadlineCard deadline={mockDeadline} isGM={false} onClick={onClick} />
      );

      // The click target is a button overlaying the card (absolute inset-0),
      // not the container itself — jsdom does no layout, so the click must be
      // aimed at the button rather than the container's center.
      await user.click(screen.getByRole('button', { name: mockDeadline.title }));

      expect(onClick).toHaveBeenCalledTimes(1);
    });

    it('has cursor-pointer class when onClick is provided', () => {
      const onClick = vi.fn();
      const { container } = render(
        <DeadlineCard deadline={mockDeadline} isGM={false} onClick={onClick} />
      );

      const card = container.firstChild as HTMLElement;
      expect(card).toHaveClass('cursor-pointer');
    });

    it('does not have cursor-pointer class when onClick is not provided', () => {
      const { container } = render(
        <DeadlineCard deadline={mockDeadline} isGM={false} />
      );

      const card = container.firstChild as HTMLElement;
      expect(card).not.toHaveClass('cursor-pointer');
    });

    it('activates via the keyboard, so the card is not mouse-only', async () => {
      const user = userEvent.setup();
      const onClick = vi.fn();
      render(<DeadlineCard deadline={mockDeadline} isGM={false} onClick={onClick} />);

      await user.tab();
      expect(screen.getByRole('button', { name: mockDeadline.title })).toHaveFocus();

      await user.keyboard('{Enter}');
      expect(onClick).toHaveBeenCalledTimes(1);
    });
  });

  describe('GM actions alongside a clickable card', () => {
    // Regression: the GM buttons used to be nested inside a role="button" card.
    // A bubbled Space keypress hit the container's preventDefault + navigate,
    // so Space on Delete navigated instead of deleting.
    it('deletes rather than navigates when Space activates the delete button', async () => {
      const user = userEvent.setup();
      const onClick = vi.fn();
      const onDelete = vi.fn();
      const { container } = render(
        <DeadlineCard
          deadline={mockDeadline}
          isGM={true}
          onClick={onClick}
          onDelete={onDelete}
        />
      );

      await user.hover(container.firstChild as HTMLElement);

      const deleteButton = await screen.findByLabelText('Delete deadline');
      deleteButton.focus();
      await user.keyboard(' ');

      expect(onDelete).toHaveBeenCalledTimes(1);
      expect(onClick).not.toHaveBeenCalled();
    });

    // The GM actions are hover-gated, so without focus revealing them they are
    // mouse-only. Tabbing must walk the whole card: click target -> Edit -> Delete.
    it('reaches every GM action by keyboard alone', async () => {
      const user = userEvent.setup();
      render(
        <DeadlineCard
          deadline={mockDeadline}
          isGM={true}
          onClick={vi.fn()}
          onEdit={vi.fn()}
          onDelete={vi.fn()}
        />
      );

      await user.tab();
      expect(screen.getByRole('button', { name: mockDeadline.title })).toHaveFocus();

      await user.tab();
      expect(screen.getByLabelText('Edit deadline')).toHaveFocus();

      await user.tab();
      expect(screen.getByLabelText('Delete deadline')).toHaveFocus();
    });
  });
});
