import { useCallback, useEffect, useRef, useState } from 'react';
import { MarkdownPreview } from './MarkdownPreview';

/** Tailwind class naming the surface the collapsed fade blends into. */
type FadeSurface = 'surface-base' | 'surface-raised' | 'info-subtle';

const FADE_GRADIENTS: Record<FadeSurface, string> = {
  'surface-base': 'from-[rgb(var(--color-surface-base))]',
  'surface-raised': 'from-[rgb(var(--color-surface-raised))]',
  'info-subtle': 'from-[rgb(var(--color-semantic-info-subtle))]',
};

interface CollapsibleMarkdownProps {
  content: string;
  /** Max rendered height while collapsed, in px. */
  collapsedMaxHeight?: number;
  /**
   * Which background the fade overlay blends into. Must match the element the
   * content actually sits on, or the gradient bands against it.
   */
  fadeSurface?: FadeSurface;
  /** Controlled expansion. Omit both to let the component own the state. */
  expanded?: boolean;
  onExpandedChange?: (expanded: boolean) => void;
  expandLabel?: string;
  collapseLabel?: string;
  /** Render the toggle above the content instead of below it. */
  togglePosition?: 'below' | 'above';
  /**
   * Suppress the built-in toggle. For call sites whose control lives elsewhere
   * in their own layout (e.g. a header row) — they drive `expanded` themselves.
   */
  showToggle?: boolean;
  className?: string;
  'data-testid'?: string;
  // Passed straight through to MarkdownPreview.
  fullWidth?: boolean;
  sheetItemRefs?: React.ComponentProps<typeof MarkdownPreview>['sheetItemRefs'];
  mentionedCharacters?: React.ComponentProps<typeof MarkdownPreview>['mentionedCharacters'];
}

/**
 * Markdown that collapses to a fixed height with a fade, and expands on click.
 *
 * Renders the **complete** markdown and clips it visually. The pattern this
 * replaces sliced the markdown *source* (`content.substring(0, 200) + '...'`)
 * and rendered the fragment, so a cut landing inside `**bold**`, `[a](b)`,
 * `[[Sheet Item]]` or a fenced code block produced broken output. Clipping the
 * rendered element cannot do that.
 *
 * Overflow is measured, not guessed from source length: 200 characters of
 * headings is far taller than 200 characters of prose, so a length threshold
 * both misses tall-but-short content and puts a useless toggle on content that
 * already fits.
 */
export function CollapsibleMarkdown({
  content,
  collapsedMaxHeight = 160,
  fadeSurface = 'surface-base',
  expanded: controlledExpanded,
  onExpandedChange,
  expandLabel = 'Show full content',
  collapseLabel = 'Show less',
  togglePosition = 'below',
  showToggle = true,
  className = '',
  'data-testid': testId,
  fullWidth,
  sheetItemRefs,
  mentionedCharacters,
}: CollapsibleMarkdownProps) {
  const contentRef = useRef<HTMLDivElement>(null);
  const [overflows, setOverflows] = useState(false);
  const [uncontrolledExpanded, setUncontrolledExpanded] = useState(false);

  const isControlled = controlledExpanded !== undefined;
  const isExpanded = isControlled ? controlledExpanded : uncontrolledExpanded;

  const toggle = useCallback(() => {
    const next = !isExpanded;
    if (!isControlled) {
      setUncontrolledExpanded(next);
    }
    onExpandedChange?.(next);
  }, [isExpanded, isControlled, onExpandedChange]);

  // Measure the rendered content against the collapsed height. Re-measures on
  // resize because reflow (window width, a font swap, an image finishing load)
  // changes height without changing `content`.
  useEffect(() => {
    const element = contentRef.current;
    if (!element) return;

    const measure = () => {
      setOverflows(element.scrollHeight > collapsedMaxHeight + 1);
    };
    measure();

    if (typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, [content, collapsedMaxHeight]);

  const isCollapsed = overflows && !isExpanded;

  const toggleButton = overflows && showToggle ? (
    <button
      type="button"
      onClick={toggle}
      aria-expanded={isExpanded}
      className={`text-sm text-interactive-primary hover:text-interactive-primary-hover font-medium flex items-center ${
        togglePosition === 'below' ? 'mt-2' : 'mb-2'
      }`}
    >
      <svg
        className={`w-4 h-4 mr-1 transition-transform ${isExpanded ? 'rotate-180' : ''}`}
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
      </svg>
      {isExpanded ? collapseLabel : expandLabel}
    </button>
  ) : null;

  return (
    <div className={className}>
      {togglePosition === 'above' && toggleButton}
      <div
        className="relative"
        style={isCollapsed ? { maxHeight: collapsedMaxHeight, overflow: 'hidden' } : undefined}
        data-testid={testId}
        data-collapsed={isCollapsed ? 'true' : 'false'}
      >
        <div ref={contentRef}>
          <MarkdownPreview
            content={content}
            fullWidth={fullWidth}
            sheetItemRefs={sheetItemRefs}
            mentionedCharacters={mentionedCharacters}
          />
        </div>
        {isCollapsed && (
          <div
            aria-hidden="true"
            className={`absolute inset-x-0 bottom-0 h-12 bg-gradient-to-t to-transparent ${FADE_GRADIENTS[fadeSurface]}`}
          />
        )}
      </div>
      {togglePosition === 'below' && toggleButton}
    </div>
  );
}
