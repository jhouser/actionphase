import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor, fireEvent } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { UpdateCharacterSheetModal } from './UpdateCharacterSheetModal';
import { renderWithProviders } from '../test-utils/render';
import { server } from '../mocks/server';

const BASE_PROPS = {
  isOpen: true,
  onClose: vi.fn(),
  gameId: 1,
  actionResultId: 10,
  characterId: 42,
  characterName: 'Aldric the Bold',
};

// Well-formed skill matching the CharacterSkill interface
const SKILL = { id: 'str', name: 'Strength', level: 2, category: 'Physical' };
const ITEM = { id: 'item-1', name: 'Healing Potion', quantity: 2 };
const ITEM_DRAFT = { id: 'item-2', name: 'Magic Sword', quantity: 1 };

const CHAR_DATA_SKILLS = [
  {
    id: 1,
    character_id: 42,
    module_type: 'skills',
    field_name: 'skills',
    field_value: JSON.stringify([SKILL]),
    field_type: 'json',
    is_public: true,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
  },
];

const CHAR_DATA_ITEMS = [
  {
    id: 2,
    character_id: 42,
    module_type: 'inventory',
    field_name: 'items',
    field_value: JSON.stringify([ITEM]),
    field_type: 'json',
    is_public: true,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
  },
];

// A draft that overrides the inventory items
const DRAFT_ITEMS = [
  {
    id: 100,
    action_result_id: 10,
    character_id: 42,
    module_type: 'inventory',
    field_name: 'items',
    field_value: JSON.stringify([ITEM_DRAFT]),
    field_type: 'json',
    operation: 'upsert',
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
  },
];

function setupHandlers({
  characterData = [] as unknown[],
  drafts = null as null | unknown[],
}: {
  characterData?: unknown[];
  drafts?: null | unknown[];
} = {}) {
  server.use(
    http.get('http://localhost:3000/api/v1/characters/:id/data', () => {
      return HttpResponse.json(characterData);
    }),
    http.get(
      'http://localhost:3000/api/v1/games/:gameId/results/:resultId/character-updates',
      () => {
        return HttpResponse.json(drafts);
      },
    ),
    http.post(
      'http://localhost:3000/api/v1/games/:gameId/results/:resultId/character-updates',
      () => {
        return HttpResponse.json({
          id: 101,
          action_result_id: 10,
          character_id: 42,
          module_type: 'skills',
          field_name: 'skills',
          field_value: '[]',
          field_type: 'json',
          operation: 'upsert',
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        });
      },
    ),
  );
}

/** Clicks the Modal's backdrop, which carries no role or label to query by. */
function clickBackdrop() {
  const backdrop = document.querySelector('.bg-black\\/60') as HTMLElement;
  expect(backdrop).toBeInTheDocument();
  fireEvent.click(backdrop);
}

async function waitForLoaded() {
  await waitFor(() => {
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });
}

describe('UpdateCharacterSheetModal', () => {
  beforeEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
  });

  describe('Loading state', () => {
    it('shows spinner while data is loading', () => {
      server.use(
        http.get('http://localhost:3000/api/v1/characters/:id/data', async () => {
          await new Promise(() => {}); // never resolves
          return HttpResponse.json([]);
        }),
        http.get(
          'http://localhost:3000/api/v1/games/:gameId/results/:resultId/character-updates',
          async () => {
            await new Promise(() => {}); // never resolves
            return HttpResponse.json(null);
          },
        ),
      );

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);

      expect(screen.getByRole('status')).toBeInTheDocument();
    });

    it('does not render when isOpen is false', () => {
      const { container } = renderWithProviders(
        <UpdateCharacterSheetModal {...BASE_PROPS} isOpen={false} />,
      );
      expect(container.firstChild).toBeNull();
    });
  });

  describe('Initialization from characterData (no drafts)', () => {
    it('shows skills from characterData when no drafts exist', async () => {
      setupHandlers({ characterData: CHAR_DATA_SKILLS, drafts: null });

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);
      await waitForLoaded();

      expect(screen.getByText('Strength')).toBeInTheDocument();
    });

    it('shows inventory items from characterData when no drafts exist', async () => {
      setupHandlers({ characterData: CHAR_DATA_ITEMS, drafts: null });

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);
      await waitForLoaded();

      fireEvent.click(screen.getByRole('button', { name: /inventory/i }));
      expect(screen.getByText('Healing Potion')).toBeInTheDocument();
    });

    it('shows empty state when characterData has no skills', async () => {
      setupHandlers({ characterData: [], drafts: null });

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);
      await waitForLoaded();

      expect(screen.getByText('No skills yet.')).toBeInTheDocument();
    });

    it('handles null drafts response gracefully (no crash)', async () => {
      setupHandlers({ characterData: CHAR_DATA_SKILLS, drafts: null });

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);
      await waitForLoaded();

      expect(screen.getByText('Strength')).toBeInTheDocument();
    });

    it('handles empty array drafts response', async () => {
      setupHandlers({ characterData: CHAR_DATA_SKILLS, drafts: [] });

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);
      await waitForLoaded();

      expect(screen.getByText('Strength')).toBeInTheDocument();
    });
  });

  describe('Draft-first initialization', () => {
    it('uses draft values over characterData for inventory', async () => {
      // characterData has "Healing Potion"; draft has "Magic Sword"
      setupHandlers({
        characterData: CHAR_DATA_ITEMS,
        drafts: DRAFT_ITEMS,
      });

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);
      await waitForLoaded();

      fireEvent.click(screen.getByRole('button', { name: /inventory/i }));

      expect(screen.getByText('Magic Sword')).toBeInTheDocument();
      expect(screen.queryByText('Healing Potion')).not.toBeInTheDocument();
    });

    it('falls back to characterData for sections not covered by a draft', async () => {
      // Draft only covers inventory; skills section has no draft
      setupHandlers({
        characterData: [...CHAR_DATA_SKILLS, ...CHAR_DATA_ITEMS],
        drafts: DRAFT_ITEMS,
      });

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);
      await waitForLoaded();

      // Skills tab (default) — no draft for skills, so characterData is used
      expect(screen.getByText('Strength')).toBeInTheDocument();
    });
  });

  describe('Section navigation', () => {
    it('starts on the skills section', async () => {
      setupHandlers({ characterData: [], drafts: null });

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);
      await waitForLoaded();

      expect(screen.getByText('No skills yet.')).toBeInTheDocument();
    });

    it('switches to inventory section when tab is clicked', async () => {
      setupHandlers({ characterData: [], drafts: null });

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);
      await waitForLoaded();

      fireEvent.click(screen.getByRole('button', { name: /inventory/i }));

      expect(screen.getByText(/no inventory yet/i)).toBeInTheDocument();
    });
  });

  describe('Header content', () => {
    it('shows character name and modal title', async () => {
      setupHandlers({ characterData: [], drafts: null });

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);
      await waitForLoaded();

      expect(screen.getByText('Aldric the Bold')).toBeInTheDocument();
      expect(screen.getByText('Update Character Sheet')).toBeInTheDocument();
    });
  });

  describe('Redundant staged updates', () => {
    /** Records which draft rows get created vs deleted during a test. */
    function trackDraftWrites() {
      const created: string[] = [];
      const deleted: number[] = [];
      server.use(
        http.post(
          'http://localhost:3000/api/v1/games/:gameId/results/:resultId/character-updates',
          async ({ request }) => {
            const body = (await request.json()) as { module_type: string; field_value: string };
            created.push(body.field_value);
            return HttpResponse.json({ id: 101, ...body });
          },
        ),
        http.delete(
          'http://localhost:3000/api/v1/games/:gameId/results/:resultId/character-updates/:draftId',
          ({ params }) => {
            deleted.push(Number(params.draftId));
            return new HttpResponse(null, { status: 204 });
          },
        ),
      );
      return { created, deleted };
    }

    it('deletes the staged row when an edit lands back on the published sheet', async () => {
      // Published inventory holds the Healing Potion; a draft row currently stages an
      // extra Magic Sword. Removing the sword returns the sheet to published state, so
      // the row should be deleted rather than re-saved as a no-op.
      setupHandlers({
        characterData: CHAR_DATA_ITEMS,
        drafts: [{ ...DRAFT_ITEMS[0], field_value: JSON.stringify([ITEM, ITEM_DRAFT]) }],
      });
      const { created, deleted } = trackDraftWrites();

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);
      await waitForLoaded();

      fireEvent.click(screen.getByRole('button', { name: /inventory/i }));
      expect(await screen.findByText('Magic Sword')).toBeInTheDocument();

      // Remove the staged item, returning the list to exactly the published contents.
      const removeButtons = screen.getAllByRole('button', { name: 'Remove item' });
      fireEvent.click(removeButtons[removeButtons.length - 1]);

      await waitFor(() => {
        expect(deleted).toEqual([100]);
      }, { timeout: 3000 });
      expect(created).toHaveLength(0);
    });

    it('still stages a genuine removal of a pre-existing item', async () => {
      // Removing an item the character actually has is a real update — it must be
      // staged, not treated as a no-op.
      setupHandlers({ characterData: CHAR_DATA_ITEMS, drafts: [] });
      const { created, deleted } = trackDraftWrites();

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);
      await waitForLoaded();

      fireEvent.click(screen.getByRole('button', { name: /inventory/i }));
      expect(await screen.findByText('Healing Potion')).toBeInTheDocument();

      const removeButtons = screen.getAllByRole('button', { name: 'Remove item' });
      fireEvent.click(removeButtons[removeButtons.length - 1]);

      await waitFor(() => {
        expect(created).toEqual(['[]']);
      }, { timeout: 3000 });
      expect(deleted).toHaveLength(0);
    });
  });

  describe('Per-field debouncing', () => {
    /** Records the (module_type, field_name) of every draft row written. */
    function trackCreatedRows() {
      const rows: string[] = [];
      server.use(
        http.post(
          'http://localhost:3000/api/v1/games/:gameId/results/:resultId/character-updates',
          async ({ request }) => {
            const body = (await request.json()) as { module_type: string; field_name: string };
            rows.push(`${body.module_type}:${body.field_name}`);
            return HttpResponse.json({ id: 101, ...body });
          },
        ),
      );
      return rows;
    }

    it('saves both modules when two sections are edited inside one debounce window', async () => {
      // Regression: the editor used to share ONE timer and ONE pending-args ref across
      // every field. Editing skills and then inventory within the 800ms window
      // cancelled the skills timer and overwrote its args, so that edit was silently
      // dropped — the GM saw "Saved" and lost the change. Drafts are stored one row per
      // (module_type, field_name), so the two edits target different rows and both must
      // survive.
      setupHandlers({
        characterData: [...CHAR_DATA_SKILLS, ...CHAR_DATA_ITEMS],
        drafts: null,
      });
      const rows = trackCreatedRows();

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);
      await waitForLoaded();

      // Edit skills, then immediately switch tabs and edit inventory. No waiting
      // between them: both edits land inside the same debounce window, which is the
      // condition that used to drop the first one.
      fireEvent.click(screen.getAllByRole('button', { name: 'Remove skill' })[0]);

      fireEvent.click(screen.getByRole('button', { name: /inventory/i }));
      expect(await screen.findByText('Healing Potion')).toBeInTheDocument();
      const removeItemButtons = screen.getAllByRole('button', { name: 'Remove item' });
      fireEvent.click(removeItemButtons[removeItemButtons.length - 1]);

      await waitFor(() => {
        expect(rows).toHaveLength(2);
      }, { timeout: 3000 });

      // Both rows written, regardless of the order the timers fired in.
      expect([...rows].sort()).toEqual(['inventory:items', 'skills:skills']);
    });

    it('collapses repeated edits to one field into a single save', async () => {
      // The per-field split must not cost us the debounce itself: three rapid edits to
      // the same module are still one row and should produce one write, not three.
      // Three items, so three successive removals are three edits to one field.
      setupHandlers({
        characterData: [
          {
            ...CHAR_DATA_ITEMS[0],
            field_value: JSON.stringify([
              ITEM,
              { id: 'item-3', name: 'Rope', quantity: 1 },
              { id: 'item-4', name: 'Torch', quantity: 1 },
            ]),
          },
        ],
        drafts: null,
      });
      const rows = trackCreatedRows();

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);
      await waitForLoaded();

      fireEvent.click(screen.getByRole('button', { name: /inventory/i }));
      expect(await screen.findByText('Healing Potion')).toBeInTheDocument();

      // Remove all three in quick succession — one field, one row, one write.
      fireEvent.click(screen.getAllByRole('button', { name: 'Remove item' })[0]);
      fireEvent.click(screen.getAllByRole('button', { name: 'Remove item' })[0]);
      fireEvent.click(screen.getAllByRole('button', { name: 'Remove item' })[0]);

      await waitFor(() => {
        expect(rows).toEqual(['inventory:items']);
      }, { timeout: 3000 });
    });

    it('flushes a pending edit from every module when the modal is closed', async () => {
      // handleClose flushes the debounce buffers. With per-field maps that means
      // flushing ALL of them — a loop that fired only one would resurrect the original
      // data-loss bug at the moment the GM clicks Done.
      setupHandlers({
        characterData: [...CHAR_DATA_SKILLS, ...CHAR_DATA_ITEMS],
        drafts: null,
      });
      const rows = trackCreatedRows();
      const onClose = vi.fn();

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} onClose={onClose} />);
      await waitForLoaded();

      fireEvent.click(screen.getAllByRole('button', { name: 'Remove skill' })[0]);

      fireEvent.click(screen.getByRole('button', { name: /inventory/i }));
      expect(await screen.findByText('Healing Potion')).toBeInTheDocument();
      const removeItemButtons = screen.getAllByRole('button', { name: 'Remove item' });
      fireEvent.click(removeItemButtons[removeItemButtons.length - 1]);

      // Close while both edits are still inside the debounce window.
      fireEvent.click(screen.getByRole('button', { name: /done/i }));

      expect(onClose).toHaveBeenCalled();
      await waitFor(() => {
        expect([...rows].sort()).toEqual(['inventory:items', 'skills:skills']);
      }, { timeout: 3000 });
    });
  });

  describe('Discarding staged updates', () => {
    it('offers no discard action when nothing is staged', async () => {
      setupHandlers({ characterData: CHAR_DATA_ITEMS, drafts: [] });

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);
      await waitForLoaded();

      expect(screen.queryByTestId('discard-sheet-drafts')).not.toBeInTheDocument();
    });

    it('deletes every staged draft row and reverts the editor to published data', async () => {
      // The staged draft replaces the published Healing Potion with a Magic Sword.
      // Discarding must delete the row AND restore the published item in the editor.
      setupHandlers({ characterData: CHAR_DATA_ITEMS, drafts: DRAFT_ITEMS });
      const deleted: number[] = [];
      server.use(
        http.delete(
          'http://localhost:3000/api/v1/games/:gameId/results/:resultId/character-updates/:draftId',
          ({ params }) => {
            deleted.push(Number(params.draftId));
            return new HttpResponse(null, { status: 204 });
          },
        ),
      );

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);
      await waitForLoaded();

      fireEvent.click(screen.getByRole('button', { name: /inventory/i }));
      expect(await screen.findByText('Magic Sword')).toBeInTheDocument();

      fireEvent.click(screen.getByTestId('discard-sheet-drafts'));
      fireEvent.click(screen.getByTestId('confirm-discard-sheet-drafts'));

      await waitFor(() => {
        expect(deleted).toEqual([100]);
      });

      // Editor now shows the published sheet, not the discarded draft.
      await waitFor(() => {
        expect(screen.getByText('Healing Potion')).toBeInTheDocument();
      });
      expect(screen.queryByText('Magic Sword')).not.toBeInTheDocument();
    });

    it('keeps the staged updates when the confirmation is dismissed', async () => {
      setupHandlers({ characterData: CHAR_DATA_ITEMS, drafts: DRAFT_ITEMS });
      let deleteCalled = false;
      server.use(
        http.delete(
          'http://localhost:3000/api/v1/games/:gameId/results/:resultId/character-updates/:draftId',
          () => {
            deleteCalled = true;
            return new HttpResponse(null, { status: 204 });
          },
        ),
      );

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);
      await waitForLoaded();

      fireEvent.click(screen.getByTestId('discard-sheet-drafts'));
      fireEvent.click(screen.getByRole('button', { name: /^keep$/i }));

      expect(deleteCalled).toBe(false);
      expect(screen.getByTestId('discard-sheet-drafts')).toBeInTheDocument();
    });

    it('surfaces an error and keeps drafts when the delete fails', async () => {
      setupHandlers({ characterData: CHAR_DATA_ITEMS, drafts: DRAFT_ITEMS });
      server.use(
        http.delete(
          'http://localhost:3000/api/v1/games/:gameId/results/:resultId/character-updates/:draftId',
          () => new HttpResponse(null, { status: 500 }),
        ),
      );

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);
      await waitForLoaded();

      fireEvent.click(screen.getByTestId('discard-sheet-drafts'));
      fireEvent.click(screen.getByTestId('confirm-discard-sheet-drafts'));

      expect(
        await screen.findByText(/may not have been discarded/i)
      ).toBeInTheDocument();
    });

    it('re-seeds the editor from surviving rows when a delete fails partway through', async () => {
      // Regression: deletes run sequentially, so a mid-loop failure leaves rows behind.
      // handleDiscard used to return before re-seeding and left initialized.current set,
      // so the refetch onSettled triggered could not correct the editor either — it kept
      // showing the discarded values while the row still existed server-side, and the
      // next edit would write a snapshot built on that phantom state.
      //
      // The refetched row differs from the seeded one so that only a genuine re-seed can
      // produce it: asserting on the originally-seeded value would pass either way.
      const SURVIVING_ITEM = { ...ITEM_DRAFT, name: 'Surviving Sword' };
      let fetchCount = 0;
      server.use(
        http.get('http://localhost:3000/api/v1/characters/:id/data', () =>
          HttpResponse.json(CHAR_DATA_ITEMS),
        ),
        http.get(
          'http://localhost:3000/api/v1/games/:gameId/results/:resultId/character-updates',
          () => {
            // First read seeds the editor; the read after the failed discard reports
            // what actually survived server-side.
            fetchCount += 1;
            const value = fetchCount === 1 ? [ITEM_DRAFT] : [SURVIVING_ITEM];
            return HttpResponse.json([
              { ...DRAFT_ITEMS[0], field_value: JSON.stringify(value) },
            ]);
          },
        ),
        http.delete(
          'http://localhost:3000/api/v1/games/:gameId/results/:resultId/character-updates/:draftId',
          () => new HttpResponse(null, { status: 500 }),
        ),
      );

      renderWithProviders(<UpdateCharacterSheetModal {...BASE_PROPS} />);
      await waitForLoaded();

      fireEvent.click(screen.getByRole('button', { name: /inventory/i }));
      expect(await screen.findByText('Magic Sword')).toBeInTheDocument();

      fireEvent.click(screen.getByTestId('discard-sheet-drafts'));
      fireEvent.click(screen.getByTestId('confirm-discard-sheet-drafts'));

      await screen.findByText(/may not have been discarded/i);

      // The row survived, so the editor must show what is actually staged — neither the
      // pre-discard snapshot nor the published sheet it failed to revert to.
      expect(await screen.findByText('Surviving Sword')).toBeInTheDocument();
      expect(screen.queryByText('Healing Potion')).not.toBeInTheDocument();
    });
  });

  describe('Done button', () => {
    it('calls onClose when Done is clicked', async () => {
      const onClose = vi.fn();
      setupHandlers({ characterData: [], drafts: null });

      renderWithProviders(
        <UpdateCharacterSheetModal {...BASE_PROPS} onClose={onClose} />,
      );
      await waitForLoaded();

      fireEvent.click(screen.getByRole('button', { name: /done/i }));

      expect(onClose).toHaveBeenCalledOnce();
    });
  });

  /**
   * A nested item/skill editor keeps its edits in local state until its own Save
   * fires, so the modal never sees them and closing silently discards the GM's
   * typing. Reported by a GM who lost work by clicking Done instead of Save.
   */
  describe('Unsaved nested edits', () => {
    // Opens the inline editor on the seeded inventory item.
    async function openItemEditor() {
      await waitForLoaded();
      fireEvent.click(screen.getByRole('button', { name: /inventory/i }));
      await waitFor(() => {
        expect(screen.getByText('Healing Potion')).toBeInTheDocument();
      });
      fireEvent.click(screen.getByRole('button', { name: 'Edit item' }));
      await waitFor(() => {
        expect(screen.getByLabelText(/^Name/)).toBeInTheDocument();
      });
    }

    it('warns instead of closing when an editor has uncommitted edits', async () => {
      const onClose = vi.fn();
      setupHandlers({ characterData: CHAR_DATA_ITEMS, drafts: null });

      renderWithProviders(
        <UpdateCharacterSheetModal {...BASE_PROPS} onClose={onClose} />,
      );
      await openItemEditor();

      fireEvent.change(screen.getByLabelText(/^Name/), {
        target: { value: 'Healing Potion of Vigor' },
      });
      fireEvent.click(screen.getByRole('button', { name: /done/i }));

      expect(onClose).not.toHaveBeenCalled();
      expect(screen.getByTestId('confirm-close-unsaved')).toBeInTheDocument();
    });

    it('closes anyway when the warning is confirmed', async () => {
      const onClose = vi.fn();
      setupHandlers({ characterData: CHAR_DATA_ITEMS, drafts: null });

      renderWithProviders(
        <UpdateCharacterSheetModal {...BASE_PROPS} onClose={onClose} />,
      );
      await openItemEditor();

      fireEvent.change(screen.getByLabelText(/^Name/), {
        target: { value: 'Healing Potion of Vigor' },
      });
      fireEvent.click(screen.getByRole('button', { name: /done/i }));
      fireEvent.click(screen.getByTestId('confirm-close-discard'));

      expect(onClose).toHaveBeenCalledOnce();
    });

    it('keeps the modal and the typed text open when the GM chooses to keep editing', async () => {
      const onClose = vi.fn();
      setupHandlers({ characterData: CHAR_DATA_ITEMS, drafts: null });

      renderWithProviders(
        <UpdateCharacterSheetModal {...BASE_PROPS} onClose={onClose} />,
      );
      await openItemEditor();

      fireEvent.change(screen.getByLabelText(/^Name/), {
        target: { value: 'Healing Potion of Vigor' },
      });
      fireEvent.click(screen.getByRole('button', { name: /done/i }));
      fireEvent.click(screen.getByRole('button', { name: /keep editing/i }));

      expect(onClose).not.toHaveBeenCalled();
      expect(screen.queryByTestId('confirm-close-unsaved')).not.toBeInTheDocument();
      expect(screen.getByLabelText(/^Name/)).toHaveValue('Healing Potion of Vigor');
    });

    /**
     * The regression this change risks introducing: a modal that will not close.
     * Committing the edit must clear the flag so Done behaves normally again.
     */
    it('closes without warning after the nested edit is saved', async () => {
      const onClose = vi.fn();
      setupHandlers({ characterData: CHAR_DATA_ITEMS, drafts: null });

      renderWithProviders(
        <UpdateCharacterSheetModal {...BASE_PROPS} onClose={onClose} />,
      );
      await openItemEditor();

      fireEvent.change(screen.getByLabelText(/^Name/), {
        target: { value: 'Healing Potion of Vigor' },
      });
      fireEvent.click(screen.getByRole('button', { name: /^save$/i }));

      await waitFor(() => {
        expect(screen.queryByLabelText(/^Name/)).not.toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole('button', { name: /done/i }));

      expect(screen.queryByTestId('confirm-close-unsaved')).not.toBeInTheDocument();
      expect(onClose).toHaveBeenCalledOnce();
    });

    it('closes without warning after the nested edit is cancelled', async () => {
      const onClose = vi.fn();
      setupHandlers({ characterData: CHAR_DATA_ITEMS, drafts: null });

      renderWithProviders(
        <UpdateCharacterSheetModal {...BASE_PROPS} onClose={onClose} />,
      );
      await openItemEditor();

      fireEvent.change(screen.getByLabelText(/^Name/), {
        target: { value: 'Healing Potion of Vigor' },
      });
      fireEvent.click(screen.getByRole('button', { name: /cancel/i }));

      await waitFor(() => {
        expect(screen.queryByLabelText(/^Name/)).not.toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole('button', { name: /done/i }));

      expect(onClose).toHaveBeenCalledOnce();
    });

    /**
     * Regression: guarding the backdrop was implemented by disabling it outright, which
     * left the modal with no way out but "Done" even when there was nothing to protect.
     * Clicking away is how most people close a modal, so it has to keep working whenever
     * no editor holds uncommitted text.
     */
    it('closes on a backdrop click when nothing is uncommitted', async () => {
      const onClose = vi.fn();
      setupHandlers({ characterData: CHAR_DATA_ITEMS, drafts: null });

      renderWithProviders(
        <UpdateCharacterSheetModal {...BASE_PROPS} onClose={onClose} />,
      );
      await waitForLoaded();

      clickBackdrop();

      expect(onClose).toHaveBeenCalledOnce();
    });

    it('closes on a backdrop click with an editor open but untouched', async () => {
      const onClose = vi.fn();
      setupHandlers({ characterData: CHAR_DATA_ITEMS, drafts: null });

      renderWithProviders(
        <UpdateCharacterSheetModal {...BASE_PROPS} onClose={onClose} />,
      );
      await openItemEditor();

      clickBackdrop();

      expect(onClose).toHaveBeenCalledOnce();
    });

    it('ignores a backdrop click while an editor holds uncommitted edits', async () => {
      const onClose = vi.fn();
      setupHandlers({ characterData: CHAR_DATA_ITEMS, drafts: null });

      renderWithProviders(
        <UpdateCharacterSheetModal {...BASE_PROPS} onClose={onClose} />,
      );
      await openItemEditor();

      fireEvent.change(screen.getByLabelText(/^Name/), {
        target: { value: 'Healing Potion of Vigor' },
      });
      clickBackdrop();

      expect(onClose).not.toHaveBeenCalled();
      expect(screen.getByLabelText(/^Name/)).toHaveValue('Healing Potion of Vigor');
    });

    it('restores backdrop dismissal once the nested edit is saved', async () => {
      const onClose = vi.fn();
      setupHandlers({ characterData: CHAR_DATA_ITEMS, drafts: null });

      renderWithProviders(
        <UpdateCharacterSheetModal {...BASE_PROPS} onClose={onClose} />,
      );
      await openItemEditor();

      fireEvent.change(screen.getByLabelText(/^Name/), {
        target: { value: 'Healing Potion of Vigor' },
      });
      fireEvent.click(screen.getByRole('button', { name: /^save$/i }));

      await waitFor(() => {
        expect(screen.queryByLabelText(/^Name/)).not.toBeInTheDocument();
      });

      clickBackdrop();

      expect(onClose).toHaveBeenCalledOnce();
    });

    it('does not warn when an editor is open but untouched', async () => {
      const onClose = vi.fn();
      setupHandlers({ characterData: CHAR_DATA_ITEMS, drafts: null });

      renderWithProviders(
        <UpdateCharacterSheetModal {...BASE_PROPS} onClose={onClose} />,
      );
      await openItemEditor();

      fireEvent.click(screen.getByRole('button', { name: /done/i }));

      expect(onClose).toHaveBeenCalledOnce();
    });
  });

  describe('Re-initialization on reopen', () => {
    it('resets initialization when modal is closed and reopened', async () => {
      setupHandlers({ characterData: CHAR_DATA_SKILLS, drafts: null });

      const { rerender } = renderWithProviders(
        <UpdateCharacterSheetModal {...BASE_PROPS} />,
      );

      await waitFor(() => {
        expect(screen.getByText('Strength')).toBeInTheDocument();
      });

      // Close
      rerender(
        <UpdateCharacterSheetModal {...BASE_PROPS} isOpen={false} onClose={vi.fn()} />,
      );

      // Reopen — should re-initialize successfully
      rerender(
        <UpdateCharacterSheetModal {...BASE_PROPS} isOpen={true} onClose={vi.fn()} />,
      );

      await waitFor(() => {
        expect(screen.getByText('Strength')).toBeInTheDocument();
      });
    });
  });
});
