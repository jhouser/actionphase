import { Link } from 'react-router-dom';
import { useGameCommunityDocuments } from '../hooks/useCommunities';

interface GameCommunitySectionProps {
  gameId: number;
  /** The owning community's name; absent for a legacy game (req 5). */
  communityName?: string;
  /** The owning community's slug, for linking. */
  communitySlug?: string;
}

/**
 * The community section of a game's Info tab (req 8).
 *
 * Identity comes from the GAME, documents are an optional list inside it. That
 * split matters: the first version keyed everything off the document list, so a
 * game in a community that had published nothing showed no community at all --
 * not even its name. Naming the community a game belongs to is not conditional
 * on that community having written anything.
 *
 * Documents render as titles that LINK, not embedded markdown. Embedding would
 * duplicate the same rules across every game in the community and make the tab
 * unbounded in length, so the reader is sent to the community page where the
 * documents actually live.
 *
 * Renders NOTHING when the game has no community -- a legacy game predating
 * communities. There is no identity to show and no placeholder worth inventing.
 */
export function GameCommunitySection({
  gameId,
  communityName,
  communitySlug,
}: GameCommunitySectionProps) {
  // Documents are supplementary: the section stands on the community's name
  // alone, so a failed or empty document fetch costs the titles, not the
  // section. No error banner for the same reason -- this is secondary content
  // on a tab whose primary content loaded fine.
  const { documents } = useGameCommunityDocuments(gameId, Boolean(communityName));

  if (!communityName) return null;

  return (
    <div
      className="mt-6 pt-6 border-t border-theme-default"
      data-testid="game-community-section"
    >
      <p className="text-xs font-semibold uppercase tracking-wider text-content-secondary mb-2">
        Community
      </p>

      {communitySlug ? (
        <Link
          to={`/communities/${communitySlug}`}
          className="text-content-primary hover:underline"
          data-testid="game-community-name"
        >
          {communityName}
        </Link>
      ) : (
        <span className="text-content-primary" data-testid="game-community-name">
          {communityName}
        </span>
      )}

      {documents.length > 0 && (
        <ul className="mt-3 space-y-2">
          {documents.map((doc) => (
            <li key={doc.id}>
              {communitySlug ? (
                <Link
                  to={`/communities/${communitySlug}`}
                  className="text-sm text-content-secondary hover:underline"
                  data-testid={`game-community-document-${doc.id}`}
                >
                  {doc.title}
                </Link>
              ) : (
                <span
                  className="text-sm text-content-secondary"
                  data-testid={`game-community-document-${doc.id}`}
                >
                  {doc.title}
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
