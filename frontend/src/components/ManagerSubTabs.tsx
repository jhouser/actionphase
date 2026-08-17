import { Button } from './ui';
import { EditorLockNotice } from './EditorLockNotice';

export interface SubTab<Id extends string> {
  id: Id;
  label: string;
  /** Shown in parentheses after the label — the number of rows under that tab. */
  count: number;
}

interface ManagerSubTabsProps<Id extends string> {
  tabs: SubTab<Id>[];
  activeTab: Id;
  onTabChange: (id: Id) => void;
  /**
   * Hold navigation and explain why. Set while an editor under the active tab holds
   * uncommitted edits: switching unmounts that editor and destroys them, so the move
   * has to be finished or cancelled first.
   */
  locked?: boolean;
}

/**
 * Tab bar for the character-sheet managers (abilities/skills, items/currency).
 *
 * Separate from TabNavigation, which is the heavier page-level bar with overflow
 * collapsing, sticky behaviour and mobile dropdown. These are two plain buttons inside
 * an already-scoped panel; sharing the lock behaviour here keeps the managers from
 * each spelling out the same disabled styling and notice.
 */
export function ManagerSubTabs<Id extends string>({
  tabs,
  activeTab,
  onTabChange,
  locked = false,
}: ManagerSubTabsProps<Id>) {
  return (
    <div className="flex items-center space-x-1 mb-6 border-b border-theme-default">
      {tabs.map((tab) => (
        <Button
          key={tab.id}
          variant="ghost"
          disabled={locked}
          onClick={() => onTabChange(tab.id)}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors rounded-none ${
            activeTab === tab.id
              ? 'border-interactive-primary text-interactive-primary'
              : 'border-transparent text-content-secondary hover:text-content-primary'
          }`}
        >
          {tab.label} ({tab.count})
        </Button>
      ))}
      {locked && <EditorLockNotice className="ml-2" />}
    </div>
  );
}
