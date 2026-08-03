import { Link } from 'react-router-dom';
import { FileText, MessageCircle, Calendar, BookOpen, CheckCircle, Bell } from 'lucide-react';
import { selectDigestEntries, DIGEST_LABELS, DIGEST_TABS } from '../utils/activityDigest';

interface ActivityDigestRowProps {
  notificationsByType?: Record<string, number>;
  gameId?: number;
  /** Draws a divider above the row when repliable inbox items precede it. */
  hasItemsAbove?: boolean;
}

const DIGEST_ICONS: Record<string, React.ReactNode> = {
  action_result: <FileText className="w-3.5 h-3.5" />,
  common_room_post: <MessageCircle className="w-3.5 h-3.5" />,
  handout_published: <BookOpen className="w-3.5 h-3.5" />,
  character_approved: <CheckCircle className="w-3.5 h-3.5" />,
  phase_created: <Calendar className="w-3.5 h-3.5" />,
};

/**
 * The inbox's second tier: non-actionable "FYI" notifications rendered as
 * compact chips, so they read as ambient status rather than competing with the
 * repliable items above them.
 */
export function ActivityDigestRow({
  notificationsByType,
  gameId,
  hasItemsAbove = false,
}: ActivityDigestRowProps) {
  const baseUrl = gameId ? `/games/${gameId}` : '/notifications';
  const { entries, otherCount } = selectDigestEntries(notificationsByType);

  if (entries.length === 0 && otherCount === 0) {
    return null;
  }

  const chipClass =
    'inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs ' +
    'surface-raised text-content-secondary hover:text-content-primary transition-colors';

  return (
    <div
      className={
        hasItemsAbove ? 'pt-3 border-t border-theme-default flex flex-wrap gap-2' : 'flex flex-wrap gap-2'
      }
      data-testid="activity-digest-row"
    >
      {entries.map(({ type, count }) => {
        const tab = DIGEST_TABS[type];
        return (
          <Link key={type} to={tab ? `${baseUrl}?tab=${tab}` : baseUrl} className={chipClass}>
            <span className="text-content-tertiary flex-shrink-0">{DIGEST_ICONS[type]}</span>
            <span>{DIGEST_LABELS[type](count)}</span>
          </Link>
        );
      })}
      {otherCount > 0 && (
        <Link to={baseUrl} className={chipClass}>
          <span className="text-content-tertiary flex-shrink-0">
            <Bell className="w-3.5 h-3.5" />
          </span>
          <span>
            {otherCount} other notification{otherCount > 1 ? 's' : ''}
          </span>
        </Link>
      )}
    </div>
  );
}
