import { useState, useEffect } from 'react';
import { ChevronDown, ChevronUp, Inbox } from 'lucide-react';
import { Badge, Button, Spinner } from './ui';
import { UnreadInboxItemCard } from './UnreadInboxItemCard';
import { ActivityDigestRow } from './ActivityDigestRow';
import { useUnreadInbox } from '../hooks/useUnreadInbox';
import { countDigestNotifications } from '../utils/activityDigest';

interface UnreadInboxSectionProps {
  /**
   * Unread notification counts keyed by type. Types the inbox itself lists as
   * repliable rows are filtered out, leaving only the FYI notifications
   * (new phase, handout published, ...) for the compact digest row.
   */
  notificationsByType?: Record<string, number>;
  /** Single-game dashboards deep-link the digest into that game's tabs. */
  gameId?: number;
}

/**
 * The dashboard's single inbox, in two tiers:
 *  1. Actionable unread items you can reply to inline.
 *  2. A compact digest row of non-actionable "FYI" notifications.
 *
 * Both tiers previously lived in separate cards ("Unread" and "New Activity"),
 * which restated the same notification twice.
 */
export function UnreadInboxSection({ notificationsByType, gameId }: UnreadInboxSectionProps) {
  const { data: items, isLoading } = useUnreadInbox();
  const [isCollapsed, setIsCollapsed] = useState(false);
  const [hasAutoOpened, setHasAutoOpened] = useState(false);

  const actionableCount = items?.length ?? 0;
  const digestCount = countDigestNotifications(notificationsByType);
  const count = actionableCount + digestCount;

  // Default to open the first time there's something to show, but don't
  // fight the user if they've since collapsed it.
  useEffect(() => {
    if (!hasAutoOpened && count > 0) {
      setIsCollapsed(false);
      setHasAutoOpened(true);
    }
  }, [count, hasAutoOpened]);

  if (!isLoading && count === 0) {
    return null;
  }

  return (
    <div className="surface-base rounded-lg shadow-md border border-theme-default">
      <Button
        variant="ghost"
        onClick={() => setIsCollapsed((prev) => !prev)}
        className="w-full justify-between p-6 font-normal"
        aria-expanded={!isCollapsed}
      >
        <div className="flex items-center gap-2">
          <Inbox className="w-5 h-5 text-content-tertiary" />
          <h2 className="text-lg font-bold text-content-primary">Inbox</h2>
          {count > 0 && (
            <Badge variant="primary" size="sm">
              {count}
            </Badge>
          )}
        </div>
        {isCollapsed ? (
          <ChevronDown className="w-5 h-5 text-content-tertiary" />
        ) : (
          <ChevronUp className="w-5 h-5 text-content-tertiary" />
        )}
      </Button>

      {!isCollapsed && (
        <div className="px-6 pb-6 space-y-3">
          {isLoading && (
            <div className="flex items-center gap-2 text-content-secondary text-sm">
              <Spinner size="sm" /> Loading...
            </div>
          )}
          {items?.map((item) => (
            <UnreadInboxItemCard key={item.notification.id} item={item} />
          ))}
          <ActivityDigestRow
            notificationsByType={notificationsByType}
            gameId={gameId}
            hasItemsAbove={actionableCount > 0}
          />
        </div>
      )}
    </div>
  );
}
