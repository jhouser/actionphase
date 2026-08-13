import React, { useEffect, useState } from 'react';

interface StagedPartPlaceholderProps {
  partNumber?: number;
  partCount?: number;
  /**
   * ISO timestamp when the server intends to reveal this part. Absent for parts
   * beyond the next one due out: their unlock time is genuinely unknown until
   * their predecessor releases, so they show a plain pending state.
   */
  unlocksAt?: string;
  /**
   * Called once when the countdown reaches zero, so the caller can refetch.
   * The server is the authority on release — the placeholder never flips itself
   * to revealed content it does not have.
   */
  onExpired?: () => void;
}

/**
 * How often to re-ask the server once the countdown has expired.
 *
 * Kept in the same order of magnitude as the backend's release interval
 * (DefaultReleaseInterval, one minute) so a player waits at most a few seconds
 * past the actual release, without polling once per second for a whole minute.
 */
const EXPIRED_POLL_INTERVAL_MS = 5000;

function formatRemaining(ms: number): string {
  const totalSeconds = Math.ceil(ms / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  const pad = (n: number) => n.toString().padStart(2, '0');
  return hours > 0 ? `${hours}:${pad(minutes)}:${pad(seconds)}` : `${minutes}:${pad(seconds)}`;
}

/**
 * The dashed stand-in a player sees where a not-yet-revealed part will appear.
 *
 * This component is never given the locked text — the server blanks it in SQL
 * (see GetUserResults) — so there is nothing here to leak even if the markup
 * changes. It renders position, timing, and nothing else.
 *
 * Hitting zero does not reveal anything locally. The release worker ticks once
 * a minute, so a countdown that has expired can precede the actual release by
 * up to ~60s; that window shows "Unlocking…" rather than a negative timer,
 * and `onExpired` asks the caller to refetch.
 *
 * That refetch REPEATS while the countdown sits at zero, on a slower cadence
 * than the display tick. A single fire would land inside the worker's lag
 * almost every time, come back still locked, and never ask again — leaving the
 * player on "Unlocking…" until they reloaded the page by hand.
 */
export const StagedPartPlaceholder: React.FC<StagedPartPlaceholderProps> = ({
  partNumber,
  partCount,
  unlocksAt,
  onExpired,
}) => {
  const [remainingMs, setRemainingMs] = useState<number | null>(() =>
    unlocksAt ? new Date(unlocksAt).getTime() - Date.now() : null
  );

  useEffect(() => {
    if (!unlocksAt) {
      setRemainingMs(null);
      return;
    }

    const target = new Date(unlocksAt).getTime();
    let lastPolledAt = 0;

    const tick = () => {
      const remaining = target - Date.now();
      setRemainingMs(remaining);

      // Poll while expired, not once at the moment of expiry. The worker
      // releases within ~60s of the deadline, so the first ask usually finds
      // the part still locked and we have to ask again. Throttled well below
      // the 1s display tick to keep this from becoming a hot loop against the
      // API; once the server does release, the part stops being locked and
      // this component unmounts, ending the polling on its own.
      if (remaining <= 0) {
        const now = Date.now();
        if (now - lastPolledAt >= EXPIRED_POLL_INTERVAL_MS) {
          lastPolledAt = now;
          onExpired?.();
        }
      }
    };

    tick();
    const interval = setInterval(tick, 1000);
    return () => clearInterval(interval);
  }, [unlocksAt, onExpired]);

  const hasCountdown = remainingMs !== null;
  const isUnlocking = hasCountdown && remainingMs <= 0;

  return (
    <div
      className="p-4 border-2 border-dashed border-theme-default rounded surface-sunken"
      data-testid={`staged-part-placeholder-${partNumber ?? 'pending'}`}
    >
      <div className="flex items-center justify-between gap-3">
        <span className="text-sm font-medium text-content-secondary">
          {partNumber && partCount ? `Part ${partNumber} of ${partCount}` : 'Next part'}
        </span>
        {isUnlocking ? (
          <span className="text-sm font-medium text-content-secondary animate-pulse">
            Unlocking…
          </span>
        ) : hasCountdown ? (
          <span className="text-sm font-mono font-medium text-content-primary">
            {formatRemaining(remainingMs)}
          </span>
        ) : (
          <span className="text-sm text-content-tertiary">Pending</span>
        )}
      </div>
      <p className="mt-2 text-sm text-content-tertiary italic">
        {hasCountdown
          ? 'The rest of this result has not been revealed yet.'
          : 'This part unlocks after the previous one is revealed.'}
      </p>
    </div>
  );
};
