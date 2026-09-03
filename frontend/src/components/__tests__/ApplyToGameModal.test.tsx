import { describe, it, expect, vi, beforeEach } from 'vitest';
import { AxiosError } from 'axios';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test-utils/render';
import { ApplyToGameModal } from '../ApplyToGameModal';

vi.mock('../../lib/api', () => ({
  apiClient: {
    auth: { getCurrentUser: vi.fn().mockResolvedValue(null) },
    games: {
      applyToGame: vi.fn(),
    },
  },
}));

import { apiClient } from '../../lib/api';

const defaultProps = {
  gameId: 10,
  gameTitle: 'Dragon Quest',
  isOpen: true,
  onClose: vi.fn(),
  onApplicationSubmitted: vi.fn(),
};

describe('ApplyToGameModal', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders modal with game title and submit button', () => {
    renderWithProviders(<ApplyToGameModal {...defaultProps} />);
    expect(screen.getByText(/apply to dragon quest/i)).toBeInTheDocument();
    expect(screen.getByTestId('submit-application')).toBeInTheDocument();
  });

  it('shows role select for non-audienceOnly mode', () => {
    renderWithProviders(<ApplyToGameModal {...defaultProps} />);
    expect(screen.getByRole('combobox')).toBeInTheDocument();
  });

  it('hides role select in audienceOnly mode', () => {
    renderWithProviders(<ApplyToGameModal {...defaultProps} audienceOnly />);
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
    expect(screen.getByTestId('submit-application')).toHaveTextContent(/join as audience/i);
  });

  it('shows auto-accept notice when autoAcceptAudience and role is audience', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ApplyToGameModal {...defaultProps} autoAcceptAudience />);
    // Switch to audience role
    await user.selectOptions(screen.getByRole('combobox'), 'audience');
    expect(screen.getByTestId('auto-accept-notice')).toBeInTheDocument();
  });

  it('does not show auto-accept notice for player role', () => {
    renderWithProviders(<ApplyToGameModal {...defaultProps} autoAcceptAudience />);
    expect(screen.queryByTestId('auto-accept-notice')).not.toBeInTheDocument();
  });

  it('calls applyToGame and fires callbacks on successful submit', async () => {
    const user = userEvent.setup();
    vi.mocked(apiClient.games.applyToGame).mockResolvedValue(undefined as never);
    const onApplicationSubmitted = vi.fn();
    const onClose = vi.fn();
    renderWithProviders(
      <ApplyToGameModal
        {...defaultProps}
        onApplicationSubmitted={onApplicationSubmitted}
        onClose={onClose}
      />
    );
    await user.type(screen.getByRole('textbox', { name: /application message/i }), 'I love this genre!');
    await user.click(screen.getByTestId('submit-application'));
    await waitFor(() => {
      expect(apiClient.games.applyToGame).toHaveBeenCalledWith(10, {
        role: 'player',
        message: 'I love this genre!',
      });
      expect(onApplicationSubmitted).toHaveBeenCalled();
      expect(onClose).toHaveBeenCalled();
    });
  });

  it('omits empty message from submission', async () => {
    const user = userEvent.setup();
    vi.mocked(apiClient.games.applyToGame).mockResolvedValue(undefined as never);
    renderWithProviders(<ApplyToGameModal {...defaultProps} />);
    await user.click(screen.getByTestId('submit-application'));
    await waitFor(() => {
      expect(apiClient.games.applyToGame).toHaveBeenCalledWith(10, {
        role: 'player',
        message: undefined,
      });
    });
  });

  it('shows error alert on API failure', async () => {
    const user = userEvent.setup();
    vi.mocked(apiClient.games.applyToGame).mockRejectedValue(new Error('Already applied'));
    renderWithProviders(<ApplyToGameModal {...defaultProps} />);
    await user.click(screen.getByTestId('submit-application'));
    await waitFor(() => {
      expect(screen.getByText('Already applied')).toBeInTheDocument();
    });
  });

  /**
   * Regression: a community ban surfaced as "Request failed with status code
   * 403" instead of the reason the server actually sent.
   *
   * The backend renders errors as {status, error} -- there is no `message` key
   * -- so reading response.data.message always found undefined and fell through
   * to axios's own generic string. Building the rejection as a real AxiosError
   * is the point of the test: the previous version rejected with a plain Error,
   * which is why the bug survived a passing suite.
   */
  it('surfaces the server error message on a 403 rather than the axios default', async () => {
    const user = userEvent.setup();
    const banned = new AxiosError('Request failed with status code 403');
    banned.response = {
      status: 403,
      data: { status: 'Forbidden.', error: 'you are banned from this community' },
    } as AxiosError['response'];
    vi.mocked(apiClient.games.applyToGame).mockRejectedValue(banned);

    renderWithProviders(<ApplyToGameModal {...defaultProps} />);
    await user.click(screen.getByTestId('submit-application'));

    await waitFor(() => {
      expect(screen.getByText('you are banned from this community')).toBeInTheDocument();
    });
    expect(
      screen.queryByText(/request failed with status code/i)
    ).not.toBeInTheDocument();
  });

  /**
   * The server sends `status` alone when it has no specific detail to add.
   * Falling back to it beats showing the axios string, which names a number the
   * user cannot act on.
   */
  it('falls back to the status field when no error detail is present', async () => {
    const user = userEvent.setup();
    const failure = new AxiosError('Request failed with status code 403');
    failure.response = {
      status: 403,
      data: { status: 'Forbidden.' },
    } as AxiosError['response'];
    vi.mocked(apiClient.games.applyToGame).mockRejectedValue(failure);

    renderWithProviders(<ApplyToGameModal {...defaultProps} />);
    await user.click(screen.getByTestId('submit-application'));

    await waitFor(() => {
      expect(screen.getByText('Forbidden.')).toBeInTheDocument();
    });
  });

  /**
   * The region stays mounted (and empty, so it occupies no height) rather than
   * being conditionally rendered: a live region has to exist before the error
   * lands in it for screen readers to announce the change.
   */
  it('keeps the error region mounted so the error is announced in place', async () => {
    const user = userEvent.setup();
    const failure = new AxiosError('Request failed with status code 403');
    failure.response = {
      status: 403,
      data: { status: 'Forbidden.', error: 'you are banned from this community' },
    } as AxiosError['response'];
    vi.mocked(apiClient.games.applyToGame).mockRejectedValue(failure);

    renderWithProviders(<ApplyToGameModal {...defaultProps} />);
    const region = screen.getByTestId('application-error');
    expect(region).toBeInTheDocument();
    expect(region).toBeEmptyDOMElement();

    await user.click(screen.getByTestId('submit-application'));

    await waitFor(() => {
      expect(screen.getByText('you are banned from this community')).toBeInTheDocument();
    });
    // Same node, now filled -- not a sibling inserted above the form.
    expect(screen.getByTestId('application-error')).toBe(region);
  });

  it('calls onClose when Cancel is clicked (not submitting)', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    renderWithProviders(<ApplyToGameModal {...defaultProps} onClose={onClose} />);
    await user.click(screen.getByRole('button', { name: /cancel/i }));
    expect(onClose).toHaveBeenCalled();
  });
});
