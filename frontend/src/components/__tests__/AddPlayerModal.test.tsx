import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { AxiosError } from 'axios';
import { screen, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test-utils/render';
import { AddParticipantModal } from '../AddParticipantModal';
import { AddPlayerModal } from '../AddPlayerModal';
import { AddAudienceMemberModal } from '../AddAudienceMemberModal';

vi.mock('../../lib/api', () => ({
  apiClient: {
    auth: {
      getCurrentUser: vi.fn().mockResolvedValue(null),
      searchUsers: vi.fn(),
    },
    games: {},
  },
}));

vi.mock('../../hooks/usePlayerManagement', () => ({
  useAddParticipant: vi.fn(),
}));

import { apiClient } from '../../lib/api';
import { useAddParticipant } from '../../hooks/usePlayerManagement';

const makeMutation = (overrides = {}) => ({
  mutateAsync: vi.fn().mockResolvedValue(undefined),
  isPending: false,
  isError: false,
  reset: vi.fn(),
  ...overrides,
});

describe('AddParticipantModal', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.clearAllMocks();
    vi.mocked(useAddParticipant).mockReturnValue(makeMutation() as never);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('renders player title and button label for role=player', () => {
    renderWithProviders(<AddParticipantModal gameId={10} role="player" isOpen onClose={vi.fn()} />);
    expect(screen.getByText('Add Player Directly')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /add player/i })).toBeDisabled();
  });

  it('renders audience title and button label for role=audience', () => {
    renderWithProviders(<AddParticipantModal gameId={10} role="audience" isOpen onClose={vi.fn()} />);
    expect(screen.getByText('Add Audience Member Directly')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /add audience member/i })).toBeDisabled();
  });

  it('passes correct role to useAddParticipant', () => {
    renderWithProviders(<AddParticipantModal gameId={10} role="audience" isOpen onClose={vi.fn()} />);
    expect(vi.mocked(useAddParticipant)).toHaveBeenCalledWith(10, 'audience');
  });

  it('shows search results dropdown after debounce', async () => {
    const user = userEvent.setup();
    vi.mocked(apiClient.auth.searchUsers).mockResolvedValue({
      data: { users: [{ id: 5, username: 'alice', created_at: '2024-01-01T00:00:00Z' }] },
    } as never);

    renderWithProviders(<AddParticipantModal gameId={10} role="player" isOpen onClose={vi.fn()} />);

    await user.type(screen.getByPlaceholderText(/type username to search/i), 'ali');
    await act(async () => { vi.runAllTimers(); });

    expect(screen.getByText('alice')).toBeInTheDocument();
  });

  it('selects a user from dropdown and enables submit button', async () => {
    const user = userEvent.setup();
    vi.mocked(apiClient.auth.searchUsers).mockResolvedValue({
      data: { users: [{ id: 5, username: 'alice', created_at: '2024-01-01T00:00:00Z' }] },
    } as never);

    renderWithProviders(<AddParticipantModal gameId={10} role="player" isOpen onClose={vi.fn()} />);

    await user.type(screen.getByPlaceholderText(/type username to search/i), 'ali');
    await act(async () => { vi.runAllTimers(); });
    await user.click(screen.getByText('alice'));

    expect(screen.getByRole('button', { name: /add player/i })).not.toBeDisabled();
    expect(screen.getByText(/selected: alice/i)).toBeInTheDocument();
  });

  it('calls mutateAsync with user id on submit and closes modal', async () => {
    const user = userEvent.setup();
    const mutation = makeMutation();
    vi.mocked(useAddParticipant).mockReturnValue(mutation as never);
    vi.mocked(apiClient.auth.searchUsers).mockResolvedValue({
      data: { users: [{ id: 5, username: 'alice', created_at: '2024-01-01T00:00:00Z' }] },
    } as never);
    const onClose = vi.fn();
    const onSuccess = vi.fn();

    renderWithProviders(
      <AddParticipantModal gameId={10} role="player" isOpen onClose={onClose} onSuccess={onSuccess} />
    );

    await user.type(screen.getByPlaceholderText(/type username to search/i), 'ali');
    await act(async () => { vi.runAllTimers(); });
    await user.click(screen.getByText('alice'));
    await user.click(screen.getByRole('button', { name: /add player/i }));

    await waitFor(() => {
      expect(mutation.mutateAsync).toHaveBeenCalledWith(5);
      expect(onClose).toHaveBeenCalled();
      expect(onSuccess).toHaveBeenCalled();
    });
  });

  it('shows no results message when search returns empty', async () => {
    const user = userEvent.setup();
    vi.mocked(apiClient.auth.searchUsers).mockResolvedValue({
      data: { users: [] },
    } as never);

    renderWithProviders(<AddParticipantModal gameId={10} role="player" isOpen onClose={vi.fn()} />);
    await user.type(screen.getByPlaceholderText(/type username to search/i), 'xyz');
    await act(async () => { vi.runAllTimers(); });

    expect(screen.getByText(/no users found matching "xyz"/i)).toBeInTheDocument();
  });

  it('calls onClose when Cancel is clicked', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    renderWithProviders(<AddParticipantModal gameId={10} role="player" isOpen onClose={onClose} />);
    await user.click(screen.getByRole('button', { name: /cancel/i }));
    expect(onClose).toHaveBeenCalled();
  });

  /**
   * Regression: a GM adding a community-banned user saw "They may already be in
   * the game, or the user may be invalid" -- both of which are false, and
   * neither of which the GM can act on.
   *
   * The server answers 403 with the actual reason; the modal was showing a
   * fixed string and discarding it.
   */
  it('shows the server error message instead of the generic fallback', async () => {
    const banned = new AxiosError('Request failed with status code 403');
    banned.response = {
      status: 403,
      data: { title: 'Forbidden', status: 403, detail: "that user is banned from this game's community" },
    } as AxiosError['response'];
    vi.mocked(useAddParticipant).mockReturnValue(
      makeMutation({ isError: true, error: banned }) as never
    );

    renderWithProviders(<AddParticipantModal gameId={10} role="player" isOpen onClose={vi.fn()} />);

    expect(
      screen.getByText("that user is banned from this game's community")
    ).toBeInTheDocument();
    expect(screen.queryByText(/may already be in the game/i)).not.toBeInTheDocument();
  });

  /**
   * The generic wording still has a job: it covers failures that carry no
   * server message at all, such as a dropped connection.
   */
  it('falls back to the generic message when the error carries no detail', () => {
    vi.mocked(useAddParticipant).mockReturnValue(
      makeMutation({ isError: true, error: new Error('Network Error') }) as never
    );

    renderWithProviders(<AddParticipantModal gameId={10} role="player" isOpen onClose={vi.fn()} />);

    expect(screen.getByText(/failed to add player/i)).toBeInTheDocument();
  });

  /**
   * The error box was red text on a red background -- 1.72:1 in dark mode,
   * against a WCAG AA floor of 4.5:1 -- because it hand-rolled
   * `text-semantic-danger` over `bg-semantic-danger-subtle`. Alert pairs that
   * same background with `text-content-primary`, which clears the bar in every
   * theme. Asserting on the role keeps this honest without pinning class names.
   */
  it('renders the error in an Alert rather than hand-rolled danger-on-danger text', () => {
    vi.mocked(useAddParticipant).mockReturnValue(
      makeMutation({ isError: true, error: new Error('Network Error') }) as never
    );

    renderWithProviders(<AddParticipantModal gameId={10} role="player" isOpen onClose={vi.fn()} />);

    const alert = screen.getByRole('alert');
    expect(alert).toHaveTextContent(/failed to add player/i);
    expect(alert.className).not.toMatch(/text-semantic-danger/);
  });

});

describe('AddPlayerModal wrapper', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAddParticipant).mockReturnValue(makeMutation() as never);
  });

  it('renders with player role', () => {
    renderWithProviders(<AddPlayerModal gameId={10} isOpen onClose={vi.fn()} />);
    expect(screen.getByText('Add Player Directly')).toBeInTheDocument();
  });
});

describe('AddAudienceMemberModal wrapper', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAddParticipant).mockReturnValue(makeMutation() as never);
  });

  it('renders with audience role', () => {
    renderWithProviders(<AddAudienceMemberModal gameId={10} isOpen onClose={vi.fn()} />);
    expect(screen.getByText('Add Audience Member Directly')).toBeInTheDocument();
  });
});
