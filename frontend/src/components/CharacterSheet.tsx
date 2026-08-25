import { useState, useEffect, useMemo } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import type { CharacterData, CharacterDataRequest, CharacterSkill, InventoryItem, NumberEntry, CharacterSheetConfig } from '../types/characters';
import { buildCharacterModules } from '../types/characters';
import { SkillsManager } from './SkillsManager';
import { ItemsManager } from './ItemsManager';
import { NumbersManager } from './NumbersManager';
import CharacterAvatar from './CharacterAvatar';
import AvatarUploadModal from './AvatarUploadModal';
import { useOptionalGameContext } from '../contexts/GameContext';
import { Modal } from './Modal';
import { TabNavigation } from './TabNavigation';
import type { Tab } from './TabNavigation';
import { Button, Badge, Input } from './ui';
import { useRenameCharacter } from '../hooks/useCharacters';
import { MarkdownPreview } from './MarkdownPreview';
import { CommentEditor } from './CommentEditor';
import { MessageCharacterButton } from './MessageCharacterButton';
import { useDirtyChildren } from '@/hooks/useDirtyChildren';
import { useSheetLabels } from '../hooks/useSheetLabels';
import { EditorLockNotice } from './EditorLockNotice';
import { ConfirmDiscardEdits } from './ConfirmDiscardEdits';

interface CharacterSheetProps {
  characterId: number;
  canEdit?: boolean;
  canEditStats?: boolean; // Separate permission for abilities, skills, items, currency (GM only)
  onClose?: () => void;
  isAnonymous?: boolean; // Whether the game is in anonymous mode
  userRole?: string; // User's role in the game ('gm', 'player', 'audience')
  gameState?: string; // Current game state (e.g. 'completed')
  /**
   * Reports whether an editor inside the sheet holds edits its Save has not committed.
   *
   * The sheet guards its own close paths. A backdrop click never reaches this component,
   * so the Modals wrapping the sheet need this signal to guard theirs: they leave
   * backdrop dismiss on until it reports true.
   */
  onDirtyChange?: (isDirty: boolean) => void;
  /**
   * Whether to show the avatar as a portrait rather than a circle. Normally
   * read from GameContext; pass it explicitly when rendering outside a
   * GameProvider (the global Utility Drawer), where there is none to read.
   */
  portraitAvatars?: boolean;
  /**
   * That game's character sheet tab labels. Normally read from GameContext;
   * pass it explicitly when rendering outside a GameProvider (the global
   * Utility Drawer), where there is none to read.
   *
   * Falling back to defaults out there would be wrong rather than merely
   * incomplete: a player whose GM renamed a tab to "Stress" would see
   * "Numbers" in the drawer and read the difference as a bug. So the drawer
   * carries the real config per character (see `game_character_sheet`).
   */
  sheetConfig?: CharacterSheetConfig;
}

/**
 * Module tabs rendered by a manager component rather than the generic field
 * list. Each manager heads itself, so the sheet skips its own module header for
 * these — keep this in step with the manager branch in the render body.
 */
const MANAGED_MODULE_TYPES = new Set(['skills', 'inventory', 'numbers']);

export function CharacterSheet({ characterId, canEdit = false, canEditStats = false, onClose, isAnonymous = false, userRole, gameState, portraitAvatars, sheetConfig, onDirtyChange }: CharacterSheetProps) {
  const gameContext = useOptionalGameContext();
  const portraitMode = portraitAvatars ?? gameContext?.game?.portrait_avatars ?? false;

  // Same precedence as portraitMode above: an explicit prop wins, then the game
  // in context, then the defaults the hook owns.
  //
  const sheetLabels = useSheetLabels(
    sheetConfig ? { character_sheet: sheetConfig } : gameContext?.game
  );

  // Rebuilt only when a label actually changes: the tab list is derived data,
  // and a fresh array each render would remount the active manager underneath
  // an open editor.
  const modules = useMemo(() => buildCharacterModules(sheetLabels), [sheetLabels]);

  const [activeModule, setActiveModule] = useState('bio');
  const [editingField, setEditingField] = useState<string | null>(null);
  // Text of the one field being edited right now, or null when nothing is open.
  // Deliberately a single value rather than a map keyed by field: only one editor
  // can be open at a time (`editingField`), so a map would just be a store of
  // stale text for fields nobody is editing — which is what it used to be, and
  // what made the saved values and the in-progress edit impossible to tell apart.
  const [editDraft, setEditDraft] = useState<string | null>(null);
  const [isAvatarModalOpen, setIsAvatarModalOpen] = useState(false);
  const [isDeleteAvatarDialogOpen, setIsDeleteAvatarDialogOpen] = useState(false);
  const [isEditingName, setIsEditingName] = useState(false);
  const [newName, setNewName] = useState('');

  // An ability/skill/item/currency editor open below with edits its own Save has not
  // committed. Those edits live in the child's local state and never reach this
  // component, so closing the sheet would silently drop them — confirm first.
  //
  // Aggregated per manager rather than stored as one boolean: a shared setter would let
  // whichever manager reported last win, so a clean one could erase a dirty one's flag.
  const { isAnyDirty: hasUncommittedEdit, report: reportDirty } = useDirtyChildren(onDirtyChange);
  const [confirmingClose, setConfirmingClose] = useState(false);

  /**
   * Close request from a path that would unmount the sheet and destroy an open editor's
   * uncommitted text. Confirms first when there is something to lose.
   *
   * The backdrop of the Modal wrapping this sheet is not ours to intercept — parents
   * watch onDirtyChange and withdraw backdrop dismiss once there is something to lose,
   * so a stray click cannot bypass this.
   */
  const requestClose = () => {
    if (!onClose) return;
    if (hasUncommittedEdit) {
      setConfirmingClose(true);
      return;
    }
    onClose();
  };

  // Drop the prompt if the edit it was asking about gets committed underneath it — the
  // editor is still on screen while the prompt shows, so its Save stays reachable.
  // Left alone, the header would go on offering to discard work that is already saved.
  useEffect(() => {
    if (!hasUncommittedEdit) setConfirmingClose(false);
  }, [hasUncommittedEdit]);

  const queryClient = useQueryClient();
  const renameMutation = useRenameCharacter();

  // Participants can view all private data if the game is completed or they are audience
  const canViewPrivate = canEdit || userRole === 'audience' || gameState === 'completed';

  // If user cannot view private data and is viewing a restricted module, switch to bio
  useEffect(() => {
    if (!canViewPrivate && activeModule !== 'bio') {
      setActiveModule('bio');
    }
  }, [canViewPrivate, activeModule]);

  const { data: character } = useQuery({
    queryKey: ['character', characterId],
    queryFn: () => apiClient.characters.getCharacter(characterId).then(res => res.data),
    enabled: !!characterId
  });

  const { data: characterData = [], isLoading } = useQuery({
    queryKey: ['characterData', characterId],
    queryFn: () => apiClient.characters.getCharacterData(characterId).then(res => res.data),
    enabled: !!characterId
  });

  const saveCharacterDataMutation = useMutation({
    mutationFn: (data: CharacterDataRequest) =>
      apiClient.characters.setCharacterData(characterId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['characterData', characterId] });
      setEditingField(null);
      setEditDraft(null);
    }
  });

  const deleteAvatarMutation = useMutation({
    mutationFn: () => apiClient.characters.deleteCharacterAvatar(characterId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['character', characterId] });
      setIsDeleteAvatarDialogOpen(false);
    }
  });

  // Handle character name editing
  const handleStartEditingName = () => {
    if (character) {
      setNewName(character.name);
      setIsEditingName(true);
    }
  };

  const handleCancelEditingName = () => {
    setIsEditingName(false);
    setNewName('');
  };

  const handleSaveName = () => {
    if (!newName.trim() || !character) return;

    renameMutation.mutate(
      { characterId, name: newName.trim() },
      {
        onSuccess: () => {
          setIsEditingName(false);
          setNewName('');
        }
      }
    );
  };

  // Saved field values, keyed `${module_type}_${field_name}`.
  //
  // Derived, not stored. Copying this into state via an effect is what produced
  // the `Maximum update depth exceeded` loop: the query's `= []` default is a new
  // array on every render, so the effect's dependency never compared equal, and it
  // set a freshly-built object each time, so React's identical-state bail-out never
  // fired either. Render -> effect -> setState -> render, until React's 50-update
  // cap cut it off. Invisible in the browser (the sheet still painted) but it hung
  // component tests, which start in exactly the unresolved-query state that spins
  // hardest. Deriving removes the cycle rather than damping it.
  const fieldValues = useMemo(() => {
    const values: Record<string, string> = {};
    characterData.forEach(data => {
      const key = `${data.module_type}_${data.field_name}`;
      values[key] = data.field_value || '';
    });
    return values;
  }, [characterData]);

  // Get saved field value for display. The open editor's uncommitted text lives in
  // `editDraft`, not here, so this keeps returning what is actually persisted.
  const getFieldValue = (moduleType: string, fieldName: string): string => {
    const key = `${moduleType}_${fieldName}`;
    return fieldValues[key] || '';
  };

  // Parse JSON field values for abilities and inventory
  const parseJsonField = (moduleType: string, fieldName: string): unknown => {
    const value = getFieldValue(moduleType, fieldName);
    if (!value) return [];
    try {
      return JSON.parse(value);
    } catch {
      return [];
    }
  };

  // Save JSON field values
  const saveJsonField = (moduleType: string, fieldName: string, data: unknown) => {
    const value = JSON.stringify(data);
    saveCharacterDataMutation.mutate({
      module_type: moduleType,
      field_name: fieldName,
      field_value: value,
      field_type: 'json',
      is_public: false // abilities/skills/items/currency access is gated at the tab level, not by is_public
    });
  };

  // Get field data object
  const getFieldData = (moduleType: string, fieldName: string): CharacterData | undefined => {
    return characterData.find(data =>
      data.module_type === moduleType && data.field_name === fieldName
    );
  };

  // Handle field edit
  const handleFieldEdit = (moduleType: string, fieldName: string) => {
    if (!canEdit) return;
    setEditingField(`${moduleType}_${fieldName}`);
    setEditDraft(getFieldValue(moduleType, fieldName));
  };

  // Handle field edit cancel
  const handleFieldCancel = () => {
    setEditingField(null);
    setEditDraft(null);
  };

  // Handle field save
  const handleFieldSave = (moduleType: string, fieldName: string, fieldType: string, isPublic: boolean) => {
    const value = editDraft ?? '';

    saveCharacterDataMutation.mutate({
      module_type: moduleType,
      field_name: fieldName,
      field_value: value,
      field_type: fieldType as 'text' | 'number' | 'boolean' | 'json',
      is_public: isPublic
    });
  };

  // Handle field value change

  if (isLoading) {
    return (
      <div className="surface-base rounded-lg shadow p-6">
        <div className="animate-pulse">
          <div className="h-8 surface-sunken rounded mb-4"></div>
          <div className="space-y-3">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="h-16 surface-sunken rounded"></div>
            ))}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="surface-base rounded-lg shadow-lg min-h-[600px] flex flex-col max-w-full">
      {/* Pinned to the top of the modal's scroll container so the close button
          and module tabs stay reachable on a long sheet — on mobile a bio full
          of text otherwise scrolls the only way out of the sheet off screen.
          Sticky resolves against the nearest scrolling ancestor, which is the
          Modal panel (max-h-[90vh] overflow-y-auto), not this component: the
          sheet root is sized by its content and never scrolls itself. That is
          also why the root can carry no `overflow-hidden` — it would become
          that ancestor and pin the header to a box that never moves. */}
      <div className="sticky top-0 z-10 surface-base rounded-t-lg border-b border-theme-default">
        <div className="flex justify-between items-start p-2 sm:p-4 md:p-8 gap-2 sm:gap-3">
          <div className="flex items-start gap-3 md:gap-6 min-w-0 flex-1">
            {/* Character Avatar */}
            {character && (
              <div className="relative flex-shrink-0">
                <CharacterAvatar
                  avatarUrl={character.avatar_url}
                  characterName={character.name}
                  size="xl"
                  shape={portraitMode ? 'portrait' : 'circle'}
                  className={portraitMode ? '' : 'w-14 h-14 sm:w-20 sm:h-20 md:w-32 md:h-32'}
                />
                {canEdit && (
                  <>
                    <Button
                      variant="primary"
                      size="sm"
                      onClick={() => setIsAvatarModalOpen(true)}
                      className="absolute -bottom-1 -right-1 rounded-full p-1.5 shadow-lg"
                      title="Upload Avatar"
                    >
                      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z" />
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 13a3 3 0 11-6 0 3 3 0 016 0z" />
                      </svg>
                    </Button>
                    {character.avatar_url && (
                      <Button
                        variant="danger"
                        size="sm"
                        onClick={() => setIsDeleteAvatarDialogOpen(true)}
                        className="absolute -bottom-1 -left-1 rounded-full p-1.5 shadow-lg"
                        title="Delete Avatar"
                      >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                      </Button>
                    )}
                  </>
                )}
              </div>
            )}
            <div className="min-w-0 flex-1">
              {isEditingName ? (
                <div className="flex items-center gap-2 mb-1">
                  <Input
                    value={newName}
                    onChange={(e) => setNewName(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') handleSaveName();
                      if (e.key === 'Escape') handleCancelEditingName();
                    }}
                    className="text-lg md:text-2xl font-bold"
                    autoFocus
                  />
                  <Button
                    variant="primary"
                    size="sm"
                    onClick={handleSaveName}
                    disabled={!newName.trim() || renameMutation.isPending}
                  >
                    Save
                  </Button>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={handleCancelEditingName}
                    disabled={renameMutation.isPending}
                  >
                    Cancel
                  </Button>
                </div>
              ) : (
                <div className="flex items-center gap-1 sm:gap-2 mb-1">
                  {/* Wraps rather than truncates: the header sits at the top of
                      a modal with nothing positioned against its height, so a
                      second line costs only vertical space, whereas truncation
                      loses the name outright ("Detective ..."). break-words
                      catches a long unbroken name that would otherwise overflow.
                      Truncates from md: up, where names fit on one line. */}
                  <h2 className="min-w-0 text-lg md:text-2xl font-bold text-content-primary break-words md:truncate">
                    {character?.name || 'Character Sheet'}
                  </h2>
                  {canEdit && character && (
                    <button
                      onClick={handleStartEditingName}
                      className="text-content-tertiary hover:text-content-primary transition-colors p-1 flex-shrink-0"
                      title="Rename character"
                    >
                      <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
                        <path d="M13.586 3.586a2 2 0 112.828 2.828l-.793.793-2.828-2.828.793-.793zM11.379 5.793L3 14.172V17h2.828l8.38-8.379-2.83-2.828z" />
                      </svg>
                    </button>
                  )}
                  {/* Dismiss the sheet on the way out — it's usually a modal,
                      and would otherwise sit over the messages page. Leaving for
                      the messages page unmounts the sheet just as surely as
                      closing it, so an open editor's edits get the same
                      confirmation; returning false holds the navigation. */}
                  <MessageCharacterButton
                    character={character}
                    onNavigate={() => {
                      if (hasUncommittedEdit) {
                        setConfirmingClose(true);
                        return false;
                      }
                      onClose?.();
                    }}
                  />
                </div>
              )}
              {/* Hidden on mobile: the badges cost a full row of the header
                  while the character name above them is truncating for want of
                  that same width. */}
              {character && (
                <div className="hidden sm:flex items-center gap-2 flex-wrap">
                  {/* Hide character type badge only for players in anonymous mode (GMs and audience can see it) */}
                  {!(isAnonymous && userRole === 'player') && (
                    <Badge variant="primary" size="sm">
                      {character.character_type?.replace('_', ' ')}
                    </Badge>
                  )}
                  <Badge variant={character.status === 'approved' ? 'success' : 'warning'} size="sm">
                    {character.status}
                  </Badge>
                </div>
              )}
            </div>
          </div>
          {onClose && (
            confirmingClose ? (
              <ConfirmDiscardEdits
                onDiscard={onClose}
                onKeepEditing={() => setConfirmingClose(false)}
                // The sheet header is cramped beside a truncating character name.
                hideTextOnMobile
              />
            ) : (
              <Button
                variant="ghost"
                size="sm"
                onClick={requestClose}
                className="text-content-tertiary hover:text-content-secondary h-auto p-2 flex-shrink-0"
              >
                <svg className="w-5 h-5 md:w-6 md:h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </Button>
            )
          )}
        </div>

        {/* Module Tabs - Filter out modules user cannot view */}
        <div data-testid="character-sheet-module-tabs">
          <TabNavigation
            tabs={modules.filter((module) => {
              // Bio is always visible (public information)
              if (module.type === 'bio') return true;
              // Private modules visible to editors (GM, owner), audience members, and all participants in completed games
              return canViewPrivate;
            }).map((module): Tab => ({
              id: module.type,
              label: module.name
            }))}
            activeTab={activeModule}
            onTabChange={setActiveModule}
            // Held while an editor below has uncommitted edits: switching modules
            // unmounts it and destroys them. See EditorLockNotice below.
            disabled={hasUncommittedEdit}
          />
          {hasUncommittedEdit && (
            <div className="px-2 pb-2">
              <EditorLockNotice />
            </div>
          )}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-2 sm:p-4 md:p-8">
        {modules.filter(module => {
          // Only render modules the user has permission to view
          if (module.type === 'bio') return true;
          return canViewPrivate;
        }).filter(module => module.type === activeModule).map((module) => (
          <div key={module.type} className="max-w-4xl mx-auto">
            {/* Only the text modules get a header here. The three stat managers
                render their own heading (the modal embeds them without this
                block and relies on it), so repeating the module name above them
                printed it twice — plus a description that only restated it
                ("Skills" / "Character skills"). */}
            {!MANAGED_MODULE_TYPES.has(module.type) && (
              <div className="mb-4 md:mb-6">
                <h3 className="text-lg md:text-xl font-semibold text-content-primary mb-2">{module.name}</h3>
                <p className="text-sm md:text-base text-content-secondary">{module.description}</p>
              </div>
            )}

            {/* One manager per stat tab. Each reports its own dirty state under its
                own key, so a clean manager cannot clear a dirty one's flag. */}
            {module.type === 'skills' ? (
              <SkillsManager
                skills={parseJsonField('skills', 'skills') as CharacterSkill[]}
                canEdit={canEditStats}
                onSkillsChange={(skills) => saveJsonField('skills', 'skills', skills)}
                onDirtyChange={(isDirty) => reportDirty('skills', isDirty)}
                label={sheetLabels.skills}
              />
            ) : module.type === 'inventory' ? (
              <ItemsManager
                characterId={characterId}
                items={parseJsonField('inventory', 'items') as InventoryItem[]}
                canEdit={canEditStats}
                onItemsChange={(items, reloadOnly) => { if (!reloadOnly) saveJsonField('inventory', 'items', items); else queryClient.invalidateQueries({ queryKey: ['characterData', characterId] }); }}
                onDirtyChange={(isDirty) => reportDirty('inventory', isDirty)}
                label={sheetLabels.inventory}
              />
            ) : module.type === 'numbers' ? (
              <NumbersManager
                numbers={parseJsonField('numbers', 'numbers') as NumberEntry[]}
                canEdit={canEditStats}
                onNumbersChange={(numbers) => saveJsonField('numbers', 'numbers', numbers)}
                onDirtyChange={(isDirty) => reportDirty('numbers', isDirty)}
                label={sheetLabels.numbers}
              />
            ) : (
              /* Regular text-based fields for bio and notes modules */
              <div className="space-y-6">
                {module.fields.map((field) => {
                  const key = `${module.type}_${field.name}`;
                  const fieldData = getFieldData(module.type, field.name);
                  const value = getFieldValue(module.type, field.name);
                  const isEditing = editingField === key;

                  // Hide private fields if user cannot view private data
                  // If fieldData exists, use its is_public value; otherwise fall back to field.isPublic
                  const isFieldPublic = fieldData ? fieldData.is_public : (field.isPublic ?? true);
                  if (!canViewPrivate && !isFieldPublic) {
                    return null; // Don't render private fields for viewers
                  }

                  return (
                    <div key={field.name} className="border border-theme-default rounded-lg p-3 sm:p-4 md:p-6 bg-surface-raised shadow-sm">
                      <div className="flex justify-between items-start mb-3 md:mb-4 gap-2">
                        <div className="flex-1 min-w-0">
                          <label className="block text-sm md:text-base font-semibold text-content-primary mb-1">
                            {field.label}
                            {field.required && <span className="text-semantic-danger ml-1">*</span>}
                            <span className="text-xs text-content-tertiary font-normal ml-2">
                              (Markdown supported)
                            </span>
                          </label>
                          {fieldData && (
                            <div className="flex items-center flex-wrap gap-2 md:gap-3 mt-2">
                              <Badge variant={isFieldPublic ? 'success' : 'warning'} size="sm">
                                {isFieldPublic ? 'Public' : 'Private'}
                              </Badge>
                              <span className="text-xs text-content-tertiary">
                                Updated {new Date(fieldData.updated_at).toLocaleDateString()}
                              </span>
                            </div>
                          )}
                        </div>

                        {canEdit && !isEditing && (
                          <Button
                            variant="secondary"
                            size="sm"
                            onClick={() => handleFieldEdit(module.type, field.name)}
                            className="flex-shrink-0"
                          >
                            <svg className="w-4 h-4 md:mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                            </svg>
                            <span className="hidden md:inline">Edit</span>
                          </Button>
                        )}
                      </div>

                      {isEditing ? (
                        <div className="space-y-4">
                          <CommentEditor
                            value={editDraft ?? ''}
                            onChange={setEditDraft}
                            placeholder={field.placeholder}
                            rows={8}
                            showPreviewByDefault={false}
                          />
                          <div className="flex justify-end space-x-3">
                            <Button
                              variant="ghost"
                              size="md"
                              onClick={handleFieldCancel}
                              disabled={saveCharacterDataMutation.isPending}
                            >
                              Cancel
                            </Button>
                            <Button
                              variant="primary"
                              size="md"
                              onClick={() => handleFieldSave(
                                module.type,
                                field.name,
                                field.type,
                                field.isPublic ?? true
                              )}
                              disabled={saveCharacterDataMutation.isPending}
                              loading={saveCharacterDataMutation.isPending}
                            >
                              {saveCharacterDataMutation.isPending ? 'Saving...' : 'Save Changes'}
                            </Button>
                          </div>
                        </div>
                      ) : (
                        <div className="mt-3">
                          {value ? (
                            <MarkdownPreview content={value} />
                          ) : (
                            <div className="text-base text-content-tertiary italic py-8 text-center">
                              {canEdit ? field.placeholder || 'Click "Edit" to add content...' : 'No content yet...'}
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        ))}

        {/* Error Display */}
        {saveCharacterDataMutation.error && (
          <div className="mt-4 p-3 bg-semantic-danger-subtle border border-semantic-danger rounded-lg">
            <p className="text-sm text-semantic-danger">
              Failed to save: {
                saveCharacterDataMutation.error instanceof Error
                  ? saveCharacterDataMutation.error.message
                  : 'Unknown error'
              }
            </p>
          </div>
        )}
      </div>

      {/* Avatar Upload Modal */}
      {character && (
        <AvatarUploadModal
          isOpen={isAvatarModalOpen}
          onClose={() => setIsAvatarModalOpen(false)}
          characterId={character.id}
          characterName={character.name}
          currentAvatarUrl={character.avatar_url}
          portraitMode={portraitMode}
          onUploadSuccess={() => {
            // Refetch character data to immediately show new avatar
            queryClient.refetchQueries({ queryKey: ['character', characterId] });
          }}
        />
      )}

      {/* Delete Avatar Confirmation Dialog */}
      {character && (
        <Modal
          isOpen={isDeleteAvatarDialogOpen}
          onClose={() => setIsDeleteAvatarDialogOpen(false)}
          title="Delete Avatar"
        >
          <div className="space-y-4">
            {/* Warning message */}
            <div className="bg-semantic-error/10 border border-semantic-error rounded-lg p-4">
              <h3 className="font-semibold text-content-primary mb-2">
                ⚠️ Confirm Removal
              </h3>
              <p className="text-content-secondary text-sm">
                <strong>{character.name}</strong> will show initials instead of an
                avatar from now on. Existing posts keep the avatar they were
                written with.
              </p>
            </div>

            {/* Action buttons */}
            <div className="flex gap-3 justify-end pt-4">
              <Button
                variant="secondary"
                onClick={() => setIsDeleteAvatarDialogOpen(false)}
                disabled={deleteAvatarMutation.isPending}
              >
                Cancel
              </Button>
              <Button
                variant="danger"
                onClick={() => deleteAvatarMutation.mutate()}
                disabled={deleteAvatarMutation.isPending}
                loading={deleteAvatarMutation.isPending}
              >
                {deleteAvatarMutation.isPending ? 'Deleting...' : 'Delete Avatar'}
              </Button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
}
