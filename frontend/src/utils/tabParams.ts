/**
 * Sub-params that belong to a single tab.
 *
 * Each tab's sub-params are cleared when navigating to a *different* tab, so
 * returning to a tab later opens it in its default state rather than resuming
 * a stale drill-down (e.g. landing back on History and being dropped into the
 * phase and sub-tab you last looked at).
 *
 * Params shared by more than one tab are listed under every tab that owns them
 * and survive navigation between those tabs — `comment` is deep-linked into
 * both the Common Room and History.
 */
export const TAB_OWNED_PARAMS: Record<string, readonly string[]> = {
  'common-room': ['view', 'poll', 'comment'],
  messages: ['conversation', 'newConversationWith'],
  audience: ['audienceConversation', 'audienceParticipants'],
  people: ['character', 'peopleTab'],
  history: ['phase', 'subTab', 'characters', 'comment'],
  loot_tables: ['table', 'new'],
};

/**
 * Removes every tab-owned param that does not belong to `targetTabId`.
 *
 * Mutates `params` in place.
 */
export function clearForeignTabParams(params: URLSearchParams, targetTabId: string): void {
  const keep = new Set(TAB_OWNED_PARAMS[targetTabId] ?? []);
  for (const owned of Object.values(TAB_OWNED_PARAMS)) {
    for (const key of owned) {
      if (!keep.has(key)) params.delete(key);
    }
  }
}
