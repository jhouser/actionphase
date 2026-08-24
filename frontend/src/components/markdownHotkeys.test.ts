import { describe, it, expect } from 'vitest';
import { applyMarkdownFormat, formatForKey } from './markdownHotkeys';

/**
 * Helper that expresses a selection with `|` markers, so cases read the way
 * they look in the editor. `a|bc|d` selects "bc".
 */
function sel(marked: string) {
  const first = marked.indexOf('|');
  const second = marked.indexOf('|', first + 1);
  return {
    value: marked.replace(/\|/g, ''),
    selectionStart: first,
    selectionEnd: second - 1,
  };
}

/** Renders a result back into the `|`-marked form for comparison. */
function show({
  value,
  selectionStart,
  selectionEnd,
}: {
  value: string;
  selectionStart: number;
  selectionEnd: number;
}) {
  return (
    value.slice(0, selectionStart) +
    '|' +
    value.slice(selectionStart, selectionEnd) +
    '|' +
    value.slice(selectionEnd)
  );
}

describe('formatForKey', () => {
  const base = { ctrlKey: true, metaKey: false, altKey: false };

  it('maps ctrl+b, ctrl+i and ctrl+k to formats', () => {
    expect(formatForKey({ ...base, key: 'b' })).toBe('bold');
    expect(formatForKey({ ...base, key: 'i' })).toBe('italic');
    expect(formatForKey({ ...base, key: 'k' })).toBe('link');
  });

  it('accepts the macOS command key', () => {
    expect(formatForKey({ key: 'b', ctrlKey: false, metaKey: true, altKey: false })).toBe('bold');
  });

  it('matches regardless of shift-induced capitalization', () => {
    expect(formatForKey({ ...base, key: 'B' })).toBe('bold');
  });

  it('ignores plain keypresses so typing "b" is unaffected', () => {
    expect(formatForKey({ key: 'b', ctrlKey: false, metaKey: false, altKey: false })).toBeNull();
  });

  it('ignores unmapped modifier combos', () => {
    expect(formatForKey({ ...base, key: 'u' })).toBeNull();
    expect(formatForKey({ ...base, key: 'a' })).toBeNull();
  });

  it('ignores combos carrying alt, which belong to the OS or IME', () => {
    expect(formatForKey({ ...base, key: 'b', altKey: true })).toBeNull();
  });
});

describe('applyMarkdownFormat — bold', () => {
  it('wraps the selection and keeps the text selected', () => {
    expect(show(applyMarkdownFormat('bold', sel('hello |world|')))).toBe('hello **|world|**');
  });

  it('inserts a selected placeholder when nothing is selected', () => {
    const result = applyMarkdownFormat('bold', sel('hello ||'));
    expect(result.value).toBe('hello **bold text**');
    // The placeholder is selected so the next keystroke replaces it
    expect(show(result)).toBe('hello **|bold text|**');
  });

  it('unwraps when the selection is already bold', () => {
    expect(show(applyMarkdownFormat('bold', sel('**|world|**')))).toBe('|world|');
  });

  it('unwraps when the markers are inside the selection', () => {
    expect(show(applyMarkdownFormat('bold', sel('|**world**|')))).toBe('|world|');
  });
});

describe('applyMarkdownFormat — italic', () => {
  it('wraps the selection in single asterisks', () => {
    expect(show(applyMarkdownFormat('italic', sel('hello |world|')))).toBe('hello *|world|*');
  });

  it('unwraps when the selection is already italic', () => {
    expect(show(applyMarkdownFormat('italic', sel('*|world|*')))).toBe('|world|');
  });

  it('nests inside bold rather than stealing bold’s asterisks', () => {
    // The `*` adjacent to the selection belongs to `**`, so this must add
    // italic, not unwrap the bold into `*world*`.
    expect(show(applyMarkdownFormat('italic', sel('**|world|**')))).toBe('***|world|***');
  });

  it('does not unwrap bold when the bold markers are inside the selection', () => {
    expect(show(applyMarkdownFormat('italic', sel('|**world**|')))).toBe('*|**world**|*');
  });
});

describe('applyMarkdownFormat — link', () => {
  it('uses the selection as the label and selects the url slot', () => {
    const result = applyMarkdownFormat('link', sel('see |the docs|'));
    expect(result.value).toBe('see [the docs](url)');
    expect(show(result)).toBe('see [the docs](|url|)');
  });

  it('selects the label placeholder when nothing is selected', () => {
    const result = applyMarkdownFormat('link', sel('see ||'));
    expect(result.value).toBe('see [link text](url)');
    expect(show(result)).toBe('see [|link text|](url)');
  });
});
