/**
 * Notification types the Unread inbox already renders as full, repliable rows.
 * They're excluded from the digest tier so a single comment reply isn't
 * counted both as an inbox item and again as a digest chip.
 *
 * Must stay in sync with `classifyNotification` in parseUnreadNotification.ts,
 * which decides what becomes an inbox item.
 */
export const INBOX_HANDLED_TYPES = new Set([
  'comment_reply',
  'character_mention',
  'private_message',
]);

export interface DigestEntry {
  type: string;
  count: number;
}

/**
 * Priority order for display. Types absorbed by the inbox tier are absent.
 */
const DISPLAY_ORDER = [
  'action_result',
  'common_room_post',
  'handout_published',
  'character_approved',
  'phase_created',
];

export const DIGEST_LABELS: Record<string, (count: number) => string> = {
  action_result: (n) => `${n} action result${n > 1 ? 's' : ''}`,
  common_room_post: (n) => `${n} new post${n > 1 ? 's' : ''}`,
  handout_published: (n) => `${n} new handout${n > 1 ? 's' : ''}`,
  character_approved: (n) => `${n} character${n > 1 ? 's' : ''} approved`,
  phase_created: (n) => `${n} new phase${n > 1 ? 's' : ''}`,
};

export const DIGEST_TABS: Record<string, string> = {
  action_result: 'actions',
  common_room_post: 'common-room',
  handout_published: 'handouts',
  character_approved: 'people',
  phase_created: '',
};

/**
 * Splits unread notification counts into the FYI entries the digest row shows,
 * dropping anything the inbox already lists above it.
 */
export function selectDigestEntries(
  notificationsByType?: Record<string, number>
): { entries: DigestEntry[]; otherCount: number } {
  const entries: DigestEntry[] = [];
  let otherCount = 0;

  for (const [type, count] of Object.entries(notificationsByType ?? {})) {
    if (count === 0) continue;
    if (INBOX_HANDLED_TYPES.has(type)) continue;
    if (DIGEST_LABELS[type]) {
      entries.push({ type, count });
    } else {
      otherCount += count;
    }
  }

  entries.sort((a, b) => {
    const ai = DISPLAY_ORDER.indexOf(a.type);
    const bi = DISPLAY_ORDER.indexOf(b.type);
    return (ai === -1 ? 99 : ai) - (bi === -1 ? 99 : bi);
  });

  return { entries, otherCount };
}

/**
 * Total number of notifications represented by the digest row — added to the
 * inbox item count for the header badge.
 */
export function countDigestNotifications(notificationsByType?: Record<string, number>): number {
  const { entries, otherCount } = selectDigestEntries(notificationsByType);
  return entries.reduce((sum, entry) => sum + entry.count, 0) + otherCount;
}
