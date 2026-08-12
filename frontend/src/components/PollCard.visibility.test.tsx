import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { PollCard } from './PollCard';
import type { Poll } from '../types/polls';

const usePollResultsMock = vi.fn(() => ({
  data: undefined,
  isLoading: false,
  isPending: false,
  error: null,
}));

vi.mock('../hooks', () => ({
  usePoll: vi.fn(() => ({
    data: undefined,
    isLoading: false,
    isPending: false,
    error: null,
  })),
  usePollResults: (...args: unknown[]) => usePollResultsMock(...(args as [])),
  useUserCharacters: vi.fn(() => ({
    characters: [],
    isLoading: false,
    isPending: false,
    error: null,
  })),
  usePolls: vi.fn(() => ({
    data: [],
    isLoading: false,
    isPending: false,
    error: null,
    deletePollMutation: {
      mutateAsync: vi.fn(),
      isPending: false,
      isError: false,
      error: null,
    },
  })),
}));

const basePoll: Poll = {
  id: 1,
  game_id: 100,
  question: 'Should we storm the keep?',
  description: undefined,
  created_by_user_id: 1,
  deadline: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
  show_individual_votes: false,
  allow_other_option: false,
  hide_results_from_players: false,
  allow_audience_voting: false,
  show_running_totals_to_players: false,
  is_deleted: false,
  user_has_voted: false,
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString(),
};

const expired = (poll: Poll): Poll => ({
  ...poll,
  deadline: new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString(),
});

describe('PollCard - hidden results', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    usePollResultsMock.mockClear();
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
  });

  const renderCard = (ui: React.ReactElement) =>
    render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);

  it('tells a player results are hidden instead of showing them', () => {
    const poll = expired({ ...basePoll, hide_results_from_players: true, user_has_voted: true });

    renderCard(<PollCard poll={poll} gameId={100} isGM={false} isAudience={false} />);

    expect(screen.getByTestId('poll-results-hidden-notice')).toHaveTextContent(/hidden the results/i);
    expect(screen.queryByRole('button', { name: /show results/i })).not.toBeInTheDocument();
  });

  it('does not request results a player is not allowed to see', () => {
    const poll = expired({ ...basePoll, hide_results_from_players: true, user_has_voted: true });

    renderCard(<PollCard poll={poll} gameId={100} isGM={false} isAudience={false} />);

    // Passing null keeps the query disabled — the backend would reject this fetch.
    expect(usePollResultsMock).toHaveBeenCalledWith(null);
  });

  it('still shows results to the GM on a hidden-results poll', () => {
    const poll = expired({ ...basePoll, hide_results_from_players: true });

    renderCard(<PollCard poll={poll} gameId={100} isGM={true} isAudience={false} />);

    expect(screen.queryByTestId('poll-results-hidden-notice')).not.toBeInTheDocument();
    expect(usePollResultsMock).toHaveBeenCalledWith(poll.id);
  });

  it('still shows results to the audience on a hidden-results poll', () => {
    const poll = expired({ ...basePoll, hide_results_from_players: true });

    renderCard(<PollCard poll={poll} gameId={100} isGM={false} isAudience={true} />);

    expect(screen.queryByTestId('poll-results-hidden-notice')).not.toBeInTheDocument();
    expect(usePollResultsMock).toHaveBeenCalledWith(poll.id);
  });

  it('leaves normal polls unchanged for players after the deadline', () => {
    const poll = expired({ ...basePoll, user_has_voted: true });

    renderCard(<PollCard poll={poll} gameId={100} isGM={false} isAudience={false} />);

    expect(screen.queryByTestId('poll-results-hidden-notice')).not.toBeInTheDocument();
    expect(usePollResultsMock).toHaveBeenCalledWith(poll.id);
  });
});

describe('PollCard - running totals', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    usePollResultsMock.mockClear();
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
  });

  const renderCard = (ui: React.ReactElement) =>
    render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);

  it('fetches results for a player on an open poll once they have voted', () => {
    const poll = { ...basePoll, show_running_totals_to_players: true, user_has_voted: true };

    renderCard(<PollCard poll={poll} gameId={100} isGM={false} isAudience={false} />);

    expect(usePollResultsMock).toHaveBeenCalledWith(poll.id);
  });

  it('does not fetch results for a player on an open poll without running totals', () => {
    const poll = { ...basePoll, show_running_totals_to_players: false, user_has_voted: true };

    renderCard(<PollCard poll={poll} gameId={100} isGM={false} isAudience={false} />);

    // The backend would reject this fetch, so the query stays disabled.
    expect(usePollResultsMock).toHaveBeenCalledWith(null);
  });

  it('offers a player a results toggle on an open running-totals poll', () => {
    const poll = { ...basePoll, show_running_totals_to_players: true };

    renderCard(<PollCard poll={poll} gameId={100} isGM={false} isAudience={false} />);

    expect(screen.getByRole('button', { name: /show results/i })).toBeInTheDocument();
  });

  it('offers no results toggle on an open poll without running totals', () => {
    const poll = { ...basePoll, show_running_totals_to_players: false };

    renderCard(<PollCard poll={poll} gameId={100} isGM={false} isAudience={false} />);

    expect(screen.queryByRole('button', { name: /show results/i })).not.toBeInTheDocument();
  });

  it('still lets the player vote while showing them the running totals', () => {
    const poll = { ...basePoll, show_running_totals_to_players: true };

    renderCard(<PollCard poll={poll} gameId={100} isGM={false} isAudience={false} />);

    expect(screen.getByRole('button', { name: /vote now/i })).toBeInTheDocument();
  });
});

describe('PollCard - audience voting', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    usePollResultsMock.mockClear();
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
  });

  const renderCard = (ui: React.ReactElement) =>
    render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);

  it('offers a vote button to audience when audience voting is enabled', () => {
    const poll = { ...basePoll, allow_audience_voting: true };

    renderCard(<PollCard poll={poll} gameId={100} isGM={false} isAudience={true} />);

    expect(screen.getByRole('button', { name: /vote now/i })).toBeInTheDocument();
  });

  it('does not offer a vote button to audience when audience voting is disabled', () => {
    const poll = { ...basePoll, allow_audience_voting: false };

    renderCard(<PollCard poll={poll} gameId={100} isGM={false} isAudience={true} />);

    expect(screen.queryByRole('button', { name: /vote now/i })).not.toBeInTheDocument();
  });

  it('never offers a vote button to the GM, even with audience voting enabled', () => {
    const poll = { ...basePoll, allow_audience_voting: true };

    renderCard(<PollCard poll={poll} gameId={100} isGM={true} isAudience={false} />);

    expect(screen.queryByRole('button', { name: /vote now/i })).not.toBeInTheDocument();
  });

  it('lets an audience member who voted change their vote', () => {
    const poll = { ...basePoll, allow_audience_voting: true, user_has_voted: true };

    renderCard(<PollCard poll={poll} gameId={100} isGM={false} isAudience={true} />);

    expect(screen.getByRole('button', { name: /change vote/i })).toBeInTheDocument();
  });

  // A completed or cancelled game is an immutable archive. The backend's
  // DeletePoll handler authorizes on GM alone and never checks game state, so
  // this button is the only thing standing between a GM and deleting a poll
  // out of a finished game's history.
  describe('delete button and game state', () => {
    it('offers the GM a delete button in a live game', () => {
      renderCard(
        <PollCard poll={basePoll} gameId={100} isGM={true} isAudience={false} gameState="in_progress" />
      );

      expect(screen.getByRole('button', { name: /delete poll/i })).toBeInTheDocument();
    });

    it('withholds delete once the game is completed', () => {
      renderCard(
        <PollCard poll={basePoll} gameId={100} isGM={true} isAudience={false} gameState="completed" />
      );

      expect(screen.queryByRole('button', { name: /delete poll/i })).not.toBeInTheDocument();
    });

    it('withholds delete once the game is cancelled', () => {
      renderCard(
        <PollCard poll={basePoll} gameId={100} isGM={true} isAudience={false} gameState="cancelled" />
      );

      expect(screen.queryByRole('button', { name: /delete poll/i })).not.toBeInTheDocument();
    });

    it('never offers delete to a non-GM', () => {
      renderCard(
        <PollCard poll={basePoll} gameId={100} isGM={false} isAudience={false} gameState="in_progress" />
      );

      expect(screen.queryByRole('button', { name: /delete poll/i })).not.toBeInTheDocument();
    });
  });
});
