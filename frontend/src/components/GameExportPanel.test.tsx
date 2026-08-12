import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { GameExportPanel } from './GameExportPanel';
import { apiClient } from '../lib/api';
import type { GameExport } from '../types/exports';

vi.mock('../lib/api', () => ({
  apiClient: {
    exports: {
      requestExport: vi.fn(),
      getLatestExport: vi.fn(),
      getDownloadUrl: (id: number) => `/api/v1/exports/${id}/download`,
    },
  },
}));

const mockedApi = vi.mocked(apiClient.exports);

function renderPanel(props: { gameId?: number; isCompleted?: boolean } = {}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <GameExportPanel gameId={props.gameId ?? 164} isCompleted={props.isCompleted ?? true} />
    </QueryClientProvider>
  );
}

/** Builds a notFound-shaped rejection matching axios error structure. */
function notFoundError() {
  return Object.assign(new Error('Request failed'), { response: { status: 404 } });
}

const completeExport: GameExport = {
  id: 6,
  game_id: 164,
  status: 'complete',
  size_bytes: 3748,
  file_count: 7,
  download_url: '/api/v1/exports/6/download',
  created_at: '2026-08-05T19:56:24Z',
  completed_at: '2026-08-05T19:56:36Z',
};

describe('GameExportPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('visibility', () => {
    it('renders nothing for a game that has not completed', () => {
      renderPanel({ isCompleted: false });

      expect(screen.queryByTestId('game-export-panel')).not.toBeInTheDocument();
    });

    it('does not query the API for a non-completed game', () => {
      renderPanel({ isCompleted: false });

      // Exports only exist for completed games; querying would be a certain 404.
      expect(mockedApi.getLatestExport).not.toHaveBeenCalled();
    });

    it('renders the panel for a completed game', async () => {
      mockedApi.getLatestExport.mockRejectedValue(notFoundError());
      renderPanel();

      expect(await screen.findByTestId('game-export-panel')).toBeInTheDocument();
    });
  });

  describe('no existing export', () => {
    it('offers to prepare a download when none exists', async () => {
      mockedApi.getLatestExport.mockRejectedValue(notFoundError());
      renderPanel();

      expect(await screen.findByTestId('export-request-button')).toHaveTextContent(
        'Prepare download'
      );
    });

    it('requests an export when the button is clicked', async () => {
      mockedApi.getLatestExport.mockRejectedValue(notFoundError());
      mockedApi.requestExport.mockResolvedValue({
        data: { id: 1, game_id: 164, status: 'pending' },
      } as never);

      renderPanel();
      await userEvent.click(await screen.findByTestId('export-request-button'));

      await waitFor(() => {
        expect(mockedApi.requestExport).toHaveBeenCalledWith(164);
      });
    });
  });

  describe('in progress', () => {
    it('shows a disabled preparing button while running', async () => {
      mockedApi.getLatestExport.mockResolvedValue({
        data: { id: 1, game_id: 164, status: 'running', progress: 'writing polls' },
      } as never);

      renderPanel();

      expect(await screen.findByTestId('export-working-button')).toBeDisabled();
      expect(screen.getByTestId('export-status-badge')).toHaveTextContent('Preparing');
    });

    it('surfaces the progress note from the worker', async () => {
      mockedApi.getLatestExport.mockResolvedValue({
        data: { id: 1, game_id: 164, status: 'running', progress: 'writing private conversations' },
      } as never);

      renderPanel();

      expect(await screen.findByTestId('export-progress')).toHaveTextContent(
        'writing private conversations'
      );
    });

    it('falls back to a queued label when pending with no note', async () => {
      mockedApi.getLatestExport.mockResolvedValue({
        data: { id: 1, game_id: 164, status: 'pending' },
      } as never);

      renderPanel();

      expect(await screen.findByTestId('export-progress')).toHaveTextContent('Queued');
    });

    it('does not offer a download while still working', async () => {
      mockedApi.getLatestExport.mockResolvedValue({
        data: { id: 1, game_id: 164, status: 'running' },
      } as never);

      renderPanel();

      await screen.findByTestId('export-working-button');
      expect(screen.queryByTestId('export-download-button')).not.toBeInTheDocument();
    });
  });

  describe('complete', () => {
    it('offers a download with file count and size', async () => {
      mockedApi.getLatestExport.mockResolvedValue({ data: completeExport } as never);

      renderPanel();

      expect(await screen.findByTestId('export-download-button')).toHaveTextContent(
        'Download ZIP'
      );
      // No "Ready" badge: the Download button already conveys readiness, and a
      // second indicator is redundant in a one-line control.
      expect(screen.queryByTestId('export-status-badge')).not.toBeInTheDocument();
      expect(screen.getByTestId('export-meta')).toHaveTextContent('7 files');
      expect(screen.getByTestId('export-meta')).toHaveTextContent('3.7 KB');
    });

    // Completed games are read-only, so rebuilding could only ever produce a
    // byte-identical archive while consuming more storage.
    it('offers no rebuild action', async () => {
      mockedApi.getLatestExport.mockResolvedValue({ data: completeExport } as never);

      renderPanel();

      await screen.findByTestId('export-download-button');
      expect(screen.queryByTestId('export-refresh-button')).not.toBeInTheDocument();
      expect(screen.queryByText(/rebuild/i)).not.toBeInTheDocument();
    });

    // The expiry date is noise: archives regenerate on demand, so there is
    // nothing for a reader to act on before that date.
    it('does not show an expiry date', async () => {
      mockedApi.getLatestExport.mockResolvedValue({
        data: { ...completeExport, expires_at: '2026-08-12T19:56:36Z' },
      } as never);

      renderPanel();

      expect(await screen.findByTestId('export-meta')).not.toHaveTextContent(/available until/i);
      expect(screen.queryByText(/available until/i)).not.toBeInTheDocument();
    });

    // Archiving is a secondary action, so it must not reintroduce a heavy
    // block: the description lives in a tooltip rather than body text.
    it('keeps details in a tooltip rather than body text', async () => {
      mockedApi.getLatestExport.mockResolvedValue({ data: completeExport } as never);

      renderPanel();

      await screen.findByTestId('export-download-button');
      // Uses the shared HelpTooltip, which renders a real hover panel — a bare
      // `title` attribute renders inconsistently and was not visible in-browser.
      expect(screen.getByRole('tooltip')).toHaveTextContent('ZIP of Markdown files');
      expect(screen.queryByRole('heading', { name: /archive this game/i })).not.toBeInTheDocument();
    });
  });

  describe('expired', () => {
    const expiredExport: GameExport = {
      id: 6,
      game_id: 164,
      status: 'complete',
      expired: true,
      size_bytes: 3748,
      file_count: 7,
      completed_at: '2026-07-01T00:00:00Z',
    };

    it('offers to prepare a download rather than a dead link', async () => {
      mockedApi.getLatestExport.mockResolvedValue({ data: expiredExport } as never);

      renderPanel();

      expect(await screen.findByTestId('export-request-button')).toHaveTextContent(
        'Prepare download'
      );
      expect(screen.queryByTestId('export-download-button')).not.toBeInTheDocument();
    });

    // An expired export and a never-created one call for the same action, so
    // the UI must not distinguish them: retention is not the reader's concern.
    it('is indistinguishable from having no export at all', async () => {
      mockedApi.getLatestExport.mockResolvedValue({ data: expiredExport } as never);
      const { unmount } = renderPanel();
      const expiredMarkup = (await screen.findByTestId('game-export-panel')).innerHTML;
      unmount();

      mockedApi.getLatestExport.mockRejectedValue(notFoundError());
      renderPanel();
      const noExportMarkup = (await screen.findByTestId('game-export-panel')).innerHTML;

      expect(expiredMarkup).toBe(noExportMarkup);
    });

    it('does not explain retention to the reader', async () => {
      mockedApi.getLatestExport.mockResolvedValue({ data: expiredExport } as never);

      renderPanel();

      await screen.findByTestId('export-request-button');
      expect(screen.queryByTestId('export-expired-note')).not.toBeInTheDocument();
      expect(screen.queryByText(/expired/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/again/i)).not.toBeInTheDocument();
    });

    it('requests a fresh export when clicked', async () => {
      mockedApi.getLatestExport.mockResolvedValue({ data: expiredExport } as never);
      mockedApi.requestExport.mockResolvedValue({
        data: { id: 7, game_id: 164, status: 'pending' },
      } as never);

      renderPanel();
      await userEvent.click(await screen.findByTestId('export-request-button'));

      await waitFor(() => {
        expect(mockedApi.requestExport).toHaveBeenCalledWith(164);
      });
    });
  });

  describe('failure', () => {
    it('shows the failure reason and offers a retry', async () => {
      mockedApi.getLatestExport.mockResolvedValue({
        data: {
          id: 1,
          game_id: 164,
          status: 'failed',
          error: 'upload archive: disk full',
        },
      } as never);

      renderPanel();

      expect(await screen.findByTestId('export-failed-alert')).toHaveTextContent('disk full');
      expect(screen.getByTestId('export-request-button')).toHaveTextContent('Try again');
    });

    it('surfaces an error when the request itself fails', async () => {
      mockedApi.getLatestExport.mockRejectedValue(notFoundError());
      mockedApi.requestExport.mockRejectedValue(new Error('only completed games can be exported'));

      renderPanel();
      await userEvent.click(await screen.findByTestId('export-request-button'));

      expect(await screen.findByTestId('export-request-error')).toHaveTextContent(
        'only completed games can be exported'
      );
    });
  });
});
