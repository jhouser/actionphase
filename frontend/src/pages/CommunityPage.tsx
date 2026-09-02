import { Link, useParams } from 'react-router-dom';
import { Card, CardBody, CardHeader, Button, Spinner, Alert, Badge } from '../components/ui';
import { MarkdownPreview } from '../components/MarkdownPreview';
import { CollapsibleMarkdown } from '../components/CollapsibleMarkdown';
import { useCommunity, useCommunityDocuments } from '../hooks/useCommunities';

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
  // Published documents only. Open to any authenticated user -- a community's
  // rules are what someone reads BEFORE deciding whether to join, so gating
  // them on membership would hide them from the person they inform.
  const { documents } = useCommunityDocuments(slug);

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

      {/* Rendered only when there is something to show. A community with no
          published documents gets no empty section -- an "it has no rules"
          heading tells a visitor nothing they need. */}
      {documents.length > 0 && (
        <Card variant="default" padding="md" className="mt-6">
          <CardHeader>
            <h2 className="text-lg font-semibold text-content-primary">Documents</h2>
          </CardHeader>
          <CardBody>
            {/* Collapsed by default. Rendering every document in full made a
                community with several of them an unreadable wall -- the titles
                are the navigation, and the body is opt-in.

                CollapsibleMarkdown measures actual overflow rather than
                guessing from source length, so a short document shows no
                toggle at all instead of a pointless "Show full content" on
                two lines of text. */}
            <ul className="divide-y divide-theme-default" data-testid="community-documents">
              {documents.map((doc) => (
                <li
                  key={doc.id}
                  className="py-4 first:pt-0 last:pb-0"
                  data-testid={`community-document-${doc.id}`}
                >
                  <h3 className="text-content-primary font-medium mb-2">{doc.title}</h3>
                  <CollapsibleMarkdown
                    content={doc.content}
                    fullWidth
                    expandLabel="Read more"
                    collapseLabel="Show less"
                    data-testid={`community-document-body-${doc.id}`}
                  />
                </li>
              ))}
            </ul>
          </CardBody>
        </Card>
      )}
    </div>
  );
}
