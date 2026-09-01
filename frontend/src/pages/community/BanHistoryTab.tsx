import { useState } from 'react';
import { Link } from 'react-router-dom';
import { Card, CardBody, CardHeader, Alert, Spinner, Badge, Button } from '../../components/ui';
import { useCommunityBanEvents } from '../../hooks/useCommunities';
import type { BanEventAction, Community } from '../../types/communities';

interface BanHistoryTabProps {
  community: Community;
  canModerate: boolean;
}

const PAGE_SIZE = 50;

/**
 * How each action reads in the log.
 *
 * 'modified' is deliberately worded as an edit rather than a ban: it means an
 * already-banned user's reason or expiry changed, and calling it a ban would
 * imply they had been unbanned in between.
 */
const ACTION_LABELS: Record<BanEventAction, string> = {
  banned: 'Banned',
  unbanned: 'Unbanned',
  modified: 'Ban edited',
};

const ACTION_VARIANTS: Record<BanEventAction, 'danger' | 'success' | 'warning'> = {
  banned: 'danger',
  unbanned: 'success',
  modified: 'warning',
};

/**
 * A community's ban audit log.
 *
 * This exists because lifting a ban DELETES its row: for an unbanned user, this
 * log is the only surviving evidence the ban ever happened. Three communities
 * share this deployment and will have disputes about who banned whom, which
 * cannot be reconstructed after the fact.
 *
 * Entries are SNAPSHOTS -- an unban records what the ban said at the moment it
 * was lifted, not a live reference to a row that no longer exists.
 */
export function BanHistoryTab({ community, canModerate }: BanHistoryTabProps) {
  const [page, setPage] = useState(0);
  const { events, isLoading, isError } = useCommunityBanEvents(community.slug, {
    enabled: canModerate,
    limit: PAGE_SIZE,
    offset: page * PAGE_SIZE,
  });

  if (!canModerate) {
    return (
      <Alert variant="info" title="Moderators only">
        Only this community's moderators can read the ban history.
      </Alert>
    );
  }

  // The endpoint returns a page, not a total, so a full page is the only
  // signal another may exist. A trailing empty page is the cost of not
  // fetching a count on every read.
  const mayHaveMore = events.length === PAGE_SIZE;

  return (
    <Card variant="default" padding="md">
      <CardHeader>
        <h2 className="text-lg font-semibold text-content-primary">Ban history</h2>
        <p className="text-sm text-content-tertiary mt-1">
          Every ban, edit, and lift in {community.name}, newest first. Lifting a ban
          removes it from the banlist, so this is the lasting record.
        </p>
      </CardHeader>
      <CardBody>
        {isLoading && (
          <div className="py-6 flex justify-center" data-testid="ban-events-loading">
            <Spinner size="md" />
          </div>
        )}

        {isError && (
          <Alert variant="danger" title="Could not load the ban history">
            Try reloading the page.
          </Alert>
        )}

        {!isLoading && !isError && events.length === 0 && (
          <p className="text-sm text-content-tertiary" data-testid="ban-events-empty">
            {page === 0
              ? 'Nothing has happened on this community’s banlist yet.'
              : 'No more history.'}
          </p>
        )}

        {!isLoading && !isError && events.length > 0 && (
          <ul className="divide-y divide-theme-default" data-testid="ban-event-list">
            {events.map((event) => (
              <li key={event.id} className="py-3" data-testid={`ban-event-${event.id}`}>
                <div className="flex items-center gap-2 flex-wrap">
                  <Badge variant={ACTION_VARIANTS[event.action]}>
                    {ACTION_LABELS[event.action]}
                  </Badge>
                  {event.target_username ? (
                    <Link
                      to={`/users/${event.target_username}`}
                      className="text-content-primary hover:underline"
                    >
                      {event.target_username}
                    </Link>
                  ) : (
                    <span className="text-content-secondary">
                      user #{event.target_user_id}
                    </span>
                  )}
                </div>

                {event.reason && (
                  <p className="text-sm text-content-secondary mt-1">{event.reason}</p>
                )}

                <p className="text-xs text-content-tertiary mt-1">
                  {new Date(event.created_at).toLocaleString()}
                  {/* A deleted moderator's events outlive them, so the actor
                      can genuinely be absent rather than merely unloaded. */}
                  {event.actor_username
                    ? ` · by ${event.actor_username}`
                    : ' · by a deleted user'}
                </p>
              </li>
            ))}
          </ul>
        )}

        {(page > 0 || mayHaveMore) && (
          <div className="flex justify-between mt-4">
            <Button
              variant="secondary"
              size="sm"
              disabled={page === 0}
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              data-testid="ban-events-prev"
            >
              Newer
            </Button>
            <Button
              variant="secondary"
              size="sm"
              disabled={!mayHaveMore}
              onClick={() => setPage((p) => p + 1)}
              data-testid="ban-events-next"
            >
              Older
            </Button>
          </div>
        )}
      </CardBody>
    </Card>
  );
}
