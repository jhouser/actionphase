# Characters

## Creating a Character

Character creation is available once the game enters the **Character Creation** state. From the game's **People** tab, click **Create Character** and fill in the character name. After submitting, your character shows as **Pending** status until the GM reviews it.

## Character Statuses

- **Pending** — Submitted, awaiting GM review
- **Approved** — Accepted and participating in the game

## The Character Sheet

Open your character sheet by clicking your character's name in the **People** tab. The sheet has five tabs:

- **Public Profile** — Character description visible to all players
- **Private Notes** — Your private notes, visible only to you, the GM, and audience members
- **Skills** — Trained skills and special powers (GM-controlled)
- **Inventory** — Equipment and items (GM-controlled)
- **Numbers** — Named quantities and tracks (GM-controlled)

The **Public Profile** tab is always visible to everyone. The other four tabs are only visible to the character's player, the GM, and audience members. In a completed game, all participants can view the full sheet.

**Your game may use different names.** The GM can rename the **Skills**,
**Inventory**, and **Numbers** tabs to suit the game — "Talents", "Load",
"Stress". The names below are the defaults; the tabs work the same whatever they
are called. See [Game Settings](./game-settings.md) if you are the GM.

### Public Profile

The **Character Description** field holds your character's public-facing information — appearance, personality, backstory, and anything else other players can know. Supports Markdown. Edit it by clicking **Edit** on the field.

### Private Notes

A single **Private Notes & Secrets** field for anything you don't want other players to see — motivations, secrets, things your character knows. Visible only to you, the GM, and audience members. Supports Markdown.

### Skills

Each skill has a name, an optional **rank** (free text — "Expert", "5", "Advanced"), an optional category (e.g. "Combat", "Social"), and a description. Descriptions support Markdown and are collapsed by default; click **Description** to expand one.

### Inventory

Each item has a name, quantity, and optional category, value, weight, and description. The total weight and value line appears only if a game actually uses those fields.

### Numbers

Tracks named quantities — money, experience, stress, heat, clocks. Each entry has a name, a current amount, and an optional description.

An entry can also carry a **maximum**, which turns it from a bare count into a track: "Stress 4 / 9". Entries with a maximum can display as a number, a bar, or a row of boxes.

## Avatar

Click the camera icon on the character avatar to upload a new image. Click the trash icon to remove it.

## Renaming a Character

Players can rename their own character by clicking the pencil icon next to the name. GMs can rename any character.

---

## GM: Approving Characters

Player-submitted characters appear in the **People** tab with a **Pending** label. Click a character to open it, then:

- **Publish** — Approves the character and makes it active in the game
- **Delete** — Removes the character; the player can create and submit a new one

## GM: Creating Characters

GMs can create both player characters and NPC characters from the **Create Character** button in the People tab. When creating a character as GM, you can assign it to a specific player account (for player characters) or leave it unassigned (for NPCs).

## GM: Editing Character Sheets

As GM, you can edit any character's Skills, Inventory, and Numbers directly from their character sheet at any time. Players can edit their own Public Profile and Private Notes but cannot edit the stat tabs — those are GM-controlled.

## GM: Draft Character Updates

To prepare character sheet changes that will take effect when you publish action results:

1. When writing an action result, open the **Draft Character Updates** section.
2. Add updates specifying the module (Skills, Inventory, Numbers), field name, value, and operation (Upsert or Delete).
3. Save the drafts.

Draft updates are not visible to the player. When you publish the result, all associated drafts are applied to the character sheet automatically.

See [Action Phases](./action-phases) for the full publish flow.

## GM: Deleting Characters

GMs can delete characters from the character card. Characters that have existing messages or action submissions cannot be deleted.

## GM: Assigning NPCs

To assign an NPC to an audience member so they can control it during action phases, click **Assign NPC** on the NPC's character card and select the audience member.
