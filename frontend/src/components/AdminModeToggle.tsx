import { useAdminMode } from '../hooks/useAdminMode';
import { Toggle } from './ui';

/**
 * AdminModeToggle
 *
 * Toggle switch for admin users to enable/disable admin mode.
 * Only visible to users with is_admin = true.
 *
 * When admin mode is enabled, admins can:
 * - View all games on the platform
 * - Delete comments and posts for moderation
 * - See admin-specific UI elements
 */
export function AdminModeToggle() {
  const { isAdmin, adminModeEnabled, toggleAdminMode } = useAdminMode();

  // Only show toggle to admin users
  if (!isAdmin) {
    return null;
  }

  return (
    <div className="flex items-center space-x-2 px-3 py-2">
      <label
        htmlFor="admin-mode-toggle"
        className="text-sm font-medium text-content-primary cursor-pointer select-none"
      >
        Admin Mode
      </label>
      <Toggle
        id="admin-mode-toggle"
        checked={adminModeEnabled}
        onChange={toggleAdminMode}
        aria-label={adminModeEnabled ? 'Disable admin mode' : 'Enable admin mode'}
      />

      {/* Active indicator badge */}
      {adminModeEnabled && (
        <span className="px-2 py-0.5 text-xs font-semibold bg-semantic-warning text-white rounded">
          ACTIVE
        </span>
      )}
    </div>
  );
}
