import { Link } from 'react-router-dom';
import { Card, CardBody, Badge, Spinner, Alert } from '../components/ui';
import { CollapsibleMarkdown } from '../components/CollapsibleMarkdown';
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
          // The card is NOT wrapped in the link. Descriptions are markdown and
          // may contain their own anchors, and an <a> inside an <a> is invalid
          // HTML -- browsers unnest it, so a link in the blurb hijacked the
          // click and navigated off-site instead of into the community. It also
          // dragged prose's `a strong` rule over every bold word, painting it
          // link-blue when nothing there was clickable.
          //
          // Instead the anchor is a sibling stretched over the card. The card
          // stays fully clickable, while markdown links inside the description
          // remain real links that sit above the overlay and work normally.
          <div key={community.id} className="relative">
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
                  // CollapsibleMarkdown renders the markdown and clips the
                  // RESULT. A CSS line-clamp cannot do this: it clamps only the
                  // first block, so a description opening with a heading would
                  // preview as just that heading.
                  //
                  // showToggle={false} -- the overlay covers the card, so an
                  // expand button under it could not be clicked. The full text
                  // is one tap away on the community page.
                  //
                  // prose gives block elements generous margins and scales
                  // headings to 24px. In a 60px preview a leading "## Heading"
                  // spent 48px on its own top margin and showed as a couple of
                  // clipped pixels, so the rhythm is flattened and headings are
                  // held at body size -- this is a teaser, not a document.
                  // relative + z-10 lifts the description above the overlay
                  // anchor so its own markdown links stay clickable; w-fit keeps
                  // the raised area to the text itself, leaving the rest of the
                  // card covered by the overlay.
                  <div
                    className="relative z-10 w-fit mt-2 text-sm text-content-secondary
                      [&_.markdown-preview>*>*:first-child]:!mt-0
                      [&_p]:!my-1 [&_ul]:!my-1 [&_ol]:!my-1 [&_blockquote]:!my-1
                      [&_h1]:!text-sm [&_h2]:!text-sm [&_h3]:!text-sm
                      [&_h4]:!text-sm [&_h5]:!text-sm [&_h6]:!text-sm
                      [&_h1]:!my-1 [&_h2]:!my-1 [&_h3]:!my-1
                      [&_h4]:!my-1 [&_h5]:!my-1 [&_h6]:!my-1"
                  >
                    <CollapsibleMarkdown
                      content={community.description}
                      fullWidth
                      collapsedMaxHeight={60}
                      showToggle={false}
                      data-testid={`community-description-${community.slug}`}
                    />
                  </div>
                )}
              </CardBody>
            </Card>

            {/* Stretched over the card, beneath any markdown link in the
                description so those stay clickable. The community name is the
                accessible label, since the anchor itself has no text. */}
            <Link
              to={`/communities/${community.slug}`}
              aria-label={community.name}
              className="absolute inset-0 z-0 rounded-lg focus:outline-none focus:ring-2 focus:ring-offset-2"
              data-testid={`community-card-${community.slug}`}
            />
          </div>
        ))}
      </div>
    </div>
  );
}
