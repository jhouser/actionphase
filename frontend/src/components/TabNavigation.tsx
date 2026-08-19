import type { ReactNode } from 'react';
import { useState, useRef, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useTabOverflow } from '../hooks/useTabOverflow';

/** Height of the global nav in Layout.tsx (h-16). Sticky bars park below it. */
const NAVBAR_HEIGHT_PX = 64;

/**
 * CSS `top` value for elements that must stick below both the global nav and a
 * sticky game tab bar. Falls back to just the navbar when no tab bar is pinned
 * above (e.g. standalone pages), since --game-tabbar-h is only set while one
 * is mounted.
 */
export const STICKY_BELOW_TABS = `calc(${NAVBAR_HEIGHT_PX}px + var(--game-tabbar-h, 0px))`;

export interface Tab {
  id: string;
  label: string;
  badge?: number | string;
  icon?: ReactNode;
}

interface TabNavigationProps {
  tabs: Tab[];
  activeTab: string;
  onTabChange: (tabId: string) => void;
  /** When provided, tabs render as <a> links for right-click / middle-click / Cmd+click support */
  getTabHref?: (tabId: string) => string;
  /**
   * Collapse tabs that do not fit into a "More" dropdown, measured from the
   * real rendered width rather than a fixed list. Opt-in because the bar must
   * render every tab for one frame to measure them, which is wasted work for
   * callers whose tabs always fit.
   */
  collapseOverflow?: boolean;
  /**
   * Pin the bar below the global nav (h-16) so tabs stay reachable while scrolling.
   * Once pinned the bar condenses (smaller padding/type) to limit how much
   * vertical space it permanently costs, which matters most on phone viewports.
   */
  sticky?: boolean;
  /**
   * Hold navigation, greying the tabs out. For surfaces where leaving the current tab
   * would destroy uncommitted work — the character sheet unmounts an open editor on
   * switch — so the move has to be finished or cancelled first. Pair with a visible
   * explanation; a disabled control that does not say why just reads as broken.
   */
  disabled?: boolean;
}

/**
 * TabNavigation - Responsive tab component with dropdown on mobile
 *
 * Desktop: Horizontal tab bar with icons and labels
 * Mobile: Dropdown select menu for better space utilization
 */
export function TabNavigation({
  tabs,
  activeTab,
  onTabChange,
  getTabHref,
  collapseOverflow = false,
  sticky = false,
  disabled = false,
}: TabNavigationProps) {
  const [moreOpen, setMoreOpen] = useState(false);
  const moreRef = useRef<HTMLDivElement>(null);
  const sentinelRef = useRef<HTMLDivElement>(null);
  const [isPinned, setIsPinned] = useState(false);

  // A zero-height sentinel sits directly above the bar. Once it scrolls out of
  // view the bar has reached its sticky offset, which is cheaper to detect than
  // measuring scroll position on every frame.
  useEffect(() => {
    if (!sticky) return;
    const el = sentinelRef.current;
    if (!el || typeof IntersectionObserver === 'undefined') return;
    const observer = new IntersectionObserver(
      ([entry]) => setIsPinned(!entry.isIntersecting),
      { rootMargin: `-${NAVBAR_HEIGHT_PX}px 0px 0px 0px`, threshold: 0 }
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [sticky]);

  const condensed = sticky && isPinned;

  // Publish the bar's height so nested sticky elements (conversation headers,
  // editor tab bars) park below it rather than underneath it.
  //
  // This assumes at most one sticky bar is mounted at a time, since the height
  // lives in a single global CSS variable. That holds today: only the game tab
  // bar passes `sticky`, and the nested TabNavigation in CharacterSheet does
  // not. Two concurrent sticky bars would fight over the variable — if that
  // becomes necessary, this needs a registry keyed by mounted bar rather than
  // a lone custom property.
  const barRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!sticky) return;
    const el = barRef.current;
    if (!el) return;
    const publish = () => {
      document.documentElement.style.setProperty(
        '--game-tabbar-h',
        `${el.getBoundingClientRect().height}px`
      );
    };
    publish();
    const ro = typeof ResizeObserver !== 'undefined' ? new ResizeObserver(publish) : null;
    ro?.observe(el);
    return () => {
      ro?.disconnect();
      document.documentElement.style.removeProperty('--game-tabbar-h');
    };
  }, [sticky]);

  useEffect(() => {
    if (!moreOpen) return;
    const handler = (e: MouseEvent) => {
      if (moreRef.current && !moreRef.current.contains(e.target as Node)) {
        setMoreOpen(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [moreOpen]);

  const { containerRef, moreButtonRef, registerTab, overflowIds, measured } = useTabOverflow({
    tabs,
    activeTab,
    enabled: collapseOverflow,
  });

  // Overflowing tabs stay mounted and are hidden with CSS instead of being
  // unmounted: an unmounted tab has no width, so dropping them from the DOM
  // would erase the measurements that decide whether they should come back when
  // the viewport widens.
  const overflowTabs = tabs.filter(t => overflowIds.has(t.id));
  const overflowActive = overflowTabs.some(t => t.id === activeTab);

  // Before the first measurement every tab renders so it can be measured. The
  // bar is held invisible for that frame — otherwise the full list paints and
  // then visibly snaps down to the fitted set.
  const awaitingMeasurement = collapseOverflow && !measured;

  const renderTabContent = (tab: Tab, isActive: boolean) => (
    <>
      {tab.icon && <span className="flex-shrink-0">{tab.icon}</span>}
      <span>{tab.label}</span>
      {tab.badge !== undefined && (
        <span
          className={`
            ml-2 py-0.5 px-2 rounded-full text-xs font-medium
            ${isActive
              ? 'bg-semantic-info-subtle text-content-primary'
              : 'surface-raised text-content-secondary'
            }
          `}
        >
          {tab.badge}
        </span>
      )}
    </>
  );

  const tabClassName = (isActive: boolean) => `
    whitespace-nowrap px-4 border-b-2 font-medium text-sm flex items-center gap-2
    ${condensed ? 'py-1.5' : 'py-3'}
    transition-[padding,color,border-color] duration-200
    ${disabled ? 'opacity-50 cursor-not-allowed' : ''}
    ${isActive
      ? 'border-interactive-primary text-interactive-primary'
      : disabled
        ? 'border-transparent text-content-secondary'
        : 'border-transparent text-content-secondary hover:text-content-primary hover:border-theme-default'
    }
  `;

  const renderTab = (tab: Tab) => {
    const isActive = activeTab === tab.id;
    const isOverflowing = overflowIds.has(tab.id);
    const sharedProps = {
      role: 'tab' as const,
      className: tabClassName(isActive),
      'aria-selected': isActive,
      'aria-current': isActive ? ('page' as const) : undefined,
      // Hidden from assistive tech and from pointer events while collapsed —
      // the dropdown copy is the reachable one, so exposing both would announce
      // every overflowing tab twice.
      'aria-hidden': isOverflowing || undefined,
      inert: isOverflowing || undefined,
      // `hidden` would remove it from layout and zero its width; visibility
      // keeps the box measurable while taking it out of view.
      style: isOverflowing
        ? ({ visibility: 'hidden', position: 'absolute', pointerEvents: 'none' } as const)
        : undefined,
      'data-testid': `tab-${tab.id}`,
      'data-overflowing': isOverflowing || undefined,
      ref: (el: HTMLElement | null) => registerTab(tab.id, el),
    };
    // A disabled Link still navigates on click, so the href is dropped entirely rather
    // than rendered inert — an anchor with no href is not a link.
    return getTabHref && !disabled ? (
      <Link key={tab.id} {...sharedProps} to={getTabHref(tab.id)}>
        {renderTabContent(tab, isActive)}
      </Link>
    ) : (
      <button
        key={tab.id}
        {...sharedProps}
        // Without this a tab bar rendered inside a <form> submits it on every
        // tab click, since <button> defaults to type="submit". A tab is never a
        // submit button.
        type="button"
        disabled={disabled}
        onClick={() => onTabChange(tab.id)}
      >
        {renderTabContent(tab, isActive)}
      </button>
    );
  };

  return (
    <>
      {sticky && <div ref={sentinelRef} aria-hidden="true" className="h-0" />}
      <div
        ref={barRef}
        className={`
          border-b border-theme-default surface-base md:rounded-t-lg
          ${sticky ? 'sticky top-16 z-30 shadow-sm' : ''}
          ${condensed ? 'md:rounded-t-none' : ''}
        `}
        data-pinned={sticky ? isPinned : undefined}
      >
      {/* Mobile: Dropdown Select */}
      <div className="md:hidden relative">
        <label htmlFor="tab-select" className="sr-only">
          Select a tab
        </label>
        <select
          id="tab-select"
          value={activeTab}
          disabled={disabled}
          onChange={(e) => onTabChange(e.target.value)}
          className={`
            block w-full pl-2 pr-10 font-semibold surface-raised text-content-primary
            border border-border-primary md:rounded-t-lg shadow-sm appearance-none cursor-pointer
            focus:outline-none focus:ring-2 focus:ring-interactive-primary focus:border-interactive-primary
            transition-all duration-200
            disabled:opacity-50 disabled:cursor-not-allowed
            ${condensed ? 'py-1.5 text-sm' : 'py-3 text-base'}
          `}
          style={{ backgroundImage: 'none' }}
        >
          {tabs.map((tab) => (
            <option key={tab.id} value={tab.id}>
              {tab.label}
              {tab.badge !== undefined ? ` (${tab.badge})` : ''}
            </option>
          ))}
        </select>
        {/* Dropdown chevron icon */}
        <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
          <svg className="h-5 w-5 text-content-secondary" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
            <path fillRule="evenodd" d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z" clipRule="evenodd" />
          </svg>
        </div>
      </div>

      {/* Desktop: Horizontal Tab Bar */}
      <div
        className={`hidden md:flex -mb-px ${awaitingMeasurement ? 'invisible' : ''}`}
      >
        <nav
          ref={containerRef}
          className={`flex ${collapseOverflow ? 'flex-1 min-w-0 relative' : 'overflow-x-auto'}`}
          role="tablist"
          aria-label="Tabs"
        >
          {tabs.map(renderTab)}
        </nav>

        {overflowTabs.length > 0 && (
          <div ref={moreRef} className="relative flex-shrink-0">
            <button
              ref={moreButtonRef}
              className={`
                whitespace-nowrap px-4 font-medium text-sm flex items-center gap-2
                transition-[padding,color,border-color] duration-200
                border border-t border-x rounded-t-lg -mb-px
                ${condensed
                  ? 'pt-1.5 pb-[calc(0.375rem+1px)]'
                  : 'pt-3 pb-[calc(0.75rem+1px)]'
                }
                ${moreOpen
                  ? 'surface-raised border-t-border-primary border-x-border-primary border-b-transparent'
                  : 'border-transparent'
                }
                ${overflowActive
                  ? 'text-interactive-primary'
                  : 'text-content-secondary hover:text-content-primary'
                }
              `}
              type="button"
              disabled={disabled}
              onClick={() => setMoreOpen(o => !o)}
              aria-haspopup="true"
              aria-expanded={moreOpen}
              data-testid="tab-more"
            >
              <span>More</span>
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={moreOpen ? 'M5 15l7-7 7 7' : 'M19 9l-7 7-7-7'} />
              </svg>
            </button>

            {/* Gated on `disabled` as well as `moreOpen`: the panel can already be open
                when navigation locks, and its items would otherwise still navigate,
                unmounting the very editor the lock protects. */}
            {moreOpen && !disabled && (
              <div className="absolute right-0 top-full z-40 min-w-[160px] surface-raised border border-border-primary rounded-b-lg rounded-tl-lg shadow-lg py-1">
                {overflowTabs.map((tab) => {
                  const isActive = activeTab === tab.id;
                  const itemClass = `w-full text-left px-4 py-2 text-sm flex items-center gap-2 transition-colors
                    ${isActive
                      ? 'text-interactive-primary bg-semantic-info-subtle'
                      : 'text-content-primary hover:bg-bg-secondary'
                    }`;
                  const handleClick = () => { onTabChange(tab.id); setMoreOpen(false); };
                  return getTabHref ? (
                    <Link
                      key={tab.id}
                      to={getTabHref(tab.id)}
                      className={itemClass}
                      onClick={() => setMoreOpen(false)}
                      data-testid={`tab-${tab.id}-overflow`}
                    >
                      {renderTabContent(tab, isActive)}
                    </Link>
                  ) : (
                    <button
                      key={tab.id}
                      type="button"
                      className={itemClass}
                      onClick={handleClick}
                      data-testid={`tab-${tab.id}-overflow`}
                    >
                      {renderTabContent(tab, isActive)}
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        )}
      </div>
      </div>
    </>
  );
}
