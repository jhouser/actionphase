import React, { useCallback } from 'react';
import { Alert, Badge, Button, HelpTooltip, Spinner } from './ui';
import { useGameExport } from '../hooks/useGameExport';

interface GameExportPanelProps {
  gameId: number;
  /**
   * Whether the game has completed. Exports exist only for completed games, so
   * the panel renders nothing otherwise rather than offering a doomed action.
   */
  isCompleted: boolean;
}

/** Renders a byte count as a short human-readable size. */
function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/**
 * What the archive contains. Shown on hover/focus rather than as body text:
 * this is reference detail a reader wants once, not every visit.
 */
const ARCHIVE_TOOLTIP =
  'Downloads a ZIP of Markdown files: common room posts with their full ' +
  'comment threads, private messages, actions and results, character sheets, ' +
  'polls, and handouts.';

/**
 * Compact download control for a completed game's archive.
 *
 * Deliberately a single row rather than a card: archiving is an occasional,
 * secondary action, so it earns a line in the Game Info tab rather than space
 * above the game itself. Details live in a tooltip for the same reason.
 */
export const GameExportPanel: React.FC<GameExportPanelProps> = ({ gameId, isCompleted }) => {
  const {
    export: gameExport,
    isWorking,
    isLoading,
    isRequesting,
    requestError,
    requestExport,
    dismissError,
  } = useGameExport(gameId, { enabled: isCompleted });

  const handleRequest = useCallback(() => {
    requestExport();
  }, [requestExport]);

  if (!isCompleted) return null;

  // A downloadable archive requires a URL: an export whose artifact was
  // reclaimed after its retention window reports 'complete' but has none, and
  // is therefore treated exactly like no export at all — the action in both
  // cases is to generate one.
  const isReady = gameExport?.status === 'complete' && !!gameExport.download_url;
  const hasFailed = gameExport?.status === 'failed';

  return (
    <div data-testid="game-export-panel">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <div className="flex items-center gap-1.5 min-w-0">
          <span className="text-xs font-semibold uppercase tracking-wider text-content-secondary">
            Archive
          </span>
          <span data-testid="export-info-icon">
            <HelpTooltip text={ARCHIVE_TOOLTIP} />
          </span>
          {isWorking && (
            <Badge variant="warning" data-testid="export-status-badge">
              Preparing
            </Badge>
          )}
        </div>

        {isLoading ? (
          <Spinner size="sm" />
        ) : isWorking ? (
          <Button variant="secondary" size="sm" disabled data-testid="export-working-button">
            <span className="flex items-center gap-2">
              <Spinner size="sm" />
              Preparing…
            </span>
          </Button>
        ) : isReady && gameExport?.download_url ? (
          // No "rebuild" action: a completed game is read-only, so
          // regenerating would produce a byte-identical archive and only
          // consume storage.
          <Button
            variant="secondary"
            size="sm"
            onClick={() => {
              window.location.href = gameExport.download_url as string;
            }}
            data-testid="export-download-button"
          >
            Download ZIP
          </Button>
        ) : (
          <Button
            variant="secondary"
            size="sm"
            onClick={handleRequest}
            loading={isRequesting}
            data-testid="export-request-button"
          >
            {hasFailed ? 'Try again' : 'Prepare download'}
          </Button>
        )}

        {/* Size/count sit beside the button as a quiet caption. The expiry date
            is deliberately omitted: the archive regenerates on demand, so it is
            not something a reader needs to act on. */}
        {isReady && gameExport && (
          <span className="text-xs text-content-tertiary" data-testid="export-meta">
            {gameExport.file_count ?? 0} files
            {gameExport.size_bytes ? ` · ${formatBytes(gameExport.size_bytes)}` : ''}
          </span>
        )}
        {isWorking && (
          <span className="text-xs text-content-tertiary" data-testid="export-progress">
            {gameExport?.progress ?? 'Queued…'}
          </span>
        )}
      </div>

      {hasFailed && gameExport?.error && (
        <Alert variant="danger" className="mt-2" data-testid="export-failed-alert">
          Export failed: {gameExport.error}
        </Alert>
      )}

      {requestError && (
        <Alert
          variant="danger"
          className="mt-2"
          dismissible
          onDismiss={dismissError}
          data-testid="export-request-error"
        >
          Could not start the export. {requestError.message}
        </Alert>
      )}
    </div>
  );
};
