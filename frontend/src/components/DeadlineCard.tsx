import { useState, useEffect } from 'react';
import { format } from 'date-fns';
import { InformationCircleIcon } from '@heroicons/react/24/outline';
import type { UnifiedDeadline } from '../types/deadlines';
import { getDeadlineUrgency } from '../utils/deadlineUrgency';


interface DeadlineCardProps {
  deadline: UnifiedDeadline;
  isGM: boolean;
  onEdit?: () => void;
  onDelete?: () => void;
  onExtend?: () => void;
  onClick?: () => void;
}


/**
 * Format countdown in compact format (e.g., "18h 23m", "2d 3h")
 */
function formatCountdown(deadlineStr?: string): string {
  if (!deadlineStr) return '';

  try {
    const deadlineDate = new Date(deadlineStr);
    const now = new Date();
    const ms = deadlineDate.getTime() - now.getTime();

    if (ms < 0) return 'Expired';

    const hours = Math.floor(ms / (1000 * 60 * 60));
    const minutes = Math.floor((ms % (1000 * 60 * 60)) / (1000 * 60));

    if (hours >= 24) {
      const days = Math.floor(hours / 24);
      const remainingHours = hours % 24;
      return `${days}d ${remainingHours}h`;
    }

    return `${hours}h ${minutes}m`;
  } catch {
    return '';
  }
}

/**
 * DeadlineCard - Compact horizontal card for displaying a single deadline
 *
 * Features:
 * - Color-coded border based on urgency (red/yellow/blue)
 * - Real-time countdown timer
 * - Description tooltip on info icon hover (when description exists)
 * - GM edit/delete actions on hover
 * - Clickable to navigate to relevant content
 * - 220px width allows 4 cards comfortably in a row
 * - 32-character title limit (increased from 20 for better readability)
 *
 * @example
 * ```tsx
 * <DeadlineCard
 *   deadline={deadline}
 *   isGM={true}
 *   onEdit={() => handleEdit(deadline)}
 *   onDelete={() => handleDelete(deadline)}
 * />
 * ```
 */
export function DeadlineCard({ deadline, isGM, onEdit, onDelete, onExtend, onClick }: DeadlineCardProps) {
  const [countdown, setCountdown] = useState(formatCountdown(deadline.deadline));
  const [urgency, setUrgency] = useState(getDeadlineUrgency(deadline.deadline));
  const [showActions, setShowActions] = useState(false);

  // Update countdown every minute
  useEffect(() => {
    const updateCountdown = () => {
      setCountdown(formatCountdown(deadline.deadline));
      setUrgency(getDeadlineUrgency(deadline.deadline));
    };

    updateCountdown();
    const interval = setInterval(updateCountdown, 60000); // Update every minute

    return () => clearInterval(interval);
  }, [deadline.deadline]);

  // Urgency color classes
  const urgencyClasses = {
    critical: 'border-semantic-danger bg-semantic-danger-subtle',
    warning: 'border-semantic-warning bg-semantic-warning-subtle',
    normal: 'border-interactive-primary bg-interactive-primary-subtle',
    expired: 'border-theme-strong surface-sunken opacity-60',
  };

  const urgencyTextClasses = {
    critical: 'text-content-primary',
    warning: 'text-content-primary',
    normal: 'text-interactive-primary',
    expired: 'text-content-tertiary',
  };

  const formattedDate = deadline.deadline ? format(new Date(deadline.deadline), 'MMM d, h:mm a') : '';

  // Truncate title if too long (increased from 20 to 32 characters for better readability)
  const displayTitle = deadline.title.length > 32 ? deadline.title.slice(0, 32) + '...' : deadline.title;

  return (
    <div
      className={`
        relative rounded-lg border-2 p-3 min-w-[200px] max-w-[220px]
        transition-all duration-200
        ${urgencyClasses[urgency]}
        ${onClick ? 'cursor-pointer hover:shadow-md' : ''}
        ${isGM ? 'hover:shadow-md' : ''}
      `}
      onMouseEnter={() => isGM && setShowActions(true)}
      onMouseLeave={() => setShowActions(false)}
      // focus/blur here are the delegated focusin/focusout, so they also fire as
      // focus moves between descendants. Ignoring moves that stay inside the card
      // keeps the GM actions mounted while tabbing into them.
      onFocus={() => isGM && setShowActions(true)}
      onBlur={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget as Node | null)) {
          setShowActions(false);
        }
      }}
      title={deadline.title} // Show full title on hover
    >
      {/* Click target. A real <button> covering the card, rather than role="button"
          on the container, so the GM action buttons are siblings instead of nested
          controls — nesting breaks the container's accessible name and lets a
          bubbled Space keypress hijack the inner button's activation. */}
      {onClick && (
        <button
          type="button"
          onClick={onClick}
          className="absolute inset-0 z-0 rounded-lg cursor-pointer focus:outline-none focus-visible:ring-2 focus-visible:ring-interactive-primary"
          aria-label={deadline.title}
        />
      )}

      {/* GM Actions (shown on hover) */}
      {isGM && showActions && (
        <div className="absolute top-1 right-1 flex gap-1 z-10">
          {onEdit && (
            <button
              onClick={(e) => {
                e.stopPropagation();
                onEdit();
              }}
              className="p-1 rounded bg-surface-page hover:bg-surface-raised transition-colors text-content-primary"
              aria-label="Edit deadline"
              title="Edit deadline"
            >
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
              </svg>
            </button>
          )}
          {onExtend && urgency !== 'expired' && (
            <button
              onClick={(e) => {
                e.stopPropagation();
                onExtend();
              }}
              className="p-1 rounded bg-surface-page hover:bg-surface-raised transition-colors text-content-primary"
              aria-label="Extend deadline"
              title="Extend deadline"
            >
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </button>
          )}
          {onDelete && (
            <button
              onClick={(e) => {
                e.stopPropagation();
                onDelete();
              }}
              className="p-1 rounded bg-surface-page hover:bg-semantic-danger-subtle transition-colors text-semantic-danger"
              aria-label="Delete deadline"
              title="Delete deadline"
            >
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </button>
          )}
        </div>
      )}

      {/* Card text. pointer-events-none so clicks fall through to the overlay
          button beneath it; it carries no interactive elements of its own. */}
      <div className="relative pointer-events-none">
        {/* Title (emoji removed for more space) */}
        <div className="mb-1">
          <span
            className="text-sm font-semibold text-content-primary"
            title={deadline.title}
            data-testid="deadline-title"
          >
            {displayTitle}
          </span>
        </div>

        {/* Countdown */}
        {countdown && (
          <div className={`text-2xl font-bold mb-1 ${urgencyTextClasses[urgency]}`}>
            {countdown}
          </div>
        )}

        {/* Date/Time */}
        {formattedDate && (
          <div className="text-xs text-content-secondary">
            {urgency === 'expired' ? 'Due: ' : ''}{formattedDate}
          </div>
        )}
      </div>

      {/* Description Tooltip (only for non-system deadlines with user-entered descriptions) */}
      {deadline.description && deadline.description.trim() && !deadline.is_system_deadline && (
        <div className="absolute bottom-2 right-2">
          <div className="group relative">
            <InformationCircleIcon
              className="w-4 h-4 text-content-tertiary hover:text-content-primary cursor-help transition-colors"
              aria-label="View description"
              title="View description"
            />

            {/* Tooltip - positioned above card */}
            <div className="
              invisible group-hover:visible
              absolute bottom-full right-0 mb-2
              w-64 p-3 rounded-lg
              bg-surface-raised border border-theme-default shadow-lg
              text-xs text-content-primary
              z-50
              pointer-events-none
            ">
              <p className="whitespace-pre-wrap">{deadline.description}</p>
              {/* Arrow pointing down to icon */}
              <div className="
                absolute top-full right-3 -mt-1
                border-8 border-transparent border-t-surface-raised
              "></div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
