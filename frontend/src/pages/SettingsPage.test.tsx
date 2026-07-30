import { describe, it, expect, beforeEach } from 'vitest';
import { screen, waitFor, fireEvent } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { SettingsPage } from './SettingsPage';
import { ThemeProvider } from '../contexts/ThemeContext';
import { renderWithProviders } from '../test-utils/render';
import { server } from '../mocks/server';

const defaultPreferences = {
  preferences: {
    theme: 'auto',
    comment_read_mode: 'manual',
    font_size: 'medium',
  },
};

function setupHandlers(preferences = defaultPreferences) {
  server.use(
    http.get('/api/v1/auth/preferences', () => HttpResponse.json(preferences)),
    http.put('/api/v1/auth/preferences', async ({ request }) => {
      const body = (await request.json()) as { preferences: unknown };
      return HttpResponse.json({ preferences: body.preferences });
    })
  );
}

describe('SettingsPage - Appearance - Text Size', () => {
  beforeEach(() => {
    server.resetHandlers();

    // ThemeProvider queries prefers-color-scheme; jsdom has no matchMedia implementation.
    window.matchMedia = window.matchMedia || ((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    })) as unknown as typeof window.matchMedia;
  });

  it('renders Small/Medium/Large text size options with Medium selected by default', async () => {
    setupHandlers();

    renderWithProviders(
      <ThemeProvider>
        <SettingsPage />
      </ThemeProvider>,
      { initialRoute: '/settings?tab=appearance' }
    );

    await waitFor(() => {
      expect(screen.getByTestId('font-size-medium')).toBeInTheDocument();
    });

    expect(screen.getByTestId('font-size-small')).toBeInTheDocument();
    expect(screen.getByTestId('font-size-large')).toBeInTheDocument();

    const mediumRadio = screen.getByTestId('font-size-medium').querySelector('input[type="radio"]');
    expect(mediumRadio).toBeChecked();
  });

  it('calls updatePreferences with font_size and preserves other fields when Large is selected', async () => {
    setupHandlers();

    let capturedBody: unknown = null;
    server.use(
      http.put('/api/v1/auth/preferences', async ({ request }) => {
        capturedBody = await request.json();
        return HttpResponse.json({
          preferences: { theme: 'auto', comment_read_mode: 'manual', font_size: 'large' },
        });
      })
    );

    renderWithProviders(
      <ThemeProvider>
        <SettingsPage />
      </ThemeProvider>,
      { initialRoute: '/settings?tab=appearance' }
    );

    const largeRow = await screen.findByTestId('font-size-large');
    const largeRadio = largeRow.querySelector('input[type="radio"]');
    expect(largeRadio).not.toBeNull();
    fireEvent.click(largeRadio!);

    await waitFor(() => {
      expect(capturedBody).not.toBeNull();
    });

    const body = capturedBody as { preferences: { theme: string; comment_read_mode: string; font_size: string } };
    expect(body.preferences.font_size).toBe('large');
    expect(body.preferences.theme).toBe('auto');
    expect(body.preferences.comment_read_mode).toBe('manual');
  });
});
