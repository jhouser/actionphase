import { describe, it, expect } from 'vitest';
import { screen } from '@testing-library/react';
import { renderWithProviders } from '../../test-utils/render';
import { PollResults } from '../PollResults';
import type { Poll, PollResults as PollResultsType } from '../../types/polls';

const basePoll: Poll = {
  id: 1,
  game_id: 1,
  created_by_user_id: 1,
  question: 'Which path?',
  deadline: new Date(Date.now() + 3600000).toISOString(),
  show_individual_votes: false,
  allow_other_option: false,
  is_deleted: false,
  created_at: '2024-01-01T00:00:00Z',
};

const makeResults = (overrides: Partial<PollResultsType> = {}): PollResultsType => ({
  poll: basePoll,
  option_results: [
    { poll_option_id: 1, option_text: 'Forest', vote_count: 3, voters: [] },
    { poll_option_id: 2, option_text: 'Mountain', vote_count: 1, voters: [] },
  ],
  other_responses: [],
  total_votes: 4,
  show_individual_votes: false,
  ...overrides,
});

describe('PollResults', () => {
  it('shows vote count in header', () => {
    renderWithProviders(<PollResults results={makeResults()} poll={basePoll} />);
    expect(screen.getByText('4 votes')).toBeInTheDocument();
  });

  it('uses singular "vote" for 1 total vote', () => {
    const results = makeResults({
      option_results: [{ poll_option_id: 1, option_text: 'Option A', vote_count: 1, voters: [] }],
      total_votes: 1,
    });
    renderWithProviders(<PollResults results={results} poll={basePoll} />);
    expect(screen.getByText('1 vote')).toBeInTheDocument();
  });

  it('shows "No votes yet" when total_votes is 0', () => {
    const results = makeResults({
      option_results: [],
      total_votes: 0,
    });
    renderWithProviders(<PollResults results={results} poll={basePoll} />);
    expect(screen.getByText('No votes yet')).toBeInTheDocument();
  });

  it('renders option names', () => {
    renderWithProviders(<PollResults results={makeResults()} poll={basePoll} />);
    expect(screen.getByText('Forest')).toBeInTheDocument();
    expect(screen.getByText('Mountain')).toBeInTheDocument();
  });

  it('shows WINNER badge for leading option when expired', () => {
    renderWithProviders(<PollResults results={makeResults()} poll={basePoll} isExpired />);
    expect(screen.getByText('WINNER')).toBeInTheDocument();
  });

  it('shows TIE badge when multiple options share the lead and poll is expired', () => {
    const tiedResults = makeResults({
      option_results: [
        { poll_option_id: 1, option_text: 'Forest', vote_count: 2, voters: [] },
        { poll_option_id: 2, option_text: 'Mountain', vote_count: 2, voters: [] },
      ],
      total_votes: 4,
    });
    renderWithProviders(<PollResults results={tiedResults} poll={basePoll} isExpired />);
    expect(screen.getAllByText('TIE')).toHaveLength(2);
  });

  it('shows Leading indicator when poll is not expired', () => {
    renderWithProviders(<PollResults results={makeResults()} poll={basePoll} isExpired={false} />);
    expect(screen.getByText('● Leading')).toBeInTheDocument();
  });

  it('does not show voter names when show_individual_votes is false and not GM/audience', () => {
    const results = makeResults({
      option_results: [
        { poll_option_id: 1, option_text: 'Forest', vote_count: 1, voters: [{ user_id: 2, character_name: 'Elara' }] },
      ],
      total_votes: 1,
    });
    renderWithProviders(<PollResults results={results} poll={basePoll} />);
    expect(screen.queryByText('Elara')).not.toBeInTheDocument();
  });

  it('shows voter names when isGM is true regardless of show_individual_votes', () => {
    const results = makeResults({
      option_results: [
        { poll_option_id: 1, option_text: 'Forest', vote_count: 1, voters: [{ user_id: 2, character_name: 'Elara' }] },
      ],
      total_votes: 1,
    });
    renderWithProviders(<PollResults results={results} poll={basePoll} isGM />);
    expect(screen.getByText('Elara')).toBeInTheDocument();
  });

  it('shows voter names when isAudience is true', () => {
    const results = makeResults({
      option_results: [
        { poll_option_id: 1, option_text: 'Forest', vote_count: 1, voters: [{ user_id: 2, character_name: 'Bob Character' }] },
      ],
      total_votes: 1,
    });
    renderWithProviders(<PollResults results={results} poll={basePoll} isAudience />);
    expect(screen.getByText('Bob Character')).toBeInTheDocument();
  });

  it('shows voter names when show_individual_votes is true on the poll', () => {
    const pollWithVotes = { ...basePoll, show_individual_votes: true };
    const results = makeResults({
      option_results: [
        { poll_option_id: 1, option_text: 'Forest', vote_count: 1, voters: [{ user_id: 2, character_name: 'Carol Character' }] },
      ],
      total_votes: 1,
      show_individual_votes: true,
    });
    renderWithProviders(<PollResults results={results} poll={pollWithVotes} />);
    expect(screen.getByText('Carol Character')).toBeInTheDocument();
  });

  it('shows full other responses list to GM', () => {
    const results = makeResults({
      other_responses: [{ vote_id: 1, other_text: 'The river route', character_name: 'Alice' }],
    });
    renderWithProviders(<PollResults results={results} poll={basePoll} isGM />);
    expect(screen.getByText(/"The river route"/)).toBeInTheDocument();
    expect(screen.getByText('Other Responses (1)')).toBeInTheDocument();
  });

  it('shows count-only other responses to non-GM players', () => {
    const results = makeResults({
      other_responses: [
        { vote_id: 1, other_text: 'Secret route', character_name: 'Alice' },
        { vote_id: 2, other_text: 'Another route', character_name: 'Bob' },
      ],
    });
    renderWithProviders(<PollResults results={results} poll={basePoll} />);
    expect(screen.getByText('2 custom responses')).toBeInTheDocument();
    expect(screen.queryByText(/"Secret route"/)).not.toBeInTheDocument();
  });

  it('uses singular "response" for 1 other response to non-GM', () => {
    const results = makeResults({
      other_responses: [{ vote_id: 1, other_text: 'Something', character_name: 'Alice' }],
    });
    renderWithProviders(<PollResults results={results} poll={basePoll} />);
    expect(screen.getByText('1 custom response')).toBeInTheDocument();
  });

  describe('anonymous audience votes', () => {
    it('labels an anonymous voter as Anonymous alongside named voters', () => {
      const results = makeResults({
        option_results: [
          {
            poll_option_id: 1,
            option_text: 'Forest',
            vote_count: 2,
            voters: [
              { user_id: 5, character_name: 'Kael' },
              { user_id: 0, character_name: 'Anonymous', is_anonymous: true },
            ],
          },
        ],
        total_votes: 2,
      });

      const { container } = renderWithProviders(
        <PollResults results={results} poll={basePoll} isGM={true} />
      );

      // Voter names render as sibling spans joined by ", ", so assert on the list.
      const voterList = container.querySelector('.ml-4');
      expect(voterList).toHaveTextContent('Kael');
      expect(voterList).toHaveTextContent('Anonymous');
    });

    it('labels an anonymous other response as Anonymous while keeping its text', () => {
      const results = makeResults({
        other_responses: [
          { vote_id: 1, other_text: 'Burn it down', character_name: 'Anonymous', is_anonymous: true },
        ],
      });

      renderWithProviders(
        <PollResults results={results} poll={basePoll} isGM={true} />
      );

      expect(screen.getByText('Anonymous:')).toBeInTheDocument();
      expect(screen.getByText('"Burn it down"')).toBeInTheDocument();
    });
  });
});
