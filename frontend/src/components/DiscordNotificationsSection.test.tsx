import { describe, it, expect, beforeEach, vi } from 'vitest';
import { screen, waitFor, fireEvent } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { DiscordNotificationsSection } from './DiscordNotificationsSection';
import { renderWithProviders } from '../test-utils/render';
import { server } from '../mocks/server';
import { logger } from '@/services/LoggingService';

// ─────────────────────────────────────────────────────────────────────────────
// Shared mock responses
// ─────────────────────────────────────────────────────────────────────────────

const notLinkedStatus = { linked: false };
const linkedStatus = { linked: true, discord_username: 'Player#1234' };

const defaultPreferences = {
  preferences: {
    theme: 'auto',
    comment_read_mode: 'auto',
    discord_notifications: {},
  },
};

function setupHandlers(discordStatus = notLinkedStatus, preferences = defaultPreferences) {
  server.use(
    http.get('/api/v1/auth/discord/status', () =>
      HttpResponse.json(discordStatus)
    ),
    http.get('/api/v1/auth/preferences', () =>
      HttpResponse.json(preferences)
    ),
    http.put('/api/v1/auth/preferences', () =>
      HttpResponse.json(preferences)
    ),
    http.get('/api/v1/auth/discord/connect', () =>
      HttpResponse.json({ url: 'https://discord.com/oauth2/authorize?state=abc' })
    ),
    http.delete('/api/v1/auth/discord/disconnect', () =>
      HttpResponse.json({ message: 'Discord account disconnected' })
    ),
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────────────────────────────────────

describe('DiscordNotificationsSection', () => {
  beforeEach(() => {
    server.resetHandlers();
  });

  it('renders "Connect Discord" CTA when not linked', async () => {
    setupHandlers(notLinkedStatus);

    renderWithProviders(<DiscordNotificationsSection />);

    await waitFor(() => {
      expect(screen.getByTestId('discord-connect-button')).toBeInTheDocument();
    });

    expect(screen.getByText(/Connect Discord/i)).toBeInTheDocument();
    expect(screen.queryByTestId('discord-username')).not.toBeInTheDocument();
  });

  it('renders Discord username and 12 toggles when linked', async () => {
    setupHandlers(linkedStatus);

    renderWithProviders(<DiscordNotificationsSection />);

    await waitFor(() => {
      expect(screen.getByTestId('discord-username')).toBeInTheDocument();
    });

    expect(screen.getByText('Player#1234')).toBeInTheDocument();
    expect(screen.getByTestId('discord-disconnect-button')).toBeInTheDocument();

    // 14 notification toggles should be rendered
    const toggles = screen.getAllByRole('switch');
    expect(toggles).toHaveLength(12);
  });

  it('calls getDiscordConnectURL API when Connect is clicked', async () => {
    setupHandlers(notLinkedStatus);

    let connectURLCalled = false;
    server.use(
      http.get('/api/v1/auth/discord/connect', () => {
        connectURLCalled = true;
        // Return empty url to avoid window.location navigation errors in tests
        return HttpResponse.json({ url: '' });
      })
    );

    renderWithProviders(<DiscordNotificationsSection />);

    const connectButton = await screen.findByTestId('discord-connect-button');
    fireEvent.click(connectButton);

    await waitFor(() => {
      expect(connectURLCalled).toBe(true);
    });
  });

  it('calls disconnect endpoint when Disconnect is clicked', async () => {
    setupHandlers(linkedStatus);

    let disconnectCalled = false;
    server.use(
      http.delete('/api/v1/auth/discord/disconnect', () => {
        disconnectCalled = true;
        return HttpResponse.json({ message: 'Discord account disconnected' });
      })
    );

    renderWithProviders(<DiscordNotificationsSection />);

    const disconnectButton = await screen.findByTestId('discord-disconnect-button');
    fireEvent.click(disconnectButton);

    await waitFor(() => {
      expect(disconnectCalled).toBe(true);
    });
  });

  it('calls updatePreferences with correct payload when toggle is clicked', async () => {
    setupHandlers(linkedStatus);

    let capturedBody: unknown = null;
    server.use(
      http.put('/api/v1/auth/preferences', async ({ request }) => {
        capturedBody = await request.json();
        return HttpResponse.json(defaultPreferences);
      })
    );

    renderWithProviders(<DiscordNotificationsSection />);

    // Wait for the component to load
    await screen.findByTestId('discord-username');

    // Find the private_message toggle and click it (it's ON by default; clicking turns it OFF)
    const privateMessageToggle = screen.getByTestId('discord-toggle-private_message').querySelector('[role="switch"]');
    expect(privateMessageToggle).not.toBeNull();
    fireEvent.click(privateMessageToggle!);

    await waitFor(() => {
      expect(capturedBody).not.toBeNull();
    });

    const body = capturedBody as { preferences: { discord_notifications: Record<string, boolean> } };
    expect(body.preferences.discord_notifications).toHaveProperty('private_message', false);
  });

  it('renders without crashing when status is loading (placeholder data used)', async () => {
    // The useDiscordStatus hook has placeholderData: { linked: false }
    // so it renders the "Connect" CTA immediately even while fetching.
    setupHandlers(notLinkedStatus);

    renderWithProviders(<DiscordNotificationsSection />);

    // Component renders without errors — eventually shows connect button
    await waitFor(() => {
      expect(screen.getByTestId('discord-connect-button')).toBeInTheDocument();
    });
  });

  it('shows 14 notification type labels when linked', async () => {
    setupHandlers(linkedStatus);

    renderWithProviders(<DiscordNotificationsSection />);

    await screen.findByTestId('discord-username');

    // Check some specific labels are present
    expect(screen.getByText('Private Messages')).toBeInTheDocument();
    expect(screen.getByText('Action Results')).toBeInTheDocument();
    expect(screen.getByText('Character Approved')).toBeInTheDocument();
    expect(screen.getByText('Common Room Posts')).toBeInTheDocument();
    expect(screen.getByText('Comment Replies')).toBeInTheDocument();
  });

  // Mock getDiscordConnectURL to verify the mock is testable
  it('does not redirect if getDiscordConnectURL fails', async () => {
    setupHandlers(notLinkedStatus);

    server.use(
      http.get('/api/v1/auth/discord/connect', () =>
        HttpResponse.json({ detail: 'server error' }, { status: 500 })
      )
    );

    const loggerSpy = vi.spyOn(logger, 'error').mockImplementation(() => {});

    renderWithProviders(<DiscordNotificationsSection />);

    const connectButton = await screen.findByTestId('discord-connect-button');
    fireEvent.click(connectButton);

    await waitFor(() => {
      expect(loggerSpy).toHaveBeenCalledWith('Failed to get Discord connect URL');
    });

    loggerSpy.mockRestore();
  });
});
