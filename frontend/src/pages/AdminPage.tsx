import { useParams, useNavigate } from 'react-router-dom';
import { AdminModeToggle } from '../components/AdminModeToggle';
import { TabNavigation } from '../components/TabNavigation';
import { AdminsTab } from './admin/AdminsTab';
import { BannedUsersTab } from './admin/BannedUsersTab';
import { UserListTab } from './admin/UserListTab';
import { PendingApprovalTab } from './admin/PendingApprovalTab';
import { IPBansTab } from './admin/IPBansTab';
import { FingerprintBansTab } from './admin/FingerprintBansTab';
import { CommunitiesTab } from './admin/CommunitiesTab';

type TabId = 'mode' | 'admins' | 'banned' | 'users' | 'pending' | 'ip-bans' | 'fingerprint-bans' | 'communities';
const VALID_TABS: TabId[] = ['mode', 'admins', 'banned', 'users', 'pending', 'ip-bans', 'fingerprint-bans', 'communities'];

const TABS: { id: TabId; label: string }[] = [
  { id: 'mode', label: 'Admin Mode' },
  { id: 'banned', label: 'Banned Users' },
  { id: 'admins', label: 'Admins' },
  { id: 'users', label: 'All Users' },
  { id: 'pending', label: 'Pending Approval' },
  { id: 'ip-bans', label: 'IP Bans' },
  { id: 'fingerprint-bans', label: 'Device Bans' },
  { id: 'communities', label: 'Communities' },
];

export function AdminPage() {
  const { tab } = useParams<{ tab?: string }>();
  const navigate = useNavigate();

  const activeTab: TabId = (tab && VALID_TABS.includes(tab as TabId)) ? (tab as TabId) : 'mode';
  const setActiveTab = (t: TabId) => navigate(`/admin/${t}`, { replace: false });

  return (
    <div className="max-w-6xl mx-auto px-4 py-8">
      <h1 className="text-3xl font-bold text-content-primary mb-8">Admin Panel</h1>

      {/* Tab Navigation */}
      <div className="mb-6">
        <TabNavigation
          tabs={TABS}
          activeTab={activeTab}
          onTabChange={(t) => setActiveTab(t as TabId)}
          getTabHref={(t) => `/admin/${t}`}
        />
      </div>

      {activeTab === 'mode' && (
        <div className="bg-surface-base rounded-lg shadow">
          <div className="px-6 py-4 border-b border-theme-default">
            <h2 className="text-xl font-semibold text-content-primary">Admin Mode</h2>
            <p className="text-sm text-content-tertiary mt-1">
              Control your administrator privileges and visibility
            </p>
          </div>
          <div className="px-6 py-6">
            <div className="max-w-2xl">
              <div className="mb-4">
                <h3 className="text-lg font-medium text-content-primary mb-2">What is Admin Mode?</h3>
                <p className="text-sm text-content-secondary mb-4">
                  When Admin Mode is enabled, you'll see additional moderation controls throughout the site,
                  such as delete buttons on posts and comments. This allows you to quickly moderate content
                  without switching to the admin panel.
                </p>
                <p className="text-sm text-content-secondary mb-4">
                  When disabled, the site appears as it does to regular users, hiding all administrative controls.
                </p>
              </div>
              <div className="border-t border-theme-default pt-4">
                <AdminModeToggle />
              </div>
            </div>
          </div>
        </div>
      )}

      {activeTab === 'banned' && <BannedUsersTab />}
      {activeTab === 'admins' && <AdminsTab />}
      {activeTab === 'users' && <UserListTab />}
      {activeTab === 'pending' && <PendingApprovalTab />}
      {activeTab === 'ip-bans' && <IPBansTab />}
      {activeTab === 'fingerprint-bans' && <FingerprintBansTab />}
      {activeTab === 'communities' && <CommunitiesTab />}
    </div>
  );
}
