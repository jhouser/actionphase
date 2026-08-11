import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import type { UseInfiniteQueryResult, UseQueryResult } from '@tanstack/react-query';
import { CharacterPage } from './CharacterPage';
import * as useCharacterCommentsModule from '../hooks/useCharacterComments';
import * as useCharacterStatsModule from '../hooks/useCharacterStats';
import type { Character } from '../types/characters';
import type { CharacterMessage, CharacterMessagesResponse } from '../types/messages';

// Mock hooks
vi.mock('../hooks/useCharacterComments');
vi.mock('../hooks/useCharacterStats');

// The envelope shortcut has its own suite; stub its gate so this file's blanket
// useQuery mock (which answers every query with the character) doesn't feed it
// a phase payload it never sees in production.
vi.mock('../hooks/useCanMessageCharacter');
import { useCanMessageCharacter } from '../hooks/useCanMessageCharacter';

vi.mock('@tanstack/react-query', async () => {
  const actual = await vi.importActual('@tanstack/react-query');
  return {
    ...actual,
    useQuery: vi.fn(),
  };
});

import { useQuery } from '@tanstack/react-query';

// Mock navigate
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Mock IntersectionObserver
const mockIntersectionObserver = vi.fn().mockReturnValue({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
});
window.IntersectionObserver = mockIntersectionObserver as unknown as typeof IntersectionObserver;

const mockCharacter: Character = {
  id: 42,
  game_id: 1,
  name: 'Aelindra',
  character_type: 'player_character',
  status: 'active',
  avatar_url: null,
  is_active: true,
  username: 'testplayer',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
};

const mockMessage: CharacterMessage = {
  id: 1,
  game_id: 1,
  parent_id: null,
  author_id: 10,
  character_id: 42,
  content: 'Hello world',
  message_type: 'post',
  created_at: '2025-03-01T10:00:00Z',
  edited_at: null,
  edit_count: 0,
  deleted_at: null,
  is_deleted: false,
  author_username: 'testplayer',
  character_name: 'Aelindra',
  character_avatar_url: null,
};

const mockComment: CharacterMessage = {
  ...mockMessage,
  id: 2,
  message_type: 'comment',
  content: 'A reply',
  parent: {
    content: 'Original post',
    created_at: '2025-03-01T09:00:00Z',
    deleted_at: null,
    is_deleted: false,
    message_type: 'post',
    author_username: 'someone',
    character_name: 'Other Character',
  },
};

function renderCharacterPage(characterId = '42') {
  return render(
    <MemoryRouter initialEntries={[`/characters/${characterId}`]}>
      <Routes>
        <Route path="/characters/:characterId" element={<CharacterPage />} />
      </Routes>
    </MemoryRouter>
  );
}

describe('CharacterPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default: no messaging affordance. Its own gate is covered by
    // useCanMessageCharacter's suite; stubbing it here also keeps this file's
    // blanket useQuery mock from feeding the real hook a phase payload it
    // would never receive in production.
    vi.mocked(useCanMessageCharacter).mockReturnValue({
      canMessage: false,
      gameId: undefined,
    });
    // Default: stats not loaded (undefined data)
    vi.mocked(useCharacterStatsModule.useCharacterStats).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
    } as Partial<ReturnType<typeof useCharacterStatsModule.useCharacterStats>>);
  });

  it('shows loading state while character loads', () => {
    vi.mocked(useQuery).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
    } as Partial<UseQueryResult<Character>>);

    vi.mocked(useCharacterCommentsModule.useCharacterComments).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
    } as Partial<UseInfiniteQueryResult<CharacterMessagesResponse>>);

    renderCharacterPage();

    // Should show skeleton loading (div with animate-pulse)
    expect(document.querySelector('.animate-pulse')).toBeInTheDocument();
  });

  it('shows character name and avatar when loaded', () => {
    vi.mocked(useQuery).mockReturnValue({
      data: mockCharacter,
      isLoading: false,
      isError: false,
    } as Partial<UseQueryResult<Character>>);

    vi.mocked(useCharacterCommentsModule.useCharacterComments).mockReturnValue({
      data: { pages: [{ messages: [], pagination: { total: 0, limit: 20, offset: 0 } }] },
      isLoading: false,
      isError: false,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
    } as Partial<UseInfiniteQueryResult<CharacterMessagesResponse>>);

    renderCharacterPage();

    expect(screen.getByText('Aelindra')).toBeInTheDocument();
    expect(screen.getByText('@testplayer')).toBeInTheDocument();
  });

  it('shows empty state when character has no messages', () => {
    vi.mocked(useQuery).mockReturnValue({
      data: mockCharacter,
      isLoading: false,
      isError: false,
    } as Partial<UseQueryResult<Character>>);

    vi.mocked(useCharacterCommentsModule.useCharacterComments).mockReturnValue({
      data: { pages: [{ messages: [], pagination: { total: 0, limit: 20, offset: 0 } }] },
      isLoading: false,
      isError: false,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
    } as Partial<UseInfiniteQueryResult<CharacterMessagesResponse>>);

    renderCharacterPage();

    expect(screen.getByText(/no public activity yet/i)).toBeInTheDocument();
  });

  it('renders posts and comments in the activity feed', () => {
    vi.mocked(useQuery).mockReturnValue({
      data: mockCharacter,
      isLoading: false,
      isError: false,
    } as Partial<UseQueryResult<Character>>);

    vi.mocked(useCharacterCommentsModule.useCharacterComments).mockReturnValue({
      data: {
        pages: [{
          messages: [mockMessage, mockComment],
          pagination: { total: 2, limit: 20, offset: 0 },
        }],
      },
      isLoading: false,
      isError: false,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
    } as Partial<UseInfiniteQueryResult<CharacterMessagesResponse>>);

    renderCharacterPage();

    expect(screen.getByText('Hello world')).toBeInTheDocument();
    expect(screen.getByText('A reply')).toBeInTheDocument();
    // Content is present (badges removed from CharacterPage activity feed)
    expect(screen.getByText('Hello world')).toBeInTheDocument();
    expect(screen.getByText('A reply')).toBeInTheDocument();
  });

  it('shows error when messages fail to load', () => {
    vi.mocked(useQuery).mockReturnValue({
      data: mockCharacter,
      isLoading: false,
      isError: false,
    } as Partial<UseQueryResult<Character>>);

    vi.mocked(useCharacterCommentsModule.useCharacterComments).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error('Network error'),
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
    } as Partial<UseInfiniteQueryResult<CharacterMessagesResponse>>);

    renderCharacterPage();

    expect(screen.getByText(/failed to load activity/i)).toBeInTheDocument();
    expect(screen.getByText('Network error')).toBeInTheDocument();
  });

  it('shows "View in thread" link for non-deleted messages', () => {
    vi.mocked(useQuery).mockReturnValue({
      data: mockCharacter,
      isLoading: false,
      isError: false,
    } as Partial<UseQueryResult<Character>>);

    vi.mocked(useCharacterCommentsModule.useCharacterComments).mockReturnValue({
      data: {
        pages: [{
          messages: [mockMessage],
          pagination: { total: 1, limit: 20, offset: 0 },
        }],
      },
      isLoading: false,
      isError: false,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
    } as Partial<UseInfiniteQueryResult<CharacterMessagesResponse>>);

    renderCharacterPage();

    expect(screen.getByText(/view in thread/i)).toBeInTheDocument();
  });

  it('navigates to game thread when "View in thread" is clicked', () => {
    vi.mocked(useQuery).mockReturnValue({
      data: mockCharacter,
      isLoading: false,
      isError: false,
    } as Partial<UseQueryResult<Character>>);

    vi.mocked(useCharacterCommentsModule.useCharacterComments).mockReturnValue({
      data: {
        pages: [{
          messages: [mockMessage],
          pagination: { total: 1, limit: 20, offset: 0 },
        }],
      },
      isLoading: false,
      isError: false,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
    } as Partial<UseInfiniteQueryResult<CharacterMessagesResponse>>);

    renderCharacterPage();

    const link = screen.getByText(/view in thread/i);
    link.click();

    expect(mockNavigate).toHaveBeenCalledWith('/games/1?tab=common-room&comment=1');
  });

  it('shows public and private stats when both are returned', () => {
    vi.mocked(useQuery).mockReturnValue({
      data: mockCharacter,
      isLoading: false,
      isError: false,
    } as Partial<UseQueryResult<Character>>);

    vi.mocked(useCharacterCommentsModule.useCharacterComments).mockReturnValue({
      data: { pages: [{ messages: [], pagination: { total: 0, limit: 20, offset: 0 } }] },
      isLoading: false,
      isError: false,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
    } as Partial<UseInfiniteQueryResult<CharacterMessagesResponse>>);

    vi.mocked(useCharacterStatsModule.useCharacterStats).mockReturnValue({
      data: { public_messages: 42, private_messages: 13 },
      isLoading: false,
      isError: false,
    } as Partial<ReturnType<typeof useCharacterStatsModule.useCharacterStats>>);

    renderCharacterPage();

    expect(screen.getByTestId('character-stats')).toBeInTheDocument();
    expect(screen.getByTestId('public-message-count')).toHaveTextContent('42');
    expect(screen.getByTestId('private-message-count')).toHaveTextContent('13');
  });

  it('shows only public stats when private_messages is absent', () => {
    vi.mocked(useQuery).mockReturnValue({
      data: mockCharacter,
      isLoading: false,
      isError: false,
    } as Partial<UseQueryResult<Character>>);

    vi.mocked(useCharacterCommentsModule.useCharacterComments).mockReturnValue({
      data: { pages: [{ messages: [], pagination: { total: 0, limit: 20, offset: 0 } }] },
      isLoading: false,
      isError: false,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
    } as Partial<UseInfiniteQueryResult<CharacterMessagesResponse>>);

    vi.mocked(useCharacterStatsModule.useCharacterStats).mockReturnValue({
      data: { public_messages: 7 },
      isLoading: false,
      isError: false,
    } as Partial<ReturnType<typeof useCharacterStatsModule.useCharacterStats>>);

    renderCharacterPage();

    expect(screen.getByTestId('public-message-count')).toHaveTextContent('7');
    expect(screen.queryByTestId('private-message-count')).not.toBeInTheDocument();
  });

  it('shows invalid character ID error for non-numeric ID', () => {
    vi.mocked(useQuery).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
    } as Partial<UseQueryResult<Character>>);

    vi.mocked(useCharacterCommentsModule.useCharacterComments).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
    } as Partial<UseInfiniteQueryResult<CharacterMessagesResponse>>);

    renderCharacterPage('not-a-number');

    expect(screen.getByText(/invalid character id/i)).toBeInTheDocument();
  });

  it('shows character type badge when character_type is present', () => {
    vi.mocked(useQuery).mockReturnValue({
      data: { ...mockCharacter, character_type: 'player_character' },
      isLoading: false,
      isError: false,
    } as Partial<UseQueryResult<Character>>);

    vi.mocked(useCharacterCommentsModule.useCharacterComments).mockReturnValue({
      data: { pages: [{ messages: [], pagination: { total: 0, limit: 20, offset: 0 } }] },
      isLoading: false,
      isError: false,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
    } as Partial<UseInfiniteQueryResult<CharacterMessagesResponse>>);

    renderCharacterPage();

    expect(screen.getByText('Player Character')).toBeInTheDocument();
  });

  it('hides character type badge when character_type is absent (anonymous mode)', () => {
    const { character_type: _, ...characterWithoutType } = mockCharacter;
    vi.mocked(useQuery).mockReturnValue({
      data: characterWithoutType as Character,
      isLoading: false,
      isError: false,
    } as Partial<UseQueryResult<Character>>);

    vi.mocked(useCharacterCommentsModule.useCharacterComments).mockReturnValue({
      data: { pages: [{ messages: [], pagination: { total: 0, limit: 20, offset: 0 } }] },
      isLoading: false,
      isError: false,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
    } as Partial<UseInfiniteQueryResult<CharacterMessagesResponse>>);

    renderCharacterPage();

    expect(screen.queryByText('Player Character')).not.toBeInTheDocument();
    expect(screen.queryByText('NPC')).not.toBeInTheDocument();
    // Character name still renders
    expect(screen.getByText('Aelindra')).toBeInTheDocument();
  });

  describe('private message shortcut', () => {
    function renderLoadedPage() {
      vi.mocked(useQuery).mockReturnValue({
        data: mockCharacter,
        isLoading: false,
        isError: false,
      } as Partial<UseQueryResult<Character>>);

      vi.mocked(useCharacterCommentsModule.useCharacterComments).mockReturnValue({
        data: { pages: [{ messages: [], pagination: { total: 0, limit: 20, offset: 0 } }] },
        isLoading: false,
        isError: false,
        fetchNextPage: vi.fn(),
        hasNextPage: false,
        isFetchingNextPage: false,
      } as Partial<UseInfiniteQueryResult<CharacterMessagesResponse>>);

      return renderCharacterPage();
    }

    it('offers the envelope when the user may message this character', () => {
      vi.mocked(useCanMessageCharacter).mockReturnValue({ canMessage: true, gameId: 7 });

      renderLoadedPage();

      expect(
        screen.getByRole('button', { name: `Send a private message to ${mockCharacter.name}` })
      ).toBeInTheDocument();
    });

    it('omits the envelope when the user may not message this character', () => {
      renderLoadedPage();

      expect(
        screen.queryByRole('button', { name: /send a private message/i })
      ).not.toBeInTheDocument();
    });
  });
});
