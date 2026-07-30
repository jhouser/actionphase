/**
 * Copy text to the clipboard, falling back to a hidden textarea + execCommand
 * for insecure contexts (http, older browsers) where the async Clipboard API
 * is unavailable. Mirrors the inline pattern used in CommentWithParentCard.
 *
 * @returns true on success; throws if both strategies fail.
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(text);
    return true;
  }

  const textArea = document.createElement('textarea');
  textArea.value = text;
  textArea.style.position = 'fixed';
  textArea.style.left = '-999999px';
  document.body.appendChild(textArea);
  textArea.focus();
  textArea.select();
  try {
    const ok = document.execCommand('copy');
    textArea.remove();
    if (!ok) throw new Error('execCommand copy returned false');
    return true;
  } catch (err) {
    textArea.remove();
    throw err;
  }
}
