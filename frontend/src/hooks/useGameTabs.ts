import { useState, useEffect, useMemo, createElement, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import type { Tab } from '../components/TabNavigation';
import type { GameState } from '../types/games';

interface UseGameTabsOptions {
  gameState: GameState | undefined;
  isGM: boolean;
  participantCount: number;
  currentPhaseType?: string;
  isPhaseLoading?: boolean;
  isAudience?: boolean;
  isParticipant?: boolean;
  hasCharacters?: boolean;
  unvotedPollsCount?: number;
  hasSubmittedAction?: boolean;
  isRoleLoading?: boolean;
}

// Icon helper to avoid JSX in .ts file
const createIcon = (path: string) =>
  createElement('svg', {
    className: 'w-4 h-4',
    fill: 'none',
    stroke: 'currentColor',
    viewBox: '0 0 24 24',
  }, createElement('path', {
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
    strokeWidth: 2,
    d: path,
  }));

const icons = {
  applications: createIcon('M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z'),
  info: createIcon('M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z'),
  people: createIcon('M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z'),
  commonRoom: createIcon('M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z'),
  polls: createIcon('M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01'),
  phases: createIcon('M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z'),
  actions: createIcon('M13 10V3L4 14h7v7l9-11h-7z'),
  messages: createIcon('M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z'),
  history: createIcon('M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z'),
  audience: createIcon('M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z'),
  handouts: createIcon('M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253'),
  stats: createIcon('M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z'),
};

export function useGameTabs({
  gameState,
  isGM,
  participantCount,
  currentPhaseType,
  isPhaseLoading = false,
  isAudience = false,
  isParticipant = false,
  hasCharacters = false,
  unvotedPollsCount = 0,
  hasSubmittedAction = false,
  isRoleLoading = false,
}: UseGameTabsOptions) {
  const [searchParams, setSearchParams] = useSearchParams();
  const tabParam = searchParams.get('tab');
  const [activeTab, setActiveTab] = useState<string>(tabParam || 'default');
  const hasSetInitialTab = useRef(false); // Track if we've set the initial tab

  // Phase-aware tab configuration
  const tabs: Tab[] = useMemo(() => {
    const tabList: Tab[] = [];

    if (!gameState) {
      return tabList; // Game not loaded yet — return empty so effect waits
    }

    if (gameState === 'recruitment') {
      if (isGM) {
        tabList.push({ id: 'applications', label: 'Applications', icon: icons.applications });
      }
      // Handouts - players can review game materials while applying
      tabList.push({ id: 'handouts', label: 'Handouts', icon: icons.handouts });
      // Note: During recruitment, applicants are shown in the Game Info tab
      // No "Participants" tab since there are no confirmed participants yet
      tabList.push({ id: 'info', label: 'Game Info', icon: icons.info });
    } else if (gameState === 'character_creation') {
      // People tab (combines Characters and Participants)
      tabList.push({ id: 'people', label: 'People', badge: participantCount, icon: icons.people });
      // Note: Applications tab removed - recruitment is closed, no new applications accepted
      // Handouts - players can reference materials while creating characters
      tabList.push({ id: 'handouts', label: 'Handouts', icon: icons.handouts });
    } else if (gameState === 'in_progress') {
      // Common Room tab - only during common_room phases
      // Badge shows unvoted polls count (polls are integrated as sub-tab within Common Room)
      if (currentPhaseType === 'common_room') {
        tabList.push({
          id: 'common-room',
          label: 'Common Room',
          icon: icons.commonRoom,
          badge: unvotedPollsCount > 0 ? unvotedPollsCount : undefined
        });
      }

      // Phases tab (GM only)
      if (isGM) {
        tabList.push({ id: 'phases', label: 'Phases', icon: icons.phases });
      }

      // Actions tab - Visible during action phases to:
      // 1. GM (can see all actions and manage)
      // 2. Regular participants (can submit actions)
      // Note: Audience members view action submissions and results on the
      // History tab, which serves every role (see HistoryView).
      if (currentPhaseType === 'action' && (isGM || isParticipant)) {
        const label = isGM ? 'Actions' : hasSubmittedAction ? 'Action Submitted ✓' : 'Submit Action';
        tabList.push({ id: 'actions', label, icon: icons.actions });
      }

      // Messages - Only visible to:
      // 1. GM (always)
      // 2. Regular participants (players)
      // 3. Audience members WITH assigned NPCs
      const canSeeMessages = isGM || isParticipant || (isAudience && hasCharacters);
      if (canSeeMessages) {
        tabList.push({ id: 'messages', label: 'Messages', icon: icons.messages });
      }

      // People tab (combines Characters and Participants)
      tabList.push({ id: 'people', label: 'People', badge: participantCount, icon: icons.people });

      // Handouts - available to all participants
      tabList.push({ id: 'handouts', label: 'Handouts', icon: icons.handouts });

      // Audience tab (GM and audience members only)
      if (isGM || isAudience) {
        tabList.push({ id: 'audience', label: 'Audience', icon: icons.audience });
      }

      // History - context-aware label
      tabList.push({ id: 'history', label: 'History', icon: icons.history });

      // Game Info - always available
      tabList.push({ id: 'info', label: 'Game Info', icon: icons.info });
    } else if (gameState === 'completed' || gameState === 'cancelled') {
      // Post-game tabs - read-only archive view
      tabList.push({ id: 'history', label: 'History', icon: icons.history });

      // People tab (combines Characters and Participants) - same as in-progress games
      tabList.push({ id: 'people', label: 'People', badge: participantCount, icon: icons.people });

      // Stats - completed games only. Cancelled games are excluded because the
      // stats endpoint serves completed games exclusively (a cancelled game is
      // not a public archive), so the tab would only ever show an error.
      if (gameState === 'completed') {
        tabList.push({ id: 'stats', label: 'Stats', icon: icons.stats });
      }

      // Handouts - available to view historical handouts
      tabList.push({ id: 'handouts', label: 'Handouts', icon: icons.handouts });

      // Audience tab
      // - Completed games: Always visible (public archive, anyone can view audience posts)
      // - Cancelled games: Only visible to GM/audience (game remains private)
      if (gameState === 'completed' || isGM || isAudience) {
        tabList.push({ id: 'audience', label: 'Audience', icon: icons.audience });
      }

      tabList.push({ id: 'info', label: 'Game Info', icon: icons.info });
    } else {
      // Setup state - allow GM to prepare content before recruitment
      tabList.push({ id: 'handouts', label: 'Handouts', icon: icons.handouts });
      tabList.push({ id: 'info', label: 'Game Info', icon: icons.info });
    }

    // Trailing tabs are ordered least-essential last on purpose: TabNavigation
    // collapses tabs into "More" from the end of this list backwards, so
    // position here is what decides survival on a narrow viewport. Reference
    // material (Loot Tables) outranks the audit trail (Game Logs).
    if (isGM) {
      // Loot Tables tab for GM only (not visible to players or audience)
      tabList.push({ id: 'loot_tables', label: 'Loot Tables', icon: icons.info });
    }

    if (isGM || (gameState === 'completed' || gameState === 'cancelled')) {
      // Game Logs for the game
      tabList.push({ id: 'logs', 'label': 'Game Logs', icon: icons.info });
    }

    return tabList;
  }, [gameState, isGM, participantCount, currentPhaseType, isParticipant, isAudience, hasCharacters, unvotedPollsCount, hasSubmittedAction]);

  // Smart default tab selection logic based on game context
  const defaultTab = useMemo(() => {
    if (tabs.length === 0) return 'default';

    // Priority 1: In-progress game - phase-aware defaults
    if (gameState === 'in_progress') {
      if (currentPhaseType === 'common_room') {
        // Common room phase - everyone goes to common room
        if (tabs.some(t => t.id === 'common-room')) return 'common-room';
      } else if (currentPhaseType === 'action') {
        // Action phase - different default for GM vs players
        // GM & Players: See Actions tab first
        // Audience: Go to Audience tab, which is now private messages only.
        // Action submissions and results moved to History, so this default no
        // longer lands a spectator on the action traffic it was chosen for.
        // Left as-is deliberately: private messages are still a reasonable
        // landing spot, and changing it is a UX call, not a comment fix.
        if (isAudience && tabs.some(t => t.id === 'audience')) {
          return 'audience';
        }
        if (tabs.some(t => t.id === 'actions')) return 'actions';
      }

      // No active phase or unknown phase type - prefer common-room or phases
      // Check for common-room tab first (most engaging for players)
      if (tabs.some(t => t.id === 'common-room')) return 'common-room';
      // GM without common room? Go to phases management
      if (isGM && tabs.some(t => t.id === 'phases')) return 'phases';
    }

    // Priority 2: Recruitment - applications for GM, info for players
    if (gameState === 'recruitment') {
      if (isGM && tabs.some(t => t.id === 'applications')) {
        return 'applications';
      }
      // Players see game info during recruitment
      if (tabs.some(t => t.id === 'info')) return 'info';
    }

    // Priority 3: Character creation - go to people tab
    if (gameState === 'character_creation') {
      if (tabs.some(t => t.id === 'people')) return 'people';
    }

    // Priority 4: Completed/cancelled games - history
    if (gameState === 'completed' || gameState === 'cancelled') {
      if (tabs.some(t => t.id === 'history')) return 'history';
    }

    // Fallback: First tab
    return tabs[0].id;
  }, [tabs, gameState, currentPhaseType, isGM, isAudience]);

  // Handle URL parameters and apply smart defaults
  useEffect(() => {
    // Don't run if tabs haven't loaded yet (avoid false positives on invalid tabs)
    if (tabs.length === 0) {
      return;
    }

    // Don't run if game data hasn't loaded yet - wait for actual game state
    // This prevents redirecting URL params before we know what tabs should exist
    if (!gameState) {
      return;
    }

    // For in_progress games, wait for phase data to load before setting default tab
    // This prevents setting wrong default (People) when currentPhaseType is still loading
    if (gameState === 'in_progress' && isPhaseLoading && !hasSetInitialTab.current) {
      return;
    }

    // Check if there's a URL param
    const urlTab = searchParams.get('tab');

    if (urlTab) {
      // If URL tab is valid, use it
      if (tabs.some(t => t.id === urlTab)) {
        hasSetInitialTab.current = true;
        if (activeTab !== urlTab) {
          setActiveTab(urlTab);
        }
        return;
      } else {
        // URL tab not found in current tab list. If role/participant data is still
        // loading, the tab list is incomplete — wait before declaring the tab invalid.
        if (isRoleLoading) {
          return;
        }

        // Invalid URL param - determine redirect target.
        // If a comment deep-link is present, redirect to history (not the phase-aware
        // default) so the comment can be resolved in the archived phase, preserving
        // the comment param for HistoryView to handle.
        const hasCommentParam = searchParams.has('comment');
        const historyTabExists = tabs.some(t => t.id === 'history');
        const redirectTab = (hasCommentParam && historyTabExists) ? 'history' : defaultTab;

        hasSetInitialTab.current = true;
        setActiveTab(redirectTab);
        const newParams = new URLSearchParams(searchParams);
        newParams.set('tab', redirectTab);
        setSearchParams(newParams, { replace: true });
        return;
      }
    }

    // No URL parameter - set the default tab in URL
    if (!hasSetInitialTab.current) {
      setActiveTab(defaultTab);
      const newParams = new URLSearchParams(searchParams);
      newParams.set('tab', defaultTab);
      setSearchParams(newParams, { replace: true });
      hasSetInitialTab.current = true;
    } else if (!tabs.some(t => t.id === activeTab)) {
      // Current tab is no longer valid (e.g., action phase ended) - reset to default
      setActiveTab(defaultTab);
      const newParams = new URLSearchParams(searchParams);
      newParams.set('tab', defaultTab);
      setSearchParams(newParams, { replace: true });
    }
  }, [tabs, defaultTab, activeTab, searchParams, setSearchParams, gameState, currentPhaseType, isPhaseLoading, isRoleLoading]);

  // Wrapper for setActiveTab that updates URL with new tab
  const handleSetActiveTab = (tabId: string) => {
    setActiveTab(tabId);
    // Update URL with new tab parameter (creates history entry)
    const newParams = new URLSearchParams(searchParams);
    newParams.set('tab', tabId);
    // Clear tab-specific sub-params when leaving their tab
    if (tabId !== 'messages') newParams.delete('conversation');
    if (tabId !== 'audience') {
      newParams.delete('audienceConversation');
      newParams.delete('audienceParticipants');
    }
    if (tabId !== 'people') {
      newParams.delete('character');
      newParams.delete('peopleTab');
    }
    setSearchParams(newParams, { replace: false });
  };

  return {
    tabs,
    activeTab: activeTab === 'default' ? defaultTab : activeTab,
    setActiveTab: handleSetActiveTab,
  };
}
