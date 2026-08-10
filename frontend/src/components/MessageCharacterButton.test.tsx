import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { MessageCharacterButton } from './MessageCharacterButton';
import type { Character } from '../types/characters';

const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => mockNavigate };
});

vi.mock('../hooks/useCanMessageCharacter');
import { useCanMessageCharacter } from '../hooks/useCanMessageCharacter';
const mockUseCanMessage = vi.mocked(useCanMessageCharacter);

const target: Character = {
  id: 42,
  game_id: 7,
  name: 'Vesper',
  status: 'approved',
  is_active: true,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
};

function renderButton(props: Partial<Parameters<typeof MessageCharacterButton>[0]> = {}) {
  return render(
    <MemoryRouter>
      <MessageCharacterButton character={target} {...props} />
    </MemoryRouter>
  );
}

describe('MessageCharacterButton', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseCanMessage.mockReturnValue({ canMessage: true, gameId: 7 });
  });

  it('renders a labelled envelope when the user can message the character', () => {
    renderButton();
    expect(
      screen.getByRole('button', { name: 'Send a private message to Vesper' })
    ).toBeInTheDocument();
  });

  it('carries visible text so it reads as a button, not a decorative icon', () => {
    renderButton();
    const button = screen.getByRole('button', { name: /send a private message/i });
    expect(button).toHaveTextContent('Message');
  });

  it('renders nothing when the user cannot message the character', () => {
    mockUseCanMessage.mockReturnValue({ canMessage: false, gameId: 7 });
    const { container } = renderButton();
    expect(container).toBeEmptyDOMElement();
  });

  it('navigates to the game messages tab with the character pre-selected', async () => {
    const user = userEvent.setup();
    renderButton();

    await user.click(screen.getByRole('button', { name: /send a private message/i }));

    expect(mockNavigate).toHaveBeenCalledWith('/games/7?tab=messages&newConversationWith=42');
  });

  it('dismisses the host surface before navigating', async () => {
    const user = userEvent.setup();
    const onNavigate = vi.fn();
    renderButton({ onNavigate });

    await user.click(screen.getByRole('button', { name: /send a private message/i }));

    expect(onNavigate).toHaveBeenCalledTimes(1);
    expect(mockNavigate).toHaveBeenCalled();
  });
});
