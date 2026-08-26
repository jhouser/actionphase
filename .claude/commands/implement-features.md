# Feature Implementation Command

**MANDATORY: Use TodoWrite tool throughout this process**

## Phase 1: Task Analysis (5 minutes max)
1. Count total features requested
2. If more than 3 features, STOP and ask user to prioritize
3. Create TodoWrite list with ALL features (don't filter by priority)

## Phase 2: Implementation Order
```
For each feature, assess:
- Dependencies (what must exist first?)
- User impact (who needs this most?)
- Implementation effort (simple/medium/complex)

Order: Dependencies first → High impact → Low effort
```

## Phase 3: Atomic Task Breakdown
**CRITICAL: Each todo item must be completable in <10 minutes**

Bad todo: "Implement user authentication system"
Good todos:
1. Create auth database migration
2. Write auth service interface
3. Implement login endpoint
4. Add login form component
5. Write auth hook tests
6. Implement auth hook

## Phase 4: Progress Tracking
- Mark todos as in_progress when starting
- Mark as completed IMMEDIATELY after finishing
- If blocked, add a new todo for the blocker
- Never have more than ONE in_progress item

## Phase 5: Incremental Delivery
After completing each feature:
1. Run relevant tests
2. Verify in browser if UI feature
3. Summarize what changed and suggest a commit message — **do not run git
   commands.** The user handles all git operations (see CLAUDE.md).
4. Update TodoWrite before moving to next

## Example:
User: "Add user profiles, notifications, and messaging"

Response:
"I'll implement these 3 features. Let me break them down:

Using TodoWrite:
1. [pending] Create user_profiles migration
2. [pending] Implement profile service
3. [pending] Create profile API endpoint
...
12. [pending] Create notification component

This will take approximately X tasks. Shall I proceed with user profiles first since the other features depend on it?"
