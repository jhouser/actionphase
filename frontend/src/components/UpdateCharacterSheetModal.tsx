import { useState, useEffect, useRef, useCallback } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Modal } from './Modal';
import { Button, Alert, Spinner } from './ui';
import { AbilitiesManager } from './AbilitiesManager';
import { InventoryManager } from './InventoryManager';
import { apiClient } from '../lib/api';
import type { CharacterAbility, CharacterSkill, InventoryItem, CurrencyEntry } from '../types/characters';
import type { CreateDraftCharacterUpdateRequest } from '../types/phases';
import { logger } from '@/services/LoggingService';
import { useDiscardSheetDrafts } from '../hooks/useDiscardSheetDrafts';
import { useDirtyChildren } from '@/hooks/useDirtyChildren';
import { EditorLockNotice } from './EditorLockNotice';
import { ConfirmDiscardEdits } from './ConfirmDiscardEdits';

interface UpdateCharacterSheetModalProps {
  isOpen: boolean;
  onClose: () => void;
  gameId: number;
  actionResultId: number;
  characterId: number;
  characterName: string;
}

type ActiveSection = 'abilities' | 'inventory';

// Parse a JSON field value from character data, returning empty array on failure
function parseJsonArray<T>(value: string | undefined): T[] {
  if (!value) return [];
  try {
    const parsed = JSON.parse(value);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

export const UpdateCharacterSheetModal: React.FC<UpdateCharacterSheetModalProps> = ({
  isOpen,
  onClose,
  gameId,
  actionResultId,
  characterId,
  characterName,
}) => {
  const [activeSection, setActiveSection] = useState<ActiveSection>('abilities');
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saveStatus, setSaveStatus] = useState<'idle' | 'saving' | 'saved'>('idle');
  const [confirmingDiscard, setConfirmingDiscard] = useState(false);

  // An item/ability/skill/currency editor open below with edits not yet committed by
  // its own Save button. Those edits live in the child's local state and never reach
  // this modal, so closing would silently discard them — warn instead.
  //
  // Aggregated per manager rather than stored as one boolean: a shared setter would let
  // whichever manager reported last win, so a clean one could erase a dirty one's flag.
  const { isAnyDirty: hasUncommittedEdit, report: reportDirty } = useDirtyChildren();
  const [confirmingClose, setConfirmingClose] = useState(false);

  // Local state for the character sheet being edited
  const [abilities, setAbilities] = useState<CharacterAbility[]>([]);
  const [skills, setSkills] = useState<CharacterSkill[]>([]);
  const [items, setItems] = useState<InventoryItem[]>([]);
  const [currency, setCurrency] = useState<CurrencyEntry[]>([]);

  // Track whether local state has been initialized from server data
  const initialized = useRef(false);

  const queryClient = useQueryClient();

  // Load the character's current sheet data
  const { data: characterData, isLoading: isLoadingCharacterData } = useQuery({
    queryKey: ['characterData', characterId],
    queryFn: () => apiClient.characters.getCharacterData(characterId).then(res => res.data),
    enabled: isOpen && !!characterId,
  });

  // Load any existing drafts for this result — these represent the GM's last saved state
  // and take precedence over raw characterData when initializing the editor
  const { data: existingDrafts, isLoading: isLoadingDrafts } = useQuery({
    queryKey: ['draftCharacterUpdates', gameId, actionResultId],
    queryFn: () => apiClient.phases.getDraftCharacterUpdates(gameId, actionResultId).then(res => res.data ?? []),
    enabled: isOpen && !!actionResultId,
  });

  const isLoading = isLoadingCharacterData || isLoadingDrafts;

  // Initialize local state once per modal open, after both queries complete.
  // Drafts take precedence over characterData — they represent the most recent intended state
  // (e.g. a save that fired just before the modal closed may not yet be in characterData).
  useEffect(() => {
    if (!isOpen) {
      initialized.current = false;
      setConfirmingDiscard(false);
      setConfirmingClose(false);
      return;
    }
    if (initialized.current || isLoading || characterData === undefined || existingDrafts === undefined) return;

    // Treat null (no drafts yet) the same as empty array
    const drafts = existingDrafts ?? [];

    // Helper: prefer draft value if one exists for this field, else fall back to characterData
    const getDraftField = (moduleType: string, fieldName: string) =>
      drafts.find(d => d.module_type === moduleType && d.field_name === fieldName)?.field_value;

    const getCharacterField = (moduleType: string, fieldName: string) =>
      characterData.find(d => d.module_type === moduleType && d.field_name === fieldName)?.field_value;

    const getField = (moduleType: string, fieldName: string) =>
      getDraftField(moduleType, fieldName) ?? getCharacterField(moduleType, fieldName);

    setAbilities(parseJsonArray<CharacterAbility>(getField('abilities', 'abilities')));
    setSkills(parseJsonArray<CharacterSkill>(getField('skills', 'skills')));
    setItems(parseJsonArray<InventoryItem>(getField('inventory', 'items')));
    setCurrency(parseJsonArray<CurrencyEntry>(getField('currency', 'currency')));

    initialized.current = true;
  }, [isOpen, isLoading, characterData, existingDrafts]);

  // Mutation to upsert a single draft row (whole-array snapshot)
  const saveDraftMutation = useMutation({
    mutationFn: async (data: CreateDraftCharacterUpdateRequest) => {
      const response = await apiClient.phases.createDraftCharacterUpdate(gameId, actionResultId, data);
      return response.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['draftCharacterUpdates', gameId, actionResultId] });
      queryClient.invalidateQueries({ queryKey: ['draftUpdateCount', gameId, actionResultId] });
      setSaveStatus('saved');
      setSaveError(null);
    },
    onError: (err) => {
      logger.error('Failed to save draft character update', { error: err, gameId, actionResultId, characterId });
      setSaveError('Failed to save changes. Please try again.');
      setSaveStatus('idle');
    },
  });

  const discardMutation = useDiscardSheetDrafts(gameId, actionResultId);

  /**
   * Removes the staged row for one module instead of writing a snapshot to it.
   *
   * Used when an edit lands back on the character's published state — the GM added
   * something and then took it away. Upserting that snapshot would leave a draft that
   * changes nothing on publish yet still counts toward the update badge and trips the
   * sibling-conflict warning, so the sheet looks untouched while a warning insists
   * otherwise. Deleting keeps "I undid my edit" identical to "I never edited".
   *
   * Deliberately removing a pre-existing item is unaffected: that snapshot differs
   * from published and is staged normally.
   */
  const clearModuleDraftMutation = useMutation({
    mutationFn: async ({ moduleType, fieldName }: { moduleType: string; fieldName: string }) => {
      const response = await apiClient.phases.getDraftCharacterUpdates(gameId, actionResultId);
      const match = (response.data ?? []).find(
        d => d.module_type === moduleType && d.field_name === fieldName
      );
      // No row yet (edited and undone before any save fired) — nothing to clear.
      if (!match) return false;
      await apiClient.phases.deleteDraftCharacterUpdate(gameId, actionResultId, match.id);
      return true;
    },
    onSuccess: (removed) => {
      setSaveStatus('saved');
      setSaveError(null);
      if (!removed) return;
      queryClient.invalidateQueries({ queryKey: ['draftCharacterUpdates', gameId, actionResultId] });
      // Unkeyed: clearing this result's row can also clear a sibling's conflict warning.
      queryClient.invalidateQueries({ queryKey: ['draftUpdateCount'] });
    },
    onError: (err) => {
      logger.error('Failed to clear redundant draft character update', {
        error: err, gameId, actionResultId, characterId,
      });
      setSaveError('Failed to save changes. Please try again.');
      setSaveStatus('idle');
    },
  });

  // Debounce state is keyed per draft row, NOT shared across the editor. Drafts are
  // stored one row per (module_type, field_name), so a single shared timer let an edit
  // in one module cancel another module's pending save — silently dropping it, since
  // the flush refs were overwritten too. Each field now debounces independently.
  const saveTimers = useRef(new Map<string, ReturnType<typeof setTimeout>>());
  const pendingSaves = useRef(new Map<string, CreateDraftCharacterUpdateRequest>());
  // A redundant edit schedules a row *deletion* rather than a save, so closing during
  // the debounce window has to flush that too — otherwise the stale row survives.
  const pendingClears = useRef(new Map<string, { moduleType: string; fieldName: string }>());

  const scheduleSave = useCallback((
    moduleType: string,
    fieldName: string,
    value: unknown,
  ) => {
    const fieldKey = `${moduleType}:${fieldName}`;
    const args: CreateDraftCharacterUpdateRequest = {
      character_id: characterId,
      module_type: moduleType as CreateDraftCharacterUpdateRequest['module_type'],
      field_name: fieldName,
      field_value: JSON.stringify(value),
      field_type: 'json',
      operation: 'upsert',
    };

    // An edit that lands back on the published sheet clears the staged row rather than
    // staging a no-op. See clearModuleDraftMutation for why.
    const published = characterData?.find(
      d => d.module_type === moduleType && d.field_name === fieldName
    )?.field_value;
    const isRedundant =
      JSON.stringify(value) === JSON.stringify(parseJsonArray<unknown>(published));

    const existing = saveTimers.current.get(fieldKey);
    if (existing) clearTimeout(existing);

    // Exactly one of the two is pending for a given field at any time.
    if (isRedundant) {
      pendingSaves.current.delete(fieldKey);
      pendingClears.current.set(fieldKey, { moduleType, fieldName });
    } else {
      pendingClears.current.delete(fieldKey);
      pendingSaves.current.set(fieldKey, args);
    }

    setSaveStatus('saving');
    saveTimers.current.set(fieldKey, setTimeout(() => {
      saveTimers.current.delete(fieldKey);
      pendingSaves.current.delete(fieldKey);
      pendingClears.current.delete(fieldKey);
      if (isRedundant) {
        clearModuleDraftMutation.mutate({ moduleType, fieldName });
      } else {
        saveDraftMutation.mutate(args);
      }
    }, 800));
  }, [characterId, characterData, saveDraftMutation, clearModuleDraftMutation]);

  // Clean up timers on unmount
  useEffect(() => {
    const timers = saveTimers.current;
    return () => {
      timers.forEach(timer => clearTimeout(timer));
      timers.clear();
    };
  }, []);

  const handleAbilitiesChange = (newAbilities: CharacterAbility[]) => {
    setAbilities(newAbilities);
    scheduleSave('abilities', 'abilities', newAbilities);
  };

  const handleSkillsChange = (newSkills: CharacterSkill[]) => {
    setSkills(newSkills);
    scheduleSave('skills', 'skills', newSkills);
  };

  const handleItemsChange = (newItems: InventoryItem[], reloadOnly: boolean) => {
    setItems(newItems);
    if (!reloadOnly) {
      scheduleSave('inventory', 'items', newItems);
    } else {
      queryClient.invalidateQueries({ queryKey: ['characterData', characterId] });
    }
  };

  const handleCurrencyChange = (newCurrency: CurrencyEntry[]) => {
    setCurrency(newCurrency);
    scheduleSave('currency', 'currency', newCurrency);
  };

  const hasStagedUpdates = (existingDrafts?.length ?? 0) > 0;

  // Discard every staged update on this result and reset the editor to the character's
  // published sheet. Cancels any pending debounced save first: a queued snapshot firing
  // after the deletes would immediately re-stage what was just discarded.
  const handleDiscard = async () => {
    saveTimers.current.forEach(timer => clearTimeout(timer));
    saveTimers.current.clear();
    pendingSaves.current.clear();
    pendingClears.current.clear();

    try {
      await discardMutation.mutateAsync();
    } catch (err) {
      logger.error('Failed to discard staged character sheet updates', {
        error: err, gameId, actionResultId, characterId,
      });
      // Deletes run one at a time, so a failure partway through has still removed some
      // rows. Re-seeding from characterData here would claim every draft is gone; instead
      // release the init guard so the effect re-seeds from the list onSettled refetched.
      // Without this the editor keeps showing discarded values while rows survive
      // server-side, and the next edit writes a snapshot built on that phantom state.
      initialized.current = false;
      setConfirmingDiscard(false);
      setSaveStatus('idle');
      setSaveError('Some staged updates may not have been discarded. Reopen to check.');
      return;
    }

    // Re-seed from published character data now that the drafts are gone.
    const fromCharacter = (moduleType: string, fieldName: string) =>
      characterData?.find(d => d.module_type === moduleType && d.field_name === fieldName)?.field_value;

    setAbilities(parseJsonArray<CharacterAbility>(fromCharacter('abilities', 'abilities')));
    setSkills(parseJsonArray<CharacterSkill>(fromCharacter('skills', 'skills')));
    setItems(parseJsonArray<InventoryItem>(fromCharacter('inventory', 'items')));
    setCurrency(parseJsonArray<CurrencyEntry>(fromCharacter('currency', 'currency')));

    setConfirmingDiscard(false);
    setSaveStatus('idle');
    setSaveError(null);
  };

  // Flush every pending debounced write before closing so changes aren't lost. Each
  // field carries at most one pending save OR clear, and distinct fields target
  // distinct rows, so firing them together cannot race against each other.
  //
  // Firing these with .mutate() and unmounting immediately is safe: React Query keeps
  // onSuccess on the Mutation in the cache rather than the observer, so the invalidations
  // still run after this component is gone. (Only call-site callbacks passed to
  // mutate(vars, {...}) are dropped on unmount.)
  const handleClose = () => {
    setConfirmingClose(false);

    saveTimers.current.forEach(timer => clearTimeout(timer));
    saveTimers.current.clear();

    pendingSaves.current.forEach(args => saveDraftMutation.mutate(args));
    pendingSaves.current.clear();

    pendingClears.current.forEach(args => clearModuleDraftMutation.mutate(args));
    pendingClears.current.clear();

    onClose();
  };

  /**
   * Close request from "Done" or the backdrop. An open editor's uncommitted text is
   * invisible to this modal — it is never staged and never flushed — so closing on it
   * destroys the GM's typing. Ask first; every other close goes straight through.
   */
  // Drop the prompt if the edit it was asking about gets committed underneath it — the
  // editor is still on screen while the prompt shows, so its Save stays reachable.
  // Left alone, the footer would go on offering to discard work that is already saved.
  useEffect(() => {
    if (!hasUncommittedEdit) setConfirmingClose(false);
  }, [hasUncommittedEdit]);

  const requestClose = () => {
    if (hasUncommittedEdit) {
      setConfirmingClose(true);
      return;
    }
    handleClose();
  };

  return (
    // dismissOnBackdrop: with nothing uncommitted the backdrop closes normally — this
    // modal renders no title and so no X, making "Done" the only other way out. While an
    // editor holds uncommitted text the backdrop is withdrawn instead of routed through
    // requestClose: a stray click is not a decision worth raising a confirmation over.
    <Modal isOpen={isOpen} onClose={requestClose} dismissOnBackdrop={!hasUncommittedEdit}>
      {/* Bounded flex column so the footer's discard action stays visible: only the
          section content scrolls, not the whole dialog. Height accounts for the
          Modal's own max-h-[90vh] minus its padding. */}
      <div className="flex flex-col gap-4 max-h-[calc(90vh-3rem)]">
        {/* Header */}
        <div className="shrink-0 flex items-center justify-between border-b border-border-primary pb-4">
          <div>
            <h2 className="text-2xl font-semibold text-content-primary">
              Update Character Sheet
            </h2>
            <p className="text-sm text-content-secondary mt-1">{characterName}</p>
          </div>
          <div className="text-sm text-content-tertiary">
            {saveStatus === 'saving' && <span className="text-content-secondary">Saving…</span>}
            {saveStatus === 'saved' && <span className="text-semantic-success">Saved</span>}
          </div>
        </div>

        <div className="shrink-0 space-y-4">
        <Alert variant="info">
          Edit the character sheet below. Changes are saved as drafts and will be applied to the character when you publish the action result.
        </Alert>

        {saveError && (
          <Alert variant="danger" dismissible onDismiss={() => setSaveError(null)}>
            {saveError}
          </Alert>
        )}

        {/* Section Navigation — locked while an editor holds uncommitted edits, since
            switching unmounts that editor and destroys them. See EditorLockNotice. */}
        <div className="border-b border-border-primary">
          <nav className="flex items-center space-x-1" aria-label="Sections">
            {(['abilities', 'inventory'] as ActiveSection[]).map((section) => (
              <button
                key={section}
                disabled={hasUncommittedEdit}
                onClick={() => setActiveSection(section)}
                className={`
                  px-4 py-2 text-sm font-medium rounded-t-lg transition-colors capitalize
                  disabled:opacity-50 disabled:cursor-not-allowed
                  ${activeSection === section
                    ? 'bg-bg-primary text-interactive-primary border-b-2 border-interactive-primary'
                    : 'text-content-secondary hover:text-content-primary hover:bg-bg-secondary'
                  }
                `}
              >
                {section}
              </button>
            ))}
            {hasUncommittedEdit && <EditorLockNotice className="ml-2" />}
          </nav>
        </div>
        </div>

        {/* Content — the only scrolling region */}
        <div className="flex-1 min-h-0 overflow-y-auto">
          {isLoading ? (
            <div className="flex justify-center items-center py-16">
              <Spinner size="lg" />
            </div>
          ) : (
            <>
              {activeSection === 'abilities' && (
                <AbilitiesManager
                  abilities={abilities}
                  skills={skills}
                  canEdit={true}
                  onAbilitiesChange={handleAbilitiesChange}
                  onSkillsChange={handleSkillsChange}
                  onDirtyChange={(isDirty) => reportDirty('abilities', isDirty)}
                />
              )}

              {activeSection === 'inventory' && (
                <InventoryManager
                  characterId={characterId}
                  items={items}
                  currency={currency}
                  canEdit={true}
                  onItemsChange={handleItemsChange}
                  onCurrencyChange={handleCurrencyChange}
                  onDirtyChange={(isDirty) => reportDirty('inventory', isDirty)}
                />
              )}
            </>
          )}
        </div>

        {/* Footer */}
        <div className="shrink-0 flex justify-between items-center border-t border-border-primary pt-4">
          <div>
            {hasStagedUpdates && (
              confirmingDiscard ? (
                <div className="flex items-center gap-2">
                  <span className="text-sm text-content-secondary">Discard all staged updates?</span>
                  <Button
                    variant="danger"
                    size="sm"
                    onClick={handleDiscard}
                    disabled={discardMutation.isPending}
                    data-testid="confirm-discard-sheet-drafts"
                  >
                    {discardMutation.isPending ? 'Discarding…' : 'Discard'}
                  </Button>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => setConfirmingDiscard(false)}
                    disabled={discardMutation.isPending}
                  >
                    Keep
                  </Button>
                </div>
              ) : (
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => setConfirmingDiscard(true)}
                  data-testid="discard-sheet-drafts"
                  className="text-semantic-danger"
                >
                  <svg className="w-4 h-4 mr-1.5 inline-block align-text-bottom" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                  Discard staged updates
                </Button>
              )
            )}
          </div>
          {confirmingClose ? (
            <ConfirmDiscardEdits
              onDiscard={handleClose}
              onKeepEditing={() => setConfirmingClose(false)}
            />
          ) : (
            <div className="flex items-center gap-3">
              {hasUncommittedEdit && (
                <span
                  className="text-sm text-semantic-warning"
                  data-testid="unsaved-edit-hint"
                >
                  Unsaved edit open
                </span>
              )}
              <Button variant="secondary" onClick={requestClose}>
                Done
              </Button>
            </div>
          )}
        </div>
      </div>
    </Modal>
  );
};
