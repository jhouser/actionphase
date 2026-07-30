/**
 * Named z-index tiers for overlays.
 *
 * Overlays used to all sit at a bare `z-50`, which meant the stacking order
 * between a modal and the utility drawer was decided by DOM order rather than
 * intent — and DOM order put the drawer underneath, since it mounts at the app
 * root above the page content.
 *
 * The drawer sits deliberately *above* modals: it's a tool you reach for while
 * looking at something else (checking your character sheet mid-thread), not a
 * thing you navigate to instead. Anything the drawer itself launches has to
 * clear the drawer in turn.
 *
 * Values are full Tailwind class names rather than numbers because Tailwind
 * scans source text for classes — a computed `z-${n}` would be purged from the
 * stylesheet.
 */
export const LAYERS = {
  /** Sticky headers, dropdowns, and other in-page overlays. */
  contentOverlay: 'z-40',
  /** Modals and dialogs: ThreadViewModal, Modal. */
  modal: 'z-50',
  /** The utility drawer's panel and backdrop — above any modal. */
  drawer: 'z-60',
  /** A sheet launched *from* the drawer, which must clear the drawer. */
  drawerChild: 'z-70',
} as const;
