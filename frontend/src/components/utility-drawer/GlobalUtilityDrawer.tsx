import { lazy, Suspense, useState } from 'react';
import { UtilityDrawer } from './UtilityDrawer';
import { Modal } from '../Modal';
import { Spinner } from '../ui';
import { useUtilityDrawer } from '../../contexts/UtilityDrawerContext';
import { LAYERS } from '../../config/layers';

// The sheet pulls in the whole character-sheet module tree; keep it out of the
// initial bundle since most page loads never open it.
const CharacterSheet = lazy(() =>
  import('../CharacterSheet').then((m) => ({ default: m.CharacterSheet }))
);

// Same reasoning as the sheet above: the handout view drags in the markdown
// renderer and comment editor, which most page loads never need.
const HandoutView = lazy(() =>
  import('../HandoutView').then((m) => ({ default: m.HandoutView }))
);

/**
 * Renders the Utility Drawer and the character-sheet modal it launches at the
 * app root, so both are available on every page. Mounted once, inside
 * UtilityDrawerProvider.
 */
export function GlobalUtilityDrawer() {
  const {
    isOpen,
    closeDrawer,
    utilityContext,
    openSheet,
    closeSheet,
    openHandoutView,
    closeHandout,
  } = useUtilityDrawer();

  // Whether an editor inside the sheet holds text its own Save has not committed. The
  // sheet reports this up because the backdrop is ours, not its — see dismissOnBackdrop
  // below. CharacterSheet reports false on unmount, so this cannot stay stuck on.
  const [sheetIsDirty, setSheetIsDirty] = useState(false);

  return (
    <>
      <UtilityDrawer open={isOpen} onClose={closeDrawer} ctx={utilityContext} />

      {openSheet && (
        // dismissOnBackdrop: backdrop dismiss stays on for a sheet with nothing to lose —
        // clicking away is how most people close a modal. It is withdrawn only while an
        // editor holds uncommitted text, where a stray click would silently discard it.
        <Modal
          isOpen
          onClose={closeSheet}
          title=""
          zIndexClass={LAYERS.drawerChild}
          dismissOnBackdrop={!sheetIsDirty}
        >
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
              sheetConfig={openSheet.options.sheetConfig}
              onDirtyChange={setSheetIsDirty}
            />
          </Suspense>
        </Modal>
      )}

      {openHandoutView && (
        // Backdrop dismiss stays on unconditionally: the modal is read-only, so
        // a stray click has no unsaved text to discard.
        <Modal
          isOpen
          onClose={closeHandout}
          // Naming the handout here gives it the same title bar and X as every
          // other modal, instead of the Handouts tab's back-link.
          title={openHandoutView.handout.title}
          zIndexClass={LAYERS.drawerChild}
          // Wider than the default 4xl: this is long-form prose you read rather
          // than a form you fill in, and the extra width keeps a handout from
          // becoming a narrow column of text. A max-width, so narrower viewports
          // are unaffected.
          size="7xl"
        >
          <Suspense
            fallback={
              <div className="flex justify-center py-12">
                <Spinner size="lg" />
              </div>
            }
          >
            <HandoutView
              gameId={openHandoutView.handout.game_id}
              handout={openHandoutView.handout}
              // Read-only regardless of role, so the reader's wording is the
              // right wording for everyone here.
              isGM={false}
              onClose={closeHandout}
              showHeader={false}
              readOnly
            />
          </Suspense>
        </Modal>
      )}
    </>
  );
}
