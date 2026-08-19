import { useState, useCallback } from 'react';
import type { GameWithDetails, CreateGameRequest } from '../types/games';
import type { CharacterSheetConfig } from '../types/characters';
import type { GameFormData } from '../components/GameFormFields';
import { convertToISO8601, formatDateTimeLocal } from '../lib/utils/dates';
import { useUploadGameBanner, useDeleteGameBanner } from './useGameBanner';

const BLANK_FORM_DATA: GameFormData = {
  title: '',
  description: '',
  genre: '',
  max_players: 6,
  recruitment_deadline: '',
  start_date: '',
  end_date: '',
  is_anonymous: false,
  auto_accept_audience: false,
  allow_group_conversations: true,
  portrait_avatars: true,
  sheet_label_skills: '',
  sheet_label_inventory: '',
  sheet_label_numbers: '',
  common_room_open_day: '',
  common_room_open_time: '',
  common_room_close_day: '',
  common_room_close_time: '',
};

export function gameToFormData(game: GameWithDetails): GameFormData {
  return {
    title: game.title || '',
    description: game.description || '',
    genre: game.genre || '',
    max_players: game.max_players || '',
    recruitment_deadline: game.recruitment_deadline ? formatDateTimeLocal(game.recruitment_deadline) : '',
    start_date: game.start_date ? formatDateTimeLocal(game.start_date) : '',
    end_date: game.end_date ? formatDateTimeLocal(game.end_date) : '',
    is_anonymous: game.is_anonymous || false,
    auto_accept_audience: game.auto_accept_audience || false,
    allow_group_conversations: game.allow_group_conversations ?? true,
    portrait_avatars: game.portrait_avatars ?? false,
    // Only genuine overrides come back from the server, so an absent label
    // hydrates as an empty box — which is exactly how the GM left it, and what
    // makes the placeholder show the default again.
    sheet_label_skills: game.character_sheet?.labels?.skills ?? '',
    sheet_label_inventory: game.character_sheet?.labels?.inventory ?? '',
    sheet_label_numbers: game.character_sheet?.labels?.numbers ?? '',
    common_room_open_day: game.common_room_open_day ?? '',
    common_room_open_time: game.common_room_open_time ? game.common_room_open_time.slice(0, 5) : '',
    common_room_close_day: game.common_room_close_day ?? '',
    common_room_close_time: game.common_room_close_time ? game.common_room_close_time.slice(0, 5) : '',
  };
}

export interface BuildPayloadResult {
  payload: CreateGameRequest | null;
  error: string | null;
}

export interface UploadPendingBannerCallbacks {
  onSuccess?: () => void;
  onError?: () => void;
}

/**
 * Folds the form's three flat label fields back into the sparse `character_sheet`
 * wire shape.
 *
 * Empty and whitespace-only boxes are dropped rather than sent as "": a blank
 * box means "use the default", which on the wire is spelled *absent*. The
 * backend would accept "" too (it trims and treats whitespace-only as "no
 * override"), but sending it would put two spellings of the same state on the
 * wire and store keys the GM never set. When nothing is overridden
 * the whole key is omitted, so a game with no customisation sends nothing at
 * all rather than an empty object.
 */
function buildCharacterSheetConfig(formData: GameFormData): CharacterSheetConfig | undefined {
  const labels: NonNullable<CharacterSheetConfig['labels']> = {};

  const skills = formData.sheet_label_skills?.trim();
  if (skills) labels.skills = skills;

  const inventory = formData.sheet_label_inventory?.trim();
  if (inventory) labels.inventory = inventory;

  const numbers = formData.sheet_label_numbers?.trim();
  if (numbers) labels.numbers = numbers;

  return Object.keys(labels).length > 0 ? { labels } : undefined;
}

export function useGameForm(initialData?: GameWithDetails) {
  const [formData, setFormData] = useState<GameFormData>(() =>
    initialData ? gameToFormData(initialData) : { ...BLANK_FORM_DATA }
  );
  // The state the form was opened in, for the unsaved-edit comparison. Held in
  // state rather than recomputed so it survives re-renders, and so Edit's
  // re-hydration effect can move the baseline when it reloads the game — a
  // fresh hydration is not an edit.
  const [initialFormData, setInitialFormData] = useState<GameFormData>(formData);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [pendingBannerFile, setPendingBannerFile] = useState<File | null>(null);
  const [bannerPreviewUrl, setBannerPreviewUrl] = useState<string | null>(null);

  const uploadBanner = useUploadGameBanner();
  const deleteBanner = useDeleteGameBanner();

  const handleChange = useCallback(
    (field: keyof GameFormData, value: string | number | boolean) => {
      setFormData(prev => ({ ...prev, [field]: value }));
    },
    []
  );

  const handleBannerFileSelect = useCallback((file: File) => {
    setBannerPreviewUrl(prev => {
      if (prev) URL.revokeObjectURL(prev);
      return URL.createObjectURL(file);
    });
    setPendingBannerFile(file);
  }, []);

  const discardPendingBanner = useCallback(() => {
    setBannerPreviewUrl(prev => {
      if (prev) URL.revokeObjectURL(prev);
      return null;
    });
    setPendingBannerFile(null);
  }, []);

  const uploadPendingBanner = useCallback(
    (gameId: number, callbacks: UploadPendingBannerCallbacks) => {
      if (!pendingBannerFile) {
        callbacks.onSuccess?.();
        return;
      }
      uploadBanner.mutate(
        { gameId, file: pendingBannerFile },
        {
          onSuccess: () => {
            setBannerPreviewUrl(prev => {
              if (prev) URL.revokeObjectURL(prev);
              return null;
            });
            setPendingBannerFile(null);
            callbacks.onSuccess?.();
          },
          onError: () => {
            setBannerPreviewUrl(prev => {
              if (prev) URL.revokeObjectURL(prev);
              return null;
            });
            setPendingBannerFile(null);
            callbacks.onError?.();
          },
        }
      );
    },
    [pendingBannerFile, uploadBanner]
  );

  const buildApiPayload = useCallback((): BuildPayloadResult => {
    if (!formData.title.trim()) {
      return { payload: null, error: 'Game title is required' };
    }
    if (!formData.description.trim()) {
      return { payload: null, error: 'Game description is required' };
    }

    // Sunday is day 0 — use explicit empty-string check so falsy 0 is not treated as absent
    const scheduleFieldsFilled = [
      formData.common_room_open_day,
      formData.common_room_open_time,
      formData.common_room_close_day,
      formData.common_room_close_time,
    ].filter(v => v !== '' && v !== null && v !== undefined).length;

    if (scheduleFieldsFilled > 0 && scheduleFieldsFilled < 4) {
      return {
        payload: null,
        error:
          'Please fill in all schedule fields (open day, open time, close day, and close time) or leave them all blank.',
      };
    }

    const hasSchedule = scheduleFieldsFilled === 4;

    // Timezone is captured from the browser at submission time rather than stored in form state.
    // On re-edit the stored timezone is discarded — the next save uses whatever the GM's browser reports.
    // This is intentional: we assume GMs configure schedules from their home timezone.
    const scheduleTimezone = hasSchedule ? Intl.DateTimeFormat().resolvedOptions().timeZone : null;
    if (hasSchedule && !scheduleTimezone) {
      return { payload: null, error: 'Could not detect your timezone. Please try again.' };
    }

    const payload: CreateGameRequest = {
      title: formData.title.trim(),
      description: formData.description.trim(),
      genre: formData.genre.trim() || undefined,
      max_players: formData.max_players === '' ? undefined : Number(formData.max_players),
      start_date: convertToISO8601(formData.start_date) || undefined,
      end_date: convertToISO8601(formData.end_date) || undefined,
      recruitment_deadline: convertToISO8601(formData.recruitment_deadline) || undefined,
      is_anonymous: formData.is_anonymous,
      auto_accept_audience: formData.auto_accept_audience,
      allow_group_conversations: formData.allow_group_conversations ?? true,
      portrait_avatars: formData.portrait_avatars ?? false,
      character_sheet: buildCharacterSheetConfig(formData),
      common_room_open_day: hasSchedule ? Number(formData.common_room_open_day) : null,
      common_room_open_time: hasSchedule ? String(formData.common_room_open_time) : null,
      common_room_close_day: hasSchedule ? Number(formData.common_room_close_day) : null,
      common_room_close_time: hasSchedule ? String(formData.common_room_close_time) : null,
      schedule_timezone: scheduleTimezone,
    };

    return { payload, error: null };
  }, [formData]);

  // Replaces both the form contents and the baseline they are compared against,
  // so re-hydrating an unedited form does not register as dirty.
  const resetFormData = useCallback((next: GameFormData) => {
    setFormData(next);
    setInitialFormData(next);
  }, []);

  return {
    formData,
    setFormData,
    initialFormData,
    resetFormData,
    handleChange,
    error,
    setError,
    loading,
    setLoading,
    pendingBannerFile,
    bannerPreviewUrl,
    handleBannerFileSelect,
    discardPendingBanner,
    uploadPendingBanner,
    uploadBanner,
    deleteBanner,
    buildApiPayload,
  };
}
