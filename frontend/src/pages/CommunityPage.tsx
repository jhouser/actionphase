import { Link, useParams } from 'react-router-dom';
import { Card, CardBody, CardHeader, Button, Spinner, Alert, Badge } from '../components/ui';
import { MarkdownPreview } from '../components/MarkdownPreview';
import { useAuth } from '../contexts/AuthContext';
import { useAdminMode } from '../contexts/AdminModeContext';
import { useCommunity } from '../hooks/useCommunities';

/**
 * A community's public profile.
 *
 * The Manage link appears only for the owner and for a site admin with admin
 * mode on. Moderators reach management from elsewhere -- this page cannot tell
 * a moderator from an ordinary visitor without calling a moderator-only
 * endpoint, and firing one for every visitor would guarantee a 403 per view.
 * The server is the authority either way; hiding the link is a courtesy, not a
 * control.
 */
export function CommunityPage() {
  const { slug } = useParams<{ slug: string }>();
  const { currentUser } = useAuth();
  const { adminModeEnabled } = useAdminMode();
  const { community, isLoading, isError } = useCommunity(slug);

  if (isLoading) {
    return (
      <div className="flex justify-center py-16" data-testid="community-loading">
        <Spinner size="lg" />
      </div>
    );
  }

  if (isError || !community) {
    return (
      <div className="max-w-3xl mx-auto px-4 py-8">
        <Alert variant="danger" title="Community not found">
          No community exists at this address, or you cannot view it.
        </Alert>
      </div>
    );
  }

  const isOwner = currentUser?.id === community.owner_user_id;
  const canManage = isOwner || Boolean(currentUser?.is_admin && adminModeEnabled);

  return (
    <div className="max-w-3xl mx-auto px-4 py-8">
      <div className="flex items-start justify-between gap-4 mb-6">
        <div>
          <h1 className="text-3xl font-bold text-content-primary">{community.name}</h1>
          <div className="mt-2 flex items-center gap-2">
            <span className="text-sm text-content-tertiary">Owned by</span>
            {community.owner_username ? (
              <Link
                to={`/users/${community.owner_username}`}
                className="text-sm text-content-secondary hover:underline"
              >
                {community.owner_username}
              </Link>
            ) : (
              <span className="text-sm text-content-secondary">a community owner</span>
            )}
            {!community.is_active && <Badge variant="warning">Inactive</Badge>}
          </div>
        </div>

        {canManage && (
          <Link to={`/communities/${community.slug}/manage/moderators`}>
            <Button variant="secondary" data-testid="manage-community">
              Manage
            </Button>
          </Link>
        )}
      </div>

      <Card variant="default" padding="md">
        <CardHeader>
          <h2 className="text-lg font-semibold text-content-primary">About</h2>
        </CardHeader>
        <CardBody>
          {community.description ? (
            <MarkdownPreview content={community.description} />
          ) : (
            <p className="text-sm text-content-tertiary">
              This community has not written a description yet.
            </p>
          )}
        </CardBody>
      </Card>
    </div>
  );
}
