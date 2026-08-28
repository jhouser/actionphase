import { useState } from 'react';
import { UpcomingDeadlinesCard } from '../UpcomingDeadlinesCard';
import { RecentActivityCard } from '../RecentActivityCard';
import type { DashboardDeadline, DashboardMessage } from '../../types/dashboard';

interface ActivityTabsProps {
  deadlines: DashboardDeadline[];
  messages: DashboardMessage[];
}

/**
 * ActivityTabs - Mobile-friendly tabbed view for Deadlines and Activity
 * Shows both in a single card with tabs to save screen space on mobile
 */
export function ActivityTabs({ deadlines, messages }: ActivityTabsProps) {
  const [activeTab, setActiveTab] = useState<'deadlines' | 'activity'>('deadlines');

  const hasDeadlines = deadlines.length > 0;
  const hasActivity = messages.length > 0;

  // If only one has content, show that one without tabs
  if (hasDeadlines && !hasActivity) {
    return <UpcomingDeadlinesCard deadlines={deadlines} />;
  }

  if (!hasDeadlines && hasActivity) {
    return <RecentActivityCard messages={messages} />;
  }

  // If neither has content, don't render
  if (!hasDeadlines && !hasActivity) {
    return null;
  }

  // Both have content - show tabs
  return (
    <div className="surface-base rounded-lg shadow-md overflow-hidden">
      {/* Tab Headers */}
      <div className="flex border-b border-theme-default">
        <button
          onClick={() => setActiveTab('deadlines')}
          className={`flex-1 px-4 py-3 text-sm font-medium transition-colors ${
            activeTab === 'deadlines'
              ? 'text-interactive-primary border-b-2 border-interactive-primary surface-raised'
              : 'text-content-secondary hover:text-content-primary'
          }`}
        >
          Deadlines
        </button>
        <button
          onClick={() => setActiveTab('activity')}
          className={`flex-1 px-4 py-3 text-sm font-medium transition-colors ${
            activeTab === 'activity'
              ? 'text-interactive-primary border-b-2 border-interactive-primary surface-raised'
              : 'text-content-secondary hover:text-content-primary'
          }`}
        >
          Activity
        </button>
      </div>

      {/* Tab Content - Remove card styling from children since we have our own card */}
      <div>
        {activeTab === 'deadlines' ? (
          <div className="p-6">
            <UpcomingDeadlinesCard deadlines={deadlines} />
          </div>
        ) : (
          <div className="p-6">
            <RecentActivityCard messages={messages} />
          </div>
        )}
      </div>
    </div>
  );
}
