import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, useSearchParams } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { PollsTab } from './PollsTab';
import * as hooks from '../hooks';
import type { Poll } from '../types/polls';

// Render a stand-in for PollCard carrying the same anchor id the real card
// exposes, so the scroll target is exercised without PollCard's dependencies.
vi.mock('./PollCard', () => ({
  PollCard: ({ poll }: { poll: Poll }) => (
    <div id={`poll-${poll.id}`}>{poll.question}</div>
  ),
}));

vi.mock('../hooks', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../hooks')>();
  return { ...actual, usePollsByPhase: vi.fn() };
});

vi.mock('../contexts/GameContext', () => ({
  useGameContext: () => ({ currentPhaseId: 10 }),
}));

function makePoll(overrides: Partial<Poll> = {}): Poll {
  return {
    id: 1,
    game_id: 1,
    phase_id: 10,
    created_by_user_id: 1,
    created_by_character_id: null,
    question: 'What should we do next?',
    description: '',
    deadline: '2026-12-31T23:59:59Z',
    show_individual_votes: false,
    allow_other_option: false,
    is_expired: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    user_has_voted: false,
    ...overrides,
  };
}

/** Surfaces the live search params so tests can assert the deep-link is cleared. */
function SearchParamsProbe() {
  const [params] = useSearchParams();
  return <div data-testid="search-params">{params.toString()}</div>;
}

function renderPollsTab(initialUrl: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <MemoryRouter initialEntries={[initialUrl]}>
      <QueryClientProvider client={queryClient}>
        <PollsTab gameId={1} phaseId={10} isGM={false} isCurrentPhase={true} />
        <SearchParamsProbe />
      </QueryClientProvider>
    </MemoryRouter>
  );
}

describe('PollsTab deep-link scrolling', () => {
  let scrollSpy: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    scrollSpy = vi.fn();
    Element.prototype.scrollIntoView = scrollSpy;
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('scrolls to and highlights the poll named in the URL', async () => {
    vi.mocked(hooks.usePollsByPhase).mockReturnValue({
      data: [makePoll({ id: 1 }), makePoll({ id: 42, question: 'Who leads?' })],
      isLoading: false,
    } as ReturnType<typeof hooks.usePollsByPhase>);

    renderPollsTab('/?tab=common-room&view=polls&poll=42');

    await waitFor(() => expect(scrollSpy).toHaveBeenCalled());

    // The scroll must land on poll 42, not merely happen.
    const scrolledElement = scrollSpy.mock.instances[0] as HTMLElement;
    expect(scrolledElement.id).toBe('poll-42');
    expect(scrolledElement).toHaveClass('ring-2');
  });

  it('clears the poll param so a re-render does not yank the page again', async () => {
    vi.mocked(hooks.usePollsByPhase).mockReturnValue({
      data: [makePoll({ id: 42 })],
      isLoading: false,
    } as ReturnType<typeof hooks.usePollsByPhase>);

    renderPollsTab('/?tab=common-room&view=polls&poll=42');

    await waitFor(() => {
      expect(screen.getByTestId('search-params').textContent).not.toContain('poll=42');
    });
    // The surrounding tab context must survive the cleanup.
    expect(screen.getByTestId('search-params').textContent).toContain('view=polls');
  });

  it('reveals the expired section when the target poll is expired', async () => {
    vi.mocked(hooks.usePollsByPhase).mockReturnValue({
      data: [makePoll({ id: 42, question: 'Old vote', is_expired: true })],
      isLoading: false,
    } as ReturnType<typeof hooks.usePollsByPhase>);

    renderPollsTab('/?tab=common-room&view=polls&poll=42');

    // Expired polls are collapsed by default; the deep-link must open them.
    await waitFor(() => expect(screen.getByText('Old vote')).toBeInTheDocument());
    expect(screen.getByLabelText(/Show expired polls/)).toBeChecked();
    await waitFor(() => expect(scrollSpy).toHaveBeenCalled());
  });

  it('does not scroll when no poll is deep-linked', async () => {
    vi.mocked(hooks.usePollsByPhase).mockReturnValue({
      data: [makePoll({ id: 42 })],
      isLoading: false,
    } as ReturnType<typeof hooks.usePollsByPhase>);

    renderPollsTab('/?tab=common-room&view=polls');

    await waitFor(() => expect(screen.getByText('What should we do next?')).toBeInTheDocument());
    expect(scrollSpy).not.toHaveBeenCalled();
  });

  it('leaves the expired toggle alone when the user has not deep-linked', async () => {
    const user = userEvent.setup();
    vi.mocked(hooks.usePollsByPhase).mockReturnValue({
      data: [makePoll({ id: 1 }), makePoll({ id: 42, question: 'Old vote', is_expired: true })],
      isLoading: false,
    } as ReturnType<typeof hooks.usePollsByPhase>);

    renderPollsTab('/?tab=common-room&view=polls');

    const toggle = screen.getByLabelText(/Show expired polls/);
    expect(toggle).not.toBeChecked();
    await user.click(toggle);
    expect(screen.getByText('Old vote')).toBeInTheDocument();
  });
});
