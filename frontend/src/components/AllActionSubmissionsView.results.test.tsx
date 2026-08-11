import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AllActionSubmissionsView } from './AllActionSubmissionsView';

const getGameResults = vi.fn();

vi.mock('../lib/api', () => ({
  apiClient: {
    phases: {
      getGamePhases: () =>
        Promise.resolve({
          data: [
            { id: 10, phase_type: 'action', phase_number: 1, title: 'Phase One', is_active: true },
          ],
        }),
      // The component unwraps `res.data`, so the mock must return the
      // axios-shaped envelope rather than a bare array.
      getGameResults: () => Promise.resolve({ data: getGameResults() }),
    },
  },
}));

const mockSubmissions = vi.fn();

vi.mock('../hooks/useAudience', () => ({
  useAllActionSubmissions: () => mockSubmissions(),
}));

vi.mock('../contexts/GameContext', () => ({
  useGameContext: () => ({ allGameCharacters: [], game: { portrait_avatars: false } }),
}));

vi.mock('../hooks/useCharacterSheetItems', () => ({
  useCharacterSheetItems: () => undefined,
}));

vi.mock('./MarkdownPreview', () => ({
  MarkdownPreview: ({ content }: { content: string }) => <div>{content}</div>,
}));

const submission = {
  id: 500,
  status: 'result_posted',
  action_result_id: 900,
  character_id: 1,
  character_name: 'Vera',
  username: 'vera_player',
  submission_number: 1,
  created_at: new Date().toISOString(),
  submitted_at: new Date().toISOString(),
  last_updated: new Date().toISOString(),
  content: 'I search the ruins.',
};

const setSubmissions = (items: unknown[]) =>
  mockSubmissions.mockReturnValue({
    data: { pages: [{ action_submissions: items, total: items.length }] },
    fetchNextPage: vi.fn(),
    hasNextPage: false,
    isFetchingNextPage: false,
    isLoading: false,
    error: null,
  });

const renderView = () =>
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <AllActionSubmissionsView gameId={100} />
    </QueryClientProvider>
  );

describe('AllActionSubmissionsView result rendering', () => {
  beforeEach(() => {
    getGameResults.mockReset();
    // Default to an empty result set so the query always resolves; individual
    // tests override. Returning undefined leaves the query unsettled, which
    // makes absence-assertions pass vacuously.
    getGameResults.mockReturnValue([]);
    mockSubmissions.mockReset();
    setSubmissions([submission]);
  });

  // A GM can answer one submission with several results (a staged reveal), so
  // the view must render every match rather than only the first.
  it('renders every result belonging to a submission', async () => {
    getGameResults.mockReturnValue([
      { id: 900, action_submission_id: 500, phase_id: 10, content: 'You find a locked door.', is_published: true },
      { id: 901, action_submission_id: 500, phase_id: 10, content: 'Behind it, a stairwell.', is_published: true },
    ]);

    renderView();
    await userEvent.click(await screen.findByRole('button', { name: /Vera/ }));

    expect(await screen.findByText('You find a locked door.')).toBeInTheDocument();
    expect(screen.getByText('Behind it, a stairwell.')).toBeInTheDocument();
    expect(screen.getByText('Results:')).toBeInTheDocument();
  });

  it('labels a lone result in the singular', async () => {
    getGameResults.mockReturnValue([
      { id: 900, action_submission_id: 500, phase_id: 10, content: 'You find nothing.', is_published: true },
    ]);

    renderView();
    await userEvent.click(await screen.findByRole('button', { name: /Vera/ }));

    expect(await screen.findByText('You find nothing.')).toBeInTheDocument();
    expect(screen.getByText('Result:')).toBeInTheDocument();
  });

  // Results created without a submission (actions collected offsite) have no
  // card to nest under and were previously invisible in this view entirely.
  it('lists published results that have no parent submission', async () => {
    getGameResults.mockReturnValue([
      {
        id: 950,
        action_submission_id: undefined,
        phase_id: 10,
        content: 'A raven arrives bearing news.',
        character_name: 'Cass',
        is_published: true,
      },
    ]);

    renderView();

    expect(await screen.findByText('Results without a submission')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: /Cass/ }));
    expect(await screen.findByText('A raven arrives bearing news.')).toBeInTheDocument();
  });

  it('does not leak unpublished standalone results to the audience', async () => {
    getGameResults.mockReturnValue([
      {
        id: 951,
        action_submission_id: undefined,
        phase_id: 10,
        content: 'Draft the players must not see.',
        character_name: 'Cass',
        is_published: false,
      },
    ]);

    renderView();
    // Wait for the results query to actually settle before asserting absence,
    // otherwise this passes simply because nothing has rendered yet.
    await waitFor(() => expect(getGameResults).toHaveBeenCalled());
    await screen.findByRole('button', { name: /Vera/ });

    expect(screen.queryByText('Results without a submission')).not.toBeInTheDocument();
    expect(screen.queryByText('Draft the players must not see.')).not.toBeInTheDocument();
  });

  it('scopes standalone results to the selected phase', async () => {
    getGameResults.mockReturnValue([
      {
        id: 952,
        action_submission_id: undefined,
        phase_id: 99,
        content: 'Belongs to another phase.',
        character_name: 'Cass',
        is_published: true,
      },
    ]);

    renderView();
    await waitFor(() => expect(getGameResults).toHaveBeenCalled());
    await screen.findByRole('button', { name: /Vera/ });

    expect(screen.queryByText('Results without a submission')).not.toBeInTheDocument();
  });
});
