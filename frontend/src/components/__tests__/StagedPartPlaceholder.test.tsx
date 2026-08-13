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
    expect(screen.getByText(/unlocks after the previous one is revealed/i)).toBeInTheDocument();
  });

  it('shows "Unlocking…" rather than a negative timer once the deadline passes', async () => {
    render(<StagedPartPlaceholder partNumber={2} partCount={3} unlocksAt={inSeconds(5)} />);

    await advance(10_000);

    expect(screen.getByText(/unlocking/i)).toBeInTheDocument();
    expect(screen.queryByText(/-/)).not.toBeInTheDocument();
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
