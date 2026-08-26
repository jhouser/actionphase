import { useState, useEffect, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { apiClient } from '../lib/api';
import type { PublicGameApplicant } from '../types/games';
import { Spinner, Alert, Badge } from './ui';
import { getInitials, getAvatarColor } from '../utils/avatar';

interface PublicApplicantsListProps {
  gameId: number;
}

/**
 * PublicApplicantsList - Shows who has applied to join a game
 *
 * PUBLIC ENDPOINT - No authentication required
 * Only shows:
 * - Username
 * - Role (player/audience)
 * - Applied date
 *
 * Does NOT show:
 * - Application status (pending/approved/rejected)
 * - Application message
 * - User email
 * - Reviewer information
 *
 * Only visible when game is in "recruitment" state.
 */
export function PublicApplicantsList({ gameId }: PublicApplicantsListProps) {
  const [applicants, setApplicants] = useState<PublicGameApplicant[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchApplicants = useCallback(async () => {
    try {
      setLoading(true);
      const response = await apiClient.games.getPublicGameApplicants(gameId);
      setApplicants(response.data);
      setError(null);
    } catch (err: unknown) {
      // If forbidden, game is probably not in recruitment anymore
      if ((err as { response?: { status?: number } })?.response?.status === 403) {
        setError('Applicant list is only visible during recruitment');
      } else {
        setError(err instanceof Error ? err.message : 'Failed to load applicants');
      }
    } finally {
      setLoading(false);
    }
  }, [gameId]);

  useEffect(() => {
    fetchApplicants();
  }, [fetchApplicants]);

  if (loading) {
    return (
      <div className="flex justify-center py-4">
        <Spinner size="md" label="Loading applicants..." />
      </div>
    );
  }

  if (error) {
    return (
      <Alert variant="info">
        {error}
      </Alert>
    );
  }

  if (applicants.length === 0) {
    return (
      <div className="text-center py-4">
        <p className="text-content-tertiary">No applications yet</p>
      </div>
    );
  }

  // Group by role
  const players = applicants.filter(a => a.role === 'player');
  const audience = applicants.filter(a => a.role === 'audience');

  return (
    <div className="space-y-4">
      <h3 className="font-semibold text-content-primary">Applicants ({applicants.length})</h3>

      {players.length > 0 && (
        <div>
          <h4 className="text-sm font-medium text-content-secondary mb-2">Players ({players.length})</h4>
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-2">
            {players.map((applicant) => (
              <div
                key={applicant.id}
                className="border border-theme-default rounded-lg p-3 bg-surface-raised"
              >
                <div className="flex items-center gap-3 mb-2">
                  {/* Avatar */}
                  {applicant.avatar_url ? (
                    <img
                      src={applicant.avatar_url}
                      alt={`${applicant.username}'s avatar`}
                      className="w-10 h-10 rounded-full object-cover flex-shrink-0"
                    />
                  ) : (
                    <div className={`w-10 h-10 rounded-full flex items-center justify-center text-white font-semibold text-sm flex-shrink-0 ${getAvatarColor(applicant.username)}`}>
                      {getInitials(applicant.username)}
                    </div>
                  )}

                  {/* Username and Badge */}
                  <div className="flex items-center justify-between gap-2 flex-1 min-w-0">
                    <Link to={`/users/${applicant.username}`} className="font-medium text-content-primary hover:underline truncate">
                      {applicant.username}
                    </Link>
                    <Badge variant="primary" size="sm">Player</Badge>
                  </div>
                </div>
                <div className="text-xs text-content-tertiary">
                  Applied {new Date(applicant.applied_at).toLocaleDateString()}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {audience.length > 0 && (
        <div>
          <h4 className="text-sm font-medium text-content-secondary mb-2">Audience ({audience.length})</h4>
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-2">
            {audience.map((applicant) => (
              <div
                key={applicant.id}
                className="border border-theme-default rounded-lg p-3 bg-surface-raised"
              >
                <div className="flex items-center gap-3 mb-2">
                  {/* Avatar */}
                  {applicant.avatar_url ? (
                    <img
                      src={applicant.avatar_url}
                      alt={`${applicant.username}'s avatar`}
                      className="w-10 h-10 rounded-full object-cover flex-shrink-0"
                    />
                  ) : (
                    <div className={`w-10 h-10 rounded-full flex items-center justify-center text-white font-semibold text-sm flex-shrink-0 ${getAvatarColor(applicant.username)}`}>
                      {getInitials(applicant.username)}
                    </div>
                  )}

                  {/* Username and Badge */}
                  <div className="flex items-center justify-between gap-2 flex-1 min-w-0">
                    <Link to={`/users/${applicant.username}`} className="font-medium text-content-primary hover:underline truncate">
                      {applicant.username}
                    </Link>
                    <Badge variant="secondary" size="sm">Audience</Badge>
                  </div>
                </div>
                <div className="text-xs text-content-tertiary">
                  Applied {new Date(applicant.applied_at).toLocaleDateString()}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
