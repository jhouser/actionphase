import { Link, useNavigate, useParams } from 'react-router-dom';
import { TabNavigation } from '../components/TabNavigation';
import { Alert, Spinner } from '../components/ui';
import { useCommunity } from '../hooks/useCommunities';
import { BanHistoryTab } from './community/BanHistoryTab';
import { BansTab } from './community/BansTab';
import { DocumentsTab } from './community/DocumentsTab';
import { ModeratorsTab } from './community/ModeratorsTab';
import { SettingsTab } from './community/SettingsTab';

/**
 * Management shell for one community.
 *
 * Webhooks arrive in a later phase; the tab list grows here rather than the
 * routing changing shape.
 */
type TabId = 'moderators' | 'bans' | 'history' | 'documents' | 'settings';
const VALID_TABS: TabId[] = ['moderators', 'bans', 'history', 'documents', 'settings'];
const TABS: { id: TabId; label: string }[] = [
  { id: 'moderators', label: 'Moderators' },
  { id: 'bans', label: 'Bans' },
  { id: 'history', label: 'Ban history' },
  { id: 'documents', label: 'Documents' },
  { id: 'settings', label: 'Settings' },
];

export function CommunityManagePage() {
  const { slug, tab } = useParams<{ slug: string; tab?: string }>();
  const navigate = useNavigate();
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

  // Both tiers come from the server's your_role, which already accounts for
  // moderator rows and for admin mode. Every write is re-checked server-side;
  // this only decides what to render.
  //
  // Roster management is owner-only; profile editing is the wider moderator
  // tier. A moderator still reaches this page for the tabs they can act on, so
  // the permissions are passed down rather than gating the whole page.
  const canAdminister = community.your_role === 'owner';
  const canModerate = canAdminister || community.your_role === 'moderator';

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

      {activeTab === 'bans' && (
        <BansTab community={community} canModerate={canModerate} />
      )}

      {activeTab === 'history' && (
        <BanHistoryTab community={community} canModerate={canModerate} />
      )}

      {activeTab === 'documents' && (
        <DocumentsTab community={community} canModerate={canModerate} />
      )}

      {activeTab === 'settings' && (
        <SettingsTab community={community} canEdit={canModerate} />
      )}
    </div>
  );
}
