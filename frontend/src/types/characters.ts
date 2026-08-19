// Character-related types for the frontend

export interface Character {
  id: number;
  game_id: number;
  user_id?: number;
  username?: string;
  name: string;
  character_type?: 'player_character' | 'npc';
  status: 'pending' | 'approved';
  avatar_url?: string | null;
  is_active: boolean;
  original_owner_user_id?: number;
  original_owner_username?: string;
  current_owner_username?: string;
  // NPC assignment fields (only present for NPCs)
  assigned_user_id?: number;
  assigned_username?: string;
  created_at: string;
  updated_at: string;
}

/**
 * A controllable character returned by the cross-game endpoint, carrying the
 * game context its sheet needs. Surfaces with no game in scope (the global
 * Utility Drawer) have no GameContext to read role/state from, so the backend
 * sends them alongside each character.
 *
 * `is_active` is omitted rather than optional: the endpoint filters to active
 * characters, so the field is absent from the payload and callers must not
 * branch on it. It's the only required `Character` field the endpoint drops.
 *
 * `username` and `assigned_username` come back for the GM's cast entries (a
 * GM/co-GM receives every character in games they run, not just the ones they
 * personally control), and stay optional because the rest of the payload — a
 * player's own characters — has no one else to credit.
 */
/**
 * Per-game character sheet configuration, as sent by the backend.
 *
 * Every level is optional because that is the wire reality: the backend stores
 * only genuine GM overrides and omits the key entirely when there are none, so
 * most games send nothing at all. Defaults are NOT filled in server-side — the
 * frontend owns them so exactly one place knows them.
 */
export interface CharacterSheetConfig {
  labels?: {
    skills?: string;
    inventory?: string;
    numbers?: string;
  };
}

export interface ControllableCharacterWithGame extends Omit<Character, 'is_active'> {
  game_title: string;
  game_state: string;
  game_is_anonymous: boolean;
  game_portrait_avatars: boolean;
  /**
   * That game's character sheet config, for the same reason the flags above
   * travel here: the drawer renders sheets outside a GameProvider and has no
   * game context to read it from. Absent when the GM has set no overrides,
   * which is the common case — the defaults live in the frontend, so an absent
   * value means "use them", never "the game has no labels".
   */
  game_character_sheet?: CharacterSheetConfig;
  /**
   * The current user's role in that character's game. `audience` is reachable:
   * an audience member assigned an NPC controls it, so it comes back here.
   */
  user_role: 'gm' | 'co_gm' | 'player' | 'audience';
}

export interface CharacterData {
  id: number;
  character_id: number;
  module_type: string;
  field_name: string;
  field_value?: string;
  field_type: 'text' | 'number' | 'boolean' | 'json';
  is_public: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateCharacterRequest {
  name: string;
  character_type: 'player_character' | 'npc';
  user_id?: number; // Optional: for GMs to assign player characters to specific players
}

export interface CharacterDataRequest {
  module_type: string;
  field_name: string;
  field_value: string;
  field_type: 'text' | 'number' | 'boolean' | 'json';
  is_public: boolean;
}

export interface ApproveCharacterRequest {
  status: 'approved';
}

export interface AssignNPCRequest {
  assigned_user_id: number;
}

export interface CharacterActivityStats {
  public_messages: number;
  private_messages?: number;
}

// Individual skill item structure for JSON fields.
//
// CharacterAbility used to sit here. Abilities were retired in the Phase 4
// refactor: they duplicated skills, which is strictly more featured (level,
// category, markdown description), so every stat feature had to be built twice.
// Verified against production before deletion — no character held ability
// content. The rows remain in character_data and are simply never read again.
export interface CharacterSkill {
  id: string;
  name: string;
  /**
   * Free text, e.g. "Expert" or "5".
   *
   * Replaces the old `level?: number | string`. The union was a fiction: the
   * editor stringified on every save, so a numeric level round-tripped into a
   * string the moment anyone touched it, and nothing in the app ever did
   * arithmetic on it. Free text is what the field already was in practice.
   *
   * Read old rows through `skillRank()` rather than this field directly —
   * `level` is still on disk and is NOT migrated.
   */
  rank?: string;
  /**
   * @deprecated Legacy key, read-only. Present on rows written before the
   * rank rename; never written again. Use `skillRank()` instead of reading it.
   */
  level?: number | string;
  description?: string;
  category?: string; // e.g., "Combat", "Social", "Academic"
}

/**
 * Resolves a skill's rank across both storage shapes.
 *
 * There is deliberately no migration for the `level` → `rank` rename: this key
 * lives inside a JSON blob, so a read-side fallback covers every old row,
 * archived payload, and rolled-back deploy at no coordination cost, where a
 * migration would need all three to line up. Old numeric values stringify here
 * rather than on write, so a row is only rewritten when a human edits it.
 *
 * Returns undefined when neither key is set, so callers can keep using the
 * `{rank && ...}` pattern to hide the field entirely.
 */
export function skillRank(skill: Pick<CharacterSkill, 'rank' | 'level'>): string | undefined {
  if (skill.rank !== undefined && skill.rank !== '') return skill.rank;
  if (skill.level === undefined || skill.level === '') return undefined;
  return String(skill.level);
}

// Individual inventory item structures for JSON fields
// `equipped` and `metadata` used to sit here and were dropped in the Phase 5
// field pass. `equipped` rendered a badge but nothing could ever set it true —
// AddItemModal hardcoded false and no edit path touched it — so the badge was
// unreachable. `metadata` had no reader anywhere. Both keys are still tolerated
// on read (old rows carry `equipped`); they are simply never written again.
export interface InventoryItem {
  id: string;
  name: string;
  description?: string;
  quantity: number;
  category?: string; // e.g., "Weapon", "Armor", "Consumable", "Tool"
  condition?: string; // e.g., "Excellent", "Good", "Damaged"
  /**
   * Unused by any game today, kept deliberately: both feed the optional
   * weight/value summary in ItemsManager, which stays hidden until a game sets
   * them. Available as defaults rather than dead weight.
   */
  value?: number;
  weight?: number;
}

/**
 * One entry on the Numbers tab: a named quantity, optionally bounded.
 *
 * Renamed from `CurrencyEntry` in the Phase 5 field pass, along with the tab
 * itself. The tab holds arbitrary numeric tracks — stress, XP, clocks, heat —
 * and "currency" described only the narrowest case.
 */
export interface NumberEntry {
  id: string;
  /**
   * The entry's label, e.g. "Gold", "Stress", "XP".
   *
   * Was `type`, which read like a discriminant. Old rows still use that key —
   * read through `numberEntryName()`, never this field directly. As with the
   * skills rename there is deliberately no migration: the key lives inside a
   * JSON blob, so a read-side fallback covers every old row, archived payload,
   * and rolled-back deploy at no coordination cost.
   */
  name?: string;
  /**
   * @deprecated Legacy key, read-only. Use `numberEntryName()`.
   */
  type?: string;
  amount: number;
  /**
   * Upper bound, which turns a bare count into a track: "Stress 4/9".
   *
   * Absent means an unbounded quantity (money, XP), which is why this is
   * optional rather than defaulted — there is no sensible maximum for a purse.
   */
  max?: number;
  /**
   * How the entry renders. Only meaningful with `max` set; a bare quantity has
   * nothing to draw a bar or boxes against, so it always renders as a number.
   * Absent means 'number'.
   */
  display?: NumberEntryDisplay;
  description?: string;
}

export type NumberEntryDisplay = 'number' | 'track' | 'boxes';

/**
 * Resolves an entry's label across both storage shapes.
 *
 * Returns '' rather than undefined when neither key is set: the name is
 * required by the form, so an entry without one is corrupt data rather than a
 * meaningful absence, and callers render it as an empty heading rather than
 * branching.
 */
export function numberEntryName(entry: Pick<NumberEntry, 'name' | 'type'>): string {
  return entry.name || entry.type || '';
}

/**
 * Whether an entry should render as a bounded track rather than a bare number.
 *
 * `max` is what makes a track possible, so `display` alone is not enough — a
 * 'boxes' entry with no maximum has no box count to draw. Guards against a
 * non-positive max for the same reason: zero boxes is not a track.
 */
export function isBoundedTrack(entry: NumberEntry): boolean {
  // Requires an explicit track display rather than merely excluding 'number':
  // absent means 'number' (see the field's doc), and the write path stores
  // exactly that — NumberForm persists undefined for the Number option instead
  // of the literal, so `display !== 'number'` admitted every saved Number entry
  // that had a maximum and drew it as a bar.
  return (
    entry.max !== undefined &&
    entry.max > 0 &&
    (entry.display === 'track' || entry.display === 'boxes')
  );
}

/**
 * Resolved labels for the three renameable character sheet tabs.
 *
 * Defined here rather than beside the hook that produces it so the type layer
 * has no dependency on the hook layer; `useSheetLabels` imports this.
 */
export type SheetLabels = Record<'skills' | 'inventory' | 'numbers', string>;

// Character module types for the modular character sheet system
export interface CharacterModule {
  type: string;
  name: string;
  description: string;
  fields: CharacterModuleField[];
}

interface CharacterModuleField {
  name: string;
  type: 'text' | 'number' | 'boolean' | 'json';
  label: string;
  placeholder?: string;
  required?: boolean;
  isPublic?: boolean;
}

/**
 * The character sheet's tabs, with the game's labels applied.
 *
 * A function rather than a constant because two of the five tabs are
 * GM-renameable, so the list is a function of the game. `labels` comes from
 * `useSheetLabels`, which is the only place that knows the default names —
 * do not default them here.
 *
 * Bio and Private Notes are deliberately NOT renameable: they are platform
 * concepts (a public description, private notes visible to GM and audience)
 * rather than game-system ones, so their names stay fixed.
 *
 * Per the refactor's invariant each renameable tab's `type` equals its storage
 * `module_type`, its field name, and its own default label. That is what keeps
 * this a straight substitution with no mapping table.
 */
export function buildCharacterModules(labels: SheetLabels): CharacterModule[] {
  return [
    {
      type: 'bio',
      name: 'Public Profile',
      description: 'Public character details',
      fields: [
        {
          name: 'background',
          type: 'text',
          label: 'Character Description',
          placeholder: 'Describe your character\'s appearance, personality, background, and any publicly visible information...',
          isPublic: true
        }
      ]
    },
    {
      type: 'notes',
      name: 'Private Notes',
      description: 'Private notes only visible to you, the audience, and the GM',
      fields: [
        {
          name: 'private_notes',
          type: 'text',
          label: 'Private Notes & Secrets',
          placeholder: 'Your private character notes, secrets, motivations, and hidden information...',
          isPublic: false
        }
      ]
    },
    {
      type: 'skills',
      name: labels.skills,
      description: `Character ${labels.skills.toLowerCase()}`,
      fields: [
        {
          name: 'skills',
          type: 'json',
          label: labels.skills,
          placeholder: `Manage your character ${labels.skills.toLowerCase()}...`,
          isPublic: true
        }
      ]
    },
    {
      type: 'inventory',
      name: labels.inventory,
      description: 'Character possessions and equipment',
      fields: [
        {
          name: 'items',
          type: 'json',
          label: 'Items',
          placeholder: 'Manage your character items...',
          isPublic: true
        }
      ]
    },
    {
      type: 'numbers',
      name: labels.numbers,
      description: 'Character resources and numeric tracks',
      fields: [
        {
          // Storage key, not a label: renamed from `currency` in the Phase 4
          // migration because this tab now holds arbitrary numeric tracks
          // (stress, XP, clocks), not money. Unlike a label, an identifier
          // cannot be overridden per game, so it had to stop saying "currency".
          name: 'numbers',
          type: 'json',
          label: labels.numbers,
          placeholder: `Track your character's ${labels.numbers.toLowerCase()}...`,
          isPublic: false
        }
      ]
    }
  ];
}
