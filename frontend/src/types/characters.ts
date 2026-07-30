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
export interface ControllableCharacterWithGame extends Omit<Character, 'is_active'> {
  game_title: string;
  game_state: string;
  game_is_anonymous: boolean;
  game_portrait_avatars: boolean;
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

// Individual ability/skill item structures for JSON fields
export interface CharacterAbility {
  id: string; // UUID or unique identifier
  name: string;
  description?: string;
  type: 'innate' | 'learned' | 'gm_assigned';
  source?: string; // Who assigned it (GM name, class, etc.)
  active: boolean;
  metadata?: Record<string, unknown>; // For game-specific stats
}

export interface CharacterSkill {
  id: string;
  name: string;
  level?: number | string; // Could be numeric or descriptive like "Expert"
  description?: string;
  category?: string; // e.g., "Combat", "Social", "Academic"
  metadata?: Record<string, unknown>;
}

// Individual inventory item structures for JSON fields
export interface InventoryItem {
  id: string;
  name: string;
  description?: string;
  quantity: number;
  category?: string; // e.g., "Weapon", "Armor", "Consumable", "Tool"
  condition?: string; // e.g., "Excellent", "Good", "Damaged"
  value?: number;
  weight?: number;
  equipped?: boolean; // For equipment/weapons
  metadata?: Record<string, unknown>; // Game-specific properties
}

export interface CurrencyEntry {
  id: string;
  type: string; // e.g., "Gold", "Credits", "XP", "Reputation"
  amount: number;
  description?: string;
}

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

// Predefined character modules for MVP
export const CHARACTER_MODULES: CharacterModule[] = [
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
    type: 'abilities',
    name: 'Abilities & Skills',
    description: 'Character abilities, skills, and special powers',
    fields: [
      {
        name: 'abilities',
        type: 'json',
        label: 'Abilities',
        placeholder: 'Manage your character abilities...',
        isPublic: true
      },
      {
        name: 'skills',
        type: 'json',
        label: 'Skills',
        placeholder: 'Manage your character skills...',
        isPublic: true
      }
    ]
  },
  {
    type: 'inventory',
    name: 'Inventory',
    description: 'Character possessions and equipment',
    fields: [
      {
        name: 'items',
        type: 'json',
        label: 'Items',
        placeholder: 'Manage your character items...',
        isPublic: true
      },
      {
        name: 'currency',
        type: 'json',
        label: 'Currency/Resources',
        placeholder: 'Track your character\'s resources...',
        isPublic: false
      }
    ]
  }
];
