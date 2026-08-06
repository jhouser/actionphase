import { BaseApiClient } from './client';
import type { GameExport } from '../../types/exports';

/**
 * Game archive exports API client.
 *
 * Exports are only available for completed games: the archive contains private
 * conversations, action submissions, and published results, which are readable
 * by any authenticated user once a game enters public archive mode.
 */
export class ExportsApi extends BaseApiClient {
  /**
   * Request an archive export.
   *
   * The backend returns 202 with a pending/running job when work was queued,
   * or 200 with an already-complete export when a cached artifact is still
   * valid. Callers should check `status` rather than the HTTP code.
   */
  async requestExport(gameId: number) {
    return this.client.post<GameExport>(`/api/v1/games/${gameId}/exports`);
  }

  /** Fetch the newest export for a game, for status polling. */
  async getLatestExport(gameId: number) {
    return this.client.get<GameExport>(`/api/v1/games/${gameId}/exports/latest`);
  }

  /**
   * Absolute URL for downloading a finished export.
   *
   * This points at the API (not storage) so authorization is re-checked on
   * every download; a stale link stops working if the game leaves public
   * archive mode.
   */
  getDownloadUrl(exportId: number): string {
    return `/api/v1/exports/${exportId}/download`;
  }
}
