/**
 * Explains why tab navigation is disabled while an editor is open.
 *
 * Switching tabs unmounts the open editor and destroys edits that live only in its
 * local state. Rather than silently losing them — or preserving them somewhere the GM
 * cannot find again, which on a list of ten abilities means hunting for the one card
 * still mid-edit — navigation is held until the editor is explicitly saved or cancelled.
 *
 * A disabled control needs to say why, or it just reads as broken.
 */
export function EditorLockNotice({ className }: { className?: string }) {
  return (
    <span
      className={`text-xs text-content-tertiary ${className ?? ''}`}
      data-testid="editor-lock-notice"
    >
      Finish or cancel your edit to switch tabs
    </span>
  );
}
