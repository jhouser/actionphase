import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { NewConversationModal } from './NewConversationModal';
import type { Character } from '../types/characters';

vi.mock('../lib/api', () => ({
  apiClient: { conversations: { createConversation: vi.fn() } },
}));

import { apiClient } from '../lib/api';
const mockCreate = vi.mocked(apiClient.conversations.createConversation);

function character(overrides: Partial<Character> = {}): Character {
  return {
    id: 1,
    game_id: 7,
    name: 'Char',
    status: 'approved',
    is_active: true,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
    ...overrides,
  };
}

const mine = character({ id: 10, name: 'Rook' });
const vesper = character({ id: 42, name: 'Vesper' });
const other = character({ id: 43, name: 'Juno' });

function renderModal(props: Partial<Parameters<typeof NewConversationModal>[0]> = {}) {
  return render(
    <NewConversationModal
      gameId={7}
      characters={[mine]}
      allCharacters={[mine, vesper, other]}
      isAnonymous={false}
      allowGroupConversations
      onClose={vi.fn()}
      onConversationCreated={vi.fn()}
      {...props}
    />
  );
}

describe('NewConversationModal pre-selected participants', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCreate.mockResolvedValue({ data: { id: 99 } } as never);
  });

  it('checks the pre-selected participant', () => {
    renderModal({ initialParticipantIds: [42] });

    expect(screen.getByRole('checkbox', { name: /vesper/i })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: /juno/i })).not.toBeChecked();
  });

  it('submits the pre-selected participant alongside the sending character', async () => {
    const user = userEvent.setup();
    const onConversationCreated = vi.fn();
    renderModal({ initialParticipantIds: [42], onConversationCreated });

    await user.type(screen.getByLabelText(/conversation title/i), 'Quiet word');
    await user.click(screen.getByRole('button', { name: 'Create Conversation' }));

    await waitFor(() =>
      expect(mockCreate).toHaveBeenCalledWith(7, {
        title: 'Quiet word',
        character_ids: [10, 42],
      })
    );
    expect(onConversationCreated).toHaveBeenCalledWith(99);
  });

  it('selects the pre-selected participant in the dropdown when group conversations are off', async () => {
    const user = userEvent.setup();
    renderModal({ initialParticipantIds: [42], allowGroupConversations: false });

    expect(screen.getByLabelText(/participant/i)).toHaveValue('42');

    await user.type(screen.getByLabelText(/conversation title/i), 'Quiet word');
    await user.click(screen.getByRole('button', { name: 'Create Conversation' }));

    await waitFor(() =>
      expect(mockCreate).toHaveBeenCalledWith(7, {
        title: 'Quiet word',
        character_ids: [10, 42],
      })
    );
  });

  it('starts with nothing selected when no pre-selection is given', () => {
    renderModal();
    expect(screen.getByRole('checkbox', { name: /vesper/i })).not.toBeChecked();
    expect(screen.getByRole('checkbox', { name: /juno/i })).not.toBeChecked();
  });

  it('drops a pre-selection that is also the sending character', async () => {
    const user = userEvent.setup();
    // Only one controllable character, so it is auto-selected as the sender.
    renderModal({ characters: [mine], initialParticipantIds: [mine.id, vesper.id] });

    await waitFor(() =>
      expect(screen.getByText(/1 participant selected/i)).toBeInTheDocument()
    );

    await user.type(screen.getByLabelText(/conversation title/i), 'Quiet word');
    await user.click(screen.getByRole('button', { name: 'Create Conversation' }));

    await waitFor(() =>
      expect(mockCreate).toHaveBeenCalledWith(7, {
        title: 'Quiet word',
        character_ids: [10, 42],
      })
    );
  });
});
