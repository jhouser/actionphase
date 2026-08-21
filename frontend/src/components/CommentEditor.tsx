import { useState, useRef, useEffect, useCallback, memo } from 'react';
import type { ReactNode, MutableRefObject, ChangeEvent, KeyboardEvent } from 'react';
import { useBlocker } from 'react-router-dom';
import { Pencil, Eye, HelpCircle } from 'lucide-react';
import { MarkdownPreview } from './MarkdownPreview';
import { TEXT_COLORS } from './textColors';
import { CharacterAutocomplete } from './CharacterAutocomplete';
import { SheetItemAutocomplete } from './SheetItemAutocomplete';
import { Button, Textarea, Modal } from './ui';
import { STICKY_BELOW_TABS } from './TabNavigation';
import type { Character } from '../types/characters';
import type { SheetItem } from '../hooks/useCharacterSheetItems';
import { postCachingService } from '@/services/PostCachingService';

/**
 * Inner component that calls useBlocker and renders the confirmation modal.
 * Extracted so useBlocker is only called when warnOnUnsavedChanges is true,
 * keeping it isolated from the main editor (hooks must not be called conditionally).
 */
function UnsavedChangesGuard({ hasContent }: { hasContent: boolean }) {
  const blocker = useBlocker(hasContent);

  useEffect(() => {
    if (!hasContent) return;
    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };
    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => window.removeEventListener('beforeunload', handleBeforeUnload);
  }, [hasContent]);

  return (
    <Modal
      isOpen={blocker.state === 'blocked'}
      onClose={() => blocker.reset?.()}
      title="Leave page?"
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={() => blocker.reset?.()}>
            Stay
          </Button>
          <Button variant="danger" onClick={() => blocker.proceed?.()}>
            Leave
          </Button>
        </>
      }
    >
      <p className="text-content-primary">
        You have unsaved text in this editor. If you leave, your changes will be lost.
      </p>
    </Modal>
  );
}

interface CommentEditorProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
  rows?: number;
  showPreviewByDefault?: boolean;
  characters?: Character[]; // Characters available for mention autocomplete
  sheetItems?: SheetItem[]; // Character sheet items for %% trigger autocomplete
  id?: string; // HTML id for label association
  maxLength?: number; // Maximum character limit
  showCharacterCount?: boolean; // Show character counter below textarea
  textareaTestId?: string; // data-testid forwarded to the inner textarea (for E2E tests)
  warnOnUnsavedChanges?: boolean; // Show confirmation dialog when navigating away with unsaved content
  stickyTabBar?: boolean; // Stick the tab bar below the nav when scrolling (only appropriate for full-page editors, not inline edit forms)
  sheetButton?: ReactNode; // Optional node rendered in the drag-handle bar (e.g. "Character Sheet" toggle)
  insertSheetItemRef?: MutableRefObject<((item: SheetItem) => void) | null>; // Ref to expose cursor-aware insert for external callers (e.g. Drawer)
  autosaveRefId?: string; //Ref to a localstorage key for autosave purposes. Undefined disables autosave.
}

/**
 * CommentEditor Component
 *
 * A markdown-enabled text editor with live preview functionality.
 * Replaces plain textareas in comment forms.
 *
 * Features:
 * - Live markdown preview toggle
 * - Split view (editor | preview)
 * - Markdown help reference
 * - Support for character mentions (@CharacterName)
 */
export const CommentEditor = memo(function CommentEditor({
  value,
  onChange,
  placeholder = 'Write your comment...',
  disabled = false,
  rows = 4,
  showPreviewByDefault = false,
  characters = [],
  sheetItems = [],
  id,
  maxLength,
  showCharacterCount = false,
  textareaTestId,
  warnOnUnsavedChanges = false,
  stickyTabBar = false,
  sheetButton,
  insertSheetItemRef,
  autosaveRefId = undefined,
}: CommentEditorProps) {
  const [showPreview, setShowPreview] = useState(showPreviewByDefault);
  const [showHelp, setShowHelp] = useState(false);
  const [showAutocomplete, setShowAutocomplete] = useState(false);
  const [autocompleteQuery, setAutocompleteQuery] = useState('');
  const [autocompletePosition, setAutocompletePosition] = useState({ top: 0, left: 0 });
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [mentionStartIndex, setMentionStartIndex] = useState(0);

  // Sheet item autocomplete state (%% trigger)
  const [showSheetAutocomplete, setShowSheetAutocomplete] = useState(false);
  const [sheetQuery, setSheetQuery] = useState('');
  const [sheetAutocompletePosition, setSheetAutocompletePosition] = useState({ top: 0, left: 0 });
  const [sheetSelectedIndex, setSheetSelectedIndex] = useState(0);
  const [sheetTriggerStartIndex, setSheetTriggerStartIndex] = useState(0);

  const [editorHeight, setEditorHeight] = useState<number | null>(null);
  const dragStartY = useRef<number | null>(null);
  const dragStartHeight = useRef<number>(0);
  const editorRef = useRef<HTMLDivElement>(null);

  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const loaded = useRef(false);

  useEffect(() => {
    if (loaded.current) {
      return;
    }

    loaded.current = true;
    //fired when component mounted
    if (!value && autosaveRefId) {
      const autosavedContent = postCachingService.get(autosaveRefId);
      if (autosavedContent) {
        onChange(autosavedContent);
      }
    }
  }, [loaded, value, autosaveRefId]); // eslint-disable-line react-hooks/exhaustive-deps

  // Calculate cursor position for autocomplete dropdown
  const getCaretCoordinates = (element: HTMLTextAreaElement, position: number) => {
    // Get the textarea's position in viewport
    const rect = element.getBoundingClientRect();

    // Create a mirror div to measure text position
    const computed = window.getComputedStyle(element);
    const div = document.createElement('div');

    // Copy styles from textarea
    const styles = [
      'fontSize', 'fontFamily', 'fontWeight', 'wordWrap',
      'whiteSpace', 'borderWidth', 'paddingLeft', 'paddingRight',
      'paddingTop', 'paddingBottom', 'lineHeight',
    ];
    styles.forEach(style => {
      const styleProp = style as keyof CSSStyleDeclaration;
      const value = computed[styleProp];
      if (typeof value === 'string') {
        (div.style as unknown as Record<string, string>)[style] = value;
      }
    });

    // Position the mirror div at the same place as textarea
    div.style.position = 'absolute';
    div.style.top = '0px';
    div.style.left = '0px';
    div.style.visibility = 'hidden';
    div.style.whiteSpace = 'pre-wrap';
    div.style.wordWrap = 'break-word';
    div.style.width = element.clientWidth + 'px';
    div.textContent = element.value.substring(0, position);

    // Add a span at cursor position
    const span = document.createElement('span');
    span.textContent = '|'; // Cursor marker
    div.appendChild(span);

    document.body.appendChild(div);

    const spanRect = span.getBoundingClientRect();
    const divRect = div.getBoundingClientRect();

    document.body.removeChild(div);

    // Calculate position relative to viewport
    const top = rect.top + (spanRect.top - divRect.top) - element.scrollTop + 20;
    const left = rect.left + (spanRect.left - divRect.left);

    return { top, left };
  };

  // Detect %% trigger for sheet item autocomplete
  const handleSheetTriggerDetect = (newValue: string, cursorPosition: number) => {
    if (sheetItems.length === 0) {
      setShowSheetAutocomplete(false);
      return;
    }

    const textBeforeCursor = newValue.substring(0, cursorPosition);
    const lastDoublePercent = textBeforeCursor.lastIndexOf('%%');

    if (lastDoublePercent === -1) {
      setShowSheetAutocomplete(false);
      return;
    }

    const textAfterTrigger = textBeforeCursor.substring(lastDoublePercent + 2);
    if (textAfterTrigger.includes(' ') || textAfterTrigger.includes('\n') || textAfterTrigger.length > 50) {
      setShowSheetAutocomplete(false);
      return;
    }

    setShowSheetAutocomplete(true);
    setShowAutocomplete(false); // mutually exclusive
    setSheetQuery(textAfterTrigger);
    setSheetTriggerStartIndex(lastDoublePercent);
    setSheetSelectedIndex(0);

    if (textareaRef.current) {
      const position = getCaretCoordinates(textareaRef.current, cursorPosition);
      setSheetAutocompletePosition(position);
    }
  };

  // Detect @ and trigger character mention autocomplete
  const handleChange = (e: ChangeEvent<HTMLTextAreaElement>) => {
    const newValue = e.target.value;
    const cursorPosition = e.target.selectionStart || 0;

    onChange(newValue);

    // Run both trigger detections — they are mutually exclusive via setShow* calls
    let atTriggered = false;

    if (characters.length > 0) {
      const textBeforeCursor = newValue.substring(0, cursorPosition);
      const lastAtIndex = textBeforeCursor.lastIndexOf('@');

      if (lastAtIndex !== -1) {
        const textAfterAt = textBeforeCursor.substring(lastAtIndex + 1);
        if (!textAfterAt.includes(' ') && !textAfterAt.includes('\n') && textAfterAt.length <= 50) {
          setShowAutocomplete(true);
          setAutocompleteQuery(textAfterAt);
          setMentionStartIndex(lastAtIndex);
          setSelectedIndex(0);
          setShowSheetAutocomplete(false);
          atTriggered = true;

          if (textareaRef.current) {
            const position = getCaretCoordinates(textareaRef.current, cursorPosition);
            setAutocompletePosition(position);
          }
        }
      }
    }

    if (!atTriggered) {
      setShowAutocomplete(false);
    }

    handleSheetTriggerDetect(newValue, cursorPosition);

    if (autosaveRefId) {
      postCachingService.save(autosaveRefId, newValue);
    }
  };

  // Handle character selection from autocomplete
  const handleSelectCharacter = (character: Character) => {
    if (!textareaRef.current) return;

    const before = value.substring(0, mentionStartIndex);
    const after = value.substring(textareaRef.current.selectionStart || 0);
    const newValue = before + `@${character.name} ` + after;

    onChange(newValue);
    setShowAutocomplete(false);

    // Set cursor after mention
    setTimeout(() => {
      if (textareaRef.current) {
        const newCursorPos = mentionStartIndex + character.name.length + 2; // @ + name + space
        textareaRef.current.setSelectionRange(newCursorPos, newCursorPos);
        textareaRef.current.focus();
      }
    }, 0);
  };

  // Insert a sheet item token at a given index (or current cursor position)
  const handleInsertSheetItem = useCallback((item: SheetItem, insertAtIndex?: number) => {
    const textarea = textareaRef.current;
    const cursorPos = textarea?.selectionStart ?? value.length;
    // When called from the %% trigger, insertAtIndex is the start of "%%".
    // Strip from that point through the current cursor (removes "%%" + any typed query).
    const before = insertAtIndex !== null && insertAtIndex !== undefined
      ? value.substring(0, insertAtIndex)
      : value.substring(0, cursorPos);
    const after = value.substring(cursorPos);
    const token = `[[${item.name}|${item.type}:${item.id}]] `;
    onChange(before + token + after);
    setShowSheetAutocomplete(false);

    // Restore cursor after the inserted token
    const insertStart = insertAtIndex ?? cursorPos;
    setTimeout(() => {
      if (textareaRef.current) {
        const newPos = insertStart + token.length;
        textareaRef.current.setSelectionRange(newPos, newPos);
        textareaRef.current.focus();
      }
    }, 0);
  }, [value, onChange]);

  // Expose cursor-aware insert to external callers (e.g. a Drawer's SheetPanel)
  useEffect(() => {
    if (insertSheetItemRef) {
      insertSheetItemRef.current = (item: SheetItem) => handleInsertSheetItem(item);
    }
  }, [insertSheetItemRef, handleInsertSheetItem]);

  // Handle keyboard navigation in autocomplete
  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (showSheetAutocomplete) {
      const filteredItems = sheetQuery
        ? sheetItems.filter((i) => i.name.toLowerCase().includes(sheetQuery.toLowerCase()))
        : sheetItems;

      switch (e.key) {
        case 'ArrowDown':
          e.preventDefault();
          setSheetSelectedIndex((prev) => (prev + 1) % Math.max(filteredItems.length, 1));
          return;
        case 'ArrowUp':
          e.preventDefault();
          setSheetSelectedIndex((prev) => (prev - 1 + Math.max(filteredItems.length, 1)) % Math.max(filteredItems.length, 1));
          return;
        case 'Enter':
          if (filteredItems.length > 0) {
            e.preventDefault();
            handleInsertSheetItem(filteredItems[sheetSelectedIndex], sheetTriggerStartIndex);
          }
          return;
        case 'Escape':
          e.preventDefault();
          setShowSheetAutocomplete(false);
          return;
      }
    }

    if (!showAutocomplete) return;

    const filteredCharacters = characters.filter((char) =>
      char.name.toLowerCase().includes(autocompleteQuery.toLowerCase())
    );

    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setSelectedIndex((prev) => (prev + 1) % filteredCharacters.length);
        break;
      case 'ArrowUp':
        e.preventDefault();
        setSelectedIndex((prev) => (prev - 1 + filteredCharacters.length) % filteredCharacters.length);
        break;
      case 'Enter':
        if (filteredCharacters.length > 0) {
          e.preventDefault();
          handleSelectCharacter(filteredCharacters[selectedIndex]);
        }
        break;
      case 'Escape':
        e.preventDefault();
        setShowAutocomplete(false);
        break;
    }
  };

  // Close autocompletes when clicking outside
  useEffect(() => {
    const handleClickOutside = () => {
      setShowAutocomplete(false);
      setShowSheetAutocomplete(false);
    };

    if (showAutocomplete || showSheetAutocomplete) {
      document.addEventListener('click', handleClickOutside);
      return () => document.removeEventListener('click', handleClickOutside);
    }
  }, [showAutocomplete, showSheetAutocomplete]);

  const handleDragStart = useCallback((clientY: number) => {
    // Measure the actual textarea or preview div height at drag start.
    // editorRef points to the panel; use the current editorHeight or fall back
    // to the panel's inner height minus padding (approx 24px for p-3).
    dragStartY.current = clientY;
    const panelHeight = editorRef.current ? editorRef.current.offsetHeight : 0;
    dragStartHeight.current = editorHeight !== null ? editorHeight : Math.max(0, panelHeight - 24);
    document.body.style.userSelect = 'none';
    document.body.style.cursor = 'row-resize';
  }, [editorHeight]);

  const handleDragMove = useCallback((clientY: number) => {
    if (dragStartY.current === null) return;
    const delta = clientY - dragStartY.current;

    // Clamp against the space actually available below the editor's top edge so
    // the editor can never grow past the visible area and push the surrounding
    // controls (e.g. a Send button) off screen. We measure the editor panel's
    // position live and use the visual viewport bottom (which shrinks correctly
    // when a mobile keyboard is open) rather than a fixed viewport fraction.
    const MIN_HEIGHT = 80;
    let maxHeight = Number.POSITIVE_INFINITY;
    const panel = editorRef.current;
    if (panel) {
      const panelTop = panel.getBoundingClientRect().top;
      const viewport = window.visualViewport;
      const viewportBottom = viewport ? viewport.offsetTop + viewport.height : window.innerHeight;
      // Reserve space for the content below the resizable area (the drag handle
      // plus roughly one control row) so it stays on screen. This is a small,
      // layout-independent safety margin, not a cap on the editor size itself.
      const RESERVED_BELOW = 96;
      // panelTop → available height from the top of the editor panel to the
      // bottom of the usable viewport, minus the reserved control space.
      // (panel padding is ~24px; subtract it since editorHeight sizes the inner field.)
      maxHeight = Math.max(MIN_HEIGHT, viewportBottom - panelTop - 24 - RESERVED_BELOW);
    }

    const newHeight = Math.min(maxHeight, Math.max(MIN_HEIGHT, dragStartHeight.current + delta));
    setEditorHeight(newHeight);
  }, []);

  const handleDragEnd = useCallback(() => {
    dragStartY.current = null;
    document.body.style.userSelect = '';
    document.body.style.cursor = '';
  }, []);

  useEffect(() => {
    const onMouseMove = (e: MouseEvent) => handleDragMove(e.clientY);
    const onTouchMove = (e: TouchEvent) => handleDragMove(e.touches[0].clientY);
    const onEnd = () => handleDragEnd();

    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onEnd);
    document.addEventListener('touchmove', onTouchMove, { passive: true });
    document.addEventListener('touchend', onEnd);
    return () => {
      document.removeEventListener('mousemove', onMouseMove);
      document.removeEventListener('mouseup', onEnd);
      document.removeEventListener('touchmove', onTouchMove);
      document.removeEventListener('touchend', onEnd);
    };
  }, [handleDragMove, handleDragEnd]);

  return (
    <div className="comment-editor">
      {/* Tab bar + secondary controls */}
      <div
        className={`${stickyTabBar ? 'sticky' : ''} z-10 surface-base border-b border-theme-default`}
        style={stickyTabBar ? { top: STICKY_BELOW_TABS } : undefined}
      >
        <div className="flex items-center justify-between pt-2 pb-1 gap-2">
          {/* Manila-style tabs */}
          <div className="flex items-end gap-1 shrink-0">
            <button
              type="button"
              onClick={() => setShowPreview(false)}
              disabled={disabled}
              title="Write"
              className={[
                'flex items-center justify-center gap-1.5 min-w-[56px] px-5 py-1.5 text-sm font-medium rounded-t-md border border-b-0 transition-colors',
                !showPreview
                  ? 'surface-base border-theme-default text-content-primary relative z-10 -mb-px'
                  : 'surface-sunken border-transparent text-content-tertiary hover:text-content-secondary',
              ].join(' ')}
            >
              <Pencil className="w-3.5 h-3.5" />
              <span className="hidden sm:inline">Write</span>
            </button>
            <button
              type="button"
              onClick={() => setShowPreview(true)}
              disabled={disabled}
              title="Preview"
              className={[
                'flex items-center justify-center gap-1.5 min-w-[56px] px-5 py-1.5 text-sm font-medium rounded-t-md border border-b-0 transition-colors',
                showPreview
                  ? 'surface-base border-theme-default text-content-primary relative z-10 -mb-px'
                  : 'surface-sunken border-transparent text-content-tertiary hover:text-content-secondary',
              ].join(' ')}
            >
              <Eye className="w-3.5 h-3.5" />
              <span className="hidden sm:inline">Preview</span>
            </button>
          </div>

          {/* Secondary controls */}
          <div className="flex items-center gap-1">
            {sheetButton && <div>{sheetButton}</div>}
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => setShowHelp(!showHelp)}
              disabled={disabled}
              title="Markdown Help"
              className="!px-2"
            >
              <HelpCircle className="w-4 h-4" />
              <span className="hidden sm:inline">Markdown Help</span>
            </Button>
            <span className="text-xs text-content-tertiary whitespace-nowrap self-center">{value.length} characters</span>
          </div>
        </div>
      </div>

      {/* Tab content panel — shares border with active tab */}
      <div ref={editorRef} className="border border-b-0 border-theme-default rounded-tr-md surface-base p-3">
        {/* Markdown Help Panel */}
        {showHelp && (
          <div className="mb-3 p-3 bg-interactive-primary-subtle border border-interactive-primary rounded text-xs">
            <div className="font-semibold text-interactive-primary mb-2">Markdown Quick Reference</div>
            <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-content-primary">
              <div>
                <code className="surface-sunken px-1 rounded">**bold**</code> → <strong>bold</strong>
              </div>
              <div>
                <code className="surface-sunken px-1 rounded">*italic*</code> → <em>italic</em>
              </div>
              <div>
                <code className="surface-sunken px-1 rounded">[link](url)</code> → link
              </div>
              <div>
                <code className="surface-sunken px-1 rounded">`code`</code> → <code className="surface-sunken px-1">code</code>
              </div>
              <div>
                <code className="surface-sunken px-1 rounded"># Heading</code> → Heading
              </div>
              <div>
                <code className="surface-sunken px-1 rounded">- list item</code> → • list item
              </div>
              <div>
                <code className="surface-sunken px-1 rounded">&gt; quote</code> → blockquote
              </div>
              <div>
                <code className="surface-sunken px-1 rounded">@CharacterName</code> → mention
              </div>
              <div>
                <code className="surface-sunken px-1 rounded">%%</code> → insert sheet item
              </div>
            </div>
            <div className="mt-2 pt-2 border-t border-theme-default text-content-primary">
              <div className="font-semibold text-content-secondary mb-1">Colored Text</div>
              <div>
                <code className="surface-sunken px-1 rounded">[color:red]text[/color]</code>
              </div>
              <div className="mt-1 flex flex-wrap gap-x-2 gap-y-0.5">
                {TEXT_COLORS.map((color) => (
                  <span key={color} data-color={color}>
                    {color}
                  </span>
                ))}
              </div>
            </div>
          </div>
        )}

        {/* Write tab */}
        {!showPreview && (
          <Textarea
            id={id}
            ref={textareaRef}
            value={value}
            onChange={handleChange}
            onKeyDown={handleKeyDown}
            placeholder={placeholder}
            disabled={disabled}
            rows={rows}
            textareaSize="sm"
            className="font-mono resize-none"
            style={editorHeight !== null ? { height: editorHeight } : undefined}
            maxLength={maxLength}
            showCharacterCount={showCharacterCount}
            data-testid={textareaTestId}
          />
        )}

        {/* Preview tab */}
        {showPreview && (
          <div
            className="overflow-auto"
            style={editorHeight !== null ? { height: editorHeight } : { minHeight: `${rows * 1.5}rem`, maxHeight: `${rows * 1.5}rem` }}
          >
            {value.trim() ? (
              <MarkdownPreview content={value} fullWidth sheetItemRefs={sheetItems} />
            ) : (
              <p className="text-xs text-content-tertiary italic">Preview will appear here...</p>
            )}
          </div>
        )}
      </div>

      {/* Drag handle */}
      <div
        className="relative flex items-center justify-center h-5 border border-t-0 border-theme-default rounded-b-md cursor-row-resize touch-none select-none surface-raised group"
        onMouseDown={(e) => { e.preventDefault(); handleDragStart(e.clientY); }}
        onTouchStart={(e) => handleDragStart(e.touches[0].clientY)}
        aria-hidden="true"
      >
        {/* Center pill — prominent on mobile */}
        <div className="w-10 h-1 rounded-full bg-gray-400 group-hover:bg-interactive-primary transition-colors" />
        {/* Corner grip dots — familiar desktop affordance */}
        <div className="absolute right-2 bottom-1 hidden sm:grid grid-cols-2 gap-px opacity-40 group-hover:opacity-80 transition-opacity">
          <div className="w-1 h-1 rounded-full bg-gray-500" />
          <div className="w-1 h-1 rounded-full bg-gray-500" />
          <div className="w-1 h-1 rounded-full bg-gray-500" />
          <div className="w-1 h-1 rounded-full bg-gray-500" />
          <div className="w-1 h-1 rounded-full bg-gray-500" />
          <div className="w-1 h-1 rounded-full bg-gray-500" />
        </div>
      </div>

      {/* Character Autocomplete */}
      {showAutocomplete && characters.length > 0 && (
        <CharacterAutocomplete
          characters={characters}
          query={autocompleteQuery}
          position={autocompletePosition}
          onSelect={handleSelectCharacter}
          selectedIndex={selectedIndex}
          onClose={() => setShowAutocomplete(false)}
        />
      )}

      {/* Sheet Item Autocomplete (%% trigger) */}
      {showSheetAutocomplete && sheetItems.length > 0 && (
        <SheetItemAutocomplete
          items={sheetItems}
          query={sheetQuery}
          position={sheetAutocompletePosition}
          onSelect={(item) => handleInsertSheetItem(item, sheetTriggerStartIndex)}
          selectedIndex={sheetSelectedIndex}
        />
      )}

      {/* Unsaved changes navigation warning */}
      {warnOnUnsavedChanges && (
        <UnsavedChangesGuard hasContent={value.trim().length > 0} />
      )}
    </div>
  );
});

