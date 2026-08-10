import { describe, it, expect } from 'vitest';
import { renderHook, waitFor, render, screen, act } from '@testing-library/react';
import { useState } from 'react';
import { MemoryRouter, useSearchParams, useNavigate } from 'react-router-dom';
import { useGameTabs } from './useGameTabs';

const wrapper = ({ children }: { children: React.ReactNode }) => (
  <MemoryRouter initialEntries={['/games/1']}>{children}</MemoryRouter>
);

describe('useGameTabs', () => {
  describe('Bug #11: Messages tab visibility', () => {
    it('should show Messages tab for regular participants', () => {
      // Arrange: User is a regular player participant
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'in_progress',
            isGM: false,
            participantCount: 3,
            currentPhaseType: 'action',
            isAudience: false,
            isParticipant: true,
            hasCharacters: true,
          }),
        { wrapper }
      );

      // Assert: Messages tab should be present
      const messagesTab = result.current.tabs.find(tab => tab.id === 'messages');
      expect(messagesTab).toBeDefined();
    });

    it('should show Messages tab for GM', () => {
      // Arrange: User is the GM
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'in_progress',
            isGM: true,
            participantCount: 3,
            currentPhaseType: 'action',
            isAudience: false,
            isParticipant: false,
            hasCharacters: false,
          }),
        { wrapper }
      );

      // Assert: Messages tab should be present
      const messagesTab = result.current.tabs.find(tab => tab.id === 'messages');
      expect(messagesTab).toBeDefined();
    });

    it('should show Messages tab for audience member WITH assigned NPC', () => {
      // Arrange: User is an audience member with a character
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'in_progress',
            isGM: false,
            participantCount: 3,
            currentPhaseType: 'action',
            isAudience: true,
            isParticipant: false,
            hasCharacters: true, // Has an NPC assigned
          }),
        { wrapper }
      );

      // Assert: Messages tab should be present
      const messagesTab = result.current.tabs.find(tab => tab.id === 'messages');
      expect(messagesTab).toBeDefined();
    });

    it('should NOT show Messages tab for audience member WITHOUT assigned NPC', () => {
      // Arrange: User is an audience member without any characters
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'in_progress',
            isGM: false,
            participantCount: 3,
            currentPhaseType: 'action',
            isAudience: true,
            isParticipant: false,
            hasCharacters: false, // No NPC assigned
          }),
        { wrapper }
      );

      // Assert: Messages tab should NOT be present
      const messagesTab = result.current.tabs.find(tab => tab.id === 'messages');
      expect(messagesTab).toBeUndefined();
    });

    it('should NOT show Messages tab for non-participants', () => {
      // Arrange: User has not joined the game
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'in_progress',
            isGM: false,
            participantCount: 3,
            currentPhaseType: 'action',
            isAudience: false,
            isParticipant: false,
            hasCharacters: false,
          }),
        { wrapper }
      );

      // Assert: Messages tab should NOT be present
      const messagesTab = result.current.tabs.find(tab => tab.id === 'messages');
      expect(messagesTab).toBeUndefined();
    });
  });

  describe('Bug #12: Submit Action button visibility', () => {
    it('should show Actions tab for GM during action phase', () => {
      // Arrange: GM during action phase
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'in_progress',
            isGM: true,
            participantCount: 3,
            currentPhaseType: 'action',
            isAudience: false,
            isParticipant: false,
            hasCharacters: false,
          }),
        { wrapper }
      );

      // Assert: Actions tab should be present
      const actionsTab = result.current.tabs.find(tab => tab.id === 'actions');
      expect(actionsTab).toBeDefined();
      expect(actionsTab?.label).toBe('Actions'); // GM sees "Actions"
    });

    it('should show Submit Action tab for regular participants during action phase', () => {
      // Arrange: Regular player participant during action phase
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'in_progress',
            isGM: false,
            participantCount: 3,
            currentPhaseType: 'action',
            isAudience: false,
            isParticipant: true,
            hasCharacters: true,
          }),
        { wrapper }
      );

      // Assert: Actions tab should be present
      const actionsTab = result.current.tabs.find(tab => tab.id === 'actions');
      expect(actionsTab).toBeDefined();
      expect(actionsTab?.label).toBe('Submit Action'); // Players see "Submit Action"
    });

    it('should NOT show Submit Action tab for audience members during action phase', () => {
      // Arrange: Audience member during action phase
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'in_progress',
            isGM: false,
            participantCount: 3,
            currentPhaseType: 'action',
            isAudience: true,
            isParticipant: false,
            hasCharacters: true, // Even with NPC
          }),
        { wrapper }
      );

      // Assert: Actions tab should NOT be present
      const actionsTab = result.current.tabs.find(tab => tab.id === 'actions');
      expect(actionsTab).toBeUndefined();
    });

    it('should NOT show Submit Action tab for non-participants during action phase', () => {
      // Arrange: Non-participant viewing game during action phase
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'in_progress',
            isGM: false,
            participantCount: 3,
            currentPhaseType: 'action',
            isAudience: false,
            isParticipant: false,
            hasCharacters: false,
          }),
        { wrapper }
      );

      // Assert: Actions tab should NOT be present
      const actionsTab = result.current.tabs.find(tab => tab.id === 'actions');
      expect(actionsTab).toBeUndefined();
    });

    it('should NOT show Actions tab during non-action phases', () => {
      // Arrange: Participant during common room phase
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'in_progress',
            isGM: false,
            participantCount: 3,
            currentPhaseType: 'common_room',
            isAudience: false,
            isParticipant: true,
            hasCharacters: true,
          }),
        { wrapper }
      );

      // Assert: Actions tab should NOT be present
      const actionsTab = result.current.tabs.find(tab => tab.id === 'actions');
      expect(actionsTab).toBeUndefined();
    });
  });

  describe('Default tab behavior', () => {
    it('should default to common-room tab for in_progress games with common_room phase', async () => {
      // Arrange: In-progress game with common room phase
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'in_progress',
            isGM: false,
            participantCount: 3,
            currentPhaseType: 'common_room',
            isAudience: false,
            isParticipant: true,
            hasCharacters: true,
          }),
        { wrapper }
      );

      // Wait for useEffect to complete and update activeTab
      await waitFor(() => {
        expect(result.current.activeTab).toBe('common-room');
      });
    });

    it('should default to actions tab for in_progress games with action phase', () => {
      // Arrange: In-progress game with action phase
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'in_progress',
            isGM: false,
            participantCount: 3,
            currentPhaseType: 'action',
            isAudience: false,
            isParticipant: true,
            hasCharacters: true,
          }),
        { wrapper }
      );

      // Assert: Active tab should be actions
      expect(result.current.activeTab).toBe('actions');
    });

    it('should default to phases tab for GM when no common room or action phase', async () => {
      // Arrange: In-progress game, GM, no common room or action phase
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'in_progress',
            isGM: true,
            participantCount: 3,
            currentPhaseType: undefined,
            isAudience: false,
            isParticipant: false,
            hasCharacters: false,
          }),
        { wrapper }
      );

      // Wait for useEffect to complete and update activeTab
      await waitFor(() => {
        expect(result.current.activeTab).toBe('phases');
      });
    });

    it('should default to applications tab for GM in recruitment state', async () => {
      // Arrange: Recruitment game, GM
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'recruitment',
            isGM: true,
            participantCount: 0,
            currentPhaseType: undefined,
            isAudience: false,
            isParticipant: false,
            hasCharacters: false,
          }),
        { wrapper }
      );

      // Wait for useEffect to complete and update activeTab
      await waitFor(() => {
        expect(result.current.activeTab).toBe('applications');
      });
    });

    it('should default to info tab for players in recruitment state', async () => {
      // Arrange: Recruitment game, regular user
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'recruitment',
            isGM: false,
            participantCount: 0,
            currentPhaseType: undefined,
            isAudience: false,
            isParticipant: false,
            hasCharacters: false,
          }),
        { wrapper }
      );

      // Wait for useEffect to complete and update activeTab
      await waitFor(() => {
        expect(result.current.activeTab).toBe('info');
      });
    });
  });

  describe('Issue 1.2: Handouts tab visibility across all game states', () => {
    it('should show Handouts tab in setup state', () => {
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'setup',
            isGM: true,
            participantCount: 0,
            currentPhaseType: undefined,
            isAudience: false,
            isParticipant: false,
            hasCharacters: false,
          }),
        { wrapper }
      );

      const handoutsTab = result.current.tabs.find(tab => tab.id === 'handouts');
      expect(handoutsTab).toBeDefined();
    });

    it('should show Handouts tab in recruitment state', () => {
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'recruitment',
            isGM: false,
            participantCount: 0,
            currentPhaseType: undefined,
            isAudience: false,
            isParticipant: false,
            hasCharacters: false,
          }),
        { wrapper }
      );

      const handoutsTab = result.current.tabs.find(tab => tab.id === 'handouts');
      expect(handoutsTab).toBeDefined();
    });

    it('should show Handouts tab in character_creation state', () => {
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'character_creation',
            isGM: false,
            participantCount: 3,
            currentPhaseType: undefined,
            isAudience: false,
            isParticipant: true,
            hasCharacters: false,
          }),
        { wrapper }
      );

      const handoutsTab = result.current.tabs.find(tab => tab.id === 'handouts');
      expect(handoutsTab).toBeDefined();
    });

    it('should show Handouts tab in in_progress state', () => {
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'in_progress',
            isGM: false,
            participantCount: 3,
            currentPhaseType: 'action',
            isAudience: false,
            isParticipant: true,
            hasCharacters: true,
          }),
        { wrapper }
      );

      const handoutsTab = result.current.tabs.find(tab => tab.id === 'handouts');
      expect(handoutsTab).toBeDefined();
    });

    it('should show Handouts tab in completed state', () => {
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'completed',
            isGM: false,
            participantCount: 3,
            currentPhaseType: undefined,
            isAudience: false,
            isParticipant: true,
            hasCharacters: true,
          }),
        { wrapper }
      );

      const handoutsTab = result.current.tabs.find(tab => tab.id === 'handouts');
      expect(handoutsTab).toBeDefined();
    });

    it('should show Stats tab in completed state', () => {
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'completed',
            isGM: false,
            participantCount: 3,
            currentPhaseType: undefined,
            isAudience: false,
            isParticipant: true,
            hasCharacters: true,
          }),
        { wrapper }
      );

      const statsTab = result.current.tabs.find(tab => tab.id === 'stats');
      expect(statsTab).toBeDefined();
      expect(statsTab?.label).toBe('Stats');
    });

    it('should show Audience tab alongside Stats for a non-GM non-audience viewer', () => {
      // The Stats tab deep-links into the Audience tab, so wherever Stats is
      // visible the link target must be too — including for a plain player or
      // an outside viewer of the public archive.
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'completed',
            isGM: false,
            participantCount: 3,
            currentPhaseType: undefined,
            isAudience: false,
            isParticipant: false,
            hasCharacters: false,
          }),
        { wrapper }
      );

      expect(result.current.tabs.find(tab => tab.id === 'stats')).toBeDefined();
      expect(result.current.tabs.find(tab => tab.id === 'audience')).toBeDefined();
      expect(result.current.tabs.find(tab => tab.id === 'history')).toBeDefined();
    });

    it('should NOT show Stats tab in cancelled state', () => {
      // The stats endpoint serves completed games only; a cancelled game is not
      // a public archive, so the tab would only ever render an error.
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'cancelled',
            isGM: true,
            participantCount: 3,
            currentPhaseType: undefined,
            isAudience: false,
            isParticipant: true,
            hasCharacters: true,
          }),
        { wrapper }
      );

      expect(result.current.tabs.find(tab => tab.id === 'stats')).toBeUndefined();
    });

    it('should NOT show Stats tab while the game is in progress', () => {
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'in_progress',
            isGM: true,
            participantCount: 3,
            currentPhaseType: 'common_room',
            isAudience: false,
            isParticipant: true,
            hasCharacters: true,
          }),
        { wrapper }
      );

      expect(result.current.tabs.find(tab => tab.id === 'stats')).toBeUndefined();
    });

    it('should show Handouts tab in cancelled state', () => {
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'cancelled',
            isGM: true,
            participantCount: 0,
            currentPhaseType: undefined,
            isAudience: false,
            isParticipant: false,
            hasCharacters: false,
          }),
        { wrapper }
      );

      const handoutsTab = result.current.tabs.find(tab => tab.id === 'handouts');
      expect(handoutsTab).toBeDefined();
    });
  });

  describe('Issue 1.3: Applications tab visibility by game state', () => {
    it('should show Applications tab for GM during recruitment state', () => {
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'recruitment',
            isGM: true,
            participantCount: 0,
            currentPhaseType: undefined,
            isAudience: false,
            isParticipant: false,
            hasCharacters: false,
          }),
        { wrapper }
      );

      const applicationsTab = result.current.tabs.find(tab => tab.id === 'applications');
      expect(applicationsTab).toBeDefined();
      expect(applicationsTab?.label).toBe('Applications');
    });

    it('should NOT show Applications tab for players during recruitment state', () => {
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'recruitment',
            isGM: false,
            participantCount: 0,
            currentPhaseType: undefined,
            isAudience: false,
            isParticipant: false,
            hasCharacters: false,
          }),
        { wrapper }
      );

      const applicationsTab = result.current.tabs.find(tab => tab.id === 'applications');
      expect(applicationsTab).toBeUndefined();
    });

    it('should NOT show Applications tab during character_creation state (even for GM)', () => {
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'character_creation',
            isGM: true,
            participantCount: 3,
            currentPhaseType: undefined,
            isAudience: false,
            isParticipant: true,
            hasCharacters: false,
          }),
        { wrapper }
      );

      const applicationsTab = result.current.tabs.find(tab => tab.id === 'applications');
      expect(applicationsTab).toBeUndefined();
    });

    it('should NOT show Applications tab during in_progress state', () => {
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'in_progress',
            isGM: true,
            participantCount: 3,
            currentPhaseType: 'action',
            isAudience: false,
            isParticipant: false,
            hasCharacters: false,
          }),
        { wrapper }
      );

      const applicationsTab = result.current.tabs.find(tab => tab.id === 'applications');
      expect(applicationsTab).toBeUndefined();
    });

    it('should NOT show Applications tab during completed state', () => {
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'completed',
            isGM: true,
            participantCount: 3,
            currentPhaseType: undefined,
            isAudience: false,
            isParticipant: true,
            hasCharacters: true,
          }),
        { wrapper }
      );

      const applicationsTab = result.current.tabs.find(tab => tab.id === 'applications');
      expect(applicationsTab).toBeUndefined();
    });

    it('should NOT show Applications tab during cancelled state', () => {
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'cancelled',
            isGM: true,
            participantCount: 0,
            currentPhaseType: undefined,
            isAudience: false,
            isParticipant: false,
            hasCharacters: false,
          }),
        { wrapper }
      );

      const applicationsTab = result.current.tabs.find(tab => tab.id === 'applications');
      expect(applicationsTab).toBeUndefined();
    });

    it('should NOT show Applications tab during setup state', () => {
      const { result } = renderHook(
        () =>
          useGameTabs({
            gameState: 'setup',
            isGM: true,
            participantCount: 0,
            currentPhaseType: undefined,
            isAudience: false,
            isParticipant: false,
            hasCharacters: false,
          }),
        { wrapper }
      );

      const applicationsTab = result.current.tabs.find(tab => tab.id === 'applications');
      expect(applicationsTab).toBeUndefined();
    });
  });

  describe('Tab param clearing when switching tabs', () => {
    // Helper: renders a component that exposes the current search params via data-testid spans
    function TabParamSpy({ tabArgs }: { tabArgs: Parameters<typeof useGameTabs>[0] }) {
      const [searchParams] = useSearchParams();
      const { setActiveTab } = useGameTabs(tabArgs);
      return (
        <div>
          <span data-testid="conversation">{searchParams.get('conversation') ?? ''}</span>
          <span data-testid="audienceConversation">{searchParams.get('audienceConversation') ?? ''}</span>
          <span data-testid="character">{searchParams.get('character') ?? ''}</span>
          <span data-testid="peopleTab">{searchParams.get('peopleTab') ?? ''}</span>
          <button onClick={() => setActiveTab('common-room')}>go-common-room</button>
          <button onClick={() => setActiveTab('messages')}>go-messages</button>
        </div>
      );
    }

    const defaultTabArgs: Parameters<typeof useGameTabs>[0] = {
      gameState: 'in_progress',
      isGM: false,
      participantCount: 3,
      currentPhaseType: 'common_room',
      isAudience: false,
      isParticipant: true,
      hasCharacters: true,
    };

    it('should clear conversation param when switching away from messages tab', async () => {
      render(
        <MemoryRouter initialEntries={['/games/1?tab=messages&conversation=42']}>
          <TabParamSpy tabArgs={defaultTabArgs} />
        </MemoryRouter>
      );

      expect(screen.getByTestId('conversation').textContent).toBe('42');

      act(() => { screen.getByRole('button', { name: 'go-common-room' }).click(); });

      await waitFor(() => {
        expect(screen.getByTestId('conversation').textContent).toBe('');
      });
    });

    it('should clear audienceConversation param when switching away from audience tab', async () => {
      const gmArgs = { ...defaultTabArgs, isGM: true, isParticipant: false, hasCharacters: false };
      render(
        <MemoryRouter initialEntries={['/games/1?tab=audience&audienceConversation=7']}>
          <TabParamSpy tabArgs={gmArgs} />
        </MemoryRouter>
      );

      expect(screen.getByTestId('audienceConversation').textContent).toBe('7');

      act(() => { screen.getByRole('button', { name: 'go-common-room' }).click(); });

      await waitFor(() => {
        expect(screen.getByTestId('audienceConversation').textContent).toBe('');
      });
    });

    it('should clear character and peopleTab params when switching away from people tab', async () => {
      render(
        <MemoryRouter initialEntries={['/games/1?tab=people&character=5&peopleTab=participants']}>
          <TabParamSpy tabArgs={defaultTabArgs} />
        </MemoryRouter>
      );

      expect(screen.getByTestId('character').textContent).toBe('5');
      expect(screen.getByTestId('peopleTab').textContent).toBe('participants');

      act(() => { screen.getByRole('button', { name: 'go-common-room' }).click(); });

      await waitFor(() => {
        expect(screen.getByTestId('character').textContent).toBe('');
        expect(screen.getByTestId('peopleTab').textContent).toBe('');
      });
    });

    it('should preserve conversation param when navigating to messages tab', async () => {
      render(
        <MemoryRouter initialEntries={['/games/1?tab=common-room&conversation=42']}>
          <TabParamSpy tabArgs={defaultTabArgs} />
        </MemoryRouter>
      );

      // conversation is in the URL but we're on common-room
      expect(screen.getByTestId('conversation').textContent).toBe('42');

      act(() => { screen.getByRole('button', { name: 'go-messages' }).click(); });

      // Switching TO messages should not clear conversation
      await waitFor(() => {
        expect(screen.getByTestId('conversation').textContent).toBe('42');
      });
    });
  });

  describe('Bug: tab switching in setup state does not update view', () => {
    // Helper: renders component that exposes activeTab and lets us simulate a Link-style navigation
    // (URL change without calling setActiveTab directly — exactly what <Link> does)
    function SetupTabSpy({ tabArgs }: { tabArgs: Parameters<typeof useGameTabs>[0] }) {
      const navigate = useNavigate();
      const { activeTab } = useGameTabs(tabArgs);
      return (
        <div>
          <span data-testid="active-tab">{activeTab}</span>
          {/* Simulate <Link to="?tab=info"> click — changes URL only, does NOT call setActiveTab */}
          <button onClick={() => navigate('?tab=info')}>link-to-info</button>
          <button onClick={() => navigate('?tab=handouts')}>link-to-handouts</button>
        </div>
      );
    }

    const setupTabArgs: Parameters<typeof useGameTabs>[0] = {
      gameState: 'setup',
      isGM: true,
      participantCount: 0,
      currentPhaseType: undefined,
      isAudience: false,
      isParticipant: false,
      hasCharacters: false,
    };

    it('should update activeTab when URL changes via Link navigation in setup state', async () => {
      // Reproduce the exact bug: <Link> changes URL but view does not update
      // because useEffect exited early when gameState === 'setup'
      render(
        <MemoryRouter initialEntries={['/games/1?tab=handouts']}>
          <SetupTabSpy tabArgs={setupTabArgs} />
        </MemoryRouter>
      );

      // Initially on handouts
      await waitFor(() => {
        expect(screen.getByTestId('active-tab').textContent).toBe('handouts');
      });

      // Simulate clicking <Link to="?tab=info"> — URL changes, setActiveTab is NOT called
      act(() => { screen.getByRole('button', { name: 'link-to-info' }).click(); });

      // activeTab must update to 'info' without a full page reload
      await waitFor(() => {
        expect(screen.getByTestId('active-tab').textContent).toBe('info');
      });
    });

    it('should read initial URL tab param in setup state', async () => {
      // If user navigates directly to ?tab=info in setup state, it should be respected
      const wrapperWithInfo = ({ children }: { children: React.ReactNode }) => (
        <MemoryRouter initialEntries={['/games/1?tab=info']}>{children}</MemoryRouter>
      );

      const { result } = renderHook(
        () =>
          useGameTabs(setupTabArgs),
        { wrapper: wrapperWithInfo }
      );

      await waitFor(() => {
        expect(result.current.activeTab).toBe('info');
      });
    });
  });

  describe('Bug: refreshing page with valid URL tab redirects to default when game data loads after render', () => {
    it('should preserve ?tab=people on refresh for character_creation game even when game loads after initial render', async () => {
      // Regression: GameDetailsPage passes game?.state (undefined while loading).
      // Before fix: undefined fell into else-branch producing [handouts, info] tabs,
      // so 'people' was treated as invalid and got redirected to 'handouts'.
      // After fix: undefined returns empty tab list, effect waits until real state arrives.
      function LazyGameTabSpy() {
        const navigate = useNavigate();
        // Simulate undefined→'character_creation' transition (game data loading)
        const [gameState, setGameState] = useState<Parameters<typeof useGameTabs>[0]['gameState']>(undefined);
        const { activeTab } = useGameTabs({
          gameState,
          isGM: true,
          participantCount: 3,
          currentPhaseType: undefined,
          isAudience: false,
          isParticipant: false,
          hasCharacters: false,
        });
        return (
          <div>
            <span data-testid="active-tab">{activeTab}</span>
            <button onClick={() => setGameState('character_creation')}>load-game</button>
            <button onClick={() => navigate('?tab=people')}>nav-to-people</button>
          </div>
        );
      }

      render(
        <MemoryRouter initialEntries={['/games/1?tab=people']}>
          <LazyGameTabSpy />
        </MemoryRouter>
      );

      // Simulate game data arriving
      act(() => { screen.getByRole('button', { name: 'load-game' }).click(); });

      // Should land on 'people', not be redirected to 'handouts'
      await waitFor(() => {
        expect(screen.getByTestId('active-tab').textContent).toBe('people');
      });
    });
  });

  describe('Bug: notification link to messages tab lands on people tab during interlude phase', () => {
    it('should wait for role data before redirecting an unrecognized URL tab', async () => {
      // Simulate the race: phase resolves before participant data.
      // Initial state: isRoleLoading=true, isParticipant=false → messages tab absent.
      // Expected: effect does NOT redirect to people while role is still loading.
      function RaceConditionSpy() {
        const [isParticipant, setIsParticipant] = useState(false);
        const [isRoleLoading, setIsRoleLoading] = useState(true);
        const { activeTab } = useGameTabs({
          gameState: 'in_progress',
          isGM: false,
          participantCount: 3,
          currentPhaseType: 'interlude',
          isPhaseLoading: false, // phase already resolved
          isAudience: false,
          isParticipant,
          hasCharacters: false,
          isRoleLoading,
        });
        return (
          <div>
            <span data-testid="active-tab">{activeTab}</span>
            <button onClick={() => { setIsParticipant(true); setIsRoleLoading(false); }}>
              resolve-role
            </button>
          </div>
        );
      }

      render(
        <MemoryRouter initialEntries={['/games/1?tab=messages&conversation=42']}>
          <RaceConditionSpy />
        </MemoryRouter>
      );

      // Role still loading — should not have redirected to people yet
      expect(screen.getByTestId('active-tab').textContent).not.toBe('people');

      // Participant data arrives
      act(() => { screen.getByRole('button', { name: 'resolve-role' }).click(); });

      // Now messages tab exists and should be active
      await waitFor(() => {
        expect(screen.getByTestId('active-tab').textContent).toBe('messages');
      });
    });
  });

  describe('Bug: comment deep-link with invalid tab redirects to history preserving comment param', () => {
    it('should redirect to history tab (not default) when common-room is invalid and comment param is present', async () => {
      // Simulate arriving at ?tab=common-room&comment=42 during an action phase
      // (common-room tab doesn't exist in the action phase tab list)
      const wrapperWithUrl = ({ children }: { children: React.ReactNode }) => (
        <MemoryRouter initialEntries={['/games/1?tab=common-room&comment=42']}>
          {children}
        </MemoryRouter>
      );

      const { result } = renderHook(
        () => useGameTabs({
          gameState: 'in_progress',
          isGM: false,
          participantCount: 3,
          currentPhaseType: 'action',
          isAudience: false,
          isParticipant: true,
          hasCharacters: true,
        }),
        { wrapper: wrapperWithUrl }
      );

      // Wait for the effect to run and redirect
      await waitFor(() => {
        // Active tab should be 'history', not 'actions' (the default for action phase)
        expect(result.current.activeTab).toBe('history');
      });
    });
  });
});
