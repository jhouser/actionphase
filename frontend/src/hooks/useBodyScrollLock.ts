import { useEffect, useState, useSyncExternalStore } from 'react';

/**
 * Ref-counted body scroll lock, shared by every overlay.
 *
 * Overlays used to each write `document.body.style.overflow` directly and
 * restore the previous value on unmount. With two overlapping owners — a thread
 * modal and the drawer over it, or ThreadViewModal's own nested recursion —
 * whichever unmounted *last* won, and it restored a value captured while the
 * other overlay had already locked. Closing the inner overlay could therefore
 * leave the page scrollable behind the outer one still on screen.
 *
 * Counting instead means N overlays lock once and the page unlocks only when
 * the last one releases.
 */

let lockCount = 0;
let originalOverflow: string | null = null;
const subscribers = new Set<() => void>();

function notify(): void {
  for (const callback of subscribers) callback();
}

function acquire(): void {
  if (lockCount === 0) {
    originalOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
  }
  lockCount += 1;
  notify();
}

function release(): void {
  lockCount = Math.max(0, lockCount - 1);
  if (lockCount === 0) {
    // Restore what was there before the *first* overlay locked, not whatever
    // the body happened to read mid-stack.
    document.body.style.overflow = originalOverflow ?? '';
    originalOverflow = null;
  }
  notify();
}

function subscribe(callback: () => void): () => void {
  subscribers.add(callback);
  return () => {
    subscribers.delete(callback);
  };
}

function getSnapshot(): number {
  return lockCount;
}

// The server never has a body to lock, and overlays render nothing there.
function getServerSnapshot(): number {
  return 0;
}

/**
 * Locks body scroll for as long as the calling component is mounted and
 * `active` is true.
 *
 * Returns whether this caller's lock is currently held. Because the lock is
 * taken in an effect, that is false on the render where `active` first turns
 * true and true from the next render on — which lets a caller subtract its own
 * lock from `useOverlayLockCount()` without guessing at the timing.
 */
export function useBodyScrollLock(active = true): boolean {
  const [held, setHeld] = useState(false);

  useEffect(() => {
    if (!active) return;
    acquire();
    setHeld(true);
    return () => {
      release();
      setHeld(false);
    };
  }, [active]);

  return held;
}

/**
 * How many overlays currently hold a scroll lock. Lets an overlay tell whether
 * it is opening on top of another one — the drawer uses this to suppress its
 * own backdrop and shrink its mobile sheet when it stacks over a modal, rather
 * than double-dimming an already-dimmed page.
 *
 * Subscribes to changes so a component that mounts before the count changes
 * still re-renders when it does.
 */
export function useOverlayLockCount(): number {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}

/** Test-only: drop all state between cases. */
export function __resetBodyScrollLockForTests(): void {
  lockCount = 0;
  originalOverflow = null;
  subscribers.clear();
}
