import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { CharacterPage } from '../CharacterPage';
import { apiClient } from '../../lib/api';
import type { Character } from '../../types/characters';

/**
 * CharacterPage shares the ['character', id] cache entry with CharacterSheet
 * and the avatar/update mutations, which invalidate that exact key. These
 * tests use a real QueryClient (rather than the blanket useQuery mock in
 * CharacterPage.test.tsx) because the thing under test *is* cache behavior.
 */

vi.mock('../../lib/api', () => ({
  apiClient: {
    characters: {
      getCharacter: vi.fn(),
      getCharacterData: vi.fn(),
      getCharacterComments: vi.fn(),
    },
    games: { getGame: vi.fn() },
  },
}));

vi.mock('../../hooks/useCharacterStats', () => ({
  useCharacterStats: () => ({ data: undefined, isLoading: false, isError: false }),
}));

vi.mock('../../hooks/useCanMessageCharacter', () => ({
  useCanMessageCharacter: () => ({ canMessage: false, gameId: undefined }),
}));

window.IntersectionObserver = vi.fn().mockReturnValue({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
}) as unknown as typeof IntersectionObserver;

const baseCharacter: Character = {
  id: 42,
  game_id: 1,
  name: 'Aelindra',
  character_type: 'player_character',
  status: 'approved',
  avatar_url: null,
  is_active: true,
  username: 'testplayer',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
};

function createClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
}

function renderPage(queryClient: QueryClient) {
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={['/characters/42']}>
        <Routes>
          <Route path="/characters/:characterId" element={<CharacterPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe('CharacterPage cache key', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(apiClient.characters.getCharacterData).mockResolvedValue({
      data: [],
    } as never);
    vi.mocked(apiClient.characters.getCharacterComments).mockResolvedValue({
      data: { messages: [], pagination: { total: 0, limit: 20, offset: 0 } },
    } as never);
    vi.mocked(apiClient.games.getGame).mockResolvedValue({
      data: { id: 1, portrait_avatars: false },
    } as never);
  });

  it('renders immediately from a character a sibling component already cached', async () => {
    const queryClient = createClient();
    // Stand in for CharacterSheet having already loaded this character.
    queryClient.setQueryData(['character', 42], baseCharacter);
    // Never resolves: if the page were reading a different key it would sit in
    // its loading skeleton forever instead of rendering the cached record.
    vi.mocked(apiClient.characters.getCharacter).mockReturnValue(
      new Promise(() => {}) as never
    );

    renderPage(queryClient);

    expect(screen.getByText('Aelindra')).toBeInTheDocument();
    expect(document.querySelector('.animate-pulse')).not.toBeInTheDocument();
  });

  it('re-renders with the new name after the shared key is invalidated', async () => {
    const queryClient = createClient();
    queryClient.setQueryData(['character', 42], baseCharacter);
    vi.mocked(apiClient.characters.getCharacter).mockResolvedValue({
      data: { ...baseCharacter, name: 'Aelindra the Renamed' },
    } as never);

    renderPage(queryClient);
    expect(await screen.findByText('Aelindra')).toBeInTheDocument();

    // Exactly what useCharacterAvatar / useCharacters do after a mutation.
    await queryClient.invalidateQueries({ queryKey: ['character', 42] });

    await waitFor(() => {
      expect(screen.getByText('Aelindra the Renamed')).toBeInTheDocument();
    });
  });
});
