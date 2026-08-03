import { GlobalUtilityDrawer } from '../components/utility-drawer/GlobalUtilityDrawer'
import { useUtilityDrawer } from '../contexts/UtilityDrawerContext'

/**
 * Stands in for the global nav's Utilities button in tests that render a game
 * surface (CommonRoom, HistoryView) without Layout around it.
 *
 * The real button lives in Layout, so a test that mounts only the game surface
 * has no way to open the drawer. This opens it through the same context the nav
 * button uses, so the provider wiring under test is the real one.
 */
function UtilityDrawerTestTrigger() {
  const { openDrawer } = useUtilityDrawer()
  return (
    <button type="button" data-testid="utility-drawer-toggle" onClick={openDrawer}>
      Utilities
    </button>
  )
}

/**
 * The drawer plus a way to open it, for tests of a surface that contributes
 * game context to the drawer but doesn't render it. Mirrors how App composes
 * GlobalUtilityDrawer at the root alongside Layout's Utilities button.
 */
export function UtilityDrawerHarness() {
  return (
    <>
      <UtilityDrawerTestTrigger />
      <GlobalUtilityDrawer />
    </>
  )
}
