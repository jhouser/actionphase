import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { ActionSubmission } from './ActionSubmission';
import type { GamePhase } from '../types/phases';
import { useCharacterSheetItems } from '../hooks/useCharacterSheetItems';

vi.mock('../hooks/useUserCharacters', () => ({
  useUserCharacters: vi.fn(() => ({ characters: [], isLoading: false })),
}));

vi.mock('../hooks/useCharacterSheetItems', () => ({
  useCharacterSheetItems: vi.fn(() => []),
}));

vi.mock('../lib/api', () => ({
  apiClient: {
    phases: {
      getUserActions: vi.fn(() => Promise.resolve({ data: [] })),
    },
  },
}));

const baseActionPhase: GamePhase = {
  id: 42,
  game_id: 1,
  phase_type: 'action',
  phase_number: 2,
  is_active: true,
  is_published: false,
  created_at: new Date().toISOString(),
};

const renderWithClient = (ui: React.ReactElement) => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const router = createMemoryRouter([{ path: '/', element: <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider> }], { initialEntries: ['/'] });
  return render(<RouterProvider router={router} />);
};

describe('ActionSubmission subtitle text', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows phase description when the GM provided one', () => {
    const phase = { ...baseActionPhase, description: 'Scouts report movement to the north.' };
    renderWithClient(<ActionSubmission gameId={1} currentPhase={phase} />);
    expect(screen.getByText('Scouts report movement to the north.')).toBeInTheDocument();
    expect(screen.queryByText('Submit your private action to the GM')).not.toBeInTheDocument();
  });

  it('falls back to default text when no description is set', () => {
    renderWithClient(<ActionSubmission gameId={1} currentPhase={baseActionPhase} />);
    expect(screen.getByText('Submit your private action to the GM')).toBeInTheDocument();
  });

  it('falls back to default text when description is an empty string', () => {
    const phase = { ...baseActionPhase, description: '' };
    renderWithClient(<ActionSubmission gameId={1} currentPhase={phase} />);
    expect(screen.getByText('Submit your private action to the GM')).toBeInTheDocument();
  });
});

describe('ActionSubmission observability', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    // vi.clearAllMocks() clears call history but not a mockReturnValue; reset the
    // sheet-items implementation so the toggle test below can't leak into later blocks.
    vi.mocked(useCharacterSheetItems).mockReturnValue([]);
  });

  it('names the submit button for Faro user-action attribution', () => {
    renderWithClient(<ActionSubmission gameId={1} currentPhase={baseActionPhase} />);
    expect(screen.getByTestId('submit-action-button')).toHaveAttribute(
      'data-faro-user-action-name',
      'submit-action'
    );
  });

  it('names the character-sheet toggle for Faro user-action attribution', () => {
    // The toggle only renders when the character has sheet items.
    vi.mocked(useCharacterSheetItems).mockReturnValue([
      { id: 's1', name: 'Fire Bolt', type: 'skill' },
    ]);
    renderWithClient(<ActionSubmission gameId={1} currentPhase={baseActionPhase} />);
    expect(screen.getByTestId('sheet-toggle-button')).toHaveAttribute(
      'data-faro-user-action-name',
      'open-character-sheet'
    );
  });
});

describe('ActionSubmission sheet drawer', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('does not show Sheet button when character has no sheet items', () => {
    renderWithClient(<ActionSubmission gameId={1} currentPhase={baseActionPhase} />);
    expect(screen.queryByTestId('sheet-toggle-button')).not.toBeInTheDocument();
  });

  it('shows Sheet button when character has sheet items', () => {
    vi.mocked(useCharacterSheetItems).mockReturnValue([
      { id: 's1', name: 'Fire Bolt', type: 'skill' },
    ]);
    renderWithClient(<ActionSubmission gameId={1} currentPhase={baseActionPhase} />);
    expect(screen.getByTestId('sheet-toggle-button')).toBeInTheDocument();
  });

  it('opens drawer when Sheet button is clicked', async () => {
    vi.mocked(useCharacterSheetItems).mockReturnValue([
      { id: 's1', name: 'Fire Bolt', type: 'skill' },
    ]);
    const user = userEvent.setup();
    renderWithClient(<ActionSubmission gameId={1} currentPhase={baseActionPhase} />);
    await user.click(screen.getByTestId('sheet-toggle-button'));
    expect(screen.getByRole('dialog', { name: 'Character Sheet' })).toBeInTheDocument();
    expect(screen.getByText('Fire Bolt')).toBeInTheDocument();
  });
});
