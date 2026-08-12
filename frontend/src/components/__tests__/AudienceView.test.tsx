import { describe, it, expect, vi } from 'vitest';
import { screen } from '@testing-library/react';
import { renderWithProviders } from '../../test-utils/render';
import { AudienceView } from '../AudienceView';

vi.mock('../AllPrivateMessagesView', () => ({
  AllPrivateMessagesView: ({ gameId }: { gameId: number }) => (
    <div data-testid="all-private-messages">Private Messages {gameId}</div>
  ),
}));

describe('AudienceView', () => {
  it('renders the private messages view', () => {
    renderWithProviders(<AudienceView gameId={5} />);
    expect(screen.getByTestId('all-private-messages')).toBeInTheDocument();
  });

  it('passes gameId through', () => {
    renderWithProviders(<AudienceView gameId={5} />);
    expect(screen.getByText('Private Messages 5')).toBeInTheDocument();
  });

  // Action submissions and results moved to the History tab, which already
  // organises every role's view by phase. Keeping a second copy here meant two
  // sub-tabs reading different endpoints to render the same content.
  it('no longer offers an action submissions sub-tab', () => {
    renderWithProviders(<AudienceView gameId={5} />);

    expect(screen.queryByRole('button', { name: /action submissions/i })).not.toBeInTheDocument();
    expect(screen.queryByTestId('all-action-submissions')).not.toBeInTheDocument();
  });
});
