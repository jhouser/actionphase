import { useMemo, useState } from 'react';
import { ChevronLeft, ChevronRight, EyeOff } from 'lucide-react';
import { Drawer, Toggle } from '../ui';
import { COMMON_ROOM_UTILITIES } from './registry';
import type { UtilityContext } from './types';
import { useScreenshotMode } from '../../hooks/useScreenshotMode';

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
 */
export function UtilityDrawer({ open, onClose, ctx }: UtilityDrawerProps) {
  const [activeId, setActiveId] = useState<string | null>(null);
  const { screenshotModeEnabled, toggleScreenshotMode } = useScreenshotMode();

  const available = useMemo(
    () => COMMON_ROOM_UTILITIES.filter((u) => u.isAvailable(ctx)),
    [ctx]
  );

  const active = activeId ? available.find((u) => u.id === activeId) ?? null : null;

  const handleClose = () => {
    onClose();
    // Reset to the list after the close transition so it reopens on the menu.
    setTimeout(() => setActiveId(null), 200);
  };

  return (
    <Drawer
      open={open}
      onClose={handleClose}
      title={active ? active.label : 'Utilities'}
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
          {ctx.isAnonymous && (
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
          {available.length === 0 && !ctx.isAnonymous && (
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
