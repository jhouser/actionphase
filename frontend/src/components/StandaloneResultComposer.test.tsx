import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { StandaloneResultComposer } from './StandaloneResultComposer';
import type { Character } from '../types/characters';

const mockCreateResult = vi.fn();

// The real form pulls in CommentEditor and the mutation stack; this stand-in
// reports the recipient props it was handed, which is what the picker owns.
vi.mock('./CreateActionResultForm', () => ({
  CreateActionResultForm: ({
    userId,
    userName,
    characterId,
    characterName,
  }: {
    userId: number;
    userName: string;
    characterId?: number;
    characterName?: string;
  }) => (
    <div
      data-testid="create-result-form"
      data-user-id={userId}
      data-user-name={userName}
      data-character-id={characterId ?? 'undefined'}
      data-character-name={characterName ?? 'undefined'}
    />
  ),
}));

let mockCharacters: Character[] = [];

vi.mock('../contexts/GameContext', () => ({
  useGameContext: () => ({ allGameCharacters: mockCharacters }),
}));

const makeCharacter = (overrides: Partial<Character> & { id: number; name: string }): Character => ({
  game_id: 100,
  status: 'approved',
  is_active: true,
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString(),
  ...overrides,
});

const renderComposer = () =>
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <StandaloneResultComposer gameId={100} />
    </QueryClientProvider>
  );

describe('StandaloneResultComposer', () => {
  beforeEach(() => {
    mockCreateResult.mockReset();
    mockCharacters = [];
  });

  it('does not render the result form until a recipient is chosen', () => {
    mockCharacters = [makeCharacter({ id: 1, name: 'Vera', user_id: 42, username: 'vera_player' })];

    renderComposer();

    expect(screen.queryByTestId('create-result-form')).not.toBeInTheDocument();
    expect(screen.getByText(/choose a recipient/i)).toBeInTheDocument();
  });

  it('passes the selected character and its controlling user to the form', async () => {
    mockCharacters = [
      makeCharacter({ id: 1, name: 'Vera', user_id: 42, username: 'vera_player' }),
      makeCharacter({ id: 2, name: 'Cass', user_id: 43, username: 'cass_player' }),
    ];

    renderComposer();
    await userEvent.selectOptions(screen.getByTestId('standalone-result-recipient'), '2');

    const form = screen.getByTestId('create-result-form');
    expect(form).toHaveAttribute('data-user-id', '43');
    expect(form).toHaveAttribute('data-user-name', 'cass_player');
    expect(form).toHaveAttribute('data-character-id', '2');
    expect(form).toHaveAttribute('data-character-name', 'Cass');
  });

  // An NPC has no owning user_id but may be assigned to a player, who is the
  // one who actually receives the result.
  it('routes an assigned NPC to its assigned player', async () => {
    mockCharacters = [
      makeCharacter({
        id: 7,
        name: 'The Innkeeper',
        character_type: 'npc',
        assigned_user_id: 99,
        assigned_username: 'npc_handler',
      }),
    ];

    renderComposer();
    await userEvent.selectOptions(screen.getByTestId('standalone-result-recipient'), '7');

    const form = screen.getByTestId('create-result-form');
    expect(form).toHaveAttribute('data-user-id', '99');
    expect(form).toHaveAttribute('data-user-name', 'npc_handler');
  });

  // A result is delivered to a user, so a character nobody controls is not a
  // valid recipient and must not be offered.
  it('omits characters with no controlling user', () => {
    mockCharacters = [
      makeCharacter({ id: 1, name: 'Vera', user_id: 42, username: 'vera_player' }),
      makeCharacter({ id: 8, name: 'Unclaimed NPC', character_type: 'npc' }),
    ];

    renderComposer();

    expect(screen.getByRole('option', { name: /Vera/ })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: /Unclaimed NPC/ })).not.toBeInTheDocument();
  });

  it('omits pending and inactive characters', () => {
    mockCharacters = [
      makeCharacter({ id: 1, name: 'Approved', user_id: 42, username: 'a' }),
      makeCharacter({ id: 2, name: 'Pending', user_id: 43, username: 'b', status: 'pending' }),
      makeCharacter({ id: 3, name: 'Retired', user_id: 44, username: 'c', is_active: false }),
    ];

    renderComposer();

    expect(screen.getByRole('option', { name: /Approved/ })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: /Pending/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('option', { name: /Retired/ })).not.toBeInTheDocument();
  });

  // is_active is cleared only when a character's player is removed from the
  // game, leaving no one to deliver a result to.
  it('omits characters orphaned by a removed player', () => {
    mockCharacters = [
      makeCharacter({ id: 1, name: 'Vera', user_id: 42, username: 'vera_player' }),
      makeCharacter({
        id: 2,
        name: 'Departed',
        user_id: 43,
        username: 'gone_player',
        is_active: false,
      }),
    ];

    renderComposer();

    expect(screen.getByRole('option', { name: /Vera/ })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: /Departed/ })).not.toBeInTheDocument();
  });

  it('explains itself when no character is addressable', () => {
    mockCharacters = [makeCharacter({ id: 8, name: 'Unclaimed NPC', character_type: 'npc' })];

    renderComposer();

    expect(screen.getByText(/no approved characters with an assigned player/i)).toBeInTheDocument();
    expect(screen.queryByTestId('standalone-result-recipient')).not.toBeInTheDocument();
  });
});
