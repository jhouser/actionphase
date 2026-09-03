import { useState } from 'react';
import { Link } from 'react-router-dom';
import { Card, CardBody, CardHeader, Button, Alert, Spinner, Badge } from '../../components/ui';
import { UserSearchSelect, type SelectedUser } from '../../components/UserSearchSelect';
import { useToast } from '../../contexts/ToastContext';
import { useCommunityModerators } from '../../hooks/useCommunities';
import type { Community } from '../../types/communities';

interface ModeratorsTabProps {
  community: Community;
  /**
   * Whether the viewer may change the roster. Moderators see the list but not
   * the controls -- managing the roster is the one power that separates an
   * owner from a moderator.
   */
  canAdminister: boolean;
}

/**
 * The moderator roster.
 *
 * The OWNER IS NOT IN THE LIST the server returns -- ownership is not a
 * moderator row. It is rendered separately below so the page answers "who holds
 * power here" completely, which the raw roster alone does not.
 */
export function ModeratorsTab({ community, canAdminister }: ModeratorsTabProps) {
  const { showSuccess, showError } = useToast();
  const { moderators, isLoading, isError, addModerator, removeModerator } =
    useCommunityModerators(community.slug);

  const [selected, setSelected] = useState<SelectedUser | null>(null);

  // The owner cannot be added (they already hold every moderator power), and
  // neither can an existing moderator. Excluding both keeps the picker from
  // offering choices the server will reject.
  const excludeUserIds = [community.owner_user_id, ...moderators.map((m) => m.user_id)];

  const handleAdd = (e: React.FormEvent) => {
    e.preventDefault();
    if (!selected) return;

    addModerator.mutate(selected.id, {
      onSuccess: () => {
        showSuccess(`${selected.username} can now moderate this community`);
        setSelected(null);
      },
      onError: (err: unknown) => {
        // `error`, not `detail` -- see the note in SettingsTab.
        const detail =
          (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
        showError(detail ?? 'Could not add that moderator');
      },
    });
  };

  const handleRemove = (userId: number, username: string) => {
    removeModerator.mutate(userId, {
      onSuccess: () => showSuccess(`${username} no longer moderates this community`),
      onError: () => showError('Could not remove that moderator'),
    });
  };

  return (
    <div className="space-y-6">
      {canAdminister && (
        <Card variant="default" padding="md">
          <CardHeader>
            <h2 className="text-lg font-semibold text-content-primary">Add a moderator</h2>
            <p className="text-sm text-content-tertiary mt-1">
              Moderators can manage bans, documents, and the community profile. Only you
              can change this roster.
            </p>
          </CardHeader>
          <CardBody>
            <form onSubmit={handleAdd} className="flex flex-col gap-3 sm:flex-row sm:items-end">
              <div className="flex-1">
                <UserSearchSelect
                  label="User"
                  placeholder="Search by username"
                  value={selected}
                  onChange={setSelected}
                  excludeUserIds={excludeUserIds}
                  dropdownId="add-community-moderator"
                  data-testid="moderator-user-search"
                />
              </div>
              <Button
                type="submit"
                variant="primary"
                disabled={!selected}
                loading={addModerator.isPending}
                data-testid="add-moderator-submit"
              >
                Add moderator
              </Button>
            </form>
          </CardBody>
        </Card>
      )}

      <Card variant="default" padding="md">
        <CardHeader>
          <h2 className="text-lg font-semibold text-content-primary">Who can moderate</h2>
        </CardHeader>
        <CardBody>
          <ul className="divide-y divide-theme-default" data-testid="moderator-list">
            {/* Rendered from the community record, not the roster: ownership is
                not a moderator row, so the server's list never includes it. */}
            <li className="flex items-center justify-between py-3">
              <div className="flex items-center gap-2">
                {community.owner_username ? (
                  <Link
                    to={`/users/${community.owner_username}`}
                    className="text-content-primary hover:underline"
                  >
                    {community.owner_username}
                  </Link>
                ) : (
                  <span className="text-content-primary">Community owner</span>
                )}
                <Badge variant="primary">Owner</Badge>
              </div>
            </li>

            {isLoading && (
              <li className="py-6 flex justify-center" data-testid="moderators-loading">
                <Spinner size="md" />
              </li>
            )}

            {isError && (
              <li className="py-3">
                <Alert variant="danger" title="Could not load the roster">
                  Try reloading the page.
                </Alert>
              </li>
            )}

            {!isLoading &&
              !isError &&
              moderators.map((mod) => (
                <li
                  key={mod.id}
                  className="flex items-center justify-between py-3"
                  data-testid={`moderator-row-${mod.user_id}`}
                >
                  <div className="flex items-center gap-2">
                    <Link
                      to={`/users/${mod.username}`}
                      className="text-content-primary hover:underline"
                    >
                      {mod.display_name || mod.username}
                    </Link>
                    <Badge variant="secondary">Moderator</Badge>
                  </div>

                  {canAdminister && (
                    <Button
                      variant="danger"
                      size="sm"
                      onClick={() => handleRemove(mod.user_id, mod.username)}
                      // Scoped to the row being removed: the mutation state is
                      // shared across every row, so an unscoped isPending would
                      // spin all of the Remove buttons at once.
                      loading={
                        removeModerator.isPending &&
                        removeModerator.variables === mod.user_id
                      }
                      data-testid={`remove-moderator-${mod.user_id}`}
                    >
                      Remove
                    </Button>
                  )}
                </li>
              ))}

            {!isLoading && !isError && moderators.length === 0 && (
              <li className="py-3 text-sm text-content-tertiary">
                No moderators yet -- only the owner can moderate this community.
              </li>
            )}
          </ul>
        </CardBody>
      </Card>
    </div>
  );
}
