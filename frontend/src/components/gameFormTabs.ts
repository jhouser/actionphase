import type { Tab } from './TabNavigation';

/**
 * Prefixed because TabNavigation derives its testid as `tab-${id}`, and the game
 * *page* behind these modals has its own tab bar with an `info` tab — an
 * unprefixed id makes `getByTestId('tab-info')` match two elements and every
 * selector for either one ambiguous.
 */
export type GameFormTabId =
  | 'game-form-info'
  | 'game-form-schedule'
  | 'game-form-rules'
  | 'game-form-appearance';

export const GAME_FORM_TABS: Tab[] = [
  { id: 'game-form-info', label: 'Info' },
  { id: 'game-form-schedule', label: 'Schedule' },
  { id: 'game-form-rules', label: 'Rules' },
  { id: 'game-form-appearance', label: 'Appearance' },
];

/**
 * Which tab each `required` control lives on, so a failed submit can reveal the
 * offending field before the browser tries to report it.
 *
 * Chromium will not focus a control inside a `display: none` panel: it logs
 * "An invalid form control ... is not focusable" and the submit does nothing at
 * all, with no message shown. Measured, not assumed — a hidden control behaves
 * exactly like a detached one here. `findFirstInvalidTab` is what keeps that
 * from being a silent dead end.
 */
const REQUIRED_FIELD_TABS: ReadonlyArray<{ id: string; tab: GameFormTabId }> = [
  { id: 'title', tab: 'game-form-info' },
  { id: 'description', tab: 'game-form-info' },
];

/**
 * Finds the tab holding the first invalid required control, or null when the
 * form is valid. Call before submitting so the panel is visible by the time the
 * browser reports validity.
 */
export function findFirstInvalidTab(form: HTMLFormElement): GameFormTabId | null {
  for (const { id, tab } of REQUIRED_FIELD_TABS) {
    const el = form.querySelector<HTMLInputElement | HTMLTextAreaElement>(`#${id}`);
    if (el && !el.validity.valid) return tab;
  }
  return null;
}
