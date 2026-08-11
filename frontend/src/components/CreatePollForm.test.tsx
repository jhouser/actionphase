import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { CreatePollForm } from './CreatePollForm';

const mutateAsync = vi.fn().mockResolvedValue({});

vi.mock('../hooks', () => ({
  usePolls: () => ({
    createPollMutation: {
      mutateAsync,
      isPending: false,
      isError: false,
      error: null,
    },
  }),
}));

// DateTimeInput wraps react-datepicker, whose interactions are covered at the E2E
// level by convention in this codebase. Stub it with a plain input so these tests
// exercise the poll option logic rather than the date widget's parsing.
vi.mock('./ui', async () => {
  const actual = await vi.importActual<typeof import('./ui')>('./ui');
  return {
    ...actual,
    DateTimeInput: ({
      value,
      onChange,
    }: {
      value: string;
      onChange: (e: { target: { value: string } }) => void;
    }) => (
      <input
        data-testid="deadline-input"
        value={value}
        onChange={(e) => onChange({ target: { value: e.target.value } })}
      />
    ),
  };
});

const renderForm = () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <CreatePollForm gameId={100} onSuccess={vi.fn()} onCancel={vi.fn()} />
    </QueryClientProvider>
  );
};

const showIndividualVotes = () =>
  screen.getByLabelText(/show individual votes/i) as HTMLInputElement;
const hideResults = () =>
  screen.getByLabelText(/hide results from players/i) as HTMLInputElement;
const audienceVoting = () =>
  screen.getByLabelText(/allow audience members to vote/i) as HTMLInputElement;
const runningTotals = () =>
  screen.getByLabelText(/show running totals to players/i) as HTMLInputElement;

// Fills everything the form requires before it will submit. Text inputs are
// queried by placeholder because the shared Input components do not associate
// their labels via htmlFor.
const fillRequiredFields = (question: string) => {
  fireEvent.change(screen.getByPlaceholderText(/what would you like to ask/i), {
    target: { value: question },
  });

  fireEvent.change(screen.getByTestId('deadline-input'), {
    target: { value: '2099-01-01T12:00' },
  });

  const optionInputs = screen.getAllByPlaceholderText(/^Option \d$/);
  fireEvent.change(optionInputs[0], { target: { value: 'Yes' } });
  fireEvent.change(optionInputs[1], { target: { value: 'No' } });
};

describe('CreatePollForm - poll visibility options', () => {
  beforeEach(() => {
    mutateAsync.mockClear();
  });

  it('offers the hidden-results and audience-voting options', () => {
    renderForm();

    expect(hideResults()).toBeInTheDocument();
    expect(audienceVoting()).toBeInTheDocument();
  });

  it('disables individual votes once results are hidden', () => {
    renderForm();

    fireEvent.click(hideResults());

    expect(hideResults().checked).toBe(true);
    expect(showIndividualVotes()).toBeDisabled();
  });

  it('clears hidden results when individual votes are turned on', () => {
    renderForm();

    fireEvent.click(hideResults());
    expect(hideResults().checked).toBe(true);

    // Turning the hidden option back off frees the individual-votes checkbox.
    fireEvent.click(hideResults());
    fireEvent.click(showIndividualVotes());

    expect(showIndividualVotes().checked).toBe(true);
    expect(hideResults().checked).toBe(false);
    expect(hideResults()).toBeDisabled();
  });

  it('blocks hiding results while individual votes are shown', () => {
    renderForm();

    fireEvent.click(showIndividualVotes());

    // A real user cannot reach the invalid combination: the control is disabled.
    expect(showIndividualVotes().checked).toBe(true);
    expect(hideResults()).toBeDisabled();
    expect(hideResults().checked).toBe(false);
  });

  it('submits only one of the two exclusive flags', async () => {
    renderForm();

    fillRequiredFields('Exclusive poll');

    // Toggle back and forth; whichever wins, the payload must never set both.
    fireEvent.click(hideResults());
    fireEvent.click(hideResults());
    fireEvent.click(showIndividualVotes());

    fireEvent.click(screen.getByRole('button', { name: /create poll/i }));

    await waitFor(() => expect(mutateAsync).toHaveBeenCalled());

    const payload = mutateAsync.mock.calls[0][0];
    expect(payload.show_individual_votes && payload.hide_results_from_players).toBe(false);
    expect(payload.show_individual_votes).toBe(true);
  });

  it('submits the new flags with the poll', async () => {
    renderForm();

    fillRequiredFields('Should we storm the keep?');

    fireEvent.click(hideResults());
    fireEvent.click(audienceVoting());

    fireEvent.click(screen.getByRole('button', { name: /create poll/i }));

    await waitFor(() => expect(mutateAsync).toHaveBeenCalled());

    expect(mutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({
        question: 'Should we storm the keep?',
        hide_results_from_players: true,
        allow_audience_voting: true,
        show_individual_votes: false,
      })
    );
  });

  it('defaults both new options to off', async () => {
    renderForm();

    fillRequiredFields('Plain poll');

    fireEvent.click(screen.getByRole('button', { name: /create poll/i }));

    await waitFor(() => expect(mutateAsync).toHaveBeenCalled());

    expect(mutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({
        hide_results_from_players: false,
        allow_audience_voting: false,
      })
    );
  });
});

describe('CreatePollForm - running totals', () => {
  beforeEach(() => {
    mutateAsync.mockClear();
  });

  it('offers the running-totals option, off by default', async () => {
    renderForm();

    expect(runningTotals()).toBeInTheDocument();
    expect(runningTotals().checked).toBe(false);

    fillRequiredFields('Plain poll');
    fireEvent.click(screen.getByRole('button', { name: /create poll/i }));

    await waitFor(() => expect(mutateAsync).toHaveBeenCalled());
    expect(mutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({ show_running_totals_to_players: false })
    );
  });

  it('blocks hiding results while running totals are shown', () => {
    renderForm();

    fireEvent.click(runningTotals());

    expect(runningTotals().checked).toBe(true);
    expect(hideResults()).toBeDisabled();
    expect(hideResults().checked).toBe(false);
  });

  it('disables running totals once results are hidden', () => {
    renderForm();

    fireEvent.click(hideResults());

    expect(hideResults().checked).toBe(true);
    expect(runningTotals()).toBeDisabled();
  });

  it('clears hidden results when running totals are turned on', () => {
    renderForm();

    fireEvent.click(hideResults());
    expect(hideResults().checked).toBe(true);

    // Freeing the control lets the GM pick the opposite option instead.
    fireEvent.click(hideResults());
    fireEvent.click(runningTotals());

    expect(runningTotals().checked).toBe(true);
    expect(hideResults().checked).toBe(false);
  });

  it('combines running totals with individual votes', async () => {
    renderForm();

    fillRequiredFields('Who is winning?');

    // The two disclosure options are complementary, not exclusive.
    fireEvent.click(runningTotals());
    fireEvent.click(showIndividualVotes());

    expect(runningTotals().checked).toBe(true);
    expect(showIndividualVotes().checked).toBe(true);

    fireEvent.click(screen.getByRole('button', { name: /create poll/i }));

    await waitFor(() => expect(mutateAsync).toHaveBeenCalled());
    expect(mutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({
        show_running_totals_to_players: true,
        show_individual_votes: true,
        hide_results_from_players: false,
      })
    );
  });

  it('never submits running totals together with hidden results', async () => {
    renderForm();

    fillRequiredFields('Exclusive poll');

    // Switching between the two requires clearing the first: while hiding is on
    // the running-totals box is disabled, and vice versa.
    fireEvent.click(hideResults());
    fireEvent.click(hideResults());
    fireEvent.click(runningTotals());

    fireEvent.click(screen.getByRole('button', { name: /create poll/i }));

    await waitFor(() => expect(mutateAsync).toHaveBeenCalled());

    const payload = mutateAsync.mock.calls[0][0];
    expect(
      payload.show_running_totals_to_players && payload.hide_results_from_players
    ).toBe(false);
    expect(payload.show_running_totals_to_players).toBe(true);
  });
});
