import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  Card,
  CardBody,
  CardHeader,
  Button,
  Alert,
  Spinner,
  Badge,
  Input,
  DateTimeInput,
} from '../../components/ui';
import { UserSearchSelect, type SelectedUser } from '../../components/UserSearchSelect';
import { useToast } from '../../contexts/ToastContext';
import { useCommunityBans, useCommunityModerators } from '../../hooks/useCommunities';
import type { Community, CommunityBan } from '../../types/communities';
import { extractApiErrorMessage } from '@/lib/errors';

interface BansTabProps {
  community: Community;
  /**
   * Whether the viewer may change the banlist. Banning is the moderator tier,
   * not the owner tier -- it is the routine work this feature exists for.
   */
  canModerate: boolean;
}

/** Renders a ban's duration as a moderator reads it, not as a raw timestamp. */
function banTerm(ban: CommunityBan): string {
  if (!ban.expires_at) return 'Permanent';
  const expiry = new Date(ban.expires_at).toLocaleString();
  // An expired ban is retained deliberately, so it must read as finished
  // rather than as a future date that has quietly slipped into the past.
  return ban.is_active ? `Until ${expiry}` : `Expired ${expiry}`;
}

/**
 * A community's banlist.
 *
 * Bans are the entire access-control mechanism here: membership is open, so
 * there is no roster to remove someone from -- only this negative space.
 *
 * Two things this screen must not do, both of which have a matching test:
 * it must not drop expired bans (they are retained so a moderator sees a ban
 * lapse rather than vanish), and it must not infer "banned" from a row being
 * present -- `is_active` is the only answer to that.
 */
export function BansTab({ community, canModerate }: BansTabProps) {
  const { showSuccess, showError } = useToast();
  const { bans, isLoading, isError, banUser, unbanUser } = useCommunityBans(
    community.slug,
    canModerate
  );

  const [selected, setSelected] = useState<SelectedUser | null>(null);
  const [reason, setReason] = useState('');
  const [expiresAt, setExpiresAt] = useState('');

  // Community staff cannot be banned -- a moderator who is also banned is a
  // state no enforcement path can read. The service refuses BOTH tiers, so the
  // picker must withhold both or it offers a name the server answers with a
  // 400, leaving the moderator to guess who is eligible.
  //
  // The owner is not a moderator row (owner is communities.owner_user_id), so
  // the two are combined rather than one covering the other. The roster is a
  // second request: until it lands the owner is still withheld, because that
  // id came with the community record.
  //
  // Already-banned users are NOT excluded: re-banning is how a moderator edits
  // a reason or extends an expiry, and it preserves the original banned_at.
  const { moderators } = useCommunityModerators(community.slug, canModerate);
  const excludeUserIds = useMemo(
    () => [community.owner_user_id, ...moderators.map((m) => m.user_id)],
    [community.owner_user_id, moderators]
  );

  const resetForm = () => {
    setSelected(null);
    setReason('');
    setExpiresAt('');
  };

  const handleBan = (e: React.FormEvent) => {
    e.preventDefault();
    if (!selected) return;

    banUser.mutate(
      {
        user_id: selected.id,
        // Omit rather than send empty strings: the server reads an absent
        // expires_at as "permanent", which is the intended default.
        reason: reason.trim() || undefined,
        expires_at: expiresAt ? new Date(expiresAt).toISOString() : undefined,
      },
      {
        onSuccess: () => {
          showSuccess(`${selected.username} is banned from ${community.name}`);
          resetForm();
        },
        onError: (err: unknown) => {
          // The server's message is specific -- staff, unknown user, past
          // expiry -- and far more useful than a generic failure line.
          const detail = extractApiErrorMessage(err);
          showError(detail ?? 'Could not ban that user');
        },
      }
    );
  };

  const handleUnban = (ban: CommunityBan) => {
    unbanUser.mutate(ban.user_id, {
      onSuccess: () => showSuccess(`${ban.username} is no longer banned`),
      onError: () => showError('Could not lift that ban'),
    });
  };

  if (!canModerate) {
    return (
      <Alert variant="info" title="Moderators only">
        Only this community's moderators can see who is banned.
      </Alert>
    );
  }

  return (
    <div className="space-y-6">
      <Card variant="default" padding="md">
        <CardHeader>
          <h2 className="text-lg font-semibold text-content-primary">Ban a user</h2>
          <p className="text-sm text-content-tertiary mt-1">
            A ban stops someone joining or creating games in {community.name}. It does
            not remove them from games already underway &mdash; that stays the GM's
            call.
          </p>
        </CardHeader>
        <CardBody>
          <form onSubmit={handleBan} className="space-y-3" data-testid="ban-user-form">
            <UserSearchSelect
              label="User"
              placeholder="Search by username"
              value={selected}
              onChange={setSelected}
              excludeUserIds={excludeUserIds}
              dropdownId="ban-community-user"
              data-testid="ban-user-search"
            />

            <Input
              label="Reason"
              optional
              placeholder="Why this user is being banned"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              helperText="Shown to moderators and kept in the audit log."
              data-testid="ban-reason"
            />

            <DateTimeInput
              label="Expires"
              optional
              value={expiresAt}
              onChange={(e) => setExpiresAt(e.target.value)}
              helperText="Leave empty for a permanent ban. Must be in the future."
              data-testid="ban-expires-at"
            />

            <Button
              type="submit"
              variant="danger"
              disabled={!selected}
              loading={banUser.isPending}
              data-testid="ban-user-submit"
            >
              Ban user
            </Button>
          </form>
        </CardBody>
      </Card>

      <Card variant="default" padding="md">
        <CardHeader>
          <h2 className="text-lg font-semibold text-content-primary">Banned users</h2>
        </CardHeader>
        <CardBody>
          {isLoading && (
            <div className="py-6 flex justify-center" data-testid="bans-loading">
              <Spinner size="md" />
            </div>
          )}

          {isError && (
            <Alert variant="danger" title="Could not load the banlist">
              Try reloading the page.
            </Alert>
          )}

          {!isLoading && !isError && bans.length === 0 && (
            <p className="text-sm text-content-tertiary" data-testid="bans-empty">
              Nobody is banned from this community.
            </p>
          )}

          {!isLoading && !isError && bans.length > 0 && (
            <ul className="divide-y divide-theme-default" data-testid="ban-list">
              {bans.map((ban) => (
                <li
                  key={ban.id}
                  className="flex items-start justify-between gap-3 py-3"
                  data-testid={`ban-row-${ban.user_id}`}
                >
                  <div className="min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <Link
                        to={`/users/${ban.username}`}
                        className="text-content-primary hover:underline"
                      >
                        {ban.display_name || ban.username}
                      </Link>
                      {/* Driven by is_active, never by the row existing: an
                          expired ban is still a row and enforces nothing. */}
                      {ban.is_active ? (
                        <Badge variant="danger" data-testid={`ban-status-${ban.user_id}`}>
                          Banned
                        </Badge>
                      ) : (
                        <Badge variant="neutral" data-testid={`ban-status-${ban.user_id}`}>
                          Expired
                        </Badge>
                      )}
                    </div>

                    <p className="text-sm text-content-secondary mt-1">
                      {ban.reason || <span className="italic">No reason given</span>}
                    </p>

                    <p className="text-xs text-content-tertiary mt-1">
                      {banTerm(ban)}
                      {ban.banned_by_username && ` · by ${ban.banned_by_username}`}
                    </p>
                  </div>

                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => handleUnban(ban)}
                    // Scoped to this row: the mutation state is shared, so an
                    // unscoped isPending would spin every button at once.
                    loading={unbanUser.isPending && unbanUser.variables === ban.user_id}
                    data-testid={`unban-${ban.user_id}`}
                  >
                    {/* An expired ban still has a row to clear, so the action
                        is offered either way -- but it is not a "lift" when
                        there is nothing being enforced. */}
                    {ban.is_active ? 'Lift ban' : 'Remove'}
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </CardBody>
      </Card>
    </div>
  );
}
