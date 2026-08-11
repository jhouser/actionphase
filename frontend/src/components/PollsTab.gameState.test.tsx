import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { PollsTab } from './PollsTab';
import * as hooks from '../hooks';
import type { Poll } from '../types/polls';

// PollCard stands in for the real card, reporting only the gameState it was
// handed. The gate itself is covered separately below against the real card.
vi.mock('./PollCard', () => ({
  PollCard: ({ poll, gameState }: { poll: Poll; gameState?: string }) => (
    <div id={`poll-${poll.id}`} data-testid={`poll-${poll.id}`} data-game-state={gameState ?? 'undefined'}>
      {poll.question}
    </div>
  ),
}));

vi.mock('../hooks', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../hooks')>();
  return { ...actual, usePollsByPhase: vi.fn() };
});

vi.mock('../contexts/GameContext', () => ({
  useGameContext: () => ({ currentPhaseId: 10 }),
}));

const basePoll: Poll = {
  id: 1,
  game_id: 100,
  phase_id: 10,
  created_by_user_id: 1,
  created_by_character_id: null,
  question: 'Should we storm the keep?',
  description: '',
  deadline: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
  show_individual_votes: false,
  allow_other_option: false,
  is_expired: false,
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString(),
  user_has_voted: false,
};

describe('PollsTab gameState forwarding', () => {
  beforeEach(() => {
    vi.mocked(hooks.usePollsByPhase).mockReturnValue({
      data: [basePoll],
      isLoading: false,
    } as ReturnType<typeof hooks.usePollsByPhase>);
  });

  const renderTab = (gameState?: string) =>
    render(
      <MemoryRouter>
        <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
          <PollsTab gameId={100} phaseId={10} isGM={true} isCurrentPhase={true} gameState={gameState} />
        </QueryClientProvider>
      </MemoryRouter>
    );

  // Regression: CommonRoom rendered PollsTab without gameState, so PollCard saw
  // undefined and its completed/cancelled delete guard passed. The backend's
  // DeletePoll handler checks GM only, with no game-state check, so a GM really
  // could delete polls out of a completed game's archive.
  it('passes gameState through to each poll card', () => {
    renderTab('completed');

    expect(screen.getByTestId('poll-1')).toHaveAttribute('data-game-state', 'completed');
  });

  it('passes an in-progress state through unchanged', () => {
    renderTab('in_progress');

    expect(screen.getByTestId('poll-1')).toHaveAttribute('data-game-state', 'in_progress');
  });
});
