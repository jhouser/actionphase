import { Link } from 'react-router-dom';
import { useDashboard } from '../hooks/useDashboard';
import { useDashboardConversations } from '../hooks/useDashboardConversations';
import { DashboardGameCard } from '../components/DashboardGameCard';
import { UrgentActionsCard } from '../components/UrgentActionsCard';
import { RecentActivityCard } from '../components/RecentActivityCard';
import { UpcomingDeadlinesCard } from '../components/UpcomingDeadlinesCard';
import { ActivityTabs } from '../components/Dashboard/ActivityTabs';
import { NotificationDigest } from '../components/NotificationDigest';
import { PrivateMessagePreview } from '../components/PrivateMessagePreview';
import { UnreadInboxSection } from '../components/UnreadInboxSection';

/**
 * DashboardPage - Main user dashboard showing games, actions, and activity
 */
export function DashboardPage() {
  const { data: dashboard, isLoading, error } = useDashboard();

  const activeGames = [...(dashboard?.player_games ?? []), ...(dashboard?.gm_games ?? [])];
  const isSingleGame = activeGames.length === 1;
  const singleGameId = isSingleGame ? activeGames[0].game_id : undefined;
  const { data: unreadConversations = [] } = useDashboardConversations(singleGameId);
  const hasUnreadConversations = unreadConversations.length > 0 && !!singleGameId;
  const hasMessages = (dashboard?.recent_messages.length ?? 0) > 0;
  const hasDeadlines = (dashboard?.upcoming_deadlines.length ?? 0) > 0;

  if (isLoading) {
    return (
      <div className="min-h-screen bg-surface-sunken py-8">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex items-center justify-center">
          <div className="text-center">
            <div className="inline-block animate-spin rounded-full h-12 w-12 border-b-2 border-interactive-primary"></div>
            <p className="mt-4 text-content-secondary">Loading your dashboard...</p>
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen bg-surface-sunken py-8">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex items-center justify-center">
          <div className="text-center">
            <p className="text-semantic-danger text-lg">Failed to load dashboard</p>
            <p className="text-content-secondary mt-2">Please try refreshing the page</p>
          </div>
        </div>
      </div>
    );
  }

  if (!dashboard) {
    return null;
  }

  return (
    <div className="min-h-screen bg-surface-sunken py-8">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        {/* Page Header */}
        <div className="mb-8">
          <div>
            <h1 className="text-3xl font-bold text-content-primary">My Dashboard</h1>
            <p className="mt-2 text-content-secondary">
              Welcome back! Here's what's happening in your games.
            </p>
          </div>
        </div>

        {/* Inbox: repliable notifications inline, plus an FYI digest row */}
        {dashboard.has_games && (
          <div className="mb-8">
            <UnreadInboxSection
              notificationsByType={dashboard.notifications_by_type}
              gameId={singleGameId}
            />
          </div>
        )}

        {/* Empty State for Users Without Games */}
        {!dashboard.has_games ? (
          <div className="bg-surface-base shadow rounded-lg overflow-hidden">
            <div className="px-8 py-12 text-center">
              <h2 className="text-2xl font-bold text-content-primary mb-4">
                Welcome to ActionPhase!
              </h2>
              <p className="text-lg text-content-secondary mb-8 max-w-2xl mx-auto">
                You're not currently in any games. Browse available games to join or create your own campaign.
              </p>
              <Link
                to="/games"
                className="inline-block bg-interactive-primary text-white px-6 py-3 rounded-md font-semibold hover:bg-interactive-primary-hover focus:outline-none focus:ring-2 focus:ring-interactive-primary focus:ring-offset-2 transition-colors"
              >
                Browse Games
              </Link>
            </div>
          </div>
        ) : isSingleGame ? (
          /* Single-game layout: game card + deadlines side by side, then activity panels */
          <div className="space-y-8">
            {/* Urgent Actions */}
            {activeGames.some((game) => game.is_urgent) && (
              <UrgentActionsCard games={activeGames} />
            )}

            {/* Top row: Game Card (2/3) + Deadlines (1/3) */}
            <div className={`grid grid-cols-1 ${hasDeadlines ? 'lg:grid-cols-3' : ''} gap-6 items-stretch`}>
              <div className={`${hasDeadlines ? 'lg:col-span-2' : ''} h-full`}>
                <DashboardGameCard game={activeGames[0]} isSingleGame />
              </div>
              {hasDeadlines && (
                <UpcomingDeadlinesCard deadlines={dashboard.upcoming_deadlines} fillHeight />
              )}
            </div>

            {/* PM preview (the notification digest now lives in the Inbox card) */}
            {hasUnreadConversations && (
              <PrivateMessagePreview
                conversations={unreadConversations}
                gameId={singleGameId!}
              />
            )}

            {/* Recent activity — ambient context, collapsed so the Inbox owns the fold */}
            {hasMessages && (
              <RecentActivityCard messages={dashboard.recent_messages} defaultCollapsed />
            )}

            {/* Audience Games */}
            {dashboard.audience_games.length > 0 && (
              <section>
                <h2 className="text-xl font-bold text-content-primary mb-4">Games I'm Watching</h2>
                <div className="space-y-4">
                  {dashboard.audience_games.map((game) => (
                    <DashboardGameCard key={game.game_id} game={game} isAudience />
                  ))}
                </div>
              </section>
            )}
          </div>
        ) : (
          /* Multi-game layout: existing two-column layout */
          <div>
            {/* Urgent Actions Section */}
            {activeGames.some((game) => game.is_urgent) && (
              <div className="mb-8">
                <UrgentActionsCard games={activeGames} />
              </div>
            )}

            {/* Mobile: Activity/Deadlines Tabs (shown at top before games) */}
            <div className="lg:hidden mb-8">
              <ActivityTabs
                deadlines={dashboard.upcoming_deadlines}
                messages={dashboard.recent_messages}
              />
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
              {/* Left Column - Games */}
              <div className="lg:col-span-2 space-y-8">
                {/* Player Games */}
                {dashboard.player_games.length > 0 && (
                  <section>
                    <h2 className="text-2xl font-bold text-content-primary mb-4">
                      My Games as Player
                    </h2>
                    <div className="space-y-6">
                      {dashboard.player_games.map((game) => (
                        <DashboardGameCard key={game.game_id} game={game} />
                      ))}
                    </div>
                  </section>
                )}

                {/* GM Games */}
                {dashboard.gm_games.length > 0 && (
                  <section>
                    <h2 className="text-2xl font-bold text-content-primary mb-4">
                      Games I'm Running
                    </h2>
                    <div className="space-y-6">
                      {dashboard.gm_games.map((game) => (
                        <DashboardGameCard key={game.game_id} game={game} />
                      ))}
                    </div>
                  </section>
                )}

                {/* Audience Games */}
                {dashboard.audience_games.length > 0 && (
                  <section>
                    <h2 className="text-2xl font-bold text-content-primary mb-4">
                      Games I'm Watching
                    </h2>
                    <div className="space-y-6">
                      {dashboard.audience_games.map((game) => (
                        <DashboardGameCard key={game.game_id} game={game} isAudience />
                      ))}
                    </div>
                  </section>
                )}

                {/* Mixed Role Games */}
                {dashboard.mixed_role_games.length > 0 && (
                  <section>
                    <h2 className="text-2xl font-bold text-content-primary mb-4">
                      Other Games
                    </h2>
                    <div className="space-y-6">
                      {dashboard.mixed_role_games.map((game) => (
                        <DashboardGameCard key={game.game_id} game={game} />
                      ))}
                    </div>
                  </section>
                )}
              </div>

              {/* Desktop: Right Column - Activity & Deadlines */}
              <div className="hidden lg:block space-y-8">
                {Object.keys(dashboard.notifications_by_type).length > 0 && (
                  <NotificationDigest
                    notificationsByType={dashboard.notifications_by_type}
                  />
                )}
                {dashboard.upcoming_deadlines.length > 0 && (
                  <UpcomingDeadlinesCard deadlines={dashboard.upcoming_deadlines} />
                )}
                {dashboard.recent_messages.length > 0 && (
                  <RecentActivityCard messages={dashboard.recent_messages} />
                )}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
