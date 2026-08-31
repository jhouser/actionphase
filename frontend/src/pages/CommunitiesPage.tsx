import { Link } from 'react-router-dom';
import { Card, CardBody, Badge, Spinner, Alert } from '../components/ui';
import { useActiveCommunities } from '../hooks/useCommunities';

/**
 * Browsable list of active communities.
 *
 * Inactive communities are absent by construction -- the endpoint omits them,
 * because a community that accepts no new games is a dead end to browse into.
 */
export function CommunitiesPage() {
  const { communities, isLoading, isError } = useActiveCommunities();

  return (
    <div className="max-w-5xl mx-auto px-4 py-8">
      <h1 className="text-3xl font-bold text-content-primary mb-2">Communities</h1>
      <p className="text-content-secondary mb-8">
        Communities run their own games, moderators, and guidelines.
      </p>

      {isLoading && (
        <div className="flex justify-center py-12" data-testid="communities-loading">
          <Spinner size="lg" />
        </div>
      )}

      {isError && (
        <Alert variant="danger" title="Could not load communities">
          Something went wrong fetching the community list. Try reloading the page.
        </Alert>
      )}

      {!isLoading && !isError && communities.length === 0 && (
        <Alert variant="info" title="No communities yet">
          No communities have been set up. Site admins create them from the admin panel.
        </Alert>
      )}

      <div className="grid gap-4 sm:grid-cols-2" data-testid="communities-list">
        {communities.map((community) => (
          <Link
            key={community.id}
            to={`/communities/${community.slug}`}
            className="block focus:outline-none focus:ring-2 focus:ring-offset-2 rounded-lg"
            data-testid={`community-card-${community.slug}`}
          >
            <Card variant="bordered" padding="md">
              <CardBody>
                <div className="flex items-start justify-between gap-3">
                  <h2 className="text-lg font-semibold text-content-primary">
                    {community.name}
                  </h2>
                  {community.owner_username && (
                    <Badge variant="neutral">{community.owner_username}</Badge>
                  )}
                </div>
                {community.description && (
                  <p className="mt-2 text-sm text-content-secondary line-clamp-3">
                    {community.description}
                  </p>
                )}
              </CardBody>
            </Card>
          </Link>
        ))}
      </div>
    </div>
  );
}
