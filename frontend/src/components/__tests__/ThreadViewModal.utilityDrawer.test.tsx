import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test-utils/render';
import { ThreadViewModal } from '../ThreadViewModal';
import { UtilityDrawerProvider } from '../../contexts/UtilityDrawerContext';
import { GlobalUtilityDrawer } from '../utility-drawer/GlobalUtilityDrawer';
import { __resetBodyScrollLockForTests } from '../../hooks/useBodyScrollLock';
import type { Message } from '../../types/messages';
import type { Character } from '../../types/characters';

/**
 * The utility drawer opened from inside a thread view.
 *
 * The complaint this fixes: opening a thread blurred and disabled the whole
 * page behind it, including the nav's Utilities button, so a player mid-thread
 * couldn't reach the dice roller or their character sheet without closing the
 * thread and losing their place.
 *
 * What's verified here is the part that would fail silently: that the drawer
 * opens over the thread without tearing it down, and that closing the drawer
 * leaves the thread open with scroll still locked.
 *
 * The thread is inert while the drawer is open — the drawer is a modal dialog
 * and Headless UI has no non-modal mode. That's an accepted trade-off, not an
 * oversight; see the UtilityDrawer docblock.
 *
 * CharacterSheet is mocked because nothing here depends on its internals — this
 * file is about the thread's scroll lock and dismissal, and the sheet is only
 * present as the thing the drawer opens. Mounting the real one would pull two
 * character queries into every test for no assertion's benefit.
 *
 * (It used to be mocked because the real component hung under jsdom. That render
 * loop is fixed — see CharacterSheet.test.tsx — so the mock is now a scoping
 * choice rather than a workaround.)
 */
vi.mock('../CharacterSheet', () => ({
  CharacterSheet: ({ characterId }: { characterId: number }) => (
    <div data-testid="character-sheet-probe" data-character-id={String(characterId)} />
  ),
}));

const GAME_ID = 1;
const POST_ID = 100;
const USER_ID = 200;

const characters: Character[] = [
  {
    id: 1,
    game_id: GAME_ID,
    user_id: USER_ID,
    username: 'testuser',
    name: 'Hero',
    character_type: 'player_character',
    status: 'active',
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
  },
];

const comment: Message = {
  id: 1,
  game_id: GAME_ID,
  author_id: USER_ID,
  character_id: 1,
  content: 'A comment inside the thread',
  message_type: 'comment',
  thread_depth: 0,
  author_username: 'testuser',
  character_name: 'Hero',
  reply_count: 0,
  is_edited: false,
  is_deleted: false,
  created_at: '2025-01-15T10:30:00Z',
  updated_at: '2025-01-15T10:30:00Z',
};

function renderThreadWithDrawer() {
  return renderWithProviders(
    <UtilityDrawerProvider>
      <ThreadViewModal
        gameId={GAME_ID}
        postId={POST_ID}
        comment={comment}
        characters={characters}
        controllableCharacters={characters}
        onClose={vi.fn()}
        onCreateReply={vi.fn()}
        currentUserId={USER_ID}
      />
      <GlobalUtilityDrawer />
    </UtilityDrawerProvider>
  );
}

describe('ThreadViewModal + utility drawer', () => {
  beforeEach(() => {
    __resetBodyScrollLockForTests();
    document.body.style.overflow = '';
    server.use(
      http.get('/api/v1/games/:gameId/details', () =>
        HttpResponse.json({ id: GAME_ID, title: 'Test Game', state: 'in_progress' })
      ),
      http.get('/api/v1/games/:gameId/messages/:messageId/replies', () =>
        HttpResponse.json([])
      )
    );
  });

  afterEach(() => {
    __resetBodyScrollLockForTests();
    document.body.style.overflow = '';
  });

  it('offers a Utilities button in the thread header', async () => {
    renderThreadWithDrawer();
    expect(await screen.findByRole('button', { name: 'Utilities' })).toBeInTheDocument();
  });

  it('opens the drawer over the thread without closing it', async () => {
    const user = userEvent.setup();
    renderThreadWithDrawer();

    await user.click(await screen.findByRole('button', { name: 'Utilities' }));

    // The drawer is up...
    expect(await screen.findByTestId('utility-list')).toBeInTheDocument();
    // ...and the thread is still there behind it, which is the whole point.
    expect(screen.getByText('Thread View')).toBeInTheDocument();
    expect(screen.getByText('A comment inside the thread')).toBeInTheDocument();
  });

  it('keeps the thread mounted behind the drawer', async () => {
    const user = userEvent.setup();
    renderThreadWithDrawer();

    await user.click(await screen.findByRole('button', { name: 'Utilities' }));
    await screen.findByTestId('utility-list');

    // The thread stays mounted and visible underneath, so closing the drawer
    // returns you to it with your place intact. It is *not* interactive while
    // the drawer is up — the drawer is a modal dialog and inerts what's behind
    // it (see Drawer.test.tsx). That's accepted: the drawer is reference
    // material you open, read, and close.
    expect(screen.getByText('Thread View')).toBeInTheDocument();
    expect(screen.getByText('A comment inside the thread')).toBeInTheDocument();
  });

  it('keeps the thread open and scroll locked after the drawer closes', async () => {
    const user = userEvent.setup();
    renderThreadWithDrawer();

    await user.click(await screen.findByRole('button', { name: 'Utilities' }));
    await screen.findByTestId('utility-list');

    await user.click(screen.getByLabelText('Close'));

    await waitFor(() => {
      expect(screen.queryByTestId('utility-list')).not.toBeInTheDocument();
    });

    // The thread survives the drawer closing...
    expect(screen.getByText('Thread View')).toBeInTheDocument();
    // ...and the page behind it must not have been unlocked by the drawer's
    // release — the thread still holds a lock of its own.
    expect(document.body.style.overflow).toBe('hidden');
  });
});
