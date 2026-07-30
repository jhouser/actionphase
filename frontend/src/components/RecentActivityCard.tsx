import { useState } from 'react';
import { Link } from 'react-router-dom';
import type { DashboardMessage } from '../types/dashboard';
import { MessageSquare, Clock, ChevronDown, ChevronUp } from 'lucide-react';
import { formatRelativeTime } from '../lib/utils/dates';

const stripMarkdown = (text: string) => text.replace(/(\*\*|__|[*_`#>])/g, '').trim();

interface RecentActivityCardProps {
  messages: DashboardMessage[];
  /**
   * Renders the card collapsed behind its header. Used on the single-game
   * dashboard, where this is ambient context competing with the Inbox for the
   * fold; the multi-game sidebar leaves it expanded.
   */
  defaultCollapsed?: boolean;
}

/**
 * Generate deep link URL for a message based on its type
 */
function getMessageLink(message: DashboardMessage): string {
  const baseUrl = `/games/${message.game_id}`;

  if (message.message_type === 'post') {
    // For posts, link to Common Room tab with comment parameter to scroll to post
    return `${baseUrl}?tab=common-room&comment=${message.message_id}`;
  } else if (message.message_type === 'comment') {
    // For comments, link to Common Room tab with comment parameter
    return `${baseUrl}?tab=common-room&comment=${message.message_id}`;
  } else if (message.message_type === 'private_message') {
    // For private messages, link to Messages tab
    return `${baseUrl}?tab=messages`;
  }

  // Fallback to game page
  return baseUrl;
}

/**
 * RecentActivityCard - Display recent messages from games
 */
export function RecentActivityCard({ messages, defaultCollapsed = false }: RecentActivityCardProps) {
  const [isCollapsed, setIsCollapsed] = useState(defaultCollapsed);

  if (messages.length === 0) {
    return null;
  }

  const header = (
    <>
      <MessageSquare className="w-5 h-5 text-content-tertiary mr-2 flex-shrink-0" />
      <h2 className="text-lg font-bold text-content-primary">Recent Activity</h2>
    </>
  );

  return (
    <div className="surface-base rounded-lg shadow-lg p-4 sm:p-8">
      {defaultCollapsed ? (
        <button
          type="button"
          onClick={() => setIsCollapsed((prev) => !prev)}
          aria-expanded={!isCollapsed}
          className={`w-full flex items-center justify-between rounded ${isCollapsed ? '' : 'mb-4'}`}
        >
          <span className="flex items-center">
            {header}
            <span className="ml-2 text-sm font-normal text-content-tertiary">
              {messages.length}
            </span>
          </span>
          {isCollapsed ? (
            <ChevronDown className="w-5 h-5 text-content-tertiary flex-shrink-0" />
          ) : (
            <ChevronUp className="w-5 h-5 text-content-tertiary flex-shrink-0" />
          )}
        </button>
      ) : (
        <div className="flex items-center mb-4">{header}</div>
      )}
      {!isCollapsed && (
      <div className="space-y-4">
        {messages.map((message) => {
          const isPrivate = message.message_type === 'private_message';
          return (
            <Link
              key={message.message_id}
              to={getMessageLink(message)}
              className={`block border-b border-theme-default pb-4 last:border-b-0 last:pb-0 hover:surface-raised sm:-mx-2 sm:px-2 py-2 rounded transition-colors ${
                isPrivate ? 'border-l-2 border-l-interactive-primary pl-3' : ''
              }`}
            >
              <div className="flex items-start justify-between mb-2">
                <div className="flex-1">
                  <p className="text-sm font-medium text-content-primary">
                    {message.game_title}
                  </p>
                  <p className="text-xs text-content-tertiary">
                    {message.character_name
                      ? message.author_name
                        ? `${message.author_name} as ${message.character_name}`
                        : message.character_name
                      : message.author_name || 'Unknown'}
                  </p>
                </div>
                <div className="ml-2 flex items-center text-xs text-content-tertiary">
                  <Clock className="w-3 h-3 mr-1" />
                  {formatRelativeTime(message.created_at)}
                </div>
              </div>
              <p className="text-sm text-content-secondary line-clamp-2">{stripMarkdown(message.content)}</p>
              <p className={`text-xs mt-1 ${isPrivate ? 'text-interactive-primary font-medium' : 'text-content-tertiary'}`}>
                {message.message_type === 'post' ? 'Post' :
                 message.message_type === 'comment' ? 'Comment' :
                 'Private message'}
              </p>
            </Link>
          );
        })}
      </div>
      )}
    </div>
  );
}

