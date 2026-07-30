import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Drawer } from './Drawer';

/**
 * The drawer's modality, pinned in a file of its own.
 *
 * Headless UI's Dialog has no non-modal mode, so the drawer always traps focus
 * and aria-hides whatever is underneath — including when it stacks over a
 * thread view. That is an accepted trade-off (the drawer is reference material
 * you open, read, and close), but it is worth asserting: an earlier attempt
 * passed `modal={false}` to Dialog to make the thread stay live, and because
 * `modal` is not part of Dialog's API it was spread onto the DOM, dropped by
 * React, and silently did nothing. Nothing failed, so the gap went unnoticed.
 *
 * ONE Drawer may be rendered in this file. Headless UI keeps a module-level
 * stack machine and portal root that persist across renders within a file;
 * after a second mount a fresh dialog stops winning the inerting and this
 * assertion inverts on its own, with no code change. Adding cases here makes
 * the file lie. Put non-modality tests in Drawer.test.tsx.
 */
describe('Drawer modality', () => {
  it('inerts the content underneath while open', () => {
    render(
      <div>
        <button type="button">Thread action</button>
        <Drawer open={true} onClose={vi.fn()} zIndexClass="z-60" hideBackdrop>
          <p>Drawer content</p>
        </Drawer>
      </div>
    );

    const outside = screen.getByText('Thread action');
    expect(outside.closest('[aria-hidden="true"]')).not.toBeNull();
  });
});
