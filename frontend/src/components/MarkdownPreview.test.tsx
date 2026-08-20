import { describe, it, expect, vi, afterEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MarkdownPreview } from './MarkdownPreview';
import { TEXT_COLORS } from './textColors';

describe('MarkdownPreview', () => {
  describe('Basic Markdown Rendering', () => {
    it('renders bold text correctly', () => {
      render(<MarkdownPreview content="This is **bold** text" />);
      const boldElement = screen.getByText('bold');
      expect(boldElement.tagName).toBe('STRONG');
    });

    it('renders italic text correctly', () => {
      render(<MarkdownPreview content="This is *italic* text" />);
      const italicElement = screen.getByText('italic');
      expect(italicElement.tagName).toBe('EM');
    });

    it('renders headers correctly', () => {
      const { rerender } = render(<MarkdownPreview content="# Heading 1" />);
      expect(screen.getByText('Heading 1').tagName).toBe('H1');

      rerender(<MarkdownPreview content="## Heading 2" />);
      expect(screen.getByText('Heading 2').tagName).toBe('H2');

      rerender(<MarkdownPreview content="### Heading 3" />);
      expect(screen.getByText('Heading 3').tagName).toBe('H3');
    });

    it('renders unordered lists correctly', () => {
      const { container } = render(<MarkdownPreview content="- Item 1\n- Item 2\n- Item 3" />);

      // Check that a list is rendered
      const ul = container.querySelector('ul');
      expect(ul).toBeInTheDocument();

      // Check that list items are present
      expect(container.textContent).toContain('Item 1');
      expect(container.textContent).toContain('Item 2');
      expect(container.textContent).toContain('Item 3');
    });

    it('renders ordered lists correctly', () => {
      const { container } = render(<MarkdownPreview content="1. First\n2. Second\n3. Third" />);

      // Check that a list is rendered
      const ol = container.querySelector('ol');
      expect(ol).toBeInTheDocument();

      // Check that list items are present
      expect(container.textContent).toContain('First');
      expect(container.textContent).toContain('Second');
      expect(container.textContent).toContain('Third');
    });

    it('renders inline code correctly', () => {
      render(<MarkdownPreview content="Use `console.log()` for debugging" />);
      const codeElement = screen.getByText('console.log()');
      expect(codeElement.tagName).toBe('CODE');
    });

    it('renders blockquotes correctly', () => {
      render(<MarkdownPreview content="> This is a quote" />);
      const blockquote = screen.getByText('This is a quote').closest('blockquote');
      expect(blockquote).toBeInTheDocument();
      expect(blockquote).toHaveClass('border-l-4');
    });

    it('preserves hard line breaks inside blockquotes', () => {
      // Regression: the blockquote renderer emitted raw source text instead of
      // parsing its child tokens, so every line collapsed onto one line.
      const { container } = render(
        <MarkdownPreview content={'> RANDOM BULLSHIT  \n12  \n313  \n2312  \n312'} />
      );
      const blockquote = container.querySelector('blockquote');
      expect(blockquote).toBeInTheDocument();
      expect(blockquote?.querySelectorAll('br')).toHaveLength(4);
      expect(blockquote?.textContent).not.toContain('BULLSHIT 12');
    });

    it('renders markdown syntax inside blockquotes', () => {
      const { container } = render(
        <MarkdownPreview content={'> This is **bold** and *italic*'} />
      );
      const blockquote = container.querySelector('blockquote');
      expect(blockquote?.querySelector('strong')?.textContent).toBe('bold');
      expect(blockquote?.querySelector('em')?.textContent).toBe('italic');
    });

    it('renders multiple paragraphs inside a blockquote', () => {
      const { container } = render(
        <MarkdownPreview content={'> First para\n>\n> Second para'} />
      );
      const blockquote = container.querySelector('blockquote');
      expect(blockquote?.querySelectorAll('p')).toHaveLength(2);
    });

    it('renders horizontal rules correctly', () => {
      const { container } = render(<MarkdownPreview content="---" />);
      const hr = container.querySelector('hr');
      expect(hr).toBeInTheDocument();
      if (hr) {
        expect(hr).toHaveClass('border-t-2');
      }
    });
  });

  describe('Link Handling', () => {
    it('renders links with target="_blank" and security attributes', () => {
      render(<MarkdownPreview content="[Click here](https://example.com)" />);
      const link = screen.getByRole('link', { name: 'Click here' });
      expect(link).toHaveAttribute('href', 'https://example.com');
      expect(link).toHaveAttribute('target', '_blank');
      expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    });

    it('applies link styling', () => {
      render(<MarkdownPreview content="[Link](https://example.com)" />);
      const link = screen.getByRole('link', { name: 'Link' });
      expect(link).toHaveClass('text-interactive-primary');
      expect(link).toHaveClass('underline');
    });
  });

  describe('Code Block Rendering', () => {
    it('renders fenced code blocks with a language as plain code', () => {
      // Syntax highlighting was removed to drop react-syntax-highlighter (~250kB
      // gzip). A language-tagged block now renders as a plain <pre><code>.
      const code = '```javascript\nconst x = 42;\n```';
      const { container } = render(<MarkdownPreview content={code} />);

      const codeBlock = container.querySelector('pre > code');
      expect(codeBlock).toBeInTheDocument();
      expect(codeBlock?.textContent).toContain('const x = 42;');
    });

    it('renders code blocks without language as plain code', () => {
      const code = '```\nplain text\n```';
      render(<MarkdownPreview content={code} />);

      // Should render as code but without syntax highlighting
      expect(screen.getByText('plain text')).toBeInTheDocument();
    });
  });

  describe('Code Block Copy Button', () => {
    /**
     * jsdom has no clipboard, so each test installs its own. Returns the spy so
     * a test can assert on what was written.
     */
    function stubClipboard(impl: () => Promise<void> = () => Promise.resolve()) {
      const writeText = vi.fn(impl);
      Object.defineProperty(navigator, 'clipboard', {
        value: { writeText },
        configurable: true,
        writable: true,
      });
      return writeText;
    }

    afterEach(() => {
      vi.useRealTimers();
      // @ts-expect-error - removing the stub installed above
      delete navigator.clipboard;
    });

    it('renders a copy button on a fenced code block', () => {
      render(<MarkdownPreview content={'```\nconst x = 42;\n```'} />);
      expect(screen.getByRole('button', { name: 'Copy code' })).toBeInTheDocument();
    });

    it('does not render a copy button for inline code', () => {
      // Only fenced blocks get one; a button beside every `word` would be noise.
      render(<MarkdownPreview content="Use the `useState` hook" />);
      expect(screen.queryByRole('button', { name: 'Copy code' })).not.toBeInTheDocument();
    });

    it('copies the code block contents to the clipboard', async () => {
      const writeText = stubClipboard();
      render(<MarkdownPreview content={'```js\nconst x = 42;\n```'} />);

      fireEvent.click(screen.getByRole('button', { name: 'Copy code' }));

      await waitFor(() => expect(writeText).toHaveBeenCalledWith('const x = 42;'));
    });

    it('copies markup-heavy code verbatim', async () => {
      // Pins the exact string handed to the clipboard for content full of
      // characters the renderer escapes. Note this does not prove the handler
      // reads the raw source rather than the rendered <code>: jsdom's
      // textContent decodes entities, so both routes produce this same string.
      // The guard against reading the DOM is 'ignores decoration added to the
      // rendered block' below.
      const writeText = stubClipboard();
      render(<MarkdownPreview content={'```\n<div class="a">& \'quoted\'</div>\n```'} />);

      fireEvent.click(screen.getByRole('button', { name: 'Copy code' }));

      await waitFor(() =>
        expect(writeText).toHaveBeenCalledWith('<div class="a">& \'quoted\'</div>')
      );
    });

    it('preserves multi-line code exactly', async () => {
      const writeText = stubClipboard();
      render(<MarkdownPreview content={'```\nline one\nline two\n\nline four\n```'} />);

      fireEvent.click(screen.getByRole('button', { name: 'Copy code' }));

      await waitFor(() =>
        expect(writeText).toHaveBeenCalledWith('line one\nline two\n\nline four')
      );
    });

    it('confirms the copy, then returns to the idle label', async () => {
      vi.useFakeTimers();
      stubClipboard();
      render(<MarkdownPreview content={'```\nhello\n```'} />);

      fireEvent.click(screen.getByRole('button', { name: 'Copy code' }));

      // The promise callback is a microtask; flush it without advancing timers.
      await vi.advanceTimersByTimeAsync(0);
      expect(screen.getByRole('button', { name: 'Copied' })).toBeInTheDocument();

      await vi.advanceTimersByTimeAsync(2000);
      expect(screen.getByRole('button', { name: 'Copy code' })).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Copied' })).not.toBeInTheDocument();
    });

    it('reports a rejected clipboard write instead of claiming success', async () => {
      // Clipboard permission can be denied; silently showing a checkmark would
      // send the user off to paste nothing.
      stubClipboard(() => Promise.reject(new Error('denied')));
      render(<MarkdownPreview content={'```\nhello\n```'} />);

      fireEvent.click(screen.getByRole('button', { name: 'Copy code' }));

      expect(await screen.findByRole('button', { name: 'Copy failed' })).toBeInTheDocument();
    });

    it('survives a missing clipboard API', async () => {
      // Insecure origins leave navigator.clipboard undefined.
      render(<MarkdownPreview content={'```\nhello\n```'} />);

      fireEvent.click(screen.getByRole('button', { name: 'Copy code' }));

      expect(await screen.findByRole('button', { name: 'Copy failed' })).toBeInTheDocument();
    });

    it('stays visible without hover on small screens', () => {
      // Touch devices have no hover state, so a hover-only reveal leaves the
      // button undiscoverable short of an accidental tap in the corner. It is
      // unconditionally visible below `sm`, and hover only sharpens it above.
      render(<MarkdownPreview content={'```\nhello\n```'} />);
      const btn = screen.getByRole('button', { name: 'Copy code' });

      expect(btn.className).toContain('opacity-100');
      expect(btn.className).toContain('sm:group-hover:opacity-100');
      // The bare `opacity-0` of a hover-only affordance must not come back.
      expect(btn.className).not.toMatch(/(^|\s)opacity-0(\s|$)/);
    });

    it('gives the button a 44px tap target on small screens', () => {
      render(<MarkdownPreview content={'```\nhello\n```'} />);
      const btn = screen.getByRole('button', { name: 'Copy code' });

      expect(btn.className).toContain('min-w-[44px]');
      expect(btn.className).toContain('min-h-[44px]');
      // ...without inflating the icon on pointer devices.
      expect(btn.className).toContain('sm:min-w-0');
    });

    it('ignores decoration added to the rendered block', async () => {
      // The real reason the handler carries its own copy of the source: a
      // mention inside a code block renders as a <mark> wrapper, and copying the
      // block's DOM text would be at the mercy of whatever decorated it. Reading
      // data-copy-code keeps the clipboard equal to what the author typed.
      const writeText = stubClipboard();
      const { container } = render(<MarkdownPreview content={'```\ngreet @Alice\n```'} />);

      // Simulate post-render decoration of the code element.
      const code = container.querySelector('code')!;
      code.innerHTML = code.innerHTML.replace('@Alice', '<mark>@Alice</mark> (Player One)');

      fireEvent.click(screen.getByRole('button', { name: 'Copy code' }));

      await waitFor(() => expect(writeText).toHaveBeenCalledWith('greet @Alice'));
    });

    it('renders the clipboard icon', () => {
      // The icon is injected after sanitization, so a stripped or un-hydrated
      // icon leaves a button that is still findable by aria-label but visually
      // empty. Assert on the glyph itself, not just the accessible name.
      const { container } = render(<MarkdownPreview content={'```\nhello\n```'} />);
      const btn = screen.getByRole('button', { name: 'Copy code' });

      expect(btn.querySelector('svg')).toBeInTheDocument();
      expect(container.querySelectorAll('button[data-copy-code] svg')).toHaveLength(1);
    });

    it('positions the button against an element that survives sanitization', () => {
      // The button is absolutely/stickily placed, so it needs a positioned
      // ancestor that DOMPurify keeps. A wrapper <div> would require
      // allowlisting 'div' for all user markdown; if the positioning context is
      // ever moved back to a stripped element the button escapes to the page
      // corner while every aria-label query still passes.
      render(<MarkdownPreview content={'```\nhello\n```'} />);
      const btn = screen.getByRole('button', { name: 'Copy code' });

      const positioned = btn.closest('.relative');
      expect(positioned).not.toBeNull();
      expect(positioned!.tagName).toBe('PRE');
    });

    it('tracks each code block independently', async () => {
      const writeText = stubClipboard();
      render(<MarkdownPreview content={'```\nfirst\n```\n\ntext\n\n```\nsecond\n```'} />);

      const buttons = screen.getAllByRole('button', { name: 'Copy code' });
      expect(buttons).toHaveLength(2);

      fireEvent.click(buttons[1]);

      await waitFor(() => expect(writeText).toHaveBeenCalledWith('second'));
      // The untouched block keeps its idle label.
      expect(buttons[0]).toHaveAttribute('aria-label', 'Copy code');
    });
  });

  describe('Character Mention Handling', () => {
    const mentionedCharacters = [
      { id: 1, name: 'Alice' },
      { id: 2, name: 'Bob Smith' },
    ];

    it('highlights character mentions with @syntax', () => {
      render(
        <MarkdownPreview
          content="Hey @Alice, can you help @Bob Smith with this?"
          mentionedCharacters={mentionedCharacters}
        />
      );

      // Check that mentions are highlighted
      const aliceMention = screen.getByText('@Alice');
      const bobMention = screen.getByText('@Bob Smith');

      expect(aliceMention.tagName).toBe('MARK');
      expect(bobMention.tagName).toBe('MARK');
      expect(aliceMention).toHaveAttribute('data-mention-id', '1');
      expect(bobMention).toHaveAttribute('data-mention-id', '2');
    });

    it('applies mention styling', () => {
      render(
        <MarkdownPreview
          content="@Alice mentioned"
          mentionedCharacters={mentionedCharacters}
        />
      );

      const mention = screen.getByText('@Alice');
      expect(mention).toHaveClass('bg-interactive-primary-subtle');
      expect(mention).toHaveClass('text-interactive-primary');
    });

    it('handles multiple mentions of the same character', () => {
      render(
        <MarkdownPreview
          content="@Alice and @Alice are the same person"
          mentionedCharacters={mentionedCharacters}
        />
      );

      const mentions = screen.getAllByText('@Alice');
      expect(mentions).toHaveLength(2);
      mentions.forEach((mention) => {
        expect(mention.tagName).toBe('MARK');
        expect(mention).toHaveAttribute('data-mention-id', '1');
      });
    });

    it('handles mentions with special characters in names', () => {
      const specialCharacters = [{ id: 3, name: "O'Brien" }];
      render(
        <MarkdownPreview
          content="@O'Brien is mentioned"
          mentionedCharacters={specialCharacters}
        />
      );

      const mention = screen.getByText("@O'Brien");
      expect(mention.tagName).toBe('MARK');
    });

    it('prioritizes longer character names to avoid partial matches', () => {
      const characters = [
        { id: 1, name: 'Bob' },
        { id: 2, name: 'Bob Smith' },
      ];

      render(
        <MarkdownPreview
          content="@Bob Smith is here"
          mentionedCharacters={characters}
        />
      );

      // Should match "Bob Smith" as one mention, not "Bob" + " Smith"
      const mention = screen.getByText('@Bob Smith');
      expect(mention).toHaveAttribute('data-mention-id', '2');
    });

    it('does not highlight mentions inside inline code', () => {
      render(
        <MarkdownPreview
          content="Use `@Alice` as the username"
          mentionedCharacters={mentionedCharacters}
        />
      );

      // @Alice inside backticks should NOT be highlighted as a mention
      const codeElement = screen.getByText('@Alice');
      expect(codeElement.tagName).toBe('CODE');
      expect(codeElement.tagName).not.toBe('MARK');
    });

    it('does not re-render the content div when tooltip becomes visible', () => {
      // Regression: MarkdownContent must be React.memo'd so hover setState doesn't replace
      // the dangerouslySetInnerHTML DOM — Chrome re-fires mouseover on replacement, causing
      // an infinite loop that traps the tooltip open.
      render(
        <MarkdownPreview
          content="@Alice mentioned"
          mentionedCharacters={mentionedCharacters}
        />
      );
      const mark = screen.getByText('@Alice');
      expect(mark.tagName).toBe('MARK');

      // Trigger hover to show tooltip (fires mouseover on the container)
      fireEvent.mouseOver(mark);

      // The mark must still be the same DOM node — not detached and replaced
      expect(document.contains(mark)).toBe(true);
      expect(mark.getAttribute('data-mention-id')).toBe('1');
    });

    it('handles content without mentions', () => {
      render(
        <MarkdownPreview
          content="Just regular text with no mentions"
          mentionedCharacters={mentionedCharacters}
        />
      );

      expect(screen.getByText(/Just regular text/)).toBeInTheDocument();
    });

    it('handles empty mentionedCharacters array', () => {
      render(
        <MarkdownPreview
          content="@Alice should not be highlighted"
          mentionedCharacters={[]}
        />
      );

      // Should render as plain text, not a mention
      expect(screen.getByText(/@Alice/)).toBeInTheDocument();
      expect(screen.queryByRole('mark')).not.toBeInTheDocument();
    });

    it('escapes mention markup inside fenced code blocks', () => {
      const content = '```\n@Alice in code block\n```';
      const { container } = render(
        <MarkdownPreview
          content={content}
          mentionedCharacters={mentionedCharacters}
        />
      );

      // Mention markup should be escaped (safe) inside code blocks
      // The <mark> tag gets inserted but is HTML-escaped, rendering as literal text
      const codeBlock = container.querySelector('code');
      expect(codeBlock).toBeInTheDocument();

      // The escaped markup should be visible as text, not executed as HTML
      // This is safe - XSS is prevented
      expect(codeBlock?.textContent).toContain('@Alice');

      // Should not have any actual MARK elements (they're escaped)
      const marks = container.querySelectorAll('mark');
      expect(marks.length).toBe(0);
    });
  });

  describe('XSS Protection', () => {
    it('strips user-authored div tags', () => {
      // Regression guard for the copy-button work: positioning it needed a
      // containing block, and allowlisting <div> to get one would also let any
      // markdown author emit <div class="...">. With 'class' already allowed
      // that is enough to lay a full-viewport overlay over another user's post,
      // so the positioning lives on the <pre> instead.
      const { container } = render(
        <MarkdownPreview content={'<div class="fixed inset-0 z-50">overlay</div>'} />
      );

      expect(container.querySelector('div.fixed')).toBeNull();
      // Unwrapped, not dropped: the text stays, the tag does not.
      expect(container.textContent).toContain('overlay');
    });

    it('strips user-authored svg tags', () => {
      // Same reasoning as <div>: the copy icon is injected post-sanitization
      // precisely so that <svg> need not be allowlisted for all markdown.
      const { container } = render(
        <MarkdownPreview content={'<svg width="10"><path d="M0 0"/></svg>'} />
      );

      // The only svg on the page belongs to a copy button, and this content has
      // no code block.
      expect(container.querySelector('svg')).toBeNull();
    });

    it('prevents script injection via content', () => {
      const maliciousContent = '<script>alert("XSS")</script>Hello';
      const { container } = render(<MarkdownPreview content={maliciousContent} />);

      // Script tag should be sanitized (removed by rehype-sanitize)
      const scripts = container.querySelectorAll('script');
      expect(scripts.length).toBeLessThanOrEqual(0);

      // Content should be rendered (Hello might be in a paragraph)
      expect(container.textContent).toContain('Hello');
    });

    it('prevents HTML injection via content', () => {
      const maliciousContent = '<div onclick="alert(1)">Click me</div>';
      const { container } = render(<MarkdownPreview content={maliciousContent} />);

      // HTML should be rendered as text, not executed
      expect(container.querySelector('div[onclick]')).not.toBeInTheDocument();
    });

    it('prevents XSS via malicious links', () => {
      const maliciousLink = '[Click](javascript:alert("XSS"))';
      const { container } = render(<MarkdownPreview content={maliciousLink} />);

      // rehype-sanitize should remove javascript: URLs entirely
      const link = container.querySelector('a');
      if (link) {
        const href = link.getAttribute('href');
        // Href might be null (removed) or sanitized to safe value
        if (href) {
          expect(href).not.toContain('javascript:');
        }
      }
      // Either way, the text "Click" should be present
      expect(container.textContent).toContain('Click');
    });

    it('allows safe HTML entities', () => {
      const { container } = render(<MarkdownPreview content="&lt;div&gt; &amp; &quot;quotes&quot;" />);
      // HTML entities should be decoded and rendered safely
      expect(container.textContent).toContain('&');
      expect(container.textContent).toContain('"quotes"');
    });
  });

  describe('Mixed Content', () => {
    it('renders complex markdown with mentions and formatting', () => {
      const content = `# Meeting Notes

**Attendees**: @Alice and @Bob Smith

## Action Items

- @Alice will review the code
- @Bob Smith will update the \`README.md\`

> Remember to push changes!

[Documentation](https://example.com)`;

      render(
        <MarkdownPreview
          content={content}
          mentionedCharacters={[
            { id: 1, name: 'Alice' },
            { id: 2, name: 'Bob Smith' },
          ]}
        />
      );

      // Check various elements are rendered
      expect(screen.getByText('Meeting Notes')).toBeInTheDocument();
      expect(screen.getByText('Attendees').tagName).toBe('STRONG');
      expect(screen.getAllByText(/@Alice/)[0].tagName).toBe('MARK');
      expect(screen.getAllByText(/@Bob Smith/)[0].tagName).toBe('MARK');
      expect(screen.getByText('README.md').tagName).toBe('CODE');
      expect(screen.getByRole('link', { name: 'Documentation' })).toHaveAttribute(
        'href',
        'https://example.com'
      );
    });
  });

  describe('Custom className', () => {
    it('applies custom className to container', () => {
      const { container } = render(
        <MarkdownPreview content="Test" className="custom-class" />
      );

      const previewDiv = container.querySelector('.markdown-preview');
      expect(previewDiv).toHaveClass('custom-class');
    });
  });

  describe('Edge Cases', () => {
    it('handles empty content', () => {
      const { container } = render(<MarkdownPreview content="" />);
      expect(container.querySelector('.markdown-preview')).toBeInTheDocument();
    });

    it('handles whitespace-only content', () => {
      const { container } = render(<MarkdownPreview content="   \n\n   " />);
      // Markdown might render whitespace as empty paragraphs, which is acceptable
      expect(container.querySelector('.markdown-preview')).toBeInTheDocument();
    });

    it('handles malformed markdown gracefully', () => {
      const malformed = '**bold without closing\n# Header without newline## Another header';
      const { container } = render(<MarkdownPreview content={malformed} />);

      // Should render something without crashing
      expect(container.querySelector('.markdown-preview')).toBeInTheDocument();
    });
  });

  describe('Inline Image Expansion', () => {
    it('renders an expand button next to image URLs', () => {
      render(<MarkdownPreview content="[photo](https://example.com/pic.png)" />);
      expect(screen.getByRole('button', { name: 'Expand image' })).toBeInTheDocument();
    });

    it('does not render an expand button for non-image URLs', () => {
      render(<MarkdownPreview content="[link](https://example.com/page)" />);
      expect(screen.queryByRole('button', { name: 'Expand image' })).not.toBeInTheDocument();
    });

    it('detects common image extensions', () => {
      const extensions = ['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg', 'avif', 'bmp'];
      for (const ext of extensions) {
        const { unmount } = render(
          <MarkdownPreview content={`[img](https://example.com/file.${ext})`} />
        );
        expect(screen.getByRole('button', { name: 'Expand image' })).toBeInTheDocument();
        unmount();
      }
    });

    it('shows the image and changes button label after clicking expand', () => {
      const { getByRole, getByAltText } = render(
        <MarkdownPreview content="[photo](https://example.com/pic.png)" />
      );
      fireEvent.click(getByRole('button', { name: 'Expand image' }));
      expect(getByAltText('')).toBeInTheDocument();
      expect(getByRole('button', { name: 'Collapse image' })).toBeInTheDocument();
    });

    it('hides the image after collapsing', () => {
      const { getByRole, queryByAltText } = render(
        <MarkdownPreview content="[photo](https://example.com/pic.png)" />
      );
      const button = getByRole('button', { name: 'Expand image' });
      fireEvent.click(button);
      fireEvent.click(button);
      expect(queryByAltText('')).not.toBeInTheDocument();
    });

    it('still renders the link itself for image URLs', () => {
      render(<MarkdownPreview content="[photo](https://example.com/pic.png)" />);
      const link = screen.getByRole('link', { name: 'photo' });
      expect(link).toHaveAttribute('href', 'https://example.com/pic.png');
    });
  });

  describe('Colored Text Rendering', () => {
    it('renders a known color as a span with data-color attribute', () => {
      const { container } = render(<MarkdownPreview content="[color:red]hello[/color]" />);
      const span = container.querySelector('[data-color="red"]');
      expect(span).toBeInTheDocument();
      expect(span?.textContent).toBe('hello');
    });

    it.each(TEXT_COLORS)('renders [color:%s] with the correct data-color attribute', (color) => {
      const { container } = render(<MarkdownPreview content={`[color:${color}]text[/color]`} />);
      const span = container.querySelector(`[data-color="${color}"]`);
      expect(span).toBeInTheDocument();
    });

    // The it.each above is driven by TEXT_COLORS, so it would still pass for a
    // color that has no styling at all. This guards the other side: every
    // accepted name must actually be paintable in both themes. The export
    // allowlist is pinned separately by TestAllowedColors in
    // backend/pkg/exports/markdown_test.go — neither test container mounts the
    // other's tree, so the two lists cannot be diffed automatically. Changing
    // the palette means updating that Go test too.
    describe('palette stays in sync across files', () => {
      // Path is relative to the vitest root (frontend/), not this file:
      // import.meta.url is an http:// URL under Vite, not a file path.
      const css = readFileSync('src/index.css', 'utf-8');

      it.each(TEXT_COLORS)('defines light and dark CSS for %s', (color) => {
        expect(css).toContain(`[data-color="${color}"]`);
        expect(css).toContain(`.dark [data-color="${color}"]`);
      });
    });

    it('renders unknown color names as literal text', () => {
      const { container } = render(<MarkdownPreview content="[color:mauve]some text[/color]" />);
      expect(container.querySelector('[data-color]')).not.toBeInTheDocument();
      expect(container.textContent).toContain('[color:mauve]some text[/color]');
    });

    it('does not process color syntax inside inline code', () => {
      const { container } = render(<MarkdownPreview content="`[color:red]text[/color]`" />);
      expect(container.querySelector('[data-color]')).not.toBeInTheDocument();
      expect(container.textContent).toContain('[color:red]text[/color]');
    });

    it('does not process color syntax inside fenced code blocks', () => {
      const content = '```\n[color:red]text[/color]\n```';
      const { container } = render(<MarkdownPreview content={content} />);
      expect(container.querySelector('[data-color]')).not.toBeInTheDocument();
    });

    it('renders multi-line content inside color tags', () => {
      const { container } = render(<MarkdownPreview content="[color:green]line one\nline two[/color]" />);
      const span = container.querySelector('[data-color="green"]');
      expect(span).toBeInTheDocument();
      expect(span?.textContent).toContain('line one');
      expect(span?.textContent).toContain('line two');
    });

    it('does not add a style attribute to colored spans', () => {
      const { container } = render(<MarkdownPreview content="[color:red]styled[/color]" />);
      const span = container.querySelector('[data-color="red"]');
      expect(span).toBeInTheDocument();
      expect(span).not.toHaveAttribute('style');
    });

    it('works with mentions inside a colored span', () => {
      const chars = [{ id: 1, name: 'Alice' }];
      const { container } = render(
        <MarkdownPreview content="[color:blue]@Alice[/color]" mentionedCharacters={chars} />
      );
      const span = container.querySelector('[data-color="blue"]');
      expect(span).toBeInTheDocument();
      const mark = span?.querySelector('[data-mention-id]');
      expect(mark).toBeInTheDocument();
    });

    it('does not create a span for injection attempts via color name', () => {
      // The color regex only matches [a-z]+, so this won't match as a color tag
      const { container } = render(
        <MarkdownPreview content='[color:red onmouseover="alert(1)"]text[/color]' />
      );
      expect(container.querySelector('[data-color]')).not.toBeInTheDocument();
    });

    it('renders bold text inside color tags', () => {
      const { container } = render(
        <MarkdownPreview content="[color:red]**bold**[/color]" />
      );
      const span = container.querySelector('[data-color="red"]');
      expect(span).toBeInTheDocument();
      const strong = span?.querySelector('strong');
      expect(strong).toBeInTheDocument();
      expect(strong?.textContent).toBe('bold');
    });

    it('renders italic text inside color tags', () => {
      const { container } = render(
        <MarkdownPreview content="[color:red]*italic*[/color]" />
      );
      const span = container.querySelector('[data-color="red"]');
      expect(span).toBeInTheDocument();
      const em = span?.querySelector('em');
      expect(em).toBeInTheDocument();
      expect(em?.textContent).toBe('italic');
    });
  });

  describe('Bold links', () => {
    it('renders a plain link without a strong element', () => {
      const { container } = render(
        <MarkdownPreview content="[Link](https://example.com)" />
      );
      const link = container.querySelector('a');
      expect(link).toBeInTheDocument();
      expect(link).toHaveTextContent('Link');
      expect(link?.querySelector('strong')).not.toBeInTheDocument();
    });

    it('renders **[Link](url)** as a link wrapped in strong', () => {
      const { container } = render(
        <MarkdownPreview content="**[Link](https://example.com)**" />
      );
      const link = container.querySelector('a');
      expect(link).toBeInTheDocument();
      expect(link).toHaveTextContent('Link');
      // Link should be inside a strong element
      expect(link?.closest('strong')).toBeInTheDocument();
    });

    it('renders [**Link**](url) as a strong element inside a link', () => {
      const { container } = render(
        <MarkdownPreview content="[**Link**](https://example.com)" />
      );
      const link = container.querySelector('a');
      expect(link).toBeInTheDocument();
      expect(link).toHaveTextContent('Link');
      // Strong should be inside the link
      expect(link?.querySelector('strong')).toBeInTheDocument();
    });
  });

  describe('Sheet Item References ([[item]] syntax)', () => {
    const sheetItems = [
      { id: 'abc-1', name: 'Fire Bolt', type: 'ability' as const, description: 'Deals fire damage', metadata: 'innate' },
      { id: 'xyz-2', name: 'Longbow', type: 'item' as const, description: 'A fine bow' },
    ];

    it('renders [[item]] tokens as amber highlighted marks', () => {
      const { container } = render(
        <MarkdownPreview
          content="I use [[Fire Bolt|ability:abc-1]]"
          sheetItemRefs={sheetItems}
        />
      );
      const mark = container.querySelector('[data-sheet-ref-id="abc-1"]');
      expect(mark).toBeInTheDocument();
      expect(mark?.textContent).toContain('Fire Bolt');
    });

    it('renders [[item]] marks even without sheetItemRefs (no tooltip, but mark shown)', () => {
      const { container } = render(
        <MarkdownPreview content="I use [[Fire Bolt|ability:abc-1]]" />
      );
      const mark = container.querySelector('[data-sheet-ref-id="abc-1"]');
      expect(mark).toBeInTheDocument();
    });

    it('does not process [[item]] syntax inside inline code', () => {
      const { container } = render(
        <MarkdownPreview content="`[[Fire Bolt|ability:abc-1]]`" sheetItemRefs={sheetItems} />
      );
      expect(container.querySelector('[data-sheet-ref-id]')).not.toBeInTheDocument();
    });

    it('does not process [[item]] syntax inside fenced code blocks', () => {
      const { container } = render(
        <MarkdownPreview content={'```\n[[Fire Bolt|ability:abc-1]]\n```'} sheetItemRefs={sheetItems} />
      );
      expect(container.querySelector('[data-sheet-ref-id]')).not.toBeInTheDocument();
    });

    it('renders multiple [[item]] references in one content string', () => {
      const { container } = render(
        <MarkdownPreview
          content="I fire [[Fire Bolt|ability:abc-1]] with my [[Longbow|item:xyz-2]]"
          sheetItemRefs={sheetItems}
        />
      );
      expect(container.querySelector('[data-sheet-ref-id="abc-1"]')).toBeInTheDocument();
      expect(container.querySelector('[data-sheet-ref-id="xyz-2"]')).toBeInTheDocument();
    });

    it('shows hover tooltip for item when sheetItemRefs contains the item', () => {
      const { container } = render(
        <MarkdownPreview
          content="I use [[Fire Bolt|ability:abc-1]]"
          sheetItemRefs={sheetItems}
        />
      );
      const mark = container.querySelector('[data-sheet-ref-id="abc-1"]') as HTMLElement;
      expect(mark).toBeInTheDocument();

      fireEvent.mouseOver(mark);

      // Tooltip with item name should appear
      expect(screen.getByText('Fire Bolt')).toBeInTheDocument();
    });

    it('renders markdown in the tooltip description', () => {
      const markdownItems = [
        {
          id: 'md-1',
          name: 'Power Attack',
          type: 'ability' as const,
          description: 'Deals **massive** damage.\n\n| Roll | Effect |\n| --- | --- |\n| 6 | Critical |',
          metadata: 'innate',
        },
      ];
      const { container } = render(
        <MarkdownPreview
          content="I use [[Power Attack|ability:md-1]]"
          sheetItemRefs={markdownItems}
        />
      );
      const mark = container.querySelector('[data-sheet-ref-id="md-1"]') as HTMLElement;
      fireEvent.mouseOver(mark);

      const tooltip = document.querySelector('[data-sheet-tooltip]') as HTMLElement;
      expect(tooltip).toBeInTheDocument();

      // Bold markdown must render as a <strong>, not literal "**massive**"
      const strong = tooltip.querySelector('strong');
      expect(strong).toBeInTheDocument();
      expect(strong?.textContent).toBe('massive');
      expect(tooltip.textContent).not.toContain('**massive**');

      // A markdown table must render as an actual <table> (can't be truncated mid-table)
      expect(tooltip.querySelector('table')).toBeInTheDocument();
    });

    it('shows the full description without a "Read more" truncation link', () => {
      const longDescription = 'A very long description. '.repeat(30); // > 200 chars
      const longItems = [
        {
          id: 'long-1',
          name: 'Epic Spell',
          type: 'ability' as const,
          description: longDescription,
        },
      ];
      render(
        <MarkdownPreview
          content="I cast [[Epic Spell|ability:long-1]]"
          sheetItemRefs={longItems}
        />
      );
      const mark = document.querySelector('[data-sheet-ref-id="long-1"]') as HTMLElement;
      fireEvent.mouseOver(mark);

      const tooltip = document.querySelector('[data-sheet-tooltip]') as HTMLElement;
      expect(tooltip).toBeInTheDocument();

      // No truncation control — full content is always shown
      expect(screen.queryByText('Read more')).not.toBeInTheDocument();
      // Full text present, not cut off with an ellipsis
      expect(tooltip.textContent).toContain(longDescription.trim());
    });
  });
});
