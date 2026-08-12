import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render as rtlRender, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { GameStatsView } from './GameStatsView';
import * as useGameStatsModule from '../hooks/useGameStats';
import type { GameStats } from '../types/gameStats';

vi.mock('../hooks/useGameStats');

const baseStats: GameStats = {
  comments: {
    total_comments: 412,
    avg_comment_length: 340,
    longest_comment_length: 2100,
    max_thread_depth: 7,
    avg_thread_depth: 2.4,
  },
  private_messages: {
    total_private_messages: 1203,
    avg_private_message_length: 96,
    longest_private_message_length: 800,
    active_conversations: 18,
  },
  characters: [
    {
      character_id: 1,
      character_name: 'Alice',
      is_active: true,
      comment_count: 61,
      avg_comment_length: 402,
      private_message_count: 180,
      avg_private_message_length: 88,
    },
    {
      character_id: 2,
      character_name: 'Bruno',
      is_active: false,
      comment_count: 44,
      avg_comment_length: 295,
      private_message_count: 151,
      avg_private_message_length: 104,
    },
  ],
  longest_conversation: {
    conversation_id: 9,
    title: 'The Pact',
    conversation_type: 'direct',
    message_count: 47,
    participant_names: ['Alice', 'Bruno'],
  },
  longest_comment: {
    message_id: 314,
    content_length: 2100,
    character_name: 'Alice',
    phase_id: 4,
  },
};

function mockStats(overrides: Partial<ReturnType<typeof useGameStatsModule.useGameStats>>) {
  vi.mocked(useGameStatsModule.useGameStats).mockReturnValue({
    data: undefined,
    isLoading: false,
    isError: false,
    error: null,
    ...overrides,
  } as ReturnType<typeof useGameStatsModule.useGameStats>);
}

/**
 * Renders inside a router at the stats tab, since the component builds deep
 * links from the current search params.
 */
function render(ui: React.ReactElement) {
  return rtlRender(
    <MemoryRouter initialEntries={['/games/1?tab=stats']}>{ui}</MemoryRouter>
  );
}

/** Reads the character table's data rows as arrays of cell text. */
function tableRows(): string[][] {
  const table = screen.getByRole('table');
  return within(table)
    .getAllByRole('row')
    .slice(1) // drop the header row
    .map((row) =>
      within(row)
        .getAllByRole('cell')
        .map((cell) => cell.textContent?.trim() ?? '')
    );
}

describe('GameStatsView', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows game-wide totals', () => {
    mockStats({ data: baseStats });
    render(<GameStatsView gameId={1} gameState="completed" />);

    expect(screen.getByText('412')).toBeInTheDocument();
    expect(screen.getByText('340 characters')).toBeInTheDocument();
    expect(screen.getByText('1,203')).toBeInTheDocument();
    expect(screen.getByText('96 characters')).toBeInTheDocument();
    expect(screen.getByText('across 18 conversations')).toBeInTheDocument();
  });

  it('singularizes a one-character length', () => {
    mockStats({
      data: {
        ...baseStats,
        comments: { ...baseStats.comments, avg_comment_length: 1 },
      },
    });
    render(<GameStatsView gameId={1} gameState="completed" />);

    expect(screen.getByText('1 character')).toBeInTheDocument();
  });

  it('shows thread depth including the average', () => {
    mockStats({ data: baseStats });
    render(<GameStatsView gameId={1} gameState="completed" />);

    expect(screen.getByText('7 deep')).toBeInTheDocument();
    expect(screen.getByText('avg reply depth 2.4')).toBeInTheDocument();
  });

  it('shows the longest conversation with its participants', () => {
    mockStats({ data: baseStats });
    render(<GameStatsView gameId={1} gameState="completed" />);

    expect(screen.getByText('Longest conversation')).toBeInTheDocument();
    expect(screen.getByText('47 messages')).toBeInTheDocument();
    expect(screen.getByText('The Pact')).toBeInTheDocument();
    expect(screen.getByText('Alice · Bruno')).toBeInTheDocument();
  });

  it('links the longest conversation into the Audience tab', () => {
    mockStats({ data: baseStats });
    render(<GameStatsView gameId={1} gameState="completed" />);

    const link = screen.getByRole('link', { name: /Read it in Audience/ });
    const href = link.getAttribute('href') ?? '';
    expect(href).toContain('tab=audience');
    expect(href).toContain('audienceConversation=9');
    // Tab-specific params from the History link must not leak across.
    expect(href).not.toContain('phase=');
    expect(href).not.toContain('comment=');
  });

  it('shows the longest comment and links it into History', () => {
    mockStats({ data: baseStats });
    render(<GameStatsView gameId={1} gameState="completed" />);

    expect(screen.getByText('Longest comment')).toBeInTheDocument();
    expect(screen.getByText('2,100 characters')).toBeInTheDocument();
    expect(screen.getByText('by Alice')).toBeInTheDocument();

    const link = screen.getByRole('link', { name: /Read it in History/ });
    const href = link.getAttribute('href') ?? '';
    expect(href).toContain('tab=history');
    expect(href).toContain('comment=314');
    // The phase must be in the link. Without it HistoryView resolves the
    // comment's phase itself and pushes a second history entry, which broke
    // the browser Back button.
    expect(href).toContain('phase=4');
  });

  it('omits phase from the comment link when the backend did not supply one', () => {
    mockStats({
      data: {
        ...baseStats,
        longest_comment: { ...baseStats.longest_comment!, phase_id: undefined },
      },
    });
    render(<GameStatsView gameId={1} gameState="completed" />);

    const href =
      screen.getByRole('link', { name: /Read it in History/ }).getAttribute('href') ?? '';
    expect(href).toContain('comment=314');
    expect(href).not.toContain('phase=');
  });

  it('omits the longest comment card when the game had no comments', () => {
    mockStats({ data: { ...baseStats, longest_comment: undefined } });
    render(<GameStatsView gameId={1} gameState="completed" />);

    expect(screen.queryByText('Longest comment')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('link', { name: /Read it in History/ })
    ).not.toBeInTheDocument();
  });

  it('omits the longest conversation card when the game had none', () => {
    mockStats({ data: { ...baseStats, longest_conversation: undefined } });
    render(<GameStatsView gameId={1} gameState="completed" />);

    expect(screen.queryByText('Longest conversation')).not.toBeInTheDocument();
    // The rest of the page still renders.
    expect(screen.getByText('412')).toBeInTheDocument();
  });

  it('lists characters sorted by comment count by default', () => {
    mockStats({ data: baseStats });
    render(<GameStatsView gameId={1} gameState="completed" />);

    const rows = tableRows();
    expect(rows[0][0]).toContain('Alice');
    expect(rows[0][1]).toBe('61');
    expect(rows[1][0]).toContain('Bruno');
    expect(rows[1][1]).toBe('44');
  });

  it('marks inactive characters', () => {
    mockStats({ data: baseStats });
    render(<GameStatsView gameId={1} gameState="completed" />);

    const rows = tableRows();
    expect(rows[1][0]).toContain('(inactive)');
    expect(rows[0][0]).not.toContain('(inactive)');
  });

  it('re-sorts when a column header is clicked', async () => {
    const user = userEvent.setup();
    mockStats({ data: baseStats });
    render(<GameStatsView gameId={1} gameState="completed" />);

    // Bruno writes longer PMs (104) than Alice (88), so sorting by average PM
    // length must put Bruno first — a different order than the default.
    await user.click(screen.getByRole('button', { name: /Avg PM length/ }));

    const rows = tableRows();
    expect(rows[0][0]).toContain('Bruno');
    expect(rows[1][0]).toContain('Alice');
  });

  it('reverses order when the active column is clicked again', async () => {
    const user = userEvent.setup();
    mockStats({ data: baseStats });
    render(<GameStatsView gameId={1} gameState="completed" />);

    // Comments is already the active column, descending. One click flips it.
    await user.click(screen.getByRole('button', { name: /Comments/ }));

    const rows = tableRows();
    expect(rows[0][0]).toContain('Bruno');
    expect(rows[1][0]).toContain('Alice');
  });

  it('tells the user stats are unavailable before the game completes', () => {
    mockStats({ data: undefined });
    render(<GameStatsView gameId={1} gameState="in_progress" />);

    expect(
      screen.getByText('Statistics become available once the game is completed.')
    ).toBeInTheDocument();
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  });

  it('shows an error when stats fail to load', () => {
    mockStats({ isError: true, error: new Error('boom') });
    render(<GameStatsView gameId={1} gameState="completed" />);

    expect(screen.getByText(/Failed to load statistics/)).toBeInTheDocument();
  });

  it('shows a loading state', () => {
    mockStats({ isLoading: true });
    render(<GameStatsView gameId={1} gameState="completed" />);

    // Spinner renders its label twice: once visibly and once for screen readers.
    expect(screen.getAllByText('Loading statistics...').length).toBeGreaterThan(0);
  });
});
