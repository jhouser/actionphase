import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CollapsibleMarkdown } from '../CollapsibleMarkdown';

/**
 * jsdom does no layout, so scrollHeight is always 0 and nothing would ever
 * register as overflowing. Stub it per-test to drive the measurement.
 * (ResizeObserver is already mocked globally in setupTests.ts.)
 */
function setRenderedHeight(px: number) {
  Object.defineProperty(HTMLElement.prototype, 'scrollHeight', {
    configurable: true,
    get() {
      return px;
    },
  });
}

describe('CollapsibleMarkdown', () => {
  beforeEach(() => {
    // Tall enough to overflow the default 160px unless a test says otherwise.
    setRenderedHeight(500);
  });

  describe('markdown integrity while collapsed', () => {
    // The bug this component exists to fix: the old approach sliced the
    // markdown *source* at 200 chars and rendered the fragment, so a cut
    // landing mid-syntax leaked raw markup. Collapsing must never do that.

    it('renders bold formatting rather than literal asterisks when collapsed', async () => {
      const content = 'x'.repeat(195) + ' **critically important**';
      render(<CollapsibleMarkdown content={content} />);

      expect(screen.getByText('critically important').tagName).toBe('STRONG');
      expect(document.body.textContent).not.toContain('**');
    });

    it('keeps a link past the cut point intact', () => {
      const content = 'y'.repeat(190) + ' [see the map](https://example.com/map)';
      render(<CollapsibleMarkdown content={content} />);

      const link = screen.getByRole('link', { name: 'see the map' });
      expect(link).toHaveAttribute('href', 'https://example.com/map');
      expect(document.body.textContent).not.toContain('](');
    });

    it('renders content that follows a fenced code block', () => {
      const content =
        'w'.repeat(180) + '\n\n```\nconst a = 1;\n```\n\nTrailing paragraph after the fence.';
      render(<CollapsibleMarkdown content={content} />);

      // A source-truncating implementation would leave the fence unterminated
      // and swallow everything after it.
      expect(screen.getByText(/Trailing paragraph after the fence/)).toBeInTheDocument();
    });

    it('renders the whole content into the DOM even when visually collapsed', () => {
      const content = 'z'.repeat(300) + ' FINAL_MARKER';
      render(<CollapsibleMarkdown content={content} />);

      expect(document.body.textContent).toContain('FINAL_MARKER');
      expect(document.body.textContent).not.toContain('...');
    });
  });

  describe('overflow detection', () => {
    it('collapses and offers a toggle when content exceeds the collapsed height', () => {
      setRenderedHeight(500);
      render(<CollapsibleMarkdown content="long" collapsedMaxHeight={160} data-testid="body" />);

      expect(screen.getByTestId('body')).toHaveAttribute('data-collapsed', 'true');
      expect(screen.getByRole('button', { name: /show full content/i })).toBeInTheDocument();
    });

    it('shows no toggle when content fits, however long the source string is', () => {
      // Source length is not the signal — a long single line that fits must not
      // get a pointless "Show more".
      setRenderedHeight(80);
      render(
        <CollapsibleMarkdown
          content={'a'.repeat(5000)}
          collapsedMaxHeight={160}
          data-testid="body"
        />
      );

      expect(screen.getByTestId('body')).toHaveAttribute('data-collapsed', 'false');
      expect(screen.queryByRole('button', { name: /show full content/i })).not.toBeInTheDocument();
    });

    it('respects a custom collapsed height', () => {
      setRenderedHeight(300);
      render(<CollapsibleMarkdown content="text" collapsedMaxHeight={400} data-testid="body" />);

      expect(screen.getByTestId('body')).toHaveAttribute('data-collapsed', 'false');
    });
  });

  describe('expansion', () => {
    it('expands and collapses on click when uncontrolled', async () => {
      const user = userEvent.setup();
      render(<CollapsibleMarkdown content="long content" data-testid="body" />);

      const toggle = screen.getByRole('button', { name: /show full content/i });
      expect(toggle).toHaveAttribute('aria-expanded', 'false');

      await user.click(toggle);

      expect(screen.getByTestId('body')).toHaveAttribute('data-collapsed', 'false');
      const collapseToggle = screen.getByRole('button', { name: /show less/i });
      expect(collapseToggle).toHaveAttribute('aria-expanded', 'true');

      await user.click(collapseToggle);
      expect(screen.getByTestId('body')).toHaveAttribute('data-collapsed', 'true');
    });

    it('defers to the controlled prop and reports changes instead of self-managing', async () => {
      const user = userEvent.setup();
      const onExpandedChange = vi.fn();
      render(
        <CollapsibleMarkdown
          content="long content"
          expanded={false}
          onExpandedChange={onExpandedChange}
          data-testid="body"
        />
      );

      await user.click(screen.getByRole('button', { name: /show full content/i }));

      expect(onExpandedChange).toHaveBeenCalledWith(true);
      // Still collapsed: the parent owns the state and hasn't changed it.
      expect(screen.getByTestId('body')).toHaveAttribute('data-collapsed', 'true');
    });

    it('renders expanded when the controlled prop says so', () => {
      render(<CollapsibleMarkdown content="long content" expanded data-testid="body" />);

      expect(screen.getByTestId('body')).toHaveAttribute('data-collapsed', 'false');
      expect(screen.getByRole('button', { name: /show less/i })).toBeInTheDocument();
    });
  });

  describe('presentation', () => {
    it('uses custom labels', () => {
      render(
        <CollapsibleMarkdown content="long" expandLabel="Expand" collapseLabel="Collapse" />
      );

      expect(screen.getByRole('button', { name: 'Expand' })).toBeInTheDocument();
    });

    it('places the toggle before the content when asked', () => {
      const { container } = render(
        <CollapsibleMarkdown content="long" togglePosition="above" data-testid="body" />
      );

      const children = Array.from(container.firstElementChild!.children);
      expect(children[0].tagName).toBe('BUTTON');
    });
  });
});
