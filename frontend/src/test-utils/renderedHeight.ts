import { afterEach, beforeEach } from 'vitest';

/**
 * jsdom does no layout, so every element reports `scrollHeight: 0`. Components
 * that decide anything from measured height — `CollapsibleMarkdown` compares
 * `scrollHeight` against its collapsed height — therefore behave in tests as if
 * all content fits, and never collapse.
 *
 * Call this in a suite that renders collapsible content to make jsdom report a
 * height tall enough to overflow. Restores the original descriptor afterwards.
 *
 * Returns a setter so a single test can override the suite default — e.g. to
 * assert that content which *fits* gets no toggle.
 *
 * @example
 * describe('MyList', () => {
 *   const setHeight = stubRenderedHeight(500); // overflows the 160px default
 *
 *   it('offers a toggle on long content', () => { ... });
 *
 *   it('offers none when content fits', () => {
 *     setHeight(40);
 *     ...
 *   });
 * });
 */
export function stubRenderedHeight(px = 500) {
  const original = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'scrollHeight');

  const setHeight = (height: number) => {
    Object.defineProperty(HTMLElement.prototype, 'scrollHeight', {
      configurable: true,
      get() {
        return height;
      },
    });
  };

  beforeEach(() => {
    setHeight(px);
  });

  afterEach(() => {
    if (original) {
      Object.defineProperty(HTMLElement.prototype, 'scrollHeight', original);
    } else {
      delete (HTMLElement.prototype as unknown as Record<string, unknown>).scrollHeight;
    }
  });

  return setHeight;
}
