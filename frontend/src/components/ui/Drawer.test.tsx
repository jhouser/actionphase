import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Drawer } from './Drawer';

describe('Drawer', () => {
  it('renders children when open', () => {
    render(
      <Drawer open={true} onClose={vi.fn()} title="My Sheet">
        <p>Drawer content</p>
      </Drawer>
    );
    expect(screen.getByText('Drawer content')).toBeInTheDocument();
  });

  it('does not render when closed', () => {
    render(
      <Drawer open={false} onClose={vi.fn()} title="My Sheet">
        <p>Hidden content</p>
      </Drawer>
    );
    expect(screen.queryByText('Hidden content')).not.toBeInTheDocument();
  });

  it('renders title', () => {
    render(
      <Drawer open={true} onClose={vi.fn()} title="Character Sheet">
        <p>Content</p>
      </Drawer>
    );
    expect(screen.getByText('Character Sheet')).toBeInTheDocument();
  });

  it('calls onClose when close button is clicked', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(
      <Drawer open={true} onClose={onClose} title="Sheet">
        <p>Content</p>
      </Drawer>
    );
    await user.click(screen.getByLabelText('Close'));
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('calls onClose when backdrop is clicked', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(
      <Drawer open={true} onClose={onClose}>
        <p>Content</p>
      </Drawer>
    );
    // Target the backdrop by testid: Headless UI renders its own hidden focus
    // sentinels, so the first [aria-hidden] node is not ours.
    await user.click(screen.getByTestId('drawer-backdrop'));
    expect(onClose).toHaveBeenCalled();
  });

  describe('stacking over another overlay', () => {
    it('renders at the default modal tier', () => {
      const { baseElement } = render(
        <Drawer open={true} onClose={vi.fn()}>
          <p>Content</p>
        </Drawer>
      );
      expect(baseElement.querySelector('.z-50')).toBeInTheDocument();
    });

    it('renders at the tier it is given', () => {
      const { baseElement } = render(
        <Drawer open={true} onClose={vi.fn()} zIndexClass="z-60">
          <p>Content</p>
        </Drawer>
      );
      expect(baseElement.querySelector('.z-60')).toBeInTheDocument();
      expect(baseElement.querySelector('.z-50')).not.toBeInTheDocument();
    });

    it('draws a backdrop scrim by default', () => {
      render(
        <Drawer open={true} onClose={vi.fn()}>
          <p>Content</p>
        </Drawer>
      );
      const backdrop = screen.getByTestId('drawer-backdrop');
      expect(backdrop.className).toContain('bg-black/40');
    });

    it('omits the scrim when hideBackdrop is set, so a dimmed page is not double-dimmed', () => {
      render(
        <Drawer open={true} onClose={vi.fn()} hideBackdrop>
          <p>Content</p>
        </Drawer>
      );
      const backdrop = screen.getByTestId('drawer-backdrop');
      expect(backdrop).toBeInTheDocument();
      expect(backdrop.className).not.toContain('bg-black/40');
    });

    it('still closes on outside click with the scrim hidden', async () => {
      const user = userEvent.setup();
      const onClose = vi.fn();
      render(
        <Drawer open={true} onClose={onClose} hideBackdrop>
          <p>Content</p>
        </Drawer>
      );
      await user.click(screen.getByTestId('drawer-backdrop'));
      expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('applies panelClassName so the mobile sheet can be capped when stacked', () => {
      const { baseElement } = render(
        <Drawer open={true} onClose={vi.fn()} panelClassName="max-h-[60vh] lg:max-h-full">
          <p>Content</p>
        </Drawer>
      );
      expect(baseElement.querySelector('.max-h-\\[60vh\\]')).toBeInTheDocument();
    });

    // The drawer's modality — that content underneath is inerted while it is
    // open — is asserted in Drawer.modality.test.tsx rather than here.
    // Headless UI keeps a module-level stack machine that persists across
    // renders within a file, so after the mounts above a fresh dialog no longer
    // inerts anything and the assertion flips regardless of the code. It needs
    // a file to itself to mean anything.
  });
});
