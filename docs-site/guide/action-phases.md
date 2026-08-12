# Action Phases

Action phases are the structured turns of the game. During an action phase, players submit what their characters do, and the GM writes and publishes results.

## Submitting an Action

When an action phase is active, the **Action Submission** panel appears on the game page.

1. If you have more than one character, select which character is acting from the dropdown. If you have exactly one character, it's shown automatically.
2. Write your action in the text editor. Describe what your character does during this phase.
3. Click **Submit Action**.

You can edit your action at any time before the deadline by clicking **Edit** on your submitted action. After the deadline passes, submissions are locked.

Your action is private — only the GM can see it during the game.

### Inserting from Your Character Sheet

While writing your action you can insert a reference to any item from your character sheet — an ability, skill, or inventory item. The reference renders as a highlighted link in the preview, and hovering or tapping it shows the item's description in a tooltip.

There are two ways to insert:

**From the Character Sheet drawer** — Click the **Character Sheet** button (person icon) in the toolbar to open a sidebar listing your abilities, skills, and inventory. Use the filter box to narrow the list, then click any item to insert it at the current cursor position.

**With the `%%` shortcut** — Type `%%` anywhere in the editor to open an inline autocomplete menu. Continue typing to filter, then press `Enter` or click a result to insert.

Both methods insert a `[[Item Name|type:id]]` tag. This is stored in your action text and remains readable as plain text if the sheet item is later removed.

### Previous Actions

A collapsible **Your Previous Actions** section below the submission panel shows your submissions from earlier phases for reference.

## Receiving Results

After the GM publishes your result, it appears alongside your submitted action. Results are private — each player sees only their own.

If your character sheet was updated as part of the result, those changes appear on your character sheet immediately.

When the game moves into a Common Room phase after an action phase, your most recent results appear at the top of the Common Room for easy reference. This panel collapses automatically after you've visited the page once.

---

## GM: Viewing Submissions

The **Submitted Actions** list is on the **Actions** tab. It shows every player's submission for the current or selected phase. Use the phase filter to switch between past action phases.

Each submission card shows the character name, player username, submission timestamp, and the action text (expandable for long submissions).

## GM: Writing Results

To write a result for a submission, click **Send Result to [player]** on the submission card. Write the result in the text editor and click **Create Draft Result**. Results are always created as drafts first — they are not visible to the player until you publish them.

## GM: Publishing Results

The **Results** section (below the submissions list) shows all draft and published results. Click **Publish Result** on a draft result to send it to the player. A confirmation dialog appears, noting how many character sheet updates will also be applied when you publish.

Once published, the result is immediately visible to the player and cannot be undone.

You can also publish all unpublished results at once using the **Publish All Results** button that appears when draft results are ready.

If you attempt to activate a new phase while draft results exist, a prompt will appear asking whether you want to publish them before proceeding or activate without publishing.

## GM: Character Sheet Updates

Before publishing a result, you can prepare character sheet changes as part of that result. Click the character sheet edit button on the result card to open the **Update Character Sheet** modal.

The modal lets you edit the character's Abilities and Inventory sections directly. Changes are saved as drafts and applied to the character sheet automatically when you publish the result.

### Keep each character's sheet updates in ONE result per phase

You can send a player several results in a single phase — for example, a first result now and a follow-up after a cliffhanger. When you do, **stage all of that character's sheet updates on exactly one of those results.**

Each result stores a *complete snapshot* of the character sheet, not a list of individual changes. The editor starts from the character's currently published sheet, which does not include updates still sitting unpublished on another result. So if you add "Item A" to a draft update on the first result, then open the second result's editor and add "Item B", the second snapshot contains only Item B. Publishing both applies them in order and the second overwrites the first — Item A disappears with no error.

The interface warns you when this situation arises: a warning appears on any draft result whose character already has staged sheet updates on another unpublished result, and again in the publish confirmation dialog — including the bulk "Publish All" and phase-activation dialogs. If you see it, consolidate the sheet changes into a single result before publishing.

If you have already published both results, re-open the character sheet editor on a new result and re-add the missing entries by hand.
