import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { ActionResultsList } from './ActionResultsList';
import type { ActionResult } from '../types/phases';

// If this string ever reaches the DOM, the feature is defeated: a player with
// devtools open reads the ending without waiting. The server blanks locked
// content in SQL, so the fixtures below model what the API actually returns —
// content: '' for a locked part — and the leak test asserts the string is
// absent from the whole rendered tree.
const LOCKED_CONTENT = 'SPOILER-XYZZY-the-blow-lands-and-you-die';

const mockUseUserActionResults = vi.fn();

vi.mock('../hooks/useActionResults', () => ({
  useUserActionResults: (gameId: number) => mockUseUserActionResults(gameId),
}));

// MarkdownPreview renders through a markdown pipeline that is irrelevant here;
// a plain passthrough keeps the assertions about content, not formatting.
vi.mock('./MarkdownPreview', () => ({
  MarkdownPreview: ({ content }: { content: string }) => <div>{content}</div>,
}));

const GAME_ID = 164;

function makeResult(overrides: Partial<ActionResult> & { id: number }): ActionResult {
  return {
    game_id: GAME_ID,
    user_id: 3,
    phase_id: 10,
    gm_user_id: 1,
    content: '',
    is_published: true,
    sent_at: '2026-08-12T10:00:00Z',
    phase_type: 'action',
    phase_number: 2,
    gm_username: 'TestGM',
    ...overrides,
  };
}

function renderList() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <ActionResultsList gameId={GAME_ID} />
    </QueryClientProvider>
  );
}

describe('ActionResultsList — staged parts', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders a countdown placeholder for a locked part', () => {
    const unlocksAt = new Date(Date.now() + 5 * 60 * 1000).toISOString();
    mockUseUserActionResults.mockReturnValue({
      data: [
        makeResult({
          id: 1,
          content: 'The sword whooshes toward your head...',
          part_number: 1,
          part_count: 2,
          released_at: '2026-08-12T10:00:00Z',
        }),
        makeResult({
          id: 2,
          content: '',
          part_number: 2,
          part_count: 2,
          unlocks_at: unlocksAt,
        }),
      ],
      isLoading: false,
      error: null,
    });

    renderList();

    expect(screen.getByTestId('staged-part-placeholder-2')).toBeInTheDocument();
    expect(screen.getByText(/The sword whooshes toward your head/)).toBeInTheDocument();
    // The countdown renders mm:ss, not a date.
    expect(screen.getByText(/^\d+:\d{2}$/)).toBeInTheDocument();
  });

  it('never renders locked content anywhere in the tree', () => {
    mockUseUserActionResults.mockReturnValue({
      data: [
        makeResult({
          id: 1,
          content: 'Part one is safe to read.',
          part_number: 1,
          part_count: 2,
          released_at: '2026-08-12T10:00:00Z',
        }),
        // The API blanks this server-side. Belt and braces: even if a
        // regression let the real text through, this assertion fails.
        makeResult({
          id: 2,
          content: LOCKED_CONTENT,
          part_number: 2,
          part_count: 2,
          unlocks_at: new Date(Date.now() + 60_000).toISOString(),
        }),
      ],
      isLoading: false,
      error: null,
    });

    const { container } = renderList();

    expect(container.textContent).not.toContain(LOCKED_CONTENT);
    expect(container.textContent).not.toContain('SPOILER');
  });

  it('labels a released part with its position in the chain', () => {
    mockUseUserActionResults.mockReturnValue({
      data: [
        makeResult({
          id: 1,
          content: 'Part one.',
          part_number: 1,
          part_count: 3,
          released_at: '2026-08-12T10:00:00Z',
        }),
      ],
      isLoading: false,
      error: null,
    });

    renderList();

    expect(screen.getByTestId('staged-part-label-1')).toHaveTextContent('Part 1 of 3');
  });

  it('leaves an ordinary single-part result untouched', () => {
    mockUseUserActionResults.mockReturnValue({
      data: [makeResult({ id: 1, content: 'Just a normal result.' })],
      isLoading: false,
      error: null,
    });

    const { container } = renderList();

    expect(screen.getByText('Just a normal result.')).toBeInTheDocument();
    // No part label, no placeholder — an unstaged result carries none of the
    // staged fields, so nothing staged should render.
    expect(screen.queryByTestId('staged-part-label-1')).not.toBeInTheDocument();
    expect(container.querySelector('[data-testid^="staged-part-placeholder"]')).toBeNull();
  });

  it('shows a pending state, not a countdown, for a part with no known unlock time', () => {
    mockUseUserActionResults.mockReturnValue({
      data: [
        makeResult({
          id: 3,
          content: '',
          part_number: 3,
          part_count: 3,
          // No unlocks_at: part 2 has not released, so part 3's time is unknown.
        }),
      ],
      isLoading: false,
      error: null,
    });

    renderList();

    expect(screen.getByTestId('staged-part-placeholder-3')).toBeInTheDocument();
    expect(screen.getByText('Pending')).toBeInTheDocument();
  });
});
