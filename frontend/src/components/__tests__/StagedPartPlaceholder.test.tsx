import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { act, render, screen } from '@testing-library/react';
import { StagedPartPlaceholder } from '../StagedPartPlaceholder';

/**
 * These tests drive the clock directly rather than waiting, because the
 * behaviour under test is entirely about *when* things happen: the countdown
 * ticks once a second, but the refetch is throttled to one every five.
 */
describe('StagedPartPlaceholder', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2024-01-15T10:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  const inSeconds = (n: number) => new Date(Date.now() + n * 1000).toISOString();

  const advance = async (ms: number) => {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(ms);
    });
  };

  it('shows position and a countdown while the part is still waiting', () => {
    render(<StagedPartPlaceholder partNumber={2} partCount={3} unlocksAt={inSeconds(90)} />);

    expect(screen.getByTestId('staged-part-placeholder-2')).toBeInTheDocument();
    expect(screen.getByText('Part 2 of 3')).toBeInTheDocument();
    expect(screen.getByText('1:30')).toBeInTheDocument();
  });

  it('counts down as time passes', async () => {
    render(<StagedPartPlaceholder partNumber={2} partCount={3} unlocksAt={inSeconds(90)} />);

    await advance(30_000);

    expect(screen.getByText('1:00')).toBeInTheDocument();
  });

  it('shows a plain pending state when the unlock time is not yet knowable', () => {
    render(<StagedPartPlaceholder partNumber={3} partCount={3} />);

    expect(screen.getByText('Pending')).toBeInTheDocument();
    // Scoped to the visible copy: the sr-only status says the same thing in a
    // sentence, so an unscoped match now finds both.
    expect(
      screen.getByText(/^This part unlocks after the previous one is revealed\.$/i)
    ).toBeInTheDocument();
  });

  it('shows "Unlocking…" rather than a negative timer once the deadline passes', async () => {
    render(<StagedPartPlaceholder partNumber={2} partCount={3} unlocksAt={inSeconds(5)} />);

    await advance(10_000);

    // Both the visible label and the sr-only status now say "unlocking", which
    // is the point of the fix; assert on the visible one specifically.
    expect(screen.getByText('Unlocking…')).toBeInTheDocument();
    // The real guard: no negative timer anywhere, visible or spoken.
    expect(screen.queryByText(/-\d/)).not.toBeInTheDocument();
  });

  it('asks the caller to refetch as soon as the countdown expires', async () => {
    const onExpired = vi.fn();
    render(<StagedPartPlaceholder partNumber={2} partCount={3} unlocksAt={inSeconds(5)} onExpired={onExpired} />);

    expect(onExpired).not.toHaveBeenCalled();

    await advance(5_000);

    expect(onExpired).toHaveBeenCalledTimes(1);
  });

  // The regression this file exists for. The release worker runs on its own
  // ~60s cadence, so the refetch at the moment of expiry nearly always finds
  // the part still locked. Firing once left the player stuck on "Unlocking…"
  // until they reloaded the page by hand.
  it('keeps asking while the server has not released the part yet', async () => {
    const onExpired = vi.fn();
    render(<StagedPartPlaceholder partNumber={2} partCount={3} unlocksAt={inSeconds(5)} onExpired={onExpired} />);

    await advance(5_000);
    expect(onExpired).toHaveBeenCalledTimes(1);

    // A full worker interval of the server lagging behind the countdown.
    await advance(60_000);

    expect(onExpired.mock.calls.length).toBeGreaterThan(1);
  });

  it('throttles those repeat asks well below the once-a-second display tick', async () => {
    const onExpired = vi.fn();
    render(<StagedPartPlaceholder partNumber={2} partCount={3} unlocksAt={inSeconds(5)} onExpired={onExpired} />);

    await advance(5_000);
    await advance(30_000);

    // 30s of lag at one poll per 5s is ~7 calls; one per second would be 30.
    expect(onExpired.mock.calls.length).toBeLessThan(12);
  });

  // Everything this component conveys is conveyed visually: position in a
  // dashed box, state by a pulsing "Unlocking…" or a mm:ss digit run. A screen
  // reader user gets a bare number with no indication it is a countdown, and no
  // announcement when the wait ends — so the one moment that matters, the part
  // becoming available, passes silently.
  it('describes the wait to a screen reader, not only visually', () => {
    render(<StagedPartPlaceholder partNumber={2} partCount={3} unlocksAt={inSeconds(90)} />);

    const status = screen.getByRole('status');
    // Polite: a countdown ticking every second must never interrupt what the
    // user is reading, which is exactly what assertive would do.
    expect(status).toHaveAttribute('aria-live', 'polite');
    expect(status).toHaveTextContent(/Part 2 of 3 unlocks in/i);
  });

  it('announces that the part is arriving once the deadline passes', async () => {
    render(<StagedPartPlaceholder partNumber={2} partCount={3} unlocksAt={inSeconds(5)} />);

    await advance(6_000);

    // The visual cue here is an animate-pulse class, which conveys nothing
    // without sight; this is the moment the player is actually waiting for.
    expect(screen.getByRole('status')).toHaveTextContent(/Part 2 of 3 is unlocking now/i);
  });

  it('stops polling once the part is no longer locked', async () => {
    const onExpired = vi.fn();
    const { unmount } = render(
      <StagedPartPlaceholder partNumber={2} partCount={3} unlocksAt={inSeconds(5)} onExpired={onExpired} />
    );

    await advance(5_000);
    // The caller swaps the placeholder for real content once the server
    // releases; polling must not outlive the component.
    unmount();
    const callsAtUnmount = onExpired.mock.calls.length;

    await advance(60_000);

    expect(onExpired).toHaveBeenCalledTimes(callsAtUnmount);
  });
});
