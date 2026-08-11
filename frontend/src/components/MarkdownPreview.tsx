import React, { useState, useRef, useEffect, useCallback } from 'react';
import { marked } from 'marked';
import DOMPurify from 'dompurify';
import CharacterAvatar from './CharacterAvatar';
import { Badge, Button } from './ui';
import type { SheetItem } from '../hooks/useCharacterSheetItems';

const ALLOWED_COLORS = new Set([
  'red', 'green', 'blue', 'purple', 'orange', 'gold', 'gray', 'teal', 'pink',
]);

const IMAGE_URL_PATTERN = /\.(png|jpe?g|gif|webp|svg|avif|bmp)(\?.*)?$/i;

function isImageUrl(url: string | undefined): boolean {
  if (!url) return false;
  try {
    const { pathname } = new URL(url);
    return IMAGE_URL_PATTERN.test(pathname);
  } catch {
    return IMAGE_URL_PATTERN.test(url);
  }
}

function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

// Configure marked once at module level via marked.use()
// This allows renderer methods to access this.parser for inline token processing
marked.use({
  gfm: true,
  renderer: {
    heading({ depth, tokens }) {
      const rendered = this.parser.parseInline(tokens);
      const classMap: Record<number, string> = {
        1: 'text-2xl font-bold mt-4 mb-2 !text-content-primary',
        2: 'text-xl font-bold mt-3 mb-2 !text-content-primary',
        3: 'text-lg font-bold mt-2 mb-1 !text-content-primary',
      };
      const cls = classMap[depth] ?? '';
      return `<h${depth} class="${cls}">${rendered}</h${depth}>\n`;
    },

    paragraph({ tokens }) {
      const rendered = this.parser.parseInline(tokens);
      return `<p class="my-2 !text-content-primary">${rendered}</p>\n`;
    },

    listitem(token) {
      const hasBlockContent = token.tokens.some(
        (t) => t.type === 'list' || t.type === 'blockquote' || t.type === 'code' || t.type === 'paragraph'
      );
      const rendered = hasBlockContent
        ? this.parser.parse(token.tokens)
        : this.parser.parseInline(token.tokens);
      return `<li class="ml-4 !text-content-primary">${rendered}</li>\n`;
    },

    blockquote({ tokens }) {
      // Parse child tokens rather than emitting raw source: raw text loses hard
      // line breaks (and all nested markdown) once the browser collapses newlines.
      const rendered = this.parser.parse(tokens);
      return `<blockquote class="border-l-4 border-theme-default pl-4 py-2 my-2 italic text-content-secondary">${rendered}</blockquote>\n`;
    },

    hr() {
      return `<hr class="my-4 border-t-2 border-theme-default" />\n`;
    },

    link({ href, tokens }) {
      if (!href) return this.parser.parseInline(tokens ?? []);
      const text = this.parser.parseInline(tokens ?? []);
      const isImage = isImageUrl(href);
      const expandBtn = isImage
        ? ` <button type="button" data-image-expand="${encodeURIComponent(href)}" class="inline-flex items-center justify-center ml-1 w-4 h-4 text-xs text-content-tertiary hover:text-interactive-primary transition-colors align-middle" title="Expand image" aria-label="Expand image">🖼</button>`
        : '';
      return `<a href="${href}" target="_blank" rel="noopener noreferrer" class="text-interactive-primary hover:text-interactive-primary-hover underline">${text}</a>${expandBtn}`;
    },

    code({ text }) {
      return `<pre class="my-2 p-3 rounded bg-bg-secondary overflow-x-auto text-sm"><code>${escapeHtml(text)}</code></pre>\n`;
    },
  },
});

const DOMPURIFY_CONFIG: Parameters<typeof DOMPurify.sanitize>[1] = {
  ALLOWED_TAGS: [
    'p', 'br', 'strong', 'em', 'code', 'pre', 'blockquote',
    'ul', 'ol', 'li', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
    'a', 'hr', 'table', 'thead', 'tbody', 'tr', 'th', 'td',
    'del', 'mark', 'span', 'img', 'button',
  ],
  ALLOWED_ATTR: [
    'href', 'target', 'rel', 'class', 'data-mention-id', 'data-sheet-ref-id',
    'data-color', 'data-image-expand',
    'src', 'alt', 'type', 'title', 'aria-label',
  ],
  ALLOW_DATA_ATTR: false,
  FORCE_BODY: false,
};

function renderMarkdown(content: string): string {
  const html = marked.parse(content) as string;
  return DOMPurify.sanitize(html, DOMPURIFY_CONFIG) as string;
}

interface MentionedCharacter {
  id: number;
  name: string;
  username?: string;
  character_type?: string;
  avatar_url?: string | null;
}

interface MarkdownPreviewProps {
  content: string;
  mentionedCharacters?: MentionedCharacter[];
  /** Character sheet items used to resolve [[item]] hover tooltips */
  sheetItemRefs?: SheetItem[];
  className?: string;
  /**
   * Allow full width for code blocks or constrain to optimal line length (65ch) for prose.
   * Default: false (constrained for optimal readability)
   */
  fullWidth?: boolean;
}

function splitByCodeBlocks(text: string): Array<{ text: string; isCode: boolean }> {
  const codeBlockRegex = /(```[\s\S]*?```|`[^`\n]+?`)/g;
  const segments: Array<{ text: string; isCode: boolean }> = [];
  let lastIndex = 0;
  let match;

  while ((match = codeBlockRegex.exec(text)) !== null) {
    if (match.index > lastIndex) {
      segments.push({ text: text.substring(lastIndex, match.index), isCode: false });
    }
    segments.push({ text: match[0], isCode: true });
    lastIndex = match.index + match[0].length;
  }

  if (lastIndex < text.length) {
    segments.push({ text: text.substring(lastIndex), isCode: false });
  }

  if (segments.length === 0) {
    segments.push({ text, isCode: false });
  }

  return segments;
}

function processContent(content: string, mentionedCharacters: MentionedCharacter[], sheetItemRefs: SheetItem[]): string {
  // Step 1: Process character mentions
  let mentionsResult = content;
  if (mentionedCharacters.length) {
    const segments = splitByCodeBlocks(content);
    const sortedCharacters = [...mentionedCharacters].sort((a, b) => b.name.length - a.name.length);

    mentionsResult = segments.map((segment) => {
      if (segment.isCode) return segment.text;

      let processedText = segment.text;
      const replacements: Array<{ placeholder: string; replacement: string }> = [];
      sortedCharacters.forEach(({ id, name }, index) => {
        const mentionRegex = new RegExp(`@(${name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'g');
        const placeholder = `___MENTION_${index}_${id}___`;
        processedText = processedText.replace(mentionRegex, (m) => {
          replacements.push({ placeholder, replacement: `<mark data-mention-id="${id}" class="bg-interactive-primary-subtle text-interactive-primary px-1 rounded font-medium cursor-pointer hover:bg-interactive-primary relative">${m}</mark>` });
          return placeholder;
        });
      });
      replacements.forEach(({ placeholder, replacement }) => {
        processedText = processedText.replace(placeholder, replacement);
      });
      return processedText;
    }).join('');
  }

  // Step 1b: Process [[DisplayName|type:uuid]] sheet item references, skipping code blocks
  if (sheetItemRefs.length > 0 || /\[\[([^\]|]+)\|(?:ability|skill|item):([^\]]+)\]\]/.test(mentionsResult)) {
    const sheetSegments = splitByCodeBlocks(mentionsResult);
    mentionsResult = sheetSegments.map((segment) => {
      if (segment.isCode) return segment.text;
      return segment.text.replace(
        /\[\[([^\]|]+)\|(?:ability|skill|item):([^\]]+)\]\]/g,
        (_match, displayName: string, refId: string) => {
          const safeDisplay = escapeHtml(displayName);
          return `<mark data-sheet-ref-id="${escapeHtml(refId)}" class="bg-amber-100 dark:bg-amber-900/30 text-amber-800 dark:text-amber-200 px-1 rounded font-medium cursor-help">[[${safeDisplay}]]</mark>`;
        }
      );
    }).join('');
  }

  // Step 2: Process [color:X]...[/color] syntax, skipping code blocks
  const colorRegex = /\[color:([a-z]+)\]([\s\S]*?)\[\/color\]/g;
  const hasColorSyntax = colorRegex.test(mentionsResult);
  if (!hasColorSyntax) return mentionsResult;

  const colorSegments = splitByCodeBlocks(mentionsResult);
  return colorSegments.map((segment) => {
    if (segment.isCode) return segment.text;
    return segment.text.replace(
      /\[color:([a-z]+)\]([\s\S]*?)\[\/color\]/g,
      (_match, colorName: string, innerText: string) => {
        if (ALLOWED_COLORS.has(colorName)) {
          return `<span data-color="${colorName}">${innerText}</span>`;
        }
        return _match;
      }
    );
  }).join('');
}

export const MarkdownPreview: React.FC<MarkdownPreviewProps> = ({
  content,
  mentionedCharacters = [],
  sheetItemRefs = [],
  className = '',
  fullWidth = false,
}) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const [hoveredMentionId, setHoveredMentionId] = useState<number | null>(null);
  const [tooltipPosition, setTooltipPosition] = useState<{ top: number; left: number } | null>(null);
  const [hoveredSheetRefId, setHoveredSheetRefId] = useState<string | null>(null);
  const [sheetTooltipPosition, setSheetTooltipPosition] = useState<{ top: number; left: number } | null>(null);
  const mouseOverTooltip = useRef(false);
  const sheetHideTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const hoveredCharacter = React.useMemo(() => {
    if (hoveredMentionId === null) return null;
    return mentionedCharacters.find((char) => char.id === hoveredMentionId) || null;
  }, [hoveredMentionId, mentionedCharacters]);

  const hoveredSheetItem = React.useMemo(() => {
    if (!hoveredSheetRefId) return null;
    return sheetItemRefs.find((item) => item.id === hoveredSheetRefId) || null;
  }, [hoveredSheetRefId, sheetItemRefs]);

  const processedContent = React.useMemo(
    () => processContent(content, mentionedCharacters, sheetItemRefs),
    [content, mentionedCharacters, sheetItemRefs]
  );

  const htmlContent = React.useMemo(() => renderMarkdown(processedContent), [processedContent]);

  // Event delegation for mention and sheet item tooltips
  const handleMouseOver = useCallback((e: MouseEvent) => {
    const relatedTarget = e.relatedTarget as Node | null;
    const mentionMark = (e.target as Element).closest('mark[data-mention-id]');
    if (mentionMark) {
      // Only act on true entry. Defensive fallback against re-render loops: if
      // MarkdownContent's React.memo is ever bypassed, Chrome re-fires mouseover
      // on DOM replacement and this guard prevents the infinite setState cycle.
      if (!mentionMark.contains(relatedTarget)) {
        const id = Number(mentionMark.getAttribute('data-mention-id'));
        const rect = mentionMark.getBoundingClientRect();
        setHoveredMentionId(id);
        setTooltipPosition({ top: rect.bottom + 4, left: rect.left });
      }
      return;
    }
    const sheetMark = (e.target as Element).closest('mark[data-sheet-ref-id]');
    if (sheetMark && !sheetMark.contains(relatedTarget)) {
      const id = sheetMark.getAttribute('data-sheet-ref-id') ?? null;
      const rect = sheetMark.getBoundingClientRect();
      setHoveredSheetRefId(id);
      setSheetTooltipPosition({ top: rect.bottom + 4, left: rect.left });
    }
  }, []);

  const handleMouseOut = useCallback((e: MouseEvent) => {
    const relatedTarget = e.relatedTarget as Node | null;
    const mentionMark = (e.target as Element).closest('mark[data-mention-id]');
    if (mentionMark && !mentionMark.contains(relatedTarget)) {
      setHoveredMentionId(null);
      setTooltipPosition(null);
    }
    const sheetMark = (e.target as Element).closest('mark[data-sheet-ref-id]');
    if (sheetMark && !sheetMark.contains(relatedTarget) && !mouseOverTooltip.current) {
      // Short delay lets the mouse cross the gap between mark and tooltip before hiding
      sheetHideTimer.current = setTimeout(() => {
        if (!mouseOverTooltip.current) {
          setHoveredSheetRefId(null);
          setSheetTooltipPosition(null);
        }
      }, 100);
    }
  }, []);

  // Click/tap on sheet mark shows tooltip (without auto-expanding)
  const handleSheetMarkClick = useCallback((e: MouseEvent) => {
    const sheetMark = (e.target as Element).closest('mark[data-sheet-ref-id]');
    if (sheetMark) {
      e.stopPropagation();
      const id = sheetMark.getAttribute('data-sheet-ref-id') ?? null;
      const rect = sheetMark.getBoundingClientRect();
      setHoveredSheetRefId(id);
      setSheetTooltipPosition({ top: rect.bottom + 4, left: rect.left });
      return;
    }
  }, []);

  // Event delegation for image expand/collapse
  const handleClick = useCallback((e: MouseEvent) => {
    const btn = (e.target as Element).closest('button[data-image-expand]');
    if (!btn) return;
    e.preventDefault();

    const href = decodeURIComponent(btn.getAttribute('data-image-expand') ?? '');
    const wrapper = btn.parentElement;
    if (!wrapper) return;

    const expandedBlock = wrapper.querySelector('.image-expand-block') as HTMLElement | null;

    if (expandedBlock) {
      expandedBlock.remove();
      btn.setAttribute('title', 'Expand image');
      btn.setAttribute('aria-label', 'Expand image');
      btn.textContent = '🖼';
    } else {
      const block = document.createElement('span');
      block.className = 'image-expand-block block mt-1';

      const img = document.createElement('img');
      img.src = href;
      img.alt = '';
      img.className = 'max-h-96 rounded border border-border-primary';
      img.style.maxWidth = '100%';
      img.addEventListener('error', () => {
        const errMsg = Object.assign(document.createElement('span'), {
          className: 'block mt-1 text-xs text-content-tertiary italic',
          textContent: 'Image could not be loaded.',
        });
        img.replaceWith(errMsg);
      });

      block.appendChild(img);
      btn.insertAdjacentElement('afterend', block);
      btn.setAttribute('title', 'Collapse image');
      btn.setAttribute('aria-label', 'Collapse image');
      btn.textContent = '▲';
    }
  }, []);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    el.addEventListener('mouseover', handleMouseOver);
    el.addEventListener('mouseout', handleMouseOut);
    el.addEventListener('click', handleSheetMarkClick);
    el.addEventListener('click', handleClick);
    return () => {
      el.removeEventListener('mouseover', handleMouseOver);
      el.removeEventListener('mouseout', handleMouseOut);
      el.removeEventListener('click', handleSheetMarkClick);
      el.removeEventListener('click', handleClick);
    };
  }, [handleMouseOver, handleMouseOut, handleSheetMarkClick, handleClick]);


  // Dismiss tooltip on click/touch outside
  useEffect(() => {
    if (!hoveredSheetRefId) return;
    const dismiss = (e: MouseEvent | TouchEvent) => {
      const el = (e instanceof TouchEvent ? e.touches[0]?.target : e.target) as Element | null;
      if (el && !el.closest('mark[data-sheet-ref-id]') && !el.closest('[data-sheet-tooltip]')) {
        setHoveredSheetRefId(null);
        setSheetTooltipPosition(null);
      }
    };
    document.addEventListener('click', dismiss);
    document.addEventListener('touchstart', dismiss);
    return () => {
      document.removeEventListener('click', dismiss);
      document.removeEventListener('touchstart', dismiss);
    };
  }, [hoveredSheetRefId]);

  return (
    <div className={`markdown-preview prose ${fullWidth ? 'max-w-none' : 'max-w-prose'} text-content-primary dark:text-white ${className}`}>
      <MarkdownContent containerRef={containerRef} htmlContent={htmlContent} />

      {hoveredCharacter && tooltipPosition && (
        <div
          className="fixed z-50 px-3 py-2 text-sm bg-gray-900 dark:bg-gray-800 border border-border-primary rounded-lg shadow-lg pointer-events-none"
          style={{ top: `${tooltipPosition.top}px`, left: `${tooltipPosition.left}px` }}
        >
          <div className="flex items-center gap-2">
            <CharacterAvatar
              avatarUrl={hoveredCharacter.avatar_url}
              characterName={hoveredCharacter.name}
              size="sm"
            />
            <div>
              <div className="font-semibold text-white">{hoveredCharacter.name}</div>
              {hoveredCharacter.username && (
                <div className="text-xs text-gray-300 mt-0.5">
                  Player: {hoveredCharacter.username}
                </div>
              )}
              {hoveredCharacter.character_type && (
                <div className="text-xs text-gray-400 mt-0.5 capitalize">
                  {hoveredCharacter.character_type.replace('_', ' ')}
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {hoveredSheetItem && sheetTooltipPosition && (() => {
        const desc = hoveredSheetItem.description ?? '';
        return (
          <div
            data-sheet-tooltip
            className="fixed z-50 px-3 py-2 text-sm surface-overlay border border-semantic-warning rounded-lg shadow-lg max-w-sm pointer-events-auto cursor-default"
            style={{ top: `${sheetTooltipPosition.top}px`, left: `${sheetTooltipPosition.left}px` }}
            onMouseEnter={() => {
              mouseOverTooltip.current = true;
              if (sheetHideTimer.current !== null) {
                clearTimeout(sheetHideTimer.current);
                sheetHideTimer.current = null;
              }
            }}
            onMouseLeave={() => {
              mouseOverTooltip.current = false;
              setHoveredSheetRefId(null);
              setSheetTooltipPosition(null);
            }}
          >
            <div className="relative pr-6">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-1.5 flex-wrap">
                  <span className="font-semibold text-content-primary">{hoveredSheetItem.name}</span>
                  <Badge variant="warning" size="sm" className="capitalize">{hoveredSheetItem.type}</Badge>
                </div>
                {hoveredSheetItem.metadata && (
                  <div className="text-xs text-content-tertiary mt-0.5">{hoveredSheetItem.metadata}</div>
                )}
                {desc && (
                  <div className="text-xs mt-1 leading-relaxed max-h-64 overflow-y-auto">
                    <MarkdownPreview content={desc} fullWidth className="!text-xs" />
                  </div>
                )}
              </div>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={(e) => {
                  e.stopPropagation();
                  setHoveredSheetRefId(null);
                  setSheetTooltipPosition(null);
                }}
                className="absolute top-0 right-0 !min-h-0 !py-0 !px-0 w-5 h-5 flex items-center justify-center"
                aria-label="Close"
              >
                ×
              </Button>
            </div>
          </div>
        );
      })()}
    </div>
  );
};

interface MarkdownContentProps {
  containerRef: React.RefObject<HTMLDivElement | null>;
  htmlContent: string;
}

const MarkdownContent = React.memo(function MarkdownContent({ containerRef, htmlContent }: MarkdownContentProps) {
  return (
    <div
      ref={containerRef}
      dangerouslySetInnerHTML={{ __html: htmlContent }}
    />
  );
});

