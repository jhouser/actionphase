import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test-utils/render';
import { ActionResultsList } from '../ActionResultsList';
import { stubRenderedHeight } from '../../test-utils/renderedHeight';

vi.mock('../../hooks/useActionResults', () => ({
  useUserActionResults: vi.fn(),
}));

vi.mock('../MarkdownPreview', () => ({
  MarkdownPreview: ({ content }: { content: string }) => <div data-testid="markdown">{content}</div>,
}));

import { useUserActionResults } from '../../hooks/useActionResults';

const makeResult = (overrides = {}) => ({
  id: 1,
  phase_number: 2,
  phase_type: 'action',
  content: 'You succeed.',
  sent_at: '2024-01-15T10:00:00Z',
  gm_username: 'gmuser',
  ...overrides,
});

describe('ActionResultsList', () => {
  // CollapsibleMarkdown decides overflow from measured height; jsdom reports 0.
  const setRenderedHeight = stubRenderedHeight(500);

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows loading state', () => {
    vi.mocked(useUserActionResults).mockReturnValue({ isLoading: true, data: undefined, error: null } as never);
    renderWithProviders(<ActionResultsList gameId={5} />);
    expect(screen.getByText(/loading your action results/i)).toBeInTheDocument();
  });

  it('shows error state', () => {
    vi.mocked(useUserActionResults).mockReturnValue({ isLoading: false, data: undefined, error: new Error('fail') } as never);
    renderWithProviders(<ActionResultsList gameId={5} />);
    expect(screen.getByText(/error loading action results/i)).toBeInTheDocument();
  });

  it('shows empty state when no results', () => {
    vi.mocked(useUserActionResults).mockReturnValue({ isLoading: false, data: [], error: null } as never);
    renderWithProviders(<ActionResultsList gameId={5} />);
    expect(screen.getByText(/no action results yet/i)).toBeInTheDocument();
  });

  it('renders result with phase info and GM username', () => {
    vi.mocked(useUserActionResults).mockReturnValue({ isLoading: false, data: [makeResult()], error: null } as never);
    renderWithProviders(<ActionResultsList gameId={5} />);
    expect(screen.getByText(/phase 2 - action/i)).toBeInTheDocument();
    expect(screen.getByText(/from: gmuser/i)).toBeInTheDocument();
  });

  it('shows full content for short results (no expand button)', () => {
    setRenderedHeight(40); // fits inside the collapsed height
    vi.mocked(useUserActionResults).mockReturnValue({ isLoading: false, data: [makeResult({ content: 'Short.' })], error: null } as never);
    renderWithProviders(<ActionResultsList gameId={5} />);
    expect(screen.getByText('Short.')).toBeInTheDocument();
    expect(screen.queryByText(/show full content/i)).not.toBeInTheDocument();
  });

  it('shows truncated content and expand button for long results', () => {
    const longContent = 'A'.repeat(300);
    vi.mocked(useUserActionResults).mockReturnValue({ isLoading: false, data: [makeResult({ content: longContent })], error: null } as never);
    renderWithProviders(<ActionResultsList gameId={5} />);
    expect(screen.getByText(/show full content/i)).toBeInTheDocument();
  });

  it('expands and collapses long content on toggle', async () => {
    const user = userEvent.setup();
    const longContent = 'B'.repeat(300);
    vi.mocked(useUserActionResults).mockReturnValue({ isLoading: false, data: [makeResult({ content: longContent })], error: null } as never);
    renderWithProviders(<ActionResultsList gameId={5} />);

    await user.click(screen.getByText(/show full content/i));
    expect(screen.getByText(/show less/i)).toBeInTheDocument();

    await user.click(screen.getByText(/show less/i));
    expect(screen.getByText(/show full content/i)).toBeInTheDocument();
  });
});
