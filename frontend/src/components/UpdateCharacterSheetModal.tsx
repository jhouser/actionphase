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

  // Keep the pending save args in a ref so handleClose can flush them synchronously
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pendingSaveArgs = useRef<CreateDraftCharacterUpdateRequest | null>(null);

  const scheduleSave = useCallback((
    moduleType: string,
    fieldName: string,
    value: unknown,
  ) => {
    const args: CreateDraftCharacterUpdateRequest = {
      character_id: characterId,
      module_type: moduleType as CreateDraftCharacterUpdateRequest['module_type'],
      field_name: fieldName,
      field_value: JSON.stringify(value),
      field_type: 'json',
      operation: 'upsert',
    };

    if (saveTimer.current) clearTimeout(saveTimer.current);
    pendingSaveArgs.current = args;
    setSaveStatus('saving');
    saveTimer.current = setTimeout(() => {
      pendingSaveArgs.current = null;
      saveDraftMutation.mutate(args);
    }, 800);
  }, [characterId, saveDraftMutation]);

  // Clean up timer on unmount
  useEffect(() => {
    return () => {
      if (saveTimer.current) clearTimeout(saveTimer.current);
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

  // Flush any pending debounced save before closing so changes aren't lost
  const handleClose = () => {
    if (saveTimer.current) {
      clearTimeout(saveTimer.current);
      saveTimer.current = null;
    }
    if (pendingSaveArgs.current) {
      saveDraftMutation.mutate(pendingSaveArgs.current);
      pendingSaveArgs.current = null;
    }
    onClose();
  };

  return (
    <Modal isOpen={isOpen} onClose={handleClose}>
      <div className="space-y-4">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-border-primary pb-4">
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

        {/* Info Alert */}
        <Alert variant="info">
          Edit the character sheet below. Changes are saved as drafts and will be applied to the character when you publish the action result.
        </Alert>

        {saveError && (
          <Alert variant="danger" dismissible onDismiss={() => setSaveError(null)}>
            {saveError}
          </Alert>
        )}

        {/* Section Navigation */}
        <div className="border-b border-border-primary">
          <nav className="flex space-x-1" aria-label="Sections">
            {(['abilities', 'inventory'] as ActiveSection[]).map((section) => (
              <button
                key={section}
                onClick={() => setActiveSection(section)}
                className={`
                  px-4 py-2 text-sm font-medium rounded-t-lg transition-colors capitalize
                  ${activeSection === section
                    ? 'bg-bg-primary text-interactive-primary border-b-2 border-interactive-primary'
                    : 'text-content-secondary hover:text-content-primary hover:bg-bg-secondary'
                  }
                `}
              >
                {section}
              </button>
            ))}
          </nav>
        </div>

        {/* Content */}
        <div className="min-h-[400px]">
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
                />
              )}
            </>
          )}
        </div>

        {/* Footer */}
        <div className="flex justify-end border-t border-border-primary pt-4">
          <Button variant="secondary" onClick={handleClose}>
            Done
          </Button>
        </div>
      </div>
    </Modal>
  );
};
