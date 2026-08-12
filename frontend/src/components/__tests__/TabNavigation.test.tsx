import { describe, it, expect } from 'vitest';
import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { TabNavigation, type Tab } from '../TabNavigation';

const getTabHref = (tabId: string) => `?tab=${tabId}`;

const renderWithRouter = (ui: React.ReactElement) =>
  render(<MemoryRouter>{ui}</MemoryRouter>);

describe('TabNavigation', () => {
  const mockOnTabChange = vi.fn();

  const mockTabs: Tab[] = [
    { id: 'tab1', label: 'First Tab' },
    { id: 'tab2', label: 'Second Tab', badge: 5 },
    { id: 'tab3', label: 'Third Tab', icon: <span>📝</span> },
    { id: 'tab4', label: 'Fourth Tab', badge: 'New', icon: <span>🔔</span> },
  ];

  beforeEach(() => {
    mockOnTabChange.mockClear();
  });

  describe('Desktop View', () => {
    it('renders all tabs as buttons on desktop', () => {
      render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />
      );

      // All tabs should be rendered as buttons (desktop view uses role="tab")
      expect(screen.getByRole('tab', { name: /First Tab/i })).toBeInTheDocument();
      expect(screen.getByRole('tab', { name: /Second Tab/i })).toBeInTheDocument();
      expect(screen.getByRole('tab', { name: /Third Tab/i })).toBeInTheDocument();
      expect(screen.getByRole('tab', { name: /Fourth Tab/i })).toBeInTheDocument();
    });

    it('displays badges for tabs that have them', () => {
      render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />
      );

      // Badge for tab2 (numeric)
      expect(screen.getByText('5')).toBeInTheDocument();
      // Badge for tab4 (string)
      expect(screen.getByText('New')).toBeInTheDocument();
    });

    it('displays icons for tabs that have them', () => {
      const { container } = render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />
      );

      // Icons should be rendered (checking for the emoji text)
      expect(container.innerHTML).toContain('📝');
      expect(container.innerHTML).toContain('🔔');
    });

    it('applies active styling to the current tab', () => {
      render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab2"
          onTabChange={mockOnTabChange}
        />
      );

      const activeTab = screen.getByRole('tab', { name: /Second Tab/i });

      // Check that aria-selected is true
      expect(activeTab).toHaveAttribute('aria-selected', 'true');
      expect(activeTab).toHaveAttribute('aria-current', 'page');

      // Check for active styling classes
      expect(activeTab.className).toContain('border-interactive-primary');
      expect(activeTab.className).toContain('text-interactive-primary');
    });

    it('applies inactive styling to non-active tabs', () => {
      render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />
      );

      const inactiveTab = screen.getByRole('tab', { name: /Second Tab/i });

      // Check that aria-selected is false (implicit when not true)
      expect(inactiveTab).toHaveAttribute('aria-selected', 'false');
      expect(inactiveTab).not.toHaveAttribute('aria-current');

      // Check for inactive styling classes
      expect(inactiveTab.className).toContain('border-transparent');
      expect(inactiveTab.className).toContain('text-content-secondary');
    });

    it('calls onTabChange when a tab is clicked', async () => {
      const user = userEvent.setup();

      render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />
      );

      const tab2 = screen.getByRole('tab', { name: /Second Tab/i });
      await user.click(tab2);

      expect(mockOnTabChange).toHaveBeenCalledTimes(1);
      expect(mockOnTabChange).toHaveBeenCalledWith('tab2');
    });

    it('has correct testid attributes for tabs', () => {
      render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />
      );

      expect(screen.getByTestId('tab-tab1')).toBeInTheDocument();
      expect(screen.getByTestId('tab-tab2')).toBeInTheDocument();
      expect(screen.getByTestId('tab-tab3')).toBeInTheDocument();
      expect(screen.getByTestId('tab-tab4')).toBeInTheDocument();
    });
  });

  describe('Mobile View (Dropdown)', () => {
    it('renders a select dropdown for mobile', () => {
      render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />
      );

      // Select should be present (hidden on desktop, visible on mobile via CSS)
      const select = screen.getByLabelText('Select a tab');
      expect(select).toBeInTheDocument();
      expect(select.tagName).toBe('SELECT');
    });

    it('displays all tabs as options in the dropdown', () => {
      render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />
      );

      // All tabs should be options
      expect(screen.getByRole('option', { name: 'First Tab' })).toBeInTheDocument();
      expect(screen.getByRole('option', { name: /Second Tab/ })).toBeInTheDocument();
      expect(screen.getByRole('option', { name: /Third Tab/ })).toBeInTheDocument();
      expect(screen.getByRole('option', { name: /Fourth Tab/ })).toBeInTheDocument();
    });

    it('includes badges in option labels', () => {
      render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />
      );

      // Badge should be included in the text
      expect(screen.getByRole('option', { name: 'Second Tab (5)' })).toBeInTheDocument();
      expect(screen.getByRole('option', { name: 'Fourth Tab (New)' })).toBeInTheDocument();
    });

    it('sets the selected option to the active tab', () => {
      render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab2"
          onTabChange={mockOnTabChange}
        />
      );

      const select = screen.getByLabelText('Select a tab') as HTMLSelectElement;
      expect(select.value).toBe('tab2');

      const selectedOption = screen.getByRole('option', { name: /Second Tab/ }) as HTMLOptionElement;
      expect(selectedOption.selected).toBe(true);
    });

    it('calls onTabChange when dropdown selection changes', async () => {
      const user = userEvent.setup();

      render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />
      );

      const select = screen.getByLabelText('Select a tab');
      await user.selectOptions(select, 'tab3');

      expect(mockOnTabChange).toHaveBeenCalledTimes(1);
      expect(mockOnTabChange).toHaveBeenCalledWith('tab3');
    });

    it('has accessible label for the dropdown', () => {
      render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />
      );

      // Label should exist (even if visually hidden with sr-only)
      const label = screen.getByText('Select a tab');
      expect(label).toBeInTheDocument();
      expect(label.tagName).toBe('LABEL');
      expect(label.className).toContain('sr-only');
    });
  });

  describe('Responsive Behavior', () => {
    it('applies correct responsive classes for mobile dropdown', () => {
      const { container } = render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />
      );

      // Mobile dropdown container should have md:hidden
      const dropdownContainer = container.querySelector('.md\\:hidden');
      expect(dropdownContainer).toBeInTheDocument();
    });

    it('applies correct responsive classes for desktop tabs', () => {
      const { container } = render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />
      );

      // Desktop wrapper div should have hidden md:flex
      const desktopWrapper = container.querySelector('.md\\:flex');
      expect(desktopWrapper).toBeInTheDocument();
      expect(desktopWrapper?.className).toContain('hidden');
    });
  });

  describe('Overflow tabs (More menu)', () => {
    const allTabs: Tab[] = [
      { id: 'tab1', label: 'First Tab' },
      { id: 'tab2', label: 'Second Tab' },
      { id: 'tab3', label: 'Third Tab' },
      { id: 'info', label: 'Game Info' },
      { id: 'logs', label: 'Game Logs' },
    ];

    // jsdom gives every element a width of 0, so overflow could never trigger on
    // its own. These helpers install a fake layout: each tab is TAB_WIDTH wide
    // and the nav is whatever the test says, which is what lets us assert on the
    // fitting maths rather than on a hard-coded list of ids.
    const TAB_WIDTH = 100;
    const MORE_WIDTH = 90;

    /**
     * @param navWidth Available width for the tab strip, in px.
     */
    function stubLayout(navWidth: number) {
      const original = HTMLElement.prototype.getBoundingClientRect;
      HTMLElement.prototype.getBoundingClientRect = function (this: HTMLElement) {
        const testid = this.getAttribute('data-testid');
        let width = 0;
        if (this.getAttribute('role') === 'tablist') {
          width = navWidth;
        } else if (testid === 'tab-more') {
          width = MORE_WIDTH;
        } else if (testid?.startsWith('tab-')) {
          width = TAB_WIDTH;
        }
        return { width, height: 0, top: 0, left: 0, right: width, bottom: 0, x: 0, y: 0, toJSON: () => ({}) } as DOMRect;
      };
      return () => {
        HTMLElement.prototype.getBoundingClientRect = original;
      };
    }

    let restoreLayout: (() => void) | undefined;

    afterEach(() => {
      restoreLayout?.();
      restoreLayout = undefined;
    });

    /** Ids of the tabs actually visible in the bar (not collapsed, not the dropdown copies). */
    const visibleTabIds = () =>
      Array.from(document.querySelectorAll('[role="tab"]'))
        .filter(el => !el.hasAttribute('data-overflowing'))
        .map(el => el.getAttribute('data-testid')?.replace(/^tab-/, ''));

    it('keeps every tab in the bar when they all fit', () => {
      restoreLayout = stubLayout(5 * TAB_WIDTH);
      renderWithRouter(
        <TabNavigation tabs={allTabs} activeTab="tab1" onTabChange={mockOnTabChange} collapseOverflow />
      );

      expect(visibleTabIds()).toEqual(['tab1', 'tab2', 'tab3', 'info', 'logs']);
      expect(screen.queryByTestId('tab-more')).not.toBeInTheDocument();
    });

    it('collapses tabs from the end when the bar is too narrow', () => {
      // Room for the More button (90) + 3 tabs (300) = 390 of 400.
      restoreLayout = stubLayout(400);
      renderWithRouter(
        <TabNavigation tabs={allTabs} activeTab="tab1" onTabChange={mockOnTabChange} collapseOverflow />
      );

      expect(visibleTabIds()).toEqual(['tab1', 'tab2', 'tab3']);
      expect(screen.getByTestId('tab-more')).toBeInTheDocument();
    });

    it('collapses more tabs as the available width shrinks', () => {
      restoreLayout = stubLayout(300); // More (90) + 2 tabs (200) = 290
      renderWithRouter(
        <TabNavigation tabs={allTabs} activeTab="tab1" onTabChange={mockOnTabChange} collapseOverflow />
      );

      expect(visibleTabIds()).toEqual(['tab1', 'tab2']);
    });

    it('keeps the active tab visible even when its position would collapse it', () => {
      restoreLayout = stubLayout(300);
      renderWithRouter(
        <TabNavigation tabs={allTabs} activeTab="logs" onTabChange={mockOnTabChange} collapseOverflow />
      );

      // 'logs' is last and would normally collapse first; it is pinned instead,
      // displacing 'tab2' which would otherwise have fit.
      const visible = visibleTabIds();
      expect(visible).toContain('logs');
      expect(visible).toEqual(['tab1', 'logs']);
    });

    it('shows collapsed tabs in the More dropdown when opened', async () => {
      const user = userEvent.setup();
      restoreLayout = stubLayout(400);
      renderWithRouter(
        <TabNavigation tabs={allTabs} activeTab="tab1" onTabChange={mockOnTabChange} collapseOverflow />
      );

      expect(screen.queryByTestId('tab-info-overflow')).not.toBeInTheDocument();

      await user.click(screen.getByTestId('tab-more'));

      expect(screen.getByTestId('tab-info-overflow')).toBeInTheDocument();
      expect(screen.getByTestId('tab-logs-overflow')).toBeInTheDocument();
    });

    it('calls onTabChange and closes dropdown when a collapsed tab is clicked', async () => {
      const user = userEvent.setup();
      restoreLayout = stubLayout(400);
      render(
        <TabNavigation tabs={allTabs} activeTab="tab1" onTabChange={mockOnTabChange} collapseOverflow />
      );

      await user.click(screen.getByTestId('tab-more'));
      await user.click(screen.getByTestId('tab-logs-overflow'));

      expect(mockOnTabChange).toHaveBeenCalledWith('logs');
      expect(screen.queryByTestId('tab-logs-overflow')).not.toBeInTheDocument();
    });

    it('closes dropdown when clicking outside', async () => {
      const user = userEvent.setup();
      restoreLayout = stubLayout(400);
      render(
        <TabNavigation tabs={allTabs} activeTab="tab1" onTabChange={mockOnTabChange} collapseOverflow />
      );

      await user.click(screen.getByTestId('tab-more'));
      expect(screen.getByTestId('tab-info-overflow')).toBeInTheDocument();

      await user.click(document.body);
      expect(screen.queryByTestId('tab-info-overflow')).not.toBeInTheDocument();
    });

    it('hides collapsed tabs from assistive tech so they are not announced twice', () => {
      restoreLayout = stubLayout(400);
      renderWithRouter(
        <TabNavigation tabs={allTabs} activeTab="tab1" onTabChange={mockOnTabChange} collapseOverflow />
      );

      expect(screen.getByTestId('tab-logs')).toHaveAttribute('aria-hidden', 'true');
      expect(screen.getByTestId('tab-tab1')).not.toHaveAttribute('aria-hidden');
    });

    it('does not collapse anything when collapseOverflow is not set', () => {
      restoreLayout = stubLayout(100); // far too narrow, but opt-in is off
      renderWithRouter(
        <TabNavigation tabs={allTabs} activeTab="tab1" onTabChange={mockOnTabChange} />
      );

      expect(visibleTabIds()).toEqual(['tab1', 'tab2', 'tab3', 'info', 'logs']);
      expect(screen.queryByTestId('tab-more')).not.toBeInTheDocument();
    });

    it('includes every tab in the mobile select regardless of collapsing', () => {
      restoreLayout = stubLayout(300);
      renderWithRouter(
        <TabNavigation tabs={allTabs} activeTab="tab1" onTabChange={mockOnTabChange} collapseOverflow />
      );

      expect(screen.getByRole('option', { name: 'Game Info' })).toBeInTheDocument();
      expect(screen.getByRole('option', { name: 'Game Logs' })).toBeInTheDocument();
    });
  });

  describe('Link behavior (getTabHref)', () => {
    it('renders tabs as <a> elements when getTabHref is provided', () => {
      renderWithRouter(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
          getTabHref={getTabHref}
        />
      );

      const tab1 = screen.getByRole('tab', { name: /First Tab/i });
      expect(tab1.tagName).toBe('A');
      // Link resolves relative to current location — href will include the path
      expect(tab1.getAttribute('href')).toContain('tab=tab1');
    });

    it('renders tabs as <button> elements when getTabHref is not provided', () => {
      render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />
      );

      const tab1 = screen.getByRole('tab', { name: /First Tab/i });
      expect(tab1.tagName).toBe('BUTTON');
      expect(tab1).not.toHaveAttribute('href');
    });

    it('each tab has correct href matching its id', () => {
      renderWithRouter(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
          getTabHref={getTabHref}
        />
      );

      mockTabs.forEach(tab => {
        const tabEl = screen.getByRole('tab', { name: new RegExp(tab.label) });
        // Link resolves relative to current location — href will include the path
        expect(tabEl.getAttribute('href')).toContain(`tab=${tab.id}`);
      });
    });
  });

  describe('Edge Cases', () => {
    it('handles single tab', () => {
      const singleTab: Tab[] = [{ id: 'only', label: 'Only Tab' }];

      render(
        <TabNavigation
          tabs={singleTab}
          activeTab="only"
          onTabChange={mockOnTabChange}
        />
      );

      expect(screen.getByRole('tab', { name: 'Only Tab' })).toBeInTheDocument();
      expect(screen.getByRole('option', { name: 'Only Tab' })).toBeInTheDocument();
    });

    it('handles tabs with no badges or icons', () => {
      const simpleTabs: Tab[] = [
        { id: 'simple1', label: 'Simple One' },
        { id: 'simple2', label: 'Simple Two' },
      ];

      render(
        <TabNavigation
          tabs={simpleTabs}
          activeTab="simple1"
          onTabChange={mockOnTabChange}
        />
      );

      expect(screen.getByRole('tab', { name: 'Simple One' })).toBeInTheDocument();
      expect(screen.getByRole('tab', { name: 'Simple Two' })).toBeInTheDocument();
    });

    it('handles badge value of 0', () => {
      const tabsWithZero: Tab[] = [
        { id: 'zero', label: 'Zero Badge', badge: 0 },
      ];

      render(
        <TabNavigation
          tabs={tabsWithZero}
          activeTab="zero"
          onTabChange={mockOnTabChange}
        />
      );

      // Badge with 0 should still be displayed
      expect(screen.getByText('0')).toBeInTheDocument();
      expect(screen.getByRole('option', { name: 'Zero Badge (0)' })).toBeInTheDocument();
    });

    it('handles long tab labels gracefully', () => {
      const longLabelTabs: Tab[] = [
        { id: 'long', label: 'This is a very long tab label that might wrap on mobile devices' },
      ];

      render(
        <TabNavigation
          tabs={longLabelTabs}
          activeTab="long"
          onTabChange={mockOnTabChange}
        />
      );

      // Should render without errors
      expect(screen.getByRole('tab', { name: /This is a very long/ })).toBeInTheDocument();
    });
  });

  describe('Accessibility', () => {
    it('has proper ARIA attributes on tablist', () => {
      render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />
      );

      const tablist = screen.getByRole('tablist');
      expect(tablist).toHaveAttribute('aria-label', 'Tabs');
    });

    it('has proper role attributes on tab buttons', () => {
      render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />
      );

      mockTabs.forEach(tab => {
        const tabButton = screen.getByRole('tab', { name: new RegExp(tab.label) });
        expect(tabButton).toHaveAttribute('role', 'tab');
      });
    });

    it('properly associates label with select element', () => {
      render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
        />
      );

      const label = screen.getByText('Select a tab');
      const select = screen.getByLabelText('Select a tab');

      expect(label).toHaveAttribute('for', 'tab-select');
      expect(select).toHaveAttribute('id', 'tab-select');
    });
  });

  describe('Sticky mode', () => {
    // jsdom ships no IntersectionObserver. Stub it so we can drive the pinned
    // state directly instead of trying to simulate scrolling.
    let triggerIntersection: ((isIntersecting: boolean) => void) | undefined;

    beforeEach(() => {
      triggerIntersection = undefined;
      vi.stubGlobal(
        'IntersectionObserver',
        class {
          constructor(cb: (entries: { isIntersecting: boolean }[]) => void) {
            triggerIntersection = (isIntersecting: boolean) =>
              act(() => cb([{ isIntersecting }]));
          }
          observe() {}
          disconnect() {}
        }
      );
    });

    afterEach(() => {
      vi.unstubAllGlobals();
      document.documentElement.style.removeProperty('--game-tabbar-h');
    });

    const renderSticky = (sticky: boolean) =>
      render(
        <TabNavigation
          tabs={mockTabs}
          activeTab="tab1"
          onTabChange={mockOnTabChange}
          sticky={sticky}
        />
      );

    it('does not pin the bar unless sticky is requested', () => {
      const { container } = renderSticky(false);

      const bar = container.querySelector('[data-pinned]');
      expect(bar).toBeNull();
      expect(container.querySelector('.sticky')).toBeNull();
    });

    it('pins the bar below the navbar when sticky', () => {
      const { container } = renderSticky(true);

      const bar = container.querySelector('[data-pinned]');
      expect(bar).toHaveClass('sticky', 'top-16');
      // Must stay below the global nav (z-50) so it cannot cover it.
      expect(bar).toHaveClass('z-30');
    });

    it('starts uncondensed and condenses once the bar reaches the navbar', () => {
      const { container } = renderSticky(true);
      const bar = container.querySelector('[data-pinned]')!;
      const firstTab = screen.getByRole('tab', { name: /First Tab/i });

      // Sentinel still visible => bar has not reached its offset yet.
      expect(bar).toHaveAttribute('data-pinned', 'false');
      expect(firstTab).toHaveClass('py-3');

      triggerIntersection!(false);

      expect(bar).toHaveAttribute('data-pinned', 'true');
      expect(firstTab).toHaveClass('py-1.5');
    });

    it('restores full height when scrolled back to the top', () => {
      const { container } = renderSticky(true);
      const bar = container.querySelector('[data-pinned]')!;

      triggerIntersection!(false);
      expect(bar).toHaveAttribute('data-pinned', 'true');

      triggerIntersection!(true);
      expect(bar).toHaveAttribute('data-pinned', 'false');
      expect(screen.getByRole('tab', { name: /First Tab/i })).toHaveClass('py-3');
    });

    it('publishes its height so nested sticky headers can clear it', () => {
      const { unmount } = renderSticky(true);

      // jsdom reports every height as 0, so the value itself proves nothing
      // about real measurement — only that the variable is published at all.
      expect(
        document.documentElement.style.getPropertyValue('--game-tabbar-h')
      ).not.toBe('');

      // Leaving the page must not leave a stale offset behind for other pages.
      unmount();
      expect(
        document.documentElement.style.getPropertyValue('--game-tabbar-h')
      ).toBe('');
    });

    it('does not publish a height when not sticky', () => {
      renderSticky(false);

      expect(
        document.documentElement.style.getPropertyValue('--game-tabbar-h')
      ).toBe('');
    });
  });
});
