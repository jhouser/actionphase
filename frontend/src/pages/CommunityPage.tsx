import { Link, useParams } from 'react-router-dom';
import { Card, CardBody, CardHeader, Button, Spinner, Alert, Badge } from '../components/ui';
import { MarkdownPreview } from '../components/MarkdownPreview';
import { useCommunity } from '../hooks/useCommunities';

/**
 * A community's public profile.
 *
 * The Manage link appears for anyone with standing here -- owner, moderator, or
 * a site admin with admin mode on -- which the server reports as your_role on
 * the community itself. The server is the authority either way; hiding the link
 * is a courtesy, not a control.
 */
export function CommunityPage() {
  const { slug } = useParams<{ slug: string }>();
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
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <Alert variant="danger" title="Community not found">
          No community exists at this address, or you cannot view it.
        </Alert>
      </div>
    );
  }

  // your_role already folds in moderator rows and admin mode, so no comparison
  // against owner_user_id is needed -- that one misses moderators entirely.
  const canManage = community.your_role === 'owner' || community.your_role === 'moderator';

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
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

        <div className="flex items-center gap-2">
          <Link to={`/communities/${community.slug}/games`}>
            <Button variant="secondary" data-testid="community-games-link">
              Games
            </Button>
          </Link>

          {canManage && (
            <Link to={`/communities/${community.slug}/manage/moderators`}>
              <Button variant="secondary" data-testid="manage-community">
                Manage
              </Button>
            </Link>
          )}
        </div>
      </div>

      <Card variant="default" padding="md">
        <CardHeader>
          <h2 className="text-lg font-semibold text-content-primary">About</h2>
        </CardHeader>
        <CardBody>
          {community.description ? (
            // fullWidth: the card now spans max-w-7xl, and the default
            // max-w-prose would cap the text at ~65ch and leave the rest of
            // the card empty.
            <MarkdownPreview content={community.description} fullWidth />
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
