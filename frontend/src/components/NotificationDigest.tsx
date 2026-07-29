import { Link } from 'react-router-dom';
import {
  MessageSquare,
  Reply,
  AtSign,
  FileText,
  MessageCircle,
  Calendar,
  BookOpen,
  CheckCircle,
  Bell,
} from 'lucide-react';
import { INBOX_HANDLED_TYPES } from '../utils/activityDigest';

interface NotificationDigestProps {
  notificationsByType: Record<string, number>;
  gameId?: number;
}

interface DigestEntry {
  icon: React.ReactNode;
  label: (count: number) => string;
  tab: string;
}

const NOTIFICATION_CONFIG: Record<string, DigestEntry> = {
  private_message: {
    icon: <MessageSquare className="w-4 h-4" />,
    label: (n) => `${n} private message${n > 1 ? 's' : ''}`,
    tab: 'messages',
  },
  comment_reply: {
    icon: <Reply className="w-4 h-4" />,
    label: (n) => `${n} repl${n > 1 ? 'ies' : 'y'} to your comment`,
    tab: 'common-room',
  },
  character_mention: {
    icon: <AtSign className="w-4 h-4" />,
    label: (n) => `${n} mention${n > 1 ? 's' : ''}`,
    tab: 'common-room',
  },
  action_result: {
    icon: <FileText className="w-4 h-4" />,
    label: (n) => `${n} action result${n > 1 ? 's' : ''} published`,
    tab: 'actions',
  },
  common_room_post: {
    icon: <MessageCircle className="w-4 h-4" />,
    label: (n) => `${n} new post${n > 1 ? 's' : ''}`,
    tab: 'common-room',
  },
  phase_created: {
    icon: <Calendar className="w-4 h-4" />,
    label: (n) => `${n} new phase${n > 1 ? 's' : ''} started`,
    tab: '',
  },
  handout_published: {
    icon: <BookOpen className="w-4 h-4" />,
    label: (n) => `${n} new handout${n > 1 ? 's' : ''}`,
    tab: 'handouts',
  },
  character_approved: {
    icon: <CheckCircle className="w-4 h-4" />,
    label: (n) => `${n} character${n > 1 ? 's' : ''} approved`,
    tab: 'people',
  },
};

// Priority order for display
const DISPLAY_ORDER = [
  'private_message',
  'action_result',
  'comment_reply',
  'character_mention',
  'common_room_post',
  'handout_published',
  'character_approved',
  'phase_created',
];

// GM-facing types that are less urgent for players — collapsed into "other"
const GM_TYPES = new Set(['action_submitted', 'application_submitted', 'application_approved', 'game_state_changed']);

/**
 * NotificationDigest - Shows a breakdown of unread notifications by type,
 * each linking directly to the relevant game tab.
 */
export function NotificationDigest({ notificationsByType, gameId }: NotificationDigestProps) {
  const baseUrl = gameId ? `/games/${gameId}` : '/notifications';

  // Separate known player-facing types from GM/other types
  const playerEntries: Array<{ type: string; count: number; config: DigestEntry }> = [];
  let otherCount = 0;

  for (const [type, count] of Object.entries(notificationsByType)) {
    if (count === 0) continue;
    if (INBOX_HANDLED_TYPES.has(type)) continue;
    if (GM_TYPES.has(type)) {
      otherCount += count;
    } else if (NOTIFICATION_CONFIG[type]) {
      playerEntries.push({ type, count, config: NOTIFICATION_CONFIG[type] });
    } else {
      otherCount += count;
    }
  }

  // Sort by display order
  playerEntries.sort((a, b) => {
    const ai = DISPLAY_ORDER.indexOf(a.type);
    const bi = DISPLAY_ORDER.indexOf(b.type);
    return (ai === -1 ? 99 : ai) - (bi === -1 ? 99 : bi);
  });

  if (playerEntries.length === 0 && otherCount === 0) {
    return null;
  }

  return (
    <div className="surface-base rounded-lg shadow-md border border-theme-default p-6">
      <div className="flex items-center gap-2 mb-4">
        <Bell className="w-5 h-5 text-content-tertiary" />
        <h2 className="text-lg font-bold text-content-primary">New Activity</h2>
      </div>
      <div className="space-y-2">
        {playerEntries.map(({ type, count, config }) => {
          const href = config.tab ? `${baseUrl}?tab=${config.tab}` : baseUrl;
          return (
            <Link
              key={type}
              to={href}
              className="flex items-center gap-3 px-3 py-2 rounded-md hover:surface-raised transition-colors text-sm text-content-primary"
            >
              <span className="text-interactive-primary flex-shrink-0">{config.icon}</span>
              <span>{config.label(count)}</span>
            </Link>
          );
        })}
        {otherCount > 0 && (
          <Link
            to={baseUrl}
            className="flex items-center gap-3 px-3 py-2 rounded-md hover:surface-raised transition-colors text-sm text-content-secondary"
          >
            <span className="flex-shrink-0"><Bell className="w-4 h-4" /></span>
            <span>{otherCount} other notification{otherCount > 1 ? 's' : ''}</span>
          </Link>
        )}
      </div>
    </div>
  );
}
