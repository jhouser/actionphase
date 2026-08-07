/**
 * Status of an archive export job.
 *
 * - pending: queued, not yet claimed by the worker
 * - running: being assembled
 * - complete: artifact stored and downloadable
 * - failed: assembly or upload failed; `error` explains why
 */
export type GameExportStatus = 'pending' | 'running' | 'complete' | 'failed';

/** An archive export job as returned by the API. */
export interface GameExport {
  id: number;
  game_id: number;
  status: GameExportStatus;
  /** Human-readable step, present only while running. */
  progress?: string;
  /** Failure reason, present only when status is 'failed'. */
  error?: string;
  size_bytes?: number;
  file_count?: number;
  /** Present only when status is 'complete' and the artifact still exists. */
  download_url?: string;
  created_at?: string;
  completed_at?: string;
  /** When the stored archive will be reclaimed. */
  expires_at?: string;
  /**
   * True for a completed export whose archive has passed its retention window
   * and been deleted.
   *
   * Reported by the API but deliberately not surfaced in the UI: an expired
   * export and a never-created one call for the same action — generate the
   * archive — so distinguishing them would only add a label the reader cannot
   * act on. Absence of `download_url` is what drives the UI.
   */
  expired?: boolean;
}

/** True when the job is still being worked on and should be polled. */
export function isExportInProgress(status: GameExportStatus): boolean {
  return status === 'pending' || status === 'running';
}
