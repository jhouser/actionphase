import { lazy, Suspense } from 'react';
import { UtilityDrawer } from './UtilityDrawer';
import { Modal } from '../Modal';
import { Spinner } from '../ui';
import { useUtilityDrawer } from '../../contexts/UtilityDrawerContext';

// The sheet pulls in the whole character-sheet module tree; keep it out of the
// initial bundle since most page loads never open it.
const CharacterSheet = lazy(() =>
  import('../CharacterSheet').then((m) => ({ default: m.CharacterSheet }))
);

/**
 * Renders the Utility Drawer and the character-sheet modal it launches at the
 * app root, so both are available on every page. Mounted once, inside
 * UtilityDrawerProvider.
 */
export function GlobalUtilityDrawer() {
  const { isOpen, closeDrawer, utilityContext, openSheet, closeSheet } = useUtilityDrawer();

  return (
    <>
      <UtilityDrawer open={isOpen} onClose={closeDrawer} ctx={utilityContext} />

      {openSheet && (
        <Modal isOpen onClose={closeSheet} title="">
          <Suspense
            fallback={
              <div className="flex justify-center py-12">
                <Spinner size="lg" />
              </div>
            }
          >
            <CharacterSheet
              characterId={openSheet.characterId}
              canEdit={openSheet.options.canEdit}
              canEditStats={openSheet.options.canEditStats}
              onClose={closeSheet}
              isAnonymous={openSheet.options.isAnonymous}
              userRole={openSheet.options.userRole}
              gameState={openSheet.options.gameState}
              portraitAvatars={openSheet.options.portraitAvatars}
            />
          </Suspense>
        </Modal>
      )}
    </>
  );
}
