import { Link, useNavigate, useParams } from 'react-router-dom';
import { TabNavigation } from '../components/TabNavigation';
import { Alert, Spinner } from '../components/ui';
import { useAuth } from '../contexts/AuthContext';
import { useAdminMode } from '../contexts/AdminModeContext';
import { useCommunity } from '../hooks/useCommunities';
import { ModeratorsTab } from './community/ModeratorsTab';

/**
 * Management shell for one community.
 *
 * Only the Moderators tab exists so far. Bans, documents, webhooks, and
 * settings arrive in later phases; the tab list grows here rather than the
 * routing changing shape.
 */
type TabId = 'moderators';
const VALID_TABS: TabId[] = ['moderators'];
const TABS: { id: TabId; label: string }[] = [{ id: 'moderators', label: 'Moderators' }];

export function CommunityManagePage() {
  const { slug, tab } = useParams<{ slug: string; tab?: string }>();
  const navigate = useNavigate();
  const { currentUser } = useAuth();
  const { adminModeEnabled } = useAdminMode();
  const { community, isLoading, isError } = useCommunity(slug);

  const activeTab: TabId =
    tab && VALID_TABS.includes(tab as TabId) ? (tab as TabId) : 'moderators';

  if (isLoading) {
    return (
      <div className="flex justify-center py-16" data-testid="community-manage-loading">
        <Spinner size="lg" />
      </div>
    );
  }

  if (isError || !community) {
    return (
      <div className="max-w-4xl mx-auto px-4 py-8">
        <Alert variant="danger" title="Community not found">
          No community exists at this address, or you cannot view it.
        </Alert>
      </div>
    );
  }

  // Roster management is owner-only (or a site admin with admin mode on). A
  // moderator still reaches this page for the tabs they can act on, so the
  // permission is passed down rather than gating the whole page. Every write is
  // re-checked server-side; this only decides what to render.
  const canAdminister =
    currentUser?.id === community.owner_user_id ||
    Boolean(currentUser?.is_admin && adminModeEnabled);

  return (
    <div className="max-w-4xl mx-auto px-4 py-8">
      <div className="mb-6">
        <Link
          to={`/communities/${community.slug}`}
          className="text-sm text-content-tertiary hover:underline"
        >
          &larr; {community.name}
        </Link>
        <h1 className="text-3xl font-bold text-content-primary mt-1">
          Manage {community.name}
        </h1>
      </div>

      <div className="mb-6">
        <TabNavigation
          tabs={TABS}
          activeTab={activeTab}
          onTabChange={(t) => navigate(`/communities/${community.slug}/manage/${t}`)}
          getTabHref={(t) => `/communities/${community.slug}/manage/${t}`}
        />
      </div>

      {activeTab === 'moderators' && (
        <ModeratorsTab community={community} canAdminister={canAdminister} />
      )}
    </div>
  );
}
