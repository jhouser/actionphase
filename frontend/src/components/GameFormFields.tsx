import type { ReactNode } from 'react';
import { Input, Textarea, DateTimeInput, Checkbox, Radio, Select } from './ui';
import { HelpTooltip } from './ui/HelpTooltip';
import { DEFAULT_SHEET_LABELS } from '../hooks/useSheetLabels';

/**
 * Mirrors MaxCharacterSheetLabelLength in the backend's character sheet config.
 * Counted in runes there and UTF-16 units here; they agree for everything short
 * of astral-plane characters, where the browser is the stricter of the two — so
 * this can only ever stop input the server would also reject.
 */
const MAX_SHEET_LABEL_LENGTH = 24;

export interface GameFormData {
  title: string;
  description: string;
  genre: string;
  max_players: number | '';
  recruitment_deadline: string;
  start_date: string;
  end_date: string;
  is_anonymous?: boolean;
  auto_accept_audience?: boolean;
  allow_group_conversations?: boolean;
  portrait_avatars?: boolean;
  /**
   * Character sheet tab labels, held flat rather than nested because the form's
   * onChange carries a single scalar per field. They are folded back into the
   * nested `character_sheet` wire shape in `useGameForm`'s buildApiPayload.
   *
   * Empty string means "use the default" — the same thing an absent value means
   * on the wire. buildApiPayload is what turns one into the other.
   */
  sheet_label_skills?: string;
  sheet_label_inventory?: string;
  sheet_label_numbers?: string;
  common_room_open_day: number | '';
  common_room_open_time: string;
  common_room_close_day: number | '';
  common_room_close_time: string;
}

interface GameFormFieldsProps {
  formData: GameFormData;
  onChange: (field: keyof GameFormData, value: string | number | boolean) => void;
  bannerUpload?: ReactNode;
}

const DAY_OPTIONS = (
  <>
    <option value="">-- Day --</option>
    <option value="0">Sunday</option>
    <option value="1">Monday</option>
    <option value="2">Tuesday</option>
    <option value="3">Wednesday</option>
    <option value="4">Thursday</option>
    <option value="5">Friday</option>
    <option value="6">Saturday</option>
  </>
);

function SectionHeading({ children }: { children: ReactNode }) {
  return (
    <div className="flex items-center gap-3 pt-1">
      <span className="text-xs font-semibold uppercase tracking-wider text-text-secondary whitespace-nowrap">
        {children}
      </span>
      <div className="flex-1 h-px bg-border-primary" />
    </div>
  );
}

export const GameFormFields = ({ formData, onChange, bannerUpload }: GameFormFieldsProps) => {
  return (
    <>
      <SectionHeading>Basic Info</SectionHeading>

      <Input
        label="Game Title"
        id="title"
        type="text"
        required
        value={formData.title}
        onChange={(e) => onChange('title', e.target.value)}
        placeholder="Enter a compelling game title"
        maxLength={255}
        data-testid="game-title"
      />

      <Textarea
        label="Description"
        id="description"
        value={formData.description}
        onChange={(e) => onChange('description', e.target.value)}
        rows={4}
        required
        placeholder="Describe your game world, setting, and what players can expect..."
        data-testid="game-description"
      />

      <div className="grid grid-cols-3 gap-4">
        <div className="col-span-2">
          <Input
            label="Genre"
            id="genre"
            type="text"
            optional
            value={formData.genre}
            onChange={(e) => onChange('genre', e.target.value)}
            placeholder="e.g., Fantasy, Sci-Fi, Horror, Modern"
            maxLength={100}
          />
        </div>
        <div>
          <Input
            label="Maximum Players"
            id="max_players"
            type="number"
            optional
            value={formData.max_players}
            onChange={(e) => onChange('max_players', parseInt(e.target.value) || '')}
            helperText="Default: 6"
            min={1}
            max={20}
            placeholder="6"
            data-testid="max-players"
          />
        </div>
      </div>

      <SectionHeading>Schedule</SectionHeading>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <DateTimeInput
          label="Recruitment Deadline"
          id="recruitment_deadline"
          optional
          value={formData.recruitment_deadline}
          onChange={(e) => onChange('recruitment_deadline', e.target.value)}
        />

        <DateTimeInput
          label="Start Date"
          id="start_date"
          optional
          value={formData.start_date}
          onChange={(e) => onChange('start_date', e.target.value)}
        />

        <DateTimeInput
          label="End Date"
          id="end_date"
          optional
          value={formData.end_date}
          onChange={(e) => onChange('end_date', e.target.value)}
        />
      </div>

      <SectionHeading>Common Room Schedule</SectionHeading>

      <p className="text-sm text-content-secondary -mt-1">
        Set the recurring weekly window when Common Room is open. Times are stored in your current timezone (<span className="font-medium">{Intl.DateTimeFormat().resolvedOptions().timeZone}</span>) and shown to players in their own local timezone. Saving will update the stored timezone to match your current browser timezone.
      </p>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="flex gap-2 items-end">
          <div className="flex-1">
            <Select
              label="Opens"
              id="common_room_open_day"
              optional
              value={formData.common_room_open_day === '' ? '' : String(formData.common_room_open_day)}
              onChange={(e) => onChange('common_room_open_day', e.target.value === '' ? '' : parseInt(e.target.value))}
            >
              {DAY_OPTIONS}
            </Select>
          </div>
          <div className="flex-1">
            <Input
              label="at"
              id="common_room_open_time"
              type="time"
              optional
              value={formData.common_room_open_time}
              onChange={(e) => onChange('common_room_open_time', e.target.value)}
            />
          </div>
        </div>

        <div className="flex gap-2 items-end">
          <div className="flex-1">
            <Select
              label="Closes"
              id="common_room_close_day"
              optional
              value={formData.common_room_close_day === '' ? '' : String(formData.common_room_close_day)}
              onChange={(e) => onChange('common_room_close_day', e.target.value === '' ? '' : parseInt(e.target.value))}
            >
              {DAY_OPTIONS}
            </Select>
          </div>
          <div className="flex-1">
            <Input
              label="at"
              id="common_room_close_time"
              type="time"
              optional
              value={formData.common_room_close_time}
              onChange={(e) => onChange('common_room_close_time', e.target.value)}
            />
          </div>
        </div>
      </div>

      <SectionHeading>Settings</SectionHeading>

      <Checkbox
        id="is_anonymous"
        label="Anonymous Mode"
        helpText="Hides character ownership and NPC status from players. Players won't see which user controls which character, and NPCs appear indistinguishable from player characters."
        checked={formData.is_anonymous ?? false}
        onChange={(e) => onChange('is_anonymous', e.target.checked)}
      />

      <Checkbox
        id="auto_accept_audience"
        label="Auto-Accept Audience Members"
        helpText="Audience applications are automatically approved without GM review. Audience members can read the game but cannot post or submit actions."
        checked={formData.auto_accept_audience ?? false}
        onChange={(e) => onChange('auto_accept_audience', e.target.checked)}
      />

      <Checkbox
        id="allow_group_conversations"
        label="Allow Group Conversations"
        helpText="Players can create private message threads with 3 or more participants. When disabled, private messages are limited to two people only."
        checked={formData.allow_group_conversations ?? true}
        onChange={(e) => onChange('allow_group_conversations', e.target.checked)}
      />

      <SectionHeading>Appearance</SectionHeading>

      <div>
        <div className="flex items-center gap-1 mb-2">
          <span className="text-sm font-medium text-content-primary">Avatar Style</span>
          <HelpTooltip text="Circular avatars appear as small round thumbnails beside each post. Portrait avatars have a 2:3 aspect ratio and float to the left with text wrapping around them, like the old Reddit flair images." />
        </div>
        <div className="flex gap-6">
          <Radio
            name="portrait_avatars"
            value="circular"
            label="Circular"
            checked={!(formData.portrait_avatars ?? true)}
            onChange={() => onChange('portrait_avatars', false)}
          />
          <Radio
            name="portrait_avatars"
            value="portrait"
            label="Portrait"
            checked={formData.portrait_avatars ?? true}
            onChange={() => onChange('portrait_avatars', true)}
          />
        </div>
      </div>

      <SectionHeading>Character Sheet</SectionHeading>

      <p className="text-sm text-content-secondary -mt-1">
        Rename the character sheet tabs to match your game's system. Leave a box
        blank to keep its default name.
      </p>

      <div className="grid gap-4 sm:grid-cols-3">
        <Input
          label="Skills tab"
          id="sheet_label_skills"
          type="text"
          value={formData.sheet_label_skills ?? ''}
          onChange={(e) => onChange('sheet_label_skills', e.target.value)}
          /* The default as placeholder, not as value: a filled-in box would
             make every game look like it had overridden its labels, and would
             send the defaults to the server as if the GM had chosen them. */
          placeholder={DEFAULT_SHEET_LABELS.skills}
          maxLength={MAX_SHEET_LABEL_LENGTH}
          data-testid="game-sheet-label-skills"
        />

        <Input
          label="Inventory tab"
          id="sheet_label_inventory"
          type="text"
          value={formData.sheet_label_inventory ?? ''}
          onChange={(e) => onChange('sheet_label_inventory', e.target.value)}
          placeholder={DEFAULT_SHEET_LABELS.inventory}
          maxLength={MAX_SHEET_LABEL_LENGTH}
          data-testid="game-sheet-label-inventory"
        />

        <Input
          label="Numbers tab"
          id="sheet_label_numbers"
          type="text"
          value={formData.sheet_label_numbers ?? ''}
          onChange={(e) => onChange('sheet_label_numbers', e.target.value)}
          placeholder={DEFAULT_SHEET_LABELS.numbers}
          maxLength={MAX_SHEET_LABEL_LENGTH}
          data-testid="game-sheet-label-numbers"
        />
      </div>

      {bannerUpload}
    </>
  );
};
