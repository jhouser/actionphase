import { Wrench } from 'lucide-react';
import { Button } from './ui';
import { useOptionalUtilityDrawer } from '../contexts/UtilityDrawerContext';

interface UtilitiesButtonProps {
  /** Overrides the default icon-button styling for the host surface. */
  className?: string;
}

/**
 * Opens the Utility Drawer from inside an overlay.
 *
 * The global nav already has a Utilities button, but a modal covers the nav
 * with a blurred backdrop — the button is there, and after the layering fix it
 * even works, but nobody will find it behind the blur. Overlays that are worth
 * reaching for a dice roller or character sheet from host their own copy.
 *
 * Renders nothing when no UtilityDrawerProvider is mounted, so the host modal
 * still works in tests and on surfaces without the drawer, rather than throwing.
 */
export function UtilitiesButton({ className = '' }: UtilitiesButtonProps) {
  const utilityDrawer = useOptionalUtilityDrawer();
  if (!utilityDrawer) return null;

  return (
    <Button
      variant="ghost"
      size="sm"
      onClick={utilityDrawer.openDrawer}
      title="Utilities"
      aria-label="Utilities"
      data-testid="overlay-utility-drawer-toggle"
      data-faro-user-action-name="open-utility-drawer"
      className={`text-content-tertiary hover:text-content-secondary h-auto p-0 ${className}`}
    >
      <Wrench className="w-5 h-5" />
    </Button>
  );
}
