import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DeadlineStrip } from './DeadlineStrip';
import type { UnifiedDeadline } from '../types/deadlines';

vi.mock('./CreateDeadlineModal', () => ({
  CreateDeadlineModal: () => <div data-testid="create-deadline-modal" />,
}));

vi.mock('./EditDeadlineModal', () => ({
  EditDeadlineModal: () => <div data-testid="edit-deadline-modal" />,
}));

const tomorrow = () => new Date(Date.now() + 86400000).toISOString();

const pollDeadline: UnifiedDeadline = {
  deadline_type: 'poll',
  source_id: 42,
  title: 'Who leads the expedition?',
  description: 'Poll voting deadline',
  deadline: tomorrow(),
  game_id: 1,
  phase_id: 3,
  poll_id: 42,
  is_system_deadline: false,
};

const arbitraryDeadline: UnifiedDeadline = {
  deadline_type: 'deadline',
  source_id: 7,
  title: 'Write your backstory',
  description: 'No particular place to go',
  deadline: tomorrow(),
  game_id: 1,
  is_system_deadline: false,
};

/** Mirrors the page: poll deadlines have a destination, arbitrary ones don't. */
const getDeadlineHref = (deadline: UnifiedDeadline) =>
  deadline.deadline_type === 'deadline' ? null : '?tab=common-room&view=polls&poll=42';

function renderStrip(props: Partial<React.ComponentProps<typeof DeadlineStrip>> = {}) {
  return render(
    <DeadlineStrip
      deadlines={[pollDeadline]}
      isGM={false}
      gameState="in_progress"
      onCreateDeadline={vi.fn()}
      onUpdateDeadline={vi.fn()}
      onDeleteDeadline={vi.fn()}
      onExtendDeadline={vi.fn()}
      {...props}
    />
  );
}

describe('DeadlineStrip navigation', () => {
  let onDeadlineClick: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    onDeadlineClick = vi.fn();
  });

  it('navigates to the deadline when a card with a destination is clicked', async () => {
    const user = userEvent.setup();
    renderStrip({ onDeadlineClick, getDeadlineHref });

    await user.click(screen.getByRole('button', { name: /Who leads the expedition\?/ }));

    expect(onDeadlineClick).toHaveBeenCalledTimes(1);
    expect(onDeadlineClick).toHaveBeenCalledWith(expect.objectContaining({ poll_id: 42 }));
  });

  it('navigates via the keyboard, so the card is not mouse-only', async () => {
    const user = userEvent.setup();
    renderStrip({ onDeadlineClick, getDeadlineHref });

    await user.tab();
    expect(screen.getByRole('button', { name: /Who leads the expedition\?/ })).toHaveFocus();

    await user.keyboard('{Enter}');
    expect(onDeadlineClick).toHaveBeenCalledWith(expect.objectContaining({ poll_id: 42 }));
  });

  it('leaves deadlines without a destination inert', async () => {
    const user = userEvent.setup();
    renderStrip({
      deadlines: [arbitraryDeadline],
      onDeadlineClick,
      getDeadlineHref,
    });

    // No button role means nothing invites a click in the first place.
    expect(screen.queryByRole('button', { name: /Write your backstory/ })).not.toBeInTheDocument();

    await user.click(screen.getByText('Write your backstory'));
    expect(onDeadlineClick).not.toHaveBeenCalled();
  });

  it('leaves every card inert when the page supplies no click handler', () => {
    renderStrip({ getDeadlineHref });

    expect(screen.queryByRole('button', { name: /Who leads the expedition\?/ })).not.toBeInTheDocument();
  });
});
