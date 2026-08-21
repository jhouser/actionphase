/**
 * Markdown formatting hotkeys for the CommentEditor.
 *
 * Kept as pure functions over (text, selection) so the wrapping/unwrapping
 * rules can be tested without mounting a textarea. The editor layer is
 * responsible for applying the result and restoring the selection.
 */

export type MarkdownFormat = 'bold' | 'italic' | 'link';

/** Marker inserted around the selection for each format. */
const MARKERS: Record<Exclude<MarkdownFormat, 'link'>, string> = {
  bold: '**',
  italic: '*',
};

/** Placeholder used when a hotkey fires with nothing selected. */
const PLACEHOLDERS: Record<MarkdownFormat, string> = {
  bold: 'bold text',
  italic: 'italic text',
  link: 'link text',
};

export interface EditorSelection {
  value: string;
  selectionStart: number;
  selectionEnd: number;
}

export interface FormatResult {
  value: string;
  /** Selection to restore — spans the text the user should now be editing. */
  selectionStart: number;
  selectionEnd: number;
}

/**
 * Maps a keyboard event to a format, or null when the combo isn't a
 * formatting hotkey. Accepts Ctrl (Windows/Linux) and Meta (macOS ⌘), and
 * ignores combos that also carry Alt so we don't shadow OS/IME shortcuts.
 */
export function formatForKey(e: {
  key: string;
  ctrlKey: boolean;
  metaKey: boolean;
  altKey: boolean;
}): MarkdownFormat | null {
  if (!(e.ctrlKey || e.metaKey) || e.altKey) return null;

  switch (e.key.toLowerCase()) {
    case 'b':
      return 'bold';
    case 'i':
      return 'italic';
    case 'k':
      return 'link';
    default:
      return null;
  }
}

/**
 * Italic uses a single `*`, which is also the first character of bold's `**`.
 * Without this guard, Ctrl+I on `**bold**` would read the outer `*` pair as
 * italic markers and unwrap bold into `*bold*`.
 */
function isBoldAt(text: string, start: number, end: number): boolean {
  return text.startsWith('**', start - 2) && text.startsWith('**', end);
}

/**
 * True when the selection is already wrapped in the given marker, meaning the
 * hotkey should remove the formatting rather than nest another layer.
 */
function isWrapped(text: string, start: number, end: number, marker: string): boolean {
  const before = text.slice(Math.max(0, start - marker.length), start);
  const after = text.slice(end, end + marker.length);
  if (before !== marker || after !== marker) return false;

  // A lone `*` on each side of an already-bold selection belongs to the bold
  // markers, not to an italic wrapper.
  if (marker === MARKERS.italic && isBoldAt(text, start, end)) return false;

  return true;
}

/**
 * Applies (or removes) a markdown format around the current selection.
 *
 * Behavior mirrors Discord/Slack:
 * - Text selected → wrap it, leaving the text selected so it can be retyped.
 * - Nothing selected → insert markers around a placeholder and select the
 *   placeholder, so typing replaces it.
 * - Already wrapped → unwrap, toggling the format off.
 */
export function applyMarkdownFormat(
  format: MarkdownFormat,
  { value, selectionStart, selectionEnd }: EditorSelection
): FormatResult {
  if (format === 'link') {
    return applyLink({ value, selectionStart, selectionEnd });
  }

  const marker = MARKERS[format];
  const selected = value.slice(selectionStart, selectionEnd);

  // Toggle off: selection sits inside an existing pair of markers.
  if (isWrapped(value, selectionStart, selectionEnd, marker)) {
    const before = value.slice(0, selectionStart - marker.length);
    const after = value.slice(selectionEnd + marker.length);
    return {
      value: before + selected + after,
      selectionStart: selectionStart - marker.length,
      selectionEnd: selectionEnd - marker.length,
    };
  }

  // Toggle off: the markers are part of the selection itself (e.g. the user
  // selected `**bold**` rather than `bold`).
  if (
    selected.length > marker.length * 2 &&
    selected.startsWith(marker) &&
    selected.endsWith(marker) &&
    !(marker === MARKERS.italic && selected.startsWith('**') && selected.endsWith('**'))
  ) {
    const inner = selected.slice(marker.length, selected.length - marker.length);
    return {
      value: value.slice(0, selectionStart) + inner + value.slice(selectionEnd),
      selectionStart,
      selectionEnd: selectionStart + inner.length,
    };
  }

  const body = selected || PLACEHOLDERS[format];
  const wrapped = marker + body + marker;

  return {
    value: value.slice(0, selectionStart) + wrapped + value.slice(selectionEnd),
    selectionStart: selectionStart + marker.length,
    selectionEnd: selectionStart + marker.length + body.length,
  };
}

/**
 * Ctrl+K builds `[text](url)`. Selected text becomes the label and the cursor
 * lands in the empty URL slot — the part the user still has to supply.
 */
function applyLink({ value, selectionStart, selectionEnd }: EditorSelection): FormatResult {
  const selected = value.slice(selectionStart, selectionEnd);
  const label = selected || PLACEHOLDERS.link;
  const inserted = `[${label}](url)`;

  const before = value.slice(0, selectionStart);
  const after = value.slice(selectionEnd);

  // With text selected the label is settled, so select `url` for replacement.
  // With nothing selected the label is a placeholder, so select that instead.
  const urlStart = selectionStart + label.length + 3; // "[" + label + "](
  if (selected) {
    return {
      value: before + inserted + after,
      selectionStart: urlStart,
      selectionEnd: urlStart + 3, // "url"
    };
  }

  return {
    value: before + inserted + after,
    selectionStart: selectionStart + 1,
    selectionEnd: selectionStart + 1 + label.length,
  };
}
