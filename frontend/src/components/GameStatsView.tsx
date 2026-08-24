import { useCallback, useMemo, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useGameStats } from '../hooks/useGameStats';
import type { CharacterWriting } from '../types/gameStats';
import { Alert, Badge, Button, Card, Spinner } from './ui';

export interface GameStatsViewProps {
  gameId: number;
  gameState: string;
}

/** Sortable columns in the per-character table. */
type SortKey =
  | 'character_name'
  | 'comment_count'
  | 'avg_comment_length'
  | 'private_message_count'
  | 'avg_private_message_length';

const numberFormat = new Intl.NumberFormat();

/**
 * Formats a text length as "1,234 characters".
 *
 * "Characters" here means letters, not cast members — the two senses collide on
 * this page, which also has a per-character table. The surrounding labels
 * ("Avg comment", "longest") carry the distinction, so the word is spelled out
 * rather than abbreviated.
 */
function formatChars(value: number): string {
  return `${numberFormat.format(value)} character${value === 1 ? '' : 's'}`;
}

interface StatTileProps {
  label: string;
  value: string;
  hint?: string;
}

/**
 * A single stat.
 *
 * surface-raised + border-theme-default is the site-wide pattern for an item
 * nested inside a section (see HistoryView, PeopleView). surface-sunken is the
 * page background, so using it here inverted the hierarchy and made the tiles
 * read as holes punched in the card rather than as raised elements.
 */
function StatTile({ label, value, hint }: StatTileProps) {
  return (
    <div className="surface-raised border border-theme-default rounded-lg p-4">
      <div className="text-xs font-medium uppercase tracking-wide text-content-tertiary">
        {label}
      </div>
      <div className="mt-1 text-2xl font-semibold text-content-primary">{value}</div>
      {hint && <div className="mt-1 text-xs text-content-tertiary">{hint}</div>}
    </div>
  );
}

/**
 * Post-game statistics for a completed game.
 *
 * "Comments" throughout means what players call posts — entries in a common
 * room discussion. The GM-authored topic post each discussion hangs off is
 * excluded by the backend and never counted here.
 *
 * Card variant is `default` (border, no shadow) rather than `elevated`. On a
 * dark background shadow-lg's rgba(0,0,0,0.1) has nothing to fall on and reads
 * as a muddy halo around the card instead of elevation; a border defines the
 * edge cleanly in both themes.
 */
export function GameStatsView({ gameId, gameState }: GameStatsViewProps) {
  const { data, isLoading, isError, error } = useGameStats(gameId, gameState);
  const [searchParams] = useSearchParams();
  const [sortKey, setSortKey] = useState<SortKey>('comment_count');
  const [sortAsc, setSortAsc] = useState(false);

  // Deep links preserve the rest of the query string and swap the tab, matching
  // how AudienceConversationCard and the tab bar build their hrefs. Tab-specific
  // params from other tabs are cleared so the target tab opens clean.
  const conversationHref = useCallback(
    (conversationId: number) => {
      const params = new URLSearchParams(searchParams);
      params.set('tab', 'audience');
      params.set('audienceConversation', String(conversationId));
      params.delete('comment');
      params.delete('phase');
      return `?${params.toString()}`;
    },
    [searchParams]
  );

  // The phase is included up front when known. HistoryView otherwise resolves
  // ?comment into its phase and pushes a second history entry, so a single
  // click cost two Back presses and the intermediate entry immediately
  // re-resolved — effectively trapping the user. Supplying `phase` short
  // circuits that resolver (it bails once a phase is selected), keeping the
  // navigation to exactly one entry.
  const commentHref = useCallback(
    (messageId: number, phaseId?: number) => {
      const params = new URLSearchParams(searchParams);
      params.set('tab', 'history');
      params.set('comment', String(messageId));
      if (phaseId) {
        params.set('phase', String(phaseId));
      }
      params.delete('audienceConversation');
      params.delete('audienceParticipants');
      return `?${params.toString()}`;
    },
    [searchParams]
  );

  const sortedCharacters = useMemo(() => {
    if (!data?.characters) return [];
    const rows = [...data.characters];
    rows.sort((a, b) => {
      if (sortKey === 'character_name') {
        return sortAsc
          ? a.character_name.localeCompare(b.character_name)
          : b.character_name.localeCompare(a.character_name);
      }
      const diff = (a[sortKey] as number) - (b[sortKey] as number);
      return sortAsc ? diff : -diff;
    });
    return rows;
  }, [data?.characters, sortKey, sortAsc]);

  const handleSort = (key: SortKey) => {
    if (key === sortKey) {
      setSortAsc((prev) => !prev);
      return;
    }
    setSortKey(key);
    // Names read best A–Z; every numeric column is most interesting highest-first.
    setSortAsc(key === 'character_name');
  };

  if (gameState !== 'completed') {
    return (
      <Card variant="default" padding="lg">
        <h2 className="mb-4 text-2xl font-bold text-content-primary">Statistics</h2>
        <Alert variant="info">
          Statistics become available once the game is completed.
        </Alert>
      </Card>
    );
  }

  if (isLoading) {
    return (
      <Card variant="default" padding="lg">
        <h2 className="mb-6 text-2xl font-bold text-content-primary">Statistics</h2>
        <div className="flex justify-center py-8">
          <Spinner size="lg" label="Loading statistics..." />
        </div>
      </Card>
    );
  }

  if (isError || !data) {
    return (
      <Card variant="default" padding="lg">
        <h2 className="mb-6 text-2xl font-bold text-content-primary">Statistics</h2>
        <Alert variant="danger">
          Failed to load statistics
          {error instanceof Error ? `: ${error.message}` : ''}
        </Alert>
      </Card>
    );
  }

  const {
    comments,
    private_messages: pms,
    longest_conversation: longest,
    longest_comment: longestComment,
  } = data;

  return (
    <div className="space-y-6">
      <Card variant="default" padding="lg">
        <h2 className="mb-1 text-2xl font-bold text-content-primary">Statistics</h2>
        <p className="mb-6 text-sm text-content-secondary">
          Totals across the whole game. Comments are common room posts; the GM&apos;s
          phase topics are not counted.
        </p>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatTile
            label="Comments"
            value={numberFormat.format(comments.total_comments)}
          />
          <StatTile
            label="Avg comment"
            value={formatChars(comments.avg_comment_length)}
          />
          <StatTile
            label="Private messages"
            value={numberFormat.format(pms.total_private_messages)}
            hint={`across ${numberFormat.format(pms.active_conversations)} conversation${
              pms.active_conversations === 1 ? '' : 's'
            }`}
          />
          <StatTile
            label="Avg private message"
            value={formatChars(pms.avg_private_message_length)}
          />
          <StatTile
            label="Deepest thread"
            value={`${numberFormat.format(comments.max_thread_depth)} deep`}
            hint={`avg reply depth ${comments.avg_thread_depth.toFixed(1)}`}
          />
        </div>
        {(longestComment || longest) && (
          <div className="mt-6 grid grid-cols-1 gap-4 lg:grid-cols-2">
            {longestComment && (
              <div className="surface-raised border border-theme-default rounded-lg p-4">
                <h3 className="mb-2 text-sm font-semibold text-content-primary">
                  Longest comment
                </h3>
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant="primary">
                    {formatChars(longestComment.content_length)}
                  </Badge>
                  {longestComment.character_name && (
                    <span className="text-sm text-content-secondary">
                      by {longestComment.character_name}
                    </span>
                  )}
                </div>
                <Link
                  to={commentHref(longestComment.message_id, longestComment.phase_id)}
                  className="mt-2 inline-block text-sm text-interactive-primary hover:text-interactive-primary-hover font-medium"
                >
                  Read it in History →
                </Link>
              </div>
            )}

            {longest && (
              <div className="surface-raised border border-theme-default rounded-lg p-4">
                <h3 className="mb-2 text-sm font-semibold text-content-primary">
                  Longest conversation
                </h3>
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant="primary">
                    {numberFormat.format(longest.message_count)} message
                    {longest.message_count === 1 ? '' : 's'}
                  </Badge>
                  {longest.title && (
                    <span className="text-sm text-content-secondary">
                      {longest.title}
                    </span>
                  )}
                </div>
                {longest.participant_names.length > 0 && (
                  <p className="mt-1 text-sm text-content-secondary">
                    {longest.participant_names.join(' · ')}
                  </p>
                )}
                <Link
                  to={conversationHref(longest.conversation_id)}
                  className="mt-2 inline-block text-sm text-interactive-primary hover:text-interactive-primary-hover font-medium"
                >
                  Read it in Audience →
                </Link>
              </div>
            )}
          </div>
        )}

        {/* A rule rather than a second Card: stacking same-coloured cards on a
            same-coloured container is what made the sections indistinguishable
            in dark mode, where the shadow has no surface to fall on. */}
        <div className="mt-8 border-t border-theme-default pt-6">
          <h3 className="mb-4 text-lg font-semibold text-content-primary">
            By character
          </h3>

          {sortedCharacters.length === 0 ? (
            <p className="text-sm text-content-secondary">
              No characters in this game.
            </p>
          ) : (
            // Horizontal scroll container so the table never forces the page to
            // scroll sideways on narrow screens.
            <div className="overflow-x-auto">
              <table className="w-full min-w-[36rem] text-sm">
                <thead>
                  <tr className="border-b border-theme-default text-left">
                    <SortableHeader
                      label="Character"
                      sortKey="character_name"
                      activeKey={sortKey}
                      ascending={sortAsc}
                      onSort={handleSort}
                    />
                    <SortableHeader
                      label="Comments"
                      sortKey="comment_count"
                      activeKey={sortKey}
                      ascending={sortAsc}
                      onSort={handleSort}
                      numeric
                    />
                    <SortableHeader
                      label="Avg length"
                      sortKey="avg_comment_length"
                      activeKey={sortKey}
                      ascending={sortAsc}
                      onSort={handleSort}
                      numeric
                    />
                    <SortableHeader
                      label="PMs"
                      sortKey="private_message_count"
                      activeKey={sortKey}
                      ascending={sortAsc}
                      onSort={handleSort}
                      numeric
                    />
                    <SortableHeader
                      label="Avg PM length"
                      sortKey="avg_private_message_length"
                      activeKey={sortKey}
                      ascending={sortAsc}
                      onSort={handleSort}
                      numeric
                    />
                  </tr>
                </thead>
                <tbody>
                  {sortedCharacters.map((row: CharacterWriting) => (
                    <tr
                      key={row.character_id}
                      className="border-b border-theme-subtle last:border-0"
                    >
                      <td className="py-2 pr-4 text-content-primary">
                        <span className="font-medium">{row.character_name}</span>
                        {!row.is_active && (
                          <span className="ml-2 text-xs text-content-tertiary">
                            (inactive)
                          </span>
                        )}
                      </td>
                      <td className="py-2 pr-4 text-right tabular-nums text-content-secondary">
                        {numberFormat.format(row.comment_count)}
                      </td>
                      <td className="py-2 pr-4 text-right tabular-nums text-content-secondary">
                        {numberFormat.format(row.avg_comment_length)}
                      </td>
                      <td className="py-2 pr-4 text-right tabular-nums text-content-secondary">
                        {numberFormat.format(row.private_message_count)}
                      </td>
                      <td className="py-2 text-right tabular-nums text-content-secondary">
                        {numberFormat.format(row.avg_private_message_length)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </Card>
    </div>
  );
}

interface SortableHeaderProps {
  label: string;
  sortKey: SortKey;
  activeKey: SortKey;
  ascending: boolean;
  onSort: (key: SortKey) => void;
  numeric?: boolean;
}

function SortableHeader({
  label,
  sortKey,
  activeKey,
  ascending,
  onSort,
  numeric = false,
}: SortableHeaderProps) {
  const isActive = activeKey === sortKey;
  return (
    <th
      scope="col"
      aria-sort={isActive ? (ascending ? 'ascending' : 'descending') : 'none'}
      className={`py-2 pr-4 font-medium ${numeric ? 'text-right' : 'text-left'}`}
    >
      {/* Ghost variant so the header reads as a label rather than a control,
          while still carrying the library's focus and hover treatment. */}
      <Button
        variant="ghost"
        size="sm"
        onClick={() => onSort(sortKey)}
        className={`!px-0 !font-medium ${
          isActive ? 'text-content-primary' : 'text-content-secondary'
        }`}
      >
        {label}
        {isActive && <span aria-hidden="true">{ascending ? ' ▲' : ' ▼'}</span>}
      </Button>
    </th>
  );
}
