import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { GameFormFields } from '../GameFormFields';
import { findFirstInvalidTab } from '../gameFormTabs';
import type { GameFormTabId } from '../gameFormTabs';
import type { GameFormData } from '../GameFormFields';

const baseFormData: GameFormData = {
  title: '',
  description: '',
  genre: '',
  max_players: '',
  recruitment_deadline: '',
  start_date: '',
  end_date: '',
  is_anonymous: false,
  auto_accept_audience: false,
  allow_group_conversations: true,
  portrait_avatars: true,
  common_room_open_day: '',
  common_room_open_time: '',
  common_room_close_day: '',
  common_room_close_time: '',
};

/**
 * The form is tab-split, and inactive panels are hidden with `display: none`.
 * `getByRole` and `toBeVisible` skip them (a11y tree), while `getByTestId` and —
 * surprisingly — `getByLabelText` still find them. So each test opens the tab
 * holding the fields it asserts on, rather than relying on which query happens
 * to reach a hidden field.
 *
 * Settings live on Rules, avatar style and sheet labels on Appearance.
 */
function renderFields({
  formData = baseFormData,
  onChange = vi.fn(),
  activeTab = 'game-form-appearance' as GameFormTabId,
}: {
  formData?: GameFormData;
  onChange?: (field: keyof GameFormData, value: string | number | boolean) => void;
  activeTab?: GameFormTabId;
} = {}) {
  return render(
    <GameFormFields
      formData={formData}
      onChange={onChange}
      activeTab={activeTab}
      onTabChange={vi.fn()}
    />
  );
}

describe('GameFormFields', () => {
  describe('Checkbox labels', () => {
    it('shows clean label text without parenthetical clarifications', () => {
      renderFields({ activeTab: 'game-form-rules' });

      expect(screen.getAllByText('Anonymous Mode').length).toBeGreaterThan(0);
      expect(screen.getAllByText('Auto-Accept Audience Members').length).toBeGreaterThan(0);
      expect(screen.getAllByText('Allow Group Conversations').length).toBeGreaterThan(0);
    });
  });

  describe('Help tooltips on settings', () => {
    it('renders help tooltip for Anonymous Mode with descriptive text', () => {
      renderFields({ activeTab: 'game-form-rules' });

      const tooltips = screen.getAllByRole('tooltip');
      const tooltipTexts = tooltips.map((t) => t.textContent ?? '');

      expect(tooltipTexts.some((t) => /character ownership/i.test(t))).toBe(true);
    });

    it('renders help tooltip for Auto-Accept Audience Members', () => {
      renderFields({ activeTab: 'game-form-rules' });

      const tooltips = screen.getAllByRole('tooltip');
      const tooltipTexts = tooltips.map((t) => t.textContent ?? '');

      expect(tooltipTexts.some((t) => /automatically approved/i.test(t))).toBe(true);
    });

    it('renders help tooltip for Allow Group Conversations', () => {
      renderFields({ activeTab: 'game-form-rules' });

      const tooltips = screen.getAllByRole('tooltip');
      const tooltipTexts = tooltips.map((t) => t.textContent ?? '');

      expect(tooltipTexts.some((t) => /3 or more participants/i.test(t))).toBe(true);
    });

    it('renders help tooltip for Avatar Style explaining both options', () => {
      renderFields({ activeTab: 'game-form-appearance' });

      const tooltips = screen.getAllByRole('tooltip');
      const tooltipTexts = tooltips.map((t) => t.textContent ?? '');

      expect(tooltipTexts.some((t) => /circular/i.test(t) && /portrait/i.test(t))).toBe(true);
    });

    it('renders exactly four setting tooltips across the Rules and Appearance tabs', () => {
      // Three settings tooltips on Rules, one avatar-style tooltip on Appearance.
      // Counted per tab rather than in one pass: role queries respect the
      // accessibility tree, and a `hidden` panel is excluded from it — the
      // tooltips are in the DOM (see the inactive-panel test below) but not
      // reachable by role while their tab is closed.
      const { unmount } = renderFields({ activeTab: 'game-form-rules' });
      expect(screen.getAllByRole('tooltip')).toHaveLength(3);
      unmount();

      renderFields({ activeTab: 'game-form-appearance' });
      expect(screen.getAllByRole('tooltip')).toHaveLength(1);
    });
  });

  describe('Avatar Style radio buttons', () => {
    it('renders Circular and Portrait radio options', () => {
      renderFields();

      expect(screen.getByRole('radio', { name: 'Circular' })).toBeInTheDocument();
      expect(screen.getByRole('radio', { name: 'Portrait' })).toBeInTheDocument();
    });

    it('selects Portrait when portrait_avatars is true', () => {
      renderFields({ formData: { ...baseFormData, portrait_avatars: true } });

      expect(screen.getByRole('radio', { name: 'Portrait' })).toBeChecked();
      expect(screen.getByRole('radio', { name: 'Circular' })).not.toBeChecked();
    });

    it('selects Circular when portrait_avatars is false', () => {
      renderFields({ formData: { ...baseFormData, portrait_avatars: false } });

      expect(screen.getByRole('radio', { name: 'Circular' })).toBeChecked();
      expect(screen.getByRole('radio', { name: 'Portrait' })).not.toBeChecked();
    });

    it('calls onChange with true when Portrait is selected', async () => {
      const onChange = vi.fn();
      const user = userEvent.setup();
      renderFields({ formData: { ...baseFormData, portrait_avatars: false }, onChange });

      await user.click(screen.getByRole('radio', { name: 'Portrait' }));

      expect(onChange).toHaveBeenCalledWith('portrait_avatars', true);
    });

    it('calls onChange with false when Circular is selected', async () => {
      const onChange = vi.fn();
      const user = userEvent.setup();
      renderFields({ formData: { ...baseFormData, portrait_avatars: true }, onChange });

      await user.click(screen.getByRole('radio', { name: 'Circular' }));

      expect(onChange).toHaveBeenCalledWith('portrait_avatars', false);
    });

    it('defaults to Portrait when portrait_avatars is undefined', () => {
      const formDataNoAvatarPref = { ...baseFormData, portrait_avatars: undefined };
      renderFields({ formData: formDataNoAvatarPref });

      expect(screen.getByRole('radio', { name: 'Portrait' })).toBeChecked();
    });
  });
  describe('Tab split', () => {
    it('shows each tab\'s own fields when it is selected', () => {
      const { unmount: u1 } = renderFields({ activeTab: 'game-form-info' });
      expect(screen.getByTestId('game-form-panel-info')).toBeVisible();
      expect(screen.getByLabelText(/game title/i)).toBeVisible();
      u1();

      const { unmount: u2 } = renderFields({ activeTab: 'game-form-schedule' });
      expect(screen.getByTestId('game-form-panel-schedule')).toBeVisible();
      expect(screen.getByLabelText(/start date/i)).toBeVisible();
      u2();

      const { unmount: u3 } = renderFields({ activeTab: 'game-form-rules' });
      expect(screen.getByTestId('game-form-panel-rules')).toBeVisible();
      expect(screen.getByLabelText(/anonymous mode/i)).toBeVisible();
      u3();

      renderFields({ activeTab: 'game-form-appearance' });
      expect(screen.getByTestId('game-form-panel-appearance')).toBeVisible();
      expect(screen.getByTestId('game-sheet-label-skills')).toBeVisible();
    });

    it('hides the panels for tabs that are not selected', () => {
      renderFields({ activeTab: 'game-form-info' });

      expect(screen.getByTestId('game-form-panel-schedule')).not.toBeVisible();
      expect(screen.getByTestId('game-form-panel-rules')).not.toBeVisible();
      expect(screen.getByTestId('game-form-panel-appearance')).not.toBeVisible();
    });

    it('keeps fields on inactive tabs mounted in the document', () => {
      // This is the test that guards the validation decision, so it asserts
      // presence rather than visibility. An unmounted `required` control is not
      // validated at all: submitting from another tab would skip the check and
      // post an empty title with no error shown. If someone later "optimises"
      // this by unmounting inactive tabs, this fails and says why.
      renderFields({ activeTab: 'game-form-appearance' });

      expect(screen.getByTestId('game-title')).toBeInTheDocument();
      expect(screen.getByTestId('game-title')).toBeRequired();
      expect(screen.getByTestId('game-description')).toBeInTheDocument();
      expect(screen.getByTestId('game-description')).toBeRequired();
      expect(screen.getByTestId('max-players')).toBeInTheDocument();
    });

    it('reports the tab holding an invalid required field', () => {
      // findFirstInvalidTab is what keeps a hidden invalid field from making the
      // submit fail silently — Chromium refuses to focus a control inside a
      // display:none panel.
      const form = document.createElement('form');
      form.innerHTML = '<input id="title" required value=""><textarea id="description" required>ok</textarea>';
      document.body.appendChild(form);

      expect(findFirstInvalidTab(form)).toBe('game-form-info');

      (form.querySelector('#title') as HTMLInputElement).value = 'filled';
      expect(findFirstInvalidTab(form)).toBeNull();

      form.remove();
    });

    it('renders tab buttons as type="button" so they cannot submit a form', () => {
      // TabNavigation's tabs live inside the game <form>. A <button> with no
      // type defaults to submit, which would post the form on every tab click.
      renderFields({ activeTab: 'game-form-info' });

      for (const id of ['game-form-info', 'game-form-schedule', 'game-form-rules', 'game-form-appearance']) {
        expect(screen.getByTestId(`tab-${id}`)).toHaveAttribute('type', 'button');
      }
    });
  });
});
