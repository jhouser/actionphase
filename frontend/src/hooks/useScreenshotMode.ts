/**
 * Custom hook for the Common Room's Screenshot Mode toggle.
 *
 * Screenshot Mode hides real usernames on posts/comments in anonymous games so
 * players and audience members can share screenshots without revealing who is
 * playing which character. Session-only — not persisted across reloads.
 *
 * This hook is implemented as a Context to share state across components.
 * Import from '../contexts/ScreenshotModeContext' for the actual implementation.
 */
export { useScreenshotMode } from '../contexts/ScreenshotModeContext';
