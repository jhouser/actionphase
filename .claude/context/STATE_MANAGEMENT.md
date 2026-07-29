# State Management Context - Read Before Frontend State Work

**IMPORTANT: Read this file before working on frontend state management.**

**Last Verified**: May 2026

**Comprehensive Documentation:** `/docs/features/STATE_MANAGEMENT.md` (single source of truth)

This file provides quick context for AI. For complete details, see the comprehensive doc.

---

## State Management Strategy

ActionPhase uses a **Hybrid State Management Strategy**:

1. **Server State**: React Query (TanStack Query) - API communication and caching
2. **Authentication State**: AuthContext + React Query - Centralized auth with single source of truth
3. **Game Context**: GameContext - Game-specific state and permissions
4. **UI State**: React useState/useReducer - Component-local state
5. **Global UI State**: React Context (sparingly) - Only for truly global concerns

---

## Critical Patterns

### 1. AuthContext - Get Current User

```typescript
import { useAuth } from '../contexts/AuthContext';

const { currentUser, isCheckingAuth, isAuthenticated } = useAuth();

// Always check auth state first
if (isCheckingAuth) {
  return <LoadingSpinner />;
}

// Safe to use currentUser
const userId = currentUser?.id;
const username = currentUser?.username;
```

**CRITICAL: Always use `isCheckingAuth` flag**

```typescript
// ❌ BAD: Race condition
{!isGM && <button>Apply</button>}

// ✅ GOOD: Wait for auth
{!isGM && !isCheckingAuth && <button>Apply</button>}
```

### 2. GameContext - Game Permissions

**Option 1: Full Context (for game pages)**
```typescript
import { GameProvider, useGameContext } from '../contexts/GameContext';

<GameProvider gameId={gameId}>
  <GameContent />
</GameProvider>

// In child component:
const { game, isGM, isParticipant, userCharacters, isUserCharacter } = useGameContext();
```

**Option 2: Hooks (for smaller components)**
```typescript
import { useGamePermissions, useUserCharacters } from '../hooks';

const { isGM, canEditGame } = useGamePermissions(gameId);
const { characters } = useUserCharacters(gameId);
```

### 3. UtilityDrawerContext - Global Drawer + Contributed Game Context

The Utility Drawer is mounted **once at the app root** (`RootLayout` in `App.tsx`),
not inside CommonRoom, so it and the character-sheet modal it launches are
reachable from every page. Game-scoped data is *contributed upward* by whichever
game surface is mounted.

```typescript
// A game surface publishes its slice for as long as it's mounted:
import { useProvideGameUtilityContext, useUtilityDrawer } from '../contexts/UtilityDrawerContext';

const gameUtilityContext = useMemo<GameUtilityContext>(() => ({
  gameId, currentPhase, isGM, userRole, gameState, userCharacters, /* … */
}), [/* all of the above */]);
useProvideGameUtilityContext(gameUtilityContext);  // withdrawn on unmount

const { openDrawer } = useUtilityDrawer();          // to trigger it
```

`ctx.game` is **null outside a game route**. Utilities must gate on it in
`isAvailable` (see `components/utility-drawer/registry.ts`) rather than assume
a game exists — e.g. Mark All Read requires `!!ctx.game`, while the character
sheet falls back to a cross-game picker via `useGlobalCharacters()`.

**Gotchas (both caused real bugs during implementation):**
- The provider **must** compare contributed contexts structurally before storing
  them (`isSameGameContext`). The publishing component re-renders whenever
  provider state changes, so storing each new object identity loops infinitely.
- Effects that call `openCharacterSheet` need a ref guard. Opening a sheet sets
  provider state → re-renders the panel → the effect re-fires without one.
- Nav-level consumers use `useOptionalUtilityDrawer()`, which returns null
  instead of throwing, so Layout still renders where no provider is mounted.

### 4. Getting User ID

```typescript
// ✅ CORRECT: Use AuthContext
const { currentUser } = useAuth();
const userId = currentUser?.id;

// ✅ CORRECT: Nullish coalescing
const currentUserId = currentUser?.id ?? null;

// ❌ WRONG: Never decode JWT client-side
const decoded = decodeJWT(token);  // SECURITY RISK
```

---

## Anti-Patterns (NEVER DO)

### ❌ Don't Decode JWT Client-Side
```typescript
// ❌ WRONG
const token = localStorage.getItem('access_token');
const decoded = JSON.parse(atob(token.split('.')[1]));
const userId = decoded.user_id;

// ✅ CORRECT
const { currentUser } = useAuth();
const userId = currentUser?.id;
```

### ❌ Don't Fetch User Data Manually
```typescript
// ❌ WRONG
const [user, setUser] = useState(null);
useEffect(() => {
  apiClient.getCurrentUser().then(setUser);
}, []);

// ✅ CORRECT
const { currentUser } = useAuth();
```

### ❌ Don't Forget isCheckingAuth
```typescript
// ❌ WRONG: Premature render
{!isGM && <button>Apply</button>}

// ✅ CORRECT: Wait for auth
{!isGM && !isCheckingAuth && <button>Apply</button>}
```

### ❌ Don't Store Server Data in useState
```typescript
// ❌ WRONG
const [games, setGames] = useState([]);
useEffect(() => {
  apiClient.getGames().then(setGames);
}, []);

// ✅ CORRECT
const { data: games } = useQuery({
  queryKey: ['games'],
  queryFn: () => apiClient.getGames(),
});
```

---

## Quick Reference

### Import Statements
```typescript
// Auth
import { useAuth } from '../contexts/AuthContext';

// Game Context
import { GameProvider, useGameContext, useOptionalGameContext } from '../contexts/GameContext';

// Other Contexts
import { useConversationContext } from '../contexts/ConversationContext';
import { useAdminMode } from '../contexts/AdminModeContext';

// Hooks
import { useGamePermissions } from '../hooks/useGamePermissions';
import { useUserCharacters } from '../hooks/useUserCharacters';
import { useCharacterOwnership } from '../hooks/useCharacterOwnership';
```

### When to Use What

| Use Case | Solution |
|----------|----------|
| Get current user anywhere | `useAuth()` |
| Game detail page | Wrap with `GameProvider` |
| Check game permissions | `useGamePermissions(gameId)` |
| List user's characters | `useUserCharacters(gameId)` |
| Check character ownership | `useCharacterOwnership(gameId)` |
| Small component needing game info | Use specific hook |
| Complex page with multiple game queries | Wrap with `GameProvider` |

---

## React Query Patterns

### Query Keys
```typescript
// Auth
['auth'] - Authentication state
['currentUser'] - Current user data

// Game
['gameDetails', gameId] - Game details
['gameParticipants', gameId] - Participants
['userControllableCharacters', gameId] - User's characters
```

### Invalidate After Mutations
```typescript
import { useQueryClient } from '@tanstack/react-query';

const queryClient = useQueryClient();
await queryClient.invalidateQueries({ queryKey: ['gameDetails', gameId] });
```

---

## Testing

### With AuthContext
```typescript
const mockAuthValue = {
  currentUser: { id: 1, username: 'testuser' },
  isAuthenticated: true,
  isLoading: false,
  isCheckingAuth: false,
  login: jest.fn(),
  logout: jest.fn(),
  error: null,
};

render(
  <AuthContext.Provider value={mockAuthValue}>
    <MyComponent />
  </AuthContext.Provider>
);
```

### With React Query
```typescript
const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return ({ children }) => (
    <QueryClientProvider client={queryClient}>
      {children}
    </QueryClientProvider>
  );
};

render(<GameDetails gameId={1} />, { wrapper: createWrapper() });
```

---

## References

- **Comprehensive Guide**: `/docs/features/STATE_MANAGEMENT.md` - Single source of truth
- **ADR**: `/docs-site/developer/architecture/adrs/005-frontend-state-management.md` - Architectural decisions

## Quick Checklist

- [ ] Use `useAuth()` hook for all user data
- [ ] Always check `isCheckingAuth` before conditional rendering
- [ ] Use React Query for all server state
- [ ] Never decode JWT client-side
- [ ] Use nullish coalescing (`??`) for user ID
- [ ] Invalidate queries after mutations
- [ ] Handle loading, error, and empty states
- [ ] Test components with mocked context/queries
