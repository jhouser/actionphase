import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import { Toggle } from './Toggle';

describe('Toggle Component', () => {
  describe('Accessibility', () => {
    it('renders a switch with the correct checked state', () => {
      render(<Toggle checked onChange={() => {}} aria-label="Setting" />);
      const sw = screen.getByRole('switch');
      expect(sw).toHaveAttribute('aria-checked', 'true');
    });

    it('reflects the unchecked state', () => {
      render(<Toggle checked={false} onChange={() => {}} aria-label="Setting" />);
      expect(screen.getByRole('switch')).toHaveAttribute('aria-checked', 'false');
    });
  });

  describe('Interaction', () => {
    it('calls onChange with the negated value when clicked', async () => {
      const onChange = vi.fn();
      render(<Toggle checked={false} onChange={onChange} aria-label="Setting" />);

      await userEvent.click(screen.getByRole('switch'));
      expect(onChange).toHaveBeenCalledWith(true);
    });

    it('negates from the on state', async () => {
      const onChange = vi.fn();
      render(<Toggle checked onChange={onChange} aria-label="Setting" />);

      await userEvent.click(screen.getByRole('switch'));
      expect(onChange).toHaveBeenCalledWith(false);
    });

    it('does not fire onChange when disabled', async () => {
      const onChange = vi.fn();
      render(<Toggle checked={false} onChange={onChange} disabled aria-label="Setting" />);

      await userEvent.click(screen.getByRole('switch'));
      expect(onChange).not.toHaveBeenCalled();
    });
  });

  describe('Off-state visibility (regression)', () => {
    // The original bug: off-state used unassigned tokens (bg-bg-secondary /
    // bg-border-primary) that render invisible in light mode. The fix uses a
    // defined surface fill plus a visible border on the track.
    it('renders a visible track fill and border when off', () => {
      const { container } = render(
        <Toggle checked={false} onChange={() => {}} aria-label="Setting" />
      );
      const track = container.querySelector('span.rounded-full');
      expect(track).not.toBeNull();
      expect(track!.className).toContain('bg-surface-sunken');
      expect(track!.className).toContain('border-theme-strong');
      expect(track!.className).not.toContain('bg-bg-secondary');
    });
  });

  describe('Label and description', () => {
    it('renders label and description as a row', () => {
      render(
        <Toggle
          checked={false}
          onChange={() => {}}
          label="Private Messages"
          description="Get a DM for new messages"
        />
      );
      expect(screen.getByText('Private Messages')).toBeInTheDocument();
      expect(screen.getByText('Get a DM for new messages')).toBeInTheDocument();
    });

    it('forwards data attributes to the switch element', () => {
      render(
        <Toggle
          checked={false}
          onChange={() => {}}
          data-testid="my-toggle"
          aria-label="Setting"
        />
      );
      expect(screen.getByTestId('my-toggle')).toHaveAttribute('role', 'switch');
    });
  });
});
