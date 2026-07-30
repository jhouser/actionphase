import { useEffect, useMemo, useState } from 'react';
import { ChevronLeft, ChevronRight, EyeOff } from 'lucide-react';
import { Drawer, Toggle } from '../ui';
import { UTILITY_DRAWER_UTILITIES } from './registry';
import type { UtilityContext } from './types';
import { useScreenshotMode } from '../../hooks/useScreenshotMode';
import { LAYERS } from '../../config/layers';
import { useBodyScrollLock, useOverlayLockCount } from '../../hooks/useBodyScrollLock';

interface UtilityDrawerProps {
  open: boolean;
  onClose: () => void;
  ctx: UtilityContext;
}

/**
 * The common-room Utility Drawer. Hosts a registry-driven set of utilities
 * (character sheet, dice roller, …). When opened it shows a list of available
 * utilities; selecting one renders that utility's panel with a back button.
 *
 * Built on the shared `ui/Drawer` primitive, so it slides in as a right sidebar
 * on desktop and a bottom sheet on mobile, with dark-mode support for free.
 *
 * Opened over another overlay (a thread view) it stacks above it and drops its
 * own scrim, but it remains a modal dialog: the overlay underneath is inert
 * while the drawer is open. That's deliberate — the drawer is reference material
 * you open, read, and close, not a second pane you work in alongside the thread.
 * Headless UI's Dialog has no non-modal mode, so making it truly non-modal would
 * mean hand-rolling focus management.
 */
export function UtilityDrawer({ open, onClose, ctx }: UtilityDrawerProps) {
  const [activeId, setActiveId] = useState<string | null>(null);
  const { screenshotModeEnabled, toggleScreenshotMode } = useScreenshotMode();

  // Screenshot mode hides usernames within an anonymous game, so it's only
  // offered there — not on pages outside a game.
  const isAnonymousGame = ctx.game?.isAnonymous ?? false;

  const available = useMemo(
    () => UTILITY_DRAWER_UTILITIES.filter((u) => u.isAvailable(ctx)),
    [ctx]
  );

  const active = activeId ? available.find((u) => u.id === activeId) ?? null : null;

  // Reset to the utility list whenever the drawer closes — however it closed.
  // A panel may close the drawer itself (e.g. the character sheet opens its own
  // modal via ctx.openCharacterSheet, which sets `open` to false directly rather
  // than calling onClose). Without this, activeId stays on that panel and the
  // drawer reopens straight into it, re-firing the panel's open-on-mount effect
  // and never showing the list again until a page refresh.
  useEffect(() => {
    if (!open) {
      setActiveId(null);
    }
  }, [open]);

  // Hold a lock so the page behind stays put, and so an overlay opening over
  // *us* can tell it is stacking.
  const hasLock = useBodyScrollLock(open);

  // Whether another overlay is on screen underneath. Our own lock is acquired in
  // an effect, so it is absent on the frame we open and present afterwards —
  // subtract it once held, and what remains is somebody else (e.g. a thread
  // modal). Comparing the raw count against a fixed threshold instead would
  // paint one un-stacked frame (full scrim, uncapped sheet) before correcting
  // itself mid-transition.
  const othersLocked = useOverlayLockCount() - (hasLock ? 1 : 0);
  const isStacked = open && othersLocked > 0;

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={active ? active.label : 'Utilities'}
      zIndexClass={LAYERS.drawer}
      // The thread modal already dims and blurs the page; a second scrim on top
      // just makes it murky.
      hideBackdrop={isStacked}
      // A 80vh sheet over a thread leaves a useless sliver of it on phones.
      // Cap it so the thread stays visibly present behind the drawer.
      panelClassName={isStacked ? 'max-h-[60vh] lg:max-h-full' : ''}
    >
      {active ? (
        <div className="flex flex-col h-full">
          <button
            type="button"
            onClick={() => setActiveId(null)}
            className="flex items-center gap-1 px-4 py-2 text-sm text-content-secondary hover:text-content-primary transition-colors shrink-0"
            data-faro-user-action-name="utility-drawer-back"
          >
            <ChevronLeft className="w-4 h-4" />
            All utilities
          </button>
          <div className="flex-1 min-h-0">
            <active.Panel ctx={ctx} />
          </div>
        </div>
      ) : (
        <ul className="p-2" data-testid="utility-list">
          {isAnonymousGame && (
            <li>
              <Toggle
                checked={screenshotModeEnabled}
                onChange={toggleScreenshotMode}
                size="sm"
                icon={<EyeOff className="w-5 h-5" />}
                label="Screenshot Mode"
                description="Hide all usernames so screenshots don't reveal who's playing."
                className="px-3 py-3 hover:surface-raised"
                data-testid="screenshot-mode-toggle"
                data-faro-user-action-name="toggle-screenshot-mode"
              />
            </li>
          )}
          {available.length === 0 && !isAnonymousGame && (
            <li className="text-sm text-content-secondary text-center py-6 px-2">
              No utilities available.
            </li>
          )}
          {available.map((u) => {
            const Icon = u.icon;
            return (
              <li key={u.id}>
                <button
                  type="button"
                  onClick={() => setActiveId(u.id)}
                  className="w-full flex items-center gap-3 px-3 py-3 rounded-md hover:surface-raised transition-colors text-left group"
                  data-testid={`utility-${u.id}`}
                  data-faro-user-action-name="utility-drawer-open-utility"
                >
                  <span className="shrink-0 text-content-secondary group-hover:text-interactive-primary">
                    <Icon className="w-5 h-5" />
                  </span>
                  <span className="flex-1 min-w-0">
                    <span className="block text-sm font-medium text-content-primary">
                      {u.label}
                    </span>
                    <span className="block text-xs text-content-secondary">
                      {u.description}
                    </span>
                  </span>
                  <ChevronRight className="w-4 h-4 shrink-0 text-content-tertiary" />
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </Drawer>
  );
}
